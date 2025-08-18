package gitlab

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/jira"
)

// EventProcessor handles GitLab webhook event processing
type EventProcessor struct {
	jiraClient interface {
		AddComment(ctx context.Context, issueID string, payload jira.CommentPayload) error
		TestConnection(ctx context.Context) error
		SearchIssues(ctx context.Context, jql string) ([]jira.JiraIssue, error)
	}
	urlBuilder *URLBuilder
	config     *config.Config
	logger     *slog.Logger
	parser     *Parser
}

// NewEventProcessor creates a new event processor
func NewEventProcessor(jiraClient interface {
	AddComment(ctx context.Context, issueID string, payload jira.CommentPayload) error
	TestConnection(ctx context.Context) error
	SearchIssues(ctx context.Context, jql string) ([]jira.JiraIssue, error)
}, urlBuilder *URLBuilder, cfg *config.Config, logger *slog.Logger) *EventProcessor {
	return &EventProcessor{
		jiraClient: jiraClient,
		urlBuilder: urlBuilder,
		config:     cfg,
		logger:     logger,
		parser:     NewParser(),
	}
}

// shouldProcessEvent checks if an event should be processed based on JQL filter
func (ep *EventProcessor) shouldProcessEvent(ctx context.Context, event *Event) (bool, error) {
	// If no JQL filter is configured, process all events
	if ep.config.JQLFilter == "" {
		return true, nil
	}

	// Extract issue IDs from the event
	issueIDs := ep.extractIssueIDsFromEvent(event)
	
	// If no issue IDs found, process the event (to avoid missing important events)
	if len(issueIDs) == 0 {
		return true, nil
	}

	// For each issue ID, check if it matches the JQL filter
	for _, issueID := range issueIDs {
		// Create JQL query to check if this specific issue matches the filter
		jql := fmt.Sprintf("(%s) AND issueKey = %s", ep.config.JQLFilter, issueID)
		
		// Execute JQL query
		issues, err := ep.jiraClient.SearchIssues(ctx, jql)
		if err != nil {
			ep.logger.Warn("Failed to execute JQL filter",
				"error", err,
				"jql", jql,
				"issue_id", issueID)
			// If JQL execution fails, process the event to avoid missing important events
			return true, nil
		}
		
		// If any issues match the filter, process the event
		if len(issues) > 0 {
			return true, nil
		}
	}
	
	// If none of the issues match the filter, skip processing
	return false, nil
}

// extractIssueIDsFromEvent extracts all Jira issue IDs from an event
func (ep *EventProcessor) extractIssueIDsFromEvent(event *Event) []string {
	var issueIDs []string
	
	// Extract from commits
	for _, commit := range event.Commits {
		issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(commit.Message)...)
	}
	
	// Extract from merge request
	if event.MergeRequest != nil {
		issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.MergeRequest.Title)...)
		issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.MergeRequest.Description)...)
	}
	
	// Extract from issue
	if event.Issue != nil {
		issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.Issue.Title)...)
		issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.Issue.Description)...)
	}
	
	// Extract from note
	if event.Note != nil {
		issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.Note.Note)...)
	}
	
	// Extract from object attributes
	if event.ObjectAttributes != nil {
		issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.ObjectAttributes.Title)...)
		issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.ObjectAttributes.Description)...)
		issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.ObjectAttributes.Note)...)
		issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.ObjectAttributes.Ref)...)
	}
	
	// Extract from pipeline
	if event.Pipeline != nil {
		issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.Pipeline.Ref)...)
	}
	
	// Extract from build
	if event.Build != nil {
		issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.Build.Name)...)
	}
	
	// Extract from deployment
	if event.Deployment != nil {
		issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.Deployment.Environment)...)
	}
	
	// Extract from release
	if event.Release != nil {
		issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.Release.Name)...)
		issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.Release.Description)...)
	}
	
	// Extract from wiki page
	if event.WikiPage != nil {
		issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.WikiPage.Title)...)
		issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.WikiPage.Content)...)
	}
	
	// Extract from feature flag
	if event.FeatureFlag != nil {
		issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.FeatureFlag.Name)...)
		issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.FeatureFlag.Description)...)
	}
	
	// Remove duplicates
	uniqueIssueIDs := make(map[string]bool)
	var result []string
	for _, id := range issueIDs {
		if !uniqueIssueIDs[id] {
			uniqueIssueIDs[id] = true
			result = append(result, id)
		}
	}
	
	return result
}

// ProcessEvent processes a GitLab webhook event
func (ep *EventProcessor) ProcessEvent(ctx context.Context, event *Event) error {
	// Check if event should be processed based on JQL filter
	shouldProcess, err := ep.shouldProcessEvent(ctx, event)
	if err != nil {
		ep.logger.Error("Failed to check if event should be processed",
			"error", err)
		// Continue processing even if JQL check fails to avoid missing important events
	}
	
	if !shouldProcess {
		ep.logger.Debug("Skipping event due to JQL filter",
			"object_kind", event.ObjectKind,
			"event_type", event.Type,
			"project_id", event.Project.ID,
			"project_name", event.Project.Name)
		return nil
	}

	ep.logger.Info("Processing GitLab event",
		"object_kind", event.ObjectKind,
		"event_type", event.Type,
		"project_id", event.Project.ID,
		"project_name", event.Project.Name,
	)

	// Use Type if ObjectKind is empty
	eventType := event.ObjectKind
	if eventType == "" {
		eventType = event.Type
	}

	// If both are empty, try to determine from event structure
	if eventType == "" {
		if len(event.Commits) > 0 {
			eventType = "push"
		} else if event.MergeRequest != nil {
			eventType = "merge_request"
		} else if event.Issue != nil {
			eventType = "issue"
		} else if event.Note != nil {
			eventType = "note"
		} else if event.Pipeline != nil {
			eventType = "pipeline"
		} else if event.Build != nil {
			eventType = "job"
		} else if event.Deployment != nil {
			eventType = "deployment"
		} else if event.Release != nil {
			eventType = "release"
		} else if event.WikiPage != nil {
			eventType = "wiki_page"
		} else if event.FeatureFlag != nil {
			eventType = "feature_flag"
		} else if event.ObjectAttributes != nil {
			// Try to determine from object attributes
			if event.ObjectAttributes.TargetBranch != "" || event.ObjectAttributes.SourceBranch != "" {
				eventType = "merge_request"
			} else if event.ObjectAttributes.Description != "" {
				eventType = "issue"
			}
		}
	}

	// Add debug logging for event type determination
	ep.logger.Debug("Event type determination",
		"original_object_kind", event.ObjectKind,
		"original_type", event.Type,
		"determined_type", eventType,
		"has_merge_request", event.MergeRequest != nil,
		"has_object_attributes", event.ObjectAttributes != nil,
		"object_attributes_target_branch", func() string {
			if event.ObjectAttributes != nil {
				return event.ObjectAttributes.TargetBranch
			}
			return ""
		}(),
		"object_attributes_source_branch", func() string {
			if event.ObjectAttributes != nil {
				return event.ObjectAttributes.SourceBranch
			}
			return ""
		}())

	switch eventType {
	case "push":
		return ep.processPushEvent(ctx, event)
	case "merge_request":
		return ep.processMergeRequestEvent(ctx, event)
	case "issue":
		return ep.processIssueEvent(ctx, event)
	case "note":
		return ep.processNoteEvent(ctx, event)
	case "pipeline":
		return ep.processPipelineEvent(ctx, event)
	case "job":
		return ep.processJobEvent(ctx, event)
	case "deployment":
		return ep.processDeploymentEvent(ctx, event)
	case "release":
		return ep.processReleaseEvent(ctx, event)
	case "wiki_page":
		return ep.processWikiPageEvent(ctx, event)
	case "feature_flag":
		return ep.processFeatureFlagEvent(ctx, event)
	case "repository_update":
		// Repository update events are usually not relevant for Jira integration
		ep.logger.Info("Skipping repository update event", "object_kind", event.ObjectKind)
		return nil
	default:
		ep.logger.Warn("Unsupported event type",
			"object_kind", event.ObjectKind,
			"event_type", event.Type,
			"determined_type", eventType,
			"has_commits", len(event.Commits) > 0,
			"has_merge_request", event.MergeRequest != nil,
			"has_issue", event.Issue != nil,
			"has_note", event.Note != nil,
			"has_pipeline", event.Pipeline != nil,
			"has_build", event.Build != nil,
			"has_deployment", event.Deployment != nil,
			"has_release", event.Release != nil,
			"has_wiki_page", event.WikiPage != nil,
			"has_feature_flag", event.FeatureFlag != nil,
			"has_object_attributes", event.ObjectAttributes != nil)
		return fmt.Errorf("unsupported event type: %s", eventType)
	}
}

// processPushEvent processes push events
func (ep *EventProcessor) processPushEvent(ctx context.Context, event *Event) error {
	if len(event.Commits) == 0 {
		ep.logger.Info("No commits in push event")
		return nil
	}

	// Process each commit
	for _, commit := range event.Commits {
		// Debug: log ALL event data to understand webhook structure
		ep.logger.Debug("Processing commit - full event debug",
			"commit_id", commit.ID,
			"event_ref", event.Ref,
			"event_before", event.Before,
			"event_after", event.After,
			"event_checkout_sha", event.CheckoutSha,
			"event_project_default_branch", func() string {
				if event.Project != nil {
					return event.Project.DefaultBranch
				}
				return "nil"
			}(),
			"author_name", commit.Author.Name,
			"author_email", commit.Author.Email,
			"commit_message", commit.Message)

		// Extract issue IDs from commit message
		issueIDs := ep.parser.ExtractIssueIDs(commit.Message)
		for _, issueID := range issueIDs {
			// Extract branch name from refs/heads/branch format
			branchName := event.Ref
			if branchName != "" {
				if strings.HasPrefix(event.Ref, "refs/heads/") {
					branchName = strings.TrimPrefix(event.Ref, "refs/heads/")
				} else if strings.HasPrefix(event.Ref, "refs/tags/") {
					branchName = strings.TrimPrefix(event.Ref, "refs/tags/")
				}
			} else {
				// Fallback: try to get default branch from GitLab API
				if event.Project != nil && event.Project.ID != 0 {
					apiDefaultBranch := ep.urlBuilder.GetProjectDefaultBranch(ctx, event.Project.ID)
					if apiDefaultBranch != "" {
						branchName = apiDefaultBranch
						ep.logger.Debug("Using default branch from GitLab API",
							"default_branch", branchName)
					} else if event.Project.DefaultBranch != "" {
						branchName = event.Project.DefaultBranch
						ep.logger.Debug("Using default branch from webhook",
							"default_branch", branchName)
					} else {
						branchName = "main" // Last resort fallback
						ep.logger.Debug("Using 'main' as final fallback for branch name")
					}
				} else {
					branchName = "main" // Last resort fallback
					ep.logger.Debug("Using 'main' as final fallback for branch name")
				}
			}

			// Get username from GitLab API
			username := ep.urlBuilder.GetUsernameByEmail(ctx, commit.Author.Email)

			// Build proper URLs using existing urlBuilder
			authorURL := ep.urlBuilder.ConstructAuthorURLFromEmail(ctx, commit.Author.Email)
			// Construct branch URL using the determined branch name
			branchURL := ""
			if event.Project != nil && event.Project.WebURL != "" && branchName != "" {
				branchURL = fmt.Sprintf("%s/-/tree/%s", event.Project.WebURL, branchName)
			}

			// Debug: log URL construction results
			ep.logger.Debug("URL construction results",
				"issue_id", issueID,
				"username", username,
				"author_url", authorURL,
				"branch_url", branchURL,
				"event_ref", event.Ref,
				"extracted_branch_name", branchName)

			// Use username if available, otherwise fallback to author name
			displayName := username
			if displayName == "" {
				displayName = commit.Author.Name
			}

			// Use existing jira.GenerateCommitADFComment function
			comment := jira.GenerateCommitADFComment(
				commit.ID,
				commit.URL,
				displayName, // Use username instead of full name
				commit.Author.Email,
				authorURL,
				commit.Message,
				commit.Timestamp,
				branchName, // Use extracted branch name instead of event.Ref
				branchURL,
				event.Project.WebURL,
				ep.config.Timezone, // Use configured timezone
				commit.Added,
				commit.Modified,
				commit.Removed,
			)

			if err := ep.jiraClient.AddComment(ctx, issueID, comment); err != nil {
				ep.logger.Error("Failed to add push comment to Jira",
					"error", err,
					"commit_id", commit.ID,
					"project_id", event.Project.ID,
				)
				return fmt.Errorf("failed to add push comment: %w", err)
			}
		}
	}

	ep.logger.Info("Successfully processed push event",
		"commits_count", len(event.Commits),
		"project_id", event.Project.ID,
	)
	return nil
}

// processMergeRequestEvent processes merge request events
func (ep *EventProcessor) processMergeRequestEvent(ctx context.Context, event *Event) error {
	var title, description, action, sourceBranch, targetBranch, mrURL string
	var mrID int
	var authorName, authorEmail string

	// Debug: Log full event structure for troubleshooting
	ep.logger.Debug("MR Event Debug - Full event structure",
		"event_user", func() string {
			if event.User != nil {
				return fmt.Sprintf("Name:%s, Email:%s, Username:%s", event.User.Name, event.User.Email, event.User.Username)
			}
			return "nil"
		}(),
		"event_merge_request", func() string {
			if event.MergeRequest != nil {
				return fmt.Sprintf("ID:%d, Title:%s, SourceBranch:%s, TargetBranch:%s",
					event.MergeRequest.ID, event.MergeRequest.Title, event.MergeRequest.SourceBranch, event.MergeRequest.TargetBranch)
			}
			return "nil"
		}(),
		"event_object_attributes", func() string {
			if event.ObjectAttributes != nil {
				return fmt.Sprintf("ID:%d, Title:%s, Action:%s, SourceBranch:%s, TargetBranch:%s",
					event.ObjectAttributes.ID, event.ObjectAttributes.Title, event.ObjectAttributes.Action,
					event.ObjectAttributes.SourceBranch, event.ObjectAttributes.TargetBranch)
			}
			return "nil"
		}())

	// Try to get data from ObjectAttributes first
	if event.ObjectAttributes != nil {
		title = event.ObjectAttributes.Title
		description = event.ObjectAttributes.Description
		action = event.ObjectAttributes.Action
		mrID = event.ObjectAttributes.ID
		sourceBranch = event.ObjectAttributes.SourceBranch
		targetBranch = event.ObjectAttributes.TargetBranch
		mrURL = event.ObjectAttributes.URL
	} else if event.MergeRequest != nil {
		// Fallback to MergeRequest structure
		title = event.MergeRequest.Title
		description = event.MergeRequest.Description
		action = "updated" // Default action for merge request
		mrID = event.MergeRequest.ID
		sourceBranch = event.MergeRequest.SourceBranch
		targetBranch = event.MergeRequest.TargetBranch
		mrURL = event.MergeRequest.WebURL
		if event.MergeRequest.Author != nil {
			authorName = event.MergeRequest.Author.Name
			authorEmail = event.MergeRequest.Author.Email
		}
	} else {
		return fmt.Errorf("missing merge request data in event")
	}

	// If branches are empty, try to get them from GitLab API (known GitLab webhook issue)
	if (sourceBranch == "" || targetBranch == "") && event.Project != nil && mrID > 0 {
		ep.logger.Debug("Source/target branches empty in webhook, fetching from GitLab API",
			"source_branch_from_webhook", sourceBranch,
			"target_branch_from_webhook", targetBranch,
			"mr_id", mrID,
			"project_id", event.Project.ID)

		// Convert MR ID to IID by making API call (GitLab webhook sends ID, API expects IID)
		// For now, assume they are the same - this is usually the case
		mrIID := mrID
		apiSourceBranch, apiTargetBranch := ep.urlBuilder.GetMergeRequestInfo(ctx, event.Project.ID, mrIID)

		if apiSourceBranch != "" && apiTargetBranch != "" {
			sourceBranch = apiSourceBranch
			targetBranch = apiTargetBranch
			ep.logger.Debug("Successfully retrieved branches from GitLab API",
				"source_branch", sourceBranch,
				"target_branch", targetBranch)
		} else {
			ep.logger.Warn("Failed to retrieve branches from GitLab API",
				"mr_id", mrID,
				"project_id", event.Project.ID)
		}
	}

	// Determine who should be displayed as the "actor" for this MR event
	var actorName, actorEmail string

	// For approve/unapprove actions, show who performed the action (event.User)
	// For other actions (open, update, merge), show the MR author
	if action == "approved" || action == "unapproved" || action == "approve" || action == "unapprove" {
		// Show who performed the approval action
		if event.User != nil {
			actorName = event.User.Name
			actorEmail = event.User.Email
		}
		ep.logger.Debug("Using action performer as actor",
			"action", action,
			"actor_name", actorName,
			"actor_email", actorEmail)
	} else {
		// Show MR author for other actions
		actorName = authorName
		actorEmail = authorEmail
		ep.logger.Debug("Using MR author as actor",
			"action", action,
			"actor_name", actorName,
			"actor_email", actorEmail)
	}

	// Get username from GitLab API for the actor
	username := ""
	authorURL := ""
	if actorEmail != "" {
		username = ep.urlBuilder.GetUsernameByEmail(ctx, actorEmail)
		authorURL = ep.urlBuilder.ConstructAuthorURLFromEmail(ctx, actorEmail)
		ep.logger.Debug("GitLab API lookup results",
			"actor_email", actorEmail,
			"username", username,
			"author_url", authorURL)
	}

	// Fallback: if no email or API failed, try to use username directly from event
	if username == "" && event.User != nil && event.User.Username != "" {
		username = event.User.Username
		if ep.config.GitLabBaseURL != "" {
			authorURL = fmt.Sprintf("%s/%s", strings.TrimSuffix(ep.config.GitLabBaseURL, "/"), username)
		}
		ep.logger.Debug("Using username from event as fallback",
			"username", username,
			"author_url", authorURL)
	}

	// Additional fallback: if still no username, check MR author
	if username == "" && event.MergeRequest != nil && event.MergeRequest.Author != nil {
		if event.MergeRequest.Author.Username != "" {
			username = event.MergeRequest.Author.Username
			if ep.config.GitLabBaseURL != "" {
				authorURL = fmt.Sprintf("%s/%s", strings.TrimSuffix(ep.config.GitLabBaseURL, "/"), username)
			}
			ep.logger.Debug("Using MR author username as fallback",
				"username", username,
				"author_url", authorURL)
		}
	}

	// Use username if available, otherwise fallback to actor name
	displayName := username
	if displayName == "" {
		displayName = actorName
	}

	ep.logger.Debug("Final display name determination",
		"username", username,
		"actor_name", actorName,
		"display_name", displayName)

	// Extract issue IDs from MR title and description
	issueIDs := ep.parser.ExtractIssueIDs(title)
	issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(description)...)

	if len(issueIDs) == 0 {
		ep.logger.Info("No Jira issue IDs found in merge request",
			"mr_id", mrID,
			"title", title,
			"project_id", event.Project.ID,
		)
		return nil
	}

	// Project web URL is not required for MR simple ADF comment

	// Create beautiful ADF comment for MR with clickable links and approver info
	comment := jira.GenerateMergeRequestADFCommentSimple(
		mrID,               // mrID
		mrURL,              // mrURL
		title,              // title
		description,        // description
		displayName,        // authorName
		authorURL,          // authorURL (clickable link)
		action,             // action
		sourceBranch,       // sourceBranch
		targetBranch,       // targetBranch
		ep.config.Timezone, // timezone
	)

	ep.logger.Debug("MR comment construction results",
		"mr_id", mrID,
		"username", username,
		"author_url", authorURL,
		"source_branch", sourceBranch,
		"target_branch", targetBranch,
		"action", action,
		"mr_url", mrURL)

	for _, issueID := range issueIDs {
		if err := ep.jiraClient.AddComment(ctx, issueID, comment); err != nil {
			ep.logger.Error("Failed to add merge request comment to Jira",
				"error", err,
				"mr_id", mrID,
				"issue_id", issueID,
				"project_id", event.Project.ID,
			)
			return fmt.Errorf("failed to add merge request comment: %w", err)
		}
	}

	ep.logger.Info("Successfully processed merge request event",
		"mr_id", mrID,
		"action", action,
		"issue_ids", issueIDs,
		"project_id", event.Project.ID,
	)
	return nil
}

// processIssueEvent processes issue events
func (ep *EventProcessor) processIssueEvent(ctx context.Context, event *Event) error {
	var title, description, action string
	var issueID int

	// Try to get data from ObjectAttributes first
	if event.ObjectAttributes != nil {
		title = event.ObjectAttributes.Title
		description = event.ObjectAttributes.Description
		action = event.ObjectAttributes.Action
		issueID = event.ObjectAttributes.ID
	} else if event.Issue != nil {
		// Fallback to Issue structure
		title = event.Issue.Title
		description = event.Issue.Description
		action = "updated" // Default action for issue
		issueID = event.Issue.ID
	} else {
		return fmt.Errorf("missing issue data in event")
	}

	// Extract issue IDs from issue title and description
	jiraIssueIDs := ep.parser.ExtractIssueIDs(title)
	jiraIssueIDs = append(jiraIssueIDs, ep.parser.ExtractIssueIDs(description)...)

	if len(jiraIssueIDs) == 0 {
		ep.logger.Info("No Jira issue IDs found in GitLab issue",
			"issue_id", issueID,
			"title", title,
			"project_id", event.Project.ID,
		)
		return nil
	}

	// Create a simple comment
	comment := ep.buildSimpleComment("Issue", title, action)

	for _, jiraID := range jiraIssueIDs {
		if err := ep.jiraClient.AddComment(ctx, jiraID, comment); err != nil {
			ep.logger.Error("Failed to add issue comment to Jira",
				"error", err,
				"gitlab_issue_id", issueID,
				"jira_issue_id", jiraID,
				"project_id", event.Project.ID,
			)
			return fmt.Errorf("failed to add issue comment: %w", err)
		}
	}

	ep.logger.Info("Successfully processed issue event",
		"issue_id", issueID,
		"action", action,
		"jira_issue_ids", jiraIssueIDs,
		"project_id", event.Project.ID,
	)
	return nil
}

// processNoteEvent processes note (comment) events
func (ep *EventProcessor) processNoteEvent(ctx context.Context, event *Event) error {
	var noteContent, noteURL, noteableType string
	var noteID int
	var authorName, authorEmail string

	// Try to get data from ObjectAttributes first
	if event.ObjectAttributes != nil {
		noteContent = event.ObjectAttributes.Note
		noteID = event.ObjectAttributes.ID
		noteURL = event.ObjectAttributes.URL
		noteableType = event.ObjectAttributes.NoteableType
	} else if event.Note != nil {
		// Fallback to Note structure
		noteContent = event.Note.Note
		noteID = event.Note.ID
		noteURL = event.Note.URL
		noteableType = event.Note.Noteable
	} else {
		return fmt.Errorf("missing note data in event")
	}

	// Get author information
	if event.User != nil {
		authorName = event.User.Name
		authorEmail = event.User.Email
	}

	// Get username from GitLab API
	username := ""
	authorURL := ""
	if authorEmail != "" {
		username = ep.urlBuilder.GetUsernameByEmail(ctx, authorEmail)
		authorURL = ep.urlBuilder.ConstructAuthorURLFromEmail(ctx, authorEmail)
	}

	// Use username if available, otherwise fallback to author name
	displayName := username
	if displayName == "" {
		displayName = authorName
	}

	// Extract issue IDs from note content
	issueIDs := ep.parser.ExtractIssueIDs(noteContent)

	// For MR comments, also check MR title and description for issue IDs
	if noteableType == "MergeRequest" && event.MergeRequest != nil {
		mrIssueIDs := ep.parser.ExtractIssueIDs(event.MergeRequest.Title)
		mrIssueIDs = append(mrIssueIDs, ep.parser.ExtractIssueIDs(event.MergeRequest.Description)...)

		// Add MR issue IDs to the list (avoid duplicates)
		for _, mrID := range mrIssueIDs {
			found := false
			for _, existingID := range issueIDs {
				if existingID == mrID {
					found = true
					break
				}
			}
			if !found {
				issueIDs = append(issueIDs, mrID)
			}
		}

		ep.logger.Debug("Found issue IDs for MR comment",
			"note_content_issues", ep.parser.ExtractIssueIDs(noteContent),
			"mr_title_issues", ep.parser.ExtractIssueIDs(event.MergeRequest.Title),
			"mr_desc_issues", ep.parser.ExtractIssueIDs(event.MergeRequest.Description),
			"total_issues", issueIDs)
	}

	if len(issueIDs) == 0 {
		ep.logger.Info("No Jira issue IDs found in note or related content",
			"note_id", noteID,
			"noteable_type", noteableType,
			"project_id", event.Project.ID,
		)
		return nil
	}
	// Get project information
	projectName := "Unknown Project"
	projectURL := ""
	if event.Project != nil {
		projectName = event.Project.Name
		projectURL = event.Project.WebURL
	}

	// Determine comment title based on noteable type
	commentTitle := "Comment"
	if noteableType == "MergeRequest" {
		commentTitle = "MR Comment"
		if event.MergeRequest != nil {
			commentTitle = fmt.Sprintf("MR Comment: %s", event.MergeRequest.Title)
		}
	} else if noteableType == "Issue" {
		commentTitle = "Issue Comment"
		if event.Issue != nil {
			commentTitle = fmt.Sprintf("Issue Comment: %s", event.Issue.Title)
		}
	}

	// Create beautiful ADF comment for note
	comment := jira.GenerateNoteADFComment(
		commentTitle,
		noteURL,
		projectName,
		projectURL,
		"added", // action is always "added" for new comments
		displayName,
		noteContent,
		ep.config.Timezone,
	)

	ep.logger.Debug("Note comment construction results",
		"note_id", noteID,
		"noteable_type", noteableType,
		"username", username,
		"author_url", authorURL,
		"note_url", noteURL,
		"comment_title", commentTitle)

	for _, issueID := range issueIDs {
		if err := ep.jiraClient.AddComment(ctx, issueID, comment); err != nil {
			ep.logger.Error("Failed to add note comment to Jira",
				"error", err,
				"note_id", noteID,
				"issue_id", issueID,
				"project_id", event.Project.ID,
			)
			return fmt.Errorf("failed to add note comment: %w", err)
		}
	}

	ep.logger.Info("Successfully processed note event",
		"note_id", noteID,
		"issue_ids", issueIDs,
		"project_id", event.Project.ID,
	)
	return nil
}

// processPipelineEvent processes pipeline events
func (ep *EventProcessor) processPipelineEvent(ctx context.Context, event *Event) error {
	if event.ObjectAttributes == nil {
		return fmt.Errorf("missing object attributes in pipeline event")
	}

	// Extract issue IDs from pipeline ref (branch name)
	issueIDs := ep.parser.ExtractIssueIDs(event.ObjectAttributes.Ref)

	// Create a simple comment
	comment := ep.buildSimpleComment("Pipeline", event.ObjectAttributes.Status, event.ObjectAttributes.Ref)

	for _, issueID := range issueIDs {
		if err := ep.jiraClient.AddComment(ctx, issueID, comment); err != nil {
			ep.logger.Error("Failed to add pipeline comment to Jira",
				"error", err,
				"pipeline_id", event.ObjectAttributes.ID,
				"project_id", event.Project.ID,
			)
			return fmt.Errorf("failed to add pipeline comment: %w", err)
		}
	}

	ep.logger.Info("Successfully processed pipeline event",
		"pipeline_id", event.ObjectAttributes.ID,
		"status", event.ObjectAttributes.Status,
		"project_id", event.Project.ID,
	)
	return nil
}

// processJobEvent processes job events
func (ep *EventProcessor) processJobEvent(ctx context.Context, event *Event) error {
	if event.Build == nil {
		return fmt.Errorf("missing build information in job event")
	}

	// Extract issue IDs from job name
	issueIDs := ep.parser.ExtractIssueIDs(event.Build.Name)

	// Create a simple comment
	comment := ep.buildSimpleComment("Job", event.Build.Name, event.Build.Status)

	for _, issueID := range issueIDs {
		if err := ep.jiraClient.AddComment(ctx, issueID, comment); err != nil {
			ep.logger.Error("Failed to add job comment to Jira",
				"error", err,
				"job_id", event.Build.ID,
				"project_id", event.Project.ID,
			)
			return fmt.Errorf("failed to add job comment: %w", err)
		}
	}

	ep.logger.Info("Successfully processed job event",
		"job_id", event.Build.ID,
		"status", event.Build.Status,
		"project_id", event.Project.ID,
	)
	return nil
}

// processDeploymentEvent processes deployment events
func (ep *EventProcessor) processDeploymentEvent(ctx context.Context, event *Event) error {
	if event.ObjectAttributes == nil {
		return fmt.Errorf("missing object attributes in deployment event")
	}

	// Extract issue IDs from deployment environment
	issueIDs := ep.parser.ExtractIssueIDs(event.ObjectAttributes.Environment)

	// Create a simple comment
	comment := ep.buildSimpleComment("Deployment", event.ObjectAttributes.Environment, event.ObjectAttributes.Status)

	for _, issueID := range issueIDs {
		if err := ep.jiraClient.AddComment(ctx, issueID, comment); err != nil {
			ep.logger.Error("Failed to add deployment comment to Jira",
				"error", err,
				"deployment_id", event.ObjectAttributes.ID,
				"project_id", event.Project.ID,
			)
			return fmt.Errorf("failed to add deployment comment: %w", err)
		}
	}

	ep.logger.Info("Successfully processed deployment event",
		"deployment_id", event.ObjectAttributes.ID,
		"status", event.ObjectAttributes.Status,
		"project_id", event.Project.ID,
	)
	return nil
}

// processReleaseEvent processes release events
func (ep *EventProcessor) processReleaseEvent(ctx context.Context, event *Event) error {
	if event.Release == nil {
		return fmt.Errorf("missing release information in release event")
	}

	// Extract issue IDs from release name and description
	issueIDs := ep.parser.ExtractIssueIDs(event.Release.Name)
	issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.Release.Description)...)

	// Create a simple comment
	comment := ep.buildSimpleComment("Release", event.Release.Name, event.Release.TagName)

	for _, issueID := range issueIDs {
		if err := ep.jiraClient.AddComment(ctx, issueID, comment); err != nil {
			ep.logger.Error("Failed to add release comment to Jira",
				"error", err,
				"release_id", event.Release.ID,
				"project_id", event.Project.ID,
			)
			return fmt.Errorf("failed to add release comment: %w", err)
		}
	}

	ep.logger.Info("Successfully processed release event",
		"release_id", event.Release.ID,
		"tag_name", event.Release.TagName,
		"project_id", event.Project.ID,
	)
	return nil
}

// processWikiPageEvent processes wiki page events
func (ep *EventProcessor) processWikiPageEvent(ctx context.Context, event *Event) error {
	if event.WikiPage == nil {
		return fmt.Errorf("missing wiki page information in wiki page event")
	}

	// Extract issue IDs from wiki page title and content
	issueIDs := ep.parser.ExtractIssueIDs(event.WikiPage.Title)
	issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.WikiPage.Content)...)

	// Create a simple comment
	comment := ep.buildSimpleComment("Wiki Page", event.WikiPage.Title, event.WikiPage.Action)

	for _, issueID := range issueIDs {
		if err := ep.jiraClient.AddComment(ctx, issueID, comment); err != nil {
			ep.logger.Error("Failed to add wiki page comment to Jira",
				"error", err,
				"wiki_title", event.WikiPage.Title,
				"project_id", event.Project.ID,
			)
			return fmt.Errorf("failed to add wiki page comment: %w", err)
		}
	}

	ep.logger.Info("Successfully processed wiki page event",
		"wiki_title", event.WikiPage.Title,
		"action", event.WikiPage.Action,
		"project_id", event.Project.ID,
	)
	return nil
}

// processFeatureFlagEvent processes feature flag events
func (ep *EventProcessor) processFeatureFlagEvent(ctx context.Context, event *Event) error {
	if event.FeatureFlag == nil {
		return fmt.Errorf("missing feature flag information in feature flag event")
	}

	// Extract issue IDs from feature flag name and description
	issueIDs := ep.parser.ExtractIssueIDs(event.FeatureFlag.Name)
	issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.FeatureFlag.Description)...)

	// Create a simple comment
	status := "inactive"
	if event.FeatureFlag.Active {
		status = "active"
	}
	comment := ep.buildSimpleComment("Feature Flag", event.FeatureFlag.Name, status)

	for _, issueID := range issueIDs {
		if err := ep.jiraClient.AddComment(ctx, issueID, comment); err != nil {
			ep.logger.Error("Failed to add feature flag comment to Jira",
				"error", err,
				"flag_name", event.FeatureFlag.Name,
				"project_id", event.Project.ID,
			)
			return fmt.Errorf("failed to add feature flag comment: %w", err)
		}
	}

	ep.logger.Info("Successfully processed feature flag event",
		"flag_name", event.FeatureFlag.Name,
		"active", event.FeatureFlag.Active,
		"project_id", event.Project.ID,
	)
	return nil
}

// buildSimpleComment builds a simple comment for events
func (ep *EventProcessor) buildSimpleComment(eventType, title, action string) jira.CommentPayload {
	return jira.CommentPayload{
		Body: jira.CommentBody{
			Type:    "doc",
			Version: 1,
			Content: []jira.Content{
				{
					Type: "paragraph",
					Content: []jira.TextContent{
						{
							Type: "text",
							Text: fmt.Sprintf("%s %s: %s", eventType, action, title),
						},
					},
				},
			},
		},
	}
}
