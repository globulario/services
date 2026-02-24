package render

import (
	"fmt"

	clusterdoctorpb "github.com/globulario/services/golang/clusterdoctor/clusterdoctorpb"
	"github.com/globulario/services/golang/clusterdoctor/clusterdoctor_server/collector"
	"github.com/globulario/services/golang/clusterdoctor/clusterdoctor_server/rules"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// DriftReport builds a DriftReport proto from the snapshot.
// nodeID filters to a single node; empty string = all nodes.
func DriftReport(snap *collector.Snapshot, nodeID string, version string) *clusterdoctorpb.DriftReport {
	var items []*clusterdoctorpb.DriftItem

	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		if nodeID != "" && nid != nodeID {
			continue
		}

		// Hash mismatches from GetClusterHealthV1
		if nh, ok := snap.NodeHealths[nid]; ok {
			if d := nh.GetDesiredServicesHash(); d != "" && d != nh.GetAppliedServicesHash() {
				items = append(items, &clusterdoctorpb.DriftItem{
					NodeId:    nid,
					EntityRef: "services",
					Category:  clusterdoctorpb.DriftCategory_STATE_HASH_MISMATCH,
					Desired:   d,
					Actual:    nh.GetAppliedServicesHash(),
					Evidence: []*clusterdoctorpb.Evidence{{
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
				items = append(items, &clusterdoctorpb.DriftItem{
					NodeId:    nid,
					EntityRef: "network",
					Category:  clusterdoctorpb.DriftCategory_STATE_HASH_MISMATCH,
					Desired:   d,
					Actual:    nh.GetAppliedNetworkHash(),
					Evidence: []*clusterdoctorpb.Evidence{{
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
				var cat clusterdoctorpb.DriftCategory
				switch state {
				case rules.UnitStateNotFound:
					cat = clusterdoctorpb.DriftCategory_MISSING_UNIT_FILE
				case rules.UnitStateInactive:
					cat = clusterdoctorpb.DriftCategory_UNIT_STOPPED
				case rules.UnitStateDisabled:
					cat = clusterdoctorpb.DriftCategory_UNIT_DISABLED
				default:
					continue
				}
				items = append(items, &clusterdoctorpb.DriftItem{
					NodeId:    nid,
					EntityRef: u.GetName(),
					Category:  cat,
					Desired:   "active",
					Actual:    u.GetState(),
					Evidence: []*clusterdoctorpb.Evidence{{
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

			// Component version drift vs desired plan
			if plan, ok := snap.NodePlans[nid]; ok && plan.GetSpec() != nil && plan.GetSpec().GetDesired() != nil {
				desiredVersions := make(map[string]string)
				for _, ds := range plan.GetSpec().GetDesired().GetServices() {
					desiredVersions[ds.GetName()] = ds.GetVersion()
				}
				for _, comp := range inv.GetComponents() {
					desired, hasDesired := desiredVersions[comp.GetName()]
					if !hasDesired || desired == "" {
						continue
					}
					if comp.GetVersion() != desired {
						items = append(items, &clusterdoctorpb.DriftItem{
							NodeId:    nid,
							EntityRef: comp.GetName(),
							Category:  clusterdoctorpb.DriftCategory_VERSION_MISMATCH,
							Desired:   desired,
							Actual:    comp.GetVersion(),
							Evidence: []*clusterdoctorpb.Evidence{{
								SourceService: "nodeagent+clustercontroller",
								SourceRpc:     "GetInventory+GetNodePlanV1",
								KeyValues: map[string]string{
									"node_id":          nid,
									"component":        comp.GetName(),
									"installed_version": comp.GetVersion(),
									"desired_version":  desired,
									"plan_id":          plan.GetPlanId(),
								},
								Timestamp: timestamppb.New(snap.GeneratedAt),
							}},
						})
					}
				}
			}

			// Components not installed
			for _, comp := range inv.GetComponents() {
				if !comp.GetInstalled() {
					items = append(items, &clusterdoctorpb.DriftItem{
						NodeId:    nid,
						EntityRef: comp.GetName(),
						Category:  clusterdoctorpb.DriftCategory_INVENTORY_INCOMPLETE,
						Desired:   fmt.Sprintf("%s installed", comp.GetName()),
						Actual:    "not installed",
						Evidence: []*clusterdoctorpb.Evidence{{
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

	return &clusterdoctorpb.DriftReport{
		Header:          buildHeader(snap, version),
		Items:           items,
		TotalDriftCount: uint32(len(items)),
	}
}
