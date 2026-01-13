package nuggets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Common OpenAI-compatible API endpoints
const (
	OpenAIBaseURL    = "https://api.openai.com/v1"
	TogetherBaseURL  = "https://api.together.xyz/v1"
	GroqBaseURL      = "https://api.groq.com/openai/v1"
	FireworksBaseURL = "https://api.fireworks.ai/inference/v1"
	OpenRouterURL    = "https://openrouter.ai/api/v1"
)

// OpenAISummarizer implements Summarizer using OpenAI-compatible API
// Works with OpenAI, Together, Groq, Fireworks, OpenRouter, LM Studio, etc.
type OpenAISummarizer struct {
	config SummarizerConfig
	client *http.Client
}

// NewOpenAISummarizer creates a new OpenAI-compatible summarizer
func NewOpenAISummarizer(config SummarizerConfig) (*OpenAISummarizer, error) {
	if config.APIKey == "" {
		// Try environment variables
		config.APIKey = os.Getenv("OPENAI_API_KEY")
		if config.APIKey == "" {
			config.APIKey = os.Getenv("LLM_API_KEY")
		}
	}
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key required (set via --api-key, OPENAI_API_KEY, or LLM_API_KEY)")
	}

	if config.Host == "" {
		config.Host = OpenAIBaseURL
	}

	if config.Model == "" {
		config.Model = "gpt-4o-mini"
	}

	return &OpenAISummarizer{
		config: config,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

// Name returns the summarizer type name
func (o *OpenAISummarizer) Name() string {
	return "openai"
}

// openaiRequest is the request body for OpenAI-compatible API
type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openaiResponse is the response from OpenAI-compatible API
type openaiResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// Extract implements Summarizer.Extract
func (o *OpenAISummarizer) Extract(sessionText string, diff string, existingNuggets []Nugget) (*ExtractionResult, error) {
	prompt := ExtractionPrompt(sessionText, diff, existingNuggets)

	reqBody := openaiRequest{
		Model: o.config.Model,
		Messages: []openaiMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   4096,
		Temperature: 0.1, // Low temperature for consistent JSON output
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", o.config.Host)
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.config.APIKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp openaiResponse
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

	var openaiResp openaiResponse
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}

	responseText := openaiResp.Choices[0].Message.Content
	if responseText == "" {
		return nil, fmt.Errorf("no content in API response")
	}

	return ParseExtractionResponse(responseText, "openai", o.config.Model)
}

// Preset configurations for popular providers
var (
	// OpenAI presets (GPT-5 series - latest API models)
	PresetOpenAIGPT5Mini = SummarizerConfig{
		Type:  SummarizerOpenAI,
		Host:  OpenAIBaseURL,
		Model: "gpt-5-mini", // Fast, cheap, good for extraction
	}
	PresetOpenAIGPT5 = SummarizerConfig{
		Type:  SummarizerOpenAI,
		Host:  OpenAIBaseURL,
		Model: "gpt-5.2", // Latest GPT-5.2
	}

	// Together AI presets (Llama 4, DeepSeek, Qwen)
	PresetTogetherLlama4 = SummarizerConfig{
		Type:  SummarizerOpenAI,
		Host:  TogetherBaseURL,
		Model: "meta-llama/Llama-4-Maverick-17B-128E-Instruct-FP8", // Latest Llama 4
	}

	// Groq presets (fast inference - Llama 4)
	PresetGroqLlama4 = SummarizerConfig{
		Type:  SummarizerOpenAI,
		Host:  GroqBaseURL,
		Model: "meta-llama/llama-4-scout-17b-16e-instruct", // Llama 4 Scout - fast
	}

	// OpenRouter (model aggregator - Llama 4)
	PresetOpenRouter = SummarizerConfig{
		Type:  SummarizerOpenAI,
		Host:  OpenRouterURL,
		Model: "meta-llama/llama-4-maverick", // Llama 4 via OpenRouter
	}
)

// GetProviderPreset returns a preset config for a known provider
func GetProviderPreset(provider string) (SummarizerConfig, bool) {
	presets := map[string]SummarizerConfig{
		"openai":     PresetOpenAIGPT5Mini, // gpt-5-mini (fast/cheap)
		"openai-5":   PresetOpenAIGPT5,     // gpt-5.2 (latest)
		"together":   PresetTogetherLlama4, // Llama 4 Maverick
		"groq":       PresetGroqLlama4,     // Llama 4 Scout (fast)
		"openrouter": PresetOpenRouter,     // Llama 4 Maverick
	}
	cfg, ok := presets[provider]
	return cfg, ok
}
