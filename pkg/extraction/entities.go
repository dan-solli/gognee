// Package extraction provides entity and relationship extraction from text
package extraction

import (
	"context"
	"fmt"
	"log"

	"github.com/dan-solli/gognee/pkg/llm"
)

// Entity represents a named entity extracted from text
type Entity struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// Valid entity types from the roadmap
var validEntityTypes = map[string]bool{
	// Original 7 types
	"Person":     true,
	"Concept":    true,
	"System":     true,
	"Decision":   true,
	"Event":      true,
	"Technology": true,
	"Pattern":    true,
	// Additional 9 types (added in v1.0.1)
	"Problem":      true,
	"Goal":         true,
	"Location":     true,
	"Organization": true,
	"Document":     true,
	"Process":      true,
	"Requirement":  true,
	"Feature":      true,
	"Task":         true,
}

// entityExtractionPrompt is the prompt template for entity extraction
const entityExtractionPrompt = `You are a knowledge graph construction assistant.

Extract all meaningful entities from this text. For each entity, provide:
- name: The entity name
- type: One of [Person, Concept, System, Decision, Event, Technology, Pattern, Problem, Goal, Location, Organization, Document, Process, Requirement, Feature, Task]
- description: Brief description (1 sentence)

Text:
---
%s
---

Return ONLY valid JSON array:
[{"name": "...", "type": "...", "description": "..."}, ...]`

// EntityExtractor extracts entities from text using an LLM
type EntityExtractor struct {
	LLM llm.LLMClient
}

// NewEntityExtractor creates a new entity extractor
func NewEntityExtractor(llmClient llm.LLMClient) *EntityExtractor {
	return &EntityExtractor{
		LLM: llmClient,
	}
}

// Extract extracts entities from the given text
func (e *EntityExtractor) Extract(ctx context.Context, text string) ([]Entity, error) {
	if text == "" {
		return []Entity{}, nil
	}

	prompt := fmt.Sprintf(entityExtractionPrompt, text)

	var entities []Entity
	if err := e.LLM.CompleteWithSchema(ctx, prompt, &entities); err != nil {
		return nil, fmt.Errorf("failed to extract entities: %w", err)
	}

	// Validate entities
	for i, entity := range entities {
		// Check required fields
		if entity.Name == "" {
			return nil, fmt.Errorf("entity at index %d has empty name", i)
		}
		if entity.Type == "" {
			return nil, fmt.Errorf("entity at index %d (%s) has empty type", i, entity.Name)
		}
		if entity.Description == "" {
			return nil, fmt.Errorf("entity at index %d (%s) has empty description", i, entity.Name)
		}

		// Normalize unknown types to Concept with warning
		if !validEntityTypes[entity.Type] {
			log.Printf("gognee: entity %q has unrecognized type %q, normalizing to Concept", entity.Name, entity.Type)
			entities[i].Type = "Concept"
		}
	}

	return entities, nil
}
