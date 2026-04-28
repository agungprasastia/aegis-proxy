package dashboard

import (
	"fmt"
	"os"
	"path/filepath"
)

func GetStaticFile(path string) ([]byte, error) {
	if path == "/" {
		path = "/index.html"
	}

	cleanPath := filepath.Clean(path)
	if cleanPath[0] == '/' {
		cleanPath = cleanPath[1:]
	}

	devPath := filepath.Join("web", "dashboard", "dist", cleanPath)
	content, err := os.ReadFile(devPath)
	if err == nil {
		return content, nil
	}

	return nil, fmt.Errorf("file not found: %s", path)
}
