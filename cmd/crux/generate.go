package main

import (
	"fmt"

	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/docgen"
	"github.com/spf13/cobra"
)

var (
	genPhase     string
	genDryRun    bool
	genModel     string
	genMaxTokens int
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate project artifacts using AI",
}

var generateDocsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Generate phase spec and prompt documents",
	RunE:  runGenerateDocs,
}

func init() {
	generateDocsCmd.Flags().StringVar(&genPhase, "phase", "", "single phase ID for streaming mode")
	generateDocsCmd.Flags().BoolVar(&genDryRun, "dry-run", false, "estimate cost without calling API")
	generateDocsCmd.Flags().StringVar(&genModel, "model", "", "model override")
	generateDocsCmd.Flags().IntVar(&genMaxTokens, "max-tokens", 0, "output token limit override")

	generateCmd.AddCommand(generateDocsCmd)
}

func runGenerateDocs(cmd *cobra.Command, args []string) error {
	log := setupLogger()

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	apiKey := resolveAPIKey()
	if apiKey == "" {
		fmt.Println("No Anthropic API key found.")
		fmt.Println("Set CRUX_ANTHROPIC_API_KEY or ANTHROPIC_API_KEY.")
		return nil
	}

	opts := docgen.GenerateOptions{
		Model:     cfg.Docgen.Model,
		MaxTokens: cfg.Docgen.MaxTokens,
		OutputDir: cfg.Docgen.OutputDir,
		DryRun:    genDryRun,
	}

	// CLI flags override config values.
	if genModel != "" {
		opts.Model = genModel
	}
	if genMaxTokens > 0 {
		opts.MaxTokens = genMaxTokens
	}

	// Select mode based on --phase flag.
	if genPhase != "" {
		opts.Mode = docgen.GenerateModeStream
	} else {
		opts.Mode = docgen.GenerateModeBatch
	}

	gen, err := docgen.NewGenerator(apiKey, opts, log)
	if err != nil {
		return fmt.Errorf("create generator: %w", err)
	}

	_ = gen
	fmt.Println("Document generation not yet implemented. Generator initialised successfully.")
	return nil
}
