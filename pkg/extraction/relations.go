package extraction

import (
	"context"
	"fmt"
	"strings"

	"github.com/dan-solli/gognee/pkg/llm"
)

// Triplet represents a relationship between two entities
type Triplet struct {
	Subject  string `json:"subject"`
	Relation string `json:"relation"`
	Object   string `json:"object"`
}

// relationExtractionPrompt is the prompt template for relationship extraction
const relationExtractionPrompt = `You are a knowledge graph construction assistant.

Given this text and the entities already extracted, identify relationships between them.
Express each relationship as a triplet: (subject, relation, object)

IMPORTANT: Use ONLY entity names from the "Known entities" list below. Do not create new entities or use partial names.

Use clear, consistent relation names like:
- USES, DEPENDS_ON, CREATED_BY, CONTAINS, IS_A, RELATES_TO, MENTIONS

Text:
---
%s
---

Known entities: %s

Return ONLY valid JSON array where subject and object are exact matches from the Known entities list:
[{"subject": "...", "relation": "...", "object": "..."}, ...]`

// RelationExtractor extracts relationships between entities from text using an LLM
type RelationExtractor struct {
	LLM llm.LLMClient
}

// NewRelationExtractor creates a new relation extractor
func NewRelationExtractor(llmClient llm.LLMClient) *RelationExtractor {
	return &RelationExtractor{
		LLM: llmClient,
	}
}

// Extract extracts relationships from the given text using the provided entities
func (r *RelationExtractor) Extract(ctx context.Context, text string, entities []Entity) ([]Triplet, error) {
	// Return empty result for empty text or no entities
	if text == "" || len(entities) == 0 {
		return []Triplet{}, nil
	}

	// Build entity names list for the prompt
	entityNames := buildEntityNamesList(entities)

	// Build the prompt
	prompt := fmt.Sprintf(relationExtractionPrompt, text, entityNames)

	// Call the LLM
	var triplets []Triplet
	if err := r.LLM.CompleteWithSchema(ctx, prompt, &triplets); err != nil {
		return nil, fmt.Errorf("failed to extract relationships: %w", err)
	}

	// Build entity lookup map for case-insensitive matching
	entityLookup := buildEntityLookup(entities)

	// Validate and process triplets
	validatedTriplets, err := validateAndProcessTriplets(triplets, entityLookup)
	if err != nil {
		return nil, err
	}

	// Deduplicate triplets
	result := deduplicateTriplets(validatedTriplets)

	return result, nil
}

// buildEntityNamesList creates a comma-separated list of entity names for the prompt
func buildEntityNamesList(entities []Entity) string {
	names := make([]string, len(entities))
	for i, entity := range entities {
		names[i] = entity.Name
	}
	return strings.Join(names, ", ")
}

// buildEntityLookup creates a case-insensitive lookup map of entity names
func buildEntityLookup(entities []Entity) map[string]bool {
	lookup := make(map[string]bool)
	for _, entity := range entities {
		// Store lowercase version for case-insensitive matching
		lookup[strings.ToLower(strings.TrimSpace(entity.Name))] = true
	}
	return lookup
}

// validateAndProcessTriplets validates each triplet and ensures linking to known entities
// Invalid triplets are filtered out rather than causing the entire extraction to fail
func validateAndProcessTriplets(triplets []Triplet, entityLookup map[string]bool) ([]Triplet, error) {
	result := make([]Triplet, 0, len(triplets))

	for _, triplet := range triplets {
		// Trim whitespace
		subject := strings.TrimSpace(triplet.Subject)
		relation := strings.TrimSpace(triplet.Relation)
		object := strings.TrimSpace(triplet.Object)

		// Skip triplets with empty fields
		if subject == "" || relation == "" || object == "" {
			continue
		}

		// Filter mode: skip triplets referencing unknown entities (case-insensitive)
		if !entityLookup[strings.ToLower(subject)] {
			continue
		}

		if !entityLookup[strings.ToLower(object)] {
			continue
		}

		// Add validated and trimmed triplet
		result = append(result, Triplet{
			Subject:  subject,
			Relation: relation,
			Object:   object,
		})
	}

	return result, nil
}

// deduplicateTriplets removes duplicate triplets, preserving first occurrence order
// Comparison is case-insensitive for subject and object (matching entity linking behavior)
func deduplicateTriplets(triplets []Triplet) []Triplet {
	seen := make(map[string]bool)
	result := make([]Triplet, 0, len(triplets))

	for _, triplet := range triplets {
		// Create a normalized key for comparison (case-insensitive)
		key := strings.ToLower(triplet.Subject) + "|" +
			strings.ToLower(triplet.Relation) + "|" +
			strings.ToLower(triplet.Object)

		if !seen[key] {
			seen[key] = true
			result = append(result, triplet)
		}
	}

	return result
}
