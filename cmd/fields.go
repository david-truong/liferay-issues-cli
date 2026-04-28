package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
)

const pullRequestField = "customfield_10201"

// assigneeClause renders an assignee JQL clause, treating "me" as currentUser().
func assigneeClause(v string) string {
	if v == "me" {
		return "assignee = currentUser()"
	}
	return fmt.Sprintf("assignee = %q", v)
}

// parseFieldValue attempts to parse v as JSON if it looks like an object or
// array. Otherwise it returns v as a plain string. This lets --field accept
// both simple strings and structured values like {"value":"foo"}.
func parseFieldValue(v string) interface{} {
	if len(v) > 0 && (v[0] == '{' || v[0] == '[') {
		var parsed interface{}
		if err := json.Unmarshal([]byte(v), &parsed); err == nil {
			return parsed
		}
	}
	return v
}

// resolveAssignee searches for a Jira user by display name and returns the
// account ID. If multiple users match, it returns the first exact match or
// an error listing the options.
func resolveAssignee(name string) (string, error) {
	users, err := client.SearchUsers(name)
	if err != nil {
		return "", fmt.Errorf("searching for user %q: %w", name, err)
	}
	if len(users) == 0 {
		return "", fmt.Errorf("no users found matching %q", name)
	}

	// Exact match (case-insensitive)
	for _, u := range users {
		if strings.EqualFold(u.DisplayName, name) {
			return u.AccountID, nil
		}
	}

	// Single result
	if len(users) == 1 {
		return users[0].AccountID, nil
	}

	// Multiple results, no exact match — list them
	var names []string
	for _, u := range users {
		names = append(names, fmt.Sprintf("  %s (%s)", u.DisplayName, u.AccountID))
	}
	return "", fmt.Errorf("multiple users match %q, use --assignee-id with one of:\n%s", name, strings.Join(names, "\n"))
}
