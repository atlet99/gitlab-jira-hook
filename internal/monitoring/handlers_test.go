package monitoring

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"log/slog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

func TestNewHandler(t *testing.T) {
	monitor := &WebhookMonitor{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	handler := NewHandler(monitor, logger)

	assert.NotNil(t, handler)
	assert.Equal(t, monitor, handler.monitor)
	assert.Equal(t, logger, handler.logger)
}

func TestHandler_HandleStatus(t *testing.T) {
	monitor := &WebhookMonitor{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(monitor, logger)

	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()

	handler.HandleStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "status")
	assert.Contains(t, response, "timestamp")
}

func TestHandler_HandleMetrics(t *testing.T) {
	monitor := &WebhookMonitor{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(monitor, logger)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	handler.HandleMetrics(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "status")
	assert.Contains(t, response, "timestamp")
	assert.Contains(t, response, "metrics")
}

func TestHandler_HandleHealth(t *testing.T) {
	monitor := &WebhookMonitor{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(monitor, logger)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.HandleHealth(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "status")
	assert.Contains(t, response, "endpoints")
	assert.Contains(t, response, "metrics")
	assert.Contains(t, response, "total_endpoints")
	assert.Contains(t, response, "healthy_endpoints")
}

func TestHandler_HandleDetailedStatus(t *testing.T) {
	monitor := &WebhookMonitor{
		statuses: map[string]*WebhookStatus{
			"/gitlab-hook": {
				Endpoint:  "/gitlab-hook",
				Status:    "healthy",
				LastCheck: time.Now(),
			},
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(monitor, logger)

	req := httptest.NewRequest("GET", "/detailed?endpoint=/gitlab-hook", nil)
	w := httptest.NewRecorder()

	handler.HandleDetailedStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "ok", response["status"])
	assert.Equal(t, "/gitlab-hook", response["endpoint"])
	assert.Contains(t, response, "endpoint_status")
}

func TestHandler_HandleReconnect(t *testing.T) {
	monitor := &WebhookMonitor{
		config: &config.Config{
			Port:         "8080",
			GitLabSecret: "test-secret",
		},
		statuses: map[string]*WebhookStatus{
			"/gitlab-hook": {
				Endpoint:  "/gitlab-hook",
				Status:    "healthy",
				LastCheck: time.Now(),
			},
		},
		metrics:    make(map[string]*WebhookMetrics),
		httpClient: &http.Client{},
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(monitor, logger)

	req := httptest.NewRequest("POST", "/reconnect?endpoint=/gitlab-hook", nil)
	w := httptest.NewRecorder()

	handler.HandleReconnect(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "ok", response["status"])
	assert.Equal(t, "/gitlab-hook", response["endpoint"])
	assert.Contains(t, response, "message")
}

func TestHandler_HandleStatus_WithUnhealthyEndpoints(t *testing.T) {
	monitor := &WebhookMonitor{
		statuses: map[string]*WebhookStatus{
			"/gitlab-hook": {
				Endpoint:  "/gitlab-hook",
				Status:    "unhealthy",
				LastCheck: time.Now().Add(-time.Hour),
				Error:     "connection timeout",
			},
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(monitor, logger)

	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()

	handler.HandleStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "ok", response["status"])
	assert.Contains(t, response, "endpoints")
}

func TestHandler_HandleMetrics_WithData(t *testing.T) {
	monitor := &WebhookMonitor{
		metrics: map[string]*WebhookMetrics{
			"/gitlab-hook": {
				TotalRequests:       100,
				SuccessfulRequests:  90,
				FailedRequests:      10,
				AverageResponseTime: time.Millisecond * 150,
			},
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(monitor, logger)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	handler.HandleMetrics(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "ok", response["status"])
	assert.Contains(t, response, "metrics")

	metrics := response["metrics"].(map[string]interface{})
	assert.Contains(t, metrics, "/gitlab-hook")
}

func TestHandler_HandleHealth_WithHealthyStatus(t *testing.T) {
	monitor := &WebhookMonitor{
		statuses: map[string]*WebhookStatus{
			"/gitlab-hook": {
				Endpoint:  "/gitlab-hook",
				Status:    "healthy",
				LastCheck: time.Now(),
			},
			"/project-hook": {
				Endpoint:  "/project-hook",
				Status:    "healthy",
				LastCheck: time.Now(),
			},
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(monitor, logger)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.HandleHealth(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response["status"])
	assert.Equal(t, float64(2), response["total_endpoints"])
	assert.Equal(t, float64(2), response["healthy_endpoints"])
}

func TestHandler_HandleDetailedStatus_WithEndpointData(t *testing.T) {
	monitor := &WebhookMonitor{
		statuses: map[string]*WebhookStatus{
			"/gitlab-hook": {
				Endpoint:     "/gitlab-hook",
				Status:       "healthy",
				LastCheck:    time.Now(),
				ResponseTime: time.Millisecond * 100,
			},
		},
		metrics: map[string]*WebhookMetrics{
			"/gitlab-hook": {
				TotalRequests:       50,
				SuccessfulRequests:  45,
				FailedRequests:      5,
				AverageResponseTime: time.Millisecond * 120,
			},
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(monitor, logger)

	req := httptest.NewRequest("GET", "/detailed?endpoint=/gitlab-hook", nil)
	w := httptest.NewRecorder()

	handler.HandleDetailedStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "ok", response["status"])
	assert.Equal(t, "/gitlab-hook", response["endpoint"])
	assert.Contains(t, response, "endpoint_status")
	assert.Contains(t, response, "metrics")
}
