package actions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHasInstallerSpec(t *testing.T) {
	t.Run("package.json only, no defaults.spec", func(t *testing.T) {
		dir := t.TempDir()
		manifest := map[string]any{"name": "test-pkg", "version": "1.0.0"}
		data, _ := json.Marshal(manifest)
		os.WriteFile(filepath.Join(dir, "package.json"), data, 0o644)

		if hasInstallerSpec(dir) {
			t.Fatal("expected false when package.json has no defaults.spec")
		}
	})

	t.Run("package.json with valid defaults.spec", func(t *testing.T) {
		dir := t.TempDir()
		specsDir := filepath.Join(dir, "specs")
		os.MkdirAll(specsDir, 0o755)
		os.WriteFile(filepath.Join(specsDir, "service.yaml"), []byte("version: 1"), 0o644)

		manifest := map[string]any{
			"name":    "test-pkg",
			"version": "1.0.0",
			"defaults": map[string]any{
				"spec": "specs/service.yaml",
			},
		}
		data, _ := json.Marshal(manifest)
		os.WriteFile(filepath.Join(dir, "package.json"), data, 0o644)

		if !hasInstallerSpec(dir) {
			t.Fatal("expected true when package.json has valid defaults.spec")
		}
	})

	t.Run("package.json with defaults.spec pointing to missing file", func(t *testing.T) {
		dir := t.TempDir()
		manifest := map[string]any{
			"name":    "test-pkg",
			"version": "1.0.0",
			"defaults": map[string]any{
				"spec": "specs/nonexistent.yaml",
			},
		}
		data, _ := json.Marshal(manifest)
		os.WriteFile(filepath.Join(dir, "package.json"), data, 0o644)

		if hasInstallerSpec(dir) {
			t.Fatal("expected false when defaults.spec points to missing file")
		}
	})

	t.Run("specs dir with yaml file", func(t *testing.T) {
		dir := t.TempDir()
		specsDir := filepath.Join(dir, "specs")
		os.MkdirAll(specsDir, 0o755)
		os.WriteFile(filepath.Join(specsDir, "service.yaml"), []byte("version: 1"), 0o644)

		if !hasInstallerSpec(dir) {
			t.Fatal("expected true when specs/ contains .yaml file")
		}
	})

	t.Run("specs dir with yml file", func(t *testing.T) {
		dir := t.TempDir()
		specsDir := filepath.Join(dir, "specs")
		os.MkdirAll(specsDir, 0o755)
		os.WriteFile(filepath.Join(specsDir, "service.yml"), []byte("version: 1"), 0o644)

		if !hasInstallerSpec(dir) {
			t.Fatal("expected true when specs/ contains .yml file")
		}
	})

	t.Run("specs dir empty", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "specs"), 0o755)

		if hasInstallerSpec(dir) {
			t.Fatal("expected false when specs/ is empty")
		}
	})

	t.Run("no package.json, no specs dir", func(t *testing.T) {
		dir := t.TempDir()

		if hasInstallerSpec(dir) {
			t.Fatal("expected false when neither package.json nor specs/ exist")
		}
	})

	t.Run("package.json with empty defaults.spec falls through to specs dir", func(t *testing.T) {
		dir := t.TempDir()
		manifest := map[string]any{
			"name":    "test-pkg",
			"version": "1.0.0",
			"defaults": map[string]any{
				"spec": "",
			},
		}
		data, _ := json.Marshal(manifest)
		os.WriteFile(filepath.Join(dir, "package.json"), data, 0o644)

		// No specs dir either — should be false.
		if hasInstallerSpec(dir) {
			t.Fatal("expected false when defaults.spec is empty and no specs/ dir")
		}
	})

	t.Run("defaults.spec pointing to directory", func(t *testing.T) {
		dir := t.TempDir()
		specsDir := filepath.Join(dir, "specs")
		os.MkdirAll(specsDir, 0o755)

		manifest := map[string]any{
			"name":    "test-pkg",
			"version": "1.0.0",
			"defaults": map[string]any{
				"spec": "specs",
			},
		}
		data, _ := json.Marshal(manifest)
		os.WriteFile(filepath.Join(dir, "package.json"), data, 0o644)

		if hasInstallerSpec(dir) {
			t.Fatal("expected false when defaults.spec points to a directory")
		}
	})
}
