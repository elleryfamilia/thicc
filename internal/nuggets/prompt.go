package nuggets

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ExtractionPrompt builds the prompt for nugget extraction
func ExtractionPrompt(sessionText string, diff string, existingNuggets []Nugget) string {
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

	// Add existing nuggets for deduplication
	if len(existingNuggets) > 0 {
		sb.WriteString("## Existing Nuggets (do not duplicate)\n\n")
		for _, n := range existingNuggets {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", n.Type, n.Title, n.Summary))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(extractionInstructions)

	return sb.String()
}

const extractionSystemPrompt = `You are an expert at extracting valuable insights from AI coding sessions.

Your job is to identify "nuggets" - valuable knowledge worth preserving for future reference. These are insights that would help a future developer (or AI) understand important decisions, avoid pitfalls, or reuse patterns.`

const extractionInstructions = `## Instructions

Analyze the session transcript and extract any valuable nuggets. Focus on:

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
  "nuggets": [
    {
      "type": "decision|discovery|gotcha|pattern|issue|context",
      "title": "Short title (< 60 chars)",
      "summary": "One-line summary of the insight",
      "significance": "Why this is worth keeping - what makes it non-obvious or valuable",
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

The "significance" field is CRITICAL - it explains why this nugget is worth preserving. Ask yourself:
- Is this something a developer would NOT easily find in documentation?
- Did it require debugging or trial-and-error to discover?
- Would forgetting this cause wasted time in the future?

If you can't articulate a strong significance, don't extract it as a nugget.

Set "incomplete": true if the conversation appears to be in the middle of something and more context is needed.

If there are no valuable nuggets to extract, respond with:
{"nuggets": [], "incomplete": false}

Respond ONLY with the JSON object, no other text.`

// ExtractionResponse represents the expected JSON response from the LLM
type ExtractionResponse struct {
	Nuggets    []ExtractedNugget `json:"nuggets"`
	Incomplete bool              `json:"incomplete"`
}

// ExtractedNugget represents a nugget as extracted from the LLM response
type ExtractedNugget struct {
	Type         string         `json:"type"`
	Title        string         `json:"title"`
	Summary      string         `json:"summary"`
	Significance string         `json:"significance"`
	Tags         []string       `json:"tags"`
	Files        []string       `json:"files"`
	Content      map[string]any `json:"content"`
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

	// Check for NO_NUGGETS response (legacy format)
	if response == "NO_NUGGETS" {
		return &ExtractionResult{
			Nuggets:    []Nugget{},
			Incomplete: false,
		}, nil
	}

	var extracted ExtractionResponse
	if err := json.Unmarshal([]byte(response), &extracted); err != nil {
		return nil, fmt.Errorf("failed to parse extraction response: %w\nResponse was: %s", err, response)
	}

	// Convert extracted nuggets to Nugget structs
	result := &ExtractionResult{
		Nuggets:    make([]Nugget, 0, len(extracted.Nuggets)),
		Incomplete: extracted.Incomplete,
	}

	for _, en := range extracted.Nuggets {
		nugget := Nugget{
			ID:           generateNuggetID(),
			Type:         NuggetType(en.Type),
			Title:        en.Title,
			Summary:      en.Summary,
			Significance: en.Significance,
			Client:       client,
			Model:        model,
			Tags:         en.Tags,
			Files:        en.Files,
			Content:      en.Content,
		}

		// Validate nugget type
		if !nugget.Type.IsValid() {
			// Default to context if invalid
			nugget.Type = NuggetContext
		}

		result.Nuggets = append(result.Nuggets, nugget)
	}

	return result, nil
}
