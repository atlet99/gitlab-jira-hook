package config

import (
	"os"
	"testing"
)

func TestParseFieldMappings_JSONFormat(t *testing.T) {
	tests := []struct {
		name     string
		jsonStr  string
		expected map[string]map[string]string
	}{
		{
			name: "valid JSON with multiple fields",
			jsonStr: `{
				"customfield_10010": {
					"target_field": "labels",
					"transform": "lowercase",
					"prefix": "bug-"
				},
				"customfield_10020": {
					"target_field": "description",
					"transform": "uppercase"
				}
			}`,
			expected: map[string]map[string]string{
				"customfield_10010": {
					"target_field": "labels",
					"transform":    "lowercase",
					"prefix":       "bug-",
				},
				"customfield_10020": {
					"target_field": "description",
					"transform":    "uppercase",
				},
			},
		},
		{
			name:     "empty JSON",
			jsonStr:  "{}",
			expected: map[string]map[string]string{},
		},
		{
			name:     "invalid JSON",
			jsonStr:  "{invalid json}",
			expected: map[string]map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFieldMappingsJSON(tt.jsonStr)

			// Compare maps
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d fields, got %d", len(tt.expected), len(result))
			}

			for jiraField, expectedConfig := range tt.expected {
				actualConfig, exists := result[jiraField]
				if !exists {
					t.Errorf("Expected field %s not found", jiraField)
					continue
				}

				if len(actualConfig) != len(expectedConfig) {
					t.Errorf("Field %s: expected %d config items, got %d", jiraField, len(expectedConfig), len(actualConfig))
				}

				for key, expectedValue := range expectedConfig {
					actualValue, exists := actualConfig[key]
					if !exists {
						t.Errorf("Field %s: expected config key %s not found", jiraField, key)
					} else if actualValue != expectedValue {
						t.Errorf("Field %s config %s: expected %s, got %s", jiraField, key, expectedValue, actualValue)
					}
				}
			}
		})
	}
}

func TestParseFieldMappings_KeyValueFormat(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected map[string]map[string]string
	}{
		{
			name: "valid key-value format with multiple fields",
			value: "customfield_10010:target_field=labels,transform=lowercase,prefix=bug-;" +
				"customfield_10020:target_field=description,transform=uppercase",
			expected: map[string]map[string]string{
				"customfield_10010": {
					"target_field": "labels",
					"transform":    "lowercase",
					"prefix":       "bug-",
				},
				"customfield_10020": {
					"target_field": "description",
					"transform":    "uppercase",
				},
			},
		},
		{
			name:     "empty value",
			value:    "",
			expected: map[string]map[string]string{},
		},
		{
			name:     "whitespace only",
			value:    "   ",
			expected: map[string]map[string]string{},
		},
		{
			name:  "single field with single config",
			value: "customfield_10010:target_field=labels",
			expected: map[string]map[string]string{
				"customfield_10010": {
					"target_field": "labels",
				},
			},
		},
		{
			name: "malformed entries",
			value: "customfield_10010:target_field=labels,invalid_entry;" +
				"invalid_field;" +
				"customfield_10020:",
			expected: map[string]map[string]string{
				"customfield_10010": {
					"target_field": "labels",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFieldMappingsKeyValue(tt.value)

			// Compare maps
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d fields, got %d", len(tt.expected), len(result))
			}

			for jiraField, expectedConfig := range tt.expected {
				actualConfig, exists := result[jiraField]
				if !exists {
					t.Errorf("Expected field %s not found", jiraField)
					continue
				}

				if len(actualConfig) != len(expectedConfig) {
					t.Errorf("Field %s: expected %d config items, got %d", jiraField, len(expectedConfig), len(actualConfig))
				}

				for key, expectedValue := range expectedConfig {
					actualValue, exists := actualConfig[key]
					if !exists {
						t.Errorf("Field %s: expected config key %s not found", jiraField, key)
					} else if actualValue != expectedValue {
						t.Errorf("Field %s config %s: expected %s, got %s", jiraField, key, expectedValue, actualValue)
					}
				}
			}
		})
	}
}

func TestParseFieldMappings(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected map[string]map[string]string
	}{
		{
			name: "JSON format",
			envValue: `{
				"customfield_10010": {
					"target_field": "labels",
					"transform": "lowercase"
				}
			}`,
			expected: map[string]map[string]string{
				"customfield_10010": {
					"target_field": "labels",
					"transform":    "lowercase",
				},
			},
		},
		{
			name:     "key-value format",
			envValue: "customfield_10010:target_field=labels,transform=lowercase",
			expected: map[string]map[string]string{
				"customfield_10010": {
					"target_field": "labels",
					"transform":    "lowercase",
				},
			},
		},
		{
			name:     "empty value",
			envValue: "",
			expected: map[string]map[string]string{},
		},
		{
			name:     "no environment variable",
			envValue: "",
			expected: map[string]map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			envKey := "TEST_FIELD_MAPPINGS"
			if tt.envValue == "" {
				os.Unsetenv(envKey)
			} else {
				os.Setenv(envKey, tt.envValue)
				defer os.Unsetenv(envKey)
			}

			result := parseFieldMappings(envKey)

			// Compare maps
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d fields, got %d", len(tt.expected), len(result))
			}

			for jiraField, expectedConfig := range tt.expected {
				actualConfig, exists := result[jiraField]
				if !exists {
					t.Errorf("Expected field %s not found", jiraField)
					continue
				}

				if len(actualConfig) != len(expectedConfig) {
					t.Errorf("Field %s: expected %d config items, got %d", jiraField, len(expectedConfig), len(actualConfig))
				}

				for key, expectedValue := range expectedConfig {
					actualValue, exists := actualConfig[key]
					if !exists {
						t.Errorf("Field %s: expected config key %s not found", jiraField, key)
					} else if actualValue != expectedValue {
						t.Errorf("Field %s config %s: expected %s, got %s", jiraField, key, expectedValue, actualValue)
					}
				}
			}
		})
	}
}
