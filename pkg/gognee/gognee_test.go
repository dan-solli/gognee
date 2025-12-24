package gognee

import (
	"testing"

	"github.com/dan-solli/gognee/pkg/embeddings"
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
}

func TestNewRespectsEmbeddingConfig(t *testing.T) {
	g, err := New(Config{
		OpenAIKey:       "k-test",
		EmbeddingModel:  "m-test",
		ChunkSize:       123,
		ChunkOverlap:    7,
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
}
