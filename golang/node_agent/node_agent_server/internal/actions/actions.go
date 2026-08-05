// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.actions.registry
// @awareness file_role=action_handler_registry_with_validate_before_apply_contract
// @awareness implements=globular.platform:intent.node_agent.is_executor_not_cluster_brain
// @awareness risk=high
package actions

// actions.go — the registry every workflow-step action handler registers into.
// Two non-negotiable handler properties:
//
//  1. Validate(args) MUST be pure (no side effects). It runs before Apply and
//     is the only safe place to reject a malformed dispatch.
//  2. Apply MUST be idempotent. The workflow service can replay a step after a
//     transient failure; non-idempotent handlers turn replay into double execution.
//
// Decorators extend an existing action without creating a second mutation
// endpoint. This is how runtime-specific implementations remain behind the same
// governed workflow action and preserve validate-before-apply semantics.

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

// Decorator wraps an existing action. A decorator may inspect and refine the
// action, but it must preserve the original action name and authority path.
type Decorator func(Handler) Handler

var (
	registry   = map[string]Handler{}
	decorators = map[string][]Decorator{}
)

func Register(handler Handler) {
	if handler == nil {
		return
	}
	name := strings.ToLower(strings.TrimSpace(handler.Name()))
	if name == "" {
		return
	}
	for _, decorate := range decorators[name] {
		handler = decorate(handler)
		if handler == nil {
			return
		}
	}
	registry[name] = handler
}

// Decorate is init-order independent. If the base action is already
// registered, it is wrapped immediately. Otherwise, the decorator is retained
// and applied when Register later receives the base handler.
func Decorate(name string, decorate Decorator) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || decorate == nil {
		return
	}
	decorators[name] = append(decorators[name], decorate)
	if existing := registry[name]; existing != nil {
		if wrapped := decorate(existing); wrapped != nil {
			registry[name] = wrapped
		}
	}
}

func Get(name string) Handler {
	return registry[strings.ToLower(strings.TrimSpace(name))]
}
