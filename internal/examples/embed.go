// Package examples provides embedded example project files.
package examples

import (
	"embed"
	"io/fs"
)

//go:embed httpapi
var httpAPIFS embed.FS

// HTTPAPIFS returns the embedded FS rooted at "httpapi".
func HTTPAPIFS() (fs.FS, error) {
	return fs.Sub(httpAPIFS, "httpapi")
}
