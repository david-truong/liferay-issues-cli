package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/david-truong/liferay-issues-cli/internal/git"
	"github.com/david-truong/liferay-issues-cli/internal/ui"
	"github.com/spf13/cobra"
)

var commentCmd = &cobra.Command{
	Use:   "comment [TICKET] [MESSAGE]",
	Short: "Add or list comments on a Jira issue",
	Long: `Add or list comments on a Jira issue.

If no ticket is specified, extracts from current git branch.
The message can be provided as a positional argument, via -m flag, -e editor, or stdin.

Examples:
  issues comment "my comment"              # ticket from branch, inline message
  issues comment LPD-123 "my comment"      # explicit ticket, inline message
  issues comment LPD-123 -m "my comment"   # explicit ticket, flag message
  issues comment LPD-123                   # explicit ticket, opens editor
  issues comment                           # ticket from branch, opens editor
  issues comment LPD-123 "see mockup" --attach mockup.png   # upload file(s), then comment

The message is written as Markdown (headings, bold/italic, code, fenced code
blocks, lists, links, blockquotes) and converted to Jira's rich text format.
Files given with --attach are uploaded to the ticket before the comment is
posted; they appear in the ticket's attachments, not inline in the comment.`,
	Args: cobra.MaximumNArgs(2),
	RunE: commentRun,
}

func init() {
	commentCmd.Flags().StringP("message", "m", "", "comment body (Markdown)")
	commentCmd.Flags().Bool("list", false, "list existing comments")
	commentCmd.Flags().StringArray("attach", nil, "file to attach before commenting (repeatable)")
	commentCmd.Flags().BoolP("editor", "e", false, "open editor for comment body")
}

func commentRun(cmd *cobra.Command, args []string) error {
	var ticket, positionalMessage string

	switch len(args) {
	case 2:
		// issues comment LPD-123 "my comment"
		ticket = args[0]
		positionalMessage = args[1]
	case 1:
		// Could be a ticket or a message — check if it looks like a ticket key
		if _, err := git.ExtractTicketFromString(args[0]); err == nil {
			ticket = args[0]
		} else {
			positionalMessage = args[0]
		}
	}

	var err error
	if ticket == "" {
		ticket, err = resolveTicket(nil)
		if err != nil {
			return err
		}
	} else {
		ticket, err = resolveTicket([]string{ticket})
		if err != nil {
			return err
		}
	}

	if err := initClient(); err != nil {
		return err
	}

	listFlag, _ := cmd.Flags().GetBool("list")
	if listFlag {
		comments, err := client.GetComments(ticket)
		if err != nil {
			return err
		}
		if len(comments) == 0 {
			fmt.Println("No comments.")
			return nil
		}
		ui.PrintComments(comments)
		return nil
	}

	attachments, _ := cmd.Flags().GetStringArray("attach")
	for _, path := range attachments {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("attachment %q: %w", path, err)
		}
	}

	message, _ := cmd.Flags().GetString("message")
	editorFlag, _ := cmd.Flags().GetBool("editor")

	// Positional message takes precedence if -m wasn't explicitly set
	if message == "" && positionalMessage != "" {
		message = positionalMessage
	}

	if message == "" && editorFlag {
		message, err = ui.OpenEditor("")
		if err != nil {
			return err
		}
	}

	// Try reading from stdin if piped
	if message == "" {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
			message = string(data)
		}
	}

	if message == "" {
		// Fall back to editor
		message, err = ui.OpenEditor("")
		if err != nil {
			return err
		}
	}

	if message == "" {
		return fmt.Errorf("comment body is required (use -m, -e, or pipe via stdin)")
	}

	if len(attachments) > 0 {
		if err := uploadAttachments(ticket, attachments); err != nil {
			return err
		}
	}

	if _, err = client.AddComment(ticket, message); err != nil {
		if len(attachments) > 0 {
			return fmt.Errorf("files were attached but the comment failed: %w", err)
		}
		return err
	}

	fmt.Printf("Comment added to %s\n", ticket)
	return nil
}
