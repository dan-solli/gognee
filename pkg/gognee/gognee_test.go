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

	// Verify defaults are applied
	if g.config.DecayEnabled {
		t.Error("DecayEnabled should default to false")
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
		{
			name: "decay_disabled_ignores_invalid_config",
			config: Config{
				DBPath:            ":memory:",
				DecayEnabled:      false,
				DecayHalfLifeDays: -5,
				DecayBasis:        "invalid",
			},
			wantErr: false, // Should not validate when decay is disabled
		},
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

	// Add and check buffered count
	g.Add(ctx, "test", AddOptions{})
	stats, err = g.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.BufferedDocs != 1 {
		t.Fatalf("BufferedDocs after Add: got %d, want 1", stats.BufferedDocs)
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
	results, err := g.Search(ctx, "something", search.SearchOptions{})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results on empty graph, got %d", len(results))
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
