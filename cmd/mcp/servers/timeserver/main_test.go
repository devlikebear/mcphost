package main

import (
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TimeServer creation test
func TestNewTimeServer(t *testing.T) {
	// Test cases: Creating new TimeServer instances with default timezones
	testCases := []struct {
		name            string
		defaultTimezone string
	}{
		{
			name:            "Set Asia/Seoul as default timezone",
			defaultTimezone: "Asia/Seoul",
		},
		{
			name:            "Set UTC as default timezone",
			defaultTimezone: "UTC",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create TimeServer instance
			ts := NewTimeServer(tc.defaultTimezone)

			// Verification
			assert.NotNil(t, ts, "TimeServer instance should be created")
			assert.Equal(t, tc.defaultTimezone, ts.defaultTimezone, "Default timezone should match")
			assert.NotNil(t, ts.server, "Internal MCPServer should be initialized")
		})
	}
}

// Server method test
func TestServer(t *testing.T) {
	// Set default timezone
	defaultTZ := "Asia/Seoul"

	// Create TimeServer instance
	ts := NewTimeServer(defaultTZ)
	assert.NotNil(t, ts, "TimeServer instance should be created")

	// Verify Server method returns valid MCPServer instance
	server := ts.Server()
	assert.NotNil(t, server, "Server method should return a valid MCPServer instance")
}

// convertTimeToTimezone function test
func TestConvertTimeToTimezone(t *testing.T) {
	// Test cases
	testCases := []struct {
		name              string
		timeStr           string
		requestedTimezone string
		defaultTimezone   string
		expectError       bool
		expectedTimezone  string // Verify which timezone is used
		checkCurrentTime  bool   // Whether to compare with current time
	}{
		{
			name:              "Explicit timezone request - Asia/Seoul (current time)",
			timeStr:           "",
			requestedTimezone: "Asia/Seoul",
			defaultTimezone:   "UTC",
			expectError:       false,
			expectedTimezone:  "Asia/Seoul",
			checkCurrentTime:  true,
		},
		{
			name:              "Explicit timezone request - UTC (current time)",
			timeStr:           "",
			requestedTimezone: "UTC",
			defaultTimezone:   "Asia/Seoul",
			expectError:       false,
			expectedTimezone:  "UTC",
			checkCurrentTime:  true,
		},
		{
			name:              "Use default timezone when unspecified (current time)",
			timeStr:           "",
			requestedTimezone: "",
			defaultTimezone:   "Asia/Seoul",
			expectError:       false,
			expectedTimezone:  "Asia/Seoul",
			checkCurrentTime:  true,
		},
		{
			name:              "Invalid timezone request",
			timeStr:           "",
			requestedTimezone: "Invalid/TimeZone",
			defaultTimezone:   "UTC",
			expectError:       true,
			expectedTimezone:  "",
			checkCurrentTime:  false,
		},
		{
			name:              "Specific time string - Asia/Seoul",
			timeStr:           "2025-04-06T14:30:00Z",
			requestedTimezone: "Asia/Seoul",
			defaultTimezone:   "UTC",
			expectError:       false,
			expectedTimezone:  "Asia/Seoul",
			checkCurrentTime:  false,
		},
		{
			name:              "Specific time string - UTC",
			timeStr:           "2025-04-06T14:30:00Z",
			requestedTimezone: "UTC",
			defaultTimezone:   "Asia/Seoul",
			expectError:       false,
			expectedTimezone:  "UTC",
			checkCurrentTime:  false,
		},
		{
			name:              "Invalid time string format",
			timeStr:           "2025/04/06 14:30:00",
			requestedTimezone: "Asia/Seoul",
			defaultTimezone:   "UTC",
			expectError:       true,
			expectedTimezone:  "",
			checkCurrentTime:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create TimeServer instance
			ts := NewTimeServer(tc.defaultTimezone)

			// Call time conversion function
			resultTimezone, resultTime, err := ts.convertTimeToTimezone(tc.timeStr, tc.requestedTimezone)

			// Verify error cases
			if tc.expectError {
				assert.Error(t, err, "An error should occur")
				assert.Empty(t, resultTimezone, "Timezone should be empty string when error occurs")
				assert.True(t, resultTime.IsZero(), "Time should be zero value when error occurs")
				return
			}

			// Verify success cases
			assert.NoError(t, err, "No error should occur")
			assert.Equal(t, tc.expectedTimezone, resultTimezone, "Returned timezone should match expected value")
			assert.False(t, resultTime.IsZero(), "Valid time should be returned")

			// Verify time is set for the correct timezone
			loc, _ := time.LoadLocation(tc.expectedTimezone)
			assert.Equal(t, loc.String(), resultTime.Location().String(), "Time's timezone should match expected value")

			if tc.checkCurrentTime {
				// Verify it's close to current time (within 1 minute)
				now := time.Now()
				diff := now.Sub(resultTime)
				assert.LessOrEqual(t, diff.Abs(), time.Minute, "Should be within 1 minute of current time")
			} else if tc.timeStr != "" {
				// If specific time string was provided, verify conversion result
				expectedTime, _ := time.Parse(time.RFC3339, tc.timeStr)
				expectedTime = expectedTime.In(loc)

				// Verify year, month, day, hour, minute, second match
				assert.Equal(t, expectedTime.Year(), resultTime.Year(), "Year should match")
				assert.Equal(t, expectedTime.Month(), resultTime.Month(), "Month should match")
				assert.Equal(t, expectedTime.Day(), resultTime.Day(), "Day should match")
				assert.Equal(t, expectedTime.Hour(), resultTime.Hour(), "Hour should match")
				assert.Equal(t, expectedTime.Minute(), resultTime.Minute(), "Minute should match")
				assert.Equal(t, expectedTime.Second(), resultTime.Second(), "Second should match")
			}

			// Verify time format
			timeStr := resultTime.Format(time.RFC3339)
			assert.Regexp(t, `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`, timeStr, "Should be in RFC3339 format")
		})
	}
}

// Time formatting validation test
func TestTimeFormatting(t *testing.T) {
	// Validate time format for multiple timezones
	timezones := []string{
		"Asia/Seoul",
		"UTC",
		"America/New_York",
	}

	for _, tz := range timezones {
		t.Run("Timezone time format: "+tz, func(t *testing.T) {
			// Verify it's a valid timezone
			loc, err := time.LoadLocation(tz)
			assert.NoError(t, err, "Should be a valid timezone")

			// Generate time string
			now := time.Now().In(loc)
			timeStr := now.Format(time.RFC3339)

			// Validate RFC3339 format
			pattern := regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`)
			assert.True(t, pattern.MatchString(timeStr), "Should be a time string in RFC3339 format")
		})
	}
}

// Test to distinguish valid and invalid timezones
func TestTimezoneFunctionality(t *testing.T) {
	// List of valid timezones
	validTimezones := []string{
		"Asia/Seoul",
		"UTC",
		"America/New_York",
		"Europe/London",
		"Australia/Sydney",
	}

	// List of invalid timezones
	invalidTimezones := []string{
		"Invalid/TimeZone",
		"Not/Real",
		"Asia/NotReal",
	}

	// Test valid timezones
	for _, tz := range validTimezones {
		t.Run("Valid timezone: "+tz, func(t *testing.T) {
			_, err := time.LoadLocation(tz)
			assert.NoError(t, err, "Valid timezone should not cause error in LoadLocation")
		})
	}

	// Test invalid timezones
	for _, tz := range invalidTimezones {
		t.Run("Invalid timezone: "+tz, func(t *testing.T) {
			_, err := time.LoadLocation(tz)
			assert.Error(t, err, "Invalid timezone should cause error in LoadLocation")
		})
	}
}

// Test default timezone logic when timezone is empty
func TestDefaultTimezoneLogic(t *testing.T) {
	defaultTZ := "Asia/Seoul"
	emptyTZ := ""

	// Verify default value logic
	t.Run("Empty timezone should use default value", func(t *testing.T) {
		// Set empty timezone value
		timezone := emptyTZ

		// Set default if empty
		if timezone == "" {
			timezone = defaultTZ
		}

		// Verify default value was set
		assert.Equal(t, defaultTZ, timezone, "Default timezone should be used when timezone is empty")

		// Verify it's a valid timezone
		_, err := time.LoadLocation(timezone)
		assert.NoError(t, err, "Default timezone should be valid")
	})
}
