package extraction

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/dan-solli/gognee/pkg/llm"
)

// fakeLLMClient is a test implementation of llm.LLMClient
type fakeLLMClient struct {
	response      string
	err           error
	capturePrompt func(string) // optional callback to capture the prompt
}

func (f *fakeLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	if f.capturePrompt != nil {
		f.capturePrompt(prompt)
	}
	if f.err != nil {
		return "", f.err
	}
	return f.response, nil
}

func (f *fakeLLMClient) CompleteWithSchema(ctx context.Context, prompt string, schema any) error {
	if f.capturePrompt != nil {
		f.capturePrompt(prompt)
	}
	if f.err != nil {
		return f.err
	}
	
	// Apply normalization just like the real OpenAI client does
	normalized, _, err := llm.NormalizeJSONArraysToStrings([]byte(f.response))
	if err != nil {
		return fmt.Errorf("failed to normalize LLM response: %w (input was: %q)", err, f.response)
	}
	
	return json.Unmarshal(normalized, schema)
}

func TestEntityExtractorExtract_Success(t *testing.T) {
	entities := []Entity{
		{
			Name:        "Go",
			Type:        "Technology",
			Description: "A programming language",
		},
		{
			Name:        "Alice",
			Type:        "Person",
			Description: "A software engineer",
		},
		{
			Name:        "Microservices",
			Type:        "Concept",
			Description: "An architectural pattern",
		},
	}

	jsonData, _ := json.Marshal(entities)
	fakeLLM := &fakeLLMClient{response: string(jsonData)}
	extractor := NewEntityExtractor(fakeLLM)

	result, err := extractor.Extract(context.Background(), "Alice is a software engineer who uses Go for microservices")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("Expected 3 entities, got %d", len(result))
	}

	if result[0].Name != "Go" || result[0].Type != "Technology" {
		t.Errorf("Expected first entity to be Go/Technology, got %s/%s", result[0].Name, result[0].Type)
	}

	if result[1].Name != "Alice" || result[1].Type != "Person" {
		t.Errorf("Expected second entity to be Alice/Person, got %s/%s", result[1].Name, result[1].Type)
	}

	if result[2].Name != "Microservices" || result[2].Type != "Concept" {
		t.Errorf("Expected third entity to be Microservices/Concept, got %s/%s", result[2].Name, result[2].Type)
	}
}

func TestEntityExtractorExtract_EmptyText(t *testing.T) {
	fakeLLM := &fakeLLMClient{response: "[]"}
	extractor := NewEntityExtractor(fakeLLM)

	result, err := extractor.Extract(context.Background(), "")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected empty result for empty text, got %d entities", len(result))
	}
}

func TestEntityExtractorExtract_EmptyEntityList(t *testing.T) {
	fakeLLM := &fakeLLMClient{response: "[]"}
	extractor := NewEntityExtractor(fakeLLM)

	result, err := extractor.Extract(context.Background(), "Some text with no entities")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d entities", len(result))
	}
}

func TestEntityExtractorExtract_MalformedJSON(t *testing.T) {
	fakeLLM := &fakeLLMClient{response: "not valid json"}
	extractor := NewEntityExtractor(fakeLLM)

	_, err := extractor.Extract(context.Background(), "Some text")
	if err == nil {
		t.Fatal("Expected error for malformed JSON, got nil")
	}

	if !strings.Contains(err.Error(), "extract entities") {
		t.Errorf("Expected extraction error, got: %v", err)
	}
}

func TestEntityExtractorExtract_LLMError(t *testing.T) {
	fakeLLM := &fakeLLMClient{err: fmt.Errorf("LLM service unavailable")}
	extractor := NewEntityExtractor(fakeLLM)

	_, err := extractor.Extract(context.Background(), "Some text")
	if err == nil {
		t.Fatal("Expected error when LLM fails, got nil")
	}

	if !strings.Contains(err.Error(), "extract entities") {
		t.Errorf("Expected extraction error, got: %v", err)
	}
}

func TestEntityExtractorExtract_EmptyName(t *testing.T) {
	entities := []Entity{
		{
			Name:        "",
			Type:        "Person",
			Description: "A person",
		},
	}

	jsonData, _ := json.Marshal(entities)
	fakeLLM := &fakeLLMClient{response: string(jsonData)}
	extractor := NewEntityExtractor(fakeLLM)

	_, err := extractor.Extract(context.Background(), "Some text")
	if err == nil {
		t.Fatal("Expected error for empty name, got nil")
	}

	if !strings.Contains(err.Error(), "empty name") {
		t.Errorf("Expected 'empty name' error, got: %v", err)
	}
}

func TestEntityExtractorExtract_EmptyType(t *testing.T) {
	entities := []Entity{
		{
			Name:        "Alice",
			Type:        "",
			Description: "A person",
		},
	}

	jsonData, _ := json.Marshal(entities)
	fakeLLM := &fakeLLMClient{response: string(jsonData)}
	extractor := NewEntityExtractor(fakeLLM)

	_, err := extractor.Extract(context.Background(), "Some text")
	if err == nil {
		t.Fatal("Expected error for empty type, got nil")
	}

	if !strings.Contains(err.Error(), "empty type") {
		t.Errorf("Expected 'empty type' error, got: %v", err)
	}
}

func TestEntityExtractorExtract_EmptyDescription(t *testing.T) {
	entities := []Entity{
		{
			Name:        "Alice",
			Type:        "Person",
			Description: "",
		},
	}

	jsonData, _ := json.Marshal(entities)
	fakeLLM := &fakeLLMClient{response: string(jsonData)}
	extractor := NewEntityExtractor(fakeLLM)

	_, err := extractor.Extract(context.Background(), "Some text")
	if err == nil {
		t.Fatal("Expected error for empty description, got nil")
	}

	if !strings.Contains(err.Error(), "empty description") {
		t.Errorf("Expected 'empty description' error, got: %v", err)
	}
}

func TestEntityExtractorExtract_UnknownTypeNormalization(t *testing.T) {
	// Test that unknown types are normalized to "Concept" with warning
	entities := []Entity{
		{
			Name:        "SomeProblemInstance",
			Type:        "UnknownType",
			Description: "A thing with unknown type",
		},
	}

	jsonData, _ := json.Marshal(entities)
	fakeLLM := &fakeLLMClient{response: string(jsonData)}
	extractor := NewEntityExtractor(fakeLLM)

	// Capture log output
	var logBuf bytes.Buffer
	originalOutput := log.Writer()
	log.SetOutput(&logBuf)
	defer log.SetOutput(originalOutput)

	result, err := extractor.Extract(context.Background(), "Some text")
	if err != nil {
		t.Fatalf("Extract should not fail for unknown type, got error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("Expected 1 entity, got %d", len(result))
	}

	// Verify type was normalized to Concept
	if result[0].Type != "Concept" {
		t.Errorf("Expected type to be normalized to 'Concept', got %s", result[0].Type)
	}

	// Verify warning was logged
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "gognee:") {
		t.Errorf("Expected log to contain 'gognee:' prefix, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "SomeProblemInstance") {
		t.Errorf("Expected log to contain entity name 'SomeProblemInstance', got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "UnknownType") {
		t.Errorf("Expected log to contain original type 'UnknownType', got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "normalizing to Concept") {
		t.Errorf("Expected log to mention normalization, got: %s", logOutput)
	}
}

func TestEntityExtractorExtract_MultipleUnknownTypes(t *testing.T) {
	// Test that multiple entities with unknown types are all normalized
	entities := []Entity{
		{Name: "Entity1", Type: "UnknownType1", Description: "First entity"},
		{Name: "Entity2", Type: "Concept", Description: "Valid type"},
		{Name: "Entity3", Type: "UnknownType2", Description: "Third entity"},
	}

	jsonData, _ := json.Marshal(entities)
	fakeLLM := &fakeLLMClient{response: string(jsonData)}
	extractor := NewEntityExtractor(fakeLLM)

	// Capture log output
	var logBuf bytes.Buffer
	originalOutput := log.Writer()
	log.SetOutput(&logBuf)
	defer log.SetOutput(originalOutput)

	result, err := extractor.Extract(context.Background(), "Some text")
	if err != nil {
		t.Fatalf("Extract should not fail, got error: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("Expected 3 entities, got %d", len(result))
	}

	// Verify types were normalized
	if result[0].Type != "Concept" {
		t.Errorf("Expected Entity1 type to be 'Concept', got %s", result[0].Type)
	}
	if result[1].Type != "Concept" {
		t.Errorf("Expected Entity2 type to remain 'Concept', got %s", result[1].Type)
	}
	if result[2].Type != "Concept" {
		t.Errorf("Expected Entity3 type to be 'Concept', got %s", result[2].Type)
	}

	// Verify two warnings were logged (one for each unknown type)
	logOutput := logBuf.String()
	warningCount := strings.Count(logOutput, "gognee:")
	if warningCount != 2 {
		t.Errorf("Expected 2 warnings, got %d. Log output: %s", warningCount, logOutput)
	}
}

func TestEntityExtractorExtract_InvalidType(t *testing.T) {
	// This test has been superseded by TestEntityExtractorExtract_UnknownTypeNormalization
	// Unknown types are now normalized to "Concept" instead of failing
	// Keeping this test to verify the new behavior explicitly
	entities := []Entity{
		{
			Name:        "Something",
			Type:        "InvalidType",
			Description: "A thing",
		},
	}

	jsonData, _ := json.Marshal(entities)
	fakeLLM := &fakeLLMClient{response: string(jsonData)}
	extractor := NewEntityExtractor(fakeLLM)

	// Capture log output
	var logBuf bytes.Buffer
	originalOutput := log.Writer()
	log.SetOutput(&logBuf)
	defer log.SetOutput(originalOutput)

	result, err := extractor.Extract(context.Background(), "Some text")
	if err != nil {
		t.Fatalf("Extract should not fail for unknown type (v1.0.1+), got error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("Expected 1 entity, got %d", len(result))
	}

	// Verify type was normalized
	if result[0].Type != "Concept" {
		t.Errorf("Expected type 'Concept', got %s", result[0].Type)
	}

	// Verify warning logged
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "InvalidType") {
		t.Errorf("Expected warning about 'InvalidType', got: %s", logOutput)
	}
}

func TestEntityExtractorExtract_AllValidTypes(t *testing.T) {
	// Test all valid entity types from the allowlist (original 7 + new 9 from v1.0.1)
	validTypes := []string{
		// Original 7 types
		"Person", "Concept", "System", "Decision", "Event", "Technology", "Pattern",
		// New 9 types (v1.0.1)
		"Problem", "Goal", "Location", "Organization", "Document", "Process", "Requirement", "Feature", "Task",
	}

	for _, entityType := range validTypes {
		t.Run(entityType, func(t *testing.T) {
			entities := []Entity{
				{
					Name:        "TestEntity",
					Type:        entityType,
					Description: "A test entity",
				},
			}

			jsonData, _ := json.Marshal(entities)
			fakeLLM := &fakeLLMClient{response: string(jsonData)}
			extractor := NewEntityExtractor(fakeLLM)

			result, err := extractor.Extract(context.Background(), "Some text")
			if err != nil {
				t.Fatalf("Extract failed for type %s: %v", entityType, err)
			}

			if len(result) != 1 {
				t.Fatalf("Expected 1 entity, got %d", len(result))
			}

			if result[0].Type != entityType {
				t.Errorf("Expected type %s, got %s", entityType, result[0].Type)
			}
		})
	}
}

func TestEntityExtractorExtract_MultipleEntities(t *testing.T) {
	entities := []Entity{
		{Name: "Entity1", Type: "Person", Description: "First entity"},
		{Name: "Entity2", Type: "Concept", Description: "Second entity"},
		{Name: "Entity3", Type: "System", Description: "Third entity"},
		{Name: "Entity4", Type: "Decision", Description: "Fourth entity"},
		{Name: "Entity5", Type: "Event", Description: "Fifth entity"},
	}

	jsonData, _ := json.Marshal(entities)
	fakeLLM := &fakeLLMClient{response: string(jsonData)}
	extractor := NewEntityExtractor(fakeLLM)

	result, err := extractor.Extract(context.Background(), "Text with multiple entities")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(result) != 5 {
		t.Fatalf("Expected 5 entities, got %d", len(result))
	}

	for i, entity := range result {
		expectedName := fmt.Sprintf("Entity%d", i+1)
		if entity.Name != expectedName {
			t.Errorf("Expected entity name %s, got %s", expectedName, entity.Name)
		}
	}
}
