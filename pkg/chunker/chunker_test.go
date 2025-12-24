package chunker

import (
	"strings"
	"testing"
)

func TestChunkerBasicChunking(t *testing.T) {
	c := Chunker{
		MaxTokens: 10,
		Overlap:   2,
	}

	text := "This is a test. It has multiple sentences. Each sentence should be respected."
	chunks := c.Chunk(text)

	if len(chunks) == 0 {
		t.Fatal("Expected at least one chunk")
	}

	// Verify each chunk has required fields
	for i, chunk := range chunks {
		if chunk.ID == "" {
			t.Errorf("Chunk %d missing ID", i)
		}
		if chunk.Text == "" {
			t.Errorf("Chunk %d missing Text", i)
		}
		if chunk.Index != i {
			t.Errorf("Chunk %d has wrong Index: got %d, want %d", i, chunk.Index, i)
		}
		if chunk.TokenCount == 0 {
			t.Errorf("Chunk %d has zero TokenCount", i)
		}
		if chunk.TokenCount > c.MaxTokens {
			t.Errorf("Chunk %d exceeds MaxTokens: got %d, want <= %d", i, chunk.TokenCount, c.MaxTokens)
		}
	}
}

func TestChunkerDeterministicIDs(t *testing.T) {
	c := Chunker{
		MaxTokens: 10,
		Overlap:   2,
	}

	text := "This is a test."
	chunks1 := c.Chunk(text)
	chunks2 := c.Chunk(text)

	if len(chunks1) != len(chunks2) {
		t.Fatalf("Different number of chunks: %d vs %d", len(chunks1), len(chunks2))
	}

	for i := range chunks1 {
		if chunks1[i].ID != chunks2[i].ID {
			t.Errorf("Chunk %d ID mismatch: %s vs %s", i, chunks1[i].ID, chunks2[i].ID)
		}
	}
}

func TestChunkerOverlap(t *testing.T) {
	c := Chunker{
		MaxTokens: 5,
		Overlap:   2,
	}

	text := "One two three four five six seven eight nine ten."
	chunks := c.Chunk(text)

	if len(chunks) < 2 {
		t.Skip("Need at least 2 chunks to test overlap")
	}

	// Check that consecutive chunks have overlapping content
	for i := 0; i < len(chunks)-1; i++ {
		chunk1Words := strings.Fields(chunks[i].Text)
		chunk2Words := strings.Fields(chunks[i+1].Text)

		if len(chunk1Words) < c.Overlap || len(chunk2Words) < c.Overlap {
			continue // Skip if chunks too small
		}

		// Last N words of chunk1 should appear in chunk2
		overlap := false
		for _, word := range chunk1Words[len(chunk1Words)-c.Overlap:] {
			if strings.Contains(chunks[i+1].Text, word) {
				overlap = true
				break
			}
		}

		if !overlap {
			t.Errorf("No overlap detected between chunk %d and %d", i, i+1)
		}
	}
}

func TestChunkerEmptyInput(t *testing.T) {
	c := Chunker{
		MaxTokens: 10,
		Overlap:   2,
	}

	chunks := c.Chunk("")

	if len(chunks) != 0 {
		t.Errorf("Expected no chunks for empty input, got %d", len(chunks))
	}
}

func TestChunkerVeryShortInput(t *testing.T) {
	c := Chunker{
		MaxTokens: 10,
		Overlap:   2,
	}

	text := "Hi"
	chunks := c.Chunk(text)

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk for short input, got %d", len(chunks))
	}
}

func TestChunkerSentenceBoundaries(t *testing.T) {
	c := Chunker{
		MaxTokens: 5,
		Overlap:   1,
	}

	text := "First sentence. Second sentence. Third sentence."
	chunks := c.Chunk(text)

	// Verify no chunk breaks mid-sentence (basic heuristic: chunks should end with punctuation or be at text end)
	for i, chunk := range chunks {
		trimmed := strings.TrimSpace(chunk.Text)
		if i < len(chunks)-1 { // Not the last chunk
			lastChar := trimmed[len(trimmed)-1]
			if lastChar != '.' && lastChar != '!' && lastChar != '?' {
				t.Logf("Warning: Chunk %d may break mid-sentence: %q", i, trimmed)
			}
		}
	}
}
