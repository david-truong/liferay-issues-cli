package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/david-truong/liferay-issues-cli/internal/jira"
)

// SelectTransition prompts the user to select a transition.
func SelectTransition(transitions []jira.Transition) (*jira.Transition, error) {
	if len(transitions) == 0 {
		return nil, fmt.Errorf("no transitions available")
	}

	var options []huh.Option[string]
	for _, t := range transitions {
		label := fmt.Sprintf("%s → %s", t.Name, t.To.Name)
		options = append(options, huh.NewOption(label, t.ID))
	}

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select transition").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	for _, t := range transitions {
		if t.ID == selected {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("transition not found")
}

// SelectIssueType prompts the user to select an issue type.
func SelectIssueType(types []jira.IssueType) (*jira.IssueType, error) {
	if len(types) == 0 {
		return nil, fmt.Errorf("no issue types available")
	}

	var options []huh.Option[string]
	for _, t := range types {
		options = append(options, huh.NewOption(t.Name, t.ID))
	}

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Issue type").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	for _, t := range types {
		if t.ID == selected {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("issue type not found")
}

// PromptCreateFields interactively prompts for issue creation fields.
func PromptCreateFields(projectKey string, issueTypeName string) (summary, description string, err error) {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Summary").
				Prompt("> ").
				Value(&summary),
			huh.NewText().
				Title("Description").
				Value(&description),
		),
	)

	if err := form.Run(); err != nil {
		return "", "", err
	}

	return summary, description, nil
}

// Confirm asks for yes/no confirmation.
func Confirm(message string) (bool, error) {
	var confirmed bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(message).
				Value(&confirmed),
		),
	)

	if err := form.Run(); err != nil {
		return false, err
	}
	return confirmed, nil
}

// OpenEditor opens the user's preferred editor and returns the content.
func OpenEditor(initial string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	tmpFile, err := os.CreateTemp("", "issues-*.md")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())

	if initial != "" {
		tmpFile.WriteString(initial)
	}
	tmpFile.Close()

	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor failed: %w", err)
	}

	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(content)), nil
}

// SelectBoard prompts the user to select a board from a list.
func SelectBoard(boards []jira.Board) (*jira.Board, error) {
	if len(boards) == 0 {
		return nil, fmt.Errorf("no boards available")
	}

	var options []huh.Option[int]
	for _, b := range boards {
		label := fmt.Sprintf("%s (%s)", b.Name, b.Type)
		options = append(options, huh.NewOption(label, b.ID))
	}

	var selected int
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("Select board").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	for _, b := range boards {
		if b.ID == selected {
			return &b, nil
		}
	}
	return nil, fmt.Errorf("board not found")
}

// SelectSprint prompts the user to select a sprint from a list.
func SelectSprint(sprints []jira.Sprint) (*jira.Sprint, error) {
	if len(sprints) == 0 {
		return nil, fmt.Errorf("no sprints available")
	}

	var options []huh.Option[int]
	for _, s := range sprints {
		label := fmt.Sprintf("%s (%s)", s.Name, s.State)
		options = append(options, huh.NewOption(label, s.ID))
	}

	var selected int
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("Select sprint").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	for _, s := range sprints {
		if s.ID == selected {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("sprint not found")
}

// SelectComponent prompts the user to select a component from a list.
func SelectComponent(components []jira.Component) (*jira.Component, error) {
	if len(components) == 0 {
		return nil, fmt.Errorf("no components available")
	}

	var options []huh.Option[string]
	for _, c := range components {
		options = append(options, huh.NewOption(c.Name, c.ID))
	}

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select component").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	for _, c := range components {
		if c.ID == selected {
			return &c, nil
		}
	}
	return nil, fmt.Errorf("component not found")
}

// FindSprintByName finds a sprint by fuzzy name matching.
func FindSprintByName(sprints []jira.Sprint, name string) []jira.Sprint {
	if name == "" {
		return nil
	}
	name = strings.ToLower(name)

	// Exact match first
	for _, s := range sprints {
		if strings.ToLower(s.Name) == name {
			return []jira.Sprint{s}
		}
	}

	// Contains match
	var matches []jira.Sprint
	for _, s := range sprints {
		if strings.Contains(strings.ToLower(s.Name), name) {
			matches = append(matches, s)
		}
	}
	return matches
}

// FindTransitionByName finds a transition by fuzzy name matching.
func FindTransitionByName(transitions []jira.Transition, name string) *jira.Transition {
	if name == "" {
		return nil
	}
	name = strings.ToLower(name)

	// Exact match first
	for _, t := range transitions {
		if strings.ToLower(t.Name) == name || strings.ToLower(t.To.Name) == name {
			return &t
		}
	}

	// Prefix match
	for _, t := range transitions {
		if strings.HasPrefix(strings.ToLower(t.Name), name) ||
			strings.HasPrefix(strings.ToLower(t.To.Name), name) {
			return &t
		}
	}

	// Contains match
	for _, t := range transitions {
		if strings.Contains(strings.ToLower(t.Name), name) ||
			strings.Contains(strings.ToLower(t.To.Name), name) {
			return &t
		}
	}

	return nil
}
