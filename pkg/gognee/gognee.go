// Package gognee provides a knowledge graph memory system for AI assistants
package gognee

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dan-solli/gognee/pkg/chunker"
	"github.com/dan-solli/gognee/pkg/embeddings"
	"github.com/dan-solli/gognee/pkg/extraction"
	"github.com/dan-solli/gognee/pkg/llm"
	"github.com/dan-solli/gognee/pkg/metrics"
	"github.com/dan-solli/gognee/pkg/search"
	"github.com/dan-solli/gognee/pkg/store"
	tracepkg "github.com/dan-solli/gognee/pkg/trace"
	"github.com/google/uuid"
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

	// DecayEnabled enables time-based memory decay scoring (default: false)
	DecayEnabled bool

	// DecayHalfLifeDays is the number of days after which a node's score is halved (default: 30)
	DecayHalfLifeDays int

	// DecayBasis determines decay calculation: "access" (last access time) or "creation" (creation time)
	// Default: "access"
	DecayBasis string

	// AccessFrequencyEnabled enables frequency-based decay modification (default: true when DecayEnabled)
	// When enabled, frequently accessed memories resist time-based decay
	AccessFrequencyEnabled bool

	// ReferenceAccessCount is the access count at which heat_multiplier = 1.0 (default: 10)
	// Memories with this many accesses get full heat protection from decay
	ReferenceAccessCount int
}

// Gognee is the main entry point for the memory system
type Gognee struct {
	config            Config
	chunker           *chunker.Chunker
	embeddings        embeddings.EmbeddingClient
	llm               llm.LLMClient
	graphStore        store.GraphStore
	vectorStore       store.VectorStore
	memoryStore       *store.SQLiteMemoryStore
	searcher          search.Searcher
	entityExtractor   *extraction.EntityExtractor
	relationExtractor *extraction.RelationExtractor
	buffer            []AddedDocument
	lastCognified     time.Time
	metricsCollector  metrics.Collector // Optional metrics collector
	traceExporter     tracepkg.Exporter // Optional trace exporter (Plan 016 M4)
}

// RetentionPolicyDef defines the parameters for a retention policy (M6: Plan 021)
type RetentionPolicyDef struct {
	HalfLifeDays int  // Decay half-life in days (0 = no decay for permanent)
	Prunable     bool // Whether memories with this policy can be pruned
}

// RetentionPolicies defines all available retention policies (M6: Plan 021)
var RetentionPolicies = map[string]RetentionPolicyDef{
	"permanent": {HalfLifeDays: 0, Prunable: false},  // Never decays, never pruned
	"decision":  {HalfLifeDays: 365, Prunable: true}, // 1 year half-life, prunable only when superseded
	"standard":  {HalfLifeDays: 90, Prunable: true},  // 3 months half-life (default)
	"ephemeral": {HalfLifeDays: 7, Prunable: true},   // 1 week half-life
	"session":   {HalfLifeDays: 1, Prunable: true},   // 1 day half-life
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
	// SkipProcessed enables incremental mode, skipping previously processed documents.
	// Default: true (incremental by default). Use pointer to distinguish unset from explicit false.
	// When true, documents are identified by content hash (SHA-256).
	// Documents with matching hash are skipped unless Force is true.
	SkipProcessed *bool

	// Force reprocesses all documents regardless of cached state.
	// Overrides SkipProcessed when true.
	// Use after changing chunker settings or to rebuild the knowledge graph.
	Force bool

	// TraceEnabled enables detailed timing instrumentation for performance analysis.
	// Default: false (off by default to minimize overhead).
	// When enabled, timing spans are collected and returned in CognifyResult.Trace.
	TraceEnabled bool
}

// CognifyResult reports the outcome of a Cognify() operation
type CognifyResult struct {
	DocumentsProcessed int // Documents actually processed (chunked + extracted)
	DocumentsSkipped   int // Documents skipped due to incremental caching
	ChunksProcessed    int
	ChunksFailed       int
	NodesCreated       int
	EdgesCreated       int
	EdgesSkipped       int             // Count of edges skipped due to entity lookup failure or ambiguity
	Errors             []error         // Includes details of skipped edges ("skipped edge" in message)
	Trace              *OperationTrace // Timing data (populated when CognifyOptions.TraceEnabled is true)
}

// SearchResponse wraps search results with optional timing trace
type SearchResponse struct {
	Results []search.SearchResult // The search results
	Trace   *OperationTrace       // Timing data (populated when SearchOptions.TraceEnabled is true)
}

// Stats reports basic telemetry about the knowledge graph
type Stats struct {
	NodeCount     int64
	EdgeCount     int64
	MemoryCount   int64
	BufferedDocs  int
	LastCognified time.Time
}

// PruneOptions configures the Prune() method
type PruneOptions struct {
	// MaxAgeDays prunes nodes older than this many days (based on decay basis).
	// If zero, this criterion is not used.
	MaxAgeDays int

	// MinDecayScore prunes nodes with decay score below this threshold.
	// If zero, this criterion is not used.
	// Score is calculated using current decay settings.
	MinDecayScore float64

	// DryRun reports what would be pruned without actually deleting.
	DryRun bool

	// PruneSuperseded enables pruning of Superseded memories (M5: Plan 021, default: true)
	PruneSuperseded bool

	// SupersededAgeDays only prunes Superseded memories older than this (M5: Plan 021, default: 30)
	SupersededAgeDays int
}

// PruneResult reports the outcome of a Prune() operation
type PruneResult struct {
	NodesEvaluated int      // Total number of nodes considered
	NodesPruned    int      // Number of nodes deleted
	EdgesPruned    int      // Number of edges deleted (via cascade)
	NodeIDs        []string // IDs of pruned nodes (for verification)
	// SupersededMemoriesPruned is the count of Superseded memories pruned (M5: Plan 021)
	SupersededMemoriesPruned int
	// MemoriesEvaluated is the total number of memories considered for pruning (M5: Plan 021)
	MemoriesEvaluated int
}

// New creates a new Gognee instance using OpenAI clients
func New(cfg Config) (*Gognee, error) {
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

	return NewWithClients(cfg, embeddingsClient, llmClient)
}

// NewWithClients creates a new Gognee instance with custom embedding and LLM clients.
// This allows using alternative providers like Ollama for local inference.
func NewWithClients(cfg Config, embClient embeddings.EmbeddingClient, llmClient llm.LLMClient) (*Gognee, error) {
	// Apply defaults
	if cfg.ChunkSize == 0 {
		cfg.ChunkSize = 512
	}
	if cfg.ChunkOverlap == 0 {
		cfg.ChunkOverlap = 50
	}
	if cfg.DecayBasis == "" {
		cfg.DecayBasis = "access"
	}

	// Validate decay configuration (before applying half-life default)
	if cfg.DecayEnabled {
		if cfg.DecayHalfLifeDays < 0 {
			return nil, fmt.Errorf("DecayHalfLifeDays must be positive, got %d", cfg.DecayHalfLifeDays)
		}
		if cfg.DecayBasis != "access" && cfg.DecayBasis != "creation" {
			return nil, fmt.Errorf("DecayBasis must be 'access' or 'creation', got %q", cfg.DecayBasis)
		}
	}

	// Apply half-life default after validation
	if cfg.DecayHalfLifeDays == 0 {
		cfg.DecayHalfLifeDays = 30
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
	// Use SQLiteVectorStore for persistent databases, MemoryVectorStore for :memory:
	var vectorStore store.VectorStore
	if dbPath == ":memory:" {
		vectorStore = store.NewMemoryVectorStore()
	} else {
		// Share the database connection from GraphStore
		vectorStore = store.NewSQLiteVectorStore(graphStore.DB())
	}

	// Initialize extractors
	entityExtractor := extraction.NewEntityExtractor(llmClient)
	relationExtractor := extraction.NewRelationExtractor(llmClient)

	// Initialize searcher
	baseSearcher := search.NewHybridSearcher(embClient, vectorStore, graphStore)

	// Wrap with DecayingSearcher if decay is enabled
	var searcher search.Searcher
	if cfg.DecayEnabled {
		// Initialize MemoryStore early for DecayingSearcher (M2: Plan 021)
		memoryStore := store.NewSQLiteMemoryStore(graphStore.DB())
		searcher = search.NewDecayingSearcher(
			baseSearcher,
			graphStore,
			memoryStore,
			cfg.DecayEnabled,
			cfg.DecayHalfLifeDays,
			cfg.DecayBasis,
			cfg.AccessFrequencyEnabled,
			cfg.ReferenceAccessCount,
		)
	} else {
		searcher = baseSearcher
	}

	// Initialize MemoryStore (shares DB connection with GraphStore)
	// Note: If decay is enabled, this is a second instance; consider refactoring if needed
	memoryStore := store.NewSQLiteMemoryStore(graphStore.DB())

	// Initialize chunker
	c := &chunker.Chunker{
		MaxTokens: cfg.ChunkSize,
		Overlap:   cfg.ChunkOverlap,
	}

	return &Gognee{
		config:            cfg,
		chunker:           c,
		embeddings:        embClient,
		llm:               llmClient,
		graphStore:        graphStore,
		vectorStore:       vectorStore,
		memoryStore:       memoryStore,
		searcher:          searcher,
		entityExtractor:   entityExtractor,
		relationExtractor: relationExtractor,
		buffer:            make([]AddedDocument, 0),
		lastCognified:     time.Time{},
		metricsCollector:  nil, // Set via WithMetricsCollector
		traceExporter:     nil, // Set via WithTraceExporter (Plan 016 M4)
	}, nil
}

// WithMetricsCollector sets the metrics collector for this Gognee instance
func (g *Gognee) WithMetricsCollector(collector metrics.Collector) *Gognee {
	g.metricsCollector = collector
	return g
}

// WithTraceExporter sets the trace exporter for this Gognee instance (Plan 016 M4)
func (g *Gognee) WithTraceExporter(exporter tracepkg.Exporter) *Gognee {
	g.traceExporter = exporter
	return g
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

// normalizeEntityName applies normalization for entity lookup matching.
// Normalization: ToLower() + TrimSpace() + collapse internal whitespace
func normalizeEntityName(name string) string {
	// Trim leading/trailing whitespace
	normalized := strings.TrimSpace(name)
	// Convert to lowercase for case-insensitive matching
	normalized = strings.ToLower(normalized)
	// Collapse internal whitespace
	fields := strings.Fields(normalized)
	return strings.Join(fields, " ")
}

// buildEntityTypeMap creates a map from normalized entity names to their types.
// Returns the map and a set of ambiguous names (names that map to multiple types).
func buildEntityTypeMap(entities []extraction.Entity) (map[string]string, map[string]bool) {
	entityMap := make(map[string]string)
	typeCounts := make(map[string]map[string]bool) // normalized name -> set of types

	for _, entity := range entities {
		normalized := normalizeEntityName(entity.Name)
		if normalized == "" {
			continue // Skip empty names
		}

		// Track all types seen for this normalized name
		if typeCounts[normalized] == nil {
			typeCounts[normalized] = make(map[string]bool)
		}
		typeCounts[normalized][entity.Type] = true
	}

	// Build entity map, marking ambiguous names
	ambiguous := make(map[string]bool)
	for normalized, types := range typeCounts {
		if len(types) > 1 {
			// Multiple types for same name = ambiguous
			ambiguous[normalized] = true
		} else {
			// Single type - safe to use
			for typ := range types {
				entityMap[normalized] = typ
				break
			}
		}
	}

	return entityMap, ambiguous
}

// lookupEntityType looks up the entity type by name using the entity map.
// Returns empty string if not found or ambiguous.
func lookupEntityType(name string, entityMap map[string]string, ambiguous map[string]bool) (string, bool) {
	normalized := normalizeEntityName(name)

	if ambiguous[normalized] {
		return "", false // Ambiguous - multiple types
	}

	typ, found := entityMap[normalized]
	return typ, found
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
	startTime := time.Now()
	operationID := uuid.New().String() // Generate operation ID for trace correlation

	result := &CognifyResult{
		Errors: make([]error, 0),
	}

	// Initialize trace if enabled
	var trace *OperationTrace
	if opts.TraceEnabled {
		trace = newTrace()
		result.Trace = trace
	}

	// No-op if buffer is empty
	if len(g.buffer) == 0 {
		return result, nil
	}

	// Apply default for SkipProcessed (incremental by default)
	skipProcessed := true
	if opts.SkipProcessed != nil {
		skipProcessed = *opts.SkipProcessed
	}

	// Try to get DocumentTracker interface from graphStore (optional)
	// If not available, incremental mode is disabled
	tracker, _ := g.graphStore.(store.DocumentTracker)

	// Process each document
	for _, doc := range g.buffer {
		// Compute document hash for identity
		hash := computeDocumentHash(doc.Text)

		// Check if document is already processed (incremental mode)
		// Only if tracker is available and incremental mode is enabled
		if tracker != nil && skipProcessed && !opts.Force {
			processed, err := tracker.IsDocumentProcessed(ctx, hash)
			if err != nil {
				return nil, fmt.Errorf("failed to check document processed status: %w", err)
			}

			if processed {
				result.DocumentsSkipped++
				continue // Skip this document
			}
		}

		// Track chunks for this document
		docChunkCount := 0
		result.DocumentsProcessed++

		// Chunk the text
		chunkTimer := newSpanTimer("chunk", trace, opts.TraceEnabled)
		chunks := g.chunker.Chunk(doc.Text)
		chunkTimer.finish(true, nil, map[string]int64{"chunkCount": int64(len(chunks))})

		// Process each chunk
		for _, chunk := range chunks {
			result.ChunksProcessed++
			docChunkCount++

			// Extract entities
			extractTimer := newSpanTimer("extract", trace, opts.TraceEnabled)
			entities, err := g.entityExtractor.Extract(ctx, chunk.Text)
			if err != nil {
				extractTimer.finish(false, err, nil)
				result.ChunksFailed++
				result.Errors = append(result.Errors, fmt.Errorf("entity extraction failed for chunk %s: %w", chunk.ID, err))
				continue
			}

			// Build entity name->type lookup map before processing triplets
			entityMap, ambiguous := buildEntityTypeMap(entities)

			// Extract relations
			triplets, err := g.relationExtractor.Extract(ctx, chunk.Text, entities)
			if err != nil {
				extractTimer.finish(false, err, nil)
				result.ChunksFailed++
				result.Errors = append(result.Errors, fmt.Errorf("relation extraction failed for chunk %s: %w", chunk.ID, err))
				// Continue with entities only if relations fail
			} else {
				extractTimer.finish(true, nil, map[string]int64{
					"entityCount":   int64(len(entities)),
					"relationCount": int64(len(triplets)),
				})
			}

			// Create nodes for each entity
			graphWriteTimer := newSpanTimer("write-graph", trace, opts.TraceEnabled)
			embedTimer := newSpanTimer("embed", trace, opts.TraceEnabled)
			vectorWriteTimer := newSpanTimer("write-vector", trace, opts.TraceEnabled)

			// Collect all entity texts for batch embedding (Plan 019: M1)
			var textsToEmbed []string
			var entityIndices []int
			for i, entity := range entities {
				text := strings.TrimSpace(entity.Name + " " + entity.Description)
				if text != "" {
					textsToEmbed = append(textsToEmbed, text)
					entityIndices = append(entityIndices, i)
				}
			}

			// Batch embed all entities in single API call (Plan 019: M2)
			var embeddings [][]float32
			var embedErr error
			if len(textsToEmbed) > 0 {
				embeddings, embedErr = g.embeddings.Embed(ctx, textsToEmbed)
				if embedErr != nil {
					embedTimer.finish(false, embedErr, nil)
					result.ChunksFailed++
					result.Errors = append(result.Errors, fmt.Errorf("batch embedding failed for chunk %s: %w", chunk.ID, embedErr))
					// Continue without embeddings - nodes will be created but not indexed
				} else {
					embedTimer.finish(true, nil, map[string]int64{"embeddingCount": int64(len(embeddings))})
				}
			}

			// Create nodes and assign embeddings (Plan 019: M3)
			nodesAdded := 0
			for i, entity := range entities {
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
				nodesAdded++

				// Find embedding for this entity from batch results
				var embedding []float32
				if embedErr == nil {
					// Find index in entityIndices that corresponds to this entity
					for j, entityIdx := range entityIndices {
						if entityIdx == i && j < len(embeddings) {
							embedding = embeddings[j]
							break
						}
					}
				}

				// Update node with embedding if available
				if embedding != nil {
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
			}

			vectorWriteTimer.finish(true, nil, map[string]int64{"nodeUpserts": int64(nodesAdded)})

			// Create edges for each triplet
			edgesAdded := 0
			for _, triplet := range triplets {
				// Look up source entity type
				sourceType, sourceFound := lookupEntityType(triplet.Subject, entityMap, ambiguous)
				if !sourceFound {
					result.EdgesSkipped++
					if ambiguous[normalizeEntityName(triplet.Subject)] {
						result.Errors = append(result.Errors, fmt.Errorf("skipped edge %s-%s-%s: subject '%s' is ambiguous (multiple types)",
							triplet.Subject, triplet.Relation, triplet.Object, triplet.Subject))
					} else {
						result.Errors = append(result.Errors, fmt.Errorf("skipped edge %s-%s-%s: subject '%s' not found in extracted entities",
							triplet.Subject, triplet.Relation, triplet.Object, triplet.Subject))
					}
					continue
				}

				// Look up target entity type
				targetType, targetFound := lookupEntityType(triplet.Object, entityMap, ambiguous)
				if !targetFound {
					result.EdgesSkipped++
					if ambiguous[normalizeEntityName(triplet.Object)] {
						result.Errors = append(result.Errors, fmt.Errorf("skipped edge %s-%s-%s: object '%s' is ambiguous (multiple types)",
							triplet.Subject, triplet.Relation, triplet.Object, triplet.Object))
					} else {
						result.Errors = append(result.Errors, fmt.Errorf("skipped edge %s-%s-%s: object '%s' not found in extracted entities",
							triplet.Subject, triplet.Relation, triplet.Object, triplet.Object))
					}
					continue
				}

				// Generate edge IDs using correct entity types (FIX: was using empty string)
				sourceID := generateDeterministicNodeID(triplet.Subject, sourceType)
				targetID := generateDeterministicNodeID(triplet.Object, targetType)

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
				edgesAdded++
			}

			graphWriteTimer.finish(true, nil, map[string]int64{
				"nodeUpserts": int64(nodesAdded),
				"edgeUpserts": int64(edgesAdded),
			})
		}

		// Mark document as processed after successful processing (if tracker available)
		if tracker != nil {
			if err := tracker.MarkDocumentProcessed(ctx, hash, doc.Source, docChunkCount); err != nil {
				// Log but don't fail - tracking failure shouldn't break Cognify
				result.Errors = append(result.Errors, fmt.Errorf("failed to mark document as processed: %w", err))
			}
		}
	}

	// Always clear buffer after processing (best-effort semantics)
	g.buffer = make([]AddedDocument, 0)
	g.lastCognified = time.Now()

	// Record metrics if collector is available
	if g.metricsCollector != nil {
		durationMs := time.Since(startTime).Milliseconds()
		status := "success"
		if len(result.Errors) > 0 {
			status = "error"
		}
		g.metricsCollector.RecordOperation(ctx, "cognify", status, durationMs)

		// Record stage timings from trace if available
		if trace != nil {
			for _, span := range trace.Spans {
				g.metricsCollector.RecordStage(ctx, "cognify", span.Name, span.DurationMs)
				if !span.OK {
					g.metricsCollector.RecordError(ctx, "cognify", span.ErrorType)
				}
			}
		}
	}

	// Export trace if enabled (Plan 016 M4)
	if trace != nil {
		var err error
		if len(result.Errors) > 0 {
			err = result.Errors[0] // Use first error for classification
		}
		g.exportTrace(ctx, operationID, "cognify", trace, startTime, err, map[string]interface{}{
			"documentsProcessed": result.DocumentsProcessed,
			"documentsSkipped":   result.DocumentsSkipped,
			"nodesCreated":       result.NodesCreated,
			"edgesCreated":       result.EdgesCreated,
		})
	}

	return result, nil
}

// Search queries the knowledge graph
func (g *Gognee) Search(ctx context.Context, query string, opts search.SearchOptions) (*SearchResponse, error) {
	startTime := time.Now()
	operationID := uuid.New().String() // Generate operation ID for trace correlation
	search.ApplyDefaults(&opts)

	// Initialize trace if enabled
	var trace *OperationTrace
	var searchTimer *spanTimer
	if opts.TraceEnabled {
		trace = newTrace()
		searchTimer = newSpanTimer("search-vector", trace, true)
	}

	// Apply default for IncludeMemoryIDs (true by default)
	includeMemoryIDs := true
	if opts.IncludeMemoryIDs != nil {
		includeMemoryIDs = *opts.IncludeMemoryIDs
	}

	results, err := g.searcher.Search(ctx, query, opts)
	if err != nil {
		if searchTimer != nil {
			searchTimer.finish(false, err, nil)
		}
		// Record error metrics
		if g.metricsCollector != nil {
			durationMs := time.Since(startTime).Milliseconds()
			g.metricsCollector.RecordOperation(ctx, "search", "error", durationMs)
			g.metricsCollector.RecordError(ctx, "search", "search_error")
		}
		// Export trace on error (Plan 016 M4)
		if trace != nil {
			g.exportTrace(ctx, operationID, "search", trace, startTime, err, map[string]interface{}{
				"query": "(redacted)", // Don't include query text in trace
			})
		}
		return nil, err
	}

	if searchTimer != nil {
		searchTimer.finish(true, nil, map[string]int64{"resultsReturned": int64(len(results))})
	}

	// Update access times for returned results (for decay reinforcement)
	if len(results) > 0 {
		nodeIDs := make([]string, len(results))
		for i, result := range results {
			nodeIDs[i] = result.NodeID
		}

		// Cast to SQLiteGraphStore to access UpdateAccessTime
		if sqlStore, ok := g.graphStore.(*store.SQLiteGraphStore); ok {
			// Best-effort update - don't fail search if access tracking fails
			_ = sqlStore.UpdateAccessTime(ctx, nodeIDs)
		}

		// Enrich with memory provenance (batched query, no N+1)
		if includeMemoryIDs {
			memoryMap, err := g.memoryStore.GetMemoriesByNodeIDBatched(ctx, nodeIDs)
			if err != nil {
				// Log but don't fail - provenance enrichment is optional
				// In production, could use a logger here
			} else {
				// Populate MemoryIDs for each result
				for i := range results {
					if memIDs, ok := memoryMap[results[i].NodeID]; ok {
						results[i].MemoryIDs = memIDs
					} else {
						results[i].MemoryIDs = []string{} // Empty for legacy nodes
					}
				}

				// CRITICAL: Update access tracking for all memories returned by search
				// This ensures the primary read path (search) drives frequency signal (Milestone 1)
				allMemoryIDs := make([]string, 0)
				for _, memIDs := range memoryMap {
					allMemoryIDs = append(allMemoryIDs, memIDs...)
				}
				if len(allMemoryIDs) > 0 {
					// Best-effort update - don't fail search if access tracking fails
					_ = g.memoryStore.BatchUpdateMemoryAccess(ctx, allMemoryIDs)
				}
			}
		}
	}

	// Record success metrics
	if g.metricsCollector != nil {
		durationMs := time.Since(startTime).Milliseconds()
		g.metricsCollector.RecordOperation(ctx, "search", "success", durationMs)

		// Record stage timings from trace if available
		if trace != nil {
			for _, span := range trace.Spans {
				g.metricsCollector.RecordStage(ctx, "search", span.Name, span.DurationMs)
			}
		}
	}

	// Export trace if enabled (Plan 016 M4)
	if trace != nil {
		g.exportTrace(ctx, operationID, "search", trace, startTime, nil, map[string]interface{}{
			"resultsReturned": len(results),
		})
	}

	return &SearchResponse{
		Results: results,
		Trace:   trace,
	}, nil
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

	memoryCount, err := g.memoryStore.CountMemories(ctx)
	if err != nil {
		return Stats{}, fmt.Errorf("failed to get memory count: %w", err)
	}

	return Stats{
		NodeCount:     nodeCount,
		EdgeCount:     edgeCount,
		MemoryCount:   memoryCount,
		BufferedDocs:  len(g.buffer),
		LastCognified: g.lastCognified,
	}, nil
}

// Prune removes old or low-scoring nodes from the knowledge graph.
// Edges connected to pruned nodes are also deleted (cascade).
// Use DryRun to preview what would be pruned without actually deleting.
func (g *Gognee) Prune(ctx context.Context, opts PruneOptions) (*PruneResult, error) {
	result := &PruneResult{
		NodeIDs: make([]string, 0),
	}

	// Set default values for supersession options (M5: Plan 021)
	if opts.PruneSuperseded && opts.SupersededAgeDays == 0 {
		opts.SupersededAgeDays = 30 // Default grace period
	}

	// **Phase 1: Evaluate and prune memories based on supersession and retention policies (M5, M8: Plan 021)**
	if opts.PruneSuperseded {
		// Query all memories with status='Superseded'
		allMemories, err := g.memoryStore.ListMemories(ctx, store.ListMemoriesOptions{
			Offset: 0,
			Limit:  10000, // Large limit to get all memories
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list memories: %w", err)
		}

		result.MemoriesEvaluated = len(allMemories)

		now := time.Now()
		memoriesToPrune := make([]string, 0)

		for _, summary := range allMemories {
			shouldPrune := false

			// M9: Never prune pinned memories
			if summary.Pinned {
				continue
			}

			// M8: Check retention_until override (if set and in the past, eligible for prune)
			// For this we need to fetch the full memory record to check retention_until
			memory, err := g.memoryStore.GetMemory(ctx, summary.ID)
			if err != nil {
				continue // Skip on error
			}

			// M8: retention_until override
			if memory.RetentionUntil != nil {
				if now.After(*memory.RetentionUntil) {
					shouldPrune = true
				} else {
					continue // Not yet expired
				}
			}

			// M8: permanent retention policy - never pruned
			if memory.RetentionPolicy == "permanent" {
				continue
			}

			// M8: decision retention policy - only pruned when Superseded + grace period
			if memory.RetentionPolicy == "decision" && summary.Status == "Superseded" {
				age := now.Sub(summary.UpdatedAt)
				ageDays := int(age.Hours() / 24)
				if ageDays >= opts.SupersededAgeDays {
					shouldPrune = true
				}
			}

			// M5: Superseded memories past grace period
			if summary.Status == "Superseded" && memory.RetentionPolicy != "decision" {
				age := now.Sub(summary.UpdatedAt)
				ageDays := int(age.Hours() / 24)
				if ageDays >= opts.SupersededAgeDays {
					shouldPrune = true
				}
			}

			if shouldPrune {
				memoriesToPrune = append(memoriesToPrune, summary.ID)
			}
		}

		result.SupersededMemoriesPruned = len(memoriesToPrune)

		// If not dry run, delete the memories
		if !opts.DryRun {
			for _, memoryID := range memoriesToPrune {
				if err := g.DeleteMemory(ctx, memoryID); err != nil {
					// Continue on error to prune as much as possible
					_ = err
				}
			}
		}
	}

	// **Phase 2: Evaluate and prune nodes based on decay/age (existing logic)**
	// Get all nodes for evaluation
	sqlStore, ok := g.graphStore.(*store.SQLiteGraphStore)
	if !ok {
		return nil, fmt.Errorf("prune requires SQLiteGraphStore")
	}

	// Query all nodes
	allNodes, err := sqlStore.GetAllNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	result.NodesEvaluated = len(allNodes)

	// Evaluate each node for pruning
	now := time.Now()
	nodesToPrune := make([]string, 0)

	for _, node := range allNodes {
		shouldPrune := false

		// Check MaxAgeDays criterion
		if opts.MaxAgeDays > 0 {
			var age time.Duration
			if g.config.DecayBasis == "access" && node.LastAccessedAt != nil {
				age = now.Sub(*node.LastAccessedAt)
			} else {
				age = now.Sub(node.CreatedAt)
			}

			ageDays := int(age.Hours() / 24)
			if ageDays > opts.MaxAgeDays {
				shouldPrune = true
			}
		}

		// Check MinDecayScore criterion
		if opts.MinDecayScore > 0 && g.config.DecayEnabled {
			var age time.Duration
			if g.config.DecayBasis == "access" && node.LastAccessedAt != nil {
				age = now.Sub(*node.LastAccessedAt)
			} else {
				age = now.Sub(node.CreatedAt)
			}

			decayScore := calculateDecay(age, g.config.DecayHalfLifeDays)
			if decayScore < opts.MinDecayScore {
				shouldPrune = true
			}
		}

		if shouldPrune {
			nodesToPrune = append(nodesToPrune, node.ID)
		}
	}

	result.NodesPruned = len(nodesToPrune)
	result.NodeIDs = nodesToPrune

	// If dry run, stop here
	if opts.DryRun {
		// Estimate edges that would be pruned
		for _, nodeID := range nodesToPrune {
			edges, err := g.graphStore.GetEdges(ctx, nodeID)
			if err == nil {
				result.EdgesPruned += len(edges)
			}
		}
		return result, nil
	}

	// Actually prune nodes and edges
	for _, nodeID := range nodesToPrune {
		// Delete edges first (cascade)
		edges, err := g.graphStore.GetEdges(ctx, nodeID)
		if err != nil {
			continue
		}
		result.EdgesPruned += len(edges)

		// Delete the edges
		for _, edge := range edges {
			if err := sqlStore.DeleteEdge(ctx, edge.ID); err != nil {
				// Continue on error to prune as much as possible
				continue
			}
		}

		// Delete from vector store (ignore errors to prune as much as possible)
		if err := g.vectorStore.Delete(ctx, nodeID); err != nil {
			_ = err
		}

		// Delete the node
		if err := sqlStore.DeleteNode(ctx, nodeID); err != nil {
			// Continue on error
			continue
		}
	}

	return result, nil
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

// computeDocumentHash computes a SHA-256 hash of document text for identity.
// Used for document-level deduplication in incremental Cognify.
// Hash is computed on exact text without normalization to detect any changes.
func computeDocumentHash(text string) string {
	hash := sha256.Sum256([]byte(text))
	return fmt.Sprintf("%x", hash[:])
}

// ========================================
// Memory CRUD APIs (v1.0.0)
// ========================================

// MemoryInput represents the input for creating a memory.
type MemoryInput struct {
	Topic     string
	Context   string
	Decisions []string
	Rationale []string
	Metadata  map[string]interface{}
	Source    string
	// TraceEnabled enables timing instrumentation (Plan 015)
	TraceEnabled bool
	// Supersedes lists memory IDs that this new memory replaces (M4: Plan 021)
	Supersedes []string
	// SupersessionReason explains why this memory supersedes the old ones (M4: Plan 021)
	SupersessionReason string
	// RetentionPolicy sets the retention policy for this memory (M6: Plan 021)
	// Valid values: permanent, decision, standard, ephemeral, session (default: standard)
	RetentionPolicy string
}

// MemoryResult reports the outcome of memory operations.
type MemoryResult struct {
	MemoryID     string
	NodesCreated int
	EdgesCreated int
	NodesDeleted int
	EdgesDeleted int
	Errors       []error
	// Trace contains timing data when TraceEnabled is true (Plan 015)
	Trace *OperationTrace
	// MemoriesSuperseded is the count of memories marked as Superseded (M4: Plan 021)
	MemoriesSuperseded int
}

// AddMemory creates a new first-class memory with full CRUD support.
// Uses two-phase model: persist memory record → cognify → link provenance.
func (g *Gognee) AddMemory(ctx context.Context, input MemoryInput) (*MemoryResult, error) {
	startTime := time.Now()
	operationID := uuid.New().String() // Generate operation ID for trace correlation

	result := &MemoryResult{
		Errors: make([]error, 0),
	}

	// Initialize trace if enabled (Plan 015 M2)
	var trace *OperationTrace
	if input.TraceEnabled {
		trace = newTrace()
		result.Trace = trace
	}

	// Validate input
	if strings.TrimSpace(input.Topic) == "" {
		return nil, fmt.Errorf("topic cannot be empty")
	}
	if strings.TrimSpace(input.Context) == "" {
		return nil, fmt.Errorf("context cannot be empty")
	}

	// Validate retention policy (M6: Plan 021)
	if input.RetentionPolicy == "" {
		input.RetentionPolicy = "standard" // Default
	}
	if _, valid := RetentionPolicies[input.RetentionPolicy]; !valid {
		return nil, fmt.Errorf("invalid retention_policy '%s': must be one of: permanent, decision, standard, ephemeral, session", input.RetentionPolicy)
	}

	// Compute doc_hash
	docHash := store.ComputeDocHash(input.Topic, input.Context, input.Decisions, input.Rationale)

	// **Phase 1: Short transaction - persist memory record**
	// Check for duplicate by doc_hash
	// For v1.0.0, we'll do a simple query to check existence
	// If exists, return existing memory_id

	existingQuery := `SELECT id FROM memories WHERE doc_hash = ? LIMIT 1`
	var existingID string
	err := g.memoryStore.DB().QueryRowContext(ctx, existingQuery, docHash).Scan(&existingID)
	if err == nil {
		// Duplicate found
		result.MemoryID = existingID
		return result, nil
	}
	// If error is not ErrNoRows, return error
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check for duplicate memory: %w", err)
	}

	// Create memory record with status "pending"
	memoryID := uuid.New().String()
	memory := &store.MemoryRecord{
		ID:              memoryID,
		Topic:           strings.TrimSpace(input.Topic),
		Context:         strings.TrimSpace(input.Context),
		Decisions:       input.Decisions,
		Rationale:       input.Rationale,
		Metadata:        input.Metadata,
		DocHash:         docHash,
		Source:          input.Source,
		Status:          "pending",
		RetentionPolicy: input.RetentionPolicy, // M6: Plan 021
	}

	if err := g.memoryStore.AddMemory(ctx, memory); err != nil {
		return nil, fmt.Errorf("failed to add memory record: %w", err)
	}

	result.MemoryID = memoryID

	// **Phase 2: Cognify (outside transaction, idempotent)**
	cognifyTimer := newSpanTimer("cognify", trace, input.TraceEnabled)

	// Format text for cognify
	text := fmt.Sprintf("Topic: %s\n\n%s", input.Topic, input.Context)

	// Track created node/edge IDs
	createdNodeIDs := make([]string, 0)
	createdEdgeIDs := make([]string, 0)

	// Chunk the text
	chunks := g.chunker.Chunk(text)
	fmt.Fprintf(os.Stderr, "gognee: AddMemory starting: memoryID=%s chunks=%d\n", memoryID, len(chunks))

	// Process each chunk
	for chunkIdx, chunk := range chunks {
		chunkStart := time.Now()
		fmt.Fprintf(os.Stderr, "gognee: chunk[%d] processing chunkLen=%d\n", chunkIdx, len(chunk.Text))

		// Extract entities
		entityStart := time.Now()
		entities, err := g.entityExtractor.Extract(ctx, chunk.Text)
		entityDuration := time.Since(entityStart)
		fmt.Fprintf(os.Stderr, "gognee: chunk[%d] entity extraction: duration=%v count=%d\n", chunkIdx, entityDuration, len(entities))
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("entity extraction failed for memory %s: %w", memoryID, err))
			continue
		}

		// Build entity name->type lookup map
		entityMap, ambiguous := buildEntityTypeMap(entities)

		// Extract relations
		relationStart := time.Now()
		triplets, err := g.relationExtractor.Extract(ctx, chunk.Text, entities)
		relationDuration := time.Since(relationStart)
		fmt.Fprintf(os.Stderr, "gognee: chunk[%d] relation extraction: duration=%v count=%d\n", chunkIdx, relationDuration, len(triplets))
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("relation extraction failed for memory %s: %w", memoryID, err))
			// Continue with entities only
		}

		// Create nodes for each entity
		// First pass: collect texts for batch embedding
		entityTexts := make([]string, len(entities))
		for i, entity := range entities {
			entityTexts[i] = entity.Name + " " + entity.Description
		}

		// Batch embed all entities at once
		embedStart := time.Now()
		embeddings, err := g.embeddings.Embed(ctx, entityTexts)
		embedDuration := time.Since(embedStart)
		fmt.Fprintf(os.Stderr, "gognee: chunk[%d] batch embedding: duration=%v count=%d\n", chunkIdx, embedDuration, len(entityTexts))
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("batch embed failed for memory %s: %w", memoryID, err))
			// Continue without embeddings
			embeddings = make([][]float32, len(entities))
		}

		// Second pass: create nodes with embeddings
		dbStart := time.Now()
		for i, entity := range entities {
			nodeID := generateDeterministicNodeID(entity.Name, entity.Type)
			node := &store.Node{
				ID:          nodeID,
				Name:        entity.Name,
				Type:        entity.Type,
				Description: entity.Description,
				CreatedAt:   time.Now(),
				Metadata:    make(map[string]interface{}),
				Embedding:   embeddings[i],
			}

			// Add to graph store (upsert) with embedding
			if err := g.graphStore.AddNode(ctx, node); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("failed to add node %s: %w", entity.Name, err))
				continue
			}
			createdNodeIDs = append(createdNodeIDs, nodeID)
			result.NodesCreated++

			// Index in vector store
			if len(embeddings[i]) > 0 {
				if err := g.vectorStore.Add(ctx, nodeID, embeddings[i]); err != nil {
					result.Errors = append(result.Errors, fmt.Errorf("failed to index node %s in vector store: %w", entity.Name, err))
				}
			}
		}
		dbNodesDuration := time.Since(dbStart)

		// Create edges for each triplet
		edgeStart := time.Now()
		for _, triplet := range triplets {
			// Look up source and target entity types
			sourceType, sourceFound := lookupEntityType(triplet.Subject, entityMap, ambiguous)
			if !sourceFound {
				continue
			}

			targetType, targetFound := lookupEntityType(triplet.Object, entityMap, ambiguous)
			if !targetFound {
				continue
			}

			sourceID := generateDeterministicNodeID(triplet.Subject, sourceType)
			targetID := generateDeterministicNodeID(triplet.Object, targetType)

			edgeID := fmt.Sprintf("%s-%s-%s", sourceID, sanitizeRelation(triplet.Relation), targetID)
			edge := &store.Edge{
				ID:        edgeID,
				SourceID:  sourceID,
				Relation:  triplet.Relation,
				TargetID:  targetID,
				Weight:    1.0,
				CreatedAt: time.Now(),
			}

			if err := g.graphStore.AddEdge(ctx, edge); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("failed to add edge: %w", err))
				continue
			}
			createdEdgeIDs = append(createdEdgeIDs, edgeID)
			result.EdgesCreated++
		}
		edgeDuration := time.Since(edgeStart)
		chunkDuration := time.Since(chunkStart)
		fmt.Fprintf(os.Stderr, "gognee: chunk[%d] complete: total=%v dbNodes=%v dbEdges=%v\n", chunkIdx, chunkDuration, dbNodesDuration, edgeDuration)
	}

	// Finish cognify timer
	cognifyTimer.finish(true, nil, map[string]int64{
		"nodesCreated": int64(result.NodesCreated),
		"edgesCreated": int64(result.EdgesCreated),
	})

	// **Phase 3: Short transaction - link provenance and mark complete**
	if err := g.memoryStore.LinkProvenance(ctx, memoryID, createdNodeIDs, createdEdgeIDs); err != nil {
		return nil, fmt.Errorf("failed to link provenance: %w", err)
	}

	// **Phase 4: Handle supersession if provided (M4: Plan 021)**
	if len(input.Supersedes) > 0 {
		for _, supersededID := range input.Supersedes {
			// Validate that superseded memory exists and is Active
			supersededMemory, err := g.memoryStore.GetMemory(ctx, supersededID)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("cannot supersede memory %s: %w", supersededID, err))
				continue
			}

			// Allow superseding Active or already-Superseded memories (creates chains)
			if supersededMemory.Status != "Active" && supersededMemory.Status != "Superseded" {
				result.Errors = append(result.Errors, fmt.Errorf("cannot supersede memory %s: status is '%s', must be 'Active' or 'Superseded'", supersededID, supersededMemory.Status))
				continue
			}

			// Record supersession
			if err := g.memoryStore.RecordSupersession(ctx, memoryID, supersededID, input.SupersessionReason); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("failed to record supersession: %w", err))
				continue
			}

			// Mark superseded memory as Superseded
			supersededStatus := "Superseded"
			supersededUpdates := store.MemoryUpdate{
				Status: &supersededStatus,
			}
			if err := g.memoryStore.UpdateMemory(ctx, supersededID, supersededUpdates); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("failed to mark memory %s as superseded: %w", supersededID, err))
				continue
			}

			result.MemoriesSuperseded++
		}
	}

	// Update memory status to "complete"
	completeStatus := "complete"
	updates := store.MemoryUpdate{
		Topic:   &memory.Topic, // Keep same
		Context: &memory.Context,
		Status:  &completeStatus,
	}
	if err := g.memoryStore.UpdateMemory(ctx, memoryID, updates); err != nil {
		return nil, fmt.Errorf("failed to mark memory complete: %w", err)
	}

	// Record success metrics
	if g.metricsCollector != nil {
		durationMs := time.Since(startTime).Milliseconds()
		status := "success"
		if len(result.Errors) > 0 {
			status = "error"
		}
		g.metricsCollector.RecordOperation(ctx, "add_memory", status, durationMs)

		// Record stage timings from trace if available
		if trace != nil {
			for _, span := range trace.Spans {
				g.metricsCollector.RecordStage(ctx, "add_memory", span.Name, span.DurationMs)
				if !span.OK {
					g.metricsCollector.RecordError(ctx, "add_memory", span.ErrorType)
				}
			}
		}
	}

	// Export trace if enabled (Plan 016 M4)
	if trace != nil {
		var err error
		if len(result.Errors) > 0 {
			err = result.Errors[0]
		}
		g.exportTrace(ctx, operationID, "add_memory", trace, startTime, err, map[string]interface{}{
			"memoryId":     result.MemoryID,
			"nodesCreated": result.NodesCreated,
			"edgesCreated": result.EdgesCreated,
		})
	}

	return result, nil
}

// GetMemory retrieves a memory by ID.
func (g *Gognee) GetMemory(ctx context.Context, id string) (*store.MemoryRecord, error) {
	return g.memoryStore.GetMemory(ctx, id)
}

// ListMemories returns paginated memory summaries.
func (g *Gognee) ListMemories(ctx context.Context, opts store.ListMemoriesOptions) ([]store.MemorySummary, error) {
	return g.memoryStore.ListMemories(ctx, opts)
}

// CountMemories returns the total number of memories in the store.
func (g *Gognee) CountMemories(ctx context.Context) (int64, error) {
	return g.memoryStore.CountMemories(ctx)
}

// UpdateMemory applies partial updates to a memory and re-cognifies if content changed.
func (g *Gognee) UpdateMemory(ctx context.Context, id string, updates store.MemoryUpdate) (*MemoryResult, error) {
	result := &MemoryResult{
		MemoryID: id,
		Errors:   make([]error, 0),
	}

	// Fetch existing memory
	existing, err := g.memoryStore.GetMemory(ctx, id)
	if err != nil {
		return nil, err
	}

	// Compute new doc_hash
	topic := existing.Topic
	context := existing.Context
	decisions := existing.Decisions
	rationale := existing.Rationale

	if updates.Topic != nil {
		topic = *updates.Topic
	}
	if updates.Context != nil {
		context = *updates.Context
	}
	if updates.Decisions != nil {
		decisions = *updates.Decisions
	}
	if updates.Rationale != nil {
		rationale = *updates.Rationale
	}

	newDocHash := store.ComputeDocHash(topic, context, decisions, rationale)

	// If hash unchanged, just update metadata/timestamps (no re-cognify)
	if newDocHash == existing.DocHash {
		if err := g.memoryStore.UpdateMemory(ctx, id, updates); err != nil {
			return nil, fmt.Errorf("failed to update memory: %w", err)
		}
		return result, nil
	}

	// **Phase 1: Set status to "pending"**
	pendingUpdate := store.MemoryUpdate{
		Topic:   &topic,
		Context: &context,
		Status:  stringPtr("pending"),
	}
	pendingUpdate.Decisions = &decisions
	pendingUpdate.Rationale = &rationale
	if updates.Metadata != nil {
		pendingUpdate.Metadata = updates.Metadata
	}

	// Update the memory with new content (will recompute hash in store)
	if err := g.memoryStore.UpdateMemory(ctx, id, pendingUpdate); err != nil {
		return nil, fmt.Errorf("failed to update memory to pending: %w", err)
	}

	// **Phase 2: Get old provenance, unlink, and GC candidates**
	oldNodeIDs, oldEdgeIDs, err := g.memoryStore.GetProvenanceByMemory(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get old provenance: %w", err)
	}

	if err := g.memoryStore.UnlinkProvenance(ctx, id); err != nil {
		return nil, fmt.Errorf("failed to unlink old provenance: %w", err)
	}

	// GC candidates: old artifacts
	nodesDeleted, edgesDeleted, err := g.memoryStore.GarbageCollectCandidates(ctx, oldNodeIDs, oldEdgeIDs)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("garbage collection failed: %w", err))
	}
	result.NodesDeleted = nodesDeleted
	result.EdgesDeleted = edgesDeleted

	// **Phase 3: Re-cognify (same as AddMemory Phase 2)**
	text := fmt.Sprintf("Topic: %s\n\n%s", topic, context)
	createdNodeIDs := make([]string, 0)
	createdEdgeIDs := make([]string, 0)

	chunks := g.chunker.Chunk(text)
	for _, chunk := range chunks {
		entities, err := g.entityExtractor.Extract(ctx, chunk.Text)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("entity extraction failed: %w", err))
			continue
		}

		entityMap, ambiguous := buildEntityTypeMap(entities)

		triplets, err := g.relationExtractor.Extract(ctx, chunk.Text, entities)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("relation extraction failed: %w", err))
		}

		// First pass: collect texts for batch embedding
		entityTexts := make([]string, len(entities))
		for i, entity := range entities {
			entityTexts[i] = entity.Name + " " + entity.Description
		}

		// Batch embed all entities at once
		embeddings, err := g.embeddings.Embed(ctx, entityTexts)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("batch embed failed: %w", err))
			// Continue without embeddings
			embeddings = make([][]float32, len(entities))
		}

		// Second pass: create nodes with embeddings
		for i, entity := range entities {
			nodeID := generateDeterministicNodeID(entity.Name, entity.Type)
			node := &store.Node{
				ID:          nodeID,
				Name:        entity.Name,
				Type:        entity.Type,
				Description: entity.Description,
				CreatedAt:   time.Now(),
				Metadata:    make(map[string]interface{}),
				Embedding:   embeddings[i],
			}

			if err := g.graphStore.AddNode(ctx, node); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("failed to add node: %w", err))
				continue
			}
			createdNodeIDs = append(createdNodeIDs, nodeID)
			result.NodesCreated++

			if len(embeddings[i]) > 0 {
				if err := g.vectorStore.Add(ctx, nodeID, embeddings[i]); err != nil {
					result.Errors = append(result.Errors, fmt.Errorf("failed to index node in vector store: %w", err))
				}
			}
		}

		for _, triplet := range triplets {
			sourceType, sourceFound := lookupEntityType(triplet.Subject, entityMap, ambiguous)
			if !sourceFound {
				continue
			}

			targetType, targetFound := lookupEntityType(triplet.Object, entityMap, ambiguous)
			if !targetFound {
				continue
			}

			sourceID := generateDeterministicNodeID(triplet.Subject, sourceType)
			targetID := generateDeterministicNodeID(triplet.Object, targetType)
			edgeID := fmt.Sprintf("%s-%s-%s", sourceID, sanitizeRelation(triplet.Relation), targetID)

			edge := &store.Edge{
				ID:        edgeID,
				SourceID:  sourceID,
				Relation:  triplet.Relation,
				TargetID:  targetID,
				Weight:    1.0,
				CreatedAt: time.Now(),
			}

			if err := g.graphStore.AddEdge(ctx, edge); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("failed to add edge: %w", err))
				continue
			}
			createdEdgeIDs = append(createdEdgeIDs, edgeID)
			result.EdgesCreated++
		}
	}

	// **Phase 4: Link new provenance and mark complete**
	if err := g.memoryStore.LinkProvenance(ctx, id, createdNodeIDs, createdEdgeIDs); err != nil {
		return nil, fmt.Errorf("failed to link new provenance: %w", err)
	}

	completeUpdate := store.MemoryUpdate{
		Topic:   &topic,
		Context: &context,
		Status:  stringPtr("complete"),
	}
	if err := g.memoryStore.UpdateMemory(ctx, id, completeUpdate); err != nil {
		return nil, fmt.Errorf("failed to mark memory complete: %w", err)
	}

	return result, nil
}

// DeleteMemory removes a memory and runs garbage collection on orphaned artifacts.
func (g *Gognee) DeleteMemory(ctx context.Context, id string) error {
	// Get provenance before delete
	nodeIDs, edgeIDs, err := g.memoryStore.GetProvenanceByMemory(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get provenance: %w", err)
	}

	// Delete memory (CASCADE will remove provenance links)
	if err := g.memoryStore.DeleteMemory(ctx, id); err != nil {
		return err
	}

	// Run GC on candidates
	_, _, err = g.memoryStore.GarbageCollectCandidates(ctx, nodeIDs, edgeIDs)
	if err != nil {
		return fmt.Errorf("garbage collection failed: %w", err)
	}

	return nil
}

// GarbageCollect manually triggers garbage collection.
// Returns counts of deleted nodes and edges.
func (g *Gognee) GarbageCollect(ctx context.Context) (nodesDeleted, edgesDeleted int, err error) {
	// For manual GC, we need to identify all orphaned artifacts
	// This is complex without tracking; for v1.0.0, this is a placeholder
	return 0, 0, fmt.Errorf("manual garbage collection not yet implemented; use DeleteMemory/UpdateMemory for automatic GC")
}

// PinMemory marks a memory as pinned, exempting it from decay and prune (M9: Plan 021).
func (g *Gognee) PinMemory(ctx context.Context, id string, reason string) error {
	// Verify memory exists
	memory, err := g.memoryStore.GetMemory(ctx, id)
	if err != nil {
		return fmt.Errorf("cannot pin memory: %w", err)
	}

	// Already pinned?
	if memory.Pinned {
		return nil // Idempotent
	}

	now := time.Now()
	pinnedStatus := "Pinned"
	updates := store.MemoryUpdate{
		Status: &pinnedStatus,
	}
	if err := g.memoryStore.UpdateMemory(ctx, id, updates); err != nil {
		return fmt.Errorf("failed to update memory status: %w", err)
	}

	// Update pinned fields directly (since MemoryUpdate doesn't have them yet)
	query := `UPDATE memories SET pinned = TRUE, pinned_at = ?, pinned_reason = ? WHERE id = ?`
	reasonPtr := &reason
	_, err = g.memoryStore.DB().ExecContext(ctx, query, now, reasonPtr, id)
	if err != nil {
		return fmt.Errorf("failed to set pinned fields: %w", err)
	}

	return nil
}

// UnpinMemory removes pinning from a memory, allowing normal decay/prune (M9: Plan 021).
func (g *Gognee) UnpinMemory(ctx context.Context, id string) error {
	// Verify memory exists
	memory, err := g.memoryStore.GetMemory(ctx, id)
	if err != nil {
		return fmt.Errorf("cannot unpin memory: %w", err)
	}

	// Not pinned?
	if !memory.Pinned {
		return nil // Idempotent
	}

	activeStatus := "Active"
	updates := store.MemoryUpdate{
		Status: &activeStatus,
	}
	if err := g.memoryStore.UpdateMemory(ctx, id, updates); err != nil {
		return fmt.Errorf("failed to update memory status: %w", err)
	}

	// Update pinned fields directly
	query := `UPDATE memories SET pinned = FALSE, pinned_at = NULL, pinned_reason = NULL WHERE id = ?`
	_, err = g.memoryStore.DB().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to clear pinned fields: %w", err)
	}

	return nil
}

// stringPtr returns a pointer to a string (helper for optional fields).
func stringPtr(s string) *string {
	return &s
}
