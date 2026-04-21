package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

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
	fieldFlags, _ := cmd.Flags().GetStringArray("field")
	textFlag, _ := cmd.Flags().GetBool("text")

	if textFlag && len(fieldFlags) == 0 {
		return fmt.Errorf("--text requires --field")
	}
	if textFlag && jsonFlag {
		return fmt.Errorf("--text and --json are mutually exclusive")
	}

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
				result := navigateJSON(data, fieldFlags[0])
				if textFlag {
					printADFResult(result)
				} else {
					printField(result, true)
				}
			} else {
				for _, f := range fieldFlags {
					result := navigateJSON(data, f)
					if textFlag {
						fmt.Printf("%s:\n", f)
						printADFResult(result)
					} else {
						fmt.Printf("%s: ", f)
						printField(result, false)
					}
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

func printField(v interface{}, indent bool) {
	if s, ok := v.(string); ok {
		fmt.Println(s)
		return
	}
	var out []byte
	if indent {
		out, _ = json.MarshalIndent(v, "", "  ")
	} else {
		out, _ = json.Marshal(v)
	}
	fmt.Println(string(out))
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
// Supports paths like ".fields.summary", "fields.labels[0]", or
// ".fields.issuelinks[].outwardIssue.key".
func navigateJSON(data interface{}, path string) interface{} {
	if len(path) > 0 && path[0] == '.' {
		path = path[1:]
	}
	if path == "" {
		return data
	}
	return navigateParts(data, splitPath(path))
}

func navigateParts(data interface{}, parts []string) interface{} {
	if len(parts) == 0 {
		return data
	}

	part := parts[0]
	rest := parts[1:]

	if part == "[]" {
		arr, ok := data.([]interface{})
		if !ok {
			fmt.Fprintf(os.Stderr, "cannot iterate non-array at \"[]\"\n")
			return nil
		}
		if len(rest) == 0 {
			return arr
		}
		results := make([]interface{}, 0, len(arr))
		for _, elem := range arr {
			if r := navigateParts(elem, rest); r != nil {
				results = append(results, r)
			}
		}
		return results
	}

	if len(part) > 2 && part[0] == '[' && part[len(part)-1] == ']' {
		idx, err := strconv.Atoi(part[1 : len(part)-1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid index %q\n", part)
			return nil
		}
		arr, ok := data.([]interface{})
		if !ok {
			fmt.Fprintf(os.Stderr, "cannot index into non-array at %q\n", part)
			return nil
		}
		if idx < 0 || idx >= len(arr) {
			return nil
		}
		return navigateParts(arr[idx], rest)
	}

	m, ok := data.(map[string]interface{})
	if !ok {
		fmt.Fprintf(os.Stderr, "cannot navigate into non-object at %q\n", part)
		return nil
	}
	val, ok := m[part]
	if !ok {
		return nil
	}
	return navigateParts(val, rest)
}

func splitPath(path string) []string {
	var parts []string
	for _, seg := range strings.Split(path, ".") {
		if seg == "" {
			continue
		}
		if i := strings.Index(seg, "["); i >= 0 {
			if i > 0 {
				parts = append(parts, seg[:i])
			}
			parts = append(parts, seg[i:])
		} else {
			parts = append(parts, seg)
		}
	}
	return parts
}
