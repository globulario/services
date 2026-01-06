package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

var ErrInvalidObjectStoreContract = errors.New("invalid object store contract")

// LoadMinioProxyConfigFrom parses the provided reader as the installer contract.
func LoadMinioProxyConfigFrom(r io.Reader) (*MinioProxyConfig, error) {
	contract := ObjectStoreContract{Secure: true}
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&contract); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidObjectStoreContract, err)
	}
	if contract.Type != "" && !strings.EqualFold(strings.TrimSpace(contract.Type), "minio") {
		return nil, fmt.Errorf("%w: unsupported contract type %q", ErrInvalidObjectStoreContract, contract.Type)
	}
	cfg := contract.toMinioProxyConfig()
	cfg = NormalizeMinioProxyConfig(cfg)
	if cfg == nil {
		return nil, fmt.Errorf("%w: config missing", ErrInvalidObjectStoreContract)
	}
	if err := ValidateMinioProxyConfig(cfg); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidObjectStoreContract, err)
	}
	return cfg, nil
}

// SaveMinioProxyConfigTo writes the installer contract for the provided config.
func SaveMinioProxyConfigTo(w io.Writer, cfg *MinioProxyConfig) error {
	cfg = NormalizeMinioProxyConfig(cfg)
	if cfg == nil {
		return fmt.Errorf("minio config is nil")
	}
	if err := ValidateMinioProxyConfig(cfg); err != nil {
		return err
	}
	contract := contractFromMinioProxyConfig(cfg)
	data, err := json.MarshalIndent(contract, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal contract: %w", err)
	}
	if _, err := w.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write contract: %w", err)
	}
	return nil
}

// NormalizeMinioProxyConfig trims and normalizes all string fields.
func NormalizeMinioProxyConfig(cfg *MinioProxyConfig) *MinioProxyConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Endpoint = strings.TrimSpace(normalized.Endpoint)
	normalized.Bucket = strings.TrimSpace(normalized.Bucket)
	normalized.Prefix = strings.Trim(strings.TrimSpace(normalized.Prefix), "/")
	normalized.CABundlePath = strings.TrimSpace(normalized.CABundlePath)
	normalized.Auth = normalizeMinioProxyAuth(normalized.Auth)
	return &normalized
}

// ValidateMinioProxyConfig ensures required fields are present.
func ValidateMinioProxyConfig(cfg *MinioProxyConfig) error {
	if cfg == nil {
		return fmt.Errorf("minio config is nil")
	}
	if cfg.Endpoint == "" {
		return fmt.Errorf("minio endpoint is required")
	}
	if cfg.Bucket == "" {
		return fmt.Errorf("minio bucket is required")
	}
	return nil
}

func normalizeMinioProxyAuth(auth *MinioProxyAuth) *MinioProxyAuth {
	if auth == nil {
		return nil
	}
	result := *auth
	result.AccessKey = strings.TrimSpace(result.AccessKey)
	result.SecretKey = strings.TrimSpace(result.SecretKey)
	result.CredFile = strings.TrimSpace(result.CredFile)
	result.Mode = normalizeAuthMode(result.Mode)
	switch result.Mode {
	case MinioProxyAuthModeAccessKey:
		if result.AccessKey == "" || result.SecretKey == "" {
			result.Mode = MinioProxyAuthModeNone
			result.AccessKey = ""
			result.SecretKey = ""
		}
	case MinioProxyAuthModeFile:
		if result.CredFile == "" {
			result.Mode = MinioProxyAuthModeNone
		}
	}
	if result.Mode == MinioProxyAuthModeNone {
		result.AccessKey = ""
		result.SecretKey = ""
		result.CredFile = ""
	}
	return &result
}

func normalizeAuthMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "file":
		return MinioProxyAuthModeFile
	case "none":
		return MinioProxyAuthModeNone
	case "", "accesskey":
		return MinioProxyAuthModeAccessKey
	default:
		return MinioProxyAuthModeNone
	}
}

// ObjectStoreContract mirrors the JSON shape created by the installer.
type ObjectStoreContract struct {
	Type         string                   `json:"type"`
	Endpoint     string                   `json:"endpoint"`
	Bucket       string                   `json:"bucket"`
	Prefix       string                   `json:"prefix,omitempty"`
	Secure       bool                     `json:"secure"`
	CABundlePath string                   `json:"caBundlePath,omitempty"`
	Auth         *ObjectStoreContractAuth `json:"auth"`
}

// ObjectStoreContractAuth describes authentication details in the contract.
type ObjectStoreContractAuth struct {
	Mode      string `json:"mode"`
	AccessKey string `json:"accessKey,omitempty"`
	SecretKey string `json:"secretKey,omitempty"`
	CredFile  string `json:"credFile,omitempty"`
}

func (c *ObjectStoreContract) toMinioProxyConfig() *MinioProxyConfig {
	if c == nil {
		return nil
	}
	prefix := strings.Trim(c.Prefix, "/")
	return &MinioProxyConfig{
		Endpoint:     strings.TrimSpace(c.Endpoint),
		Bucket:       strings.TrimSpace(c.Bucket),
		Prefix:       prefix,
		Secure:       c.Secure,
		CABundlePath: strings.TrimSpace(c.CABundlePath),
		Auth:         c.Auth.toMinioProxyAuth(),
	}
}

func (a *ObjectStoreContractAuth) toMinioProxyAuth() *MinioProxyAuth {
	if a == nil {
		return &MinioProxyAuth{Mode: MinioProxyAuthModeNone}
	}
	mode := normalizeAuthMode(a.Mode)
	auth := &MinioProxyAuth{Mode: mode}
	switch mode {
	case MinioProxyAuthModeAccessKey:
		auth.AccessKey = strings.TrimSpace(a.AccessKey)
		auth.SecretKey = strings.TrimSpace(a.SecretKey)
	case MinioProxyAuthModeFile:
		auth.CredFile = strings.TrimSpace(a.CredFile)
	}
	if mode == MinioProxyAuthModeNone {
		auth.AccessKey = ""
		auth.SecretKey = ""
		auth.CredFile = ""
	}
	return auth
}

func contractFromMinioProxyConfig(cfg *MinioProxyConfig) *ObjectStoreContract {
	prefix := strings.Trim(cfg.Prefix, "/")
	return &ObjectStoreContract{
		Type:         "minio",
		Endpoint:     cfg.Endpoint,
		Bucket:       cfg.Bucket,
		Prefix:       prefix,
		Secure:       cfg.Secure,
		CABundlePath: cfg.CABundlePath,
		Auth:         contractAuthFromMinioProxyAuth(cfg.Auth),
	}
}

func contractAuthFromMinioProxyAuth(auth *MinioProxyAuth) *ObjectStoreContractAuth {
	if auth == nil {
		return &ObjectStoreContractAuth{Mode: MinioProxyAuthModeNone}
	}
	normalized := normalizeMinioProxyAuth(auth)
	result := &ObjectStoreContractAuth{Mode: normalized.Mode}
	switch normalized.Mode {
	case MinioProxyAuthModeAccessKey:
		result.AccessKey = normalized.AccessKey
		result.SecretKey = normalized.SecretKey
	case MinioProxyAuthModeFile:
		result.CredFile = normalized.CredFile
	}
	return result
}
