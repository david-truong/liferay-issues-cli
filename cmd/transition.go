package cmd

import (
	"fmt"
	"strings"

	"github.com/david-truong/liferay-issues-cli/internal/jira"
	"github.com/david-truong/liferay-issues-cli/internal/ui"
	"github.com/spf13/cobra"
)

// statusChain describes the forward status ordering for a given project and
// set of issue types. The walker handles both directions, so a single chain
// covers forward progress and backward returns.
type statusChain struct {
	project    string
	issueTypes []string // empty matches any
	chain      []string
}

var statusChains = []statusChain{
	{
		project:    "LPD",
		issueTypes: []string{"Story", "Bug", "Task"},
		chain:      []string{"Open", "Selected for Development", "In Progress", "In Peer Review", "Closed"},
	},
	{
		project:    "LPD",
		issueTypes: []string{"Technical Task"},
		chain:      []string{"Open", "In Progress", "In Peer Review", "Closed"},
	},
	{
		project:    "BPR",
		issueTypes: []string{"Backport Request"},
		chain:      []string{"Open", "Awaiting Original Fix", "In Review", "Closed"},
	},
}

// chainContaining returns the chain matching the given project + issue type
// that contains both current and target (and where they are distinct), or nil
// if none does.
func chainContaining(project, issueType, current, target string) []string {
	for _, sc := range statusChains {
		if sc.project != "" && !strings.EqualFold(sc.project, project) {
			continue
		}
		if !matchesIssueType(sc.issueTypes, issueType) {
			continue
		}
		ci, ti := chainIndex(sc.chain, current), chainIndex(sc.chain, target)
		if ci >= 0 && ti >= 0 && ci != ti {
			return sc.chain
		}
	}
	return nil
}

func matchesIssueType(allowed []string, issueType string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, t := range allowed {
		if strings.EqualFold(t, issueType) {
			return true
		}
	}
	return false
}

func chainIndex(chain []string, status string) int {
	for i, s := range chain {
		if strings.EqualFold(s, status) {
			return i
		}
	}
	return -1
}

// pickChainStep selects the next transition to take from `current` toward
// `target` along `chain`. Prefers transitions that move strictly toward target
// without overshooting; falls back to any chain status closer to target than
// current (e.g. routing backward via "Open" when no direct backward edge
// exists). Returns nil if no chain-aware step is available.
func pickChainStep(transitions []jira.Transition, chain []string, current, target string) *jira.Transition {
	currentIdx := chainIndex(chain, current)
	targetIdx := chainIndex(chain, target)
	if currentIdx < 0 || targetIdx < 0 {
		return nil
	}
	forward := targetIdx > currentIdx

	var best *jira.Transition
	bestIdx := -1
	for j, t := range transitions {
		idx := chainIndex(chain, t.To.Name)
		if idx < 0 {
			continue
		}
		inRange := (forward && idx > currentIdx && idx <= targetIdx) ||
			(!forward && idx >= targetIdx && idx < currentIdx)
		if !inRange {
			continue
		}
		if best == nil || (forward && idx > bestIdx) || (!forward && idx < bestIdx) {
			best = &transitions[j]
			bestIdx = idx
		}
	}
	if best != nil {
		return best
	}

	// Fallback: pick the chain status that gets us closest to target, even if
	// it overshoots. Lets us escape dead ends like "In Progress → Open → SfD".
	bestDist := -1
	for j, t := range transitions {
		idx := chainIndex(chain, t.To.Name)
		if idx < 0 {
			continue
		}
		dist := idx - targetIdx
		if dist < 0 {
			dist = -dist
		}
		curDist := currentIdx - targetIdx
		if curDist < 0 {
			curDist = -curDist
		}
		if dist > curDist {
			continue // don't pick a step that lands farther from target
		}
		if best == nil || dist < bestDist {
			best = &transitions[j]
			bestDist = dist
		}
	}
	return best
}

func formatAvailable(transitions []jira.Transition) string {
	names := make([]string, 0, len(transitions))
	for _, t := range transitions {
		names = append(names, t.To.Name)
	}
	return strings.Join(names, ", ")
}

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

	fields, err := buildTransitionFields(cmd)
	if err != nil {
		return err
	}

	var transitionID string
	var transitionName string

	if len(args) > 1 {
		target := args[1]
		t := ui.FindTransitionByName(transitions, target)
		if t == nil {
			// No direct transition — try walking a known status chain.
			issue, ierr := client.GetIssue(ticket)
			if ierr != nil {
				return fmt.Errorf("fetching issue: %w", ierr)
			}
			current := ""
			if issue.Fields.Status != nil {
				current = issue.Fields.Status.Name
			}
			if strings.EqualFold(current, target) {
				fmt.Printf("%s is already in status %q\n", ticket, current)
				return nil
			}
			project := ""
			if issue.Fields.Project != nil {
				project = issue.Fields.Project.Key
			}
			issueType := ""
			if issue.Fields.IssueType != nil {
				issueType = issue.Fields.IssueType.Name
			}
			if chain := chainContaining(project, issueType, current, target); chain != nil {
				return walkToTarget(ticket, current, target, chain, comment, fields)
			}
			fmt.Println("Available transitions:")
			for _, t := range transitions {
				fmt.Printf("  %s → %s\n", t.Name, t.To.Name)
			}
			return fmt.Errorf("no matching transition for %q", target)
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

	if err := client.DoTransition(ticket, transitionID, comment, fields); err != nil {
		return err
	}

	fmt.Printf("Transitioned %s via %q\n", ticket, transitionName)
	return nil
}

func buildTransitionFields(cmd *cobra.Command) (map[string]interface{}, error) {
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
				return nil, fmt.Errorf("invalid --field format %q (expected key=value)", f)
			}
			fields[k] = v
		}
	}

	return fields, nil
}

// walkToTarget steps the issue toward `target` along `chain`, picking the
// next move from the live available-transitions list at each step. The
// user-supplied comment and fields are applied only on the final step so
// intermediate statuses don't get spammed.
func walkToTarget(ticket, current, target string, chain []string, comment string, finalFields map[string]interface{}) error {
	visited := map[string]bool{}
	fmt.Printf("Walking %s: %s", ticket, current)
	for !strings.EqualFold(current, target) {
		key := strings.ToLower(current)
		if visited[key] {
			fmt.Println()
			return fmt.Errorf("walk loop detected at %q", current)
		}
		visited[key] = true

		transitions, err := client.GetTransitions(ticket)
		if err != nil {
			fmt.Println()
			return fmt.Errorf("fetching transitions at %q: %w", current, err)
		}
		next := pickChainStep(transitions, chain, current, target)
		if next == nil {
			fmt.Println()
			return fmt.Errorf("no chain transition from %q toward %q (available: %s)", current, target, formatAvailable(transitions))
		}

		var stepComment string
		var stepFields map[string]interface{}
		if strings.EqualFold(next.To.Name, target) {
			stepComment = comment
			stepFields = finalFields
		}
		if err := client.DoTransition(ticket, next.ID, stepComment, stepFields); err != nil {
			fmt.Println()
			return fmt.Errorf("transitioning %q → %q: %w", current, next.To.Name, err)
		}
		fmt.Printf(" → %s", next.To.Name)
		current = next.To.Name
	}
	fmt.Println()
	return nil
}
