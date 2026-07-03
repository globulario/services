package main

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureArtifactEntrypointMaterialized_Noop(t *testing.T) {
	tmp := t.TempDir()
	artifact := filepath.Join(tmp, "keepalived.tgz")
	if err := writeTestArtifactWithNoop(artifact); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	oldBinDir := globularBinDir
	globularBinDir = filepath.Join(tmp, "bin")
	t.Cleanup(func() { globularBinDir = oldBinDir })

	if err := ensureArtifactEntrypointMaterialized("keepalived", artifact); err != nil {
		t.Fatalf("ensureArtifactEntrypointMaterialized: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(globularBinDir, "noop"))
	if err != nil {
		t.Fatalf("read extracted noop: %v", err)
	}
	if string(data) != "#!/bin/sh\nexit 0\n" {
		t.Fatalf("unexpected noop contents: %q", string(data))
	}
}

func writeTestArtifactWithNoop(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	files := []struct {
		name string
		data string
		mode int64
	}{
		{
			name: "package.json",
			data: `{"name":"keepalived","entrypoint":"bin/noop"}`,
			mode: 0o644,
		},
		{
			name: "bin/noop",
			data: "#!/bin/sh\nexit 0\n",
			mode: 0o755,
		},
	}
	for _, file := range files {
		hdr := &tar.Header{
			Name: file.name,
			Mode: file.mode,
			Size: int64(len(file.data)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := tw.Write([]byte(file.data)); err != nil {
			return err
		}
	}
	return nil
}
