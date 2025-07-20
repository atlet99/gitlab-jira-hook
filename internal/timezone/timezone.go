// Package timezone provides timezone management utilities for the application.
package timezone

import (
	"fmt"
	"time"
)

// Manager manages timezone operations for the application
type Manager struct {
	timezone string
	location *time.Location
}

// NewManager creates a new timezone manager
func NewManager(timezone string) (*Manager, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("failed to load timezone %s: %w", timezone, err)
	}

	return &Manager{
		timezone: timezone,
		location: location,
	}, nil
}

// GetLocation returns the timezone location
func (tm *Manager) GetLocation() *time.Location {
	return tm.location
}

// GetTimezone returns the timezone string
func (tm *Manager) GetTimezone() string {
	return tm.timezone
}

// Now returns the current time in the configured timezone
func (tm *Manager) Now() time.Time {
	return time.Now().In(tm.location)
}

// ParseTime parses a time string in the configured timezone
func (tm *Manager) ParseTime(layout, value string) (time.Time, error) {
	return time.ParseInLocation(layout, value, tm.location)
}

// FormatTime formats a time in the configured timezone
func (tm *Manager) FormatTime(t time.Time, layout string) string {
	return t.In(tm.location).Format(layout)
}

// FormatTimeGOST formats a time according to GOST 7.64-90 standard
func (tm *Manager) FormatTimeGOST(t time.Time) string {
	return t.In(tm.location).Format("02.01.2006 15:04:05")
}

// FormatDateGOST formats a date according to GOST 7.64-90 standard
func (tm *Manager) FormatDateGOST(t time.Time) string {
	return t.In(tm.location).Format("02.01.2006")
}

// FormatDateTimeGOST formats date and time according to GOST 7.64-90 standard
func (tm *Manager) FormatDateTimeGOST(t time.Time) string {
	return t.In(tm.location).Format("02.01.2006 15:04:05")
}

// FormatTimeForJira formats time for Jira comments (RFC 3339)
func (tm *Manager) FormatTimeForJira(t time.Time) string {
	return t.In(tm.location).Format("2006-01-02T15:04:05-07:00")
}

// FormatTimeForLogs formats time for log entries
func (tm *Manager) FormatTimeForLogs(t time.Time) string {
	return t.In(tm.location).Format("2006-01-02 15:04:05.000")
}

// ConvertToTimezone converts a time to the configured timezone
func (tm *Manager) ConvertToTimezone(t time.Time) time.Time {
	return t.In(tm.location)
}

// IsValidTimezone checks if a timezone string is valid
func IsValidTimezone(timezone string) bool {
	_, err := time.LoadLocation(timezone)
	return err == nil
}

// GetCommonTimezones returns a list of common timezones
func GetCommonTimezones() map[string]string {
	return map[string]string{
		"UTC":                 "UTC",
		"Etc/GMT-5":           "UTC+5 (Kazakhstan Standard Time)",
		"Asia/Almaty":         "Asia/Almaty (Kazakhstan)",
		"Asia/Astana":         "Asia/Astana (Kazakhstan)",
		"Europe/Moscow":       "Europe/Moscow (MSK)",
		"America/New_York":    "America/New_York (EST/EDT)",
		"America/Los_Angeles": "America/Los_Angeles (PST/PDT)",
		"Europe/London":       "Europe/London (GMT/BST)",
		"Asia/Tokyo":          "Asia/Tokyo (JST)",
		"Asia/Shanghai":       "Asia/Shanghai (CST)",
	}
}
