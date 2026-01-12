package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ellery/thicc/internal/gems"
)

// handleGemCommand handles the "gem" subcommand
// Returns true if the command was handled (and program should exit)
func handleGemCommand(args []string) bool {
	if len(args) < 2 || args[1] != "gem" {
		return false
	}

	// Parse gem subcommand
	if len(args) < 3 {
		printGemUsage()
		os.Exit(1)
	}

	switch args[2] {
	case "list":
		listGems()
	case "pending":
		listPendingGems()
	case "show":
		if len(args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: thicc gem show <id>")
			os.Exit(1)
		}
		showGem(args[3])
	case "accept":
		if len(args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: thicc gem accept <id>")
			os.Exit(1)
		}
		acceptGem(args[3])
	case "reject":
		if len(args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: thicc gem reject <id>")
			os.Exit(1)
		}
		rejectGem(args[3])
	case "search":
		if len(args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: thicc gem search <query>")
			os.Exit(1)
		}
		searchGems(args[3])
	case "add":
		addGemInteractive()
	case "help", "-h", "--help":
		printGemUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown gem command: %s\n", args[2])
		printGemUsage()
		os.Exit(1)
	}

	return true
}

// printGemUsage prints usage information for the gem subcommand
func printGemUsage() {
	fmt.Println("Usage: thicc gem <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  list               List committed gems")
	fmt.Println("  pending            List pending gems awaiting review")
	fmt.Println("  show <id>          Show gem details")
	fmt.Println("  accept <id>        Accept a pending gem (move to committed)")
	fmt.Println("  reject <id>        Reject a pending gem (delete)")
	fmt.Println("  search <query>     Search gems by text")
	fmt.Println("  add                Manually add a gem (interactive)")
	fmt.Println("  help               Show this help message")
	fmt.Println("")
	fmt.Println("Gems are valuable insights captured from AI coding sessions:")
	fmt.Println("  - Architectural decisions and their rationale")
	fmt.Println("  - Non-obvious gotchas and edge cases")
	fmt.Println("  - Reusable patterns and solutions")
	fmt.Println("  - Bug resolutions and their causes")
	fmt.Println("")
	fmt.Println("Committed gems are stored in .agent-gems.json (version controlled)")
	fmt.Println("Pending gems are stored in .agent-history/pending-gems.json (local)")
}

// openGemStore opens the gem store for CLI commands
func openGemStore() *gems.Store {
	store, err := gems.NewStoreFromCwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open gem store: %v\n", err)
		os.Exit(1)
	}
	return store
}

// listGems lists all committed gems
func listGems() {
	store := openGemStore()

	gemFile, err := store.LoadGems()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load gems: %v\n", err)
		os.Exit(1)
	}

	if len(gemFile.Gems) == 0 {
		fmt.Println("No committed gems found.")
		fmt.Println("")
		fmt.Println("Use 'thicc gem add' to manually add a gem,")
		fmt.Println("or gems will be auto-extracted from AI sessions.")
		return
	}

	fmt.Printf("Found %d committed gems:\n\n", len(gemFile.Gems))
	for _, g := range gemFile.Gems {
		printGemSummary(g)
	}
}

// listPendingGems lists pending gems awaiting review
func listPendingGems() {
	store := openGemStore()

	pending, err := store.LoadPendingGems()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load pending gems: %v\n", err)
		os.Exit(1)
	}

	if len(pending.Gems) == 0 {
		fmt.Println("No pending gems.")
		return
	}

	fmt.Printf("Found %d pending gems awaiting review:\n\n", len(pending.Gems))
	for _, g := range pending.Gems {
		printGemSummary(g)
	}
	fmt.Println("")
	fmt.Println("Use 'thicc gem accept <id>' to accept a gem")
	fmt.Println("Use 'thicc gem reject <id>' to reject a gem")
}

// printGemSummary prints a one-line summary of a gem
func printGemSummary(g gems.Gem) {
	id := g.ID
	if len(id) > 12 {
		id = id[:12]
	}
	fmt.Printf("  %s  [%s]  %s\n", id, g.Type, g.Title)
	fmt.Printf("           %s\n", g.Summary)
	if len(g.Tags) > 0 {
		fmt.Printf("           Tags: %s\n", strings.Join(g.Tags, ", "))
	}
	fmt.Println()
}

// showGem displays detailed information about a gem
func showGem(id string) {
	store := openGemStore()

	gem, isPending, err := store.GetGem(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Gem not found: %s\n", id)
		os.Exit(1)
	}

	status := "committed"
	if isPending {
		status = "pending"
	}

	fmt.Printf("Gem: %s (%s)\n", gem.ID, status)
	fmt.Printf("Type: %s\n", gem.Type)
	fmt.Printf("Title: %s\n", gem.Title)
	fmt.Printf("Summary: %s\n", gem.Summary)
	fmt.Printf("Created: %s\n", gem.Created.Format("2006-01-02 15:04:05"))
	if gem.Commit != "" {
		fmt.Printf("Commit: %s\n", gem.Commit)
	}
	if gem.Client != "" {
		fmt.Printf("Client: %s\n", gem.Client)
	}
	if gem.Model != "" {
		fmt.Printf("Model: %s\n", gem.Model)
	}
	if len(gem.Tags) > 0 {
		fmt.Printf("Tags: %s\n", strings.Join(gem.Tags, ", "))
	}
	if len(gem.Files) > 0 {
		fmt.Printf("Files: %s\n", strings.Join(gem.Files, ", "))
	}
	fmt.Println()

	// Print content fields
	if len(gem.Content) > 0 {
		fmt.Println("Content:")
		for key, value := range gem.Content {
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

	if gem.UserNotes != "" {
		fmt.Println()
		fmt.Printf("User Notes: %s\n", gem.UserNotes)
	}
}

// acceptGem accepts a pending gem
func acceptGem(id string) {
	store := openGemStore()

	if err := store.AcceptGem(id); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to accept gem: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Gem %s accepted and moved to .agent-gems.json\n", id)
}

// rejectGem rejects a pending gem
func rejectGem(id string) {
	store := openGemStore()

	if err := store.RejectGem(id); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to reject gem: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Gem %s rejected and removed from pending list\n", id)
}

// searchGems searches gems by query
func searchGems(query string) {
	store := openGemStore()

	results, err := store.SearchGems(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Search failed: %v\n", err)
		os.Exit(1)
	}

	if len(results) == 0 {
		fmt.Printf("No gems found matching: %s\n", query)
		return
	}

	fmt.Printf("Found %d gems matching '%s':\n\n", len(results), query)
	for _, g := range results {
		printGemSummary(g)
	}
}

// addGemInteractive adds a gem interactively via prompts
func addGemInteractive() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Add a new gem")
	fmt.Println("=============")
	fmt.Println()

	// Select type
	fmt.Println("Gem types:")
	for i, t := range gems.AllGemTypes() {
		fmt.Printf("  %d. %s - %s\n", i+1, t, t.Description())
	}
	fmt.Print("\nSelect type (1-6): ")
	typeInput, _ := reader.ReadString('\n')
	typeInput = strings.TrimSpace(typeInput)
	var gemType gems.GemType
	switch typeInput {
	case "1":
		gemType = gems.GemDecision
	case "2":
		gemType = gems.GemDiscovery
	case "3":
		gemType = gems.GemGotcha
	case "4":
		gemType = gems.GemPattern
	case "5":
		gemType = gems.GemIssue
	case "6":
		gemType = gems.GemContext
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

	// Create the gem
	gem := &gems.Gem{
		Type:      gemType,
		Title:     title,
		Summary:   summary,
		Created:   time.Now(),
		Client:    "thicc-cli",
		Tags:      tags,
		Files:     files,
		Content:   make(map[string]any),
		UserNotes: notes,
	}

	// Save directly to committed gems
	store := openGemStore()
	if err := store.AddGem(gem); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add gem: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("Gem added: %s\n", gem.ID)
	fmt.Printf("Saved to: %s\n", store.GetProjectRoot()+"/.agent-gems.json")
}
