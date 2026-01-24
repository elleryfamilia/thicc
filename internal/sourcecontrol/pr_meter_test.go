package sourcecontrol

import (
	"testing"
)

func TestGetFileTypeWeight(t *testing.T) {
	tests := []struct {
		path     string
		expected float64
	}{
		// Core source files: 1.0x
		{"main.go", 1.0},
		{"src/app.ts", 1.0},
		{"lib/utils.py", 1.0},
		{"index.tsx", 1.0},

		// Test files: 0.5x
		{"main_test.go", 0.5},
		{"app.test.ts", 0.5},
		{"utils.spec.tsx", 0.5},

		// Config files: 0.3x
		{"config.json", 0.3},
		{"settings.yaml", 0.3},
		{"Cargo.toml", 0.3},

		// Documentation: 0.2x
		{"README.md", 0.2},
		{"docs/guide.txt", 0.2},

		// Generated/vendor: 0.1x
		{"vendor/lib/foo.go", 0.1},
		{"node_modules/react/index.js", 0.1},
		{"api.pb.go", 0.1},
		{"styles.min.css", 0.1},

		// Lock files: 0.1x
		{"yarn.lock", 0.1},
		{"Cargo.lock", 0.1},
		{"package-lock.json", 0.1},
		{"pnpm-lock.yaml", 0.1},

		// CSS: 0.5x
		{"styles.css", 0.5},
		{"app.scss", 0.5},

		// HTML: 0.4x
		{"index.html", 0.4},
		{"template.tmpl", 0.4},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := GetFileTypeWeight(tt.path)
			if got != tt.expected {
				t.Errorf("GetFileTypeWeight(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestCalculateSpreadFactor(t *testing.T) {
	tests := []struct {
		name       string
		files      int
		lines      int
		expected   float64
	}{
		{"no changes", 0, 0, 1.0},
		{"focused", 2, 500, 0.85},      // 2 / (500/50) = 0.2 < 0.5
		{"normal", 5, 250, 1.0},        // 5 / (250/50) = 1.0
		{"scattered", 20, 100, 1.25},   // 20 / (100/50) = 10 > 2.0
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateSpreadFactor(tt.files, tt.lines)
			if got != tt.expected {
				t.Errorf("CalculateSpreadFactor(%d, %d) = %v, want %v", tt.files, tt.lines, got, tt.expected)
			}
		})
	}
}

func TestCalculatePatience(t *testing.T) {
	tests := []struct {
		lines    int
		minP     float64 // minimum expected patience
		maxP     float64 // maximum expected patience
	}{
		{0, 1.0, 1.0},       // No changes = 100%
		{50, 0.85, 0.95},    // XS PR
		{100, 0.75, 0.85},   // Border of XS/S
		{250, 0.55, 0.65},   // S/M border
		{400, 0.35, 0.45},   // M/L border
		{600, 0.15, 0.25},   // L/XL border
		{1000, 0.03, 0.10},  // XL/XXL border
		{2000, 0.0, 0.02},   // XXL
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := CalculatePatience(tt.lines)
			if got < tt.minP || got > tt.maxP {
				t.Errorf("CalculatePatience(%d) = %v, want between %v and %v", tt.lines, got, tt.minP, tt.maxP)
			}
		})
	}
}

func TestCalculateMeterState(t *testing.T) {
	// Test nil stats
	meter := CalculateMeterState(nil)
	if meter.Patience != 1.0 {
		t.Errorf("nil stats should give 100%% patience, got %v", meter.Patience)
	}
	if meter.EatenPellets != 0 {
		t.Errorf("nil stats should have 0 eaten pellets, got %v", meter.EatenPellets)
	}

	// Test small PR
	stats := &PRStats{
		Additions:  30,
		Deletions:  20,
		FilesCount: 2,
		FileStats: []PRFileStats{
			{Path: "main.go", Additions: 30, Deletions: 20, Weight: 1.0},
		},
	}
	meter = CalculateMeterState(stats)
	if meter.Patience < 0.8 {
		t.Errorf("small PR should have high patience, got %v", meter.Patience)
	}

	// Test large PR
	stats = &PRStats{
		Additions:  800,
		Deletions:  200,
		FilesCount: 20,
		FileStats: []PRFileStats{
			{Path: "main.go", Additions: 800, Deletions: 200, Weight: 1.0},
		},
	}
	meter = CalculateMeterState(stats)
	if meter.Patience > 0.2 {
		t.Errorf("large PR should have low patience, got %v", meter.Patience)
	}

	// Test minimum 1 pellet eaten for tiny changes
	tinyStats := &PRStats{
		Additions:  5,
		Deletions:  2,
		FilesCount: 1,
		FileStats: []PRFileStats{
			{Path: "main.go", Additions: 5, Deletions: 2, Weight: 1.0},
		},
	}
	tinyMeter := CalculateMeterState(tinyStats)
	if tinyMeter.EatenPellets < 1 {
		t.Errorf("tiny PR should have at least 1 pellet eaten, got %d", tinyMeter.EatenPellets)
	}

	// Test that test files reduce impact
	statsWithTests := &PRStats{
		Additions:  400,
		Deletions:  100,
		FilesCount: 2,
		FileStats: []PRFileStats{
			{Path: "main.go", Additions: 100, Deletions: 50, Weight: 1.0},
			{Path: "main_test.go", Additions: 300, Deletions: 50, Weight: 0.5},
		},
	}
	meterWithTests := CalculateMeterState(statsWithTests)

	statsNoTests := &PRStats{
		Additions:  400,
		Deletions:  100,
		FilesCount: 2,
		FileStats: []PRFileStats{
			{Path: "main.go", Additions: 400, Deletions: 100, Weight: 1.0},
		},
	}
	meterNoTests := CalculateMeterState(statsNoTests)

	if meterWithTests.Patience <= meterNoTests.Patience {
		t.Errorf("PR with tests should have more patience than without: with=%v, without=%v",
			meterWithTests.Patience, meterNoTests.Patience)
	}
}
