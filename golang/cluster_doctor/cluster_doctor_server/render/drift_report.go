package render

import (
	"fmt"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// DriftReport builds a DriftReport proto from the snapshot.
// nodeID filters to a single node; empty string = all nodes.
func DriftReport(snap *collector.Snapshot, nodeID string, version string) *cluster_doctorpb.DriftReport {
	var items []*cluster_doctorpb.DriftItem

	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		if nodeID != "" && nid != nodeID {
			continue
		}

		// Hash mismatches from GetClusterHealthV1
		if nh, ok := snap.NodeHealths[nid]; ok {
			if d := nh.GetDesiredServicesHash(); d != "" && d != "services:none" && d != nh.GetAppliedServicesHash() {
				items = append(items, &cluster_doctorpb.DriftItem{
					NodeId:    nid,
					EntityRef: "services",
					Category:  cluster_doctorpb.DriftCategory_STATE_HASH_MISMATCH,
					Desired:   d,
					Actual:    nh.GetAppliedServicesHash(),
					Evidence: []*cluster_doctorpb.Evidence{{
						SourceService: "clustercontroller",
						SourceRpc:     "GetClusterHealthV1",
						KeyValues: map[string]string{
							"node_id":      nid,
							"desired_hash": d,
							"applied_hash": nh.GetAppliedServicesHash(),
							"scope":        "services",
						},
						Timestamp: timestamppb.New(snap.GeneratedAt),
					}},
				})
			}
			if d := nh.GetDesiredNetworkHash(); d != "" && d != nh.GetAppliedNetworkHash() {
				items = append(items, &cluster_doctorpb.DriftItem{
					NodeId:    nid,
					EntityRef: "network",
					Category:  cluster_doctorpb.DriftCategory_STATE_HASH_MISMATCH,
					Desired:   d,
					Actual:    nh.GetAppliedNetworkHash(),
					Evidence: []*cluster_doctorpb.Evidence{{
						SourceService: "clustercontroller",
						SourceRpc:     "GetClusterHealthV1",
						KeyValues: map[string]string{
							"node_id":      nid,
							"desired_hash": d,
							"applied_hash": nh.GetAppliedNetworkHash(),
							"scope":        "network",
						},
						Timestamp: timestamppb.New(snap.GeneratedAt),
					}},
				})
			}
		}

		// Unit-level drift from GetInventory
		if inv, ok := snap.Inventories[nid]; ok {
			for _, u := range inv.GetUnits() {
				state := rules.NormalizeUnitState(u.GetState())
				var cat cluster_doctorpb.DriftCategory
				switch state {
				case rules.UnitStateNotFound:
					cat = cluster_doctorpb.DriftCategory_MISSING_UNIT_FILE
				case rules.UnitStateInactive:
					cat = cluster_doctorpb.DriftCategory_UNIT_STOPPED
				case rules.UnitStateDisabled:
					cat = cluster_doctorpb.DriftCategory_UNIT_DISABLED
				default:
					continue
				}
				items = append(items, &cluster_doctorpb.DriftItem{
					NodeId:    nid,
					EntityRef: u.GetName(),
					Category:  cat,
					Desired:   "active",
					Actual:    u.GetState(),
					Evidence: []*cluster_doctorpb.Evidence{{
						SourceService: "nodeagent",
						SourceRpc:     "GetInventory",
						KeyValues: map[string]string{
							"node_id":   nid,
							"unit_name": u.GetName(),
							"state":     u.GetState(),
						},
						Timestamp: timestamppb.New(snap.GeneratedAt),
					}},
				})
			}

			// Plan-based version drift check removed — plan system deleted.

			// Components not installed
			for _, comp := range inv.GetComponents() {
				if !comp.GetInstalled() {
					items = append(items, &cluster_doctorpb.DriftItem{
						NodeId:    nid,
						EntityRef: comp.GetName(),
						Category:  cluster_doctorpb.DriftCategory_INVENTORY_INCOMPLETE,
						Desired:   fmt.Sprintf("%s installed", comp.GetName()),
						Actual:    "not installed",
						Evidence: []*cluster_doctorpb.Evidence{{
							SourceService: "nodeagent",
							SourceRpc:     "GetInventory",
							KeyValues: map[string]string{
								"node_id":   nid,
								"component": comp.GetName(),
								"version":   comp.GetVersion(),
								"installed": "false",
							},
							Timestamp: timestamppb.New(snap.GeneratedAt),
						}},
					})
				}
			}
		}
	}

	return &cluster_doctorpb.DriftReport{
		Header:          buildHeader(snap, version),
		Items:           items,
		TotalDriftCount: uint32(len(items)),
	}
}
