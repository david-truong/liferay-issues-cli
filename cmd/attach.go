package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var attachCmd = &cobra.Command{
	Use:   "attach <TICKET> <FILE>...",
	Short: "Attach files to a Jira issue",
	Long: `Attach one or more files to a Jira issue.

All files are uploaded in a single request, and each attachment's ID, filename,
and size is printed.

Examples:
  issues attach LPD-123 mockup.png
  issues attach LPD-123 mockup.png trace.log`,
	Args: cobra.MinimumNArgs(2),
	RunE: attachRun,
}

func attachRun(cmd *cobra.Command, args []string) error {
	ticket, err := resolveTicket(args[:1])
	if err != nil {
		return err
	}

	return uploadAttachments(ticket, args[1:])
}

// uploadAttachments uploads files to a ticket and prints each attachment.
func uploadAttachments(ticket string, paths []string) error {
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("attachment %q: %w", path, err)
		}
		if info.IsDir() {
			return fmt.Errorf("attachment %q is a directory", path)
		}
	}

	if err := initClient(); err != nil {
		return err
	}

	attachments, err := client.AddAttachments(ticket, paths)
	if err != nil {
		return fmt.Errorf("attaching files to %s: %w", ticket, err)
	}

	for _, a := range attachments {
		fmt.Printf("Attached %s (id %s, %d bytes) to %s\n", a.Filename, a.ID, a.Size, ticket)
	}
	return nil
}
