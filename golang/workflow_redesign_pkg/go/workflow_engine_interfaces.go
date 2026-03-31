package workflowengine

import (
    "context"
    "errors"
    "time"
)

// -----------------------------------------------------------------------------
// Core type model
// -----------------------------------------------------------------------------

type DefinitionVersion string

type WorkflowDefinition struct {
    APIVersion string                `json:"apiVersion" yaml:"apiVersion"`
    Kind       string                `json:"kind" yaml:"kind"`
    Metadata   WorkflowMetadata      `json:"metadata" yaml:"metadata"`
    Spec       WorkflowDefinitionSpec `json:"spec" yaml:"spec"`
}

type WorkflowMetadata struct {
    Name        string            `json:"name" yaml:"name"`
    DisplayName string            `json:"displayName,omitempty" yaml:"displayName,omitempty"`
    Description string            `json:"description,omitempty" yaml:"description,omitempty"`
    Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

type WorkflowDefinitionSpec struct {
    InputSchema map[string]any     `json:"inputSchema,omitempty" yaml:"inputSchema,omitempty"`
    Defaults    map[string]any     `json:"defaults,omitempty" yaml:"defaults,omitempty"`
    Strategy    ExecutionStrategy  `json:"strategy" yaml:"strategy"`
    Steps       []WorkflowStepSpec `json:"steps" yaml:"steps"`
    OnFailure   *WorkflowHook      `json:"onFailure,omitempty" yaml:"onFailure,omitempty"`
    OnSuccess   *WorkflowHook      `json:"onSuccess,omitempty" yaml:"onSuccess,omitempty"`
}

type ExecutionStrategy struct {
    Mode        StrategyMode `json:"mode" yaml:"mode"`
    Collection  string       `json:"collection,omitempty" yaml:"collection,omitempty"`
    Concurrency int          `json:"concurrency,omitempty" yaml:"concurrency,omitempty"`
    ItemName    string       `json:"itemName,omitempty" yaml:"itemName,omitempty"`
}

type StrategyMode string

const (
    StrategySingle  StrategyMode = "single"
    StrategyForeach StrategyMode = "foreach"
    StrategyDAG     StrategyMode = "dag"
)

type WorkflowStepSpec struct {
    ID        string         `json:"id" yaml:"id"`
    Title     string         `json:"title,omitempty" yaml:"title,omitempty"`
    Actor     ActorType      `json:"actor" yaml:"actor"`
    Action    string         `json:"action" yaml:"action"`
    DependsOn []string       `json:"dependsOn,omitempty" yaml:"dependsOn,omitempty"`
    When      *StepCondition `json:"when,omitempty" yaml:"when,omitempty"`
    Foreach   string         `json:"foreach,omitempty" yaml:"foreach,omitempty"`
    With      map[string]any `json:"with,omitempty" yaml:"with,omitempty"`
    Retry     *RetryPolicy   `json:"retry,omitempty" yaml:"retry,omitempty"`
    Timeout   string         `json:"timeout,omitempty" yaml:"timeout,omitempty"`
    WaitFor   *WaitPolicy    `json:"waitFor,omitempty" yaml:"waitFor,omitempty"`
    Export    string         `json:"export,omitempty" yaml:"export,omitempty"`
}

type StepCondition struct {
    Expr  string          `json:"expr,omitempty" yaml:"expr,omitempty"`
    AnyOf []StepCondition `json:"anyOf,omitempty" yaml:"anyOf,omitempty"`
    AllOf []StepCondition `json:"allOf,omitempty" yaml:"allOf,omitempty"`
    Not   *StepCondition  `json:"not,omitempty" yaml:"not,omitempty"`
}

type RetryPolicy struct {
    MaxAttempts int    `json:"maxAttempts" yaml:"maxAttempts"`
    Backoff     string `json:"backoff,omitempty" yaml:"backoff,omitempty"`
}

type WaitPolicy struct {
    Condition string `json:"condition" yaml:"condition"`
    Timeout   string `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

type WorkflowHook struct {
    Actor  ActorType      `json:"actor" yaml:"actor"`
    Action string         `json:"action" yaml:"action"`
    With   map[string]any `json:"with,omitempty" yaml:"with,omitempty"`
}

type ActorType string

const (
    ActorWorkflowService   ActorType = "workflow-service"
    ActorClusterController ActorType = "cluster-controller"
    ActorNodeAgent         ActorType = "node-agent"
    ActorInstaller         ActorType = "installer"
    ActorRepository        ActorType = "repository"
    ActorOperator          ActorType = "operator"
)

// -----------------------------------------------------------------------------
// Runtime model
// -----------------------------------------------------------------------------

type RunStatus string

const (
    RunPending   RunStatus = "PENDING"
    RunRunning   RunStatus = "RUNNING"
    RunSucceeded RunStatus = "SUCCEEDED"
    RunFailed    RunStatus = "FAILED"
    RunCanceled  RunStatus = "CANCELED"
    RunTimedOut  RunStatus = "TIMED_OUT"
)

type StepStatus string

const (
    StepPending   StepStatus = "PENDING"
    StepReady     StepStatus = "READY"
    StepRunning   StepStatus = "RUNNING"
    StepSucceeded StepStatus = "SUCCEEDED"
    StepSkipped   StepStatus = "SKIPPED"
    StepFailed    StepStatus = "FAILED"
    StepTimedOut  StepStatus = "TIMED_OUT"
    StepWaiting   StepStatus = "WAITING"
)

type WorkflowRun struct {
    ID             string
    DefinitionName string
    ClusterID      string
    CorrelationID  string
    Inputs         map[string]any
    Outputs        map[string]any
    Context        RunContext
    Status         RunStatus
    StartedAt      time.Time
    UpdatedAt      time.Time
    FinishedAt     *time.Time
}

type RunContext struct {
    NodeID       string
    NodeHostname string
    ReleaseID    string
    ReleaseKind  string
    DesiredHash  string
    PlanIDs      map[string]string
    ActorState   map[string]any
}

type WorkflowStepRun struct {
    RunID       string
    StepID      string
    Title       string
    Actor       ActorType
    Action      string
    Status      StepStatus
    Attempt     int
    StartedAt   *time.Time
    FinishedAt  *time.Time
    Input       map[string]any
    Output      map[string]any
    Error       string
    ExportName  string
    CollectionKey string
}

type ActionRequest struct {
    Run         WorkflowRun
    Step        WorkflowStepRun
    Actor       ActorType
    Action      string
    Input       map[string]any
    Context     map[string]any
}

type ActionResult struct {
    Status      StepStatus
    Output      map[string]any
    Message     string
    Terminal    bool
    Retryable   bool
    NextPollIn  time.Duration
}

// -----------------------------------------------------------------------------
// Engine contracts
// -----------------------------------------------------------------------------

type Engine interface {
    RegisterDefinition(ctx context.Context, def WorkflowDefinition) error
    ValidateDefinition(ctx context.Context, def WorkflowDefinition) error
    StartRun(ctx context.Context, request StartRunRequest) (*WorkflowRun, error)
    ResumeRun(ctx context.Context, runID string) error
    CancelRun(ctx context.Context, runID string, reason string) error
    GetRun(ctx context.Context, runID string) (*WorkflowRun, error)
    ExecuteReadySteps(ctx context.Context, runID string) error
}

type StartRunRequest struct {
    DefinitionName string
    Inputs         map[string]any
    CorrelationID  string
    Context        RunContext
}

type DefinitionRegistry interface {
    Put(ctx context.Context, def WorkflowDefinition) error
    Get(ctx context.Context, name string) (*WorkflowDefinition, error)
    List(ctx context.Context) ([]WorkflowMetadata, error)
}

type StateStore interface {
    CreateRun(ctx context.Context, run *WorkflowRun) error
    UpdateRun(ctx context.Context, run *WorkflowRun) error
    GetRun(ctx context.Context, runID string) (*WorkflowRun, error)

    PutStep(ctx context.Context, step *WorkflowStepRun) error
    UpdateStep(ctx context.Context, step *WorkflowStepRun) error
    ListSteps(ctx context.Context, runID string) ([]*WorkflowStepRun, error)

    AppendEvent(ctx context.Context, evt WorkflowEvent) error
}

type WorkflowEvent struct {
    RunID      string
    StepID     string
    Actor      ActorType
    Type       string
    Message    string
    CreatedAt  time.Time
    Attributes map[string]string
}

type Dispatcher interface {
    Dispatch(ctx context.Context, req ActionRequest) (*ActionResult, error)
}

type ConditionEvaluator interface {
    EvaluateStepCondition(ctx context.Context, cond *StepCondition, run *WorkflowRun, step *WorkflowStepRun) (bool, error)
    EvaluateWaitCondition(ctx context.Context, conditionName string, run *WorkflowRun, step *WorkflowStepRun) (*WaitResult, error)
}

type WaitResult struct {
    Satisfied bool
    Output    map[string]any
    Message   string
}

// -----------------------------------------------------------------------------
// Actor-side contracts
// -----------------------------------------------------------------------------

type Actor interface {
    Type() ActorType
    Supports(action string) bool
    Execute(ctx context.Context, req ActionRequest) (*ActionResult, error)
}

type ActionHandler func(ctx context.Context, req ActionRequest) (*ActionResult, error)

type ActionRouter interface {
    Register(actor ActorType, action string, handler ActionHandler) error
    Resolve(actor ActorType, action string) (ActionHandler, error)
}

// -----------------------------------------------------------------------------
// Cluster-controller bridge
// -----------------------------------------------------------------------------

type ClusterControllerActions interface {
    SetBootstrapPhase(ctx context.Context, nodeID string, phase string) error
    WaitBootstrapCondition(ctx context.Context, nodeID string, condition string) (*WaitResult, error)

    MarkReleaseResolved(ctx context.Context, releaseID string) error
    MarkReleaseApplying(ctx context.Context, releaseID string) error
    MarkReleaseFailed(ctx context.Context, releaseID string, reason string) error
    FinalizeInfrastructureApply(ctx context.Context, releaseID string, aggregate map[string]any) error

    FilterInfrastructureTarget(ctx context.Context, releaseID, nodeID string) (bool, map[string]any, error)
    WaitForPlanSlot(ctx context.Context, nodeID string) (*WaitResult, error)
    CompileInfrastructurePlan(ctx context.Context, req CompilePlanRequest) (*CompiledPlan, error)
    DispatchPlan(ctx context.Context, plan *CompiledPlan) error
    AggregateNodeResults(ctx context.Context, releaseID string) (map[string]any, error)
    ReconcileUntilStable(ctx context.Context, clusterID string) (*WaitResult, error)
    SeedDesiredFromInstalled(ctx context.Context, clusterID string) error
}

type CompilePlanRequest struct {
    ClusterID    string
    ReleaseID    string
    NodeID       string
    PackageName  string
    Version      string
    DesiredHash  string
}

type CompiledPlan struct {
    PlanID      string
    NodeID      string
    Generation  int64
    DesiredHash string
    Raw         []byte
    Metadata    map[string]string
}

// -----------------------------------------------------------------------------
// Node-agent bridge
// -----------------------------------------------------------------------------

type NodeAgentActions interface {
    ExecutePlan(ctx context.Context, nodeID string, planID string) (*ActionResult, error)
    WaitForPlanTerminal(ctx context.Context, nodeID string, planID string) (*WaitResult, error)
}

type NodePlanCommandBus interface {
    SubmitPlan(ctx context.Context, plan *CompiledPlan) error
    GetPlanStatus(ctx context.Context, nodeID, planID string) (*PlanTerminalStatus, error)
}

type PlanTerminalStatus struct {
    NodeID     string
    PlanID     string
    Terminal   bool
    Succeeded  bool
    RolledBack bool
    Failed     bool
    Message    string
    Output     map[string]any
}

// -----------------------------------------------------------------------------
// Installer bridge
// -----------------------------------------------------------------------------

type InstallerActions interface {
    SetupTLS(ctx context.Context, clusterID string) error
    EnableBootstrapWindow(ctx context.Context, ttl time.Duration) error
    DisableBootstrapWindow(ctx context.Context) error
    WriteBootstrapCredentials(ctx context.Context) error
    InstallPackage(ctx context.Context, packageName string) error
    InstallPackageSet(ctx context.Context, packages []string) error
    InstallProfileSets(ctx context.Context, profiles []string) error
    ConfigureSharedStorage(ctx context.Context) error
    BootstrapDNS(ctx context.Context, domain string) error
    GenerateJoinToken(ctx context.Context) (string, error)
    RestartServices(ctx context.Context, services []string) error
    ClusterBootstrap(ctx context.Context, clusterID string, nodeID string) error
    ValidateClusterHealth(ctx context.Context) (*WaitResult, error)
    CaptureBootstrapFailureBundle(ctx context.Context, runID string) error
}

// -----------------------------------------------------------------------------
// Repository bridge
// -----------------------------------------------------------------------------

type RepositoryActions interface {
    PublishBootstrapArtifacts(ctx context.Context, source string) error
}

// -----------------------------------------------------------------------------
// Validation helpers
// -----------------------------------------------------------------------------

var (
    ErrDefinitionNotFound = errors.New("workflow definition not found")
    ErrActionNotSupported = errors.New("workflow action not supported")
    ErrRunNotFound        = errors.New("workflow run not found")
)
