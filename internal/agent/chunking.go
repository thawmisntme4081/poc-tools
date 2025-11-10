package agent

import (
	"errors"
	"strings"
)

type Chunking struct {
	// chunkSize is the size of the chunk
	chunkSize int
	// overlap is the overlap between chunks
	overlap int
}

// NewChunking creates a new Chunking instance with validation
func NewChunking(chunkSize, overlap int) (*Chunking, error) {
	if chunkSize <= 0 {
		return nil, errors.New("chunk_size must be > 0")
	}
	if overlap < 0 {
		return nil, errors.New("overlap must be >= 0")
	}
	if overlap >= chunkSize {
		return nil, errors.New("overlap must be < chunk_size")
	}
	return &Chunking{
		chunkSize: chunkSize,
		overlap:   overlap,
	}, nil
}

func (c *Chunking) FixedSizeChunking(text string) []string {
	step := c.chunkSize - c.overlap
	if step <= 0 {
		return nil
	}

	var chunks []string
	for i := 0; i < len(text); i += step {
		end := min(i+c.chunkSize, len(text))
		chunks = append(chunks, text[i:end])
	}
	return chunks
}

func (c *Chunking) FixedSizeByWordChunking(text string) []string {
	// Split text into words
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	step := c.chunkSize - c.overlap
	if step <= 0 {
		return nil
	}

	var chunks []string
	for i := 0; i < len(words); i += step {
		end := min(i+c.chunkSize, len(words))
		chunkWords := words[i:end]
		chunks = append(chunks, strings.Join(chunkWords, " "))
	}
	return chunks
}

func (c *Chunking) FixedSizeChunkingWithTokenization(text string) []string {
	tokens := strings.Fields(text)
	step := c.chunkSize - c.overlap
	if step <= 0 {
		return nil
	}

	var chunks []string
	for i := 0; i < len(tokens); i += step {
		end := min(i+c.chunkSize, len(tokens))
		chunkTokens := tokens[i:end]
		chunks = append(chunks, strings.Join(chunkTokens, " "))
	}
	return chunks
}

func (c *Chunking) RecursiveChunking(text string) []string {
	var chunks []string

	appendChunk := func(s string) {
		if s == "" {
			return
		}
		chunks = append(chunks, s)
	}

	// Helper function to add overlap between chunks
	addOverlapToChunks := func() {
		if c.overlap == 0 || len(chunks) <= 1 {
			return
		}

		var overlappedChunks []string
		for i := 0; i < len(chunks); i++ {
			currentChunk := chunks[i]

			// Add overlap from previous chunk
			if i > 0 {
				prevChunk := chunks[i-1]
				if len(prevChunk) >= c.overlap {
					overlapText := prevChunk[len(prevChunk)-c.overlap:]
					currentChunk = overlapText + " " + currentChunk
				}
			}

			// Add overlap from next chunk
			if i < len(chunks)-1 {
				nextChunk := chunks[i+1]
				if len(nextChunk) >= c.overlap {
					overlapText := nextChunk[:c.overlap]
					currentChunk = currentChunk + " " + overlapText
				}
			}

			overlappedChunks = append(overlappedChunks, currentChunk)
		}
		chunks = overlappedChunks
	}

	packWords := func(words []string) {
		current := ""
		for _, w := range words {
			if current == "" {
				if len(w) <= c.chunkSize {
					current = w
				} else {
					for start := 0; start < len(w); start += c.chunkSize {
						end := min(start+c.chunkSize, len(w))
						appendChunk(w[start:end])
					}
					current = ""
				}
				continue
			}
			if len(current)+1+len(w) <= c.chunkSize {
				current += " " + w
			} else {
				appendChunk(current)
				if len(w) <= c.chunkSize {
					current = w
				} else {
					for start := 0; start < len(w); start += c.chunkSize {
						end := min(start+c.chunkSize, len(w))
						appendChunk(w[start:end])
					}
					current = ""
				}
			}
		}
		if current != "" {
			appendChunk(current)
		}
	}

	// "\n\n" - Double new line, commonly indicating paragraph breaks
	paragraphs := strings.Split(text, "\n\n")
	for _, p := range paragraphs {
		if p == "" {
			continue
		}
		if len(p) <= c.chunkSize {
			appendChunk(p)
			continue
		}

		// "\n" - Single new line, often used for line breaks
		lines := strings.Split(p, "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			if len(line) <= c.chunkSize {
				appendChunk(line)
				continue
			}

			// "." - Period, commonly used for sentence breaks
			raw := strings.Split(line, ".")
			sentences := make([]string, 0, len(raw))
			for i, s := range raw {
				if s == "" {
					continue
				}

				if i < len(raw)-1 || strings.HasSuffix(line, ".") {
					sentences = append(sentences, s+".")
				} else {
					sentences = append(sentences, s)
				}
			}

			for _, s := range sentences {
				if len(s) <= c.chunkSize {
					appendChunk(s)
					continue
				}
				// " " - Words
				words := strings.Fields(s)
				if len(words) == 0 {
					for start := 0; start < len(s); start += c.chunkSize {
						end := min(start+c.chunkSize, len(s))
						appendChunk(s[start:end])
					}
					continue
				}
				packWords(words)
			}
		}
	}

	// Apply overlap after all chunks are created
	addOverlapToChunks()

	return chunks
}

func (c *Chunking) RecursiveChunkingWithTokenization(text string) []string {
	var chunks []string
	return chunks
}

func (c *Chunking) SemanticChunking(text string) {
	// Implement the logic for SemanticChunking
}

func (c *Chunking) AgenticChunking(text string) {
	// Implement the logic for AgenticChunking
}
