//go:build integration

package extraction

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/dan-solli/gognee/pkg/llm"
)

// getAPIKey retrieves the OpenAI API key from environment or file
func getAPIKey(t *testing.T) string {
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

func TestEntityExtractorIntegration_RealAPI(t *testing.T) {
	apiKey := getAPIKey(t)

	// Create real LLM client
	llmClient := llm.NewOpenAILLM(apiKey)
	extractor := NewEntityExtractor(llmClient)

	// Sample text for extraction
	text := `Alice is a software engineer who works with Go programming language.
She recently made a decision to adopt microservices architecture for the new system.
The team is using Docker containers and Kubernetes for deployment.`

	// Extract entities
	entities, err := extractor.Extract(context.Background(), text)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Verify we got some entities
	if len(entities) == 0 {
		t.Fatal("Expected at least one entity, got none")
	}

	t.Logf("Extracted %d entities:", len(entities))
	for i, entity := range entities {
		t.Logf("  %d. %s (%s): %s", i+1, entity.Name, entity.Type, entity.Description)

		// Verify all fields are populated
		if entity.Name == "" {
			t.Errorf("Entity %d has empty name", i)
		}
		if entity.Type == "" {
			t.Errorf("Entity %d has empty type", i)
		}
		if entity.Description == "" {
			t.Errorf("Entity %d has empty description", i)
		}

		// Verify type is valid
		if !validEntityTypes[entity.Type] {
			t.Errorf("Entity %d (%s) has invalid type: %s", i, entity.Name, entity.Type)
		}
	}

	// Check for expected entities (these should be found by the LLM)
	foundAlice := false
	foundGo := false
	foundMicroservices := false

	for _, entity := range entities {
		nameLower := strings.ToLower(entity.Name)
		if strings.Contains(nameLower, "alice") {
			foundAlice = true
			if entity.Type != "Person" {
				t.Errorf("Expected Alice to be type Person, got %s", entity.Type)
			}
		}
		if strings.Contains(nameLower, "go") {
			foundGo = true
			if entity.Type != "Technology" {
				t.Errorf("Expected Go to be type Technology, got %s", entity.Type)
			}
		}
		if strings.Contains(nameLower, "microservices") {
			foundMicroservices = true
			if entity.Type != "Concept" && entity.Type != "Pattern" {
				t.Errorf("Expected Microservices to be type Concept or Pattern, got %s", entity.Type)
			}
		}
	}

	if !foundAlice {
		t.Error("Expected to find entity 'Alice' but didn't")
	}
	if !foundGo {
		t.Error("Expected to find entity 'Go' but didn't")
	}
	if !foundMicroservices {
		t.Error("Expected to find entity 'Microservices' but didn't")
	}
}

func TestEntityExtractorIntegration_EmptyText(t *testing.T) {
	apiKey := getAPIKey(t)

	llmClient := llm.NewOpenAILLM(apiKey)
	extractor := NewEntityExtractor(llmClient)

	entities, err := extractor.Extract(context.Background(), "")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(entities) != 0 {
		t.Errorf("Expected 0 entities for empty text, got %d", len(entities))
	}
}

func TestEntityExtractorIntegration_NoEntities(t *testing.T) {
	apiKey := getAPIKey(t)

	llmClient := llm.NewOpenAILLM(apiKey)
	extractor := NewEntityExtractor(llmClient)

	// Text with no meaningful entities
	text := "The sky is blue."

	entities, err := extractor.Extract(context.Background(), text)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// It's okay to have 0 or a few entities for simple text
	t.Logf("Extracted %d entities from simple text", len(entities))
}
