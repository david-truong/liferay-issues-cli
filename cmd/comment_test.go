package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/david-truong/liferay-issues-cli/internal/jira"
)

func TestResolveMentions(t *testing.T) {
	users := map[string][]jira.User{
		"Daniel Mijarra": {
			{DisplayName: "Daniel Mijarra Lopez", AccountID: "id-lopez"},
			{DisplayName: "Daniel Mijarra", AccountID: "id-daniel"},
		},
		"Dave": {
			{DisplayName: "Dave Truong", AccountID: "id-dave"},
		},
		"Daniel": {
			{DisplayName: "Daniel Couso", AccountID: "id-couso"},
			{DisplayName: "Daniel Mijarra", AccountID: "id-daniel"},
		},
	}

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr string
	}{
		{
			name: "no mention syntax",
			in:   "plain @ text [brackets]",
			want: "plain @ text [brackets]",
		},
		{
			name: "exact match among several",
			in:   "Hi @[Daniel Mijarra], thanks",
			want: "Hi [~accountid:id-daniel], thanks",
		},
		{
			name: "single result",
			in:   "@[Dave] and @[Daniel Mijarra]",
			want: "[~accountid:id-dave] and [~accountid:id-daniel]",
		},
		{
			name:    "no users",
			in:      "@[Nobody]",
			wantErr: `no users found matching "Nobody"`,
		},
		{
			name:    "ambiguous",
			in:      "@[Daniel]",
			wantErr: "write [~accountid:ID] with one of",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/user/search" {
					t.Errorf("unexpected request %s", r.URL.Path)
				}
				json.NewEncoder(w).Encode(users[r.URL.Query().Get("query")])
			}))
			defer server.Close()

			savedClient := client
			defer func() { client = savedClient }()
			client = &jira.Client{BaseURL: server.URL, HTTPClient: server.Client()}

			got, err := resolveMentions(tt.in)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("resolveMentions(%q) error = %v, want it to contain %q", tt.in, err, tt.wantErr)
				}
				if strings.Contains(err.Error(), "--assignee-id") {
					t.Errorf("mention error should not suggest --assignee-id: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveMentions(%q): %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("resolveMentions(%q)\n got: %s\nwant: %s", tt.in, got, tt.want)
			}
		})
	}
}
