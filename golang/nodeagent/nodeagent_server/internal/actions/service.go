package actions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/supervisor"
	"google.golang.org/protobuf/types/known/structpb"
)

type serviceAction struct {
	name string
	op   func(context.Context, string) error
}

func (a *serviceAction) Name() string {
	return a.name
}

func (a *serviceAction) Validate(args *structpb.Struct) error {
	if args == nil {
		return errors.New("args required")
	}
	unit := strings.TrimSpace(args.GetFields()["unit"].GetStringValue())
	if unit == "" {
		return errors.New("unit is required")
	}
	return nil
}

func (a *serviceAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	unit := strings.TrimSpace(args.GetFields()["unit"].GetStringValue())
	if unit == "" {
		return "", errors.New("unit is required")
	}
	switch a.name {
	case "service.start":
		if active, err := supervisor.IsActive(ctx, unit); err == nil && active {
			return "already running", nil
		}
	case "service.stop":
		if active, err := supervisor.IsActive(ctx, unit); err == nil && !active {
			return "already stopped", nil
		}
	}
	if err := a.op(ctx, unit); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s completed", a.name), nil
}

func init() {
	Register(&serviceAction{name: "service.start", op: supervisor.Start})
	Register(&serviceAction{name: "service.stop", op: supervisor.Stop})
	Register(&serviceAction{name: "service.restart", op: supervisor.Restart})
}
