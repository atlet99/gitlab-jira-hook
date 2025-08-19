// Package jira provides Jira Software (Agile) integration functionality.
package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// AgileService provides methods for interacting with Jira Software (Agile) API
type AgileService struct {
	client *Client
	logger *slog.Logger
}

// NewAgileService creates a new Agile service
func NewAgileService(client *Client, logger *slog.Logger) *AgileService {
	return &AgileService{
		client: client,
		logger: logger,
	}
}

// getSingleResource makes a GET request to get a single resource by ID
func (s *AgileService) getSingleResource(ctx context.Context, endpoint string, resourceID int, resourceType string) (interface{}, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/%s/%d", s.client.baseURL, endpoint, resourceID)

	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var result interface{}
	if err := s.client.do(req, &result); err != nil {
		return nil, fmt.Errorf("failed to get %s: %w", resourceType, err)
	}

	return result, nil
}

// getIssuesForResource makes a GET request to get issues for a specific resource
func (s *AgileService) getIssuesForResource(ctx context.Context, endpoint string, resourceID int, startAt, maxResults int, resourceType string) ([]AgileIssue, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/%s/%d/issue", s.client.baseURL, endpoint, resourceID)

	// Add pagination parameters
	if startAt > 0 || maxResults > 0 {
		url += fmt.Sprintf("?startAt=%d&maxResults=%d", startAt, maxResults)
	}

	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var issuesResponse struct {
		Issues []AgileIssue `json:"issues"`
		Total  int          `json:"total"`
	}

	if err := s.client.do(req, &issuesResponse); err != nil {
		return nil, fmt.Errorf("failed to get %s issues: %w", resourceType, err)
	}

	s.logger.Info("Retrieved issues", "type", resourceType, "count", len(issuesResponse.Issues), "total", issuesResponse.Total)
	return issuesResponse.Issues, nil
}

// AgileBoard represents a Jira Agile board
type AgileBoard struct {
	ID         int                `json:"id"`
	Name       string             `json:"name"`
	Type       string             `json:"type"`
	Self       string             `json:"self"`
	Location   AgileBoardLocation `json:"location"`
	State      string             `json:"state"`
	FilterID   int                `json:"filterId"`
	FilterName string             `json:"filterName"`
	IsFavorite bool               `json:"isFavorite"`
	ViewURL    string             `json:"viewUrl"`
	Admin      AgileBoardAdmin    `json:"admin"`
}

// AgileBoardLocation represents the location of a board
type AgileBoardLocation struct {
	ProjectID   int    `json:"projectId"`
	ProjectName string `json:"projectName"`
	ProjectKey  string `json:"projectKey"`
	ProjectType string `json:"projectType"`
	AvatarID    int    `json:"avatarId"`
	Name        string `json:"name"`
}

// AgileBoardAdmin represents the admin of a board
type AgileBoardAdmin struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Active      bool   `json:"active"`
}

// AgileSprint represents a Jira Agile sprint
type AgileSprint struct {
	ID            int       `json:"id"`
	Name          string    `json:"name"`
	State         string    `json:"state"`
	StartDate     time.Time `json:"startDate"`
	EndDate       time.Time `json:"endDate"`
	CompleteDate  time.Time `json:"completeDate,omitempty"`
	OriginBoardID int       `json:"originBoardId"`
	Goal          string    `json:"goal,omitempty"`
}

// AgileIssue represents a Jira issue in Agile context
type AgileIssue struct {
	ID     string            `json:"id"`
	Key    string            `json:"key"`
	Self   string            `json:"self"`
	Fields *AgileIssueFields `json:"fields"`
}

// AgileIssueFields represents fields for Agile issues
type AgileIssueFields struct {
	Summary     string       `json:"summary"`
	Description string       `json:"description,omitempty"`
	Status      *Status      `json:"status"`
	Priority    *Priority    `json:"priority,omitempty"`
	Assignee    *JiraUser    `json:"assignee,omitempty"`
	Reporter    *JiraUser    `json:"reporter,omitempty"`
	Project     *JiraProject `json:"project"`
	IssueType   *IssueType   `json:"issuetype"`
	Created     time.Time    `json:"created"`
	Updated     time.Time    `json:"updated"`
}

// AgileEpic represents a Jira epic
type AgileEpic struct {
	ID      int         `json:"id"`
	Key     string      `json:"key"`
	Self    string      `json:"self"`
	Name    string      `json:"name"`
	Summary string      `json:"summary"`
	Color   *AgileColor `json:"color,omitempty"`
	Done    bool        `json:"done"`
}

// AgileColor represents a color
type AgileColor struct {
	Key string `json:"key"`
}

// Epic represents a Jira epic
type Epic struct {
	ID      int    `json:"id"`
	Key     string `json:"key"`
	Self    string `json:"self"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Color   *Color `json:"color,omitempty"`
	Done    bool   `json:"done"`
}

// Color represents a color
type Color struct {
	Key string `json:"key"`
}

// GetBoards retrieves all boards for a project
func (s *AgileService) GetBoards(ctx context.Context, projectKey string) ([]AgileBoard, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/board", s.client.baseURL)

	// Add project filter if provided
	params := []string{}
	if projectKey != "" {
		params = append(params, fmt.Sprintf("projectKeyOrId=%s", projectKey))
	}

	if len(params) > 0 {
		url += "?" + strings.Join(params, "&")
	}

	s.logger.Debug("GetBoards URL", "url", url, "projectKey", projectKey, "params", params)

	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var boardsResponse struct {
		Values []AgileBoard `json:"values"`
	}

	if err := s.client.do(req, &boardsResponse); err != nil {
		return nil, fmt.Errorf("failed to get boards: %w", err)
	}

	s.logger.Info("Retrieved boards", "count", len(boardsResponse.Values))
	return boardsResponse.Values, nil
}

// GetBoard retrieves a specific board by ID
func (s *AgileService) GetBoard(ctx context.Context, boardID int) (*AgileBoard, error) {
	board, err := s.getSingleResource(ctx, "board", boardID, "board")
	if err != nil {
		return nil, err
	}

	if board, ok := board.(*AgileBoard); ok {
		s.logger.Info("Retrieved board", "boardID", boardID, "name", board.Name)
		return board, nil
	}
	return nil, fmt.Errorf("failed to parse board response")
}

// GetBoardIssues retrieves issues for a specific board
func (s *AgileService) GetBoardIssues(ctx context.Context, boardID int, startAt, maxResults int) ([]AgileIssue, error) {
	return s.getIssuesForResource(ctx, "board", boardID, startAt, maxResults, "board")
}

// GetSprints retrieves all sprints for a board
func (s *AgileService) GetSprints(_ context.Context, boardID int, state string) ([]AgileSprint, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/board/%d/sprint", s.client.baseURL, boardID)

	// Add state filter if provided
	params := []string{}
	if state != "" {
		params = append(params, fmt.Sprintf("state=%s", state))
	}

	if len(params) > 0 {
		url += "?" + strings.Join(params, "&")
	}

	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var sprintsResponse struct {
		Values []AgileSprint `json:"values"`
	}

	if err := s.client.do(req, &sprintsResponse); err != nil {
		return nil, fmt.Errorf("failed to get sprints: %w", err)
	}

	s.logger.Info("Retrieved sprints", "boardID", boardID, "count", len(sprintsResponse.Values))
	return sprintsResponse.Values, nil
}

// GetSprint retrieves a specific sprint by ID
func (s *AgileService) GetSprint(ctx context.Context, sprintID int) (*AgileSprint, error) {
	sprint, err := s.getSingleResource(ctx, "sprint", sprintID, "sprint")
	if err != nil {
		return nil, err
	}

	if sprint, ok := sprint.(*AgileSprint); ok {
		s.logger.Info("Retrieved sprint", "sprintID", sprintID, "name", sprint.Name)
		return sprint, nil
	}
	return nil, fmt.Errorf("failed to parse sprint response")
}

// GetSprintIssues retrieves issues for a specific sprint
func (s *AgileService) GetSprintIssues(ctx context.Context, sprintID int, startAt, maxResults int) ([]AgileIssue, error) {
	return s.getIssuesForResource(ctx, "sprint", sprintID, startAt, maxResults, "sprint")
}

// CreateSprint creates a new sprint for a board
func (s *AgileService) CreateSprint(_ context.Context, boardID int, name, goal string, startDate, endDate time.Time) (*AgileSprint, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/sprint", s.client.baseURL)

	payload := map[string]interface{}{
		"name":      name,
		"boardId":   boardID,
		"goal":      goal,
		"startDate": startDate.Format(time.RFC3339),
		"endDate":   endDate.Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sprint payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var sprint AgileSprint
	if err := s.client.do(req, &sprint); err != nil {
		return nil, fmt.Errorf("failed to create sprint: %w", err)
	}

	s.logger.Info("Created sprint", "sprintID", sprint.ID, "name", sprint.Name)
	return &sprint, nil
}

// UpdateSprint updates an existing sprint
func (s *AgileService) UpdateSprint(_ context.Context, sprintID int, name, goal string, startDate, endDate *time.Time) (*AgileSprint, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/sprint/%d", s.client.baseURL, sprintID)

	payload := map[string]interface{}{
		"name": name,
		"goal": goal,
	}

	if startDate != nil {
		payload["startDate"] = startDate.Format(time.RFC3339)
	}
	if endDate != nil {
		payload["endDate"] = endDate.Format(time.RFC3339)
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sprint payload: %w", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var sprint AgileSprint
	if err := s.client.do(req, &sprint); err != nil {
		return nil, fmt.Errorf("failed to update sprint: %w", err)
	}

	s.logger.Info("Updated sprint", "sprintID", sprintID, "name", sprint.Name)
	return &sprint, nil
}

// StartSprint starts a sprint
func (s *AgileService) StartSprint(_ context.Context, sprintID int) (*AgileSprint, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/sprint/%d/issue/picker", s.client.baseURL, sprintID)

	req, err := http.NewRequest("POST", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var sprint AgileSprint
	if err := s.client.do(req, &sprint); err != nil {
		return nil, fmt.Errorf("failed to start sprint: %w", err)
	}

	s.logger.Info("Started sprint", "sprintID", sprintID)
	return &sprint, nil
}

// CompleteSprint completes a sprint
func (s *AgileService) CompleteSprint(_ context.Context, sprintID int) (*AgileSprint, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/sprint/%d", s.client.baseURL, sprintID)

	payload := map[string]interface{}{
		"state": "closed",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sprint payload: %w", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var sprint AgileSprint
	if err := s.client.do(req, &sprint); err != nil {
		return nil, fmt.Errorf("failed to complete sprint: %w", err)
	}

	s.logger.Info("Completed sprint", "sprintID", sprintID)
	return &sprint, nil
}

// GetEpics retrieves all epics
func (s *AgileService) GetEpics(_ context.Context, startAt, maxResults int, query string) ([]AgileEpic, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/epic", s.client.baseURL)

	// Add pagination and search parameters
	params := []string{}
	if startAt >= 0 {
		params = append(params, fmt.Sprintf("startAt=%d", startAt))
	}
	if maxResults > 0 {
		params = append(params, fmt.Sprintf("maxResults=%d", maxResults))
	}
	if query != "" {
		params = append(params, fmt.Sprintf("query=%s", query))
	}

	if len(params) > 0 {
		url += "?" + strings.Join(params, "&")
	}

	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var epicsResponse struct {
		Values []AgileEpic `json:"values"`
		Total  int         `json:"total"`
	}

	if err := s.client.do(req, &epicsResponse); err != nil {
		return nil, fmt.Errorf("failed to get epics: %w", err)
	}

	s.logger.Info("Retrieved epics", "count", len(epicsResponse.Values), "total", epicsResponse.Total)
	return epicsResponse.Values, nil
}

// GetEpic retrieves a specific epic by ID
func (s *AgileService) GetEpic(ctx context.Context, epicID int) (*AgileEpic, error) {
	epic, err := s.getSingleResource(ctx, "epic", epicID, "epic")
	if err != nil {
		return nil, err
	}

	if epic, ok := epic.(*AgileEpic); ok {
		s.logger.Info("Retrieved epic", "epicID", epicID, "name", epic.Name)
		return epic, nil
	}
	return nil, fmt.Errorf("failed to parse epic response")
}

// CreateEpic creates a new epic
func (s *AgileService) CreateEpic(_ context.Context, name, summary string, colorKey string) (*AgileEpic, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/epic", s.client.baseURL)

	payload := map[string]interface{}{
		"name":    name,
		"summary": summary,
	}

	if colorKey != "" {
		payload["color"] = map[string]string{"key": colorKey}
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal epic payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var epic AgileEpic
	if err := s.client.do(req, &epic); err != nil {
		return nil, fmt.Errorf("failed to create epic: %w", err)
	}

	s.logger.Info("Created epic", "epicID", epic.ID, "name", epic.Name)
	return &epic, nil
}

// UpdateEpic updates an existing epic
func (s *AgileService) UpdateEpic(_ context.Context, epicID int, name, summary *string, colorKey *string) (*AgileEpic, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/epic/%d", s.client.baseURL, epicID)

	payload := map[string]interface{}{}

	if name != nil {
		payload["name"] = *name
	}
	if summary != nil {
		payload["summary"] = *summary
	}
	if colorKey != nil {
		payload["color"] = map[string]string{"key": *colorKey}
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal epic payload: %w", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var epic AgileEpic
	if err := s.client.do(req, &epic); err != nil {
		return nil, fmt.Errorf("failed to update epic: %w", err)
	}

	s.logger.Info("Updated epic", "epicID", epicID, "name", epic.Name)
	return &epic, nil
}

// GetEpicIssues retrieves issues for a specific epic
func (s *AgileService) GetEpicIssues(ctx context.Context, epicID int, startAt, maxResults int) ([]AgileIssue, error) {
	return s.getIssuesForResource(ctx, "epic", epicID, startAt, maxResults, "epic")
}

// RankIssue ranks an issue within a sprint
func (s *AgileService) RankIssue(ctx context.Context, issueKey, afterIssueKey string) error {
	url := fmt.Sprintf("%s/rest/agile/1.0/issue/rank", s.client.baseURL)

	payload := map[string]interface{}{
		"issues": []string{issueKey},
	}

	if afterIssueKey != "" {
		payload["rankBeforeIssue"] = afterIssueKey
	} else {
		payload["rankAfterIssue"] = ""
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal rank payload: %w", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if err := s.client.do(req, nil); err != nil {
		return fmt.Errorf("failed to rank issue: %w", err)
	}

	s.logger.Info("Ranked issue", "issueKey", issueKey, "afterIssueKey", afterIssueKey)
	return nil
}

// MoveIssueToSprint moves an issue to a specific sprint
func (s *AgileService) MoveIssueToSprint(ctx context.Context, issueKey string, sprintID int) error {
	url := fmt.Sprintf("%s/rest/agile/1.0/backlog/issue", s.client.baseURL)

	payload := map[string]interface{}{
		"issues": []string{issueKey},
		"rank": map[string]interface{}{
			"rankBeforeIssue": "",
			"rankAfterIssue":  "",
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal move payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if err := s.client.do(req, nil); err != nil {
		return fmt.Errorf("failed to move issue to sprint: %w", err)
	}

	s.logger.Info("Moved issue to sprint", "issueKey", issueKey, "sprintID", sprintID)
	return nil
}
