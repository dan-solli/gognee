package embeddings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIClientEmbedOne(t *testing.T) {
	// Create fake server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-key" {
			t.Errorf("Expected Bearer test-key, got %s", authHeader)
		}

		// Return fake embedding
		resp := openAIResponse{
			Data: []struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{
					Embedding: []float32{0.1, 0.2, 0.3},
					Index:     0,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClient("test-key")
	client.BaseURL = server.URL

	embedding, err := client.EmbedOne(context.Background(), "test text")
	if err != nil {
		t.Fatalf("EmbedOne failed: %v", err)
	}

	if len(embedding) != 3 {
		t.Errorf("Expected embedding length 3, got %d", len(embedding))
	}

	expected := []float32{0.1, 0.2, 0.3}
	for i, v := range expected {
		if embedding[i] != v {
			t.Errorf("Embedding[%d]: expected %f, got %f", i, v, embedding[i])
		}
	}
}

func TestOpenAIClientEmbedMultiple(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIResponse{
			Data: []struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{
					Embedding: []float32{0.1, 0.2},
					Index:     0,
				},
				{
					Embedding: []float32{0.3, 0.4},
					Index:     1,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClient("test-key")
	client.BaseURL = server.URL

	embeddings, err := client.Embed(context.Background(), []string{"text1", "text2"})
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(embeddings) != 2 {
		t.Fatalf("Expected 2 embeddings, got %d", len(embeddings))
	}

	if embeddings[0][0] != 0.1 || embeddings[0][1] != 0.2 {
		t.Errorf("Unexpected embedding 0: %v", embeddings[0])
	}

	if embeddings[1][0] != 0.3 || embeddings[1][1] != 0.4 {
		t.Errorf("Unexpected embedding 1: %v", embeddings[1])
	}
}

func TestOpenAIClientEmptyInput(t *testing.T) {
	client := NewOpenAIClient("test-key")

	embeddings, err := client.Embed(context.Background(), []string{})
	if err != nil {
		t.Fatalf("Embed with empty input should not error: %v", err)
	}

	if len(embeddings) != 0 {
		t.Errorf("Expected 0 embeddings for empty input, got %d", len(embeddings))
	}
}

func TestOpenAIClientAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		resp := openAIResponse{
			Error: &openAIError{
				Message: "Invalid API key",
				Type:    "invalid_request_error",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClient("bad-key")
	client.BaseURL = server.URL

	_, err := client.EmbedOne(context.Background(), "test")
	if err == nil {
		t.Fatal("Expected error for invalid API key")
	}

	if err.Error() != "API error (400): Invalid API key" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestOpenAIClientNon200Response(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
	}))
	defer server.Close()

	client := NewOpenAIClient("test-key")
	client.BaseURL = server.URL

	_, err := client.EmbedOne(context.Background(), "test")
	if err == nil {
		t.Fatal("Expected error for non-200 response")
	}
}

func TestOpenAIClientInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewOpenAIClient("test-key")
	client.BaseURL = server.URL

	_, err := client.EmbedOne(context.Background(), "test")
	if err == nil {
		t.Fatal("Expected error for invalid JSON response")
	}
}

func TestOpenAIClientContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This should never be reached because context is cancelled
		t.Error("Request should have been cancelled")
	}))
	defer server.Close()

	client := NewOpenAIClient("test-key")
	client.BaseURL = server.URL

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.EmbedOne(ctx, "test")
	if err == nil {
		t.Fatal("Expected error for cancelled context")
	}
}
