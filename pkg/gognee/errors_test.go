package gognee

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
)

func TestClassifyError_Timeout(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"context deadline", context.DeadlineExceeded},
		{"string timeout", fmt.Errorf("operation timeout")},
		{"deadline exceeded", fmt.Errorf("context deadline exceeded")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyError(tt.err); got != ErrTypeTimeout {
				t.Errorf("ClassifyError() = %v, want %v", got, ErrTypeTimeout)
			}
		})
	}
}

func TestClassifyError_Network(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"connection refused", fmt.Errorf("connection refused")},
		{"connection reset", fmt.Errorf("connection reset by peer")},
		{"no such host", fmt.Errorf("no such host")},
		{"dial tcp error", fmt.Errorf("dial tcp: connection refused")},
		{"eof", fmt.Errorf("unexpected EOF")},
		{"net.OpError", &net.OpError{Op: "dial", Net: "tcp", Err: fmt.Errorf("refused")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyError(tt.err); got != ErrTypeNetwork {
				t.Errorf("ClassifyError() = %v, want %v for error: %v", got, ErrTypeNetwork, tt.err)
			}
		})
	}
}

func TestClassifyError_LLM(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"api error", fmt.Errorf("API error (429): rate limit exceeded")},
		{"rate limit", fmt.Errorf("rate limit exceeded")},
		{"invalid response", fmt.Errorf("invalid response from API")},
		{"embedding error", fmt.Errorf("embedding generation failed")},
		{"openai error", fmt.Errorf("OpenAI API returned error")},
		{"model not found", fmt.Errorf("model not found")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyError(tt.err); got != ErrTypeLLM {
				t.Errorf("ClassifyError() = %v, want %v", got, ErrTypeLLM)
			}
		})
	}
}

func TestClassifyError_Database(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"sql error", fmt.Errorf("SQL error: syntax error")},
		{"database locked", fmt.Errorf("database is locked")},
		{"constraint violation", fmt.Errorf("UNIQUE constraint failed")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyError(tt.err); got != ErrTypeDatabase {
				t.Errorf("ClassifyError() = %v, want %v", got, ErrTypeDatabase)
			}
		})
	}
}

func TestClassifyError_Validation(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"validation failed", fmt.Errorf("validation failed")},
		{"invalid input", fmt.Errorf("invalid input")},
		{"required field", fmt.Errorf("field is required")},
		{"cannot be empty", fmt.Errorf("topic cannot be empty")},
		{"must be positive", fmt.Errorf("value must be positive")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyError(tt.err); got != ErrTypeValidation {
				t.Errorf("ClassifyError() = %v, want %v", got, ErrTypeValidation)
			}
		})
	}
}

func TestClassifyError_Unknown(t *testing.T) {
	err := fmt.Errorf("some random error")
	if got := ClassifyError(err); got != ErrTypeUnknown {
		t.Errorf("ClassifyError() = %v, want %v", got, ErrTypeUnknown)
	}
}

func TestClassifyError_Nil(t *testing.T) {
	if got := ClassifyError(nil); got != "" {
		t.Errorf("ClassifyError(nil) = %v, want empty string", got)
	}
}

func TestClassifyError_WrappedErrors(t *testing.T) {
	baseErr := context.DeadlineExceeded
	wrappedErr := fmt.Errorf("operation failed: %w", baseErr)
	
	if got := ClassifyError(wrappedErr); got != ErrTypeTimeout {
		t.Errorf("ClassifyError(wrapped) = %v, want %v", got, ErrTypeTimeout)
	}
}

func TestClassifyError_NetworkOpError(t *testing.T) {
	netErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: errors.New("connection refused"),
	}

	if got := ClassifyError(netErr); got != ErrTypeNetwork {
		t.Errorf("ClassifyError(net.OpError) = %v, want %v", got, ErrTypeNetwork)
	}
}
