package main

import (
	"path/filepath"
	"testing"
)

func TestDefaultConfigValidates(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Name != "ldap.LdapService" {
		t.Fatalf("Name=%q want ldap.LdapService", cfg.Name)
	}
	if cfg.Port != defaultPort || cfg.Proxy != defaultProxy {
		t.Fatalf("ports mismatch: %d/%d", cfg.Port, cfg.Proxy)
	}
	if cfg.Protocol != "grpc" {
		t.Fatalf("Protocol=%q want grpc", cfg.Protocol)
	}
	if cfg.LdapListenAddr == "" || cfg.LdapsListenAddr == "" {
		t.Fatal("LDAP listen addresses should be defaulted")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConfigFileRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ID = "ldap-1"
	cfg.Domain = "example.com"
	cfg.Address = "example.com:1234"
	cfg.LdapListenAddr = "127.0.0.1:3890"
	cfg.LdapsListenAddr = "127.0.0.1:6360"

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := cfg.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile: %v", err)
	}
	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if loaded.ID != cfg.ID || loaded.Address != cfg.Address || loaded.LdapListenAddr != cfg.LdapListenAddr {
		t.Fatalf("round-trip mismatch: %+v vs %+v", loaded, cfg)
	}
}

func TestCloneDeepCopy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Keywords = []string{"LDAP"}
	cfg.Dependencies = []string{"dep"}
	cfg.Connections = map[string]connection{"c1": {Id: "c1", Host: "h", Port: 389}}
	cfg.LdapSyncInfos = map[string]interface{}{"k": map[string]interface{}{"v": "x"}}
	cfg.Permissions = []any{"p1"}

	clone := cfg.Clone()

	clone.Keywords[0] = "changed"
	clone.Dependencies[0] = "changed"
	clone.Connections["c1"] = connection{Id: "c1", Host: "other"}
	clone.LdapSyncInfos["k"].(map[string]interface{})["v"] = "y"
	clone.Permissions[0] = "p2"

	if cfg.Keywords[0] == clone.Keywords[0] ||
		cfg.Dependencies[0] == clone.Dependencies[0] ||
		cfg.Connections["c1"].Host == clone.Connections["c1"].Host ||
		cfg.LdapSyncInfos["k"].(map[string]interface{})["v"] == clone.LdapSyncInfos["k"].(map[string]interface{})["v"] ||
		cfg.Permissions[0] == clone.Permissions[0] {
		t.Fatal("clone mutation leaked to original")
	}
}

func TestLdapSpecificValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LdapListenAddr = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for empty LdapListenAddr")
	}
}
