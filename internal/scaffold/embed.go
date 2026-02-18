// Package scaffold provides embedded default configuration and template
// files for the crux init command. Resources are compiled into the binary
// via go:embed so they are available regardless of working directory.
package scaffold

import (
	"embed"
	"io/fs"
)

//go:embed default-config.yaml
var defaultConfig []byte

//go:embed templates
var templatesFS embed.FS

// DefaultConfig returns the raw bytes of the default config.yaml.
func DefaultConfig() []byte {
	return defaultConfig
}

// TemplatesFS returns the embedded filesystem rooted at "templates".
func TemplatesFS() (fs.FS, error) {
	return fs.Sub(templatesFS, "templates")
}
