package cmd

import (
	"testing"

	"github.com/david-truong/liferay-issues-cli/internal/jira"
)

func TestChainContaining(t *testing.T) {
	tests := []struct {
		name      string
		project   string
		issueType string
		current   string
		target    string
		wantLen   int // 0 for nil
	}{
		{"LPD Story has SfD", "LPD", "Story", "Open", "Selected for Development", 5},
		{"LPD Bug has SfD", "LPD", "Bug", "Open", "Selected for Development", 5},
		{"LPD Technical Task lacks SfD", "LPD", "Technical Task", "In Progress", "Selected for Development", 0},
		{"LPD Technical Task standard chain", "LPD", "Technical Task", "Open", "In Peer Review", 4},
		{"BPR Backport Request", "BPR", "Backport Request", "Open", "In Review", 4},
		{"unknown project returns nil", "ZZZ", "Story", "Open", "Closed", 0},
		{"same status returns nil", "LPD", "Story", "Open", "Open", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chainContaining(tt.project, tt.issueType, tt.current, tt.target)
			if tt.wantLen == 0 {
				if got != nil {
					t.Errorf("got %v, want nil", got)
				}
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("got chain len %d, want %d (chain=%v)", len(got), tt.wantLen, got)
			}
		})
	}
}

func TestPickChainStep(t *testing.T) {
	lpd := statusChains[0].chain // LPD Story/Bug/Task chain

	tx := func(toName string) jira.Transition {
		return jira.Transition{ID: toName, Name: toName, To: jira.Status{Name: toName}}
	}

	tests := []struct {
		name        string
		transitions []jira.Transition
		current     string
		target      string
		wantTo      string // expected To.Name, or "" for nil
	}{
		{
			name:        "forward picks closest in-range step",
			transitions: []jira.Transition{tx("Closed"), tx("Selected for Development"), tx("Escalated")},
			current:     "Open",
			target:      "In Peer Review",
			wantTo:      "Selected for Development", // Closed (idx 4) overshoots target (idx 3)
		},
		{
			name:        "forward prefers idx closest to target without overshoot",
			transitions: []jira.Transition{tx("Selected for Development"), tx("In Progress")},
			current:     "Open",
			target:      "In Peer Review",
			wantTo:      "In Progress", // both in-range; max idx wins
		},
		{
			name:        "backward direct step",
			transitions: []jira.Transition{tx("Closed"), tx("In Progress")},
			current:     "In Peer Review",
			target:      "Selected for Development",
			wantTo:      "In Progress",
		},
		{
			name:        "backward fallback via Open (the LPD-85907 case)",
			transitions: []jira.Transition{tx("Closed"), tx("In Peer Review"), tx("Open")},
			current:     "In Progress",
			target:      "Selected for Development",
			wantTo:      "Open", // overshoots, but is the only step that gets us closer
		},
		{
			name:        "no chain candidate returns nil",
			transitions: []jira.Transition{tx("Escalated")},
			current:     "Open",
			target:      "Closed",
			wantTo:      "",
		},
		{
			name:        "fallback rejects steps that don't reduce distance",
			transitions: []jira.Transition{tx("Closed")},
			current:     "In Progress",
			target:      "Selected for Development",
			wantTo:      "", // Closed is farther from target than current
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickChainStep(tt.transitions, lpd, tt.current, tt.target)
			if tt.wantTo == "" {
				if got != nil {
					t.Errorf("pickChainStep = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("pickChainStep = nil, want %q", tt.wantTo)
			}
			if got.To.Name != tt.wantTo {
				t.Errorf("pickChainStep.To.Name = %q, want %q", got.To.Name, tt.wantTo)
			}
		})
	}
}
