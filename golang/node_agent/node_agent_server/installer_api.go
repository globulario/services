package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	"github.com/globulario/services/golang/plan/versionutil"
	"google.golang.org/protobuf/types/known/structpb"
)

const defaultPublisherID = "core@globular.io"

// InstallPackage fetches a package artifact from the repository and installs
// it locally. This is the public entry point for the workflow engine bridge.
//
// Parameters:
//   - name: package name (e.g. "dns", "envoy")
//   - kind: SERVICE, INFRASTRUCTURE, or COMMAND
//   - repositoryAddr: gRPC address of the repository (e.g. "10.0.0.63:443")
func (srv *NodeAgentServer) InstallPackage(ctx context.Context, name, kind, repositoryAddr string) error {
	platform := runtime.GOOS + "_" + runtime.GOARCH

	if repositoryAddr == "" {
		repositoryAddr = srv.discoverRepositoryAddr()
	}
	if repositoryAddr == "" {
		return fmt.Errorf("no repository address available")
	}

	artifactPath := fmt.Sprintf("/var/lib/globular/staging/%s/%s/latest.artifact",
		defaultPublisherID, name)

	// Fetch.
	fetchHandler := actions.Get("artifact.fetch")
	if fetchHandler == nil {
		return fmt.Errorf("action artifact.fetch not registered")
	}
	fetchArgs, err := structpb.NewStruct(map[string]any{
		"service":         name,
		"version":         "",
		"platform":        platform,
		"artifact_path":   artifactPath,
		"publisher_id":    defaultPublisherID,
		"repository_addr": repositoryAddr,
		"artifact_kind":   kind,
	})
	if err != nil {
		return fmt.Errorf("build fetch args: %w", err)
	}

	log.Printf("installer-api: fetching %s (%s) from %s", name, kind, repositoryAddr)
	if _, err := fetchHandler.Apply(ctx, fetchArgs); err != nil {
		return fmt.Errorf("fetch %s: %w", name, err)
	}

	// Install.
	switch strings.ToUpper(kind) {
	case "INFRASTRUCTURE":
		return srv.installInfra(ctx, name, artifactPath)
	case "COMMAND":
		return srv.installPayload(ctx, name, artifactPath)
	default:
		return srv.installPayload(ctx, name, artifactPath)
	}
}

func (srv *NodeAgentServer) installPayload(ctx context.Context, name, artifactPath string) error {
	handler := actions.Get("service.install_payload")
	if handler == nil {
		return fmt.Errorf("action service.install_payload not registered")
	}
	args, err := structpb.NewStruct(map[string]any{
		"service":       name,
		"version":       "0.0.1",
		"artifact_path": artifactPath,
	})
	if err != nil {
		return err
	}
	if _, err := handler.Apply(ctx, args); err != nil {
		return fmt.Errorf("install %s: %w", name, err)
	}
	return srv.writeMarker(name, "0.0.1")
}

func (srv *NodeAgentServer) installInfra(ctx context.Context, name, artifactPath string) error {
	handler := actions.Get("infrastructure.install")
	if handler == nil {
		return fmt.Errorf("action infrastructure.install not registered")
	}
	args, err := structpb.NewStruct(map[string]any{
		"name":          name,
		"version":       "0.0.1",
		"artifact_path": artifactPath,
	})
	if err != nil {
		return err
	}
	if _, err := handler.Apply(ctx, args); err != nil {
		return fmt.Errorf("install infra %s: %w", name, err)
	}
	return srv.writeMarker(name, "0.0.1")
}

func (srv *NodeAgentServer) writeMarker(name, version string) error {
	path := versionutil.MarkerPath(name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(version+"\n"), 0o644)
}

func (srv *NodeAgentServer) discoverRepositoryAddr() string {
	if srv.state != nil && srv.state.ControllerEndpoint != "" {
		host, _, err := splitHostPort(srv.state.ControllerEndpoint)
		if err == nil && host != "" {
			return host + ":443"
		}
	}
	return ""
}

func splitHostPort(addr string) (string, string, error) {
	// Handle addresses without port.
	if !strings.Contains(addr, ":") {
		return addr, "", nil
	}
	idx := strings.LastIndex(addr, ":")
	return addr[:idx], addr[idx+1:], nil
}
