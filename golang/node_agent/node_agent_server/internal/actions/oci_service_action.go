// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.actions.oci_service_lifecycle
// @awareness file_role=node_local_oci_package_materialization_inside_existing_workflow_actions
// @awareness implements=globular.platform:intent.node_agent.is_executor_not_cluster_brain
// @awareness risk=critical
package actions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
	"github.com/globulario/services/golang/oci"
	ocilifecycle "github.com/globulario/services/golang/oci/lifecycle"
	"google.golang.org/protobuf/types/known/structpb"
)

var ActionOCIDockerSocket = "unix:///var/run/docker.sock"

type actionOCISupervisor struct{}

func (actionOCISupervisor) DaemonReload(ctx context.Context) error {
	if ActionSkipSystemd || ActionSkipDaemonReload {
		return nil
	}
	return supervisor.DaemonReload(ctx)
}
func (actionOCISupervisor) Stop(ctx context.Context, unit string) error {
	if ActionSkipSystemd {
		return nil
	}
	return supervisor.Stop(ctx, unit)
}
func (actionOCISupervisor) Disable(ctx context.Context, unit string) error {
	if ActionSkipSystemd {
		return nil
	}
	return supervisor.Disable(ctx, unit)
}

func actionOCIManager() ocilifecycle.Manager {
	return ocilifecycle.Manager{Layout: ocilifecycle.Layout{
		ConfigRoot:   filepath.Join(ActionConfigDir, "oci", "services"),
		SystemdRoot:  ActionSystemdDir,
		StateRoot:    filepath.Join(ActionStateDir, "oci"),
		RunnerPath:   filepath.Join(ActionBinDir, "globular-oci-runner"),
		PolicyPath:   filepath.Join(ActionConfigDir, "oci", "policy.json"),
		DockerSocket: ActionOCIDockerSocket,
	}, Supervisor: actionOCISupervisor{}}
}

func loadActionOCIPolicy() (oci.Policy, error) {
	path := filepath.Join(ActionConfigDir, "oci", "policy.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return oci.DefaultPolicy(), nil
	} else if err != nil {
		return oci.Policy{}, fmt.Errorf("inspect node OCI policy: %w", err)
	}
	return oci.LoadPolicy(path)
}

type ociInstallPayloadHandler struct{ next Handler }

func (h ociInstallPayloadHandler) Name() string { return h.next.Name() }
func (h ociInstallPayloadHandler) Validate(args *structpb.Struct) error {
	if err := h.next.Validate(args); err != nil {
		return err
	}
	inspection, found, err := inspectOCIActionArtifact(args, "service", "artifact_path")
	if err != nil || !found {
		return err
	}
	policy, err := loadActionOCIPolicy()
	if err != nil {
		return err
	}
	return inspection.Validate(policy)
}
func (h ociInstallPayloadHandler) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	inspection, found, err := inspectOCIActionArtifact(args, "service", "artifact_path")
	if err != nil {
		return "", err
	}
	if !found {
		return h.next.Apply(ctx, args)
	}
	policy, err := loadActionOCIPolicy()
	if err != nil {
		return "", err
	}
	if err := inspection.Validate(policy); err != nil {
		return "", err
	}
	result, err := h.next.Apply(ctx, args)
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(args.GetFields()["version"].GetStringValue())
	receipt, err := actionOCIManager().Install(ctx, inspection, policy, version)
	if err != nil {
		return "", fmt.Errorf("materialize OCI service lifecycle: %w", err)
	}
	return fmt.Sprintf("%s; OCI runtime materialized unit=%s image=%s", result, receipt.UnitName, receipt.Image), nil
}

type ociPackageUninstallHandler struct{ next Handler }

func (h ociPackageUninstallHandler) Name() string { return h.next.Name() }
func (h ociPackageUninstallHandler) Validate(args *structpb.Struct) error {
	return h.next.Validate(args)
}
func (h ociPackageUninstallHandler) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	name := strings.TrimSpace(args.GetFields()["name"].GetStringValue())
	manager := actionOCIManager()
	managed, err := manager.Uninstall(ctx, name)
	if err != nil {
		return "", err
	}
	result, err := h.next.Apply(ctx, args)
	if err != nil {
		return "", err
	}
	if managed {
		return result + "; OCI runtime files removed, persistent data retained", nil
	}
	return result, nil
}

type ociPackageVerifyHandler struct{ next Handler }

func (h ociPackageVerifyHandler) Name() string                         { return h.next.Name() }
func (h ociPackageVerifyHandler) Validate(args *structpb.Struct) error { return h.next.Validate(args) }
func (h ociPackageVerifyHandler) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	name := strings.TrimSpace(args.GetFields()["name"].GetStringValue())
	manager := actionOCIManager()
	if !manager.IsManaged(name) {
		return h.next.Apply(ctx, args)
	}
	verification, err := manager.Verify(name)
	if err != nil {
		return "", fmt.Errorf("verify OCI package %s: %w", name, err)
	}
	return fmt.Sprintf("OCI package %s verified image=%s spec=%s phase=%s", name, verification.Receipt.Image, verification.Receipt.SpecDigest, verification.Observed.Phase), nil
}

type ociPackageReportStateHandler struct{ next Handler }

func (h ociPackageReportStateHandler) Name() string { return h.next.Name() }
func (h ociPackageReportStateHandler) Validate(args *structpb.Struct) error {
	return h.next.Validate(args)
}
func (h ociPackageReportStateHandler) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	if args == nil {
		return h.next.Apply(ctx, args)
	}
	fields := args.GetFields()
	name := strings.TrimSpace(fields["name"].GetStringValue())
	receipt, err := actionOCIManager().ReadReceipt(name)
	if err == nil {
		fields["runtime_kind"] = structpb.NewStringValue("oci")
		fields["oci_image"] = structpb.NewStringValue(receipt.Image)
		fields["oci_spec_digest"] = structpb.NewStringValue(receipt.SpecDigest)
		fields["oci_instance"] = structpb.NewStringValue(receipt.Instance)
		fields["oci_unit"] = structpb.NewStringValue(receipt.UnitName)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read OCI package receipt before installed-state projection: %w", err)
	}
	return h.next.Apply(ctx, args)
}

func inspectOCIActionArtifact(args *structpb.Struct, serviceField, artifactField string) (ocilifecycle.Inspection, bool, error) {
	if args == nil {
		return ocilifecycle.Inspection{}, false, fmt.Errorf("args are required")
	}
	fields := args.GetFields()
	service := strings.TrimSpace(fields[serviceField].GetStringValue())
	artifact := strings.TrimSpace(fields[artifactField].GetStringValue())
	if service == "" || artifact == "" {
		return ocilifecycle.Inspection{}, false, nil
	}
	return ocilifecycle.InspectArtifact(artifact, service)
}

func init() {
	Decorate("service.install_payload", func(next Handler) Handler { return ociInstallPayloadHandler{next: next} })
	Decorate("package.uninstall", func(next Handler) Handler { return ociPackageUninstallHandler{next: next} })
	Decorate("package.verify", func(next Handler) Handler { return ociPackageVerifyHandler{next: next} })
	Decorate("package.report_state", func(next Handler) Handler { return ociPackageReportStateHandler{next: next} })
}
