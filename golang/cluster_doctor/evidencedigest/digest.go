// Package evidencedigest produces the canonical sha256 digest of a
// finding's evidence collection. The digest is the "generation" field
// the approval-token contract binds against — so server-side audit and
// operator-side minting MUST produce identical bytes for the same input.
//
// Extracted from cluster_doctor_server/handler_remediation.go so both the
// server binary and the globularcli mint-approval command import the
// same implementation. See docs/intent/remediation.token_contract.yaml.
// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor
// @awareness file_role=evidence_digest_fingerprinting
// @awareness implements=globular.platform:intent.evidence.provenance_trust_levels
// @awareness risk=high
package evidencedigest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// Of returns the canonical "sha256:<hex>" digest of evidence. Empty
// input returns "" so callers can pass a nil slice without crashing.
// The digest is stable: order of evidence entries, key/value ordering
// within each entry, and timestamp seconds-precision all normalize.
func Of(evidence []*cluster_doctorpb.Evidence) string {
	if len(evidence) == 0 {
		return ""
	}
	parts := make([]string, 0, len(evidence))
	for _, ev := range evidence {
		if ev == nil {
			continue
		}
		kvPairs := make([]string, 0, len(ev.GetKeyValues()))
		for k, v := range ev.GetKeyValues() {
			kvPairs = append(kvPairs, k+"="+v)
		}
		sort.Strings(kvPairs)
		timestamp := ""
		if ev.GetTimestamp() != nil {
			timestamp = fmt.Sprintf("%d", ev.GetTimestamp().GetSeconds())
		}
		parts = append(parts,
			ev.GetSourceService()+"|"+ev.GetSourceRpc()+"|"+timestamp+"|"+strings.Join(kvPairs, ","),
		)
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return "sha256:" + hex.EncodeToString(sum[:])
}
