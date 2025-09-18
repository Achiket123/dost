package repository

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
)

var (
	// ErrDataNotFound is returned when data is not found
	ErrDataNotFound = errors.New("data not found")

	// ErrDataExists is returned when data already exists
	ErrDataExists = errors.New("data already exists")

	// ErrInvalidData is returned when data is invalid
	ErrInvalidData = errors.New("invalid data")
)

// RateLimitError represents a structured rate limit error
type RateLimitError struct {
	Code       int    `json:"code"`
	Message    string `json:"message"`
	Status     string `json:"status"`
	RetryDelay string `json:"retryDelay,omitempty"`
}

// parseRetryDelay extracts retry delay from error response
func ParseRetryDelay(errorBody string) time.Duration {
	// Try to parse the structured error response
	var errorResponse struct {
		Error struct {
			Details []struct {
				Type       string `json:"@type"`
				RetryDelay string `json:"retryDelay,omitempty"`
			} `json:"details"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(errorBody), &errorResponse); err == nil {
		for _, detail := range errorResponse.Error.Details {
			if detail.Type == "type.googleapis.com/google.rpc.RetryInfo" && detail.RetryDelay != "" {
				// Parse delay like "53s"
				if duration, err := time.ParseDuration(detail.RetryDelay); err == nil {
					return duration
				}
			}
		}
	}

	// Fallback: try to extract delay from error message
	if strings.Contains(errorBody, `"retryDelay"`) {
		start := strings.Index(errorBody, `"retryDelay": "`) + len(`"retryDelay": "`)
		end := strings.Index(errorBody[start:], `"`)
		if end > 0 {
			delayStr := errorBody[start : start+end]
			if duration, err := time.ParseDuration(delayStr); err == nil {
				return duration
			}
		}
	}

	return 0
}

// exponentialBackoff calculates delay with jitter
func ExponentialBackoff(attempt int) time.Duration {
	baseDelay := time.Second
	maxDelay := 5 * time.Minute

	delay := time.Duration(math.Pow(2, float64(attempt))) * baseDelay
	if delay > maxDelay {
		delay = maxDelay
	}

	// Add 10% jitter
	jitter := time.Duration(float64(delay) * 0.1)
	return delay + jitter
}
