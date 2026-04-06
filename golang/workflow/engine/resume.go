// resume.go implements policy-driven step resume for the workflow engine.
//
// When the engine runs in resume mode (IsResume=true), steps that are about
// to execute are checked against their resume_policy metadata before
// execution. This replaces blind re-execution with fact-based decisions.
//
// See docs/architecture/workflow-hardening-implementation.md.
package engine

import (
	"context"
	"fmt"
	"log"

	"github.com/globulario/services/golang/workflow/compiler"
	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// VerificationOutcome is the tri-state result of a verification check.
type VerificationOutcome string

const (
	// VerifyPresent means the effect exists — proof is clear.
	VerifyPresent VerificationOutcome = "present"
	// VerifyAbsent means the effect does not exist — safe to execute.
	VerifyAbsent VerificationOutcome = "absent"
	// VerifyInconclusive means the check cannot determine — partial state, timeout, error.
	VerifyInconclusive VerificationOutcome = "inconclusive"
)

// resolveResumeAction decides what the engine should do for a step during
// resume. Returns true if the step should be skipped (effect already exists),
// false if it should be executed normally.
//
// If the step should be blocked (pause_for_approval) or failed, it updates
// the step state directly and returns true (skip execution).
func (e *Engine) resolveResumeAction(ctx context.Context, run *Run, step *compiler.CompiledStep, st *StepState) (skip bool) {
	if step.Execution == nil {
		// No metadata — legacy behavior: re-execute unconditionally.
		return false
	}

	policy := step.Execution.ResumePolicy
	if policy == "" {
		policy = v1alpha1.ResumePolicyRetry // default
	}

	switch policy {
	case v1alpha1.ResumePolicyRetry:
		// Re-execute unconditionally.
		log.Printf("workflow: resume %s: policy=retry, re-executing", step.ID)
		return false

	case v1alpha1.ResumePolicyVerifyEffect:
		return e.resumeWithVerification(ctx, run, step, st)

	case v1alpha1.ResumePolicyRerunIfNoReceipt:
		// Check receipt first, then verify if absent.
		if step.Execution.ReceiptKey != "" {
			if _, ok := run.Outputs[step.Execution.ReceiptKey]; ok {
				log.Printf("workflow: resume %s: receipt found for %q, skipping",
					step.ID, step.Execution.ReceiptKey)
				st.Status = StepSucceeded
				e.notifyStep(run, st)
				return true
			}
		}
		// No receipt — fall through to verification.
		return e.resumeWithVerification(ctx, run, step, st)

	case v1alpha1.ResumePolicyPauseForApproval:
		log.Printf("workflow: resume %s: policy=pause_for_approval, blocking run", step.ID)
		st.Status = StepFailed
		st.Error = "step requires approval to resume after executor crash"
		e.notifyStep(run, st)
		return true // skip execution, step is FAILED → run will fail

	case v1alpha1.ResumePolicyFail:
		log.Printf("workflow: resume %s: policy=fail, failing step conservatively", step.ID)
		st.Status = StepFailed
		st.Error = "step resume_policy=fail: conservative failure after executor crash"
		e.notifyStep(run, st)
		return true

	default:
		log.Printf("workflow: resume %s: unknown policy %q, re-executing", step.ID, policy)
		return false
	}
}

// resumeWithVerification runs the step's verification action and decides
// based on the tri-state outcome.
func (e *Engine) resumeWithVerification(ctx context.Context, run *Run, step *compiler.CompiledStep, st *StepState) (skip bool) {
	if step.Verification == nil {
		// No verification defined — fall back to re-execute.
		log.Printf("workflow: resume %s: verify_effect but no verification defined, re-executing", step.ID)
		return false
	}

	outcome := e.runVerification(ctx, run, step)

	switch outcome {
	case VerifyPresent:
		log.Printf("workflow: resume %s: verification=present, skipping re-execution", step.ID)
		st.Status = StepSucceeded
		st.Error = "" // clear any prior error
		e.notifyStep(run, st)
		return true

	case VerifyAbsent:
		log.Printf("workflow: resume %s: verification=absent, executing", step.ID)
		return false

	case VerifyInconclusive:
		// Behavior depends on idempotency class.
		idempotency := ""
		if step.Execution != nil {
			idempotency = step.Execution.Idempotency
		}
		switch idempotency {
		case v1alpha1.IdempotencySafeRetry, v1alpha1.IdempotencyVerifyThenContinue:
			log.Printf("workflow: resume %s: verification=inconclusive, idempotency=%s → re-executing",
				step.ID, idempotency)
			return false
		case v1alpha1.IdempotencyManualApproval:
			log.Printf("workflow: resume %s: verification=inconclusive, idempotency=manual_approval → blocking",
				step.ID)
			st.Status = StepFailed
			st.Error = "verification inconclusive and step requires manual approval to resume"
			e.notifyStep(run, st)
			return true
		default:
			// Conservative: re-execute for unknown idempotency class.
			log.Printf("workflow: resume %s: verification=inconclusive, unknown idempotency → re-executing", step.ID)
			return false
		}

	default:
		return false
	}
}

// runVerification dispatches the step's verification action and evaluates
// the success expression to produce a tri-state outcome.
func (e *Engine) runVerification(ctx context.Context, run *Run, step *compiler.CompiledStep) VerificationOutcome {
	v := step.Verification
	if v == nil {
		return VerifyInconclusive
	}

	handler, ok := e.Router.resolveByName(v.Actor, v.Action)
	if !ok {
		log.Printf("workflow: verification %s::%s has no handler", v.Actor, v.Action)
		return VerifyInconclusive
	}

	req := ActionRequest{
		RunID:   run.ID,
		StepID:  step.ID + ".verify",
		Actor:   v1alpha1.ActorType(v.Actor),
		Action:  v.Action,
		With:    resolveCompiledWith(v.With, run.Inputs, run.Outputs),
		Inputs:  run.Inputs,
		Outputs: run.Outputs,
	}

	result, err := handler(ctx, req)
	if err != nil {
		log.Printf("workflow: verification %s::%s failed: %v", v.Actor, v.Action, err)
		return VerifyInconclusive
	}
	if result == nil || !result.OK {
		msg := ""
		if result != nil {
			msg = result.Message
		}
		log.Printf("workflow: verification %s::%s returned not-ok: %s", v.Actor, v.Action, msg)
		return VerifyAbsent
	}

	// Evaluate success expression against the result output.
	if v.SuccessExpr != "" && result.Output != nil {
		ok, err := DefaultEvalCond(ctx, v.SuccessExpr, result.Output, run.Outputs)
		if err != nil {
			log.Printf("workflow: verification %s expr %q eval error: %v",
				step.ID, v.SuccessExpr, err)
			return VerifyInconclusive
		}
		if ok {
			return VerifyPresent
		}
		return VerifyAbsent
	}

	// No expression or no output — treat OK response as present.
	if result.OK {
		return VerifyPresent
	}
	return VerifyInconclusive
}

// _ ensures resolveCompiledWith is accessible (defined in engine.go).
var _ = fmt.Sprintf
