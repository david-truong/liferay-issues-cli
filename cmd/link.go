package cmd

import (
	"fmt"
	"strings"

	"github.com/david-truong/liferay-issues-cli/internal/jira"
	"github.com/david-truong/liferay-issues-cli/internal/ui"
	"github.com/spf13/cobra"
)

var linkCmd = &cobra.Command{
	Use:   "link <TICKET> <RELATION> <TICKET>",
	Short: "Link two Jira issues",
	Long: `Link two Jira issues. The command reads as a sentence: the relation is a
link type name or either of its phrases. An inward phrase such as
"is blocked by" links the tickets in the reverse direction.

A type name always means its outward phrase, which does not always read like
the name: "Depends" is "is a dependency of", so "A Depends B" records that A is
a dependency of B. Prefer the phrases.

A type name beats a phrase and an exact-case match beats a case-insensitive
one; a remaining tie is an error.

Run "issues link --types" to list the link types and their phrases.

Examples:
  issues link LPD-1 blocks LPD-2
  issues link LPD-1 "is blocked by" LPD-2
  issues link LPD-1 relates LPD-2
  issues link LPD-1 duplicates LPD-2
  issues link --types`,
	Args: func(cmd *cobra.Command, args []string) error {
		if typesFlag, _ := cmd.Flags().GetBool("types"); typesFlag {
			return cobra.ExactArgs(0)(cmd, args)
		}
		return cobra.ExactArgs(3)(cmd, args)
	},
	RunE: linkRun,
}

func init() {
	linkCmd.Flags().Bool("types", false, "list the available link types")
}

func linkRun(cmd *cobra.Command, args []string) error {
	if err := initClient(); err != nil {
		return err
	}

	types, err := client.GetIssueLinkTypes()
	if err != nil {
		return fmt.Errorf("fetching issue link types: %w", err)
	}

	if typesFlag, _ := cmd.Flags().GetBool("types"); typesFlag {
		ui.PrintIssueLinkTypeTable(types)
		return nil
	}

	from, err := resolveTicket(args[:1])
	if err != nil {
		return err
	}
	to, err := resolveTicket(args[2:])
	if err != nil {
		return err
	}

	linkType, inward, err := resolveLinkRelation(types, args[1])
	if err != nil {
		return err
	}

	inwardKey, outwardKey := from, to
	if inward {
		inwardKey, outwardKey = to, from
	}

	if err := client.LinkIssues(linkType.Name, inwardKey, outwardKey); err != nil {
		return fmt.Errorf("linking %s to %s: %w", from, to, err)
	}

	fmt.Printf("Linked %s %s %s\n", inwardKey, linkType.Outward, outwardKey)
	return nil
}

// resolveLinkRelation finds the link type whose name or phrase matches
// relation. inward reports whether relation is the type's inward phrase, in
// which case the tickets must be swapped.
//
// Instances define overlapping types (e.g. "Blocks" with "is blocked by" and
// "Blocked" with "Is blocked by"), so a type name beats a phrase and an
// exact-case match beats a case-insensitive one. Only a tie at the best rank
// is ambiguous.
func resolveLinkRelation(types []jira.IssueLinkType, relation string) (jira.IssueLinkType, bool, error) {
	relation = strings.TrimSpace(relation)

	type match struct {
		linkType jira.IssueLinkType
		inward   bool
	}

	var matches []match
	var bestRank int
	for _, t := range types {
		rank, inward := -1, false
		switch {
		case t.Name == relation:
			rank = 0
		case strings.EqualFold(t.Name, relation):
			rank = 1
		case t.Outward == relation:
			rank = 2
		case t.Inward == relation:
			rank, inward = 2, true
		case strings.EqualFold(t.Outward, relation):
			rank = 3
		case strings.EqualFold(t.Inward, relation):
			rank, inward = 3, true
		}
		if rank < 0 || (len(matches) > 0 && rank > bestRank) {
			continue
		}
		if len(matches) > 0 && rank < bestRank {
			matches = nil
		}
		bestRank = rank
		matches = append(matches, match{t, inward})
	}

	if len(matches) == 1 {
		return matches[0].linkType, matches[0].inward, nil
	}

	if len(matches) == 0 {
		var phrases []string
		for _, t := range types {
			phrases = append(phrases, fmt.Sprintf("%q (%s / %s)", t.Name, t.Outward, t.Inward))
		}
		return jira.IssueLinkType{}, false, fmt.Errorf("unknown link relation %q; valid relations: %s", relation, strings.Join(phrases, ", "))
	}

	var names []string
	for _, m := range matches {
		names = append(names, m.linkType.Name)
	}
	return jira.IssueLinkType{}, false, fmt.Errorf("link relation %q is ambiguous between types %s; use the exact-case phrase or the type name", relation, strings.Join(names, ", "))
}
