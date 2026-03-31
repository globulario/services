# Node-Agent Actor Adapter Definition

## 1. Role of the adapter

The node-agent actor adapter is the execution bridge between the workflow engine and the existing node-agent runtime.

It translates a compiled workflow step such as:

- actor: `node-agent`
- action: `install_package`
- action: `start_service`
- action: `run_installer_plan`
- action: `write_file`
- action: `verify_health`
- action: `restart_service`

into a concrete node-agent operation on a target node.

The adapter must be thin. It is not allowed to contain orchestration logic.

## 2. Ownership boundaries

### Workflow service owns
- run lifecycle
- step lifecycle
- dependency resolution
- retries
- timeout policy
- concurrency policy
- compensation / rollback policy
- cancellation intent
- final success / failure decision
- event emission for the workflow model

### Node-agent owns
- local execution of an assigned action
- local process management
- package fetch / unpack / install details
- service start / stop / restart details
- invariant checks and local verification
- local logs and progress emission
- local idempotency for command execution when applicable

### Adapter owns
- request translation
- response translation
- correlation field propagation
- callback delivery
- heartbeat forwarding
- mapping node-agent errors into structured step results

## 3. Non-goals

The adapter must not:
- decide the next step
- inspect upstream dependencies
- mark a run complete
- implement release state machines
- queue unrelated work outside the assigned step
- mutate desired cluster state

## 4. Dispatch model

The workflow engine dispatches a step to the adapter with:
- run_id
- step_id
- attempt
- workflow_name
- target node_id
- actor = `node-agent`
- action name
- resolved inputs
- execution deadline
- cancellation token or lease id
- correlation metadata

The adapter turns that into one node-agent call.

## 5. Recommended action surface

Keep the action vocabulary small and stable.

Recommended first set:
- `install_package`
- `uninstall_package`
- `start_service`
- `stop_service`
- `restart_service`
- `run_installer_operation`
- `write_config`
- `verify_package_installed`
- `verify_service_healthy`
- `collect_facts`

Those actions should map to existing node-agent capabilities and installer library calls.

## 6. Execution modes

Two execution modes are acceptable.

### Synchronous short step
Use for quick operations:
- verify installed marker
- read local facts
- verify health endpoint
- write small config file

Flow:
1. workflow-service dispatches request
2. node-agent executes inline
3. node-agent returns terminal result immediately

### Asynchronous long-running step
Use for:
- package install
- heavy reconfiguration
- bootstrap tasks
- service restart with wait loops

Flow:
1. workflow-service dispatches step
2. node-agent acknowledges acceptance and starts local execution
3. node-agent sends callbacks:
   - accepted
   - progress
   - heartbeat
   - terminal result
4. workflow-service updates authoritative step state

## 7. Required invariants

The adapter must preserve:
- `run_id`
- `step_id`
- `attempt`
- `node_id`
- `workflow_name`
- `correlation_id`

Every callback must include those fields.

A terminal callback for a step attempt must be idempotent.
Repeated delivery of the same terminal callback must not corrupt workflow state.

## 8. Suggested local execution model inside node-agent

The node-agent may internally expose:

- action registry
- executor
- local operation journal
- local cancellation registry
- local heartbeats

But those are internal details.

The public contract seen by workflow-service should stay narrow:
- ExecuteStep
- CancelStep
- callback stream or callback RPC

## 9. Failure model

Node-agent must classify failures into stable categories.

Recommended classes:
- `INVALID_INPUT`
- `PRECONDITION_FAILED`
- `NOT_FOUND`
- `PERMISSION_DENIED`
- `DOWNLOAD_FAILED`
- `VERIFY_FAILED`
- `TIMEOUT`
- `CANCELLED`
- `TRANSIENT`
- `INTERNAL`

The adapter converts node-agent failures into structured callback payloads.

## 10. Retry rule

Node-agent may retry tiny local sub-operations internally only when they are strictly local and invisible.

Examples:
- retry reading a local file
- retry querying a local process status

Node-agent must not own workflow retry policy for the assigned step.
If the overall step needs another attempt, workflow-service schedules a new attempt.

## 11. Cancellation rule

Workflow-service is the source of cancellation intent.

Node-agent must:
- accept cancel request for a given `(run_id, step_id, attempt)`
- try best-effort termination
- emit cancellation acknowledgement
- eventually emit terminal result with status `CANCELLED` or `FAILED`

## 12. Golden sentence

The node-agent actor adapter is a translation membrane.
It converts compiled workflow instructions into local node execution and sends back facts about what happened.
