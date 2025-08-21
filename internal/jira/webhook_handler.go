// Package jira provides Jira webhook handling and GitLab synchronization.
package jira

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/monitoring"
	"github.com/atlet99/gitlab-jira-hook/internal/webhook"
)

// Context keys for user information
type contextKey string

// HTTP status constants
const (
	successStatusCode = 200
	requestIDLength   = 16
)

const (
	jiraUserIDKey    contextKey = "jira_user_id"
	jiraUsernameKey  contextKey = "jira_username"
	jiraClientKeyKey contextKey = "jira_client_key"
	requestIDKey     contextKey = "request_id"
)

// responseWriterWrapper captures the status code for audit logging
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code
func (rw *responseWriterWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures the status code if not already set
func (rw *responseWriterWrapper) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

// AuditEventType represents different types of audit events
type AuditEventType string

const (
	// AuditEventTypeAPIRequest represents API request events
	AuditEventTypeAPIRequest AuditEventType = "api_request"
	// AuditEventTypeAPIResponse represents API response events
	AuditEventTypeAPIResponse AuditEventType = "api_response"
	// AuditEventTypeAuthentication represents authentication events
	AuditEventTypeAuthentication AuditEventType = "authentication"
	// AuditEventTypeAuthorization represents authorization events
	AuditEventTypeAuthorization AuditEventType = "authorization"
	// AuditEventTypeDataChange represents data change events
	AuditEventTypeDataChange AuditEventType = "data_change"
	// AuditEventTypeSystemOperation represents system operation events
	AuditEventTypeSystemOperation AuditEventType = "system_operation"
	// AuditEventTypeError represents error events
	AuditEventTypeError AuditEventType = "error"
)

// String constants for audit logging
const (
	StatusSuccess = "success"
	StatusFailed  = "failed"
	RedactedText  = "***REDACTED***"
	ActionReopen  = "reopen"
)

// AuditEvent represents an audit log entry
type AuditEvent struct {
	ID           string                 `json:"id"`
	Timestamp    time.Time              `json:"timestamp"`
	EventType    AuditEventType         `json:"event_type"`
	Category     string                 `json:"category"`
	Action       string                 `json:"action"`
	Status       string                 `json:"status"`
	StatusCode   int                    `json:"status_code,omitempty"`
	UserID       string                 `json:"user_id,omitempty"`
	Username     string                 `json:"username,omitempty"`
	ClientIP     string                 `json:"client_ip,omitempty"`
	UserAgent    string                 `json:"user_agent,omitempty"`
	RequestID    string                 `json:"request_id,omitempty"`
	ResourceType string                 `json:"resource_type,omitempty"`
	ResourceID   string                 `json:"resource_id,omitempty"`
	Method       string                 `json:"method,omitempty"`
	Path         string                 `json:"path,omitempty"`
	QueryParams  map[string]interface{} `json:"query_params,omitempty"`
	RequestData  map[string]interface{} `json:"request_data,omitempty"`
	ResponseData map[string]interface{} `json:"response_data,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Duration     int64                  `json:"duration_ms,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// AuditLogger handles audit logging operations
type AuditLogger struct {
	logger *slog.Logger
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logger *slog.Logger) *AuditLogger {
	return &AuditLogger{
		logger: logger,
	}
}

// LogEvent logs an audit event
func (al *AuditLogger) LogEvent(event *AuditEvent) {
	// Convert event to structured log fields
	fields := make([]interface{}, 0)
	fields = append(fields,
		"audit_event", event.EventType,
		"audit_category", event.Category,
		"audit_action", event.Action,
		"audit_status", event.Status,
		"audit_timestamp", event.Timestamp,
		"audit_user_id", event.UserID,
		"audit_username", event.Username,
		"audit_client_ip", event.ClientIP,
		"audit_user_agent", event.UserAgent,
		"audit_request_id", event.RequestID,
		"audit_resource_type", event.ResourceType,
		"audit_resource_id", event.ResourceID,
		"audit_method", event.Method,
		"audit_path", event.Path,
		"audit_status_code", event.StatusCode,
		"audit_error", event.Error,
		"audit_duration_ms", event.Duration,
		"audit_tags", event.Tags,
	)

	// Add metadata as key-value pairs
	for k, v := range event.Metadata {
		fields = append(fields, fmt.Sprintf("audit_metadata_%s", k), v)
	}

	// Log the event
	switch event.Status {
	case StatusSuccess:
		al.logger.Info("Audit event", fields...)
	case StatusFailed:
		al.logger.Error("Audit event", fields...)
	default:
		al.logger.Info("Audit event", fields...)
	}
}

// LogAPIRequest logs an API request
func (al *AuditLogger) LogAPIRequest(
	r *http.Request,
	requestID, userID, username string,
	startTime time.Time,
	tags ...string,
) {
	event := &AuditEvent{
		ID:          generateRequestID(),
		Timestamp:   startTime,
		EventType:   AuditEventTypeAPIRequest,
		Category:    "api",
		Action:      "request",
		Status:      "success",
		UserID:      userID,
		Username:    username,
		ClientIP:    getClientIP(r),
		UserAgent:   r.UserAgent(),
		RequestID:   requestID,
		Method:      r.Method,
		Path:        r.URL.Path,
		QueryParams: extractQueryParams(r),
		Tags:        tags,
		Metadata:    make(map[string]interface{}),
	}

	al.LogEvent(event)
}

// LogAPIResponse logs an API response
func (al *AuditLogger) LogAPIResponse(
	r *http.Request,
	requestID, userID, username string,
	startTime time.Time,
	statusCode int,
	responseData interface{},
	tags ...string,
) {
	duration := time.Since(startTime).Milliseconds()

	event := &AuditEvent{
		ID:         generateRequestID(),
		Timestamp:  time.Now(),
		EventType:  AuditEventTypeAPIResponse,
		Category:   "api",
		Action:     "response",
		Status:     getResponseStatus(statusCode),
		StatusCode: statusCode,
		UserID:     userID,
		Username:   username,
		ClientIP:   getClientIP(r),
		UserAgent:  r.UserAgent(),
		RequestID:  requestID,
		Method:     r.Method,
		Path:       r.URL.Path,
		Duration:   duration,
		Tags:       tags,
		Metadata:   make(map[string]interface{}),
	}

	if responseData != nil {
		event.ResponseData = sanitizeResponseData(responseData)
	}

	al.LogEvent(event)
}

// LogAuthentication logs authentication events
func (al *AuditLogger) LogAuthentication(
	r *http.Request,
	requestID, authType, userID, username string,
	success bool,
	errorDetails string,
) {
	event := &AuditEvent{
		ID:        generateRequestID(),
		Timestamp: time.Now(),
		EventType: AuditEventTypeAuthentication,
		Category:  "security",
		Action:    authType,
		Status:    getAuthStatus(success),
		UserID:    userID,
		Username:  username,
		ClientIP:  getClientIP(r),
		UserAgent: r.UserAgent(),
		RequestID: requestID,
		Error:     errorDetails,
		Metadata: map[string]interface{}{
			"auth_type": authType,
		},
	}

	al.LogEvent(event)
}

// LogAuthorization logs authorization events
func (al *AuditLogger) LogAuthorization(
	r *http.Request,
	requestID, resourceType, resourceID, action string,
	success bool,
	errorDetails string,
) {
	event := &AuditEvent{
		ID:           generateRequestID(),
		Timestamp:    time.Now(),
		EventType:    AuditEventTypeAuthorization,
		Category:     "security",
		Action:       action,
		Status:       getAuthStatus(success),
		UserID:       getUserIDFromContext(r.Context()),
		ClientIP:     getClientIP(r),
		UserAgent:    r.UserAgent(),
		RequestID:    requestID,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Error:        errorDetails,
		Metadata: map[string]interface{}{
			"resource_type": resourceType,
			"resource_id":   resourceID,
			"action":        action,
		},
	}

	al.LogEvent(event)
}

// LogDataChange logs data change events
func (al *AuditLogger) LogDataChange(
	r *http.Request,
	requestID, resourceType, resourceID, action string,
	changes map[string]interface{},
	userID, username string,
) {
	event := &AuditEvent{
		ID:           generateRequestID(),
		Timestamp:    time.Now(),
		EventType:    AuditEventTypeDataChange,
		Category:     "data",
		Action:       action,
		Status:       "success",
		UserID:       userID,
		Username:     username,
		ClientIP:     getClientIP(r),
		UserAgent:    r.UserAgent(),
		RequestID:    requestID,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		RequestData:  sanitizeSensitiveData(changes).(map[string]interface{}),
		Metadata: map[string]interface{}{
			"resource_type": resourceType,
			"resource_id":   resourceID,
			"action":        action,
		},
	}

	al.LogEvent(event)
}

// LogSystemOperation logs system operation events
func (al *AuditLogger) LogSystemOperation(
	operation string,
	success bool,
	errorDetails string,
	metadata map[string]interface{},
) {
	event := &AuditEvent{
		ID:        generateRequestID(),
		Timestamp: time.Now(),
		EventType: AuditEventTypeSystemOperation,
		Category:  "system",
		Action:    operation,
		Status:    getOperationStatus(success),
		Error:     errorDetails,
		Metadata:  metadata,
	}

	al.LogEvent(event)
}

// LogError logs error events
func (al *AuditLogger) LogError(r *http.Request, requestID, errorType, errorDetails string, tags ...string) {
	event := &AuditEvent{
		ID:        generateRequestID(),
		Timestamp: time.Now(),
		EventType: AuditEventTypeError,
		Category:  "error",
		Action:    errorType,
		Status:    "failed",
		RequestID: requestID,
		Error:     errorDetails,
		Tags:      tags,
		Metadata: map[string]interface{}{
			"error_type": errorType,
		},
	}

	// Only set request-specific fields if request is not nil
	if r != nil {
		event.UserID = getUserIDFromContext(r.Context())
		event.ClientIP = getClientIP(r)
		event.UserAgent = r.UserAgent()
	}

	al.LogEvent(event)
}

// Helper functions

func getResponseStatus(statusCode int) string {
	if statusCode >= 200 && statusCode < 300 {
		return StatusSuccess
	}
	return StatusFailed
}

func getAuthStatus(success bool) string {
	if success {
		return "success"
	}
	return "failed"
}

func getOperationStatus(success bool) string {
	if success {
		return "success"
	}
	return "failed"
}

func getClientIP(r *http.Request) string {
	// Check for X-Forwarded-For header first (for reverse proxies)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check for X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

func extractQueryParams(r *http.Request) map[string]interface{} {
	params := make(map[string]interface{})

	for key, values := range r.URL.Query() {
		if len(values) == 1 {
			params[key] = values[0]
		} else {
			params[key] = values
		}
	}

	return params
}

func sanitizeResponseData(data interface{}) map[string]interface{} {
	// This is a basic implementation - in production, you'd want more sophisticated sanitization
	result := make(map[string]interface{})

	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			// Mask sensitive fields
			if isSensitiveField(key) {
				result[key] = RedactedText
			} else {
				result[key] = value
			}
		}
	case string:
		// If it's a string, check if it contains sensitive information
		if containsSensitiveData(v) {
			result["data"] = "***REDACTED***"
		} else {
			result["data"] = v
		}
	default:
		result["data"] = fmt.Sprintf("%v", v)
	}

	return result
}

func sanitizeSensitiveData(data interface{}) interface{} {
	// This is a basic implementation - in production, you'd want more sophisticated sanitization
	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			if isSensitiveField(key) {
				result[key] = RedactedText
			} else {
				result[key] = sanitizeSensitiveData(value)
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = sanitizeSensitiveData(item)
		}
		return result
	default:
		return v
	}
}

func isSensitiveField(field string) bool {
	sensitiveFields := []string{
		"password", "token", "secret", "key", "auth", "credential",
		"email", "phone", "address", "ssn", "credit_card",
	}

	fieldLower := strings.ToLower(field)
	for _, sensitiveField := range sensitiveFields {
		if strings.Contains(fieldLower, sensitiveField) {
			return true
		}
	}

	return false
}

func containsSensitiveData(data string) bool {
	sensitivePatterns := []string{
		"password=", "token=", "secret=", "key=",
		"@", "phone:", "address:",
	}

	dataLower := strings.ToLower(data)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(dataLower, pattern) {
			return true
		}
	}

	return false
}

func getUserIDFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value(jiraUserIDKey).(string); ok {
		return userID
	}
	return ""
}

// APIErrorResponse represents a standardized API error response
type APIErrorResponse struct {
	Success   bool        `json:"success"`
	Error     string      `json:"error"`
	ErrorCode string      `json:"error_code,omitempty"`
	Message   string      `json:"message,omitempty"`
	Details   interface{} `json:"details,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// ErrorType represents different types of errors
type ErrorType string

const (
	// ErrorTypeValidation represents validation errors
	ErrorTypeValidation ErrorType = "validation"
	// ErrorTypeAuthentication represents authentication errors
	ErrorTypeAuthentication ErrorType = "authentication"
	// ErrorTypeAuthorization represents authorization errors
	ErrorTypeAuthorization ErrorType = "authorization"
	// ErrorTypeNotFound represents not found errors
	ErrorTypeNotFound ErrorType = "not_found"
	// ErrorTypeConflict represents conflict errors
	ErrorTypeConflict ErrorType = "conflict"
	// ErrorTypeRateLimit represents rate limit errors
	ErrorTypeRateLimit ErrorType = "rate_limit"
	// ErrorTypeServer represents server errors
	ErrorTypeServer ErrorType = "server"
	// ErrorTypeClient represents client errors
	ErrorTypeClient ErrorType = "client"
	// ErrorTypeTimeout represents timeout errors
	ErrorTypeTimeout ErrorType = "timeout"
)

// APIError represents a detailed API error
type APIError struct {
	Type       ErrorType `json:"type"`
	Code       string    `json:"code"`
	Message    string    `json:"message"`
	Details    string    `json:"details,omitempty"`
	Suggestion string    `json:"suggestion,omitempty"`
	Retryable  bool      `json:"retryable"`
	Status     int       `json:"status"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s (code: %s, status: %d)", e.Type, e.Message, e.Code, e.Status)
}

// NewAPIError creates a new API error with the specified parameters
func NewAPIError(errorType ErrorType, code, message string, status int) *APIError {
	return &APIError{
		Type:      errorType,
		Code:      code,
		Message:   message,
		Status:    status,
		Retryable: isRetryableError(errorType),
	}
}

// NewAPIErrorWithDetails creates a new API error with additional details
func NewAPIErrorWithDetails(errorType ErrorType, code, message, details string, status int) *APIError {
	return &APIError{
		Type:      errorType,
		Code:      code,
		Message:   message,
		Details:   details,
		Status:    status,
		Retryable: isRetryableError(errorType),
	}
}

// isRetryableError determines if an error type is retryable
func isRetryableError(errorType ErrorType) bool {
	retryableTypes := []ErrorType{ErrorTypeRateLimit, ErrorTypeTimeout, ErrorTypeServer}
	for _, t := range retryableTypes {
		if t == errorType {
			return true
		}
	}
	return false
}

// WriteErrorResponse writes a standardized error response to the HTTP response writer
func WriteErrorResponse(w http.ResponseWriter, r *http.Request, apiError *APIError) {
	var requestID string
	if r != nil {
		requestID = r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
	} else {
		requestID = generateRequestID()
	}

	response := APIErrorResponse{
		Success:   false,
		Error:     apiError.Message,
		ErrorCode: apiError.Code,
		Message:   apiError.Message,
		Details:   apiError.Details,
		RequestID: requestID,
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(apiError.Status)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("Failed to encode error response", "error", err, "request_id", requestID)
	}
}

// WriteSuccessResponse writes a standardized success response
func WriteSuccessResponse(w http.ResponseWriter, r *http.Request, data interface{}) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = generateRequestID()
	}

	response := APIErrorResponse{
		Success:   true,
		Error:     "",
		ErrorCode: "",
		Message:   "Request processed successfully",
		Details:   data,
		RequestID: requestID,
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("Failed to encode success response", "error", err, "request_id", requestID)
	}
}

// HandleErrorWithMetrics handles an error and records metrics
func HandleErrorWithMetrics(r *http.Request, apiError *APIError, start time.Time, monitor *monitoring.WebhookMonitor) {
	if r == nil || apiError == nil {
		return
	}

	requestPath := r.URL.Path
	requestMethod := r.Method

	// Record error metrics
	if monitor != nil {
		monitor.RecordRequest(requestPath, false, time.Since(start))
	}

	slog.Error("API request failed",
		"method", requestMethod,
		"path", requestPath,
		"error_type", apiError.Type,
		"error_code", apiError.Code,
		"error_message", apiError.Message,
		"status", apiError.Status,
		"retryable", apiError.Retryable,
		"request_id", r.Header.Get("X-Request-ID"),
		"user_agent", r.UserAgent(),
		"remote_addr", r.RemoteAddr,
	)
}

// WrapHandler wraps an HTTP handler with enhanced error handling
func WrapHandler(handler http.HandlerFunc, monitor *monitoring.WebhookMonitor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := generateRequestID()

		// Add request ID to context
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		r = r.WithContext(ctx)

		// Add request ID to response headers
		w.Header().Set("X-Request-ID", requestID)

		// Create a response writer that captures the status code
		rw := &responseWriterWrapper{ResponseWriter: w, statusCode: http.StatusOK}

		// Execute the handler
		handler.ServeHTTP(rw, r)

		// Record metrics if monitor is available
		if monitor != nil {
			monitor.RecordRequest(r.URL.Path, rw.statusCode < successStatusCode, time.Since(start))
		}

		// Log request completion
		slog.Info("Request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration", time.Since(start),
			"request_id", requestID,
			"user_agent", r.UserAgent(),
			"remote_addr", r.RemoteAddr)
	}
}

// RecoverPanic recovers from panics and returns a proper API error
func RecoverPanic(r *http.Request) *APIError {
	if panicValue := recover(); panicValue != nil {
		// Extract request ID if available, otherwise use a generic one
		var requestID string
		if r != nil {
			requestID = r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = generateRequestID()
			}
		} else {
			requestID = generateRequestID()
		}

		slog.Error("Recovered from panic", "panic", panicValue, "request_id", requestID)
		return NewAPIError(
			ErrorTypeServer,
			"internal_server_error",
			"an unexpected error occurred",
			http.StatusInternalServerError,
		)
	}
	return nil
}

// GitLabAPIClient interface for GitLab API operations
type GitLabAPIClient interface {
	CreateIssue(ctx context.Context, projectID string, request *GitLabIssueCreateRequest) (*GitLabIssue, error)
	UpdateIssue(ctx context.Context, projectID string, issueIID int,
		request *GitLabIssueUpdateRequest) (*GitLabIssue, error)
	GetIssue(ctx context.Context, projectID string, issueIID int) (*GitLabIssue, error)
	CreateComment(ctx context.Context, projectID string, issueIID int,
		request *GitLabCommentCreateRequest) (*GitLabComment, error)
	SearchIssuesByTitle(ctx context.Context, projectID, title string) ([]*GitLabIssue, error)
	FindUserByEmail(ctx context.Context, email string) (*GitLabUser, error)
	TestConnection(ctx context.Context) error
	// Milestone management methods
	GetMilestones(ctx context.Context, projectID string) ([]*GitLabMilestone, error)
	CreateMilestone(ctx context.Context, projectID string, request *GitLabMilestoneCreateRequest) (*GitLabMilestone, error)
	UpdateMilestone(ctx context.Context, projectID string, milestoneID int,
		request *GitLabMilestoneUpdateRequest) (*GitLabMilestone, error)
	DeleteMilestone(ctx context.Context, projectID string, milestoneID int) error
}

// Manager interface for bidirectional synchronization
type Manager interface {
	SyncJiraToGitLab(ctx context.Context, event *WebhookEvent) error
}

// GitLabIssueCreateRequest represents a request to create a GitLab issue
type GitLabIssueCreateRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	AssigneeID  int      `json:"assignee_id,omitempty"`
	MilestoneID int      `json:"milestone_id,omitempty"`
}

// GitLabIssueUpdateRequest represents a request to update a GitLab issue
type GitLabIssueUpdateRequest struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	AssigneeID  *int     `json:"assignee_id,omitempty"`
	MilestoneID *int     `json:"milestone_id,omitempty"`
	StateEvent  string   `json:"state_event,omitempty"`
}

// GitLabIssue represents a GitLab issue response
type GitLabIssue struct {
	ID          int         `json:"id"`
	IID         int         `json:"iid"`
	ProjectID   int         `json:"project_id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	State       string      `json:"state"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	ClosedAt    *time.Time  `json:"closed_at"`
	Labels      []string    `json:"labels"`
	Author      *GitLabUser `json:"author"`
	Assignee    *GitLabUser `json:"assignee"`
	WebURL      string      `json:"web_url"`
}

// GitLabUser represents a GitLab user
type GitLabUser struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	WebURL    string `json:"web_url"`
}

// GitLabCommentCreateRequest represents a request to create a comment
type GitLabCommentCreateRequest struct {
	Body string `json:"body"`
}

// GitLabComment represents a GitLab comment/note
type GitLabComment struct {
	ID        int         `json:"id"`
	Body      string      `json:"body"`
	Author    *GitLabUser `json:"author"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
	System    bool        `json:"system"`
	WebURL    string      `json:"web_url"`
}

// GitLabMilestone represents a GitLab milestone
type GitLabMilestone struct {
	ID          int        `json:"id"`
	IID         int        `json:"iid"`
	ProjectID   int        `json:"project_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	DueDate     *time.Time `json:"due_date"`
	StartDate   *time.Time `json:"start_date"`
	State       string     `json:"state"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	WebURL      string     `json:"web_url"`
}

// GitLabMilestoneCreateRequest represents a request to create a milestone
type GitLabMilestoneCreateRequest struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	DueDate     *time.Time `json:"due_date"`
	StartDate   *time.Time `json:"start_date"`
}

// GitLabMilestoneUpdateRequest represents a request to update a milestone
type GitLabMilestoneUpdateRequest struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	DueDate     *time.Time `json:"due_date"`
	StartDate   *time.Time `json:"start_date"`
	StateEvent  string     `json:"state_event"`
}

// WebhookHandler handles incoming Jira webhook requests
type WebhookHandler struct {
	config        *config.Config
	logger        *slog.Logger
	jiraClient    *Client
	gitlabClient  GitLabAPIClient
	monitor       *monitoring.WebhookMonitor
	workerPool    webhook.WorkerPoolInterface
	eventCache    map[string]*WebhookEvent // Cache for passing events to async processing
	authValidator *AuthValidator
	syncManager   Manager // Bidirectional sync manager
	auditLogger   *AuditLogger
	// jqlFilter     *JQLFilter // JQL-based event filter - TODO: implement JQL filtering
}

// NewWebhookHandler creates a new Jira webhook handler
func NewWebhookHandler(cfg *config.Config, logger *slog.Logger) *WebhookHandler {
	// Initialize auth validator with HMAC secret and development mode
	authValidator := NewAuthValidator(logger, cfg.JiraWebhookSecret, cfg.DebugMode)

	// Initialize audit logger
	auditLogger := NewAuditLogger(logger)

	return &WebhookHandler{
		config:        cfg,
		logger:        logger,
		eventCache:    make(map[string]*WebhookEvent),
		authValidator: authValidator,
		auditLogger:   auditLogger,
	}
}

// SetJiraClient sets the Jira API client
func (h *WebhookHandler) SetJiraClient(client *Client) {
	h.jiraClient = client
}

// SetAuditLogger sets the audit logger for the webhook handler
func (h *WebhookHandler) SetAuditLogger(auditLogger *AuditLogger) {
	h.auditLogger = auditLogger
}

// SetGitLabClient sets the GitLab API client
func (h *WebhookHandler) SetGitLabClient(client GitLabAPIClient) {
	h.gitlabClient = client
}

// SetMonitor sets the webhook monitor for metrics recording
func (h *WebhookHandler) SetMonitor(monitor *monitoring.WebhookMonitor) {
	h.monitor = monitor
}

// SetManager sets the bidirectional sync manager
func (h *WebhookHandler) SetManager(syncManager Manager) {
	h.syncManager = syncManager
}

// SetWorkerPool sets the worker pool for async processing
func (h *WebhookHandler) SetWorkerPool(workerPool webhook.WorkerPoolInterface) {
	h.workerPool = workerPool
}

// GetAuthValidator returns the auth validator for configuration
func (h *WebhookHandler) GetAuthValidator() *AuthValidator {
	return h.authValidator
}

// ConfigureJWTValidation configures JWT validation for Connect apps
func (h *WebhookHandler) ConfigureJWTValidation(expectedAudience string, allowedIssuers []string) {
	if h.authValidator != nil {
		h.authValidator = h.authValidator.WithJWTValidator(expectedAudience, allowedIssuers)
	}
}

// HandleWebhook handles incoming Jira webhook requests with enhanced error handling and audit logging
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// Wrap the handler with enhanced error handling
	handler := func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		success := false
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		// Log API request
		userID := getUserIDFromContext(r.Context())
		username := getUsernameFromContext(r.Context())
		h.auditLogger.LogAPIRequest(r, requestID, userID, username, start, "webhook")

		defer func() {
			// Record metrics if monitor is available
			if h.monitor != nil {
				h.monitor.RecordRequest("/jira-webhook", success, time.Since(start))
			}

			// Log API response
			statusCode := getStatusCodeFromResponseWriter(w)
			responseData := map[string]interface{}{
				"event": r.Header.Get("X-Event-Type"),
			}
			h.auditLogger.LogAPIResponse(r, requestID, userID, username, start, statusCode, responseData, "webhook")
		}()

		// Recover from panics
		if apiError := RecoverPanic(r); apiError != nil {
			HandleErrorWithMetrics(r, apiError, start, h.monitor)
			WriteErrorResponse(w, r, apiError)
			h.auditLogger.LogError(r, requestID, "panic_recovery", apiError.Error(), "webhook", "error")
			return
		}

		// Validate request method
		if err := h.validateRequestMethod(w, r, start, requestID); err != nil {
			return
		}

		// Read and validate request body
		body, err := h.readAndValidateRequestBody(w, r, start, requestID)
		if err != nil {
			return
		}

		// Validate authentication
		authResult, err := h.validateAuthentication(w, r, body, start, requestID)
		if err != nil {
			return
		}

		// Add authentication context
		h.addAuthenticationContext(r, authResult)

		// Parse webhook event
		event, err := h.parseWebhookEvent(body)
		if err != nil {
			h.handleParseError(w, err, start, requestID)
			return
		}

		// Log webhook event for debugging
		if h.config.DebugMode {
			h.logWebhookEvent(r, event)
		}

		// Log data change event
		h.logWebhookProcessing(r, event, requestID, authResult)

		// Submit job for async processing
		if err := h.submitAsyncJob(w, event, requestID); err != nil {
			return
		}

		h.logger.Info("Jira webhook job submitted for async processing",
			"eventType", event.WebhookEvent,
			"request_id", requestID)

		// Return success immediately - processing will happen asynchronously
		WriteSuccessResponse(w, r, map[string]string{
			"status":  "accepted",
			"message": "jira webhook queued for processing",
			"event":   event.WebhookEvent,
		})

		success = true
	}

	// Wrap the handler with enhanced error handling
	WrapHandler(handler, h.monitor).ServeHTTP(w, r)
}

// validateRequestMethod validates the HTTP request method
func (h *WebhookHandler) validateRequestMethod(
	w http.ResponseWriter,
	r *http.Request,
	start time.Time,
	requestID string,
) error {
	if r.Method != http.MethodPost {
		apiError := NewAPIError(
			ErrorTypeClient,
			"method_not_allowed",
			"only post method is allowed",
			http.StatusMethodNotAllowed,
		)
		HandleErrorWithMetrics(r, apiError, start, h.monitor)
		WriteErrorResponse(w, r, apiError)
		h.auditLogger.LogError(r, requestID, "method_not_allowed", apiError.Error(), "webhook")
		return apiError
	}
	return nil
}

// readAndValidateRequestBody reads and validates the request body
func (h *WebhookHandler) readAndValidateRequestBody(
	w http.ResponseWriter,
	r *http.Request,
	start time.Time,
	requestID string,
) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		apiError := NewAPIErrorWithDetails(
			ErrorTypeClient,
			"read_failed",
			"failed to read request body",
			err.Error(),
			http.StatusBadRequest,
		)
		HandleErrorWithMetrics(r, apiError, start, h.monitor)
		WriteErrorResponse(w, r, apiError)
		h.auditLogger.LogError(r, requestID, "read_failed", apiError.Error(), "webhook")
		return nil, apiError
	}
	if closeErr := r.Body.Close(); closeErr != nil {
		h.logger.Warn("Failed to close request body", "error", closeErr)
	}
	return body, nil
}

// validateAuthentication validates the request authentication
func (h *WebhookHandler) validateAuthentication(
	w http.ResponseWriter,
	r *http.Request,
	body []byte,
	start time.Time,
	requestID string,
) (*AuthValidationResult, error) {
	// Validate authentication (HMAC signature or JWT)
	authResult := h.authValidator.ValidateRequest(r.Context(), r, body)
	h.authValidator.LogAuthenticationAttempt(r, authResult)

	// Log authentication event
	authType := "unknown"
	if authResult.AuthType != AuthTypeNone {
		authType = string(authResult.AuthType)
	}
	h.auditLogger.LogAuthentication(r, requestID, authType, authResult.UserID, "", authResult.Valid, "")

	if !authResult.Valid {
		authError := h.authValidator.CreateAuthError(authResult, "webhook processing")
		apiError := NewAPIErrorWithDetails(
			ErrorTypeAuthentication,
			"auth_failed",
			"authentication failed",
			authError.Error(),
			http.StatusUnauthorized,
		)
		apiError.Suggestion = "Please check your webhook secret configuration and ensure the request is properly signed."
		HandleErrorWithMetrics(r, apiError, start, h.monitor)
		WriteErrorResponse(w, r, apiError)
		h.auditLogger.LogError(r, requestID, "authentication_failed", apiError.Error(), "webhook")
		return nil, apiError
	}

	return authResult, nil
}

// addAuthenticationContext adds authentication context to the request
func (h *WebhookHandler) addAuthenticationContext(r *http.Request, authResult *AuthValidationResult) {
	authContext := h.authValidator.GetUserContext(authResult)
	if authResult.UserID != "" {
		r = r.WithContext(context.WithValue(r.Context(), jiraUserIDKey, authResult.UserID))
	}
	if authResult.IssuerClientKey != "" {
		_ = r.WithContext(context.WithValue(r.Context(), jiraClientKeyKey, authResult.IssuerClientKey))
	}

	h.logger.Info("Authentication successful",
		"auth_type", authResult.AuthType,
		"auth_context", authContext)
}

// handleParseError handles webhook event parsing errors
func (h *WebhookHandler) handleParseError(w http.ResponseWriter, err error, start time.Time, requestID string) {
	apiError := NewAPIErrorWithDetails(
		ErrorTypeValidation,
		"parse_failed",
		"failed to parse jira webhook event",
		err.Error(),
		http.StatusBadRequest,
	)
	apiError.Suggestion = "please ensure the webhook event payload is valid json " +
		"and matches the expected jira webhook format."
	HandleErrorWithMetrics(nil, apiError, start, h.monitor)
	WriteErrorResponse(w, nil, apiError)
	h.auditLogger.LogError(nil, requestID, "parse_failed", apiError.Error(), "webhook")
}

// logWebhookProcessing logs webhook processing data change event
func (h *WebhookHandler) logWebhookProcessing(
	r *http.Request,
	event *WebhookEvent,
	requestID string,
	authResult *AuthValidationResult,
) {
	webhookData := map[string]interface{}{
		"event_type": event.WebhookEvent,
		"issue_key":  getIssueKey(event),
		"timestamp":  event.Timestamp,
	}
	h.auditLogger.LogDataChange(
		r,
		requestID,
		"webhook",
		getIssueKey(event),
		"process",
		webhookData,
		authResult.UserID,
		"",
	)
}

// submitAsyncJob submits the webhook job for async processing
func (h *WebhookHandler) submitAsyncJob(w http.ResponseWriter, event *WebhookEvent, requestID string) error {
	// Convert to interface event for async processing
	interfaceEvent := h.convertToInterfaceEvent(event)

	// Submit job for async processing
	if h.workerPool == nil {
		apiError := NewAPIError(
			ErrorTypeServer,
			"worker_pool_unavailable",
			"worker pool not initialized",
			http.StatusServiceUnavailable,
		)
		apiError.Suggestion = "The service is currently unavailable. Please try again later."
		HandleErrorWithMetrics(nil, apiError, time.Now(), h.monitor)
		WriteErrorResponse(w, nil, apiError)
		h.auditLogger.LogError(nil, requestID, "worker_pool_unavailable", apiError.Error(), "webhook")
		return apiError
	}

	if err := h.workerPool.SubmitJob(interfaceEvent, h); err != nil {
		apiError := NewAPIErrorWithDetails(
			ErrorTypeServer,
			"job_submission_failed",
			"failed to submit job to worker pool",
			err.Error(),
			http.StatusInternalServerError,
		)
		apiError.Suggestion = "The webhook could not be processed. Please try again later."
		HandleErrorWithMetrics(nil, apiError, time.Now(), h.monitor)
		WriteErrorResponse(w, nil, apiError)
		h.auditLogger.LogError(nil, requestID, "job_submission_failed", apiError.Error(), "webhook")
		return apiError
	}

	return nil
}

// Helper functions for audit logging

func getStatusCodeFromResponseWriter(w http.ResponseWriter) int {
	if rw, ok := w.(*responseWriterWrapper); ok {
		return rw.statusCode
	}
	return http.StatusOK
}

func getUsernameFromContext(ctx context.Context) string {
	if username, ok := ctx.Value(jiraUsernameKey).(string); ok {
		return username
	}
	return ""
}

// generateRequestID generates a unique request ID for audit logging
func generateRequestID() string {
	bytes := make([]byte, requestIDLength)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random generation fails
		return fmt.Sprintf("req_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("req_%s", hex.EncodeToString(bytes))
}

// HandleGitLabWebhook handles incoming GitLab webhook requests for bidirectional sync
func (h *WebhookHandler) HandleGitLabWebhook(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	success := false

	defer func() {
		// Record metrics if monitor is available
		if h.monitor != nil {
			h.monitor.RecordRequest("/gitlab-webhook", success, time.Since(start))
		}
	}()

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("failed to read GitLab webhook body", "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if closeErr := r.Body.Close(); closeErr != nil {
		h.logger.Warn("Failed to close GitLab request body", "error", closeErr)
	}

	// Parse GitLab webhook event
	var gitlabEvent GitLabWebhookEvent
	if err := json.Unmarshal(body, &gitlabEvent); err != nil {
		h.logger.Error("failed to parse GitLab webhook event", "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	h.logger.Info("Processing GitLab webhook",
		"event_type", gitlabEvent.ObjectKind,
		"action", gitlabEvent.ObjectAttributes.Action)

	// Only process issue events for status sync
	if gitlabEvent.ObjectKind != "issue" {
		h.logger.Debug("Skipping non-issue GitLab event", "kind", gitlabEvent.ObjectKind)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Process the GitLab issue event
	if err := h.processGitLabIssueEvent(r.Context(), &gitlabEvent); err != nil {
		h.logger.Error("failed to process GitLab issue event", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"processed","message":"gitlab webhook processed"}`)); err != nil {
		h.logger.Error("Failed to write response", "error", err)
		return
	}

	success = true
}

// GitLabWebhookEvent represents a GitLab webhook event
type GitLabWebhookEvent struct {
	ObjectKind       string           `json:"object_kind"`
	ObjectAttributes GitLabIssueEvent `json:"object_attributes"`
	User             GitLabUser       `json:"user"`
	Project          GitLabProject    `json:"project"`
}

// GitLabIssueEvent represents GitLab issue event attributes
type GitLabIssueEvent struct {
	ID          int    `json:"id"`
	IID         int    `json:"iid"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"`
	Action      string `json:"action"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// GitLabProject represents GitLab project information
type GitLabProject struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	NameWithNamespace string `json:"name_with_namespace"`
	WebURL            string `json:"web_url"`
}

// processGitLabIssueEvent processes GitLab issue events for bidirectional sync
func (h *WebhookHandler) processGitLabIssueEvent(ctx context.Context, event *GitLabWebhookEvent) error {
	// Only process issue update/close/reopen events
	if event.ObjectAttributes.Action != "update" &&
		event.ObjectAttributes.Action != ActionClose &&
		event.ObjectAttributes.Action != ActionReopen {
		return nil
	}

	h.logger.Info("Processing GitLab issue state change",
		"projectID", event.Project.ID,
		"issueIID", event.ObjectAttributes.IID,
		"state", event.ObjectAttributes.State,
		"action", event.ObjectAttributes.Action)

	// Find the corresponding Jira issue
	jiraKey := h.extractJiraKeyFromTitle(event.ObjectAttributes.Title)
	if jiraKey == "" {
		h.logger.Debug("No Jira key found in GitLab issue title", "title", event.ObjectAttributes.Title)
		return nil
	}

	// Get the corresponding Jira issue
	jiraClient := h.getJiraClient()
	if jiraClient == nil {
		return fmt.Errorf("jira client not available")
	}

	jiraIssue, err := h.getJiraClient().GetIssue(ctx, jiraKey)
	if err != nil {
		return fmt.Errorf("failed to get Jira issue: %w", err)
	}

	// Determine target Jira status based on GitLab state
	targetJiraStatus := h.getJiraStatusFromGitLabState(event.ObjectAttributes.State, jiraIssue)
	if targetJiraStatus == "" {
		return nil // No status change needed
	}

	// Check if status actually changed
	if jiraIssue.Fields.Status != nil && jiraIssue.Fields.Status.Name == targetJiraStatus {
		return nil // Status already matches
	}

	// Update Jira issue status using proper workflow transitions
	if err := h.getJiraClient().TransitionToStatus(ctx, jiraKey, targetJiraStatus); err != nil {
		return fmt.Errorf("failed to transition Jira issue status: %w", err)
	}

	h.logger.Info("Synced GitLab status to Jira",
		"jiraIssueKey", jiraKey,
		"gitlabState", event.ObjectAttributes.State,
		"jiraTargetStatus", targetJiraStatus)

	return nil
}

// parseWebhookEvent parses the Jira webhook event from JSON
func (h *WebhookHandler) parseWebhookEvent(body []byte) (*WebhookEvent, error) {
	var event WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Jira webhook event: %w", err)
	}

	// Validate required fields
	if event.WebhookEvent == "" {
		return nil, fmt.Errorf("webhookEvent field is required")
	}

	// For issue-related events, validate that issue data is present
	if strings.HasPrefix(event.WebhookEvent, "jira:issue_") ||
		event.WebhookEvent == "comment_created" ||
		event.WebhookEvent == "comment_updated" ||
		event.WebhookEvent == "comment_deleted" ||
		event.WebhookEvent == "worklog_created" ||
		event.WebhookEvent == "worklog_updated" ||
		event.WebhookEvent == "worklog_deleted" {
		if event.Issue == nil {
			return nil, fmt.Errorf("issue data is required for webhook event: %s", event.WebhookEvent)
		}
	}

	// For comment events, validate that comment data is present
	if strings.HasPrefix(event.WebhookEvent, "comment_") {
		if event.Comment == nil {
			return nil, fmt.Errorf("comment data is required for webhook event: %s", event.WebhookEvent)
		}
	}

	// For worklog events, validate that worklog data is present
	if strings.HasPrefix(event.WebhookEvent, "worklog_") {
		if event.Worklog == nil {
			return nil, fmt.Errorf("worklog data is required for webhook event: %s", event.WebhookEvent)
		}
	}

	return &event, nil
}

// logWebhookEvent logs webhook event for debugging
func (h *WebhookHandler) logWebhookEvent(r *http.Request, event *WebhookEvent) {
	h.logger.Debug("Received Jira webhook",
		"eventType", event.WebhookEvent,
		"timestamp", event.Timestamp,
		"userAgent", r.Header.Get("User-Agent"),
		"remoteAddr", r.RemoteAddr,
		"issueKey", getIssueKey(event),
		"projectKey", getProjectKey(event))
}

// convertToInterfaceEvent converts Jira webhook event to interface event
func (h *WebhookHandler) convertToInterfaceEvent(event *WebhookEvent) *webhook.Event {
	// Create a unique key for this event
	eventKey := fmt.Sprintf("%s-%d", event.WebhookEvent, event.Timestamp)

	// Store event in cache
	h.eventCache[eventKey] = event

	return &webhook.Event{
		Type:      event.WebhookEvent,
		EventName: event.WebhookEvent,
		// Use User field to pass the event key
		User: &webhook.User{
			ID:       int(event.Timestamp),
			Username: eventKey,
			Name:     event.WebhookEvent,
			Email:    "",
		},
	}
}

// ProcessEventAsync processes the Jira webhook event asynchronously
//
//nolint:gocyclo // This function coordinates multiple processing steps
func (h *WebhookHandler) ProcessEventAsync(ctx context.Context, event *webhook.Event) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Continue processing
	}

	// Validate event data
	if err := h.validateEventData(event); err != nil {
		return err
	}

	// Retrieve Jira event from cache
	jiraEvent, eventKey, err := h.retrieveFromCache(event.User.Username)
	if err != nil {
		return err
	}
	defer delete(h.eventCache, eventKey)

	h.logger.Info("Processing Jira webhook event",
		"eventType", jiraEvent.WebhookEvent,
		"issueKey", getIssueKey(jiraEvent),
		"projectKey", getProjectKey(jiraEvent))

	// Process the event by type
	if err := h.processEventByType(ctx, jiraEvent); err != nil {
		return err
	}

	// Perform bidirectional sync only if enabled
	if h.config.BidirectionalEnabled {
		h.performBidirectionalSync(ctx, jiraEvent)
	} else {
		h.logger.Debug("Bidirectional sync disabled, skipping Jira → GitLab synchronization",
			"eventType", jiraEvent.WebhookEvent,
			"issueKey", getIssueKey(jiraEvent))
	}

	return nil
}

// validateEventData validates the incoming event data
func (h *WebhookHandler) validateEventData(event *webhook.Event) error {
	if event == nil || event.User == nil {
		return fmt.Errorf("invalid event data")
	}
	return nil
}

// retrieveFromCache retrieves the Jira event from cache
func (h *WebhookHandler) retrieveFromCache(eventKey string) (*WebhookEvent, string, error) {
	jiraEvent, exists := h.eventCache[eventKey]
	if !exists {
		return nil, "", fmt.Errorf("jira event not found in cache: %s", eventKey)
	}
	return jiraEvent, eventKey, nil
}

// processEventByType processes the event based on its type
func (h *WebhookHandler) processEventByType(ctx context.Context, jiraEvent *WebhookEvent) error {
	switch jiraEvent.WebhookEvent {
	case "jira:issue_created":
		return h.processIssueCreated(ctx, jiraEvent)
	case "jira:issue_updated":
		return h.processIssueUpdated(ctx, jiraEvent)
	case "jira:issue_deleted":
		return h.processIssueDeleted(ctx, jiraEvent)
	case "comment_created":
		return h.processCommentCreated(ctx, jiraEvent)
	case "comment_updated":
		return h.processCommentUpdated(ctx, jiraEvent)
	case "comment_deleted":
		return h.processCommentDeleted(ctx, jiraEvent)
	case "worklog_created":
		return h.processWorklogCreated(ctx, jiraEvent)
	case "worklog_updated":
		return h.processWorklogUpdated(ctx, jiraEvent)
	case "worklog_deleted":
		return h.processWorklogDeleted(ctx, jiraEvent)
	default:
		h.logger.Debug("Unsupported Jira webhook event type", "type", jiraEvent.WebhookEvent)
		return nil
	}
}

// performBidirectionalSync performs bidirectional sync if available
func (h *WebhookHandler) performBidirectionalSync(ctx context.Context, jiraEvent *WebhookEvent) {
	if h.syncManager == nil {
		return
	}

	if syncErr := h.syncManager.SyncJiraToGitLab(ctx, jiraEvent); syncErr != nil {
		// Log sync error but don't fail the main processing
		h.logger.Warn("Failed to sync Jira event to GitLab",
			"event_type", jiraEvent.WebhookEvent,
			"issue_key", getIssueKey(jiraEvent),
			"error", syncErr)
	}
}

// processIssueCreated handles Jira issue creation events
func (h *WebhookHandler) processIssueCreated(ctx context.Context, event *WebhookEvent) error {
	if event.Issue == nil {
		return fmt.Errorf("issue data is nil")
	}

	issue := event.Issue
	h.logger.Info("Processing Jira issue created",
		"issueKey", issue.Key,
		"summary", issue.Fields.Summary,
		"projectKey", issue.Fields.Project.Key)

	// Find the corresponding GitLab project
	gitlabProjectID := h.findGitLabProject(issue.Fields.Project.Key)
	if gitlabProjectID == "" {
		h.logger.Debug("No GitLab project mapping found", "jiraProjectKey", issue.Fields.Project.Key)
		return nil
	}

	// Create GitLab issue
	createRequest := &GitLabIssueCreateRequest{
		Title:       fmt.Sprintf("[%s] %s", issue.Key, issue.Fields.Summary),
		Description: h.buildIssueDescription(issue),
		Labels:      h.buildLabels(issue),
	}

	// Set assignee if available
	if assigneeID := h.findGitLabUser(ctx, issue.Fields.Assignee); assigneeID > 0 {
		createRequest.AssigneeID = assigneeID
	}

	// Set milestone if available
	if milestoneID := h.findGitLabMilestone(ctx, gitlabProjectID, issue); milestoneID > 0 {
		createRequest.MilestoneID = milestoneID
	}

	gitlabIssue, err := h.gitlabClient.CreateIssue(ctx, gitlabProjectID, createRequest)
	if err != nil {
		return fmt.Errorf("failed to create GitLab issue: %w", err)
	}

	h.logger.Info("Created GitLab issue",
		"jiraIssueKey", issue.Key,
		"gitlabIssueIID", gitlabIssue.IID,
		"gitlabProjectID", gitlabProjectID,
		"url", gitlabIssue.WebURL)

	return nil
}

// processIssueUpdated handles Jira issue update events
func (h *WebhookHandler) processIssueUpdated(ctx context.Context, event *WebhookEvent) error {
	if event.Issue == nil {
		return fmt.Errorf("issue data is nil")
	}

	issue := event.Issue
	h.logger.Info("Processing Jira issue updated",
		"issueKey", issue.Key,
		"summary", issue.Fields.Summary)

	// Find the corresponding GitLab project and issue
	gitlabProjectID, gitlabIssue, err := h.findExistingGitLabIssue(ctx, issue)
	if err != nil {
		return err
	}
	if gitlabIssue == nil {
		return nil
	}

	// Build update request
	updateRequest := h.buildIssueUpdateRequest(issue, gitlabProjectID, gitlabIssue, event)
	if err != nil {
		return err
	}

	// Update GitLab issue
	updatedIssue, err := h.gitlabClient.UpdateIssue(ctx, gitlabProjectID, gitlabIssue.IID, updateRequest)
	if err != nil {
		return fmt.Errorf("failed to update GitLab issue: %w", err)
	}

	h.logger.Info("Updated GitLab issue",
		"jiraIssueKey", issue.Key,
		"gitlabIssueIID", updatedIssue.IID,
		"url", updatedIssue.WebURL)

	return nil
}

// findExistingGitLabIssue finds an existing GitLab issue for a Jira issue
func (h *WebhookHandler) findExistingGitLabIssue(ctx context.Context, issue *JiraIssue) (string, *GitLabIssue, error) {
	// Find the corresponding GitLab project
	gitlabProjectID := h.findGitLabProject(issue.Fields.Project.Key)
	if gitlabProjectID == "" {
		h.logger.Debug("No GitLab project mapping found", "jiraProjectKey", issue.Fields.Project.Key)
		return "", nil, nil
	}

	// Search for existing GitLab issue by title pattern
	existingIssues, err := h.gitlabClient.SearchIssuesByTitle(ctx, gitlabProjectID, fmt.Sprintf("[%s]", issue.Key))
	if err != nil {
		return "", nil, fmt.Errorf("failed to search GitLab issues: %w", err)
	}

	var gitlabIssue *GitLabIssue
	for _, existingIssue := range existingIssues {
		if strings.Contains(existingIssue.Title, fmt.Sprintf("[%s]", issue.Key)) {
			gitlabIssue = existingIssue
			break
		}
	}

	if gitlabIssue == nil {
		h.logger.Debug("No corresponding GitLab issue found", "jiraIssueKey", issue.Key)
		return "", nil, nil
	}

	return gitlabProjectID, gitlabIssue, nil
}

// buildIssueUpdateRequest builds an update request for a GitLab issue
func (h *WebhookHandler) buildIssueUpdateRequest(
	issue *JiraIssue,
	gitlabProjectID string,
	gitlabIssue *GitLabIssue,
	event *WebhookEvent,
) *GitLabIssueUpdateRequest {
	updateRequest := &GitLabIssueUpdateRequest{
		Title:  fmt.Sprintf("[%s] %s", issue.Key, issue.Fields.Summary),
		Labels: h.buildLabels(issue),
	}

	// Update state based on Jira status change
	h.updateIssueState(issue, gitlabIssue, updateRequest, event)

	// Set assignee if available
	if assigneeID := h.findGitLabUser(context.Background(), issue.Fields.Assignee); assigneeID > 0 {
		updateRequest.AssigneeID = &assigneeID
	}

	// Update milestone if available
	if milestoneID := h.findGitLabMilestone(context.Background(), gitlabProjectID, issue); milestoneID > 0 {
		updateRequest.MilestoneID = &milestoneID
	} else if len(issue.Fields.FixVersions) == 0 {
		// If no fix versions in Jira, clear the milestone in GitLab
		clearMilestoneID := 0
		updateRequest.MilestoneID = &clearMilestoneID
	}

	return updateRequest
}

// updateIssueState updates the state of a GitLab issue based on Jira status change
func (h *WebhookHandler) updateIssueState(
	issue *JiraIssue,
	gitlabIssue *GitLabIssue,
	updateRequest *GitLabIssueUpdateRequest,
	event *WebhookEvent,
) {
	var oldStatus string
	if event.Changelog != nil && len(event.Changelog.Items) > 0 {
		// Find status change in the changelog
		for i := range event.Changelog.Items {
			item := &event.Changelog.Items[i]
			if item.Field == "status" {
				oldStatus = item.ToString
				break
			}
		}
	}

	if issue.Fields.Status != nil {
		// Use enhanced status transition logic
		stateTransition := h.getStatusTransition(oldStatus, issue.Fields.Status.Name, gitlabIssue.State)
		if stateTransition != "" {
			updateRequest.StateEvent = stateTransition
			h.logger.Info("Status transition detected",
				"jiraIssueKey", issue.Key,
				"oldStatus", oldStatus,
				"newStatus", issue.Fields.Status.Name,
				"gitlabState", gitlabIssue.State,
				"transition", stateTransition)
		}
	}
}

// processIssueDeleted handles Jira issue deletion events
func (h *WebhookHandler) processIssueDeleted(ctx context.Context, event *WebhookEvent) error {
	if event.Issue == nil {
		return fmt.Errorf("issue data is nil")
	}

	issue := event.Issue
	h.logger.Info("Processing Jira issue deleted", "issueKey", issue.Key)

	// Find the corresponding GitLab project and issue
	gitlabProjectID := h.findGitLabProject(issue.Fields.Project.Key)
	if gitlabProjectID == "" {
		h.logger.Debug("No GitLab project mapping found", "jiraProjectKey", issue.Fields.Project.Key)
		return nil
	}

	// Search for existing GitLab issue
	existingIssues, err := h.gitlabClient.SearchIssuesByTitle(ctx, gitlabProjectID, fmt.Sprintf("[%s]", issue.Key))
	if err != nil {
		return fmt.Errorf("failed to search GitLab issues: %w", err)
	}

	for _, existingIssue := range existingIssues {
		if !strings.Contains(existingIssue.Title, fmt.Sprintf("[%s]", issue.Key)) {
			continue
		}

		// Add a comment indicating the Jira issue was deleted
		deletionMessage := fmt.Sprintf("🗑️ **Jira Issue Deleted**\n\n"+
			"The corresponding Jira issue [%s] has been deleted.\n\n"+
			"*This GitLab issue remains for reference.*", issue.Key)
		comment := &GitLabCommentCreateRequest{
			Body: deletionMessage,
		}

		_, err := h.gitlabClient.CreateComment(ctx, gitlabProjectID, existingIssue.IID, comment)
		if err != nil {
			h.logger.Error("Failed to add deletion comment", "error", err)
		}

		h.logger.Info("Added deletion comment to GitLab issue",
			"jiraIssueKey", issue.Key,
			"gitlabIssueIID", existingIssue.IID)
		break
	}

	return nil
}

// processCommentCreated handles Jira comment creation events
func (h *WebhookHandler) processCommentCreated(ctx context.Context, event *WebhookEvent) error {
	if event.Issue == nil || event.Comment == nil {
		return fmt.Errorf("issue or comment data is nil")
	}

	h.logger.Info("Processing Jira comment created",
		"issueKey", event.Issue.Key,
		"commentID", event.Comment.ID,
		"author", event.Comment.Author.DisplayName)

	return h.syncCommentToGitLab(ctx, event, "created")
}

// processCommentUpdated handles Jira comment update events
func (h *WebhookHandler) processCommentUpdated(ctx context.Context, event *WebhookEvent) error {
	if event.Issue == nil || event.Comment == nil {
		return fmt.Errorf("issue or comment data is nil")
	}

	h.logger.Info("Processing Jira comment updated",
		"issueKey", event.Issue.Key,
		"commentID", event.Comment.ID,
		"author", event.Comment.Author.DisplayName)

	return h.syncCommentToGitLab(ctx, event, "updated")
}

// processCommentDeleted handles Jira comment deletion events
func (h *WebhookHandler) processCommentDeleted(ctx context.Context, event *WebhookEvent) error {
	if event.Issue == nil || event.Comment == nil {
		return fmt.Errorf("issue or comment data is nil")
	}

	h.logger.Info("Processing Jira comment deleted",
		"issueKey", event.Issue.Key,
		"commentID", event.Comment.ID)

	return h.syncCommentToGitLab(ctx, event, "deleted")
}

// processWorklogCreated handles Jira worklog creation events
func (h *WebhookHandler) processWorklogCreated(ctx context.Context, event *WebhookEvent) error {
	if event.Issue == nil || event.Worklog == nil {
		return fmt.Errorf("issue or worklog data is nil")
	}

	h.logger.Info("Processing Jira worklog created",
		"issueKey", event.Issue.Key,
		"worklogID", event.Worklog.ID,
		"timeSpent", event.Worklog.TimeSpent)

	return h.syncWorklogToGitLab(ctx, event, "created")
}

// processWorklogUpdated handles Jira worklog update events
func (h *WebhookHandler) processWorklogUpdated(ctx context.Context, event *WebhookEvent) error {
	if event.Issue == nil || event.Worklog == nil {
		return fmt.Errorf("issue or worklog data is nil")
	}

	h.logger.Info("Processing Jira worklog updated",
		"issueKey", event.Issue.Key,
		"worklogID", event.Worklog.ID,
		"timeSpent", event.Worklog.TimeSpent)

	return h.syncWorklogToGitLab(ctx, event, "updated")
}

// processWorklogDeleted handles Jira worklog deletion events
func (h *WebhookHandler) processWorklogDeleted(ctx context.Context, event *WebhookEvent) error {
	if event.Issue == nil || event.Worklog == nil {
		return fmt.Errorf("issue or worklog data is nil")
	}

	h.logger.Info("Processing Jira worklog deleted",
		"issueKey", event.Issue.Key,
		"worklogID", event.Worklog.ID)

	return h.syncWorklogToGitLab(ctx, event, "deleted")
}

// Helper functions

// findGitLabProject finds the GitLab project ID for a Jira project key
func (h *WebhookHandler) findGitLabProject(jiraProjectKey string) string {
	// Check for explicit mapping first
	if gitlabProject, exists := h.config.ProjectMappings[jiraProjectKey]; exists {
		h.logger.Debug("Using explicit project mapping",
			"jiraProject", jiraProjectKey,
			"gitlabProject", gitlabProject)
		return gitlabProject
	}

	// Fallback to namespace-based convention
	defaultProject := fmt.Sprintf("%s/%s", h.config.GitLabNamespace, strings.ToLower(jiraProjectKey))

	h.logger.Debug("Using default project mapping",
		"jiraProject", jiraProjectKey,
		"gitlabProject", defaultProject,
		"namespace", h.config.GitLabNamespace)

	return defaultProject
}

// findGitLabUser finds the GitLab user ID for a Jira user using accountId
func (h *WebhookHandler) findGitLabUser(ctx context.Context, jiraUser *JiraUser) int {
	if jiraUser == nil {
		return 0
	}

	// Handle special assignee cases
	if h.isSpecialAssignee(jiraUser) {
		h.logger.Debug("Special assignee case detected",
			"accountId", jiraUser.AccountID,
			"displayName", jiraUser.DisplayName)
		return 0 // Let the caller handle special cases
	}

	// Try to find GitLab user by email (primary method)
	if jiraUser.EmailAddress != "" {
		gitlabUser, err := h.gitlabClient.FindUserByEmail(ctx, jiraUser.EmailAddress)
		if err != nil {
			h.logger.Warn("Failed to find GitLab user by email",
				"email", jiraUser.EmailAddress,
				"error", err)
		} else if gitlabUser != nil {
			h.logger.Info("Found GitLab user by email",
				"jiraEmail", jiraUser.EmailAddress,
				"gitlabUser", gitlabUser.Username,
				"gitlabID", gitlabUser.ID)
			return gitlabUser.ID
		}
	}

	// Fallback: try to find by username if email is not available
	if jiraUser.DisplayName != "" {
		// Search for GitLab users by username
		gitlabUsers, err := h.searchGitLabUsersByUsername(ctx, jiraUser.DisplayName)
		if err == nil && len(gitlabUsers) > 0 {
			// Return the first match
			matchedUser := gitlabUsers[0]
			h.logger.Info("Found GitLab user by username",
				"jiraUsername", jiraUser.DisplayName,
				"gitlabUser", matchedUser.Username,
				"gitlabID", matchedUser.ID)
			return matchedUser.ID
		}
	}

	h.logger.Debug("No GitLab user found for Jira user",
		"accountId", jiraUser.AccountID,
		"email", jiraUser.EmailAddress,
		"name", jiraUser.DisplayName)

	return 0
}

// isSpecialAssignee checks if the Jira user represents a special assignee case
func (h *WebhookHandler) isSpecialAssignee(jiraUser *JiraUser) bool {
	if jiraUser == nil {
		return false
	}

	// Check for null/empty accountId (Unassigned)
	if jiraUser.AccountID == "" || jiraUser.AccountID == "null" {
		return true
	}

	// Check for default assignee (-1)
	if jiraUser.AccountID == "-1" {
		return true
	}

	// Check for system accounts
	systemAccounts := []string{"-1", "null", "system", "automation", "bot"}
	for _, systemAccount := range systemAccounts {
		if strings.EqualFold(jiraUser.AccountID, systemAccount) ||
			strings.EqualFold(jiraUser.DisplayName, systemAccount) {
			return true
		}
	}

	return false
}

// searchGitLabUsersByUsername searches GitLab users by username
func (h *WebhookHandler) searchGitLabUsersByUsername(_ context.Context, _ string) ([]*GitLabUser, error) {
	// This is a simplified implementation
	// In a real implementation, you might need to call GitLab API with search functionality
	// For now, return empty slice to indicate not implemented
	return []*GitLabUser{}, nil
}

// findGitLabMilestone finds the GitLab milestone ID for a Jira issue's fix versions
func (h *WebhookHandler) findGitLabMilestone(ctx context.Context, gitlabProjectID string, issue *JiraIssue) int {
	if len(issue.Fields.FixVersions) == 0 {
		return 0
	}

	// For now, use the first fix version as the milestone
	// In a more sophisticated implementation, we could:
	// 1. Search for existing milestones by name
	// 2. Create new milestones if they don't exist
	// 3. Handle multiple fix versions more intelligently

	targetVersion := issue.Fields.FixVersions[0]
	h.logger.Debug("Looking for GitLab milestone for Jira version",
		"jiraVersion", targetVersion.Name,
		"gitlabProject", gitlabProjectID)

	// Try to find existing milestone by name
	if h.gitlabClient != nil {
		milestones, err := h.gitlabClient.GetMilestones(ctx, gitlabProjectID)
		if err != nil {
			h.logger.Warn("failed to get GitLab milestones", "error", err)
		} else {
			for _, milestone := range milestones {
				if milestone.Title == targetVersion.Name {
					h.logger.Info("Found existing GitLab milestone",
						"milestoneID", milestone.ID,
						"milestoneTitle", milestone.Title,
						"jiraVersion", targetVersion.Name)
					return milestone.ID
				}
			}
		}
	}

	// If no milestone found, create one if it's a released version
	if targetVersion.Released && targetVersion.Name != "" {
		if h.gitlabClient != nil {
			createRequest := &GitLabMilestoneCreateRequest{
				Title:       targetVersion.Name,
				Description: fmt.Sprintf("Jira version: %s\n%s", targetVersion.Name, targetVersion.Description),
			}

			// Parse release date if available
			if targetVersion.ReleaseDate != "" {
				if releaseDate, err := time.Parse("2006-01-02", targetVersion.ReleaseDate); err == nil {
					createRequest.DueDate = &releaseDate
				}
			}

			milestone, err := h.gitlabClient.CreateMilestone(ctx, gitlabProjectID, createRequest)
			if err != nil {
				h.logger.Warn("failed to create GitLab milestone", "error", err, "version", targetVersion.Name)
			} else {
				h.logger.Info("Created new GitLab milestone",
					"milestoneID", milestone.ID,
					"milestoneTitle", milestone.Title,
					"jiraVersion", targetVersion.Name)
				return milestone.ID
			}
		}
	}

	h.logger.Debug("No milestone mapping found, using version name as label",
		"version", targetVersion.Name)

	// Add version as a label instead
	return 0
}

// buildIssueDescription builds GitLab issue description from Jira issue
func (h *WebhookHandler) buildIssueDescription(issue *JiraIssue) string {
	var description strings.Builder

	description.WriteString(fmt.Sprintf("**Jira Issue**: [%s](%s)\n\n", issue.Key, issue.Self))

	if issue.Fields.Description != nil {
		// Handle both string and ADF descriptions
		switch desc := issue.Fields.Description.(type) {
		case string:
			if desc != "" {
				description.WriteString("**Description**:\n")
				description.WriteString(desc)
				description.WriteString("\n\n")
			}
		default:
			// For ADF content, we'd need to convert it to markdown
			// For now, just indicate it's rich content
			description.WriteString("**Description**: *Rich content from Jira*\n\n")
		}
	}

	if issue.Fields.Priority != nil {
		description.WriteString(fmt.Sprintf("**Priority**: %s\n", issue.Fields.Priority.Name))
	}

	if issue.Fields.IssueType != nil {
		description.WriteString(fmt.Sprintf("**Issue Type**: %s\n", issue.Fields.IssueType.Name))
	}

	if issue.Fields.Reporter != nil {
		description.WriteString(fmt.Sprintf("**Reporter**: %s\n", issue.Fields.Reporter.DisplayName))
	}

	description.WriteString("\n---\n*Synchronized from Jira*")

	return description.String()
}

// buildLabels builds GitLab labels from Jira issue
func (h *WebhookHandler) buildLabels(issue *JiraIssue) []string {
	var labels []string

	// Add Jira sync label
	labels = append(labels, "jira-sync")

	// Add issue type
	if issue.Fields.IssueType != nil {
		labels = append(labels, fmt.Sprintf("jira-type:%s", strings.ToLower(issue.Fields.IssueType.Name)))
	}

	// Add priority
	if issue.Fields.Priority != nil {
		labels = append(labels, fmt.Sprintf("jira-priority:%s", strings.ToLower(issue.Fields.Priority.Name)))
	}

	// Add status
	if issue.Fields.Status != nil {
		labels = append(labels, fmt.Sprintf("jira-status:%s", strings.ToLower(issue.Fields.Status.Name)))
	}

	// Add existing labels
	labels = append(labels, issue.Fields.Labels...)

	// Add fix versions as labels
	for _, version := range issue.Fields.FixVersions {
		if version.Name != "" {
			labels = append(labels, fmt.Sprintf("jira-version:%s", version.Name))
		}
	}

	return labels
}

// isClosedStatus checks if a Jira status indicates a closed state
func (h *WebhookHandler) isClosedStatus(statusName string) bool {
	if statusName == "" {
		return false
	}

	// Check custom status mapping first
	if h.config.StatusMappingEnabled {
		if gitlabLabel, exists := h.config.StatusMapping[statusName]; exists {
			// If mapped to a closed-related label, consider it closed
			closedLabels := []string{"closed", "done", "resolved", "completed"}
			for _, closedLabel := range closedLabels {
				if strings.Contains(strings.ToLower(gitlabLabel), closedLabel) {
					return true
				}
			}
		}
	}

	// Default closed statuses
	closedStatuses := []string{"closed", "done", "resolved", "fixed", "completed", "canceled", "rejected"}
	statusLower := strings.ToLower(statusName)

	for _, closed := range closedStatuses {
		if statusLower == closed {
			return true
		}
	}

	return false
}

// getStatusTransition determines the appropriate GitLab state event for a Jira status change
func (h *WebhookHandler) getStatusTransition(oldStatus, newStatus, _ string) string {
	// If status mapping is enabled, check for specific transitions
	if h.config.StatusMappingEnabled {
		// Check if new status maps to a closed state
		newIsClosed := h.isClosedStatus(newStatus)
		oldIsClosed := h.isClosedStatus(oldStatus)

		// Handle close transition
		if !oldIsClosed && newIsClosed {
			return "close"
		}
		// Handle reopen transition
		if oldIsClosed && !newIsClosed {
			return "reopen"
		}
	}

	// Fallback to basic logic
	newIsClosed := h.isClosedStatus(newStatus)
	oldIsClosed := h.isClosedStatus(oldStatus)

	// Handle close transition
	if !oldIsClosed && newIsClosed {
		return "close"
	}
	// Handle reopen transition
	if oldIsClosed && !newIsClosed {
		return "reopen"
	}

	// No state change needed
	return ""
}

// extractJiraKeyFromTitle extracts Jira issue key from GitLab issue title
func (h *WebhookHandler) extractJiraKeyFromTitle(title string) string {
	// Look for pattern like "[PROJ-123] Title"
	if len(title) > 2 && title[0] == '[' && title[1] != ']' {
		// Find closing bracket
		endBracket := strings.Index(title, "]")
		if endBracket > 1 {
			key := title[1:endBracket]
			// Validate key format (should be like PROJ-123)
			if strings.Contains(key, "-") {
				return key
			}
		}
	}
	return ""
}

// getJiraStatusFromGitLabState determines appropriate Jira status based on GitLab state
func (h *WebhookHandler) getJiraStatusFromGitLabState(gitlabState string, jiraIssue *JiraIssue) string {
	switch gitlabState {
	case "closed":
		return h.getClosedJiraStatus(jiraIssue)
	case "opened":
		return h.getOpenJiraStatus(jiraIssue)
	default:
		return "" // Unknown state, no change
	}
}

// getClosedJiraStatus determines appropriate closed Jira status
func (h *WebhookHandler) getClosedJiraStatus(jiraIssue *JiraIssue) string {
	if jiraIssue.Fields.Status == nil {
		return "Closed" // Default closed status
	}

	// Try to find a closed status through transitions
	if closedStatus := h.findClosedStatusViaTransitions(jiraIssue); closedStatus != "" {
		return closedStatus
	}

	// Fallback to known closed statuses
	closedStatuses := []string{"Closed", "Done", "Resolved", "Fixed", "Completed"}
	for _, closedStatus := range closedStatuses {
		if jiraIssue.Fields.Status.Name != closedStatus {
			return closedStatus
		}
	}

	return "Closed" // Default closed status
}

// getOpenJiraStatus determines appropriate open Jira status
func (h *WebhookHandler) getOpenJiraStatus(jiraIssue *JiraIssue) string {
	if jiraIssue.Fields.Status == nil {
		return "Open" // Default open status
	}

	// Try to find an open status through transitions
	if openStatus := h.findOpenStatusViaTransitions(jiraIssue); openStatus != "" {
		return openStatus
	}

	// Fallback to known open statuses
	openStatuses := []string{"Open", "To Do", "In Progress", "Backlog"}
	for _, openStatus := range openStatuses {
		if jiraIssue.Fields.Status.Name != openStatus {
			return openStatus
		}
	}

	return "Open" // Default open status
}

// findStatusViaTransitions finds a status through Jira transitions based on keywords
func (h *WebhookHandler) findStatusViaTransitions(jiraIssue *JiraIssue, keywords []string) string {
	jiraClient := h.getJiraClient()
	if jiraClient == nil {
		return ""
	}

	transitions, err := jiraClient.GetTransitions(context.Background(), jiraIssue.Key)
	if err != nil {
		return ""
	}

	for i := range transitions {
		transition := &transitions[i]
		for _, keyword := range keywords {
			if strings.Contains(strings.ToLower(transition.Name), keyword) {
				return transition.To.Name
			}
		}
	}

	return ""
}

// findClosedStatusViaTransitions finds a closed status through Jira transitions
func (h *WebhookHandler) findClosedStatusViaTransitions(jiraIssue *JiraIssue) string {
	keywords := []string{"close", "done", "resolve"}
	return h.findStatusViaTransitions(jiraIssue, keywords)
}

// findOpenStatusViaTransitions finds an open status through Jira transitions
func (h *WebhookHandler) findOpenStatusViaTransitions(jiraIssue *JiraIssue) string {
	keywords := []string{"open", "reopen", "start"}
	return h.findStatusViaTransitions(jiraIssue, keywords)
}

// getJiraClient returns a Jira client instance
func (h *WebhookHandler) getJiraClient() *Client {
	return h.jiraClient
}

// syncCommentToGitLab syncs a Jira comment to GitLab with ADF validation
func (h *WebhookHandler) syncCommentToGitLab(ctx context.Context, event *WebhookEvent, action string) error {
	// Find the corresponding GitLab issue
	gitlabProjectID := h.findGitLabProject(event.Issue.Fields.Project.Key)
	if gitlabProjectID == "" {
		return nil
	}

	// Search for GitLab issue
	existingIssues, err := h.gitlabClient.SearchIssuesByTitle(ctx, gitlabProjectID, fmt.Sprintf("[%s]", event.Issue.Key))
	if err != nil {
		return fmt.Errorf("failed to search GitLab issues: %w", err)
	}

	var gitlabIssue *GitLabIssue
	for _, issue := range existingIssues {
		if strings.Contains(issue.Title, fmt.Sprintf("[%s]", event.Issue.Key)) {
			gitlabIssue = issue
			break
		}
	}

	if gitlabIssue == nil {
		return nil
	}

	// Build comment content with ADF validation
	var commentBody string
	switch action {
	case "created":
		commentBody = fmt.Sprintf("💬 **Jira Comment Added** by %s\n\n%s\n\n---\n*From Jira at %s*",
			event.Comment.Author.DisplayName,
			h.extractCommentTextWithValidation(event.Comment.Body),
			event.Comment.Created)
	case "updated":
		commentBody = fmt.Sprintf("✏️ **Jira Comment Updated** by %s\n\n%s\n\n---\n*Updated in Jira at %s*",
			event.Comment.UpdateAuthor.DisplayName,
			h.extractCommentTextWithValidation(event.Comment.Body),
			event.Comment.Updated)
	case "deleted":
		commentBody = fmt.Sprintf("🗑️ **Jira Comment Deleted**\n\n"+
			"A comment by %s was deleted from Jira.\n\n---\n*Deleted from Jira*",
			event.Comment.Author.DisplayName)
	}

	// Create comment in GitLab
	comment := &GitLabCommentCreateRequest{Body: commentBody}
	_, err = h.gitlabClient.CreateComment(ctx, gitlabProjectID, gitlabIssue.IID, comment)
	if err != nil {
		return fmt.Errorf("failed to create GitLab comment: %w", err)
	}

	h.logger.Info("Synced Jira comment to GitLab",
		"action", action,
		"jiraIssueKey", event.Issue.Key,
		"gitlabIssueIID", gitlabIssue.IID)

	return nil
}

// syncWorklogToGitLab syncs a Jira worklog to GitLab
func (h *WebhookHandler) syncWorklogToGitLab(ctx context.Context, event *WebhookEvent, action string) error {
	// Find the corresponding GitLab issue
	gitlabProjectID := h.findGitLabProject(event.Issue.Fields.Project.Key)
	if gitlabProjectID == "" {
		return nil
	}

	// Search for GitLab issue
	existingIssues, err := h.gitlabClient.SearchIssuesByTitle(ctx, gitlabProjectID, fmt.Sprintf("[%s]", event.Issue.Key))
	if err != nil {
		return fmt.Errorf("failed to search GitLab issues: %w", err)
	}

	var gitlabIssue *GitLabIssue
	for _, issue := range existingIssues {
		if strings.Contains(issue.Title, fmt.Sprintf("[%s]", event.Issue.Key)) {
			gitlabIssue = issue
			break
		}
	}

	if gitlabIssue == nil {
		return nil
	}

	// Build worklog comment content
	var commentBody string
	switch action {
	case "created":
		commentBody = fmt.Sprintf("⏱️ **Time Logged** by %s\n\n"+
			"**Time Spent**: %s\n**Date**: %s\n\n%s\n\n---\n*Logged in Jira*",
			event.Worklog.Author.DisplayName,
			event.Worklog.TimeSpent,
			event.Worklog.Started,
			h.extractCommentText(event.Worklog.Comment))
	case "updated":
		commentBody = fmt.Sprintf("⏱️ **Time Log Updated** by %s\n\n"+
			"**Time Spent**: %s\n**Date**: %s\n\n%s\n\n---\n*Updated in Jira*",
			event.Worklog.UpdateAuthor.DisplayName,
			event.Worklog.TimeSpent,
			event.Worklog.Started,
			h.extractCommentText(event.Worklog.Comment))
	case "deleted":
		commentBody = fmt.Sprintf("🗑️ **Time Log Deleted**\n\n"+
			"A time log entry by %s (%s) was deleted from Jira.\n\n---\n*Deleted from Jira*",
			event.Worklog.Author.DisplayName,
			event.Worklog.TimeSpent)
	}

	// Create comment in GitLab
	comment := &GitLabCommentCreateRequest{Body: commentBody}
	_, err = h.gitlabClient.CreateComment(ctx, gitlabProjectID, gitlabIssue.IID, comment)
	if err != nil {
		return fmt.Errorf("failed to create GitLab comment: %w", err)
	}

	h.logger.Info("Synced Jira worklog to GitLab",
		"action", action,
		"jiraIssueKey", event.Issue.Key,
		"gitlabIssueIID", gitlabIssue.IID,
		"timeSpent", event.Worklog.TimeSpent)

	return nil
}

// extractCommentTextWithValidation extracts text from comment body with ADF validation and fallback
func (h *WebhookHandler) extractCommentTextWithValidation(body interface{}) string {
	if body == nil {
		return ""
	}

	switch content := body.(type) {
	case string:
		return content
	default:
		// Try to validate and convert ADF content
		if h.config.CommentSyncEnabled {
			// Create a CommentPayload for validation
			commentPayload := CommentPayload{
				Body: CommentBody{
					Type:    "doc",
					Version: 1,
					Content: []Content{
						{
							Type: "paragraph",
							Content: []TextContent{
								{
									Type: "text",
									Text: "*Rich content from Jira*",
								},
							},
						},
					},
				},
			}

			// Use ADF validation with fallback
			validatedContent := validateAndFallback(commentPayload)
			// Extract text from validated ADF
			return h.extractTextFromADF(validatedContent)
		}

		// Fallback to simple text indication
		return "*Rich content from Jira*"
	}
}

// extractCommentText extracts text from comment body (handles both string and ADF)
func (h *WebhookHandler) extractCommentText(body interface{}) string {
	return h.extractCommentTextWithValidation(body)
}

// extractTextFromADF extracts plain text from validated ADF content
func (h *WebhookHandler) extractTextFromADF(content CommentPayload) string {
	var result strings.Builder

	// Extract text content from ADF
	for i, item := range content.Body.Content {
		for _, textContent := range item.Content {
			if textContent.Type == "text" {
				result.WriteString(textContent.Text)
			}
		}
		// Add a newline after each paragraph
		if i < len(content.Body.Content)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// Utility functions for extracting data from events

func getIssueKey(event *WebhookEvent) string {
	if event.Issue != nil {
		return event.Issue.Key
	}
	return ""
}

func getProjectKey(event *WebhookEvent) string {
	if event.Issue != nil && event.Issue.Fields != nil && event.Issue.Fields.Project != nil {
		return event.Issue.Fields.Project.Key
	}
	return ""
}
