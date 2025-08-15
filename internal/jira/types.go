package jira

import "time"

// WebhookEvent represents a Jira webhook event
type WebhookEvent struct {
	// Webhook metadata
	Timestamp    int64  `json:"timestamp"`
	WebhookEvent string `json:"webhookEvent"`

	// Issue events
	Issue   *JiraIssue   `json:"issue,omitempty"`
	User    *JiraUser    `json:"user,omitempty"`
	Project *JiraProject `json:"project,omitempty"`
	Comment *Comment     `json:"comment,omitempty"`
	Worklog *Worklog     `json:"worklog,omitempty"`

	// Change log for updates
	Changelog *Changelog `json:"changelog,omitempty"`

	// Version events
	Version *Version `json:"version,omitempty"`

	// Sprint events (if using Jira Software)
	Sprint *Sprint `json:"sprint,omitempty"`
	Board  *Board  `json:"board,omitempty"`
}

// JiraIssue represents a Jira issue
//
//nolint:revive // avoid conflict with existing Issue type
type JiraIssue struct {
	ID     string           `json:"id"`
	Key    string           `json:"key"`
	Self   string           `json:"self"`
	Fields *JiraIssueFields `json:"fields"`
}

// JiraIssueFields represents issue fields
//
//nolint:revive // avoid conflict with existing IssueFields type
type JiraIssueFields struct {
	Summary     string       `json:"summary"`
	Description interface{}  `json:"description"` // Can be string or ADF
	IssueType   *IssueType   `json:"issuetype"`
	Priority    *Priority    `json:"priority"`
	Status      *Status      `json:"status"`
	Resolution  *Resolution  `json:"resolution"`
	Creator     *JiraUser    `json:"creator"`
	Reporter    *JiraUser    `json:"reporter"`
	Assignee    *JiraUser    `json:"assignee"`
	Created     string       `json:"created"`
	Updated     string       `json:"updated"`
	Labels      []string     `json:"labels"`
	Components  []Component  `json:"components"`
	FixVersions []Version    `json:"fixVersions"`
	Project     *JiraProject `json:"project"`
}

// JiraUser represents a Jira user
//
//nolint:revive // avoid conflict with existing User type
type JiraUser struct {
	AccountID    string `json:"accountId"`
	EmailAddress string `json:"emailAddress"`
	DisplayName  string `json:"displayName"`
	Active       bool   `json:"active"`
	TimeZone     string `json:"timeZone"`
	AccountType  string `json:"accountType"`
	Self         string `json:"self"`
}

// JiraProject represents a Jira project
//
//nolint:revive // avoid conflict with existing Project type
type JiraProject struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	ProjectType struct {
		Key string `json:"key"`
	} `json:"projectTypeKey"`
	Self string `json:"self"`
}

// Comment represents a Jira comment
type Comment struct {
	ID           string      `json:"id"`
	Self         string      `json:"self"`
	Author       *JiraUser   `json:"author"`
	Body         interface{} `json:"body"` // Can be string or ADF
	UpdateAuthor *JiraUser   `json:"updateAuthor"`
	Created      string      `json:"created"`
	Updated      string      `json:"updated"`
	Visibility   *Visibility `json:"visibility,omitempty"`
}

// Worklog represents a Jira worklog entry
type Worklog struct {
	ID               string      `json:"id"`
	Self             string      `json:"self"`
	Author           *JiraUser   `json:"author"`
	UpdateAuthor     *JiraUser   `json:"updateAuthor"`
	Comment          interface{} `json:"comment"` // Can be string or ADF
	Created          string      `json:"created"`
	Updated          string      `json:"updated"`
	Started          string      `json:"started"`
	TimeSpent        string      `json:"timeSpent"`
	TimeSpentSeconds int         `json:"timeSpentSeconds"`
	Visibility       *Visibility `json:"visibility,omitempty"`
}

// Changelog represents changes made to an issue
type Changelog struct {
	ID    string          `json:"id"`
	Items []ChangelogItem `json:"items"`
}

// ChangelogItem represents a single change in the changelog
type ChangelogItem struct {
	Field      string `json:"field"`
	FieldType  string `json:"fieldtype"`
	From       string `json:"from"`
	FromString string `json:"fromString"`
	To         string `json:"to"`
	ToString   string `json:"toString"`
}

// IssueType represents a Jira issue type
type IssueType struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IconURL     string `json:"iconUrl"`
	Self        string `json:"self"`
	Subtask     bool   `json:"subtask"`
}

// Priority represents a Jira priority
type Priority struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IconURL string `json:"iconUrl"`
	Self    string `json:"self"`
}

// Status represents a Jira status
type Status struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	IconURL        string         `json:"iconUrl"`
	Self           string         `json:"self"`
	StatusCategory StatusCategory `json:"statusCategory"`
}

// StatusCategory represents a status category
type StatusCategory struct {
	ID        int    `json:"id"`
	Key       string `json:"key"`
	ColorName string `json:"colorName"`
	Name      string `json:"name"`
	Self      string `json:"self"`
}

// Resolution represents a Jira resolution
type Resolution struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Self        string `json:"self"`
}

// Component represents a Jira component
type Component struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Self string `json:"self"`
}

// Version represents a Jira version
type Version struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Archived        bool   `json:"archived"`
	Released        bool   `json:"released"`
	ReleaseDate     string `json:"releaseDate"`
	UserReleaseDate string `json:"userReleaseDate"`
	ProjectID       int    `json:"projectId"`
	Self            string `json:"self"`
}

// Sprint represents a Jira sprint (Jira Software)
type Sprint struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	State        string    `json:"state"`
	BoardID      int       `json:"originBoardId"`
	Goal         string    `json:"goal"`
	StartDate    time.Time `json:"startDate"`
	EndDate      time.Time `json:"endDate"`
	CompleteDate time.Time `json:"completeDate"`
	Self         string    `json:"self"`
}

// Board represents a Jira board (Jira Software)
type Board struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Self     string `json:"self"`
	Location struct {
		ProjectID      int    `json:"projectId"`
		ProjectName    string `json:"projectName"`
		ProjectKey     string `json:"projectKey"`
		ProjectTypeKey string `json:"projectTypeKey"`
	} `json:"location"`
}

// Visibility represents comment/worklog visibility
type Visibility struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// CommentPayload represents the payload for adding a comment
type CommentPayload struct {
	Body CommentBody `json:"body"`
}

// Transition represents a Jira issue transition
type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   Status `json:"to"`
}

// TransitionsResponse represents the response from the transitions API
type TransitionsResponse struct {
	Transitions []Transition `json:"transitions"`
}

// TransitionPayload represents the payload for executing a transition
type TransitionPayload struct {
	Transition TransitionID `json:"transition"`
}

// TransitionID represents the transition ID in the payload
type TransitionID struct {
	ID string `json:"id"`
}

// TransitionRequest represents a request to execute a transition
type TransitionRequest struct {
	TransitionID string `json:"transitionId"`
}

// CommentBody represents the body of a comment
type CommentBody struct {
	Type    string    `json:"type"`
	Version int       `json:"version"`
	Content []Content `json:"content"`
}

// Content represents content in a comment
type Content struct {
	Type    string        `json:"type"`
	Content []TextContent `json:"content"`
}

// TextContent represents text content
// Marks for formatting (bold, link, etc.)
type TextContent struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Marks []Mark `json:"marks,omitempty"`
}

// Mark describes text formatting (bold, link, etc.)
type Mark struct {
	Type  string                 `json:"type"`
	Attrs map[string]interface{} `json:"attrs,omitempty"`
}
