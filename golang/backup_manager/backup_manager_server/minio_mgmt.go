package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
	"github.com/globulario/services/golang/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	minioCredentialsPath = "/var/lib/globular/minio/credentials"
	minioContractPath    = "/var/lib/globular/objectstore/minio.json"
)

// tryLoadMinioCredentials loads MinIO credentials from etcd. etcd is the only
// source of truth — no env vars, no disk files, no loopback fallbacks.
func (srv *server) tryLoadMinioCredentials() {
	cfg, err := config.LoadMinIOConfig()
	if err != nil {
		slog.Warn("minio config unavailable", "err", err)
		return
	}
	srv.MinioAccessKey = cfg.AccessKey
	srv.MinioSecretKey = cfg.SecretKey
	srv.MinioEndpoint = cfg.Endpoint
	srv.MinioSecure = cfg.Secure
	slog.Info("loaded MinIO credentials from etcd", "endpoint", cfg.Endpoint)
}

// loadMinioCA tries to load the Globular internal CA certificate pool.
func (srv *server) loadMinioCA() *x509.CertPool {
	paths := []string{
		"/var/lib/globular/pki/ca.crt",
		"/var/lib/globular/pki/ca.pem",
		srv.EtcdCACert,
	}
	for _, p := range paths {
		if p == "" {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		pool := x509.NewCertPool()
		if pool.AppendCertsFromPEM(data) {
			return pool
		}
	}
	return nil
}

// caCertPath returns the path to a CA cert file if one exists, empty string otherwise.
func (srv *server) caCertPath() string {
	paths := []string{
		"/var/lib/globular/pki/ca.crt",
		"/var/lib/globular/pki/ca.pem",
		srv.EtcdCACert,
	}
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// newMinioClient creates a MinIO client from server config.
func (srv *server) newMinioClient() (*minio.Client, error) {
	if srv.MinioEndpoint == "" {
		return nil, fmt.Errorf("MinioEndpoint not configured")
	}
	if srv.MinioAccessKey == "" || srv.MinioSecretKey == "" {
		return nil, fmt.Errorf("MinIO credentials not configured (set MinioAccessKey/MinioSecretKey, or create %s)", minioCredentialsPath)
	}

	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(srv.MinioAccessKey, srv.MinioSecretKey, ""),
		Secure: srv.MinioSecure,
	}

	// Cluster DNS dialer for *.globular.internal names + TLS.
	transport := &http.Transport{DialContext: config.ClusterDialContext}
	if srv.MinioSecure {
		tlsCfg := &tls.Config{}
		if caCert := srv.loadMinioCA(); caCert != nil {
			pool := caCert
			tlsCfg.RootCAs = pool
		} else {
			tlsCfg.InsecureSkipVerify = true
		}
		transport.TLSClientConfig = tlsCfg
	}
	opts.Transport = transport

	return minio.New(srv.MinioEndpoint, opts)
}

// ListMinioBuckets lists all buckets on the configured MinIO endpoint.
func (srv *server) ListMinioBuckets(ctx context.Context, _ *backup_managerpb.ListMinioBucketsRequest) (*backup_managerpb.ListMinioBucketsResponse, error) {
	client, err := srv.newMinioClient()
	if err != nil {
		return nil, fmt.Errorf("connect to MinIO: %w", err)
	}

	buckets, err := client.ListBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list buckets: %w", err)
	}

	resp := &backup_managerpb.ListMinioBucketsResponse{
		Endpoint: srv.MinioEndpoint,
	}

	for _, b := range buckets {
		resp.Buckets = append(resp.Buckets, &backup_managerpb.MinioBucketInfo{
			Name:         b.Name,
			CreationDate: b.CreationDate.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	return resp, nil
}

// CreateMinioBucket creates a new bucket and optionally configures it as a backup destination.
func (srv *server) CreateMinioBucket(ctx context.Context, req *backup_managerpb.CreateMinioBucketRequest) (*backup_managerpb.CreateMinioBucketResponse, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("bucket name is required")
	}

	client, err := srv.newMinioClient()
	if err != nil {
		return nil, fmt.Errorf("connect to MinIO: %w", err)
	}

	// Check if bucket already exists
	exists, err := client.BucketExists(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("check bucket: %w", err)
	}

	if !exists {
		if err := client.MakeBucket(ctx, req.Name, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("create bucket %q: %w", req.Name, err)
		}
		slog.Info("created MinIO bucket", "bucket", req.Name)
	}

	msg := fmt.Sprintf("Bucket %q ready", req.Name)

	if req.SetAsBackupDestination {
		srv.addMinioDestination(req.Name)
		msg += "; added as backup destination"
	}

	if req.SetAsScyllaLocation {
		srv.ScyllaLocation = "s3:" + req.Name
		msg += fmt.Sprintf("; ScyllaLocation set to s3:%s", req.Name)
		// Auto-configure scylla-manager-agent S3 access
		srv.ensureScyllaAgentS3Config()
	}

	return &backup_managerpb.CreateMinioBucketResponse{
		Ok:         true,
		Message:    msg,
		BucketName: req.Name,
	}, nil
}

// DeleteMinioBucket removes a MinIO bucket and optionally its contents.
func (srv *server) DeleteMinioBucket(ctx context.Context, req *backup_managerpb.DeleteMinioBucketRequest) (*backup_managerpb.DeleteMinioBucketResponse, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("bucket name is required")
	}

	mc, err := srv.newMinioClient()
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}

	exists, err := mc.BucketExists(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("check bucket: %w", err)
	}
	if !exists {
		return &backup_managerpb.DeleteMinioBucketResponse{Ok: true, Message: fmt.Sprintf("Bucket %q does not exist", req.Name)}, nil
	}

	if req.Force {
		// Remove all objects first
		objectsCh := mc.ListObjects(ctx, req.Name, minio.ListObjectsOptions{Recursive: true})
		for obj := range objectsCh {
			if obj.Err != nil {
				return nil, fmt.Errorf("list objects: %w", obj.Err)
			}
			if err := mc.RemoveObject(ctx, req.Name, obj.Key, minio.RemoveObjectOptions{}); err != nil {
				return nil, fmt.Errorf("remove object %q: %w", obj.Key, err)
			}
		}
	}

	if err := mc.RemoveBucket(ctx, req.Name); err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "not empty") && !req.Force {
			return nil, fmt.Errorf("bucket is not empty — enable force delete to remove all objects first")
		}
		return nil, fmt.Errorf("remove bucket: %w", err)
	}

	// Remove from destinations if present
	var kept []DestinationConfig
	for _, d := range srv.Destinations {
		if d.Type == "minio" && d.Path == req.Name {
			continue
		}
		kept = append(kept, d)
	}
	srv.Destinations = kept

	slog.Info("deleted MinIO bucket", "bucket", req.Name, "force", req.Force)
	return &backup_managerpb.DeleteMinioBucketResponse{
		Ok:      true,
		Message: fmt.Sprintf("Bucket %q deleted", req.Name),
	}, nil
}

// addMinioDestination adds a MinIO bucket as a backup destination if not already present.
func (srv *server) addMinioDestination(bucket string) {
	// Check if already configured
	for _, d := range srv.Destinations {
		if d.Type == "minio" && d.Path == bucket {
			return
		}
	}

	scheme := "http"
	if srv.MinioSecure {
		scheme = "https"
	}

	srv.Destinations = append(srv.Destinations, DestinationConfig{
		Name: "minio-" + bucket,
		Type: "minio",
		Path: bucket,
		Options: map[string]string{
			"endpoint":   fmt.Sprintf("%s://%s", scheme, srv.MinioEndpoint),
			"access_key": srv.MinioAccessKey,
			"secret_key": srv.MinioSecretKey,
		},
	})
}

// ── Scylla Manager Agent S3 auto-configuration ─────────────────────────────

const scyllaAgentConfigPath = "/var/lib/globular/scylla-manager-agent/scylla-manager-agent.yaml"

// ensureScyllaAgentS3Config ensures the scylla-manager-agent YAML has S3
// credentials pointing at the configured MinIO instance. Called during Init
// and when creating a bucket with SetAsScyllaLocation.
func (srv *server) ensureScyllaAgentS3Config() {
	if srv.MinioEndpoint == "" || srv.MinioAccessKey == "" || srv.MinioSecretKey == "" {
		return
	}

	var content string
	data, err := os.ReadFile(scyllaAgentConfigPath)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("cannot read scylla-manager-agent config", "path", scyllaAgentConfigPath, "error", err)
			return
		}
		// File doesn't exist — create the directory and start with empty content.
		if mkErr := os.MkdirAll(filepath.Dir(scyllaAgentConfigPath), 0755); mkErr != nil {
			slog.Warn("cannot create scylla-manager-agent config dir", "error", mkErr)
			return
		}
		slog.Info("scylla-manager-agent config does not exist, will create it", "path", scyllaAgentConfigPath)
	} else {
		content = string(data)
	}

	// If an s3 section already exists with the right endpoint and no stale fields, skip.
	hasS3 := strings.Contains(content, "s3:")
	hasEndpoint := strings.Contains(content, srv.MinioEndpoint)
	hasBadFields := strings.Contains(content, "disable_ssl") || strings.Contains(content, "skip_ssl_verification") || strings.Contains(content, "ca_cert")
	// Check that rclone insecure_skip_verify is present when needed (or absent when not)
	hasRcloneSkipVerify := strings.Contains(content, "rclone:") && strings.Contains(content, "insecure_skip_verify:")
	skipVerifyCorrect := (srv.MinioSecure && hasRcloneSkipVerify) || (!srv.MinioSecure && !hasRcloneSkipVerify)
	if hasS3 && hasEndpoint && !hasBadFields && skipVerifyCorrect {
		return
	}

	scheme := "https"
	if !srv.MinioSecure {
		scheme = "http"
	}

	// Build the s3 block
	s3Block := fmt.Sprintf(`
# Auto-configured by backup_manager for MinIO S3 access
s3:
  access_key_id: %s
  secret_access_key: %s
  provider: Minio
  region: us-east-1
  endpoint: %s://%s
`, srv.MinioAccessKey, srv.MinioSecretKey, scheme, srv.MinioEndpoint)

	// insecure_skip_verify is a rclone global option — goes under the rclone: section,
	// not under s3: or at the agent config top level.
	var rcloneBlock string
	if srv.MinioSecure {
		rcloneBlock = "\n# Skip TLS verification for internal MinIO with self-signed certs\nrclone:\n  insecure_skip_verify: true\n"
	}

	// Strip old s3 block, old insecure_skip_verify, and old auto-config comments
	lines := strings.Split(content, "\n")
	var out []string
	inS3, inRclone := false, false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip old s3: block
		if trimmed == "s3:" || strings.HasPrefix(trimmed, "s3:") {
			inS3 = true
			continue
		}
		if inS3 {
			if trimmed == "" || (len(line) > 0 && (line[0] == ' ' || line[0] == '\t')) {
				continue
			}
			inS3 = false
		}
		// Skip old rclone: block (with insecure_skip_verify underneath)
		if trimmed == "rclone:" || strings.HasPrefix(trimmed, "rclone:") {
			inRclone = true
			continue
		}
		if inRclone {
			if trimmed == "" || (len(line) > 0 && (line[0] == ' ' || line[0] == '\t')) {
				continue
			}
			inRclone = false
		}
		// Skip old top-level insecure_skip_verify and auto-config comments
		if strings.HasPrefix(trimmed, "insecure_skip_verify:") {
			continue
		}
		if strings.Contains(trimmed, "Auto-configured by backup_manager") ||
			strings.Contains(trimmed, "Skip TLS verification for internal MinIO") {
			continue
		}
		out = append(out, line)
	}
	content = strings.Join(out, "\n")

	content = strings.TrimRight(content, "\n") + "\n" + s3Block + rcloneBlock

	if err := os.WriteFile(scyllaAgentConfigPath, []byte(content), 0600); err != nil {
		slog.Warn("cannot write scylla-manager-agent config", "path", scyllaAgentConfigPath, "error", err)
		return
	}
	slog.Info("updated scylla-manager-agent S3 config", "endpoint", srv.MinioEndpoint)

	// Restart the agent so it picks up the new S3 config.
	if out, err := exec.Command("sudo", "systemctl", "restart", "globular-scylla-manager-agent.service").CombinedOutput(); err != nil {
		slog.Warn("failed to restart scylla-manager-agent", "error", err, "output", strings.TrimSpace(string(out)))
	} else {
		slog.Info("restarted scylla-manager-agent to apply S3 config")
	}
}
