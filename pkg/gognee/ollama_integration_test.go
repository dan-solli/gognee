//go:build integration_ollama

// Hybrid integration tests:
// - Ollama for embeddings (nomic-embed-text) - fast locally
// - OpenAI for LLM extraction (gpt-4o-mini) - fast and production-like
// Run with: go test -tags=integration_ollama -v ./pkg/gognee -timeout 10m

package gognee

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/dan-solli/gognee/pkg/embeddings"
	"github.com/dan-solli/gognee/pkg/llm"
	"github.com/dan-solli/gognee/pkg/search"
)

const (
	ollamaURL        = "http://localhost:11434"
	ollamaEmbedModel = "nomic-embed-text"
	openAILLMModel   = "gpt-4o-mini" // Fast and cheap for extraction
)

func ollamaAvailable() bool {
	client := embeddings.NewOllamaClient(ollamaURL, ollamaEmbedModel)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.EmbedOne(ctx, "test")
	return err == nil
}

// TestOpenAI_EmbeddingLatency compares OpenAI embedding latency to local Ollama
func TestOpenAI_EmbeddingLatency(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	client := embeddings.NewOpenAIClient(apiKey)
	ctx := context.Background()

	// Warm up
	_, _ = client.EmbedOne(ctx, "warmup")

	// Time single embeddings
	var times []time.Duration
	for i := 0; i < 10; i++ {
		start := time.Now()
		_, err := client.EmbedOne(ctx, fmt.Sprintf("Test sentence number %d for embedding latency measurement.", i))
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("EmbedOne failed: %v", err)
		}
		times = append(times, elapsed)
	}

	var total time.Duration
	var max time.Duration
	for _, d := range times {
		total += d
		if d > max {
			max = d
		}
	}
	avg := total / time.Duration(len(times))

	t.Logf("OpenAI Embedding latency (n=10): avg=%v, max=%v", avg, max)

	if avg > 500*time.Millisecond {
		t.Errorf("SLOW: Average embedding latency %v exceeds 500ms", avg)
	}
}

func TestOllama_EmbeddingLatency(t *testing.T) {
	if !ollamaAvailable() {
		t.Skip("Ollama not available at " + ollamaURL)
	}

	client := embeddings.NewOllamaClient(ollamaURL, ollamaEmbedModel)
	ctx := context.Background()

	// Warm up
	_, _ = client.EmbedOne(ctx, "warmup")

	// Time single embeddings
	var times []time.Duration
	for i := 0; i < 10; i++ {
		start := time.Now()
		_, err := client.EmbedOne(ctx, fmt.Sprintf("Test sentence number %d for embedding latency measurement.", i))
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("EmbedOne failed: %v", err)
		}
		times = append(times, elapsed)
	}

	var total time.Duration
	var max time.Duration
	for _, d := range times {
		total += d
		if d > max {
			max = d
		}
	}
	avg := total / time.Duration(len(times))

	t.Logf("Embedding latency (n=10): avg=%v, max=%v", avg, max)

	if avg > 500*time.Millisecond {
		t.Errorf("SLOW: Average embedding latency %v exceeds 500ms", avg)
	}
}

func TestOpenAI_LLMLatency(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	client := llm.NewOpenAILLM(apiKey)
	client.Model = openAILLMModel
	ctx := context.Background()

	// Warm up
	_, _ = client.Complete(ctx, "Say hi.")

	// Time completions
	var times []time.Duration
	prompts := []string{
		"Say hello in one word.",
		"What is 2+2? Answer with just the number.",
		"Name a color.",
	}

	for _, p := range prompts {
		start := time.Now()
		_, err := client.Complete(ctx, p)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("Complete failed: %v", err)
		}
		times = append(times, elapsed)
		t.Logf("  Prompt: %q -> %v", p, elapsed)
	}

	var total time.Duration
	for _, d := range times {
		total += d
	}
	avg := total / time.Duration(len(times))
	t.Logf("LLM completion latency (n=%d): avg=%v", len(times), avg)
}

func TestOpenAI_EntityExtraction(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	client := llm.NewOpenAILLM(apiKey)
	client.Model = openAILLMModel
	ctx := context.Background()

	type Entity struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	type Entities struct {
		Entities []Entity `json:"entities"`
	}

	prompt := `Extract entities from this text. Return JSON: {"entities": [{"name": "...", "type": "person|organization|location|concept"}]}

Text: "John Smith works at Microsoft in Seattle on the Azure cloud platform."

Return only valid JSON:`

	start := time.Now()
	var entities Entities
	err := client.CompleteWithSchema(ctx, prompt, &entities)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Entity extraction failed: %v", err)
	}

	t.Logf("Entity extraction took %v, found %d entities:", elapsed, len(entities.Entities))
	for _, e := range entities.Entities {
		t.Logf("  - %s (%s)", e.Name, e.Type)
	}

	if elapsed > 10*time.Second {
		t.Errorf("SLOW: Entity extraction took %v, exceeds 10s", elapsed)
	}
}

func TestOpenAI_CognifyPipeline(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	// Create temp directory for test DB
	tmpDir, err := os.MkdirTemp("", "gognee-openai-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := tmpDir + "/test.db"

	// Use OpenAI for BOTH embeddings and LLM (production-like)
	embClient := embeddings.NewOpenAIClient(apiKey)
	llmClient := llm.NewOpenAILLM(apiKey)
	llmClient.Model = openAILLMModel

	cfg := Config{
		DBPath:       dbPath,
		ChunkSize:    256,
		ChunkOverlap: 25,
	}

	g, err := NewWithClients(cfg, embClient, llmClient)
	if err != nil {
		t.Fatalf("Failed to create gognee: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	testText := `The development team decided to use PostgreSQL for the database 
	and React for the frontend. The project manager Alice reviewed the architecture 
	with lead developer Bob. They discussed integration with external services 
	including Stripe for payments and SendGrid for email notifications.`

	t.Log("=== OPENAI COGNIFY PIPELINE PROFILING ===")

	// Add to buffer then cognify
	if err := g.Add(ctx, testText, AddOptions{Source: "test"}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	start := time.Now()
	result, err := g.Cognify(ctx, CognifyOptions{TraceEnabled: true})
	totalElapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Cognify failed: %v", err)
	}

	t.Logf("Total Cognify time: %v", totalElapsed)
	t.Logf("Documents processed: %d", result.DocumentsProcessed)
	t.Logf("Chunks processed: %d", result.ChunksProcessed)
	t.Logf("Nodes created: %d", result.NodesCreated)
	t.Logf("Edges created: %d", result.EdgesCreated)

	if result.Trace != nil {
		t.Logf("\nTiming breakdown (trace):")
		for _, span := range result.Trace.Spans {
			pct := float64(span.DurationMs) / float64(result.Trace.TotalDurationMs) * 100
			status := "OK"
			if !span.OK {
				status = "FAIL: " + span.Error
			}
			t.Logf("  %-20s %6d ms (%5.1f%%)  %s", span.Name, span.DurationMs, pct, status)
		}
		t.Logf("  %-20s %6d ms", "TOTAL", result.Trace.TotalDurationMs)
	}

	// Production timeout is 30s
	if totalElapsed > 30*time.Second {
		t.Errorf("TIMEOUT RISK: Cognify took %v, exceeds 30s production timeout", totalElapsed)
	}

	t.Log("==========================================")
}

func TestHybrid_SearchPipeline(t *testing.T) {
	if !ollamaAvailable() {
		t.Skip("Ollama not available at " + ollamaURL)
	}
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	tmpDir, err := os.MkdirTemp("", "gognee-ollama-search-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := tmpDir + "/test.db"

	embClient := embeddings.NewOllamaClient(ollamaURL, ollamaEmbedModel)
	llmClient := llm.NewOpenAILLM(apiKey)
	llmClient.Model = openAILLMModel

	cfg := Config{DBPath: dbPath}
	g, err := NewWithClients(cfg, embClient, llmClient)
	if err != nil {
		t.Fatalf("Failed to create gognee: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	// Add test data
	docs := []string{
		"Implemented user authentication using JWT tokens and bcrypt password hashing.",
		"Configured PostgreSQL database with connection pooling via pgbouncer.",
		"Added React components for the dashboard with Redux state management.",
		"Set up CI/CD pipeline using GitHub Actions for automated testing.",
		"Integrated Stripe payment processing for subscription billing.",
	}

	t.Log("Adding test documents...")
	for i, doc := range docs {
		if err := g.Add(ctx, doc, AddOptions{Source: fmt.Sprintf("doc-%d", i)}); err != nil {
			t.Fatalf("Add doc %d failed: %v", i, err)
		}
	}

	start := time.Now()
	_, err = g.Cognify(ctx, CognifyOptions{})
	elapsed := time.Since(start)
	t.Logf("  Cognify %d docs: %v", len(docs), elapsed)

	// Search
	query := "What database technology was used?"

	t.Log("\n=== SEARCH PIPELINE PROFILING ===")
	searchStart := time.Now()
	results, err := g.Search(ctx, query, search.SearchOptions{TraceEnabled: true, TopK: 5})
	totalElapsed := time.Since(searchStart)

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	t.Logf("Total Search time: %v", totalElapsed)
	t.Logf("Results found: %d", len(results.Results))

	if results.Trace != nil {
		t.Logf("\nTiming breakdown (trace):")
		for _, span := range results.Trace.Spans {
			pct := float64(span.DurationMs) / float64(results.Trace.TotalDurationMs) * 100
			t.Logf("  %-20s %6d ms (%5.1f%%)", span.Name, span.DurationMs, pct)
		}
		t.Logf("  %-20s %6d ms", "TOTAL", results.Trace.TotalDurationMs)
	}

	for i, r := range results.Results {
		nodeName := "<nil>"
		if r.Node != nil {
			nodeName = r.Node.Name
		}
		t.Logf("  %d. Score=%.3f: %s", i+1, r.Score, nodeName)
	}

	if totalElapsed > 5*time.Second {
		t.Errorf("SLOW: Search took %v, exceeds 5s threshold", totalElapsed)
	}

	t.Log("=================================")
}

func TestHybrid_StressTest(t *testing.T) {
	if !ollamaAvailable() {
		t.Skip("Ollama not available at " + ollamaURL)
	}
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "gognee-stress-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := tmpDir + "/stress.db"

	embClient := embeddings.NewOllamaClient(ollamaURL, ollamaEmbedModel)
	llmClient := llm.NewOpenAILLM(apiKey)
	llmClient.Model = openAILLMModel

	cfg := Config{DBPath: dbPath}
	g, err := NewWithClients(cfg, embClient, llmClient)
	if err != nil {
		t.Fatalf("Failed to create gognee: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	const numDocs = 10 // Start small - this uses real LLM calls

	t.Logf("=== STRESS TEST: %d documents ===", numDocs)

	var cognifyTimes []time.Duration
	for i := 0; i < numDocs; i++ {
		text := fmt.Sprintf("Document %d discusses topic %d. It mentions important concepts like feature-%d and relates to milestone-%d. The content includes technical details about implementation and testing strategies.", i, i%5, i%3, i%7)

		if err := g.Add(ctx, text, AddOptions{Source: fmt.Sprintf("stress-%d", i)}); err != nil {
			t.Fatalf("Add %d failed: %v", i, err)
		}

		start := time.Now()
		result, err := g.Cognify(ctx, CognifyOptions{TraceEnabled: true})
		elapsed := time.Since(start)
		cognifyTimes = append(cognifyTimes, elapsed)

		if err != nil {
			t.Fatalf("Cognify %d failed: %v", i, err)
		}

		t.Logf("  Doc %d: %v (nodes=%d, edges=%d)", i+1, elapsed, result.NodesCreated, result.EdgesCreated)
	}

	// Stats
	var total time.Duration
	var max time.Duration
	for _, d := range cognifyTimes {
		total += d
		if d > max {
			max = d
		}
	}
	avg := total / time.Duration(len(cognifyTimes))

	t.Logf("\n=== STRESS TEST RESULTS ===")
	t.Logf("Documents: %d", numDocs)
	t.Logf("Avg Cognify: %v", avg)
	t.Logf("Max Cognify: %v", max)
	t.Logf("Total time: %v", total)

	// Search after load
	start := time.Now()
	results, err := g.Search(ctx, "implementation testing", search.SearchOptions{TraceEnabled: true, TopK: 5})
	searchElapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	t.Logf("Search time: %v (results: %d)", searchElapsed, len(results.Results))
	t.Logf("===========================")

	// Thresholds
	if avg > 30*time.Second {
		t.Errorf("PERFORMANCE PROBLEM: Avg Cognify %v exceeds 30s threshold", avg)
	}
	if max > 60*time.Second {
		t.Errorf("PERFORMANCE PROBLEM: Max Cognify %v exceeds 60s threshold", max)
	}
}
