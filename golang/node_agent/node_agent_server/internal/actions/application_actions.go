package actions

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/types/known/structpb"
)

// ── application.install ─────────────────────────────────────────────────────
//
// Extracts a web application archive to the applications directory and
// optionally calls Resource-side helpers for metadata, RBAC, and route
// registration. Those Resource calls are substeps — Node Agent owns the
// lifecycle, Resource provides helpers.
//
// Archive layout expected:
//
//	{anything}/   → extracted to /var/lib/globular/applications/{name}/
//
// Args:
//
//	name          (string, required)
//	version       (string, required)
//	artifact_path (string, required) — path to .tar.gz archive
//	route         (string, optional) — URL path, e.g. "/apps/myapp"
//	index_file    (string, optional) — entry HTML file, default "index.html"
type applicationInstallAction struct{}

func (applicationInstallAction) Name() string { return "application.install" }

func (applicationInstallAction) Validate(args *structpb.Struct) error {
	fields := args.GetFields()
	if strings.TrimSpace(fields["name"].GetStringValue()) == "" {
		return fmt.Errorf("application.install: name is required")
	}
	if strings.TrimSpace(fields["artifact_path"].GetStringValue()) == "" {
		return fmt.Errorf("application.install: artifact_path is required")
	}
	return nil
}

func (applicationInstallAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	name := strings.TrimSpace(fields["name"].GetStringValue())
	version := strings.TrimSpace(fields["version"].GetStringValue())
	artifactPath := strings.TrimSpace(fields["artifact_path"].GetStringValue())

	destDir := filepath.Join(appsDir(), name)

	// Remove previous version if it exists.
	if _, err := os.Stat(destDir); err == nil {
		if err := os.RemoveAll(destDir); err != nil {
			return "", fmt.Errorf("application.install: remove old version: %w", err)
		}
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("application.install: create app dir: %w", err)
	}

	// Extract archive contents into destDir.
	f, err := os.Open(artifactPath)
	if err != nil {
		return "", fmt.Errorf("application.install: open artifact: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("application.install: gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	fileCount := 0

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("application.install: read tar: %w", err)
		}

		// Clean up the path — strip leading components to flatten into destDir.
		entryName := strings.TrimLeft(hdr.Name, "./")
		if entryName == "" {
			continue
		}

		dest := filepath.Join(destDir, entryName)

		// Prevent path traversal.
		if !strings.HasPrefix(dest, destDir+string(os.PathSeparator)) && dest != destDir {
			continue
		}

		if hdr.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return "", fmt.Errorf("application.install: mkdir %s: %w", dest, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return "", fmt.Errorf("application.install: mkdir parent %s: %w", dest, err)
		}

		df, err := os.Create(dest)
		if err != nil {
			return "", fmt.Errorf("application.install: create %s: %w", dest, err)
		}
		if _, err := io.Copy(df, tr); err != nil {
			df.Close()
			return "", fmt.Errorf("application.install: write %s: %w", dest, err)
		}
		df.Close()
		fileCount++
	}

	if fileCount == 0 {
		return "", fmt.Errorf("application.install: archive contained no files")
	}

	// Write app metadata to etcd so gateway can discover and serve this application.
	// This is a helper substep — Node Agent owns the lifecycle.
	route := strings.TrimSpace(fields["route"].GetStringValue())
	if route == "" {
		route = "/applications/" + name
	}
	indexFile := strings.TrimSpace(fields["index_file"].GetStringValue())
	if indexFile == "" {
		indexFile = "index.html"
	}

	appMeta := map[string]string{
		"name":       name,
		"version":    version,
		"route":      route,
		"index_file": indexFile,
		"state":      "installed",
	}
	if err := writeAppMetadata(ctx, name, appMeta); err != nil {
		// Non-fatal: files are installed, metadata registration failed.
		// Log but don't fail the action — the package.report_state step
		// will still record installed-state.
		return fmt.Sprintf("application %s@%s installed (%d files to %s) [warning: metadata write failed: %v]", name, version, fileCount, destDir, err), nil
	}

	return fmt.Sprintf("application %s@%s installed (%d files to %s)", name, version, fileCount, destDir), nil
}

// ── application.uninstall ───────────────────────────────────────────────────
//
// Removes an installed web application from the applications directory.
//
// Args:
//
//	name (string, required)
type applicationUninstallAction struct{}

func (applicationUninstallAction) Name() string { return "application.uninstall" }

func (applicationUninstallAction) Validate(args *structpb.Struct) error {
	fields := args.GetFields()
	if strings.TrimSpace(fields["name"].GetStringValue()) == "" {
		return fmt.Errorf("application.uninstall: name is required")
	}
	return nil
}

func (applicationUninstallAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	name := strings.TrimSpace(fields["name"].GetStringValue())

	destDir := filepath.Join(appsDir(), name)
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		return fmt.Sprintf("application %s already removed", name), nil
	}

	if err := os.RemoveAll(destDir); err != nil {
		return "", fmt.Errorf("application.uninstall: remove %s: %w", destDir, err)
	}

	// Remove app metadata from etcd.
	_ = deleteAppMetadata(ctx, name)

	return fmt.Sprintf("application %s uninstalled", name), nil
}

// writeAppMetadata writes application discovery metadata to etcd.
// Key: /globular/applications/{name}
func writeAppMetadata(ctx context.Context, name string, meta map[string]string) error {
	cli, err := getEtcdClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err = cli.Put(tctx, "/globular/applications/"+name, string(data))
	return err
}

// deleteAppMetadata removes application discovery metadata from etcd.
func deleteAppMetadata(ctx context.Context, name string) error {
	cli, err := getEtcdClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err = cli.Delete(tctx, "/globular/applications/"+name)
	return err
}

// getEtcdClient returns an etcd v3 client using the standard Globular config.
func getEtcdClient() (*clientv3.Client, error) {
	return config.GetEtcdClient()
}

func init() {
	Register(applicationInstallAction{})
	Register(applicationUninstallAction{})
}
