package gitlab

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/jira"
)

// Handler handles GitLab webhook requests
type Handler struct {
	config *config.Config
	logger *slog.Logger
	jira   *jira.Client
	parser *Parser
}

// NewHandler creates a new GitLab webhook handler
func NewHandler(cfg *config.Config, logger *slog.Logger) *Handler {
	return &Handler{
		config: cfg,
		logger: logger,
		jira:   jira.NewClient(cfg),
		parser: NewParser(),
	}
}

// HandleWebhook handles incoming GitLab webhook requests
func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate GitLab secret token
	if !h.validateToken(r) {
		h.logger.Warn("Invalid GitLab secret token", "remoteAddr", r.RemoteAddr)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("Failed to read request body", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if err := r.Body.Close(); err != nil {
		h.logger.Warn("Failed to close request body", "error", err)
	}

	// Parse webhook event
	event, err := h.parseEvent(body)
	if err != nil {
		h.logger.Error("Failed to parse webhook event", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Process the event
	if err := h.processEvent(event); err != nil {
		h.logger.Error("Failed to process event", "error", err, "eventType", event.Type)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Return success
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		h.logger.Error("Failed to write response", "error", err)
	}
}

// validateToken validates the GitLab secret token
func (h *Handler) validateToken(r *http.Request) bool {
	token := r.Header.Get("X-Gitlab-Token")
	return token == h.config.GitLabSecret
}

// parseEvent parses the webhook event from JSON
func (h *Handler) parseEvent(body []byte) (*Event, error) {
	var event Event
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event: %w", err)
	}

	// Set event type based on object_kind or event_name
	if event.ObjectKind != "" {
		event.Type = event.ObjectKind
	} else if event.EventName != "" {
		event.Type = event.EventName
	}

	return &event, nil
}

// processEvent processes the webhook event
func (h *Handler) processEvent(event *Event) error {
	h.logger.Info("Processing webhook event", "type", event.Type)

	switch event.Type {
	case "push":
		return h.processPushEvent(event)
	case "merge_request":
		return h.processMergeRequestEvent(event)
	default:
		h.logger.Debug("Unsupported event type", "type", event.Type)
		return nil
	}
}

// processPushEvent processes push events
func (h *Handler) processPushEvent(event *Event) error {
	if len(event.Commits) == 0 {
		return nil
	}

	for _, commit := range event.Commits {
		issueIDs := h.parser.ExtractIssueIDs(commit.Message)
		for _, issueID := range issueIDs {
			comment := h.createCommitComment(commit, event)
			if err := h.jira.AddComment(issueID, comment); err != nil {
				h.logger.Error("Failed to add comment to Jira",
					"error", err,
					"issueID", issueID,
					"commitID", commit.ID)
			} else {
				h.logger.Info("Added comment to Jira issue",
					"issueID", issueID,
					"commitID", commit.ID)
			}
		}
	}

	return nil
}

// processMergeRequestEvent processes merge request events
func (h *Handler) processMergeRequestEvent(event *Event) error {
	if event.ObjectAttributes == nil {
		return nil
	}

	issueIDs := h.parser.ExtractIssueIDs(event.ObjectAttributes.Title)
	for _, issueID := range issueIDs {
		comment := h.createMergeRequestComment(event)
		if err := h.jira.AddComment(issueID, comment); err != nil {
			h.logger.Error("Failed to add comment to Jira",
				"error", err,
				"issueID", issueID,
				"mrID", event.ObjectAttributes.ID)
		} else {
			h.logger.Info("Added comment to Jira issue",
				"issueID", issueID,
				"mrID", event.ObjectAttributes.ID)
		}
	}

	return nil
}

// createCommitComment creates a comment for a commit
func (h *Handler) createCommitComment(commit Commit, event *Event) string {
	return fmt.Sprintf("Commit [%s](%s) by %s: %s",
		commit.ID[:8],
		commit.URL,
		commit.Author.Name,
		commit.Message)
}

// createMergeRequestComment creates a comment for a merge request
func (h *Handler) createMergeRequestComment(event *Event) string {
	action := event.ObjectAttributes.Action
	title := event.ObjectAttributes.Title
	url := event.ObjectAttributes.URL

	return fmt.Sprintf("Merge Request %s: [%s](%s)",
		cases.Title(language.English).String(action),
		title,
		url)
}
