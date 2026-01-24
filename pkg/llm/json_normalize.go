package llm

import (
	"encoding/json"
	"fmt"
	"strings"
)

// NormalizeJSONArraysToStrings walks a JSON structure and converts arrays of strings
// to comma-joined strings. This handles cases where the LLM returns arrays where
// strings are expected (e.g., {"object": ["a", "b"]} becomes {"object": "a, b"}).
//
// Note: Top-level arrays are preserved. Only arrays within object fields are normalized.
//
// Returns:
//   - normalized JSON bytes
//   - bool indicating whether any normalization occurred
//   - error if JSON parsing fails
func NormalizeJSONArraysToStrings(jsonBytes []byte) ([]byte, bool, error) {
	var data interface{}
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return nil, false, fmt.Errorf("failed to parse JSON: %w", err)
	}

	changed := false
	// Pass isTopLevel=true for the root value
	normalized := normalizeValue(data, &changed, true)

	result, err := json.Marshal(normalized)
	if err != nil {
		return nil, false, fmt.Errorf("failed to marshal normalized JSON: %w", err)
	}

	return result, changed, nil
}

// normalizeValue recursively walks a JSON value and normalizes string arrays to strings.
// It modifies the changed flag when normalization occurs.
// isTopLevel indicates if this is the root value (which should not be normalized if it's an array).
func normalizeValue(value interface{}, changed *bool, isTopLevel bool) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		// Recursively process object fields
		result := make(map[string]interface{})
		for key, val := range v {
			result[key] = normalizeValue(val, changed, false)
		}
		return result

	case []interface{}:
		// Don't normalize top-level arrays (these are valid return values like [])
		if isTopLevel {
			// Still recursively process elements
			result := make([]interface{}, len(v))
			for i, elem := range v {
				result[i] = normalizeValue(elem, changed, false)
			}
			return result
		}
		
		// Check if this is an array of strings (and we're not at top level)
		if isStringArray(v) {
			// Convert to comma-joined string
			*changed = true
			return joinStringArray(v)
		}
		// Otherwise, recursively process array elements
		result := make([]interface{}, len(v))
		for i, elem := range v {
			result[i] = normalizeValue(elem, changed, false)
		}
		return result

	default:
		// Primitive values pass through unchanged
		return value
	}
}

// isStringArray checks if a JSON array contains only string values
func isStringArray(arr []interface{}) bool {
	if len(arr) == 0 {
		return true // Empty array is considered a string array
	}
	for _, elem := range arr {
		if _, ok := elem.(string); !ok {
			return false
		}
	}
	return true
}

// joinStringArray converts an array of strings to a comma-joined string
func joinStringArray(arr []interface{}) string {
	if len(arr) == 0 {
		return ""
	}
	
	strs := make([]string, len(arr))
	for i, elem := range arr {
		strs[i] = elem.(string)
	}
	return strings.Join(strs, ", ")
}
