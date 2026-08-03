package pkgpack

import (
	"encoding/json"
	"strings"
	"testing"
)

// Declared no-entrypoint binary identity.
//
// A package may declare `identity.proof: binary_sha256` while shipping no
// entrypoint of its own — the payload arrives via a bundled .deb, an OS
// package, or a script. That declaration is a PROMISE the node-agent must be
// able to keep: "re-hash the binary at this path and compare it to this
// checksum".
//
// The defect these pin: the builder emitted the proof mode and the checksum but
// never serialized identity.installed_path, so the archive carried a promise
// with no subject. The node-agent, seeing entrypoint=="none" first, returned
// NotApplicable and recorded success without hashing anything.

const (
	testValidSHA = "1111222233334444555566667777888899990000aaaabbbbccccddddeeeeffff"
	testAbsPath  = "/usr/lib/x86_64-linux-gnu/libexample.so.2"
)

// 1. The supported shape serializes all four fields.
func TestDeclaredIdentity_NoEntrypointBinarySHA_SerializesInstalledPath(t *testing.T) {
	id := &PackageIdentity{
		Proof:         ProofBinarySHA256,
		InstalledPath: testAbsPath,
		Checksum:      "sha256:" + testValidSHA,
	}
	if err := validateDeclaredIdentity(id, true); err != nil {
		t.Fatalf("valid declaration rejected: %v", err)
	}
	if got := normalizeIdentityInstalledPath(id.InstalledPath); got != testAbsPath {
		t.Errorf("installed path = %q, want the exact declared value %q", got, testAbsPath)
	}
	if got := normalizeIdentityChecksum(id.Checksum); got != "sha256:"+testValidSHA {
		t.Errorf("checksum = %q, want canonical prefixed form", got)
	}
}

// 2. Missing installed path for a no-entrypoint package fails the build. This
// is the exact unfulfillable-promise shape.
func TestDeclaredIdentity_NoEntrypointWithoutInstalledPathFails(t *testing.T) {
	err := validateDeclaredIdentity(&PackageIdentity{
		Proof:    ProofBinarySHA256,
		Checksum: testValidSHA,
	}, true)
	if err == nil {
		t.Fatal("entrypoint:none + binary_sha256 with no installed_path must fail the build")
	}
	if !strings.Contains(err.Error(), "installed_path") {
		t.Errorf("error should name the missing field: %v", err)
	}
}

// 3. Relative paths fail: the node-agent has no meaningful cwd to resolve against.
func TestDeclaredIdentity_RelativeInstalledPathFails(t *testing.T) {
	for _, p := range []string{"usr/lib/x.so", "./x.so", "../../etc/passwd", "bin/wrapper"} {
		t.Run(p, func(t *testing.T) {
			err := validateDeclaredIdentity(&PackageIdentity{
				Proof: ProofBinarySHA256, Checksum: testValidSHA, InstalledPath: p,
			}, true)
			if err == nil {
				t.Fatalf("relative/traversing path %q must fail the build", p)
			}
		})
	}
}

// Traversal is rejected BEFORE cleaning: path.Clean would turn "/usr/../etc/x"
// into a different, plausible-looking path, converting an ambiguous declaration
// into a confident wrong subject.
func TestDeclaredIdentity_TraversalIsRejectedNotNormalized(t *testing.T) {
	if got := normalizeIdentityInstalledPath("/usr/../etc/shadow"); got != "" {
		t.Fatalf("traversal silently normalized to %q instead of being rejected", got)
	}
}

// A directory is not a hashable subject.
func TestDeclaredIdentity_DirectoryPathRejected(t *testing.T) {
	for _, p := range []string{"/", "/usr/lib/"} {
		if got := normalizeIdentityInstalledPath(p); got != "" {
			t.Errorf("directory %q accepted as %q", p, got)
		}
	}
}

// 4./5. Checksum must be present and well-formed.
func TestDeclaredIdentity_ChecksumRequiredAndValidated(t *testing.T) {
	cases := []struct{ name, checksum, want string }{
		{"missing", "", "requires identity.checksum"},
		{"too short", "abc123", "not a valid sha256"},
		{"non-hex", strings.Repeat("z", 64), "not a valid sha256"},
		{"has prefix, valid", "sha256:" + testValidSHA, ""},
		{"no prefix, valid", testValidSHA, ""},
		{"uppercase, valid", strings.ToUpper(testValidSHA), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDeclaredIdentity(&PackageIdentity{
				Proof: ProofBinarySHA256, Checksum: tc.checksum, InstalledPath: testAbsPath,
			}, true)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("valid checksum %q rejected: %v", tc.checksum, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("checksum %q must fail the build", tc.checksum)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %v does not mention %q", err, tc.want)
			}
		})
	}
}

// 6. An unknown proof mode fails closed at build time — it must never reach an
// artifact where the node-agent would have to guess what was meant.
func TestDeclaredIdentity_UnknownProofModeFails(t *testing.T) {
	for _, mode := range []string{"sha512", "gpg", "trust_me", "binary"} {
		t.Run(mode, func(t *testing.T) {
			err := validateDeclaredIdentity(&PackageIdentity{
				Proof: mode, Checksum: testValidSHA, InstalledPath: testAbsPath,
			}, true)
			if err == nil {
				t.Fatalf("unknown proof mode %q must fail the build", mode)
			}
			if !strings.Contains(err.Error(), "not a supported mode") {
				t.Errorf("error should say the mode is unsupported: %v", err)
			}
		})
	}
}

// An identity block with no proof mode at all is malformed, not "no identity".
func TestDeclaredIdentity_EmptyProofInBlockFails(t *testing.T) {
	if err := validateDeclaredIdentity(&PackageIdentity{InstalledPath: testAbsPath}, true); err == nil {
		t.Fatal("identity block with no proof mode must fail the build")
	}
}

// version proof carries no binary subject; it must not require one.
func TestDeclaredIdentity_VersionProofNeedsNoBinarySubject(t *testing.T) {
	if err := validateDeclaredIdentity(&PackageIdentity{Proof: ProofVersion}, true); err != nil {
		t.Fatalf("version proof rejected: %v", err)
	}
}

// 7. The honest no-proof case stays legal — this is libnss-resolve.
func TestDeclaredIdentity_NoIdentityBlockRemainsLegal(t *testing.T) {
	if err := validateDeclaredIdentity(nil, true); err != nil {
		t.Fatalf("entrypoint:none with no identity block must remain legal: %v", err)
	}
}

// 8. An entrypoint-bearing package needs no explicit installed path; the
// archive layout supplies the subject.
func TestDeclaredIdentity_EntrypointBearingNeedsNoInstalledPath(t *testing.T) {
	if err := validateDeclaredIdentity(&PackageIdentity{
		Proof: ProofBinarySHA256, Checksum: testValidSHA,
	}, false); err != nil {
		t.Fatalf("entrypoint-bearing package should not require installed_path: %v", err)
	}
	// …but a declared one must still be well-formed rather than ignored.
	if err := validateDeclaredIdentity(&PackageIdentity{
		Proof: ProofBinarySHA256, Checksum: testValidSHA, InstalledPath: "relative/x",
	}, false); err == nil {
		t.Error("a malformed installed_path must fail even when optional")
	}
}

// 9. Negative control: the manifest field must actually be wired. If
// IdentityInstalledPath stopped being serialized, this fails.
func TestDeclaredIdentity_ManifestCarriesInstalledPathField(t *testing.T) {
	m := Manifest{
		Name:                  "example",
		Entrypoint:            "none",
		IdentityProof:         ProofBinarySHA256,
		IdentityInstalledPath: testAbsPath,
		EntrypointChecksum:    "sha256:" + testValidSHA,
	}
	blob := mustMarshalManifest(t, m)
	for _, want := range []string{
		`"identity_installed_path":"` + testAbsPath + `"`,
		`"identity_proof":"binary_sha256"`,
	} {
		if !strings.Contains(blob, want) {
			t.Errorf("manifest JSON missing %s\ngot: %s", want, blob)
		}
	}
	// The honest no-identity package must emit neither field (omitempty).
	bare := mustMarshalManifest(t, Manifest{Name: "libnss-resolve", Entrypoint: "none"})
	for _, absent := range []string{"identity_installed_path", "identity_proof", "entrypoint_checksum"} {
		if strings.Contains(bare, absent) {
			t.Errorf("no-identity manifest should omit %s\ngot: %s", absent, bare)
		}
	}
}

func mustMarshalManifest(t *testing.T, m Manifest) string {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	return string(b)
}
