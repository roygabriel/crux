package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show world state (agents, phases, progress)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("no active session")
		return nil
	},
}
