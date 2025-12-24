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

Use clear, consistent relation names like:
- USES, DEPENDS_ON, CREATED_BY, CONTAINS, IS_A, RELATES_TO, MENTIONS

Text:
---
%s
---

Known entities: %s

Return ONLY valid JSON array:
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
func validateAndProcessTriplets(triplets []Triplet, entityLookup map[string]bool) ([]Triplet, error) {
	result := make([]Triplet, 0, len(triplets))

	for i, triplet := range triplets {
		// Trim whitespace
		subject := strings.TrimSpace(triplet.Subject)
		relation := strings.TrimSpace(triplet.Relation)
		object := strings.TrimSpace(triplet.Object)

		// Validate non-empty fields
		if subject == "" {
			return nil, fmt.Errorf("triplet at index %d has empty subject", i)
		}
		if relation == "" {
			return nil, fmt.Errorf("triplet at index %d has empty relation", i)
		}
		if object == "" {
			return nil, fmt.Errorf("triplet at index %d has empty object", i)
		}

		// Strict mode: validate subject exists in entity list (case-insensitive)
		if !entityLookup[strings.ToLower(subject)] {
			return nil, fmt.Errorf("triplet at index %d has unknown subject: %q (not in known entities)", i, subject)
		}

		// Strict mode: validate object exists in entity list (case-insensitive)
		if !entityLookup[strings.ToLower(object)] {
			return nil, fmt.Errorf("triplet at index %d has unknown object: %q (not in known entities)", i, object)
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
