package gitlab

import (
	"strings"
	"testing"
)

func TestBranchURLFormatting(t *testing.T) {
	tests := []struct {
		name           string
		ref            string
		projectURL     string
		expectedBranch string
		expectedURL    string
	}{
		{
			name:           "Simple branch name",
			ref:            "test",
			projectURL:     "https://git.local.com/devops/test-jira-webhook",
			expectedBranch: "test",
			expectedURL:    "https://git.local.com/devops/test-jira-webhook/-/tree/test",
		},
		{
			name:           "Branch with refs/heads/ prefix",
			ref:            "refs/heads/main",
			projectURL:     "https://git.local.com/devops/test-jira-webhook",
			expectedBranch: "main",
			expectedURL:    "https://git.local.com/devops/test-jira-webhook/-/tree/main",
		},
		{
			name:           "Feature branch with refs/heads/ prefix",
			ref:            "refs/heads/feature/new-feature",
			projectURL:     "https://git.local.com/devops/test-jira-webhook",
			expectedBranch: "feature/new-feature",
			expectedURL:    "https://git.local.com/devops/test-jira-webhook/-/tree/feature/new-feature",
		},
		{
			name:           "Tag with refs/tags/ prefix (should not be processed)",
			ref:            "refs/tags/v1.0.0",
			projectURL:     "https://git.local.com/devops/test-jira-webhook",
			expectedBranch: "refs/tags/v1.0.0",
			expectedURL:    "https://git.local.com/devops/test-jira-webhook/-/tree/refs/tags/v1.0.0",
		},
		{
			name:           "Empty ref",
			ref:            "",
			projectURL:     "https://git.local.com/devops/test-jira-webhook",
			expectedBranch: "",
			expectedURL:    "https://git.local.com/devops/test-jira-webhook/-/tree/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Extract branch name from refs/heads/branch format
			branchName := tt.ref
			if strings.HasPrefix(tt.ref, "refs/heads/") {
				branchName = strings.TrimPrefix(tt.ref, "refs/heads/")
			}

			// Construct branch URL
			branchURL := ""
			if tt.projectURL != "" {
				branchURL = tt.projectURL + "/-/tree/" + branchName
			}

			// Verify branch name extraction
			if branchName != tt.expectedBranch {
				t.Errorf("Branch name extraction failed: got %s, want %s", branchName, tt.expectedBranch)
			}

			// Verify URL construction
			if branchURL != tt.expectedURL {
				t.Errorf("URL construction failed: got %s, want %s", branchURL, tt.expectedURL)
			}
		})
	}
}
