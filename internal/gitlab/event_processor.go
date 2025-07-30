package gitlab

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/atlet99/gitlab-jira-hook/internal/jira"
)

// EventProcessor handles GitLab webhook event processing
type EventProcessor struct {
	jiraClient interface {
		AddComment(issueID string, payload jira.CommentPayload) error
		TestConnection() error
	}
	urlBuilder *URLBuilder
	logger     *slog.Logger
	parser     *Parser
}

// NewEventProcessor creates a new event processor
func NewEventProcessor(jiraClient interface {
	AddComment(issueID string, payload jira.CommentPayload) error
	TestConnection() error
}, urlBuilder *URLBuilder, logger *slog.Logger) *EventProcessor {
	return &EventProcessor{
		jiraClient: jiraClient,
		urlBuilder: urlBuilder,
		logger:     logger,
		parser:     NewParser(),
	}
}

// ProcessEvent processes a GitLab webhook event
func (ep *EventProcessor) ProcessEvent(ctx context.Context, event *Event) error {
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
		// Extract issue IDs from commit message
		issueIDs := ep.parser.ExtractIssueIDs(commit.Message)
		for _, issueID := range issueIDs {
			// Use existing jira.GenerateCommitADFComment function
			comment := jira.GenerateCommitADFComment(
				commit.ID,
				commit.URL,
				commit.Author.Name,
				commit.Author.Email,
				"", // authorURL - will be empty for now
				commit.Message,
				commit.Timestamp,
				event.Ref,
				"", // branchURL - will be empty for now
				event.Project.WebURL,
				"Etc/GMT-5", // Default timezone
				commit.Added,
				commit.Modified,
				commit.Removed,
			)

			if err := ep.jiraClient.AddComment(issueID, comment); err != nil {
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
	var title, description, action string
	var mrID int

	// Try to get data from ObjectAttributes first
	if event.ObjectAttributes != nil {
		title = event.ObjectAttributes.Title
		description = event.ObjectAttributes.Description
		action = event.ObjectAttributes.Action
		mrID = event.ObjectAttributes.ID
	} else if event.MergeRequest != nil {
		// Fallback to MergeRequest structure
		title = event.MergeRequest.Title
		description = event.MergeRequest.Description
		action = "updated" // Default action for merge request
		mrID = event.MergeRequest.ID
	} else {
		return fmt.Errorf("missing merge request data in event")
	}

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

	// Create a simple comment
	comment := ep.buildSimpleComment("Merge Request", title, action)

	for _, issueID := range issueIDs {
		if err := ep.jiraClient.AddComment(issueID, comment); err != nil {
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
		if err := ep.jiraClient.AddComment(jiraID, comment); err != nil {
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
	var noteContent string
	var noteID int

	// Try to get data from ObjectAttributes first
	if event.ObjectAttributes != nil {
		noteContent = event.ObjectAttributes.Note
		noteID = event.ObjectAttributes.ID
	} else if event.Note != nil {
		// Fallback to Note structure
		noteContent = event.Note.Note
		noteID = event.Note.ID
	} else {
		return fmt.Errorf("missing note data in event")
	}

	// Extract issue IDs from note content
	issueIDs := ep.parser.ExtractIssueIDs(noteContent)

	if len(issueIDs) == 0 {
		ep.logger.Info("No Jira issue IDs found in note",
			"note_id", noteID,
			"project_id", event.Project.ID,
		)
		return nil
	}

	// Create a simple comment
	comment := ep.buildSimpleComment("Comment", noteContent, "added")

	for _, issueID := range issueIDs {
		if err := ep.jiraClient.AddComment(issueID, comment); err != nil {
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
		if err := ep.jiraClient.AddComment(issueID, comment); err != nil {
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
		if err := ep.jiraClient.AddComment(issueID, comment); err != nil {
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
		if err := ep.jiraClient.AddComment(issueID, comment); err != nil {
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
		if err := ep.jiraClient.AddComment(issueID, comment); err != nil {
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
		if err := ep.jiraClient.AddComment(issueID, comment); err != nil {
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
		if err := ep.jiraClient.AddComment(issueID, comment); err != nil {
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
