package nuggets

import (
	"errors"
	"fmt"
)

// SummarizerType represents the type of summarizer backend
type SummarizerType string

const (
	SummarizerOllama    SummarizerType = "ollama"
	SummarizerAnthropic SummarizerType = "anthropic"
	SummarizerOpenAI    SummarizerType = "openai" // OpenAI-compatible API (works with many providers)
)

// SummarizerConfig holds configuration for the summarizer
type SummarizerConfig struct {
	Type   SummarizerType // "ollama" or "anthropic"
	Model  string         // Model name (e.g., "llama3.2", "claude-3-haiku-20240307")
	APIKey string         // API key (for Anthropic)
	Host   string         // API host (for Ollama, defaults to localhost:11434)
}

// DefaultOllamaConfig returns default config for Ollama
func DefaultOllamaConfig() SummarizerConfig {
	return SummarizerConfig{
		Type:  SummarizerOllama,
		Model: "llama3.2",
		Host:  "http://localhost:11434",
	}
}

// DefaultAnthropicConfig returns default config for Anthropic
func DefaultAnthropicConfig(apiKey string) SummarizerConfig {
	return SummarizerConfig{
		Type:   SummarizerAnthropic,
		Model:  "claude-3-haiku-20240307",
		APIKey: apiKey,
	}
}

// ExtractionResult contains the result of nugget extraction
type ExtractionResult struct {
	Nuggets    []Nugget // Extracted nuggets
	Incomplete bool     // True if conversation seems unfinished
}

// Summarizer is the interface for LLM-based nugget extraction
type Summarizer interface {
	// Extract analyzes session text and returns extracted nuggets
	// sessionText: The AI session transcript
	// diff: Git diff of changes made (optional, can be empty)
	// existingNuggets: Previously extracted nuggets (for deduplication)
	Extract(sessionText string, diff string, existingNuggets []Nugget) (*ExtractionResult, error)

	// Name returns the summarizer type name
	Name() string
}

// NewSummarizer creates a new summarizer based on the config
func NewSummarizer(config SummarizerConfig) (Summarizer, error) {
	switch config.Type {
	case SummarizerOllama:
		return NewOllamaSummarizer(config)
	case SummarizerAnthropic:
		return NewAnthropicSummarizer(config)
	case SummarizerOpenAI:
		return NewOpenAISummarizer(config)
	default:
		return nil, fmt.Errorf("unknown summarizer type: %s", config.Type)
	}
}

// ErrNoNuggets is returned when extraction finds no valuable nuggets
var ErrNoNuggets = errors.New("no nuggets found in session")

// ErrAPIError is returned when the LLM API returns an error
type ErrAPIError struct {
	StatusCode int
	Message    string
}

func (e *ErrAPIError) Error() string {
	return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.Message)
}
