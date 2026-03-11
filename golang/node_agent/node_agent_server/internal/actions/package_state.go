package actions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/protobuf/types/known/structpb"
)

// packageReportStateAction writes an InstalledPackage record to etcd after
// successful lifecycle execution. This is the canonical installed-state writer
// for all package kinds (SERVICE, APPLICATION, INFRASTRUCTURE).
//
// Plan step args:
//
//	node_id      (string, required)
//	name         (string, required)
//	version      (string, required)
//	kind         (string, required: "SERVICE", "APPLICATION", "INFRASTRUCTURE")
//	publisher_id (string, optional)
//	platform     (string, optional)
//	checksum     (string, optional)
//	operation_id (string, optional)
//	status       (string, optional, default: "installed")
type packageReportStateAction struct{}

func (packageReportStateAction) Name() string { return "package.report_state" }

func (packageReportStateAction) Validate(args *structpb.Struct) error {
	fields := args.GetFields()
	if strings.TrimSpace(fields["node_id"].GetStringValue()) == "" {
		return fmt.Errorf("package.report_state: node_id is required")
	}
	if strings.TrimSpace(fields["name"].GetStringValue()) == "" {
		return fmt.Errorf("package.report_state: name is required")
	}
	if strings.TrimSpace(fields["version"].GetStringValue()) == "" {
		return fmt.Errorf("package.report_state: version is required")
	}
	kind := strings.ToUpper(strings.TrimSpace(fields["kind"].GetStringValue()))
	switch kind {
	case "SERVICE", "APPLICATION", "INFRASTRUCTURE":
		// valid
	default:
		return fmt.Errorf("package.report_state: kind must be SERVICE, APPLICATION, or INFRASTRUCTURE (got %q)", kind)
	}
	return nil
}

func (packageReportStateAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()

	nodeID := strings.TrimSpace(fields["node_id"].GetStringValue())
	name := strings.TrimSpace(fields["name"].GetStringValue())
	version := strings.TrimSpace(fields["version"].GetStringValue())
	kind := strings.ToUpper(strings.TrimSpace(fields["kind"].GetStringValue()))
	publisherID := strings.TrimSpace(fields["publisher_id"].GetStringValue())
	platform := strings.TrimSpace(fields["platform"].GetStringValue())
	checksum := strings.TrimSpace(fields["checksum"].GetStringValue())
	operationID := strings.TrimSpace(fields["operation_id"].GetStringValue())
	status := strings.TrimSpace(fields["status"].GetStringValue())
	if status == "" {
		status = "installed"
	}

	now := time.Now().Unix()

	// Check if there's an existing record (to preserve installed_unix).
	existing, _ := installed_state.GetInstalledPackage(ctx, nodeID, kind, name)
	installedUnix := now
	if existing != nil && existing.InstalledUnix > 0 {
		installedUnix = existing.InstalledUnix
	}

	// Build metadata from extra fields.
	metadata := make(map[string]string)
	for k, v := range fields {
		switch k {
		case "node_id", "name", "version", "kind", "publisher_id", "platform",
			"checksum", "operation_id", "status":
			continue
		default:
			if s := v.GetStringValue(); s != "" {
				metadata[k] = s
			}
		}
	}
	if len(metadata) == 0 {
		metadata = nil
	}

	pkg := &node_agentpb.InstalledPackage{
		NodeId:       nodeID,
		Name:         name,
		Version:      version,
		PublisherId:  publisherID,
		Platform:     platform,
		Kind:         kind,
		Checksum:     checksum,
		InstalledUnix: installedUnix,
		UpdatedUnix:  now,
		Status:       status,
		OperationId:  operationID,
		Metadata:     metadata,
	}

	if err := installed_state.WriteInstalledPackage(ctx, pkg); err != nil {
		return "", fmt.Errorf("package.report_state: %w", err)
	}

	return fmt.Sprintf("installed-state written: %s/%s@%s on %s", kind, name, version, nodeID), nil
}

func init() {
	Register(packageReportStateAction{})
}
