package gitlab

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// DebugLogger provides debug logging functionality for webhook data
type DebugLogger struct {
	logger *slog.Logger
}

// NewDebugLogger creates a new debug logger instance
func NewDebugLogger(logger *slog.Logger) *DebugLogger {
	return &DebugLogger{
		logger: logger,
	}
}

// LogWebhookRequest logs detailed information about incoming webhook requests
func (dl *DebugLogger) LogWebhookRequest(r *http.Request, body []byte, event *Event) {
	if dl.logger == nil {
		return
	}

	// Log request headers
	dl.logger.Debug("=== GITLAB WEBHOOK DEBUG START ===")
	dl.logger.Debug("Request Headers",
		"method", r.Method,
		"url", r.URL.String(),
		"remote_addr", r.RemoteAddr,
		"user_agent", r.UserAgent(),
		"content_type", r.Header.Get("Content-Type"),
		"content_length", r.Header.Get("Content-Length"),
		"x_gitlab_token", maskToken(r.Header.Get("X-Gitlab-Token")),
		"x_gitlab_event", r.Header.Get("X-Gitlab-Event"),
		"x_gitlab_instance", r.Header.Get("X-Gitlab-Instance"),
		"x_gitlab_webhook_id", r.Header.Get("X-Gitlab-Webhook-Id"),
		"x_gitlab_webhook_uuid", r.Header.Get("X-Gitlab-Webhook-Uuid"),
	)

	// Log request body
	if len(body) > 0 {
		dl.logger.Debug("Request Body (raw)",
			"body_size", len(body),
			"body_preview", dl.truncateString(string(body), 500),
		)

		// Pretty print JSON if possible
		if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			var prettyJSON interface{}
			if err := json.Unmarshal(body, &prettyJSON); err == nil {
				if prettyBody, err := json.MarshalIndent(prettyJSON, "", "  "); err == nil {
					dl.logger.Debug("Request Body (formatted)",
						"body_formatted", string(prettyBody),
					)
				}
			}
		}
	}

	// Log parsed event information
	if event != nil {
		dl.LogWebhookEvent(event)
	}

	dl.logger.Debug("=== GITLAB WEBHOOK DEBUG END ===")
}

// LogWebhookEvent logs detailed information about the parsed webhook event
func (dl *DebugLogger) LogWebhookEvent(event *Event) {
	if dl.logger == nil || event == nil {
		return
	}

	dl.logger.Debug("Parsed Webhook Event",
		"object_kind", event.ObjectKind,
		"event_type", event.Type,
		"event_name", event.EventName,
		"user_id", event.UserID,
		"user_name", event.UserName,
		"username", event.Username,
		"project_id", event.ProjectID,
		"project_name", event.ProjectName,
		"path_with_namespace", event.PathWithNamespace,
		"git_http_url", event.GitHTTPURL,
		"git_ssh_url", event.GitSSHURL,
		"visibility", event.Visibility,
		"default_branch", event.DefaultBranch,
		"created_at", event.CreatedAt,
		"updated_at", event.UpdatedAt,
	)

	// Log specific event data based on event type
	switch event.ObjectKind {
	case "push":
		dl.logger.Debug("Push Event Details",
			"before", event.Before,
			"after", event.After,
			"ref", event.Ref,
			"checkout_sha", event.CheckoutSha,
			"commits_count", len(event.Commits),
		)
	case "merge_request":
		if event.MergeRequest != nil {
			dl.logger.Debug("Merge Request Event Details",
				"mr_id", event.MergeRequest.ID,
				"mr_title", event.MergeRequest.Title,
				"mr_state", event.MergeRequest.State,
				"source_branch", event.MergeRequest.SourceBranch,
				"target_branch", event.MergeRequest.TargetBranch,
			)
		}
	case "issue":
		if event.Issue != nil {
			dl.logger.Debug("Issue Event Details",
				"issue_id", event.Issue.ID,
				"issue_title", event.Issue.Title,
				"issue_state", event.Issue.State,
			)
		}
	case "note":
		if event.Note != nil {
			dl.logger.Debug("Note Event Details",
				"note_id", event.Note.ID,
				"note_type", event.Note.Noteable,
				"note_preview", dl.truncateString(event.Note.Note, 100),
			)
		}
	case "pipeline":
		if event.Pipeline != nil {
			dl.logger.Debug("Pipeline Event Details",
				"pipeline_id", event.Pipeline.ID,
				"pipeline_ref", event.Pipeline.Ref,
				"pipeline_status", event.Pipeline.Status,
			)
		}
	case "build":
		if event.Build != nil {
			dl.logger.Debug("Build Event Details",
				"build_id", event.Build.ID,
				"build_name", event.Build.Name,
				"build_stage", event.Build.Stage,
				"build_status", event.Build.Status,
			)
		}
	default:
		dl.logger.Debug("Event type details", "object_kind", event.ObjectKind)
	}
}

// Helper functions
func (dl *DebugLogger) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Don't truncate at a space if possible
	if maxLen > 0 && s[maxLen-1] == ' ' {
		maxLen--
	}
	return s[:maxLen] + "..."
}

func maskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}
