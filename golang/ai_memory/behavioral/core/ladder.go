package core

import (
	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/google/uuid"
)

// newID returns a fresh canonical entity id. crypto-random UUIDv4 keeps the
// kernel free of the gocql dependency (TimeUUID lives in the driver) while still
// producing a stable id that becomes the entity's RDF identity.
func newID() string { return uuid.NewString() }

// pr2AllowedStatuses are the only governance-ladder rungs PR-2 may write. The
// promotion/revocation rungs are introduced by later PRs and are rejected here so
// the ingestion half cannot accidentally fabricate a promoted principle.
var pr2AllowedStatuses = map[api.GovernanceStatus]bool{
	api.StatusUnspecified:         true,
	api.StatusRawSignal:           true,
	api.StatusExtractedClaim:      true,
	api.StatusCandidateFact:       true,
	api.StatusEvidenceLinked:      true,
	api.StatusAuthorityMapped:     true,
	api.StatusConditionScoped:     true,
	api.StatusContradictionTested: true,
}

// statusAllowedInPR2 reports whether a caller-supplied status is permitted.
func statusAllowedInPR2(s api.GovernanceStatus) bool { return pr2AllowedStatuses[s] }
