//go:build plan009
// +build plan009

package gognee

import (
	"context"
	"testing"

	"github.com/dan-solli/gognee/pkg/extraction"
)

func boolPtr(v bool) *bool { return &v }

func TestPlan009_IncrementalCognify_DefaultSkipsOnSecondRun(t *testing.T) {
	ctx := context.Background()

	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer g.Close()

	// Inject mocks (offline)
	mockLLM := &MockLLMClient{}
	mockEmb := &MockEmbeddingClient{}
	g.llm = mockLLM
	g.embeddings = mockEmb
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)

	if err := g.Add(ctx, "alpha", AddOptions{Source: "s1"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := g.Add(ctx, "beta", AddOptions{Source: "s2"}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	result1, err := g.Cognify(ctx, CognifyOptions{})
	if err != nil {
		t.Fatalf("Cognify(1): %v", err)
	}
	if result1.DocumentsSkipped != 0 {
		t.Fatalf("expected DocumentsSkipped=0, got %d", result1.DocumentsSkipped)
	}
	callsAfter1 := mockLLM.CallCount

	// Re-add same docs
	_ = g.Add(ctx, "alpha", AddOptions{Source: "s1"})
	_ = g.Add(ctx, "beta", AddOptions{Source: "s2"})

	result2, err := g.Cognify(ctx, CognifyOptions{})
	if err != nil {
		t.Fatalf("Cognify(2): %v", err)
	}
	if result2.DocumentsSkipped != 2 {
		t.Fatalf("expected DocumentsSkipped=2, got %d", result2.DocumentsSkipped)
	}
	if mockLLM.CallCount != callsAfter1 {
		t.Fatalf("expected no additional LLM calls for skipped docs")
	}
}

func TestPlan009_IncrementalCognify_ForceOverridesSkip(t *testing.T) {
	ctx := context.Background()

	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer g.Close()

	mockLLM := &MockLLMClient{}
	mockEmb := &MockEmbeddingClient{}
	g.llm = mockLLM
	g.embeddings = mockEmb
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)

	_ = g.Add(ctx, "alpha", AddOptions{})
	_, _ = g.Cognify(ctx, CognifyOptions{})
	callsAfter1 := mockLLM.CallCount

	_ = g.Add(ctx, "alpha", AddOptions{})
	_, err = g.Cognify(ctx, CognifyOptions{SkipProcessed: boolPtr(true), Force: true})
	if err != nil {
		t.Fatalf("Cognify(force): %v", err)
	}
	if mockLLM.CallCount == callsAfter1 {
		t.Fatalf("expected additional LLM calls when Force=true")
	}
}

func TestPlan009_IncrementalCognify_SkipProcessedFalseReprocesses(t *testing.T) {
	ctx := context.Background()

	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer g.Close()

	mockLLM := &MockLLMClient{}
	mockEmb := &MockEmbeddingClient{}
	g.llm = mockLLM
	g.embeddings = mockEmb
	g.entityExtractor = extraction.NewEntityExtractor(mockLLM)
	g.relationExtractor = extraction.NewRelationExtractor(mockLLM)

	_ = g.Add(ctx, "alpha", AddOptions{})
	_, _ = g.Cognify(ctx, CognifyOptions{})
	callsAfter1 := mockLLM.CallCount

	_ = g.Add(ctx, "alpha", AddOptions{})
	_, err = g.Cognify(ctx, CognifyOptions{SkipProcessed: boolPtr(false)})
	if err != nil {
		t.Fatalf("Cognify(SkipProcessed=false): %v", err)
	}
	if mockLLM.CallCount == callsAfter1 {
		t.Fatalf("expected additional LLM calls when SkipProcessed=false")
	}
}
