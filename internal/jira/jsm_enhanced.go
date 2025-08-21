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

// ServiceDesk represents a Jira Service Desk
type ServiceDesk struct {
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
	Workflow      *Workflow              `json:"workflow,omitempty"`
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

// Workflow represents a workflow for a request type
type Workflow struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Statuses    []WorkflowStatus     `json:"statuses"`
	Transitions []WorkflowTransition `json:"transitions"`
}

// WorkflowStatus represents a status in a workflow
type WorkflowStatus struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// WorkflowTransition represents a transition in a workflow
type WorkflowTransition struct {
	ID         string                `json:"id"`
	Name       string                `json:"name"`
	ToStatus   string                `json:"toStatus"`
	Conditions []TransitionCondition `json:"conditions,omitempty"`
	Validators []TransitionValidator `json:"validators,omitempty"`
}

// TransitionCondition represents a condition for a transition
type TransitionCondition struct {
	Type   string      `json:"type"`
	Params interface{} `json:"params,omitempty"`
}

// TransitionValidator represents a validator for a transition
type TransitionValidator struct {
	Type   string      `json:"type"`
	Params interface{} `json:"params,omitempty"`
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
	Groups      []CustomerGroup        `json:"groups,omitempty"`
}

// CustomerLinks represents links for a customer
type CustomerLinks struct {
	Self string `json:"self"`
}

// CustomerGroup represents a group for a customer
type CustomerGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
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
	Comments    []RequestComment       `json:"comments,omitempty"`
	Attachments []RequestAttachment    `json:"attachments,omitempty"`
	Transitions []RequestTransition    `json:"transitions,omitempty"`
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
	Metrics              []SLAMetric `json:"metrics,omitempty"`
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

// SLAMetric represents a metric for an SLA
type SLAMetric struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Active      bool   `json:"active"`
}

// GetServiceDesks retrieves all service desks accessible to the user
func (s *JSMServiceEnhanced) GetServiceDesks(ctx context.Context) ([]ServiceDesk, error) {
	url := fmt.Sprintf("%s/servicedeskapi/servicedesk", s.client.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting service desks", "url", url)

	var serviceDesks []ServiceDesk
	if err := s.client.do(req, &serviceDesks); err != nil {
		return nil, fmt.Errorf("failed to get service desks: %w", err)
	}

	s.logger.Info("Retrieved service desks", "count", len(serviceDesks))
	return serviceDesks, nil
}

// GetServiceDesk retrieves a specific service desk by ID
func (s *JSMServiceEnhanced) GetServiceDesk(ctx context.Context, serviceDeskID string) (*ServiceDesk, error) {
	url := fmt.Sprintf("%s/servicedeskapi/servicedesk/%s", s.client.baseURL, serviceDeskID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting service desk", "serviceDeskID", serviceDeskID, "url", url)

	var serviceDesk ServiceDesk
	if err := s.client.do(req, &serviceDesk); err != nil {
		return nil, fmt.Errorf("failed to get service desk: %w", err)
	}

	s.logger.Info("Retrieved service desk", "serviceDeskID", serviceDeskID, "name", serviceDesk.Name)
	return &serviceDesk, nil
}

// GetServiceDeskPermissions retrieves permissions for a service desk
func (s *JSMServiceEnhanced) GetServiceDeskPermissions(ctx context.Context, serviceDeskID string) (*ServiceDeskPermissions, error) {
	url := fmt.Sprintf("%s/servicedeskapi/servicedesk/%s/permission", s.client.baseURL, serviceDeskID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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
func (s *JSMServiceEnhanced) GetServiceDeskCustomFields(ctx context.Context, serviceDeskID string) ([]CustomField, error) {
	url := fmt.Sprintf("%s/servicedeskapi/servicedesk/%s/customField", s.client.baseURL, serviceDeskID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting service desk custom fields", "serviceDeskID", serviceDeskID, "url", url)

	var customFields []CustomField
	if err := s.client.do(req, &customFields); err != nil {
		return nil, fmt.Errorf("failed to get service desk custom fields: %w", err)
	}

	s.logger.Info("Retrieved service desk custom fields", "serviceDeskID", serviceDeskID, "count", len(customFields))
	return customFields, nil
}

// GetRequestTypes retrieves all request types for a service desk
func (s *JSMServiceEnhanced) GetRequestTypes(ctx context.Context, serviceDeskID string) ([]RequestType, error) {
	url := fmt.Sprintf("%s/servicedeskapi/servicedesk/%s/requesttype", s.client.baseURL, serviceDeskID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting request types", "serviceDeskID", serviceDeskID, "url", url)

	var requestTypes []RequestType
	if err := s.client.do(req, &requestTypes); err != nil {
		return nil, fmt.Errorf("failed to get request types: %w", err)
	}

	s.logger.Info("Retrieved request types", "serviceDeskID", serviceDeskID, "count", len(requestTypes))
	return requestTypes, nil
}

// GetRequestType retrieves a specific request type by ID
func (s *JSMServiceEnhanced) GetRequestType(ctx context.Context, serviceDeskID, requestTypeID string) (*RequestType, error) {
	url := fmt.Sprintf("%s/servicedeskapi/servicedesk/%s/requesttype/%s", s.client.baseURL, serviceDeskID, requestTypeID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting request type", "serviceDeskID", serviceDeskID, "requestTypeID", requestTypeID, "url", url)

	var requestType RequestType
	if err := s.client.do(req, &requestType); err != nil {
		return nil, fmt.Errorf("failed to get request type: %w", err)
	}

	s.logger.Info("Retrieved request type", "serviceDeskID", serviceDeskID, "requestTypeID", requestTypeID, "name", requestType.Name)
	return &requestType, nil
}

// CreateRequest creates a new service desk request
func (s *JSMServiceEnhanced) CreateRequest(ctx context.Context, serviceDeskID string, requestType string, fields map[string]interface{}) (*Request, error) {
	url := fmt.Sprintf("%s/servicedeskapi/request", s.client.baseURL)

	requestData := map[string]interface{}{
		"serviceDeskId":      serviceDeskID,
		"requestTypeId":      requestType,
		"requestFieldValues": fields,
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	s.logger.Debug("Creating service desk request", "serviceDeskID", serviceDeskID, "requestType", requestType, "url", url)

	var request Request
	if err := s.client.do(req, &request); err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Info("Created service desk request", "requestID", request.ID, "key", request.Key)
	return &request, nil
}

// GetRequest retrieves a specific service desk request by ID
func (s *JSMServiceEnhanced) GetRequest(ctx context.Context, requestID string) (*Request, error) {
	url := fmt.Sprintf("%s/servicedeskapi/request/%s", s.client.baseURL, requestID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting service desk request", "requestID", requestID, "url", url)

	var request Request
	if err := s.client.do(req, &request); err != nil {
		return nil, fmt.Errorf("failed to get request: %w", err)
	}

	s.logger.Info("Retrieved service desk request", "requestID", requestID, "key", request.Key)
	return &request, nil
}

// UpdateRequest updates a service desk request
func (s *JSMServiceEnhanced) UpdateRequest(ctx context.Context, requestID string, fields map[string]interface{}) (*Request, error) {
	url := fmt.Sprintf("%s/servicedeskapi/request/%s", s.client.baseURL, requestID)

	requestData := map[string]interface{}{
		"requestFieldValues": fields,
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	s.logger.Debug("Updating service desk request", "requestID", requestID, "url", url)

	var request Request
	if err := s.client.do(req, &request); err != nil {
		return nil, fmt.Errorf("failed to update request: %w", err)
	}

	s.logger.Info("Updated service desk request", "requestID", requestID, "key", request.Key)
	return &request, nil
}

// AddComment adds a comment to a service desk request
func (s *JSMServiceEnhanced) AddComment(ctx context.Context, requestID string, comment string, visibility *CommentVisibility) error {
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

// GetComments retrieves comments for a service desk request
func (s *JSMServiceEnhanced) GetComments(ctx context.Context, requestID string) ([]RequestComment, error) {
	url := fmt.Sprintf("%s/servicedeskapi/request/%s/comment", s.client.baseURL, requestID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting comments for service desk request", "requestID", requestID, "url", url)

	var comments []RequestComment
	if err := s.client.do(req, &comments); err != nil {
		return nil, fmt.Errorf("failed to get comments: %w", err)
	}

	s.logger.Info("Retrieved comments for service desk request", "requestID", requestID, "count", len(comments))
	return comments, nil
}

// AddAttachment adds an attachment to a service desk request
func (s *JSMServiceEnhanced) AddAttachment(ctx context.Context, requestID string, filename string, content []byte) (*RequestAttachment, error) {
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

// GetAttachments retrieves attachments for a service desk request
func (s *JSMServiceEnhanced) GetAttachments(ctx context.Context, requestID string) ([]RequestAttachment, error) {
	url := fmt.Sprintf("%s/servicedeskapi/request/%s/attachment", s.client.baseURL, requestID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting attachments for service desk request", "requestID", requestID, "url", url)

	var attachments []RequestAttachment
	if err := s.client.do(req, &attachments); err != nil {
		return nil, fmt.Errorf("failed to get attachments: %w", err)
	}

	s.logger.Info("Retrieved attachments for service desk request", "requestID", requestID, "count", len(attachments))
	return attachments, nil
}

// GetTransitions retrieves available transitions for a service desk request
func (s *JSMServiceEnhanced) GetTransitions(ctx context.Context, requestID string) ([]RequestTransition, error) {
	url := fmt.Sprintf("%s/servicedeskapi/request/%s/transitions", s.client.baseURL, requestID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting transitions for service desk request", "requestID", requestID, "url", url)

	var transitions []RequestTransition
	if err := s.client.do(req, &transitions); err != nil {
		return nil, fmt.Errorf("failed to get transitions: %w", err)
	}

	s.logger.Info("Retrieved transitions for service desk request", "requestID", requestID, "count", len(transitions))
	return transitions, nil
}

// ExecuteTransition executes a transition for a service desk request
func (s *JSMServiceEnhanced) ExecuteTransition(ctx context.Context, requestID string, transitionID string, fields map[string]interface{}) error {
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

	s.logger.Debug("Executing transition for service desk request", "requestID", requestID, "transitionID", transitionID, "url", url)

	if err := s.client.do(req, nil); err != nil {
		return fmt.Errorf("failed to execute transition: %w", err)
	}

	s.logger.Info("Executed transition for service desk request", "requestID", requestID, "transitionID", transitionID)
	return nil
}

// GetSLAs retrieves all SLAs for a service desk
func (s *JSMServiceEnhanced) GetSLAs(ctx context.Context, serviceDeskID string) ([]SLA, error) {
	url := fmt.Sprintf("%s/servicedeskapi/sla/%s", s.client.baseURL, serviceDeskID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting SLAs for service desk", "serviceDeskID", serviceDeskID, "url", url)

	var slas []SLA
	if err := s.client.do(req, &slas); err != nil {
		return nil, fmt.Errorf("failed to get SLAs: %w", err)
	}

	s.logger.Info("Retrieved SLAs for service desk", "serviceDeskID", serviceDeskID, "count", len(slas))
	return slas, nil
}

// GetSLA retrieves a specific SLA by ID
func (s *JSMServiceEnhanced) GetSLA(ctx context.Context, slaID string) (*SLA, error) {
	url := fmt.Sprintf("%s/servicedeskapi/sla/%s", s.client.baseURL, slaID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting SLA", "slaID", slaID, "url", url)

	var sla SLA
	if err := s.client.do(req, &sla); err != nil {
		return nil, fmt.Errorf("failed to get SLA: %w", err)
	}

	s.logger.Info("Retrieved SLA", "slaID", slaID, "name", sla.Name)
	return &sla, nil
}

// GetCustomers retrieves all customers for a service desk
func (s *JSMServiceEnhanced) GetCustomers(ctx context.Context, serviceDeskID string) ([]Customer, error) {
	url := fmt.Sprintf("%s/servicedeskapi/servicedesk/%s/customer", s.client.baseURL, serviceDeskID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting customers for service desk", "serviceDeskID", serviceDeskID, "url", url)

	var customers []Customer
	if err := s.client.do(req, &customers); err != nil {
		return nil, fmt.Errorf("failed to get customers: %w", err)
	}

	s.logger.Info("Retrieved customers for service desk", "serviceDeskID", serviceDeskID, "count", len(customers))
	return customers, nil
}

// GetCustomer retrieves a specific customer by ID
func (s *JSMServiceEnhanced) GetCustomer(ctx context.Context, customerID string) (*Customer, error) {
	url := fmt.Sprintf("%s/servicedeskapi/customer/%s", s.client.baseURL, customerID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting customer", "customerID", customerID, "url", url)

	var customer Customer
	if err := s.client.do(req, &customer); err != nil {
		return nil, fmt.Errorf("failed to get customer: %w", err)
	}

	s.logger.Info("Retrieved customer", "customerID", customerID, "name", customer.Name)
	return &customer, nil
}

// CreateCustomer creates a new customer
func (s *JSMServiceEnhanced) CreateCustomer(ctx context.Context, name, email string) (*Customer, error) {
	url := fmt.Sprintf("%s/servicedeskapi/customer", s.client.baseURL)

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

	s.logger.Debug("Creating customer", "name", name, "email", email, "url", url)

	var customer Customer
	if err := s.client.do(req, &customer); err != nil {
		return nil, fmt.Errorf("failed to create customer: %w", err)
	}

	s.logger.Info("Created customer", "customerID", customer.ID, "name", customer.Name)
	return &customer, nil
}

// AttachCustomerToServiceDesk attaches a customer to a service desk
func (s *JSMServiceEnhanced) AttachCustomerToServiceDesk(ctx context.Context, customerID, serviceDeskID string) error {
	url := fmt.Sprintf("%s/servicedeskapi/servicedesk/%s/customer", s.client.baseURL, serviceDeskID)

	customerData := map[string]interface{}{
		"customer": map[string]string{
			"accountId": customerID,
		},
	}

	jsonData, err := json.Marshal(customerData)
	if err != nil {
		return fmt.Errorf("failed to marshal customer data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	s.logger.Debug("Attaching customer to service desk", "customerID", customerID, "serviceDeskID", serviceDeskID, "url", url)

	if err := s.client.do(req, nil); err != nil {
		return fmt.Errorf("failed to attach customer to service desk: %w", err)
	}

	s.logger.Info("Attached customer to service desk", "customerID", customerID, "serviceDeskID", serviceDeskID)
	return nil
}

// GetCustomerGroups retrieves groups for a customer
func (s *JSMServiceEnhanced) GetCustomerGroups(ctx context.Context, customerID string) ([]CustomerGroup, error) {
	url := fmt.Sprintf("%s/servicedeskapi/customer/%s/group", s.client.baseURL, customerID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.logger.Debug("Getting customer groups", "customerID", customerID, "url", url)

	var groups []CustomerGroup
	if err := s.client.do(req, &groups); err != nil {
		return nil, fmt.Errorf("failed to get customer groups: %w", err)
	}

	s.logger.Info("Retrieved customer groups", "customerID", customerID, "count", len(groups))
	return groups, nil
}

// AddCustomerToGroup adds a customer to a group
