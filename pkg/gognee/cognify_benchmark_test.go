package gognee

import (
	"context"
	"testing"
	"time"

	"github.com/dan-solli/gognee/pkg/extraction"
)

// mockEmbeddingClientWithLatency simulates API latency for benchmarking
type mockEmbeddingClientWithLatency struct {
	latency time.Duration
}

func (m *mockEmbeddingClientWithLatency) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	time.Sleep(m.latency) // Simulate single batch API call latency
	results := make([][]float32, len(texts))
	for i := range texts {
		results[i] = []float32{0.1, 0.2, 0.3}
	}
	return results, nil
}

func (m *mockEmbeddingClientWithLatency) EmbedOne(ctx context.Context, text string) ([]float32, error) {
	time.Sleep(m.latency) // Simulate per-call API latency
	return []float32{0.1, 0.2, 0.3}, nil
}

// mockLLMWithLatency simulates LLM API latency
type mockLLMWithLatency struct {
	latency time.Duration
}

func (m *mockLLMWithLatency) Complete(ctx context.Context, prompt string) (string, error) {
	time.Sleep(m.latency)
	return "", nil
}

func (m *mockLLMWithLatency) CompleteWithSchema(ctx context.Context, prompt string, result interface{}) error {
	time.Sleep(m.latency)

	// Return mock entities or relations based on result type
	switch v := result.(type) {
	case *[]extraction.Entity:
		// Return mock entities (16 entities to match real-world scenario)
		*v = make([]extraction.Entity, 16)
		for i := range *v {
			(*v)[i] = extraction.Entity{
				Name:        "Entity" + string(rune('A'+i)),
				Type:        "Concept",
				Description: "Description for entity",
			}
		}
	case *[]extraction.Triplet:
		// Return mock triplets
		*v = []extraction.Triplet{
			{Subject: "EntityA", Relation: "RELATES_TO", Object: "EntityB"},
			{Subject: "EntityB", Relation: "DEPENDS_ON", Object: "EntityC"},
		}
	}

	return nil
}

// BenchmarkCognify_BatchEmbeddings measures Cognify performance with batch embeddings
// Simulates realistic scenario: 16 entities, 100ms per API call
func BenchmarkCognify_BatchEmbeddings(b *testing.B) {
	// Skip for now - use standard `New()` approach in benchmarks or refactor
	// to properly setup mock environment
	b.Skip("Benchmark needs refactoring to work with mock clients")

	cfg := Config{
		ChunkSize:    512,
		ChunkOverlap: 50,
		DBPath:       ":memory:",
	}

	// Mock clients with simulated latency (100ms per API call)
	embClient := &mockEmbeddingClientWithLatency{latency: 100 * time.Millisecond}
	llmClient := &mockLLMWithLatency{latency: 100 * time.Millisecond}

	g, err := NewWithClients(cfg, embClient, llmClient)
	if err != nil {
		b.Fatalf("Failed to create Gognee: %v", err)
	}
	defer g.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Add document to buffer
		text := "This is a test document for benchmarking. It contains enough text to trigger entity extraction and embedding generation. The document discusses multiple concepts and their relationships."
		if err := g.Add(context.Background(), text, AddOptions{Source: "benchmark"}); err != nil {
			b.Fatalf("Failed to add document: %v", err)
		}

		result, err := g.Cognify(context.Background(), CognifyOptions{})
		if err != nil {
			b.Fatalf("Cognify failed: %v", err)
		}
		if result.NodesCreated == 0 {
			b.Fatalf("Expected nodes to be created, got 0")
		}
	}
}

// BenchmarkCognify_BatchEmbeddings_Parallel benchmarks concurrent Cognify operations
func BenchmarkCognify_BatchEmbeddings_Parallel(b *testing.B) {
	// Skip for now - use standard `New()` approach in benchmarks
	b.Skip("Benchmark needs refactoring to work with mock clients")

	cfg := Config{
		ChunkSize:    512,
		ChunkOverlap: 50,
		DBPath:       ":memory:",
	}

	embClient := &mockEmbeddingClientWithLatency{latency: 100 * time.Millisecond}
	llmClient := &mockLLMWithLatency{latency: 100 * time.Millisecond}

	g, err := NewWithClients(cfg, embClient, llmClient)
	if err != nil {
		b.Fatalf("Failed to create Gognee: %v", err)
	}
	defer g.Close()

	text := "This is a test document for benchmarking. It contains enough text to trigger entity extraction and embedding generation."

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := g.Add(context.Background(), text, AddOptions{Source: "benchmark"}); err != nil {
				b.Fatalf("Failed to add document: %v", err)
			}

			_, err := g.Cognify(context.Background(), CognifyOptions{})
			if err != nil {
				b.Fatalf("Cognify failed: %v", err)
			}
		}
	})
}
