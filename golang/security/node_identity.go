package security

import (
	"fmt"
	"os"
	"strings"
)

// Node identity enforcement flags (Gap 2+3).
//
// DEPRECATE_SA_NODE_AUTH=true  → warn when sa principal calls node-agent paths
// REQUIRE_NODE_IDENTITY=true  → reject unless principal matches node_<uuid>

// DeprecateSANodeAuth returns true if the system should warn when the legacy
// "sa" service account is used for node-agent-specific cluster paths.
func DeprecateSANodeAuth() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("DEPRECATE_SA_NODE_AUTH")), "true")
}

// RequireNodeIdentity returns true if the system must reject requests where
// the caller principal does not match node_<uuid> on node-agent paths.
func RequireNodeIdentity() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("REQUIRE_NODE_IDENTITY")), "true")
}

// IsNodePrincipal returns true if the subject follows the node_<uuid> pattern.
func IsNodePrincipal(subject string) bool {
	return strings.HasPrefix(subject, "node_") && len(subject) > 5
}

// ExtractNodeID returns the node UUID portion from a node_<uuid> principal.
// Returns "" if the subject is not a node principal.
func ExtractNodeID(subject string) string {
	if !IsNodePrincipal(subject) {
		return ""
	}
	return subject[5:]
}

// ValidateNodeOwnership checks that the caller principal matches the
// node_id in the request. This enforces own-node-only scope.
//
// Rules:
//   - node_<uuid> principals can only operate on their own node
//   - "sa" principals get warning (if DEPRECATE_SA_NODE_AUTH) or rejection (if REQUIRE_NODE_IDENTITY)
//   - admin principals are allowed (not node-scoped)
//
// Returns nil if allowed, error if denied.
func ValidateNodeOwnership(callerSubject, requestNodeID string) error {
	return ValidateNodeOwnershipForMethod(callerSubject, requestNodeID, "")
}

// ValidateNodeOwnershipForMethod is like ValidateNodeOwnership but includes
// the gRPC method name in warning/error messages for better observability.
func ValidateNodeOwnershipForMethod(callerSubject, requestNodeID, method string) error {
	if callerSubject == "" {
		return fmt.Errorf("anonymous caller cannot access node-scoped resources")
	}

	methodCtx := ""
	if method != "" {
		methodCtx = fmt.Sprintf(" method=%s", method)
	}

	// Node principals: strict own-node enforcement
	if IsNodePrincipal(callerSubject) {
		callerNodeID := ExtractNodeID(callerSubject)
		if requestNodeID != "" && callerNodeID != requestNodeID {
			return fmt.Errorf("node principal %q cannot operate on node %q (own-node-only)%s", callerSubject, requestNodeID, methodCtx)
		}
		return nil
	}

	// Legacy "sa" principal: deprecation / enforcement
	if callerSubject == "sa" {
		if RequireNodeIdentity() {
			return fmt.Errorf("sa principal rejected on node-scoped path (REQUIRE_NODE_IDENTITY=true)%s: use node_%s identity", methodCtx, requestNodeID)
		}
		if DeprecateSANodeAuth() {
			return &SADeprecationWarning{
				Subject: callerSubject,
				NodeID:  requestNodeID,
				Method:  method,
				Message: fmt.Sprintf("sa principal used on node-scoped path (deprecated)%s — will be rejected when REQUIRE_NODE_IDENTITY=true", methodCtx),
			}
		}
		return nil
	}

	// Other principals (admin, controller, operator) — allow without node scope check
	return nil
}

// SADeprecationWarning is a soft warning (not a hard rejection).
// Callers should check for this type and log a warning but allow the request.
type SADeprecationWarning struct {
	Subject string
	NodeID  string
	Method  string
	Message string
}

func (w *SADeprecationWarning) Error() string {
	return w.Message
}

// IsSADeprecationWarning returns true if the error is a soft deprecation warning.
func IsSADeprecationWarning(err error) bool {
	_, ok := err.(*SADeprecationWarning)
	return ok
}
