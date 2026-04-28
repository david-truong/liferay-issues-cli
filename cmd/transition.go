package cmd

import (
	"fmt"
	"strings"

	"github.com/david-truong/liferay-issues-cli/internal/ui"
	"github.com/spf13/cobra"
)

var transitionCmd = &cobra.Command{
	Use:   "transition <TICKET> [STATUS]",
	Short: "Transition a Jira issue to a new status",
	Long:  "Move an issue to a new status. If no status is given, shows an interactive picker of available transitions.",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  transitionRun,
}

func init() {
	transitionCmd.Flags().StringP("comment", "m", "", "add a comment with the transition")
	transitionCmd.Flags().String("pull-request", "", "set Git Pull Request URL")
	transitionCmd.Flags().String("fix-version", "", "set fix version")
	transitionCmd.Flags().StringSlice("field", nil, "set a custom field (format: customfield_XXXXX=value)")
}

func transitionRun(cmd *cobra.Command, args []string) error {
	ticket, err := resolveTicket(args[:1])
	if err != nil {
		return err
	}

	if err := initClient(); err != nil {
		return err
	}

	comment, _ := cmd.Flags().GetString("comment")

	transitions, err := client.GetTransitions(ticket)
	if err != nil {
		return fmt.Errorf("fetching transitions: %w", err)
	}

	if len(transitions) == 0 {
		return fmt.Errorf("no transitions available for %s", ticket)
	}

	var transitionID string
	var transitionName string

	if len(args) > 1 {
		// Status provided as argument
		t := ui.FindTransitionByName(transitions, args[1])
		if t == nil {
			fmt.Println("Available transitions:")
			for _, t := range transitions {
				fmt.Printf("  %s → %s\n", t.Name, t.To.Name)
			}
			return fmt.Errorf("no matching transition for %q", args[1])
		}
		transitionID = t.ID
		transitionName = t.Name
	} else {
		// Interactive picker
		t, err := ui.SelectTransition(transitions)
		if err != nil {
			return err
		}
		transitionID = t.ID
		transitionName = t.Name
	}

	fields := map[string]interface{}{}

	if v, _ := cmd.Flags().GetString("pull-request"); v != "" {
		fields[pullRequestField] = v
	}

	if v, _ := cmd.Flags().GetString("fix-version"); v != "" {
		fields["fixVersions"] = []map[string]string{{"name": v}}
	}

	if customFields, _ := cmd.Flags().GetStringSlice("field"); len(customFields) > 0 {
		for _, f := range customFields {
			k, v, ok := strings.Cut(f, "=")
			if !ok {
				return fmt.Errorf("invalid --field format %q (expected key=value)", f)
			}
			fields[k] = v
		}
	}

	if err := client.DoTransition(ticket, transitionID, comment, fields); err != nil {
		return err
	}

	fmt.Printf("Transitioned %s via %q\n", ticket, transitionName)
	return nil
}
