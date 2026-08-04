// Package actortransport owns the mapping between the workflow engine's
// ActionHandler contract and the WorkflowActorService gRPC callback.
//
// It exists because that mapping is where a whole class of defect lived and was
// invisible to tests. In-process, ActionRequest.Outputs is the run's live map,
// so a handler that mutated it appeared to work. Across this transport Outputs
// is a deserialized copy that dies with the call, and a receipt attached to a
// FAILED callback used to be discarded entirely — so a governed refusal reached
// the caller as a bare executor failure and charged a circuit breaker for a
// decision that was working correctly.
//
// Extracting it means tests can exercise the PRODUCTION transport rather than a
// reimplementation of it. A test that reimplements this mapping proves only
// that the test agrees with itself.
package actortransport

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

// ClientProvider returns the actor client for an actor type, dialing lazily if
// needed. Returning an error fails the step as a transport error, which carries
// no output — nothing was executed, so there is no receipt to preserve.
type ClientProvider func(actorType string) (workflowpb.WorkflowActorServiceClient, error)

// StaticClient adapts one already-connected client into a ClientProvider.
func StaticClient(c workflowpb.WorkflowActorServiceClient) ClientProvider {
	return func(string) (workflowpb.WorkflowActorServiceClient, error) { return c, nil }
}

// Handler returns the engine.ActionHandler that dispatches an action to a
// remote actor over WorkflowActorService.
//
// Ordering is load-bearing: the output delta is decoded BEFORE the response's
// Ok flag is consulted. A semantic failure — a governed refusal, a
// governance-unavailable outage — must still deliver its structured receipt, or
// the caller cannot tell it apart from a broken executor.
func Handler(actorType string, provider ClientProvider) engine.ActionHandler {
	return func(ctx context.Context, req engine.ActionRequest) (*engine.ActionResult, error) {
		client, err := provider(actorType)
		if err != nil {
			return nil, fmt.Errorf("actor %s: %w", actorType, err)
		}

		withJSON, err := json.Marshal(req.With)
		if err != nil {
			return nil, fmt.Errorf("actor %s action %s: encode with: %w", actorType, req.Action, err)
		}
		inputsJSON, err := json.Marshal(req.Inputs)
		if err != nil {
			return nil, fmt.Errorf("actor %s action %s: encode inputs: %w", actorType, req.Action, err)
		}
		// The accumulated outputs travel so the remote handler can READ prior
		// steps. It must not write them back: the engine is the single writer
		// of run outputs, and a handler's copy is discarded with the call.
		outputsJSON, err := json.Marshal(req.Outputs)
		if err != nil {
			return nil, fmt.Errorf("actor %s action %s: encode outputs: %w", actorType, req.Action, err)
		}

		resp, err := client.ExecuteAction(ctx, &workflowpb.ExecuteActionRequest{
			RunId:       req.RunID,
			StepId:      req.StepID,
			Actor:       actorType,
			Action:      req.Action,
			WithJson:    string(withJSON),
			InputsJson:  string(inputsJSON),
			OutputsJson: string(outputsJSON),
		})
		if err != nil {
			// Transport failure: no response, so no receipt and no fabricated
			// output. The step fails with nothing merged.
			return nil, fmt.Errorf("actor %s action %s: %w", actorType, req.Action, err)
		}

		var output map[string]any
		if resp.GetOutputJson() != "" {
			if err := json.Unmarshal([]byte(resp.GetOutputJson()), &output); err != nil {
				// Fail closed. A malformed receipt is not the same as no
				// receipt, and continuing with an empty map would silently
				// recreate the discarded-receipt bug this package documents.
				return nil, fmt.Errorf("actor %s action %s: malformed output json: %w",
					actorType, req.Action, err)
			}
		}

		// OK carries the actor's verdict. The engine merges Output on every
		// terminal result and then fails the step when OK is false.
		return &engine.ActionResult{
			OK:      resp.GetOk(),
			Output:  output,
			Message: resp.GetMessage(),
		}, nil
	}
}
