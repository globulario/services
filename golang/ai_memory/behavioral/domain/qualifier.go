package domain

// qualifier.go defines how a domain pack answers the one question CheckAction
// cannot answer generically: does the stored evidence for this requirement
// actually COUNT, here, now, for this action?
//
// WHY THIS IS A DOMAIN CONCERN
//
// Whether evidence qualifies depends on which authority produced it, which
// cluster it belongs to, which subject it is about, how recent it must be, and
// which governed action it is bound to. Every one of those is cluster semantics.
// Encoding them in the kernel would hardcode one domain's rules into machinery
// meant to serve others — and the kernel already forbids importing a pack.
//
// So the kernel asks and the pack answers. This interface is the question.
//
// WHY IT IS OPTIONAL
//
// A pack that does not implement it keeps the kernel's original evidence lane
// (a recorded row targeting the principle). That lane is weaker — it has no
// cluster scope, no authority floor, no freshness bound and no action binding —
// so a pack that governs real actions SHOULD implement this. Making it optional
// keeps existing packs working rather than failing them closed at startup for a
// capability they never claimed.

import (
	"context"

	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

// RequirementQuery is the kernel's question about one required-evidence ref,
// scoped to the action being judged.
//
// The scope fields come from the caller's action context, not from the kernel's
// imagination: a gate that invents a subject would qualify evidence about
// something else.
type RequirementQuery struct {
	Project       string
	Domain        string
	RequirementID string

	// Subject and action scope. A pack may require any subset; an empty field
	// means the caller did not supply it, which a strict rule must treat as a
	// failure to qualify rather than as a wildcard.
	ClusterID    string
	EntityRef    string
	ConditionRef string
	SourceRef    string
	ActionRef    string

	// ActionDispatchedAt bounds "after the action" for post-action evidence.
	// Zero when the action has not been dispatched — which is the normal case
	// for a PRE-action gate.
	ActionDispatchedAt int64
	// EvaluatedAt is the injected clock. The kernel supplies it so a verdict is
	// reproducible for a given state and time.
	EvaluatedAt int64
}

// RequirementVerdict is the pack's answer.
//
// Satisfied and Reason are kept separate so a gate can distinguish "no evidence
// exists" from "evidence exists and does not count" — an operator sent to gather
// evidence that is already there, or told to wait for evidence that could never
// qualify, has been given the wrong instruction.
type RequirementVerdict struct {
	Satisfied bool
	// EvidenceIDs are the rows that qualified. Carried so the resulting
	// ActionCheck can name what it rested on rather than asserting a bare yes.
	EvidenceIDs []string
	// Reason is a stable, machine-readable rejection code when Satisfied is
	// false and candidates existed. Empty when nothing was found at all.
	Reason string
	// Detail is the comparison that failed — enough to explain, not enough to
	// leak the evidence payload.
	Detail string
}

// ReasonRequirementNotDeclared is the one reason code the KERNEL must
// understand, because it changes the kernel's own behaviour: it means the domain
// expresses no opinion about this requirement.
//
// Every other reason is opaque to the kernel and merely reported. This one is
// not, because it draws the line for caller self-assertion: a requirement the
// domain RULED on is closed to assertion — otherwise a caller could bypass every
// discriminator a rule exists to apply by claiming to hold the evidence — while
// a requirement no rule describes stays open to it.
//
// A pack must return exactly this string for that case, or the kernel will treat
// its silence as a judgement.
const ReasonRequirementNotDeclared = "requirement_not_declared"

// EvidenceQualifier is implemented by a domain pack that owns explicit
// satisfaction rules.
//
// FAILING CLOSED IS THE IMPLEMENTOR'S DUTY TOO. An error return must mean "I
// could not determine this", and the kernel will treat it as unsatisfied. An
// implementation must never return Satisfied:true alongside an error, and must
// never swallow a store failure into a false-but-quiet verdict — a governor that
// cannot read its evidence has not learned that the evidence is absent.
type EvidenceQualifier interface {
	QualifyRequirement(ctx context.Context, s store.Store, q RequirementQuery) (RequirementVerdict, error)
}
