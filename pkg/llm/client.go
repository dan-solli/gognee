// Package llm provides interfaces and implementations for LLM completion clients
package llm

import "context"

// LLMClient defines the interface for interacting with large language models
type LLMClient interface {
	// Complete sends a prompt to the LLM and returns the raw completion text
	Complete(ctx context.Context, prompt string) (string, error)

	// CompleteWithSchema sends a prompt and unmarshals the response into the provided schema
	// The schema parameter should be a pointer to the target struct
	CompleteWithSchema(ctx context.Context, prompt string, schema any) error
}
