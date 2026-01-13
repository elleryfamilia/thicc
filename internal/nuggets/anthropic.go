package nuggets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const anthropicAPIURL = "https://api.anthropic.com/v1/messages"
const anthropicAPIVersion = "2023-06-01"

// AnthropicSummarizer implements Summarizer using Anthropic API
type AnthropicSummarizer struct {
	config SummarizerConfig
	client *http.Client
}

// NewAnthropicSummarizer creates a new Anthropic summarizer
func NewAnthropicSummarizer(config SummarizerConfig) (*AnthropicSummarizer, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("Anthropic API key is required")
	}
	if config.Model == "" {
		config.Model = "claude-3-haiku-20240307"
	}

	return &AnthropicSummarizer{
		config: config,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

// Name returns the summarizer type name
func (a *AnthropicSummarizer) Name() string {
	return "anthropic"
}

// anthropicRequest is the request body for Anthropic API
type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse is the response from Anthropic API
type anthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Error      *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Extract implements Summarizer.Extract
func (a *AnthropicSummarizer) Extract(sessionText string, diff string, existingNuggets []Nugget) (*ExtractionResult, error) {
	prompt := ExtractionPrompt(sessionText, diff, existingNuggets)

	reqBody := anthropicRequest{
		Model:     a.config.Model,
		MaxTokens: 4096,
		Messages: []anthropicMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", anthropicAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.config.APIKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Anthropic API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp anthropicResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != nil {
			return nil, &ErrAPIError{
				StatusCode: resp.StatusCode,
				Message:    errResp.Error.Message,
			}
		}
		return nil, &ErrAPIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic response: %w", err)
	}

	// Extract text from response
	if len(anthropicResp.Content) == 0 {
		return nil, fmt.Errorf("empty response from Anthropic")
	}

	var responseText string
	for _, block := range anthropicResp.Content {
		if block.Type == "text" {
			responseText = block.Text
			break
		}
	}

	if responseText == "" {
		return nil, fmt.Errorf("no text content in Anthropic response")
	}

	return ParseExtractionResponse(responseText, "anthropic", a.config.Model)
}
