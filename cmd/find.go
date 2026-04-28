package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/david-truong/liferay-issues-cli/internal/jira"
	"github.com/david-truong/liferay-issues-cli/internal/ui"
	"github.com/spf13/cobra"
)

var findCmd = &cobra.Command{
	Use:   "find <query>",
	Short: "Find Jira issues by text",
	Long: `Find Jira issues using full-text search across summary,
description, comments, and other text fields.

Examples:
  issues find "login timeout"
  issues find "null pointer" -p LPD
  issues find "Safari crash" -t Bug -a me
  issues find "API error" -c "REST Builder"
  issues find "login" -v 7.4.0
  issues find "login" --fixed
  issues find "login" --after 7.4.0 --include-master -p LPD`,
	Args: cobra.ExactArgs(1),
	RunE: findRun,
}

func init() {
	findCmd.Flags().StringP("project", "p", "", "filter by project")
	findCmd.Flags().StringP("assignee", "a", "", "filter by assignee (use 'me' for current user)")
	findCmd.Flags().String("status", "", "filter by status")
	findCmd.Flags().StringP("type", "t", "", "filter by issue type (Bug, Story, Task, etc.)")
	findCmd.Flags().StringP("component", "c", "", "filter by component (overrides default from config)")
	findCmd.Flags().StringP("label", "l", "", "filter by label")
	findCmd.Flags().StringP("resolution", "r", "", "filter by resolution (e.g. Fixed, Duplicate)")
	findCmd.Flags().Bool("fixed", false, "shorthand for --status Closed --resolution Fixed")
	findCmd.Flags().StringP("version", "v", "", "filter by exact affects version")
	findCmd.Flags().String("after", "", "fix version >= this version")
	findCmd.Flags().String("before", "", "fix version <= this version")
	findCmd.Flags().Bool("include-master", false, "include master-version tickets by creation date (requires --project or default project)")
	findCmd.Flags().IntP("limit", "n", 20, "max results")
	findCmd.Flags().String("order-by", "", "override sort order (e.g. updated, created, priority)")
}

func findRun(cmd *cobra.Command, args []string) error {
	if err := initClient(); err != nil {
		return err
	}

	query := args[0]
	limit, _ := cmd.Flags().GetInt("limit")
	includeMaster, _ := cmd.Flags().GetBool("include-master")

	// Resolve partial component name before building JQL
	if err := resolveComponentFlag(cmd); err != nil {
		return err
	}

	jql, err := buildFindJQL(cmd, query, includeMaster)
	if err != nil {
		return err
	}

	result, err := client.Search(jql, limit, 0)
	if err != nil {
		return err
	}

	if len(result.Issues) == 0 {
		fmt.Println("No issues found.")
		return nil
	}

	ui.PrintIssueTable(result.Issues)
	if result.Total > len(result.Issues) {
		fmt.Printf("\nShowing %d of %d results\n", len(result.Issues), result.Total)
	}
	return nil
}

func buildFindJQL(cmd *cobra.Command, query string, includeMaster bool) (string, error) {
	var clauses []string

	// Primary text search clause
	clauses = append(clauses, fmt.Sprintf("text ~ %q", query))

	// Project
	project, _ := cmd.Flags().GetString("project")
	if project != "" {
		clauses = append(clauses, fmt.Sprintf("project = %q", project))
	}

	// Assignee
	if v, _ := cmd.Flags().GetString("assignee"); v != "" {
		if v == "me" {
			clauses = append(clauses, "assignee = currentUser()")
		} else {
			clauses = append(clauses, fmt.Sprintf("assignee = %q", v))
		}
	}

	// --fixed shorthand overrides --status and --resolution
	fixed, _ := cmd.Flags().GetBool("fixed")
	if fixed {
		clauses = append(clauses, fmt.Sprintf("status = %q", "Closed"))
		clauses = append(clauses, fmt.Sprintf("resolution = %q", "Fixed"))
	} else {
		if v, _ := cmd.Flags().GetString("status"); v != "" {
			clauses = append(clauses, fmt.Sprintf("status = %q", v))
		}
		if v, _ := cmd.Flags().GetString("resolution"); v != "" {
			clauses = append(clauses, fmt.Sprintf("resolution = %q", v))
		}
	}

	// Issue type
	if v, _ := cmd.Flags().GetString("type"); v != "" {
		clauses = append(clauses, fmt.Sprintf("issuetype = %q", v))
	}

	// Component: flag overrides config default
	if v, _ := cmd.Flags().GetString("component"); v != "" {
		clauses = append(clauses, fmt.Sprintf("component = %q", v))
	} else if cfg.Jira.DefaultComponent != "" {
		clauses = append(clauses, fmt.Sprintf("component = %q", cfg.Jira.DefaultComponent))
	}

	// Label
	if v, _ := cmd.Flags().GetString("label"); v != "" {
		clauses = append(clauses, fmt.Sprintf("labels = %q", v))
	}

	// Exact version filter
	if v, _ := cmd.Flags().GetString("version"); v != "" {
		clauses = append(clauses, fmt.Sprintf("affectedVersion = %q", v))
	}

	// Version range filters
	after, _ := cmd.Flags().GetString("after")
	before, _ := cmd.Flags().GetString("before")

	if includeMaster && after == "" && before == "" {
		return "", fmt.Errorf("--include-master requires --after or --before")
	}

	if includeMaster && (after != "" || before != "") {
		// Build a grouped clause: (version range OR (master AND created in date range))
		err := addVersionRangeWithMaster(&clauses, after, before, project)
		if err != nil {
			return "", err
		}
	} else {
		if after != "" {
			clauses = append(clauses, fmt.Sprintf("fixVersion >= %q", after))
		}
		if before != "" {
			clauses = append(clauses, fmt.Sprintf("fixVersion <= %q", before))
		}
	}

	jql := strings.Join(clauses, " AND ")

	// Only add ORDER BY if explicitly requested; otherwise Jira sorts by relevance
	if v, _ := cmd.Flags().GetString("order-by"); v != "" {
		allowed := map[string]bool{
			"updated": true, "created": true, "priority": true,
			"key": true, "status": true, "assignee": true,
			"summary": true, "issuetype": true, "resolution": true,
		}
		if !allowed[strings.ToLower(v)] {
			return "", fmt.Errorf("invalid --order-by value %q (allowed: updated, created, priority, key, status, assignee, summary, issuetype, resolution)", v)
		}
		jql += " ORDER BY " + v + " DESC"
	}

	return jql, nil
}

func addVersionRangeWithMaster(clauses *[]string, after, before, project string) error {
	// Resolve project for version lookup
	proj := project
	if proj == "" {
		proj = cfg.Jira.DefaultProject
	}
	if proj == "" {
		return fmt.Errorf("--include-master requires --project or a default project (issues config set jira.default_project)")
	}

	versions, err := client.GetProjectVersions(proj)
	if err != nil {
		return fmt.Errorf("fetching project versions: %w", err)
	}

	// Build the version range part
	var versionClauses []string
	if after != "" {
		versionClauses = append(versionClauses, fmt.Sprintf("fixVersion >= %q", after))
	}
	if before != "" {
		versionClauses = append(versionClauses, fmt.Sprintf("fixVersion <= %q", before))
	}
	versionRange := strings.Join(versionClauses, " AND ")

	// Build the master date range part
	masterClause, err := buildMasterDateClause(versions, after, before)
	if err != nil {
		return err
	}

	*clauses = append(*clauses, "("+versionRange+" OR (fixVersion = \"master\" AND "+masterClause+"))")

	return nil
}

func buildMasterDateClause(versions []jira.Version, after, before string) (string, error) {
	versionDates := make(map[string]string)
	for _, v := range versions {
		if v.ReleaseDate != "" {
			versionDates[v.Name] = v.ReleaseDate
		}
	}

	var dateClauses []string

	if after != "" {
		date, ok := versionDates[after]
		if !ok {
			return "", fmt.Errorf("cannot resolve release date for version %q (needed for --include-master)", after)
		}
		dateClauses = append(dateClauses, fmt.Sprintf("created >= %q", date))
	}

	if before != "" {
		date, ok := versionDates[before]
		if !ok {
			return "", fmt.Errorf("cannot resolve release date for version %q (needed for --include-master)", before)
		}
		dateClauses = append(dateClauses, fmt.Sprintf("created <= %q", date))
	}

	return strings.Join(dateClauses, " AND "), nil
}

// resolveComponentFlag resolves a partial component name to the full Jira component name.
// It modifies the --component flag value in place if a match is found.
func resolveComponentFlag(cmd *cobra.Command) error {
	component, _ := cmd.Flags().GetString("component")
	if component == "" {
		component = cfg.Jira.DefaultComponent
	}
	if component == "" {
		return nil
	}

	// Determine project for component lookup
	project, _ := cmd.Flags().GetString("project")
	if project == "" {
		project = cfg.Jira.DefaultProject
	}
	if project == "" {
		return nil // Can't resolve without a project; use the name as-is
	}

	components, err := client.GetProjectComponents(project)
	if err != nil {
		// Surface the error when --component was explicitly set
		if cmd.Flags().Changed("component") {
			return fmt.Errorf("looking up components for project %s: %w", project, err)
		}
		return nil // Fallback for default component config
	}

	resolved, err := resolveComponentName(component, components)
	if err != nil {
		return err
	}
	if resolved != component {
		cmd.Flags().Set("component", resolved)
	}
	return nil
}

func resolveComponentName(name string, components []jira.Component) (string, error) {
	lower := strings.ToLower(name)

	// Exact match (case-insensitive)
	for _, c := range components {
		if strings.ToLower(c.Name) == lower {
			return c.Name, nil
		}
	}

	// Contains match (case-insensitive)
	var matches []jira.Component
	for _, c := range components {
		if strings.Contains(strings.ToLower(c.Name), lower) {
			matches = append(matches, c)
		}
	}

	if len(matches) == 1 {
		return matches[0].Name, nil
	}
	if len(matches) > 1 {
		fmt.Fprintf(os.Stderr, "Multiple components match %q:\n", name)
		for _, c := range matches {
			fmt.Fprintf(os.Stderr, "  %s\n", c.Name)
		}
		return "", fmt.Errorf("ambiguous component %q — use a more specific name", name)
	}

	return name, nil // No match, use as-is
}
