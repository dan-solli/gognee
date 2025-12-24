# gognee - A Go Knowledge Graph Memory System

> **gognee** is an importable Go library that mimics [Cognee](https://github.com/topoteretes/cognee), providing knowledge graph memory for AI assistants. It will be used as the foundation for **Glowbabe** (a Go implementation of Flowbaby).

## 🎯 Project Vision

Build a Go library package that provides AI assistants with:
- **Persistent memory** across conversations
- **Knowledge graph** for understanding relationships between concepts
- **Vector search** for semantic retrieval
- **Hybrid search** combining graph traversal and vector similarity

**Design Goals:**
- Pure library (no CLI) - importable via `import "github.com/dan-solli/gognee/pkg/gognee"`
- Mimic Cognee's API design as closely as reasonable
- No Python. No external dependencies beyond SQLite
- Embeddable in other Go projects (e.g., Glowbabe)

---

## 📋 Roadmap Overview

| Phase | Focus | Duration | Status |
|-------|-------|----------|--------|
| [Phase 1](#phase-1-foundation) | Foundation (Chunking + Embeddings) | 1 week | ✅ Delivered (v0.1.0) |
| [Phase 2](#phase-2-entity-extraction) | Entity Extraction via LLM | 1 week | ✅ Delivered (v0.2.0) |
| [Phase 3](#phase-3-relationship-extraction) | Relationship Extraction | 1 week | ✅ Delivered (v0.3.0) |
| [Phase 4](#phase-4-storage-layer) | Storage Layer (SQLite Graph + Vector) | 1 week | ✅ Delivered (v0.4.0) |
| [Phase 5](#phase-5-search) | Hybrid Search | 1 week | ✅ Delivered (v0.5.0) |
| [Phase 6](#phase-6-integration) | Full Pipeline + API | 1-2 weeks | ⬜ Not Started |

**Total estimated time: 6-8 weeks for MVP**
**Elapsed time: 5 weeks (Phases 1-5 complete)**

---

## Phase 1: Foundation

### Goals
- [ ] Set up Go project structure with modules
- [ ] Implement text chunking (split documents into processable pieces)
- [ ] Implement OpenAI embedding client
- [ ] Write unit tests for chunking

### Deliverables

#### 1.1 Project Structure
```
gognee/
├── pkg/
│   ├── chunker/
│   │   ├── chunker.go        # Text chunking logic
│   │   └── chunker_test.go
│   ├── embeddings/
│   │   ├── client.go         # Embedding client interface
│   │   ├── openai.go         # OpenAI implementation
│   │   └── openai_test.go
│   └── gognee/
│       └── gognee.go         # Main library interface
├── go.mod
├── go.sum
└── README.md
```

**Note:** No `cmd/` directory - gognee is a library package, not an executable.

#### 1.2 Text Chunker
Split text into chunks of ~500 tokens with sentence boundary awareness.

```go
// pkg/chunker/chunker.go
type Chunk struct {
    ID         string
    Text       string
    Index      int
    TokenCount int
}

type Chunker struct {
    MaxTokens int  // Default: 512
    Overlap   int  // Default: 50 tokens overlap between chunks
}

func (c *Chunker) Chunk(text string) []Chunk
```

**Implementation notes:**
- **Chunk IDs:** Generate deterministically using a content hash (e.g., SHA-256 of text) plus index, or use UUIDs if deduplication is not required.
- **Token counting:** Use a simple word-based heuristic (split on whitespace) or adopt a tiktoken port like `github.com/pkoukk/tiktoken-go`. Document the chosen method so callers understand sizing behavior.
```

#### 1.3 Embedding Client
```go
// pkg/embeddings/client.go
type EmbeddingClient interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    EmbedOne(ctx context.Context, text string) ([]float32, error)
}

// pkg/embeddings/openai.go
type OpenAIClient struct {
    APIKey string
    Model  string // "text-embedding-3-small"
}
```

### Testing Strategy
- **Unit tests** must not require network access or an `OPENAI_API_KEY`. Use interface mocks or an in-process fake HTTP server.
- **Integration tests** (optional) that hit the real OpenAI API should be gated behind a build tag (e.g., `//go:build integration`) or environment variable so they don't run by default.

### Learning Outcomes
- Go project structure and modules
- HTTP clients in Go
- JSON marshaling/unmarshaling
- Interface design

---

## Phase 2: Entity Extraction

### Goals
- [ ] Implement LLM client for completions
- [ ] Design entity extraction prompt
- [ ] Parse JSON responses from LLM
- [ ] Handle extraction errors gracefully

### Deliverables

#### 2.1 LLM Client
```go
// pkg/llm/client.go
type LLMClient interface {
    Complete(ctx context.Context, prompt string) (string, error)
    CompleteWithSchema(ctx context.Context, prompt string, schema any) error
}

// pkg/llm/openai.go
type OpenAILLM struct {
    APIKey string
    Model  string // "gpt-4o-mini"
}
```

#### 2.2 Entity Extractor
```go
// pkg/extraction/entities.go
type Entity struct {
    Name        string `json:"name"`
    Type        string `json:"type"`        // Person, Concept, System, Decision, etc.
    Description string `json:"description"`
}

type EntityExtractor struct {
    LLM LLMClient
}

func (e *EntityExtractor) Extract(ctx context.Context, text string) ([]Entity, error)
```

**Error handling:** LLM calls should use exponential backoff with a maximum of 3 retries. If extraction fails after retries, return a clear error (do not silently skip). Callers can choose to proceed without entities if appropriate.

#### 2.3 Entity Extraction Prompt
```
You are a knowledge graph construction assistant.

Extract all meaningful entities from this text. For each entity, provide:
- name: The entity name
- type: One of [Person, Concept, System, Decision, Event, Technology, Pattern]
- description: Brief description (1 sentence)

Text:
---
{text}
---

Return ONLY valid JSON array:
[{"name": "...", "type": "...", "description": "..."}, ...]
```

### Learning Outcomes
- LLM API integration
- Prompt engineering
- JSON schema validation
- Error handling strategies

---

## Phase 3: Relationship Extraction

### Goals
- [x] Design relationship extraction prompt
- [x] Implement triplet extraction (subject, relation, object)
- [x] Link relationships to extracted entities
- [x] Handle cases where entities aren't found

### Deliverables

#### 3.1 Triplet Structure
```go
// pkg/extraction/relations.go
type Triplet struct {
    Subject  string `json:"subject"`
    Relation string `json:"relation"`
    Object   string `json:"object"`
}

type RelationExtractor struct {
    LLM LLMClient
}

func (r *RelationExtractor) Extract(ctx context.Context, text string, entities []Entity) ([]Triplet, error)
```

#### 3.2 Relationship Extraction Prompt
```
You are a knowledge graph construction assistant.

Given this text and the entities already extracted, identify relationships between them.
Express each relationship as a triplet: (subject, relation, object)

Use clear, consistent relation names like:
- USES, DEPENDS_ON, CREATED_BY, CONTAINS, IS_A, RELATES_TO, MENTIONS

Text:
---
{text}
---

Known entities: {entity_names}

Return ONLY valid JSON array:
[{"subject": "...", "relation": "...", "object": "..."}, ...]
```

### Learning Outcomes
- Multi-step LLM pipelines
- Passing context between extraction stages
- Designing consistent relation schemas

---

## Phase 4: Storage Layer

### Goals
- [x] Design SQLite schema for nodes and edges
- [x] Implement graph storage with node/edge CRUD
- [x] Implement in-memory vector store
- [x] Add vector storage with cosine similarity search
- [x] Write integration tests

### Deliverables

#### 4.1 Graph Store
```go
// pkg/store/graph.go
type Node struct {
    ID          string
    Name        string
    Type        string
    Description string
    Embedding   []float32
    CreatedAt   time.Time
    Metadata    map[string]any
}

type Edge struct {
    ID        string
    SourceID  string
    Relation  string
    TargetID  string
    Weight    float64
    CreatedAt time.Time
}

type GraphStore interface {
    // Nodes
    AddNode(ctx context.Context, node *Node) error
    GetNode(ctx context.Context, id string) (*Node, error)
    FindNodeByName(ctx context.Context, name string) (*Node, error)
    
    // Edges
    AddEdge(ctx context.Context, edge *Edge) error
    GetEdges(ctx context.Context, nodeID string) ([]*Edge, error)
    
    // Graph traversal
    GetNeighbors(ctx context.Context, nodeID string, depth int) ([]*Node, error)
}
```

#### 4.2 SQLite Schema
```sql
CREATE TABLE nodes (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT,
    description TEXT,
    embedding BLOB,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    metadata TEXT  -- JSON
);

CREATE TABLE edges (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL,
    relation TEXT NOT NULL,
    target_id TEXT NOT NULL,
    weight REAL DEFAULT 1.0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (source_id) REFERENCES nodes(id),
    FOREIGN KEY (target_id) REFERENCES nodes(id)
);

CREATE INDEX idx_edges_source ON edges(source_id);
CREATE INDEX idx_edges_target ON edges(target_id);
CREATE INDEX idx_nodes_name ON nodes(name);
```

#### 4.3 Vector Store
```go
// pkg/store/vector.go
type VectorStore interface {
    Add(ctx context.Context, id string, embedding []float32) error
    Search(ctx context.Context, query []float32, topK int) ([]SearchResult, error)
    Delete(ctx context.Context, id string) error
}

type SearchResult struct {
    ID    string
    Score float64  // Cosine similarity
}

// In-memory implementation for MVP
type MemoryVectorStore struct {
    vectors map[string][]float32
    mu      sync.RWMutex
}

func CosineSimilarity(a, b []float32) float64
```

**MVP limitation:** The in-memory vector store does not persist embeddings across restarts. Full "persistent memory" requires either re-running `Cognify()` after restart or implementing a SQLite-backed vector store (see Future Enhancements).
```

### Learning Outcomes
- SQL schema design
- Graph data structures
- Vector mathematics
- Concurrent data structures in Go

---

## Phase 5: Search

### Goals
- [x] Implement vector-only search
- [x] Implement graph traversal search
- [x] Implement hybrid search combining both
- [x] Add result ranking and scoring

### Deliverables

#### 5.1 Search Interface
```go
// pkg/search/search.go
type SearchType string

const (
    SearchTypeVector SearchType = "vector"
    SearchTypeGraph  SearchType = "graph"
    SearchTypeHybrid SearchType = "hybrid"
)

type SearchResult struct {
    NodeID      string
    Node        *Node
    Score       float64
    Source      string  // "vector" or "graph"
    GraphDepth  int     // For graph results
}

type Searcher interface {
    Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)
}

type SearchOptions struct {
    Type       SearchType
    TopK       int
    GraphDepth int
}
```

#### 5.2 Hybrid Search Algorithm
```go
// pkg/search/hybrid.go
type HybridSearcher struct {
    Embeddings  EmbeddingClient
    VectorStore VectorStore
    GraphStore  GraphStore
}

func (h *HybridSearcher) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
    // 1. Embed the query
    // 2. Vector search for top-K similar nodes
    // 3. For each vector result, expand via graph neighbors
    // 4. Deduplicate and merge scores
    // 5. Sort by combined score
    // 6. Return top results
}
```

### Learning Outcomes
- Search algorithm design
- Score normalization and ranking
- Combining multiple signals

---

## Phase 6: Integration

### Goals
- [ ] Create unified `Gognee` API (mirroring Cognee)
- [ ] Implement `Add()`, `Cognify()`, `Search()` methods
- [ ] Add configuration options
- [ ] Write end-to-end tests
- [ ] Add documentation and usage examples
- [ ] Ensure API compatibility for Glowbabe integration

### Deliverables

#### 6.1 Main API
```go
// pkg/gognee/gognee.go
type Gognee struct {
    config      Config
    chunker     *Chunker
    embeddings  EmbeddingClient
    llm         LLMClient
    extractor   *Extractor
    graphStore  GraphStore
    vectorStore VectorStore
    searcher    Searcher
}

type Config struct {
    DBPath        string
    OpenAIKey     string
    EmbeddingModel string
    LLMModel      string
    ChunkSize     int
    ChunkOverlap  int
}

func New(cfg Config) (*Gognee, error)

// Core API (mirrors Cognee)
func (g *Gognee) Add(ctx context.Context, text string, opts AddOptions) error
func (g *Gognee) Cognify(ctx context.Context, opts CognifyOptions) error
func (g *Gognee) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)

// Additional utilities
func (g *Gognee) Close() error
func (g *Gognee) Stats() Stats
```

#### 6.2 Example Usage (Library Import)
```go
package main

import (
    "context"
    "fmt"
    "github.com/dan-solli/gognee/pkg/gognee"
)

func main() {
    ctx := context.Background()
    
    // Initialize
    g, err := gognee.New(gognee.Config{
        DBPath:    "./memory.db",
        OpenAIKey: os.Getenv("OPENAI_API_KEY"),
    })
    if err != nil {
        log.Fatal(err)
    }
    defer g.Close()
    
    // Add some knowledge
    g.Add(ctx, "The project uses React with TypeScript for the frontend.")
    g.Add(ctx, "We decided to use PostgreSQL for the database.")
    g.Add(ctx, "The API follows RESTful conventions.")
    
    // Build the knowledge graph
    g.Cognify(ctx)
    
    // Search
    results, _ := g.Search(ctx, "What technology stack does the project use?")
    for _, r := range results {
        fmt.Printf("- %s (score: %.2f)\n", r.Node.Name, r.Score)
    }
}
```

### Learning Outcomes
- Library API design (mirroring Cognee)
- Configuration management
- End-to-end testing
- Package structure for importable libraries

---

## 🔧 Technical Decisions

### Dependencies (Minimal)

| Dependency | Purpose | Why |
|------------|---------|-----|
| `modernc.org/sqlite` | SQLite driver | Pure Go, no CGO, easy cross-compilation |
| `github.com/google/uuid` | UUID generation | Standard, reliable |
| `github.com/spf13/cobra` | CLI framework | Industry standard for Go CLIs |

### Why Not Use...

| Technology | Reason |
|------------|--------|
| **External vector DB** | Adds deployment complexity. In-memory works for personal use. |
| **Neo4j/other graph DB** | SQLite with edges table is sufficient for our scale. |
| **LangChain** | Overengineered for our needs. Direct API calls are clearer. |

### CGO Policy

**CGO is allowed** in this project. A pure-Go SQLite driver (e.g., `modernc.org/sqlite`) is recommended for easier cross-compilation, but CGO-based drivers (e.g., `mattn/go-sqlite3`) are acceptable if needed for performance or compatibility.

---

## 📊 Success Metrics

### MVP (Phase 6 Complete)
- [ ] Can add text and build knowledge graph
- [ ] Can search and retrieve relevant context
- [ ] Single binary, no external dependencies
- [ ] Works on macOS, Linux, Windows
- [ ] < 5MB binary size
- [ ] < 100ms search latency for small graphs

### Future Enhancements (Post-MVP)
- [ ] Multiple LLM provider support (Anthropic, Ollama)
- [ ] Persistent vector store (not just in-memory)
- [ ] Graph visualization
- [ ] Incremental cognify (only process new text)
- [ ] Memory decay/forgetting
- [ ] Session/context awareness

---

## 🚀 Getting Started

### Prerequisites
- Go 1.21+
- OpenAI API key

### Quick Start (After Phase 6)
```bash
# Install
go install github.com/dan-solli/gognee/cmd/gognee@latest

# Set API key
export OPENAI_API_KEY=sk-...

# Use
gognee add "Some text to remember"
gognee cognify
gognee search "What do I know about..."
```

---

## 📚 Resources

### Go Learning
- [Effective Go](https://golang.org/doc/effective_go)
- [Go by Example](https://gobyexample.com/)
- [Go Database/SQL Tutorial](http://go-database-sql.org/)

### Knowledge Graphs
- [Knowledge Graphs: Fundamentals, Techniques, and Applications](https://kgbook.org/)
- [Building Knowledge Graphs](https://www.oreilly.com/library/view/building-knowledge-graphs/9781098127091/)

### Vector Search
- [Understanding Vector Similarity Search](https://www.pinecone.io/learn/what-is-similarity-search/)
- [Cosine Similarity Explained](https://www.machinelearningplus.com/nlp/cosine-similarity/)

---

## 🤝 Contributing

This is a teaching project. The goal is to learn, not to build the most performant system.

1. Start simple, iterate
2. Write tests as you go
3. Document your learnings
4. Ask questions

---

## 📄 License

MIT License - Use it, learn from it, build on it.
