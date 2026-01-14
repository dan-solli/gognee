//go:build metrics

package gognee

import (
	"context"
	"testing"
	"time"

	"github.com/dan-solli/gognee/pkg/extraction"
	"github.com/dan-solli/gognee/pkg/metrics"
)

// MockSlowLLMClient simulates realistic API latency
type MockSlowLLMClient struct {
	MockLLMClient
	LatencyMs int
}

func (m *MockSlowLLMClient) CompleteWithSchema(ctx context.Context, prompt string, schema interface{}) error {
	// Simulate realistic API latency (100-500ms typical for OpenAI)
	time.Sleep(time.Duration(m.LatencyMs) * time.Millisecond)
	return m.MockLLMClient.CompleteWithSchema(ctx, prompt, schema)
}

// MockSlowEmbeddingClient simulates realistic embedding API latency
type MockSlowEmbeddingClient struct {
	MockEmbeddingClient
	LatencyMs int
}

func (m *MockSlowEmbeddingClient) EmbedOne(ctx context.Context, text string) ([]float32, error) {
	// Simulate realistic API latency (50-200ms typical)
	time.Sleep(time.Duration(m.LatencyMs) * time.Millisecond)
	return m.MockEmbeddingClient.EmbedOne(ctx, text)
}

// BenchmarkRealisticCognify_NoMetrics benchmarks Cognify with realistic API latency, no metrics
func BenchmarkRealisticCognify_NoMetrics(b *testing.B) {
	ctx := context.Background()
	g := setupRealisticBenchmark(b, false, 100, 50)
	defer g.Close()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		_ = g.Add(ctx, "Quantum computing leverages quantum mechanics for computation.", AddOptions{})
		b.StartTimer()

		_, err := g.Cognify(ctx, CognifyOptions{})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRealisticCognify_WithMetrics benchmarks Cognify with realistic API latency, with metrics
func BenchmarkRealisticCognify_WithMetrics(b *testing.B) {
	ctx := context.Background()
	g := setupRealisticBenchmark(b, true, 100, 50)
	defer g.Close()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		_ = g.Add(ctx, "Quantum computing leverages quantum mechanics for computation.", AddOptions{})
		b.StartTimer()

		_, err := g.Cognify(ctx, CognifyOptions{})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// setupRealisticBenchmark creates a Gognee instance with realistic API latency simulation
func setupRealisticBenchmark(b *testing.B, withMetrics bool, llmLatencyMs, embedLatencyMs int) *Gognee {
	b.Helper()

	g, err := New(Config{
		DBPath: ":memory:",
	})
	if err != nil {
		b.Fatalf("Failed to create Gognee instance: %v", err)
	}

	// Inject slow mock clients to simulate real-world API latency
	mockLLM := &MockSlowLLMClient{
		LatencyMs: llmLatencyMs,
	}
	mockEmbed := &MockSlowEmbeddingClient{
		LatencyMs: embedLatencyMs,
	}

	g.llm = mockLLM
	g.embeddings = mockEmbed
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)

	if withMetrics {
		g.metricsCollector = metrics.NewCollector()
	}

	return g
}
