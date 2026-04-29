package dashboard

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
)

//go:embed all:dist
var embeddedFiles embed.FS

func GetStaticFile(path string) ([]byte, error) {
	if path == "/" || path == "" {
		path = "/index.html"
	}

	// Clean the path - use forward slashes for embed.FS
	cleanPath := strings.TrimPrefix(path, "/")
	cleanPath = strings.ReplaceAll(cleanPath, "\\", "/")

	// Read from embedded filesystem
	content, err := fs.ReadFile(embeddedFiles, "dist/"+cleanPath)
	if err != nil {
		return nil, fmt.Errorf("file not found: %s (tried dist/%s)", path, cleanPath)
	}

	return content, nil
}
