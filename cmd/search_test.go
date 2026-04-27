package cmd

import (
	"testing"

	"github.com/david-truong/liferay-issues-cli/internal/config"
	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "search", RunE: func(cmd *cobra.Command, args []string) error { return nil }}
	cmd.Flags().StringP("project", "p", "", "")
	cmd.Flags().StringP("assignee", "a", "", "")
	cmd.Flags().String("status", "", "")
	cmd.Flags().StringP("type", "t", "", "")
	cmd.Flags().StringP("component", "c", "", "")
	cmd.Flags().StringP("label", "l", "", "")
	cmd.Flags().StringP("resolution", "r", "", "")
	cmd.Flags().StringP("version", "v", "", "")
	cmd.Flags().Bool("fixed", false, "")
	cmd.Flags().String("after", "", "")
	cmd.Flags().String("before", "", "")
	cmd.Flags().Bool("include-master", false, "")
	cmd.Flags().IntP("limit", "n", 20, "")
	cmd.Flags().String("order-by", "", "")
	return cmd
}

func TestBuildSearchJQL(t *testing.T) {
	// Save and restore global cfg
	origCfg := cfg
	cfg = &config.Config{}
	defer func() { cfg = origCfg }()

	tests := []struct {
		name  string
		query string
		flags map[string]string
		cfg   config.JiraConfig
		want  string
	}{
		{
			name:  "basic query",
			query: "login timeout",
			want:  `text ~ "login timeout"`,
		},
		{
			name:  "query with project",
			query: "null pointer",
			flags: map[string]string{"project": "LPD"},
			want:  `text ~ "null pointer" AND project = "LPD"`,
		},
		{
			name:  "query with assignee me",
			query: "login",
			flags: map[string]string{"assignee": "me"},
			want:  `text ~ "login" AND assignee = currentUser()`,
		},
		{
			name:  "query with assignee name",
			query: "login",
			flags: map[string]string{"assignee": "Jane Doe"},
			want:  `text ~ "login" AND assignee = "Jane Doe"`,
		},
		{
			name:  "query with status",
			query: "login",
			flags: map[string]string{"status": "Open"},
			want:  `text ~ "login" AND status = "Open"`,
		},
		{
			name:  "query with type",
			query: "crash",
			flags: map[string]string{"type": "Bug"},
			want:  `text ~ "crash" AND issuetype = "Bug"`,
		},
		{
			name:  "query with component flag",
			query: "error",
			flags: map[string]string{"component": "REST Builder"},
			want:  `text ~ "error" AND component = "REST Builder"`,
		},
		{
			name:  "query with default component from config",
			query: "error",
			cfg:   config.JiraConfig{DefaultComponent: "Frontend"},
			want:  `text ~ "error" AND component = "Frontend"`,
		},
		{
			name:  "component flag overrides default",
			query: "error",
			flags: map[string]string{"component": "Backend"},
			cfg:   config.JiraConfig{DefaultComponent: "Frontend"},
			want:  `text ~ "error" AND component = "Backend"`,
		},
		{
			name:  "query with label",
			query: "login",
			flags: map[string]string{"label": "regression"},
			want:  `text ~ "login" AND labels = "regression"`,
		},
		{
			name:  "query with resolution",
			query: "login",
			flags: map[string]string{"resolution": "Fixed"},
			want:  `text ~ "login" AND resolution = "Fixed"`,
		},
		{
			name:  "fixed shorthand",
			query: "login",
			flags: map[string]string{"fixed": "true"},
			want:  `text ~ "login" AND status = "Closed" AND resolution = "Fixed"`,
		},
		{
			name:  "fixed overrides status and resolution",
			query: "login",
			flags: map[string]string{"fixed": "true", "status": "Open", "resolution": "Duplicate"},
			want:  `text ~ "login" AND status = "Closed" AND resolution = "Fixed"`,
		},
		{
			name:  "exact version",
			query: "404",
			flags: map[string]string{"version": "2024.Q4.0"},
			want:  `text ~ "404" AND affectedVersion = "2024.Q4.0"`,
		},
		{
			name:  "after version",
			query: "login",
			flags: map[string]string{"after": "7.4.0"},
			want:  `text ~ "login" AND fixVersion >= "7.4.0"`,
		},
		{
			name:  "before version",
			query: "login",
			flags: map[string]string{"before": "7.4.3"},
			want:  `text ~ "login" AND fixVersion <= "7.4.3"`,
		},
		{
			name:  "version range",
			query: "login",
			flags: map[string]string{"after": "7.3.0", "before": "7.4.3"},
			want:  `text ~ "login" AND fixVersion >= "7.3.0" AND fixVersion <= "7.4.3"`,
		},
		{
			name:  "order by",
			query: "login",
			flags: map[string]string{"order-by": "updated"},
			want:  `text ~ "login" ORDER BY updated DESC`,
		},
		{
			name:  "multiple flags combined",
			query: "Safari crash",
			flags: map[string]string{"project": "LPD", "type": "Bug", "assignee": "me"},
			want:  `text ~ "Safari crash" AND project = "LPD" AND assignee = currentUser() AND issuetype = "Bug"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newSearchCmd()

			for k, v := range tt.flags {
				cmd.Flags().Set(k, v)
			}

			cfg.Jira = tt.cfg

			got, err := buildSearchJQL(cmd, tt.query, false)
			if err != nil {
				t.Fatalf("buildSearchJQL() error = %v", err)
			}

			if got != tt.want {
				t.Errorf("buildSearchJQL() =\n  %q\nwant:\n  %q", got, tt.want)
			}
		})
	}
}
