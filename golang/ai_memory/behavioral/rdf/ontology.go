// Package rdf is the behavioral-memory RDF/Ontology semantic PROJECTION layer.
//
// ScyllaDB remains the operational source of truth; this package derives a
// read-only RDF view (N-Triples) for semantic inspection, AWG alignment,
// explanation, and future linked-data integration. If the projection disagrees
// with Scylla, the projection is stale — the repair model is "rebuild from
// Scylla", never graph-to-graph sync.
//
// It is NOT on the runtime path: CheckAction / ResolveGovernedContext keep using
// the Scylla lookup tables and never touch RDF/SPARQL. This package reads through
// the Reader interface (a projection-specific reader), imports only behavioral/api
// + stdlib, and contains NO database driver — so the generic-kernel hygiene rule
// holds. The Scylla-backed Reader lives outside behavioral/.
//
// URIs reuse the canonical Scylla ids (api.CanonicalURI) — there is no separate
// RDF-only identity.
package rdf

import (
	"strings"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
)

// Namespaces.
const (
	// bmNS is the behavioral-memory ontology namespace (classes + predicates).
	bmNS = "https://globular.io/behavioral#"
	// instanceBase is the base for instance resources; the path segment reuses
	// the canonical id scheme (behavioral:<kind>/<id>).
	instanceBase = "https://globular.io/behavioral/instance/"
	// awgNS is the awareness-graph namespace, reused for overlapping classes so
	// the behavioral graph aligns with AWG rather than inventing rival terms.
	awgNS = "https://globular.io/awareness#"

	rdfType   = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
	rdfsLabel = "http://www.w3.org/2000/01/rdf-schema#label"
)

// bm classes.
const (
	ClassSignal                = "Signal"
	ClassClaim                 = "Claim"
	ClassEvidence              = "Evidence"
	ClassAuthority             = "Authority"
	ClassCondition             = "Condition"
	ClassContradiction         = "Contradiction"
	ClassPrinciple             = "Principle"
	ClassForbiddenMove         = "ForbiddenMove"
	ClassRequiredEvidence      = "RequiredEvidence"
	ClassPromotionDecision     = "PromotionDecision"
	ClassRevocationRule        = "RevocationRule"
	ClassActionCheck           = "ActionCheck"
	ClassOutcome               = "Outcome"
	ClassOperationalKnowledge  = "OperationalKnowledgeSource"
	ClassGeneratedPrinciple    = "GeneratedPrinciple"
	ClassBackfilledMemory      = "BackfilledMemory"
)

// awg-compatible classes (additional rdf:type for overlapping concepts).
const (
	AWGRuntimeEvidence    = "RuntimeEvidence"
	AWGOutcomeFeedback    = "OutcomeFeedback"
	AWGForbiddenRepairMove = "ForbiddenRepairMove"
	AWGPromotionDecision  = "PromotionDecision"
	AWGRequiredEvidence   = "RequiredEvidence"
)

// bm predicates (the §6 required set + a few literal-valued attributes).
const (
	PredProducesClaim       = "producesClaim"
	PredDerivedFromSignal   = "derivedFromSignal"
	PredSupportedBy         = "supportedBy"
	PredSupportsTarget      = "supportsTarget"
	PredObservedFrom        = "observedFrom"
	PredSatisfies           = "satisfies"
	PredGovernedBy          = "governedBy"
	PredGoverns             = "governs"
	PredAppliesWhen         = "appliesWhen"
	PredRequiresEvidence    = "requiresEvidence"
	PredForbidsMove         = "forbidsMove"
	PredContradictedBy      = "contradictedBy"
	PredPromotedBy          = "promotedBy"
	PredRevokedBy           = "revokedBy"
	PredSupersededBy        = "supersededBy"
	PredNarrowedBy          = "narrowedBy"
	PredCheckedAgainst      = "checkedAgainst"
	PredBlockedBy           = "blockedBy"
	PredMissingEvidence     = "missingEvidence"
	PredResultedFrom        = "resultedFrom"
	PredSupportsPrinciple   = "supportsPrinciple"
	PredWeakensPrinciple    = "weakensPrinciple"
	PredGroupedByTheme      = "groupedByTheme"
	PredGeneratedFrom       = "generatedFrom"
	PredSourceRef           = "sourceRef"
	PredBackfilledFromMemory = "backfilledFromMemory"
	PredDecides             = "decides"
	PredRevokes             = "revokes"

	// literal-valued attributes.
	PredGovernanceStatus = "governanceStatus"
	PredSignalKind       = "signalKind"
	PredEvidenceLane     = "evidenceLane"
	PredRiskLevel        = "riskLevel"
	PredSubjectEntity    = "subjectEntity"
	PredPredicate        = "predicate"
	PredObjectValue      = "objectValue"
	PredResolution       = "resolution"
	PredVerdict          = "verdict"
	PredCheckStatus      = "checkStatus"
	PredOutcomeStatus    = "outcomeStatus"
)

// iri wraps a full IRI in angle brackets for N-Triples.
func iri(full string) string { return "<" + full + ">" }

func classIRI(name string) string  { return iri(bmNS + name) }
func awgIRI(name string) string     { return iri(awgNS + name) }
func predIRI(name string) string    { return iri(bmNS + name) }

// escapeIRIPath percent-encodes characters not allowed in an N-Triples IRIREF.
func escapeIRIPath(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r <= 0x20, r == '<', r == '>', r == '"', r == '{', r == '}', r == '|', r == '^', r == '`', r == '\\', r == '%':
			b.WriteString("%")
			const hex = "0123456789ABCDEF"
			b.WriteByte(hex[(r>>4)&0xF])
			b.WriteByte(hex[r&0xF])
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// instanceIRI builds the full instance IRI for an entity kind + id, reusing the
// canonical id scheme so the Scylla id is the semantic identity.
func instanceIRI(kind api.EntityKind, id string) string {
	return iri(instanceBase + string(kind) + "/" + escapeIRIPath(id))
}

// literal escapes a string into an N-Triples quoted literal.
func literal(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
