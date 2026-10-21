package ui

import (
	"testing"

	"github.com/david-truong/liferay-issues-cli/internal/jira"
)

func TestFindTransitionByName(t *testing.T) {
	transitions := []jira.Transition{
		{ID: "11", Name: "Start Progress", To: jira.Status{Name: "In Progress"}},
		{ID: "21", Name: "Resolve Issue", To: jira.Status{Name: "Resolved"}},
		{ID: "31", Name: "Close Issue", To: jira.Status{Name: "Closed"}},
		{ID: "41", Name: "Reopen Issue", To: jira.Status{Name: "Reopened"}},
	}

	tests := []struct {
		name    string
		search  string
		wantID  string
		wantNil bool
	}{
		{
			name:   "exact match by transition name",
			search: "Start Progress",
			wantID: "11",
		},
		{
			name:   "exact match by target status",
			search: "In Progress",
			wantID: "11",
		},
		{
			name:   "case insensitive match",
			search: "resolve issue",
			wantID: "21",
		},
		{
			name:   "case insensitive target status",
			search: "closed",
			wantID: "31",
		},
		{
			name:   "prefix match",
			search: "Start",
			wantID: "11",
		},
		{
			name:   "prefix match on target",
			search: "Reopen",
			wantID: "41",
		},
		{
			name:   "contains match",
			search: "Progress",
			wantID: "11",
		},
		{
			name:   "contains match on target",
			search: "solve",
			wantID: "21",
		},
		{
			name:    "no match",
			search:  "nonexistent",
			wantNil: true,
		},
		{
			name:    "empty string",
			search:  "",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindTransitionByName(transitions, tt.search)
			if tt.wantNil {
				if got != nil {
					t.Errorf("FindTransitionByName(%q) = %v, want nil", tt.search, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("FindTransitionByName(%q) = nil, want ID %q", tt.search, tt.wantID)
			}
			if got.ID != tt.wantID {
				t.Errorf("FindTransitionByName(%q).ID = %q, want %q", tt.search, got.ID, tt.wantID)
			}
		})
	}
}

func TestFindTransitionByName_EmptyList(t *testing.T) {
	got := FindTransitionByName(nil, "anything")
	if got != nil {
		t.Errorf("FindTransitionByName with nil transitions = %v, want nil", got)
	}
}
