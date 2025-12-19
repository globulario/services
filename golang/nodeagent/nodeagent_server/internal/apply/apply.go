package apply

import (
	"context"
	"fmt"
	"time"

	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/planner"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/supervisor"
)

// ApplyActions runs each action sequentially via the supervisor.
func ApplyActions(ctx context.Context, actions []planner.Action) error {
	if ctx == nil {
		ctx = context.Background()
	}
	for _, act := range actions {
		var err error
		switch act.Op {
		case planner.OpStart:
			err = supervisor.Start(ctx, act.Unit)
		case planner.OpStop:
			err = supervisor.Stop(ctx, act.Unit)
		case planner.OpRestart:
			err = supervisor.Restart(ctx, act.Unit)
		case planner.OpEnable:
			err = supervisor.Enable(ctx, act.Unit)
		case planner.OpDisable:
			err = supervisor.Disable(ctx, act.Unit)
		default:
			continue
		}
		if err != nil {
			return fmt.Errorf("%s %s: %w", act.Op, act.Unit, err)
		}
		if act.Wait {
			if err := supervisor.WaitActive(ctx, act.Unit, 30*time.Second); err != nil {
				return fmt.Errorf("wait active %s: %w", act.Unit, err)
			}
		}
	}
	return nil
}
