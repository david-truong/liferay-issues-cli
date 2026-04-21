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
	viewCmd.Flags().StringArrayP("field", "f", nil, "extract a field (jq-style path, e.g. .fields.summary); repeat for multiple fields")
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
	fieldFlags, _ := cmd.Flags().GetStringArray("field")

	if jsonFlag || len(fieldFlags) > 0 {
		raw, err := client.GetIssueRaw(ticket)
		if err != nil {
			return err
		}

		if len(fieldFlags) > 0 {
			var data interface{}
			if err := json.Unmarshal(raw, &data); err != nil {
				return err
			}
			if len(fieldFlags) == 1 {
				printField(navigateJSON(data, fieldFlags[0]))
			} else {
				for _, f := range fieldFlags {
					fmt.Printf("%s: ", f)
					printFieldCompact(navigateJSON(data, f))
				}
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

// printField prints a single field value: strings raw, everything else as indented JSON.
func printField(v interface{}) {
	if s, ok := v.(string); ok {
		fmt.Println(s)
		return
	}
	out, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(out))
}

// printFieldCompact prints a field value on one line (used in multi-field labeled output).
func printFieldCompact(v interface{}) {
	if s, ok := v.(string); ok {
		fmt.Println(s)
		return
	}
	out, _ := json.Marshal(v)
	fmt.Println(string(out))
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
