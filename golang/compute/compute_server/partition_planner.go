// partition_planner.go implements deterministic input partitioning for multi-unit
// compute jobs. Given a job's input refs and the definition's PartitionStrategy,
// it produces a ComputePartitionPlan with one Partition per unit.
//
// Phase 2A supports two partition strategies:
//   - "count": split into N partitions (from strategy.unit or job parallelism)
//   - "per_input": one partition per input ref (default for multi-input jobs)
//
// No adaptive partitioning yet — the plan is fully deterministic at submission time.
package main

import (
	"fmt"
	"log/slog"

	"github.com/globulario/services/golang/compute/computepb"
	"github.com/gocql/gocql"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// planPartitions generates a ComputePartitionPlan from the job spec and
// definition. Returns nil if the job should use the single-unit path.
func planPartitions(def *computepb.ComputeDefinition, spec *computepb.ComputeJobSpec) *computepb.ComputePartitionPlan {
	strategy := def.GetPartitionStrategy()
	inputRefs := spec.GetInputRefs()
	parallelism := int(spec.GetDesiredParallelism())

	// Determine partition count.
	var partitions []*computepb.Partition

	strategyType := "single"
	if strategy != nil && strategy.Type != "" {
		strategyType = strategy.Type
	}

	switch strategyType {
	case "per_input":
		// One partition per input ref.
		if len(inputRefs) <= 1 {
			return nil // single-unit path
		}
		for i, ref := range inputRefs {
			partitions = append(partitions, &computepb.Partition{
				PartitionId: fmt.Sprintf("part-%d", i),
				InputRefs:   []*computepb.ObjectRef{ref},
			})
		}

	case "count":
		// Split into N partitions. N comes from strategy.unit or desired_parallelism.
		n := parallelism
		if n == 0 && strategy != nil && strategy.Unit != "" {
			fmt.Sscanf(strategy.Unit, "%d", &n)
		}
		if n <= 1 {
			return nil // single-unit path
		}
		// Distribute inputs across partitions round-robin.
		partitions = make([]*computepb.Partition, n)
		for i := 0; i < n; i++ {
			partitions[i] = &computepb.Partition{
				PartitionId: fmt.Sprintf("part-%d", i),
			}
		}
		for i, ref := range inputRefs {
			idx := i % n
			partitions[idx].InputRefs = append(partitions[idx].InputRefs, ref)
		}

	default:
		// "single" or unrecognized — check if parallelism > 1.
		if parallelism > 1 && len(inputRefs) > 1 {
			// Auto-partition per input when parallelism is requested.
			for i, ref := range inputRefs {
				partitions = append(partitions, &computepb.Partition{
					PartitionId: fmt.Sprintf("part-%d", i),
					InputRefs:   []*computepb.ObjectRef{ref},
				})
			}
		} else {
			return nil // single-unit path
		}
	}

	if len(partitions) <= 1 {
		return nil
	}

	plan := &computepb.ComputePartitionPlan{
		PlanId:              gocql.TimeUUID().String(),
		JobId:               "",
		Partitions:          partitions,
		AggregationRequired: def.GetMergeStrategy() != nil,
		CreatedAt:           timestamppb.Now(),
	}

	slog.Info("compute planner: partition plan created",
		"partitions", len(partitions), "strategy", strategyType)
	return plan
}

// createUnitsFromPlan creates one ComputeUnit per partition.
func createUnitsFromPlan(jobID string, plan *computepb.ComputePartitionPlan) []*computepb.ComputeUnit {
	units := make([]*computepb.ComputeUnit, len(plan.Partitions))
	for i, p := range plan.Partitions {
		units[i] = &computepb.ComputeUnit{
			UnitId:      gocql.TimeUUID().String(),
			JobId:       jobID,
			PartitionId: p.PartitionId,
			State:       computepb.UnitState_UNIT_PENDING,
			InputRefs:   p.InputRefs,
			Attempt:     1,
		}
	}
	return units
}
