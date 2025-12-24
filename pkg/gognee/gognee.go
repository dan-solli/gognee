// Package gognee provides a knowledge graph memory system for AI assistants
package gognee

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/dan-solli/gognee/pkg/chunker"
	"github.com/dan-solli/gognee/pkg/embeddings"
	"github.com/dan-solli/gognee/pkg/extraction"
	"github.com/dan-solli/gognee/pkg/llm"
	"github.com/dan-solli/gognee/pkg/search"
	"github.com/dan-solli/gognee/pkg/store"
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

	// DBPath is the path to the SQLite database file.
	// If empty or ":memory:", an in-memory database is used.
	DBPath string
}

// Gognee is the main entry point for the memory system
type Gognee struct {
	config            Config
	chunker           *chunker.Chunker
	embeddings        embeddings.EmbeddingClient
	llm               llm.LLMClient
	graphStore        store.GraphStore
	vectorStore       store.VectorStore
	searcher          search.Searcher
	entityExtractor   *extraction.EntityExtractor
	relationExtractor *extraction.RelationExtractor
	buffer            []AddedDocument
	lastCognified     time.Time
}

// AddedDocument represents a document added to the buffer for processing
type AddedDocument struct {
	Text    string
	Source  string
	AddedAt time.Time
}

// AddOptions configures the Add() method
type AddOptions struct {
	Source string
}

// CognifyOptions configures the Cognify() method
type CognifyOptions struct {
	// Reserved for future options like ChunkSize override
}

// CognifyResult reports the outcome of a Cognify() operation
type CognifyResult struct {
	DocumentsProcessed int
	ChunksProcessed    int
	ChunksFailed       int
	NodesCreated       int
	EdgesCreated       int
	Errors             []error
}

// Stats reports basic telemetry about the knowledge graph
type Stats struct {
	NodeCount     int64
	EdgeCount     int64
	BufferedDocs  int
	LastCognified time.Time
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

	// Initialize GraphStore
	dbPath := cfg.DBPath
	if dbPath == "" {
		dbPath = ":memory:"
	}
	graphStore, err := store.NewSQLiteGraphStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize graph store: %w", err)
	}

	// Initialize VectorStore
	vectorStore := store.NewMemoryVectorStore()

	// Initialize extractors
	entityExtractor := extraction.NewEntityExtractor(llmClient)
	relationExtractor := extraction.NewRelationExtractor(llmClient)

	// Initialize searcher
	searcher := search.NewHybridSearcher(embeddingsClient, vectorStore, graphStore)

	return &Gognee{
		config:            cfg,
		chunker:           c,
		embeddings:        embeddingsClient,
		llm:               llmClient,
		graphStore:        graphStore,
		vectorStore:       vectorStore,
		searcher:          searcher,
		entityExtractor:   entityExtractor,
		relationExtractor: relationExtractor,
		buffer:            make([]AddedDocument, 0),
		lastCognified:     time.Time{},
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

// GetGraphStore returns the configured graph store
func (g *Gognee) GetGraphStore() store.GraphStore {
	return g.graphStore
}

// GetVectorStore returns the configured vector store
func (g *Gognee) GetVectorStore() store.VectorStore {
	return g.vectorStore
}

// Add buffers text for processing via Cognify()
func (g *Gognee) Add(ctx context.Context, text string, opts AddOptions) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("text cannot be empty")
	}

	doc := AddedDocument{
		Text:    text,
		Source:  opts.Source,
		AddedAt: time.Now(),
	}
	g.buffer = append(g.buffer, doc)
	return nil
}

// BufferedCount returns the number of documents currently in the buffer
func (g *Gognee) BufferedCount() int {
	return len(g.buffer)
}

// Cognify processes all buffered documents through the extraction pipeline
func (g *Gognee) Cognify(ctx context.Context, opts CognifyOptions) (*CognifyResult, error) {
	result := &CognifyResult{
		Errors: make([]error, 0),
	}

	// No-op if buffer is empty
	if len(g.buffer) == 0 {
		return result, nil
	}

	// Process each document
	for _, doc := range g.buffer {
		result.DocumentsProcessed++

		// Chunk the text
		chunks := g.chunker.Chunk(doc.Text)

		// Process each chunk
		for _, chunk := range chunks {
			result.ChunksProcessed++

			// Extract entities
			entities, err := g.entityExtractor.Extract(ctx, chunk.Text)
			if err != nil {
				result.ChunksFailed++
				result.Errors = append(result.Errors, fmt.Errorf("entity extraction failed for chunk %s: %w", chunk.ID, err))
				continue
			}

			// Extract relations
			triplets, err := g.relationExtractor.Extract(ctx, chunk.Text, entities)
			if err != nil {
				result.ChunksFailed++
				result.Errors = append(result.Errors, fmt.Errorf("relation extraction failed for chunk %s: %w", chunk.ID, err))
				// Continue with entities only if relations fail
			}

			// Create nodes for each entity
			for _, entity := range entities {
				nodeID := generateDeterministicNodeID(entity.Name, entity.Type)
				node := &store.Node{
					ID:          nodeID,
					Name:        entity.Name,
					Type:        entity.Type,
					Description: entity.Description,
					CreatedAt:   time.Now(),
					Metadata:    make(map[string]interface{}),
				}

				// Add to graph store
				if err := g.graphStore.AddNode(ctx, node); err != nil {
					result.Errors = append(result.Errors, fmt.Errorf("failed to add node %s: %w", entity.Name, err))
					continue
				}
				result.NodesCreated++

				// Generate embedding for the node
				embedding, err := g.embeddings.EmbedOne(ctx, entity.Name+" "+entity.Description)
				if err != nil {
					result.Errors = append(result.Errors, fmt.Errorf("failed to embed node %s: %w", entity.Name, err))
					continue
				}

				// Update node with embedding
				node.Embedding = embedding
				if err := g.graphStore.AddNode(ctx, node); err != nil {
					result.Errors = append(result.Errors, fmt.Errorf("failed to update node embedding %s: %w", entity.Name, err))
					continue
				}

				// Index in vector store
				if err := g.vectorStore.Add(ctx, nodeID, embedding); err != nil {
					result.Errors = append(result.Errors, fmt.Errorf("failed to index node %s in vector store: %w", entity.Name, err))
				}
			}

			// Create edges for each triplet
			for _, triplet := range triplets {
				sourceID := generateDeterministicNodeID(triplet.Subject, "")
				targetID := generateDeterministicNodeID(triplet.Object, "")

				edge := &store.Edge{
					ID:        fmt.Sprintf("%s-%s-%s", sourceID, sanitizeRelation(triplet.Relation), targetID),
					SourceID:  sourceID,
					Relation:  triplet.Relation,
					TargetID:  targetID,
					Weight:    1.0,
					CreatedAt: time.Now(),
				}

				if err := g.graphStore.AddEdge(ctx, edge); err != nil {
					result.Errors = append(result.Errors, fmt.Errorf("failed to add edge %s-%s-%s: %w", triplet.Subject, triplet.Relation, triplet.Object, err))
					continue
				}
				result.EdgesCreated++
			}
		}
	}

	// Always clear buffer after processing (best-effort semantics)
	g.buffer = make([]AddedDocument, 0)
	g.lastCognified = time.Now()

	return result, nil
}

// Search queries the knowledge graph
func (g *Gognee) Search(ctx context.Context, query string, opts search.SearchOptions) ([]search.SearchResult, error) {
	search.ApplyDefaults(&opts)
	return g.searcher.Search(ctx, query, opts)
}

// Close releases all resources
func (g *Gognee) Close() error {
	g.buffer = make([]AddedDocument, 0)
	return g.graphStore.Close()
}

// Stats returns basic telemetry
func (g *Gognee) Stats() (Stats, error) {
	ctx := context.Background()
	nodeCount, err := g.graphStore.NodeCount(ctx)
	if err != nil {
		return Stats{}, fmt.Errorf("failed to get node count: %w", err)
	}

	edgeCount, err := g.graphStore.EdgeCount(ctx)
	if err != nil {
		return Stats{}, fmt.Errorf("failed to get edge count: %w", err)
	}

	return Stats{
		NodeCount:     nodeCount,
		EdgeCount:     edgeCount,
		BufferedDocs:  len(g.buffer),
		LastCognified: g.lastCognified,
	}, nil
}

// generateDeterministicNodeID creates a deterministic node ID from name and type
func generateDeterministicNodeID(name, nodeType string) string {
	// Normalize the name
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.Join(strings.Fields(normalized), " ") // Collapse spaces

	// Create the key
	key := normalized + "|" + nodeType

	// Hash with SHA-256
	hash := sha256.Sum256([]byte(key))

	// Return hex-encoded first 16 bytes (32 chars)
	return fmt.Sprintf("%x", hash[:16])
}

// sanitizeRelation converts relation names to safe edge IDs
func sanitizeRelation(relation string) string {
	return strings.ToUpper(strings.ReplaceAll(relation, " ", "_"))
}
