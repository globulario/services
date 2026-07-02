package main

// release_artifact_shape.go — artifact-shape law for the release channel.
//
// Enforces invariant publish.release_artifact_must_be_stripped (the strip half):
// a binary published to the release (STABLE) channel must be built stripped
// (`-trimpath -ldflags "-s -w"`). An unstripped debug build — the ~2x 15-18MB
// local artifact — must be rejected at publish, before promotion to PUBLISHED.
//
// Scope (CG-3 slice, by decision 2026-06-25): ELF only (the cluster platform).
// Non-ELF artifacts (darwin/windows) and archives without a detectable
// entrypoint binary pass through unchecked — they are out of scope for this
// gate, not implicitly trusted. The size-envelope half compares the archive
// size against the latest published release on the same platform.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"debug/elf"
	"fmt"
	"io"
	"path"
	"strings"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

const releaseArtifactMaxGrowthFactor int64 = 2

// extractEntrypointBinary returns the bytes of the entrypoint executable inside
// a .tgz package payload (the executable under bin/), mirroring the discovery
// rule used by computeBinaryChecksumFromArchive. ok=false means no such binary
// was found or the archive is not a readable tgz.
func extractEntrypointBinary(data []byte) (binary []byte, ok bool) {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, false
	}
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			return nil, false
		}
		name := path.Clean(hdr.Name)
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if !strings.HasPrefix(name, "bin/") && !strings.Contains(name, "/bin/") {
			continue
		}
		if hdr.Mode&0111 == 0 {
			continue
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return nil, false
		}
		return b, true
	}
}

// elfDebugSection reports the name of the first symbol-table / DWARF section
// found in an ELF binary, or "" if the binary is stripped. A Go binary built
// with `-ldflags "-s -w"` carries neither a symbol table (.symtab) nor DWARF
// (.debug_*/.zdebug_*) sections; an unstripped debug build carries both.
//
// It operates on parsed section names so the policy is unit-testable without
// crafting ELF fixtures.
func elfDebugSectionName(sectionNames []string) string {
	for _, n := range sectionNames {
		if n == ".symtab" {
			return n
		}
		if strings.HasPrefix(n, ".debug_") || strings.HasPrefix(n, ".zdebug_") {
			return n
		}
	}
	return ""
}

// validateReleaseArtifactStripped rejects a release-channel upload whose
// entrypoint ELF binary is unstripped. Returns nil (pass) when there is no
// detectable binary, the binary is not ELF, or the ELF is stripped — those are
// out of scope for this gate, handled by other laws, or compliant.
//
// The returned error is suitable to wrap in codes.FailedPrecondition.
func validateReleaseArtifactStripped(data []byte) error {
	bin, ok := extractEntrypointBinary(data)
	if !ok {
		return nil
	}
	f, err := elf.NewFile(bytes.NewReader(bin))
	if err != nil {
		// Not an ELF (darwin/windows or not a binary) — out of scope here.
		return nil
	}
	defer func() { _ = f.Close() }()

	names := make([]string, 0, len(f.Sections))
	for _, s := range f.Sections {
		names = append(names, s.Name)
	}
	if dbg := elfDebugSectionName(names); dbg != "" {
		return fmt.Errorf(
			"release artifact carries debug section %q — release-channel builds must be stripped (build with -trimpath -ldflags \"-s -w\")",
			dbg,
		)
	}
	return nil
}

// latestPublishedReleaseSize returns the latest published release size for the
// same package/platform from the release ledger. ok=false means there is no
// usable prior release size, so the size-envelope gate must skip.
func latestPublishedReleaseSize(ctx context.Context, srv *server, manifest *repopb.ArtifactManifest) (size int64, ok bool) {
	if srv == nil || manifest == nil || manifest.GetRef() == nil {
		return 0, false
	}
	ledger := srv.readLedger(ctx, manifest.GetRef().GetPublisherId(), manifest.GetRef().GetName())
	if ledger == nil {
		return 0, false
	}
	platform := strings.TrimSpace(manifest.GetRef().GetPlatform())
	for i := len(ledger.Releases) - 1; i >= 0; i-- {
		rel := ledger.Releases[i]
		if rel == nil || rel.SizeBytes <= 0 {
			continue
		}
		if strings.TrimSpace(rel.Platform) != platform {
			continue
		}
		return rel.SizeBytes, true
	}
	return 0, false
}

// validateReleaseArtifactSizeEnvelope rejects a release-channel upload whose
// archive size exceeds 2x the latest published release size for the same
// package/platform. This is a coarse ratchet for the debug/unstripped class
// without inventing a fuzzy heuristic. First publish, missing history, or zero
// prior size skip the check.
func validateReleaseArtifactSizeEnvelope(ctx context.Context, srv *server, manifest *repopb.ArtifactManifest) error {
	prior, ok := latestPublishedReleaseSize(ctx, srv, manifest)
	if !ok {
		return nil
	}
	current := manifest.GetSizeBytes()
	if current <= 0 {
		return nil
	}
	if current <= prior*releaseArtifactMaxGrowthFactor {
		return nil
	}
	return fmt.Errorf(
		"release artifact size %d exceeds the %dx prior-release envelope (prior=%d)",
		current, releaseArtifactMaxGrowthFactor, prior,
	)
}
