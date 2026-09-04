package security

// The SAN config must carry the node's own IP addresses, not just its DNS names.
//
// This pins the property whose absence produced the 1.2.360 node-3 outage.
// reissueLeafViaCAGateway passed only spec.GetAlternateDomains() to the signing
// path, so a certificate re-issued on a non-issuer node came back with DNS SANs
// and NO IP SANs. The node agent detected it ("repair reported success but
// certificate is still unusable: missing IP SAN: 10.10.0.13") and the cluster
// then spent 55 reconcile cycles blaming a healthy etcd, because the controller
// dials node agents BY IP and etcd peers verify each other by IP.
//
// The scenario covering that repair reported PASS 12/12 throughout, since it
// asserted only expiry and chain validity. These tests assert the property the
// repair exists to establish: a leaf that is actually usable on the wire.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readSanConfig generates a SAN config into a temp dir and returns its text.
func readSanConfig(t *testing.T, domain string, alts []string) string {
	t.Helper()
	dir := t.TempDir()
	if err := GenerateSanConfig(domain, dir, "CA", "QC", "Montreal", "Globular", alts); err != nil {
		t.Fatalf("GenerateSanConfig: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	var found string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".conf") {
			found = filepath.Join(dir, e.Name())
			break
		}
	}
	if found == "" {
		t.Fatalf("GenerateSanConfig wrote no .conf file into %s", dir)
	}
	b, err := os.ReadFile(found)
	if err != nil {
		t.Fatalf("read san config: %v", err)
	}
	return string(b)
}

// TestSanConfig_IPAlternatesBecomeIPSANs is the property the gateway re-issue
// path depends on: an alternate that parses as an IP must land in the IP.N
// block, not the DNS block. If this stops holding, passing IPs through
// alternateDomains silently stops working and the outage recurs.
func TestSanConfig_IPAlternatesBecomeIPSANs(t *testing.T) {
	cfg := readSanConfig(t, "globular.internal", []string{
		"globular.internal", "*.globular.internal",
		"10.10.0.13", "10.10.0.100",
	})

	for _, ip := range []string{"10.10.0.13", "10.10.0.100"} {
		if !strings.Contains(cfg, ip) {
			t.Fatalf("IP %s did not reach the SAN config at all:\n%s", ip, cfg)
		}
		// It must be an IP entry. A DNS entry holding a dotted quad does not
		// satisfy a peer that verifies by address.
		if !sanHasIPEntry(cfg, ip) {
			t.Fatalf("IP %s was emitted as a DNS SAN rather than an IP SAN — "+
				"peers that dial by address still reject the leaf:\n%s", ip, cfg)
		}
	}
}

// TestSanConfig_DNSOnlyAlternatesProduceNoIPSANs is the negative control, and
// it reproduces the defect exactly: the call the gateway path used to make
// yields a certificate with no IP coverage at all. Without this control, the
// test above would also pass against an implementation that adds IP SANs
// unconditionally, and the regression would be invisible.
func TestSanConfig_DNSOnlyAlternatesProduceNoIPSANs(t *testing.T) {
	cfg := readSanConfig(t, "globular.internal", []string{
		"globular.internal", "*.globular.internal",
	})
	if sanHasAnyIPEntry(cfg) {
		t.Fatalf("a DNS-only alternate list produced IP SANs; this control can no "+
			"longer distinguish the defect from the fix:\n%s", cfg)
	}
}

// sanHasIPEntry reports whether cfg contains `IP.<n> = <ip>`.
func sanHasIPEntry(cfg, ip string) bool {
	for _, line := range strings.Split(cfg, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "IP.") {
			continue
		}
		if _, v, ok := strings.Cut(line, "="); ok && strings.TrimSpace(v) == ip {
			return true
		}
	}
	return false
}

func sanHasAnyIPEntry(cfg string) bool {
	for _, line := range strings.Split(cfg, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "IP.") {
			return true
		}
	}
	return false
}
