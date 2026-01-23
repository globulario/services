package actions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeNetworkOverlayProtoJSON(t *testing.T) {
	input := []byte(`{
		"clusterDomain":"new.example.com",
		"protocol":"https",
		"portHttp":80,
		"portHttps":443,
		"acmeEnabled":true,
		"adminEmail":"admin@new.example.com",
		"alternateDomains":["*.new.example.com"]
	}`)
	got, err := decodeNetworkOverlayFromProtoJSON(input)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	want := map[string]bool{
		"Domain":           true,
		"Protocol":         true,
		"PortHTTP":         true,
		"PortHTTPS":        true,
		"ACMEEnabled":      true,
		"AdminEmail":       true,
		"AlternateDomains": true,
	}
	for key := range want {
		if _, ok := got[key]; !ok {
			t.Fatalf("expected key %s", key)
		}
	}
	if got["Domain"] != "new.example.com" {
		t.Fatalf("expected domain new.example.com got %v", got["Domain"])
	}
}

func TestMergeNetworkOverlayCanonical(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "network.json")
	// existing config with extra key
	existing := []byte(`{"Domain":"old.example.com","Protocol":"http","Extra":"keep"}`)
	if err := os.WriteFile(target, existing, 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}
	overlay := map[string]interface{}{
		"Domain":   "old.example.com",
		"Protocol": "https",
		"PortHTTP": float64(8080),
	}
	if err := mergeNetworkIntoConfig(target, overlay); err != nil {
		t.Fatalf("merge error: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read merged: %v", err)
	}
	if !contains(data, `"Protocol": "https"`) {
		t.Fatalf("protocol not updated: %s", data)
	}
	if !contains(data, `"Extra": "keep"`) {
		t.Fatalf("extra key removed unexpectedly: %s", data)
	}
}

func contains(b []byte, substr string) bool {
	return string(b) == substr || (len(b) > 0 && len(substr) > 0 && (string(b) == substr || func() bool {
		return len(b) >= len(substr) && string(b[len(b)-len(substr):]) == substr || string(b[:len(substr)]) == substr || string(b) == substr
	}()))
}
