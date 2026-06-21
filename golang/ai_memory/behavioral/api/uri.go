package api

// Canonical-ID / URI-readiness (RDF design constraint).
//
// ScyllaDB remains the operational source of record. RDF/Ontology is a deferred
// semantic *projection* layer (see plan §18, PR-7) — it is NOT implemented here.
// What IS guaranteed from PR-1 onward: every first-class entity has a stable
// canonical id, and that same id becomes its RDF URI. There is no separate
// RDF-only identity minted later.
//
// This file defines only the ID→URI naming scheme (pure strings, no RDF deps),
// so the eventual projection is mechanical. The hot runtime path never calls it.

// EntityKind is the URI path segment for each first-class behavioral entity.
// These map 1:1 to the future bm:* ontology classes (bm:Signal, bm:Claim, …).
type EntityKind string

const (
	KindSignal               EntityKind = "signal"
	KindClaim                EntityKind = "claim"
	KindEvidence             EntityKind = "evidence"
	KindAuthority            EntityKind = "authority"
	KindCondition            EntityKind = "condition"
	KindContradiction        EntityKind = "contradiction"
	KindPrinciple            EntityKind = "principle"
	KindForbiddenMove        EntityKind = "forbidden_move"
	KindRequiredEvidence     EntityKind = "required_evidence"
	KindOutcome              EntityKind = "outcome"
	KindPromotionCandidate   EntityKind = "promotion_candidate"
	KindReconciliationReport EntityKind = "reconciliation_report"
	KindPromotionDecision    EntityKind = "promotion_decision"
	KindRevocationRule       EntityKind = "revocation_rule"
	KindActionCheck          EntityKind = "action_check"
)

// AllEntityKinds is the closed set of first-class entity kinds. The RDF-readiness
// test iterates this to assert every kind has a stable-ID-bearing Go type.
var AllEntityKinds = []EntityKind{
	KindSignal, KindClaim, KindEvidence, KindAuthority, KindCondition,
	KindContradiction, KindPrinciple, KindForbiddenMove, KindRequiredEvidence,
	KindOutcome, KindPromotionCandidate, KindReconciliationReport, KindPromotionDecision, KindRevocationRule, KindActionCheck,
}

// URIPrefix is the CURIE prefix for behavioral-memory canonical URIs. The
// projection (PR-7) binds it to a full namespace; until then it is a stable
// prefix string only.
const URIPrefix = "behavioral"

// CanonicalURI returns the stable URI for an entity id, e.g.
// CanonicalURI(KindPrinciple, "abc") == "behavioral:principle/abc".
// An empty id yields an empty string (no synthetic identity is invented).
func CanonicalURI(kind EntityKind, id string) string {
	if id == "" {
		return ""
	}
	return URIPrefix + ":" + string(kind) + "/" + id
}
