package planner

import (
	"sort"
	"strings"
	"time"

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
	Unit         string
	Op           Operation
	Wait         bool
	WaitDuration time.Duration
	index        int
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
			Unit:         unit,
			Op:           op,
			Wait:         wait,
			WaitDuration: timeoutForUnit(unit),
			index:        idx,
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

var waitDurations = map[string]time.Duration{
	"globular-etcd.service":  60 * time.Second,
	"etcd.service":           60 * time.Second,
	"globular-dns.service":   45 * time.Second,
	"dns.service":            45 * time.Second,
	"globular-minio.service": 40 * time.Second,
	"minio.service":          40 * time.Second,
	"globular-file.service":  30 * time.Second,
	"file.service":           30 * time.Second,
}

func timeoutForUnit(unit string) time.Duration {
	if d, ok := waitDurations[strings.ToLower(unit)]; ok {
		return d
	}
	return 30 * time.Second
}
