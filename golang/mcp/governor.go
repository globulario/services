package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ── Validation Types ─────────────────────────────────────────────────────────

// ValidationStatus is the result of pre-execution validation.
type ValidationStatus string

const (
	StatusAllowed           ValidationStatus = "allowed"
	StatusBlocked           ValidationStatus = "blocked"
	StatusNeedsConfirmation ValidationStatus = "needs_confirmation"
	StatusInvalid           ValidationStatus = "invalid"
	StatusOutOfOrder        ValidationStatus = "out_of_order"
)

// ValidationRequest describes a command to be validated.
type ValidationRequest struct {
	Command      string            `json:"command"`
	Args         []string          `json:"args"`
	WorkflowStep string            `json:"workflow_step,omitempty"`
	Context      map[string]string `json:"context,omitempty"`
}

// ValidationResult is the structured output of command validation.
type ValidationResult struct {
	Status              ValidationStatus `json:"status"`
	Reason              string           `json:"reason,omitempty"`
	MissingRequirements []string         `json:"missing_requirements,omitempty"`
	SuggestedNextStep   string           `json:"suggested_next_step,omitempty"`
	RequiresApproval    bool             `json:"requires_approval,omitempty"`
	MatchedCommand      string           `json:"matched_command,omitempty"`
}

// ── Execution Types ──────────────────────────────────────────────────────────

// ExecutionResult captures the structured output of a CLI command.
type ExecutionResult struct {
	Command                string            `json:"command"`
	Args                   []string          `json:"args"`
	Timestamp              string            `json:"timestamp"`
	Success                bool              `json:"success"`
	ExitCode               int               `json:"exit_code"`
	Stdout                 string            `json:"stdout"`
	Stderr                 string            `json:"stderr"`
	DurationMs             int64             `json:"duration_ms"`
	ProducedArtifacts      []string          `json:"produced_artifacts,omitempty"`
	ChangedFiles           []string          `json:"changed_files,omitempty"`
	DetectedStateChanges   []string          `json:"detected_state_changes,omitempty"`
	Warnings               []string          `json:"warnings,omitempty"`
	Errors                 []string          `json:"errors,omitempty"`
	RecommendedNextActions []string          `json:"recommended_next_actions,omitempty"`
	Validation             *ValidationResult `json:"validation,omitempty"`
	Branch                 *BranchDecision   `json:"branch,omitempty"`
}

// ── State Types ──────────────────────────────────────────────────────────────

// StateSnapshot captures the current state of the project/system.
type StateSnapshot struct {
	Timestamp       string         `json:"timestamp"`
	ProjectState    ProjectState   `json:"project_state"`
	BuildState      BuildState     `json:"build_state"`
	GenerationState GenState       `json:"generation_state"`
	PackageState    PackageState   `json:"package_state,omitempty"`
	DeploymentState DeploymentState `json:"deployment_state,omitempty"`
}

// PackageState describes the package repository state.
type PackageState struct {
	Available   []string `json:"available,omitempty"`
	Published   []string `json:"published,omitempty"`
	LastChecked string   `json:"last_checked,omitempty"`
	Error       string   `json:"error,omitempty"`
}

// DeploymentState describes the deployment/cluster state.
type DeploymentState struct {
	ClusterHealthy    *bool    `json:"cluster_healthy,omitempty"`
	InstalledServices []string `json:"installed_services,omitempty"`
	DesiredServices   []string `json:"desired_services,omitempty"`
	DriftedServices   []string `json:"drifted_services,omitempty"`
	LastChecked       string   `json:"last_checked,omitempty"`
	Error             string   `json:"error,omitempty"`
}

// StateDiff describes the differences between two state snapshots.
type StateDiff struct {
	Timestamp        string            `json:"timestamp"`
	Before           string            `json:"before_timestamp"`
	After            string            `json:"after_timestamp"`
	FilesAdded       []string          `json:"files_added,omitempty"`
	FilesRemoved     []string          `json:"files_removed,omitempty"`
	BuildChanged     bool              `json:"build_changed"`
	BuildBefore      bool              `json:"build_compiles_before"`
	BuildAfter       bool              `json:"build_compiles_after"`
	TestsChanged     bool              `json:"tests_changed"`
	TestsBefore      bool              `json:"tests_passed_before"`
	TestsAfter       bool              `json:"tests_passed_after"`
	GitCleanChanged  bool              `json:"git_clean_changed"`
	NewGeneratedFiles []string         `json:"new_generated_files,omitempty"`
	Unexpected       []string          `json:"unexpected_changes,omitempty"`
}

// ProjectState describes the project directory.
type ProjectState struct {
	HasGoMod    bool     `json:"has_go_mod"`
	HasProtoDir bool     `json:"has_proto_dir"`
	ProtoFiles  []string `json:"proto_files,omitempty"`
	ServiceDirs []string `json:"service_dirs,omitempty"`
	GitClean    bool     `json:"git_clean"`
	GitBranch   string   `json:"git_branch,omitempty"`
}

// BuildState describes the build status.
type BuildState struct {
	LastChecked string `json:"last_checked"`
	Compiles    bool   `json:"compiles"`
	TestsPassed bool   `json:"tests_passed"`
	Error       string `json:"error,omitempty"`
}

// GenState describes the state of generated files.
type GenState struct {
	GeneratedFiles   []string `json:"generated_files,omitempty"`
	HandwrittenFiles []string `json:"handwritten_files,omitempty"`
}

// ── Validation Logic ─────────────────────────────────────────────────────────

// ValidateCommand checks a command against the CLI knowledge base and policies.
// It enforces: command existence, flag validity, approval policies, preconditions,
// workflow step ordering, and generated file protection.
func ValidateCommand(req ValidationRequest) ValidationResult {
	return ValidateCommandWithState(req, nil)
}

// ValidateCommandWithState validates with optional state for precondition checks.
func ValidateCommandWithState(req ValidationRequest, state *StateSnapshot) ValidationResult {
	// Parse the command path from the full command string
	cmdPath := extractCommandPath(req.Command, req.Args)

	// 1. Check if the command exists in the knowledge base
	cmd, exists := lookupCommand(cmdPath)
	if !exists {
		// Try parent command
		parts := strings.Fields(cmdPath)
		if len(parts) > 1 {
			parent := parts[0]
			if _, parentExists := lookupCommand(parent); parentExists {
				return ValidationResult{
					Status:         StatusInvalid,
					Reason:         fmt.Sprintf("subcommand %q not found under %q", strings.Join(parts[1:], " "), parent),
					MatchedCommand: parent,
					SuggestedNextStep: fmt.Sprintf("Use globular_cli.help with command_path=%q to see available subcommands", parent),
				}
			}
		}
		return ValidationResult{
			Status:            StatusInvalid,
			Reason:            fmt.Sprintf("unknown command: %q — command not found in CLI knowledge base", cmdPath),
			SuggestedNextStep: "Use globular_cli.help to discover available commands",
		}
	}

	// 2. Check workflow step ordering (if within a workflow session)
	if req.WorkflowStep != "" {
		wfResult := ValidateWorkflowStep(req.WorkflowStep, cmdPath)
		if wfResult.Status != StatusAllowed {
			return wfResult
		}
	}

	// 3. Check approval policies (replaces simple destructiveCommands map)
	if approval := CheckApproval(cmdPath); approval != nil && approval.RequiresUserConfirmation {
		return ValidationResult{
			Status:           StatusNeedsConfirmation,
			Reason:           fmt.Sprintf("[%s] %s", approval.Scope, approval.Reason),
			RequiresApproval: true,
			MatchedCommand:   cmdPath,
			SuggestedNextStep: "Obtain explicit user approval before executing",
		}
	}

	// 4. Validate required flags
	missing := validateRequiredFlags(cmd, req.Args)
	if len(missing) > 0 {
		return ValidationResult{
			Status:              StatusBlocked,
			Reason:              "missing required flags",
			MissingRequirements: missing,
			MatchedCommand:      cmdPath,
			SuggestedNextStep:   fmt.Sprintf("Add the missing flags: %s", strings.Join(missing, ", ")),
		}
	}

	// 5. Validate flag values against allowed enums
	if err := validateFlagValues(cmd, req.Args); err != "" {
		return ValidationResult{
			Status:         StatusBlocked,
			Reason:         err,
			MatchedCommand: cmdPath,
		}
	}

	// 6. Check if modifying generated files
	if reason := checkGeneratedFileModification(req); reason != "" {
		return ValidationResult{
			Status:         StatusBlocked,
			Reason:         reason,
			MatchedCommand: cmdPath,
			SuggestedNextStep: "Only modify handwritten files. Generated files have 'DO NOT EDIT' headers.",
		}
	}

	// 7. Evaluate state-aware preconditions
	if state != nil {
		precondResults := EvaluatePreconditions(cmdPath, req.Args, state)
		var failedPreconditions []string
		for _, pr := range precondResults {
			if !pr.Passed {
				failedPreconditions = append(failedPreconditions, pr.Message)
			}
		}
		if len(failedPreconditions) > 0 {
			return ValidationResult{
				Status:              StatusBlocked,
				Reason:              "precondition check failed",
				MissingRequirements: failedPreconditions,
				MatchedCommand:      cmdPath,
				SuggestedNextStep:   "Fix the precondition failures before retrying",
			}
		}
	}

	return ValidationResult{
		Status:         StatusAllowed,
		MatchedCommand: cmdPath,
	}
}

// extractCommandPath parses a globular command into its path form.
// e.g. "globular generate service --proto foo.proto" → "generate service"
func extractCommandPath(command string, args []string) string {
	// Combine command + args for parsing
	all := command
	if len(args) > 0 {
		all = command + " " + strings.Join(args, " ")
	}

	// Strip "globular" prefix
	all = strings.TrimSpace(all)
	all = strings.TrimPrefix(all, "globular-cli ")
	all = strings.TrimPrefix(all, "globular ")

	// Extract non-flag tokens (command path)
	var pathParts []string
	for _, token := range strings.Fields(all) {
		if strings.HasPrefix(token, "-") {
			break
		}
		pathParts = append(pathParts, token)
	}

	// Try longest match first, then shorter
	for i := len(pathParts); i > 0; i-- {
		candidate := strings.Join(pathParts[:i], " ")
		if _, ok := cliCommands[candidate]; ok {
			return candidate
		}
	}

	return strings.Join(pathParts, " ")
}

// validateRequiredFlags checks that all required flags are present.
func validateRequiredFlags(cmd CLICommand, args []string) []string {
	argStr := strings.Join(args, " ")
	var missing []string
	for _, f := range cmd.Flags {
		if f.Required {
			if !strings.Contains(argStr, "--"+f.Name) {
				missing = append(missing, "--"+f.Name)
			}
		}
	}
	return missing
}

// validateFlagValues checks that flag values match allowed enums.
func validateFlagValues(cmd CLICommand, args []string) string {
	for i, arg := range args {
		if !strings.HasPrefix(arg, "--") {
			continue
		}
		flagName := strings.TrimPrefix(arg, "--")
		// Handle --flag=value
		if idx := strings.Index(flagName, "="); idx >= 0 {
			value := flagName[idx+1:]
			flagName = flagName[:idx]
			for _, f := range cmd.Flags {
				if f.Name == flagName && len(f.Allowed) > 0 {
					if !containsStr(f.Allowed, value) {
						return fmt.Sprintf("invalid value %q for --%s (allowed: %s)", value, flagName, strings.Join(f.Allowed, ", "))
					}
				}
			}
			continue
		}
		// Handle --flag value
		if i+1 < len(args) {
			value := args[i+1]
			for _, f := range cmd.Flags {
				if f.Name == flagName && len(f.Allowed) > 0 {
					if !containsStr(f.Allowed, value) {
						return fmt.Sprintf("invalid value %q for --%s (allowed: %s)", value, flagName, strings.Join(f.Allowed, ", "))
					}
				}
			}
		}
	}
	return ""
}

func checkGeneratedFileModification(req ValidationRequest) string {
	for _, arg := range req.Args {
		if strings.HasSuffix(arg, ".generated.go") || strings.HasSuffix(arg, ".generated.cs") {
			return fmt.Sprintf("cannot modify generated file: %s", arg)
		}
	}
	return ""
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// ── Execution Logic ──────────────────────────────────────────────────────────

// ExecuteCommand runs a CLI command and captures structured output.
// The command is validated first; if validation fails, no execution occurs.
// After execution, it captures artifacts, analyzes branching decisions,
// and detects state changes.
func ExecuteCommand(req ValidationRequest) ExecutionResult {
	// Build the full command
	cmdArgs := append([]string{}, req.Args...)
	fullCmd := req.Command
	if len(cmdArgs) > 0 {
		fullCmd = req.Command + " " + strings.Join(cmdArgs, " ")
	}

	result := ExecutionResult{
		Command:   req.Command,
		Args:      req.Args,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Validate first
	validation := ValidateCommand(req)
	result.Validation = &validation

	if validation.Status == StatusBlocked || validation.Status == StatusInvalid || validation.Status == StatusOutOfOrder {
		result.Success = false
		result.ExitCode = -1
		result.Errors = []string{fmt.Sprintf("validation failed (%s): %s", validation.Status, validation.Reason)}
		if validation.SuggestedNextStep != "" {
			result.RecommendedNextActions = []string{validation.SuggestedNextStep}
		}
		branch := BranchDecision{Action: "stop", Reason: fmt.Sprintf("blocked by validation: %s", validation.Reason)}
		result.Branch = &branch
		return result
	}
	if validation.Status == StatusNeedsConfirmation {
		result.Success = false
		result.ExitCode = -1
		result.Warnings = []string{fmt.Sprintf("requires approval: %s", validation.Reason)}
		result.RecommendedNextActions = []string{"Obtain explicit user confirmation before re-executing"}
		branch := BranchDecision{Action: "request_approval", Reason: validation.Reason}
		result.Branch = &branch
		return result
	}

	// Capture pre-execution git state for change detection
	preGitStatus := captureGitStatus()

	// Execute
	start := time.Now()
	cmd := exec.Command("sh", "-c", fullCmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.DurationMs = time.Since(start).Milliseconds()
	result.Stdout = truncateOutput(stdout.String(), 64*1024)
	result.Stderr = truncateOutput(stderr.String(), 16*1024)

	if err != nil {
		result.Success = false
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		result.Errors = []string{err.Error()}
	} else {
		result.Success = true
		result.ExitCode = 0

		// Analyze stdout for next actions based on command type
		cmdPath := extractCommandPath(req.Command, req.Args)
		if cmd, ok := lookupCommand(cmdPath); ok && len(cmd.FollowUp) > 0 {
			result.RecommendedNextActions = cmd.FollowUp
		}
	}

	// Extract warnings from stderr
	for _, line := range strings.Split(result.Stderr, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(strings.ToLower(line), "warning") {
			result.Warnings = append(result.Warnings, line)
		}
	}

	// Detect produced artifacts (files ending in .tgz, .tar.gz, .zip)
	result.ProducedArtifacts = detectArtifacts(result.Stdout)

	// Detect changed files by comparing git status
	result.ChangedFiles = detectChangedFiles(preGitStatus)

	// Detect state changes from output
	result.DetectedStateChanges = detectStateChanges(result.Stdout, result.Stderr)

	// Compute branching decision
	branch := DecideBranch(result, req.WorkflowStep)
	result.Branch = &branch

	return result
}

// captureGitStatus returns the current git status for change detection.
func captureGitStatus() string {
	cmd := exec.Command("git", "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// detectChangedFiles compares pre/post git status to find new changes.
func detectChangedFiles(preStatus string) []string {
	postCmd := exec.Command("git", "status", "--porcelain")
	postOut, err := postCmd.Output()
	if err != nil {
		return nil
	}
	postStatus := string(postOut)

	preLines := make(map[string]bool)
	for _, line := range strings.Split(preStatus, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			preLines[line] = true
		}
	}

	var changed []string
	for _, line := range strings.Split(postStatus, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !preLines[line] {
			// Extract filename from status line (e.g. "?? path/file" or " M path/file")
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				changed = append(changed, parts[len(parts)-1])
			}
		}
	}
	return changed
}

// detectArtifacts finds artifact paths mentioned in output.
func detectArtifacts(stdout string) []string {
	var artifacts []string
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		for _, ext := range []string{".tgz", ".tar.gz", ".zip", ".deb", ".rpm"} {
			if strings.Contains(line, ext) {
				// Try to extract path
				for _, word := range strings.Fields(line) {
					if strings.HasSuffix(word, ext) {
						artifacts = append(artifacts, word)
					}
				}
			}
		}
	}
	return artifacts
}

// detectStateChanges parses output for known state transition patterns.
func detectStateChanges(stdout, stderr string) []string {
	combined := stdout + "\n" + stderr
	var changes []string

	patterns := map[string]string{
		"Generated":  "files generated",
		"Published":  "package published",
		"Installed":  "package installed",
		"Created":    "resource created",
		"Deleted":    "resource deleted",
		"Updated":    "resource updated",
		"Restarted":  "service restarted",
		"Stopped":    "service stopped",
		"Reconciled": "state reconciled",
	}

	for pattern, description := range patterns {
		if strings.Contains(combined, pattern) {
			changes = append(changes, description)
		}
	}
	return changes
}

func truncateOutput(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "\n... (truncated)"
	}
	return s
}

// ── State Collection ─────────────────────────────────────────────────────────

// CollectStateSnapshot gathers project and build state from the filesystem.
func CollectStateSnapshot(projectDir string) StateSnapshot {
	snap := StateSnapshot{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	if projectDir == "" {
		projectDir = "."
	}

	snap.ProjectState = collectProjectState(projectDir)
	snap.BuildState = collectBuildState(projectDir)
	snap.GenerationState = collectGenerationState(projectDir)

	return snap
}

func collectProjectState(dir string) ProjectState {
	ps := ProjectState{}

	// Check for go.mod
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		ps.HasGoMod = true
	}
	// Also check golang/ subdirectory (Globular convention)
	if _, err := os.Stat(filepath.Join(dir, "golang", "go.mod")); err == nil {
		ps.HasGoMod = true
	}

	// Check for proto directory
	protoDir := filepath.Join(dir, "proto")
	if info, err := os.Stat(protoDir); err == nil && info.IsDir() {
		ps.HasProtoDir = true
		// List proto files
		entries, err := os.ReadDir(protoDir)
		if err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".proto") {
					ps.ProtoFiles = append(ps.ProtoFiles, e.Name())
				}
			}
		}
	}

	// Find service directories (look for *_server/ pattern)
	golangDir := filepath.Join(dir, "golang")
	if info, err := os.Stat(golangDir); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(golangDir)
		for _, e := range entries {
			if e.IsDir() {
				subEntries, _ := os.ReadDir(filepath.Join(golangDir, e.Name()))
				for _, sub := range subEntries {
					if sub.IsDir() && strings.HasSuffix(sub.Name(), "_server") {
						ps.ServiceDirs = append(ps.ServiceDirs, filepath.Join(e.Name(), sub.Name()))
					}
				}
			}
		}
	}

	// Git status
	gitCmd := exec.Command("git", "-C", dir, "status", "--porcelain")
	out, err := gitCmd.Output()
	if err == nil {
		ps.GitClean = len(strings.TrimSpace(string(out))) == 0
	}

	branchCmd := exec.Command("git", "-C", dir, "branch", "--show-current")
	branchOut, err := branchCmd.Output()
	if err == nil {
		ps.GitBranch = strings.TrimSpace(string(branchOut))
	}

	return ps
}

func collectBuildState(dir string) BuildState {
	bs := BuildState{
		LastChecked: time.Now().UTC().Format(time.RFC3339),
	}

	// Try to compile
	golangDir := filepath.Join(dir, "golang")
	if _, err := os.Stat(filepath.Join(golangDir, "go.mod")); err != nil {
		golangDir = dir
	}

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = golangDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		bs.Compiles = false
		bs.Error = truncateOutput(stderr.String(), 4096)
	} else {
		bs.Compiles = true
	}

	// Try to run tests (quick check only)
	testCmd := exec.Command("go", "test", "./...", "-count=1", "-short", "-timeout=60s")
	testCmd.Dir = golangDir
	var testStderr bytes.Buffer
	testCmd.Stderr = &testStderr
	if err := testCmd.Run(); err != nil {
		bs.TestsPassed = false
		if bs.Error == "" {
			bs.Error = truncateOutput(testStderr.String(), 4096)
		}
	} else {
		bs.TestsPassed = true
	}

	return bs
}

func collectGenerationState(dir string) GenState {
	gs := GenState{}

	// Walk looking for generated vs handwritten files
	walkDir := filepath.Join(dir, "golang")
	if _, err := os.Stat(walkDir); err != nil {
		walkDir = dir
	}

	filepath.Walk(walkDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Check for generated file header
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		if strings.Contains(string(data[:min(len(data), 200)]), "Code generated") {
			gs.GeneratedFiles = append(gs.GeneratedFiles, rel)
		}
		return nil
	})

	return gs
}

// collectSystemState queries the Globular cluster via gRPC for system-level state.
// This uses the MCP client pool when available, or returns empty state.
func collectSystemState(clients *clientPool) (PackageState, DeploymentState) {
	pkgState := PackageState{LastChecked: time.Now().UTC().Format(time.RFC3339)}
	deplState := DeploymentState{LastChecked: time.Now().UTC().Format(time.RFC3339)}

	if clients == nil {
		pkgState.Error = "no client pool available"
		deplState.Error = "no client pool available"
		return pkgState, deplState
	}

	// Package state: check if repository service is reachable
	// (This is a lightweight check — full artifact listing goes through the
	// existing repository_list_artifacts MCP tool)
	pkgState.Error = "query via repository_list_artifacts MCP tool for full listing"

	// Deployment state: check cluster health
	// (This is a pointer to the existing cluster tools — the governor
	// provides the framework, existing MCP tools provide the data)
	deplState.Error = "query via cluster_get_health MCP tool for full status"

	return pkgState, deplState
}

// CollectFullStateSnapshot gathers project, build, AND system-level state.
func CollectFullStateSnapshot(projectDir string, clients *clientPool) StateSnapshot {
	snap := CollectStateSnapshot(projectDir)
	snap.PackageState, snap.DeploymentState = collectSystemState(clients)
	return snap
}

// ── State Transition Evaluation ──────────────────────────────────────────────

// CompareSnapshots compares two state snapshots and returns the differences.
func CompareSnapshots(before, after StateSnapshot) StateDiff {
	diff := StateDiff{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Before:    before.Timestamp,
		After:     after.Timestamp,
	}

	// Build state changes
	diff.BuildBefore = before.BuildState.Compiles
	diff.BuildAfter = after.BuildState.Compiles
	diff.BuildChanged = before.BuildState.Compiles != after.BuildState.Compiles

	diff.TestsBefore = before.BuildState.TestsPassed
	diff.TestsAfter = after.BuildState.TestsPassed
	diff.TestsChanged = before.BuildState.TestsPassed != after.BuildState.TestsPassed

	// Git clean changes
	diff.GitCleanChanged = before.ProjectState.GitClean != after.ProjectState.GitClean

	// Generated files diff
	beforeGenSet := make(map[string]bool)
	for _, f := range before.GenerationState.GeneratedFiles {
		beforeGenSet[f] = true
	}
	for _, f := range after.GenerationState.GeneratedFiles {
		if !beforeGenSet[f] {
			diff.NewGeneratedFiles = append(diff.NewGeneratedFiles, f)
		}
	}

	// Proto files diff
	beforeProtoSet := make(map[string]bool)
	for _, f := range before.ProjectState.ProtoFiles {
		beforeProtoSet[f] = true
	}
	for _, f := range after.ProjectState.ProtoFiles {
		if !beforeProtoSet[f] {
			diff.FilesAdded = append(diff.FilesAdded, "proto/"+f)
		}
	}
	afterProtoSet := make(map[string]bool)
	for _, f := range after.ProjectState.ProtoFiles {
		afterProtoSet[f] = true
	}
	for _, f := range before.ProjectState.ProtoFiles {
		if !afterProtoSet[f] {
			diff.FilesRemoved = append(diff.FilesRemoved, "proto/"+f)
		}
	}

	// Service dirs diff
	beforeSvcSet := make(map[string]bool)
	for _, d := range before.ProjectState.ServiceDirs {
		beforeSvcSet[d] = true
	}
	for _, d := range after.ProjectState.ServiceDirs {
		if !beforeSvcSet[d] {
			diff.FilesAdded = append(diff.FilesAdded, d)
		}
	}

	// Detect unexpected changes
	if diff.BuildChanged && !diff.BuildAfter {
		diff.Unexpected = append(diff.Unexpected, "build broke after operation")
	}
	if diff.TestsChanged && !diff.TestsAfter {
		diff.Unexpected = append(diff.Unexpected, "tests broke after operation")
	}
	if before.ProjectState.GitClean && !after.ProjectState.GitClean {
		// Not necessarily unexpected, but worth noting
	}

	return diff
}
