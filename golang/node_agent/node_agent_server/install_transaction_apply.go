package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

func rollbackInstallTransactionMetadata(name, buildID string, cause error) (string, map[string]string) {
	status := "failed"
	metadata := map[string]string{"error": cause.Error()}
	rec, err := actions.RollbackActiveInstallTransaction(name, buildID, cause.Error())
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			status = "partial_install_recovery"
			metadata["recovery_required"] = "true"
			metadata["recovery_error"] = err.Error()
		}
		return status, metadata
	}
	if rec != nil {
		metadata["transaction_id"] = rec.TransactionID
		metadata["transaction_phase"] = rec.Phase
	}
	if rec != nil && rec.Phase == actions.InstallTxnPhasePartialInstallRecovery {
		status = "partial_install_recovery"
		metadata["recovery_required"] = "true"
	} else if rec != nil {
		metadata["rollback"] = "completed"
	}
	return status, metadata
}

func (srv *NodeAgentServer) writeInstallFailureState(
	ctx context.Context,
	req *node_agentpb.ApplyPackageReleaseRequest,
	name, kind, version, buildID, transactionID string,
	cause error,
) *node_agentpb.ApplyPackageReleaseResponse {
	status, metadata := rollbackInstallTransactionMetadata(name, buildID, cause)
	ownershipState := actions.InstallOwnershipStateRolledBack
	if status == "partial_install_recovery" {
		ownershipState = actions.InstallOwnershipStatePartialInstallRecovery
	}
	_ = actions.CloseInstallOwnership(srv.nodeID, name, buildID, transactionID, ownershipState, cause.Error(), 0)
	commitCtx, cancel := installStateCommitContext()
	defer cancel()
	_ = installed_state.CommitInstalledPackage(commitCtx, &node_agentpb.InstalledPackage{
		NodeId:      srv.nodeID,
		Name:        name,
		Version:     version,
		Kind:        kind,
		Status:      status,
		UpdatedUnix: time.Now().Unix(),
		OperationId: req.GetOperationId(),
		BuildNumber: req.GetBuildNumber(),
		BuildId:     buildID,
		Metadata:    metadata,
	})
	return &node_agentpb.ApplyPackageReleaseResponse{
		Ok:          false,
		Message:     fmt.Sprintf("install failed: %v", cause),
		PackageName: name,
		Version:     version,
		Status:      status,
		ErrorDetail: cause.Error(),
		OperationId: req.GetOperationId(),
		BuildId:     buildID,
	}
}
