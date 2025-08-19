// Package jira provides tests for Jira Service Management (JSM) integration.
package jira

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

func TestJSMService_GetServiceDesks(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/servicedeskapi/servicedesk", r.URL.Path)

		// Mock response
		response := []ServiceDesk{
			{
				ID:          "SD1",
				Name:        "Service Desk 1",
				Key:         "SD1",
				ProjectID:   10001,
				ProjectName: "Test Project",
				Description: "Test service desk",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Logf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Create test config
	cfg := &config.Config{
		JiraBaseURL: server.URL,
		JiraEmail:   "test@example.com",
		JiraToken:   "test-token",
	}

	// Create client and JSM service
	client := NewClient(cfg)
	jsm := NewJSMService(client, slog.Default())

	// Test GetServiceDesks
	serviceDesks, err := jsm.GetServiceDesks(context.Background())
	require.NoError(t, err)
	require.Len(t, serviceDesks, 1)
	assert.Equal(t, "SD1", serviceDesks[0].ID)
	assert.Equal(t, "Service Desk 1", serviceDesks[0].Name)
}

func TestJSMService_GetServiceDesk(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/servicedeskapi/servicedesk/SD1", r.URL.Path)

		// Mock response
		response := ServiceDesk{
			ID:          "SD1",
			Name:        "Service Desk 1",
			Key:         "SD1",
			ProjectID:   10001,
			ProjectName: "Test Project",
			Description: "Test service desk",
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Logf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Create test config
	cfg := &config.Config{
		JiraBaseURL: server.URL,
		JiraEmail:   "test@example.com",
		JiraToken:   "test-token",
	}

	// Create client and JSM service
	client := NewClient(cfg)
	jsm := NewJSMService(client, slog.Default())

	// Test GetServiceDesk
	serviceDesk, err := jsm.GetServiceDesk(context.Background(), "SD1")
	require.NoError(t, err)
	assert.Equal(t, "SD1", serviceDesk.ID)
	assert.Equal(t, "Service Desk 1", serviceDesk.Name)
}

func TestJSMService_GetRequestTypes(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/servicedeskapi/servicedesk/SD1/requesttype", r.URL.Path)

		// Mock response
		response := []RequestType{
			{
				ID:            "RT1",
				Name:          "Bug Report",
				Description:   "Report a bug",
				HelpText:      "Please describe the bug you encountered",
				ServiceDeskID: "SD1",
				Fields: []RequestTypeField{
					{
						ID:          "summary",
						Name:        "Summary",
						Type:        "text",
						Required:    true,
						Description: "Brief description of the issue",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Logf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Create test config
	cfg := &config.Config{
		JiraBaseURL: server.URL,
		JiraEmail:   "test@example.com",
		JiraToken:   "test-token",
	}

	// Create client and JSM service
	client := NewClient(cfg)
	jsm := NewJSMService(client, slog.Default())

	// Test GetRequestTypes
	requestTypes, err := jsm.GetRequestTypes(context.Background(), "SD1")
	require.NoError(t, err)
	require.Len(t, requestTypes, 1)
	assert.Equal(t, "RT1", requestTypes[0].ID)
	assert.Equal(t, "Bug Report", requestTypes[0].Name)
}

func TestJSMService_CreateRequest(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/servicedeskapi/request", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse request body
		var reqBody map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		assert.Equal(t, "SD1", reqBody["serviceDeskId"])
		assert.Equal(t, "RT1", reqBody["requestTypeId"])

		// Mock response
		response := Request{
			ID:          "REQ1",
			Key:         "REQ-1",
			ProjectID:   10001,
			ProjectKey:  "PROJ",
			IssueType:   "Service Request",
			Status:      RequestStatus{ID: "1", Name: "Open", StatusCategory: "Open"},
			CreatedDate: time.Now(),
			UpdatedDate: time.Now(),
			Reporter:    RequestUser{ID: "user1", Name: "Test User"},
			ServiceDesk: ServiceDesk{ID: "SD1", Name: "Service Desk 1"},
			RequestType: RequestType{ID: "RT1", Name: "Bug Report"},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Logf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Create test config
	cfg := &config.Config{
		JiraBaseURL: server.URL,
		JiraEmail:   "test@example.com",
		JiraToken:   "test-token",
	}

	// Create client and JSM service
	client := NewClient(cfg)
	jsm := NewJSMService(client, slog.Default())

	// Test CreateRequest
	fields := map[string]interface{}{
		"summary":     "Test request",
		"description": "This is a test request",
	}

	request, err := jsm.CreateRequest(context.Background(), "SD1", "RT1", fields)
	require.NoError(t, err)
	assert.Equal(t, "REQ1", request.ID)
	assert.Equal(t, "REQ-1", request.Key)
	assert.Equal(t, "Service Request", request.IssueType)
}

func TestJSMService_GetRequest(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/servicedeskapi/request/REQ1", r.URL.Path)

		// Mock response
		response := Request{
			ID:          "REQ1",
			Key:         "REQ-1",
			ProjectID:   10001,
			ProjectKey:  "PROJ",
			IssueType:   "Service Request",
			Status:      RequestStatus{ID: "1", Name: "Open", StatusCategory: "Open"},
			CreatedDate: time.Now(),
			UpdatedDate: time.Now(),
			Reporter:    RequestUser{ID: "user1", Name: "Test User"},
			ServiceDesk: ServiceDesk{ID: "SD1", Name: "Service Desk 1"},
			RequestType: RequestType{ID: "RT1", Name: "Bug Report"},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Logf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Create test config
	cfg := &config.Config{
		JiraBaseURL: server.URL,
		JiraEmail:   "test@example.com",
		JiraToken:   "test-token",
	}

	// Create client and JSM service
	client := NewClient(cfg)
	jsm := NewJSMService(client, slog.Default())

	// Test GetRequest
	request, err := jsm.GetRequest(context.Background(), "REQ1")
	require.NoError(t, err)
	assert.Equal(t, "REQ1", request.ID)
	assert.Equal(t, "REQ-1", request.Key)
}

func TestJSMService_AddComment(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/servicedeskapi/request/REQ1/comment", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse request body
		var reqBody map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		assert.Equal(t, "This is a test comment", reqBody["body"])

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	// Create test config
	cfg := &config.Config{
		JiraBaseURL: server.URL,
		JiraEmail:   "test@example.com",
		JiraToken:   "test-token",
	}

	// Create client and JSM service
	client := NewClient(cfg)
	jsm := NewJSMService(client, slog.Default())

	// Test AddComment
	err := jsm.AddComment(context.Background(), "REQ1", "This is a test comment")
	require.NoError(t, err)
}

func TestJSMService_GetSLAs(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/servicedeskapi/sla", r.URL.Path)

		// Mock response
		response := []SLA{
			{
				ID:            "SLA1",
				Name:          "Standard SLA",
				GroupID:       "group1",
				ServiceDeskID: "SD1",
				Active:        true,
				IsDefault:     true,
				Target:        SLATarget{Type: "businessHours", Value: 4, Unit: "hours"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Logf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Create test config
	cfg := &config.Config{
		JiraBaseURL: server.URL,
		JiraEmail:   "test@example.com",
		JiraToken:   "test-token",
	}

	// Create client and JSM service
	client := NewClient(cfg)
	jsm := NewJSMService(client, slog.Default())

	// Test GetSLAs
	slas, err := jsm.GetSLAs(context.Background(), "SD1")
	require.NoError(t, err)
	require.Len(t, slas, 1)
	assert.Equal(t, "SLA1", slas[0].ID)
	assert.Equal(t, "Standard SLA", slas[0].Name)
}

func TestJSMService_GetCustomers(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/servicedeskapi/customer", r.URL.Path)

		// Mock response
		response := []Customer{
			{
				ID:          "CUST1",
				Name:        "John Doe",
				Key:         "CUST1",
				AccountID:   "account1",
				Email:       "john@example.com",
				DisplayName: "John Doe",
				Active:      true,
				Links:       CustomerLinks{Self: "https://example.com/rest/servicedeskapi/customer/CUST1"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Logf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Create test config
	cfg := &config.Config{
		JiraBaseURL: server.URL,
		JiraEmail:   "test@example.com",
		JiraToken:   "test-token",
	}

	// Create client and JSM service
	client := NewClient(cfg)
	jsm := NewJSMService(client, slog.Default())

	// Test GetCustomers
	customers, err := jsm.GetCustomers(context.Background(), "SD1")
	require.NoError(t, err)
	require.Len(t, customers, 1)
	assert.Equal(t, "CUST1", customers[0].ID)
	assert.Equal(t, "John Doe", customers[0].Name)
	assert.Equal(t, "john@example.com", customers[0].Email)
}

func TestJSMService_CreateCustomer(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/servicedeskapi/customer", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse request body
		var reqBody map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		assert.Equal(t, "Jane Doe", reqBody["name"])
		assert.Equal(t, "jane@example.com", reqBody["email"])

		// Mock response
		response := Customer{
			ID:          "CUST2",
			Name:        "Jane Doe",
			Key:         "CUST2",
			AccountID:   "account2",
			Email:       "jane@example.com",
			DisplayName: "Jane Doe",
			Active:      true,
			Links:       CustomerLinks{Self: "https://example.com/rest/servicedeskapi/customer/CUST2"},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Logf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Create test config
	cfg := &config.Config{
		JiraBaseURL: server.URL,
		JiraEmail:   "test@example.com",
		JiraToken:   "test-token",
	}

	// Create client and JSM service
	client := NewClient(cfg)
	jsm := NewJSMService(client, slog.Default())

	// Test CreateCustomer
	customer, err := jsm.CreateCustomer(context.Background(), "Jane Doe", "jane@example.com")
	require.NoError(t, err)
	assert.Equal(t, "CUST2", customer.ID)
	assert.Equal(t, "Jane Doe", customer.Name)
	assert.Equal(t, "jane@example.com", customer.Email)
}

func TestJSMService_AttachCustomerToServiceDesk(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/servicedeskapi/servicedesk/SD1/customer", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse request body
		var reqBody map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		customer := reqBody["customer"].(map[string]interface{})
		assert.Equal(t, "CUST1", customer["accountId"])

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	// Create test config
	cfg := &config.Config{
		JiraBaseURL: server.URL,
		JiraEmail:   "test@example.com",
		JiraToken:   "test-token",
	}

	// Create client and JSM service
	client := NewClient(cfg)
	jsm := NewJSMService(client, slog.Default())

	// Test AttachCustomerToServiceDesk
	err := jsm.AttachCustomerToServiceDesk(context.Background(), "CUST1", "SD1")
	require.NoError(t, err)
}
