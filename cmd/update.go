package cmd

import (
	"fmt"
	"strings"

	"github.com/david-truong/liferay-issues-cli/internal/jira"
	"github.com/david-truong/liferay-issues-cli/internal/ui"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update <TICKET>",
	Short: "Update a Jira issue",
	Args:  cobra.MinimumNArgs(1),
	RunE:  updateRun,
}

func init() {
	updateCmd.Flags().StringP("summary", "s", "", "new summary")
	updateCmd.Flags().StringP("description", "d", "", "new description")
	updateCmd.Flags().StringP("assignee", "a", "", "new assignee display name (searches Jira)")
	updateCmd.Flags().String("assignee-id", "", "new assignee account ID")
	updateCmd.Flags().String("priority", "", "new priority")
	updateCmd.Flags().StringSlice("labels", nil, "set labels (replaces all)")
	updateCmd.Flags().String("add-label", "", "add a label")
	updateCmd.Flags().String("remove-label", "", "remove a label")
	updateCmd.Flags().String("component", "", "set component")
	updateCmd.Flags().String("pull-request", "", "set Git Pull Request URL")
	updateCmd.Flags().String("fix-version", "", "set fix version")
	updateCmd.Flags().StringArray("field", nil, "set arbitrary field (key=value, value may be JSON)")
	updateCmd.Flags().String("affects-version", "", "set affects version")
	updateCmd.Flags().String("sprint", "", "move issue to a sprint (ID or name)")
	updateCmd.Flags().Bool("remove-sprint", false, "remove issue from its current sprint")
	updateCmd.Flags().Bool("include-parent", false, "if issue is a subtask, update the parent instead")
}

func updateRun(cmd *cobra.Command, args []string) error {
	ticket, err := resolveTicket(args)
	if err != nil {
		return err
	}

	if err := initClient(); err != nil {
		return err
	}

	fields := map[string]interface{}{}

	if v, _ := cmd.Flags().GetString("summary"); v != "" {
		fields["summary"] = v
	}
	if v, _ := cmd.Flags().GetString("description"); v != "" {
		fields["description"] = jira.MakeADFBody(v)
	}
	if v, _ := cmd.Flags().GetString("assignee-id"); v != "" {
		fields["assignee"] = map[string]string{"accountId": v}
	} else if v, _ := cmd.Flags().GetString("assignee"); v != "" {
		accountID, err := resolveAssignee(v)
		if err != nil {
			return err
		}
		fields["assignee"] = map[string]string{"accountId": accountID}
	}
	if v, _ := cmd.Flags().GetString("priority"); v != "" {
		fields["priority"] = map[string]string{"name": v}
	}
	if v, _ := cmd.Flags().GetStringSlice("labels"); len(v) > 0 {
		fields["labels"] = v
	}
	if v, _ := cmd.Flags().GetString("component"); v != "" {
		fields["components"] = []map[string]string{{"name": v}}
	}

	// Handle add-label and remove-label via the issue's current labels
	addLabel, _ := cmd.Flags().GetString("add-label")
	removeLabel, _ := cmd.Flags().GetString("remove-label")

	if addLabel != "" || removeLabel != "" {
		issue, err := client.GetIssue(ticket)
		if err != nil {
			return fmt.Errorf("fetching current issue: %w", err)
		}
		labels := issue.Fields.Labels

		if addLabel != "" {
			found := false
			for _, l := range labels {
				if l == addLabel {
					found = true
					break
				}
			}
			if !found {
				labels = append(labels, addLabel)
			}
		}

		if removeLabel != "" {
			var filtered []string
			for _, l := range labels {
				if l != removeLabel {
					filtered = append(filtered, l)
				}
			}
			labels = filtered
		}

		fields["labels"] = labels
	}

	if v, _ := cmd.Flags().GetString("pull-request"); v != "" {
		fields[pullRequestField] = v
	}

	if v, _ := cmd.Flags().GetString("fix-version"); v != "" {
		fields["fixVersions"] = []map[string]string{{"name": v}}
	}

	if v, _ := cmd.Flags().GetString("affects-version"); v != "" {
		fields["versions"] = []map[string]string{{"name": v}}
	}

	if customFields, _ := cmd.Flags().GetStringArray("field"); len(customFields) > 0 {
		for _, f := range customFields {
			k, v, ok := strings.Cut(f, "=")
			if !ok {
				return fmt.Errorf("invalid --field format %q (expected key=value)", f)
			}
			fields[k] = parseFieldValue(v)
		}
	}

	sprintFlag, _ := cmd.Flags().GetString("sprint")
	removeSprintFlag, _ := cmd.Flags().GetBool("remove-sprint")
	includeParent, _ := cmd.Flags().GetBool("include-parent")

	if sprintFlag != "" && removeSprintFlag {
		return fmt.Errorf("cannot use --sprint and --remove-sprint together")
	}

	hasSprint := sprintFlag != "" || removeSprintFlag

	if len(fields) == 0 && !hasSprint {
		return fmt.Errorf("no fields to update — use flags like --summary, --assignee, --sprint, etc.")
	}

	if len(fields) > 0 {
		if err := client.UpdateIssue(ticket, fields); err != nil {
			return err
		}
		fmt.Printf("Updated %s\n", ticket)
	}

	if hasSprint {
		sprintTarget := ticket
		if removeSprintFlag {
			if err := handleSprintRemove(sprintTarget, includeParent); err != nil {
				return err
			}
		} else {
			if err := handleSprintMove(sprintTarget, sprintFlag, includeParent); err != nil {
				return err
			}
		}
	}

	return nil
}

func handleSprintMove(ticket string, sprintFlag string, includeParent bool) error {
	sprintID, err := resolveSprintFlag(sprintFlag)
	if err != nil {
		return err
	}

	err = client.MoveToSprint(sprintID, []string{ticket})
	if err != nil && isSubtaskError(err) {
		return handleSubtaskSprint(ticket, includeParent, func(parentKey string) error {
			return client.MoveToSprint(sprintID, []string{parentKey})
		})
	}
	if err != nil {
		return err
	}

	fmt.Printf("Moved %s to sprint %d\n", ticket, sprintID)
	return nil
}

func handleSprintRemove(ticket string, includeParent bool) error {
	err := client.RemoveFromSprint([]string{ticket})
	if err != nil && isSubtaskError(err) {
		return handleSubtaskSprint(ticket, includeParent, func(parentKey string) error {
			return client.RemoveFromSprint([]string{parentKey})
		})
	}
	if err != nil {
		return err
	}

	fmt.Printf("Removed %s from sprint\n", ticket)
	return nil
}

func handleSubtaskSprint(ticket string, includeParent bool, action func(string) error) error {
	issue, err := client.GetIssue(ticket)
	if err != nil {
		return fmt.Errorf("subtasks cannot be moved to a sprint directly; failed to look up parent: %w", err)
	}
	if issue.Fields.Parent == nil {
		return fmt.Errorf("subtasks cannot be moved to a sprint directly, and no parent issue was found")
	}

	parentKey := issue.Fields.Parent.Key
	if !includeParent {
		confirmed, err := ui.Confirm(fmt.Sprintf("%s is a subtask — update parent %s instead?", ticket, parentKey))
		if err != nil {
			return err
		}
		if !confirmed {
			return fmt.Errorf("subtasks cannot be associated to a sprint directly — use --include-parent to update the parent")
		}
	}

	if err := action(parentKey); err != nil {
		return err
	}
	fmt.Printf("Updated parent %s instead of subtask %s\n", parentKey, ticket)
	return nil
}
