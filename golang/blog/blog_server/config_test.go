package main

import (
    "os"
    "path/filepath"
    "testing"
)

func TestDefaultConfigValidates(t *testing.T) {
    cfg := DefaultConfig()

    if cfg.Name != "blog.BlogService" {
        t.Fatalf("Name = %q, want blog.BlogService", cfg.Name)
    }
    if cfg.Port != defaultPort {
        t.Fatalf("Port = %d, want %d", cfg.Port, defaultPort)
    }
    if cfg.Proxy != defaultProxy {
        t.Fatalf("Proxy = %d, want %d", cfg.Proxy, defaultProxy)
    }
    if cfg.Protocol != "grpc" {
        t.Fatalf("Protocol = %q, want grpc", cfg.Protocol)
    }
    if cfg.Version != "0.0.1" {
        t.Fatalf("Version = %q, want 0.0.1", cfg.Version)
    }
    if !cfg.AllowAllOrigins {
        t.Fatal("AllowAllOrigins should default to true")
    }
    if !cfg.KeepAlive {
        t.Fatal("KeepAlive should default to true")
    }
    if err := cfg.Validate(); err != nil {
        t.Fatalf("Validate() error = %v, want nil", err)
    }
}

func TestConfigFileRoundTrip(t *testing.T) {
    cfg := DefaultConfig()
    cfg.ID = "blog-test-id"
    cfg.Domain = "example.local"
    cfg.Address = "example.local:12345"
    cfg.Port = 23456
    cfg.Root = "/var/lib/globular/blog"

    dir := t.TempDir()
    path := filepath.Join(dir, "config.json")

    if err := cfg.SaveToFile(path); err != nil {
        t.Fatalf("SaveToFile() error = %v", err)
    }

    if _, err := os.Stat(path); err != nil {
        t.Fatalf("config file missing: %v", err)
    }

    loaded, err := LoadFromFile(path)
    if err != nil {
        t.Fatalf("LoadFromFile() error = %v", err)
    }

    if loaded.ID != cfg.ID {
        t.Errorf("ID = %q, want %q", loaded.ID, cfg.ID)
    }
    if loaded.Domain != cfg.Domain {
        t.Errorf("Domain = %q, want %q", loaded.Domain, cfg.Domain)
    }
    if loaded.Address != cfg.Address {
        t.Errorf("Address = %q, want %q", loaded.Address, cfg.Address)
    }
    if loaded.Port != cfg.Port {
        t.Errorf("Port = %d, want %d", loaded.Port, cfg.Port)
    }
    if loaded.Root != cfg.Root {
        t.Errorf("Root = %q, want %q", loaded.Root, cfg.Root)
    }
}

func TestCloneDeepCopy(t *testing.T) {
    cfg := DefaultConfig()
    cfg.Keywords = []string{"Example", "Blog", "Service"}
    cfg.Repositories = []string{"repo1"}
    cfg.Discoveries = []string{"disc1"}
    cfg.Dependencies = []string{"dep1", "dep2"}
    cfg.Permissions = []any{"p1", map[string]any{"k": "v"}}
    cfg.TLS.Enabled = true
    cfg.TLS.CertFile = "cert.pem"
    cfg.TLS.KeyFile = "key.pem"
    cfg.TLS.CertAuthorityTrust = "ca.pem"
    cfg.Root = "/data/blog"

    clone := cfg.Clone()

    // Value equality
    if clone.Root != cfg.Root {
        t.Fatalf("clone.Root = %q, want %q", clone.Root, cfg.Root)
    }
    if len(clone.Keywords) != len(cfg.Keywords) {
        t.Fatalf("clone.Keywords len = %d, want %d", len(clone.Keywords), len(cfg.Keywords))
    }
    if clone.TLS.CertFile != cfg.TLS.CertFile {
        t.Fatalf("clone.TLS.CertFile = %q, want %q", clone.TLS.CertFile, cfg.TLS.CertFile)
    }

    // Deep copy checks
    clone.Keywords[0] = "modified"
    clone.Repositories[0] = "changed"
    clone.Dependencies[0] = "changed"
    clone.Permissions[0] = "changed"
    clone.TLS.CertFile = "other.pem"

    if cfg.Keywords[0] == clone.Keywords[0] {
        t.Error("mutating clone.Keywords affected original")
    }
    if cfg.Repositories[0] == clone.Repositories[0] {
        t.Error("mutating clone.Repositories affected original")
    }
    if cfg.Dependencies[0] == clone.Dependencies[0] {
        t.Error("mutating clone.Dependencies affected original")
    }
    if cfg.Permissions[0] == clone.Permissions[0] {
        t.Error("mutating clone.Permissions affected original")
    }
    if cfg.TLS.CertFile == clone.TLS.CertFile {
        t.Error("mutating clone.TLS affected original")
    }
}

func TestBlogSpecificValidation(t *testing.T) {
    cfg := DefaultConfig()

    cfg.Port = 0
    if err := cfg.Validate(); err == nil {
        t.Error("Validate should fail when Port is zero")
    }

    cfg = DefaultConfig()
    cfg.Protocol = ""
    if err := cfg.Validate(); err == nil {
        t.Error("Validate should fail when Protocol is empty")
    }
}
