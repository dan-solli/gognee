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

## Storage

### Default Behavior

If `DBPath` is empty or `:memory:`, gognee uses SQLite in-memory storage. This is useful for testing but data is lost when the process exits.

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

## MVP Limitations

This is the MVP (Minimum Viable Product). Known limitations:

1. **In-Memory Vector Index**: Embeddings are not persisted. To preserve embeddings across restarts, run `Cognify()` again on startup (with cached documents)
2. **No Parallelization**: Document processing is sequential. Large batches may take time
3. **Single LLM Provider**: Only OpenAI is supported
4. **Basic Chunking**: Token-based chunking without semantic awareness
5. **No Memory Decay**: All entities and relationships are equally important forever

### Future Enhancements

- Persistent vector store (SQLite-backed)
- Multiple LLM providers (Anthropic, Ollama, local models)
- Parallel processing of documents
- Memory decay/forgetting
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
