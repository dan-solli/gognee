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

	result, err := extractor.Extract(context.Background(), "Some text", entities)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Empty subject should be filtered out
	if len(result) != 0 {
		t.Errorf("Expected empty result (filtered), got %d triplets", len(result))
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

	result, err := extractor.Extract(context.Background(), "Some text", entities)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Empty relation should be filtered out
	if len(result) != 0 {
		t.Errorf("Expected empty result (filtered), got %d triplets", len(result))
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

	result, err := extractor.Extract(context.Background(), "Some text", entities)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Empty object should be filtered out
	if len(result) != 0 {
		t.Errorf("Expected empty result (filtered), got %d triplets", len(result))
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

	result, err := extractor.Extract(context.Background(), "Some text", entities)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Unknown subject should be filtered out
	if len(result) != 0 {
		t.Errorf("Expected empty result (filtered), got %d triplets", len(result))
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

	result, err := extractor.Extract(context.Background(), "Some text", entities)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Unknown object should be filtered out
	if len(result) != 0 {
		t.Errorf("Expected empty result (filtered), got %d triplets", len(result))
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

// TestRelationExtractorExtract_ObjectIsArray verifies that when the LLM returns
// an array for the object field, it gets normalized to a comma-joined string.
// NOTE: The normalized string may not match any entity, so validation may filter it out.
// This test verifies the normalization happens without error (no unmarshal failure).
func TestRelationExtractorExtract_ObjectIsArray(t *testing.T) {
	entities := []Entity{
		{Name: "Wishlist", Type: "Feature", Description: "Shopping wishlist feature"},
		{Name: "Plan, Shopping Flow", Type: "Concept", Description: "Combined concept"}, // Entity matching normalized form
	}

	// LLM returns array for object field (production error case)
	fakeLLM := &fakeLLMClient{
		response: `[{"subject": "Wishlist", "relation": "USES", "object": ["Plan", "Shopping Flow"]}]`,
	}
	extractor := NewRelationExtractor(fakeLLM)

	result, err := extractor.Extract(context.Background(), "Wishlist uses plan and shopping flow", entities)
	if err != nil {
		t.Fatalf("Extract should succeed with array normalization, got error: %v", err)
	}

	// With proper entity matching, we should get 1 result
	if len(result) != 1 {
		t.Fatalf("Expected 1 triplet, got %d", len(result))
	}

	// Verify normalization occurred: array became comma-joined string
	if result[0].Subject != "Wishlist" {
		t.Errorf("Expected subject='Wishlist', got %q", result[0].Subject)
	}
	if result[0].Relation != "USES" {
		t.Errorf("Expected relation='USES', got %q", result[0].Relation)
	}
	if result[0].Object != "Plan, Shopping Flow" {
		t.Errorf("Expected object='Plan, Shopping Flow' (normalized), got %q", result[0].Object)
	}
}

// TestRelationExtractorExtract_SubjectIsArray verifies subject array normalization
func TestRelationExtractorExtract_SubjectIsArray(t *testing.T) {
	entities := []Entity{
		{Name: "Alice, Bob", Type: "Team", Description: "Team members"}, // Entity matching normalized form
		{Name: "Project", Type: "Thing", Description: "Software project"},
	}

	fakeLLM := &fakeLLMClient{
		response: `[{"subject": ["Alice", "Bob"], "relation": "WORKS_ON", "object": "Project"}]`,
	}
	extractor := NewRelationExtractor(fakeLLM)

	result, err := extractor.Extract(context.Background(), "Alice and Bob work on project", entities)
	if err != nil {
		t.Fatalf("Extract should succeed, got error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("Expected 1 triplet, got %d", len(result))
	}

	if result[0].Subject != "Alice, Bob" {
		t.Errorf("Expected subject='Alice, Bob', got %q", result[0].Subject)
	}
	if result[0].Object != "Project" {
		t.Errorf("Expected object='Project', got %q", result[0].Object)
	}
}

// TestRelationExtractorExtract_MultipleArrayFields verifies handling of multiple array fields
func TestRelationExtractorExtract_MultipleArrayFields(t *testing.T) {
	entities := []Entity{
		{Name: "Service A, Service B", Type: "ServiceGroup", Description: "Service group"},
		{Name: "Database, Cache", Type: "ResourceGroup", Description: "Resource group"},
	}

	// LLM returns arrays for both subject and object
	fakeLLM := &fakeLLMClient{
		response: `[{"subject": ["Service A", "Service B"], "relation": "DEPENDS_ON", "object": ["Database", "Cache"]}]`,
	}
	extractor := NewRelationExtractor(fakeLLM)

	result, err := extractor.Extract(context.Background(), "Services depend on database and cache", entities)
	if err != nil {
		t.Fatalf("Extract should succeed, got error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("Expected 1 triplet, got %d", len(result))
	}

	if result[0].Subject != "Service A, Service B" {
		t.Errorf("Expected subject='Service A, Service B', got %q", result[0].Subject)
	}
	if result[0].Relation != "DEPENDS_ON" {
		t.Errorf("Expected relation='DEPENDS_ON', got %q", result[0].Relation)
	}
	if result[0].Object != "Database, Cache" {
		t.Errorf("Expected object='Database, Cache', got %q", result[0].Object)
	}
}
