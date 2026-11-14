package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/david-truong/liferay-issues-cli/internal/jira"
)

var (
	blocksType       = jira.IssueLinkType{Name: "Blocks", Inward: "is blocked by", Outward: "blocks"}
	dependsType      = jira.IssueLinkType{Name: "Depends", Inward: "depends on", Outward: "is a dependency of"}
	relatesType      = jira.IssueLinkType{Name: "Relates", Inward: "relates to", Outward: "relates to"}
	relationshipType = jira.IssueLinkType{Name: "Relationship", Inward: "is related to", Outward: "relates"}
)

// outwardLink is a link on the current issue reading "current <outward> key".
func outwardLink(id string, linkType jira.IssueLinkType, key string) jira.IssueLink {
	return jira.IssueLink{ID: id, Type: linkType, OutwardIssue: &jira.Issue{Key: key}}
}

// inwardLink is a link on the current issue reading "key <outward> current".
func inwardLink(id string, linkType jira.IssueLinkType, key string) jira.IssueLink {
	return jira.IssueLink{ID: id, Type: linkType, InwardIssue: &jira.Issue{Key: key}}
}

func TestSelectIssueLink(t *testing.T) {
	tests := []struct {
		name     string
		links    []jira.IssueLink
		to       string
		relation string
		wantID   string
		wantErr  bool
	}{
		{
			name:   "only link without relation",
			links:  []jira.IssueLink{outwardLink("1", blocksType, "LPD-2"), outwardLink("2", blocksType, "LPD-3")},
			to:     "lpd-2",
			wantID: "1",
		},
		{
			name:    "several links without relation",
			links:   []jira.IssueLink{outwardLink("1", blocksType, "LPD-2"), inwardLink("2", dependsType, "LPD-2")},
			to:      "LPD-2",
			wantErr: true,
		},
		{
			name:    "no link to the other ticket",
			links:   []jira.IssueLink{outwardLink("1", blocksType, "LPD-3")},
			to:      "LPD-2",
			wantErr: true,
		},
		{
			name:     "outward phrase",
			links:    []jira.IssueLink{outwardLink("1", blocksType, "LPD-2"), inwardLink("2", dependsType, "LPD-2")},
			to:       "LPD-2",
			relation: "blocks",
			wantID:   "1",
		},
		{
			name:     "inward phrase",
			links:    []jira.IssueLink{outwardLink("1", blocksType, "LPD-2"), inwardLink("2", blocksType, "LPD-2")},
			to:       "LPD-2",
			relation: "is blocked by",
			wantID:   "2",
		},
		{
			name:     "direction must match",
			links:    []jira.IssueLink{inwardLink("1", blocksType, "LPD-2")},
			to:       "LPD-2",
			relation: "blocks",
			wantErr:  true,
		},
		{
			name:     "symmetric type matches either direction",
			links:    []jira.IssueLink{inwardLink("1", relatesType, "LPD-2")},
			to:       "LPD-2",
			relation: "relates to",
			wantID:   "1",
		},
		{
			name:     "relation resolves against existing links only",
			links:    []jira.IssueLink{outwardLink("1", relationshipType, "LPP-44905")},
			to:       "LPP-44905",
			relation: "relates",
			wantID:   "1",
		},
		{
			name:     "relation not among existing links",
			links:    []jira.IssueLink{outwardLink("1", blocksType, "LPD-2")},
			to:       "LPD-2",
			relation: "duplicates",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectIssueLink(tt.links, "LPD-1", tt.to, tt.relation)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("selectIssueLink() = %+v, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("selectIssueLink() error = %v", err)
			}
			if got.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", got.ID, tt.wantID)
			}
		})
	}
}

func TestDescribeIssueLink(t *testing.T) {
	if got := describeIssueLink("LPD-1", outwardLink("1", blocksType, "LPD-2")); got != "LPD-1 blocks LPD-2 (Blocks)" {
		t.Errorf("outward = %q", got)
	}
	if got := describeIssueLink("LPD-1", inwardLink("1", blocksType, "LPD-2")); got != "LPD-2 blocks LPD-1 (Blocks)" {
		t.Errorf("inward = %q", got)
	}
}

func TestUnlinkRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantDelete string
		wantErr    bool
	}{
		{name: "deletes the selected link", args: []string{"LPD-1", "is blocked by", "LPD-2"}, wantDelete: "/issueLink/200"},
		{name: "several links without relation", args: []string{"LPD-1", "LPD-2"}, wantErr: true},
		{name: "no link to the other ticket", args: []string{"LPD-1", "LPD-3"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var deleted []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "DELETE" {
					deleted = append(deleted, r.URL.Path)
					w.WriteHeader(204)
					return
				}
				if r.URL.Path != "/issue/LPD-1" || r.URL.Query().Get("fields") != "issuelinks" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				json.NewEncoder(w).Encode(map[string]interface{}{
					"fields": map[string]interface{}{
						"issuelinks": []jira.IssueLink{
							outwardLink("100", blocksType, "LPD-2"),
							inwardLink("200", blocksType, "LPD-2"),
						},
					},
				})
			}))
			defer server.Close()

			savedClient := client
			defer func() { client = savedClient }()
			client = &jira.Client{BaseURL: server.URL, HTTPClient: server.Client()}

			err := unlinkRun(unlinkCmd, tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("unlinkRun() want error")
				}
				if len(deleted) > 0 {
					t.Errorf("deleted %v on error", deleted)
				}
				return
			}
			if err != nil {
				t.Fatalf("unlinkRun() error = %v", err)
			}
			if len(deleted) != 1 || deleted[0] != tt.wantDelete {
				t.Errorf("deleted = %v, want [%s]", deleted, tt.wantDelete)
			}
		})
	}
}
