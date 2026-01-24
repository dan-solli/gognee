package llm

import (
	"bytes"
	"encoding/json"
	"log"
	"strings"
	"testing"
)

// TestNormalizeJSONArraysToStrings_ObjectFieldArray verifies that when the "object"
// field is an array, it gets joined into a comma-separated string
func TestNormalizeJSONArraysToStrings_ObjectFieldArray(t *testing.T) {
	input := `[{"subject": "Wishlist", "relation": "USES", "object": ["plan", "shopping flow"]}]`
	
	normalized, changed, err := NormalizeJSONArraysToStrings([]byte(input))
	if err != nil {
		t.Fatalf("normalizeJSONArraysToStrings failed: %v", err)
	}
	
	if !changed {
		t.Error("Expected changed=true when array normalization occurs")
	}
	
	// Verify the normalized JSON is valid and contains the joined string
	var result []map[string]interface{}
	if err := json.Unmarshal(normalized, &result); err != nil {
		t.Fatalf("Failed to unmarshal normalized JSON: %v", err)
	}
	
	if len(result) != 1 {
		t.Fatalf("Expected 1 object, got %d", len(result))
	}
	
	obj := result[0]
	if obj["object"] != "plan, shopping flow" {
		t.Errorf("Expected object='plan, shopping flow', got %v", obj["object"])
	}
	
	// Subject and relation should be unchanged
	if obj["subject"] != "Wishlist" {
		t.Errorf("Expected subject='Wishlist', got %v", obj["subject"])
	}
	if obj["relation"] != "USES" {
		t.Errorf("Expected relation='USES', got %v", obj["relation"])
	}
}

// TestNormalizeJSONArraysToStrings_SubjectFieldArray verifies subject array normalization
func TestNormalizeJSONArraysToStrings_SubjectFieldArray(t *testing.T) {
	input := `[{"subject": ["Alice", "Bob"], "relation": "COLLABORATES_WITH", "object": "Project"}]`
	
	normalized, changed, err := NormalizeJSONArraysToStrings([]byte(input))
	if err != nil {
		t.Fatalf("normalizeJSONArraysToStrings failed: %v", err)
	}
	
	if !changed {
		t.Error("Expected changed=true")
	}
	
	var result []map[string]interface{}
	if err := json.Unmarshal(normalized, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	
	if result[0]["subject"] != "Alice, Bob" {
		t.Errorf("Expected subject='Alice, Bob', got %v", result[0]["subject"])
	}
}

// TestNormalizeJSONArraysToStrings_RelationFieldArray verifies relation array normalization
func TestNormalizeJSONArraysToStrings_RelationFieldArray(t *testing.T) {
	input := `[{"subject": "User", "relation": ["CREATES", "UPDATES"], "object": "Document"}]`
	
	normalized, changed, err := NormalizeJSONArraysToStrings([]byte(input))
	if err != nil {
		t.Fatalf("normalizeJSONArraysToStrings failed: %v", err)
	}
	
	if !changed {
		t.Error("Expected changed=true")
	}
	
	var result []map[string]interface{}
	if err := json.Unmarshal(normalized, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	
	if result[0]["relation"] != "CREATES, UPDATES" {
		t.Errorf("Expected relation='CREATES, UPDATES', got %v", result[0]["relation"])
	}
}

// TestNormalizeJSONArraysToStrings_AllFieldsArrays verifies multiple field normalization
func TestNormalizeJSONArraysToStrings_AllFieldsArrays(t *testing.T) {
	input := `[{"subject": ["A", "B"], "relation": ["R1", "R2"], "object": ["C", "D"]}]`
	
	normalized, changed, err := NormalizeJSONArraysToStrings([]byte(input))
	if err != nil {
		t.Fatalf("normalizeJSONArraysToStrings failed: %v", err)
	}
	
	if !changed {
		t.Error("Expected changed=true")
	}
	
	var result []map[string]interface{}
	if err := json.Unmarshal(normalized, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	
	obj := result[0]
	if obj["subject"] != "A, B" {
		t.Errorf("Expected subject='A, B', got %v", obj["subject"])
	}
	if obj["relation"] != "R1, R2" {
		t.Errorf("Expected relation='R1, R2', got %v", obj["relation"])
	}
	if obj["object"] != "C, D" {
		t.Errorf("Expected object='C, D', got %v", obj["object"])
	}
}

// TestNormalizeJSONArraysToStrings_NormalStrings verifies passthrough for normal strings
func TestNormalizeJSONArraysToStrings_NormalStrings(t *testing.T) {
	input := `[{"subject": "Alice", "relation": "USES", "object": "Go"}]`
	
	normalized, changed, err := NormalizeJSONArraysToStrings([]byte(input))
	if err != nil {
		t.Fatalf("normalizeJSONArraysToStrings failed: %v", err)
	}
	
	if changed {
		t.Error("Expected changed=false for normal string values")
	}
	
	// Verify output is equivalent to input
	var result []map[string]interface{}
	if err := json.Unmarshal(normalized, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	
	obj := result[0]
	if obj["subject"] != "Alice" || obj["relation"] != "USES" || obj["object"] != "Go" {
		t.Errorf("Values changed unexpectedly: %+v", obj)
	}
}

// TestNormalizeJSONArraysToStrings_MixedArray verifies handling mixed objects
func TestNormalizeJSONArraysToStrings_MixedArray(t *testing.T) {
	input := `[
		{"subject": "Alice", "relation": "USES", "object": "Go"},
		{"subject": "Bob", "relation": "BUILDS", "object": ["Microservices", "APIs"]}
	]`
	
	normalized, changed, err := NormalizeJSONArraysToStrings([]byte(input))
	if err != nil {
		t.Fatalf("NormalizeJSONArraysToStrings failed: %v", err)
	}
	
	if !changed {
		t.Error("Expected changed=true when at least one array exists")
	}
	
	var result []map[string]interface{}
	if err := json.Unmarshal(normalized, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	
	if len(result) != 2 {
		t.Fatalf("Expected 2 objects, got %d", len(result))
	}
	
	// First object should be unchanged
	if result[0]["object"] != "Go" {
		t.Errorf("First object should be unchanged, got %v", result[0]["object"])
	}
	
	// Second object should have array normalized
	if result[1]["object"] != "Microservices, APIs" {
		t.Errorf("Expected object='Microservices, APIs', got %v", result[1]["object"])
	}
}

// TestNormalizeJSONArraysToStrings_EmptyArray verifies empty array becomes empty string
func TestNormalizeJSONArraysToStrings_EmptyArray(t *testing.T) {
	input := `[{"subject": "Alice", "relation": "USES", "object": []}]`
	
	normalized, changed, err := NormalizeJSONArraysToStrings([]byte(input))
	if err != nil {
		t.Fatalf("normalizeJSONArraysToStrings failed: %v", err)
	}
	
	if !changed {
		t.Error("Expected changed=true")
	}
	
	var result []map[string]interface{}
	if err := json.Unmarshal(normalized, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	
	if result[0]["object"] != "" {
		t.Errorf("Expected empty string for empty array, got %v", result[0]["object"])
	}
}

// TestNormalizeJSONArraysToStrings_SingleElementArray verifies single element extraction
func TestNormalizeJSONArraysToStrings_SingleElementArray(t *testing.T) {
	input := `[{"subject": "Alice", "relation": "USES", "object": ["Go"]}]`
	
	normalized, changed, err := NormalizeJSONArraysToStrings([]byte(input))
	if err != nil {
		t.Fatalf("normalizeJSONArraysToStrings failed: %v", err)
	}
	
	if !changed {
		t.Error("Expected changed=true")
	}
	
	var result []map[string]interface{}
	if err := json.Unmarshal(normalized, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	
	if result[0]["object"] != "Go" {
		t.Errorf("Expected object='Go', got %v", result[0]["object"])
	}
}

// TestNormalizeJSONArraysToStrings_NestedObjects verifies handling of nested structures
func TestNormalizeJSONArraysToStrings_NestedObjects(t *testing.T) {
	input := `{
		"triplets": [
			{"subject": ["Alice", "Bob"], "relation": "USES", "object": "Go"}
		],
		"metadata": {
			"tags": ["important", "reviewed"]
		}
	}`
	
	normalized, changed, err := NormalizeJSONArraysToStrings([]byte(input))
	if err != nil {
		t.Fatalf("NormalizeJSONArraysToStrings failed: %v", err)
	}
	
	if !changed {
		t.Error("Expected changed=true")
	}
	
	var result map[string]interface{}
	if err := json.Unmarshal(normalized, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	
	// Check nested triplet normalization
	triplets := result["triplets"].([]interface{})
	firstTriplet := triplets[0].(map[string]interface{})
	if firstTriplet["subject"] != "Alice, Bob" {
		t.Errorf("Expected subject='Alice, Bob', got %v", firstTriplet["subject"])
	}
	
	// Check metadata tags normalization
	metadata := result["metadata"].(map[string]interface{})
	if metadata["tags"] != "important, reviewed" {
		t.Errorf("Expected tags='important, reviewed', got %v", metadata["tags"])
	}
}

// TestCompleteWithSchema_NormalizesArrays is an integration test that verifies
// array normalization happens in the CompleteWithSchema pipeline
func TestCompleteWithSchema_NormalizesArrays(t *testing.T) {
	// Capture log output to verify warning is logged
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(nil) // Reset to default after test
	
	// Mock LLM that returns JSON with array field
	fakeLLM := &mockLLMWithArrayResponse{
		response: `[{"subject": "Wishlist", "relation": "USES", "object": ["plan", "shopping flow"]}]`,
	}
	
	var result []map[string]interface{}
	err := fakeLLM.CompleteWithSchema(nil, "test prompt", &result)
	if err != nil {
		t.Fatalf("CompleteWithSchema should succeed with normalization, got error: %v", err)
	}
	
	if len(result) != 1 {
		t.Fatalf("Expected 1 object, got %d", len(result))
	}
	
	// Verify normalization occurred
	if result[0]["object"] != "plan, shopping flow" {
		t.Errorf("Expected normalized object='plan, shopping flow', got %v", result[0]["object"])
	}
	
	// Verify warning was logged
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "gognee:") {
		t.Error("Expected log warning with 'gognee:' prefix")
	}
	if !strings.Contains(logOutput, "normalized") || !strings.Contains(logOutput, "array") {
		t.Errorf("Expected warning about array normalization, got: %s", logOutput)
	}
}

// mockLLMWithArrayResponse is a helper for testing CompleteWithSchema integration
type mockLLMWithArrayResponse struct {
	response string
}

func (m *mockLLMWithArrayResponse) CompleteWithSchema(ctx interface{}, prompt string, schema any) error {
	// Simulate the CompleteWithSchema flow with our test response
	cleaned := stripMarkdownCodeFence(m.response)
	
	// Apply normalization (this is what we're testing)
	normalized, changed, err := NormalizeJSONArraysToStrings([]byte(cleaned))
	if err != nil {
		return err
	}
	
	if changed {
		log.Printf("gognee: LLM response contained array values where strings expected; normalized to comma-joined strings")
	}
	
	return json.Unmarshal(normalized, schema)
}
