package chunker

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
)

// Chunk represents a single chunk of text with metadata
type Chunk struct {
	ID         string
	Text       string
	Index      int
	TokenCount int
}

// Chunker splits text into overlapping chunks with sentence boundary awareness
type Chunker struct {
	MaxTokens int // Maximum tokens per chunk (default: 512)
	Overlap   int // Token overlap between chunks (default: 50)
}

// Chunk splits the input text into chunks
func (c *Chunker) Chunk(text string) []Chunk {
	if text == "" {
		return []Chunk{}
	}

	// Apply defaults if not set
	maxTokens := c.MaxTokens
	if maxTokens == 0 {
		maxTokens = 512
	}
	overlap := c.Overlap
	if overlap == 0 {
		overlap = 50
	}

	// Split text into sentences for boundary awareness
	sentences := splitSentences(text)
	if len(sentences) == 0 {
		return []Chunk{}
	}

	var chunks []Chunk
	var currentChunk []string
	var currentTokenCount int

	for _, sentence := range sentences {
		sentenceTokens := countTokens(sentence)

		// If adding this sentence would exceed max tokens, finalize current chunk
		if currentTokenCount+sentenceTokens > maxTokens && len(currentChunk) > 0 {
			chunkText := strings.Join(currentChunk, " ")
			chunks = append(chunks, Chunk{
				ID:         generateChunkID(chunkText, len(chunks)),
				Text:       chunkText,
				Index:      len(chunks),
				TokenCount: currentTokenCount,
			})

			// Keep overlap tokens for next chunk
			currentChunk = getOverlapSentences(currentChunk, overlap)
			currentTokenCount = countTokensForSentences(currentChunk)
		}

		currentChunk = append(currentChunk, sentence)
		currentTokenCount += sentenceTokens
	}

	// Add final chunk if there's remaining content
	if len(currentChunk) > 0 {
		chunkText := strings.Join(currentChunk, " ")
		chunks = append(chunks, Chunk{
			ID:         generateChunkID(chunkText, len(chunks)),
			Text:       chunkText,
			Index:      len(chunks),
			TokenCount: currentTokenCount,
		})
	}

	return chunks
}

// splitSentences splits text into sentences based on common terminators
func splitSentences(text string) []string {
	// Simple sentence splitting on ., !, ? followed by space or end
	var sentences []string
	var current strings.Builder

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		current.WriteRune(runes[i])

		// Check for sentence terminators
		if runes[i] == '.' || runes[i] == '!' || runes[i] == '?' {
			// Check if followed by space/end
			if i+1 >= len(runes) || unicode.IsSpace(runes[i+1]) {
				sentence := strings.TrimSpace(current.String())
				if sentence != "" {
					sentences = append(sentences, sentence)
				}
				current.Reset()
			}
		}
	}

	// Add any remaining text
	if current.Len() > 0 {
		sentence := strings.TrimSpace(current.String())
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
	}

	// Fallback: if no sentences detected, treat whole text as one sentence
	if len(sentences) == 0 && strings.TrimSpace(text) != "" {
		sentences = append(sentences, strings.TrimSpace(text))
	}

	return sentences
}

// countTokens estimates token count using word-based heuristic
// Note: This is an approximation. For accurate token counting, use a proper tokenizer.
func countTokens(text string) int {
	words := strings.Fields(text)
	return len(words)
}

// countTokensForSentences counts total tokens for a slice of sentences
func countTokensForSentences(sentences []string) int {
	total := 0
	for _, s := range sentences {
		total += countTokens(s)
	}
	return total
}

// getOverlapSentences returns the last N tokens worth of sentences for overlap
func getOverlapSentences(sentences []string, overlapTokens int) []string {
	if overlapTokens == 0 || len(sentences) == 0 {
		return []string{}
	}

	// Count backwards from end to get ~overlapTokens
	totalTokens := 0
	startIdx := len(sentences)

	for i := len(sentences) - 1; i >= 0; i-- {
		tokens := countTokens(sentences[i])
		if totalTokens+tokens > overlapTokens && startIdx != len(sentences) {
			break
		}
		totalTokens += tokens
		startIdx = i
	}

	return sentences[startIdx:]
}

// generateChunkID creates a deterministic ID using content hash and index
func generateChunkID(text string, index int) string {
	hash := sha256.Sum256([]byte(text))
	hashStr := hex.EncodeToString(hash[:8]) // Use first 8 bytes for brevity
	return fmt.Sprintf("%s-%d", hashStr, index)
}
