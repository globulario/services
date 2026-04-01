package apply

import (
	"context"
	"fmt"
	"time"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
)

// Operation represents a systemd unit action.
type Operation string

const (
	OpStart   Operation = "start"
	OpStop    Operation = "stop"
	OpRestart Operation = "restart"
	OpEnable  Operation = "enable"
	OpDisable Operation = "disable"
)

// Action describes a single unit operation to execute.
type Action struct {
	Unit         string
	Op           Operation
	Wait         bool
	WaitDuration time.Duration
}

// ApplyActions runs each action sequentially via the supervisor.
func ApplyActions(ctx context.Context, actions []Action, before func(Action)) error {
	if ctx == nil {
		ctx = context.Background()
	}
	for _, act := range actions {
		if before != nil {
			before(act)
		}
		var err error
		switch act.Op {
		case OpStart:
			err = supervisor.Start(ctx, act.Unit)
		case OpStop:
			err = supervisor.Stop(ctx, act.Unit)
		case OpRestart:
			err = supervisor.Restart(ctx, act.Unit)
		case OpEnable:
			err = supervisor.Enable(ctx, act.Unit)
		case OpDisable:
			err = supervisor.Disable(ctx, act.Unit)
		default:
			continue
		}
		if err != nil {
			return fmt.Errorf("%s %s: %w", act.Op, act.Unit, err)
		}
		if act.Wait {
			timeout := act.WaitDuration
			if timeout <= 0 {
				timeout = 30 * time.Second
			}
			if err := supervisor.WaitActive(ctx, act.Unit, timeout); err != nil {
				return fmt.Errorf("wait active %s: %w", act.Unit, err)
			}
		}
	}
	return nil
}
