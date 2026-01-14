//go:build metrics

package gognee

import (
	"context"
	"testing"

	"github.com/dan-solli/gognee/pkg/extraction"
	"github.com/dan-solli/gognee/pkg/metrics"
	"github.com/dan-solli/gognee/pkg/search"
)

// BenchmarkCognify_NoMetrics benchmarks Cognify without metrics collection
func BenchmarkCognify_NoMetrics(b *testing.B) {
	ctx := context.Background()
	g := setupBenchmarkGognee(b, false)
	defer g.Close()

	// Add text to buffer
	if err := g.Add(ctx, "The quick brown fox jumps over the lazy dog. Machine learning is fascinating.", AddOptions{}); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := g.Cognify(ctx, CognifyOptions{})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCognify_WithMetrics benchmarks Cognify with metrics collection enabled
func BenchmarkCognify_WithMetrics(b *testing.B) {
	ctx := context.Background()
	g := setupBenchmarkGognee(b, true)
	defer g.Close()

	// Add text to buffer
	if err := g.Add(ctx, "The quick brown fox jumps over the lazy dog. Machine learning is fascinating.", AddOptions{}); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := g.Cognify(ctx, CognifyOptions{})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSearch_NoMetrics benchmarks Search without metrics collection
func BenchmarkSearch_NoMetrics(b *testing.B) {
	ctx := context.Background()
	g := setupBenchmarkGognee(b, false)
	defer g.Close()

	// Setup: Add and cognify some data
	if err := g.Add(ctx, "The quick brown fox jumps over the lazy dog. Machine learning is fascinating.", AddOptions{}); err != nil {
		b.Fatal(err)
	}
	_, err := g.Cognify(ctx, CognifyOptions{})
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := g.Search(ctx, "machine learning", search.SearchOptions{TopK: 5})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSearch_WithMetrics benchmarks Search with metrics collection enabled
func BenchmarkSearch_WithMetrics(b *testing.B) {
	ctx := context.Background()
	g := setupBenchmarkGognee(b, true)
	defer g.Close()

	// Setup: Add and cognify some data
	if err := g.Add(ctx, "The quick brown fox jumps over the lazy dog. Machine learning is fascinating.", AddOptions{}); err != nil {
		b.Fatal(err)
	}
	_, err := g.Cognify(ctx, CognifyOptions{})
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := g.Search(ctx, "machine learning", search.SearchOptions{TopK: 5})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// setupBenchmarkGognee creates a Gognee instance for benchmarking
func setupBenchmarkGognee(b *testing.B, withMetrics bool) *Gognee {
	b.Helper()

	// Create temporary in-memory database
	g, err := New(Config{
		DBPath: ":memory:",
	})
	if err != nil {
		b.Fatalf("Failed to create Gognee instance: %v", err)
	}

	// Inject mock clients (package-scope access)
	mockLLM := &MockLLMClient{}
	mockEmbed := &MockEmbeddingClient{}
	g.llm = mockLLM
	g.embeddings = mockEmbed
	// Re-create extractors with mock LLM
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)
	// Re-create searcher with mock embedding client
	g.searcher = search.NewHybridSearcher(mockEmbed, g.vectorStore, g.graphStore)

	// Optionally inject metrics collector
	if withMetrics {
		g.metricsCollector = metrics.NewCollector()
	}

	return g
}
