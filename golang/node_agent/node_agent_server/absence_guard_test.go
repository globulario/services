package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/globular_service/lkg"
)

func setupAbsenceGuardLKGDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	lkg.OverrideBaseDir(dir)
	t.Cleanup(func() { lkg.OverrideBaseDir("/var/lib/globular") })
	return dir
}

// TestMissingKeyDoesNotStopRuntime verifies the non-ingress rollout of
// critical_state.absence_is_not_destructive_intent. Objectstore, xDS, and DNS
// consumers must hold/restore last-known-good state when controller-rendered
// state is absent instead of converting absence into stop/delete intent.
func TestMissingKeyDoesNotStopRuntime(t *testing.T) {
	base := setupAbsenceGuardLKGDir(t)

	assertObjectstoreMissingStateUsesLKG(t, base)
	assertXDSMissingStateUsesLKG(t, base)
	assertDNSMissingStateUsesLKG(t, base)
}

// TestDeleteKeyWhileRunningKeepsRuntimeActive covers the same invariant for
// delete/race shape: after a valid state has been observed, removing the
// controller-rendered file must leave the runtime consumer with LKG state.
func TestDeleteKeyWhileRunningKeepsRuntimeActive(t *testing.T) {
	base := setupAbsenceGuardLKGDir(t)

	xdsPath := filepath.Join(base, "xds", "config.json")
	if err := os.MkdirAll(filepath.Dir(xdsPath), 0o750); err != nil {
		t.Fatalf("mkdir xds: %v", err)
	}
	initialXDS := validXDSConfigJSON("10.0.0.10:2379")
	if err := os.WriteFile(xdsPath, initialXDS, 0o644); err != nil {
		t.Fatalf("write xds: %v", err)
	}
	if _, source, err := loadXDSConfigWithLKGPath(xdsPath); err != nil || source != "file" {
		t.Fatalf("prime xds LKG: source=%q err=%v", source, err)
	}
	if err := os.Remove(xdsPath); err != nil {
		t.Fatalf("remove xds: %v", err)
	}
	cfg, source, err := loadXDSConfigWithLKGPath(xdsPath)
	if err != nil {
		t.Fatalf("xds missing state should use LKG, got error: %v", err)
	}
	if source != "lkg" || cfg == nil || len(cfg.EtcdEndpoints) != 1 || cfg.EtcdEndpoints[0] != "10.0.0.10:2379" {
		t.Fatalf("xds did not hold LKG after delete: source=%q cfg=%+v", source, cfg)
	}

	dnsPath := filepath.Join(base, "dns", "dns_init.json")
	if err := os.MkdirAll(filepath.Dir(dnsPath), 0o750); err != nil {
		t.Fatalf("mkdir dns: %v", err)
	}
	initialDNS := dnsInitConfig{Domain: "globular.internal", IsPrimary: true}
	rawDNS, err := json.Marshal(initialDNS)
	if err != nil {
		t.Fatalf("marshal dns: %v", err)
	}
	if err := os.WriteFile(dnsPath, rawDNS, 0o644); err != nil {
		t.Fatalf("write dns: %v", err)
	}
	if _, source, err := loadDNSInitConfigWithLKGPath(dnsPath); err != nil || source != "file" {
		t.Fatalf("prime dns LKG: source=%q err=%v", source, err)
	}
	if err := os.Remove(dnsPath); err != nil {
		t.Fatalf("remove dns: %v", err)
	}
	dnsCfg, source, err := loadDNSInitConfigWithLKGPath(dnsPath)
	if err != nil {
		t.Fatalf("dns missing state should use LKG, got error: %v", err)
	}
	if source != "lkg" || dnsCfg == nil || dnsCfg.Domain != "globular.internal" {
		t.Fatalf("dns did not hold LKG after delete: source=%q cfg=%+v", source, dnsCfg)
	}
}

func assertObjectstoreMissingStateUsesLKG(t *testing.T, base string) {
	t.Helper()

	cfg := validContract()
	raw, err := marshalMinioContract(cfg)
	if err != nil {
		t.Fatalf("marshal objectstore LKG: %v", err)
	}
	if err := lkg.StoreRaw(minioContractLKGSubsystem, minioContractLKGKey, 7, raw); err != nil {
		t.Fatalf("store objectstore LKG: %v", err)
	}

	path := filepath.Join(base, "objectstore", "minio.json")
	srv := &NodeAgentServer{}
	srv.reconcileMinioContractFromLKGPath(path)
	restored, err := loadMinioContractFromDisk(path)
	if err != nil {
		t.Fatalf("objectstore did not restore LKG: %v", err)
	}
	if !minioContractsEqual(cfg, restored) {
		t.Fatalf("objectstore restored wrong LKG: got=%+v want=%+v", restored, cfg)
	}
}

func assertXDSMissingStateUsesLKG(t *testing.T, base string) {
	t.Helper()

	path := filepath.Join(base, "xds-missing", "config.json")
	raw := validXDSConfigJSON("10.0.0.11:2379")
	if err := lkg.StoreRaw(xdsConfigLKGSubsystem, xdsConfigLKGKey, 8, raw); err != nil {
		t.Fatalf("store xds LKG: %v", err)
	}
	cfg, source, err := loadXDSConfigWithLKGPath(path)
	if err != nil {
		t.Fatalf("xds missing state should use LKG: %v", err)
	}
	if source != "lkg" || cfg == nil || len(cfg.EtcdEndpoints) != 1 || cfg.EtcdEndpoints[0] != "10.0.0.11:2379" {
		t.Fatalf("xds missing state did not use LKG: source=%q cfg=%+v", source, cfg)
	}
}

func assertDNSMissingStateUsesLKG(t *testing.T, base string) {
	t.Helper()

	path := filepath.Join(base, "dns-missing", "dns_init.json")
	raw, err := json.Marshal(dnsInitConfig{Domain: "globular.internal", IsPrimary: true})
	if err != nil {
		t.Fatalf("marshal dns LKG: %v", err)
	}
	if err := lkg.StoreRaw(dnsInitLKGSubsystem, dnsInitLKGKey, 9, raw); err != nil {
		t.Fatalf("store dns LKG: %v", err)
	}
	cfg, source, err := loadDNSInitConfigWithLKGPath(path)
	if err != nil {
		t.Fatalf("dns missing state should use LKG: %v", err)
	}
	if source != "lkg" || cfg == nil || cfg.Domain != "globular.internal" {
		t.Fatalf("dns missing state did not use LKG: source=%q cfg=%+v", source, cfg)
	}
}
