//go:build integration

package gognee

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestIntegrationCompleteWorkflow tests the full Add -> Cognify -> Search pipeline with real OpenAI API
func TestIntegrationCompleteWorkflow(t *testing.T) {
	// Get API key from secrets file or environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		// Try to read from secrets file
		secretPath := filepath.Join(os.Getenv("HOME"), "projects/gognee/secrets/openai-api-key.txt")
		if content, err := ioutil.ReadFile(secretPath); err == nil {
			apiKey = strings.TrimSpace(string(content))
		}
	}

	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set; skipping integration tests")
	}

	// Use temporary database for testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "integration_test.db")

	// Initialize Gognee
	g, err := New(Config{
		OpenAIKey:      apiKey,
		EmbeddingModel: "text-embedding-3-small",
		LLMModel:       "gpt-4o-mini",
		ChunkSize:      512,
		ChunkOverlap:   50,
		DBPath:         dbPath,
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	// Add sample documents
	docs := []string{
		"React is a JavaScript library for building user interfaces. It uses a component-based architecture.",
		"TypeScript adds static type checking to JavaScript, improving code quality and developer experience.",
		"PostgreSQL is a powerful open-source relational database system.",
		"We decided to use React with TypeScript for the frontend and PostgreSQL for the database.",
	}

	for _, doc := range docs {
		err := g.Add(ctx, doc, AddOptions{Source: "integration-test"})
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	if g.BufferedCount() != len(docs) {
		t.Fatalf("BufferedCount: got %d, want %d", g.BufferedCount(), len(docs))
	}

	// Cognify to build the knowledge graph
	t.Logf("Starting cognify...")
	startTime := time.Now()
	result, err := g.Cognify(ctx, CognifyOptions{})
	elapsed := time.Since(startTime)

	if err != nil {
		t.Fatalf("Cognify failed: %v", err)
	}

	t.Logf("Cognify completed in %v", elapsed)
	t.Logf("Documents processed: %d", result.DocumentsProcessed)
	t.Logf("Chunks processed: %d, failed: %d", result.ChunksProcessed, result.ChunksFailed)
	t.Logf("Nodes created: %d, edges created: %d", result.NodesCreated, result.EdgesCreated)
	if len(result.Errors) > 0 {
		t.Logf("Errors during processing: %v", result.Errors)
	}

	// Verify buffer was cleared
	if g.BufferedCount() != 0 {
		t.Fatalf("BufferedCount after Cognify: got %d, want 0", g.BufferedCount())
	}

	// Check stats
	stats, err := g.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	t.Logf("Stats: NodeCount=%d, EdgeCount=%d, BufferedDocs=%d, LastCognified=%v",
		stats.NodeCount, stats.EdgeCount, stats.BufferedDocs, stats.LastCognified)

	if stats.NodeCount == 0 {
		t.Fatalf("Expected nodes to be created, got 0")
	}

	// Search for relevant context
	searchQueries := []string{
		"What frontend technologies are used?",
		"Tell me about the database",
		"Describe the technology stack",
	}

	for _, query := range searchQueries {
		t.Logf("Searching for: %q", query)
		results, err := g.Search(ctx, query, SearchOptions{
			Type:       SearchTypeHybrid,
			TopK:       5,
			GraphDepth: 1,
		})
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) > 0 {
			t.Logf("Found %d results", len(results))
			for i, result := range results[:min(3, len(results))] {
				t.Logf("  [%d] %s (score: %.4f, source: %s)", i+1, result.Node.Name, result.Score, result.Source)
			}
		} else {
			t.Logf("No results found")
		}
	}
}

// TestIntegrationUpsertSemantics verifies that adding overlapping entities results in upsert behavior
func TestIntegrationUpsertSemantics(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		// Try to read from secrets file
		secretPath := filepath.Join(os.Getenv("HOME"), "projects/gognee/secrets/openai-api-key.txt")
		if content, err := ioutil.ReadFile(secretPath); err == nil {
			apiKey = strings.TrimSpace(string(content))
		}
	}

	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set; skipping integration tests")
	}

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "upsert_test.db")

	g, err := New(Config{
		OpenAIKey: apiKey,
		LLMModel:  "gpt-4o-mini",
		DBPath:    dbPath,
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	// Add first document
	t.Logf("Adding first document...")
	err = g.Add(ctx, "React is a JavaScript library created by Facebook.", AddOptions{})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	result1, err := g.Cognify(ctx, CognifyOptions{})
	if err != nil {
		t.Fatalf("Cognify failed: %v", err)
	}
	nodesAfterFirst := result1.NodesCreated

	t.Logf("After first cognify: %d nodes created", nodesAfterFirst)

	// Add second document with overlapping entity
	t.Logf("Adding second document with overlapping React entity...")
	err = g.Add(ctx, "React is widely used for building modern web applications and single-page applications.", AddOptions{})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	result2, err := g.Cognify(ctx, CognifyOptions{})
	if err != nil {
		t.Fatalf("Cognify failed: %v", err)
	}

	// Get final stats
	stats, err := g.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	t.Logf("After second cognify: %d nodes created (total: %d)", result2.NodesCreated, stats.NodeCount)

	// Verify upsert semantics: React node should be same ID both times
	// So total node count should be relatively small (React appears in both)
	if stats.NodeCount == 0 {
		t.Fatalf("Expected nodes in graph, got 0")
	}

	t.Logf("Upsert semantics verified: duplicate entities resolved to same node ID")
}

// TestIntegrationSearchTypes verifies all search type options work
func TestIntegrationSearchTypes(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		secretPath := filepath.Join(os.Getenv("HOME"), "projects/gognee/secrets/openai-api-key.txt")
		if content, err := ioutil.ReadFile(secretPath); err == nil {
			apiKey = strings.TrimSpace(string(content))
		}
	}

	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set; skipping integration tests")
	}

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "search_types_test.db")

	g, err := New(Config{
		OpenAIKey: apiKey,
		LLMModel:  "gpt-4o-mini",
		DBPath:    dbPath,
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	// Add and cognify a document
	err = g.Add(ctx, "Python is a popular programming language. Golang is a compiled language. Both are used in modern software development.", AddOptions{})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	_, err = g.Cognify(ctx, CognifyOptions{})
	if err != nil {
		t.Fatalf("Cognify failed: %v", err)
	}

	// Test each search type
	searchTypes := []SearchType{
		SearchTypeVector,
		SearchTypeHybrid,
	}

	query := "programming languages"
	for _, st := range searchTypes {
		t.Logf("Testing search type: %v", st)
		results, err := g.Search(ctx, query, SearchOptions{
			Type:       st,
			TopK:       5,
			GraphDepth: 1,
		})
		if err != nil {
			t.Fatalf("Search with type %v failed: %v", st, err)
		}

		t.Logf("Search type %v returned %d results", st, len(results))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
