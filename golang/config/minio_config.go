package config

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	// ClusterConfigBucket is the MinIO bucket for cluster-wide configuration.
	ClusterConfigBucket = "globular-config"
)

// Well-known config keys in the cluster config bucket.
const (
	ConfigKeyCA       = "pki/ca.pem"           // CA certificate
	ConfigKeyCAKey    = "pki/ca.key"           // CA private key
	ConfigKeyClaudeMD = "ai/CLAUDE.md"         // AI executor system prompt / rules
	ConfigKeyRBAC     = "policy/rbac/cluster-roles.json"
)

// MinIOConfig holds connection info for the shared MinIO instance.
//go:schemalint:ignore — schema owned by marker type in schema_annotations.go
type MinIOConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Secure    bool
	// Bucket is the MinIO bucket where Globular stores objects. Empty = "globular".
	Bucket string
	// Prefix is the key prefix within the bucket (typically the cluster domain,
	// e.g. "globular.internal"). Services prepend this to their storage keys.
	Prefix string
}

// EtcdKeyMinioConfig is the sole source of truth for MinIO connection info.
// Written by the cluster controller whenever the MinIO pool state changes.
// JSON schema: {"endpoint":"minio.globular.internal:9000","access_key":"...","secret_key":"...","secure":true}
const EtcdKeyMinioConfig = "/globular/cluster/minio/config"

// GetMinIOConfig reads MinIO connection info from etcd. etcd is the only
// source of truth for cluster configuration — no environment variables, no
// disk fallbacks, no hardcoded defaults. The endpoint is a DNS name
// (minio.globular.internal) resolved via the cluster DNS, so no IP is ever
// baked into a service or a systemd unit.
func GetMinIOConfig() MinIOConfig {
	cfg, err := LoadMinIOConfig()
	if err != nil {
		// Return zero config; callers that actually need MinIO will fail at
		// connect time with a clear error instead of silently connecting to a
		// stale/wrong endpoint.
		return MinIOConfig{}
	}
	return cfg
}

// LoadMinIOConfig reads and validates the MinIO cluster config from etcd.
// Returns an error if the key is missing or malformed. Use this when the caller
// wants to surface configuration errors explicitly.
func LoadMinIOConfig() (MinIOConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cli, err := GetEtcdClient()
	if err != nil {
		return MinIOConfig{}, fmt.Errorf("minio config: etcd unavailable: %w", err)
	}

	resp, err := cli.Get(ctx, EtcdKeyMinioConfig)
	if err != nil {
		return MinIOConfig{}, fmt.Errorf("minio config: etcd get %s: %w", EtcdKeyMinioConfig, err)
	}
	if len(resp.Kvs) == 0 {
		return MinIOConfig{}, fmt.Errorf("minio config: %s not set in etcd (cluster controller must publish it)", EtcdKeyMinioConfig)
	}

	var stored struct {
		Endpoint  string `json:"endpoint"`
		AccessKey string `json:"access_key"`
		SecretKey string `json:"secret_key"`
		Secure    bool   `json:"secure"`
		Bucket    string `json:"bucket"`
		Prefix    string `json:"prefix"`
	}
	if err := json.Unmarshal(resp.Kvs[0].Value, &stored); err != nil {
		return MinIOConfig{}, fmt.Errorf("minio config: parse %s: %w", EtcdKeyMinioConfig, err)
	}
	if stored.Endpoint == "" {
		return MinIOConfig{}, fmt.Errorf("minio config: endpoint is empty in %s", EtcdKeyMinioConfig)
	}
	if strings.Contains(stored.Endpoint, "127.0.0.1") || strings.Contains(stored.Endpoint, "localhost") {
		return MinIOConfig{}, fmt.Errorf("minio config: endpoint %q uses loopback — refuse to use (rule: no 127.0.0.1/localhost, ever)", stored.Endpoint)
	}
	if stored.Bucket == "" {
		stored.Bucket = "globular"
	}
	return MinIOConfig{
		Endpoint:  stored.Endpoint,
		AccessKey: stored.AccessKey,
		SecretKey: stored.SecretKey,
		Secure:    stored.Secure,
		Bucket:    stored.Bucket,
		Prefix:    stored.Prefix,
	}, nil
}

// BuildMinioProxyConfig returns a MinioProxyConfig populated from the etcd
// cluster config plus well-known defaults (bucket, prefix, CA bundle). This
// is the one and only way services should obtain MinIO connection info.
// Returns an error if the etcd key is missing or invalid.
func BuildMinioProxyConfig() (*MinioProxyConfig, error) {
	cfg, err := LoadMinIOConfig()
	if err != nil {
		return nil, err
	}
	bucket := cfg.Bucket
	if bucket == "" {
		bucket = "globular"
	}
	return &MinioProxyConfig{
		Endpoint:     cfg.Endpoint,
		Bucket:       bucket,
		Prefix:       cfg.Prefix,
		Secure:       cfg.Secure,
		CABundlePath: "/var/lib/globular/pki/ca.pem",
		Auth: &MinioProxyAuth{
			Mode:      MinioProxyAuthModeAccessKey,
			AccessKey: cfg.AccessKey,
			SecretKey: cfg.SecretKey,
		},
	}, nil
}

// SaveMinIOConfig writes the MinIO connection info to etcd. Called by the
// cluster controller after pool state changes.
func SaveMinIOConfig(cfg MinIOConfig) error {
	if cfg.Endpoint == "" {
		return fmt.Errorf("minio config: endpoint required")
	}
	if strings.Contains(cfg.Endpoint, "127.0.0.1") || strings.Contains(cfg.Endpoint, "localhost") {
		return fmt.Errorf("minio config: endpoint %q uses loopback — refuse to write (rule: no 127.0.0.1/localhost, ever)", cfg.Endpoint)
	}
	stored := struct {
		Endpoint  string `json:"endpoint"`
		AccessKey string `json:"access_key"`
		SecretKey string `json:"secret_key"`
		Secure    bool   `json:"secure"`
		Bucket    string `json:"bucket"`
		Prefix    string `json:"prefix"`
	}{
		Endpoint:  cfg.Endpoint,
		AccessKey: cfg.AccessKey,
		SecretKey: cfg.SecretKey,
		Secure:    cfg.Secure,
		Bucket:    cfg.Bucket,
		Prefix:    cfg.Prefix,
	}
	data, err := json.Marshal(stored)
	if err != nil {
		return fmt.Errorf("minio config: marshal: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cli, err := GetEtcdClient()
	if err != nil {
		return fmt.Errorf("minio config: etcd unavailable: %w", err)
	}
	if _, err := cli.Put(ctx, EtcdKeyMinioConfig, string(data)); err != nil {
		return fmt.Errorf("minio config: etcd put %s: %w", EtcdKeyMinioConfig, err)
	}
	return nil
}

// newMinIOClient creates a MinIO client from the current config.
// When Secure is true, the cluster CA is loaded so the client trusts
// the internal PKI certificate used by MinIO.
func newMinIOClient(cfg MinIOConfig) (*minio.Client, error) {
	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.Secure,
	}

	// Wire the cluster DNS resolver into the MinIO SDK so that *.globular.internal
	// names resolve via Globular DNS (not the system resolver, which has no
	// knowledge of cluster names).
	transport := &http.Transport{DialContext: ClusterDialContext}
	if cfg.Secure {
		tlsCfg := &tls.Config{}
		// Load the cluster CA so we trust the internal MinIO certificate.
		caPath := GetLocalCACertificate()
		if caPath != "" {
			caPEM, err := os.ReadFile(caPath)
			if err == nil {
				pool := x509.NewCertPool()
				pool.AppendCertsFromPEM(caPEM)
				tlsCfg.RootCAs = pool
			}
		}
		transport.TLSClientConfig = tlsCfg
	}
	opts.Transport = transport

	return minio.New(cfg.Endpoint, opts)
}

// EnsureClusterConfigBucket creates the config bucket if it doesn't exist.
func EnsureClusterConfigBucket() error {
	cfg := GetMinIOConfig()
	client, err := newMinIOClient(cfg)
	if err != nil {
		return fmt.Errorf("minio client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, ClusterConfigBucket)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, ClusterConfigBucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}
	}
	return nil
}

// PutClusterConfig uploads a config object to the shared bucket.
func PutClusterConfig(key string, data []byte) error {
	cfg := GetMinIOConfig()
	client, err := newMinIOClient(cfg)
	if err != nil {
		return fmt.Errorf("minio client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = client.PutObject(ctx, ClusterConfigBucket, key, bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		return fmt.Errorf("put %s: %w", key, err)
	}
	return nil
}

// GetClusterConfig downloads a config object from the shared bucket.
// Returns nil, nil if the key doesn't exist.
func GetClusterConfig(key string) ([]byte, error) {
	cfg := GetMinIOConfig()
	client, err := newMinIOClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	obj, err := client.GetObject(ctx, ClusterConfigBucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", key, err)
	}
	defer obj.Close()

	data, err := io.ReadAll(obj)
	if err != nil {
		// Check if it's a "key does not exist" error.
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "does not exist") {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", key, err)
	}
	return data, nil
}

// ListClusterConfigPrefix returns all keys in the config bucket matching the given prefix.
func ListClusterConfigPrefix(prefix string) ([]string, error) {
	cfg := GetMinIOConfig()
	client, err := newMinIOClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var keys []string
	objCh := client.ListObjects(ctx, ClusterConfigBucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	for obj := range objCh {
		if obj.Err != nil {
			return nil, fmt.Errorf("list %s: %w", prefix, obj.Err)
		}
		keys = append(keys, obj.Key)
	}
	return keys, nil
}

// PutClusterConfigFile uploads a local file to the config bucket.
func PutClusterConfigFile(key, localPath string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", localPath, err)
	}
	return PutClusterConfig(key, data)
}

// GetClusterConfigToFile downloads a config object and writes it to a local file.
// Creates parent directories as needed. Returns false if the key doesn't exist.
func GetClusterConfigToFile(key, localPath string) (bool, error) {
	data, err := GetClusterConfig(key)
	if err != nil {
		return false, err
	}
	if data == nil {
		return false, nil // key doesn't exist
	}

	dir := localPath[:strings.LastIndex(localPath, "/")]
	if dir != "" {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return false, fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(localPath, data, 0640); err != nil {
		return false, fmt.Errorf("write %s: %w", localPath, err)
	}
	return true, nil
}
