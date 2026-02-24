package render

import (
	"time"

	clusterdoctorpb "github.com/globulario/services/golang/clusterdoctor/clusterdoctorpb"
	"github.com/globulario/services/golang/clusterdoctor/clusterdoctor_server/collector"
	"github.com/globulario/services/golang/clusterdoctor/clusterdoctor_server/rules"
)

// NodeReport builds a NodeReport proto for a single node.
func NodeReport(snap *collector.Snapshot, nodeID string, findings []rules.Finding, version string) *clusterdoctorpb.NodeReport {
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

	return &clusterdoctorpb.NodeReport{
		Header:             buildHeader(snap, version),
		NodeId:             nodeID,
		Reachable:          reachable,
		HeartbeatAgeSeconds: heartbeatAge,
		Findings:           protoFindings,
	}
}
