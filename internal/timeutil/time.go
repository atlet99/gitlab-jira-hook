// Package timeutil provides utility functions for time formatting
package timeutil

import (
	"time"
)

// DateFormatGOST is the date format according to GOST 7.64-90 standard (DD.MM.YYYY HH:MM)
const DateFormatGOST = "02.01.2006 15:04" // without seconds
// DateFormatGOSTWithSeconds is the date format according to GOST 7.64-90 standard (DD.MM.YYYY HH:MM:SS)
const DateFormatGOSTWithSeconds = "02.01.2006 15:04:05"

// FormatDateGOST formats a date string in RFC3339 format to GOST format with timezone
func FormatDateGOST(dateStr, timezone string) string {
	// Parse the input time in UTC
	parsedTime, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return dateStr
	}

	// Load the timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// If timezone is invalid, use UTC
		loc = time.UTC
	}

	// Convert to the specified timezone and format
	return parsedTime.In(loc).Format(DateFormatGOSTWithSeconds)
}

// FormatCurrentTimeGOST formats current time to GOST format with timezone
func FormatCurrentTimeGOST(timezone string) string {
	// Load the timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// If timezone is invalid, use UTC
		loc = time.UTC
	}

	// Get current time in the specified timezone and format
	return time.Now().In(loc).Format(DateFormatGOSTWithSeconds)
}
