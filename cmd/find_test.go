package cmd

import (
	"strings"
	"testing"

	"github.com/david-truong/liferay-issues-cli/internal/config"
	"github.com/david-truong/liferay-issues-cli/internal/jira"
	"github.com/spf13/cobra"
)

func newFindCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "find", RunE: func(cmd *cobra.Command, args []string) error { return nil }}
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

func TestBuildFindJQL(t *testing.T) {
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
			cmd := newFindCmd()

			for k, v := range tt.flags {
				cmd.Flags().Set(k, v)
			}

			cfg.Jira = tt.cfg

			got, err := buildFindJQL(cmd, tt.query, false)
			if err != nil {
				t.Fatalf("buildFindJQL() error = %v", err)
			}

			if got != tt.want {
				t.Errorf("buildFindJQL() =\n  %q\nwant:\n  %q", got, tt.want)
			}
		})
	}
}

func TestBuildFindJQL_IncludeMasterRequiresBounds(t *testing.T) {
	origCfg := cfg
	cfg = &config.Config{}
	defer func() { cfg = origCfg }()

	cmd := newFindCmd()
	_, err := buildFindJQL(cmd, "login", true)
	if err == nil {
		t.Fatal("expected error when --include-master is set without --after/--before")
	}
	if !strings.Contains(err.Error(), "--include-master") {
		t.Errorf("unexpected error %q", err.Error())
	}
}

func TestBuildMasterDateClause(t *testing.T) {
	versions := []jira.Version{
		{Name: "7.4.0", ReleaseDate: "2023-01-01"},
		{Name: "7.4.3", ReleaseDate: "2023-06-01"},
		{Name: "7.5.0"}, // no release date
	}

	tests := []struct {
		name      string
		after     string
		before    string
		want      string
		wantErr   bool
		errSubstr string
	}{
		{name: "both bounds resolve", after: "7.4.0", before: "7.4.3", want: `created >= "2023-01-01" AND created <= "2023-06-01"`},
		{name: "only after", after: "7.4.0", want: `created >= "2023-01-01"`},
		{name: "only before", before: "7.4.3", want: `created <= "2023-06-01"`},
		{name: "empty bounds", want: ""},
		{name: "after has no release date", after: "7.5.0", wantErr: true, errSubstr: "7.5.0"},
		{name: "before has no release date", before: "7.5.0", wantErr: true, errSubstr: "7.5.0"},
		{name: "after unknown", after: "9.9.9", wantErr: true, errSubstr: "9.9.9"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildMasterDateClause(versions, tt.after, tt.before)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (result %q)", got)
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveComponentName(t *testing.T) {
	components := []jira.Component{
		{Name: "Frontend Infrastructure"},
		{Name: "REST Builder"},
		{Name: "Site Management"},
		{Name: "Site Templates"},
	}

	tests := []struct {
		name      string
		input     string
		want      string
		wantErr   bool
		errSubstr string
	}{
		{name: "exact match", input: "REST Builder", want: "REST Builder"},
		{name: "case-insensitive exact", input: "rest builder", want: "REST Builder"},
		{name: "single contains", input: "frontend", want: "Frontend Infrastructure"},
		{name: "ambiguous contains", input: "site", wantErr: true, errSubstr: "ambiguous"},
		{name: "no match returns input", input: "Nonexistent", want: "Nonexistent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveComponentName(tt.input, components)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (result %q)", got)
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
