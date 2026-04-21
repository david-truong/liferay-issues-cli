package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/david-truong/liferay-issues-cli/internal/ui"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

var viewCmd = &cobra.Command{
	Use:   "view [TICKET]",
	Short: "View a Jira issue",
	Long:  "Display details of a Jira issue. If no ticket is specified, extracts from current git branch.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  viewRun,
}

func init() {
	viewCmd.Flags().BoolP("json", "j", false, "output raw JSON")
	viewCmd.Flags().StringP("field", "f", "", "extract a specific field (jq-style path, e.g. .fields.summary)")
	viewCmd.Flags().BoolP("text", "t", false, "render the selected field's ADF body as plain text (requires --field)")
	viewCmd.Flags().BoolP("web", "w", false, "open in browser")
}

func viewRun(cmd *cobra.Command, args []string) error {
	ticket, err := resolveTicket(args)
	if err != nil {
		return err
	}

	webFlag, _ := cmd.Flags().GetBool("web")
	if webFlag {
		url := "https://" + cfg.Jira.Instance + "/browse/" + ticket
		fmt.Println(url)
		return browser.OpenURL(url)
	}

	if err := initClient(); err != nil {
		return err
	}

	jsonFlag, _ := cmd.Flags().GetBool("json")
	fieldFlag, _ := cmd.Flags().GetString("field")
	textFlag, _ := cmd.Flags().GetBool("text")

	if textFlag && fieldFlag == "" {
		return fmt.Errorf("--text requires --field")
	}
	if textFlag && jsonFlag {
		return fmt.Errorf("--text and --json are mutually exclusive")
	}

	if jsonFlag || fieldFlag != "" {
		raw, err := client.GetIssueRaw(ticket)
		if err != nil {
			return err
		}

		if fieldFlag != "" {
			var data interface{}
			if err := json.Unmarshal(raw, &data); err != nil {
				return err
			}
			result := navigateJSON(data, fieldFlag)
			if textFlag {
				printADFResult(result)
				return nil
			}
			if s, ok := result.(string); ok {
				fmt.Println(s)
			} else {
				out, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(out))
			}
			return nil
		}

		var out json.RawMessage
		json.Unmarshal(raw, &out)
		pretty, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(pretty))
		return nil
	}

	issue, err := client.GetIssue(ticket)
	if err != nil {
		return err
	}

	// If called as root command (no explicit "view" subcommand) and no flags,
	// just print the summary like the original script
	if cmd.CalledAs() == "issues" || cmd.CalledAs() == "" {
		fmt.Println(issue.Fields.Summary)
		return nil
	}

	ui.PrintIssueDetail(issue, cfg.Jira.Instance)
	return nil
}

// printADFResult renders a field value (or slice of values) through the ADF
// text extractor. Array results are printed as blocks separated by blank lines.
func printADFResult(v interface{}) {
	if arr, ok := v.([]interface{}); ok {
		for i, elem := range arr {
			fmt.Println(ui.ExtractText(elem))
			if i < len(arr)-1 {
				fmt.Println()
			}
		}
		return
	}
	fmt.Println(ui.ExtractText(v))
}

// navigateJSON traverses a JSON structure using a dot-separated path.
// Supports paths like ".fields.summary" or "fields.summary".
func navigateJSON(data interface{}, path string) interface{} {
	// Strip leading dot
	if len(path) > 0 && path[0] == '.' {
		path = path[1:]
	}

	if path == "" {
		return data
	}

	parts := splitPath(path)
	current := data

	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			fmt.Fprintf(os.Stderr, "cannot navigate into non-object at %q\n", part)
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}

	return current
}

func splitPath(path string) []string {
	var parts []string
	current := ""
	for _, c := range path {
		if c == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
