package actions

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
)

// probe.exec runs a command and succeeds only if exit code is 0.
type execProbeAction struct{}

func (execProbeAction) Name() string { return "probe.exec" }

func (execProbeAction) Validate(args *structpb.Struct) error { return nil }

func (execProbeAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	cmdStr := fields["cmd"].GetStringValue()
	timeoutMs := fields["timeout_ms"].GetNumberValue()
	if cmdStr == "" {
		return "", fmt.Errorf("cmd is required")
	}
	timeout := 30 * time.Second
	if timeoutMs > 0 {
		timeout = time.Duration(timeoutMs) * time.Millisecond
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "bash", "-c", cmdStr)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("probe exec failed: %v, output=%s", err, string(out))
	}
	return "probe exec ok", nil
}

func init() {
	Register(execProbeAction{})
}
