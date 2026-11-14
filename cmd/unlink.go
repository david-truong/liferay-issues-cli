package cmd

import (
	"fmt"
	"strings"

	"github.com/david-truong/liferay-issues-cli/internal/jira"
	"github.com/spf13/cobra"
)

var unlinkCmd = &cobra.Command{
	Use:   "unlink <TICKET> [RELATION] <TICKET>",
	Short: "Remove a link between two Jira issues",
	Long: `Remove a link between two Jira issues.

With two tickets, removes the only link between them. When they have several
links, pass the relation the same way as "issues link" to pick one; it is
matched against the types of the links that exist between the two tickets.

Examples:
  issues unlink LPD-1 LPD-2
  issues unlink LPD-1 blocks LPD-2
  issues unlink LPD-1 "is blocked by" LPD-2`,
	Args: cobra.RangeArgs(2, 3),
	RunE: unlinkRun,
}

func unlinkRun(cmd *cobra.Command, args []string) error {
	from, err := resolveTicket(args[:1])
	if err != nil {
		return err
	}
	to, err := resolveTicket(args[len(args)-1:])
	if err != nil {
		return err
	}

	var relation string
	if len(args) == 3 {
		relation = args[1]
	}

	if err := initClient(); err != nil {
		return err
	}

	links, err := client.GetIssueLinks(from)
	if err != nil {
		return fmt.Errorf("fetching links on %s: %w", from, err)
	}

	link, err := selectIssueLink(links, from, to, relation)
	if err != nil {
		return err
	}

	if err := client.DeleteIssueLink(link.ID); err != nil {
		return fmt.Errorf("removing link %s: %w", describeIssueLink(from, link), err)
	}

	fmt.Printf("Removed link %s\n", describeIssueLink(from, link))
	return nil
}

// selectIssueLink picks the one link on from whose other side is to. An empty
// relation requires exactly one such link; otherwise the relation is resolved
// against the types of those links only, and a symmetric type (same inward and
// outward phrase) matches in either direction.
func selectIssueLink(links []jira.IssueLink, from, to, relation string) (jira.IssueLink, error) {
	var between []jira.IssueLink
	for _, l := range links {
		other := l.OutwardIssue
		if other == nil {
			other = l.InwardIssue
		}
		if other != nil && strings.EqualFold(other.Key, to) {
			between = append(between, l)
		}
	}

	if len(between) == 0 {
		return jira.IssueLink{}, fmt.Errorf("%s has no link to %s", from, to)
	}

	candidates := between
	if relation != "" {
		var types []jira.IssueLinkType
		seen := map[string]bool{}
		for _, l := range between {
			if !seen[l.Type.Name] {
				seen[l.Type.Name] = true
				types = append(types, l.Type)
			}
		}

		linkType, inward, err := resolveLinkRelation(types, relation)
		if err != nil {
			return jira.IssueLink{}, fmt.Errorf("%w; links from %s to %s: %s", err, from, to, describeIssueLinks(from, between))
		}

		candidates = nil
		for _, l := range between {
			if l.Type.Name != linkType.Name {
				continue
			}
			if l.Type.Inward == l.Type.Outward || inward == (l.InwardIssue != nil) {
				candidates = append(candidates, l)
			}
		}
	}

	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) == 0 {
		return jira.IssueLink{}, fmt.Errorf("%s has no %q link to %s; links: %s", from, relation, to, describeIssueLinks(from, between))
	}
	return jira.IssueLink{}, fmt.Errorf("%s has %d links to %s: %s; pass the relation to pick one", from, len(candidates), to, describeIssueLinks(from, candidates))
}

// describeIssueLink renders a link on from as a sentence with its outward
// phrase, e.g. "LPD-1 blocks LPD-2 (Blocks)".
func describeIssueLink(from string, l jira.IssueLink) string {
	if l.OutwardIssue != nil {
		return fmt.Sprintf("%s %s %s (%s)", from, l.Type.Outward, l.OutwardIssue.Key, l.Type.Name)
	}
	return fmt.Sprintf("%s %s %s (%s)", l.InwardIssue.Key, l.Type.Outward, from, l.Type.Name)
}

func describeIssueLinks(from string, links []jira.IssueLink) string {
	var descriptions []string
	for _, l := range links {
		descriptions = append(descriptions, describeIssueLink(from, l))
	}
	return strings.Join(descriptions, ", ")
}
