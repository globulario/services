package actions

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	defaultMinioContractPath = "/var/lib/globular/objectstore/minio.json"
	defaultSentinelName      = ".keep"
	defaultRetryAttempts     = 30
	defaultRetryDelay        = time.Second
)

type ensureObjectstoreLayoutArgs struct {
	ContractPath    string
	Domain          string
	CreateSentinels bool
	SentinelName    string
	Retry           int
	RetryDelay      time.Duration
	StrictContract  bool
}

type objectstoreLayout struct {
	usersBucket   string
	webrootBucket string
	usersPrefix   string
	webrootPrefix string
	domain        string
}

type ensureObjectstoreLayoutAction struct{}

func (a *ensureObjectstoreLayoutAction) Name() string { return "ensure_objectstore_layout" }

func (a *ensureObjectstoreLayoutAction) Validate(args *structpb.Struct) error {
	parsed := parseEnsureObjectstoreArgs(args)
	if parsed.Retry < 1 {
		return errors.New("retry must be >= 1")
	}
	return nil
}

func (a *ensureObjectstoreLayoutAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	parsed := parseEnsureObjectstoreArgs(args)
	fmt.Printf("[objectstore] Loading contract from: %s\n", parsed.ContractPath)

	cfg, source, err := loadMinioConfig(parsed.ContractPath, parsed.StrictContract)
	if err != nil {
		fmt.Printf("[objectstore] ERROR loading config: %v\n", err)
		return "", err
	}
	fmt.Printf("[objectstore] MinIO config source: %s endpoint: %s (secure=%t)\n", source, cfg.Endpoint, cfg.Secure)

	layout, err := deriveMinioLayoutForNodeAgent(cfg, parsed.Domain)
	if err != nil {
		fmt.Printf("[objectstore] ERROR deriving layout: %v\n", err)
		return "", err
	}
	fmt.Printf("[objectstore] Layout derived:\n")
	fmt.Printf("    domain: %s\n", layout.domain)
	fmt.Printf("    users_bucket: %s\n", layout.usersBucket)
	fmt.Printf("    users_prefix: %s\n", layout.usersPrefix)
	fmt.Printf("    webroot_bucket: %s\n", layout.webrootBucket)
	fmt.Printf("    webroot_prefix: %s\n", layout.webrootPrefix)

	client, err := buildMinioClient(cfg)
	if err != nil {
		fmt.Printf("[objectstore] ERROR building client: %v\n", err)
		return "", err
	}

	attempts := parsed.Retry
	delay := parsed.RetryDelay
	var lastErr error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			fmt.Printf("[objectstore] Retry attempt %d/%d\n", i+1, attempts)
		}
		if err := ensureLayout(ctx, client, layout, parsed.CreateSentinels, parsed.SentinelName); err == nil {
			fmt.Printf("[objectstore] SUCCESS: objectstore layout ensured for domain %s\n", layout.domain)
			return fmt.Sprintf("objectstore layout ensured for domain %s", layout.domain), nil
		} else {
			lastErr = err
			fmt.Printf("[objectstore] Attempt %d failed: %v\n", i+1, err)
			if i < attempts-1 {
				time.Sleep(delay)
			}
		}
	}
	fmt.Printf("[objectstore] FAILED after %d attempts: %v\n", attempts, lastErr)
	return "", fmt.Errorf("ensure_objectstore_layout failed after %d attempts: %w", attempts, lastErr)
}

func parseEnsureObjectstoreArgs(args *structpb.Struct) ensureObjectstoreLayoutArgs {
	out := ensureObjectstoreLayoutArgs{
		ContractPath:    defaultMinioContractPath,
		CreateSentinels: true,
		SentinelName:    defaultSentinelName,
		Retry:           defaultRetryAttempts,
		RetryDelay:      defaultRetryDelay,
		StrictContract:  true,
	}
	if args == nil {
		out.ContractPath = resolveContractPath()
		return out
	}
	fields := args.GetFields()
	if v, ok := fields["contract_path"]; ok {
		out.ContractPath = strings.TrimSpace(v.GetStringValue())
	}
	if v, ok := fields["domain"]; ok {
		out.Domain = strings.TrimSpace(v.GetStringValue())
	}
	if v, ok := fields["create_sentinels"]; ok {
		out.CreateSentinels = v.GetBoolValue()
	}
	if v, ok := fields["sentinel_name"]; ok {
		if s := strings.TrimSpace(v.GetStringValue()); s != "" {
			out.SentinelName = s
		}
	}
	if v, ok := fields["retry"]; ok {
		if n := int(v.GetNumberValue()); n > 0 {
			out.Retry = n
		}
	}
	if v, ok := fields["retry_delay_ms"]; ok {
		if ms := int64(v.GetNumberValue()); ms > 0 {
			out.RetryDelay = time.Duration(ms) * time.Millisecond
		}
	}
	if v, ok := fields["strict_contract"]; ok {
		out.StrictContract = v.GetBoolValue()
	}
	if out.ContractPath == "" {
		out.ContractPath = resolveContractPath()
	}
	return out
}

func resolveContractPath() string {
	if env := strings.TrimSpace(os.Getenv("GLOBULAR_MINIO_CONTRACT_PATH")); env != "" {
		return env
	}
	if env := strings.TrimSpace(os.Getenv("NODE_AGENT_MINIO_CONTRACT")); env != "" {
		return env
	}
	return defaultMinioContractPath
}

func loadMinioConfig(path string, strict bool) (*config.MinioProxyConfig, string, error) {
	if strings.TrimSpace(path) == "" {
		path = defaultMinioContractPath
	}
	f, err := os.Open(path)
	if err == nil {
		defer f.Close()
		cfg, cfgErr := config.LoadMinioProxyConfigFrom(f)
		if cfgErr != nil {
			return nil, "", fmt.Errorf("load minio contract %s: %w", path, cfgErr)
		}
		return cfg, "contract:" + path, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		fmt.Printf("[objectstore] Contract file not found at %s, trying fallbacks...\n", path)
		if strict {
			return nil, "", fmt.Errorf("minio contract not found at %s (strict contract mode enabled)", path)
		}
		// Fallback to env defaults so Day0 does not silently skip provisioning.
		if envCfg := minioConfigFromEnv(); envCfg != nil {
			fmt.Printf("[objectstore] Using MinIO config from environment variables\n")
			return envCfg, "env", nil
		}
		// Last resort: try localhost defaults if MinIO appears to be running
		if defaultCfg := tryLocalMinioDefaults(); defaultCfg != nil {
			fmt.Printf("[objectstore] Using localhost default MinIO config\n")
			return defaultCfg, "local-default", nil
		}
		return nil, "", fmt.Errorf("minio contract not found at %s and no fallback configuration available (set MINIO_ENDPOINT or create contract file)", path)
	}
	return nil, "", fmt.Errorf("open minio contract %s: %w", path, err)
}

func deriveMinioLayoutForNodeAgent(cfg *config.MinioProxyConfig, domain string) (objectstoreLayout, error) {
	if cfg == nil {
		return objectstoreLayout{}, errors.New("minio config is nil")
	}
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		if dom, err := config.GetDomain(); err == nil && dom != "" {
			domain = dom
		} else {
			domain = "localhost"
		}
	}

	basePrefix := strings.Trim(strings.TrimSpace(cfg.Prefix), "/")
	join := func(parts ...string) string {
		segments := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.Trim(strings.TrimSpace(p), "/"); s != "" {
				segments = append(segments, s)
			}
		}
		return strings.Join(segments, "/")
	}

	usersBucket := strings.TrimSpace(os.Getenv("GLOBULAR_MINIO_USERS_BUCKET"))
	if usersBucket == "" {
		usersBucket = cfg.Bucket
	}
	webrootBucket := strings.TrimSpace(os.Getenv("GLOBULAR_MINIO_WEBROOT_BUCKET"))
	if webrootBucket == "" {
		webrootBucket = cfg.Bucket
	}

	usersPrefix := strings.TrimSpace(os.Getenv("GLOBULAR_MINIO_USERS_PREFIX"))
	if usersPrefix == "" {
		usersPrefix = join(basePrefix, domain, "users")
	} else {
		usersPrefix = strings.Trim(usersPrefix, "/")
	}
	webrootPrefix := strings.TrimSpace(os.Getenv("GLOBULAR_MINIO_WEBROOT_PREFIX"))
	if webrootPrefix == "" {
		webrootPrefix = join(basePrefix, domain, "webroot")
	} else {
		webrootPrefix = strings.Trim(webrootPrefix, "/")
	}

	return objectstoreLayout{
		usersBucket:   usersBucket,
		webrootBucket: webrootBucket,
		usersPrefix:   usersPrefix,
		webrootPrefix: webrootPrefix,
		domain:        domain,
	}, nil
}

func buildMinioClient(cfg *config.MinioProxyConfig) (*minio.Client, error) {
	auth := cfg.Auth
	if auth == nil {
		auth = &config.MinioProxyAuth{Mode: config.MinioProxyAuthModeNone}
	}

	creds, err := minioCredentials(auth)
	if err != nil {
		return nil, err
	}

	opts := &minio.Options{
		Secure: cfg.Secure,
		Creds:  creds,
	}

	if bundle := strings.TrimSpace(cfg.CABundlePath); bundle != "" {
		pem, err := os.ReadFile(bundle)
		if err != nil {
			return nil, fmt.Errorf("read CA bundle: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("parse CA bundle: %s", bundle)
		}
		opts.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: pool,
			},
		}
	}

	return minio.New(cfg.Endpoint, opts)
}

func minioCredentials(auth *config.MinioProxyAuth) (*credentials.Credentials, error) {
	switch auth.Mode {
	case config.MinioProxyAuthModeAccessKey:
		return credentials.NewStaticV4(auth.AccessKey, auth.SecretKey, ""), nil
	case config.MinioProxyAuthModeFile:
		if auth.CredFile == "" {
			return nil, errors.New("credential file path required when auth mode is file")
		}
		info, err := os.Stat(auth.CredFile)
		if err != nil {
			return nil, fmt.Errorf("stat minio credential file: %w", err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			return nil, fmt.Errorf("credential file %s must have permissions 0600 or stricter", auth.CredFile)
		}
		data, err := os.ReadFile(auth.CredFile)
		if err != nil {
			return nil, fmt.Errorf("read minio credential file: %w", err)
		}
		parts := strings.Split(strings.TrimSpace(string(data)), ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid minio credential file format (expected access:secret)")
		}
		return credentials.NewStaticV4(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), ""), nil
	case config.MinioProxyAuthModeNone, "":
		return credentials.NewStaticV4("", "", ""), nil
	default:
		return nil, fmt.Errorf("unknown minio auth mode: %s", auth.Mode)
	}
}

// minioConfigFromEnv returns a MinIO config derived from environment variables when no contract is present.
// Recognizes MINIO_ENDPOINT, MINIO_BUCKET, MINIO_PREFIX, MINIO_SECURE, MINIO_ACCESS_KEY, MINIO_SECRET_KEY.
func minioConfigFromEnv() *config.MinioProxyConfig {
	endpoint := strings.TrimSpace(os.Getenv("MINIO_ENDPOINT"))
	if endpoint == "" {
		return nil
	}
	bucket := strings.TrimSpace(os.Getenv("MINIO_BUCKET"))
	if bucket == "" {
		bucket = "globular"
	}
	prefix := strings.Trim(strings.TrimSpace(os.Getenv("MINIO_PREFIX")), "/")
	secure := strings.EqualFold(strings.TrimSpace(os.Getenv("MINIO_SECURE")), "true")
	access := strings.TrimSpace(os.Getenv("MINIO_ACCESS_KEY"))
	secret := strings.TrimSpace(os.Getenv("MINIO_SECRET_KEY"))
	authMode := config.MinioProxyAuthModeNone
	if access != "" && secret != "" {
		authMode = config.MinioProxyAuthModeAccessKey
	}
	return &config.MinioProxyConfig{
		Endpoint: endpoint,
		Bucket:   bucket,
		Prefix:   prefix,
		Secure:   secure,
		Auth: &config.MinioProxyAuth{
			Mode:      authMode,
			AccessKey: access,
			SecretKey: secret,
		},
	}
}

// tryLocalMinioDefaults attempts to use localhost defaults if MinIO appears to be running.
// This is a last-resort fallback to avoid failing Day-0 due to missing contract.
// Returns nil if localhost MinIO is not detected or credentials are unavailable.
//
// SINGLE SOURCE OF TRUTH: /var/lib/globular/minio/credentials (created by MinIO package)
func tryLocalMinioDefaults() *config.MinioProxyConfig {
	// Standard credentials file location (created by MinIO package)
	credFile := "/var/lib/globular/minio/credentials"

	data, err := os.ReadFile(credFile)
	if err != nil {
		fmt.Printf("[objectstore] Could not read credentials from %s: %v\n", credFile, err)
		return nil
	}

	parts := strings.Split(strings.TrimSpace(string(data)), ":")
	if len(parts) != 2 {
		fmt.Printf("[objectstore] Invalid credentials format in %s\n", credFile)
		return nil
	}

	access := strings.TrimSpace(parts[0])
	secret := strings.TrimSpace(parts[1])

	if access == "" || secret == "" {
		fmt.Printf("[objectstore] Empty credentials in %s\n", credFile)
		return nil
	}

	fmt.Printf("[objectstore] Using localhost defaults with credentials from %s\n", credFile)
	return &config.MinioProxyConfig{
		Endpoint: "127.0.0.1:9000",
		Bucket:   "globular",
		Prefix:   "",
		Secure:   false,
		Auth: &config.MinioProxyAuth{
			Mode:      config.MinioProxyAuthModeAccessKey,
			AccessKey: access,
			SecretKey: secret,
		},
	}
}

func ensureLayout(ctx context.Context, client *minio.Client, layout objectstoreLayout, createSentinels bool, sentinelName string) error {
	fmt.Printf("[objectstore] Ensuring bucket: %s\n", layout.usersBucket)
	if err := ensureBucket(ctx, client, layout.usersBucket); err != nil {
		fmt.Printf("[objectstore] ERROR ensuring users bucket %s: %v\n", layout.usersBucket, err)
		return fmt.Errorf("ensure users bucket: %w", err)
	}
	fmt.Printf("[objectstore] Bucket %s exists or created\n", layout.usersBucket)

	fmt.Printf("[objectstore] Ensuring bucket: %s\n", layout.webrootBucket)
	if err := ensureBucket(ctx, client, layout.webrootBucket); err != nil {
		fmt.Printf("[objectstore] ERROR ensuring webroot bucket %s: %v\n", layout.webrootBucket, err)
		return fmt.Errorf("ensure webroot bucket: %w", err)
	}
	fmt.Printf("[objectstore] Bucket %s exists or created\n", layout.webrootBucket)

	if !createSentinels {
		fmt.Printf("[objectstore] Skipping sentinel creation (disabled)\n")
		return nil
	}
	usersKey := sentinelName
	if up := strings.Trim(strings.Trim(layout.usersPrefix, "/"), "/"); up != "" {
		usersKey = up + "/" + sentinelName
	}
	webrootKey := sentinelName
	if wp := strings.Trim(strings.Trim(layout.webrootPrefix, "/"), "/"); wp != "" {
		webrootKey = wp + "/" + sentinelName
	}

	fmt.Printf("[objectstore] Creating sentinel: %s/%s\n", layout.usersBucket, usersKey)
	if err := ensureSentinel(ctx, client, layout.usersBucket, usersKey); err != nil {
		fmt.Printf("[objectstore] ERROR creating users sentinel: %v\n", err)
		return fmt.Errorf("ensure users sentinel: %w", err)
	}
	fmt.Printf("[objectstore] Sentinel created: %s/%s\n", layout.usersBucket, usersKey)

	fmt.Printf("[objectstore] Creating sentinel: %s/%s\n", layout.webrootBucket, webrootKey)
	if err := ensureSentinel(ctx, client, layout.webrootBucket, webrootKey); err != nil {
		fmt.Printf("[objectstore] ERROR creating webroot sentinel: %v\n", err)
		return fmt.Errorf("ensure webroot sentinel: %w", err)
	}
	fmt.Printf("[objectstore] Sentinel created: %s/%s\n", layout.webrootBucket, webrootKey)

	return nil
}

func ensureBucket(ctx context.Context, client *minio.Client, bucket string) error {
	if bucket == "" {
		return errors.New("bucket name is empty")
	}
	fmt.Printf("[objectstore]   Checking if bucket %s exists...\n", bucket)
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		fmt.Printf("[objectstore]   ERROR checking bucket existence: %v\n", err)
		return err
	}
	if exists {
		fmt.Printf("[objectstore]   Bucket %s already exists\n", bucket)
		return nil
	}
	fmt.Printf("[objectstore]   Bucket %s does not exist, creating...\n", bucket)
	if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		fmt.Printf("[objectstore]   ERROR creating bucket %s: %v\n", bucket, err)
		return err
	}
	fmt.Printf("[objectstore]   Bucket %s created successfully\n", bucket)
	return nil
}

func ensureSentinel(ctx context.Context, client *minio.Client, bucket, key string) error {
	if bucket == "" || key == "" {
		return errors.New("bucket or key is empty")
	}
	if _, err := client.StatObject(ctx, bucket, key, minio.StatObjectOptions{}); err == nil {
		return nil
	} else if !isNotFoundErr(err) {
		return fmt.Errorf("stat sentinel %q failed: %w", key, err)
	}

	reader := bytes.NewReader([]byte{})
	if _, err := client.PutObject(ctx, bucket, key, reader, 0, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	}); err != nil {
		return err
	}
	if _, err := client.StatObject(ctx, bucket, key, minio.StatObjectOptions{}); err != nil {
		return fmt.Errorf("verify sentinel %q: %w", key, err)
	}
	return nil
}

func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	// minio.ToErrorResponse handles wrapped errors too.
	resp := minio.ToErrorResponse(err)
	code := strings.ToLower(resp.Code)
	if code == "nosuchkey" || code == "nosuchobject" || code == "notfound" {
		return true
	}
	// Fallback: conservative string match.
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "no such key") || strings.Contains(msg, "no such object") {
		return true
	}
	return false
}

func init() {
	Register(&ensureObjectstoreLayoutAction{})
}
