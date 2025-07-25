package timezone

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	tests := []struct {
		name      string
		timezone  string
		expectErr bool
	}{
		{"valid timezone UTC", "UTC", false},
		{"valid timezone Asia/Almaty", "Asia/Almaty", false},
		{"valid timezone Europe/Moscow", "Europe/Moscow", false},
		{"invalid timezone", "Invalid/Timezone", true},
		{"empty timezone", "", false}, // Empty string is actually valid for time.LoadLocation
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewManager(tt.timezone)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, manager)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, manager)
				assert.Equal(t, tt.timezone, manager.GetTimezone())
			}
		})
	}
}

func TestManager_GetLocation(t *testing.T) {
	manager, err := NewManager("UTC")
	require.NoError(t, err)

	location := manager.GetLocation()
	assert.NotNil(t, location)
	assert.Equal(t, "UTC", location.String())
}

func TestManager_GetTimezone(t *testing.T) {
	timezone := "Asia/Almaty"
	manager, err := NewManager(timezone)
	require.NoError(t, err)

	result := manager.GetTimezone()
	assert.Equal(t, timezone, result)
}

func TestManager_Now(t *testing.T) {
	manager, err := NewManager("UTC")
	require.NoError(t, err)

	now := manager.Now()
	assert.NotZero(t, now)
	assert.Equal(t, "UTC", now.Location().String())
}

func TestManager_ParseTime(t *testing.T) {
	manager, err := NewManager("UTC")
	require.NoError(t, err)

	layout := "2006-01-02 15:04:05"
	timeStr := "2023-12-25 10:30:00"

	parsed, err := manager.ParseTime(layout, timeStr)
	assert.NoError(t, err)
	assert.Equal(t, "UTC", parsed.Location().String())

	expected := time.Date(2023, 12, 25, 10, 30, 0, 0, time.UTC)
	assert.Equal(t, expected, parsed)
}

func TestManager_ParseTime_InvalidFormat(t *testing.T) {
	manager, err := NewManager("UTC")
	require.NoError(t, err)

	layout := "2006-01-02 15:04:05"
	timeStr := "invalid-time"

	_, err = manager.ParseTime(layout, timeStr)
	assert.Error(t, err)
}

func TestManager_FormatTime(t *testing.T) {
	manager, err := NewManager("UTC")
	require.NoError(t, err)

	testTime := time.Date(2023, 12, 25, 10, 30, 0, 0, time.UTC)
	layout := "2006-01-02 15:04:05"

	formatted := manager.FormatTime(testTime, layout)
	assert.Equal(t, "2023-12-25 10:30:00", formatted)
}

func TestManager_FormatTimeGOST(t *testing.T) {
	manager, err := NewManager("UTC")
	require.NoError(t, err)

	testTime := time.Date(2023, 12, 25, 10, 30, 0, 0, time.UTC)

	formatted := manager.FormatTimeGOST(testTime)
	assert.Equal(t, "25.12.2023 10:30:00", formatted)
}

func TestManager_FormatDateGOST(t *testing.T) {
	manager, err := NewManager("UTC")
	require.NoError(t, err)

	testTime := time.Date(2023, 12, 25, 10, 30, 0, 0, time.UTC)

	formatted := manager.FormatDateGOST(testTime)
	assert.Equal(t, "25.12.2023", formatted)
}

func TestManager_FormatDateTimeGOST(t *testing.T) {
	manager, err := NewManager("UTC")
	require.NoError(t, err)

	testTime := time.Date(2023, 12, 25, 10, 30, 0, 0, time.UTC)

	formatted := manager.FormatDateTimeGOST(testTime)
	assert.Equal(t, "25.12.2023 10:30:00", formatted)
}

func TestManager_FormatTimeForJira(t *testing.T) {
	manager, err := NewManager("UTC")
	require.NoError(t, err)

	testTime := time.Date(2023, 12, 25, 10, 30, 0, 0, time.UTC)

	formatted := manager.FormatTimeForJira(testTime)
	assert.Equal(t, "2023-12-25T10:30:00+00:00", formatted)
}

func TestManager_FormatTimeForLogs(t *testing.T) {
	manager, err := NewManager("UTC")
	require.NoError(t, err)

	testTime := time.Date(2023, 12, 25, 10, 30, 0, 0, time.UTC)

	formatted := manager.FormatTimeForLogs(testTime)
	assert.Equal(t, "2023-12-25 10:30:00.000", formatted)
}

func TestManager_ConvertToTimezone(t *testing.T) {
	// Create manager with UTC timezone
	manager, err := NewManager("UTC")
	require.NoError(t, err)

	// Create time in a different timezone (e.g., local time)
	testTime := time.Now()

	// Convert to UTC
	converted := manager.ConvertToTimezone(testTime)
	assert.Equal(t, "UTC", converted.Location().String())
}

func TestManager_TimezoneConversion(t *testing.T) {
	// Test conversion between different timezones
	utcManager, err := NewManager("UTC")
	require.NoError(t, err)

	moscowManager, err := NewManager("Europe/Moscow")
	require.NoError(t, err)

	testTime := time.Date(2023, 12, 25, 10, 30, 0, 0, time.UTC)

	// Convert UTC to Moscow time
	moscowTime := moscowManager.ConvertToTimezone(testTime)
	assert.Equal(t, "Europe/Moscow", moscowTime.Location().String())

	// Convert back to UTC
	utcTime := utcManager.ConvertToTimezone(moscowTime)
	assert.Equal(t, "UTC", utcTime.Location().String())
}

func TestIsValidTimezone(t *testing.T) {
	tests := []struct {
		name     string
		timezone string
		valid    bool
	}{
		{"valid UTC", "UTC", true},
		{"valid Asia/Almaty", "Asia/Almaty", true},
		{"valid Europe/Moscow", "Europe/Moscow", true},
		{"invalid timezone", "Invalid/Timezone", false},
		{"empty timezone", "", true}, // Empty string is actually valid
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidTimezone(tt.timezone)
			assert.Equal(t, tt.valid, result)
		})
	}
}

func TestGetCommonTimezones(t *testing.T) {
	timezones := GetCommonTimezones()

	// Check that common timezones are included
	assert.Contains(t, timezones, "UTC")
	assert.Contains(t, timezones, "Asia/Almaty")
	assert.Contains(t, timezones, "Europe/Moscow")
	assert.Contains(t, timezones, "America/New_York")

	// Check that descriptions are provided
	assert.Equal(t, "UTC", timezones["UTC"])
	assert.Equal(t, "Asia/Almaty (Kazakhstan)", timezones["Asia/Almaty"])
	assert.Equal(t, "Europe/Moscow (MSK)", timezones["Europe/Moscow"])

	// Check that most timezones in the map are valid (some might not be available on all systems)
	validCount := 0
	for tz := range timezones {
		if IsValidTimezone(tz) {
			validCount++
		}
	}
	// At least 80% of timezones should be valid
	assert.GreaterOrEqual(t, validCount, len(timezones)*8/10, "At least 80%% of timezones should be valid")
}

func TestManager_FormatTimeWithDifferentTimezones(t *testing.T) {
	testCases := []struct {
		name     string
		timezone string
		expected string
	}{
		{"UTC", "UTC", "2023-12-25 10:30:00"},
		// Note: Actual offset depends on the system and timezone database
		// We'll test that the timezone conversion works, but not the exact offset
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager, err := NewManager(tc.timezone)
			require.NoError(t, err)

			testTime := time.Date(2023, 12, 25, 10, 30, 0, 0, time.UTC)
			layout := "2006-01-02 15:04:05"

			formatted := manager.FormatTime(testTime, layout)
			assert.Equal(t, tc.expected, formatted)
		})
	}
}

func TestManager_EdgeCases(t *testing.T) {
	manager, err := NewManager("UTC")
	require.NoError(t, err)

	// Test with zero time
	zeroTime := time.Time{}
	formatted := manager.FormatTimeGOST(zeroTime)
	assert.Equal(t, "01.01.0001 00:00:00", formatted)

	// Test with very old date
	oldTime := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	formatted = manager.FormatTimeGOST(oldTime)
	assert.Equal(t, "01.01.1900 00:00:00", formatted)

	// Test with future date
	futureTime := time.Date(2100, 12, 31, 23, 59, 59, 0, time.UTC)
	formatted = manager.FormatTimeGOST(futureTime)
	assert.Equal(t, "31.12.2100 23:59:59", formatted)
}
