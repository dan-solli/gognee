package gognee

import (
	"testing"

	"github.com/dan-solli/gognee/pkg/embeddings"
	"github.com/dan-solli/gognee/pkg/llm"
)

func TestNewAppliesDefaults(t *testing.T) {
	g, err := New(Config{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if g.GetChunker() == nil {
		t.Fatalf("GetChunker returned nil")
	}
	if g.GetChunker().MaxTokens != 512 {
		t.Fatalf("MaxTokens: got %d, want %d", g.GetChunker().MaxTokens, 512)
	}
	if g.GetChunker().Overlap != 50 {
		t.Fatalf("Overlap: got %d, want %d", g.GetChunker().Overlap, 50)
	}

	if g.GetEmbeddings() == nil {
		t.Fatalf("GetEmbeddings returned nil")
	}

	if g.GetLLM() == nil {
		t.Fatalf("GetLLM returned nil")
	}
}

func TestNewRespectsEmbeddingConfig(t *testing.T) {
	g, err := New(Config{
		OpenAIKey:      "k-test",
		EmbeddingModel: "m-test",
		LLMModel:       "llm-test",
		ChunkSize:      123,
		ChunkOverlap:   7,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if g.GetChunker().MaxTokens != 123 {
		t.Fatalf("MaxTokens: got %d, want %d", g.GetChunker().MaxTokens, 123)
	}
	if g.GetChunker().Overlap != 7 {
		t.Fatalf("Overlap: got %d, want %d", g.GetChunker().Overlap, 7)
	}

	client, ok := g.GetEmbeddings().(*embeddings.OpenAIClient)
	if !ok {
		t.Fatalf("GetEmbeddings type: got %T, want *embeddings.OpenAIClient", g.GetEmbeddings())
	}
	if client.APIKey != "k-test" {
		t.Fatalf("APIKey: got %q, want %q", client.APIKey, "k-test")
	}
	if client.Model != "m-test" {
		t.Fatalf("Model: got %q, want %q", client.Model, "m-test")
	}

	llmClient, ok := g.GetLLM().(*llm.OpenAILLM)
	if !ok {
		t.Fatalf("GetLLM type: got %T, want *llm.OpenAILLM", g.GetLLM())
	}
	if llmClient.APIKey != "k-test" {
		t.Fatalf("LLM APIKey: got %q, want %q", llmClient.APIKey, "k-test")
	}
	if llmClient.Model != "llm-test" {
		t.Fatalf("LLM Model: got %q, want %q", llmClient.Model, "llm-test")
	}
}
