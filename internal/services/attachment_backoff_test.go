package services

import (
	"testing"
	"time"
)

type mockZeroJitterSource struct{}

func (m mockZeroJitterSource) Int63n(n int64) int64 {
	return 0
}

func TestCalculateNextAttempt(t *testing.T) {
	now := time.Date(2026, 8, 10, 20, 0, 0, 0, time.UTC)
	s := &attachmentService{
		jitterSource: mockZeroJitterSource{},
	}

	tests := []struct {
		name             string
		attempts         int
		expectedDuration time.Duration
	}{
		{
			name:             "Zero or negative attempts maps to first attempt (30s)",
			attempts:         0,
			expectedDuration: 30 * time.Second,
		},
		{
			name:             "Negative attempts maps to first attempt (30s)",
			attempts:         -5,
			expectedDuration: 30 * time.Second,
		},
		{
			name:             "Attempt 1 maps to 30s",
			attempts:         1,
			expectedDuration: 30 * time.Second,
		},
		{
			name:             "Attempt 2 maps to 1m",
			attempts:         2,
			expectedDuration: 60 * time.Second,
		},
		{
			name:             "Attempt 3 maps to 2m",
			attempts:         3,
			expectedDuration: 120 * time.Second,
		},
		{
			name:             "Attempt 7 maps to 32m",
			attempts:         7,
			expectedDuration: 30 * (1 << 6) * time.Second, // 30 * 64 = 1920s = 32m
		},
		{
			name:             "Attempt 8 maps to 1 hour (cap)",
			attempts:         8,
			expectedDuration: 1 * time.Hour,
		},
		{
			name:             "Attempt 100 maps to 1 hour (cap and overflow prevention)",
			attempts:         100,
			expectedDuration: 1 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.calculateNextAttempt(tt.attempts, now)
			actualDuration := result.Sub(now)
			if actualDuration != tt.expectedDuration {
				t.Errorf("calculateNextAttempt(%d) = %v; expected duration %v, got %v",
					tt.attempts, result, tt.expectedDuration, actualDuration)
			}
		})
	}
}
