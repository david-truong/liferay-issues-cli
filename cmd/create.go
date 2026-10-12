package cmd

import (
	"fmt"
	"strings"

	"github.com/david-truong/liferay-issues-cli/internal/jira"
	"github.com/david-truong/liferay-issues-cli/internal/ui"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new Jira issue",
	RunE:  createRun,
}

func init() {
	createCmd.Flags().StringP("project", "p", "", "project key (default from config)")
	createCmd.Flags().StringP("type", "t", "", "issue type (Bug, Story, Task, etc.)")
	createCmd.Flags().StringP("summary", "s", "", "issue summary")
	createCmd.Flags().StringP("description", "d", "", "issue description (Markdown)")
	createCmd.Flags().StringP("assignee", "a", "", "assignee display name (searches Jira)")
	createCmd.Flags().String("assignee-id", "", "assignee account ID")
	createCmd.Flags().String("priority", "", "priority (e.g. High, Medium, Low)")
	createCmd.Flags().StringSlice("labels", nil, "labels (comma-separated)")
	createCmd.Flags().String("component", "", "component name")
	createCmd.Flags().BoolP("interactive", "i", false, "interactive mode")
	createCmd.Flags().String("affects-version", "", "set affects version")
	createCmd.Flags().StringArray("field", nil, "set arbitrary field (key=value, value may be JSON)")
}

func createRun(cmd *cobra.Command, args []string) error {
	if err := initClient(); err != nil {
		return err
	}

	interactive, _ := cmd.Flags().GetBool("interactive")
	project, _ := cmd.Flags().GetString("project")
	issueType, _ := cmd.Flags().GetString("type")
	summary, _ := cmd.Flags().GetString("summary")
	description, _ := cmd.Flags().GetString("description")
	assigneeName, _ := cmd.Flags().GetString("assignee")
	assigneeID, _ := cmd.Flags().GetString("assignee-id")
	priority, _ := cmd.Flags().GetString("priority")
	labels, _ := cmd.Flags().GetStringSlice("labels")
	component, _ := cmd.Flags().GetString("component")

	// Use defaults from config
	if project == "" {
		project = cfg.Jira.DefaultProject
	}
	if issueType == "" {
		issueType = cfg.Defaults.IssueType
	}

	if project == "" {
		return fmt.Errorf("project is required (use -p or set jira.default_project in config)")
	}

	// Interactive mode: prompt for missing required fields
	if interactive || (summary == "") {
		if issueType == "" {
			types, err := client.GetIssueTypes(project)
			if err != nil {
				return fmt.Errorf("fetching issue types: %w", err)
			}
			selected, err := ui.SelectIssueType(types)
			if err != nil {
				return err
			}
			issueType = selected.Name
		}

		if summary == "" {
			var err error
			summary, description, err = ui.PromptCreateFields(project, issueType)
			if err != nil {
				return err
			}
		}
	}

	if summary == "" {
		return fmt.Errorf("summary is required (use -s or --interactive)")
	}

	// Build fields
	fields := map[string]interface{}{
		"project":   map[string]string{"key": project},
		"issuetype": map[string]string{"name": issueType},
		"summary":   summary,
	}

	if description != "" {
		fields["description"] = jira.MakeADFBody(description)
	}

	if assigneeID != "" {
		fields["assignee"] = map[string]string{"accountId": assigneeID}
	} else if assigneeName != "" {
		accountID, err := resolveAssignee(assigneeName)
		if err != nil {
			return err
		}
		fields["assignee"] = map[string]string{"accountId": accountID}
	}

	if priority != "" {
		fields["priority"] = map[string]string{"name": priority}
	}

	if len(labels) > 0 {
		fields["labels"] = labels
	}

	if component != "" {
		fields["components"] = []map[string]string{{"name": component}}
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

	resp, err := client.CreateIssue(fields)
	if err != nil {
		return err
	}

	fmt.Printf("Created %s\n", resp.Key)
	fmt.Printf("https://%s/browse/%s\n", cfg.Jira.Instance, resp.Key)
	return nil
}
