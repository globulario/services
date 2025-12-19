package main

import (
	"sort"
	"strings"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
)

var coreUnits = []string{
	"globular-etcd.service",
	"globular-dns.service",
	"globular-discovery.service",
	"globular-event.service",
	"globular-rbac.service",
	"globular-file.service",
	"globular-minio.service",
}

var profileUnitMap = map[string][]string{
	"core":    coreUnits,
	"compute": coreUnits,
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
	sort.SliceStable(actions, func(i, j int) bool {
		pi := unitPriority[strings.ToLower(actions[i].UnitName)]
		pj := unitPriority[strings.ToLower(actions[j].UnitName)]
		if pi != pj {
			return pi < pj
		}
		orderI := actionOrder[strings.ToLower(actions[i].Action)]
		orderJ := actionOrder[strings.ToLower(actions[j].Action)]
		return orderI < orderJ
	})
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

var unitPriority = map[string]int{
	"globular-etcd.service":      1,
	"etcd.service":               1,
	"globular-dns.service":       2,
	"dns.service":                2,
	"globular-discovery.service": 3,
	"discovery.service":          3,
	"globular-event.service":     4,
	"event.service":              4,
	"globular-rbac.service":      5,
	"rbac.service":               5,
	"globular-minio.service":     6,
	"minio.service":              6,
	"globular-file.service":      7,
	"file.service":               7,
	"globular-gateway.service":   8,
	"envoy.service":              9,
}

var actionOrder = map[string]int{
	"enable": 0,
	"start":  1,
}
