package cmd

import (
	"fmt"
	"net/http"
	"os"

	"github.com/david-truong/liferay-issues-cli/internal/config"
	"github.com/david-truong/liferay-issues-cli/internal/git"
	"github.com/david-truong/liferay-issues-cli/internal/jira"
	"github.com/spf13/cobra"
)

var (
	cfg        *config.Config
	authHeader http.Header
	client     *jira.Client
	debug      bool
)

var rootCmd = &cobra.Command{
	Use:     "issues",
	Short:   "CLI for managing Liferay Jira tickets",
	Long:    "A command-line tool for creating, viewing, updating, and transitioning Jira issues on Liferay's Atlassian instance.",
	Version: config.Version,
	// Default behavior: if no subcommand, act like `issues view`
	RunE: func(cmd *cobra.Command, args []string) error {
		return viewRun(cmd, args)
	},
	SilenceUsage: true,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug output")
	rootCmd.PersistentFlags().Bool("no-color", false, "disable color output")

	rootCmd.AddCommand(viewCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(transitionCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(jqlCmd)
	rootCmd.AddCommand(commentCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(boardCmd)
	rootCmd.AddCommand(sprintCmd)
	rootCmd.AddCommand(findCmd)
	rootCmd.AddCommand(configCmd)
}

func initConfig() {
	var err error
	cfg, err = config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
}

// initClient lazily initializes the Jira client (only when needed).
func initClient() error {
	if client != nil {
		return nil
	}

	var err error
	authHeader, err = config.ResolveAuth(cfg)
	if err != nil {
		return err
	}

	client = jira.NewClient(cfg.Jira.Instance, authHeader)
	client.Debug = debug
	return nil
}

// resolveTicket resolves a ticket ID from args or git branch.
func resolveTicket(args []string) (string, error) {
	if len(args) > 0 {
		ticket, err := git.ExtractTicketFromString(args[0])
		if err != nil {
			// If it doesn't match the pattern, use it as-is (might be a full key)
			return args[0], nil
		}
		return ticket, nil
	}

	ticket, err := git.ExtractTicket()
	if err != nil {
		return "", fmt.Errorf("no ticket specified and could not extract from git branch: %w", err)
	}
	return ticket, nil
}
