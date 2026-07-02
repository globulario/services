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

// CanonicalInstallReceiptInput is the explicit input set allowed to influence
// installed_state receipt content for a package.
type CanonicalInstallReceiptInput struct {
	PackageName    string
	Version        string
	Kind           string
	UnitFilePath   string
	InstalledBy    string
	BinaryPath     string
	PackageSha256  string
	ArtifactDigest string
}

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
	if local := srv.findLocalPackage(name, version, runtime.GOOS+"_"+runtime.GOARCH); local != "" {
		return local
	}
	staging := filepath.Join("/var/lib/globular/staging", defaultPublisherID, name, "latest.artifact")
	if fi, err := os.Stat(staging); err == nil && !fi.IsDir() {
		return staging
	}
	return ""
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
		name := strings.TrimPrefix(filepath.Clean(hdr.Name), "./")
		switch {
		case (strings.HasPrefix(name, "systemd/") || strings.HasPrefix(name, "units/")) && filepath.Base(name) == unitName:
			rawUnit, err = io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", hdr.Name, err)
			}
		case strings.HasPrefix(name, "specs/") && (strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")):
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

func receiptUnitContent(renderedUnit, diskUnit []byte) []byte {
	if !canonicalUnitContentEqual(renderedUnit, diskUnit) {
		return diskUnit
	}
	return renderedUnit
}

func clearUnitReceiptMetadata(pkg *node_agentpb.InstalledPackage) {
	if pkg == nil || pkg.Metadata == nil {
		return
	}
	delete(pkg.Metadata, installreceipt.KeyUnitFilePath)
	delete(pkg.Metadata, installreceipt.KeyUnitFileSha256)
	delete(pkg.Metadata, installreceipt.KeyUnitRendererVersion)
}

func (srv *NodeAgentServer) canonicalInstallReceiptOpts(ctx context.Context, in CanonicalInstallReceiptInput) (installreceipt.ReceiptOpts, error) {
	opts := installreceipt.ReceiptOpts{
		InstalledBy:         in.InstalledBy,
		PackageSha256:       in.PackageSha256,
		ArtifactDigest:      in.ArtifactDigest,
		UnitRendererVersion: canonicalUnitRendererVersion,
	}
	if in.BinaryPath != "" {
		if fi, err := os.Stat(in.BinaryPath); err == nil && !fi.IsDir() {
			opts.BinaryPath = in.BinaryPath
		}
	}

	unitPath := strings.TrimSpace(in.UnitFilePath)
	if unitPath == "" {
		unitPath = filepath.Join("/etc/systemd/system", "globular-"+in.PackageName+".service")
	}
	fi, err := os.Stat(unitPath)
	if err != nil || fi.IsDir() {
		return opts, nil
	}
	diskUnit, readErr := os.ReadFile(unitPath)
	if readErr != nil {
		return opts, fmt.Errorf("canonical unit receipt: read %s: %w", unitPath, readErr)
	}

	renderedUnit, renderErr := srv.renderCanonicalUnitFromLocalArtifact(ctx, in.PackageName, in.Version, in.Kind, filepath.Base(unitPath))
	if renderErr != nil {
		return opts, fmt.Errorf("canonical unit render unavailable for %s/%s@%s: %w", in.Kind, in.PackageName, in.Version, renderErr)
	}
	opts.UnitFilePath = unitPath
	opts.UnitFileContent = receiptUnitContent(renderedUnit, diskUnit)
	if !canonicalUnitContentEqual(renderedUnit, diskUnit) {
		log.Printf("canonical unit receipt: %s/%s@%s rendered unit differs from disk; stamping disk evidence for %s",
			in.Kind, in.PackageName, in.Version, unitPath)
	}
	return opts, nil
}

func (srv *NodeAgentServer) stampCanonicalReceiptForInstalledPackage(ctx context.Context, pkg *node_agentpb.InstalledPackage, installedBy string, binPath string) {
	if pkg == nil || pkg.GetName() == "" {
		return
	}
	opts, err := srv.canonicalInstallReceiptOpts(ctx, CanonicalInstallReceiptInput{
		PackageName:    pkg.GetName(),
		Version:        pkg.GetVersion(),
		Kind:           pkg.GetKind(),
		InstalledBy:    installedBy,
		BinaryPath:     binPath,
		PackageSha256:  pkg.GetChecksum(),
		ArtifactDigest: pkg.GetChecksum(),
	})
	if err != nil {
		clearUnitReceiptMetadata(pkg)
		log.Printf("install_receipt: %v (clearing stale unit receipt; fail-closed)", err)
	}
	if serr := installreceipt.Stamp(pkg, opts); serr != nil {
		log.Printf("install_receipt: receipt skipped for %s/%s: %v", pkg.GetKind(), pkg.GetName(), serr)
	}
}
