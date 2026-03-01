package render

import (
	"time"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
)

// NodeReport builds a NodeReport proto for a single node.
func NodeReport(snap *collector.Snapshot, nodeID string, findings []rules.Finding, version string) *cluster_doctorpb.NodeReport {
	protoFindings := toProtoFindings(findings)
	sortFindingsBySeverity(protoFindings)

	reachable := true
	var heartbeatAge int64
	now := time.Now()

	for _, n := range snap.Nodes {
		if n.GetNodeId() == nodeID {
			lastSeen := n.GetLastSeen().AsTime()
			heartbeatAge = int64(now.Sub(lastSeen).Seconds())
			// Determine reachability from findings
			for _, f := range findings {
				if f.InvariantID == "node.reachable" && f.EntityRef == nodeID {
					reachable = false
					break
				}
			}
			break
		}
	}

	return &cluster_doctorpb.NodeReport{
		Header:             buildHeader(snap, version),
		NodeId:             nodeID,
		Reachable:          reachable,
		HeartbeatAgeSeconds: heartbeatAge,
		Findings:           protoFindings,
	}
}
