package gitlab

import (
	"fmt"
	"strings"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

// URLBuilder handles URL construction for GitLab entities
type URLBuilder struct {
	config *config.Config
}

// NewURLBuilder creates a new URL builder
func NewURLBuilder(cfg *config.Config) *URLBuilder {
	return &URLBuilder{
		config: cfg,
	}
}

// ConstructAuthorURL constructs the URL for an author
func (b *URLBuilder) ConstructAuthorURL(event *Event, author Author) string {
	if event.Project != nil && event.Project.WebURL != "" {
		// Extract base URL from project web URL
		baseURL := b.extractBaseURL(event.Project.WebURL)
		return fmt.Sprintf("%s/%s", baseURL, author.Name)
	}

	// Fallback to configured GitLab URL
	if b.config.GitLabBaseURL != "" {
		return fmt.Sprintf("%s/%s", strings.TrimSuffix(b.config.GitLabBaseURL, "/"), author.Name)
	}

	return ""
}

// ConstructBranchURL constructs the URL for a branch
func (b *URLBuilder) ConstructBranchURL(event *Event, ref string) string {
	if event.Project != nil && event.Project.WebURL != "" {
		return fmt.Sprintf("%s/-/tree/%s", event.Project.WebURL, ref)
	}
	return ""
}

// ConstructProjectURL constructs the project URL and returns both URL and path
func (b *URLBuilder) ConstructProjectURL(event *Event) (string, string) {
	if event.Project != nil {
		return event.Project.WebURL, event.Project.PathWithNamespace
	}
	return "", ""
}

// ConstructUserProfileURL constructs the URL for a user profile
func (b *URLBuilder) ConstructUserProfileURL(user *User) string {
	if user == nil {
		return ""
	}

	// Try to construct from project URL first
	if b.config.GitLabBaseURL != "" {
		return fmt.Sprintf("%s/%s", strings.TrimSuffix(b.config.GitLabBaseURL, "/"), user.Username)
	}

	return ""
}

// ConstructMergeRequestURL constructs the URL for a merge request
func (b *URLBuilder) ConstructMergeRequestURL(event *Event) string {
	if event.Project != nil && event.Project.WebURL != "" && event.ObjectAttributes != nil {
		return fmt.Sprintf("%s/-/merge_requests/%d", event.Project.WebURL, event.ObjectAttributes.ID)
	}
	return ""
}

// ConstructCommitURL constructs the URL for a commit
func (b *URLBuilder) ConstructCommitURL(event *Event, commitID string) string {
	if event.Project != nil && event.Project.WebURL != "" {
		return fmt.Sprintf("%s/-/commit/%s", event.Project.WebURL, commitID)
	}
	return ""
}

// ConstructIssueURL constructs the URL for an issue
func (b *URLBuilder) ConstructIssueURL(event *Event, issueID int) string {
	if event.Project != nil && event.Project.WebURL != "" {
		return fmt.Sprintf("%s/-/issues/%d", event.Project.WebURL, issueID)
	}
	return ""
}

// ConstructPipelineURL constructs the URL for a pipeline
func (b *URLBuilder) ConstructPipelineURL(event *Event, pipelineID int) string {
	if event.Project != nil && event.Project.WebURL != "" {
		return fmt.Sprintf("%s/-/pipelines/%d", event.Project.WebURL, pipelineID)
	}
	return ""
}

// ConstructJobURL constructs the URL for a job
func (b *URLBuilder) ConstructJobURL(event *Event, jobID int) string {
	if event.Project != nil && event.Project.WebURL != "" {
		return fmt.Sprintf("%s/-/jobs/%d", event.Project.WebURL, jobID)
	}
	return ""
}

// ConstructWikiPageURL constructs the URL for a wiki page
func (b *URLBuilder) ConstructWikiPageURL(event *Event, pageSlug string) string {
	if event.Project != nil && event.Project.WebURL != "" {
		return fmt.Sprintf("%s/-/wikis/%s", event.Project.WebURL, pageSlug)
	}
	return ""
}

// ConstructReleaseURL constructs the URL for a release
func (b *URLBuilder) ConstructReleaseURL(event *Event, tagName string) string {
	if event.Project != nil && event.Project.WebURL != "" {
		return fmt.Sprintf("%s/-/releases/%s", event.Project.WebURL, tagName)
	}
	return ""
}

// ConstructDeploymentURL constructs the URL for a deployment
func (b *URLBuilder) ConstructDeploymentURL(event *Event, deploymentID int) string {
	if event.Project != nil && event.Project.WebURL != "" {
		return fmt.Sprintf("%s/-/deployments/%d", event.Project.WebURL, deploymentID)
	}
	return ""
}

// ConstructFeatureFlagURL constructs the URL for a feature flag
func (b *URLBuilder) ConstructFeatureFlagURL(event *Event, flagName string) string {
	if event.Project != nil && event.Project.WebURL != "" {
		return fmt.Sprintf("%s/-/feature_flags/%s", event.Project.WebURL, flagName)
	}
	return ""
}

// extractBaseURL extracts the base URL from a project web URL
func (b *URLBuilder) extractBaseURL(projectURL string) string {
	// Remove the project path from the URL
	// Example: https://gitlab.com/group/project -> https://gitlab.com
	parts := strings.Split(projectURL, "/")
	if len(parts) >= 3 {
		return strings.Join(parts[:3], "/")
	}
	return projectURL
}

// GetGitLabBaseURL returns the base GitLab URL
func (b *URLBuilder) GetGitLabBaseURL() string {
	if b.config.GitLabBaseURL != "" {
		return strings.TrimSuffix(b.config.GitLabBaseURL, "/")
	}
	return ""
}

// IsValidURL checks if a URL is valid
func (b *URLBuilder) IsValidURL(url string) bool {
	return url != "" && (strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://"))
}

// SanitizeURL sanitizes a URL by removing sensitive information
func (b *URLBuilder) SanitizeURL(url string) string {
	// Remove any potential tokens or sensitive data from URL
	// This is a basic implementation - could be enhanced
	if strings.Contains(url, "token=") {
		parts := strings.Split(url, "token=")
		if len(parts) > 1 {
			tokenParts := strings.Split(parts[1], "&")
			if len(tokenParts) > 0 {
				return parts[0] + "token=***&" + strings.Join(tokenParts[1:], "&")
			}
		}
	}
	return url
}
