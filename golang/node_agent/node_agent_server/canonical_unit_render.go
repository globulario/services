package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globularcli/pkgpack"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/installreceipt"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/unitrender"
)

const canonicalUnitRendererVersion = "artifact-canonical-v1"

// CanonicalUnitRenderInput is the explicit input set allowed to influence
// canonical unit rendering.
type CanonicalUnitRenderInput struct {
	PackageName   string
	Version       string
	Kind          string
	PublisherID   string
	Platform      string
	StateDir      string
	Prefix        string
	BinDir        string
	LogDir        string
	MinioDataDir  string
	NodeIP        string
	RuntimeInputs map[string]string
}

func (srv *NodeAgentServer) canonicalUnitRenderInput(ctx context.Context, name, version, kind string) CanonicalUnitRenderInput {
	nodeIP := strings.TrimSpace(config.GetRoutableIPv4())
	if nodeIP == "" {
		nodeIP = nodeRoutableIP()
	}
	input := CanonicalUnitRenderInput{
		PackageName:   name,
		Version:       version,
		Kind:          strings.ToUpper(strings.TrimSpace(kind)),
		PublisherID:   defaultPublisherID,
		Platform:      runtime.GOOS + "_" + runtime.GOARCH,
		StateDir:      "/var/lib/globular",
		Prefix:        "/usr/lib/globular",
		BinDir:        "/usr/lib/globular/bin",
		LogDir:        "/var/log/globular",
		MinioDataDir:  "/var/lib/globular/minio/data",
		NodeIP:        nodeIP,
		RuntimeInputs: map[string]string{"node_ip": nodeIP},
	}
	if strings.EqualFold(name, "minio") {
		if path := srv.canonicalMinioDataDir(ctx, nodeIP); path != "" {
			input.MinioDataDir = path
		}
	}
	return input
}

func (srv *NodeAgentServer) canonicalMinioDataDir(ctx context.Context, nodeIP string) string {
	state, err := config.LoadObjectStoreDesiredState(ctx)
	if err != nil || state == nil || nodeIP == "" {
		return ""
	}
	if state.NodePaths != nil {
		if path := strings.TrimSpace(state.NodePaths[nodeIP]); path != "" {
			return path + "/data"
		}
	}
	return ""
}

func renderInputsToUnit(in CanonicalUnitRenderInput) unitrender.Inputs {
	return unitrender.Inputs{
		StateDir:     in.StateDir,
		Prefix:       in.Prefix,
		BinDir:       in.BinDir,
		LogDir:       in.LogDir,
		MinioDataDir: in.MinioDataDir,
		NodeIP:       in.NodeIP,
	}
}

func renderCanonicalUnitBytes(raw []byte, in CanonicalUnitRenderInput) []byte {
	return unitrender.RenderSystemdUnitBytes(raw, renderInputsToUnit(in))
}

func (srv *NodeAgentServer) resolveArtifactForCanonicalUnit(name, version string) string {
	staging := filepath.Join("/var/lib/globular/staging", defaultPublisherID, name, "latest.artifact")
	if fi, err := os.Stat(staging); err == nil && !fi.IsDir() {
		return staging
	}
	return srv.findLocalPackage(name, version, runtime.GOOS+"_"+runtime.GOARCH)
}

func (srv *NodeAgentServer) renderCanonicalUnitFromLocalArtifact(ctx context.Context, name, version, kind, unitName string) ([]byte, error) {
	artifactPath := srv.resolveArtifactForCanonicalUnit(name, version)
	if artifactPath == "" {
		return nil, fmt.Errorf("canonical unit render: artifact not found for %s@%s", name, version)
	}
	return renderCanonicalUnitFromArtifactPath(artifactPath, unitName, srv.canonicalUnitRenderInput(ctx, name, version, kind))
}

func renderCanonicalUnitFromArtifactPath(artifactPath, unitName string, in CanonicalUnitRenderInput) ([]byte, error) {
	f, err := os.Open(artifactPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var rawUnit []byte
	var specBytes [][]byte

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		switch {
		case (strings.HasPrefix(hdr.Name, "systemd/") || strings.HasPrefix(hdr.Name, "units/")) && filepath.Base(hdr.Name) == unitName:
			rawUnit, err = io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", hdr.Name, err)
			}
		case strings.HasPrefix(hdr.Name, "specs/") && (strings.HasSuffix(hdr.Name, ".yaml") || strings.HasSuffix(hdr.Name, ".yml")):
			b, rerr := io.ReadAll(tr)
			if rerr != nil {
				return nil, fmt.Errorf("read %s: %w", hdr.Name, rerr)
			}
			specBytes = append(specBytes, b)
		}
	}

	if len(rawUnit) > 0 {
		return renderCanonicalUnitBytes(rawUnit, in), nil
	}
	for _, specData := range specBytes {
		if content, ok := unitContentFromSpec(specData, unitName); ok {
			return renderCanonicalUnitBytes([]byte(content), in), nil
		}
	}
	return nil, fmt.Errorf("canonical unit render: unit %s not found in artifact %s", unitName, artifactPath)
}

func unitContentFromSpec(specData []byte, unitName string) (string, bool) {
	spec, err := pkgpack.ParseSpecBytes(specData, "artifact")
	if err != nil || spec == nil {
		return "", false
	}
	for _, step := range spec.Steps {
		if !strings.EqualFold(step.Type, "install_services") {
			continue
		}
		rawUnits, ok := step.Args["units"].([]any)
		if !ok {
			continue
		}
		for _, rawUnit := range rawUnits {
			unitMap, ok := rawUnit.(map[string]any)
			if !ok {
				continue
			}
			name, _ := unitMap["name"].(string)
			content, _ := unitMap["content"].(string)
			if strings.TrimSpace(name) == unitName && strings.TrimSpace(content) != "" {
				return content, true
			}
		}
	}
	return "", false
}

func canonicalUnitContentEqual(a, b []byte) bool {
	return bytes.Equal(a, b)
}

func (srv *NodeAgentServer) stampCanonicalReceiptForInstalledPackage(ctx context.Context, pkg *node_agentpb.InstalledPackage, installedBy string, binPath string) {
	if pkg == nil || pkg.GetName() == "" {
		return
	}
	opts := installreceipt.ReceiptOpts{
		InstalledBy:         installedBy,
		PackageSha256:       pkg.GetChecksum(),
		ArtifactDigest:      pkg.GetChecksum(),
		UnitRendererVersion: canonicalUnitRendererVersion,
	}
	unitPath := "/etc/systemd/system/globular-" + pkg.GetName() + ".service"
	if fi, err := os.Stat(unitPath); err == nil && !fi.IsDir() {
		if renderedUnit, renderErr := srv.renderCanonicalUnitFromLocalArtifact(ctx, pkg.GetName(), pkg.GetVersion(), pkg.GetKind(), filepath.Base(unitPath)); renderErr == nil {
			opts.UnitFilePath = unitPath
			opts.UnitFileContent = renderedUnit
		} else {
			log.Printf("install_receipt: canonical unit render unavailable for %s/%s@%s: %v (skipping unit hash; fail-closed)", pkg.GetKind(), pkg.GetName(), pkg.GetVersion(), renderErr)
		}
	}
	if binPath != "" {
		if fi, err := os.Stat(binPath); err == nil && !fi.IsDir() {
			opts.BinaryPath = binPath
		}
	}
	if err := installreceipt.Stamp(pkg, opts); err != nil {
		log.Printf("install_receipt: receipt skipped for %s/%s: %v", pkg.GetKind(), pkg.GetName(), err)
	}
}
