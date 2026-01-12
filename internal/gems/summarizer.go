package gems

import (
	"errors"
	"fmt"
)

// SummarizerType represents the type of summarizer backend
type SummarizerType string

const (
	SummarizerOllama    SummarizerType = "ollama"
	SummarizerAnthropic SummarizerType = "anthropic"
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

// ExtractionResult contains the result of gem extraction
type ExtractionResult struct {
	Gems       []Gem // Extracted gems
	Incomplete bool  // True if conversation seems unfinished
}

// Summarizer is the interface for LLM-based gem extraction
type Summarizer interface {
	// Extract analyzes session text and returns extracted gems
	// sessionText: The AI session transcript
	// diff: Git diff of changes made (optional, can be empty)
	// existingGems: Previously extracted gems (for deduplication)
	Extract(sessionText string, diff string, existingGems []Gem) (*ExtractionResult, error)

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
	default:
		return nil, fmt.Errorf("unknown summarizer type: %s", config.Type)
	}
}

// ErrNoGems is returned when extraction finds no valuable gems
var ErrNoGems = errors.New("no gems found in session")

// ErrAPIError is returned when the LLM API returns an error
type ErrAPIError struct {
	StatusCode int
	Message    string
}

func (e *ErrAPIError) Error() string {
	return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.Message)
}
