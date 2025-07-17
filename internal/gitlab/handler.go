package gitlab

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/jira"
)

// Handler handles GitLab webhook requests
type Handler struct {
	config *config.Config
	logger *slog.Logger
	jira   JiraClient
	parser *Parser
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

			// Construct author URL if we have GitLab base URL
			authorURL := ""
			if h.config.GitLabBaseURL != "" {
				authorURL = fmt.Sprintf("%s/%s", h.config.GitLabBaseURL, commit.Author.Name)
			}

			comment := jira.GenerateCommitADFComment(
				commit.ID,
				commit.URL,
				commit.Author.Name,
				commit.Author.Email,
				authorURL,
				commit.Message,
				commit.Timestamp,
				event.Ref,
				branchURL,
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

// constructBranchURL constructs a proper branch URL using project information
func (h *Handler) constructBranchURL(event *Event, ref string) string {
	// Extract branch name from refs/heads/branch format
	branchName := ref
	if strings.HasPrefix(ref, "refs/heads/") {
		branchName = strings.TrimPrefix(ref, "refs/heads/")
	}

	// Try to use project information from system hook first
	if event.Project != nil && event.Project.WebURL != "" {
		return fmt.Sprintf("%s/-/tree/%s", event.Project.WebURL, branchName)
	}

	// Fallback to using PathWithNamespace and GitLabBaseURL
	if event.PathWithNamespace != "" && h.config.GitLabBaseURL != "" {
		return fmt.Sprintf("%s/%s/-/tree/%s", h.config.GitLabBaseURL, event.PathWithNamespace, branchName)
	}

	// If no project information available, return empty string
	return ""
}

// constructProjectURL constructs a project URL using available project information
func (h *Handler) constructProjectURL(event *Event) (string, string) {
	// Try to use project information from system hook first
	if event.Project != nil {
		return event.Project.Name, event.Project.WebURL
	}

	// Fallback to using PathWithNamespace and GitLabBaseURL
	if event.PathWithNamespace != "" && h.config.GitLabBaseURL != "" {
		projectName := event.PathWithNamespace
		if event.Name != "" {
			projectName = event.Name
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

	// Extract participants and approvers from MR
	var participants, approvedBy, reviewers, approvers []string
	if event.MergeRequest != nil {
		if event.MergeRequest.Author != nil {
			participants = append(participants, event.MergeRequest.Author.Name)
		}
		if event.MergeRequest.Assignee != nil {
			participants = append(participants, event.MergeRequest.Assignee.Name)
		}
		for _, participant := range event.MergeRequest.Participants {
			participants = append(participants, participant.Name)
		}
		for _, user := range event.MergeRequest.ApprovedBy {
			approvedBy = append(approvedBy, user.Name)
		}
		for _, user := range event.MergeRequest.Reviewers {
			reviewers = append(reviewers, user.Name)
		}
		for _, user := range event.MergeRequest.Approvers {
			approvers = append(approvers, user.Name)
		}
	}

	// Get project information and construct branch URLs
	projectName, projectURL := h.constructProjectURL(event)
	sourceBranchURL := h.constructBranchURL(event, "refs/heads/"+attrs.SourceBranch)
	targetBranchURL := h.constructBranchURL(event, "refs/heads/"+attrs.TargetBranch)

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
