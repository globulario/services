package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/globulario/services/golang/awareness/graph"
)

// RuntimeBridge collects a read-only runtime snapshot from pluggable sources.
// Missing sources add a warning to the snapshot but never fail the collection.
// RuntimeBridge must never dispatch remediation, modify desired state, or
// write installed state.
type RuntimeBridge struct {
	NodeID    string
	ClusterID string

	Doctor      DoctorSource
	Events      EventSource
	Workflows   WorkflowSource
	State       StateSource
	Services    ServiceStatusSource
	Repository  RepositoryStatusSource
	Objectstore ObjectstoreStatusSource
	XDS         XDSStatusSource
	Systemd     SystemdStatusSource
	Metrics     MetricsSource
}

// NewBridge returns a RuntimeBridge with all noop sources.
// Callers replace sources with real implementations as available.
func NewBridge(nodeID, clusterID string) *RuntimeBridge {
	return &RuntimeBridge{
		NodeID:      nodeID,
		ClusterID:   clusterID,
		Doctor:      NoopDoctorSource{},
		Events:      NoopEventSource{},
		Workflows:   NoopWorkflowSource{},
		State:       NoopStateSource{},
		Services:    NoopServiceStatusSource{},
		Repository:  NoopRepositoryStatusSource{},
		Objectstore: NoopObjectstoreStatusSource{},
		XDS:         NoopXDSStatusSource{},
		Systemd:     NoopSystemdStatusSource{},
		Metrics:     NoopMetricsSource{},
	}
}

// Snapshot collects a read-only runtime snapshot from all sources.
// since is the lookback window for event and workflow queries.
// The returned snapshot's Match fields are populated using the provided graph (may be nil).
func (b *RuntimeBridge) Snapshot(ctx context.Context, since time.Duration, g *graph.Graph) (*RuntimeSnapshot, error) {
	now := time.Now().UTC()
	snap := &RuntimeSnapshot{
		ID:         fmt.Sprintf("snapshot-%d", now.Unix()),
		CapturedAt: now,
		NodeID:     b.NodeID,
		ClusterID:  b.ClusterID,
	}

	// Collect from each source — failures add warnings, not errors.
	// Source-level warnings go into SourceWarnings so that Match() can
	// recompute Warnings idempotently without duplicating them.
	addSourceWarn := func(msg string) {
		snap.SourceWarnings = append(snap.SourceWarnings, msg)
	}

	if findings, err := b.Doctor.Findings(ctx); err != nil {
		addSourceWarn("doctor source unavailable: " + err.Error())
	} else {
		snap.DoctorFindings = findings
	}

	if events, err := b.Events.RecentEvents(ctx, since); err != nil {
		addSourceWarn("event source unavailable: " + err.Error())
	} else {
		snap.RecentEvents = events
	}

	if receipts, err := b.Workflows.RecentReceipts(ctx, since); err != nil {
		addSourceWarn("workflow source unavailable: " + err.Error())
	} else {
		snap.WorkflowReceipts = receipts
	}

	if desired, err := b.State.DesiredState(ctx); err != nil {
		addSourceWarn("desired-state source unavailable: " + err.Error())
	} else {
		snap.DesiredState = desired
	}

	if installed, err := b.State.InstalledState(ctx); err != nil {
		addSourceWarn("installed-state source unavailable: " + err.Error())
	} else {
		snap.InstalledState = installed
	}

	if services, err := b.Services.Services(ctx); err != nil {
		addSourceWarn("service-status source unavailable: " + err.Error())
	} else {
		snap.RuntimeServices = services
	}

	if repoStatus, err := b.Repository.Status(ctx); err != nil {
		addSourceWarn("repository source unavailable: " + err.Error())
	} else {
		snap.RepositoryStatus = repoStatus
	}

	if osStatus, err := b.Objectstore.Status(ctx); err != nil {
		addSourceWarn("objectstore source unavailable: " + err.Error())
	} else {
		snap.ObjectstoreStatus = osStatus
	}

	if xdsStatus, err := b.XDS.Status(ctx); err != nil {
		addSourceWarn("xDS source unavailable: " + err.Error())
	} else {
		snap.XDSStatus = xdsStatus
	}

	if units, err := b.Systemd.Units(ctx); err != nil {
		addSourceWarn("systemd source unavailable: " + err.Error())
	} else {
		snap.SystemdUnits = units
	}

	if samples, err := b.Metrics.Samples(ctx); err != nil {
		addSourceWarn("metrics source unavailable: " + err.Error())
	} else {
		snap.Metrics = samples
	}

	// Populate Warnings from SourceWarnings before Match() so callers
	// can inspect them without calling Match.
	snap.Warnings = append([]string(nil), snap.SourceWarnings...)

	// Match against known graph invariants/failure modes if graph is available.
	if g != nil {
		invs, _ := g.AllInvariants(ctx)
		fms, _ := g.AllFailureModes(ctx)
		invIDs := make([]string, 0, len(invs))
		for _, inv := range invs {
			invIDs = append(invIDs, inv.ID)
		}
		fmIDs := make([]string, 0, len(fms))
		for _, fm := range fms {
			fmIDs = append(fmIDs, fm.ID)
		}
		snap = snap.Match(invIDs, fmIDs)
	}

	return snap, nil
}

// WriteToGraph writes the snapshot as nodes and edges into the awareness graph.
// This is the only write operation the bridge performs — it only records evidence.
func (b *RuntimeBridge) WriteToGraph(ctx context.Context, snap *RuntimeSnapshot, g *graph.Graph) error {
	snapNodeID := "runtime_snapshot:" + snap.ID

	// Snapshot node.
	if err := g.AddNode(ctx, graph.Node{
		ID:      snapNodeID,
		Type:    graph.NodeTypeRuntimeSnapshot,
		Name:    snap.ID,
		Summary: fmt.Sprintf("captured at %s on node %s", snap.CapturedAt.Format(time.RFC3339), snap.NodeID),
	}); err != nil {
		return fmt.Errorf("WriteToGraph snapshot node: %w", err)
	}

	// Doctor findings → doctor_evidence nodes linked to snapshot.
	for _, f := range snap.DoctorFindings {
		nodeID := "doctor_evidence:" + snap.ID + ":" + f.FindingID
		_ = g.AddNode(ctx, graph.Node{
			ID:      nodeID,
			Type:    graph.NodeTypeDoctorEvidence,
			Name:    f.FindingID,
			Summary: f.Title,
		})
		_ = g.AddEdge(ctx, graph.Edge{Src: nodeID, Kind: graph.EdgeCapturedIn, Dst: snapNodeID})
		if f.InvariantRef != "" {
			_ = g.AddEdge(ctx, graph.Edge{Src: nodeID, Kind: graph.EdgeMatchesInvariant, Dst: "invariant:" + f.InvariantRef})
		}
	}

	// State deltas → state_delta nodes.
	for i, delta := range snap.StateDelta {
		nodeID := fmt.Sprintf("state_delta:%s:%s:%d", snap.ID, delta.ServiceID, i)
		_ = g.AddNode(ctx, graph.Node{
			ID:      nodeID,
			Type:    graph.NodeTypeStateDelta,
			Name:    delta.ServiceID,
			Summary: delta.DeltaType,
		})
		_ = g.AddEdge(ctx, graph.Edge{Src: snapNodeID, Kind: graph.EdgeHasStateDelta, Dst: nodeID})
	}

	// Matched invariants — link snapshot to invariant nodes.
	for _, invID := range snap.MatchedInvariants {
		_ = g.AddEdge(ctx, graph.Edge{
			Src:  snapNodeID,
			Kind: graph.EdgeMatchesInvariant,
			Dst:  "invariant:" + invID,
		})
	}

	// Matched failure modes — link snapshot to failure mode nodes.
	for _, fmID := range snap.MatchedFailureModes {
		_ = g.AddEdge(ctx, graph.Edge{
			Src:  snapNodeID,
			Kind: graph.EdgeMatchesFailureMode,
			Dst:  "failure_mode:" + fmID,
		})
	}

	// Store snapshot JSON in runtime_snapshots table.
	snapJSON, err := marshalSnapshot(snap)
	if err != nil {
		return fmt.Errorf("WriteToGraph marshal: %w", err)
	}
	return g.UpsertRuntimeSnapshot(ctx, snap.ID, snap.CapturedAt.Unix(), snap.NodeID, snap.ClusterID, snapJSON)
}

// marshalSnapshot serialises a RuntimeSnapshot to JSON for storage.
func marshalSnapshot(snap *RuntimeSnapshot) ([]byte, error) {
	return json.Marshal(snap)
}
