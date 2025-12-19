package planner

import (
	"sort"
	"strings"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
)

type Operation string

const (
	OpStart   Operation = "start"
	OpStop    Operation = "stop"
	OpRestart Operation = "restart"
	OpEnable  Operation = "enable"
	OpDisable Operation = "disable"
)

type Action struct {
	Unit  string
	Op    Operation
	Wait  bool
	index int
}

var validOps = map[string]Operation{
	"start":   OpStart,
	"stop":    OpStop,
	"restart": OpRestart,
	"enable":  OpEnable,
	"disable": OpDisable,
}

var priorityUnits = map[string]int{
	"globular-etcd.service":  1,
	"etcd.service":           1,
	"globular-dns.service":   2,
	"dns.service":            2,
	"globular-minio.service": 3,
	"minio.service":          3,
	"globular-file.service":  4,
	"file.service":           4,
	"globular-media.service": 4,
	"media.service":          4,
}

func ComputeActions(plan *clustercontrollerpb.NodePlan) []Action {
	if plan == nil {
		return nil
	}
	var actions []Action
	for idx, ua := range plan.GetUnitActions() {
		opName := strings.ToLower(strings.TrimSpace(ua.GetAction()))
		op, ok := validOps[opName]
		if !ok {
			continue
		}
		unit := strings.TrimSpace(ua.GetUnitName())
		if unit == "" {
			continue
		}
		wait := op == OpStart || op == OpRestart
		actions = append(actions, Action{
			Unit:  unit,
			Op:    op,
			Wait:  wait,
			index: idx,
		})
	}
	sort.SliceStable(actions, func(i, j int) bool {
		pi := priorityUnits[strings.ToLower(actions[i].Unit)]
		pj := priorityUnits[strings.ToLower(actions[j].Unit)]
		if pi != pj {
			return pi < pj
		}
		return actions[i].index < actions[j].index
	})
	return actions
}
