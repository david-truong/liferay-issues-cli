package cmd

import (
	"github.com/david-truong/liferay-issues-cli/internal/ui"
	"github.com/spf13/cobra"
)

var jqlCmd = &cobra.Command{
	Use:   "jql <query>",
	Short: "Run a raw JQL query",
	Long:  "Search for Jira issues using a raw JQL string. Use when the structured filters on `list` and `find` aren't expressive enough.",
	Args:  cobra.ExactArgs(1),
	RunE:  jqlRun,
}

func init() {
	jqlCmd.Flags().IntP("limit", "n", 20, "max results")
}

func jqlRun(cmd *cobra.Command, args []string) error {
	if err := initClient(); err != nil {
		return err
	}

	limit, _ := cmd.Flags().GetInt("limit")

	result, err := client.Search(args[0], limit, 0)
	if err != nil {
		return err
	}

	ui.PrintSearchResults(result.Issues, result.Total)
	return nil
}
