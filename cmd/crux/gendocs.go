package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var genManDir string
var genDocsDir string

var genManCmd = &cobra.Command{
	Use:    "__gen-man",
	Short:  "Generate man pages",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := os.MkdirAll(genManDir, 0o755); err != nil {
			return err
		}
		header := &doc.GenManHeader{
			Title:   "CRUX",
			Section: "1",
		}
		return doc.GenManTree(rootCmd, header, genManDir)
	},
}

var genDocsCmd = &cobra.Command{
	Use:    "__gen-docs",
	Short:  "Generate markdown documentation",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := os.MkdirAll(genDocsDir, 0o755); err != nil {
			return err
		}
		rootCmd.DisableAutoGenTag = true
		return doc.GenMarkdownTree(rootCmd, genDocsDir)
	},
}

func init() {
	genManCmd.Flags().StringVar(&genManDir, "dir", "man/man1", "output directory for man pages")
	genDocsCmd.Flags().StringVar(&genDocsDir, "dir", "docs/reference", "output directory for markdown docs")

	rootCmd.AddCommand(genManCmd)
	rootCmd.AddCommand(genDocsCmd)
}
