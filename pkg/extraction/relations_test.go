package extraction

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Test helper: generate valid JSON triplet response
func tripletsJSON(triplets []Triplet) string {
	data, _ := json.Marshal(triplets)
	return string(data)
}

func TestRelationExtractorExtract_Success(t *testing.T) {
	entities := []Entity{
		{Name: "Alice", Type: "Person", Description: "A software engineer"},
		{Name: "Go", Type: "Technology", Description: "A programming language"},
		{Name: "Microservices", Type: "Concept", Description: "An architectural pattern"},
	}

	triplets := []Triplet{
		{Subject: "Alice", Relation: "USES", Object: "Go"},
		{Subject: "Alice", Relation: "BUILDS", Object: "Microservices"},
	}

	fakeLLM := &fakeLLMClient{response: tripletsJSON(triplets)}
	extractor := NewRelationExtractor(fakeLLM)

	result, err := extractor.Extract(context.Background(), "Alice uses Go to build microservices", entities)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("Expected 2 triplets, got %d", len(result))
	}

	if result[0].Subject != "Alice" || result[0].Relation != "USES" || result[0].Object != "Go" {
		t.Errorf("Unexpected first triplet: %+v", result[0])
	}

	if result[1].Subject != "Alice" || result[1].Relation != "BUILDS" || result[1].Object != "Microservices" {
		t.Errorf("Unexpected second triplet: %+v", result[1])
	}
}

func TestRelationExtractorExtract_EmptyText(t *testing.T) {
	entities := []Entity{
		{Name: "Alice", Type: "Person", Description: "A person"},
	}

	fakeLLM := &fakeLLMClient{response: "[]"}
	extractor := NewRelationExtractor(fakeLLM)

	result, err := extractor.Extract(context.Background(), "", entities)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected empty result for empty text, got %d triplets", len(result))
	}
}

func TestRelationExtractorExtract_EmptyEntities(t *testing.T) {
	fakeLLM := &fakeLLMClient{response: "[]"}
	extractor := NewRelationExtractor(fakeLLM)

	result, err := extractor.Extract(context.Background(), "Some text", []Entity{})
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected empty result for empty entities, got %d triplets", len(result))
	}
}

func TestRelationExtractorExtract_EmptyTripletList(t *testing.T) {
	entities := []Entity{
		{Name: "Alice", Type: "Person", Description: "A person"},
	}

	fakeLLM := &fakeLLMClient{response: "[]"}
	extractor := NewRelationExtractor(fakeLLM)

	result, err := extractor.Extract(context.Background(), "Some text with no relationships", entities)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d triplets", len(result))
	}
}

func TestRelationExtractorExtract_MalformedJSON(t *testing.T) {
	entities := []Entity{
		{Name: "Alice", Type: "Person", Description: "A person"},
	}

	fakeLLM := &fakeLLMClient{response: "not valid json"}
	extractor := NewRelationExtractor(fakeLLM)

	_, err := extractor.Extract(context.Background(), "Some text", entities)
	if err == nil {
		t.Fatal("Expected error for malformed JSON, got nil")
	}

	if !strings.Contains(err.Error(), "extract relationships") {
		t.Errorf("Expected extraction error, got: %v", err)
	}
}

func TestRelationExtractorExtract_LLMError(t *testing.T) {
	entities := []Entity{
		{Name: "Alice", Type: "Person", Description: "A person"},
	}

	fakeLLM := &fakeLLMClient{err: fmt.Errorf("LLM service unavailable")}
	extractor := NewRelationExtractor(fakeLLM)

	_, err := extractor.Extract(context.Background(), "Some text", entities)
	if err == nil {
		t.Fatal("Expected error when LLM fails, got nil")
	}

	if !strings.Contains(err.Error(), "extract relationships") {
		t.Errorf("Expected extraction error, got: %v", err)
	}
}

func TestRelationExtractorExtract_EmptySubject(t *testing.T) {
	entities := []Entity{
		{Name: "Alice", Type: "Person", Description: "A person"},
		{Name: "Go", Type: "Technology", Description: "A language"},
	}

	triplets := []Triplet{
		{Subject: "", Relation: "USES", Object: "Go"},
	}

	fakeLLM := &fakeLLMClient{response: tripletsJSON(triplets)}
	extractor := NewRelationExtractor(fakeLLM)

	_, err := extractor.Extract(context.Background(), "Some text", entities)
	if err == nil {
		t.Fatal("Expected error for empty subject, got nil")
	}

	if !strings.Contains(err.Error(), "empty subject") {
		t.Errorf("Expected 'empty subject' error, got: %v", err)
	}
}

func TestRelationExtractorExtract_EmptyRelation(t *testing.T) {
	entities := []Entity{
		{Name: "Alice", Type: "Person", Description: "A person"},
		{Name: "Go", Type: "Technology", Description: "A language"},
	}

	triplets := []Triplet{
		{Subject: "Alice", Relation: "", Object: "Go"},
	}

	fakeLLM := &fakeLLMClient{response: tripletsJSON(triplets)}
	extractor := NewRelationExtractor(fakeLLM)

	_, err := extractor.Extract(context.Background(), "Some text", entities)
	if err == nil {
		t.Fatal("Expected error for empty relation, got nil")
	}

	if !strings.Contains(err.Error(), "empty relation") {
		t.Errorf("Expected 'empty relation' error, got: %v", err)
	}
}

func TestRelationExtractorExtract_EmptyObject(t *testing.T) {
	entities := []Entity{
		{Name: "Alice", Type: "Person", Description: "A person"},
		{Name: "Go", Type: "Technology", Description: "A language"},
	}

	triplets := []Triplet{
		{Subject: "Alice", Relation: "USES", Object: ""},
	}

	fakeLLM := &fakeLLMClient{response: tripletsJSON(triplets)}
	extractor := NewRelationExtractor(fakeLLM)

	_, err := extractor.Extract(context.Background(), "Some text", entities)
	if err == nil {
		t.Fatal("Expected error for empty object, got nil")
	}

	if !strings.Contains(err.Error(), "empty object") {
		t.Errorf("Expected 'empty object' error, got: %v", err)
	}
}

func TestRelationExtractorExtract_UnknownSubject(t *testing.T) {
	entities := []Entity{
		{Name: "Alice", Type: "Person", Description: "A person"},
		{Name: "Go", Type: "Technology", Description: "A language"},
	}

	triplets := []Triplet{
		{Subject: "Bob", Relation: "USES", Object: "Go"},
	}

	fakeLLM := &fakeLLMClient{response: tripletsJSON(triplets)}
	extractor := NewRelationExtractor(fakeLLM)

	_, err := extractor.Extract(context.Background(), "Some text", entities)
	if err == nil {
		t.Fatal("Expected error for unknown subject (strict mode), got nil")
	}

	if !strings.Contains(err.Error(), "unknown subject") || !strings.Contains(err.Error(), "Bob") {
		t.Errorf("Expected 'unknown subject' error mentioning 'Bob', got: %v", err)
	}
}

func TestRelationExtractorExtract_UnknownObject(t *testing.T) {
	entities := []Entity{
		{Name: "Alice", Type: "Person", Description: "A person"},
		{Name: "Go", Type: "Technology", Description: "A language"},
	}

	triplets := []Triplet{
		{Subject: "Alice", Relation: "USES", Object: "Python"},
	}

	fakeLLM := &fakeLLMClient{response: tripletsJSON(triplets)}
	extractor := NewRelationExtractor(fakeLLM)

	_, err := extractor.Extract(context.Background(), "Some text", entities)
	if err == nil {
		t.Fatal("Expected error for unknown object (strict mode), got nil")
	}

	if !strings.Contains(err.Error(), "unknown object") || !strings.Contains(err.Error(), "Python") {
		t.Errorf("Expected 'unknown object' error mentioning 'Python', got: %v", err)
	}
}

func TestRelationExtractorExtract_CaseInsensitiveMatching(t *testing.T) {
	entities := []Entity{
		{Name: "Alice", Type: "Person", Description: "A person"},
		{Name: "Go", Type: "Technology", Description: "A language"},
	}

	triplets := []Triplet{
		{Subject: "ALICE", Relation: "USES", Object: "go"},
	}

	fakeLLM := &fakeLLMClient{response: tripletsJSON(triplets)}
	extractor := NewRelationExtractor(fakeLLM)

	result, err := extractor.Extract(context.Background(), "Some text", entities)
	if err != nil {
		t.Fatalf("Extract failed with case-insensitive match: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("Expected 1 triplet, got %d", len(result))
	}

	// The triplet should preserve the original casing from LLM response
	if result[0].Subject != "ALICE" || result[0].Object != "go" {
		t.Errorf("Expected original casing preserved, got: %+v", result[0])
	}
}

func TestRelationExtractorExtract_WhitespaceTrimming(t *testing.T) {
	entities := []Entity{
		{Name: "Alice", Type: "Person", Description: "A person"},
		{Name: "Go", Type: "Technology", Description: "A language"},
	}

	triplets := []Triplet{
		{Subject: "  Alice  ", Relation: "  USES  ", Object: "  Go  "},
	}

	fakeLLM := &fakeLLMClient{response: tripletsJSON(triplets)}
	extractor := NewRelationExtractor(fakeLLM)

	result, err := extractor.Extract(context.Background(), "Some text", entities)
	if err != nil {
		t.Fatalf("Extract failed with whitespace: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("Expected 1 triplet, got %d", len(result))
	}

	// Whitespace should be trimmed
	if result[0].Subject != "Alice" || result[0].Relation != "USES" || result[0].Object != "Go" {
		t.Errorf("Expected whitespace trimmed, got: %+v", result[0])
	}
}

func TestRelationExtractorExtract_Deduplication(t *testing.T) {
	entities := []Entity{
		{Name: "Alice", Type: "Person", Description: "A person"},
		{Name: "Go", Type: "Technology", Description: "A language"},
	}

	triplets := []Triplet{
		{Subject: "Alice", Relation: "USES", Object: "Go"},
		{Subject: "Alice", Relation: "USES", Object: "Go"}, // duplicate
		{Subject: "Alice", Relation: "USES", Object: "Go"}, // duplicate
	}

	fakeLLM := &fakeLLMClient{response: tripletsJSON(triplets)}
	extractor := NewRelationExtractor(fakeLLM)

	result, err := extractor.Extract(context.Background(), "Some text", entities)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("Expected 1 triplet after deduplication, got %d", len(result))
	}
}

func TestRelationExtractorExtract_DeduplicationPreservesOrder(t *testing.T) {
	entities := []Entity{
		{Name: "Alice", Type: "Person", Description: "A person"},
		{Name: "Bob", Type: "Person", Description: "Another person"},
		{Name: "Go", Type: "Technology", Description: "A language"},
	}

	triplets := []Triplet{
		{Subject: "Alice", Relation: "USES", Object: "Go"},
		{Subject: "Bob", Relation: "USES", Object: "Go"},
		{Subject: "Alice", Relation: "USES", Object: "Go"}, // duplicate of first
		{Subject: "Alice", Relation: "KNOWS", Object: "Bob"},
		{Subject: "Bob", Relation: "USES", Object: "Go"}, // duplicate of second
	}

	fakeLLM := &fakeLLMClient{response: tripletsJSON(triplets)}
	extractor := NewRelationExtractor(fakeLLM)

	result, err := extractor.Extract(context.Background(), "Some text", entities)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("Expected 3 triplets after deduplication, got %d", len(result))
	}

	// First occurrence order should be preserved
	expected := []Triplet{
		{Subject: "Alice", Relation: "USES", Object: "Go"},
		{Subject: "Bob", Relation: "USES", Object: "Go"},
		{Subject: "Alice", Relation: "KNOWS", Object: "Bob"},
	}

	for i, triplet := range result {
		if triplet.Subject != expected[i].Subject ||
			triplet.Relation != expected[i].Relation ||
			triplet.Object != expected[i].Object {
			t.Errorf("Triplet %d mismatch: expected %+v, got %+v", i, expected[i], triplet)
		}
	}
}

func TestRelationExtractorExtract_MultipleTriplets(t *testing.T) {
	entities := []Entity{
		{Name: "Alice", Type: "Person", Description: "A person"},
		{Name: "Bob", Type: "Person", Description: "Another person"},
		{Name: "Go", Type: "Technology", Description: "A language"},
		{Name: "Microservices", Type: "Concept", Description: "Architecture"},
	}

	triplets := []Triplet{
		{Subject: "Alice", Relation: "USES", Object: "Go"},
		{Subject: "Bob", Relation: "USES", Object: "Go"},
		{Subject: "Alice", Relation: "KNOWS", Object: "Bob"},
		{Subject: "Alice", Relation: "BUILDS", Object: "Microservices"},
		{Subject: "Microservices", Relation: "DEPENDS_ON", Object: "Go"},
	}

	fakeLLM := &fakeLLMClient{response: tripletsJSON(triplets)}
	extractor := NewRelationExtractor(fakeLLM)

	result, err := extractor.Extract(context.Background(), "Text with multiple relationships", entities)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(result) != 5 {
		t.Fatalf("Expected 5 triplets, got %d", len(result))
	}
}

func TestRelationExtractorExtract_DeduplicationCaseInsensitive(t *testing.T) {
	entities := []Entity{
		{Name: "Alice", Type: "Person", Description: "A person"},
		{Name: "Go", Type: "Technology", Description: "A language"},
	}

	triplets := []Triplet{
		{Subject: "Alice", Relation: "USES", Object: "Go"},
		{Subject: "alice", Relation: "uses", Object: "go"},     // same after normalization
		{Subject: "ALICE", Relation: "USES", Object: "GO"},     // same after normalization
		{Subject: "  Alice  ", Relation: "USES", Object: "Go"}, // same after trimming
	}

	fakeLLM := &fakeLLMClient{response: tripletsJSON(triplets)}
	extractor := NewRelationExtractor(fakeLLM)

	result, err := extractor.Extract(context.Background(), "Some text", entities)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// All should dedupe to 1 triplet (first occurrence wins)
	if len(result) != 1 {
		t.Fatalf("Expected 1 triplet after case-insensitive deduplication, got %d", len(result))
	}

	// First occurrence should be preserved with trimmed whitespace
	if result[0].Subject != "Alice" || result[0].Relation != "USES" || result[0].Object != "Go" {
		t.Errorf("Expected first occurrence (trimmed), got: %+v", result[0])
	}
}

func TestRelationExtractorExtract_PromptContainsText(t *testing.T) {
	entities := []Entity{
		{Name: "Test", Type: "Concept", Description: "A test"},
	}

	var capturedPrompt string
	fakeLLM := &fakeLLMClient{
		response: "[]",
		capturePrompt: func(prompt string) {
			capturedPrompt = prompt
		},
	}
	extractor := NewRelationExtractor(fakeLLM)

	inputText := "This is the specific text to extract from"
	_, err := extractor.Extract(context.Background(), inputText, entities)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if !strings.Contains(capturedPrompt, inputText) {
		t.Errorf("Expected prompt to contain input text")
	}
}

func TestRelationExtractorExtract_PromptContainsEntityNames(t *testing.T) {
	entities := []Entity{
		{Name: "Alice", Type: "Person", Description: "A person"},
		{Name: "Bob", Type: "Person", Description: "Another person"},
		{Name: "Go", Type: "Technology", Description: "A language"},
	}

	var capturedPrompt string
	fakeLLM := &fakeLLMClient{
		response: "[]",
		capturePrompt: func(prompt string) {
			capturedPrompt = prompt
		},
	}
	extractor := NewRelationExtractor(fakeLLM)

	_, err := extractor.Extract(context.Background(), "Some text", entities)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Prompt should contain entity names
	if !strings.Contains(capturedPrompt, "Alice") {
		t.Errorf("Expected prompt to contain 'Alice'")
	}
	if !strings.Contains(capturedPrompt, "Bob") {
		t.Errorf("Expected prompt to contain 'Bob'")
	}
	if !strings.Contains(capturedPrompt, "Go") {
		t.Errorf("Expected prompt to contain 'Go'")
	}
}
