// Package assets provides embedded static YAML configuration files for localregistry.
// All *.yaml files in this directory are embedded at compile time using Go's embed directive.
package assets

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
)

// Known asset filenames.
const (
	TraefikConfig     = "traefik.yaml"
	DefaultConfigFile = "default_config.yaml"
)

// FS contains all embedded YAML files from the assets directory.
//
//go:embed *.yaml
var FS embed.FS

// ReadFile reads and returns the content of an embedded asset file.
// It returns an error if the file doesn't exist in the embedded filesystem.
func ReadFile(name string) ([]byte, error) {
	data, err := FS.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("failed to read asset %q: %w", name, err)
	}
	return data, nil
}

// Open opens an embedded asset file for reading.
// It returns an error if the file doesn't exist in the embedded filesystem.
func Open(name string) (fs.File, error) {
	file, err := FS.Open(name)
	if err != nil {
		return nil, fmt.Errorf("failed to open asset %q: %w", name, err)
	}
	return file, nil
}

// Exists checks if a file exists in the embedded filesystem.
func Exists(name string) bool {
	_, err := FS.Open(name)
	return err == nil
}

// List returns a sorted list of all embedded asset filenames.
func List() ([]string, error) {
	var files []string
	err := fs.WalkDir(FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list assets: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

// MustReadFile is like ReadFile but panics if the file cannot be read.
// This is useful for assets that must always be present; failure indicates a programming error.
func MustReadFile(name string) []byte {
	data, err := ReadFile(name)
	if err != nil {
		panic(err)
	}
	return data
}
