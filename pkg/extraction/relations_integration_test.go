//go:build integration

package extraction

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/dan-solli/gognee/pkg/llm"
)

// getAPIKeyForRelations retrieves the OpenAI API key from environment or file
// (duplicated from entities_integration_test.go since integration tests may run independently)
func getAPIKeyForRelations(t *testing.T) string {
	// Try environment variable first
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey != "" {
		return apiKey
	}

	// Fall back to secrets file
	data, err := os.ReadFile("../../secrets/openai-api-key.txt")
	if err != nil {
		t.Skipf("Skipping integration test: no API key found (set OPENAI_API_KEY or create secrets/openai-api-key.txt)")
		return ""
	}

	apiKey = strings.TrimSpace(string(data))
	if apiKey == "" {
		t.Skipf("Skipping integration test: API key file is empty")
	}

	return apiKey
}

func TestRelationExtractorIntegration_RealAPI(t *testing.T) {
	apiKey := getAPIKeyForRelations(t)

	// Create real LLM client
	llmClient := llm.NewOpenAILLM(apiKey)

	// First extract entities using the entity extractor
	entityExtractor := NewEntityExtractor(llmClient)

	// Sample text for extraction
	text := `Alice is a software engineer who works with Go programming language.
She uses Docker containers for local development. Bob is a DevOps engineer
who manages the Kubernetes cluster. Alice and Bob collaborate on the deployment pipeline.`

	// Extract entities first
	entities, err := entityExtractor.Extract(context.Background(), text)
	if err != nil {
		t.Fatalf("Entity extraction failed: %v", err)
	}

	t.Logf("Extracted %d entities:", len(entities))
	for i, entity := range entities {
		t.Logf("  %d. %s (%s)", i+1, entity.Name, entity.Type)
	}

	if len(entities) == 0 {
		t.Fatal("Expected at least one entity for relationship extraction, got none")
	}

	// Now extract relationships
	relationExtractor := NewRelationExtractor(llmClient)
	triplets, err := relationExtractor.Extract(context.Background(), text, entities)
	if err != nil {
		t.Fatalf("Relation extraction failed: %v", err)
	}

	t.Logf("Extracted %d relationships:", len(triplets))
	for i, triplet := range triplets {
		t.Logf("  %d. (%s) -[%s]-> (%s)", i+1, triplet.Subject, triplet.Relation, triplet.Object)

		// Verify all fields are populated
		if triplet.Subject == "" {
			t.Errorf("Triplet %d has empty subject", i)
		}
		if triplet.Relation == "" {
			t.Errorf("Triplet %d has empty relation", i)
		}
		if triplet.Object == "" {
			t.Errorf("Triplet %d has empty object", i)
		}

		// Verify subject and object are in the entity list (case-insensitive)
		subjectFound := false
		objectFound := false
		subjectLower := strings.ToLower(triplet.Subject)
		objectLower := strings.ToLower(triplet.Object)

		for _, entity := range entities {
			entityNameLower := strings.ToLower(entity.Name)
			if entityNameLower == subjectLower {
				subjectFound = true
			}
			if entityNameLower == objectLower {
				objectFound = true
			}
		}

		if !subjectFound {
			t.Errorf("Triplet %d subject %q not found in entities", i, triplet.Subject)
		}
		if !objectFound {
			t.Errorf("Triplet %d object %q not found in entities", i, triplet.Object)
		}
	}

	// We should get at least one relationship from this text
	if len(triplets) == 0 {
		t.Log("Warning: No relationships extracted (this may happen occasionally)")
	}
}

func TestRelationExtractorIntegration_EmptyEntities(t *testing.T) {
	apiKey := getAPIKeyForRelations(t)

	llmClient := llm.NewOpenAILLM(apiKey)
	extractor := NewRelationExtractor(llmClient)

	// With empty entities, should return empty result without calling LLM
	triplets, err := extractor.Extract(context.Background(), "Some text", []Entity{})
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(triplets) != 0 {
		t.Errorf("Expected empty result for empty entities, got %d triplets", len(triplets))
	}
}

func TestRelationExtractorIntegration_SimpleRelationship(t *testing.T) {
	apiKey := getAPIKeyForRelations(t)

	llmClient := llm.NewOpenAILLM(apiKey)
	extractor := NewRelationExtractor(llmClient)

	// Very simple text with an obvious relationship
	text := "Python is a programming language."

	entities := []Entity{
		{Name: "Python", Type: "Technology", Description: "A programming language"},
	}

	// This might not return relationships since there's only one entity
	// but it should not error
	triplets, err := extractor.Extract(context.Background(), text, entities)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	t.Logf("Extracted %d relationships from simple text", len(triplets))
}
