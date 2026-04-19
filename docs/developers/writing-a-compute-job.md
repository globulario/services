# Writing a Compute Job

> Build a distributed compute job for the Globular platform — from entrypoint binary to verified result.

This guide walks through building a real compute job: packaging the binary, registering a definition, submitting a job, and reading the result. By the end you'll understand the full lifecycle from code to cluster execution.

---

## The contract

Your compute job is a **native Linux binary** that:

1. Reads inputs from `$COMPUTE_STAGING_PATH/input/`
2. Does its work
3. Writes outputs to `$COMPUTE_STAGING_PATH/output/`
4. Exits 0 on success, non-zero on failure
5. Optionally writes `$COMPUTE_STAGING_PATH/progress.json` for live progress tracking

That's it. No SDK required. Any language, any binary.

---

## Step 1: Write the entrypoint

```bash
#!/bin/bash
# transcode.sh — Transcode all .mp4 files from input/ to output/ at 1080p

set -euo pipefail

INPUT_DIR="${COMPUTE_STAGING_PATH}/input"
OUTPUT_DIR="${COMPUTE_STAGING_PATH}/output"
PROGRESS_FILE="${COMPUTE_STAGING_PATH}/progress.json"

mkdir -p "$OUTPUT_DIR"

files=(${INPUT_DIR}/*.mp4)
total=${#files[@]}
done=0

for f in "${files[@]}"; do
    name=$(basename "$f")
    ffmpeg -i "$f" -vf scale=1920:1080 -c:v libx264 -crf 23 \
        "$OUTPUT_DIR/${name}" -y -loglevel error

    done=$((done + 1))
    pct=$(echo "scale=2; $done / $total" | bc)

    # Write progress (optional but recommended for long jobs)
    cat > "$PROGRESS_FILE" <<EOF
{
  "progress": $pct,
  "message": "Transcoded $done of $total files",
  "items_done": $done,
  "items_total": $total
}
EOF
done

echo "Transcoded $total files to $OUTPUT_DIR"
```

### Key points

- `$COMPUTE_STAGING_PATH` is always set — use it, don't hardcode paths
- `input/` and `output/` are subdirectories the runner creates for you
- Write `progress.json` anytime; the runner picks it up every 5s
- Exit 0 = success. Any non-zero exit = `EXECUTION_NONZERO_EXIT` failure
- stdout and stderr are captured to `stdout.log` and `stderr.log` in the staging directory

---

## Step 2: Package the binary

Compute entrypoints are delivered via the package repository, the same as any other Globular service. The binary is fetched and verified before execution.

Build and publish your package:

```bash
# Build your binary
go build -o dist/my-job ./cmd/my-job
# or for scripts, just include them in the package

# Package it
globular pkg build \
    --name my-job \
    --version 1.0.0 \
    --bin dist/my-job \
    --install-path /usr/local/bin/my-job \
    --out dist/

# Publish to the cluster repository
globular pkg publish \
    --repository globular.internal:443 \
    --file dist/my-job-1.0.0-linux-amd64.tgz
```

The `artifact_uri` for the definition is `repo://core@globular.io/my-job/1.0.0`.

---

## Step 3: Register a compute definition

A definition is the template — register it once, use it for every job.

```go
import (
    computepb "github.com/globulario/services/golang/compute/computepb"
    "google.golang.org/grpc"
)

conn, _ := grpc.Dial("globular.internal:443", mtlsCredentials())
client := computepb.NewComputeServiceClient(conn)

_, err = client.RegisterComputeDefinition(ctx, &computepb.ComputeDefinitionRequest{
    Definition: &computepb.ComputeDefinition{
        Name:        "video-transcode",
        Version:     "1.0.0",
        ArtifactUri: "repo://core@globular.io/ffmpeg-transcoder/1.0.0",
        Entrypoint:  "/usr/local/bin/transcode.sh",
        Runtime:     computepb.RuntimeType_NATIVE_BINARY,

        Resources: &computepb.ResourceProfile{
            MinCpuMillis:   2000,               // 2 cores minimum
            MinMemoryBytes: 1 * 1024 * 1024 * 1024, // 1 GiB
            LocalDiskBytes: 10 * 1024 * 1024 * 1024, // 10 GiB staging
        },

        // Video transcoding is not reproducible (compression parameters vary)
        Determinism: computepb.DeterminismLevel_NON_DETERMINISTIC_BOUNDED,
        Idempotency: computepb.IdempotencyMode_SAFE_RETRY,

        Verification: &computepb.VerificationStrategy{
            Type: computepb.VerificationType_STRUCTURAL_VERIFY,
            // STRUCTURAL: output dir must be non-empty. For deterministic
            // outputs, use CHECKSUM with expected sha256 per output file.
        },

        Placement: &computepb.PlacementRules{
            RequireProfiles: []string{"compute"},
            DefaultPolicy:   computepb.PlacementPolicy_LOWEST_LOAD,
        },

        Kind: computepb.ComputeDefinitionKind_SINGLE_NODE,
    },
})
```

### Definition versioning

Definitions are versioned. If you need to update the entrypoint or resources, register a new version — don't mutate the existing one. In-flight jobs use the version they were submitted with.

---

## Step 4: Upload inputs

Inputs are `ObjectRef` values — content-addressed pointers to objects in MinIO.

```go
// Upload your input file to MinIO
minioClient.PutObject(ctx, "globular-compute", "inputs/video-123.mp4",
    file, fileSize, minio.PutObjectOptions{})

// Compute the SHA256 for the ObjectRef
sha256sum := computeSHA256(file)

inputRef := &computepb.ObjectRef{
    Uri:       "minio://globular-compute/inputs/video-123.mp4",
    Sha256:    sha256sum,
    SizeBytes: uint64(fileSize),
}
```

The compute runner verifies the SHA256 when fetching. If there's a mismatch, the unit fails with `ARTIFACT_FETCH_FAILED`.

---

## Step 5: Submit a job

```go
job, err := client.SubmitComputeJob(ctx, &computepb.ComputeJobSpec{
    DefinitionName:    "video-transcode",
    DefinitionVersion: "1.0.0",
    InputRefs:         []*computepb.ObjectRef{inputRef},
    OutputLocation:    &computepb.ObjectRef{
        Uri: "minio://globular-compute/outputs/video-123/",
    },
    Priority: 5,
    Deadline: timestamppb.New(time.Now().Add(2 * time.Hour)),
})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Job submitted: %s (state: %s)\n", job.JobId, job.State)
```

The job is admitted immediately and `compute.job.submit` workflow starts in the background.

---

## Step 6: Poll for completion

```go
for {
    job, err = client.GetComputeJob(ctx, &computepb.GetComputeJobRequest{
        JobId: job.JobId,
    })
    if err != nil {
        log.Printf("poll error: %v", err)
        time.Sleep(5 * time.Second)
        continue
    }

    fmt.Printf("  state: %s", job.State)
    if job.State == computepb.JobState_JOB_RUNNING {
        // Optional: check unit progress
        units, _ := client.ListComputeUnits(ctx, &computepb.ListComputeUnitsRequest{
            JobId: job.JobId,
        })
        for _, u := range units.Units {
            fmt.Printf("  unit %s: %.0f%%\n", u.UnitId, u.ObservedProgress*100)
        }
    }

    if job.State == computepb.JobState_JOB_COMPLETED ||
       job.State == computepb.JobState_JOB_FAILED {
        break
    }

    time.Sleep(5 * time.Second)
}
```

### Job states

```
PENDING → ADMITTED → QUEUED → RUNNING → AGGREGATING → VERIFYING → COMPLETED
                                                                  → FAILED
                                                                  → DEGRADED  (partial success)
                                                                  → CANCELLED
```

---

## Step 7: Read the result

```go
result, err := client.GetComputeResult(ctx, &computepb.GetComputeResultRequest{
    JobId: job.JobId,
})
if err != nil || result == nil {
    log.Fatal("no result")
}

fmt.Printf("Result URI:    %s\n", result.ResultRef.Uri)
fmt.Printf("Trust level:   %s\n", result.TrustLevel)
fmt.Printf("Completed at:  %s\n", result.CompletedAt.AsTime())

// Download from MinIO
minioClient.GetObject(ctx, "globular-compute", "outputs/video-123/output.mp4", ...)
```

### Trust levels

| Level | Meaning |
|-------|---------|
| `UNVERIFIED` | Output present, no verification run |
| `STRUCTURALLY_VERIFIED` | Output directory is non-empty |
| `CONTENT_VERIFIED` | Output SHA256 matched expected checksums |
| `FULLY_REPRODUCED` | Two independent runs produced identical output |
| `DEGRADED_ACCEPTED` | Some units failed; partial result accepted |

---

## Partitioned batch jobs

For jobs that naturally split across inputs, use `PARTITIONABLE_BATCH`:

```go
// Register a partition-aware definition
Definition: &computepb.ComputeDefinition{
    Name: "image-resize-batch",
    Kind: computepb.ComputeDefinitionKind_PARTITIONABLE_BATCH,

    Partition: &computepb.PartitionStrategy{
        Method: "per_input",  // one unit per input ref
    },
    // ...
},

// Submit with multiple inputs — each becomes a separate unit
job, _ = client.SubmitComputeJob(ctx, &computepb.ComputeJobSpec{
    DefinitionName: "image-resize-batch",
    InputRefs: []*computepb.ObjectRef{
        {Uri: "minio://data/img/001.jpg", Sha256: "..."},
        {Uri: "minio://data/img/002.jpg", Sha256: "..."},
        {Uri: "minio://data/img/003.jpg", Sha256: "..."},
    },
    DesiredParallelism: 3, // run all 3 simultaneously
    OutputLocation: &computepb.ObjectRef{Uri: "minio://data/resized/"},
})
```

Each unit receives one input in its `input/` directory. The aggregate step collects all unit outputs and builds a manifest.

### Partitioned entrypoint

```bash
#!/bin/bash
# resize.sh — called once per partition
# COMPUTE_STAGING_PATH/input/ contains exactly the inputs for this partition

INPUT_DIR="${COMPUTE_STAGING_PATH}/input"
OUTPUT_DIR="${COMPUTE_STAGING_PATH}/output"
mkdir -p "$OUTPUT_DIR"

for f in "${INPUT_DIR}"/*.jpg "${INPUT_DIR}"/*.png; do
    [ -f "$f" ] || continue
    name=$(basename "$f")
    convert "$f" -resize 1920x1080 "$OUTPUT_DIR/$name"
done
```

---

## Error handling patterns

### Idempotent jobs

If your job can safely run twice with the same result, declare it:

```go
Idempotency: computepb.IdempotencyMode_SAFE_RETRY,
Determinism: computepb.DeterminismLevel_DETERMINISTIC,
```

The retry engine will re-run failed units without hesitation. On `DETERMINISTIC` + `SAFE_RETRY`, even `EXECUTION_NONZERO_EXIT` failures are not retried — a deterministic failure is a bug, not a transient.

### Non-reproducible jobs

For ML inference, video encoding, or anything with randomness:

```go
Determinism: computepb.DeterminismLevel_NON_DETERMINISTIC_BOUNDED,
Idempotency: computepb.IdempotencyMode_SAFE_RETRY,
```

Failed units can be retried on a different node. Output verification uses `STRUCTURAL_VERIFY` since checksums won't match across runs.

### Jobs that must not retry

For jobs with side effects (sending emails, charging accounts):

```go
Idempotency: computepb.IdempotencyMode_NO_AUTOMATIC_RETRY,
```

A failed unit stays failed. The job fails. The caller decides what to do.

---

## Validating a definition

Before submitting jobs at scale, validate the definition:

```go
resp, err := client.ValidateComputeDefinition(ctx, &computepb.ValidateComputeDefinitionRequest{
    Name:    "video-transcode",
    Version: "1.0.0",
})
// resp.Valid = true/false
// resp.Errors = list of validation failures
```

Validation checks:
- Artifact URI resolves in the repository
- Entrypoint path is declared
- Resource profile is sane (min ≤ max)
- At least one `compute` node can satisfy the resource constraints

---

## CLI reference

```bash
# Register a definition from a JSON/YAML spec file
globular compute define --file transcode.yaml

# List registered definitions
globular compute definitions list

# Submit a job
globular compute submit \
    --definition video-transcode \
    --version 1.0.0 \
    --input minio://data/video.mp4 \
    --output minio://data/out/

# Get job status
globular compute job get <job_id>

# List jobs (filtered)
globular compute jobs list --state running
globular compute jobs list --definition video-transcode

# Get units for a job
globular compute units list --job <job_id>

# Cancel a job
globular compute job cancel <job_id>

# Get result
globular compute result get <job_id>

# Inspect workflow execution for a job
globular workflow list --correlation compute/<job_id>
```

---

## Deploying the compute service

The compute service (`compute_server`) deploys like any other Globular service:

```bash
# The compute service is packaged and available in the repository.
# Deploy it to all nodes with the compute profile:
globular deploy --service compute

# Or target specific nodes:
globular deploy --service compute --node globule-ryzen --node globule-nuc
```

The service runs on port 10300 and registers in etcd. Nodes with the `compute` profile automatically become eligible for job placement.

### Node profile requirements

```bash
# Add compute profile to a node
globular node profiles set --node <node_id> --add compute

# Verify placement eligibility
globular compute placement check --definition video-transcode
```

---

## See also

- [Computing (operator guide)](../operators/computing.md) — Architecture, placement engine, metrics, failure scenarios
- [Service Packaging](service-packaging.md) — How to build and publish compute binaries
- [Workflow Integration](workflow-integration.md) — How `compute.job.submit` works end-to-end
- [Writing a Microservice](writing-a-microservice.md) — For services that submit compute jobs
