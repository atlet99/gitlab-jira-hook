package jira

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

func TestAgileService_GetBoards(t *testing.T) {
	t.Run("successful board retrieval", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/rest/agile/1.0/board", r.URL.Path)
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"values": [
					{
						"id": 1,
						"name": "Scrum Board",
						"type": "scrum",
						"self": "https://jira.example.com/rest/agile/1.0/board/1",
						"location": {
							"projectId": 100,
							"projectName": "Test Project",
							"projectKey": "TEST",
							"projectType": "software",
							"avatarId": 1,
							"name": "Test Project"
						},
						"state": "active",
						"filterId": 100,
						"filterName": "Test Filter",
						"isFavorite": true,
						"viewUrl": "https://jira.example.com/secure/RapidBoard.jspa?rapidView=1",
						"admin": {
							"accountId": "user-account-id",
							"displayName": "Admin User",
							"active": true
						}
					}
				]
			}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          server.URL,
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		agile := NewAgileService(client, slog.Default())

		boards, err := agile.GetBoards(context.Background(), "")
		assert.NoError(t, err)
		assert.Len(t, boards, 1)
		assert.Equal(t, "Scrum Board", boards[0].Name)
		assert.Equal(t, "scrum", boards[0].Type)
		assert.Equal(t, "TEST", boards[0].Location.ProjectKey)
	})

	t.Run("board retrieval with project filter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/rest/agile/1.0/board", r.URL.Path)
			assert.Equal(t, "projectKeyOrId=TEST", r.URL.RawQuery)
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"values": []}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          server.URL,
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		agile := NewAgileService(client, slog.Default())

		boards, err := agile.GetBoards(context.Background(), "TEST")
		assert.NoError(t, err)
		assert.Empty(t, boards)
	})
}

func TestAgileService_GetBoard(t *testing.T) {
	t.Run("successful board retrieval by ID", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/rest/agile/1.0/board/1", r.URL.Path)
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"id": 1,
				"name": "Scrum Board",
				"type": "scrum",
				"self": "https://jira.example.com/rest/agile/1.0/board/1",
				"location": {
					"projectId": 100,
					"projectName": "Test Project",
					"projectKey": "TEST",
					"projectType": "software",
					"avatarId": 1,
					"name": "Test Project"
				},
				"state": "active",
				"filterId": 100,
				"filterName": "Test Filter",
				"isFavorite": true,
				"viewUrl": "https://jira.example.com/secure/RapidBoard.jspa?rapidView=1",
				"admin": {
					"accountId": "user-account-id",
					"displayName": "Admin User",
					"active": true
				}
			}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          server.URL,
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		agile := NewAgileService(client, slog.Default())

		board, err := agile.GetBoard(context.Background(), 1)
		assert.NoError(t, err)
		require.NotNil(t, board)
		assert.Equal(t, "Scrum Board", board.Name)
		assert.Equal(t, "scrum", board.Type)
		assert.Equal(t, "TEST", board.Location.ProjectKey)
	})
}

func TestAgileService_GetBoardIssues(t *testing.T) {
	t.Run("successful board issues retrieval", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/rest/agile/1.0/board/1/issue", r.URL.Path)
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"issues": [
					{
						"id": "10001",
						"key": "TEST-1",
						"self": "https://jira.example.com/rest/api/3/issue/10001",
						"fields": {
							"summary": "Test issue 1",
							"status": {"name": "To Do"},
							"priority": {"name": "High"},
							"assignee": {"accountId": "user-account-id"},
							"reporter": {"accountId": "reporter-account-id"},
							"project": {"id": "100", "key": "TEST"},
							"issuetype": {"id": "1", "name": "Story"},
							"created": "2023-01-01T00:00:00.000Z",
							"updated": "2023-01-01T00:00:00.000Z"
						}
					}
				],
				"total": 1
			}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          server.URL,
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		agile := NewAgileService(client, slog.Default())

		issues, err := agile.GetBoardIssues(context.Background(), 1, 0, 50)
		assert.NoError(t, err)
		assert.Len(t, issues, 1)
		assert.Equal(t, "TEST-1", issues[0].Key)
		assert.Equal(t, "Test issue 1", issues[0].Fields.Summary)
	})

	t.Run("board issues retrieval with pagination", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/rest/agile/1.0/board/1/issue", r.URL.Path)
			assert.Equal(t, "startAt=10&maxResults=20", r.URL.RawQuery)
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"issues": [],
				"total": 0
			}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          server.URL,
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		agile := NewAgileService(client, slog.Default())

		issues, err := agile.GetBoardIssues(context.Background(), 1, 10, 20)
		assert.NoError(t, err)
		assert.Empty(t, issues)
	})
}

func TestAgileService_GetSprints(t *testing.T) {
	t.Run("successful sprints retrieval", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/rest/agile/1.0/board/1/sprint", r.URL.Path)
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"values": [
					{
						"id": 1,
						"name": "Sprint 1",
						"state": "active",
						"boardId": 1,
						"goal": "Complete user authentication",
						"startDate": "2023-01-01T00:00:00.000Z",
						"endDate": "2023-01-14T23:59:59.000Z",
						"completeDate": null,
						"self": "https://jira.example.com/rest/agile/1.0/sprint/1"
					}
				]
			}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          server.URL,
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		agile := NewAgileService(client, slog.Default())

		sprints, err := agile.GetSprints(context.Background(), 1, "")
		assert.NoError(t, err)
		assert.Len(t, sprints, 1)
		assert.Equal(t, "Sprint 1", sprints[0].Name)
		assert.Equal(t, "active", sprints[0].State)
	})

	t.Run("sprints retrieval with state filter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/rest/agile/1.0/board/1/sprint", r.URL.Path)
			assert.Equal(t, "state=active", r.URL.RawQuery)
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"values": []}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          server.URL,
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		agile := NewAgileService(client, slog.Default())

		sprints, err := agile.GetSprints(context.Background(), 1, "active")
		assert.NoError(t, err)
		assert.Empty(t, sprints)
	})
}

func TestAgileService_GetSprintIssues(t *testing.T) {
	t.Run("successful sprint issues retrieval", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/rest/agile/1.0/sprint/1/issue", r.URL.Path)
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"issues": [
					{
						"id": "10001",
						"key": "TEST-1",
						"self": "https://jira.example.com/rest/api/3/issue/10001",
						"fields": {
							"summary": "Test issue 1",
							"status": {"name": "To Do"},
							"priority": {"name": "High"},
							"assignee": {"accountId": "user-account-id"},
							"reporter": {"accountId": "reporter-account-id"},
							"project": {"id": "100", "key": "TEST"},
							"issuetype": {"id": "1", "name": "Story"},
							"created": "2023-01-01T00:00:00.000Z",
							"updated": "2023-01-01T00:00:00.000Z"
						}
					}
				],
				"total": 1
			}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          server.URL,
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		agile := NewAgileService(client, slog.Default())

		issues, err := agile.GetSprintIssues(context.Background(), 1, 0, 50)
		assert.NoError(t, err)
		assert.Len(t, issues, 1)
		assert.Equal(t, "TEST-1", issues[0].Key)
		assert.Equal(t, "Test issue 1", issues[0].Fields.Summary)
	})
}

func TestAgileService_GetEpics(t *testing.T) {
	t.Run("successful epics retrieval", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/rest/agile/1.0/epic", r.URL.Path)
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"values": [
					{
						"id": 1,
						"key": "TEST-100",
						"self": "https://jira.example.com/rest/agile/1.0/epic/1",
						"name": "User Authentication",
						"summary": "Implement user authentication system",
						"color": {"key": "color_1"},
						"done": false
					}
				],
				"total": 1
			}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          server.URL,
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		agile := NewAgileService(client, slog.Default())

		epics, err := agile.GetEpics(context.Background(), 0, 50, "")
		assert.NoError(t, err)
		assert.Len(t, epics, 1)
		assert.Equal(t, "User Authentication", epics[0].Name)
		assert.Equal(t, "Implement user authentication system", epics[0].Summary)
		assert.False(t, epics[0].Done)
	})

	t.Run("epics retrieval with search query", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/rest/agile/1.0/epic", r.URL.Path)
			// Check that all required parameters are present, order doesn't matter
			assert.Contains(t, r.URL.RawQuery, "query=authentication")
			assert.Contains(t, r.URL.RawQuery, "startAt=0")
			assert.Contains(t, r.URL.RawQuery, "maxResults=50")
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"values": [], "total": 0}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          server.URL,
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		agile := NewAgileService(client, slog.Default())

		epics, err := agile.GetEpics(context.Background(), 0, 50, "authentication")
		assert.NoError(t, err)
		assert.Empty(t, epics)
	})
}

func TestAgileService_GetEpicIssues(t *testing.T) {
	t.Run("successful epic issues retrieval", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/rest/agile/1.0/epic/1/issue", r.URL.Path)
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"issues": [
					{
						"id": "10001",
						"key": "TEST-1",
						"self": "https://jira.example.com/rest/api/3/issue/10001",
						"fields": {
							"summary": "Test issue 1",
							"status": {"name": "To Do"},
							"priority": {"name": "High"},
							"assignee": {"accountId": "user-account-id"},
							"reporter": {"accountId": "reporter-account-id"},
							"project": {"id": "100", "key": "TEST"},
							"issuetype": {"id": "1", "name": "Story"},
							"created": "2023-01-01T00:00:00.000Z",
							"updated": "2023-01-01T00:00:00.000Z"
						}
					}
				],
				"total": 1
			}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          server.URL,
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		agile := NewAgileService(client, slog.Default())

		issues, err := agile.GetEpicIssues(context.Background(), 1, 0, 50)
		assert.NoError(t, err)
		assert.Len(t, issues, 1)
		assert.Equal(t, "TEST-1", issues[0].Key)
		assert.Equal(t, "Test issue 1", issues[0].Fields.Summary)
	})
}

func TestAgileService_RankIssue(t *testing.T) {
	t.Run("successful issue ranking", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "PUT", r.Method)
			assert.Equal(t, "/rest/agile/1.0/issue/rank", r.URL.Path)
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			var payload map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&payload)
			require.NoError(t, err)
			assert.Equal(t, []interface{}{"TEST-1"}, payload["issues"])
			assert.Equal(t, "", payload["rankAfterIssue"])

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          server.URL,
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		agile := NewAgileService(client, slog.Default())

		err := agile.RankIssue(context.Background(), "TEST-1", "")
		assert.NoError(t, err)
	})

	t.Run("issue ranking with after issue", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "PUT", r.Method)
			assert.Equal(t, "/rest/agile/1.0/issue/rank", r.URL.Path)
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			var payload map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&payload)
			require.NoError(t, err)
			assert.Equal(t, []interface{}{"TEST-1"}, payload["issues"])
			assert.Equal(t, "TEST-2", payload["rankBeforeIssue"])

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          server.URL,
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		agile := NewAgileService(client, slog.Default())

		err := agile.RankIssue(context.Background(), "TEST-1", "TEST-2")
		assert.NoError(t, err)
	})
}

func TestAgileService_MoveIssueToSprint(t *testing.T) {
	t.Run("successful issue move to sprint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/rest/agile/1.0/backlog/issue", r.URL.Path)
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			var payload map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&payload)
			require.NoError(t, err)
			assert.Equal(t, []interface{}{"TEST-1"}, payload["issues"])

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          server.URL,
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		agile := NewAgileService(client, slog.Default())

		err := agile.MoveIssueToSprint(context.Background(), "TEST-1", 1)
		assert.NoError(t, err)
	})
}
