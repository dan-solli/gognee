package gognee

import (
	"context"
	"errors"
	"testing"

	"github.com/dan-solli/gognee/pkg/embeddings"
	"github.com/dan-solli/gognee/pkg/extraction"
	"github.com/dan-solli/gognee/pkg/llm"
	"github.com/dan-solli/gognee/pkg/search"
	"github.com/dan-solli/gognee/pkg/store"
)

// MockEmbeddingClient provides deterministic embeddings for testing
type MockEmbeddingClient struct {
	CallCount int
}

func (m *MockEmbeddingClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	m.CallCount++
	result := make([][]float32, len(texts))
	for i, text := range texts {
		result[i] = deterministicEmbedding(text)
	}
	return result, nil
}

func (m *MockEmbeddingClient) EmbedOne(ctx context.Context, text string) ([]float32, error) {
	m.CallCount++
	return deterministicEmbedding(text), nil
}

// deterministicEmbedding creates a deterministic embedding from text
func deterministicEmbedding(text string) []float32 {
	// Simple hash-based embedding for testing
	hash := 0
	for _, ch := range text {
		hash = ((hash << 5) - hash) + int(ch)
	}

	embedding := make([]float32, 4) // Small embedding for testing
	embedding[0] = float32(hash%256) / 256.0
	embedding[1] = float32((hash/256)%256) / 256.0
	embedding[2] = float32((hash/65536)%256) / 256.0
	embedding[3] = float32((hash/16777216)%256) / 256.0
	return embedding
}

// MockLLMClient provides canned responses for testing
type MockLLMClient struct {
	EntityResponses   [][]extraction.Entity
	RelationResponses [][]extraction.Triplet
	CallCount         int
}

func (m *MockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	m.CallCount++
	return `[{"name": "test", "type": "Concept", "description": "test entity"}]`, nil
}

func (m *MockLLMClient) CompleteWithSchema(ctx context.Context, prompt string, schema interface{}) error {
	m.CallCount++

	// Determine what to return based on the target schema type.
	switch s := schema.(type) {
	case *[]extraction.Entity:
		if len(m.EntityResponses) > 0 {
			entities := m.EntityResponses[0]
			if len(m.EntityResponses) > 1 {
				m.EntityResponses = m.EntityResponses[1:]
			}
			*s = entities
			return nil
		}
		*s = []extraction.Entity{
			{Name: "TestEntity", Type: "Concept", Description: "A test entity"},
		}
	case *[]extraction.Triplet:
		if len(m.RelationResponses) > 0 {
			triplets := m.RelationResponses[0]
			if len(m.RelationResponses) > 1 {
				m.RelationResponses = m.RelationResponses[1:]
			}
			*s = triplets
			return nil
		}
		// Default: Return an empty slice (no relations) to avoid validation errors.
		// The relation extractor validates that subjects/objects reference known entities.
		*s = []extraction.Triplet{}
	}

	return nil
}

func TestNewAppliesDefaults(t *testing.T) {
	g, err := New(Config{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer g.Close()

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

	// New stores should be initialized
	if g.GetGraphStore() == nil {
		t.Fatalf("GetGraphStore returned nil")
	}
	if g.GetVectorStore() == nil {
		t.Fatalf("GetVectorStore returned nil")
	}
}

func TestNewRespectsConfig(t *testing.T) {
	g, err := New(Config{
		OpenAIKey:      "k-test",
		EmbeddingModel: "m-test",
		LLMModel:       "llm-test",
		ChunkSize:      123,
		ChunkOverlap:   7,
		DBPath:         ":memory:",
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer g.Close()

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

func TestNew_DecayDefaults(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	// Verify defaults are applied (Plan 022 M2: decay now defaults to ON)
	if !g.config.DecayEnabled {
		t.Error("DecayEnabled should default to true (Plan 022 M2)")
	}
	if g.config.DecayHalfLifeDays != 30 {
		t.Errorf("DecayHalfLifeDays: got %d, want 30", g.config.DecayHalfLifeDays)
	}
	if g.config.DecayBasis != "access" {
		t.Errorf("DecayBasis: got %q, want 'access'", g.config.DecayBasis)
	}
}

func TestNew_DecayValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid_decay_config",
			config: Config{
				DBPath:            ":memory:",
				DecayEnabled:      true,
				DecayHalfLifeDays: 15,
				DecayBasis:        "creation",
			},
			wantErr: false,
		},
		{
			name: "decay_default_half_life_when_zero",
			config: Config{
				DBPath:            ":memory:",
				DecayEnabled:      true,
				DecayHalfLifeDays: 0, // Should get default of 30
			},
			wantErr: false,
		},
		{
			name: "invalid_half_life_negative",
			config: Config{
				DBPath:            ":memory:",
				DecayEnabled:      true,
				DecayHalfLifeDays: -5,
			},
			wantErr: true,
			errMsg:  "DecayHalfLifeDays must be positive",
		},
		{
			name: "invalid_decay_basis",
			config: Config{
				DBPath:            ":memory:",
				DecayEnabled:      true,
				DecayHalfLifeDays: 30,
				DecayBasis:        "invalid",
			},
			wantErr: true,
			errMsg:  "DecayBasis must be 'access' or 'creation'",
		},
		// Note: "decay_disabled_ignores_invalid_config" test removed in Plan 022 M2
		// because DecayEnabled now defaults to true and cannot be easily disabled
		// due to Go's zero-value behavior for booleans.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := New(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Error message: got %q, want to contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error, got %v", err)
				}
				if g != nil {
					g.Close()
				}
			}
		})
	}
}

// TestNew_DecayDefaultsActivated verifies that decay features are enabled by default (Plan 022 M2).
func TestNew_DecayDefaultsActivated(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	// Verify decay is enabled by default
	if !g.config.DecayEnabled {
		t.Error("DecayEnabled should default to true (Plan 022 M2)")
	}

	// Verify AccessFrequencyEnabled is enabled by default
	if !g.config.AccessFrequencyEnabled {
		t.Error("AccessFrequencyEnabled should default to true (Plan 022 M2)")
	}

	// Verify ReferenceAccessCount defaults to 10
	if g.config.ReferenceAccessCount != 10 {
		t.Errorf("ReferenceAccessCount: got %d, want 10 (Plan 022 M2)", g.config.ReferenceAccessCount)
	}

	// Verify half-life still defaults to 30
	if g.config.DecayHalfLifeDays != 30 {
		t.Errorf("DecayHalfLifeDays: got %d, want 30", g.config.DecayHalfLifeDays)
	}

	// Verify decay basis still defaults to "access"
	if g.config.DecayBasis != "access" {
		t.Errorf("DecayBasis: got %q, want 'access'", g.config.DecayBasis)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestAddBuffersText(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	// Add text
	err = g.Add(ctx, "This is a test document", AddOptions{Source: "test-source"})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if g.BufferedCount() != 1 {
		t.Fatalf("BufferedCount: got %d, want 1", g.BufferedCount())
	}

	// Add another
	err = g.Add(ctx, "Another document", AddOptions{})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if g.BufferedCount() != 2 {
		t.Fatalf("BufferedCount: got %d, want 2", g.BufferedCount())
	}
}

func TestAddRejectsEmpty(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	// Empty string should fail
	err = g.Add(ctx, "", AddOptions{})
	if err == nil {
		t.Fatalf("Add empty string: expected error, got nil")
	}

	// Whitespace-only should fail
	err = g.Add(ctx, "   ", AddOptions{})
	if err == nil {
		t.Fatalf("Add whitespace string: expected error, got nil")
	}
}

func TestGenerateDeterministicNodeID(t *testing.T) {
	tests := []struct {
		name     string
		nodeType string
		want     string
	}{
		{"Entity", "Concept", generateDeterministicNodeID("Entity", "Concept")},
		{"entity", "Concept", generateDeterministicNodeID("Entity", "Concept")},           // Case insensitive
		{"  Entity  ", "Concept", generateDeterministicNodeID("Entity", "Concept")},       // Trim
		{"Entity Name", "Concept", generateDeterministicNodeID("Entity Name", "Concept")}, // Spaces
	}

	// Verify determinism: same input -> same output
	for _, tt := range tests {
		id1 := generateDeterministicNodeID(tt.name, tt.nodeType)
		id2 := generateDeterministicNodeID(tt.name, tt.nodeType)
		if id1 != id2 {
			t.Fatalf("ID not deterministic for %q: %s != %s", tt.name, id1, id2)
		}
	}

	// Verify case insensitivity
	id1 := generateDeterministicNodeID("Test", "Concept")
	id2 := generateDeterministicNodeID("test", "Concept")
	if id1 != id2 {
		t.Fatalf("IDs should be case-insensitive: %s != %s", id1, id2)
	}

	// Verify different inputs -> different IDs
	id1 = generateDeterministicNodeID("Entity1", "Concept")
	id2 = generateDeterministicNodeID("Entity2", "Concept")
	if id1 == id2 {
		t.Fatalf("Different entities should have different IDs")
	}
}

func TestCognifyEmptyBuffer(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	// Cognify with empty buffer should return empty result, not error
	result, err := g.Cognify(ctx, CognifyOptions{})
	if err != nil {
		t.Fatalf("Cognify empty buffer failed: %v", err)
	}

	if result.DocumentsProcessed != 0 {
		t.Fatalf("DocumentsProcessed: got %d, want 0", result.DocumentsProcessed)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("Errors: got %v, want none", result.Errors)
	}
}

func TestClose(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()
	err = g.Add(ctx, "test", AddOptions{})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if g.BufferedCount() != 1 {
		t.Fatalf("BufferedCount before Close: got %d, want 1", g.BufferedCount())
	}

	// Close should clear buffer
	err = g.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if g.BufferedCount() != 0 {
		t.Fatalf("BufferedCount after Close: got %d, want 0", g.BufferedCount())
	}
}

func TestStats(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	// Initial stats should have zeros
	stats, err := g.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.NodeCount != 0 {
		t.Fatalf("Initial NodeCount: got %d, want 0", stats.NodeCount)
	}
	if stats.EdgeCount != 0 {
		t.Fatalf("Initial EdgeCount: got %d, want 0", stats.EdgeCount)
	}
	if stats.BufferedDocs != 0 {
		t.Fatalf("Initial BufferedDocs: got %d, want 0", stats.BufferedDocs)
	}
	if stats.MemoryCount != 0 {
		t.Fatalf("Initial MemoryCount: got %d, want 0", stats.MemoryCount)
	}

	// Add and check buffered count
	g.Add(ctx, "test", AddOptions{})
	stats, err = g.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.BufferedDocs != 1 {
		t.Fatalf("BufferedDocs after Add: got %d, want 1", stats.BufferedDocs)
	}

	// Add a memory and verify count
	_, err = g.AddMemory(ctx, MemoryInput{
		Topic:   "Test Memory",
		Context: "Test context",
	})
	if err != nil {
		t.Fatalf("AddMemory failed: %v", err)
	}

	stats, err = g.Stats()
	if err != nil {
		t.Fatalf("Stats after AddMemory failed: %v", err)
	}
	if stats.MemoryCount != 1 {
		t.Fatalf("MemoryCount after AddMemory: got %d, want 1", stats.MemoryCount)
	}
}

// TestCognifyWithMockedDependencies exercises the full Cognify path using injected mocks.
func TestCognifyWithMockedDependencies(t *testing.T) {
	// Build Gognee via New then replace internal clients with mocks (package-scope access).
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	// Inject mocks.
	mockLLM := &MockLLMClient{}
	mockEmbed := &MockEmbeddingClient{}
	g.llm = mockLLM
	g.embeddings = mockEmbed
	// Re-create extractors with mock LLM.
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)

	ctx := context.Background()

	// Add a document so Cognify has work.
	if err := g.Add(ctx, "React is a frontend library.", AddOptions{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	result, err := g.Cognify(ctx, CognifyOptions{})
	if err != nil {
		t.Fatalf("Cognify failed: %v", err)
	}

	// The MockLLMClient returns 1 entity (TestEntity) and 1 triplet (TestEntity IS_A Concept).
	if result.DocumentsProcessed != 1 {
		t.Errorf("DocumentsProcessed: got %d, want 1", result.DocumentsProcessed)
	}
	if result.ChunksProcessed < 1 {
		t.Errorf("ChunksProcessed: got %d, want >=1", result.ChunksProcessed)
	}
	if result.NodesCreated < 1 {
		t.Errorf("NodesCreated: got %d, want >=1", result.NodesCreated)
	}
	// Edges may be 0 because edge creation uses generateDeterministicNodeID with empty type for subject/object,
	// resulting in IDs that might not match created node IDs. We'll just assert no catastrophic error here.
	if len(result.Errors) != 0 {
		t.Errorf("Unexpected errors: %v", result.Errors)
	}

	// Buffer should be cleared.
	if g.BufferedCount() != 0 {
		t.Errorf("Buffer not cleared after Cognify, count=%d", g.BufferedCount())
	}

	// Stats should reflect created nodes.
	stats, err := g.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.NodeCount < 1 {
		t.Errorf("Stats.NodeCount: got %d, want >=1", stats.NodeCount)
	}
}

// TestSearchWithMockedDependencies exercises Search path using injected mocks.
func TestSearchWithMockedDependencies(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	// Inject mock embeddings client so EmbedOne succeeds.
	mockEmbed := &MockEmbeddingClient{}
	g.embeddings = mockEmbed

	// Re-create searcher with mocked embeddings.
	g.searcher = search.NewHybridSearcher(mockEmbed, g.vectorStore, g.graphStore)

	ctx := context.Background()

	// Without cognifying, search should return empty but not error.
	response, err := g.Search(ctx, "something", search.SearchOptions{})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(response.Results) != 0 {
		t.Errorf("Expected 0 results on empty graph, got %d", len(response.Results))
	}
}

// TestSanitizeRelation exercises the helper.
func TestSanitizeRelation(t *testing.T) {
	got := sanitizeRelation("depends on")
	want := "DEPENDS_ON"
	if got != want {
		t.Errorf("sanitizeRelation: got %q, want %q", got, want)
	}
}

// ErrorLLMClient returns errors to exercise error paths
type ErrorLLMClient struct {
	EntityError   error
	RelationError error
}

func (e *ErrorLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "", e.EntityError
}

func (e *ErrorLLMClient) CompleteWithSchema(ctx context.Context, prompt string, schema interface{}) error {
	switch schema.(type) {
	case *[]extraction.Entity:
		return e.EntityError
	case *[]extraction.Triplet:
		return e.RelationError
	}
	return nil
}

var _ llm.LLMClient = (*ErrorLLMClient)(nil)

// ErrorEmbeddingClient returns errors to exercise error paths
type ErrorEmbeddingClient struct {
	Err error
}

func (e *ErrorEmbeddingClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, e.Err
}

func (e *ErrorEmbeddingClient) EmbedOne(ctx context.Context, text string) ([]float32, error) {
	return nil, e.Err
}

var _ embeddings.EmbeddingClient = (*ErrorEmbeddingClient)(nil)

// ErrorGraphStore returns errors on specific operations
type ErrorGraphStore struct {
	store.GraphStore
	AddNodeErr error
	AddEdgeErr error
}

func (e *ErrorGraphStore) AddNode(ctx context.Context, node *store.Node) error {
	if e.AddNodeErr != nil {
		return e.AddNodeErr
	}
	return e.GraphStore.AddNode(ctx, node)
}

func (e *ErrorGraphStore) AddEdge(ctx context.Context, edge *store.Edge) error {
	if e.AddEdgeErr != nil {
		return e.AddEdgeErr
	}
	return e.GraphStore.AddEdge(ctx, edge)
}

// ErrorVectorStore returns errors to exercise error paths
type ErrorVectorStore struct {
	Err error
}

func (e *ErrorVectorStore) Add(ctx context.Context, id string, embedding []float32) error {
	return e.Err
}

func (e *ErrorVectorStore) Search(ctx context.Context, query []float32, topK int) ([]store.SearchResult, error) {
	return nil, e.Err
}

func (e *ErrorVectorStore) Delete(ctx context.Context, id string) error {
	return e.Err
}

var _ store.VectorStore = (*ErrorVectorStore)(nil)

// TestCognifyEntityExtractionError exercises entity extraction failure path.
func TestCognifyEntityExtractionError(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	// Inject error LLM.
	errLLM := &ErrorLLMClient{EntityError: errors.New("entity extraction failed")}
	g.llm = errLLM
	g.entityExtractor = extraction.NewEntityExtractor(errLLM)
	g.relationExtractor = extraction.NewRelationExtractor(errLLM)

	ctx := context.Background()
	g.Add(ctx, "some test text", AddOptions{})

	result, err := g.Cognify(ctx, CognifyOptions{})
	if err != nil {
		t.Fatalf("Cognify should not return fatal error: %v", err)
	}
	if result.ChunksFailed < 1 {
		t.Errorf("ChunksFailed: got %d, want >=1", result.ChunksFailed)
	}
	if len(result.Errors) < 1 {
		t.Errorf("Expected at least 1 error, got %d", len(result.Errors))
	}
}

// TestCognifyEmbeddingError exercises embedding failure path.
func TestCognifyEmbeddingError(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	// Inject mock LLM that succeeds, but error embedding client.
	mockLLM := &MockLLMClient{}
	errEmbed := &ErrorEmbeddingClient{Err: errors.New("embedding failed")}
	g.llm = mockLLM
	g.embeddings = errEmbed
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)

	ctx := context.Background()
	g.Add(ctx, "some test text", AddOptions{})

	result, err := g.Cognify(ctx, CognifyOptions{})
	if err != nil {
		t.Fatalf("Cognify should not return fatal error: %v", err)
	}
	// Node created but embedding failed, so we should have errors.
	if len(result.Errors) < 1 {
		t.Errorf("Expected at least 1 error for embedding failure, got %d", len(result.Errors))
	}
}

// TestCognifyAddNodeError exercises graph store AddNode failure path.
func TestCognifyAddNodeError(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	// Inject mock LLM and embeddings that succeed, but error graph store.
	mockLLM := &MockLLMClient{}
	mockEmbed := &MockEmbeddingClient{}
	errGraph := &ErrorGraphStore{
		GraphStore: g.graphStore,
		AddNodeErr: errors.New("add node failed"),
	}

	g.llm = mockLLM
	g.embeddings = mockEmbed
	g.graphStore = errGraph
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)

	ctx := context.Background()
	g.Add(ctx, "some test text", AddOptions{})

	result, err := g.Cognify(ctx, CognifyOptions{})
	if err != nil {
		t.Fatalf("Cognify should not return fatal error: %v", err)
	}
	if len(result.Errors) < 1 {
		t.Errorf("Expected at least 1 error for AddNode failure, got %d", len(result.Errors))
	}
	if result.NodesCreated != 0 {
		t.Errorf("NodesCreated should be 0 when AddNode fails, got %d", result.NodesCreated)
	}
}

// TestCognifyVectorStoreError exercises vector store Add failure path.
func TestCognifyVectorStoreError(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	// Inject mock LLM and embeddings that succeed, but error vector store.
	mockLLM := &MockLLMClient{}
	mockEmbed := &MockEmbeddingClient{}
	errVector := &ErrorVectorStore{Err: errors.New("vector store failed")}

	g.llm = mockLLM
	g.embeddings = mockEmbed
	g.vectorStore = errVector
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)

	ctx := context.Background()
	g.Add(ctx, "some test text", AddOptions{})

	result, err := g.Cognify(ctx, CognifyOptions{})
	if err != nil {
		t.Fatalf("Cognify should not return fatal error: %v", err)
	}
	// Nodes should be created, but vector indexing fails.
	if result.NodesCreated < 1 {
		t.Errorf("NodesCreated should be >=1, got %d", result.NodesCreated)
	}
	if len(result.Errors) < 1 {
		t.Errorf("Expected at least 1 error for vector store failure, got %d", len(result.Errors))
	}
}

// TestCognifyAddEdgeError exercises graph store AddEdge failure path.
func TestCognifyAddEdgeError(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	// Inject mock LLM that returns entities AND triplets.
	mockLLM := &MockLLMClient{
		EntityResponses: [][]extraction.Entity{
			{
				{Name: "EntityA", Type: "Concept", Description: "First entity"},
				{Name: "EntityB", Type: "Concept", Description: "Second entity"},
			},
		},
		RelationResponses: [][]extraction.Triplet{
			{
				{Subject: "EntityA", Relation: "RELATES_TO", Object: "EntityB"},
			},
		},
	}
	mockEmbed := &MockEmbeddingClient{}
	errGraph := &ErrorGraphStore{
		GraphStore: g.graphStore,
		AddEdgeErr: errors.New("add edge failed"),
	}

	g.llm = mockLLM
	g.embeddings = mockEmbed
	g.graphStore = errGraph
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)

	ctx := context.Background()
	g.Add(ctx, "some test text", AddOptions{})

	result, err := g.Cognify(ctx, CognifyOptions{})
	if err != nil {
		t.Fatalf("Cognify should not return fatal error: %v", err)
	}
	// Nodes should be created.
	if result.NodesCreated < 2 {
		t.Errorf("NodesCreated should be >=2, got %d", result.NodesCreated)
	}
	// But edges fail.
	if result.EdgesCreated != 0 {
		t.Errorf("EdgesCreated should be 0 when AddEdge fails, got %d", result.EdgesCreated)
	}
	if len(result.Errors) < 1 {
		t.Errorf("Expected at least 1 error for AddEdge failure, got %d", len(result.Errors))
	}
}

// TestEdgeIDConsistency verifies that edge source/target IDs match node IDs (Plan 008)
func TestEdgeIDConsistency(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	// Setup mock LLM with entities and triplets
	mockLLM := &MockLLMClient{
		EntityResponses: [][]extraction.Entity{
			{
				{Name: "React", Type: "Technology", Description: "A JavaScript library"},
				{Name: "TypeScript", Type: "Technology", Description: "A typed superset of JavaScript"},
			},
		},
		RelationResponses: [][]extraction.Triplet{
			{
				{Subject: "React", Relation: "USES", Object: "TypeScript"},
			},
		},
	}
	mockEmbed := &MockEmbeddingClient{}

	g.llm = mockLLM
	g.embeddings = mockEmbed
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)

	ctx := context.Background()
	g.Add(ctx, "React uses TypeScript", AddOptions{})

	result, err := g.Cognify(ctx, CognifyOptions{})
	if err != nil {
		t.Fatalf("Cognify failed: %v", err)
	}

	if result.NodesCreated != 2 {
		t.Errorf("Expected 2 nodes created, got %d", result.NodesCreated)
	}
	if result.EdgesCreated != 1 {
		t.Errorf("Expected 1 edge created, got %d", result.EdgesCreated)
	}

	// Verify that edge source/target IDs reference actual nodes
	// Get the edge
	reactNodeID := generateDeterministicNodeID("React", "Technology")
	edges, err := g.graphStore.GetEdges(ctx, reactNodeID)
	if err != nil {
		t.Fatalf("GetEdges failed: %v", err)
	}

	if len(edges) != 1 {
		t.Fatalf("Expected 1 edge from React node, got %d", len(edges))
	}

	edge := edges[0]

	// Verify source node exists
	sourceNode, err := g.graphStore.GetNode(ctx, edge.SourceID)
	if err != nil {
		t.Errorf("Source node %s not found: %v", edge.SourceID, err)
	}
	if sourceNode != nil && sourceNode.Name != "React" {
		t.Errorf("Expected source node name 'React', got '%s'", sourceNode.Name)
	}

	// Verify target node exists
	targetNode, err := g.graphStore.GetNode(ctx, edge.TargetID)
	if err != nil {
		t.Errorf("Target node %s not found: %v", edge.TargetID, err)
	}
	if targetNode != nil && targetNode.Name != "TypeScript" {
		t.Errorf("Expected target node name 'TypeScript', got '%s'", targetNode.Name)
	}
}

// TestEdgeIDMissingEntity verifies that edges referencing non-existent entities are skipped (Plan 008)
// Note: We test this indirectly because the relation extractor validates entities exist.
// Instead, we test the normalization and lookup logic with case/whitespace variations.
func TestEdgeIDMissingEntity(t *testing.T) {
	// This test validates the lookup logic by testing entity name variations.
	// The actual "missing entity" scenario is prevented by the relation extractor's validation.
	// However, the edge creation code still needs the defensive logic for future flexibility.

	// We'll test the helper functions directly
	entities := []extraction.Entity{
		{Name: "React", Type: "Technology", Description: "A library"},
	}

	entityMap, ambiguous := buildEntityTypeMap(entities)

	// Test that "React" is found
	typ, found := lookupEntityType("React", entityMap, ambiguous)
	if !found {
		t.Error("Expected 'React' to be found")
	}
	if typ != "Technology" {
		t.Errorf("Expected type 'Technology', got '%s'", typ)
	}

	// Test that "TypeScript" is NOT found
	_, found = lookupEntityType("TypeScript", entityMap, ambiguous)
	if found {
		t.Error("Expected 'TypeScript' to NOT be found")
	}

	// Test case-insensitive match
	typ, found = lookupEntityType("react", entityMap, ambiguous)
	if !found {
		t.Error("Expected 'react' (lowercase) to be found")
	}
	if typ != "Technology" {
		t.Errorf("Expected type 'Technology', got '%s'", typ)
	}

	// Test whitespace normalization
	typ, found = lookupEntityType("  React  ", entityMap, ambiguous)
	if !found {
		t.Error("Expected '  React  ' (with whitespace) to be found")
	}
	if typ != "Technology" {
		t.Errorf("Expected type 'Technology', got '%s'", typ)
	}
}

// TestEdgeIDCaseInsensitive verifies case-insensitive entity name matching (Plan 008)
func TestEdgeIDCaseInsensitive(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	// Setup: entity extracted as "React", triplet uses "react" (lowercase)
	mockLLM := &MockLLMClient{
		EntityResponses: [][]extraction.Entity{
			{
				{Name: "React", Type: "Technology", Description: "A JavaScript library"},
				{Name: "TypeScript", Type: "Technology", Description: "A typed superset"},
			},
		},
		RelationResponses: [][]extraction.Triplet{
			{
				// Lowercase names should still match
				{Subject: "react", Relation: "USES", Object: "typescript"},
			},
		},
	}
	mockEmbed := &MockEmbeddingClient{}

	g.llm = mockLLM
	g.embeddings = mockEmbed
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)

	ctx := context.Background()
	g.Add(ctx, "react uses typescript", AddOptions{})

	result, err := g.Cognify(ctx, CognifyOptions{})
	if err != nil {
		t.Fatalf("Cognify failed: %v", err)
	}

	if result.NodesCreated != 2 {
		t.Errorf("Expected 2 nodes created, got %d", result.NodesCreated)
	}

	// Edge SHOULD be created despite case mismatch
	if result.EdgesCreated != 1 {
		t.Errorf("Expected 1 edge created (case-insensitive match), got %d", result.EdgesCreated)
	}

	if result.EdgesSkipped != 0 {
		t.Errorf("Expected 0 edges skipped, got %d", result.EdgesSkipped)
	}
}

// TestEdgeIDWhitespaceNormalization verifies whitespace normalization (Plan 008)
func TestEdgeIDWhitespaceNormalization(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	// Setup: entity "React" vs triplet "  React  " (extra whitespace)
	mockLLM := &MockLLMClient{
		EntityResponses: [][]extraction.Entity{
			{
				{Name: "React", Type: "Technology", Description: "A JavaScript library"},
				{Name: "TypeScript", Type: "Technology", Description: "A typed superset"},
			},
		},
		RelationResponses: [][]extraction.Triplet{
			{
				{Subject: "  React  ", Relation: "USES", Object: "  TypeScript  "},
			},
		},
	}
	mockEmbed := &MockEmbeddingClient{}

	g.llm = mockLLM
	g.embeddings = mockEmbed
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)

	ctx := context.Background()
	g.Add(ctx, "React uses TypeScript", AddOptions{})

	result, err := g.Cognify(ctx, CognifyOptions{})
	if err != nil {
		t.Fatalf("Cognify failed: %v", err)
	}

	if result.NodesCreated != 2 {
		t.Errorf("Expected 2 nodes created, got %d", result.NodesCreated)
	}

	// Edge SHOULD be created despite whitespace differences
	if result.EdgesCreated != 1 {
		t.Errorf("Expected 1 edge created (whitespace normalized), got %d", result.EdgesCreated)
	}
}

// TestEdgeIDUnicode verifies Unicode entity names handled correctly (Plan 008)
func TestEdgeIDUnicode(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	// Setup: Unicode entity names
	mockLLM := &MockLLMClient{
		EntityResponses: [][]extraction.Entity{
			{
				{Name: "Café", Type: "Concept", Description: "A coffee shop"},
				{Name: "François", Type: "Person", Description: "Owner"},
			},
		},
		RelationResponses: [][]extraction.Triplet{
			{
				{Subject: "François", Relation: "OWNS", Object: "Café"},
			},
		},
	}
	mockEmbed := &MockEmbeddingClient{}

	g.llm = mockLLM
	g.embeddings = mockEmbed
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)

	ctx := context.Background()
	g.Add(ctx, "François owns Café", AddOptions{})

	result, err := g.Cognify(ctx, CognifyOptions{})
	if err != nil {
		t.Fatalf("Cognify failed: %v", err)
	}

	// Debug: check for errors
	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			t.Logf("Error: %v", e)
		}
	}

	if result.NodesCreated != 2 {
		t.Errorf("Expected 2 nodes created, got %d (ChunksProcessed=%d, ChunksFailed=%d)",
			result.NodesCreated, result.ChunksProcessed, result.ChunksFailed)
	}

	if result.EdgesCreated != 1 {
		t.Errorf("Expected 1 edge created, got %d", result.EdgesCreated)
	}

	// Verify edge connects to actual nodes
	francoisNodeID := generateDeterministicNodeID("François", "Person")
	edges, err := g.graphStore.GetEdges(ctx, francoisNodeID)
	if err != nil {
		t.Fatalf("GetEdges failed: %v", err)
	}

	if len(edges) != 1 {
		t.Fatalf("Expected 1 edge from François node, got %d", len(edges))
	}
}

// TestEdgeIDAmbiguousEntity verifies ambiguous entity names cause edge skip (Plan 008)
func TestEdgeIDAmbiguousEntity(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	// Setup: "Python" exists as both Technology and Concept
	mockLLM := &MockLLMClient{
		EntityResponses: [][]extraction.Entity{
			{
				{Name: "Python", Type: "Technology", Description: "Programming language"},
				{Name: "Python", Type: "Concept", Description: "A type of snake"},
				{Name: "Django", Type: "Technology", Description: "Web framework"},
			},
		},
		RelationResponses: [][]extraction.Triplet{
			{
				// Ambiguous: which Python?
				{Subject: "Django", Relation: "USES", Object: "Python"},
			},
		},
	}
	mockEmbed := &MockEmbeddingClient{}

	g.llm = mockLLM
	g.embeddings = mockEmbed
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)

	ctx := context.Background()
	g.Add(ctx, "Django uses Python", AddOptions{})

	result, err := g.Cognify(ctx, CognifyOptions{})
	if err != nil {
		t.Fatalf("Cognify failed: %v", err)
	}

	if result.NodesCreated != 3 {
		t.Errorf("Expected 3 nodes created, got %d", result.NodesCreated)
	}

	// Edge SHOULD be skipped due to ambiguity
	if result.EdgesCreated != 0 {
		t.Errorf("Expected 0 edges created (Python is ambiguous), got %d", result.EdgesCreated)
	}

	if result.EdgesSkipped != 1 {
		t.Errorf("Expected 1 edge skipped, got %d", result.EdgesSkipped)
	}

	// Verify error logged mentions ambiguity
	foundAmbiguityError := false
	for _, err := range result.Errors {
		if err != nil && (contains(err.Error(), "ambiguous") || contains(err.Error(), "skipped edge")) {
			foundAmbiguityError = true
			break
		}
	}
	if !foundAmbiguityError {
		t.Error("Expected ambiguity-related error for skipped edge")
	}
}

// ==================================================
// Memory CRUD API Tests (Plan 011)
// ==================================================

// TestAddMemory_Success validates the AddMemory API with mocked LLM.
func TestAddMemory_Success(t *testing.T) {
	ctx := context.Background()

	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	// Inject mocks
	mockLLM := &MockLLMClient{
		EntityResponses: [][]extraction.Entity{
			{
				{Name: "Storage Layer", Type: "System", Description: "SQLite-backed graph store"},
				{Name: "Provenance", Type: "Concept", Description: "Tracking memory to artifact mapping"},
			},
		},
		RelationResponses: [][]extraction.Triplet{
			{
				{Subject: "Storage Layer", Relation: "IMPLEMENTS", Object: "Provenance"},
			},
		},
	}
	mockEmbed := &MockEmbeddingClient{}
	g.llm = mockLLM
	g.embeddings = mockEmbed
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)

	// Add memory
	input := MemoryInput{
		Topic:     "Phase 4 Implementation",
		Context:   "Implemented SQLite graph store with provenance tracking",
		Decisions: []string{"Use SQLite", "Enable foreign keys"},
		Rationale: []string{"ACID guarantees", "Cascade deletes"},
		Metadata:  map[string]interface{}{"plan": "004"},
		Source:    "implementation-doc",
	}

	result, err := g.AddMemory(ctx, input)
	if err != nil {
		t.Fatalf("AddMemory failed: %v", err)
	}

	// Validate result
	if result.MemoryID == "" {
		t.Error("Memory ID not generated")
	}
	if result.NodesCreated != 2 {
		t.Errorf("NodesCreated: got %d, want 2", result.NodesCreated)
	}
	if result.EdgesCreated != 1 {
		t.Errorf("EdgesCreated: got %d, want 1", result.EdgesCreated)
	}

	// Verify memory is retrievable and has correct data
	retrieved, err := g.GetMemory(ctx, result.MemoryID)
	if err != nil {
		t.Fatalf("GetMemory failed: %v", err)
	}
	if retrieved.Topic != input.Topic {
		t.Errorf("Retrieved topic mismatch: got %s, want %s", retrieved.Topic, input.Topic)
	}
	if retrieved.Status != "complete" {
		t.Errorf("Status should be 'complete', got %s", retrieved.Status)
	}
	if len(retrieved.Decisions) != 2 {
		t.Errorf("Decisions count: got %d, want 2", len(retrieved.Decisions))
	}
}

// TestAddMemory_Deduplication validates that duplicate memories are detected.
func TestAddMemory_Deduplication(t *testing.T) {
	ctx := context.Background()

	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	mockLLM := &MockLLMClient{
		EntityResponses: [][]extraction.Entity{
			{{Name: "Test", Type: "Concept", Description: "Test entity"}},
		},
	}
	mockEmbed := &MockEmbeddingClient{}
	g.llm = mockLLM
	g.embeddings = mockEmbed
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)

	input := MemoryInput{
		Topic:   "Test Memory",
		Context: "Test context",
	}

	// Add first memory
	result1, err := g.AddMemory(ctx, input)
	if err != nil {
		t.Fatalf("AddMemory 1 failed: %v", err)
	}

	// Add duplicate (same content)
	result2, err := g.AddMemory(ctx, input)
	if err != nil {
		t.Fatalf("AddMemory 2 failed: %v", err)
	}

	// Should return existing memory
	if result1.MemoryID != result2.MemoryID {
		t.Errorf("Expected same memory ID for duplicate, got %s and %s", result1.MemoryID, result2.MemoryID)
	}
	if result2.NodesCreated != 0 {
		t.Errorf("Duplicate should not create nodes, got %d", result2.NodesCreated)
	}
}

// TestListMemories validates pagination.
func TestListMemories(t *testing.T) {
	ctx := context.Background()

	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	mockLLM := &MockLLMClient{}
	mockEmbed := &MockEmbeddingClient{}
	g.llm = mockLLM
	g.embeddings = mockEmbed
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)

	// Add multiple memories
	for i := 0; i < 5; i++ {
		input := MemoryInput{
			Topic:   "Memory " + string(rune('A'+i)),
			Context: "Context " + string(rune('A'+i)),
		}
		_, err := g.AddMemory(ctx, input)
		if err != nil {
			t.Fatalf("AddMemory %d failed: %v", i, err)
		}
	}

	// List with default options
	results, err := g.ListMemories(ctx, store.ListMemoriesOptions{})
	if err != nil {
		t.Fatalf("ListMemories failed: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Expected 5 memories, got %d", len(results))
	}

	// List with limit
	results, err = g.ListMemories(ctx, store.ListMemoriesOptions{Limit: 2})
	if err != nil {
		t.Fatalf("ListMemories with limit failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 memories with limit, got %d", len(results))
	}

	// List with offset
	results, err = g.ListMemories(ctx, store.ListMemoriesOptions{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("ListMemories with offset failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 memories with offset, got %d", len(results))
	}
}

// TestUpdateMemory validates re-cognify and provenance update.
func TestUpdateMemory(t *testing.T) {
	ctx := context.Background()

	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	mockLLM := &MockLLMClient{
		EntityResponses: [][]extraction.Entity{
			// First AddMemory
			{{Name: "Original", Type: "Concept", Description: "Original entity"}},
			// UpdateMemory
			{{Name: "Updated", Type: "Concept", Description: "Updated entity"}},
		},
	}
	mockEmbed := &MockEmbeddingClient{}
	g.llm = mockLLM
	g.embeddings = mockEmbed
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)

	// Add initial memory
	input := MemoryInput{
		Topic:   "Test",
		Context: "Original context",
	}
	result, err := g.AddMemory(ctx, input)
	if err != nil {
		t.Fatalf("AddMemory failed: %v", err)
	}
	memoryID := result.MemoryID

	// Update memory
	newContext := "Updated context"
	updates := store.MemoryUpdate{
		Context: &newContext,
	}
	updateResult, err := g.UpdateMemory(ctx, memoryID, updates)
	if err != nil {
		t.Fatalf("UpdateMemory failed: %v", err)
	}

	// Validate update result
	if updateResult.MemoryID != memoryID {
		t.Errorf("MemoryID mismatch: got %s, want %s", updateResult.MemoryID, memoryID)
	}
	// Retrieve updated memory to verify changes
	updated, err := g.GetMemory(ctx, memoryID)
	if err != nil {
		t.Fatalf("GetMemory after update failed: %v", err)
	}
	if updated.Context != newContext {
		t.Errorf("Context not updated: got %s, want %s", updated.Context, newContext)
	}
	if updated.Version < 2 {
		t.Errorf("Version not incremented: got %d, want >= 2", updated.Version)
	}
	if updateResult.NodesCreated == 0 {
		t.Error("Expected re-cognify to create nodes")
	}
}

// TestDeleteMemory validates deletion and garbage collection.
func TestDeleteMemory(t *testing.T) {
	ctx := context.Background()

	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	mockLLM := &MockLLMClient{
		EntityResponses: [][]extraction.Entity{
			{{Name: "ToDelete", Type: "Concept", Description: "Entity to delete"}},
		},
	}
	mockEmbed := &MockEmbeddingClient{}
	g.llm = mockLLM
	g.embeddings = mockEmbed
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)

	// Add memory
	input := MemoryInput{
		Topic:   "Test",
		Context: "Test context",
	}
	result, err := g.AddMemory(ctx, input)
	if err != nil {
		t.Fatalf("AddMemory failed: %v", err)
	}
	memoryID := result.MemoryID

	// Delete memory
	err = g.DeleteMemory(ctx, memoryID)
	if err != nil {
		t.Fatalf("DeleteMemory failed: %v", err)
	}

	// Verify memory is gone
	_, err = g.GetMemory(ctx, memoryID)
	if err == nil {
		t.Error("Expected error when retrieving deleted memory")
	}
}

// TestDeleteMemory_PreservesSharedNodes validates GC preserves shared nodes.
func TestDeleteMemory_PreservesSharedNodes(t *testing.T) {
	ctx := context.Background()

	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	mockLLM := &MockLLMClient{
		EntityResponses: [][]extraction.Entity{
			// Memory 1
			{
				{Name: "Shared", Type: "Concept", Description: "Shared entity"},
				{Name: "Unique1", Type: "Concept", Description: "Unique to memory 1"},
			},
			// Memory 2
			{
				{Name: "Shared", Type: "Concept", Description: "Shared entity"},
				{Name: "Unique2", Type: "Concept", Description: "Unique to memory 2"},
			},
		},
	}
	mockEmbed := &MockEmbeddingClient{}
	g.llm = mockLLM
	g.embeddings = mockEmbed
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)

	// Add two memories sharing a node
	input1 := MemoryInput{Topic: "Memory 1", Context: "Context 1"}
	result1, err := g.AddMemory(ctx, input1)
	if err != nil {
		t.Fatalf("AddMemory 1 failed: %v", err)
	}

	input2 := MemoryInput{Topic: "Memory 2", Context: "Context 2"}
	_, err = g.AddMemory(ctx, input2)
	if err != nil {
		t.Fatalf("AddMemory 2 failed: %v", err)
	}

	// Delete memory 1
	err = g.DeleteMemory(ctx, result1.MemoryID)
	if err != nil {
		t.Fatalf("DeleteMemory failed: %v", err)
	}

	// Verify shared node still exists (referenced by memory 2)
	sharedNodeID := generateDeterministicNodeID("Shared", "Concept")
	sharedNode, err := g.graphStore.GetNode(ctx, sharedNodeID)
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}
	if sharedNode == nil {
		t.Error("Shared node was incorrectly deleted")
	}

	// Verify memory 2's unique node still exists
	unique2NodeID := generateDeterministicNodeID("Unique2", "Concept")
	unique2Node, err := g.graphStore.GetNode(ctx, unique2NodeID)
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}
	if unique2Node == nil {
		t.Error("Memory 2's unique node was incorrectly deleted")
	}
}

// TestSearch_MemoryIDsEnrichment validates search provenance enrichment.
func TestSearch_MemoryIDsEnrichment(t *testing.T) {
	ctx := context.Background()

	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	mockLLM := &MockLLMClient{
		EntityResponses: [][]extraction.Entity{
			{{Name: "SearchEntity", Type: "Concept", Description: "Entity for search"}},
		},
	}
	mockEmbed := &MockEmbeddingClient{}
	g.llm = mockLLM
	g.embeddings = mockEmbed
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)
	// Recreate searcher with mock embeddings to avoid OpenAI calls
	g.searcher = search.NewHybridSearcher(mockEmbed, g.vectorStore, g.graphStore)

	// Add memory
	input := MemoryInput{
		Topic:   "Search Test",
		Context: "Test search enrichment",
	}
	result, err := g.AddMemory(ctx, input)
	if err != nil {
		t.Fatalf("AddMemory failed: %v", err)
	}
	memoryID := result.MemoryID

	// Search with default options (MemoryIDs enabled)
	searchResponse, err := g.Search(ctx, "search", SearchOptions{
		Type: SearchTypeVector,
		TopK: 10,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(searchResponse.Results) == 0 {
		t.Fatal("Expected search results")
	}

	// Verify MemoryIDs are populated
	foundMemoryID := false
	for _, sr := range searchResponse.Results {
		if len(sr.MemoryIDs) > 0 {
			foundMemoryID = true
			if sr.MemoryIDs[0] == memoryID {
				break
			}
		}
	}
	if !foundMemoryID {
		t.Error("Expected search results to include MemoryIDs")
	}

	// Search with MemoryIDs disabled
	includeMemoryIDs := false
	searchResponse2, err := g.Search(ctx, "search", SearchOptions{
		Type:             SearchTypeVector,
		TopK:             10,
		IncludeMemoryIDs: &includeMemoryIDs,
	})
	if err != nil {
		t.Fatalf("Search with IncludeMemoryIDs=false failed: %v", err)
	}

	// Verify MemoryIDs are not populated
	for _, sr := range searchResponse2.Results {
		if len(sr.MemoryIDs) > 0 {
			t.Error("Expected no MemoryIDs when IncludeMemoryIDs=false")
		}
	}
}

// TestGarbageCollect_Placeholder validates the placeholder GC method.
func TestGarbageCollect_Placeholder(t *testing.T) {
	ctx := context.Background()

	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	// Call placeholder GC
	nodesDeleted, edgesDeleted, err := g.GarbageCollect(ctx)

	// Should return error (not yet implemented)
	if err == nil {
		t.Error("Expected error from placeholder GarbageCollect")
	}
	if nodesDeleted != 0 || edgesDeleted != 0 {
		t.Errorf("Expected (0,0) from placeholder, got (%d,%d)", nodesDeleted, edgesDeleted)
	}
}
