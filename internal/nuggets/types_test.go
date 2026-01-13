package nuggets

import (
	"testing"
)

func TestNuggetTypeIsValid(t *testing.T) {
	tests := []struct {
		nuggetType NuggetType
		valid      bool
	}{
		{NuggetDecision, true},
		{NuggetDiscovery, true},
		{NuggetGotcha, true},
		{NuggetPattern, true},
		{NuggetIssue, true},
		{NuggetContext, true},
		{NuggetType("invalid"), false},
		{NuggetType(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.nuggetType), func(t *testing.T) {
			if got := tt.nuggetType.IsValid(); got != tt.valid {
				t.Errorf("NuggetType(%q).IsValid() = %v, want %v", tt.nuggetType, got, tt.valid)
			}
		})
	}
}

func TestNuggetTypeDescription(t *testing.T) {
	tests := []struct {
		nuggetType NuggetType
		contains   string
	}{
		{NuggetDecision, "Architectural"},
		{NuggetDiscovery, "Unexpected"},
		{NuggetGotcha, "pitfall"},
		{NuggetPattern, "Reusable"},
		{NuggetIssue, "Bug"},
		{NuggetContext, "background"},
		{NuggetType("unknown"), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.nuggetType), func(t *testing.T) {
			desc := tt.nuggetType.Description()
			if desc == "" {
				t.Error("Description() returned empty string")
			}
		})
	}
}

func TestAllNuggetTypes(t *testing.T) {
	types := AllNuggetTypes()

	if len(types) != 6 {
		t.Errorf("expected 6 nugget types, got %d", len(types))
	}

	// Verify all types are unique
	seen := make(map[NuggetType]bool)
	for _, nt := range types {
		if seen[nt] {
			t.Errorf("duplicate nugget type: %s", nt)
		}
		seen[nt] = true

		// Verify each type is valid
		if !nt.IsValid() {
			t.Errorf("AllNuggetTypes() contains invalid type: %s", nt)
		}
	}
}

func TestNewNuggetFile(t *testing.T) {
	nf := NewNuggetFile()

	if nf == nil {
		t.Fatal("NewNuggetFile() returned nil")
	}

	if nf.Version != 1 {
		t.Errorf("expected Version 1, got %d", nf.Version)
	}

	if nf.Nuggets == nil {
		t.Error("Nuggets slice is nil")
	}

	if len(nf.Nuggets) != 0 {
		t.Errorf("expected empty Nuggets slice, got %d items", len(nf.Nuggets))
	}
}

func TestNewPendingNuggetFile(t *testing.T) {
	pf := NewPendingNuggetFile()

	if pf == nil {
		t.Fatal("NewPendingNuggetFile() returned nil")
	}

	if pf.Version != 1 {
		t.Errorf("expected Version 1, got %d", pf.Version)
	}

	if pf.Nuggets == nil {
		t.Error("Nuggets slice is nil")
	}

	if len(pf.Nuggets) != 0 {
		t.Errorf("expected empty Nuggets slice, got %d items", len(pf.Nuggets))
	}
}
