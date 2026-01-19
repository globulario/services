package actions

import (
	"os"
	"testing"
	"time"

	"github.com/globulario/services/golang/config"
)

func TestDeriveMinioLayout_Defaults(t *testing.T) {
	cfg := &config.MinioProxyConfig{
		Bucket: "main",
		Prefix: "",
	}

	layout, err := deriveMinioLayoutForNodeAgent(cfg, "Example.com")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if layout.usersBucket != "main" || layout.webrootBucket != "main" {
		t.Fatalf("expected default bucket main, got users=%s webroot=%s", layout.usersBucket, layout.webrootBucket)
	}
	if layout.usersPrefix != "example.com/users" {
		t.Fatalf("unexpected users prefix: %s", layout.usersPrefix)
	}
	if layout.webrootPrefix != "example.com/webroot" {
		t.Fatalf("unexpected webroot prefix: %s", layout.webrootPrefix)
	}
}

func TestDeriveMinioLayout_WithOverrides(t *testing.T) {
	t.Setenv("GLOBULAR_MINIO_USERS_BUCKET", "users-bkt")
	t.Setenv("GLOBULAR_MINIO_WEBROOT_BUCKET", "web-bkt")
	t.Setenv("GLOBULAR_MINIO_USERS_PREFIX", "/custom/users")
	t.Setenv("GLOBULAR_MINIO_WEBROOT_PREFIX", "custom/webroot/")

	cfg := &config.MinioProxyConfig{
		Bucket: "ignored",
		Prefix: "base",
	}

	layout, err := deriveMinioLayoutForNodeAgent(cfg, "example.com")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if layout.usersBucket != "users-bkt" || layout.webrootBucket != "web-bkt" {
		t.Fatalf("unexpected buckets: %s %s", layout.usersBucket, layout.webrootBucket)
	}
	if layout.usersPrefix != "custom/users" {
		t.Fatalf("unexpected users prefix: %s", layout.usersPrefix)
	}
	if layout.webrootPrefix != "custom/webroot" {
		t.Fatalf("unexpected webroot prefix: %s", layout.webrootPrefix)
	}
}

func TestParseEnsureObjectstoreArgsDefaults(t *testing.T) {
	args := parseEnsureObjectstoreArgs(nil)
	if args.ContractPath != defaultMinioContractPath {
		t.Fatalf("default contract path mismatch: %s", args.ContractPath)
	}
	if args.Retry != defaultRetryAttempts {
		t.Fatalf("default retry mismatch: %d", args.Retry)
	}
	if args.RetryDelay != defaultRetryDelay {
		t.Fatalf("default retry delay mismatch: %s", args.RetryDelay)
	}
}

func TestMinioCredentialsFilePerms(t *testing.T) {
	tmp := t.TempDir()
	credPath := tmp + "/creds"
	if err := os.WriteFile(credPath, []byte("a:b"), 0o644); err != nil {
		t.Fatalf("write cred file: %v", err)
	}
	_, err := minioCredentials(&config.MinioProxyAuth{
		Mode:     config.MinioProxyAuthModeFile,
		CredFile: credPath,
	})
	if err == nil {
		t.Fatalf("expected permission error")
	}

	if err := os.WriteFile(credPath, []byte("a:b"), 0o600); err != nil {
		t.Fatalf("write cred file: %v", err)
	}
	if _, err := minioCredentials(&config.MinioProxyAuth{
		Mode:     config.MinioProxyAuthModeFile,
		CredFile: credPath,
	}); err != nil {
		t.Fatalf("unexpected error with strict perms: %v", err)
	}
}

func TestParseEnsureObjectstoreArgsOverride(t *testing.T) {
	fields := map[string]interface{}{
		"contract_path":    "/tmp/minio.json",
		"domain":           "example.com",
		"create_sentinels": false,
		"sentinel_name":    ".sentinel",
		"retry":            5,
		"retry_delay_ms":   500.0,
	}
	s, err := structpb.NewStruct(fields)
	if err != nil {
		t.Fatalf("struct build err: %v", err)
	}
	args := parseEnsureObjectstoreArgs(s)
	if args.ContractPath != "/tmp/minio.json" || args.Domain != "example.com" || args.CreateSentinels != false || args.SentinelName != ".sentinel" {
		t.Fatalf("argument parsing failed: %+v", args)
	}
	if args.Retry != 5 || args.RetryDelay != 500*time.Millisecond {
		t.Fatalf("retry parsing failed: %d %s", args.Retry, args.RetryDelay)
	}
}

func TestMinioConfigFromEnv(t *testing.T) {
	t.Setenv("MINIO_ENDPOINT", "127.0.0.1:9000")
	t.Setenv("MINIO_BUCKET", "bkt")
	t.Setenv("MINIO_PREFIX", "/pref/")
	t.Setenv("MINIO_SECURE", "true")
	t.Setenv("MINIO_ACCESS_KEY", "a")
	t.Setenv("MINIO_SECRET_KEY", "b")

	cfg := minioConfigFromEnv()
	if cfg == nil {
		t.Fatalf("expected config from env")
	}
	if cfg.Endpoint != "127.0.0.1:9000" || cfg.Bucket != "bkt" || cfg.Prefix != "pref" || !cfg.Secure {
		t.Fatalf("unexpected env config: %+v", cfg)
	}
	if cfg.Auth == nil || cfg.Auth.Mode != config.MinioProxyAuthModeAccessKey {
		t.Fatalf("unexpected auth mode from env: %+v", cfg.Auth)
	}
}
