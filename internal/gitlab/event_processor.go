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
		"project_id", event.Project.ID,
		"project_name", event.Project.Name,
	)

	switch event.ObjectKind {
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
	default:
		ep.logger.Warn("Unsupported event type", "object_kind", event.ObjectKind)
		return fmt.Errorf("unsupported event type: %s", event.ObjectKind)
	}
}

// processPushEvent processes push events
func (ep *EventProcessor) processPushEvent(ctx context.Context, event *Event) error {
	if event.Commits == nil || len(event.Commits) == 0 {
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
	if event.ObjectAttributes == nil {
		return fmt.Errorf("missing object attributes in merge request event")
	}

	// Extract issue IDs from MR title and description
	issueIDs := ep.parser.ExtractIssueIDs(event.ObjectAttributes.Title)
	issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.ObjectAttributes.Description)...)

	// Create a simple comment
	comment := ep.buildSimpleComment("Merge Request", event.ObjectAttributes.Title, event.ObjectAttributes.Action)

	for _, issueID := range issueIDs {
		if err := ep.jiraClient.AddComment(issueID, comment); err != nil {
			ep.logger.Error("Failed to add merge request comment to Jira",
				"error", err,
				"mr_id", event.ObjectAttributes.ID,
				"project_id", event.Project.ID,
			)
			return fmt.Errorf("failed to add merge request comment: %w", err)
		}
	}

	ep.logger.Info("Successfully processed merge request event",
		"mr_id", event.ObjectAttributes.ID,
		"action", event.ObjectAttributes.Action,
		"project_id", event.Project.ID,
	)
	return nil
}

// processIssueEvent processes issue events
func (ep *EventProcessor) processIssueEvent(ctx context.Context, event *Event) error {
	if event.ObjectAttributes == nil {
		return fmt.Errorf("missing object attributes in issue event")
	}

	// Extract issue IDs from issue title and description
	issueIDs := ep.parser.ExtractIssueIDs(event.ObjectAttributes.Title)
	issueIDs = append(issueIDs, ep.parser.ExtractIssueIDs(event.ObjectAttributes.Description)...)

	// Create a simple comment
	comment := ep.buildSimpleComment("Issue", event.ObjectAttributes.Title, event.ObjectAttributes.Action)

	for _, issueID := range issueIDs {
		if err := ep.jiraClient.AddComment(issueID, comment); err != nil {
			ep.logger.Error("Failed to add issue comment to Jira",
				"error", err,
				"issue_id", event.ObjectAttributes.ID,
				"project_id", event.Project.ID,
			)
			return fmt.Errorf("failed to add issue comment: %w", err)
		}
	}

	ep.logger.Info("Successfully processed issue event",
		"issue_id", event.ObjectAttributes.ID,
		"action", event.ObjectAttributes.Action,
		"project_id", event.Project.ID,
	)
	return nil
}

// processNoteEvent processes note (comment) events
func (ep *EventProcessor) processNoteEvent(ctx context.Context, event *Event) error {
	if event.ObjectAttributes == nil {
		return fmt.Errorf("missing object attributes in note event")
	}

	// Extract issue IDs from note content
	issueIDs := ep.parser.ExtractIssueIDs(event.ObjectAttributes.Note)

	// Create a simple comment
	comment := ep.buildSimpleComment("Comment", event.ObjectAttributes.Note, "added")

	for _, issueID := range issueIDs {
		if err := ep.jiraClient.AddComment(issueID, comment); err != nil {
			ep.logger.Error("Failed to add note comment to Jira",
				"error", err,
				"note_id", event.ObjectAttributes.ID,
				"project_id", event.Project.ID,
			)
			return fmt.Errorf("failed to add note comment: %w", err)
		}
	}

	ep.logger.Info("Successfully processed note event",
		"note_id", event.ObjectAttributes.ID,
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
