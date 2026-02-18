package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/roygabriel/crux/internal/examples"
	"github.com/spf13/cobra"
)

var (
	forceFlag   bool
	exampleFlag bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize project directory structure",
	Long:  "Creates .crux/ state directory, copies default config, creates docs/phases/ and work-notes/ directories, copies templates, and updates .gitignore.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().BoolVar(&forceFlag, "force", false, "Overwrite template files (never overwrites config)")
	initCmd.Flags().BoolVar(&exampleFlag, "example", false, "Seed project with HTTP API example")
}

func runInit(cmd *cobra.Command, args []string) error {
	dirs := []string{
		".crux",
		"docs/phases",
		"work-notes",
	}

	for _, dir := range dirs {
		created, err := ensureDir(dir)
		if err != nil {
			return err
		}
		if created {
			fmt.Printf("\u2713 Created %s/\n", dir)
		} else {
			fmt.Printf("\u2022 Skipped %s/ (exists)\n", dir)
		}
	}

	// Copy default config (never overwritten, even with --force).
	cfgPath := filepath.Join(".crux", "config.yaml")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		src, err := os.ReadFile("configs/default.yaml")
		if err != nil {
			fmt.Printf("\u2022 Skipped %s (source not found)\n", cfgPath)
		} else {
			if err := os.WriteFile(cfgPath, src, 0o644); err != nil {
				return fmt.Errorf("write config: %w", err)
			}
			fmt.Printf("\u2713 Created %s\n", cfgPath)
		}
	} else {
		fmt.Printf("\u2022 Skipped %s (exists)\n", cfgPath)
	}

	// Copy templates.
	created, skipped, err := copyTemplates("templates", "templates", forceFlag)
	if err != nil {
		return fmt.Errorf("copy templates: %w", err)
	}
	for _, name := range created {
		fmt.Printf("\u2713 Created templates/%s\n", name)
	}
	for _, name := range skipped {
		fmt.Printf("\u2022 Skipped templates/%s (exists)\n", name)
	}

	// Ensure .gitignore entries.
	gitignoreEntries := []string{
		".crux/memory.db",
		".crux/vectors/",
		".crux/audit.log",
		".crux/secrets.env",
	}
	added, err := ensureGitignoreEntries(".gitignore", gitignoreEntries)
	if err != nil {
		return fmt.Errorf("update gitignore: %w", err)
	}
	if added > 0 {
		fmt.Printf("\u2713 Updated .gitignore (added %d entries)\n", added)
	} else {
		fmt.Printf("\u2022 .gitignore already up to date\n")
	}

	if exampleFlag {
		exFS, err := examples.HTTPAPIFS()
		if err != nil {
			return fmt.Errorf("load example: %w", err)
		}
		exCreated, exSkipped, err := writeEmbeddedFS(exFS, ".", forceFlag)
		if err != nil {
			return fmt.Errorf("write example: %w", err)
		}
		for _, name := range exCreated {
			fmt.Printf("\u2713 Created %s\n", name)
		}
		for _, name := range exSkipped {
			fmt.Printf("\u2022 Skipped %s (exists)\n", name)
		}
	}

	return nil
}

// ensureDir creates a directory if it doesn't exist.
// Returns true if the directory was created, false if it already existed.
func ensureDir(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return false, nil
	}
	if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat %s: %w", path, err)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return false, fmt.Errorf("create directory %s: %w", path, err)
	}
	return true, nil
}

// copyTemplates copies files from srcDir to dstDir.
// If force is false, existing files in dstDir are skipped.
// Returns lists of created and skipped filenames.
func copyTemplates(srcDir, dstDir string, force bool) (created, skipped []string, err error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("read template dir: %w", err)
	}

	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create template dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())

		if !force {
			if _, statErr := os.Stat(dstPath); statErr == nil {
				skipped = append(skipped, entry.Name())
				continue
			}
		}

		data, readErr := os.ReadFile(srcPath)
		if readErr != nil {
			return created, skipped, fmt.Errorf("read template %s: %w", entry.Name(), readErr)
		}

		if writeErr := os.WriteFile(dstPath, data, 0o644); writeErr != nil {
			return created, skipped, fmt.Errorf("write template %s: %w", entry.Name(), writeErr)
		}
		created = append(created, entry.Name())
	}

	return created, skipped, nil
}

// ensureGitignoreEntries appends missing entries to a .gitignore file.
// Returns the number of entries added.
func ensureGitignoreEntries(path string, entries []string) (int, error) {
	var existing string
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("read gitignore: %w", err)
	}
	if err == nil {
		existing = string(data)
	}

	var missing []string
	for _, entry := range entries {
		if !gitignoreContains(existing, entry) {
			missing = append(missing, entry)
		}
	}

	if len(missing) == 0 {
		return 0, nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open gitignore: %w", err)
	}
	defer f.Close()

	if len(existing) > 0 && !strings.HasSuffix(existing, "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return 0, fmt.Errorf("write gitignore: %w", err)
		}
	}

	for _, entry := range missing {
		if _, err := f.WriteString(entry + "\n"); err != nil {
			return 0, fmt.Errorf("write gitignore: %w", err)
		}
	}

	return len(missing), nil
}

// gitignoreContains checks whether a .gitignore content string contains the given entry.
func gitignoreContains(content, entry string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == entry {
			return true
		}
	}
	return false
}

// writeEmbeddedFS walks an fs.FS and writes all files to dstDir.
// Directories are created as needed. Existing files are skipped unless
// force is true. Returns lists of created and skipped relative paths.
func writeEmbeddedFS(fsys fs.FS, dstDir string, force bool) (created, skipped []string, err error) {
	err = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		dstPath := filepath.Join(dstDir, path)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}

		if !force {
			if _, statErr := os.Stat(dstPath); statErr == nil {
				skipped = append(skipped, path)
				return nil
			}
		}

		if mkErr := os.MkdirAll(filepath.Dir(dstPath), 0o755); mkErr != nil {
			return fmt.Errorf("create parent dir for %s: %w", path, mkErr)
		}

		data, readErr := fs.ReadFile(fsys, path)
		if readErr != nil {
			return fmt.Errorf("read embedded %s: %w", path, readErr)
		}

		if writeErr := os.WriteFile(dstPath, data, 0o644); writeErr != nil {
			return fmt.Errorf("write %s: %w", path, writeErr)
		}
		created = append(created, path)
		return nil
	})
	return created, skipped, err
}
