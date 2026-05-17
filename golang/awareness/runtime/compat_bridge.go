package runtime

// compat_bridge.go implements the runtime.BridgeSnapshotter interface from the
// standalone awareness module. This allows RuntimeBridge to be passed as
// preflight.Options.Bridge without a type mismatch between modules.

import (
	"context"
	"time"

	standaloneRuntime "github.com/globulario/awareness/runtime"
	"github.com/globulario/awareness/graph"
)

// BridgeSnapshot implements standaloneRuntime.BridgeSnapshotter by calling
// the full Snapshot() method and converting the result to BridgeSnapshot.
// The graph argument is passed as interface{} per the interface contract and
// must be a *graph.Graph (or nil).
func (b *RuntimeBridge) BridgeSnapshot(ctx context.Context, since time.Duration, g interface{}) (*standaloneRuntime.BridgeSnapshot, error) {
	var gGraph *graph.Graph
	if g != nil {
		if typed, ok := g.(*graph.Graph); ok {
			gGraph = typed
		}
	}
	snap, err := b.Snapshot(ctx, since, gGraph)
	if err != nil {
		return nil, err
	}
	return convertToBridgeSnapshot(snap), nil
}

// convertToBridgeSnapshot converts a full RuntimeSnapshot to a BridgeSnapshot
// containing only the fields that the standalone preflight package needs.
func convertToBridgeSnapshot(snap *RuntimeSnapshot) *standaloneRuntime.BridgeSnapshot {
	bs := &standaloneRuntime.BridgeSnapshot{
		CapturedAt:          snap.CapturedAt,
		MatchedInvariants:   snap.MatchedInvariants,
		MatchedFailureModes: snap.MatchedFailureModes,
		Warnings:            snap.Warnings,
		DoctorFindings:      make([]standaloneRuntime.DoctorFindingCompat, 0, len(snap.DoctorFindings)),
		ServiceStatuses:     make([]standaloneRuntime.ServiceStatusCompat, 0, len(snap.RuntimeServices)),
		WorkflowReceipts:    make([]standaloneRuntime.WorkflowReceiptCompat, 0, len(snap.WorkflowReceipts)),
		StateDeltas:         make([]standaloneRuntime.StateDeltaCompat, 0, len(snap.StateDelta)),
		RepositoryStatuses:  make([]standaloneRuntime.RepositoryStatusCompat, 0, len(snap.RepositoryStatus)),
	}
	for _, f := range snap.DoctorFindings {
		bs.DoctorFindings = append(bs.DoctorFindings, standaloneRuntime.DoctorFindingCompat{
			FindingID:  f.FindingID,
			Severity:   f.Severity,
			Title:      f.Title,
			Suppressed: f.Suppressed,
		})
	}
	for _, svc := range snap.RuntimeServices {
		bs.ServiceStatuses = append(bs.ServiceStatuses, standaloneRuntime.ServiceStatusCompat{
			ServiceID: svc.ServiceID,
			NodeID:    svc.NodeID,
			State:     svc.State,
		})
	}
	for _, wf := range snap.WorkflowReceipts {
		bs.WorkflowReceipts = append(bs.WorkflowReceipts, standaloneRuntime.WorkflowReceiptCompat{
			WorkflowType: wf.WorkflowType,
			Status:       wf.Status,
			ErrorMsg:     wf.ErrorMsg,
		})
	}
	for _, d := range snap.StateDelta {
		bs.StateDeltas = append(bs.StateDeltas, standaloneRuntime.StateDeltaCompat{
			ServiceID:        d.ServiceID,
			DeltaType:        d.DeltaType,
			DesiredVersion:   d.DesiredVersion,
			InstalledVersion: d.InstalledVersion,
		})
	}
	for _, rs := range snap.RepositoryStatus {
		bs.RepositoryStatuses = append(bs.RepositoryStatuses, standaloneRuntime.RepositoryStatusCompat{
			Mode:      rs.Mode,
			NodeID:    rs.NodeID,
			Reachable: rs.Reachable,
			LastError: rs.LastError,
		})
	}
	return bs
}
