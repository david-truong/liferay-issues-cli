package ui

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/david-truong/liferay-issues-cli/internal/jira"
)

// PrintIssueDetail prints a formatted view of a single issue.
func PrintIssueDetail(issue *jira.Issue, instance string) {
	fmt.Printf("\033[1m%s\033[0m  %s\n", issue.Key, issue.Fields.Summary)
	fmt.Println(strings.Repeat("─", 60))

	if issue.Fields.Status != nil {
		fmt.Printf("  Status:   %s\n", issue.Fields.Status.Name)
	}
	if issue.Fields.IssueType != nil {
		fmt.Printf("  Type:     %s\n", issue.Fields.IssueType.Name)
	}
	if issue.Fields.Priority != nil {
		fmt.Printf("  Priority: %s\n", issue.Fields.Priority.Name)
	}
	if issue.Fields.Assignee != nil {
		fmt.Printf("  Assignee: %s\n", issue.Fields.Assignee.DisplayName)
	}
	if issue.Fields.Reporter != nil {
		fmt.Printf("  Reporter: %s\n", issue.Fields.Reporter.DisplayName)
	}
	if len(issue.Fields.Labels) > 0 {
		fmt.Printf("  Labels:   %s\n", strings.Join(issue.Fields.Labels, ", "))
	}
	if len(issue.Fields.Components) > 0 {
		var names []string
		for _, c := range issue.Fields.Components {
			names = append(names, c.Name)
		}
		fmt.Printf("  Components: %s\n", strings.Join(names, ", "))
	}
	fmt.Printf("  URL:      https://%s/browse/%s\n", instance, issue.Key)

	desc := ExtractText(issue.Fields.Description)
	if desc != "" {
		fmt.Println()
		fmt.Println(desc)
	}
}

// PrintIssueTable prints a table of issues.
func PrintIssueTable(issues []jira.Issue) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "\033[1mKEY\tSTATUS\tTYPE\tAFFECTS\tFIX VERSION\tASSIGNEE\tSUMMARY\033[0m\n")

	for _, issue := range issues {
		status := ""
		if issue.Fields.Status != nil {
			status = issue.Fields.Status.Name
		}
		issueType := ""
		if issue.Fields.IssueType != nil {
			issueType = issue.Fields.IssueType.Name
		}
		affects := joinVersionNames(issue.Fields.Versions, issue.Fields.Created)
		fixVersion := joinVersionNames(issue.Fields.FixVersions)
		assignee := "Unassigned"
		if issue.Fields.Assignee != nil {
			assignee = issue.Fields.Assignee.DisplayName
		}

		summary := issue.Fields.Summary
		if len(summary) > 60 {
			summary = summary[:57] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			issue.Key, status, issueType, affects, fixVersion, assignee, summary)
	}
	w.Flush()
}

func joinVersionNames(versions []jira.Version, fallbackDate ...string) string {
	if len(versions) == 0 {
		return ""
	}
	var names []string
	for _, v := range versions {
		if strings.EqualFold(v.Name, "master") && len(fallbackDate) > 0 {
			date := formatDate(fallbackDate[0])
			if date != "" {
				names = append(names, v.Name+" ("+date+")")
				continue
			}
		}
		names = append(names, v.Name)
	}
	return strings.Join(names, ", ")
}

// PrintComments prints a list of comments.
func PrintComments(comments []jira.Comment) {
	for i, c := range comments {
		author := "Unknown"
		if c.Author != nil {
			author = c.Author.DisplayName
		}
		created := c.Created
		if t, err := jira.ParseJiraTime(c.Created); err == nil {
			created = t.Format("2006-01-02 15:04")
		}

		fmt.Printf("\033[1m%s\033[0m  %s\n", author, created)
		body := ExtractText(c.Body)
		if body != "" {
			fmt.Println(body)
		}
		if i < len(comments)-1 {
			fmt.Println(strings.Repeat("─", 40))
		}
	}
}

// PrintSprintTable prints a table of sprints.
func PrintSprintTable(sprints []jira.Sprint) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "\033[1mID\tNAME\tSTATE\tSTART\tEND\033[0m\n")

	for _, s := range sprints {
		start := formatDate(s.StartDate)
		end := formatDate(s.EndDate)
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", s.ID, s.Name, s.State, start, end)
	}
	w.Flush()
}

// PrintSprintDetail prints a formatted view of a single sprint.
func PrintSprintDetail(s *jira.Sprint) {
	fmt.Printf("\033[1m%s\033[0m  (ID: %d)\n", s.Name, s.ID)
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("  State:  %s\n", s.State)
	if s.StartDate != "" {
		fmt.Printf("  Start:  %s\n", formatDate(s.StartDate))
	}
	if s.EndDate != "" {
		fmt.Printf("  End:    %s\n", formatDate(s.EndDate))
	}
	if s.CompleteDate != "" {
		fmt.Printf("  Completed: %s\n", formatDate(s.CompleteDate))
	}
	if s.Goal != "" {
		fmt.Printf("\n  Goal: %s\n", s.Goal)
	}
}

func formatDate(s string) string {
	if s == "" {
		return ""
	}
	if t, err := jira.ParseJiraTime(s); err == nil {
		return t.Format("2006-01-02")
	}
	// Try RFC3339 format (used by agile API)
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// ExtractText pulls plain text from either a string or ADF document.
func ExtractText(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}

	// Handle ADF (Atlassian Document Format)
	doc, ok := v.(map[string]interface{})
	if !ok {
		return fmt.Sprintf("%v", v)
	}

	content, ok := doc["content"].([]interface{})
	if !ok {
		return ""
	}

	var parts []string
	for _, block := range content {
		blockMap, ok := block.(map[string]interface{})
		if !ok {
			continue
		}
		parts = append(parts, extractBlockText(blockMap))
	}
	return strings.Join(parts, "\n")
}

func extractBlockText(block map[string]interface{}) string {
	blockType, _ := block["type"].(string)

	switch blockType {
	case "paragraph", "heading":
		return extractInlineText(block)
	case "bulletList", "orderedList":
		items, ok := block["content"].([]interface{})
		if !ok {
			return ""
		}
		var lines []string
		for i, item := range items {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			prefix := "  - "
			if blockType == "orderedList" {
				prefix = fmt.Sprintf("  %d. ", i+1)
			}
			lines = append(lines, prefix+extractInlineText(itemMap))
		}
		return strings.Join(lines, "\n")
	case "codeBlock":
		return "```\n" + extractInlineText(block) + "\n```"
	case "listItem":
		return extractInlineText(block)
	default:
		return extractInlineText(block)
	}
}

func extractInlineText(block map[string]interface{}) string {
	content, ok := block["content"].([]interface{})
	if !ok {
		return ""
	}

	var parts []string
	for _, inline := range content {
		inlineMap, ok := inline.(map[string]interface{})
		if !ok {
			continue
		}
		inlineType, _ := inlineMap["type"].(string)
		switch inlineType {
		case "text":
			text, _ := inlineMap["text"].(string)
			parts = append(parts, text)
		case "hardBreak":
			parts = append(parts, "\n")
		case "mention":
			attrs, _ := inlineMap["attrs"].(map[string]interface{})
			text, _ := attrs["text"].(string)
			parts = append(parts, text)
		case "inlineCard":
			attrs, _ := inlineMap["attrs"].(map[string]interface{})
			u, _ := attrs["url"].(string)
			parts = append(parts, u)
		case "paragraph", "listItem":
			parts = append(parts, extractInlineText(inlineMap))
		default:
			text, _ := inlineMap["text"].(string)
			if text != "" {
				parts = append(parts, text)
			} else {
				parts = append(parts, extractInlineText(inlineMap))
			}
		}
	}
	return strings.Join(parts, "")
}
