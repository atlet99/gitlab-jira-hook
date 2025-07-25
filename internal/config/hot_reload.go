package config

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"log/slog"
)

const (
	// Default configuration values
	defaultCheckInterval = 30 * time.Second
	defaultMaxRetries    = 3
	defaultRetryDelay    = 5 * time.Second
)

// HotReloadConfig holds configuration for hot reload functionality
type HotReloadConfig struct {
	Enabled       bool          `json:"enabled"`
	ConfigFile    string        `json:"config_file"`
	CheckInterval time.Duration `json:"check_interval"`
	MaxRetries    int           `json:"max_retries"`
	RetryDelay    time.Duration `json:"retry_delay"`
}

// ConfigWatcher watches for configuration changes and triggers reloads.
// Note: This type name stutters with package name but is kept for API compatibility.
//
//nolint:revive // API compatibility
type ConfigWatcher struct {
	config     *HotReloadConfig
	logger     *slog.Logger
	reloadChan chan *Config
	stopChan   chan struct{}
	mu         sync.RWMutex
	currentCfg *Config
	fileInfo   os.FileInfo
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewConfigWatcher creates a new configuration watcher
func NewConfigWatcher(hotReloadCfg *HotReloadConfig, logger *slog.Logger) *ConfigWatcher {
	if hotReloadCfg == nil {
		hotReloadCfg = &HotReloadConfig{
			Enabled:       false,
			ConfigFile:    "",
			CheckInterval: defaultCheckInterval,
			MaxRetries:    defaultMaxRetries,
			RetryDelay:    defaultRetryDelay,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ConfigWatcher{
		config:     hotReloadCfg,
		logger:     logger,
		reloadChan: make(chan *Config, 1),
		stopChan:   make(chan struct{}),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start begins watching for configuration changes
func (cw *ConfigWatcher) Start(initialConfig *Config) error {
	if !cw.config.Enabled {
		cw.logger.Info("Hot reload disabled")
		return nil
	}

	cw.mu.Lock()
	cw.currentCfg = initialConfig
	cw.mu.Unlock()

	// Get initial file info if config file is specified
	if cw.config.ConfigFile != "" {
		if err := cw.updateFileInfo(); err != nil {
			return fmt.Errorf("failed to get initial file info: %w", err)
		}
	}

	cw.logger.Info("Starting configuration watcher",
		"config_file", cw.config.ConfigFile,
		"check_interval", cw.config.CheckInterval)

	go cw.watchLoop()

	return nil
}

// Stop stops the configuration watcher
func (cw *ConfigWatcher) Stop() {
	cw.logger.Info("Stopping configuration watcher")
	cw.cancel()
	close(cw.stopChan)
}

// GetReloadChannel returns the channel for configuration reloads
func (cw *ConfigWatcher) GetReloadChannel() <-chan *Config {
	return cw.reloadChan
}

// watchLoop continuously monitors for configuration changes
func (cw *ConfigWatcher) watchLoop() {
	ticker := time.NewTicker(cw.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := cw.checkForChanges(); err != nil {
				cw.logger.Error("Error checking for configuration changes", "error", err)
			}
		case <-cw.ctx.Done():
			return
		case <-cw.stopChan:
			return
		}
	}
}

// checkForChanges checks if configuration has changed and triggers reload if needed
func (cw *ConfigWatcher) checkForChanges() error {
	// Check file-based configuration
	if cw.config.ConfigFile != "" {
		if changed, err := cw.checkFileChanges(); err != nil {
			return err
		} else if changed {
			if err := cw.reloadFromFile(); err != nil {
				return err
			}
		}
	}

	// Check environment-based configuration
	if changed := cw.checkEnvironmentChanges(); changed {
		if err := cw.reloadFromEnvironment(); err != nil {
			return err
		}
	}

	return nil
}

// checkFileChanges checks if the configuration file has been modified
func (cw *ConfigWatcher) checkFileChanges() (bool, error) {
	fileInfo, err := os.Stat(cw.config.ConfigFile)
	if err != nil {
		return false, fmt.Errorf("failed to stat config file: %w", err)
	}

	cw.mu.RLock()
	oldFileInfo := cw.fileInfo
	cw.mu.RUnlock()

	if oldFileInfo == nil {
		// First time checking, just store the info
		cw.mu.Lock()
		cw.fileInfo = fileInfo
		cw.mu.Unlock()
		return false, nil
	}

	// Check if file has been modified
	if fileInfo.ModTime().After(oldFileInfo.ModTime()) || fileInfo.Size() != oldFileInfo.Size() {
		cw.logger.Info("Configuration file changed",
			"file", cw.config.ConfigFile,
			"old_mod_time", oldFileInfo.ModTime(),
			"new_mod_time", fileInfo.ModTime(),
			"old_size", oldFileInfo.Size(),
			"new_size", fileInfo.Size())

		cw.mu.Lock()
		cw.fileInfo = fileInfo
		cw.mu.Unlock()

		return true, nil
	}

	return false, nil
}

// checkEnvironmentChanges checks if environment variables have changed
func (cw *ConfigWatcher) checkEnvironmentChanges() bool {
	// For now, we'll use a simple approach: check if any relevant env vars have changed
	// In a more sophisticated implementation, you might want to hash the relevant env vars
	relevantVars := []string{
		"PORT", "LOG_LEVEL", "TIMEZONE",
		"GITLAB_SECRET", "GITLAB_BASE_URL",
		"JIRA_EMAIL", "JIRA_TOKEN", "JIRA_BASE_URL",
		"MIN_WORKERS", "MAX_WORKERS", "SCALE_INTERVAL",
	}

	cw.mu.RLock()
	oldConfig := cw.currentCfg
	cw.mu.RUnlock()

	if oldConfig == nil {
		return false
	}

	for _, envVar := range relevantVars {
		if oldValue := getEnvValue(envVar); oldValue != os.Getenv(envVar) {
			cw.logger.Info("Environment variable changed", "variable", envVar)
			return true
		}
	}

	return false
}

// reloadFromFile reloads configuration from file
func (cw *ConfigWatcher) reloadFromFile() error {
	cw.logger.Info("Reloading configuration from file", "file", cw.config.ConfigFile)

	// Load new configuration
	newConfig, err := LoadFromFile(cw.config.ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration from file: %w", err)
	}

	// Validate new configuration
	if err := newConfig.validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Update current configuration
	cw.mu.Lock()
	cw.currentCfg = newConfig
	cw.mu.Unlock()

	// Send reload signal
	select {
	case cw.reloadChan <- newConfig:
		cw.logger.Info("Configuration reloaded successfully")
	case <-cw.ctx.Done():
		return cw.ctx.Err()
	}

	return nil
}

// reloadFromEnvironment reloads configuration from environment variables
func (cw *ConfigWatcher) reloadFromEnvironment() error {
	cw.logger.Info("Reloading configuration from environment")

	// Load new configuration from environment
	newConfig := NewConfigFromEnv(cw.logger)

	// Update current configuration
	cw.mu.Lock()
	cw.currentCfg = newConfig
	cw.mu.Unlock()

	// Send reload signal
	select {
	case cw.reloadChan <- newConfig:
		cw.logger.Info("Configuration reloaded successfully from environment")
	case <-cw.ctx.Done():
		return cw.ctx.Err()
	}

	return nil
}

// updateFileInfo updates the stored file information
func (cw *ConfigWatcher) updateFileInfo() error {
	if cw.config.ConfigFile == "" {
		return nil
	}

	fileInfo, err := os.Stat(cw.config.ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to stat config file: %w", err)
	}

	cw.mu.Lock()
	cw.fileInfo = fileInfo
	cw.mu.Unlock()

	return nil
}

// LoadFromFile loads configuration from a specific file
func LoadFromFile(filePath string) (*Config, error) {
	// Set the file path for godotenv
	originalEnvFile := os.Getenv("ENV_FILE")
	if err := os.Setenv("ENV_FILE", filePath); err != nil {
		return nil, fmt.Errorf("failed to set ENV_FILE: %w", err)
	}

	// Load configuration using the existing logic
	config, err := Load()
	if err != nil {
		// Restore original ENV_FILE
		if originalEnvFile != "" {
			if restoreErr := os.Setenv("ENV_FILE", originalEnvFile); restoreErr != nil {
				return nil, fmt.Errorf("failed to restore ENV_FILE: %w", restoreErr)
			}
		} else {
			if unsetErr := os.Unsetenv("ENV_FILE"); unsetErr != nil {
				return nil, fmt.Errorf("failed to unset ENV_FILE: %w", unsetErr)
			}
		}
		return nil, err
	}

	// Restore original ENV_FILE
	if originalEnvFile != "" {
		if err := os.Setenv("ENV_FILE", originalEnvFile); err != nil {
			return nil, fmt.Errorf("failed to restore ENV_FILE: %w", err)
		}
	} else {
		if err := os.Unsetenv("ENV_FILE"); err != nil {
			return nil, fmt.Errorf("failed to unset ENV_FILE: %w", err)
		}
	}

	return config, nil
}

// getEnvValue gets the current value of an environment variable
func getEnvValue(key string) string {
	return os.Getenv(key)
}

// ConfigReloader provides a high-level interface for configuration reloading.
// Note: This type name stutters with package name but is kept for API compatibility.
//
//nolint:revive // API compatibility
type ConfigReloader struct {
	watcher  *ConfigWatcher
	logger   *slog.Logger
	handlers []ConfigReloadHandler
	mu       sync.RWMutex
}

// ConfigReloadHandler defines the interface for components that need to handle config reloads.
// Note: This type name stutters with package name but is kept for API compatibility.
//
//nolint:revive // API compatibility
type ConfigReloadHandler interface {
	OnConfigReload(newConfig *Config) error
}

// NewConfigReloader creates a new configuration reloader
func NewConfigReloader(hotReloadCfg *HotReloadConfig, logger *slog.Logger) *ConfigReloader {
	return &ConfigReloader{
		watcher:  NewConfigWatcher(hotReloadCfg, logger),
		logger:   logger,
		handlers: make([]ConfigReloadHandler, 0),
	}
}

// Start starts the configuration reloader
func (cr *ConfigReloader) Start(initialConfig *Config) error {
	if err := cr.watcher.Start(initialConfig); err != nil {
		return err
	}

	go cr.handleReloads()

	return nil
}

// Stop stops the configuration reloader
func (cr *ConfigReloader) Stop() {
	cr.watcher.Stop()
}

// RegisterHandler registers a handler for configuration reloads
func (cr *ConfigReloader) RegisterHandler(handler ConfigReloadHandler) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.handlers = append(cr.handlers, handler)
}

// handleReloads handles configuration reload events
func (cr *ConfigReloader) handleReloads() {
	for {
		select {
		case newConfig := <-cr.watcher.GetReloadChannel():
			cr.logger.Info("Processing configuration reload")

			cr.mu.RLock()
			handlers := make([]ConfigReloadHandler, len(cr.handlers))
			copy(handlers, cr.handlers)
			cr.mu.RUnlock()

			// Notify all handlers
			for _, handler := range handlers {
				if err := handler.OnConfigReload(newConfig); err != nil {
					cr.logger.Error("Handler failed to process config reload", "error", err)
				}
			}

		case <-cr.watcher.ctx.Done():
			return
		}
	}
}
