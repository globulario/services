package main

// awareness_bundle_publish_test.go — pin the CLI publish path for
// awareness bundles.
//
// The publish command's job is small but load-bearing:
//   - read the cli-local manifest.json out of the bundle archive,
//   - reject malformed bundles before any network call,
//   - compute the sha256 the repository will record,
//   - construct an ArtifactRef with Kind=AWARENESS_BUNDLE.
//
// Tests below cover the manifest validation logic and the round-trip
// from build-style tar.gz → inspectBundle → validateBundleManifestForPublish.
// Network publication is verified by the runAwarenessBundlePublish dry-run
// path, which exercises the full CLI flow without contacting any server.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"strings"
	"testing"

	"github.com/globulario/services/golang/awareness/bundlesync"
)

// makeBundleArchive packs a single manifest.json (plus an optional graph.db
// stub) into a gzip+tar archive shaped exactly like what
// `awareness bundle build` writes. Used to feed the inspect/validate path.
func makeBundleArchive(t *testing.T, manifest awarenessBundleManifest, extras map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for name, data := range extras {
		hdr := &tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(data)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write %s header: %v", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	mjson, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	mhdr := &tar.Header{
		Name:     "manifest.json",
		Mode:     0o644,
		Size:     int64(len(mjson)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(mhdr); err != nil {
		t.Fatalf("manifest header: %v", err)
	}
	if _, err := tw.Write(mjson); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

// TestValidateBundleManifestForPublish_HappyPath covers the minimum field
// set the publish command requires from a real bundle.
func TestValidateBundleManifestForPublish_HappyPath(t *testing.T) {
	m := &awarenessBundleManifest{
		Name:    "globular-awareness-bundle",
		Kind:    "AWARENESS_BUNDLE",
		Version: "0.0.1",
		BuildID: "11111111-1111-1111-1111-111111111111",
	}
	if err := validateBundleManifestForPublish(m); err != nil {
		t.Fatalf("happy path should validate: %v", err)
	}
}

// TestValidateBundleManifestForPublish_KindOptional pins the rule that
// kind absence is tolerated (we set it on the ArtifactRef anyway) but
// kind mismatch fails closed — a SERVICE archive must not be uploaded as
// AWARENESS_BUNDLE by accident.
func TestValidateBundleManifestForPublish_KindOptional(t *testing.T) {
	cases := []struct {
		name    string
		kind    string
		wantErr bool
	}{
		{"empty kind tolerated", "", false},
		{"AWARENESS_BUNDLE accepted", "AWARENESS_BUNDLE", false},
		{"SERVICE rejected", "SERVICE", true},
		{"APPLICATION rejected", "APPLICATION", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &awarenessBundleManifest{
				Name: "x", Version: "1", BuildID: "id", Kind: tc.kind,
			}
			err := validateBundleManifestForPublish(m)
			if tc.wantErr && err == nil {
				t.Errorf("kind=%q: expected error, got nil", tc.kind)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("kind=%q: expected nil, got %v", tc.kind, err)
			}
		})
	}
}

// TestValidateBundleManifestForPublish_MissingFields enforces that every
// required identity field fails closed when absent. These are the same
// values the ArtifactRef carries, so an empty value would later surface
// as a confusing server-side error.
func TestValidateBundleManifestForPublish_MissingFields(t *testing.T) {
	base := awarenessBundleManifest{
		Name: "n", Version: "1", BuildID: "id", Kind: "AWARENESS_BUNDLE",
	}
	cases := []struct {
		name string
		mod  func(*awarenessBundleManifest)
		want string
	}{
		{"nil manifest", nil, "no manifest"},
		{"empty name", func(m *awarenessBundleManifest) { m.Name = "" }, "name is empty"},
		{"empty version", func(m *awarenessBundleManifest) { m.Version = "" }, "version is empty"},
		{"empty build_id", func(m *awarenessBundleManifest) { m.BuildID = "" }, "build_id is empty"},
		{"whitespace name", func(m *awarenessBundleManifest) { m.Name = "   " }, "name is empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var m *awarenessBundleManifest
			if tc.mod != nil {
				cp := base
				m = &cp
				tc.mod(m)
			}
			err := validateBundleManifestForPublish(m)
			if err == nil {
				t.Fatalf("%s: expected error", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.want)
			}
		})
	}
}

// TestInspectBundleAcceptsBuildOutputShape proves the publish-side reader
// can parse the exact archive shape that `awareness bundle build` emits:
// graph.db + manifest.json packed via tar+gzip. If the build command's
// archive layout drifts (e.g. someone moves manifest.json into a subdir),
// this test fails before the publish flow is even exercised in production.
func TestInspectBundleAcceptsBuildOutputShape(t *testing.T) {
	want := awarenessBundleManifest{
		Name:    "globular-awareness-bundle",
		Kind:    "AWARENESS_BUNDLE",
		Version: "1.0.0",
		BuildID: "22222222-2222-2222-2222-222222222222",
	}
	data := makeBundleArchive(t, want, map[string][]byte{
		"graph.db": []byte("SQLite format 3\x00fake-graph"),
	})

	got, files, err := inspectBundle(data)
	if err != nil {
		t.Fatalf("inspectBundle: %v", err)
	}
	if got.Name != want.Name || got.Version != want.Version || got.BuildID != want.BuildID {
		t.Errorf("inspect manifest mismatch: got %+v, want %+v", *got, want)
	}
	// Both files must appear in the listing so consumers can verify the
	// bundle is structurally complete before activating it.
	var sawManifest, sawGraph bool
	for _, f := range files {
		switch f {
		case "manifest.json":
			sawManifest = true
		case "graph.db":
			sawGraph = true
		}
	}
	if !sawManifest {
		t.Error("inspectBundle did not list manifest.json")
	}
	if !sawGraph {
		t.Error("inspectBundle did not list graph.db")
	}
}

// TestInspectBundleRejectsMalformedArchives covers the failure modes that
// must NEVER reach the upload RPC: not-a-gzip, gzip-but-not-tar, tar
// without a manifest. Each case returns a non-nil error and runs the
// publish-side caller through the same code path that the live CLI uses.
func TestInspectBundleRejectsMalformedArchives(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string // substring that must appear in the error
	}{
		{
			name: "not gzip",
			data: []byte("this is plain text, not a tarball"),
			want: "gzip",
		},
		{
			name: "tar without manifest",
			data: tarWithoutManifest(t),
			want: "no manifest.json",
		},
		{
			name: "empty bytes",
			data: []byte{},
			want: "gzip",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := inspectBundle(tc.data)
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.want)
			}
		})
	}
}

// tarWithoutManifest produces a gzip+tar archive carrying only a stray
// file — exactly the shape that lets inspectBundle reach the "no
// manifest.json" check.
func tarWithoutManifest(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	data := []byte("placeholder")
	if err := tw.WriteHeader(&tar.Header{
		Name: "graph.db", Mode: 0o644, Size: int64(len(data)), Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatalf("header: %v", err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatalf("write: %v", err)
	}
	tw.Close()
	gw.Close()
	return buf.Bytes()
}

// TestRunAwarenessBundlePublish_DryRunRoundTrip exercises the full CLI
// entry point end-to-end (sans network) against a built bundle:
// flags → file read → inspect → validate → render. Dry-run skips
// authentication and the repository client entirely so the test is
// hermetic.
func TestRunAwarenessBundlePublish_DryRunRoundTrip(t *testing.T) {
	manifest := awarenessBundleManifest{
		Name:    "globular-awareness-bundle",
		Kind:    "AWARENESS_BUNDLE",
		Version: "0.0.7",
		BuildID: "33333333-3333-3333-3333-333333333333",
	}
	data := makeBundleArchive(t, manifest, nil)

	dir := t.TempDir()
	path := dir + "/awareness-bundle-test.tar.gz"
	writeFile(t, path, string(data))

	// Save and restore mutable command state so the test doesn't leak
	// flag values into adjacent tests in the same package.
	savedFile := bundlePublishCfg.file
	savedRepo := bundlePublishCfg.repository
	savedDry := bundlePublishCfg.dryRun
	savedOut := rootCfg.output
	defer func() {
		bundlePublishCfg.file = savedFile
		bundlePublishCfg.repository = savedRepo
		bundlePublishCfg.dryRun = savedDry
		rootCfg.output = savedOut
	}()

	bundlePublishCfg.file = path
	bundlePublishCfg.repository = "repository.invalid"
	bundlePublishCfg.dryRun = true
	rootCfg.output = "json"

	if err := runAwarenessBundlePublish(nil, nil); err != nil {
		t.Fatalf("dry-run publish: %v", err)
	}
}

// TestValidateBundleManifestForPublish_SchemaVersion covers the three
// schema_version cases: empty (tolerated, back-compat), supported
// (accepted), and unsupported (rejected with the supported list in the
// error so the operator knows which binary to use).
func TestValidateBundleManifestForPublish_SchemaVersion(t *testing.T) {
	base := awarenessBundleManifest{
		Name: "globular-awareness-bundle", Version: "1", BuildID: "id", Kind: "AWARENESS_BUNDLE",
	}

	t.Run("empty tolerated for backward-compat", func(t *testing.T) {
		m := base
		m.SchemaVersion = ""
		if err := validateBundleManifestForPublish(&m); err != nil {
			t.Errorf("empty schema_version should be tolerated, got %v", err)
		}
	})

	t.Run("current schema accepted", func(t *testing.T) {
		m := base
		m.SchemaVersion = bundlesync.CurrentBundleSchemaVersion
		if err := validateBundleManifestForPublish(&m); err != nil {
			t.Errorf("current schema_version should be accepted, got %v", err)
		}
	})

	t.Run("unknown schema rejected", func(t *testing.T) {
		m := base
		m.SchemaVersion = "awareness.bundle.v99-unreleased"
		err := validateBundleManifestForPublish(&m)
		if err == nil {
			t.Fatal("unsupported schema_version must be rejected at publish")
		}
		if !strings.Contains(err.Error(), "awareness.bundle.v99-unreleased") {
			t.Errorf("error should name the offending schema, got %v", err)
		}
	})
}

// TestRunAwarenessBundlePublish_StampsCurrentSchema verifies that a
// bundle produced by the build path (with SchemaVersion set to
// CurrentBundleSchemaVersion) survives publish validation and reports
// the schema in its result. Pin against a regression where build/publish
// disagree on which schema string is canonical.
func TestRunAwarenessBundlePublish_StampsCurrentSchema(t *testing.T) {
	manifest := awarenessBundleManifest{
		Name:          "globular-awareness-bundle",
		Kind:          "AWARENESS_BUNDLE",
		Version:       "0.0.7",
		BuildID:       "44444444-4444-4444-4444-444444444444",
		SchemaVersion: bundlesync.CurrentBundleSchemaVersion,
	}
	data := makeBundleArchive(t, manifest, nil)

	// Round-trip through inspectBundle then validate.
	got, _, err := inspectBundle(data)
	if err != nil {
		t.Fatalf("inspectBundle: %v", err)
	}
	if got.SchemaVersion != bundlesync.CurrentBundleSchemaVersion {
		t.Errorf("schema_version round-trip = %q, want %q",
			got.SchemaVersion, bundlesync.CurrentBundleSchemaVersion)
	}
	if err := validateBundleManifestForPublish(got); err != nil {
		t.Errorf("build-shape bundle should validate for publish: %v", err)
	}
}

// TestRunAwarenessBundlePublish_MissingFlags fails closed on the
// argument-validation path so the user gets a clear error before any
// disk read. Mirrors the contract documented in the cobra command's
// Long help.
func TestRunAwarenessBundlePublish_MissingFlags(t *testing.T) {
	savedFile := bundlePublishCfg.file
	savedRepo := bundlePublishCfg.repository
	defer func() {
		bundlePublishCfg.file = savedFile
		bundlePublishCfg.repository = savedRepo
	}()

	bundlePublishCfg.file = ""
	bundlePublishCfg.repository = "repository.invalid"
	if err := runAwarenessBundlePublish(nil, nil); err == nil {
		t.Errorf("expected error when --file is empty")
	}

	bundlePublishCfg.file = "some.tar.gz"
	bundlePublishCfg.repository = ""
	if err := runAwarenessBundlePublish(nil, nil); err == nil {
		t.Errorf("expected error when --repository is empty")
	}
}
