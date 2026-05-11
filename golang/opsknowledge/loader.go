package opsknowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadFile parses a single YAML file into a typed File.
func LoadFile(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	f.Path = path
	return &f, nil
}

// LoadDir walks rootDir recursively and loads every .yaml file under
// stages/, runbooks/, or service-roles/. Returns the parsed files in
// path-sorted order so subsequent operations are deterministic.
//
// Files in the directory tree but NOT under one of the three known kinds
// (e.g. README.md, SCHEMA.md, stray text files) are skipped silently.
func LoadDir(rootDir string) ([]*File, error) {
	var files []*File
	known := map[string]bool{"stages": true, "runbooks": true, "service-roles": true}

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".yaml") {
			return nil
		}

		// Require the file to live directly under one of the known kind dirs
		// (skip stray YAML elsewhere in the tree).
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) < 2 || !known[parts[0]] {
			return nil
		}

		f, err := LoadFile(path)
		if err != nil {
			return err
		}
		files = append(files, f)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}
