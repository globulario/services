package actions

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/plan/versionutil"
	"google.golang.org/protobuf/types/known/structpb"
)

// artifact.fetch copies a local artifact into a deterministic staging path.
// It supports local sources only for now; remote fetch can be added later.
type artifactFetchAction struct{}

func (artifactFetchAction) Name() string { return "artifact.fetch" }

func (artifactFetchAction) Validate(args *structpb.Struct) error { return nil }

func (artifactFetchAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	source := strings.TrimSpace(fields["source"].GetStringValue())
	dest := strings.TrimSpace(fields["artifact_path"].GetStringValue())
	if dest == "" {
		return "", fmt.Errorf("artifact_path is required")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("create dest dir: %w", err)
	}
	if source == "" {
		if _, err := os.Stat(dest); err == nil {
			return "artifact already present", nil
		}
		return "", fmt.Errorf("source is required when artifact not present")
	}
	in, err := os.Open(source)
	if err != nil {
		return "", fmt.Errorf("open source: %w", err)
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("create dest: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return "", fmt.Errorf("copy artifact: %w", err)
	}
	_ = out.Close()
	return "artifact fetched", nil
}

// artifact.verify performs a simple existence/digest check if provided.
type artifactVerifyAction struct{}

func (artifactVerifyAction) Name() string { return "artifact.verify" }

func (artifactVerifyAction) Validate(args *structpb.Struct) error { return nil }

func (artifactVerifyAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	path := strings.TrimSpace(fields["artifact_path"].GetStringValue())
	if path == "" {
		return "", fmt.Errorf("artifact_path is required")
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("artifact missing: %w", err)
	}
	// TODO: add sha256 verification when digest provided.
	return "artifact verified", nil
}

type serviceInstallPayloadAction struct{}

func (serviceInstallPayloadAction) Name() string { return "service.install_payload" }

func (serviceInstallPayloadAction) Validate(args *structpb.Struct) error { return nil }

func (serviceInstallPayloadAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	service := strings.TrimSpace(fields["service"].GetStringValue())
	artifact := strings.TrimSpace(fields["artifact_path"].GetStringValue())
	dest := strings.TrimSpace(fields["install_path"].GetStringValue())
	if service == "" {
		return "", fmt.Errorf("service is required")
	}
	if artifact == "" {
		return "", fmt.Errorf("artifact_path is required")
	}
	if dest == "" {
		dest = filepath.Join("/var/lib/globular/staging", service, filepath.Base(artifact))
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("create install dir: %w", err)
	}
	in, err := os.Open(artifact)
	if err != nil {
		return "", fmt.Errorf("open artifact: %w", err)
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("create install dest: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return "", fmt.Errorf("install copy: %w", err)
	}
	_ = out.Close()
	return "service payload installed", nil
}

type serviceWriteVersionMarkerAction struct{}

func (serviceWriteVersionMarkerAction) Name() string { return "service.write_version_marker" }

func (serviceWriteVersionMarkerAction) Validate(args *structpb.Struct) error { return nil }

func (serviceWriteVersionMarkerAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	service := strings.TrimSpace(fields["service"].GetStringValue())
	version := fields["version"].GetStringValue()
	path := strings.TrimSpace(fields["path"].GetStringValue())
	if service == "" {
		return "", fmt.Errorf("service is required")
	}
	if path == "" {
		path = versionutil.MarkerPath(service)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create marker dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(version), 0o644); err != nil {
		return "", fmt.Errorf("write marker: %w", err)
	}
	return "version marker written", nil
}

func init() {
	Register(artifactFetchAction{})
	Register(artifactVerifyAction{})
	Register(serviceInstallPayloadAction{})
	Register(serviceWriteVersionMarkerAction{})
}
