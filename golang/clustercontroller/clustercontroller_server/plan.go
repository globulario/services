package main

import (
	"strings"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
)

var profileUnitMap = map[string][]string{
	"core": {
		"globular-etcd.service",
		"globular-dns.service",
		"globular-discovery.service",
		"globular-event.service",
		"globular-rbac.service",
		"globular-file.service",
		"globular-minio.service",
	},
	"control-plane": {
		"globular-etcd.service",
		"globular-dns.service",
		"globular-discovery.service",
	},
	"gateway": {
		"globular-gateway.service",
		"envoy.service",
	},
	"storage": {
		"globular-minio.service",
		"globular-file.service",
	},
}

func buildPlanActions(profiles []string) []*clustercontrollerpb.UnitAction {
	if len(profiles) == 0 {
		profiles = []string{"core"}
	}
	seen := make(map[string]struct{})
	var actions []*clustercontrollerpb.UnitAction
	for _, profile := range profiles {
		for _, action := range actionsForProfile(profile) {
			key := action.UnitName + ":" + action.Action
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			actions = append(actions, action)
		}
	}
	return actions
}

func actionsForProfile(profile string) []*clustercontrollerpb.UnitAction {
	normalized := strings.ToLower(strings.TrimSpace(profile))
	units, ok := profileUnitMap[normalized]
	if !ok {
		units = profileUnitMap["core"]
	}
	result := make([]*clustercontrollerpb.UnitAction, 0, len(units)*2)
	for _, unit := range units {
		result = append(result, &clustercontrollerpb.UnitAction{
			UnitName: unit,
			Action:   "enable",
		})
		result = append(result, &clustercontrollerpb.UnitAction{
			UnitName: unit,
			Action:   "start",
		})
	}
	return result
}
