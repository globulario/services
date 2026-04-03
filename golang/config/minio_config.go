package config

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
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
type MinIOConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Secure    bool
}

// GetMinIOConfig reads MinIO credentials from environment or well-known paths.
func GetMinIOConfig() MinIOConfig {
	// Default Secure to true — the cluster always uses TLS.
	// Only disable with MINIO_SECURE=false explicitly.
	secure := true
	if v := os.Getenv("MINIO_SECURE"); v == "false" {
		secure = false
	}

	cfg := MinIOConfig{
		Endpoint:  os.Getenv("MINIO_ENDPOINT"),
		AccessKey: os.Getenv("MINIO_ACCESS_KEY"),
		SecretKey: os.Getenv("MINIO_SECRET_KEY"),
		Secure:    secure,
	}

	if cfg.Endpoint == "" {
		cfg.Endpoint = "127.0.0.1:9000"
	}

	// Try reading credentials from file if env not set.
	if cfg.AccessKey == "" {
		credFile := os.Getenv("MINIO_CREDENTIALS_FILE")
		if credFile == "" {
			credFile = "/var/lib/globular/objectstore/minio.json"
		}
		if data, err := os.ReadFile(credFile); err == nil {
			// Simple key:secret format or JSON.
			parts := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)
			if len(parts) >= 2 {
				cfg.AccessKey = strings.TrimSpace(parts[0])
				cfg.SecretKey = strings.TrimSpace(parts[1])
			}
		}
	}

	// Fallback: default MinIO creds from the installer.
	if cfg.AccessKey == "" {
		cfg.AccessKey = "minioadmin"
		cfg.SecretKey = "minioadmin"
	}

	return cfg
}

// newMinIOClient creates a MinIO client from the current config.
// When Secure is true, the cluster CA is loaded so the client trusts
// the internal PKI certificate used by MinIO.
func newMinIOClient(cfg MinIOConfig) (*minio.Client, error) {
	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.Secure,
	}

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
		opts.Transport = &http.Transport{TLSClientConfig: tlsCfg}
	}

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
