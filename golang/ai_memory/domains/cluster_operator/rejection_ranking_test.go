package cluster_operator

import "testing"

// TestRejectionRank_NearMissOutranksWrongSubject guards the diagnosability
// defect found on 2026-08-01.
//
// Only ONE rejection reaches the operator. It used to be chosen by sorting on
// evidence id — a UUID — so which reason surfaced was arbitrary. A refused
// remediation reported
//
//	subject_mismatch: entity: want ".../globular-torrent.service"
//	                   got  ".../sha256sum"
//
// while qualifying-shaped evidence for the right unit sat in the same partition.
// The message described an unrelated row, and the real reason was never shown.
//
// A subject mismatch means "this evidence is about something else" — true of
// most rows in a shared partition, and useless for diagnosing one action. A
// rejection that got past the subject and failed on authority, freshness,
// result or action-binding is a near miss, and that is what an operator needs.
func TestRejectionRank_NearMissOutranksWrongSubject(t *testing.T) {
	nearMisses := []RejectionReason{
		RejectAuthorityInsufficient,
		RejectAuthorityUnknown,
		RejectResultNotAccepted,
		RejectEvidenceStale,
		RejectTimestampMissing,
		RejectTimestampInFuture,
		RejectNotBoundToAction,
	}
	wrongSubject := []RejectionReason{
		RejectSubjectMismatch,
		RejectClusterMismatch,
	}

	for _, near := range nearMisses {
		for _, wrong := range wrongSubject {
			if rejectionRank(near) >= rejectionRank(wrong) {
				t.Errorf("%q must outrank %q: a rejection about the RIGHT subject explains the\n"+
					"refusal, while a wrong-subject row only says other evidence exists", near, wrong)
			}
		}
	}
}

// TestRejectionRank_IsStableForUnknownReasons verifies a reason nobody ranked
// still counts as a near miss.
//
// The safe default is to treat an unclassified rejection as informative: a new
// discriminator added later will surface rather than be silently outranked by
// wrong-subject noise, which is the failure this ranking exists to prevent.
func TestRejectionRank_IsStableForUnknownReasons(t *testing.T) {
	if rejectionRank(RejectionReason("some_future_reason")) != 0 {
		t.Error("an unranked rejection reason must default to the informative bucket,\n" +
			"or adding a discriminator silently makes refusals harder to diagnose")
	}
}
