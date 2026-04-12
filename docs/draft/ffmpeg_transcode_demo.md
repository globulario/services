# Demo: ffmpeg Transcode

## Prerequisites

- Compute service running on at least one node
- ffmpeg installed at `/usr/local/bin/ffmpeg`
- Script `/usr/local/bin/compute-transcode.sh` deployed to all compute nodes

## Register the definition

```bash
# Via gRPC (through gateway or direct)
grpc_call compute.ComputeService RegisterComputeDefinition '{
  "definition": {
    "name": "media-transcode",
    "version": "1.0.0",
    "entrypoint": "/usr/local/bin/compute-transcode.sh",
    "runtime_type": 1,
    "kind": 1,
    "determinism_level": 1,
    "idempotency_mode": 1,
    "verify_strategy": {"type": 2}
  }
}'
```

## Submit a job

### Without input (generates test tone)

```bash
grpc_call compute.ComputeService SubmitComputeJob '{
  "spec": {
    "definition_name": "media-transcode",
    "definition_version": "1.0.0",
    "tags": ["demo"]
  }
}'
# Returns: job_id
```

### With input (transcode a file from MinIO)

Upload a media file to MinIO first, then:

```bash
grpc_call compute.ComputeService SubmitComputeJob '{
  "spec": {
    "definition_name": "media-transcode",
    "definition_version": "1.0.0",
    "input_refs": [
      {"uri": "minio://globular/path/to/video.mp4"}
    ]
  }
}'
```

## Watch progress

```bash
# Check job state
grpc_call compute.ComputeService GetComputeJob '{"job_id": "<JOB_ID>"}'

# Check unit state (execution details)
grpc_call compute.ComputeService ListComputeUnits '{"job_id": "<JOB_ID>"}'
```

States to watch:
- `JOB_ADMITTED` → job accepted, workflow starting
- `JOB_RUNNING` → unit dispatched and executing
- `JOB_COMPLETED` → done
- `JOB_FAILED` → check unit failure_reason

## Inspect result

```bash
grpc_call compute.ComputeService GetComputeResult '{"job_id": "<JOB_ID>"}'
```

### Expected output (no input)

```json
{
  "resultRef": {
    "uri": "minio://globular/globular.internal/compute/outputs/<job_id>/<unit_id>/test-tone.mp3",
    "sha256": "<hex>",
    "sizeBytes": "81127"
  },
  "trustLevel": "STRUCTURALLY_VERIFIED"
}
```

### Expected output (with video input)

```json
{
  "resultRef": {
    "uri": "minio://globular/.../video_720p.mp4",
    "sha256": "<hex>",
    "sizeBytes": "<varies>"
  },
  "trustLevel": "STRUCTURALLY_VERIFIED"
}
```

## Failure signals

| Signal | Meaning |
|--------|---------|
| `JOB_FAILED` + `ARTIFACT_FETCH_FAILED` | Input file not found in MinIO |
| `JOB_FAILED` + `EXECUTION_NONZERO_EXIT` | ffmpeg failed (bad format, missing codec) |
| `JOB_RUNNING` for >60s | Check compute service logs on the runner node |
| Unit `nodeId` is hostname (not IP:port) | Old binary — redeploy compute service |

## What the script does

1. Looks for media files in `$COMPUTE_STAGING_PATH/input/`
2. If found: transcodes video to 720p MP4 (h264+aac) or audio to 192k MP3
3. If not found: generates a 5-second 440Hz test tone as MP3
4. Writes output to `$COMPUTE_STAGING_PATH/output/`
5. Runner uploads output to MinIO automatically
