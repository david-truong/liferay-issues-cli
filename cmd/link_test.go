package cmd

import (
	"testing"

	"github.com/david-truong/liferay-issues-cli/internal/jira"
)

func TestResolveLinkRelation(t *testing.T) {
	// A subset of the types on liferay.atlassian.net, including the ones whose
	// names and phrases overlap.
	types := []jira.IssueLinkType{
		{Name: "Blocked", Inward: "Is blocked by", Outward: "Is blocking"},
		{Name: "Blocks", Inward: "is blocked by", Outward: "blocks"},
		{Name: "Duplicate", Inward: "is duplicated by", Outward: "duplicates"},
		{Name: "Relates", Inward: "relates to", Outward: "relates to"},
		{Name: "Relationship", Inward: "is related to", Outward: "relates"},
	}

	tests := []struct {
		name       string
		types      []jira.IssueLinkType
		relation   string
		wantType   string
		wantInward bool
		wantErr    bool
	}{
		{name: "outward phrase", types: types, relation: "blocks", wantType: "Blocks"},
		{name: "inward phrase swaps", types: types, relation: "is blocked by", wantType: "Blocks", wantInward: true},
		{name: "exact case picks overlapping type", types: types, relation: "Is blocked by", wantType: "Blocked", wantInward: true},
		{name: "type name is outward", types: types, relation: "Duplicate", wantType: "Duplicate"},
		{name: "case insensitive and trimmed", types: types, relation: "  Is Duplicated By ", wantType: "Duplicate", wantInward: true},
		{name: "symmetric phrase is outward", types: types, relation: "relates to", wantType: "Relates"},
		{name: "type name beats another type's phrase", types: types, relation: "relates", wantType: "Relates"},
		{name: "unknown relation", types: types, relation: "unblocks", wantErr: true},
		{name: "case insensitive tie is ambiguous", types: types, relation: "IS BLOCKED BY", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, inward, err := resolveLinkRelation(tt.types, tt.relation)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveLinkRelation() = %+v, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveLinkRelation() error = %v", err)
			}
			if got.Name != tt.wantType {
				t.Errorf("type = %q, want %q", got.Name, tt.wantType)
			}
			if inward != tt.wantInward {
				t.Errorf("inward = %v, want %v", inward, tt.wantInward)
			}
		})
	}
}
