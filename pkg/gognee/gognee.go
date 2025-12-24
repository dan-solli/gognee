// Package gognee provides a knowledge graph memory system for AI assistants
package gognee

import (
	"github.com/dan-solli/gognee/pkg/chunker"
	"github.com/dan-solli/gognee/pkg/embeddings"
	"github.com/dan-solli/gognee/pkg/llm"
)

// Config holds configuration for the Gognee system
type Config struct {
	// OpenAI API key for embeddings and LLM
	OpenAIKey string

	// Embedding model (default: "text-embedding-3-small")
	EmbeddingModel string

	// LLM model for entity extraction (default: "gpt-4o-mini")
	LLMModel string

	// Chunk size in tokens (default: 512)
	ChunkSize int

	// Chunk overlap in tokens (default: 50)
	ChunkOverlap int
}

// Gognee is the main entry point for the memory system
type Gognee struct {
	config     Config
	chunker    *chunker.Chunker
	embeddings embeddings.EmbeddingClient
	llm        llm.LLMClient
}

// New creates a new Gognee instance
func New(cfg Config) (*Gognee, error) {
	// Apply defaults
	if cfg.ChunkSize == 0 {
		cfg.ChunkSize = 512
	}
	if cfg.ChunkOverlap == 0 {
		cfg.ChunkOverlap = 50
	}

	// Initialize chunker
	c := &chunker.Chunker{
		MaxTokens: cfg.ChunkSize,
		Overlap:   cfg.ChunkOverlap,
	}

	// Initialize embeddings client
	embeddingsClient := embeddings.NewOpenAIClient(cfg.OpenAIKey)
	if cfg.EmbeddingModel != "" {
		embeddingsClient.Model = cfg.EmbeddingModel
	}

	// Initialize LLM client
	llmClient := llm.NewOpenAILLM(cfg.OpenAIKey)
	if cfg.LLMModel != "" {
		llmClient.Model = cfg.LLMModel
	}

	return &Gognee{
		config:     cfg,
		chunker:    c,
		embeddings: embeddingsClient,
		llm:        llmClient,
	}, nil
}

// GetChunker returns the configured chunker
func (g *Gognee) GetChunker() *chunker.Chunker {
	return g.chunker
}

// GetEmbeddings returns the configured embeddings client
func (g *Gognee) GetEmbeddings() embeddings.EmbeddingClient {
	return g.embeddings
}

// GetLLM returns the configured LLM client
func (g *Gognee) GetLLM() llm.LLMClient {
	return g.llm
}
