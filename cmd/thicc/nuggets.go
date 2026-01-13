package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ellery/thicc/internal/nuggets"
)

// handleNuggetCommand handles the "nugget" subcommand
// Returns true if the command was handled (and program should exit)
func handleNuggetCommand(args []string) bool {
	if len(args) < 2 || args[1] != "nugget" {
		return false
	}

	// Parse nugget subcommand
	if len(args) < 3 {
		printNuggetUsage()
		os.Exit(1)
	}

	switch args[2] {
	case "list":
		listNuggets()
	case "pending":
		listPendingNuggets()
	case "show":
		if len(args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: thicc nugget show <id>")
			os.Exit(1)
		}
		showNugget(args[3])
	case "accept":
		if len(args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: thicc nugget accept <id>")
			os.Exit(1)
		}
		acceptNugget(args[3])
	case "reject":
		if len(args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: thicc nugget reject <id>")
			os.Exit(1)
		}
		rejectNugget(args[3])
	case "search":
		if len(args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: thicc nugget search <query>")
			os.Exit(1)
		}
		searchNuggetsCmd(args[3])
	case "add":
		addNuggetInteractive()
	case "extract":
		extractNuggets(args[3:])
	case "help", "-h", "--help":
		printNuggetUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown nugget command: %s\n", args[2])
		printNuggetUsage()
		os.Exit(1)
	}

	return true
}

// printNuggetUsage prints usage information for the nugget subcommand
func printNuggetUsage() {
	fmt.Println("Usage: thicc nugget <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  list               List committed nuggets")
	fmt.Println("  pending            List pending nuggets awaiting review")
	fmt.Println("  show <id>          Show nugget details")
	fmt.Println("  accept <id>        Accept a pending nugget (move to committed)")
	fmt.Println("  reject <id>        Reject a pending nugget (delete)")
	fmt.Println("  search <query>     Search nuggets by text")
	fmt.Println("  add                Manually add a nugget (interactive)")
	fmt.Println("  extract [options]  Extract nuggets from a session file")
	fmt.Println("  help               Show this help message")
	fmt.Println("")
	fmt.Println("Extract options:")
	fmt.Println("  --file <path>      Session file to extract from (required)")
	fmt.Println("  --summarizer       Summarizer: ollama (default), anthropic, openai")
	fmt.Println("  --provider         Provider preset: openai, openai-5, together, groq, openrouter")
	fmt.Println("  --model            Model name (default varies by provider)")
	fmt.Println("  --host             API endpoint URL (for custom deployments)")
	fmt.Println("  --api-key          API key (or set OPENAI_API_KEY, ANTHROPIC_API_KEY)")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  thicc nugget extract --file session.txt                     # Ollama (local)")
	fmt.Println("  thicc nugget extract --file session.txt --provider openai   # OpenAI")
	fmt.Println("  thicc nugget extract --file session.txt --provider groq     # Groq (fast)")
	fmt.Println("  thicc nugget extract --file session.txt --provider together # Together AI")
	fmt.Println("")
	fmt.Println("Nuggets are valuable insights captured from AI coding sessions:")
	fmt.Println("  - Architectural decisions and their rationale")
	fmt.Println("  - Non-obvious gotchas and edge cases")
	fmt.Println("  - Reusable patterns and solutions")
	fmt.Println("  - Bug resolutions and their causes")
	fmt.Println("")
	fmt.Println("Committed nuggets are stored in .agent-nuggets.json (version controlled)")
	fmt.Println("Pending nuggets are stored in .agent-history/pending-nuggets.json (local)")
}

// openNuggetStore opens the nugget store for CLI commands
func openNuggetStore() *nuggets.Store {
	store, err := nuggets.NewStoreFromCwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open nugget store: %v\n", err)
		os.Exit(1)
	}
	return store
}

// listNuggets lists all committed nuggets
func listNuggets() {
	store := openNuggetStore()

	nuggetFile, err := store.LoadNuggets()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load nuggets: %v\n", err)
		os.Exit(1)
	}

	if len(nuggetFile.Nuggets) == 0 {
		fmt.Println("No committed nuggets found.")
		fmt.Println("")
		fmt.Println("Use 'thicc nugget add' to manually add a nugget,")
		fmt.Println("or nuggets will be auto-extracted from AI sessions.")
		return
	}

	fmt.Printf("Found %d committed nuggets:\n\n", len(nuggetFile.Nuggets))
	for _, n := range nuggetFile.Nuggets {
		printNuggetSummary(n)
	}
}

// listPendingNuggets lists pending nuggets awaiting review
func listPendingNuggets() {
	store := openNuggetStore()

	pending, err := store.LoadPendingNuggets()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load pending nuggets: %v\n", err)
		os.Exit(1)
	}

	if len(pending.Nuggets) == 0 {
		fmt.Println("No pending nuggets.")
		return
	}

	fmt.Printf("Found %d pending nuggets awaiting review:\n\n", len(pending.Nuggets))
	for _, n := range pending.Nuggets {
		printNuggetSummary(n)
	}
	fmt.Println("")
	fmt.Println("Use 'thicc nugget accept <id>' to accept a nugget")
	fmt.Println("Use 'thicc nugget reject <id>' to reject a nugget")
}

// printNuggetSummary prints a one-line summary of a nugget
func printNuggetSummary(n nuggets.Nugget) {
	id := n.ID
	if len(id) > 12 {
		id = id[:12]
	}
	fmt.Printf("  %s  [%s]  %s\n", id, n.Type, n.Title)
	fmt.Printf("           %s\n", n.Summary)
	if n.Significance != "" {
		fmt.Printf("           Why: %s\n", n.Significance)
	}
	if len(n.Tags) > 0 {
		fmt.Printf("           Tags: %s\n", strings.Join(n.Tags, ", "))
	}
	fmt.Println()
}

// showNugget displays detailed information about a nugget
func showNugget(id string) {
	store := openNuggetStore()

	nugget, isPending, err := store.GetNugget(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Nugget not found: %s\n", id)
		os.Exit(1)
	}

	status := "committed"
	if isPending {
		status = "pending"
	}

	fmt.Printf("Nugget: %s (%s)\n", nugget.ID, status)
	fmt.Printf("Type: %s\n", nugget.Type)
	fmt.Printf("Title: %s\n", nugget.Title)
	fmt.Printf("Summary: %s\n", nugget.Summary)
	if nugget.Significance != "" {
		fmt.Printf("Significance: %s\n", nugget.Significance)
	}
	fmt.Printf("Created: %s\n", nugget.Created.Format("2006-01-02 15:04:05"))
	if nugget.Commit != "" {
		fmt.Printf("Commit: %s\n", nugget.Commit)
	}
	if nugget.Client != "" {
		fmt.Printf("Client: %s\n", nugget.Client)
	}
	if nugget.Model != "" {
		fmt.Printf("Model: %s\n", nugget.Model)
	}
	if len(nugget.Tags) > 0 {
		fmt.Printf("Tags: %s\n", strings.Join(nugget.Tags, ", "))
	}
	if len(nugget.Files) > 0 {
		fmt.Printf("Files: %s\n", strings.Join(nugget.Files, ", "))
	}
	fmt.Println()

	// Print content fields
	if len(nugget.Content) > 0 {
		fmt.Println("Content:")
		for key, value := range nugget.Content {
			fmt.Printf("  %s:\n", key)
			switch v := value.(type) {
			case []interface{}:
				for _, item := range v {
					fmt.Printf("    - %v\n", item)
				}
			case string:
				fmt.Printf("    %s\n", v)
			default:
				fmt.Printf("    %v\n", v)
			}
		}
	}

	if nugget.UserNotes != "" {
		fmt.Println()
		fmt.Printf("User Notes: %s\n", nugget.UserNotes)
	}
}

// acceptNugget accepts a pending nugget
func acceptNugget(id string) {
	store := openNuggetStore()

	if err := store.AcceptNugget(id); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to accept nugget: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Nugget %s accepted and moved to .agent-nuggets.json\n", id)
}

// rejectNugget rejects a pending nugget
func rejectNugget(id string) {
	store := openNuggetStore()

	if err := store.RejectNugget(id); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to reject nugget: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Nugget %s rejected and removed from pending list\n", id)
}

// searchNuggetsCmd searches nuggets by query
func searchNuggetsCmd(query string) {
	store := openNuggetStore()

	results, err := store.SearchNuggets(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Search failed: %v\n", err)
		os.Exit(1)
	}

	if len(results) == 0 {
		fmt.Printf("No nuggets found matching: %s\n", query)
		return
	}

	fmt.Printf("Found %d nuggets matching '%s':\n\n", len(results), query)
	for _, n := range results {
		printNuggetSummary(n)
	}
}

// addNuggetInteractive adds a nugget interactively via prompts
func addNuggetInteractive() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Add a new nugget")
	fmt.Println("================")
	fmt.Println()

	// Select type
	fmt.Println("Nugget types:")
	for i, t := range nuggets.AllNuggetTypes() {
		fmt.Printf("  %d. %s - %s\n", i+1, t, t.Description())
	}
	fmt.Print("\nSelect type (1-6): ")
	typeInput, _ := reader.ReadString('\n')
	typeInput = strings.TrimSpace(typeInput)
	var nuggetType nuggets.NuggetType
	switch typeInput {
	case "1":
		nuggetType = nuggets.NuggetDecision
	case "2":
		nuggetType = nuggets.NuggetDiscovery
	case "3":
		nuggetType = nuggets.NuggetGotcha
	case "4":
		nuggetType = nuggets.NuggetPattern
	case "5":
		nuggetType = nuggets.NuggetIssue
	case "6":
		nuggetType = nuggets.NuggetContext
	default:
		fmt.Fprintln(os.Stderr, "Invalid type selection")
		os.Exit(1)
	}

	// Title
	fmt.Print("Title (short, < 60 chars): ")
	title, _ := reader.ReadString('\n')
	title = strings.TrimSpace(title)
	if title == "" {
		fmt.Fprintln(os.Stderr, "Title is required")
		os.Exit(1)
	}

	// Summary
	fmt.Print("Summary (one line): ")
	summary, _ := reader.ReadString('\n')
	summary = strings.TrimSpace(summary)
	if summary == "" {
		fmt.Fprintln(os.Stderr, "Summary is required")
		os.Exit(1)
	}

	// Tags (optional)
	fmt.Print("Tags (comma-separated, optional): ")
	tagsInput, _ := reader.ReadString('\n')
	tagsInput = strings.TrimSpace(tagsInput)
	var tags []string
	if tagsInput != "" {
		for _, tag := range strings.Split(tagsInput, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}

	// Files (optional)
	fmt.Print("Related files (comma-separated, optional): ")
	filesInput, _ := reader.ReadString('\n')
	filesInput = strings.TrimSpace(filesInput)
	var files []string
	if filesInput != "" {
		for _, file := range strings.Split(filesInput, ",") {
			file = strings.TrimSpace(file)
			if file != "" {
				files = append(files, file)
			}
		}
	}

	// User notes (optional)
	fmt.Print("Notes (optional): ")
	notes, _ := reader.ReadString('\n')
	notes = strings.TrimSpace(notes)

	// Create the nugget
	nugget := &nuggets.Nugget{
		Type:      nuggetType,
		Title:     title,
		Summary:   summary,
		Created:   time.Now(),
		Client:    "thicc-cli",
		Tags:      tags,
		Files:     files,
		Content:   make(map[string]any),
		UserNotes: notes,
	}

	// Save directly to committed nuggets
	store := openNuggetStore()
	if err := store.AddNugget(nugget); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add nugget: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("Nugget added: %s\n", nugget.ID)
	fmt.Printf("Saved to: %s\n", store.GetProjectRoot()+"/.agent-nuggets.json")
}

// extractNuggets extracts nuggets from a session file using an LLM summarizer
func extractNuggets(args []string) {
	var sessionFile string
	var summarizerType string = "ollama"
	var model string
	var apiKey string
	var host string
	var provider string

	// Parse arguments
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--file", "-f":
			if i+1 < len(args) {
				sessionFile = args[i+1]
				i++
			}
		case "--summarizer", "-s":
			if i+1 < len(args) {
				summarizerType = args[i+1]
				i++
			}
		case "--model", "-m":
			if i+1 < len(args) {
				model = args[i+1]
				i++
			}
		case "--api-key", "-k":
			if i+1 < len(args) {
				apiKey = args[i+1]
				i++
			}
		case "--host", "-h":
			if i+1 < len(args) {
				host = args[i+1]
				i++
			}
		case "--provider", "-p":
			if i+1 < len(args) {
				provider = args[i+1]
				i++
			}
		default:
			// If no flag, treat as file path
			if sessionFile == "" && !strings.HasPrefix(args[i], "-") {
				sessionFile = args[i]
			}
		}
	}

	if sessionFile == "" {
		fmt.Fprintln(os.Stderr, "Error: session file is required")
		fmt.Fprintln(os.Stderr, "Usage: thicc nugget extract --file <path>")
		os.Exit(1)
	}

	// Read session file
	sessionData, err := os.ReadFile(sessionFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read session file: %v\n", err)
		os.Exit(1)
	}
	sessionText := string(sessionData)

	// Configure summarizer
	var config nuggets.SummarizerConfig

	// Check for provider preset first
	if provider != "" {
		preset, ok := nuggets.GetProviderPreset(provider)
		if !ok {
			fmt.Fprintf(os.Stderr, "Unknown provider: %s\n", provider)
			fmt.Fprintln(os.Stderr, "Available: openai, openai-5, together, groq, openrouter")
			os.Exit(1)
		}
		config = preset
		config.APIKey = apiKey
		if model != "" {
			config.Model = model
		}
		if host != "" {
			config.Host = host
		}
	} else {
		switch summarizerType {
		case "ollama":
			config = nuggets.DefaultOllamaConfig()
			if model != "" {
				config.Model = model
			}
			if host != "" {
				config.Host = host
			}
			// Check if Ollama is available
			if err := nuggets.CheckOllamaAvailable(config.Host, config.Model); err != nil {
				fmt.Fprintf(os.Stderr, "Ollama not available: %v\n", err)
				os.Exit(1)
			}
		case "anthropic":
			if apiKey == "" {
				apiKey = os.Getenv("ANTHROPIC_API_KEY")
			}
			if apiKey == "" {
				fmt.Fprintln(os.Stderr, "Error: Anthropic API key required (--api-key or ANTHROPIC_API_KEY)")
				os.Exit(1)
			}
			config = nuggets.DefaultAnthropicConfig(apiKey)
			if model != "" {
				config.Model = model
			}
		case "openai":
			config = nuggets.SummarizerConfig{
				Type:   nuggets.SummarizerOpenAI,
				APIKey: apiKey,
				Model:  model,
				Host:   host,
			}
			if config.Model == "" {
				config.Model = "gpt-4o-mini"
			}
			if config.Host == "" {
				config.Host = nuggets.OpenAIBaseURL
			}
		default:
			fmt.Fprintf(os.Stderr, "Unknown summarizer type: %s\n", summarizerType)
			fmt.Fprintln(os.Stderr, "Supported: ollama, anthropic, openai")
			fmt.Fprintln(os.Stderr, "Or use --provider: openai, together, groq, openrouter")
			os.Exit(1)
		}
	}

	// Create summarizer
	summarizer, err := nuggets.NewSummarizer(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create summarizer: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Extracting nuggets using %s (%s)...\n", summarizer.Name(), config.Model)

	// Load existing nuggets for deduplication
	store := openNuggetStore()
	nuggetFile, err := store.LoadNuggets()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load existing nuggets: %v\n", err)
		nuggetFile = nuggets.NewNuggetFile()
	}

	// Extract nuggets
	result, err := summarizer.Extract(sessionText, "", nuggetFile.Nuggets)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Extraction failed: %v\n", err)
		os.Exit(1)
	}

	if len(result.Nuggets) == 0 {
		fmt.Println("No nuggets found in session.")
		if result.Incomplete {
			fmt.Println("Note: The conversation appears incomplete.")
		}
		return
	}

	fmt.Printf("\nFound %d nuggets:\n\n", len(result.Nuggets))
	for _, n := range result.Nuggets {
		printNuggetSummary(n)
	}

	if result.Incomplete {
		fmt.Println("Note: The conversation appears incomplete. More nuggets may be extracted with additional context.")
	}

	// Add nuggets to pending
	fmt.Println("")
	fmt.Print("Add these nuggets to pending? [Y/n] ")
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response == "" || response == "y" || response == "yes" {
		for _, n := range result.Nuggets {
			n.Created = time.Now()
			if err := store.AddPendingNugget(&n); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to add nugget %s: %v\n", n.ID, err)
			}
		}
		fmt.Printf("Added %d nuggets to pending. Use 'thicc nugget pending' to review.\n", len(result.Nuggets))
	} else {
		fmt.Println("Nuggets not saved.")
	}
}
