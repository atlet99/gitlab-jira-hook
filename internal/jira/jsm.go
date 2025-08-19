// Package jira provides Jira Service Management (JSM) integration capabilities.
package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"log/slog"
)

// JSMService represents the Jira Service Management service
type JSMService struct {
	client *Client
	logger *slog.Logger
}

// NewJSMService creates a new JSM service instance
func NewJSMService(client *Client, logger *slog.Logger) *JSMService {
	return &JSMService{
		client: client,
		logger: logger,
	}
}

// getSingleResource makes a GET request to get a single resource by ID
func (s *JSMService) getSingleResource(ctx context.Context, endpoint, id string, resourceType string) (interface{}, error) {
	apiURL := fmt.Sprintf("%s/servicedeskapi/%s/%s", s.client.baseURL, endpoint, id)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting "+resourceType, "id", id, "url", apiURL)

	var result interface{}
	if err := s.client.do(req, &result); err != nil {
		return nil, fmt.Errorf("failed to get %s: %w", resourceType, err)
	}

	s.logger.Info("Retrieved "+resourceType, "id", id)
	return result, nil
}

// getMultipleResources makes a GET request to get multiple resources
func (s *JSMService) getMultipleResources(ctx context.Context, endpoint string, resourceType string) ([]interface{}, error) {
	apiURL := fmt.Sprintf("%s/servicedeskapi/%s", s.client.baseURL, endpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting "+resourceType+"s", "url", apiURL)

	var results []interface{}
	if err := s.client.do(req, &results); err != nil {
		return nil, fmt.Errorf("failed to get %ss: %w", resourceType, err)
	}

	s.logger.Info("Retrieved "+resourceType+"s", "count", len(results))
	return results, nil
}

// ServiceDesk represents a Jira Service Desk
type ServiceDesk struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Key         string    `json:"key"`
	ProjectID   int       `json:"projectId"`
	ProjectName string    `json:"projectName"`
	ProjectKey  string    `json:"projectKey"`
	Description string    `json:"description"`
	Website     string    `json:"website"`
	AvatarURL   string    `json:"avatarUrl"`
	ProjectType string    `json:"projectType"`
	CreatedDate time.Time `json:"createdDate"`
	UpdatedDate time.Time `json:"updatedDate"`
}

// RequestType represents a Jira Service Desk request type
type RequestType struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	HelpText      string                 `json:"helpText"`
	IconURL       string                 `json:"iconUrl"`
	GroupID       string                 `json:"groupId"`
	ServiceDeskID string                 `json:"serviceDeskId"`
	Fields        []RequestTypeField     `json:"requestTypeFields"`
	Properties    map[string]interface{} `json:"properties"`
}

// RequestTypeField represents a field in a request type
type RequestTypeField struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Type          string                 `json:"type"`
	Required      bool                   `json:"required"`
	Orderable     bool                   `json:"orderable"`
	Visible       bool                   `json:"visible"`
	Description   string                 `json:"description"`
	AllowedValues []string               `json:"allowedValues,omitempty"`
	Schema        map[string]interface{} `json:"schema"`
}

// Customer represents a Jira Service Desk customer
type Customer struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Key         string                 `json:"key"`
	AccountID   string                 `json:"accountId"`
	Email       string                 `json:"email"`
	DisplayName string                 `json:"displayName"`
	Active      bool                   `json:"active"`
	Links       CustomerLinks          `json:"_links"`
	Properties  map[string]interface{} `json:"properties"`
}

// CustomerLinks represents links for a customer
type CustomerLinks struct {
	Self string `json:"self"`
}

// SLA represents a Service Level Agreement
type SLA struct {
	ID                   string      `json:"id"`
	Name                 string      `json:"name"`
	GroupID              string      `json:"groupId"`
	ServiceDeskID        string      `json:"serviceDeskId"`
	Active               bool        `json:"active"`
	IsDefault            bool        `json:"isDefault"`
	NotificationSchemeID string      `json:"notificationSchemeId"`
	Schedule             SLASchedule `json:"schedule"`
	Target               SLATarget   `json:"target"`
	Warning              SLAWarning  `json:"warning"`
}

// SLASchedule represents the schedule for an SLA
type SLASchedule struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Active      bool       `json:"active"`
	IsDefault   bool       `json:"isDefault"`
	Entries     []SLAEntry `json:"entries"`
}

// SLAEntry represents an entry in an SLA schedule
type SLAEntry struct {
	ID        string `json:"id"`
	DayOfWeek int    `json:"dayOfWeek"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

// SLATarget represents the target for an SLA
type SLATarget struct {
	Type              string `json:"type"`
	Value             int    `json:"value"`
	Unit              string `json:"unit"`
	BusinessHoursOnly bool   `json:"businessHoursOnly"`
}

// SLAWarning represents warning settings for an SLA
type SLAWarning struct {
	Type              string `json:"type"`
	Value             int    `json:"value"`
	Unit              string `json:"unit"`
	BusinessHoursOnly bool   `json:"businessHoursOnly"`
}

// Request represents a service desk request
type Request struct {
	ID          string                 `json:"id"`
	Key         string                 `json:"key"`
	ProjectID   int                    `json:"projectId"`
	ProjectKey  string                 `json:"projectKey"`
	IssueType   string                 `json:"issueType"`
	Fields      map[string]interface{} `json:"fields"`
	Status      RequestStatus          `json:"status"`
	Priority    RequestPriority        `json:"priority"`
	CreatedDate time.Time              `json:"createdDate"`
	UpdatedDate time.Time              `json:"updatedDate"`
	Reporter    RequestUser            `json:"reporter"`
	Assignee    RequestUser            `json:"assignee,omitempty"`
	ServiceDesk ServiceDesk            `json:"serviceDesk"`
	RequestType RequestType            `json:"requestType"`
}

// RequestStatus represents the status of a service desk request
type RequestStatus struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	StatusCategory string `json:"statusCategory"`
}

// RequestPriority represents the priority of a service desk request
type RequestPriority struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IconURL string `json:"iconUrl"`
}

// RequestUser represents a user in the context of service desk requests
type RequestUser struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Active      bool   `json:"active"`
}

// GetServiceDesks retrieves all service desks accessible to the user
func (s *JSMService) GetServiceDesks(ctx context.Context) ([]ServiceDesk, error) {
	apiURL := fmt.Sprintf("%s/servicedeskapi/servicedesk", s.client.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting service desks", "url", apiURL)

	var serviceDesks []ServiceDesk
	if err := s.client.do(req, &serviceDesks); err != nil {
		return nil, fmt.Errorf("failed to get service desks: %w", err)
	}

	s.logger.Info("Retrieved service desks", "count", len(serviceDesks))
	return serviceDesks, nil
}

// GetServiceDesk retrieves a specific service desk by ID
func (s *JSMService) GetServiceDesk(ctx context.Context, serviceDeskID string) (*ServiceDesk, error) {
	result, err := s.getSingleResource(ctx, "servicedesk", serviceDeskID, "service desk")
	if err != nil {
		return nil, err
	}

	if serviceDesk, ok := result.(*ServiceDesk); ok {
		s.logger.Info("Retrieved service desk", "serviceDeskID", serviceDeskID, "name", serviceDesk.Name)
		return serviceDesk, nil
	}
	return nil, fmt.Errorf("failed to parse service desk response")
}

// GetRequestTypes retrieves all request types for a service desk
func (s *JSMService) GetRequestTypes(ctx context.Context, serviceDeskID string) ([]RequestType, error) {
	apiURL := fmt.Sprintf("%s/servicedeskapi/servicedesk/%s/requesttype", s.client.baseURL, serviceDeskID)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting request types", "serviceDeskID", serviceDeskID, "url", apiURL)

	var requestTypes []RequestType
	if err := s.client.do(req, &requestTypes); err != nil {
		return nil, fmt.Errorf("failed to get request types: %w", err)
	}

	s.logger.Info("Retrieved request types", "serviceDeskID", serviceDeskID, "count", len(requestTypes))
	return requestTypes, nil
}

// GetRequestType retrieves a specific request type by ID
func (s *JSMService) GetRequestType(ctx context.Context, serviceDeskID, requestTypeID string) (*RequestType, error) {
	apiURL := fmt.Sprintf("%s/servicedeskapi/servicedesk/%s/requesttype/%s", s.client.baseURL, serviceDeskID, requestTypeID)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting request type", "serviceDeskID", serviceDeskID, "requestTypeID", requestTypeID, "url", apiURL)

	var requestType RequestType
	if err := s.client.do(req, &requestType); err != nil {
		return nil, fmt.Errorf("failed to get request type: %w", err)
	}

	s.logger.Info("Retrieved request type", "serviceDeskID", serviceDeskID, "requestTypeID", requestTypeID, "name", requestType.Name)
	return &requestType, nil
}

// CreateRequest creates a new service desk request
func (s *JSMService) CreateRequest(ctx context.Context, serviceDeskID string, requestType string, fields map[string]interface{}) (*Request, error) {
	apiURL := fmt.Sprintf("%s/servicedeskapi/request", s.client.baseURL)

	requestData := map[string]interface{}{
		"serviceDeskId":      serviceDeskID,
		"requestTypeId":      requestType,
		"requestFieldValues": fields,
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	s.logger.Debug("Creating service desk request", "serviceDeskID", serviceDeskID, "requestType", requestType, "url", apiURL)

	var request Request
	if err := s.client.do(req, &request); err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Info("Created service desk request", "requestID", request.ID, "key", request.Key)
	return &request, nil
}

// GetRequest retrieves a specific service desk request by ID
func (s *JSMService) GetRequest(ctx context.Context, requestID string) (*Request, error) {
	result, err := s.getSingleResource(ctx, "request", requestID, "service desk request")
	if err != nil {
		return nil, err
	}

	if request, ok := result.(*Request); ok {
		s.logger.Info("Retrieved service desk request", "requestID", requestID, "key", request.Key)
		return request, nil
	}
	return nil, fmt.Errorf("failed to parse request response")
}

// UpdateRequest updates a service desk request
func (s *JSMService) UpdateRequest(ctx context.Context, requestID string, fields map[string]interface{}) (*Request, error) {
	apiURL := fmt.Sprintf("%s/servicedeskapi/request/%s", s.client.baseURL, requestID)

	requestData := map[string]interface{}{
		"requestFieldValues": fields,
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	s.logger.Debug("Updating service desk request", "requestID", requestID, "url", apiURL)

	var request Request
	if err := s.client.do(req, &request); err != nil {
		return nil, fmt.Errorf("failed to update request: %w", err)
	}

	s.logger.Info("Updated service desk request", "requestID", requestID, "key", request.Key)
	return &request, nil
}

// AddComment adds a comment to a service desk request
func (s *JSMService) AddComment(ctx context.Context, requestID string, comment string) error {
	apiURL := fmt.Sprintf("%s/servicedeskapi/request/%s/comment", s.client.baseURL, requestID)

	commentData := map[string]interface{}{
		"body": comment,
	}

	jsonData, err := json.Marshal(commentData)
	if err != nil {
		return fmt.Errorf("failed to marshal comment data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	s.logger.Debug("Adding comment to service desk request", "requestID", requestID, "url", apiURL)

	if err := s.client.do(req, nil); err != nil {
		return fmt.Errorf("failed to add comment: %w", err)
	}

	s.logger.Info("Added comment to service desk request", "requestID", requestID)
	return nil
}

// GetSLAs retrieves all SLAs for a service desk
func (s *JSMService) GetSLAs(ctx context.Context, serviceDeskID string) ([]SLA, error) {
	results, err := s.getMultipleResources(ctx, "sla", "SLA")
	if err != nil {
		return nil, err
	}

	var slas []SLA
	for _, result := range results {
		if sla, ok := result.(*SLA); ok {
			slas = append(slas, *sla)
		}
	}

	s.logger.Info("Retrieved SLAs", "serviceDeskID", serviceDeskID, "count", len(slas))
	return slas, nil
}

// GetSLA retrieves a specific SLA by ID
func (s *JSMService) GetSLA(ctx context.Context, slaID string) (*SLA, error) {
	result, err := s.getSingleResource(ctx, "sla", slaID, "SLA")
	if err != nil {
		return nil, err
	}

	if sla, ok := result.(*SLA); ok {
		s.logger.Info("Retrieved SLA", "slaID", slaID, "name", sla.Name)
		return sla, nil
	}
	return nil, fmt.Errorf("failed to parse SLA response")
}

// GetCustomers retrieves all customers for a service desk
func (s *JSMService) GetCustomers(ctx context.Context, serviceDeskID string) ([]Customer, error) {
	results, err := s.getMultipleResources(ctx, "customer", "customer")
	if err != nil {
		return nil, err
	}

	var customers []Customer
	for _, result := range results {
		if customer, ok := result.(*Customer); ok {
			customers = append(customers, *customer)
		}
	}

	s.logger.Info("Retrieved customers", "serviceDeskID", serviceDeskID, "count", len(customers))
	return customers, nil
}

// GetCustomer retrieves a specific customer by ID
func (s *JSMService) GetCustomer(ctx context.Context, customerID string) (*Customer, error) {
	result, err := s.getSingleResource(ctx, "customer", customerID, "customer")
	if err != nil {
		return nil, err
	}

	if customer, ok := result.(*Customer); ok {
		s.logger.Info("Retrieved customer", "customerID", customerID, "name", customer.Name)
		return customer, nil
	}
	return nil, fmt.Errorf("failed to parse customer response")
}

// CreateCustomer creates a new customer
func (s *JSMService) CreateCustomer(ctx context.Context, name, email string) (*Customer, error) {
	apiURL := fmt.Sprintf("%s/servicedeskapi/customer", s.client.baseURL)

	customerData := map[string]interface{}{
		"name":  name,
		"email": email,
	}

	jsonData, err := json.Marshal(customerData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal customer data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	s.logger.Debug("Creating customer", "name", name, "email", email, "url", apiURL)

	var customer Customer
	if err := s.client.do(req, &customer); err != nil {
		return nil, fmt.Errorf("failed to create customer: %w", err)
	}

	s.logger.Info("Created customer", "customerID", customer.ID, "name", customer.Name)
	return &customer, nil
}

// AttachCustomerToServiceDesk attaches a customer to a service desk
func (s *JSMService) AttachCustomerToServiceDesk(ctx context.Context, customerID, serviceDeskID string) error {
	apiURL := fmt.Sprintf("%s/servicedeskapi/servicedesk/%s/customer", s.client.baseURL, serviceDeskID)

	customerData := map[string]interface{}{
		"customer": map[string]string{
			"accountId": customerID,
		},
	}

	jsonData, err := json.Marshal(customerData)
	if err != nil {
		return fmt.Errorf("failed to marshal customer data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	s.logger.Debug("Attaching customer to service desk", "customerID", customerID, "serviceDeskID", serviceDeskID, "url", apiURL)

	if err := s.client.do(req, nil); err != nil {
		return fmt.Errorf("failed to attach customer to service desk: %w", err)
	}

	s.logger.Info("Attached customer to service desk", "customerID", customerID, "serviceDeskID", serviceDeskID)
	return nil
}
