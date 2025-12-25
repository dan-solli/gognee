# gognee - A Go Knowledge Graph Memory System

> **gognee** is an importable Go library that provides persistent knowledge graph memory for AI assistants. It enables applications to extract, store, and retrieve information relationships using a combination of vector search and graph traversal.

## Features

- 📚 **Knowledge Graph Storage**: Persistent storage of entities and relationships in SQLite
- 🔍 **Hybrid Search**: Combine vector similarity and graph traversal for semantic retrieval
- 🧠 **LLM-Powered Extraction**: Automatic entity and relationship extraction using OpenAI's APIs
- 📝 **Chunking**: Intelligent text splitting with token awareness and overlap handling
- 🔄 **Deterministic Deduplication**: Same entities across documents resolve to the same node
- 💾 **Persistent Memory**: Knowledge persists across application restarts

## Quick Start

### Installation

```bash
go get github.com/dan-solli/gognee
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
