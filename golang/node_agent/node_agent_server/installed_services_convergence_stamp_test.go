package main

import (
	"testing"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// The convergence stamp and the entrypoint checksum are two different hash
// schemas over two different subjects: desired_hash is computed by the
// controller from the release schema, entrypoint_checksum is the sha256 of the
// installed binary computed by the apply path. install_package.hash_schemas_must_not_alias
// forbids one becoming the other.
//
// The stamp used to "preserve" existing.Checksum into entrypoint_checksum when
// that metadata key was empty. On a re-apply existing.Checksum is already the
// convergence hash from the previous stamp, so the preserve step published a
// convergence hash under a binary-domain key. cluster-doctor compared it to the
// manifest digest and raised artifact.installed_digest_mismatch (ERROR) against
// a node whose bytes were fine — INFRASTRUCTURE/scylladb on node-2, 2026-08-11.
func TestApplyInfraConvergenceStamp_DoesNotAuthorEntrypointChecksum(t *testing.T) {
	const convergenceHash = "fc2998b7617c276319c7f7c07f26a9ae4c09b7ff7da3eccb0cca038ff268aedc"

	// Second stamp of the same package: Checksum already holds the convergence
	// hash from the first stamp, and the package declared no binary to hash so
	// the apply path never wrote entrypoint_checksum.
	pkg := &node_agentpb.InstalledPackage{
		NodeId:   "node-2",
		Name:     "scylladb",
		Kind:     "INFRASTRUCTURE",
		Checksum: convergenceHash,
	}

	applyInfraConvergenceStamp(pkg, convergenceHash)

	if got, ok := pkg.GetMetadata()["entrypoint_checksum"]; ok {
		t.Fatalf("stamp authored entrypoint_checksum = %q; the convergence writer must never write the binary-domain key", got)
	}
	if pkg.GetChecksum() != convergenceHash {
		t.Fatalf("Checksum = %q, want the convergence hash %q", pkg.GetChecksum(), convergenceHash)
	}
}

// The binary hash the apply path recorded is the canonical one and must survive
// the stamp untouched (identity.has_single_canonical_source_and_is_immutable).
func TestApplyInfraConvergenceStamp_PreservesBinaryEntrypointChecksum(t *testing.T) {
	const (
		binarySHA       = "02e6662741f43000ee356bab8e40236a496714a9651d5b8537a9abdb614d7d73"
		convergenceHash = "fc2998b7617c276319c7f7c07f26a9ae4c09b7ff7da3eccb0cca038ff268aedc"
	)

	pkg := &node_agentpb.InstalledPackage{
		NodeId:   "node-2",
		Name:     "etcd",
		Kind:     "INFRASTRUCTURE",
		Checksum: binarySHA,
		Metadata: map[string]string{"entrypoint_checksum": binarySHA},
	}

	applyInfraConvergenceStamp(pkg, convergenceHash)

	if got := pkg.GetMetadata()["entrypoint_checksum"]; got != binarySHA {
		t.Fatalf("entrypoint_checksum = %q, want the untouched binary hash %q", got, binarySHA)
	}
	if pkg.GetChecksum() != convergenceHash {
		t.Fatalf("Checksum = %q, want the convergence hash %q", pkg.GetChecksum(), convergenceHash)
	}
}

// An empty convergence hash is not a value — it must not clear the record.
func TestApplyInfraConvergenceStamp_EmptyHashIsNoOp(t *testing.T) {
	const binarySHA = "02e6662741f43000ee356bab8e40236a496714a9651d5b8537a9abdb614d7d73"

	pkg := &node_agentpb.InstalledPackage{
		NodeId:   "node-2",
		Name:     "etcd",
		Kind:     "INFRASTRUCTURE",
		Checksum: binarySHA,
	}

	applyInfraConvergenceStamp(pkg, "")

	if pkg.GetChecksum() != binarySHA {
		t.Fatalf("Checksum = %q, want it left at %q", pkg.GetChecksum(), binarySHA)
	}
}
