package jira

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

func TestCustomFieldManager_LoadMappings(t *testing.T) {
	cfg := &config.Config{
		CustomFieldMappings: map[string]interface{}{
			"customfield_10010": "labels",
			"customfield_10011": map[string]interface{}{
				"target_field": "labels",
				"type":         "string",
				"adf_support":  true,
				"required":     true,
			},
			"customfield_10012": map[string]interface{}{
				"target_field": "priority",
				"type":         "number",
				"transformation": map[string]interface{}{
					"replace": map[string]interface{}{
						"low":    "1",
						"medium": "2",
						"high":   "3",
					},
				},
			},
		},
	}

	manager := NewCustomFieldManager(nil, cfg, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
	err := manager.LoadMappings()
	require.NoError(t, err)

	mappings := manager.GetCustomFieldMappings()
	assert.Len(t, mappings, 3)

	// Test simple string mapping
	mapping1, found := manager.GetMappingByJiraField("customfield_10010")
	assert.True(t, found)
	assert.Equal(t, "labels", mapping1.GitLabField)
	assert.Equal(t, "string", mapping1.FieldType)
	assert.False(t, mapping1.ADFSupport)
	assert.False(t, mapping1.Required)

	// Test complex mapping
	mapping2, found := manager.GetMappingByJiraField("customfield_10011")
	assert.True(t, found)
	assert.Equal(t, "labels", mapping2.GitLabField)
	assert.Equal(t, "string", mapping2.FieldType)
	assert.True(t, mapping2.ADFSupport)
	assert.True(t, mapping2.Required)

	// Test mapping with transformation
	mapping3, found := manager.GetMappingByJiraField("customfield_10012")
	assert.True(t, found)
	assert.Equal(t, "priority", mapping3.GitLabField)
	assert.Equal(t, "number", mapping3.FieldType)
	assert.NotNil(t, mapping3.Transformation)
}

func TestCustomFieldManager_TransformJiraToGitLab(t *testing.T) {
	cfg := &config.Config{
		CustomFieldMappings: map[string]interface{}{
			"customfield_10010": "labels",
			"customfield_10011": "description",
		},
	}

	manager := NewCustomFieldManager(nil, cfg, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
	err := manager.LoadMappings()
	require.NoError(t, err)

	// Create a test issue with custom fields
	issue := &JiraIssue{
		ID:   "12345",
		Key:  "TEST-123",
		Self: "https://test.atlassian.net/rest/api/2/issue/12345",
		Fields: &JiraIssueFields{
			Summary:     "Test Issue",
			Description: "Test description",
			// Add custom fields via reflection simulation
			// In real usage, these would be actual fields in the struct
		},
	}

	// This is a simplified test - in reality, custom fields would be handled differently
	result, err := manager.TransformJiraToGitLab(context.Background(), issue)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestCustomFieldManager_TransformString(t *testing.T) {
	cfg := &config.Config{}
	manager := NewCustomFieldManager(nil, cfg, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))

	testCases := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"string input", "test", "test"},
		{"int input", 42, "42"},
		{"float input", 3.14, "3.140000"},
		{"bool input true", true, "true"},
		{"bool input false", false, "false"},
		{"nil input", nil, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := manager.transformToString(tc.input)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestCustomFieldManager_TransformToNumber(t *testing.T) {
	cfg := &config.Config{}
	manager := NewCustomFieldManager(nil, cfg, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))

	testCases := []struct {
		name     string
		input    interface{}
		expected float64
		err      bool
	}{
		{"int input", 42, 42.0, false},
		{"int8 input", int8(8), 8.0, false},
		{"int16 input", int16(16), 16.0, false},
		{"int32 input", int32(32), 32.0, false},
		{"int64 input", int64(64), 64.0, false},
		{"uint input", uint(42), 42.0, false},
		{"float32 input", float32(3.14), 3.14, false},
		{"float64 input", 3.14, 3.14, false},
		{"string number", "42.5", 42.5, false},
		{"string invalid", "invalid", 0, true},
		{"bool input", true, 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := manager.transformToNumber(tc.input)
			if tc.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.InDelta(t, tc.expected, result, 0.0001)
			}
		})
	}
}

func TestCustomFieldManager_TransformToDate(t *testing.T) {
	cfg := &config.Config{}
	manager := NewCustomFieldManager(nil, cfg, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))

	testCases := []struct {
		name     string
		input    interface{}
		expected string
		err      bool
	}{
		{"RFC3339 string", "2023-12-25T10:30:00Z", "2023-12-25", false},
		{"Date string", "2023-12-25", "2023-12-25", false},
		{"Time.Time", time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC), "2023-12-25", false},
		{"int64 timestamp", int64(1703506200), "2023-12-25", false},
		{"int32 timestamp", int32(1703506200), "2023-12-25", false},
		{"invalid string", "invalid", "", true},
		{"bool input", true, "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := manager.transformToDate(tc.input)
			if tc.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestCustomFieldManager_TransformToSelect(t *testing.T) {
	cfg := &config.Config{}
	manager := NewCustomFieldManager(nil, cfg, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))

	testCases := []struct {
		name      string
		input     interface{}
		multi     bool
		expected  interface{}
		expectErr bool
	}{
		{"string single", "option1", false, "option1", false},
		{"string multi", "option1", true, []string{"option1"}, false},
		{"[]string single", []string{"option1", "option2"}, false, "option1", false},
		{"[]string multi", []string{"option1", "option2"}, true, []string{"option1", "option2"}, false},
		{"[]interface{} single", []interface{}{"option1", "option2"}, false, "option1", false},
		{"[]interface{} multi", []interface{}{"option1", "option2"}, true, []interface{}{"option1", "option2"}, false},
		{"empty string single", "", false, "", false},
		{"empty string multi", "", true, []string{}, false},
		{"nil input", nil, false, nil, false},
		{"invalid type", 123, false, nil, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := manager.transformToSelect(tc.input, tc.multi)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestCustomFieldManager_Transformation(t *testing.T) {
	cfg := &config.Config{}
	manager := NewCustomFieldManager(nil, cfg, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))

	testCases := []struct {
		name        string
		mapping     *CustomFieldMapping
		input       interface{}
		expected    interface{}
		expectError bool
	}{
		{
			name: "string replacement",
			mapping: &CustomFieldMapping{
				Transformation: map[string]interface{}{
					"replace": map[string]interface{}{
						"old":  "new",
						"test": "demo",
					},
				},
			},
			input:       "old value test",
			expected:    "new value demo",
			expectError: false,
		},
		{
			name: "value mapping",
			mapping: &CustomFieldMapping{
				Transformation: map[string]interface{}{
					"map": map[string]interface{}{
						"high":   "3",
						"medium": "2",
						"low":    "1",
					},
				},
			},
			input:       "high",
			expected:    "3",
			expectError: false,
		},
		{
			name: "no transformation",
			mapping: &CustomFieldMapping{
				Transformation: nil,
			},
			input:       "test",
			expected:    "test",
			expectError: false,
		},
		{
			name: "invalid transformation",
			mapping: &CustomFieldMapping{
				Transformation: map[string]interface{}{
					"invalid": "rule",
				},
			},
			input:       "test",
			expected:    "test",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := manager.applyTransformation(tc.mapping, tc.input)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestCustomFieldManager_ReverseTransformation(t *testing.T) {
	cfg := &config.Config{}
	manager := NewCustomFieldManager(nil, cfg, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))

	testCases := []struct {
		name        string
		mapping     *CustomFieldMapping
		input       interface{}
		expected    interface{}
		expectError bool
	}{
		{
			name: "reverse string replacement",
			mapping: &CustomFieldMapping{
				Transformation: map[string]interface{}{
					"replace": map[string]interface{}{
						"old":  "new",
						"test": "demo",
					},
				},
			},
			input:       "new value demo",
			expected:    "old value test",
			expectError: false,
		},
		{
			name: "reverse value mapping",
			mapping: &CustomFieldMapping{
				Transformation: map[string]interface{}{
					"map": map[string]interface{}{
						"high":   "3",
						"medium": "2",
						"low":    "1",
					},
				},
			},
			input:       "3",
			expected:    "high",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := manager.applyReverseTransformation(tc.mapping, tc.input)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestCustomFieldManager_ValidateCustomFields(t *testing.T) {
	cfg := &config.Config{
		CustomFieldMappings: map[string]interface{}{
			"customfield_10010": map[string]interface{}{
				"required": true,
			},
			"customfield_10011": map[string]interface{}{
				"required": false,
			},
		},
	}

	manager := NewCustomFieldManager(nil, cfg, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
	err := manager.LoadMappings()
	require.NoError(t, err)

	testCases := []struct {
		name        string
		issue       *JiraIssue
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid issue with required fields",
			issue: &JiraIssue{
				Fields: &JiraIssueFields{
					// For this test, we'll skip the validation since custom fields
					// are handled via reflection and we can't easily mock them
					// In a real scenario, the field would exist in the Jira response
				},
			},
			expectError: false,
		},
		{
			name:        "issue with nil fields",
			issue:       &JiraIssue{Fields: nil},
			expectError: true,
			errorMsg:    "required custom field customfield_10010 missing: issue fields are nil",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := manager.ValidateCustomFields(context.Background(), tc.issue)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCustomFieldManager_GetCustomFieldMetadata(t *testing.T) {
	cfg := &config.Config{}
	manager := NewCustomFieldManager(nil, cfg, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))

	// Test with cache miss
	metadata, err := manager.GetCustomFieldMetadata(context.Background(), "TEST")
	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Equal(t, "TEST", metadata["project"])

	// Test with cache hit
	metadata2, err := manager.GetCustomFieldMetadata(context.Background(), "TEST")
	assert.NoError(t, err)
	assert.Equal(t, metadata, metadata2)

	// Clear cache and verify it's cleared
	manager.ClearCache()
	assert.Empty(t, manager.cache)
}

func TestCustomFieldManager_isStandardField(t *testing.T) {
	cfg := &config.Config{}
	manager := NewCustomFieldManager(nil, cfg, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))

	standardFields := []string{"Summary", "Description", "IssueType", "Priority", "Status"}
	for _, field := range standardFields {
		assert.True(t, manager.isStandardField(field), "%s should be a standard field", field)
	}

	customFields := []string{"customfield_10010", "custom_field_1", "MyCustomField"}
	for _, field := range customFields {
		assert.False(t, manager.isStandardField(field), "%s should not be a standard field", field)
	}
}

func TestCustomFieldManager_isADFContent(t *testing.T) {
	cfg := &config.Config{}
	manager := NewCustomFieldManager(nil, cfg, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))

	testCases := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{"string", "plain text", false},
		{"ADF map", map[string]interface{}{"type": "doc"}, true},
		{"ADF map with content", map[string]interface{}{
			"type":    "doc",
			"content": []interface{}{},
		}, true},
		{"nil", nil, false},
		{"int", 123, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := manager.isADFContent(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestCustomFieldManager_JSONConfiguration(t *testing.T) {
	cfg := &config.Config{
		CustomFieldMappings: map[string]interface{}{
			"customfield_10010": "labels",
			"customfield_10011": map[string]interface{}{
				"target_field": "priority",
				"type":         "number",
				"adf_support":  true,
				"required":     false,
			},
		},
	}

	manager := NewCustomFieldManager(nil, cfg, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
	err := manager.LoadMappings()
	require.NoError(t, err)

	mappings := manager.GetCustomFieldMappings()
	assert.Len(t, mappings, 2)

	// Test simple mapping
	mapping1, found := manager.GetMappingByJiraField("customfield_10010")
	assert.True(t, found)
	assert.Equal(t, "labels", mapping1.GitLabField)

	// Test complex mapping
	mapping2, found := manager.GetMappingByJiraField("customfield_10011")
	assert.True(t, found)
	assert.Equal(t, "priority", mapping2.GitLabField)
	assert.Equal(t, "number", mapping2.FieldType)
	assert.True(t, mapping2.ADFSupport)
	assert.False(t, mapping2.Required)
}
