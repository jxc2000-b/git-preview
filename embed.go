package main

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed templates static
var embeddedAssets embed.FS

// materializeAssets writes the embedded templates/static dirs to a temp
// directory and returns their paths. Used when the config leaves
// dirs.templates / dirs.static unset, so the binary is self-contained.
func materializeAssets() (templates string, static string, err error) {
	root, err := os.MkdirTemp("", "git-preview-assets-")
	if err != nil {
		return "", "", err
	}

	err = fs.WalkDir(embeddedAssets, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dest := filepath.Join(root, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		b, err := embeddedAssets.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, b, 0644)
	})
	if err != nil {
		return "", "", err
	}

	return filepath.Join(root, "templates"), filepath.Join(root, "static"), nil
}
