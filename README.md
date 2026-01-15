**This project is 100% AI-slop. Use at own risk for life and limb.**

# gognee - A Go Knowledge Graph Memory System

> **gognee** is an importable Go library that provides persistent knowledge graph memory for AI assistants. It enables applications to extract, store, and retrieve information relationships using a combination of vector search and graph traversal.

## Features

- 📚 **Knowledge Graph Storage**: Persistent storage of entities and relationships in SQLite
- 🔍 **Hybrid Search**: Combine vector similarity and graph traversal for semantic retrieval
- 🧠 **LLM-Powered Extraction**: Automatic entity and relationship extraction using OpenAI's APIs
- 📝 **Chunking**: Intelligent text splitting with token awareness and overlap handling
- 🔄 **Deterministic Deduplication**: Same entities across documents resolve to the same node
- 💾 **Persistent Memory**: Knowledge persists across application restarts

## Importing

gognee is a library package intended to be imported into your Go application. Use the package entrypoint at

```go
import "github.com/dan-solli/gognee/pkg/gognee"
```

Minimal import-and-use example:

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/dan-solli/gognee/pkg/gognee"
)

func main() {
	ctx := context.Background()

	g, err := gognee.New(gognee.Config{
		DBPath:    "./memory.db",
		OpenAIKey: os.Getenv("OPENAI_API_KEY"),
	})
	if err != nil {
		panic(err)
	}
	defer g.Close()

	// Add text and process
	_ = g.Add(ctx, "Gognee is a Go knowledge graph memory library.", gognee.AddOptions{})
	_, _ = g.Cognify(ctx, gognee.CognifyOptions{})

	// Search
	results, _ := g.Search(ctx, "What do I know about gognee?", gognee.SearchOptions{})
	fmt.Printf("Found %d results\n", len(results))
}
```

Types and convenience values are re-exported from the package (for example `SearchOptions`, `SearchResult`, `SearchTypeHybrid`, `Node`).

## Quick Start

### Prerequisites

**CGO Requirement**: gognee v1.2.0+ requires CGO for sqlite-vec vector indexing:

```bash
export CGO_ENABLED=1
```

**Platform-specific notes**:
- **Linux**: Requires GCC or Clang
- **macOS**: Requires Xcode Command Line Tools (`xcode-select --install`)
- **Windows**: Requires MinGW-w64 or MSVC

### Installation

```bash
CGO_ENABLED=1 go get github.com/dan-solli/gognee
```

### Basic Usage

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/dan-solli/gognee/pkg/gognee"
)

func main() {
	ctx := context.Background()

	// Initialize Gognee with OpenAI API key
	g, err := gognee.New(gognee.Config{
		OpenAIKey: os.Getenv("OPENAI_API_KEY"),
		DBPath:    "./memory.db", // Persistent SQLite database
	})
	if err != nil {
		log.Fatal(err)
	}
	defer g.Close()

	// Add documents to the knowledge base
	documents := []string{
		"React is a JavaScript library for building user interfaces using components.",
		"We use TypeScript to add static typing to our React applications.",
		"PostgreSQL is our primary database for storing application data.",
		"The frontend uses React with TypeScript, and the backend uses PostgreSQL.",
	}

	for _, doc := range documents {
		if err := g.Add(ctx, doc, gognee.AddOptions{}); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Printf("Buffered %d documents for processing\n", g.BufferedCount())

	// Process buffered documents through the extraction pipeline
	result, err := g.Cognify(ctx, gognee.CognifyOptions{})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Processed %d documents: %d nodes, %d edges\n",
		result.DocumentsProcessed,
		result.NodesCreated,
		result.EdgesCreated,
	)

	// Query the knowledge graph
	results, err := g.Search(ctx, "What technologies does the project use?", gognee.SearchOptions{
		Type:       gognee.SearchTypeHybrid,
		TopK:       5,
		GraphDepth: 1,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nSearch Results:")
	for i, result := range results {
		fmt.Printf("%d. %s (%s) - Score: %.4f\n", i+1, result.Node.Name, result.Node.Type, result.Score)
	}

	// Check statistics
	stats, err := g.Stats()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nKnowledge Graph Stats: %d nodes, %d edges\n", stats.NodeCount, stats.EdgeCount)
}
```

## API Reference

### Core Methods

#### New(cfg Config) (*Gognee, error)

Initializes a new Gognee instance.

**Config fields:**
- `OpenAIKey` (required): OpenAI API key for embeddings and LLM
- `DBPath` (optional): Path to SQLite database file. Defaults to `:memory:` if empty
- `EmbeddingModel` (optional): Embedding model to use. Default: `text-embedding-3-small`
- `LLMModel` (optional): LLM model for extraction. Default: `gpt-4o-mini`
- `ChunkSize` (optional): Token size for text chunks. Default: `512`
- `ChunkOverlap` (optional): Token overlap between chunks. Default: `50`

#### Add(ctx context.Context, text string, opts AddOptions) error

Buffers text for later processing.

- **Parameters:**
  - `text`: Document text to add (non-empty)
  - `opts.Source` (optional): Source identifier for the document
- **Returns:** Error if text is empty
- **Note:** Text is buffered but NOT processed until `Cognify()` is called

#### Cognify(ctx context.Context, opts CognifyOptions) (*CognifyResult, error)

Processes all buffered documents through the full extraction pipeline:

1. Chunks text into segments
2. Extracts entities (Person, Concept, System, etc.) via LLM
3. Extracts relationships between entities via LLM
4. Creates nodes and edges in the knowledge graph
5. Generates embeddings for semantic search
6. Clears the buffer

**CognifyResult fields:**
- `DocumentsProcessed`: Count of documents in the buffer
- `ChunksProcessed`: Total chunks created
- `ChunksFailed`: Chunks that failed extraction
- `NodesCreated`: Entities added to graph
- `EdgesCreated`: Relationships added to graph
- `Errors`: Individual errors encountered (processing continues best-effort)

**Note:** The buffer is always cleared after Cognify, even if errors occur. Return error is only for catastrophic failures (context canceled, DB connection lost).

#### Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)

Searches the knowledge graph.

**SearchOptions fields:**
- `Type` (optional): Search type - `SearchTypeVector`, `SearchTypeGraph`, or `SearchTypeHybrid`. Default: `SearchTypeHybrid`
- `TopK` (optional): Maximum results to return. Default: `10`
- `GraphDepth` (optional): Max depth for graph traversal. Default: `1`
- `SeedNodeIDs` (optional): Starting nodes for graph search

**SearchResult fields:**
- `Node`: Full node data
- `Score`: Relevance score (0-1)
- `Source`: How the node was found ("vector" or "graph")
- `GraphDepth`: Distance from search origin

#### Close() error

Releases all resources (database connections, buffered data).

#### Stats() (Stats, error)

Returns knowledge graph statistics:

- `NodeCount`: Total entities in graph
- `EdgeCount`: Total relationships in graph
- `BufferedDocs`: Documents waiting for Cognify
- `LastCognified`: Timestamp of last successful Cognify

### Advanced Access

For custom pipelines, these components are accessible:

- `GetChunker()`: Text chunking
- `GetEmbeddings()`: Embedding client
- `GetLLM()`: LLM client
- `GetGraphStore()`: Graph storage
- `GetVectorStore()`: Vector storage

## Type Re-exports

Common types are re-exported from the top-level package for convenience:

- `SearchResult`, `SearchOptions`, `SearchType`
- `Node`, `Edge`
- `SearchTypeVector`, `SearchTypeGraph`, `SearchTypeHybrid` (constants)

### Default Behavior

gognee uses SQLite for both graph storage and vector embeddings. Choose the storage mode with `DBPath`:

- **Persistent Storage** (recommended): `DBPath: "./memory.db"` - Data persists across restarts
- **In-Memory Storage** (testing/dev): `DBPath: ":memory:"` - Data is cleared when process exits

### Persistence

When using a file-based `DBPath`, both the knowledge graph (nodes and edges) and vector embeddings persist across application restarts. This means:

✅ No need to re-run `Cognify()` after restart
✅ Instant search availability on startup
✅ Zero-downtime deployment support

Example workflow:

```go
// Session 1: Build knowledge graph
g1, _ := gognee.New(gognee.Config{
    DBPath: "./memory.db",
    OpenAIKey: apiKey,
})
g1.Add(ctx, "Document 1...", gognee.AddOptions{})
g1.Cognify(ctx, gognee.CognifyOptions{})
g1.Close()

// Session 2: Reopen and immediately search (no Cognify needed)
g2, _ := gognee.New(gognee.Config{
    DBPath: "./memory.db",  // Same database
    OpenAIKey: apiKey,
})
results, _ := g2.Search(ctx, "query", gognee.SearchOptions{})
// ✅ Results immediately available - embeddings were persisted
```

### Migration from v0.6.0 and Earlier

In v0.6.0 and earlier, vector embeddings were stored in memory and lost on restart. If you're upgrading:

- **Existing databases** will work without migration - simply run `Cognify()` once after upgrading to v0.7.0 to populate the persistent embeddings
- **New databases** get persistent embeddings automatically
- **In-memory mode** (`:memory:`) behavior is unchanged

## MVP Limitations

This is the MVP (Minimum Viable Product). Known limitations:

### Performance

- **Vector search**: Optimized with sqlite-vec indexed ANN search (O(log n) complexity)
- **Graph traversal**: In-memory BFS implementation (acceptable for <100K nodes)
- **Concurrent writes**: Serializable transactions may cause contention under heavy load

### Persistent Storage

Provide a file path to `DBPath` to enable persistent storage:

```go
cfg := gognee.Config{
    OpenAIKey: "sk-...",
    DBPath: "./knowledge.db",
}
g, _ := gognee.New(cfg)
```

The database file is created automatically if it doesn't exist.

## Incremental Cognify

By default, gognee tracks processed documents to avoid redundant processing. This reduces costs and processing time when re-adding documents.

### How It Works

When you call `Cognify()`, gognee:
1. Computes a SHA-256 hash of each document's text
2. Checks if that hash has been processed before
3. **Skips** documents that have already been processed (incremental mode)
4. Processes new/changed documents normally

**Benefits:**
- ⚡ Near-instant processing for duplicate documents (~0ms vs 5-10s)
- 💰 Zero LLM API costs for cached documents
- 🔄 Enables continuous updates without full reprocessing

### Default Behavior

Incremental mode is **ON by default**:

```go
// Second Cognify() call skips already-processed documents
g.Add(ctx, "React is a UI library", gognee.AddOptions{})
g.Cognify(ctx, gognee.CognifyOptions{}) // Processes document (hash: abc123)

g.Add(ctx, "React is a UI library", gognee.AddOptions{}) // Same text
g.Cognify(ctx, gognee.CognifyOptions{}) // Skips (hash: abc123 already processed)
// DocumentsProcessed=0, DocumentsSkipped=1
```

### Controlling Incremental Behavior

Use `CognifyOptions` to control incremental processing:

```go
// Disable incremental mode (always reprocess)
skipProcessed := false
g.Cognify(ctx, gognee.CognifyOptions{
    SkipProcessed: &skipProcessed,
})

// Force reprocessing even with incremental mode enabled
g.Cognify(ctx, gognee.CognifyOptions{
    Force: true, // Overrides SkipProcessed
})
```

**When to use `Force: true`:**
- After changing `ChunkSize` or `ChunkOverlap` settings
- To rebuild the knowledge graph from scratch
- After updating extraction prompts or LLM models

### Document Identity

Documents are identified by **exact text content** (SHA-256 hash). Any change to the text (including whitespace) creates a new document:

```go
g.Add(ctx, "React is great", gognee.AddOptions{Source: "file-a"})
g.Cognify(ctx, gognee.CognifyOptions{}) // Processes

g.Add(ctx, "React is great", gognee.AddOptions{Source: "file-b"}) // Different source
g.Cognify(ctx, gognee.CognifyOptions{}) // Skips (same text)

g.Add(ctx, "React is great!", gognee.AddOptions{}) // Punctuation changed
g.Cognify(ctx, gognee.CognifyOptions{}) // Processes (different hash)
```

**Note:** The `Source` field is metadata only and does NOT affect document identity. Identity is purely content-based.

### Persistence

Document tracking persists in the SQLite database (table: `processed_documents`). Tracking survives application restarts when using file-based `DBPath`.

For `:memory:` mode, tracking is lost on restart (incremental behavior only applies within a single session).

### Resetting Tracking

To clear processed document history without deleting the knowledge graph:

```go
// Access DocumentTracker interface
tracker := g.GetGraphStore().(store.DocumentTracker)
tracker.ClearProcessedDocuments(ctx)

// Now all documents will be reprocessed
g.Cognify(ctx, gognee.CognifyOptions{})
```

## Memory Decay and Forgetting

gognee supports time-based memory decay to keep the knowledge graph relevant and bounded. Older or rarely-accessed nodes receive lower scores in search results, and can be explicitly pruned.

### Configuration

Enable decay by setting decay-related fields in `Config`:

```go
cfg := gognee.Config{
    OpenAIKey:         "sk-...",
    DBPath:            "./knowledge.db",
    DecayEnabled:      true,          // Enable time-based decay (default: false)
    DecayHalfLifeDays: 30,            // Nodes' scores halve after 30 days (default: 30)
    DecayBasis:        "access",      // Decay based on last access ("access" or "creation", default: "access")
}
```

**Decay Options:**
- **DecayEnabled**: When `true`, search results are scored with decay multipliers. Off by default for backward compatibility
- **DecayHalfLifeDays**: Number of days after which a node's score is multiplied by 0.5. Shorter values mean faster decay
- **DecayBasis**: 
  - `"access"`: Decay based on `last_accessed_at` timestamp (nodes accessed recently resist decay)
  - `"creation"`: Decay based on `created_at` timestamp (age since creation)
  - If a node has never been accessed, falls back to `created_at`

### Access Reinforcement

When decay is enabled, nodes returned in search results have their `last_accessed_at` timestamp updated automatically. This means frequently searched nodes resist decay (mimicking human memory reinforcement).

### Pruning Nodes

Use `Prune()` to permanently delete nodes that are too old or have decayed below a threshold:

```go
// Preview what would be pruned (dry run)
result, err := g.Prune(ctx, gognee.PruneOptions{
    MaxAgeDays:    60,      // Prune nodes older than 60 days
    MinDecayScore: 0.1,     // Prune nodes with decay score < 0.1
    DryRun:        true,    // Don't actually delete
})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Would prune %d nodes and %d edges\n", result.NodesPruned, result.EdgesPruned)

// Actually prune
result, err = g.Prune(ctx, gognee.PruneOptions{
    MaxAgeDays: 60,
    DryRun:     false,
})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Pruned %d nodes\n", result.NodesPruned)
```

**PruneOptions:**
- **MaxAgeDays**: Remove nodes older than this many days (based on `DecayBasis`). If 0, this criterion is not used
- **MinDecayScore**: Remove nodes with decay score below this value. If 0, this criterion is not used. Requires `DecayEnabled=true`
- **DryRun**: If `true`, reports what would be pruned without actually deleting

**PruneResult:**
- **NodesEvaluated**: Total number of nodes checked
- **NodesPruned**: Number of nodes deleted
- **EdgesPruned**: Number of edges deleted (cascade deletion when endpoints are removed)
- **NodeIDs**: List of pruned node IDs (for verification)

**Important:** Pruning is permanent. Use `DryRun=true` first to preview the impact.

### Decay Math

Decay uses an exponential formula:

```
score_multiplier = 0.5 ^ (age_days / half_life_days)
```

Examples with 30-day half-life:
- 0 days old: multiplier = 1.0 (no decay)
- 30 days old: multiplier = 0.5 (half score)
- 60 days old: multiplier = 0.25 (quarter score)
- 90 days old: multiplier = 0.125

### Best Practices

1. **Start with decay disabled** to build your knowledge graph, then enable it once populated
2. **Use access-based decay** (`DecayBasis="access"`) to preserve frequently queried nodes
3. **Run dry-run prunes** periodically to understand decay behavior before committing
4. **Adjust half-life** based on your domain:
   - News/events: 7-14 days
   - Product documentation: 90-180 days
   - Reference knowledge: 365+ days

## Memory Management (v1.0.0+)

gognee v1.0.0 introduces **first-class memory CRUD** - a higher-level abstraction for managing discrete units of knowledge with full lifecycle management and provenance tracking.

### Overview

Instead of using `Add()` + `Cognify()` to process raw text, you can now use **Memory APIs** for structured knowledge management:

- **AddMemory**: Create a memory with topic, context, decisions, rationale
- **GetMemory**: Retrieve a specific memory by ID
- **ListMemories**: List all memories with pagination
- **UpdateMemory**: Modify an existing memory (re-cognifies automatically)
- **DeleteMemory**: Remove a memory and run garbage collection
- **Search**: Now includes `MemoryIDs` field showing which memories contributed to each result

**Key Benefits:**
- 🔖 **Structured Storage**: Memories have explicit fields (topic, context, decisions, rationale)
- 🔗 **Provenance Tracking**: Know which knowledge artifacts came from which memory
- ♻️ **Garbage Collection**: Deleting/updating a memory cleans up orphaned nodes/edges automatically
- 🎯 **Deduplication**: Identical memories (same content) are not re-processed
- 🔄 **Re-Cognify on Update**: Updating a memory automatically re-extracts entities and relationships

### API Overview

#### MemoryInput

```go
type MemoryInput struct {
    Topic     string                 // Required: 3-7 word title
    Context   string                 // Required: 300-1500 char summary
    Decisions []string               // Optional: list of decisions made
    Rationale []string               // Optional: explanations for decisions
    Metadata  map[string]interface{} // Optional: arbitrary metadata
    Source    string                 // Optional: source identifier
}
```

#### MemoryResult

```go
type MemoryResult struct {
    Memory       store.MemoryRecord   // Full memory record
    NodeIDs      []string             // IDs of extracted nodes
    EdgeIDs      []string             // IDs of extracted edges
    NodesCreated int                  // Count of new nodes
    EdgesCreated int                  // Count of new edges
}
```

### Adding a Memory

```go
memory := gognee.MemoryInput{
    Topic:   "Phase 4 Storage Layer Implementation",
    Context: "Implemented SQLite-backed graph store with nodes, edges, and vector storage. Added provenance tracking for memory CRUD operations. Foreign keys enabled for CASCADE deletes.",
    Decisions: []string{
        "Use SQLite for both graph and vector storage",
        "Enable PRAGMA foreign_keys=ON for automatic cascade deletes",
        "Implement two-phase transaction model for memory updates",
    },
    Rationale: []string{
        "SQLite provides ACID guarantees and simplifies deployment",
        "Foreign keys ensure provenance integrity without manual cleanup",
        "Two-phase model prevents long transactions during LLM calls",
    },
    Source: "implementation-doc-004",
}

result, err := g.AddMemory(ctx, memory)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Created memory %s: %d nodes, %d edges\n",
    result.Memory.ID,
    result.NodesCreated,
    result.EdgesCreated,
)
```

**Deduplication:** If a memory with identical content already exists, `AddMemory` returns the existing memory without reprocessing.

### Retrieving Memories

```go
// Get a specific memory by ID
memory, err := g.GetMemory(ctx, memoryID)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Memory: %s\n", memory.Topic)
fmt.Printf("Decisions: %d\n", len(memory.Decisions))

// List all memories with pagination
memories, err := g.ListMemories(ctx, gognee.ListMemoriesOptions{
    Limit:  10,
    Offset: 0,
})
if err != nil {
    log.Fatal(err)
}

for _, summary := range memories {
    fmt.Printf("- %s (%s)\n", summary.Topic, summary.ID)
}
```

### Updating a Memory

Updating a memory triggers automatic re-cognify:

1. Unlinks old provenance (nodes/edges from previous version)
2. Runs garbage collection on orphaned artifacts
3. Re-cognifies with new content
4. Links new provenance

```go
updates := gognee.MemoryUpdate{
    Context: stringPtr("Updated context with new findings..."),
    Decisions: &[]string{
        "Decision 1 (updated)",
        "Decision 2 (new)",
    },
}

result, err := g.UpdateMemory(ctx, memoryID, updates)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Updated: %d old nodes, %d new nodes\n",
    len(result.OldNodeIDs),
    result.NodesCreated,
)
```

**Important:** Only provide fields you want to update. Omitted fields are preserved from the original memory.

### Deleting a Memory

Deleting a memory removes it and runs garbage collection:

```go
err := g.DeleteMemory(ctx, memoryID)
if err != nil {
    log.Fatal(err)
}
```

**Garbage Collection Behavior:**
- Deletes nodes/edges that **only** belong to this memory
- Preserves shared nodes/edges (used by other memories or legacy Add/Cognify)
- Legacy data (from Add/Cognify) is **never** deleted by GC

### Search Integration

Search results now include `MemoryIDs` showing which memories contributed to each node:

```go
results, err := g.Search(ctx, "storage implementation", gognee.SearchOptions{
    TopK:              5,
    IncludeMemoryIDs:  boolPtr(true), // Default: true
})
if err != nil {
    log.Fatal(err)
}

for _, result := range results {
    fmt.Printf("Node: %s\n", result.Node.Name)
    fmt.Printf("From memories: %v\n", result.MemoryIDs)
}
```

**Memory IDs** are sorted by `updated_at DESC`, showing the most recent memory first.

### Migration from Legacy Add/Cognify

The legacy `Add()` + `Cognify()` workflow continues to work:

```go
// Legacy workflow (still supported)
g.Add(ctx, "Some text...", gognee.AddOptions{})
g.Cognify(ctx, gognee.CognifyOptions{})
```

**Differences:**

| Feature | Legacy Add/Cognify | Memory CRUD |
|---------|-------------------|-------------|
| **Structured Storage** | No (raw text only) | Yes (topic, decisions, rationale) |
| **Provenance Tracking** | No | Yes |
| **Garbage Collection** | No | Yes |
| **Update Support** | No (must delete + re-add) | Yes (UpdateMemory re-cognifies) |
| **Deduplication** | Document-level (doc_hash) | Content-level (doc_hash) |
| **Search Integration** | Results only | Results + MemoryIDs |

**When to use each:**

- **Memory CRUD**: Structured knowledge management, agent memory, decision logs, planning artifacts
- **Legacy Add/Cognify**: Bulk document ingestion, unstructured text processing

**Interoperability:** Both systems share the same graph store. Nodes/edges created by legacy Add/Cognify are visible in Search, and vice versa. However, legacy artifacts are not tracked by provenance and won't be affected by garbage collection.

### Two-Phase Transaction Model

Memory operations use a two-phase model to avoid long transactions during LLM calls:

**Phase 1 (Transaction):**
- Persist memory record with `status="pending"`
- Compute doc_hash for deduplication

**Phase 2 (No Transaction):**
- Call LLM APIs for entity/relationship extraction
- Generate embeddings

**Phase 3 (Transaction):**
- Update graph with nodes/edges
- Link provenance
- Set `status="complete"`

This design:
- ✅ Prevents database locks during slow LLM calls
- ✅ Allows crash recovery (pending memories can be retried)
- ✅ Maintains transactional integrity for metadata

### Garbage Collection Details

Garbage collection uses **reference counting** via the `memory_nodes` and `memory_edges` junction tables:

```sql
-- Check if a node is orphaned
SELECT COUNT(*) FROM memory_nodes WHERE node_id = ?
-- If count = 0, node is safe to delete
```

**Preserved Artifacts:**
- Nodes/edges with `COUNT(*) > 0` (shared across memories)
- Nodes/edges without any provenance records (legacy data)

**Deleted Artifacts:**
- Nodes/edges with `COUNT(*) = 0` after unlinking

**Foreign Key Cascade:** Deleting a memory automatically deletes its provenance records via `ON DELETE CASCADE`.

### Helper Functions

```go
// Helper for optional string fields
func stringPtr(s string) *string {
    return &s
}

// Helper for optional bool fields
func boolPtr(b bool) *bool {
    return &b
}
```

### Example: Agent Memory Loop

```go
// Agent stores a decision
memory, _ := g.AddMemory(ctx, gognee.MemoryInput{
    Topic:   "API Design Decision",
    Context: "Chose REST over GraphQL for simplicity...",
    Decisions: []string{"Use REST API"},
    Rationale: []string{"Team familiarity", "Lower complexity"},
})

// Later: Search recalls the decision
results, _ := g.Search(ctx, "API design approach", gognee.SearchOptions{})
for _, r := range results {
    fmt.Printf("Found in memories: %v\n", r.MemoryIDs)
    // Retrieve full memory for context
    mem, _ := g.GetMemory(ctx, r.MemoryIDs[0])
    fmt.Printf("Decision: %s\n", mem.Decisions[0])
}

// Update the decision with new findings
g.UpdateMemory(ctx, memory.Memory.ID, gognee.MemoryUpdate{
    Rationale: &[]string{
        "Team familiarity",
        "Lower complexity",
        "Better caching support (discovered)",
    },
})
```

## MVP Limitations

This is the MVP (Minimum Viable Product). Known limitations:

1. **Linear Vector Search**: Vector search uses a direct-query linear scan. Acceptable for <10K nodes; larger graphs may need ANN indexing
2. **No Parallelization**: Document processing is sequential. Large batches may take time
3. **Single LLM Provider**: Only OpenAI is supported
4. **Basic Chunking**: Token-based chunking without semantic awareness

### Future Enhancements

- ANN indexing for vector search (e.g., HNSW)
- Multiple LLM providers (Anthropic, Ollama, local models)
- Parallel processing of documents
- Graph visualization
- Incremental cognify (process only new documents)

## Error Handling

gognee uses a best-effort model for batch processing:

- **Per-Chunk Errors**: If a chunk fails extraction, that chunk is skipped; processing continues with remaining chunks
- **Buffer Clearing**: The buffer is always cleared after `Cognify()`, regardless of errors. Inspect `CognifyResult.Errors` to see what failed
- **Return Error**: Only returned for catastrophic failures (DB connection lost, context canceled)

```go
result, err := g.Cognify(ctx, gognee.CognifyOptions{})

if err != nil {
    log.Fatal(err) // Catastrophic error
}

if len(result.Errors) > 0 {
    log.Printf("Processing had %d errors:\n", len(result.Errors))
    for _, perr := range result.Errors {
        log.Printf("  - %v\n", perr)
    }
}
```

## Integration Testing

Integration tests with real OpenAI API are gated behind a build tag:

```bash
# Run only unit tests (no API calls)
go test ./...

# Run unit + integration tests (requires OPENAI_API_KEY)
OPENAI_API_KEY=sk-... go test -tags=integration ./...
```

## Testing

The library includes:

- **Unit Tests**: Fast, offline tests with mocked dependencies (~80% coverage)
- **Integration Tests**: End-to-end tests with real OpenAI API (gated)
- **SQLite Tests**: Storage layer tests including concurrent access

Run tests:

```bash
go test ./... -v
```

## Development

This library follows:

- **TDD**: Tests are written before implementation
- **Interfaces**: Core components (LLM, Embeddings, Storage) use interfaces for easy mocking and extension
- **Minimal Dependencies**: Only SQLite beyond the standard library

## License

MIT License - See LICENSE file for details

## Acknowledgments

Inspired by [Cognee](https://github.com/topoteretes/cognee) - a Python knowledge graph library.
