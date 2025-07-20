package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	userlink "github.com/atlet99/gitlab-jira-hook/internal/common"
	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/jira"
	"github.com/atlet99/gitlab-jira-hook/internal/monitoring"
	"github.com/atlet99/gitlab-jira-hook/internal/webhook"
)

// Handler handles GitLab webhook requests
type Handler struct {
	config     *config.Config
	logger     *slog.Logger
	jira       JiraClient
	parser     *Parser
	monitor    *monitoring.WebhookMonitor
	workerPool webhook.WorkerPoolInterface
}

// JiraClient defines the interface for Jira client operations
// (AddComment and TestConnection)
type JiraClient interface {
	AddComment(issueID string, payload jira.CommentPayload) error
	TestConnection() error
}

// NewHandler creates a new GitLab webhook handler
func NewHandler(cfg *config.Config, logger *slog.Logger) *Handler {
	return &Handler{
		config:     cfg,
		logger:     logger,
		jira:       jira.NewClient(cfg),
		parser:     NewParser(),
		monitor:    nil, // Will be set by server
		workerPool: nil, // Will be set by server
	}
}

// SetMonitor sets the webhook monitor for metrics recording
func (h *Handler) SetMonitor(monitor *monitoring.WebhookMonitor) {
	h.monitor = monitor
}

// SetWorkerPool sets the worker pool for async processing
func (h *Handler) SetWorkerPool(workerPool webhook.WorkerPoolInterface) {
	h.workerPool = workerPool
}

// HandleWebhook handles incoming GitLab webhook requests
func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	success := false

	defer func() {
		// Record metrics if monitor is available
		if h.monitor != nil {
			h.monitor.RecordRequest("/gitlab-hook", success, time.Since(start))
		}
	}()

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

	// Event filtering by project/group
	if !h.isAllowedEvent(event) {
		h.logger.Info("Event filtered by project/group",
			"eventType", event.Type,
			"projectPath", h.getProjectPath(event),
			"groupPath", h.getGroupPath(event),
			"allowedProjects", h.config.AllowedProjects,
			"allowedGroups", h.config.AllowedGroups)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Convert to interface event
	interfaceEvent := h.convertToInterfaceEvent(event)

	// Submit job for async processing if worker pool is available
	if h.workerPool != nil {
		if err := h.workerPool.SubmitJob(interfaceEvent, h); err != nil {
			h.logger.Error("Failed to submit job to worker pool", "error", err, "eventType", event.Type)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		h.logger.Info("Job submitted for async processing", "eventType", event.Type)
	} else {
		// Fallback to synchronous processing
		if err := h.processEvent(event); err != nil {
			h.logger.Error("Failed to process event", "error", err, "eventType", event.Type)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	// Return success
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		h.logger.Error("Failed to write response", "error", err)
		return
	}

	success = true
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
	case "project_create", "project_destroy":
		return h.processProjectEvent(event)
	case "user_create", "user_destroy":
		return h.processUserEvent(event)
	case "user_add_to_team", "user_remove_from_team":
		return h.processTeamEvent(event)
	case "user_add_to_group", "user_remove_from_group":
		return h.processGroupEvent(event)
	// System Hook events
	case "repository_create", "repository_destroy":
		return h.processRepositoryEvent(event)
	case "repository_update":
		return h.processRepositoryUpdateEvent(event)
	case "team_create", "team_destroy":
		return h.processTeamCreateDestroyEvent(event)
	case "group_create", "group_destroy":
		return h.processGroupCreateDestroyEvent(event)
	case "user_add_to_project", "user_remove_from_project":
		return h.processProjectMembershipEvent(event)
	case "key_create", "key_destroy":
		return h.processKeyEvent(event)
	case "tag_push":
		return h.processTagPushEvent(event)
	case "release":
		return h.processReleaseEvent(event)
	case "deployment":
		return h.processDeploymentEvent(event)
	case "feature_flag":
		return h.processFeatureFlagEvent(event)
	case "wiki_page":
		return h.processWikiPageEvent(event)
	case "pipeline":
		return h.processPipelineEvent(event)
	case "build":
		return h.processBuildEvent(event)
	case "note":
		return h.processNoteEvent(event)
	case "issue":
		return h.processIssueEvent(event)
	default:
		h.logger.Debug("Unsupported event type", "type", event.Type)
		return nil
	}
}

// ProcessEventAsync processes the webhook event asynchronously with context (webhook.Event)
func (h *Handler) ProcessEventAsync(ctx context.Context, event *webhook.Event) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Continue processing
	}
	if event == nil {
		return nil
	}
	// Преобразуем обратно в *Event для существующей логики
	orig := &Event{
		Type:      event.Type,
		EventName: event.EventName,
	}
	if event.Project != nil {
		orig.Project = &Project{
			ID:                event.Project.ID,
			Name:              event.Project.Name,
			PathWithNamespace: event.Project.PathWithNamespace,
			WebURL:            event.Project.WebURL,
		}
	}
	if event.Group != nil {
		orig.Group = &Group{
			ID:       event.Group.ID,
			Name:     event.Group.Name,
			FullPath: event.Group.FullPath,
		}
	}
	if event.User != nil {
		orig.User = &User{
			ID:       event.User.ID,
			Username: event.User.Username,
			Name:     event.User.Name,
			Email:    event.User.Email,
		}
	}
	if event.Commits != nil {
		for _, c := range event.Commits {
			orig.Commits = append(orig.Commits, Commit{
				ID:        c.ID,
				Message:   c.Message,
				URL:       c.URL,
				Author:    Author{Name: c.Author.Name, Email: c.Author.Email},
				Timestamp: c.Timestamp.Format(time.RFC3339),
				Added:     c.Added,
				Modified:  c.Modified,
				Removed:   c.Removed,
			})
		}
	}
	if event.ObjectAttributes != nil {
		orig.ObjectAttributes = &ObjectAttributes{
			ID:          event.ObjectAttributes.ID,
			Title:       event.ObjectAttributes.Title,
			Description: event.ObjectAttributes.Description,
			State:       event.ObjectAttributes.State,
			Action:      event.ObjectAttributes.Action,
			Ref:         event.ObjectAttributes.Ref,
			URL:         event.ObjectAttributes.URL,
			Sha:         event.ObjectAttributes.SHA,
			Name:        event.ObjectAttributes.Name,
			Duration:    event.ObjectAttributes.Duration,
			Status:      event.ObjectAttributes.Status,
			IssueType:   event.ObjectAttributes.IssueType,
			Priority:    event.ObjectAttributes.Priority,
		}
	}
	return h.processEvent(orig)
}

// processPushEvent processes push events
func (h *Handler) processPushEvent(event *Event) error {
	if len(event.Commits) == 0 {
		return nil
	}

	for _, commit := range event.Commits {
		issueIDs := h.parser.ExtractIssueIDs(commit.Message)
		for _, issueID := range issueIDs {
			// Construct branch URL using project information from system hook
			branchURL := h.constructBranchURL(event, event.Ref)

			// For system hooks, we need to handle username differently
			// In push events (including merge commits), we have user_id but not username
			authorName := event.Username
			if authorName == "" && event.User != nil {
				authorName = event.User.Username
			}
			// For system hooks, we have user_id but not username
			if authorName == "" {
				// Use commit author name as display name
				authorName = commit.Author.Name
			}

			// Construct author URL using improved function
			authorURL := h.constructAuthorURL(event, commit.Author)

			// Get project web URL for MR links
			projectWebURL := ""
			if event.Project != nil {
				projectWebURL = event.Project.WebURL
			}

			// Use commit timestamp for push events (this is the actual commit time)
			eventTime := commit.Timestamp

			comment := jira.GenerateCommitADFComment(
				commit.ID,
				commit.URL,
				authorName,
				commit.Author.Email,
				authorURL,
				commit.Message,
				eventTime,
				event.Ref,
				branchURL,
				projectWebURL,
				h.config.Timezone,
				commit.Added,
				commit.Modified,
				commit.Removed,
			)
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

// constructAuthorURL constructs a user profile URL for commit authors
func (h *Handler) constructAuthorURL(event *Event, author Author) string {
	if h.config.GitLabBaseURL == "" {
		return ""
	}

	// Priority 1: Use user_id from system hook (most reliable)
	if event.UserID > 0 {
		return fmt.Sprintf("%s/-/profile/%d", h.config.GitLabBaseURL, event.UserID)
	}

	// Priority 2: Use username from system hook
	if event.Username != "" {
		return fmt.Sprintf("%s/%s", h.config.GitLabBaseURL, event.Username)
	}

	// Priority 3: Try to extract username from author email (common pattern)
	if author.Email != "" {
		// Extract username from email (e.g., "user@example.com" -> "user")
		parts := strings.Split(author.Email, "@")
		if len(parts) > 0 && parts[0] != "" {
			return fmt.Sprintf("%s/%s", h.config.GitLabBaseURL, parts[0])
		}
	}

	// If no user information available, return empty string
	return ""
}

// constructBranchURL constructs a proper branch URL using project information
func (h *Handler) constructBranchURL(event *Event, ref string) string {
	// Extract branch name from refs/heads/branch format
	branchName := ref
	if strings.HasPrefix(ref, "refs/heads/") {
		branchName = strings.TrimPrefix(ref, "refs/heads/")
	}

	// Priority 1: Use project information from system hook (most reliable)
	if event.Project != nil && event.Project.WebURL != "" {
		return fmt.Sprintf("%s/-/tree/%s", event.Project.WebURL, branchName)
	}

	// Priority 2: Use path_with_namespace from system hook (from documentation)
	if event.PathWithNamespace != "" && h.config.GitLabBaseURL != "" {
		return fmt.Sprintf("%s/%s/-/tree/%s", h.config.GitLabBaseURL, event.PathWithNamespace, branchName)
	}

	// Priority 3: Use project information from project hook
	if event.Project != nil && event.Project.PathWithNamespace != "" && h.config.GitLabBaseURL != "" {
		return fmt.Sprintf("%s/%s/-/tree/%s", h.config.GitLabBaseURL, event.Project.PathWithNamespace, branchName)
	}

	// Priority 4: Use project information from project hook with web_url
	if event.Project != nil && event.Project.WebURL != "" {
		return fmt.Sprintf("%s/-/tree/%s", event.Project.WebURL, branchName)
	}

	// Priority 5: Fallback to using PathWithNamespace and GitLabBaseURL (legacy)
	if event.PathWithNamespace != "" && h.config.GitLabBaseURL != "" {
		return fmt.Sprintf("%s/%s/-/tree/%s", h.config.GitLabBaseURL, event.PathWithNamespace, branchName)
	}

	// If no project information available, return empty string
	return ""
}

// constructProjectURL constructs a project URL using available project information
func (h *Handler) constructProjectURL(event *Event) (string, string) {
	// Priority 1: Use project information from system hook (most reliable)
	if event.Project != nil {
		projectName := event.Project.Name
		if projectName == "" {
			projectName = event.Name
		}
		return projectName, event.Project.WebURL
	}

	// Priority 2: Use path_with_namespace from system hook (from documentation)
	if event.PathWithNamespace != "" && h.config.GitLabBaseURL != "" {
		projectName := event.Name
		if projectName == "" {
			// Extract project name from path_with_namespace
			parts := strings.Split(event.PathWithNamespace, "/")
			if len(parts) > 0 {
				projectName = parts[len(parts)-1]
			}
		}
		return projectName, fmt.Sprintf("%s/%s", h.config.GitLabBaseURL, event.PathWithNamespace)
	}

	// Priority 3: Use project information from project hook
	if event.Project != nil && event.Project.PathWithNamespace != "" && h.config.GitLabBaseURL != "" {
		projectName := event.Project.Name
		if projectName == "" {
			// Extract project name from path_with_namespace
			parts := strings.Split(event.Project.PathWithNamespace, "/")
			if len(parts) > 0 {
				projectName = parts[len(parts)-1]
			}
		}
		return projectName, fmt.Sprintf("%s/%s", h.config.GitLabBaseURL, event.Project.PathWithNamespace)
	}

	// Priority 4: Fallback to using PathWithNamespace and GitLabBaseURL (legacy)
	if event.PathWithNamespace != "" && h.config.GitLabBaseURL != "" {
		projectName := event.Name
		if projectName == "" {
			projectName = event.PathWithNamespace
		}
		return projectName, fmt.Sprintf("%s/%s", h.config.GitLabBaseURL, event.PathWithNamespace)
	}

	// If no project information available, return empty strings
	return "", ""
}

// processMergeRequestEvent processes merge request events
func (h *Handler) processMergeRequestEvent(event *Event) error {
	if event.ObjectAttributes == nil {
		return nil
	}

	attrs := event.ObjectAttributes
	// Get issue-keys
	var allTexts []string
	allTexts = append(allTexts, attrs.Title)
	allTexts = append(allTexts, attrs.Description)
	allTexts = append(allTexts, attrs.SourceBranch)
	allTexts = append(allTexts, attrs.TargetBranch)
	// TODO: if there are comments, add them here

	issueKeySet := make(map[string]struct{})
	for _, text := range allTexts {
		for _, key := range h.parser.ExtractIssueIDs(text) {
			if key != "" {
				issueKeySet[key] = struct{}{}
			}
		}
	}

	// If no issue keys are found, do nothing
	if len(issueKeySet) == 0 {
		return nil
	}

	// Extract participants and approvers from MR using usernames instead of full names
	var participants, approvedBy, reviewers, approvers []userlink.UserWithLink
	if event.MergeRequest != nil {
		if event.MergeRequest.Author != nil {
			name := event.MergeRequest.Author.Username
			if name == "" {
				name = event.MergeRequest.Author.Name
			}
			participants = append(participants, userlink.UserWithLink{Name: name, URL: h.constructUserProfileURL(event.MergeRequest.Author)})
		}
		if event.MergeRequest.Assignee != nil {
			name := event.MergeRequest.Assignee.Username
			if name == "" {
				name = event.MergeRequest.Assignee.Name
			}
			participants = append(participants, userlink.UserWithLink{Name: name, URL: h.constructUserProfileURL(event.MergeRequest.Assignee)})
		}
		for _, participant := range event.MergeRequest.Participants {
			name := participant.Username
			if name == "" {
				name = participant.Name
			}
			participants = append(participants, userlink.UserWithLink{Name: name, URL: h.constructUserProfileURL(&participant)})
		}
		for _, user := range event.MergeRequest.ApprovedBy {
			name := user.Username
			if name == "" {
				name = user.Name
			}
			approvedBy = append(approvedBy, userlink.UserWithLink{Name: name, URL: h.constructUserProfileURL(&user)})
		}
		for _, user := range event.MergeRequest.Reviewers {
			name := user.Username
			if name == "" {
				name = user.Name
			}
			reviewers = append(reviewers, userlink.UserWithLink{Name: name, URL: h.constructUserProfileURL(&user)})
		}
		for _, user := range event.MergeRequest.Approvers {
			name := user.Username
			if name == "" {
				name = user.Name
			}
			approvers = append(approvers, userlink.UserWithLink{Name: name, URL: h.constructUserProfileURL(&user)})
		}

		// Log MR data for debugging
		h.logger.Debug("MR data extracted",
			"participants", participants,
			"approvedBy", approvedBy,
			"reviewers", reviewers,
			"approvers", approvers,
			"mrID", attrs.ID)
	}

	// Get project information and construct branch URLs
	projectName, projectURL := h.constructProjectURL(event)
	sourceBranchURL := h.constructBranchURL(event, "refs/heads/"+attrs.SourceBranch)
	targetBranchURL := h.constructBranchURL(event, "refs/heads/"+attrs.TargetBranch)

	// Use event time from the event itself for MR comments
	// This ensures we get the actual time when the MR event occurred
	eventTime := event.UpdatedAt
	if eventTime == "" {
		eventTime = event.CreatedAt
	}
	// If event level doesn't have time, fallback to object_attributes
	if eventTime == "" {
		eventTime = attrs.UpdatedAt
		if eventTime == "" {
			eventTime = attrs.CreatedAt
		}
	}

	// Generate ADF comment for MR with clickable branch links
	comment := jira.GenerateMergeRequestADFCommentWithBranchURLs(
		attrs.Title,
		attrs.URL,
		projectName,
		projectURL,
		cases.Title(language.English).String(attrs.Action),
		attrs.SourceBranch,
		sourceBranchURL,
		attrs.TargetBranch,
		targetBranchURL,
		attrs.State,
		attrs.Name,
		attrs.Description,
		eventTime,
		h.config.Timezone,
		participants,
		approvedBy,
		reviewers,
		approvers,
	)

	// Add comment to each issue
	for issueID := range issueKeySet {
		if err := h.jira.AddComment(issueID, comment); err != nil {
			h.logger.Error("Failed to add MR comment to Jira",
				"error", err,
				"issueID", issueID,
				"mrID", attrs.ID)
		} else {
			h.logger.Info("Added MR comment to Jira issue",
				"issueID", issueID,
				"mrID", attrs.ID)
		}
	}

	return nil
}

// processProjectEvent processes project creation and destruction events
func (h *Handler) processProjectEvent(event *Event) error {
	// Use System Hook specific fields
	projectName := event.Name
	projectPath := event.Path
	projectPathWithNamespace := event.PathWithNamespace
	ownerName := event.OwnerName
	ownerEmail := event.OwnerEmail
	projectVisibility := event.ProjectVisibility

	action := "created"
	if event.Type == "project_destroy" {
		action = "destroyed"
	} else if event.Type == "project_rename" {
		action = "renamed"
		// Include old path information
		oldPath := event.OldPathWithNamespace
		comment := fmt.Sprintf("Project %s: [%s](%s) - %s by %s (%s)\nOld path: %s",
			action,
			projectName,
			fmt.Sprintf("%s/%s", h.config.GitLabBaseURL, projectPathWithNamespace),
			projectVisibility,
			ownerName,
			ownerEmail,
			oldPath)

		// Extract Jira issue IDs from project name and path
		text := projectName + " " + projectPath + " " + oldPath
		issueIDs := h.parser.ExtractIssueIDs(text)

		for _, issueID := range issueIDs {
			if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
				h.logger.Error("Failed to add project rename comment to Jira",
					"error", err,
					"issueID", issueID,
					"projectName", projectName)
			} else {
				h.logger.Info("Added project rename comment to Jira issue",
					"issueID", issueID,
					"projectName", projectName)
			}
		}
		return nil
	}

	comment := fmt.Sprintf("Project %s: [%s](%s) - %s by %s (%s)",
		action,
		projectName,
		fmt.Sprintf("%s/%s", h.config.GitLabBaseURL, projectPathWithNamespace),
		projectVisibility,
		ownerName,
		ownerEmail)

	// Extract Jira issue IDs from project name and path
	text := projectName + " " + projectPath
	issueIDs := h.parser.ExtractIssueIDs(text)

	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
			h.logger.Error("Failed to add project comment to Jira",
				"error", err,
				"issueID", issueID,
				"projectName", projectName,
				"eventType", event.Type)
		} else {
			h.logger.Info("Added project comment to Jira issue",
				"issueID", issueID,
				"projectName", projectName,
				"eventType", event.Type)
		}
	}

	return nil
}

// processUserEvent processes user creation and destruction events
func (h *Handler) processUserEvent(event *Event) error {
	// Use System Hook specific fields
	userName := event.Name
	userEmail := event.Email
	userUsername := event.Username
	userState := event.State

	action := "created"
	if event.Type == "user_destroy" {
		action = "removed"
	} else if event.Type == "user_rename" {
		action = "renamed"
		oldUsername := event.OldUsername
		comment := fmt.Sprintf("User %s: [%s](%s) (%s) - %s\nOld username: %s",
			action,
			userName,
			fmt.Sprintf("%s/%s", h.config.GitLabBaseURL, userUsername),
			userEmail,
			userState,
			oldUsername)

		// Extract Jira issue IDs from user name and username
		text := userName + " " + userUsername + " " + oldUsername
		issueIDs := h.parser.ExtractIssueIDs(text)

		for _, issueID := range issueIDs {
			if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
				h.logger.Error("Failed to add user rename comment to Jira",
					"error", err,
					"issueID", issueID,
					"userName", userName)
			} else {
				h.logger.Info("Added user rename comment to Jira issue",
					"issueID", issueID,
					"userName", userName)
			}
		}
		return nil
	} else if event.Type == "user_failed_login" {
		comment := fmt.Sprintf("User failed login: [%s](%s) (%s) - %s",
			userName,
			fmt.Sprintf("%s/%s", h.config.GitLabBaseURL, userUsername),
			userEmail,
			userState)

		// Extract Jira issue IDs from user name and email
		text := userName + " " + userEmail
		issueIDs := h.parser.ExtractIssueIDs(text)

		for _, issueID := range issueIDs {
			if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
				h.logger.Error("Failed to add user failed login comment to Jira",
					"error", err,
					"issueID", issueID,
					"userName", userName)
			} else {
				h.logger.Info("Added user failed login comment to Jira issue",
					"issueID", issueID,
					"userName", userName)
			}
		}
		return nil
	}

	comment := fmt.Sprintf("User %s: [%s](%s) (%s) - %s",
		action,
		userName,
		fmt.Sprintf("%s/%s", h.config.GitLabBaseURL, userUsername),
		userEmail,
		userState)

	// Extract Jira issue IDs from user name and email
	text := userName + " " + userEmail + " " + userUsername
	issueIDs := h.parser.ExtractIssueIDs(text)

	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
			h.logger.Error("Failed to add user comment to Jira",
				"error", err,
				"issueID", issueID,
				"userName", userName,
				"eventType", event.Type)
		} else {
			h.logger.Info("Added user comment to Jira issue",
				"issueID", issueID,
				"userName", userName,
				"eventType", event.Type)
		}
	}

	return nil
}

// processTeamEvent processes team membership events
func (h *Handler) processTeamEvent(event *Event) error {
	if event.Team == nil || event.User == nil {
		h.logger.Debug("Team event without team or user data", "type", event.Type)
		return nil
	}

	team := event.Team
	user := event.User
	action := "added to"
	if event.Type == "user_remove_from_team" {
		action = "removed from"
	}

	comment := fmt.Sprintf("User %s](%s) %s team [%s](%s) - %s",
		user.Name,
		user.AvatarURL,
		action,
		team.Name,
		team.WebURL,
		team.Description)

	// Try to extract Jira issue ID from team name, description, or user info
	text := team.Name + " " + team.Description + " " + user.Name + " " + user.Email
	issueIDs := h.parser.ExtractIssueIDs(text)

	if len(issueIDs) == 0 {
		h.logger.Debug("No Jira issue IDs found in team event", "teamName", team.Name,
			"userName", user.Name,
			"eventType", event.Type)
		return nil
	}

	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
			h.logger.Error("Failed to add team comment to Jira",
				"error", err,
				"issueID", issueID,
				"teamID", team.ID,
				"userID", user.ID,
				"eventType", event.Type)
		} else {
			h.logger.Info("Added team comment to Jira issue",
				"issueID", issueID,
				"teamID", team.ID,
				"userID", user.ID,
				"eventType", event.Type)
		}
	}

	return nil
}

// processGroupEvent processes group membership events
func (h *Handler) processGroupEvent(event *Event) error {
	// Use System Hook specific fields
	groupName := event.GroupName
	groupPath := event.Path
	groupFullPath := event.FullPath

	action := "created"
	if event.Type == "group_destroy" {
		action = "destroyed"
	} else if event.Type == "group_rename" {
		action = "renamed"
		oldPath := event.OldPath
		oldFullPath := event.OldFullPath
		comment := fmt.Sprintf("Group %s: [%s](%s) - %s\nOld path: %s\nOld full path: %s",
			action,
			groupName,
			fmt.Sprintf("%s/%s", h.config.GitLabBaseURL, groupFullPath),
			groupPath,
			oldPath,
			oldFullPath)

		// Extract Jira issue IDs from group name and path
		text := groupName + " " + groupPath + " " + oldPath
		issueIDs := h.parser.ExtractIssueIDs(text)

		for _, issueID := range issueIDs {
			if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
				h.logger.Error("Failed to add group rename comment to Jira",
					"error", err,
					"issueID", issueID,
					"groupName", groupName)
			} else {
				h.logger.Info("Added group rename comment to Jira issue",
					"issueID", issueID,
					"groupName", groupName)
			}
		}
		return nil
	}

	comment := fmt.Sprintf("Group %s: [%s](%s) - %s",
		action,
		groupName,
		fmt.Sprintf("%s/%s", h.config.GitLabBaseURL, groupFullPath),
		groupPath)

	// Extract Jira issue IDs from group name and path
	text := groupName + " " + groupPath
	issueIDs := h.parser.ExtractIssueIDs(text)

	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
			h.logger.Error("Failed to add group comment to Jira",
				"error", err,
				"issueID", issueID,
				"groupName", groupName,
				"eventType", event.Type)
		} else {
			h.logger.Info("Added group comment to Jira issue",
				"issueID", issueID,
				"groupName", groupName,
				"eventType", event.Type)
		}
	}

	return nil
}

// isAllowedEvent checks if the event is allowed by project/group filter
func (h *Handler) isAllowedEvent(event *Event) bool {
	// If no filters set, allow all
	if len(h.config.AllowedProjects) == 0 && len(h.config.AllowedGroups) == 0 {
		return true
	}

	// Check project filtering
	if len(h.config.AllowedProjects) > 0 {
		projectNames := []string{}
		if event.Project != nil {
			projectNames = append(projectNames, event.Project.Name, event.Project.PathWithNamespace)
		}
		if event.PathWithNamespace != "" {
			projectNames = append(projectNames, event.PathWithNamespace)
		}
		if event.ProjectName != "" {
			projectNames = append(projectNames, event.ProjectName)
		}
		for _, p := range h.config.AllowedProjects {
			for _, name := range projectNames {
				// Check exact match
				if name == p {
					return true
				}
				// Check if project path starts with allowed group (for group-based filtering)
				if strings.HasPrefix(name, p+"/") {
					return true
				}
			}
		}
	}

	// Check group filtering
	if len(h.config.AllowedGroups) > 0 {
		groupNames := []string{}
		if event.Group != nil {
			groupNames = append(groupNames, event.Group.Name, event.Group.FullPath)
		}
		if event.FullPath != "" {
			groupNames = append(groupNames, event.FullPath)
		}
		if event.GroupName != "" {
			groupNames = append(groupNames, event.GroupName)
		}
		for _, g := range h.config.AllowedGroups {
			for _, name := range groupNames {
				if name == g {
					return true
				}
			}
		}
	}

	return false
}

// getProjectPath extracts project path from event
func (h *Handler) getProjectPath(event *Event) string {
	if event.Project != nil {
		return event.Project.PathWithNamespace
	} else if event.PathWithNamespace != "" {
		return event.PathWithNamespace
	} else if event.ProjectName != "" && event.Username != "" {
		return event.Username + "/" + event.ProjectName
	}
	return ""
}

// getGroupPath extracts group path from event
func (h *Handler) getGroupPath(event *Event) string {
	if event.Group != nil {
		return event.Group.FullPath
	} else if event.FullPath != "" {
		return event.FullPath
	} else if event.GroupName != "" {
		return event.GroupName
	}
	return ""
}

// Added event handlers for new event types

func (h *Handler) processRepositoryEvent(event *Event) error {
	h.logger.Info("Repository event", "type", event.Type)

	action := "created"
	if event.Type == "repository_destroy" {
		action = "destroyed"
	}

	comment := fmt.Sprintf("Repository %s: [%s](%s) - %s",
		action,
		event.Repository.Name,
		event.Repository.URL,
		event.Repository.Description)

	// Extract Jira issue IDs from repository name and description
	text := event.Repository.Name + " " + event.Repository.Description
	issueIDs := h.parser.ExtractIssueIDs(text)

	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
			h.logger.Error("Failed to add repository comment to Jira",
				"error", err,
				"issueID", issueID,
				"repositoryName", event.Repository.Name)
		} else {
			h.logger.Info("Added repository comment to Jira issue",
				"issueID", issueID,
				"repositoryName", event.Repository.Name)
		}
	}

	return nil
}

func (h *Handler) processRepositoryUpdateEvent(event *Event) error {
	h.logger.Info("Repository update event", "type", event.Type)

	// Get project information
	projectName := "Unknown Project"
	projectURL := ""
	if event.Project != nil {
		projectName = event.Project.Name
		projectURL = event.Project.WebURL
	}

	// Get user information
	userName := event.UserName
	if userName == "" {
		userName = event.Username
	}

	// Build comment with project and user information
	comment := fmt.Sprintf("Repository updated: [%s](%s)\nUser: %s\nProject: [%s](%s)",
		projectName,
		projectURL,
		userName,
		projectName,
		projectURL)

	// Add information about changes if available
	if len(event.Changes) > 0 {
		comment += "\nChanges:"
		for _, change := range event.Changes {
			// Extract branch name from ref
			branchName := change.Ref
			if strings.HasPrefix(change.Ref, "refs/heads/") {
				branchName = strings.TrimPrefix(change.Ref, "refs/heads/")
			} else if strings.HasPrefix(change.Ref, "refs/tags/") {
				branchName = strings.TrimPrefix(change.Ref, "refs/tags/")
			}

			comment += fmt.Sprintf("\n- Branch: `%s` (%s → %s)",
				branchName,
				change.Before[:8], // Show first 8 characters of commit hash
				change.After[:8])
		}
	}

	// Extract Jira issue IDs from project name and user name
	text := projectName + " " + userName
	issueIDs := h.parser.ExtractIssueIDs(text)

	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
			h.logger.Error("Failed to add repository update comment to Jira",
				"error", err,
				"issueID", issueID,
				"projectName", projectName)
		} else {
			h.logger.Info("Added repository update comment to Jira issue",
				"issueID", issueID,
				"projectName", projectName)
		}
	}

	return nil
}

func (h *Handler) processTeamCreateDestroyEvent(event *Event) error {
	h.logger.Info("Team create/destroy event", "type", event.Type)

	action := "created"
	if event.Type == "team_destroy" {
		action = "destroyed"
	}

	comment := fmt.Sprintf("Team %s: [%s](%s) - %s",
		action,
		event.Team.Name,
		event.Team.WebURL,
		event.Team.Description)

	// Extract Jira issue IDs from team name and description
	text := event.Team.Name + " " + event.Team.Description
	issueIDs := h.parser.ExtractIssueIDs(text)

	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
			h.logger.Error("Failed to add team comment to Jira",
				"error", err,
				"issueID", issueID,
				"teamName", event.Team.Name)
		} else {
			h.logger.Info("Added team comment to Jira issue",
				"issueID", issueID,
				"teamName", event.Team.Name)
		}
	}

	return nil
}

func (h *Handler) processGroupCreateDestroyEvent(event *Event) error {
	h.logger.Info("Group create/destroy event", "type", event.Type)

	action := "created"
	if event.Type == "group_destroy" {
		action = "destroyed"
	}

	comment := fmt.Sprintf("Group %s: [%s](%s) - %s",
		action,
		event.Group.Name,
		event.Group.WebURL,
		event.Group.Description)

	// Extract Jira issue IDs from group name and description
	text := event.Group.Name + " " + event.Group.Description
	issueIDs := h.parser.ExtractIssueIDs(text)

	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
			h.logger.Error("Failed to add group comment to Jira",
				"error", err,
				"issueID", issueID,
				"groupName", event.Group.Name)
		} else {
			h.logger.Info("Added group comment to Jira issue",
				"issueID", issueID,
				"groupName", event.Group.Name)
		}
	}

	return nil
}

func (h *Handler) processProjectMembershipEvent(event *Event) error {
	h.logger.Info("Project membership event", "type", event.Type)

	action := "added to"
	if event.Type == "user_remove_from_project" {
		action = "removed from"
	}

	comment := fmt.Sprintf("User [%s](%s) %s project [%s](%s) - Access Level: %d",
		event.Username,
		fmt.Sprintf("%s/%s", h.config.GitLabBaseURL, event.Username),
		action,
		event.ProjectName,
		fmt.Sprintf("%s/%s", h.config.GitLabBaseURL, event.PathWithNamespace),
		event.ProjectAccessLevel)

	// Extract Jira issue IDs from user and project names
	text := event.Username + " " + event.ProjectName
	issueIDs := h.parser.ExtractIssueIDs(text)

	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
			h.logger.Error("Failed to add project membership comment to Jira",
				"error", err,
				"issueID", issueID,
				"username", event.Username,
				"projectName", event.ProjectName)
		} else {
			h.logger.Info("Added project membership comment to Jira issue",
				"issueID", issueID,
				"username", event.Username,
				"projectName", event.ProjectName)
		}
	}

	return nil
}

func (h *Handler) processKeyEvent(event *Event) error {
	h.logger.Info("Key event", "type", event.Type)

	action := "created"
	if event.Type == "key_destroy" {
		action = "destroyed"
	}

	username := event.Username
	keyID := event.KeyID

	comment := fmt.Sprintf("SSH Key %s for user [%s](%s) (ID: %d)",
		action,
		username,
		fmt.Sprintf("%s/%s", h.config.GitLabBaseURL, username),
		keyID)

	// Extract Jira issue IDs from username
	issueIDs := h.parser.ExtractIssueIDs(username)

	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
			h.logger.Error("Failed to add key comment to Jira",
				"error", err,
				"issueID", issueID,
				"username", username)
		} else {
			h.logger.Info("Added key comment to Jira issue",
				"issueID", issueID,
				"username", username)
		}
	}

	return nil
}

func (h *Handler) processTagPushEvent(event *Event) error {
	h.logger.Info("Tag push event", "type", event.Type)
	if event.ObjectAttributes == nil {
		return nil
	}
	// Extract issueID from ref
	issueIDs := h.parser.ExtractIssueIDs(event.ObjectAttributes.Ref)
	comment := jira.GenerateTagPushADFComment(
		event.ObjectAttributes.Ref,
		event.ObjectAttributes.URL,
		"", // projectName - not available in System Hook
		"", // projectURL - not available in System Hook
		cases.Title(language.English).String(event.ObjectAttributes.Action),
		event.ObjectAttributes.Name,
		h.config.Timezone,
	)
	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, comment); err != nil {
			h.logger.Error("Failed to add tag push comment to Jira", "error", err, "issueID", issueID)
		} else {
			h.logger.Info("Added tag push comment to Jira issue", "issueID", issueID)
		}
	}
	return nil
}

func (h *Handler) processReleaseEvent(event *Event) error {
	h.logger.Info("Release event", "type", event.Type)
	if event.ObjectAttributes == nil {
		return nil
	}
	// Extract issueID from name and description
	text := event.ObjectAttributes.Name
	if event.ObjectAttributes.Description != "" {
		text += " " + event.ObjectAttributes.Description
	}
	issueIDs := h.parser.ExtractIssueIDs(text)
	comment := jira.GenerateReleaseADFComment(
		event.ObjectAttributes.Name,
		event.ObjectAttributes.URL,
		"", // projectName - not available in System Hook
		"", // projectURL - not available in System Hook
		cases.Title(language.English).String(event.ObjectAttributes.Action),
		event.ObjectAttributes.Ref,
		event.ObjectAttributes.Description,
		event.ObjectAttributes.Name,
		h.config.Timezone,
	)
	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, comment); err != nil {
			h.logger.Error("Failed to add release comment to Jira", "error", err, "issueID", issueID)
		} else {
			h.logger.Info("Added release comment to Jira issue", "issueID", issueID)
		}
	}
	return nil
}

func (h *Handler) processDeploymentEvent(event *Event) error {
	h.logger.Info("Deployment event", "type", event.Type)
	if event.ObjectAttributes == nil {
		return nil
	}
	// Extract issueID from ref and environment
	text := event.ObjectAttributes.Ref
	if event.ObjectAttributes.Environment != "" {
		text += " " + event.ObjectAttributes.Environment
	}
	issueIDs := h.parser.ExtractIssueIDs(text)
	comment := jira.GenerateDeploymentADFComment(
		event.ObjectAttributes.Ref,
		event.ObjectAttributes.URL,
		"", // projectName - not available in System Hook
		"", // projectURL - not available in System Hook
		cases.Title(language.English).String(event.ObjectAttributes.Action),
		event.ObjectAttributes.Environment,
		event.ObjectAttributes.Status,
		event.ObjectAttributes.Sha,
		event.ObjectAttributes.Name,
		h.config.Timezone,
	)
	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, comment); err != nil {
			h.logger.Error("Failed to add deployment comment to Jira", "error", err, "issueID", issueID)
		} else {
			h.logger.Info("Added deployment comment to Jira issue", "issueID", issueID)
		}
	}
	return nil
}

func (h *Handler) processFeatureFlagEvent(event *Event) error {
	h.logger.Info("Feature flag event", "type", event.Type)
	if event.ObjectAttributes == nil {
		return nil
	}
	// Extract issueID from name and description
	text := event.ObjectAttributes.Name
	if event.ObjectAttributes.Description != "" {
		text += " " + event.ObjectAttributes.Description
	}
	issueIDs := h.parser.ExtractIssueIDs(text)
	comment := jira.GenerateFeatureFlagADFComment(
		event.ObjectAttributes.Name,
		event.ObjectAttributes.URL,
		"", // projectName - not available in System Hook
		"", // projectURL - not available in System Hook
		cases.Title(language.English).String(event.ObjectAttributes.Action),
		event.ObjectAttributes.Description,
		event.ObjectAttributes.Name,
		h.config.Timezone,
	)
	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, comment); err != nil {
			h.logger.Error("Failed to add feature flag comment to Jira", "error", err, "issueID", issueID)
		} else {
			h.logger.Info("Added feature flag comment to Jira issue", "issueID", issueID)
		}
	}
	return nil
}

func (h *Handler) processWikiPageEvent(event *Event) error {
	h.logger.Info("Wiki page event", "type", event.Type)
	if event.ObjectAttributes == nil {
		return nil
	}
	// Extract issueID from title and content
	text := event.ObjectAttributes.Title
	if event.ObjectAttributes.Content != "" {
		text += " " + event.ObjectAttributes.Content
	}
	issueIDs := h.parser.ExtractIssueIDs(text)
	comment := jira.GenerateWikiPageADFComment(
		event.ObjectAttributes.Title,
		event.ObjectAttributes.URL,
		"", // projectName - not available in System Hook
		"", // projectURL - not available in System Hook
		cases.Title(language.English).String(event.ObjectAttributes.Action),
		event.ObjectAttributes.Name,
		event.ObjectAttributes.Content,
		h.config.Timezone,
	)
	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, comment); err != nil {
			h.logger.Error("Failed to add wiki page comment to Jira", "error", err, "issueID", issueID)
		} else {
			h.logger.Info("Added wiki page comment to Jira issue", "issueID", issueID)
		}
	}
	return nil
}

func (h *Handler) processPipelineEvent(event *Event) error {
	h.logger.Info("Pipeline event", "type", event.Type)
	if event.ObjectAttributes == nil {
		return nil
	}
	// Extract issueID from ref and title
	text := event.ObjectAttributes.Ref
	if event.ObjectAttributes.Title != "" {
		text += " " + event.ObjectAttributes.Title
	}
	issueIDs := h.parser.ExtractIssueIDs(text)

	// Get project information
	projectName, projectURL := h.constructProjectURL(event)

	comment := jira.GeneratePipelineADFComment(
		event.ObjectAttributes.Ref,
		event.ObjectAttributes.URL,
		projectName,
		projectURL,
		cases.Title(language.English).String(event.ObjectAttributes.Action),
		event.ObjectAttributes.Status,
		event.ObjectAttributes.Sha,
		event.ObjectAttributes.Name,
		event.ObjectAttributes.Duration,
		h.config.Timezone,
	)
	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, comment); err != nil {
			h.logger.Error("Failed to add pipeline comment to Jira", "error", err, "issueID", issueID)
		} else {
			h.logger.Info("Added pipeline comment to Jira issue", "issueID", issueID)
		}
	}
	return nil
}

func (h *Handler) processBuildEvent(event *Event) error {
	h.logger.Info("Build event", "type", event.Type)
	if event.ObjectAttributes == nil {
		return nil
	}
	// Extract issueID from name and stage
	text := event.ObjectAttributes.Name
	if event.ObjectAttributes.Stage != "" {
		text += " " + event.ObjectAttributes.Stage
	}
	issueIDs := h.parser.ExtractIssueIDs(text)

	// Get project information
	projectName, projectURL := h.constructProjectURL(event)

	comment := jira.GenerateBuildADFComment(
		event.ObjectAttributes.Name,
		event.ObjectAttributes.URL,
		projectName,
		projectURL,
		cases.Title(language.English).String(event.ObjectAttributes.Action),
		event.ObjectAttributes.Status,
		event.ObjectAttributes.Stage,
		event.ObjectAttributes.Ref,
		event.ObjectAttributes.Sha,
		event.ObjectAttributes.Name,
		event.ObjectAttributes.Duration,
		h.config.Timezone,
	)
	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, comment); err != nil {
			h.logger.Error("Failed to add build comment to Jira", "error", err, "issueID", issueID)
		} else {
			h.logger.Info("Added build comment to Jira issue", "issueID", issueID)
		}
	}
	return nil
}

func (h *Handler) processNoteEvent(event *Event) error {
	h.logger.Info("Note event", "type", event.Type)
	if event.ObjectAttributes == nil {
		return nil
	}
	// Extract issueID from note
	issueIDs := h.parser.ExtractIssueIDs(event.ObjectAttributes.Note)
	comment := jira.GenerateNoteADFComment(
		event.ObjectAttributes.Note,
		event.ObjectAttributes.URL,
		"", // projectName - not available in System Hook
		"", // projectURL - not available in System Hook
		cases.Title(language.English).String(event.ObjectAttributes.Action),
		event.ObjectAttributes.Name,
		event.ObjectAttributes.Note,
		h.config.Timezone,
	)
	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, comment); err != nil {
			h.logger.Error("Failed to add note comment to Jira", "error", err, "issueID", issueID)
		} else {
			h.logger.Info("Added note comment to Jira issue", "issueID", issueID)
		}
	}
	return nil
}

func (h *Handler) processIssueEvent(event *Event) error {
	h.logger.Info("Issue event", "type", event.Type)
	if event.ObjectAttributes == nil {
		return nil
	}
	// Extract issueID from title and description
	text := event.ObjectAttributes.Title
	if event.ObjectAttributes.Description != "" {
		text += " " + event.ObjectAttributes.Description
	}
	issueIDs := h.parser.ExtractIssueIDs(text)

	// Get project information
	projectName, projectURL := h.constructProjectURL(event)

	comment := jira.GenerateIssueADFComment(
		event.ObjectAttributes.Title,
		event.ObjectAttributes.URL,
		projectName,
		projectURL,
		cases.Title(language.English).String(event.ObjectAttributes.Action),
		event.ObjectAttributes.State,
		event.ObjectAttributes.IssueType,
		event.ObjectAttributes.Priority,
		event.ObjectAttributes.Name,
		event.ObjectAttributes.Description,
		h.config.Timezone,
	)
	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, comment); err != nil {
			h.logger.Error("Failed to add issue comment to Jira", "error", err, "issueID", issueID)
		} else {
			h.logger.Info("Added issue comment to Jira issue", "issueID", issueID)
		}
	}
	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// convertToInterfaceEvent преобразует *gitlab.Event в *webhook.Event
func (h *Handler) convertToInterfaceEvent(e *Event) *webhook.Event {
	if e == nil {
		return nil
	}
	var commits []webhook.Commit
	for _, c := range e.Commits {
		var ts time.Time
		if t, err := parseTime(c.Timestamp); err == nil {
			ts = t
		}
		commits = append(commits, webhook.Commit{
			ID:        c.ID,
			Message:   c.Message,
			URL:       c.URL,
			Author:    webhook.Author{Name: c.Author.Name, Email: c.Author.Email},
			Timestamp: ts,
			Added:     c.Added,
			Modified:  c.Modified,
			Removed:   c.Removed,
		})
	}

	result := &webhook.Event{
		Type:      e.Type,
		EventName: e.EventName,
		Commits:   commits,
	}

	// Safely set Project if available
	if e.Project != nil {
		result.Project = &webhook.Project{
			ID:                e.Project.ID,
			Name:              e.Project.Name,
			PathWithNamespace: e.Project.PathWithNamespace,
			WebURL:            e.Project.WebURL,
		}
	}

	// Safely set Group if available
	if e.Group != nil {
		result.Group = &webhook.Group{
			ID:       e.Group.ID,
			Name:     e.Group.Name,
			FullPath: e.Group.FullPath,
		}
	}

	// Safely set User if available
	if e.User != nil {
		result.User = &webhook.User{
			ID:       e.User.ID,
			Username: e.User.Username,
			Name:     e.User.Name,
			Email:    e.User.Email,
		}
	}

	// Safely set ObjectAttributes if available
	if e.ObjectAttributes != nil {
		result.ObjectAttributes = &webhook.ObjectAttributes{
			ID:          e.ObjectAttributes.ID,
			Title:       e.ObjectAttributes.Title,
			Description: e.ObjectAttributes.Description,
			State:       e.ObjectAttributes.State,
			Action:      e.ObjectAttributes.Action,
			Ref:         e.ObjectAttributes.Ref,
			URL:         e.ObjectAttributes.URL,
			SHA:         e.ObjectAttributes.Sha,
			Name:        e.ObjectAttributes.Name,
			Duration:    e.ObjectAttributes.Duration,
			Status:      e.ObjectAttributes.Status,
			IssueType:   e.ObjectAttributes.IssueType,
			Priority:    e.ObjectAttributes.Priority,
		}
	}

	return result
}

// parseTime пытается распарсить строку времени в time.Time
func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	// Попробуем RFC3339, иначе вернем zero time
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, err
	}
	return ts, nil
}

// constructUserProfileURL constructs a user profile URL using available user information
func (h *Handler) constructUserProfileURL(user *User) string {
	if h.config.GitLabBaseURL == "" || user == nil {
		return ""
	}
	if user.ID > 0 {
		return fmt.Sprintf("%s/-/profile/%d", h.config.GitLabBaseURL, user.ID)
	}
	if user.Username != "" {
		return fmt.Sprintf("%s/%s", h.config.GitLabBaseURL, user.Username)
	}
	return ""
}
