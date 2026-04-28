package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/david-truong/liferay-issues-cli/internal/jira"
	"github.com/david-truong/liferay-issues-cli/internal/ui"
	"github.com/spf13/cobra"
)

var sprintCmd = &cobra.Command{
	Use:   "sprint",
	Short: "Manage sprints",
}

var sprintListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sprints on a board",
	Long: `List sprints for a board. Defaults to active sprints.

Examples:
  issues sprint list                          # active sprints on default board
  issues sprint list --board 123              # active sprints on board 123
  issues sprint list --board "My Board"       # search board by name
  issues sprint list --state future           # future sprints
  issues sprint list --state active,future    # active and future sprints`,
	RunE: sprintListRun,
}

var sprintViewCmd = &cobra.Command{
	Use:   "view <ID>",
	Short: "View sprint details",
	Args:  cobra.ExactArgs(1),
	RunE:  sprintViewRun,
}

func init() {
	sprintListCmd.Flags().String("board", "", "board ID or name (defaults to first board found)")
	sprintListCmd.Flags().String("state", "active", "sprint state filter (active, closed, future — comma-separated)")

	sprintCmd.AddCommand(sprintListCmd)
	sprintCmd.AddCommand(sprintViewCmd)
}

func sprintListRun(cmd *cobra.Command, args []string) error {
	if err := initClient(); err != nil {
		return err
	}

	boardFlag, _ := cmd.Flags().GetString("board")
	boardID, err := resolveBoard(boardFlag)
	if err != nil {
		return err
	}

	state, _ := cmd.Flags().GetString("state")

	sprints, err := client.GetSprints(boardID, state)
	if err != nil {
		return err
	}

	if len(sprints) == 0 {
		fmt.Println("No sprints found.")
		return nil
	}

	ui.PrintSprintTable(sprints)
	return nil
}

func sprintViewRun(cmd *cobra.Command, args []string) error {
	id, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("sprint ID must be a number: %s", args[0])
	}

	if err := initClient(); err != nil {
		return err
	}

	sprint, err := client.GetSprint(id)
	if err != nil {
		return err
	}

	ui.PrintSprintDetail(sprint)
	return nil
}

// resolveBoard resolves a board ID from a flag value (numeric ID, name search,
// or empty to fall back to config default → scrum boards for default project).
func resolveBoard(value string) (int, error) {
	if value != "" {
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
		return pickBoard(boards)
	}

	if cfg.Jira.DefaultBoard > 0 {
		return cfg.Jira.DefaultBoard, nil
	}

	boards, err := client.GetBoards("scrum", "", cfg.Jira.DefaultProject)
	if err != nil {
		return 0, fmt.Errorf("fetching boards: %w", err)
	}
	if len(boards) == 0 {
		return 0, fmt.Errorf("no scrum boards found — use --board or set jira.default_board in config")
	}
	return pickBoard(boards)
}

func pickBoard(boards []jira.Board) (int, error) {
	if len(boards) == 1 {
		return boards[0].ID, nil
	}
	selected, err := ui.SelectBoard(boards)
	if err != nil {
		return 0, err
	}
	return selected.ID, nil
}

// resolveSprintFlag resolves a sprint from the --sprint flag value.
// Searches active/future sprints on the default board for name matches.
func resolveSprintFlag(value string) (int, error) {
	if id, err := strconv.Atoi(value); err == nil {
		return id, nil
	}

	boardID, err := resolveBoard("")
	if err != nil {
		return 0, fmt.Errorf("resolving board for sprint search: %w", err)
	}

	sprints, err := client.GetSprints(boardID, "active,future")
	if err != nil {
		return 0, err
	}

	matches := ui.FindSprintByName(sprints, value)
	if len(matches) == 0 {
		return 0, fmt.Errorf("no active/future sprint found matching %q", value)
	}
	if len(matches) == 1 {
		return matches[0].ID, nil
	}

	selected, err := ui.SelectSprint(matches)
	if err != nil {
		return 0, err
	}
	return selected.ID, nil
}

// isSubtaskError checks if an error is the "subtasks cannot be associated to a sprint" error.
func isSubtaskError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "subtask")
}
