# Compute Scheduling Behavior

## Placement pipeline

Every unit placement goes through this pipeline:

```
1. Discover compute service instances (etcd)
2. Enrich with node profiles + capabilities (controller ListNodes)
3. Hard filter: allowed_node_profiles (strict, no fallback)
4. Hard filter: minimum CPU/memory/disk (from ResourceProfile)
5. Default filter: prefer nodes with "compute" profile
6. Score eligible nodes: capacity × priority / (1 + load)
7. Tie-break: round-robin
```

## Scoring formula

```
score = (0.4×cpu_norm + 0.4×ram_norm + 0.2×disk_norm) × priority_boost / (1 + active_units)
```

- `cpu_norm`, `ram_norm`, `disk_norm`: normalized to [0,1] against max in candidate set
- `priority_boost`: job_priority / 5 (normal=1.0x, critical=2.0x, low=0.2x)
- `active_units`: count of PENDING/ASSIGNED/STAGING/RUNNING units on that node

## Priority classes

| Priority | Value | Boost | Behavior |
|----------|-------|-------|----------|
| low | 1 | 0.2x | Strongly prefers idle nodes |
| normal | 5 | 1.0x | Default, balanced spread |
| high | 8 | 1.6x | Tolerates moderate node load |
| critical | 10 | 2.0x | Gets access to busiest nodes |

Set via `ComputeJobSpec.priority` (string). Default: "normal".

## Profile filtering

- `ComputeDefinition.allowed_node_profiles`: strict filter, no fallback
- Default: prefers nodes with "compute" profile (soft filter)
- No match + strict filter → `placement failed` error
- Profiles assigned via `globular cluster nodes profiles set`

## Resource filtering

`ComputeDefinition.resource_profile` fields checked:

| Field | Node field | Check |
|-------|-----------|-------|
| min_cpu_millis | CPUCount × 1000 | Node must have at least this many millicores |
| min_memory_bytes | RAMBytes | Node must have at least this much RAM |
| local_disk_bytes | DiskFreeBytes | Node must have at least this much free disk |

## Load tracking

- `activeUnitsPerNode()` scans all compute jobs in etcd
- Counts units in PENDING, ASSIGNED, STAGING, RUNNING states
- Keyed by node address (unique per node)
- Within a batch dispatch, local counter incremented per unit to spread evenly

## Deadline enforcement

- `ComputeJobSpec.deadline`: hard timestamp cutoff
- Pre-dispatch: past deadlines → no units dispatched
- Await loop: exceeded deadline → cancel all running units
- Job transitions to JOB_FAILED with "job deadline exceeded"
- Late unit success cannot override timeout

## Batch dispatch behavior

When `computeDispatchAllUnits` dispatches N units:

1. Load map queried once from etcd
2. For each unit: `placeUnit()` → score → pick best → increment local load counter
3. Each unit gets a different node (if enough eligible nodes exist)
4. If all nodes are equal, spread is round-robin within the batch

## Retry placement

Failed units re-enter the full placement pipeline:
- Fresh load map from etcd
- Fresh scoring with current priority
- May land on a different node than the original attempt
- Same hard filters apply
