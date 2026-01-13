package nuggets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaSummarizer implements Summarizer using Ollama API
type OllamaSummarizer struct {
	config SummarizerConfig
	client *http.Client
}

// NewOllamaSummarizer creates a new Ollama summarizer
func NewOllamaSummarizer(config SummarizerConfig) (*OllamaSummarizer, error) {
	if config.Host == "" {
		config.Host = "http://localhost:11434"
	}
	if config.Model == "" {
		config.Model = "llama3.2"
	}

	return &OllamaSummarizer{
		config: config,
		client: &http.Client{
			Timeout: 120 * time.Second, // LLM responses can be slow
		},
	}, nil
}

// Name returns the summarizer type name
func (o *OllamaSummarizer) Name() string {
	return "ollama"
}

// ollamaRequest is the request body for Ollama API
type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Format string `json:"format,omitempty"`
}

// ollamaResponse is the response from Ollama API
type ollamaResponse struct {
	Model      string `json:"model"`
	Response   string `json:"response"`
	Done       bool   `json:"done"`
	DoneReason string `json:"done_reason,omitempty"`
}

// Extract implements Summarizer.Extract
func (o *OllamaSummarizer) Extract(sessionText string, diff string, existingNuggets []Nugget) (*ExtractionResult, error) {
	prompt := ExtractionPrompt(sessionText, diff, existingNuggets)

	reqBody := ollamaRequest{
		Model:  o.config.Model,
		Prompt: prompt,
		Stream: false,
		Format: "json",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", o.config.Host)
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &ErrAPIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var ollamaResp ollamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to parse Ollama response: %w", err)
	}

	return ParseExtractionResponse(ollamaResp.Response, "ollama", o.config.Model)
}

// CheckOllamaAvailable checks if Ollama is running and the model is available
func CheckOllamaAvailable(host string, model string) error {
	if host == "" {
		host = "http://localhost:11434"
	}

	client := &http.Client{Timeout: 5 * time.Second}

	// Check if Ollama is running
	resp, err := client.Get(host + "/api/tags")
	if err != nil {
		return fmt.Errorf("Ollama is not running at %s: %w", host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama returned status %d", resp.StatusCode)
	}

	// Parse available models
	var tagsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if err := json.Unmarshal(body, &tagsResp); err != nil {
		return fmt.Errorf("failed to parse models: %w", err)
	}

	// Check if requested model is available
	for _, m := range tagsResp.Models {
		// Ollama model names can have :tag suffix
		if m.Name == model || m.Name == model+":latest" {
			return nil
		}
	}

	var available []string
	for _, m := range tagsResp.Models {
		available = append(available, m.Name)
	}

	return fmt.Errorf("model %q not found. Available models: %v", model, available)
}
