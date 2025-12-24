package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOpenAILLMComplete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and headers
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Expected Bearer test-key, got %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Return valid response
		resp := openAIResponse{
			Choices: []struct {
				Message message `json:"message"`
			}{
				{
					Message: message{
						Role:    "assistant",
						Content: "Test response from LLM",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAILLM("test-key")
	client.BaseURL = server.URL

	result, err := client.Complete(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	if result != "Test response from LLM" {
		t.Errorf("Expected 'Test response from LLM', got %s", result)
	}
}

func TestOpenAILLMComplete_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIResponse{
			Choices: []struct {
				Message message `json:"message"`
			}{},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAILLM("test-key")
	client.BaseURL = server.URL

	_, err := client.Complete(context.Background(), "test prompt")
	if err == nil {
		t.Fatal("Expected error for empty response, got nil")
	}

	if !strings.Contains(err.Error(), "no completion choices") {
		t.Errorf("Expected 'no completion choices' error, got: %v", err)
	}
}

func TestOpenAILLMComplete_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad request"))
	}))
	defer server.Close()

	client := NewOpenAILLM("test-key")
	client.BaseURL = server.URL

	_, err := client.Complete(context.Background(), "test prompt")
	if err == nil {
		t.Fatal("Expected error for 400 status, got nil")
	}

	if !strings.Contains(err.Error(), "HTTP 400") {
		t.Errorf("Expected 'HTTP 400' error, got: %v", err)
	}
}

func TestOpenAILLMComplete_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	client := NewOpenAILLM("test-key")
	client.BaseURL = server.URL

	_, err := client.Complete(context.Background(), "test prompt")
	if err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}

	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("Expected unmarshal error, got: %v", err)
	}
}

func TestOpenAILLMComplete_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIResponse{
			Error: &struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			}{
				Message: "Invalid API key",
				Type:    "invalid_request_error",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAILLM("test-key")
	client.BaseURL = server.URL

	_, err := client.Complete(context.Background(), "test prompt")
	if err == nil {
		t.Fatal("Expected error for API error, got nil")
	}

	if !strings.Contains(err.Error(), "Invalid API key") {
		t.Errorf("Expected 'Invalid API key' error, got: %v", err)
	}
}

func TestOpenAILLMComplete_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		resp := openAIResponse{
			Choices: []struct {
				Message message `json:"message"`
			}{
				{
					Message: message{
						Role:    "assistant",
						Content: "Response",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAILLM("test-key")
	client.BaseURL = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.Complete(ctx, "test prompt")
	if err == nil {
		t.Fatal("Expected context cancellation error, got nil")
	}

	if !strings.Contains(err.Error(), "context") {
		t.Errorf("Expected context error, got: %v", err)
	}
}

func TestOpenAILLMComplete_RetryOn500(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Server error"))
			return
		}

		resp := openAIResponse{
			Choices: []struct {
				Message message `json:"message"`
			}{
				{
					Message: message{
						Role:    "assistant",
						Content: "Success after retries",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAILLM("test-key")
	client.BaseURL = server.URL

	result, err := client.Complete(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	if result != "Success after retries" {
		t.Errorf("Expected 'Success after retries', got %s", result)
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}
}

func TestOpenAILLMComplete_RetryOn429(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("Rate limited"))
			return
		}

		resp := openAIResponse{
			Choices: []struct {
				Message message `json:"message"`
			}{
				{
					Message: message{
						Role:    "assistant",
						Content: "Success after rate limit",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAILLM("test-key")
	client.BaseURL = server.URL

	result, err := client.Complete(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	if result != "Success after rate limit" {
		t.Errorf("Expected 'Success after rate limit', got %s", result)
	}

	if attemptCount != 2 {
		t.Errorf("Expected 2 attempts, got %d", attemptCount)
	}
}

func TestOpenAILLMComplete_MaxRetriesExceeded(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Persistent error"))
	}))
	defer server.Close()

	client := NewOpenAILLM("test-key")
	client.BaseURL = server.URL

	_, err := client.Complete(context.Background(), "test prompt")
	if err == nil {
		t.Fatal("Expected error after max retries, got nil")
	}

	if !strings.Contains(err.Error(), "failed after") {
		t.Errorf("Expected 'failed after' error, got: %v", err)
	}

	// Should be 4 attempts total (initial + 3 retries)
	if attemptCount != 4 {
		t.Errorf("Expected 4 attempts (initial + 3 retries), got %d", attemptCount)
	}
}

func TestOpenAILLMCompleteWithSchema_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIResponse{
			Choices: []struct {
				Message message `json:"message"`
			}{
				{
					Message: message{
						Role:    "assistant",
						Content: `{"name": "John", "age": 30}`,
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAILLM("test-key")
	client.BaseURL = server.URL

	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	var person Person
	err := client.CompleteWithSchema(context.Background(), "test prompt", &person)
	if err != nil {
		t.Fatalf("CompleteWithSchema failed: %v", err)
	}

	if person.Name != "John" {
		t.Errorf("Expected name 'John', got %s", person.Name)
	}
	if person.Age != 30 {
		t.Errorf("Expected age 30, got %d", person.Age)
	}
}

func TestOpenAILLMCompleteWithSchema_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIResponse{
			Choices: []struct {
				Message message `json:"message"`
			}{
				{
					Message: message{
						Role:    "assistant",
						Content: `not valid json`,
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAILLM("test-key")
	client.BaseURL = server.URL

	type Person struct {
		Name string `json:"name"`
	}

	var person Person
	err := client.CompleteWithSchema(context.Background(), "test prompt", &person)
	if err == nil {
		t.Fatal("Expected error for invalid JSON in schema, got nil")
	}

	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("Expected unmarshal error, got: %v", err)
	}
}
