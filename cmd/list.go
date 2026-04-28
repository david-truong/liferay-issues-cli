package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/david-truong/liferay-issues-cli/internal/ui"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List Jira issues",
	Long:  "List Jira issues using structural filters. For raw JQL, use `issues jql`.",
	RunE:  listRun,
}

func init() {
	listCmd.Flags().StringP("project", "p", "", "filter by project")
	listCmd.Flags().StringP("assignee", "a", "", "filter by assignee (use 'me' for current user)")
	listCmd.Flags().String("status", "", "filter by status")
	listCmd.Flags().IntP("limit", "n", 20, "max results")
	listCmd.Flags().String("board", "", "list issues on a board (ID or name)")
	listCmd.Flags().String("sprint", "", "list issues in a sprint (ID or name)")
}

func listRun(cmd *cobra.Command, args []string) error {
	if err := initClient(); err != nil {
		return err
	}

	limit, _ := cmd.Flags().GetInt("limit")
	boardFlag, _ := cmd.Flags().GetString("board")
	sprintFlag, _ := cmd.Flags().GetString("sprint")

	// Sprint mode: use Agile API to list sprint issues
	if sprintFlag != "" {
		sprintID, err := resolveSprintFlag(sprintFlag)
		if err != nil {
			return err
		}

		issues, err := client.GetSprintIssues(sprintID, limit)
		if err != nil {
			return err
		}

		if len(issues) == 0 {
			fmt.Println("No issues found.")
			return nil
		}

		ui.PrintIssueTable(issues)
		return nil
	}

	// Board mode: use Agile API to list board issues
	if boardFlag != "" {
		boardID, err := resolveBoardFlag(boardFlag)
		if err != nil {
			return err
		}

		// Build optional JQL from other flags to further filter board issues
		jql := buildFilterJQL(cmd, false)

		issues, total, err := client.GetBoardIssues(boardID, jql, limit)
		if err != nil {
			return err
		}

		if len(issues) == 0 {
			fmt.Println("No issues found.")
			return nil
		}

		ui.PrintIssueTable(issues)
		if total > len(issues) {
			fmt.Printf("\nShowing %d of %d results\n", len(issues), total)
		}
		return nil
	}

	// Standard JQL search
	jql := buildFilterJQL(cmd, true)
	if jql == "" {
		return fmt.Errorf("provide at least one filter (--project, --assignee, --status, --board, --sprint), or use `issues jql` for raw JQL")
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

// buildFilterJQL builds a JQL string from individual filter flags.
// If requireProject is true, includes the default project.
func buildFilterJQL(cmd *cobra.Command, requireProject bool) string {
	var clauses []string

	if v, _ := cmd.Flags().GetString("project"); v != "" {
		clauses = append(clauses, fmt.Sprintf("project = %q", v))
	} else if requireProject && cfg.Jira.DefaultProject != "" {
		clauses = append(clauses, fmt.Sprintf("project = %q", cfg.Jira.DefaultProject))
	}

	if v, _ := cmd.Flags().GetString("assignee"); v != "" {
		if v == "me" {
			clauses = append(clauses, "assignee = currentUser()")
		} else {
			clauses = append(clauses, fmt.Sprintf("assignee = %q", v))
		}
	}

	if v, _ := cmd.Flags().GetString("status"); v != "" {
		clauses = append(clauses, fmt.Sprintf("status = %q", v))
	}

	if len(clauses) == 0 {
		return ""
	}

	return strings.Join(clauses, " AND ") + " ORDER BY updated DESC"
}

// resolveBoardFlag resolves a board ID from a flag value (numeric ID or name search).
func resolveBoardFlag(value string) (int, error) {
	if id, err := strconv.Atoi(value); err == nil {
		return id, nil
	}

	boards, err := client.GetBoards("", value, "")
	if err != nil {
		return 0, fmt.Errorf("searching boards: %w", err)
	}
	if len(boards) == 0 {
		return 0, fmt.Errorf("no board found matching %q", value)
	}
	if len(boards) == 1 {
		return boards[0].ID, nil
	}

	selected, err := ui.SelectBoard(boards)
	if err != nil {
		return 0, err
	}
	return selected.ID, nil
}
