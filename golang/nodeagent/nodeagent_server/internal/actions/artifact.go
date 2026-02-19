package actions

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/actions/serviceports"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/ports"
	"github.com/globulario/services/golang/plan/versionutil"
	"github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
	service := strings.TrimSpace(fields["service"].GetStringValue())
	version := strings.TrimSpace(fields["version"].GetStringValue())
	platform := strings.TrimSpace(fields["platform"].GetStringValue())
	publisherID := strings.TrimSpace(fields["publisher_id"].GetStringValue())
	repositoryAddr := strings.TrimSpace(fields["repository_addr"].GetStringValue())
	expectedSHA := strings.TrimSpace(fields["expected_sha256"].GetStringValue())

	if dest == "" {
		return "", fmt.Errorf("artifact_path is required")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("create dest dir: %w", err)
	}
	// Artifact already present and valid — skip fetch.
	if _, err := os.Stat(dest); err == nil {
		return "artifact already present", nil
	}

	// Resolve local source path if not explicitly provided.
	if source == "" && (service != "" && version != "" && platform != "") {
		source = resolveArtifactPath(service, version, platform)
	}

	// Try local copy first.
	if source != "" {
		if _, err := os.Stat(source); err == nil {
			in, err := os.Open(source)
			if err != nil {
				return "", fmt.Errorf("open source: %w", err)
			}
			defer in.Close()
			if err := copyFileAtomic(dest, in); err != nil {
				return "", err
			}
			return "artifact fetched (local)", nil
		}
	}

	// Fall back to remote repository download.
	if repositoryAddr == "" {
		repositoryAddr = strings.TrimSpace(os.Getenv("REPOSITORY_ADDRESS"))
	}
	if repositoryAddr == "" {
		return "", fmt.Errorf("artifact not found locally and REPOSITORY_ADDRESS is not set")
	}
	if service == "" || version == "" || platform == "" {
		return "", fmt.Errorf("service, version, and platform are required for remote fetch")
	}
	ref := &repositorypb.ArtifactRef{
		Name:     service,
		Version:  version,
		Platform: platform,
		Kind:     repositorypb.ArtifactKind_SERVICE,
	}
	if publisherID != "" {
		ref.PublisherId = publisherID
	}
	if err := downloadArtifactFromRepository(ctx, repositoryAddr, ref, dest, expectedSHA); err != nil {
		return "", err
	}
	return fmt.Sprintf("artifact fetched (remote) from %s", repositoryAddr), nil
}

// artifact.verify performs a simple existence/digest check if provided.
type artifactVerifyAction struct{}

func (artifactVerifyAction) Name() string { return "artifact.verify" }

func (artifactVerifyAction) Validate(args *structpb.Struct) error { return nil }

func (artifactVerifyAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	path := strings.TrimSpace(fields["artifact_path"].GetStringValue())
	expected := strings.ToLower(strings.TrimSpace(fields["expected_sha256"].GetStringValue()))
	if path == "" {
		return "", fmt.Errorf("artifact_path is required")
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("artifact missing: %w", err)
	}
	if expected == "" {
		return "artifact verified (no checksum)", nil
	}
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open artifact: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash artifact: %w", err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != expected {
		return "", fmt.Errorf("artifact digest mismatch: want %s got %s", expected, got)
	}
	return fmt.Sprintf("artifact verified sha256=%s", got), nil
}

type serviceInstallPayloadAction struct{}

func (serviceInstallPayloadAction) Name() string { return "service.install_payload" }

func (serviceInstallPayloadAction) Validate(args *structpb.Struct) error { return nil }

func (serviceInstallPayloadAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	service := strings.TrimSpace(fields["service"].GetStringValue())
	artifact := strings.TrimSpace(fields["artifact_path"].GetStringValue())
	version := strings.TrimSpace(fields["version"].GetStringValue())
	if service == "" {
		return "", fmt.Errorf("service is required")
	}
	if artifact == "" {
		return "", fmt.Errorf("artifact_path is required")
	}
	stateRoot := strings.TrimSpace(os.Getenv("GLOBULAR_STATE_DIR"))
	if stateRoot == "" {
		stateRoot = "/var/lib/globular"
	}
	stagingRoot := filepath.Join(stateRoot, "staging", service)
	if testRoot := os.Getenv("GLOBULAR_STAGING_ROOT"); testRoot != "" {
		stagingRoot = filepath.Join(testRoot, service)
	}
	if err := os.MkdirAll(stagingRoot, 0o755); err != nil {
		return "", fmt.Errorf("create staging dir: %w", err)
	}
	if _, err := os.MkdirTemp(stagingRoot, "extract-"); err != nil {
		return "", fmt.Errorf("create extract dir: %w", err)
	}
	f, err := os.Open(artifact)
	if err != nil {
		return "", fmt.Errorf("open artifact: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	binDir, systemdDir, configDir, skipSystemd := installPaths()
	var wroteUnit bool

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read tar: %w", err)
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		name := strings.TrimLeft(hdr.Name, "./")
		var dest string
		switch {
		case strings.HasPrefix(name, "bin/"):
			dest = filepath.Join(binDir, filepath.Base(name))
		case strings.HasPrefix(name, "systemd/"), strings.HasPrefix(name, "units/"):
			dest = filepath.Join(systemdDir, filepath.Base(name))
			wroteUnit = true
		case strings.HasPrefix(name, "config/"):
			dest = filepath.Join(configDir, service, strings.TrimPrefix(name, "config/"))
		default:
			// ignore unsupported paths
			continue
		}
		if dest == "" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return "", fmt.Errorf("mkdir for %s: %w", dest, err)
		}
		tmp := dest + ".tmp"
		df, err := os.Create(tmp)
		if err != nil {
			return "", fmt.Errorf("create %s: %w", tmp, err)
		}
		if _, err := io.Copy(df, tr); err != nil {
			df.Close()
			os.Remove(tmp)
			return "", fmt.Errorf("write %s: %w", dest, err)
		}
		if err := df.Chmod(hdr.FileInfo().Mode()); err != nil {
			df.Close()
			os.Remove(tmp)
			return "", fmt.Errorf("chmod %s: %w", dest, err)
		}
		if err := df.Close(); err != nil {
			os.Remove(tmp)
			return "", fmt.Errorf("close %s: %w", dest, err)
		}
		if err := os.Rename(tmp, dest); err != nil {
			os.Remove(tmp)
			return "", fmt.Errorf("rename %s: %w", dest, err)
		}
	}

	if wroteUnit && !skipSystemd {
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(cctx, "systemctl", "daemon-reload")
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("systemctl daemon-reload: %v (output: %s)", err, string(out))
		}
	}

	if version == "" {
		version = filepath.Base(artifact)
	}

	// Ensure runtime config + port normalization
	if err := serviceports.EnsureServicePortConfig(ctx, service, binDir); err != nil {
		return "", err
	}

	return fmt.Sprintf("service payload installed version=%s", version), nil
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

func resolveArtifactPath(service, version, platform string) string {
	root := strings.TrimSpace(os.Getenv("GLOBULAR_ARTIFACT_REPO_ROOT"))
	if root == "" {
		root = "/var/lib/globular/repository/artifacts"
	}
	filename := fmt.Sprintf("%s.%s.%s.tgz", service, version, platform)
	return filepath.Join(root, service, version, platform, filename)
}

// downloadArtifactFromRepository fetches an artifact from a remote repository gRPC endpoint
// via streaming DownloadArtifact RPC and writes it atomically to dest.
//
// If expectedSHA256 is non-empty, the downloaded bytes are hashed and compared; a mismatch
// causes the temp file to be deleted and an error to be returned (hard invariant: never
// accept a corrupted artifact).
//
// TLS configuration uses:
//   - REPOSITORY_CA_PATH env var for the CA certificate (required unless REPOSITORY_INSECURE=true)
//   - REPOSITORY_INSECURE=true disables TLS (development only)
func downloadArtifactFromRepository(ctx context.Context, addr string, ref *repositorypb.ArtifactRef, dest, expectedSHA256 string) error {
	var opts []grpc.DialOption
	if strings.EqualFold(os.Getenv("REPOSITORY_INSECURE"), "true") {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}))) //nolint:gosec // dev mode only
	} else {
		caPath := strings.TrimSpace(os.Getenv("REPOSITORY_CA_PATH"))
		if caPath == "" {
			caPath = strings.TrimSpace(os.Getenv("NODE_AGENT_CONTROLLER_CA"))
		}
		tlsCfg := &tls.Config{}
		if caPath != "" {
			data, err := os.ReadFile(caPath)
			if err != nil {
				return fmt.Errorf("read repository CA %s: %w", caPath, err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(data) {
				return fmt.Errorf("parse repository CA %s: no certificates found", caPath)
			}
			tlsCfg.RootCAs = pool
		}
		if host, _, err := net.SplitHostPort(addr); err == nil {
			tlsCfg.ServerName = host
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	}
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, addr, opts...)
	if err != nil {
		return fmt.Errorf("dial repository %s: %w", addr, err)
	}
	defer conn.Close()

	client := repositorypb.NewPackageRepositoryClient(conn)
	stream, err := client.DownloadArtifact(ctx, &repositorypb.DownloadArtifactRequest{Ref: ref})
	if err != nil {
		return fmt.Errorf("download artifact %s/%s@%s: %w", ref.GetPublisherId(), ref.GetName(), ref.GetVersion(), err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), "artifact-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	hasher := sha256.New()
	var hw io.Writer = tmp
	if expectedSHA256 != "" {
		hw = io.MultiWriter(tmp, hasher)
	}

	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			tmp.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("receive chunk: %w", err)
		}
		if _, err := hw.Write(resp.GetData()); err != nil {
			tmp.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("write chunk: %w", err)
		}
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	if expectedSHA256 != "" {
		got := hex.EncodeToString(hasher.Sum(nil))
		if got != strings.ToLower(expectedSHA256) {
			os.Remove(tmpPath)
			return fmt.Errorf("artifact digest mismatch: want %s got %s", expectedSHA256, got)
		}
	}

	if err := os.Rename(tmpPath, dest); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename artifact: %w", err)
	}
	return nil
}

func copyFileAtomic(dest string, r io.Reader) error {
	tmp, err := os.CreateTemp(filepath.Dir(dest), "artifact-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("copy artifact: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename artifact: %w", err)
	}
	return nil
}

// ensureServicePortConfig guarantees that a service runtime config exists with a port
// inside the configured range. It is best-effort: unknown services are skipped.
func ensureServicePortConfig(ctx context.Context, service, binDir string) error {
	exe := executableForService(service)
	if exe == "" {
		return nil
	}
	binPath := filepath.Join(binDir, exe)

	alloc, err := ports.NewFromEnv()
	if err != nil {
		return err
	}

	desc, err := runDescribe(ctx, binPath)
	if err != nil {
		return err
	}
	if desc.Id == "" {
		return fmt.Errorf("describe %s returned empty Id", binPath)
	}

	stateRoot := strings.TrimSpace(os.Getenv("GLOBULAR_STATE_DIR"))
	if stateRoot == "" {
		stateRoot = "/var/lib/globular"
	}
	cfgPath := filepath.Join(stateRoot, "services", desc.Id+".json")

	cfg, _ := readServiceConfig(cfgPath)
	if cfg == nil {
		cfg = desc
	}

	currentPort := portFromAddress(cfg.Address)
	if currentPort == 0 {
		currentPort = desc.Port
	}

	needsRewrite := cfgPathMissing(cfgPath)
	start, end := alloc.Range()
	if currentPort < start || currentPort > end {
		needsRewrite = true
	}

	if !needsRewrite {
		return nil
	}

	newPort, err := alloc.Reserve(service, currentPort)
	if err != nil {
		return err
	}

	cfg.Port = newPort
	cfg.Address = fmt.Sprintf("localhost:%d", newPort)
	cfg.Id = desc.Id

	if err := writeServiceConfig(cfgPath, cfg); err != nil {
		return err
	}

	fmt.Printf("INFO service %s port normalized %d->%d range=%d-%d config=%s\n", service, currentPort, newPort, start, end, cfgPath)
	return nil
}

func executableForService(svc string) string {
	switch normalizeServiceName(svc) {
	case "rbac":
		return "rbac_server"
	case "resource":
		return "resource_server"
	case "repository":
		return "repository_server"
	case "xds":
		return "xds_server"
	case "gateway":
		return "gateway_server"
	default:
		return ""
	}
}

func normalizeServiceName(svc string) string {
	s := strings.ToLower(strings.TrimSpace(svc))
	s = strings.TrimPrefix(s, "globular-")
	s = strings.TrimSuffix(s, ".service")
	return s
}

type describePayload struct {
	Id      string `json:"Id"`
	Address string `json:"Address"`
	Port    int    `json:"Port"`
}

func runDescribe(ctx context.Context, binPath string) (*describePayload, error) {
	cmd := exec.CommandContext(ctx, binPath, "--describe")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("describe %s: %w", binPath, err)
	}
	var payload describePayload
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parse describe: %w", err)
	}
	if payload.Port == 0 {
		payload.Port = portFromAddress(payload.Address)
	}
	return &payload, nil
}

func portFromAddress(addr string) int {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	_ = host
	p, _ := strconv.Atoi(port)
	return p
}

func readServiceConfig(path string) (*describePayload, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg describePayload
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func writeServiceConfig(path string, cfg *describePayload) error {
	if cfg == nil {
		return errors.New("nil config")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func cfgPathMissing(path string) bool {
	_, err := os.Stat(path)
	return errors.Is(err, os.ErrNotExist)
}

func installPaths() (binDir, systemdDir, configDir string, skipSystemd bool) {
	binDir = os.Getenv("GLOBULAR_INSTALL_BIN_DIR")
	if binDir == "" {
		binDir = "/usr/local/bin"
	}
	systemdDir = os.Getenv("GLOBULAR_INSTALL_SYSTEMD_DIR")
	if systemdDir == "" {
		systemdDir = "/etc/systemd/system"
	}
	configDir = os.Getenv("GLOBULAR_INSTALL_CONFIG_DIR")
	if configDir == "" {
		configDir = "/etc/globular"
	}
	skipSystemd = os.Getenv("GLOBULAR_SKIP_SYSTEMD") == "1"
	return
}

func init() {
	Register(artifactFetchAction{})
	Register(artifactVerifyAction{})
	Register(serviceInstallPayloadAction{})
	Register(serviceWriteVersionMarkerAction{})
}
