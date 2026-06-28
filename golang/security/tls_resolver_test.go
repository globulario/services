package security

import "testing"

func TestResolveCAAuthorityRejectsEmptyAddress(t *testing.T) {
	if _, _, err := resolveCAAuthority("", 443); err == nil {
		t.Fatal("expected error for empty CA authority address")
	}
}

func TestResolveCAAuthorityUsesEmbeddedPort(t *testing.T) {
	host, port, err := resolveCAAuthority("gateway.globular.internal:8443", 443)
	if err != nil {
		t.Fatalf("resolveCAAuthority: %v", err)
	}
	if host != "gateway.globular.internal" {
		t.Fatalf("unexpected host %q", host)
	}
	if port != 8443 {
		t.Fatalf("unexpected port %d", port)
	}
}
