package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalTLSPathsAreDomainAgnostic(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("GLOBULAR_STATE_DIR", tmp)

	tlsDir, fullchain, privkey, ca := CanonicalTLSPaths(GetRuntimeConfigDir())
	for name, p := range map[string]string{
		"tlsDir":    tlsDir,
		"fullchain": fullchain,
		"privkey":   privkey,
		"ca":        ca,
	} {
		if !strings.Contains(p, filepath.Join("config", "tls")) {
			t.Fatalf("%s should contain /config/tls/, got %s", name, p)
		}
		if strings.Contains(p, "example.com") {
			t.Fatalf("%s should not contain domain-scoped path: %s", name, p)
		}
	}
}
