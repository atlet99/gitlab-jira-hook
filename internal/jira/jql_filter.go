// Package jira provides Jira webhook handling and GitLab synchronization.
package jira

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

// JQLFilter handles JQL-based event filtering
type JQLFilter struct {
	config     *config.Config
	logger     *slog.Logger
	jiraClient *Client
}

// NewJQLFilter creates a new JQL filter instance
func NewJQLFilter(cfg *config.Config, logger *slog.Logger, jiraClient *Client) *JQLFilter {
	return &JQLFilter{
		config:     cfg,
		logger:     logger,
		jiraClient: jiraClient,
	}
}

// ShouldProcessEvent determines if a webhook event should be processed based on JQL filter
func (jf *JQLFilter) ShouldProcessEvent(event *WebhookEvent) bool {
	// If config is nil, process all events (fail-safe)
	if jf.config == nil {
		return true
	}

	// If no JQL filter is configured, process all events
	if jf.config.JQLFilter == "" {
		return true
	}

	// Skip processing if the event is too old (if configured)
	if jf.shouldSkipOldEvent(event) {
		jf.logger.Debug("Skipping old event based on MaxEventAge configuration",
			"event_type", event.WebhookEvent,
			"event_timestamp", event.Timestamp,
			"max_age_hours", jf.config.MaxEventAge)
		return false
	}

	// For issue-related events, validate against JQL filter
	if jf.isIssueEvent(event) {
		return jf.validateIssueEvent(event)
	}

	// For non-issue events, process them if they're in the allowed events list
	return jf.isAllowedEvent(event.WebhookEvent)
}

// shouldSkipOldEvent checks if the event is too old based on configuration
func (jf *JQLFilter) shouldSkipOldEvent(event *WebhookEvent) bool {
	if !jf.config.SkipOldEvents || jf.config.MaxEventAge <= 0 {
		return false
	}

	// Parse event timestamp (convert from Unix timestamp to time.Time)
	eventTime := time.Unix(event.Timestamp, 0)

	// Calculate maximum allowed age
	maxAge := time.Duration(jf.config.MaxEventAge) * time.Hour
	age := time.Since(eventTime)

	return age > maxAge
}

// isIssueEvent checks if the event is related to issues
func (jf *JQLFilter) isIssueEvent(event *WebhookEvent) bool {
	return event.Issue != nil && event.Issue.Key != ""
}

// validateIssueEvent validates an issue event against the JQL filter
func (jf *JQLFilter) validateIssueEvent(event *WebhookEvent) bool {
	// Build JQL query parameters from the event
	jqlParams := jf.buildJQLParameters(event)

	// Execute JQL query to check if the issue matches
	matches, err := jf.executeJQLCheck(event.Issue.Key, jqlParams)
	if err != nil {
		jf.logger.Error("Failed to validate event against JQL filter",
			"event_type", event.WebhookEvent,
			"issue_key", event.Issue.Key,
			"error", err)
		// In case of error, allow the event to be processed (fail-safe)
		return true
	}

	if matches {
		jf.logger.Debug("Event matches JQL filter, processing",
			"event_type", event.WebhookEvent,
			"issue_key", event.Issue.Key)
	} else {
		jf.logger.Debug("Event does not match JQL filter, skipping",
			"event_type", event.WebhookEvent,
			"issue_key", event.Issue.Key)
	}

	return matches
}

// buildJQLParameters builds JQL query parameters from the event data
func (jf *JQLFilter) buildJQLParameters(event *WebhookEvent) map[string]interface{} {
	params := make(map[string]interface{})

	// Basic issue information
	jf.addBasicInfo(params, event)

	// Issue fields
	jf.addIssueFields(params, event)

	// User information
	jf.addUserInfo(params, event)

	// Collections (labels, versions, components)
	jf.addCollections(params, event)

	// Dates
	jf.addDates(params, event)

	// Event-specific information
	params["event_type"] = event.WebhookEvent
	params["timestamp"] = event.Timestamp

	return params
}

// addBasicInfo adds basic issue information to parameters
func (jf *JQLFilter) addBasicInfo(params map[string]interface{}, event *WebhookEvent) {
	params["issue_key"] = event.Issue.Key
	params["project"] = event.Issue.Fields.Project.Key
}

// addIssueFields adds issue fields to parameters
func (jf *JQLFilter) addIssueFields(params map[string]interface{}, event *WebhookEvent) {
	if event.Issue.Fields.Summary != "" {
		params["summary"] = event.Issue.Fields.Summary
	}

	if event.Issue.Fields.Status != nil {
		params["status"] = event.Issue.Fields.Status.Name
	}

	if event.Issue.Fields.Priority != nil {
		params["priority"] = event.Issue.Fields.Priority.Name
	}

	if event.Issue.Fields.IssueType != nil {
		params["issue_type"] = event.Issue.Fields.IssueType.Name
	}
}

// addUserInfo adds user information to parameters
func (jf *JQLFilter) addUserInfo(params map[string]interface{}, event *WebhookEvent) {
	// Assignee information
	if event.Issue.Fields.Assignee != nil {
		params["assignee"] = event.Issue.Fields.Assignee.AccountID
		if event.Issue.Fields.Assignee.EmailAddress != "" {
			params["assignee_email"] = event.Issue.Fields.Assignee.EmailAddress
		}
	} else {
		params["assignee"] = "null" // Unassigned
	}

	// Reporter information
	if event.Issue.Fields.Reporter != nil {
		params["reporter"] = event.Issue.Fields.Reporter.AccountID
		if event.Issue.Fields.Reporter.EmailAddress != "" {
			params["reporter_email"] = event.Issue.Fields.Reporter.EmailAddress
		}
	}
}

// addCollections adds collection fields to parameters
func (jf *JQLFilter) addCollections(params map[string]interface{}, event *WebhookEvent) {
	// Labels
	if len(event.Issue.Fields.Labels) > 0 {
		params["labels"] = event.Issue.Fields.Labels
	}

	// Fix versions
	if len(event.Issue.Fields.FixVersions) > 0 {
		versionNames := jf.extractVersionNames(event.Issue.Fields.FixVersions)
		params["fix_versions"] = versionNames
	}

	// Components
	if len(event.Issue.Fields.Components) > 0 {
		componentNames := jf.extractComponentNames(event.Issue.Fields.Components)
		params["components"] = componentNames
	}
}

// addDates adds date fields to parameters
func (jf *JQLFilter) addDates(params map[string]interface{}, event *WebhookEvent) {
	// Created date
	if event.Issue.Fields.Created != "" {
		if createdTime, err := time.Parse(time.RFC3339, event.Issue.Fields.Created); err == nil {
			params["created"] = createdTime
		}
	}

	// Updated date
	if event.Issue.Fields.Updated != "" {
		if updatedTime, err := time.Parse(time.RFC3339, event.Issue.Fields.Updated); err == nil {
			params["updated"] = updatedTime
		}
	}
}

// extractVersionNames extracts version names from fix versions
func (jf *JQLFilter) extractVersionNames(fixVersions []Version) []string {
	versionNames := make([]string, len(fixVersions))
	for i, version := range fixVersions {
		versionNames[i] = version.Name
	}
	return versionNames
}

// extractComponentNames extracts component names from components
func (jf *JQLFilter) extractComponentNames(components []Component) []string {
	componentNames := make([]string, len(components))
	for i, component := range components {
		componentNames[i] = component.Name
	}
	return componentNames
}

// executeJQLCheck executes a JQL query to check if the issue matches the filter
func (jf *JQLFilter) executeJQLCheck(issueKey string, params map[string]interface{}) (bool, error) {
	if jf.jiraClient == nil {
		return false, fmt.Errorf("jira client not available")
	}

	// Parse the JQL filter and replace placeholders with actual values
	jqlQuery := jf.parseJQLWithParams(jf.config.JQLFilter, params)

	// Debug logging
	jf.logger.Debug("Executing JQL check",
		"original_jql", jf.config.JQLFilter,
		"params", params,
		"final_jql", jqlQuery,
		"issue_key", issueKey)

	// Execute the JQL query
	matchingIssues, err := jf.jiraClient.SearchIssues(context.Background(), jqlQuery)
	if err != nil {
		return false, fmt.Errorf("failed to execute JQL query: %w", err)
	}

	// Debug logging
	jf.logger.Debug("JQL query results",
		"matching_issues_count", len(matchingIssues),
		"matching_issues", matchingIssues)

	// Check if the issue key is in the search results
	for _, issue := range matchingIssues {
		if issue.Key == issueKey {
			return true, nil
		}
	}

	return false, nil
}

// parseJQLWithParams replaces placeholders in JQL query with actual values
func (jf *JQLFilter) parseJQLWithParams(jqlTemplate string, params map[string]interface{}) string {
	result := jqlTemplate

	// Replace simple placeholders
	for key, value := range params {
		placeholder := fmt.Sprintf("{{%s}}", key)

		var valueStr string
		switch v := value.(type) {
		case string:
			valueStr = jf.escapeJQLValue(v)
		case []string:
			// Handle array values for IN clauses
			escapedValues := make([]string, len(v))
			for i, item := range v {
				escapedValues[i] = jf.escapeJQLValue(item)
			}
			valueStr = fmt.Sprintf("(%s)", strings.Join(escapedValues, ","))
		case time.Time:
			valueStr = v.Format("2006/01/02 15:04")
		default:
			valueStr = fmt.Sprintf("%v", v)
		}

		result = strings.ReplaceAll(result, placeholder, valueStr)
	}

	return result
}

// escapeJQLValue escapes a string value for use in JQL queries
func (jf *JQLFilter) escapeJQLValue(value string) string {
	// Escape single quotes by doubling them
	escaped := strings.ReplaceAll(value, "'", "''")

	// Wrap in quotes if it contains spaces or special characters
	if strings.ContainsAny(escaped, " ~!@#$%^&*()_+={}[]|\\:;\"<>?,/") || strings.Contains(escaped, " ") {
		return fmt.Sprintf("'%s'", escaped)
	}

	return escaped
}

// isAllowedEvent checks if the event type is in the allowed events list
func (jf *JQLFilter) isAllowedEvent(eventType string) bool {
	// If no bidirectional events are configured, allow all events
	if len(jf.config.BidirectionalEvents) == 0 {
		return true
	}

	// Check if the event type is in the allowed list
	for _, allowedEvent := range jf.config.BidirectionalEvents {
		if eventType == allowedEvent {
			return true
		}
	}

	return false
}

// GetFilterStats returns statistics about the filtering process
func (jf *JQLFilter) GetFilterStats() map[string]interface{} {
	return map[string]interface{}{
		"jql_filter_enabled":    jf.config.JQLFilter != "",
		"jql_filter":            jf.config.JQLFilter,
		"skip_old_events":       jf.config.SkipOldEvents,
		"max_event_age_hours":   jf.config.MaxEventAge,
		"bidirectional_events":  jf.config.BidirectionalEvents,
		"bidirectional_enabled": jf.config.BidirectionalEnabled,
	}
}
