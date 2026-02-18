package instruct

import (
	"embed"
	"io/fs"
)

//go:embed templates
var templatesFS embed.FS

// TemplatesFS returns the embedded filesystem rooted at "templates".
func TemplatesFS() (fs.FS, error) {
	return fs.Sub(templatesFS, "templates")
}
