package gems

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ExtractionPrompt builds the prompt for gem extraction
func ExtractionPrompt(sessionText string, diff string, existingGems []Gem) string {
	var sb strings.Builder

	sb.WriteString(extractionSystemPrompt)
	sb.WriteString("\n\n")

	// Add session transcript
	sb.WriteString("## Session Transcript\n\n")
	sb.WriteString("```\n")
	sb.WriteString(sessionText)
	sb.WriteString("\n```\n\n")

	// Add git diff if provided
	if diff != "" {
		sb.WriteString("## Files Changed (git diff)\n\n")
		sb.WriteString("```diff\n")
		sb.WriteString(diff)
		sb.WriteString("\n```\n\n")
	}

	// Add existing gems for deduplication
	if len(existingGems) > 0 {
		sb.WriteString("## Existing Gems (do not duplicate)\n\n")
		for _, g := range existingGems {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", g.Type, g.Title, g.Summary))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(extractionInstructions)

	return sb.String()
}

const extractionSystemPrompt = `You are an expert at extracting valuable insights from AI coding sessions.

Your job is to identify "gems" - valuable knowledge worth preserving for future reference. These are insights that would help a future developer (or AI) understand important decisions, avoid pitfalls, or reuse patterns.`

const extractionInstructions = `## Instructions

Analyze the session transcript and extract any valuable gems. Focus on:

1. **Decisions** - Architectural or design choices with lasting impact
   - Why was this approach chosen over alternatives?
   - What trade-offs were considered?

2. **Discoveries** - Unexpected findings during development
   - Surprising behavior or limitations
   - Undocumented features or quirks

3. **Gotchas** - Non-obvious pitfalls or edge cases
   - Things that could trip up future developers
   - Subtle bugs or issues encountered

4. **Patterns** - Reusable solutions or approaches
   - Code patterns worth remembering
   - Best practices established

5. **Issues** - Bugs encountered and how they were resolved
   - Root cause analysis
   - Fix or workaround applied

6. **Context** - Important background info for understanding code
   - Why something exists
   - Historical context that isn't obvious from code

## What NOT to Extract

Skip mundane interactions:
- Simple syntax questions
- Routine code changes (typo fixes, formatting)
- Standard library/framework usage with no novel insight
- Temporary debugging steps

## Output Format

Respond with a JSON object in this exact format:

{
  "gems": [
    {
      "type": "decision|discovery|gotcha|pattern|issue|context",
      "title": "Short title (< 60 chars)",
      "summary": "One-line summary of the insight",
      "tags": ["tag1", "tag2"],
      "files": ["path/to/file.go"],
      "content": {
        "rationale": ["reason 1", "reason 2"],
        "gotchas": ["gotcha 1"],
        "implementation": ["detail 1"]
      }
    }
  ],
  "incomplete": false
}

Set "incomplete": true if the conversation appears to be in the middle of something and more context is needed.

If there are no valuable gems to extract, respond with:
{"gems": [], "incomplete": false}

Respond ONLY with the JSON object, no other text.`

// ExtractionResponse represents the expected JSON response from the LLM
type ExtractionResponse struct {
	Gems       []ExtractedGem `json:"gems"`
	Incomplete bool           `json:"incomplete"`
}

// ExtractedGem represents a gem as extracted from the LLM response
type ExtractedGem struct {
	Type    string         `json:"type"`
	Title   string         `json:"title"`
	Summary string         `json:"summary"`
	Tags    []string       `json:"tags"`
	Files   []string       `json:"files"`
	Content map[string]any `json:"content"`
}

// ParseExtractionResponse parses the LLM response into an ExtractionResult
func ParseExtractionResponse(response string, client string, model string) (*ExtractionResult, error) {
	// Clean up response - remove markdown code blocks if present
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	}

	// Check for NO_GEMS response (legacy format)
	if response == "NO_GEMS" {
		return &ExtractionResult{
			Gems:       []Gem{},
			Incomplete: false,
		}, nil
	}

	var extracted ExtractionResponse
	if err := json.Unmarshal([]byte(response), &extracted); err != nil {
		return nil, fmt.Errorf("failed to parse extraction response: %w\nResponse was: %s", err, response)
	}

	// Convert extracted gems to Gem structs
	result := &ExtractionResult{
		Gems:       make([]Gem, 0, len(extracted.Gems)),
		Incomplete: extracted.Incomplete,
	}

	for _, eg := range extracted.Gems {
		gem := Gem{
			ID:      generateGemID(),
			Type:    GemType(eg.Type),
			Title:   eg.Title,
			Summary: eg.Summary,
			Client:  client,
			Model:   model,
			Tags:    eg.Tags,
			Files:   eg.Files,
			Content: eg.Content,
		}

		// Validate gem type
		if !gem.Type.IsValid() {
			// Default to context if invalid
			gem.Type = GemContext
		}

		result.Gems = append(result.Gems, gem)
	}

	return result, nil
}
