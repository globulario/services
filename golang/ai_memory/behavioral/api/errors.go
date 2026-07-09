package api

import (
	"fmt"
	"strings"
)

// errors.go — the governance error taxonomy (Priority 8) and the structured
// GovernanceError the kernel returns for validation/gate refusals.
//
// LEGIBILITY PRINCIPLE (docs/design/governance-tools-legibility.md):
// "Keep the correctness tax. Remove the blindfold." A refusal must carry the
// COMPLETE contract for becoming valid in ONE response — the error code, the
// offending values, and the expected format — so a capable agent never has to
// discover the contract by repeated rejection.
//
// This type is deliberately transport-agnostic: it imports no grpc/codes. The
// handler layer (behavioral_handlers.go) maps ErrorCode → gRPC status code, which
// keeps the kernel cleanly extractable.

// ErrorCode is the stable, machine-readable classification of a governance
// refusal. Codes are contract identities, not English — the human message may
// change, the code must not.
type ErrorCode string

const (
	// Validation (input shape) — map to INVALID_ARGUMENT.
	CodeMissingRequiredFields  ErrorCode = "MISSING_REQUIRED_FIELDS"
	CodeUnknownField           ErrorCode = "UNKNOWN_FIELD"
	CodeInvalidFieldType       ErrorCode = "INVALID_FIELD_TYPE"
	CodeInvalidEnumValue       ErrorCode = "INVALID_ENUM_VALUE"
	CodeInvalidReferenceFormat ErrorCode = "INVALID_REFERENCE_FORMAT"

	// Resolution — map to NOT_FOUND.
	CodeReferenceNotFound ErrorCode = "REFERENCE_NOT_FOUND"

	// Promotion-gate contract (state, not shape) — map to FAILED_PRECONDITION.
	CodeAuthorityNotMapped           ErrorCode = "AUTHORITY_NOT_MAPPED"
	CodeEvidenceNotObservable        ErrorCode = "EVIDENCE_NOT_OBSERVABLE"
	CodeEvidencePostHoc              ErrorCode = "EVIDENCE_POST_HOC"
	CodeEvidenceStale                ErrorCode = "EVIDENCE_STALE"
	CodeContradictionDetected        ErrorCode = "CONTRADICTION_DETECTED"
	CodeRequiredTestsMissing         ErrorCode = "REQUIRED_TESTS_MISSING"
	CodeApproverRequired             ErrorCode = "APPROVER_REQUIRED"
	CodePromotionContractUnsatisfied ErrorCode = "PROMOTION_CONTRACT_UNSATISFIED"

	// Safety — map to PERMISSION_DENIED.
	CodeUnsafeOperationRefused ErrorCode = "UNSAFE_OPERATION_REFUSED"
)

// FieldOffense is one field-level validation failure: which field, the offending
// value, and why it was rejected. Collected so a whole batch of problems is
// reported at once rather than one-per-rejection.
type FieldOffense struct {
	Field          string
	OffendingValue string
	Reason         string
}

// GovernanceError is a structured, self-describing refusal. Its Error() renders
// the complete contract in a single line so the message survives the gRPC status
// boundary even before the proto-level structured detail lands (a later PR).
type GovernanceError struct {
	Code      ErrorCode
	Message   string
	Offenders []FieldOffense
	Expected  string
}

func (e *GovernanceError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s]", e.Code)
	if e.Message != "" {
		b.WriteString(" ")
		b.WriteString(e.Message)
	}
	if len(e.Offenders) > 0 {
		fmt.Fprintf(&b, " %d offending value(s):", len(e.Offenders))
		for _, o := range e.Offenders {
			fmt.Fprintf(&b, " %s=%q (%s);", o.Field, o.OffendingValue, o.Reason)
		}
	}
	if e.Expected != "" {
		b.WriteString(" expected: ")
		b.WriteString(e.Expected)
	}
	return b.String()
}

// refForbiddenChars are the characters whose presence in a catalog reference is
// the fingerprint of comma-split prose (e.g. "incident(foo", " bar)") rather than
// a canonical id. The check is purely SYNTACTIC — the kernel still never
// interprets a ref's meaning (that is the domain registry's job).
const refForbiddenChars = " \t\r\n,()"

// IsWellFormedRef reports whether s is syntactically usable as a catalog
// reference: non-empty and free of whitespace, commas, and parentheses.
func IsWellFormedRef(s string) bool {
	return s != "" && !strings.ContainsAny(s, refForbiddenChars)
}

// RefFormatReason explains, in one short phrase, why s is not a well-formed ref.
func RefFormatReason(s string) string {
	if s == "" {
		return "empty reference"
	}
	switch {
	case strings.ContainsAny(s, " \t\r\n"):
		return "contains whitespace"
	case strings.Contains(s, ","):
		return "contains ',' (looks like comma-split prose, not a single ref)"
	case strings.ContainsAny(s, "()"):
		return "contains parentheses"
	default:
		return "malformed reference"
	}
}

// NewInvalidReferenceFormatError builds the standard INVALID_REFERENCE_FORMAT
// error for a batch of offending references, with the canonical expected format.
func NewInvalidReferenceFormatError(offenders []FieldOffense) *GovernanceError {
	return &GovernanceError{
		Code:      CodeInvalidReferenceFormat,
		Message:   "one or more references are malformed (a comma inside prose becomes a split, mangled ref)",
		Offenders: offenders,
		Expected:  `each reference must be a single canonical catalog id with no spaces, commas, or parentheses, e.g. "authority.cluster.ai_executor.runtime_state"; pass multiple refs as separate list elements, not one comma-joined string`,
	}
}
