package gognee

import (
	"context"
	"errors"
	"net"
	"strings"
)

// Error type constants for classification
const (
	ErrTypeNetwork    = "network"
	ErrTypeTimeout    = "timeout"
	ErrTypeLLM        = "llm"
	ErrTypeDatabase   = "database"
	ErrTypeValidation = "validation"
	ErrTypeUnknown    = "unknown"
)

// ClassifyError inspects an error and returns its type classification.
// This enables grouping errors by category in metrics and traces.
func ClassifyError(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()
	errStrLower := strings.ToLower(errStr)

	// Check for timeout errors
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(errStrLower, "timeout") || strings.Contains(errStrLower, "deadline exceeded") {
		return ErrTypeTimeout
	}

	// Check for network errors
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return ErrTypeNetwork
	}
	if strings.Contains(errStrLower, "connection refused") ||
		strings.Contains(errStrLower, "connection reset") ||
		strings.Contains(errStrLower, "no such host") ||
		strings.Contains(errStrLower, "network is unreachable") ||
		strings.Contains(errStrLower, "dial tcp") ||
		strings.Contains(errStrLower, "eof") {
		return ErrTypeNetwork
	}

	// Check for LLM/API errors (OpenAI specific)
	if strings.Contains(errStrLower, "api error") ||
		strings.Contains(errStrLower, "rate limit") ||
		strings.Contains(errStrLower, "invalid response") ||
		strings.Contains(errStrLower, "embedding") ||
		strings.Contains(errStrLower, "openai") ||
		strings.Contains(errStrLower, "model") && strings.Contains(errStrLower, "not found") {
		return ErrTypeLLM
	}

	// Check for database errors (SQLite specific)
	if strings.Contains(errStrLower, "sql") ||
		strings.Contains(errStrLower, "database") ||
		strings.Contains(errStrLower, "constraint") ||
		strings.Contains(errStrLower, "unique") && strings.Contains(errStrLower, "failed") {
		return ErrTypeDatabase
	}

	// Check for validation errors
	if strings.Contains(errStrLower, "validation") ||
		strings.Contains(errStrLower, "invalid") ||
		strings.Contains(errStrLower, "required") ||
		strings.Contains(errStrLower, "cannot be empty") ||
		strings.Contains(errStrLower, "must be") {
		return ErrTypeValidation
	}

	// Default to unknown
	return ErrTypeUnknown
}
