package gognee

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/dan-solli/gognee/pkg/extraction"
	"github.com/dan-solli/gognee/pkg/search"
)

// FakeEmbeddingClient returns deterministic embeddings based on text hash
type FakeEmbeddingClient struct{}

func (f *FakeEmbeddingClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := f.EmbedOne(ctx, text)
		if err != nil {
			return nil, err
		}
		results[i] = emb
	}
	return results, nil
}

func (f *FakeEmbeddingClient) EmbedOne(ctx context.Context, text string) ([]float32, error) {
	// Generate deterministic 1536-dimension vector from text hash
	hash := sha256.Sum256([]byte(text))
	embedding := make([]float32, 1536)
	
	// Use hash bytes to seed the embedding
	for i := 0; i < 1536; i++ {
		byteIdx := (i * 4) % len(hash)
		val := binary.BigEndian.Uint32(hash[byteIdx:byteIdx+4])
		// Normalize to [-1, 1] range
		embedding[i] = float32((float64(val)/float64(^uint32(0)))*2 - 1)
	}
	
	return embedding, nil
}

// FakeLLMClient returns canned entity/relation responses
type FakeLLMClient struct{}

func (f *FakeLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	// Return deterministic response based on prompt type
	if len(prompt) > 100 && prompt[:50] == "Extract entities from the following text. Return" {
		// Entity extraction prompt
		return `[
			{"name": "Alice", "type": "Person", "description": "A software engineer"},
			{"name": "Project X", "type": "Project", "description": "A new initiative"},
			{"name": "Database", "type": "Technology", "description": "Storage system"}
		]`, nil
	}
	
	// Relation extraction prompt
	return `[
		{"subject": "Alice", "relation": "works_on", "object": "Project X"},
		{"subject": "Project X", "relation": "uses", "object": "Database"}
	]`, nil
}

func (f *FakeLLMClient) CompleteWithSchema(ctx context.Context, prompt string, schema any) error {
	// Not used in benchmarks, but required by interface
	return fmt.Errorf("CompleteWithSchema not implemented in FakeLLMClient")
}

// Benchmark: Cognify on empty graph
func BenchmarkCognify_Empty(b *testing.B) {
	ctx := context.Background()
	
	cfg := Config{
		OpenAIKey:        "fake-key",
		EmbeddingModel:   "fake-model",
		LLMModel:         "fake-model",
		ChunkSize:        512,
		ChunkOverlap:     50,
		DBPath:           ":memory:",
		DecayEnabled:     false,
	}
	
	g, err := New(cfg)
	if err != nil {
		b.Fatalf("Failed to create gognee: %v", err)
	}
	defer g.Close()
	
	// Replace with fake clients
	g.embeddings = &FakeEmbeddingClient{}
	g.llm = &FakeLLMClient{}
	g.entityExtractor = extraction.NewEntityExtractor(&FakeLLMClient{})
	g.relationExtractor = extraction.NewRelationExtractor(&FakeLLMClient{})
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		g.Add(ctx, fmt.Sprintf("Test document %d with some content about software development", i), AddOptions{})
		_, err := g.Cognify(ctx, CognifyOptions{})
		if err != nil {
			b.Fatalf("Cognify failed: %v", err)
		}
	}
}

// Benchmark: Cognify with 100 pre-seeded memories
func BenchmarkCognify_100Memories(b *testing.B) {
	ctx := context.Background()
	
	cfg := Config{
		OpenAIKey:        "fake-key",
		EmbeddingModel:   "fake-model",
		LLMModel:         "fake-model",
		ChunkSize:        512,
		ChunkOverlap:     50,
		DBPath:           ":memory:",
		DecayEnabled:     false,
	}
	
	g, err := New(cfg)
	if err != nil {
		b.Fatalf("Failed to create gognee: %v", err)
	}
	defer g.Close()
	
	// Replace with fake clients
	g.embeddings = &FakeEmbeddingClient{}
	g.llm = &FakeLLMClient{}
	g.entityExtractor = extraction.NewEntityExtractor(&FakeLLMClient{})
	g.relationExtractor = extraction.NewRelationExtractor(&FakeLLMClient{})
	
	// Pre-seed with 100 memories
	for i := 0; i < 100; i++ {
		_, err := g.AddMemory(ctx, MemoryInput{
			Topic:   fmt.Sprintf("Memory %d", i),
			Context: fmt.Sprintf("Content about topic %d with details", i),
		})
		if err != nil {
			b.Fatalf("Failed to seed memory: %v", err)
		}
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		g.Add(ctx, fmt.Sprintf("New document %d", i), AddOptions{})
		_, err := g.Cognify(ctx, CognifyOptions{})
		if err != nil {
			b.Fatalf("Cognify failed: %v", err)
		}
	}
}

// Benchmark: Cognify with 1000 pre-seeded memories (stress test)
func BenchmarkCognify_1000Memories(b *testing.B) {
	ctx := context.Background()
	
	cfg := Config{
		OpenAIKey:        "fake-key",
		EmbeddingModel:   "fake-model",
		LLMModel:         "fake-model",
		ChunkSize:        512,
		ChunkOverlap:     50,
		DBPath:           ":memory:",
		DecayEnabled:     false,
	}
	
	g, err := New(cfg)
	if err != nil {
		b.Fatalf("Failed to create gognee: %v", err)
	}
	defer g.Close()
	
	// Replace with fake clients
	g.embeddings = &FakeEmbeddingClient{}
	g.llm = &FakeLLMClient{}
	g.entityExtractor = extraction.NewEntityExtractor(&FakeLLMClient{})
	g.relationExtractor = extraction.NewRelationExtractor(&FakeLLMClient{})
	
	// Pre-seed with 1000 memories
	b.Log("Seeding 1000 memories...")
	for i := 0; i < 1000; i++ {
		_, err := g.AddMemory(ctx, MemoryInput{
			Topic:   fmt.Sprintf("Memory %d", i),
			Context: fmt.Sprintf("Content about topic %d with details", i),
		})
		if err != nil {
			b.Fatalf("Failed to seed memory: %v", err)
		}
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		g.Add(ctx, fmt.Sprintf("New document %d", i), AddOptions{})
		_, err := g.Cognify(ctx, CognifyOptions{})
		if err != nil {
			b.Fatalf("Cognify failed: %v", err)
		}
	}
}

// Benchmark: Search on empty graph
func BenchmarkSearch_Empty(b *testing.B) {
	ctx := context.Background()
	
	cfg := Config{
		OpenAIKey:        "fake-key",
		EmbeddingModel:   "fake-model",
		LLMModel:         "fake-model",
		ChunkSize:        512,
		ChunkOverlap:     50,
		DBPath:           ":memory:",
		DecayEnabled:     false,
	}
	
	g, err := New(cfg)
	if err != nil {
		b.Fatalf("Failed to create gognee: %v", err)
	}
	defer g.Close()
	
	// Replace with fake clients
	fakeEmbed := &FakeEmbeddingClient{}
	g.embeddings = fakeEmbed
	
	// Rebuild searcher with fake embedding client
	baseSearcher := search.NewHybridSearcher(fakeEmbed, g.vectorStore, g.graphStore)
	g.searcher = search.NewDecayingSearcher(baseSearcher, g.graphStore, false, 30.0, "last_accessed")
	
	opts := search.SearchOptions{
		Type: search.SearchTypeVector,
		TopK: 10,
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := g.Search(ctx, "test query", opts)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}

// Benchmark: Search with 100 pre-seeded memories
func BenchmarkSearch_100Memories(b *testing.B) {
	ctx := context.Background()
	
	cfg := Config{
		OpenAIKey:        "fake-key",
		EmbeddingModel:   "fake-model",
		LLMModel:         "fake-model",
		ChunkSize:        512,
		ChunkOverlap:     50,
		DBPath:           ":memory:",
		DecayEnabled:     false,
	}
	
	g, err := New(cfg)
	if err != nil {
		b.Fatalf("Failed to create gognee: %v", err)
	}
	defer g.Close()
	
	// Replace with fake clients
	fakeEmbed := &FakeEmbeddingClient{}
	g.embeddings = fakeEmbed
	g.llm = &FakeLLMClient{}
	g.entityExtractor = extraction.NewEntityExtractor(&FakeLLMClient{})
	g.relationExtractor = extraction.NewRelationExtractor(&FakeLLMClient{})
	
	// Rebuild searcher with fake embedding client
	baseSearcher := search.NewHybridSearcher(fakeEmbed, g.vectorStore, g.graphStore)
	g.searcher = search.NewDecayingSearcher(baseSearcher, g.graphStore, false, 30.0, "last_accessed")
	
	// Pre-seed with 100 memories
	for i := 0; i < 100; i++ {
		_, err := g.AddMemory(ctx, MemoryInput{
			Topic:   fmt.Sprintf("Memory %d", i),
			Context: fmt.Sprintf("Content about topic %d with details", i),
		})
		if err != nil {
			b.Fatalf("Failed to seed memory: %v", err)
		}
	}
	
	opts := search.SearchOptions{
		Type: search.SearchTypeVector,
		TopK: 10,
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := g.Search(ctx, "software development", opts)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}

// Benchmark: Search with 1000 pre-seeded memories (stress test)
func BenchmarkSearch_1000Memories(b *testing.B) {
	ctx := context.Background()
	
	cfg := Config{
		OpenAIKey:        "fake-key",
		EmbeddingModel:   "fake-model",
		LLMModel:         "fake-model",
		ChunkSize:        512,
		ChunkOverlap:     50,
		DBPath:           ":memory:",
		DecayEnabled:     false,
	}
	
	g, err := New(cfg)
	if err != nil {
		b.Fatalf("Failed to create gognee: %v", err)
	}
	defer g.Close()
	
	// Replace with fake clients
	fakeEmbed := &FakeEmbeddingClient{}
	g.embeddings = fakeEmbed
	g.llm = &FakeLLMClient{}
	g.entityExtractor = extraction.NewEntityExtractor(&FakeLLMClient{})
	g.relationExtractor = extraction.NewRelationExtractor(&FakeLLMClient{})
	
	// Rebuild searcher with fake embedding client
	baseSearcher := search.NewHybridSearcher(fakeEmbed, g.vectorStore, g.graphStore)
	g.searcher = search.NewDecayingSearcher(baseSearcher, g.graphStore, false, 30.0, "last_accessed")
	
	// Pre-seed with 1000 memories
	b.Log("Seeding 1000 memories...")
	for i := 0; i < 1000; i++ {
		_, err := g.AddMemory(ctx, MemoryInput{
			Topic:   fmt.Sprintf("Memory %d", i),
			Context: fmt.Sprintf("Content about topic %d with details", i),
		})
		if err != nil {
			b.Fatalf("Failed to seed memory: %v", err)
		}
	}
	
	opts := search.SearchOptions{
		Type: search.SearchTypeVector,
		TopK: 10,
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := g.Search(ctx, "software development", opts)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}
