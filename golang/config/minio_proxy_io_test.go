package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func TestLoadMinioProxyConfigFrom_ValidContract(t *testing.T) {
	contract := ObjectStoreContract{
		Type:         "minio",
		Endpoint:     " https://example.com ",
		Bucket:       " bucket ",
		Prefix:       " /prefix/ ",
		Secure:       true,
		CABundlePath: " /tmp/ca.pem ",
		Auth: &ObjectStoreContractAuth{
			Mode:      "accessKey",
			AccessKey: " ak ",
			SecretKey: " sk ",
		},
	}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatalf("marshal contract: %v", err)
	}
	cfg, err := LoadMinioProxyConfigFrom(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}
	if cfg.Endpoint != "https://example.com" {
		t.Fatalf("unexpected endpoint %q", cfg.Endpoint)
	}
	if cfg.Bucket != "bucket" {
		t.Fatalf("unexpected bucket %q", cfg.Bucket)
	}
	if cfg.Prefix != "prefix" {
		t.Fatalf("unexpected prefix %q", cfg.Prefix)
	}
	if cfg.CABundlePath != "/tmp/ca.pem" {
		t.Fatalf("unexpected CABundlePath %q", cfg.CABundlePath)
	}
	if cfg.Auth == nil || cfg.Auth.Mode != MinioProxyAuthModeAccessKey {
		t.Fatalf("unexpected auth mode %+v", cfg.Auth)
	}
	if cfg.Auth.AccessKey != "ak" || cfg.Auth.SecretKey != "sk" {
		t.Fatalf("unexpected auth keys %+v", cfg.Auth)
	}
}

func TestLoadMinioProxyConfigFrom_InvalidContract(t *testing.T) {
	_, err := LoadMinioProxyConfigFrom(bytes.NewReader([]byte("{")))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrInvalidObjectStoreContract) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadMinioProxyConfigFrom_InvalidType(t *testing.T) {
	contract := ObjectStoreContract{
		Type:     "s3",
		Endpoint: "https://example.com",
		Bucket:   "bucket",
		Auth: &ObjectStoreContractAuth{
			Mode: "none",
		},
	}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatalf("marshal contract: %v", err)
	}
	_, err = LoadMinioProxyConfigFrom(bytes.NewReader(data))
	if err == nil || !errors.Is(err, ErrInvalidObjectStoreContract) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadMinioProxyConfigFrom_MissingEndpoint(t *testing.T) {
	contract := ObjectStoreContract{
		Type:   "minio",
		Bucket: "bucket",
		Auth: &ObjectStoreContractAuth{
			Mode: "none",
		},
	}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatalf("marshal contract: %v", err)
	}
	_, err = LoadMinioProxyConfigFrom(bytes.NewReader(data))
	if err == nil || !errors.Is(err, ErrInvalidObjectStoreContract) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSaveMinioProxyConfigTo_RoundTrip(t *testing.T) {
	cfg := &MinioProxyConfig{
		Endpoint:     "https://proxy",
		Bucket:       "bucket",
		Prefix:       "/content/",
		Secure:       false,
		CABundlePath: "/tmp/ca",
		Auth: &MinioProxyAuth{
			Mode:      MinioProxyAuthModeAccessKey,
			AccessKey: "ak",
			SecretKey: "sk",
		},
	}
	var buf bytes.Buffer
	if err := SaveMinioProxyConfigTo(&buf, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	loaded, err := LoadMinioProxyConfigFrom(&buf)
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	if loaded.Endpoint != cfg.Endpoint {
		t.Fatalf("endpoint mismatch: %q", loaded.Endpoint)
	}
	if loaded.Prefix != "content" {
		t.Fatalf("prefix mismatch: %q", loaded.Prefix)
	}
	if loaded.Auth == nil || loaded.Auth.Mode != MinioProxyAuthModeAccessKey {
		t.Fatalf("auth mismatch: %+v", loaded.Auth)
	}
}
