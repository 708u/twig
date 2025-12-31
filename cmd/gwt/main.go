package main

import (
	"os"

	"github.com/708u/gwt"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gwt",
	Short: "Git worktree helper",
}

var addCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Create a new worktree with a new branch",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return gwt.Add(args[0])
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
