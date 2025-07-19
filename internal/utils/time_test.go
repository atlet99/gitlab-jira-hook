package utils

import (
	"testing"
	"time"
)

func TestFormatDateGOST(t *testing.T) {
	tests := []struct {
		name     string
		dateStr  string
		timezone string
		expected string
	}{
		{
			name:     "UTC time to Asia/Almaty",
			dateStr:  "2024-01-15T10:30:00Z",
			timezone: "Asia/Almaty",
			expected: "15.01.2024 16:30:00", // UTC+6 (old tzdata)
		},
		{
			name:     "UTC time to UTC",
			dateStr:  "2024-01-15T10:30:00Z",
			timezone: "UTC",
			expected: "15.01.2024 10:30:00",
		},
		{
			name:     "Invalid timezone falls back to UTC",
			dateStr:  "2024-01-15T10:30:00Z",
			timezone: "Invalid/Timezone",
			expected: "15.01.2024 10:30:00",
		},
		{
			name:     "Invalid date string returns original",
			dateStr:  "invalid-date",
			timezone: "Asia/Almaty",
			expected: "invalid-date",
		},
		{
			name:     "UTC time to Etc/GMT-5",
			dateStr:  "2024-01-15T10:30:00Z",
			timezone: "Etc/GMT-5",
			expected: "15.01.2024 15:30:00", // UTC+5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDateGOST(tt.dateStr, tt.timezone)
			if result != tt.expected {
				t.Errorf("FormatDateGOST(%s, %s) = %s, want %s", tt.dateStr, tt.timezone, result, tt.expected)
			}
		})
	}
}

func TestFormatCurrentTimeGOST(t *testing.T) {
	// Test that the function returns a valid date format
	result := FormatCurrentTimeGOST("Asia/Almaty")

	// Parse the result to ensure it's a valid date
	_, err := time.Parse(DateFormatGOSTWithSeconds, result)
	if err != nil {
		t.Errorf("FormatCurrentTimeGOST returned invalid date format: %s, error: %v", result, err)
	}

	// Check that it's in the expected format (DD.MM.YYYY HH:MM:SS)
	if len(result) != 19 {
		t.Errorf("FormatCurrentTimeGOST returned unexpected length: %s (length: %d)", result, len(result))
	}
}
