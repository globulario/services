// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.actions
// @awareness file_role=action_registry_and_dispatch
// @awareness implements=globular.platform:intent.node_agent.is_executor_not_cluster_brain
// @awareness risk=high
package actions

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
