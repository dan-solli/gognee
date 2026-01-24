package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	defaultOpenAIBaseURL = "https://api.openai.com/v1"
	defaultModel         = "gpt-4o-mini"
	maxRetries           = 3
	initialRetryDelay    = 1 * time.Second
	backoffFactor        = 2.0
)

// OpenAILLM implements LLMClient for OpenAI's Chat Completions API
type OpenAILLM struct {
	APIKey  string
	Model   string
	BaseURL string
	client  *http.Client
}

// NewOpenAILLM creates a new OpenAI LLM client
func NewOpenAILLM(apiKey string) *OpenAILLM {
	return &OpenAILLM{
		APIKey:  apiKey,
		Model:   defaultModel,
		BaseURL: defaultOpenAIBaseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

type openAIRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// Complete sends a prompt to the OpenAI Chat Completions API and returns the response
func (o *OpenAILLM) Complete(ctx context.Context, prompt string) (string, error) {
	var lastErr error
	delay := initialRetryDelay

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Add jitter to delay: random value between 0.5x and 1.5x of delay
			jitter := delay/2 + time.Duration(rand.Int63n(int64(delay)))
			select {
			case <-time.After(jitter):
			case <-ctx.Done():
				return "", ctx.Err()
			}
			delay = time.Duration(float64(delay) * backoffFactor)
		}

		result, err := o.makeRequest(ctx, prompt)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check if we should retry
		if !shouldRetry(err) {
			return "", err
		}

		// Check if context is cancelled
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
	}

	return "", fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// CompleteWithSchema sends a prompt and unmarshals the JSON response into the provided schema
func (o *OpenAILLM) CompleteWithSchema(ctx context.Context, prompt string, schema any) error {
	response, err := o.Complete(ctx, prompt)
	if err != nil {
		return err
	}

	// Strip markdown code fences if present (LLM sometimes wraps JSON in ```json ... ```)
	cleaned := stripMarkdownCodeFence(response)

	// Normalize arrays to strings where needed (handles LLM non-compliance)
	normalized, changed, err := NormalizeJSONArraysToStrings([]byte(cleaned))
	if err != nil {
		return fmt.Errorf("failed to normalize LLM response: %w", err)
	}

	if changed {
		log.Printf("gognee: LLM response contained array values where strings expected; normalized to comma-joined strings")
	}

	if err := json.Unmarshal(normalized, schema); err != nil {
		return fmt.Errorf("failed to unmarshal LLM response: %w", err)
	}

	return nil
}

// stripMarkdownCodeFence removes markdown code fences from LLM responses.
// Handles formats like: ```json\n...\n``` or ```\n...\n```
func stripMarkdownCodeFence(s string) string {
	s = strings.TrimSpace(s)

	// Regex to match ```json or ``` at start, and ``` at end
	// Pattern: optional ```json or ```, content, optional ```
	re := regexp.MustCompile("(?s)^```(?:json)?\\s*\n?(.*?)\\s*```$")
	if matches := re.FindStringSubmatch(s); len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}

	return s
}

func (o *OpenAILLM) makeRequest(ctx context.Context, prompt string) (string, error) {
	reqBody := openAIRequest{
		Model: o.Model,
		Messages: []message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.APIKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return "", &retryableError{err: fmt.Errorf("request failed: %w", err)}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle non-200 status codes
	if resp.StatusCode != http.StatusOK {
		// Retry on 429 (rate limit) and 5xx errors
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return "", &retryableError{err: fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))}
		}
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var apiResp openAIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if apiResp.Error != nil {
		return "", fmt.Errorf("OpenAI API error: %s", apiResp.Error.Message)
	}

	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned")
	}

	return apiResp.Choices[0].Message.Content, nil
}

// retryableError indicates an error that should be retried
type retryableError struct {
	err error
}

func (e *retryableError) Error() string {
	return e.err.Error()
}

func (e *retryableError) Unwrap() error {
	return e.err
}

func shouldRetry(err error) bool {
	var retryErr *retryableError
	// Use type assertion to check for retryableError
	if re, ok := err.(*retryableError); ok {
		retryErr = re
	}
	return retryErr != nil
}
