// Package jira provides enhanced Jira Service Management (JSM) functionality
package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// JSMServiceEnhanced provides enhanced Jira Service Management functionality
type JSMServiceEnhanced struct {
	client *Client
	logger *slog.Logger
}

// NewJSMServiceEnhanced creates a new enhanced JSM service
func NewJSMServiceEnhanced(client *Client, logger *slog.Logger) *JSMServiceEnhanced {
	return &JSMServiceEnhanced{
		client: client,
		logger: logger,
	}
}

// ServiceDeskEnhanced represents an enhanced Jira Service Desk
type ServiceDeskEnhanced struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Key          string                 `json:"key"`
	ProjectID    int                    `json:"projectId"`
	ProjectName  string                 `json:"projectName"`
	ProjectKey   string                 `json:"projectKey"`
	Description  string                 `json:"description"`
	Website      string                 `json:"website"`
	AvatarURL    string                 `json:"avatarUrl"`
	ProjectType  string                 `json:"projectType"`
	CreatedDate  time.Time              `json:"createdDate"`
	UpdatedDate  time.Time              `json:"updatedDate"`
	CustomFields []CustomField          `json:"customFields,omitempty"`
	Permissions  ServiceDeskPermissions `json:"permissions,omitempty"`
}

// ServiceDeskPermissions represents permissions for a service desk
type ServiceDeskPermissions struct {
	RequestTypeCreate bool `json:"requestTypeCreate"`
	RequestTypeRead   bool `json:"requestTypeRead"`
	RequestTypeUpdate bool `json:"requestTypeUpdate"`
	RequestTypeDelete bool `json:"requestTypeDelete"`
	CustomerRead      bool `json:"customerRead"`
	CustomerCreate    bool `json:"customerCreate"`
	CustomerUpdate    bool `json:"customerUpdate"`
	CustomerDelete    bool `json:"customerDelete"`
}

// CustomField represents a custom field in Jira
type CustomField struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Required bool        `json:"required"`
	Order    int         `json:"order"`
	Options  []string    `json:"options,omitempty"`
	Schema   interface{} `json:"schema,omitempty"`
}

// RequestTypeEnhanced represents an enhanced Jira Service Desk request type
type RequestTypeEnhanced struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	HelpText      string                 `json:"helpText"`
	IconURL       string                 `json:"iconUrl"`
	GroupID       string                 `json:"groupId"`
	ServiceDeskID string                 `json:"serviceDeskId"`
	Fields        []RequestTypeField     `json:"requestTypeFields"`
	Properties    map[string]interface{} `json:"properties"`
	Workflow      *Workflow              `json:"workflow,omitempty"`
}

// CustomerEnhanced represents an enhanced Jira Service Desk customer
type CustomerEnhanced struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Key         string                 `json:"key"`
	AccountID   string                 `json:"accountId"`
	Email       string                 `json:"email"`
	DisplayName string                 `json:"displayName"`
	Active      bool                   `json:"active"`
	Links       CustomerLinks          `json:"_links"`
	Properties  map[string]interface{} `json:"properties"`
	Groups      []CustomerGroup        `json:"groups,omitempty"`
}

// RequestEnhanced represents an enhanced service desk request
type RequestEnhanced struct {
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
	ServiceDesk ServiceDeskEnhanced    `json:"serviceDesk"`
	RequestType RequestTypeEnhanced    `json:"requestType"`
	Comments    []RequestComment       `json:"comments,omitempty"`
	Attachments []RequestAttachment    `json:"attachments,omitempty"`
	Transitions []RequestTransition    `json:"transitions,omitempty"`
}

// RequestComment represents a comment on a service desk request
type RequestComment struct {
	ID          string                 `json:"id"`
	Author      RequestUser            `json:"author"`
	Body        string                 `json:"body"`
	CreatedDate time.Time              `json:"createdDate"`
	UpdatedDate time.Time              `json:"updatedDate"`
	Visibility  *CommentVisibility     `json:"visibility,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
}

// CommentVisibility represents visibility settings for a comment
type CommentVisibility struct {
	Type  string `json:"type"`
	Value string `json:"value,omitempty"`
}

// RequestAttachment represents an attachment on a service desk request
type RequestAttachment struct {
	ID          string                 `json:"id"`
	Filename    string                 `json:"filename"`
	Author      RequestUser            `json:"author"`
	CreatedDate time.Time              `json:"createdDate"`
	Size        int64                  `json:"size"`
	MimeType    string                 `json:"mimeType"`
	Content     []byte                 `json:"content,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
}

// RequestTransition represents a transition for a service desk request
type RequestTransition struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	ToStatus   string                 `json:"toStatus"`
	Fields     map[string]interface{} `json:"fields,omitempty"`
	Conditions []TransitionCondition  `json:"conditions,omitempty"`
	Validators []TransitionValidator  `json:"validators,omitempty"`
}

// SLAEnhanced represents an enhanced Service Level Agreement
type SLAEnhanced struct {
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
	Metrics              []SLAMetric `json:"metrics,omitempty"`
}

// SLAMetric represents a metric for an SLA
type SLAMetric struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Active      bool   `json:"active"`
}

// GetServiceDesks retrieves all service desks accessible to the user

// GetServiceDesk retrieves a specific service desk by ID

// GetServiceDeskPermissions retrieves permissions for a service desk
func (s *JSMServiceEnhanced) GetServiceDeskPermissions(
	ctx context.Context, serviceDeskID string,
) (*ServiceDeskPermissions, error) {
	url := fmt.Sprintf("%s/servicedeskapi/servicedesk/%s/permission", s.client.baseURL, serviceDeskID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting service desk permissions", "serviceDeskID", serviceDeskID, "url", url)

	var permissions ServiceDeskPermissions
	if err := s.client.do(req, &permissions); err != nil {
		return nil, fmt.Errorf("failed to get service desk permissions: %w", err)
	}

	s.logger.Info("Retrieved service desk permissions", "serviceDeskID", serviceDeskID)
	return &permissions, nil
}

// GetServiceDeskCustomFields retrieves custom fields for a service desk

// AddComment adds a comment to a service desk request
func (s *JSMServiceEnhanced) AddComment(
	ctx context.Context, requestID, comment string, visibility *CommentVisibility,
) error {
	url := fmt.Sprintf("%s/servicedeskapi/request/%s/comment", s.client.baseURL, requestID)

	commentData := map[string]interface{}{
		"body": comment,
	}

	if visibility != nil {
		commentData["visibility"] = visibility
	}

	jsonData, err := json.Marshal(commentData)
	if err != nil {
		return fmt.Errorf("failed to marshal comment data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	s.logger.Debug("Adding comment to service desk request", "requestID", requestID, "url", url)

	if err := s.client.do(req, nil); err != nil {
		return fmt.Errorf("failed to add comment: %w", err)
	}

	s.logger.Info("Added comment to service desk request", "requestID", requestID)
	return nil
}

// AddAttachment adds an attachment to a service desk request
func (s *JSMServiceEnhanced) AddAttachment(
	ctx context.Context, requestID, filename string, content []byte,
) (*RequestAttachment, error) {
	url := fmt.Sprintf("%s/servicedeskapi/request/%s/attachment", s.client.baseURL, requestID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "multipart/form-data")
	req.Header.Set("X-Atlassian-Token", "no-check")

	s.logger.Debug("Adding attachment to service desk request", "requestID", requestID, "filename", filename, "url", url)

	var attachment RequestAttachment
	if err := s.client.do(req, &attachment); err != nil {
		return nil, fmt.Errorf("failed to add attachment: %w", err)
	}

	s.logger.Info("Added attachment to service desk request", "requestID", requestID, "attachmentID", attachment.ID)
	return &attachment, nil
}

// ExecuteTransition executes a transition for a service desk request
func (s *JSMServiceEnhanced) ExecuteTransition(
	ctx context.Context, requestID, transitionID string, fields map[string]interface{},
) error {
	url := fmt.Sprintf("%s/servicedeskapi/request/%s/transitions", s.client.baseURL, requestID)

	payload := map[string]interface{}{
		"transitionId": transitionID,
	}

	if fields != nil {
		payload["fields"] = fields
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal transition payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	s.logger.Debug("Executing transition for service desk request",
		"requestID", requestID, "transitionID", transitionID, "url", url)

	if err := s.client.do(req, nil); err != nil {
		return fmt.Errorf("failed to execute transition: %w", err)
	}

	s.logger.Info("Executed transition for service desk request", "requestID", requestID, "transitionID", transitionID)
	return nil
}

// GetCustomer retrieves a specific customer by ID
func (s *JSMServiceEnhanced) GetCustomer(ctx context.Context, customerID string) (*CustomerEnhanced, error) {
	url := fmt.Sprintf("%s/servicedeskapi/customer/%s", s.client.baseURL, customerID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting customer", "customerID", customerID, "url", url)

	var customer CustomerEnhanced
	if err := s.client.do(req, &customer); err != nil {
		return nil, fmt.Errorf("failed to get customer: %w", err)
	}

	s.logger.Info("Retrieved customer", "customerID", customerID, "name", customer.Name)
	return &customer, nil
}

// CreateCustomer creates a new customer (enhanced version)
func (s *JSMServiceEnhanced) CreateCustomer(ctx context.Context, name, email string) (*CustomerEnhanced, error) {
	// Enhanced version of CreateCustomer with additional validation and logging
	url := fmt.Sprintf("%s/servicedeskapi/customer", s.client.baseURL)

	// Validate input parameters
	if name == "" || email == "" {
		return nil, fmt.Errorf("name and email are required")
	}

	customerData := map[string]interface{}{
		"name":  name,
		"email": email,
	}

	jsonData, err := json.Marshal(customerData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal customer data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	s.logger.Debug("Creating enhanced customer", "name", name, "email", email, "url", url)

	var customer CustomerEnhanced
	if err := s.client.do(req, &customer); err != nil {
		return nil, fmt.Errorf("failed to create enhanced customer: %w", err)
	}

	s.logger.Info("Created enhanced customer", "customerID", customer.ID, "name", customer.Name)
	return &customer, nil
}

// AttachCustomerToServiceDesk attaches a customer to a service desk (enhanced version)
func (s *JSMServiceEnhanced) AttachCustomerToServiceDesk(ctx context.Context, customerID, serviceDeskID string) error {
	// Enhanced version with additional validation and logging
	if customerID == "" || serviceDeskID == "" {
		return fmt.Errorf("customerID and serviceDeskID are required")
	}

	// Use the shared function for the core logic
	err := attachCustomerToServiceDesk(ctx, s.client, s.logger, customerID, serviceDeskID)
	if err != nil {
		return fmt.Errorf("failed to attach customer to service desk in enhanced version: %w", err)
	}

	s.logger.Info("Enhanced customer attached to service desk", "customerID", customerID, "serviceDeskID", serviceDeskID)
	return nil
}

// GetCustomerGroups retrieves groups for a customer
func (s *JSMServiceEnhanced) GetCustomerGroups(ctx context.Context, customerID string) ([]CustomerGroup, error) {
	// Enhanced version of GetCustomerGroups with additional validation and logging
	url := fmt.Sprintf("%s/servicedeskapi/customer/%s/group", s.client.baseURL, customerID)

	// Validate input parameters
	if customerID == "" {
		return nil, fmt.Errorf("customerID is required")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting enhanced customer groups", "customerID", customerID, "url", url)

	var groups []CustomerGroup
	if err := s.client.do(req, &groups); err != nil {
		return nil, fmt.Errorf("failed to get enhanced customer groups: %w", err)
	}

	s.logger.Info("Retrieved enhanced customer groups", "customerID", customerID, "count", len(groups))
	return groups, nil
}

// AddCustomerToGroup adds a customer to a group
