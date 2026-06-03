// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.actions.registry
// @awareness file_role=action_handler_registry_with_validate_before_apply_contract
// @awareness implements=globular.platform:intent.node_agent.is_executor_not_cluster_brain
// @awareness risk=high
package actions

// actions.go — the registry every workflow-step action handler
// registers into. Two non-negotiable handler properties:
//
//   1. Validate(args) MUST be pure (no side effects). It runs
//      before Apply and is the only safe place to reject a
//      malformed dispatch.
//
//   2. Apply MUST be idempotent. The workflow service can
//      replay a step after a transient failure; non-idempotent
//      handlers turn replay into double-execution.
//
// Adding a handler that performs validation inside Apply by
// returning an error after a side effect breaks the
// "Validate-then-Apply" contract that every replay assumes.

import (
	"context"
	"strings"

	"google.golang.org/protobuf/types/known/structpb"
)

type Handler interface {
	Name() string
	Validate(args *structpb.Struct) error
	Apply(ctx context.Context, args *structpb.Struct) (string, error)
}

var registry = map[string]Handler{}

func Register(handler Handler) {
	if handler == nil {
		return
	}
	name := strings.ToLower(strings.TrimSpace(handler.Name()))
	if name == "" {
		return
	}
	registry[name] = handler
}

func Get(name string) Handler {
	return registry[strings.ToLower(strings.TrimSpace(name))]
}
