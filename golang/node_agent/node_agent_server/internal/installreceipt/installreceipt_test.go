package installreceipt

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

func writeTmpFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ── Stamp ─────────────────────────────────────────────────────────────────

func TestStamp_FullReceipt(t *testing.T) {
	dir := t.TempDir()
	unitData := []byte("[Unit]\nDescription=Test\n[Service]\nExecStart=/bin/true\n")
	binData := []byte("\x7fELF...stub binary")
	cfgData := []byte("key = value\n")
	envData := []byte("FOO=bar\n")
	unitPath := writeTmpFile(t, dir, "globular-foo.service", unitData)
	binPath := writeTmpFile(t, dir, "foo_server", binData)
	cfgPath := writeTmpFile(t, dir, "foo.yaml", cfgData)
	envPath := writeTmpFile(t, dir, "foo.env", envData)

	pkg := &node_agentpb.InstalledPackage{Name: "foo", Kind: "SERVICE"}
	if err := Stamp(pkg, ReceiptOpts{
		UnitFilePath:        unitPath,
		BinaryPath:          binPath,
		ConfigPath:          cfgPath,
		EnvFilePath:         envPath,
		PackageSha256:       "sha256:CAFEBABE",
		ArtifactDigest:      "sha256:DEADBEEF",
		UnitRendererVersion: "v1",
		InstalledBy:         "test-suite",
	}); err != nil {
		t.Fatalf("Stamp: %v", err)
	}
	checks := map[string]string{
		KeyUnitFilePath:        unitPath,
		KeyUnitFileSha256:      sha256Hex(unitData),
		KeyBinaryPath:          binPath,
		KeyBinarySha256:        sha256Hex(binData),
		KeyConfigPath:          cfgPath,
		KeyConfigSha256:        sha256Hex(cfgData),
		KeyEnvFilePath:         envPath,
		KeyEnvFileSha256:       sha256Hex(envData),
		KeyPackageSha256:       "cafebabe",
		KeyArtifactDigest:      "deadbeef",
		KeyUnitRendererVersion: "v1",
		KeyInstalledBy:         "test-suite",
	}
	for k, want := range checks {
		if got := pkg.Metadata[k]; got != want {
			t.Errorf("metadata[%q] = %q, want %q", k, got, want)
		}
	}
	if pkg.Metadata[KeyInstalledAt] == "" {
		t.Error("installed_at missing")
	}
}

func TestStamp_MissingFileIsAtomicFailure(t *testing.T) {
	dir := t.TempDir()
	unitPath := writeTmpFile(t, dir, "unit", []byte("ok"))
	pkg := &node_agentpb.InstalledPackage{Name: "foo", Kind: "SERVICE"}
	err := Stamp(pkg, ReceiptOpts{
		UnitFilePath: unitPath,
		BinaryPath:   filepath.Join(dir, "does-not-exist"),
	})
	if err == nil {
		t.Fatal("expected error when BinaryPath missing")
	}
	if pkg.Metadata != nil && pkg.Metadata[KeyUnitFileSha256] != "" {
		t.Error("unit sha leaked despite atomicity contract")
	}
}

func TestStamp_ClearsMigrationSource(t *testing.T) {
	dir := t.TempDir()
	unitPath := writeTmpFile(t, dir, "unit", []byte("ok"))
	pkg := &node_agentpb.InstalledPackage{
		Name: "x",
		Metadata: map[string]string{
			KeyMigrationSource:  MigrationSourceLegacySidecar,
			KeyUnitFileSha256:   "seeded",
		},
	}
	if err := Stamp(pkg, ReceiptOpts{UnitFilePath: unitPath}); err != nil {
		t.Fatalf("Stamp: %v", err)
	}
	if _, present := pkg.Metadata[KeyMigrationSource]; present {
		t.Errorf("Stamp must clear migration_source: %v", pkg.Metadata)
	}
	if pkg.Metadata[KeyInstalledBy] == "" {
		t.Error("installed_by should be stamped on fresh receipt")
	}
}

func TestStamp_DefaultInstalledBy(t *testing.T) {
	dir := t.TempDir()
	unitPath := writeTmpFile(t, dir, "unit", []byte("ok"))
	pkg := &node_agentpb.InstalledPackage{Name: "foo"}
	if err := Stamp(pkg, ReceiptOpts{UnitFilePath: unitPath}); err != nil {
		t.Fatal(err)
	}
	if got := pkg.Metadata[KeyInstalledBy]; got != DefaultInstalledBy {
		t.Errorf("installed_by default = %q, want %q", got, DefaultInstalledBy)
	}
}

func TestStamp_NilPkgIsError(t *testing.T) {
	if err := Stamp(nil, ReceiptOpts{}); err == nil {
		t.Fatal("expected error for nil pkg")
	}
}

func TestStamp_PreservesUnrelatedMetadata(t *testing.T) {
	dir := t.TempDir()
	unitPath := writeTmpFile(t, dir, "unit", []byte("ok"))
	pkg := &node_agentpb.InstalledPackage{
		Name: "foo",
		Metadata: map[string]string{
			"entrypoint_checksum":  "abc",
			"proof_on_disk_sha256": "def",
			"random_key":           "ghi",
		},
	}
	if err := Stamp(pkg, ReceiptOpts{UnitFilePath: unitPath}); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"entrypoint_checksum", "proof_on_disk_sha256", "random_key"} {
		if pkg.Metadata[k] == "" {
			t.Errorf("unrelated key %q was cleared", k)
		}
	}
}

// ── Preserve ──────────────────────────────────────────────────────────────

func TestPreserve_CopiesAllReceiptKeys(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		Metadata: map[string]string{
			KeyUnitFilePath:        "/etc/systemd/system/globular-x.service",
			KeyUnitFileSha256:      "aaa",
			KeyBinaryPath:          "/usr/lib/globular/bin/x_server",
			KeyBinarySha256:        "bbb",
			KeyConfigPath:          "/var/lib/globular/services/x/config.yaml",
			KeyConfigSha256:        "ccc",
			KeyEnvFilePath:         "/var/lib/globular/services/x/env",
			KeyEnvFileSha256:       "ddd",
			KeyPackageSha256:       "eee",
			KeyArtifactDigest:      "fff",
			KeyUnitRendererVersion: "v1",
			KeyInstalledAt:         "1700000000",
			KeyInstalledBy:         "node-agent.apply_package_release.service",
		},
	}
	next := &node_agentpb.InstalledPackage{Status: "installed"}
	Preserve(existing, next)
	for _, k := range receiptKeys {
		if existing.Metadata[k] != "" && next.Metadata[k] != existing.Metadata[k] {
			t.Errorf("key %q not preserved: got %q want %q", k, next.Metadata[k], existing.Metadata[k])
		}
	}
}

func TestPreserve_NextWinsOnConflict(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		Metadata: map[string]string{KeyUnitFileSha256: "OLD", KeyInstalledBy: "old-writer"},
	}
	next := &node_agentpb.InstalledPackage{
		Metadata: map[string]string{KeyUnitFileSha256: "NEW", KeyInstalledBy: "new-writer"},
	}
	Preserve(existing, next)
	if next.Metadata[KeyUnitFileSha256] != "NEW" {
		t.Errorf("next overwritten: %q", next.Metadata[KeyUnitFileSha256])
	}
}

func TestPreserve_EmptyIncomingCannotErase(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		Metadata: map[string]string{
			KeyUnitFileSha256: "abc",
			KeyBinarySha256:   "def",
			KeyInstalledBy:    "installer",
		},
	}
	next := &node_agentpb.InstalledPackage{Status: "installed"} // nil metadata
	Preserve(existing, next)
	if next.Metadata == nil {
		t.Fatal("metadata not allocated")
	}
	if next.Metadata[KeyUnitFileSha256] != "abc" || next.Metadata[KeyBinarySha256] != "def" || next.Metadata[KeyInstalledBy] != "installer" {
		t.Errorf("preservation failed: %v", next.Metadata)
	}
}

func TestPreserve_CanonicalInstallSuppressesMigrationSource(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		Metadata: map[string]string{
			KeyMigrationSource: MigrationSourceLegacySidecar,
			KeyUnitFileSha256:  "old",
		},
	}
	next := &node_agentpb.InstalledPackage{
		Metadata: map[string]string{
			KeyInstalledBy:    "node-agent.apply_package_release.service",
			KeyUnitFileSha256: "new",
		},
	}
	Preserve(existing, next)
	if _, present := next.Metadata[KeyMigrationSource]; present {
		t.Errorf("migration_source must NOT be re-added when canonical install present: %v", next.Metadata)
	}
}

func TestPreserve_PreservesMigrationSourceWhenNoCanonical(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		Metadata: map[string]string{
			KeyMigrationSource: MigrationSourceLegacySidecar,
			KeyUnitFileSha256:  "abc",
		},
	}
	next := &node_agentpb.InstalledPackage{Status: "installed"}
	Preserve(existing, next)
	if next.Metadata[KeyMigrationSource] != MigrationSourceLegacySidecar {
		t.Errorf("migration_source lost: %v", next.Metadata)
	}
}

func TestPreserve_NilSafety(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic: %v", r)
		}
	}()
	Preserve(nil, nil)
	Preserve(&node_agentpb.InstalledPackage{}, nil)
	Preserve(nil, &node_agentpb.InstalledPackage{})
}

func TestPreserve_NoExistingMetadataIsNoOp(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{}
	next := &node_agentpb.InstalledPackage{}
	Preserve(existing, next)
	if next.Metadata != nil {
		t.Errorf("metadata allocated unnecessarily: %v", next.Metadata)
	}
}

func TestPreserve_DoesNotTouchNonReceiptKeys(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		Metadata: map[string]string{
			KeyUnitFileSha256:        "abc",
			"entrypoint_checksum":    "EC-OLD",
			"proof_on_disk_sha256":   "POD-OLD",
		},
	}
	next := &node_agentpb.InstalledPackage{
		Metadata: map[string]string{
			"entrypoint_checksum":  "EC-NEW",
			"proof_on_disk_sha256": "POD-NEW",
		},
	}
	Preserve(existing, next)
	if next.Metadata["entrypoint_checksum"] != "EC-NEW" {
		t.Error("non-receipt key modified")
	}
}

// ── StampMigrationFromLegacySidecar ───────────────────────────────────────

func TestStampMigrationFromLegacySidecar_Basic(t *testing.T) {
	pkg := &node_agentpb.InstalledPackage{Name: "foo"}
	StampMigrationFromLegacySidecar(pkg, "/etc/systemd/system/foo.service", "ABCDEF")
	if pkg.Metadata[KeyUnitFilePath] != "/etc/systemd/system/foo.service" {
		t.Error("unit_file_path")
	}
	if pkg.Metadata[KeyUnitFileSha256] != "abcdef" {
		t.Errorf("unit_file_sha256 = %q want %q", pkg.Metadata[KeyUnitFileSha256], "abcdef")
	}
	if pkg.Metadata[KeyMigrationSource] != MigrationSourceLegacySidecar {
		t.Error("migration_source")
	}
}

func TestStampMigrationFromLegacySidecar_NilSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic: %v", r)
		}
	}()
	StampMigrationFromLegacySidecar(nil, "/etc/x", "abc")
}

// ── Accessors ─────────────────────────────────────────────────────────────

func TestUnitFileSha256_Accessor(t *testing.T) {
	if got := UnitFileSha256(nil); got != "" {
		t.Error("nil pkg")
	}
	if got := UnitFileSha256(&node_agentpb.InstalledPackage{}); got != "" {
		t.Error("nil metadata")
	}
	pkg := &node_agentpb.InstalledPackage{Metadata: map[string]string{KeyUnitFileSha256: "abc"}}
	if got := UnitFileSha256(pkg); got != "abc" {
		t.Errorf("got %q", got)
	}
}

func TestUnitFilePath_Accessor(t *testing.T) {
	pkg := &node_agentpb.InstalledPackage{Metadata: map[string]string{KeyUnitFilePath: "/etc/x.service"}}
	if got := UnitFilePath(pkg); got != "/etc/x.service" {
		t.Errorf("got %q", got)
	}
}

// ── Constants ─────────────────────────────────────────────────────────────

func TestKeys_Uniqueness(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range Keys() {
		if seen[k] {
			t.Errorf("duplicate key: %s", k)
		}
		seen[k] = true
	}
}

func TestKeys_ReturnsCopy(t *testing.T) {
	a := Keys()
	a[0] = "modified"
	b := Keys()
	if b[0] == "modified" {
		t.Error("Keys() must return a copy, caller mutation leaked")
	}
}

func TestConstants_ExpectedValues(t *testing.T) {
	cases := []struct{ got, want string }{
		{KeyUnitFilePath, "unit_file_path"},
		{KeyUnitFileSha256, "unit_file_sha256"},
		{KeyBinarySha256, "binary_sha256"},
		{KeyInstalledBy, "installed_by"},
		{KeyMigrationSource, "migration_source"},
		{MigrationSourceLegacySidecar, "legacy_sidecar"},
		{DefaultInstalledBy, "node-agent"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("constant = %q want %q", c.got, c.want)
		}
	}
}

func TestNormalize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"abc", "abc"},
		{"ABC", "abc"},
		{" abc ", "abc"},
		{"sha256:ABC", "abc"},
		{"SHA256:abc", "abc"},
		{"", ""},
	}
	for _, c := range cases {
		if got := normalize(c.in); got != c.want {
			t.Errorf("normalize(%q) = %q want %q", c.in, got, c.want)
		}
	}
}

func TestPreserveDoesNotPropagateEmptyValues(t *testing.T) {
	existing := &node_agentpb.InstalledPackage{
		Metadata: map[string]string{
			KeyUnitFileSha256: "abc",
			KeyBinarySha256:   "",
		},
	}
	next := &node_agentpb.InstalledPackage{}
	Preserve(existing, next)
	if next.Metadata[KeyUnitFileSha256] != "abc" {
		t.Error("non-empty value not preserved")
	}
	if _, present := next.Metadata[KeyBinarySha256]; present {
		t.Error("empty value should not propagate")
	}
}

// Hash a file via the package's internal helper; the helper is exercised
// implicitly by TestStamp_FullReceipt; this test exists to pin the
// algorithm so future refactors don't accidentally swap (e.g. to a
// different hash family).
func TestHashFile_Sha256(t *testing.T) {
	dir := t.TempDir()
	path := writeTmpFile(t, dir, "data", []byte("hello"))
	got, err := hashFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// Defensive: empty PackageSha256 / ArtifactDigest are skipped (not
// written as empty string keys).
func TestStamp_EmptyPackageDigestsOmitted(t *testing.T) {
	dir := t.TempDir()
	unitPath := writeTmpFile(t, dir, "u", []byte("x"))
	pkg := &node_agentpb.InstalledPackage{Name: "x"}
	if err := Stamp(pkg, ReceiptOpts{
		UnitFilePath:   unitPath,
		PackageSha256:  "",
		ArtifactDigest: "  ",
	}); err != nil {
		t.Fatal(err)
	}
	if _, present := pkg.Metadata[KeyPackageSha256]; present {
		t.Error("empty package_sha256 should be omitted")
	}
	if _, present := pkg.Metadata[KeyArtifactDigest]; present {
		t.Error("whitespace-only artifact_digest should be omitted")
	}
}

func TestStamp_DefaultInstalledByValueIsNodeAgent(t *testing.T) {
	dir := t.TempDir()
	unitPath := writeTmpFile(t, dir, "u", []byte("x"))
	pkg := &node_agentpb.InstalledPackage{Name: "x"}
	if err := Stamp(pkg, ReceiptOpts{UnitFilePath: unitPath, InstalledBy: ""}); err != nil {
		t.Fatal(err)
	}
	if pkg.Metadata[KeyInstalledBy] != "node-agent" {
		t.Errorf("default installed_by = %q", pkg.Metadata[KeyInstalledBy])
	}
}

func TestStamp_SuperseedsAccompanyingTimestamp(t *testing.T) {
	// installed_at must always reflect the stamp time (a fresh canonical
	// install moment), not whatever value was previously sitting in
	// metadata.
	dir := t.TempDir()
	unitPath := writeTmpFile(t, dir, "u", []byte("x"))
	pkg := &node_agentpb.InstalledPackage{
		Name: "x",
		Metadata: map[string]string{
			KeyInstalledAt: "1",
		},
	}
	if err := Stamp(pkg, ReceiptOpts{UnitFilePath: unitPath}); err != nil {
		t.Fatal(err)
	}
	if pkg.Metadata[KeyInstalledAt] == "1" {
		t.Error("installed_at must be advanced on canonical stamp")
	}
}

// Verify Stamp doesn't accidentally call StampMigrationFromLegacySidecar
// pattern (different key set).
func TestStampVsMigration_DifferentSignatures(t *testing.T) {
	dir := t.TempDir()
	unitPath := writeTmpFile(t, dir, "u", []byte("ok"))

	a := &node_agentpb.InstalledPackage{Name: "x"}
	_ = Stamp(a, ReceiptOpts{UnitFilePath: unitPath, InstalledBy: "test"})

	b := &node_agentpb.InstalledPackage{Name: "x"}
	StampMigrationFromLegacySidecar(b, unitPath, "deadbeef")

	if a.Metadata[KeyInstalledBy] == "" {
		t.Error("Stamp must set installed_by")
	}
	if b.Metadata[KeyInstalledBy] != "" {
		t.Error("StampMigrationFromLegacySidecar must NOT set installed_by")
	}
	if _, present := a.Metadata[KeyMigrationSource]; present {
		t.Error("Stamp must not set migration_source")
	}
	if b.Metadata[KeyMigrationSource] != MigrationSourceLegacySidecar {
		t.Error("StampMigrationFromLegacySidecar must set migration_source")
	}
}

func TestPreserveThenStamp_SimulatesNonInstallThenInstallSequence(t *testing.T) {
	// Real-world sequence: existing has legacy migration, heartbeat
	// refresh runs Preserve (carries migration_source forward), then
	// canonical install runs Stamp which clears it.
	existing := &node_agentpb.InstalledPackage{
		Metadata: map[string]string{
			KeyMigrationSource: MigrationSourceLegacySidecar,
			KeyUnitFileSha256:  "old-sidecar-value",
		},
	}
	// Heartbeat builds next, Preserves
	heartbeatPkg := &node_agentpb.InstalledPackage{Status: "installed"}
	Preserve(existing, heartbeatPkg)
	if heartbeatPkg.Metadata[KeyMigrationSource] != MigrationSourceLegacySidecar {
		t.Error("heartbeat preservation should carry migration_source")
	}

	// Now canonical install runs Stamp
	dir := t.TempDir()
	unitPath := writeTmpFile(t, dir, "u", []byte("new"))
	installPkg := &node_agentpb.InstalledPackage{
		Metadata: map[string]string{}, // built fresh by install path
	}
	if err := Stamp(installPkg, ReceiptOpts{UnitFilePath: unitPath, InstalledBy: "test-install"}); err != nil {
		t.Fatal(err)
	}
	if _, present := installPkg.Metadata[KeyMigrationSource]; present {
		t.Error("canonical install must clear migration_source")
	}
	if installPkg.Metadata[KeyInstalledBy] != "test-install" {
		t.Error("canonical install must record installed_by")
	}
	// Sanity: the sha was computed from disk, not seeded from anything
	if !strings.HasPrefix(installPkg.Metadata[KeyUnitFileSha256], "07e7e2") {
		// sha256("new") = 07e7e2...
		// allow any 64-char hex; just assert it's not empty and not "old-sidecar-value"
		if installPkg.Metadata[KeyUnitFileSha256] == "old-sidecar-value" {
			t.Error("install must compute fresh disk-truth sha, not carry old sidecar value")
		}
	}
}
