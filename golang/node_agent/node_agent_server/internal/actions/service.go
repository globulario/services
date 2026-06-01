package actions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions/serviceports"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
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
	svc := serviceFromUnit(unit)
	switch a.name {
	case "service.start":
		if svc != "" {
			if err := serviceports.EnsureServicePortReady(ctx, svc, unit); err != nil {
				return "", err
			}
		}
		if active, err := supervisor.IsActive(ctx, unit); err == nil && active {
			return "already running", nil
		}
	case "service.stop":
		if active, err := supervisor.IsActive(ctx, unit); err == nil && !active {
			return "already stopped", nil
		}
	case "service.restart":
		if svc != "" {
			if err := serviceports.EnsureServicePortReady(ctx, svc, unit); err != nil {
				return "", err
			}
		}
	case "service.enable":
		// Enable is always safe — no idempotency check needed
	case "service.disable":
		// Disable is always safe — no idempotency check needed
	}
	if err := a.op(ctx, unit); err != nil {
		return "", err
	}

	// After restart, reconcile the config file port with the service's actual
	// listening port. Services load their port from etcd (not the JSON file),
	// so the JSON can drift. The probe reads the JSON, so keeping it in sync
	// prevents false invariant failures.
	if a.name == "service.restart" && svc != "" {
		serviceports.ReconcilePortAfterRestart(ctx, svc)
	}

	return fmt.Sprintf("%s completed", a.name), nil
}

// serviceFromUnit returns the service name if the unit is a Globular service
// with a --describe binary. Returns "" for infrastructure components (scylladb,
// etcd, minio, envoy, etc.) that don't have --describe support.
func serviceFromUnit(unit string) string {
	u := strings.ToLower(strings.TrimSpace(unit))
	u = strings.TrimSuffix(u, ".service")
	u = strings.TrimPrefix(u, "globular-")
	if u == "" {
		return ""
	}
	// Non-globular units (e.g. scylla-server.service) are never Globular
	// services. Only "globular-<name>.service" units can have --describe.
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(unit)), "globular-") {
		return ""
	}
	// Use the identity registry to check if this is a known Globular service
	// with a _server binary (has --describe). Infrastructure components
	// (scylladb, etcd, minio, envoy, prometheus, etc.) return "".
	exe := executableForService(u)
	if exe == "" {
		return ""
	}
	// Only return service name if the binary follows the _server convention
	// (meaning it supports --describe). Third-party infra binaries don't.
	if !strings.HasSuffix(exe, "_server") {
		return ""
	}
	// Final safety: verify the binary actually exists on disk. Infra
	// packages (scylladb, etc.) that aren't in the identity registry hit
	// the fallback naming convention (name + "_server"), which may not
	// exist. Calling --describe on a missing binary crashes the plan.
	binDir := installBinDir()
	if _, err := os.Stat(filepath.Join(binDir, exe)); err != nil {
		return ""
	}
	return u
}

func init() {
	Register(&serviceAction{name: "service.start", op: supervisor.Start})
	Register(&serviceAction{name: "service.stop", op: supervisor.Stop})
	Register(&serviceAction{name: "service.restart", op: supervisor.Restart})
	Register(&serviceAction{name: "service.enable", op: supervisor.Enable})
	Register(&serviceAction{name: "service.disable", op: supervisor.Disable})
}
