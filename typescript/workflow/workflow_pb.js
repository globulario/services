// source: workflow.proto
/**
 * @fileoverview
 * @enhanceable
 * @suppress {missingRequire} reports error on implicit type usages.
 * @suppress {messageConventions} JS Compiler reports an error if a variable or
 *     field starts with 'MSG_' and isn't a translatable message.
 * @public
 */
// GENERATED CODE -- DO NOT EDIT!
/* eslint-disable */
// @ts-nocheck

var jspb = require('google-protobuf');
var goog = jspb;
var global =
    (typeof globalThis !== 'undefined' && globalThis) ||
    (typeof window !== 'undefined' && window) ||
    (typeof global !== 'undefined' && global) ||
    (typeof self !== 'undefined' && self) ||
    (function () { return this; }).call(null) ||
    Function('return this')();

var google_protobuf_timestamp_pb = require('google-protobuf/google/protobuf/timestamp_pb.js');
goog.object.extend(proto, google_protobuf_timestamp_pb);
var google_protobuf_empty_pb = require('google-protobuf/google/protobuf/empty_pb.js');
goog.object.extend(proto, google_protobuf_empty_pb);
goog.exportSymbol('proto.workflow.AcknowledgeRunRequest', null, global);
goog.exportSymbol('proto.workflow.AddArtifactRefRequest', null, global);
goog.exportSymbol('proto.workflow.AppendEventRequest', null, global);
goog.exportSymbol('proto.workflow.ArtifactKind', null, global);
goog.exportSymbol('proto.workflow.CancelRunRequest', null, global);
goog.exportSymbol('proto.workflow.ClearDriftObservationRequest', null, global);
goog.exportSymbol('proto.workflow.CodePatch', null, global);
goog.exportSymbol('proto.workflow.CommandList', null, global);
goog.exportSymbol('proto.workflow.ComponentKind', null, global);
goog.exportSymbol('proto.workflow.ConfigPatch', null, global);
goog.exportSymbol('proto.workflow.DiagnoseRunRequest', null, global);
goog.exportSymbol('proto.workflow.DiagnoseRunResponse', null, global);
goog.exportSymbol('proto.workflow.DiagnosisItem', null, global);
goog.exportSymbol('proto.workflow.DriftUnresolved', null, global);
goog.exportSymbol('proto.workflow.EvidenceItem', null, global);
goog.exportSymbol('proto.workflow.FailStepRequest', null, global);
goog.exportSymbol('proto.workflow.FailureClass', null, global);
goog.exportSymbol('proto.workflow.FinishRunRequest', null, global);
goog.exportSymbol('proto.workflow.FixStatus', null, global);
goog.exportSymbol('proto.workflow.GetComponentHistoryRequest', null, global);
goog.exportSymbol('proto.workflow.GetCurrentRunsForNodeRequest', null, global);
goog.exportSymbol('proto.workflow.GetIncidentRequest', null, global);
goog.exportSymbol('proto.workflow.GetRunEventsRequest', null, global);
goog.exportSymbol('proto.workflow.GetRunEventsResponse', null, global);
goog.exportSymbol('proto.workflow.GetRunRequest', null, global);
goog.exportSymbol('proto.workflow.GetWorkflowDefinitionRequest', null, global);
goog.exportSymbol('proto.workflow.GetWorkflowDefinitionResponse', null, global);
goog.exportSymbol('proto.workflow.GetWorkflowGraphRequest', null, global);
goog.exportSymbol('proto.workflow.Incident', null, global);
goog.exportSymbol('proto.workflow.IncidentAction', null, global);
goog.exportSymbol('proto.workflow.IncidentSeverity', null, global);
goog.exportSymbol('proto.workflow.IncidentStatus', null, global);
goog.exportSymbol('proto.workflow.ListDriftUnresolvedRequest', null, global);
goog.exportSymbol('proto.workflow.ListDriftUnresolvedResponse', null, global);
goog.exportSymbol('proto.workflow.ListIncidentsRequest', null, global);
goog.exportSymbol('proto.workflow.ListIncidentsResponse', null, global);
goog.exportSymbol('proto.workflow.ListPhaseTransitionsRequest', null, global);
goog.exportSymbol('proto.workflow.ListPhaseTransitionsResponse', null, global);
goog.exportSymbol('proto.workflow.ListRunsRequest', null, global);
goog.exportSymbol('proto.workflow.ListRunsResponse', null, global);
goog.exportSymbol('proto.workflow.ListStepOutcomesRequest', null, global);
goog.exportSymbol('proto.workflow.ListStepOutcomesResponse', null, global);
goog.exportSymbol('proto.workflow.ListWorkflowDefinitionsRequest', null, global);
goog.exportSymbol('proto.workflow.ListWorkflowDefinitionsResponse', null, global);
goog.exportSymbol('proto.workflow.ListWorkflowSummariesRequest', null, global);
goog.exportSymbol('proto.workflow.ListWorkflowSummariesResponse', null, global);
goog.exportSymbol('proto.workflow.PhaseTransitionEvent', null, global);
goog.exportSymbol('proto.workflow.ProposedFix', null, global);
goog.exportSymbol('proto.workflow.Provenance', null, global);
goog.exportSymbol('proto.workflow.RecordDriftObservationRequest', null, global);
goog.exportSymbol('proto.workflow.RecordOutcomeRequest', null, global);
goog.exportSymbol('proto.workflow.RecordPhaseTransitionRequest', null, global);
goog.exportSymbol('proto.workflow.RecordStepOutcomeRequest', null, global);
goog.exportSymbol('proto.workflow.RecordStepRequest', null, global);
goog.exportSymbol('proto.workflow.RestartAction', null, global);
goog.exportSymbol('proto.workflow.RetryRunRequest', null, global);
goog.exportSymbol('proto.workflow.RunStatus', null, global);
goog.exportSymbol('proto.workflow.StartRunRequest', null, global);
goog.exportSymbol('proto.workflow.StepStatus', null, global);
goog.exportSymbol('proto.workflow.SubmitProposedFixRequest', null, global);
goog.exportSymbol('proto.workflow.TriggerReason', null, global);
goog.exportSymbol('proto.workflow.UpdateRunRequest', null, global);
goog.exportSymbol('proto.workflow.UpdateStepRequest', null, global);
goog.exportSymbol('proto.workflow.WatchNodeRunsRequest', null, global);
goog.exportSymbol('proto.workflow.WatchRunRequest', null, global);
goog.exportSymbol('proto.workflow.WorkflowActor', null, global);
goog.exportSymbol('proto.workflow.WorkflowActorLane', null, global);
goog.exportSymbol('proto.workflow.WorkflowArtifactRef', null, global);
goog.exportSymbol('proto.workflow.WorkflowContext', null, global);
goog.exportSymbol('proto.workflow.WorkflowDefinitionSummary', null, global);
goog.exportSymbol('proto.workflow.WorkflowEvent', null, global);
goog.exportSymbol('proto.workflow.WorkflowEventEnvelope', null, global);
goog.exportSymbol('proto.workflow.WorkflowGraph', null, global);
goog.exportSymbol('proto.workflow.WorkflowPhase', null, global);
goog.exportSymbol('proto.workflow.WorkflowPhaseKind', null, global);
goog.exportSymbol('proto.workflow.WorkflowRun', null, global);
goog.exportSymbol('proto.workflow.WorkflowRunDetail', null, global);
goog.exportSymbol('proto.workflow.WorkflowRunSummary', null, global);
goog.exportSymbol('proto.workflow.WorkflowStep', null, global);
goog.exportSymbol('proto.workflow.WorkflowStepOutcome', null, global);
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.WorkflowContext = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.WorkflowContext, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.WorkflowContext.displayName = 'proto.workflow.WorkflowContext';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.WorkflowRun = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.WorkflowRun, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.WorkflowRun.displayName = 'proto.workflow.WorkflowRun';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.WorkflowStep = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.WorkflowStep, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.WorkflowStep.displayName = 'proto.workflow.WorkflowStep';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.WorkflowArtifactRef = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.WorkflowArtifactRef, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.WorkflowArtifactRef.displayName = 'proto.workflow.WorkflowArtifactRef';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.WorkflowEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.WorkflowEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.WorkflowEvent.displayName = 'proto.workflow.WorkflowEvent';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.WorkflowPhase = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.workflow.WorkflowPhase.repeatedFields_, null);
};
goog.inherits(proto.workflow.WorkflowPhase, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.WorkflowPhase.displayName = 'proto.workflow.WorkflowPhase';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.WorkflowActorLane = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.workflow.WorkflowActorLane.repeatedFields_, null);
};
goog.inherits(proto.workflow.WorkflowActorLane, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.WorkflowActorLane.displayName = 'proto.workflow.WorkflowActorLane';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.WorkflowGraph = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.workflow.WorkflowGraph.repeatedFields_, null);
};
goog.inherits(proto.workflow.WorkflowGraph, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.WorkflowGraph.displayName = 'proto.workflow.WorkflowGraph';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.WorkflowRunDetail = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.workflow.WorkflowRunDetail.repeatedFields_, null);
};
goog.inherits(proto.workflow.WorkflowRunDetail, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.WorkflowRunDetail.displayName = 'proto.workflow.WorkflowRunDetail';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.StartRunRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.StartRunRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.StartRunRequest.displayName = 'proto.workflow.StartRunRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.UpdateRunRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.UpdateRunRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.UpdateRunRequest.displayName = 'proto.workflow.UpdateRunRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.FinishRunRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.FinishRunRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.FinishRunRequest.displayName = 'proto.workflow.FinishRunRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.RecordStepRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.RecordStepRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.RecordStepRequest.displayName = 'proto.workflow.RecordStepRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.UpdateStepRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.UpdateStepRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.UpdateStepRequest.displayName = 'proto.workflow.UpdateStepRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.FailStepRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.FailStepRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.FailStepRequest.displayName = 'proto.workflow.FailStepRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.AddArtifactRefRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.AddArtifactRefRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.AddArtifactRefRequest.displayName = 'proto.workflow.AddArtifactRefRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.AppendEventRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.AppendEventRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.AppendEventRequest.displayName = 'proto.workflow.AppendEventRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.GetRunRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.GetRunRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.GetRunRequest.displayName = 'proto.workflow.GetRunRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.ListRunsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.ListRunsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.ListRunsRequest.displayName = 'proto.workflow.ListRunsRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.ListRunsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.workflow.ListRunsResponse.repeatedFields_, null);
};
goog.inherits(proto.workflow.ListRunsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.ListRunsResponse.displayName = 'proto.workflow.ListRunsResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.GetRunEventsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.GetRunEventsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.GetRunEventsRequest.displayName = 'proto.workflow.GetRunEventsRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.GetRunEventsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.workflow.GetRunEventsResponse.repeatedFields_, null);
};
goog.inherits(proto.workflow.GetRunEventsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.GetRunEventsResponse.displayName = 'proto.workflow.GetRunEventsResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.GetCurrentRunsForNodeRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.GetCurrentRunsForNodeRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.GetCurrentRunsForNodeRequest.displayName = 'proto.workflow.GetCurrentRunsForNodeRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.GetComponentHistoryRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.GetComponentHistoryRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.GetComponentHistoryRequest.displayName = 'proto.workflow.GetComponentHistoryRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.GetWorkflowGraphRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.GetWorkflowGraphRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.GetWorkflowGraphRequest.displayName = 'proto.workflow.GetWorkflowGraphRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.WatchRunRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.WatchRunRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.WatchRunRequest.displayName = 'proto.workflow.WatchRunRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.WatchNodeRunsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.WatchNodeRunsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.WatchNodeRunsRequest.displayName = 'proto.workflow.WatchNodeRunsRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.WorkflowEventEnvelope = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.WorkflowEventEnvelope, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.WorkflowEventEnvelope.displayName = 'proto.workflow.WorkflowEventEnvelope';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.RetryRunRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.RetryRunRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.RetryRunRequest.displayName = 'proto.workflow.RetryRunRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.CancelRunRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.CancelRunRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.CancelRunRequest.displayName = 'proto.workflow.CancelRunRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.AcknowledgeRunRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.AcknowledgeRunRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.AcknowledgeRunRequest.displayName = 'proto.workflow.AcknowledgeRunRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.DiagnoseRunRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.DiagnoseRunRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.DiagnoseRunRequest.displayName = 'proto.workflow.DiagnoseRunRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.DiagnoseRunResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.workflow.DiagnoseRunResponse.repeatedFields_, null);
};
goog.inherits(proto.workflow.DiagnoseRunResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.DiagnoseRunResponse.displayName = 'proto.workflow.DiagnoseRunResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.WorkflowRunSummary = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.WorkflowRunSummary, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.WorkflowRunSummary.displayName = 'proto.workflow.WorkflowRunSummary';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.RecordOutcomeRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.RecordOutcomeRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.RecordOutcomeRequest.displayName = 'proto.workflow.RecordOutcomeRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.ListWorkflowSummariesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.ListWorkflowSummariesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.ListWorkflowSummariesRequest.displayName = 'proto.workflow.ListWorkflowSummariesRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.ListWorkflowSummariesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.workflow.ListWorkflowSummariesResponse.repeatedFields_, null);
};
goog.inherits(proto.workflow.ListWorkflowSummariesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.ListWorkflowSummariesResponse.displayName = 'proto.workflow.ListWorkflowSummariesResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.WorkflowStepOutcome = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.WorkflowStepOutcome, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.WorkflowStepOutcome.displayName = 'proto.workflow.WorkflowStepOutcome';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.RecordStepOutcomeRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.RecordStepOutcomeRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.RecordStepOutcomeRequest.displayName = 'proto.workflow.RecordStepOutcomeRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.ListStepOutcomesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.ListStepOutcomesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.ListStepOutcomesRequest.displayName = 'proto.workflow.ListStepOutcomesRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.ListStepOutcomesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.workflow.ListStepOutcomesResponse.repeatedFields_, null);
};
goog.inherits(proto.workflow.ListStepOutcomesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.ListStepOutcomesResponse.displayName = 'proto.workflow.ListStepOutcomesResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.PhaseTransitionEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.PhaseTransitionEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.PhaseTransitionEvent.displayName = 'proto.workflow.PhaseTransitionEvent';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.RecordPhaseTransitionRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.RecordPhaseTransitionRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.RecordPhaseTransitionRequest.displayName = 'proto.workflow.RecordPhaseTransitionRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.ListPhaseTransitionsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.ListPhaseTransitionsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.ListPhaseTransitionsRequest.displayName = 'proto.workflow.ListPhaseTransitionsRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.ListPhaseTransitionsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.workflow.ListPhaseTransitionsResponse.repeatedFields_, null);
};
goog.inherits(proto.workflow.ListPhaseTransitionsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.ListPhaseTransitionsResponse.displayName = 'proto.workflow.ListPhaseTransitionsResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.DriftUnresolved = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.DriftUnresolved, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.DriftUnresolved.displayName = 'proto.workflow.DriftUnresolved';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.RecordDriftObservationRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.RecordDriftObservationRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.RecordDriftObservationRequest.displayName = 'proto.workflow.RecordDriftObservationRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.ClearDriftObservationRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.ClearDriftObservationRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.ClearDriftObservationRequest.displayName = 'proto.workflow.ClearDriftObservationRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.ListDriftUnresolvedRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.ListDriftUnresolvedRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.ListDriftUnresolvedRequest.displayName = 'proto.workflow.ListDriftUnresolvedRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.ListDriftUnresolvedResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.workflow.ListDriftUnresolvedResponse.repeatedFields_, null);
};
goog.inherits(proto.workflow.ListDriftUnresolvedResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.ListDriftUnresolvedResponse.displayName = 'proto.workflow.ListDriftUnresolvedResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.EvidenceItem = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.EvidenceItem, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.EvidenceItem.displayName = 'proto.workflow.EvidenceItem';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.DiagnosisItem = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.workflow.DiagnosisItem.repeatedFields_, null);
};
goog.inherits(proto.workflow.DiagnosisItem, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.DiagnosisItem.displayName = 'proto.workflow.DiagnosisItem';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.CodePatch = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.CodePatch, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.CodePatch.displayName = 'proto.workflow.CodePatch';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.ConfigPatch = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.ConfigPatch, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.ConfigPatch.displayName = 'proto.workflow.ConfigPatch';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.CommandList = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.workflow.CommandList.repeatedFields_, null);
};
goog.inherits(proto.workflow.CommandList, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.CommandList.displayName = 'proto.workflow.CommandList';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.RestartAction = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.workflow.RestartAction.repeatedFields_, null);
};
goog.inherits(proto.workflow.RestartAction, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.RestartAction.displayName = 'proto.workflow.RestartAction';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.ProposedFix = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.workflow.ProposedFix.repeatedFields_, null);
};
goog.inherits(proto.workflow.ProposedFix, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.ProposedFix.displayName = 'proto.workflow.ProposedFix';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.Incident = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.workflow.Incident.repeatedFields_, null);
};
goog.inherits(proto.workflow.Incident, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.Incident.displayName = 'proto.workflow.Incident';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.IncidentAction = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.IncidentAction, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.IncidentAction.displayName = 'proto.workflow.IncidentAction';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.ListIncidentsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.ListIncidentsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.ListIncidentsRequest.displayName = 'proto.workflow.ListIncidentsRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.ListIncidentsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.workflow.ListIncidentsResponse.repeatedFields_, null);
};
goog.inherits(proto.workflow.ListIncidentsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.ListIncidentsResponse.displayName = 'proto.workflow.ListIncidentsResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.GetIncidentRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.GetIncidentRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.GetIncidentRequest.displayName = 'proto.workflow.GetIncidentRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.SubmitProposedFixRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.SubmitProposedFixRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.SubmitProposedFixRequest.displayName = 'proto.workflow.SubmitProposedFixRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.ListWorkflowDefinitionsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.ListWorkflowDefinitionsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.ListWorkflowDefinitionsRequest.displayName = 'proto.workflow.ListWorkflowDefinitionsRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.WorkflowDefinitionSummary = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.WorkflowDefinitionSummary, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.WorkflowDefinitionSummary.displayName = 'proto.workflow.WorkflowDefinitionSummary';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.ListWorkflowDefinitionsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.workflow.ListWorkflowDefinitionsResponse.repeatedFields_, null);
};
goog.inherits(proto.workflow.ListWorkflowDefinitionsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.ListWorkflowDefinitionsResponse.displayName = 'proto.workflow.ListWorkflowDefinitionsResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.GetWorkflowDefinitionRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.GetWorkflowDefinitionRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.GetWorkflowDefinitionRequest.displayName = 'proto.workflow.GetWorkflowDefinitionRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.workflow.GetWorkflowDefinitionResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.workflow.GetWorkflowDefinitionResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.workflow.GetWorkflowDefinitionResponse.displayName = 'proto.workflow.GetWorkflowDefinitionResponse';
}



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.WorkflowContext.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.WorkflowContext.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.WorkflowContext} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowContext.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
nodeId: jspb.Message.getFieldWithDefault(msg, 2, ""),
nodeHostname: jspb.Message.getFieldWithDefault(msg, 3, ""),
componentName: jspb.Message.getFieldWithDefault(msg, 4, ""),
componentKind: jspb.Message.getFieldWithDefault(msg, 5, 0),
componentVersion: jspb.Message.getFieldWithDefault(msg, 6, ""),
releaseKind: jspb.Message.getFieldWithDefault(msg, 7, ""),
releaseObjectId: jspb.Message.getFieldWithDefault(msg, 8, ""),
desiredObjectId: jspb.Message.getFieldWithDefault(msg, 9, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.WorkflowContext}
 */
proto.workflow.WorkflowContext.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.WorkflowContext;
  return proto.workflow.WorkflowContext.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.WorkflowContext} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.WorkflowContext}
 */
proto.workflow.WorkflowContext.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeId(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeHostname(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setComponentName(value);
      break;
    case 5:
      var value = /** @type {!proto.workflow.ComponentKind} */ (reader.readEnum());
      msg.setComponentKind(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setComponentVersion(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setReleaseKind(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setReleaseObjectId(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setDesiredObjectId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.WorkflowContext.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.WorkflowContext.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.WorkflowContext} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowContext.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getNodeHostname();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getComponentName();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getComponentKind();
  if (f !== 0.0) {
    writer.writeEnum(
      5,
      f
    );
  }
  f = message.getComponentVersion();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getReleaseKind();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getReleaseObjectId();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
  f = message.getDesiredObjectId();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.WorkflowContext.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowContext} returns this
 */
proto.workflow.WorkflowContext.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string node_id = 2;
 * @return {string}
 */
proto.workflow.WorkflowContext.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowContext} returns this
 */
proto.workflow.WorkflowContext.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string node_hostname = 3;
 * @return {string}
 */
proto.workflow.WorkflowContext.prototype.getNodeHostname = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowContext} returns this
 */
proto.workflow.WorkflowContext.prototype.setNodeHostname = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string component_name = 4;
 * @return {string}
 */
proto.workflow.WorkflowContext.prototype.getComponentName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowContext} returns this
 */
proto.workflow.WorkflowContext.prototype.setComponentName = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional ComponentKind component_kind = 5;
 * @return {!proto.workflow.ComponentKind}
 */
proto.workflow.WorkflowContext.prototype.getComponentKind = function() {
  return /** @type {!proto.workflow.ComponentKind} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {!proto.workflow.ComponentKind} value
 * @return {!proto.workflow.WorkflowContext} returns this
 */
proto.workflow.WorkflowContext.prototype.setComponentKind = function(value) {
  return jspb.Message.setProto3EnumField(this, 5, value);
};


/**
 * optional string component_version = 6;
 * @return {string}
 */
proto.workflow.WorkflowContext.prototype.getComponentVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowContext} returns this
 */
proto.workflow.WorkflowContext.prototype.setComponentVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string release_kind = 7;
 * @return {string}
 */
proto.workflow.WorkflowContext.prototype.getReleaseKind = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowContext} returns this
 */
proto.workflow.WorkflowContext.prototype.setReleaseKind = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional string release_object_id = 8;
 * @return {string}
 */
proto.workflow.WorkflowContext.prototype.getReleaseObjectId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowContext} returns this
 */
proto.workflow.WorkflowContext.prototype.setReleaseObjectId = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};


/**
 * optional string desired_object_id = 9;
 * @return {string}
 */
proto.workflow.WorkflowContext.prototype.getDesiredObjectId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowContext} returns this
 */
proto.workflow.WorkflowContext.prototype.setDesiredObjectId = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.WorkflowRun.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.WorkflowRun.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.WorkflowRun} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowRun.toObject = function(includeInstance, msg) {
  var f, obj = {
id: jspb.Message.getFieldWithDefault(msg, 1, ""),
correlationId: jspb.Message.getFieldWithDefault(msg, 2, ""),
parentRunId: jspb.Message.getFieldWithDefault(msg, 3, ""),
context: (f = msg.getContext()) && proto.workflow.WorkflowContext.toObject(includeInstance, f),
triggerReason: jspb.Message.getFieldWithDefault(msg, 5, 0),
status: jspb.Message.getFieldWithDefault(msg, 6, 0),
currentActor: jspb.Message.getFieldWithDefault(msg, 7, 0),
failureClass: jspb.Message.getFieldWithDefault(msg, 8, 0),
summary: jspb.Message.getFieldWithDefault(msg, 9, ""),
errorMessage: jspb.Message.getFieldWithDefault(msg, 10, ""),
retryCount: jspb.Message.getFieldWithDefault(msg, 11, 0),
acknowledged: jspb.Message.getBooleanFieldWithDefault(msg, 12, false),
acknowledgedBy: jspb.Message.getFieldWithDefault(msg, 13, ""),
acknowledgedAt: (f = msg.getAcknowledgedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
startedAt: (f = msg.getStartedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
updatedAt: (f = msg.getUpdatedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
finishedAt: (f = msg.getFinishedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
workflowName: jspb.Message.getFieldWithDefault(msg, 18, ""),
waitReason: jspb.Message.getFieldWithDefault(msg, 19, ""),
retryAttempt: jspb.Message.getFieldWithDefault(msg, 20, 0),
maxRetries: jspb.Message.getFieldWithDefault(msg, 21, 0),
backoffUntilMs: jspb.Message.getFieldWithDefault(msg, 22, 0),
supersededBy: jspb.Message.getFieldWithDefault(msg, 23, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.WorkflowRun}
 */
proto.workflow.WorkflowRun.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.WorkflowRun;
  return proto.workflow.WorkflowRun.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.WorkflowRun} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.WorkflowRun}
 */
proto.workflow.WorkflowRun.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setCorrelationId(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setParentRunId(value);
      break;
    case 4:
      var value = new proto.workflow.WorkflowContext;
      reader.readMessage(value,proto.workflow.WorkflowContext.deserializeBinaryFromReader);
      msg.setContext(value);
      break;
    case 5:
      var value = /** @type {!proto.workflow.TriggerReason} */ (reader.readEnum());
      msg.setTriggerReason(value);
      break;
    case 6:
      var value = /** @type {!proto.workflow.RunStatus} */ (reader.readEnum());
      msg.setStatus(value);
      break;
    case 7:
      var value = /** @type {!proto.workflow.WorkflowActor} */ (reader.readEnum());
      msg.setCurrentActor(value);
      break;
    case 8:
      var value = /** @type {!proto.workflow.FailureClass} */ (reader.readEnum());
      msg.setFailureClass(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setSummary(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.setErrorMessage(value);
      break;
    case 11:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setRetryCount(value);
      break;
    case 12:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setAcknowledged(value);
      break;
    case 13:
      var value = /** @type {string} */ (reader.readString());
      msg.setAcknowledgedBy(value);
      break;
    case 14:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setAcknowledgedAt(value);
      break;
    case 15:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setStartedAt(value);
      break;
    case 16:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setUpdatedAt(value);
      break;
    case 17:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setFinishedAt(value);
      break;
    case 18:
      var value = /** @type {string} */ (reader.readString());
      msg.setWorkflowName(value);
      break;
    case 19:
      var value = /** @type {string} */ (reader.readString());
      msg.setWaitReason(value);
      break;
    case 20:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setRetryAttempt(value);
      break;
    case 21:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setMaxRetries(value);
      break;
    case 22:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBackoffUntilMs(value);
      break;
    case 23:
      var value = /** @type {string} */ (reader.readString());
      msg.setSupersededBy(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.WorkflowRun.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.WorkflowRun.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.WorkflowRun} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowRun.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getCorrelationId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getParentRunId();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getContext();
  if (f != null) {
    writer.writeMessage(
      4,
      f,
      proto.workflow.WorkflowContext.serializeBinaryToWriter
    );
  }
  f = message.getTriggerReason();
  if (f !== 0.0) {
    writer.writeEnum(
      5,
      f
    );
  }
  f = message.getStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      6,
      f
    );
  }
  f = message.getCurrentActor();
  if (f !== 0.0) {
    writer.writeEnum(
      7,
      f
    );
  }
  f = message.getFailureClass();
  if (f !== 0.0) {
    writer.writeEnum(
      8,
      f
    );
  }
  f = message.getSummary();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
  f = message.getErrorMessage();
  if (f.length > 0) {
    writer.writeString(
      10,
      f
    );
  }
  f = message.getRetryCount();
  if (f !== 0) {
    writer.writeInt32(
      11,
      f
    );
  }
  f = message.getAcknowledged();
  if (f) {
    writer.writeBool(
      12,
      f
    );
  }
  f = message.getAcknowledgedBy();
  if (f.length > 0) {
    writer.writeString(
      13,
      f
    );
  }
  f = message.getAcknowledgedAt();
  if (f != null) {
    writer.writeMessage(
      14,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getStartedAt();
  if (f != null) {
    writer.writeMessage(
      15,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getUpdatedAt();
  if (f != null) {
    writer.writeMessage(
      16,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getFinishedAt();
  if (f != null) {
    writer.writeMessage(
      17,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getWorkflowName();
  if (f.length > 0) {
    writer.writeString(
      18,
      f
    );
  }
  f = message.getWaitReason();
  if (f.length > 0) {
    writer.writeString(
      19,
      f
    );
  }
  f = message.getRetryAttempt();
  if (f !== 0) {
    writer.writeInt32(
      20,
      f
    );
  }
  f = message.getMaxRetries();
  if (f !== 0) {
    writer.writeInt32(
      21,
      f
    );
  }
  f = message.getBackoffUntilMs();
  if (f !== 0) {
    writer.writeInt64(
      22,
      f
    );
  }
  f = message.getSupersededBy();
  if (f.length > 0) {
    writer.writeString(
      23,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.workflow.WorkflowRun.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string correlation_id = 2;
 * @return {string}
 */
proto.workflow.WorkflowRun.prototype.getCorrelationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.setCorrelationId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string parent_run_id = 3;
 * @return {string}
 */
proto.workflow.WorkflowRun.prototype.getParentRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.setParentRunId = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional WorkflowContext context = 4;
 * @return {?proto.workflow.WorkflowContext}
 */
proto.workflow.WorkflowRun.prototype.getContext = function() {
  return /** @type{?proto.workflow.WorkflowContext} */ (
    jspb.Message.getWrapperField(this, proto.workflow.WorkflowContext, 4));
};


/**
 * @param {?proto.workflow.WorkflowContext|undefined} value
 * @return {!proto.workflow.WorkflowRun} returns this
*/
proto.workflow.WorkflowRun.prototype.setContext = function(value) {
  return jspb.Message.setWrapperField(this, 4, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.clearContext = function() {
  return this.setContext(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowRun.prototype.hasContext = function() {
  return jspb.Message.getField(this, 4) != null;
};


/**
 * optional TriggerReason trigger_reason = 5;
 * @return {!proto.workflow.TriggerReason}
 */
proto.workflow.WorkflowRun.prototype.getTriggerReason = function() {
  return /** @type {!proto.workflow.TriggerReason} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {!proto.workflow.TriggerReason} value
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.setTriggerReason = function(value) {
  return jspb.Message.setProto3EnumField(this, 5, value);
};


/**
 * optional RunStatus status = 6;
 * @return {!proto.workflow.RunStatus}
 */
proto.workflow.WorkflowRun.prototype.getStatus = function() {
  return /** @type {!proto.workflow.RunStatus} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {!proto.workflow.RunStatus} value
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.setStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 6, value);
};


/**
 * optional WorkflowActor current_actor = 7;
 * @return {!proto.workflow.WorkflowActor}
 */
proto.workflow.WorkflowRun.prototype.getCurrentActor = function() {
  return /** @type {!proto.workflow.WorkflowActor} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/**
 * @param {!proto.workflow.WorkflowActor} value
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.setCurrentActor = function(value) {
  return jspb.Message.setProto3EnumField(this, 7, value);
};


/**
 * optional FailureClass failure_class = 8;
 * @return {!proto.workflow.FailureClass}
 */
proto.workflow.WorkflowRun.prototype.getFailureClass = function() {
  return /** @type {!proto.workflow.FailureClass} */ (jspb.Message.getFieldWithDefault(this, 8, 0));
};


/**
 * @param {!proto.workflow.FailureClass} value
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.setFailureClass = function(value) {
  return jspb.Message.setProto3EnumField(this, 8, value);
};


/**
 * optional string summary = 9;
 * @return {string}
 */
proto.workflow.WorkflowRun.prototype.getSummary = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.setSummary = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};


/**
 * optional string error_message = 10;
 * @return {string}
 */
proto.workflow.WorkflowRun.prototype.getErrorMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 10, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.setErrorMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 10, value);
};


/**
 * optional int32 retry_count = 11;
 * @return {number}
 */
proto.workflow.WorkflowRun.prototype.getRetryCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 11, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.setRetryCount = function(value) {
  return jspb.Message.setProto3IntField(this, 11, value);
};


/**
 * optional bool acknowledged = 12;
 * @return {boolean}
 */
proto.workflow.WorkflowRun.prototype.getAcknowledged = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 12, false));
};


/**
 * @param {boolean} value
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.setAcknowledged = function(value) {
  return jspb.Message.setProto3BooleanField(this, 12, value);
};


/**
 * optional string acknowledged_by = 13;
 * @return {string}
 */
proto.workflow.WorkflowRun.prototype.getAcknowledgedBy = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 13, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.setAcknowledgedBy = function(value) {
  return jspb.Message.setProto3StringField(this, 13, value);
};


/**
 * optional google.protobuf.Timestamp acknowledged_at = 14;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.WorkflowRun.prototype.getAcknowledgedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 14));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.WorkflowRun} returns this
*/
proto.workflow.WorkflowRun.prototype.setAcknowledgedAt = function(value) {
  return jspb.Message.setWrapperField(this, 14, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.clearAcknowledgedAt = function() {
  return this.setAcknowledgedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowRun.prototype.hasAcknowledgedAt = function() {
  return jspb.Message.getField(this, 14) != null;
};


/**
 * optional google.protobuf.Timestamp started_at = 15;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.WorkflowRun.prototype.getStartedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 15));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.WorkflowRun} returns this
*/
proto.workflow.WorkflowRun.prototype.setStartedAt = function(value) {
  return jspb.Message.setWrapperField(this, 15, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.clearStartedAt = function() {
  return this.setStartedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowRun.prototype.hasStartedAt = function() {
  return jspb.Message.getField(this, 15) != null;
};


/**
 * optional google.protobuf.Timestamp updated_at = 16;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.WorkflowRun.prototype.getUpdatedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 16));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.WorkflowRun} returns this
*/
proto.workflow.WorkflowRun.prototype.setUpdatedAt = function(value) {
  return jspb.Message.setWrapperField(this, 16, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.clearUpdatedAt = function() {
  return this.setUpdatedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowRun.prototype.hasUpdatedAt = function() {
  return jspb.Message.getField(this, 16) != null;
};


/**
 * optional google.protobuf.Timestamp finished_at = 17;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.WorkflowRun.prototype.getFinishedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 17));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.WorkflowRun} returns this
*/
proto.workflow.WorkflowRun.prototype.setFinishedAt = function(value) {
  return jspb.Message.setWrapperField(this, 17, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.clearFinishedAt = function() {
  return this.setFinishedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowRun.prototype.hasFinishedAt = function() {
  return jspb.Message.getField(this, 17) != null;
};


/**
 * optional string workflow_name = 18;
 * @return {string}
 */
proto.workflow.WorkflowRun.prototype.getWorkflowName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 18, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.setWorkflowName = function(value) {
  return jspb.Message.setProto3StringField(this, 18, value);
};


/**
 * optional string wait_reason = 19;
 * @return {string}
 */
proto.workflow.WorkflowRun.prototype.getWaitReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 19, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.setWaitReason = function(value) {
  return jspb.Message.setProto3StringField(this, 19, value);
};


/**
 * optional int32 retry_attempt = 20;
 * @return {number}
 */
proto.workflow.WorkflowRun.prototype.getRetryAttempt = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 20, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.setRetryAttempt = function(value) {
  return jspb.Message.setProto3IntField(this, 20, value);
};


/**
 * optional int32 max_retries = 21;
 * @return {number}
 */
proto.workflow.WorkflowRun.prototype.getMaxRetries = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 21, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.setMaxRetries = function(value) {
  return jspb.Message.setProto3IntField(this, 21, value);
};


/**
 * optional int64 backoff_until_ms = 22;
 * @return {number}
 */
proto.workflow.WorkflowRun.prototype.getBackoffUntilMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 22, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.setBackoffUntilMs = function(value) {
  return jspb.Message.setProto3IntField(this, 22, value);
};


/**
 * optional string superseded_by = 23;
 * @return {string}
 */
proto.workflow.WorkflowRun.prototype.getSupersededBy = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 23, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowRun} returns this
 */
proto.workflow.WorkflowRun.prototype.setSupersededBy = function(value) {
  return jspb.Message.setProto3StringField(this, 23, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.WorkflowStep.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.WorkflowStep.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.WorkflowStep} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowStep.toObject = function(includeInstance, msg) {
  var f, obj = {
runId: jspb.Message.getFieldWithDefault(msg, 1, ""),
seq: jspb.Message.getFieldWithDefault(msg, 2, 0),
stepKey: jspb.Message.getFieldWithDefault(msg, 3, ""),
title: jspb.Message.getFieldWithDefault(msg, 4, ""),
actor: jspb.Message.getFieldWithDefault(msg, 5, 0),
phase: jspb.Message.getFieldWithDefault(msg, 6, 0),
status: jspb.Message.getFieldWithDefault(msg, 7, 0),
attempt: jspb.Message.getFieldWithDefault(msg, 8, 0),
createdAt: (f = msg.getCreatedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
startedAt: (f = msg.getStartedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
finishedAt: (f = msg.getFinishedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
durationMs: jspb.Message.getFieldWithDefault(msg, 12, 0),
message: jspb.Message.getFieldWithDefault(msg, 13, ""),
errorCode: jspb.Message.getFieldWithDefault(msg, 14, ""),
errorMessage: jspb.Message.getFieldWithDefault(msg, 15, ""),
retryable: jspb.Message.getBooleanFieldWithDefault(msg, 16, false),
operatorActionRequired: jspb.Message.getBooleanFieldWithDefault(msg, 17, false),
actionHint: jspb.Message.getFieldWithDefault(msg, 18, ""),
sourceActor: jspb.Message.getFieldWithDefault(msg, 19, 0),
targetActor: jspb.Message.getFieldWithDefault(msg, 20, 0),
detailsJson: jspb.Message.getFieldWithDefault(msg, 21, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.WorkflowStep}
 */
proto.workflow.WorkflowStep.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.WorkflowStep;
  return proto.workflow.WorkflowStep.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.WorkflowStep} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.WorkflowStep}
 */
proto.workflow.WorkflowStep.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRunId(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setSeq(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setStepKey(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setTitle(value);
      break;
    case 5:
      var value = /** @type {!proto.workflow.WorkflowActor} */ (reader.readEnum());
      msg.setActor(value);
      break;
    case 6:
      var value = /** @type {!proto.workflow.WorkflowPhaseKind} */ (reader.readEnum());
      msg.setPhase(value);
      break;
    case 7:
      var value = /** @type {!proto.workflow.StepStatus} */ (reader.readEnum());
      msg.setStatus(value);
      break;
    case 8:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setAttempt(value);
      break;
    case 9:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setCreatedAt(value);
      break;
    case 10:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setStartedAt(value);
      break;
    case 11:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setFinishedAt(value);
      break;
    case 12:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setDurationMs(value);
      break;
    case 13:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
      break;
    case 14:
      var value = /** @type {string} */ (reader.readString());
      msg.setErrorCode(value);
      break;
    case 15:
      var value = /** @type {string} */ (reader.readString());
      msg.setErrorMessage(value);
      break;
    case 16:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setRetryable(value);
      break;
    case 17:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setOperatorActionRequired(value);
      break;
    case 18:
      var value = /** @type {string} */ (reader.readString());
      msg.setActionHint(value);
      break;
    case 19:
      var value = /** @type {!proto.workflow.WorkflowActor} */ (reader.readEnum());
      msg.setSourceActor(value);
      break;
    case 20:
      var value = /** @type {!proto.workflow.WorkflowActor} */ (reader.readEnum());
      msg.setTargetActor(value);
      break;
    case 21:
      var value = /** @type {string} */ (reader.readString());
      msg.setDetailsJson(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.WorkflowStep.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.WorkflowStep.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.WorkflowStep} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowStep.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRunId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getSeq();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getStepKey();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getTitle();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getActor();
  if (f !== 0.0) {
    writer.writeEnum(
      5,
      f
    );
  }
  f = message.getPhase();
  if (f !== 0.0) {
    writer.writeEnum(
      6,
      f
    );
  }
  f = message.getStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      7,
      f
    );
  }
  f = message.getAttempt();
  if (f !== 0) {
    writer.writeInt32(
      8,
      f
    );
  }
  f = message.getCreatedAt();
  if (f != null) {
    writer.writeMessage(
      9,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getStartedAt();
  if (f != null) {
    writer.writeMessage(
      10,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getFinishedAt();
  if (f != null) {
    writer.writeMessage(
      11,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getDurationMs();
  if (f !== 0) {
    writer.writeInt64(
      12,
      f
    );
  }
  f = message.getMessage();
  if (f.length > 0) {
    writer.writeString(
      13,
      f
    );
  }
  f = message.getErrorCode();
  if (f.length > 0) {
    writer.writeString(
      14,
      f
    );
  }
  f = message.getErrorMessage();
  if (f.length > 0) {
    writer.writeString(
      15,
      f
    );
  }
  f = message.getRetryable();
  if (f) {
    writer.writeBool(
      16,
      f
    );
  }
  f = message.getOperatorActionRequired();
  if (f) {
    writer.writeBool(
      17,
      f
    );
  }
  f = message.getActionHint();
  if (f.length > 0) {
    writer.writeString(
      18,
      f
    );
  }
  f = message.getSourceActor();
  if (f !== 0.0) {
    writer.writeEnum(
      19,
      f
    );
  }
  f = message.getTargetActor();
  if (f !== 0.0) {
    writer.writeEnum(
      20,
      f
    );
  }
  f = message.getDetailsJson();
  if (f.length > 0) {
    writer.writeString(
      21,
      f
    );
  }
};


/**
 * optional string run_id = 1;
 * @return {string}
 */
proto.workflow.WorkflowStep.prototype.getRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowStep} returns this
 */
proto.workflow.WorkflowStep.prototype.setRunId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int32 seq = 2;
 * @return {number}
 */
proto.workflow.WorkflowStep.prototype.getSeq = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.WorkflowStep} returns this
 */
proto.workflow.WorkflowStep.prototype.setSeq = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string step_key = 3;
 * @return {string}
 */
proto.workflow.WorkflowStep.prototype.getStepKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowStep} returns this
 */
proto.workflow.WorkflowStep.prototype.setStepKey = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string title = 4;
 * @return {string}
 */
proto.workflow.WorkflowStep.prototype.getTitle = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowStep} returns this
 */
proto.workflow.WorkflowStep.prototype.setTitle = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional WorkflowActor actor = 5;
 * @return {!proto.workflow.WorkflowActor}
 */
proto.workflow.WorkflowStep.prototype.getActor = function() {
  return /** @type {!proto.workflow.WorkflowActor} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {!proto.workflow.WorkflowActor} value
 * @return {!proto.workflow.WorkflowStep} returns this
 */
proto.workflow.WorkflowStep.prototype.setActor = function(value) {
  return jspb.Message.setProto3EnumField(this, 5, value);
};


/**
 * optional WorkflowPhaseKind phase = 6;
 * @return {!proto.workflow.WorkflowPhaseKind}
 */
proto.workflow.WorkflowStep.prototype.getPhase = function() {
  return /** @type {!proto.workflow.WorkflowPhaseKind} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {!proto.workflow.WorkflowPhaseKind} value
 * @return {!proto.workflow.WorkflowStep} returns this
 */
proto.workflow.WorkflowStep.prototype.setPhase = function(value) {
  return jspb.Message.setProto3EnumField(this, 6, value);
};


/**
 * optional StepStatus status = 7;
 * @return {!proto.workflow.StepStatus}
 */
proto.workflow.WorkflowStep.prototype.getStatus = function() {
  return /** @type {!proto.workflow.StepStatus} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/**
 * @param {!proto.workflow.StepStatus} value
 * @return {!proto.workflow.WorkflowStep} returns this
 */
proto.workflow.WorkflowStep.prototype.setStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 7, value);
};


/**
 * optional int32 attempt = 8;
 * @return {number}
 */
proto.workflow.WorkflowStep.prototype.getAttempt = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 8, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.WorkflowStep} returns this
 */
proto.workflow.WorkflowStep.prototype.setAttempt = function(value) {
  return jspb.Message.setProto3IntField(this, 8, value);
};


/**
 * optional google.protobuf.Timestamp created_at = 9;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.WorkflowStep.prototype.getCreatedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 9));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.WorkflowStep} returns this
*/
proto.workflow.WorkflowStep.prototype.setCreatedAt = function(value) {
  return jspb.Message.setWrapperField(this, 9, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowStep} returns this
 */
proto.workflow.WorkflowStep.prototype.clearCreatedAt = function() {
  return this.setCreatedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowStep.prototype.hasCreatedAt = function() {
  return jspb.Message.getField(this, 9) != null;
};


/**
 * optional google.protobuf.Timestamp started_at = 10;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.WorkflowStep.prototype.getStartedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 10));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.WorkflowStep} returns this
*/
proto.workflow.WorkflowStep.prototype.setStartedAt = function(value) {
  return jspb.Message.setWrapperField(this, 10, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowStep} returns this
 */
proto.workflow.WorkflowStep.prototype.clearStartedAt = function() {
  return this.setStartedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowStep.prototype.hasStartedAt = function() {
  return jspb.Message.getField(this, 10) != null;
};


/**
 * optional google.protobuf.Timestamp finished_at = 11;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.WorkflowStep.prototype.getFinishedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 11));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.WorkflowStep} returns this
*/
proto.workflow.WorkflowStep.prototype.setFinishedAt = function(value) {
  return jspb.Message.setWrapperField(this, 11, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowStep} returns this
 */
proto.workflow.WorkflowStep.prototype.clearFinishedAt = function() {
  return this.setFinishedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowStep.prototype.hasFinishedAt = function() {
  return jspb.Message.getField(this, 11) != null;
};


/**
 * optional int64 duration_ms = 12;
 * @return {number}
 */
proto.workflow.WorkflowStep.prototype.getDurationMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 12, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.WorkflowStep} returns this
 */
proto.workflow.WorkflowStep.prototype.setDurationMs = function(value) {
  return jspb.Message.setProto3IntField(this, 12, value);
};


/**
 * optional string message = 13;
 * @return {string}
 */
proto.workflow.WorkflowStep.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 13, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowStep} returns this
 */
proto.workflow.WorkflowStep.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 13, value);
};


/**
 * optional string error_code = 14;
 * @return {string}
 */
proto.workflow.WorkflowStep.prototype.getErrorCode = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 14, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowStep} returns this
 */
proto.workflow.WorkflowStep.prototype.setErrorCode = function(value) {
  return jspb.Message.setProto3StringField(this, 14, value);
};


/**
 * optional string error_message = 15;
 * @return {string}
 */
proto.workflow.WorkflowStep.prototype.getErrorMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 15, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowStep} returns this
 */
proto.workflow.WorkflowStep.prototype.setErrorMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 15, value);
};


/**
 * optional bool retryable = 16;
 * @return {boolean}
 */
proto.workflow.WorkflowStep.prototype.getRetryable = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 16, false));
};


/**
 * @param {boolean} value
 * @return {!proto.workflow.WorkflowStep} returns this
 */
proto.workflow.WorkflowStep.prototype.setRetryable = function(value) {
  return jspb.Message.setProto3BooleanField(this, 16, value);
};


/**
 * optional bool operator_action_required = 17;
 * @return {boolean}
 */
proto.workflow.WorkflowStep.prototype.getOperatorActionRequired = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 17, false));
};


/**
 * @param {boolean} value
 * @return {!proto.workflow.WorkflowStep} returns this
 */
proto.workflow.WorkflowStep.prototype.setOperatorActionRequired = function(value) {
  return jspb.Message.setProto3BooleanField(this, 17, value);
};


/**
 * optional string action_hint = 18;
 * @return {string}
 */
proto.workflow.WorkflowStep.prototype.getActionHint = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 18, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowStep} returns this
 */
proto.workflow.WorkflowStep.prototype.setActionHint = function(value) {
  return jspb.Message.setProto3StringField(this, 18, value);
};


/**
 * optional WorkflowActor source_actor = 19;
 * @return {!proto.workflow.WorkflowActor}
 */
proto.workflow.WorkflowStep.prototype.getSourceActor = function() {
  return /** @type {!proto.workflow.WorkflowActor} */ (jspb.Message.getFieldWithDefault(this, 19, 0));
};


/**
 * @param {!proto.workflow.WorkflowActor} value
 * @return {!proto.workflow.WorkflowStep} returns this
 */
proto.workflow.WorkflowStep.prototype.setSourceActor = function(value) {
  return jspb.Message.setProto3EnumField(this, 19, value);
};


/**
 * optional WorkflowActor target_actor = 20;
 * @return {!proto.workflow.WorkflowActor}
 */
proto.workflow.WorkflowStep.prototype.getTargetActor = function() {
  return /** @type {!proto.workflow.WorkflowActor} */ (jspb.Message.getFieldWithDefault(this, 20, 0));
};


/**
 * @param {!proto.workflow.WorkflowActor} value
 * @return {!proto.workflow.WorkflowStep} returns this
 */
proto.workflow.WorkflowStep.prototype.setTargetActor = function(value) {
  return jspb.Message.setProto3EnumField(this, 20, value);
};


/**
 * optional string details_json = 21;
 * @return {string}
 */
proto.workflow.WorkflowStep.prototype.getDetailsJson = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 21, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowStep} returns this
 */
proto.workflow.WorkflowStep.prototype.setDetailsJson = function(value) {
  return jspb.Message.setProto3StringField(this, 21, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.WorkflowArtifactRef.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.WorkflowArtifactRef.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.WorkflowArtifactRef} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowArtifactRef.toObject = function(includeInstance, msg) {
  var f, obj = {
id: jspb.Message.getFieldWithDefault(msg, 1, ""),
runId: jspb.Message.getFieldWithDefault(msg, 2, ""),
stepSeq: jspb.Message.getFieldWithDefault(msg, 3, 0),
kind: jspb.Message.getFieldWithDefault(msg, 4, 0),
name: jspb.Message.getFieldWithDefault(msg, 5, ""),
version: jspb.Message.getFieldWithDefault(msg, 6, ""),
digest: jspb.Message.getFieldWithDefault(msg, 7, ""),
nodeId: jspb.Message.getFieldWithDefault(msg, 8, ""),
path: jspb.Message.getFieldWithDefault(msg, 9, ""),
etcdKey: jspb.Message.getFieldWithDefault(msg, 10, ""),
unitName: jspb.Message.getFieldWithDefault(msg, 11, ""),
configPath: jspb.Message.getFieldWithDefault(msg, 12, ""),
packageName: jspb.Message.getFieldWithDefault(msg, 13, ""),
packageVersion: jspb.Message.getFieldWithDefault(msg, 14, ""),
specPath: jspb.Message.getFieldWithDefault(msg, 15, ""),
scriptPath: jspb.Message.getFieldWithDefault(msg, 16, ""),
metadataJson: jspb.Message.getFieldWithDefault(msg, 17, ""),
createdAt: (f = msg.getCreatedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.WorkflowArtifactRef}
 */
proto.workflow.WorkflowArtifactRef.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.WorkflowArtifactRef;
  return proto.workflow.WorkflowArtifactRef.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.WorkflowArtifactRef} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.WorkflowArtifactRef}
 */
proto.workflow.WorkflowArtifactRef.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setRunId(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setStepSeq(value);
      break;
    case 4:
      var value = /** @type {!proto.workflow.ArtifactKind} */ (reader.readEnum());
      msg.setKind(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setVersion(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setDigest(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeId(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setPath(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.setEtcdKey(value);
      break;
    case 11:
      var value = /** @type {string} */ (reader.readString());
      msg.setUnitName(value);
      break;
    case 12:
      var value = /** @type {string} */ (reader.readString());
      msg.setConfigPath(value);
      break;
    case 13:
      var value = /** @type {string} */ (reader.readString());
      msg.setPackageName(value);
      break;
    case 14:
      var value = /** @type {string} */ (reader.readString());
      msg.setPackageVersion(value);
      break;
    case 15:
      var value = /** @type {string} */ (reader.readString());
      msg.setSpecPath(value);
      break;
    case 16:
      var value = /** @type {string} */ (reader.readString());
      msg.setScriptPath(value);
      break;
    case 17:
      var value = /** @type {string} */ (reader.readString());
      msg.setMetadataJson(value);
      break;
    case 18:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setCreatedAt(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.WorkflowArtifactRef.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.WorkflowArtifactRef.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.WorkflowArtifactRef} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowArtifactRef.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getRunId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getStepSeq();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
  f = message.getKind();
  if (f !== 0.0) {
    writer.writeEnum(
      4,
      f
    );
  }
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getVersion();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getDigest();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
  f = message.getPath();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
  f = message.getEtcdKey();
  if (f.length > 0) {
    writer.writeString(
      10,
      f
    );
  }
  f = message.getUnitName();
  if (f.length > 0) {
    writer.writeString(
      11,
      f
    );
  }
  f = message.getConfigPath();
  if (f.length > 0) {
    writer.writeString(
      12,
      f
    );
  }
  f = message.getPackageName();
  if (f.length > 0) {
    writer.writeString(
      13,
      f
    );
  }
  f = message.getPackageVersion();
  if (f.length > 0) {
    writer.writeString(
      14,
      f
    );
  }
  f = message.getSpecPath();
  if (f.length > 0) {
    writer.writeString(
      15,
      f
    );
  }
  f = message.getScriptPath();
  if (f.length > 0) {
    writer.writeString(
      16,
      f
    );
  }
  f = message.getMetadataJson();
  if (f.length > 0) {
    writer.writeString(
      17,
      f
    );
  }
  f = message.getCreatedAt();
  if (f != null) {
    writer.writeMessage(
      18,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.workflow.WorkflowArtifactRef.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowArtifactRef} returns this
 */
proto.workflow.WorkflowArtifactRef.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string run_id = 2;
 * @return {string}
 */
proto.workflow.WorkflowArtifactRef.prototype.getRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowArtifactRef} returns this
 */
proto.workflow.WorkflowArtifactRef.prototype.setRunId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int32 step_seq = 3;
 * @return {number}
 */
proto.workflow.WorkflowArtifactRef.prototype.getStepSeq = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.WorkflowArtifactRef} returns this
 */
proto.workflow.WorkflowArtifactRef.prototype.setStepSeq = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional ArtifactKind kind = 4;
 * @return {!proto.workflow.ArtifactKind}
 */
proto.workflow.WorkflowArtifactRef.prototype.getKind = function() {
  return /** @type {!proto.workflow.ArtifactKind} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {!proto.workflow.ArtifactKind} value
 * @return {!proto.workflow.WorkflowArtifactRef} returns this
 */
proto.workflow.WorkflowArtifactRef.prototype.setKind = function(value) {
  return jspb.Message.setProto3EnumField(this, 4, value);
};


/**
 * optional string name = 5;
 * @return {string}
 */
proto.workflow.WorkflowArtifactRef.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowArtifactRef} returns this
 */
proto.workflow.WorkflowArtifactRef.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string version = 6;
 * @return {string}
 */
proto.workflow.WorkflowArtifactRef.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowArtifactRef} returns this
 */
proto.workflow.WorkflowArtifactRef.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string digest = 7;
 * @return {string}
 */
proto.workflow.WorkflowArtifactRef.prototype.getDigest = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowArtifactRef} returns this
 */
proto.workflow.WorkflowArtifactRef.prototype.setDigest = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional string node_id = 8;
 * @return {string}
 */
proto.workflow.WorkflowArtifactRef.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowArtifactRef} returns this
 */
proto.workflow.WorkflowArtifactRef.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};


/**
 * optional string path = 9;
 * @return {string}
 */
proto.workflow.WorkflowArtifactRef.prototype.getPath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowArtifactRef} returns this
 */
proto.workflow.WorkflowArtifactRef.prototype.setPath = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};


/**
 * optional string etcd_key = 10;
 * @return {string}
 */
proto.workflow.WorkflowArtifactRef.prototype.getEtcdKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 10, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowArtifactRef} returns this
 */
proto.workflow.WorkflowArtifactRef.prototype.setEtcdKey = function(value) {
  return jspb.Message.setProto3StringField(this, 10, value);
};


/**
 * optional string unit_name = 11;
 * @return {string}
 */
proto.workflow.WorkflowArtifactRef.prototype.getUnitName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 11, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowArtifactRef} returns this
 */
proto.workflow.WorkflowArtifactRef.prototype.setUnitName = function(value) {
  return jspb.Message.setProto3StringField(this, 11, value);
};


/**
 * optional string config_path = 12;
 * @return {string}
 */
proto.workflow.WorkflowArtifactRef.prototype.getConfigPath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 12, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowArtifactRef} returns this
 */
proto.workflow.WorkflowArtifactRef.prototype.setConfigPath = function(value) {
  return jspb.Message.setProto3StringField(this, 12, value);
};


/**
 * optional string package_name = 13;
 * @return {string}
 */
proto.workflow.WorkflowArtifactRef.prototype.getPackageName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 13, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowArtifactRef} returns this
 */
proto.workflow.WorkflowArtifactRef.prototype.setPackageName = function(value) {
  return jspb.Message.setProto3StringField(this, 13, value);
};


/**
 * optional string package_version = 14;
 * @return {string}
 */
proto.workflow.WorkflowArtifactRef.prototype.getPackageVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 14, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowArtifactRef} returns this
 */
proto.workflow.WorkflowArtifactRef.prototype.setPackageVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 14, value);
};


/**
 * optional string spec_path = 15;
 * @return {string}
 */
proto.workflow.WorkflowArtifactRef.prototype.getSpecPath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 15, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowArtifactRef} returns this
 */
proto.workflow.WorkflowArtifactRef.prototype.setSpecPath = function(value) {
  return jspb.Message.setProto3StringField(this, 15, value);
};


/**
 * optional string script_path = 16;
 * @return {string}
 */
proto.workflow.WorkflowArtifactRef.prototype.getScriptPath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 16, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowArtifactRef} returns this
 */
proto.workflow.WorkflowArtifactRef.prototype.setScriptPath = function(value) {
  return jspb.Message.setProto3StringField(this, 16, value);
};


/**
 * optional string metadata_json = 17;
 * @return {string}
 */
proto.workflow.WorkflowArtifactRef.prototype.getMetadataJson = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 17, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowArtifactRef} returns this
 */
proto.workflow.WorkflowArtifactRef.prototype.setMetadataJson = function(value) {
  return jspb.Message.setProto3StringField(this, 17, value);
};


/**
 * optional google.protobuf.Timestamp created_at = 18;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.WorkflowArtifactRef.prototype.getCreatedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 18));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.WorkflowArtifactRef} returns this
*/
proto.workflow.WorkflowArtifactRef.prototype.setCreatedAt = function(value) {
  return jspb.Message.setWrapperField(this, 18, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowArtifactRef} returns this
 */
proto.workflow.WorkflowArtifactRef.prototype.clearCreatedAt = function() {
  return this.setCreatedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowArtifactRef.prototype.hasCreatedAt = function() {
  return jspb.Message.getField(this, 18) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.WorkflowEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.WorkflowEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.WorkflowEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
runId: jspb.Message.getFieldWithDefault(msg, 1, ""),
eventId: jspb.Message.getFieldWithDefault(msg, 2, ""),
stepSeq: jspb.Message.getFieldWithDefault(msg, 3, 0),
eventType: jspb.Message.getFieldWithDefault(msg, 4, ""),
actor: jspb.Message.getFieldWithDefault(msg, 5, 0),
oldValue: jspb.Message.getFieldWithDefault(msg, 6, ""),
newValue: jspb.Message.getFieldWithDefault(msg, 7, ""),
message: jspb.Message.getFieldWithDefault(msg, 8, ""),
createdAt: (f = msg.getCreatedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.WorkflowEvent}
 */
proto.workflow.WorkflowEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.WorkflowEvent;
  return proto.workflow.WorkflowEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.WorkflowEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.WorkflowEvent}
 */
proto.workflow.WorkflowEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRunId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setEventId(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setStepSeq(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setEventType(value);
      break;
    case 5:
      var value = /** @type {!proto.workflow.WorkflowActor} */ (reader.readEnum());
      msg.setActor(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setOldValue(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setNewValue(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
      break;
    case 9:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setCreatedAt(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.WorkflowEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.WorkflowEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.WorkflowEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRunId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getEventId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getStepSeq();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
  f = message.getEventType();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getActor();
  if (f !== 0.0) {
    writer.writeEnum(
      5,
      f
    );
  }
  f = message.getOldValue();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getNewValue();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getMessage();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
  f = message.getCreatedAt();
  if (f != null) {
    writer.writeMessage(
      9,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
};


/**
 * optional string run_id = 1;
 * @return {string}
 */
proto.workflow.WorkflowEvent.prototype.getRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowEvent} returns this
 */
proto.workflow.WorkflowEvent.prototype.setRunId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string event_id = 2;
 * @return {string}
 */
proto.workflow.WorkflowEvent.prototype.getEventId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowEvent} returns this
 */
proto.workflow.WorkflowEvent.prototype.setEventId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int32 step_seq = 3;
 * @return {number}
 */
proto.workflow.WorkflowEvent.prototype.getStepSeq = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.WorkflowEvent} returns this
 */
proto.workflow.WorkflowEvent.prototype.setStepSeq = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional string event_type = 4;
 * @return {string}
 */
proto.workflow.WorkflowEvent.prototype.getEventType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowEvent} returns this
 */
proto.workflow.WorkflowEvent.prototype.setEventType = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional WorkflowActor actor = 5;
 * @return {!proto.workflow.WorkflowActor}
 */
proto.workflow.WorkflowEvent.prototype.getActor = function() {
  return /** @type {!proto.workflow.WorkflowActor} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {!proto.workflow.WorkflowActor} value
 * @return {!proto.workflow.WorkflowEvent} returns this
 */
proto.workflow.WorkflowEvent.prototype.setActor = function(value) {
  return jspb.Message.setProto3EnumField(this, 5, value);
};


/**
 * optional string old_value = 6;
 * @return {string}
 */
proto.workflow.WorkflowEvent.prototype.getOldValue = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowEvent} returns this
 */
proto.workflow.WorkflowEvent.prototype.setOldValue = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string new_value = 7;
 * @return {string}
 */
proto.workflow.WorkflowEvent.prototype.getNewValue = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowEvent} returns this
 */
proto.workflow.WorkflowEvent.prototype.setNewValue = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional string message = 8;
 * @return {string}
 */
proto.workflow.WorkflowEvent.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowEvent} returns this
 */
proto.workflow.WorkflowEvent.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};


/**
 * optional google.protobuf.Timestamp created_at = 9;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.WorkflowEvent.prototype.getCreatedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 9));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.WorkflowEvent} returns this
*/
proto.workflow.WorkflowEvent.prototype.setCreatedAt = function(value) {
  return jspb.Message.setWrapperField(this, 9, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowEvent} returns this
 */
proto.workflow.WorkflowEvent.prototype.clearCreatedAt = function() {
  return this.setCreatedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowEvent.prototype.hasCreatedAt = function() {
  return jspb.Message.getField(this, 9) != null;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.workflow.WorkflowPhase.repeatedFields_ = [4];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.WorkflowPhase.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.WorkflowPhase.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.WorkflowPhase} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowPhase.toObject = function(includeInstance, msg) {
  var f, obj = {
kind: jspb.Message.getFieldWithDefault(msg, 1, 0),
displayName: jspb.Message.getFieldWithDefault(msg, 2, ""),
status: jspb.Message.getFieldWithDefault(msg, 3, 0),
stepsList: jspb.Message.toObjectList(msg.getStepsList(),
    proto.workflow.WorkflowStep.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.WorkflowPhase}
 */
proto.workflow.WorkflowPhase.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.WorkflowPhase;
  return proto.workflow.WorkflowPhase.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.WorkflowPhase} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.WorkflowPhase}
 */
proto.workflow.WorkflowPhase.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.workflow.WorkflowPhaseKind} */ (reader.readEnum());
      msg.setKind(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setDisplayName(value);
      break;
    case 3:
      var value = /** @type {!proto.workflow.StepStatus} */ (reader.readEnum());
      msg.setStatus(value);
      break;
    case 4:
      var value = new proto.workflow.WorkflowStep;
      reader.readMessage(value,proto.workflow.WorkflowStep.deserializeBinaryFromReader);
      msg.addSteps(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.WorkflowPhase.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.WorkflowPhase.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.WorkflowPhase} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowPhase.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getKind();
  if (f !== 0.0) {
    writer.writeEnum(
      1,
      f
    );
  }
  f = message.getDisplayName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      3,
      f
    );
  }
  f = message.getStepsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      4,
      f,
      proto.workflow.WorkflowStep.serializeBinaryToWriter
    );
  }
};


/**
 * optional WorkflowPhaseKind kind = 1;
 * @return {!proto.workflow.WorkflowPhaseKind}
 */
proto.workflow.WorkflowPhase.prototype.getKind = function() {
  return /** @type {!proto.workflow.WorkflowPhaseKind} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {!proto.workflow.WorkflowPhaseKind} value
 * @return {!proto.workflow.WorkflowPhase} returns this
 */
proto.workflow.WorkflowPhase.prototype.setKind = function(value) {
  return jspb.Message.setProto3EnumField(this, 1, value);
};


/**
 * optional string display_name = 2;
 * @return {string}
 */
proto.workflow.WorkflowPhase.prototype.getDisplayName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowPhase} returns this
 */
proto.workflow.WorkflowPhase.prototype.setDisplayName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional StepStatus status = 3;
 * @return {!proto.workflow.StepStatus}
 */
proto.workflow.WorkflowPhase.prototype.getStatus = function() {
  return /** @type {!proto.workflow.StepStatus} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.workflow.StepStatus} value
 * @return {!proto.workflow.WorkflowPhase} returns this
 */
proto.workflow.WorkflowPhase.prototype.setStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
};


/**
 * repeated WorkflowStep steps = 4;
 * @return {!Array<!proto.workflow.WorkflowStep>}
 */
proto.workflow.WorkflowPhase.prototype.getStepsList = function() {
  return /** @type{!Array<!proto.workflow.WorkflowStep>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.workflow.WorkflowStep, 4));
};


/**
 * @param {!Array<!proto.workflow.WorkflowStep>} value
 * @return {!proto.workflow.WorkflowPhase} returns this
*/
proto.workflow.WorkflowPhase.prototype.setStepsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 4, value);
};


/**
 * @param {!proto.workflow.WorkflowStep=} opt_value
 * @param {number=} opt_index
 * @return {!proto.workflow.WorkflowStep}
 */
proto.workflow.WorkflowPhase.prototype.addSteps = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 4, opt_value, proto.workflow.WorkflowStep, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.WorkflowPhase} returns this
 */
proto.workflow.WorkflowPhase.prototype.clearStepsList = function() {
  return this.setStepsList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.workflow.WorkflowActorLane.repeatedFields_ = [2];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.WorkflowActorLane.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.WorkflowActorLane.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.WorkflowActorLane} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowActorLane.toObject = function(includeInstance, msg) {
  var f, obj = {
actor: jspb.Message.getFieldWithDefault(msg, 1, 0),
stepsList: jspb.Message.toObjectList(msg.getStepsList(),
    proto.workflow.WorkflowStep.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.WorkflowActorLane}
 */
proto.workflow.WorkflowActorLane.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.WorkflowActorLane;
  return proto.workflow.WorkflowActorLane.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.WorkflowActorLane} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.WorkflowActorLane}
 */
proto.workflow.WorkflowActorLane.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.workflow.WorkflowActor} */ (reader.readEnum());
      msg.setActor(value);
      break;
    case 2:
      var value = new proto.workflow.WorkflowStep;
      reader.readMessage(value,proto.workflow.WorkflowStep.deserializeBinaryFromReader);
      msg.addSteps(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.WorkflowActorLane.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.WorkflowActorLane.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.WorkflowActorLane} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowActorLane.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getActor();
  if (f !== 0.0) {
    writer.writeEnum(
      1,
      f
    );
  }
  f = message.getStepsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      2,
      f,
      proto.workflow.WorkflowStep.serializeBinaryToWriter
    );
  }
};


/**
 * optional WorkflowActor actor = 1;
 * @return {!proto.workflow.WorkflowActor}
 */
proto.workflow.WorkflowActorLane.prototype.getActor = function() {
  return /** @type {!proto.workflow.WorkflowActor} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {!proto.workflow.WorkflowActor} value
 * @return {!proto.workflow.WorkflowActorLane} returns this
 */
proto.workflow.WorkflowActorLane.prototype.setActor = function(value) {
  return jspb.Message.setProto3EnumField(this, 1, value);
};


/**
 * repeated WorkflowStep steps = 2;
 * @return {!Array<!proto.workflow.WorkflowStep>}
 */
proto.workflow.WorkflowActorLane.prototype.getStepsList = function() {
  return /** @type{!Array<!proto.workflow.WorkflowStep>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.workflow.WorkflowStep, 2));
};


/**
 * @param {!Array<!proto.workflow.WorkflowStep>} value
 * @return {!proto.workflow.WorkflowActorLane} returns this
*/
proto.workflow.WorkflowActorLane.prototype.setStepsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 2, value);
};


/**
 * @param {!proto.workflow.WorkflowStep=} opt_value
 * @param {number=} opt_index
 * @return {!proto.workflow.WorkflowStep}
 */
proto.workflow.WorkflowActorLane.prototype.addSteps = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 2, opt_value, proto.workflow.WorkflowStep, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.WorkflowActorLane} returns this
 */
proto.workflow.WorkflowActorLane.prototype.clearStepsList = function() {
  return this.setStepsList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.workflow.WorkflowGraph.repeatedFields_ = [2,3,4];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.WorkflowGraph.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.WorkflowGraph.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.WorkflowGraph} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowGraph.toObject = function(includeInstance, msg) {
  var f, obj = {
run: (f = msg.getRun()) && proto.workflow.WorkflowRun.toObject(includeInstance, f),
phasesList: jspb.Message.toObjectList(msg.getPhasesList(),
    proto.workflow.WorkflowPhase.toObject, includeInstance),
lanesList: jspb.Message.toObjectList(msg.getLanesList(),
    proto.workflow.WorkflowActorLane.toObject, includeInstance),
artifactsList: jspb.Message.toObjectList(msg.getArtifactsList(),
    proto.workflow.WorkflowArtifactRef.toObject, includeInstance),
currentStepSeq: jspb.Message.getFieldWithDefault(msg, 5, 0),
currentActor: jspb.Message.getFieldWithDefault(msg, 6, 0),
blockedReason: jspb.Message.getFieldWithDefault(msg, 7, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.WorkflowGraph}
 */
proto.workflow.WorkflowGraph.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.WorkflowGraph;
  return proto.workflow.WorkflowGraph.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.WorkflowGraph} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.WorkflowGraph}
 */
proto.workflow.WorkflowGraph.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.workflow.WorkflowRun;
      reader.readMessage(value,proto.workflow.WorkflowRun.deserializeBinaryFromReader);
      msg.setRun(value);
      break;
    case 2:
      var value = new proto.workflow.WorkflowPhase;
      reader.readMessage(value,proto.workflow.WorkflowPhase.deserializeBinaryFromReader);
      msg.addPhases(value);
      break;
    case 3:
      var value = new proto.workflow.WorkflowActorLane;
      reader.readMessage(value,proto.workflow.WorkflowActorLane.deserializeBinaryFromReader);
      msg.addLanes(value);
      break;
    case 4:
      var value = new proto.workflow.WorkflowArtifactRef;
      reader.readMessage(value,proto.workflow.WorkflowArtifactRef.deserializeBinaryFromReader);
      msg.addArtifacts(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setCurrentStepSeq(value);
      break;
    case 6:
      var value = /** @type {!proto.workflow.WorkflowActor} */ (reader.readEnum());
      msg.setCurrentActor(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setBlockedReason(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.WorkflowGraph.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.WorkflowGraph.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.WorkflowGraph} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowGraph.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRun();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.workflow.WorkflowRun.serializeBinaryToWriter
    );
  }
  f = message.getPhasesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      2,
      f,
      proto.workflow.WorkflowPhase.serializeBinaryToWriter
    );
  }
  f = message.getLanesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      3,
      f,
      proto.workflow.WorkflowActorLane.serializeBinaryToWriter
    );
  }
  f = message.getArtifactsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      4,
      f,
      proto.workflow.WorkflowArtifactRef.serializeBinaryToWriter
    );
  }
  f = message.getCurrentStepSeq();
  if (f !== 0) {
    writer.writeInt32(
      5,
      f
    );
  }
  f = message.getCurrentActor();
  if (f !== 0.0) {
    writer.writeEnum(
      6,
      f
    );
  }
  f = message.getBlockedReason();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
};


/**
 * optional WorkflowRun run = 1;
 * @return {?proto.workflow.WorkflowRun}
 */
proto.workflow.WorkflowGraph.prototype.getRun = function() {
  return /** @type{?proto.workflow.WorkflowRun} */ (
    jspb.Message.getWrapperField(this, proto.workflow.WorkflowRun, 1));
};


/**
 * @param {?proto.workflow.WorkflowRun|undefined} value
 * @return {!proto.workflow.WorkflowGraph} returns this
*/
proto.workflow.WorkflowGraph.prototype.setRun = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowGraph} returns this
 */
proto.workflow.WorkflowGraph.prototype.clearRun = function() {
  return this.setRun(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowGraph.prototype.hasRun = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * repeated WorkflowPhase phases = 2;
 * @return {!Array<!proto.workflow.WorkflowPhase>}
 */
proto.workflow.WorkflowGraph.prototype.getPhasesList = function() {
  return /** @type{!Array<!proto.workflow.WorkflowPhase>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.workflow.WorkflowPhase, 2));
};


/**
 * @param {!Array<!proto.workflow.WorkflowPhase>} value
 * @return {!proto.workflow.WorkflowGraph} returns this
*/
proto.workflow.WorkflowGraph.prototype.setPhasesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 2, value);
};


/**
 * @param {!proto.workflow.WorkflowPhase=} opt_value
 * @param {number=} opt_index
 * @return {!proto.workflow.WorkflowPhase}
 */
proto.workflow.WorkflowGraph.prototype.addPhases = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 2, opt_value, proto.workflow.WorkflowPhase, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.WorkflowGraph} returns this
 */
proto.workflow.WorkflowGraph.prototype.clearPhasesList = function() {
  return this.setPhasesList([]);
};


/**
 * repeated WorkflowActorLane lanes = 3;
 * @return {!Array<!proto.workflow.WorkflowActorLane>}
 */
proto.workflow.WorkflowGraph.prototype.getLanesList = function() {
  return /** @type{!Array<!proto.workflow.WorkflowActorLane>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.workflow.WorkflowActorLane, 3));
};


/**
 * @param {!Array<!proto.workflow.WorkflowActorLane>} value
 * @return {!proto.workflow.WorkflowGraph} returns this
*/
proto.workflow.WorkflowGraph.prototype.setLanesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 3, value);
};


/**
 * @param {!proto.workflow.WorkflowActorLane=} opt_value
 * @param {number=} opt_index
 * @return {!proto.workflow.WorkflowActorLane}
 */
proto.workflow.WorkflowGraph.prototype.addLanes = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 3, opt_value, proto.workflow.WorkflowActorLane, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.WorkflowGraph} returns this
 */
proto.workflow.WorkflowGraph.prototype.clearLanesList = function() {
  return this.setLanesList([]);
};


/**
 * repeated WorkflowArtifactRef artifacts = 4;
 * @return {!Array<!proto.workflow.WorkflowArtifactRef>}
 */
proto.workflow.WorkflowGraph.prototype.getArtifactsList = function() {
  return /** @type{!Array<!proto.workflow.WorkflowArtifactRef>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.workflow.WorkflowArtifactRef, 4));
};


/**
 * @param {!Array<!proto.workflow.WorkflowArtifactRef>} value
 * @return {!proto.workflow.WorkflowGraph} returns this
*/
proto.workflow.WorkflowGraph.prototype.setArtifactsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 4, value);
};


/**
 * @param {!proto.workflow.WorkflowArtifactRef=} opt_value
 * @param {number=} opt_index
 * @return {!proto.workflow.WorkflowArtifactRef}
 */
proto.workflow.WorkflowGraph.prototype.addArtifacts = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 4, opt_value, proto.workflow.WorkflowArtifactRef, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.WorkflowGraph} returns this
 */
proto.workflow.WorkflowGraph.prototype.clearArtifactsList = function() {
  return this.setArtifactsList([]);
};


/**
 * optional int32 current_step_seq = 5;
 * @return {number}
 */
proto.workflow.WorkflowGraph.prototype.getCurrentStepSeq = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.WorkflowGraph} returns this
 */
proto.workflow.WorkflowGraph.prototype.setCurrentStepSeq = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional WorkflowActor current_actor = 6;
 * @return {!proto.workflow.WorkflowActor}
 */
proto.workflow.WorkflowGraph.prototype.getCurrentActor = function() {
  return /** @type {!proto.workflow.WorkflowActor} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {!proto.workflow.WorkflowActor} value
 * @return {!proto.workflow.WorkflowGraph} returns this
 */
proto.workflow.WorkflowGraph.prototype.setCurrentActor = function(value) {
  return jspb.Message.setProto3EnumField(this, 6, value);
};


/**
 * optional string blocked_reason = 7;
 * @return {string}
 */
proto.workflow.WorkflowGraph.prototype.getBlockedReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowGraph} returns this
 */
proto.workflow.WorkflowGraph.prototype.setBlockedReason = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.workflow.WorkflowRunDetail.repeatedFields_ = [2,3];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.WorkflowRunDetail.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.WorkflowRunDetail.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.WorkflowRunDetail} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowRunDetail.toObject = function(includeInstance, msg) {
  var f, obj = {
run: (f = msg.getRun()) && proto.workflow.WorkflowRun.toObject(includeInstance, f),
stepsList: jspb.Message.toObjectList(msg.getStepsList(),
    proto.workflow.WorkflowStep.toObject, includeInstance),
artifactsList: jspb.Message.toObjectList(msg.getArtifactsList(),
    proto.workflow.WorkflowArtifactRef.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.WorkflowRunDetail}
 */
proto.workflow.WorkflowRunDetail.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.WorkflowRunDetail;
  return proto.workflow.WorkflowRunDetail.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.WorkflowRunDetail} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.WorkflowRunDetail}
 */
proto.workflow.WorkflowRunDetail.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.workflow.WorkflowRun;
      reader.readMessage(value,proto.workflow.WorkflowRun.deserializeBinaryFromReader);
      msg.setRun(value);
      break;
    case 2:
      var value = new proto.workflow.WorkflowStep;
      reader.readMessage(value,proto.workflow.WorkflowStep.deserializeBinaryFromReader);
      msg.addSteps(value);
      break;
    case 3:
      var value = new proto.workflow.WorkflowArtifactRef;
      reader.readMessage(value,proto.workflow.WorkflowArtifactRef.deserializeBinaryFromReader);
      msg.addArtifacts(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.WorkflowRunDetail.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.WorkflowRunDetail.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.WorkflowRunDetail} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowRunDetail.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRun();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.workflow.WorkflowRun.serializeBinaryToWriter
    );
  }
  f = message.getStepsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      2,
      f,
      proto.workflow.WorkflowStep.serializeBinaryToWriter
    );
  }
  f = message.getArtifactsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      3,
      f,
      proto.workflow.WorkflowArtifactRef.serializeBinaryToWriter
    );
  }
};


/**
 * optional WorkflowRun run = 1;
 * @return {?proto.workflow.WorkflowRun}
 */
proto.workflow.WorkflowRunDetail.prototype.getRun = function() {
  return /** @type{?proto.workflow.WorkflowRun} */ (
    jspb.Message.getWrapperField(this, proto.workflow.WorkflowRun, 1));
};


/**
 * @param {?proto.workflow.WorkflowRun|undefined} value
 * @return {!proto.workflow.WorkflowRunDetail} returns this
*/
proto.workflow.WorkflowRunDetail.prototype.setRun = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowRunDetail} returns this
 */
proto.workflow.WorkflowRunDetail.prototype.clearRun = function() {
  return this.setRun(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowRunDetail.prototype.hasRun = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * repeated WorkflowStep steps = 2;
 * @return {!Array<!proto.workflow.WorkflowStep>}
 */
proto.workflow.WorkflowRunDetail.prototype.getStepsList = function() {
  return /** @type{!Array<!proto.workflow.WorkflowStep>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.workflow.WorkflowStep, 2));
};


/**
 * @param {!Array<!proto.workflow.WorkflowStep>} value
 * @return {!proto.workflow.WorkflowRunDetail} returns this
*/
proto.workflow.WorkflowRunDetail.prototype.setStepsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 2, value);
};


/**
 * @param {!proto.workflow.WorkflowStep=} opt_value
 * @param {number=} opt_index
 * @return {!proto.workflow.WorkflowStep}
 */
proto.workflow.WorkflowRunDetail.prototype.addSteps = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 2, opt_value, proto.workflow.WorkflowStep, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.WorkflowRunDetail} returns this
 */
proto.workflow.WorkflowRunDetail.prototype.clearStepsList = function() {
  return this.setStepsList([]);
};


/**
 * repeated WorkflowArtifactRef artifacts = 3;
 * @return {!Array<!proto.workflow.WorkflowArtifactRef>}
 */
proto.workflow.WorkflowRunDetail.prototype.getArtifactsList = function() {
  return /** @type{!Array<!proto.workflow.WorkflowArtifactRef>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.workflow.WorkflowArtifactRef, 3));
};


/**
 * @param {!Array<!proto.workflow.WorkflowArtifactRef>} value
 * @return {!proto.workflow.WorkflowRunDetail} returns this
*/
proto.workflow.WorkflowRunDetail.prototype.setArtifactsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 3, value);
};


/**
 * @param {!proto.workflow.WorkflowArtifactRef=} opt_value
 * @param {number=} opt_index
 * @return {!proto.workflow.WorkflowArtifactRef}
 */
proto.workflow.WorkflowRunDetail.prototype.addArtifacts = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 3, opt_value, proto.workflow.WorkflowArtifactRef, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.WorkflowRunDetail} returns this
 */
proto.workflow.WorkflowRunDetail.prototype.clearArtifactsList = function() {
  return this.setArtifactsList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.StartRunRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.StartRunRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.StartRunRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.StartRunRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
run: (f = msg.getRun()) && proto.workflow.WorkflowRun.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.StartRunRequest}
 */
proto.workflow.StartRunRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.StartRunRequest;
  return proto.workflow.StartRunRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.StartRunRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.StartRunRequest}
 */
proto.workflow.StartRunRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.workflow.WorkflowRun;
      reader.readMessage(value,proto.workflow.WorkflowRun.deserializeBinaryFromReader);
      msg.setRun(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.StartRunRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.StartRunRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.StartRunRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.StartRunRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRun();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.workflow.WorkflowRun.serializeBinaryToWriter
    );
  }
};


/**
 * optional WorkflowRun run = 1;
 * @return {?proto.workflow.WorkflowRun}
 */
proto.workflow.StartRunRequest.prototype.getRun = function() {
  return /** @type{?proto.workflow.WorkflowRun} */ (
    jspb.Message.getWrapperField(this, proto.workflow.WorkflowRun, 1));
};


/**
 * @param {?proto.workflow.WorkflowRun|undefined} value
 * @return {!proto.workflow.StartRunRequest} returns this
*/
proto.workflow.StartRunRequest.prototype.setRun = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.StartRunRequest} returns this
 */
proto.workflow.StartRunRequest.prototype.clearRun = function() {
  return this.setRun(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.StartRunRequest.prototype.hasRun = function() {
  return jspb.Message.getField(this, 1) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.UpdateRunRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.UpdateRunRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.UpdateRunRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.UpdateRunRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
id: jspb.Message.getFieldWithDefault(msg, 1, ""),
clusterId: jspb.Message.getFieldWithDefault(msg, 2, ""),
status: jspb.Message.getFieldWithDefault(msg, 3, 0),
summary: jspb.Message.getFieldWithDefault(msg, 4, ""),
currentActor: jspb.Message.getFieldWithDefault(msg, 7, 0),
waitReason: jspb.Message.getFieldWithDefault(msg, 8, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.UpdateRunRequest}
 */
proto.workflow.UpdateRunRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.UpdateRunRequest;
  return proto.workflow.UpdateRunRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.UpdateRunRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.UpdateRunRequest}
 */
proto.workflow.UpdateRunRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 3:
      var value = /** @type {!proto.workflow.RunStatus} */ (reader.readEnum());
      msg.setStatus(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setSummary(value);
      break;
    case 7:
      var value = /** @type {!proto.workflow.WorkflowActor} */ (reader.readEnum());
      msg.setCurrentActor(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setWaitReason(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.UpdateRunRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.UpdateRunRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.UpdateRunRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.UpdateRunRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      3,
      f
    );
  }
  f = message.getSummary();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getCurrentActor();
  if (f !== 0.0) {
    writer.writeEnum(
      7,
      f
    );
  }
  f = message.getWaitReason();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.workflow.UpdateRunRequest.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.UpdateRunRequest} returns this
 */
proto.workflow.UpdateRunRequest.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string cluster_id = 2;
 * @return {string}
 */
proto.workflow.UpdateRunRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.UpdateRunRequest} returns this
 */
proto.workflow.UpdateRunRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional RunStatus status = 3;
 * @return {!proto.workflow.RunStatus}
 */
proto.workflow.UpdateRunRequest.prototype.getStatus = function() {
  return /** @type {!proto.workflow.RunStatus} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.workflow.RunStatus} value
 * @return {!proto.workflow.UpdateRunRequest} returns this
 */
proto.workflow.UpdateRunRequest.prototype.setStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
};


/**
 * optional string summary = 4;
 * @return {string}
 */
proto.workflow.UpdateRunRequest.prototype.getSummary = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.UpdateRunRequest} returns this
 */
proto.workflow.UpdateRunRequest.prototype.setSummary = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional WorkflowActor current_actor = 7;
 * @return {!proto.workflow.WorkflowActor}
 */
proto.workflow.UpdateRunRequest.prototype.getCurrentActor = function() {
  return /** @type {!proto.workflow.WorkflowActor} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/**
 * @param {!proto.workflow.WorkflowActor} value
 * @return {!proto.workflow.UpdateRunRequest} returns this
 */
proto.workflow.UpdateRunRequest.prototype.setCurrentActor = function(value) {
  return jspb.Message.setProto3EnumField(this, 7, value);
};


/**
 * optional string wait_reason = 8;
 * @return {string}
 */
proto.workflow.UpdateRunRequest.prototype.getWaitReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.UpdateRunRequest} returns this
 */
proto.workflow.UpdateRunRequest.prototype.setWaitReason = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.FinishRunRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.FinishRunRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.FinishRunRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.FinishRunRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
id: jspb.Message.getFieldWithDefault(msg, 1, ""),
clusterId: jspb.Message.getFieldWithDefault(msg, 2, ""),
status: jspb.Message.getFieldWithDefault(msg, 3, 0),
failureClass: jspb.Message.getFieldWithDefault(msg, 4, 0),
summary: jspb.Message.getFieldWithDefault(msg, 5, ""),
errorMessage: jspb.Message.getFieldWithDefault(msg, 6, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.FinishRunRequest}
 */
proto.workflow.FinishRunRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.FinishRunRequest;
  return proto.workflow.FinishRunRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.FinishRunRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.FinishRunRequest}
 */
proto.workflow.FinishRunRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 3:
      var value = /** @type {!proto.workflow.RunStatus} */ (reader.readEnum());
      msg.setStatus(value);
      break;
    case 4:
      var value = /** @type {!proto.workflow.FailureClass} */ (reader.readEnum());
      msg.setFailureClass(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setSummary(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setErrorMessage(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.FinishRunRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.FinishRunRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.FinishRunRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.FinishRunRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      3,
      f
    );
  }
  f = message.getFailureClass();
  if (f !== 0.0) {
    writer.writeEnum(
      4,
      f
    );
  }
  f = message.getSummary();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getErrorMessage();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.workflow.FinishRunRequest.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.FinishRunRequest} returns this
 */
proto.workflow.FinishRunRequest.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string cluster_id = 2;
 * @return {string}
 */
proto.workflow.FinishRunRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.FinishRunRequest} returns this
 */
proto.workflow.FinishRunRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional RunStatus status = 3;
 * @return {!proto.workflow.RunStatus}
 */
proto.workflow.FinishRunRequest.prototype.getStatus = function() {
  return /** @type {!proto.workflow.RunStatus} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.workflow.RunStatus} value
 * @return {!proto.workflow.FinishRunRequest} returns this
 */
proto.workflow.FinishRunRequest.prototype.setStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
};


/**
 * optional FailureClass failure_class = 4;
 * @return {!proto.workflow.FailureClass}
 */
proto.workflow.FinishRunRequest.prototype.getFailureClass = function() {
  return /** @type {!proto.workflow.FailureClass} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {!proto.workflow.FailureClass} value
 * @return {!proto.workflow.FinishRunRequest} returns this
 */
proto.workflow.FinishRunRequest.prototype.setFailureClass = function(value) {
  return jspb.Message.setProto3EnumField(this, 4, value);
};


/**
 * optional string summary = 5;
 * @return {string}
 */
proto.workflow.FinishRunRequest.prototype.getSummary = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.FinishRunRequest} returns this
 */
proto.workflow.FinishRunRequest.prototype.setSummary = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string error_message = 6;
 * @return {string}
 */
proto.workflow.FinishRunRequest.prototype.getErrorMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.FinishRunRequest} returns this
 */
proto.workflow.FinishRunRequest.prototype.setErrorMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.RecordStepRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.RecordStepRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.RecordStepRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.RecordStepRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
step: (f = msg.getStep()) && proto.workflow.WorkflowStep.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.RecordStepRequest}
 */
proto.workflow.RecordStepRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.RecordStepRequest;
  return proto.workflow.RecordStepRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.RecordStepRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.RecordStepRequest}
 */
proto.workflow.RecordStepRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = new proto.workflow.WorkflowStep;
      reader.readMessage(value,proto.workflow.WorkflowStep.deserializeBinaryFromReader);
      msg.setStep(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.RecordStepRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.RecordStepRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.RecordStepRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.RecordStepRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getStep();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.workflow.WorkflowStep.serializeBinaryToWriter
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.RecordStepRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordStepRequest} returns this
 */
proto.workflow.RecordStepRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional WorkflowStep step = 2;
 * @return {?proto.workflow.WorkflowStep}
 */
proto.workflow.RecordStepRequest.prototype.getStep = function() {
  return /** @type{?proto.workflow.WorkflowStep} */ (
    jspb.Message.getWrapperField(this, proto.workflow.WorkflowStep, 2));
};


/**
 * @param {?proto.workflow.WorkflowStep|undefined} value
 * @return {!proto.workflow.RecordStepRequest} returns this
*/
proto.workflow.RecordStepRequest.prototype.setStep = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.RecordStepRequest} returns this
 */
proto.workflow.RecordStepRequest.prototype.clearStep = function() {
  return this.setStep(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.RecordStepRequest.prototype.hasStep = function() {
  return jspb.Message.getField(this, 2) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.UpdateStepRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.UpdateStepRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.UpdateStepRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.UpdateStepRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
runId: jspb.Message.getFieldWithDefault(msg, 2, ""),
seq: jspb.Message.getFieldWithDefault(msg, 3, 0),
status: jspb.Message.getFieldWithDefault(msg, 4, 0),
message: jspb.Message.getFieldWithDefault(msg, 5, ""),
durationMs: jspb.Message.getFieldWithDefault(msg, 6, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.UpdateStepRequest}
 */
proto.workflow.UpdateStepRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.UpdateStepRequest;
  return proto.workflow.UpdateStepRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.UpdateStepRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.UpdateStepRequest}
 */
proto.workflow.UpdateStepRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setRunId(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setSeq(value);
      break;
    case 4:
      var value = /** @type {!proto.workflow.StepStatus} */ (reader.readEnum());
      msg.setStatus(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setDurationMs(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.UpdateStepRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.UpdateStepRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.UpdateStepRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.UpdateStepRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getRunId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getSeq();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
  f = message.getStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      4,
      f
    );
  }
  f = message.getMessage();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getDurationMs();
  if (f !== 0) {
    writer.writeInt64(
      6,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.UpdateStepRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.UpdateStepRequest} returns this
 */
proto.workflow.UpdateStepRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string run_id = 2;
 * @return {string}
 */
proto.workflow.UpdateStepRequest.prototype.getRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.UpdateStepRequest} returns this
 */
proto.workflow.UpdateStepRequest.prototype.setRunId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int32 seq = 3;
 * @return {number}
 */
proto.workflow.UpdateStepRequest.prototype.getSeq = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.UpdateStepRequest} returns this
 */
proto.workflow.UpdateStepRequest.prototype.setSeq = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional StepStatus status = 4;
 * @return {!proto.workflow.StepStatus}
 */
proto.workflow.UpdateStepRequest.prototype.getStatus = function() {
  return /** @type {!proto.workflow.StepStatus} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {!proto.workflow.StepStatus} value
 * @return {!proto.workflow.UpdateStepRequest} returns this
 */
proto.workflow.UpdateStepRequest.prototype.setStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 4, value);
};


/**
 * optional string message = 5;
 * @return {string}
 */
proto.workflow.UpdateStepRequest.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.UpdateStepRequest} returns this
 */
proto.workflow.UpdateStepRequest.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional int64 duration_ms = 6;
 * @return {number}
 */
proto.workflow.UpdateStepRequest.prototype.getDurationMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.UpdateStepRequest} returns this
 */
proto.workflow.UpdateStepRequest.prototype.setDurationMs = function(value) {
  return jspb.Message.setProto3IntField(this, 6, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.FailStepRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.FailStepRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.FailStepRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.FailStepRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
runId: jspb.Message.getFieldWithDefault(msg, 2, ""),
seq: jspb.Message.getFieldWithDefault(msg, 3, 0),
errorCode: jspb.Message.getFieldWithDefault(msg, 4, ""),
errorMessage: jspb.Message.getFieldWithDefault(msg, 5, ""),
actionHint: jspb.Message.getFieldWithDefault(msg, 6, ""),
retryable: jspb.Message.getBooleanFieldWithDefault(msg, 7, false),
operatorActionRequired: jspb.Message.getBooleanFieldWithDefault(msg, 8, false),
failureClass: jspb.Message.getFieldWithDefault(msg, 9, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.FailStepRequest}
 */
proto.workflow.FailStepRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.FailStepRequest;
  return proto.workflow.FailStepRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.FailStepRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.FailStepRequest}
 */
proto.workflow.FailStepRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setRunId(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setSeq(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setErrorCode(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setErrorMessage(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setActionHint(value);
      break;
    case 7:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setRetryable(value);
      break;
    case 8:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setOperatorActionRequired(value);
      break;
    case 9:
      var value = /** @type {!proto.workflow.FailureClass} */ (reader.readEnum());
      msg.setFailureClass(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.FailStepRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.FailStepRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.FailStepRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.FailStepRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getRunId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getSeq();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
  f = message.getErrorCode();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getErrorMessage();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getActionHint();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getRetryable();
  if (f) {
    writer.writeBool(
      7,
      f
    );
  }
  f = message.getOperatorActionRequired();
  if (f) {
    writer.writeBool(
      8,
      f
    );
  }
  f = message.getFailureClass();
  if (f !== 0.0) {
    writer.writeEnum(
      9,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.FailStepRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.FailStepRequest} returns this
 */
proto.workflow.FailStepRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string run_id = 2;
 * @return {string}
 */
proto.workflow.FailStepRequest.prototype.getRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.FailStepRequest} returns this
 */
proto.workflow.FailStepRequest.prototype.setRunId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int32 seq = 3;
 * @return {number}
 */
proto.workflow.FailStepRequest.prototype.getSeq = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.FailStepRequest} returns this
 */
proto.workflow.FailStepRequest.prototype.setSeq = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional string error_code = 4;
 * @return {string}
 */
proto.workflow.FailStepRequest.prototype.getErrorCode = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.FailStepRequest} returns this
 */
proto.workflow.FailStepRequest.prototype.setErrorCode = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string error_message = 5;
 * @return {string}
 */
proto.workflow.FailStepRequest.prototype.getErrorMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.FailStepRequest} returns this
 */
proto.workflow.FailStepRequest.prototype.setErrorMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string action_hint = 6;
 * @return {string}
 */
proto.workflow.FailStepRequest.prototype.getActionHint = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.FailStepRequest} returns this
 */
proto.workflow.FailStepRequest.prototype.setActionHint = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional bool retryable = 7;
 * @return {boolean}
 */
proto.workflow.FailStepRequest.prototype.getRetryable = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 7, false));
};


/**
 * @param {boolean} value
 * @return {!proto.workflow.FailStepRequest} returns this
 */
proto.workflow.FailStepRequest.prototype.setRetryable = function(value) {
  return jspb.Message.setProto3BooleanField(this, 7, value);
};


/**
 * optional bool operator_action_required = 8;
 * @return {boolean}
 */
proto.workflow.FailStepRequest.prototype.getOperatorActionRequired = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 8, false));
};


/**
 * @param {boolean} value
 * @return {!proto.workflow.FailStepRequest} returns this
 */
proto.workflow.FailStepRequest.prototype.setOperatorActionRequired = function(value) {
  return jspb.Message.setProto3BooleanField(this, 8, value);
};


/**
 * optional FailureClass failure_class = 9;
 * @return {!proto.workflow.FailureClass}
 */
proto.workflow.FailStepRequest.prototype.getFailureClass = function() {
  return /** @type {!proto.workflow.FailureClass} */ (jspb.Message.getFieldWithDefault(this, 9, 0));
};


/**
 * @param {!proto.workflow.FailureClass} value
 * @return {!proto.workflow.FailStepRequest} returns this
 */
proto.workflow.FailStepRequest.prototype.setFailureClass = function(value) {
  return jspb.Message.setProto3EnumField(this, 9, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.AddArtifactRefRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.AddArtifactRefRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.AddArtifactRefRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.AddArtifactRefRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
artifact: (f = msg.getArtifact()) && proto.workflow.WorkflowArtifactRef.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.AddArtifactRefRequest}
 */
proto.workflow.AddArtifactRefRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.AddArtifactRefRequest;
  return proto.workflow.AddArtifactRefRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.AddArtifactRefRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.AddArtifactRefRequest}
 */
proto.workflow.AddArtifactRefRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = new proto.workflow.WorkflowArtifactRef;
      reader.readMessage(value,proto.workflow.WorkflowArtifactRef.deserializeBinaryFromReader);
      msg.setArtifact(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.AddArtifactRefRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.AddArtifactRefRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.AddArtifactRefRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.AddArtifactRefRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getArtifact();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.workflow.WorkflowArtifactRef.serializeBinaryToWriter
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.AddArtifactRefRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.AddArtifactRefRequest} returns this
 */
proto.workflow.AddArtifactRefRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional WorkflowArtifactRef artifact = 2;
 * @return {?proto.workflow.WorkflowArtifactRef}
 */
proto.workflow.AddArtifactRefRequest.prototype.getArtifact = function() {
  return /** @type{?proto.workflow.WorkflowArtifactRef} */ (
    jspb.Message.getWrapperField(this, proto.workflow.WorkflowArtifactRef, 2));
};


/**
 * @param {?proto.workflow.WorkflowArtifactRef|undefined} value
 * @return {!proto.workflow.AddArtifactRefRequest} returns this
*/
proto.workflow.AddArtifactRefRequest.prototype.setArtifact = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.AddArtifactRefRequest} returns this
 */
proto.workflow.AddArtifactRefRequest.prototype.clearArtifact = function() {
  return this.setArtifact(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.AddArtifactRefRequest.prototype.hasArtifact = function() {
  return jspb.Message.getField(this, 2) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.AppendEventRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.AppendEventRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.AppendEventRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.AppendEventRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
event: (f = msg.getEvent()) && proto.workflow.WorkflowEvent.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.AppendEventRequest}
 */
proto.workflow.AppendEventRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.AppendEventRequest;
  return proto.workflow.AppendEventRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.AppendEventRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.AppendEventRequest}
 */
proto.workflow.AppendEventRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = new proto.workflow.WorkflowEvent;
      reader.readMessage(value,proto.workflow.WorkflowEvent.deserializeBinaryFromReader);
      msg.setEvent(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.AppendEventRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.AppendEventRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.AppendEventRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.AppendEventRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getEvent();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.workflow.WorkflowEvent.serializeBinaryToWriter
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.AppendEventRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.AppendEventRequest} returns this
 */
proto.workflow.AppendEventRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional WorkflowEvent event = 2;
 * @return {?proto.workflow.WorkflowEvent}
 */
proto.workflow.AppendEventRequest.prototype.getEvent = function() {
  return /** @type{?proto.workflow.WorkflowEvent} */ (
    jspb.Message.getWrapperField(this, proto.workflow.WorkflowEvent, 2));
};


/**
 * @param {?proto.workflow.WorkflowEvent|undefined} value
 * @return {!proto.workflow.AppendEventRequest} returns this
*/
proto.workflow.AppendEventRequest.prototype.setEvent = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.AppendEventRequest} returns this
 */
proto.workflow.AppendEventRequest.prototype.clearEvent = function() {
  return this.setEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.AppendEventRequest.prototype.hasEvent = function() {
  return jspb.Message.getField(this, 2) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.GetRunRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.GetRunRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.GetRunRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.GetRunRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
id: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.GetRunRequest}
 */
proto.workflow.GetRunRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.GetRunRequest;
  return proto.workflow.GetRunRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.GetRunRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.GetRunRequest}
 */
proto.workflow.GetRunRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.GetRunRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.GetRunRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.GetRunRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.GetRunRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.GetRunRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.GetRunRequest} returns this
 */
proto.workflow.GetRunRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string id = 2;
 * @return {string}
 */
proto.workflow.GetRunRequest.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.GetRunRequest} returns this
 */
proto.workflow.GetRunRequest.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.ListRunsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.ListRunsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.ListRunsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListRunsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
nodeId: jspb.Message.getFieldWithDefault(msg, 2, ""),
componentName: jspb.Message.getFieldWithDefault(msg, 3, ""),
status: jspb.Message.getFieldWithDefault(msg, 4, 0),
kind: jspb.Message.getFieldWithDefault(msg, 5, 0),
activeOnly: jspb.Message.getBooleanFieldWithDefault(msg, 6, false),
failedOnly: jspb.Message.getBooleanFieldWithDefault(msg, 7, false),
limit: jspb.Message.getFieldWithDefault(msg, 8, 0),
pageToken: jspb.Message.getFieldWithDefault(msg, 9, ""),
workflowName: jspb.Message.getFieldWithDefault(msg, 10, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.ListRunsRequest}
 */
proto.workflow.ListRunsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.ListRunsRequest;
  return proto.workflow.ListRunsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.ListRunsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.ListRunsRequest}
 */
proto.workflow.ListRunsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeId(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setComponentName(value);
      break;
    case 4:
      var value = /** @type {!proto.workflow.RunStatus} */ (reader.readEnum());
      msg.setStatus(value);
      break;
    case 5:
      var value = /** @type {!proto.workflow.ComponentKind} */ (reader.readEnum());
      msg.setKind(value);
      break;
    case 6:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setActiveOnly(value);
      break;
    case 7:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setFailedOnly(value);
      break;
    case 8:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setLimit(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setPageToken(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.setWorkflowName(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.ListRunsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.ListRunsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.ListRunsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListRunsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getComponentName();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      4,
      f
    );
  }
  f = message.getKind();
  if (f !== 0.0) {
    writer.writeEnum(
      5,
      f
    );
  }
  f = message.getActiveOnly();
  if (f) {
    writer.writeBool(
      6,
      f
    );
  }
  f = message.getFailedOnly();
  if (f) {
    writer.writeBool(
      7,
      f
    );
  }
  f = message.getLimit();
  if (f !== 0) {
    writer.writeInt32(
      8,
      f
    );
  }
  f = message.getPageToken();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
  f = message.getWorkflowName();
  if (f.length > 0) {
    writer.writeString(
      10,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.ListRunsRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ListRunsRequest} returns this
 */
proto.workflow.ListRunsRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string node_id = 2;
 * @return {string}
 */
proto.workflow.ListRunsRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ListRunsRequest} returns this
 */
proto.workflow.ListRunsRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string component_name = 3;
 * @return {string}
 */
proto.workflow.ListRunsRequest.prototype.getComponentName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ListRunsRequest} returns this
 */
proto.workflow.ListRunsRequest.prototype.setComponentName = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional RunStatus status = 4;
 * @return {!proto.workflow.RunStatus}
 */
proto.workflow.ListRunsRequest.prototype.getStatus = function() {
  return /** @type {!proto.workflow.RunStatus} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {!proto.workflow.RunStatus} value
 * @return {!proto.workflow.ListRunsRequest} returns this
 */
proto.workflow.ListRunsRequest.prototype.setStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 4, value);
};


/**
 * optional ComponentKind kind = 5;
 * @return {!proto.workflow.ComponentKind}
 */
proto.workflow.ListRunsRequest.prototype.getKind = function() {
  return /** @type {!proto.workflow.ComponentKind} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {!proto.workflow.ComponentKind} value
 * @return {!proto.workflow.ListRunsRequest} returns this
 */
proto.workflow.ListRunsRequest.prototype.setKind = function(value) {
  return jspb.Message.setProto3EnumField(this, 5, value);
};


/**
 * optional bool active_only = 6;
 * @return {boolean}
 */
proto.workflow.ListRunsRequest.prototype.getActiveOnly = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 6, false));
};


/**
 * @param {boolean} value
 * @return {!proto.workflow.ListRunsRequest} returns this
 */
proto.workflow.ListRunsRequest.prototype.setActiveOnly = function(value) {
  return jspb.Message.setProto3BooleanField(this, 6, value);
};


/**
 * optional bool failed_only = 7;
 * @return {boolean}
 */
proto.workflow.ListRunsRequest.prototype.getFailedOnly = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 7, false));
};


/**
 * @param {boolean} value
 * @return {!proto.workflow.ListRunsRequest} returns this
 */
proto.workflow.ListRunsRequest.prototype.setFailedOnly = function(value) {
  return jspb.Message.setProto3BooleanField(this, 7, value);
};


/**
 * optional int32 limit = 8;
 * @return {number}
 */
proto.workflow.ListRunsRequest.prototype.getLimit = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 8, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.ListRunsRequest} returns this
 */
proto.workflow.ListRunsRequest.prototype.setLimit = function(value) {
  return jspb.Message.setProto3IntField(this, 8, value);
};


/**
 * optional string page_token = 9;
 * @return {string}
 */
proto.workflow.ListRunsRequest.prototype.getPageToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ListRunsRequest} returns this
 */
proto.workflow.ListRunsRequest.prototype.setPageToken = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};


/**
 * optional string workflow_name = 10;
 * @return {string}
 */
proto.workflow.ListRunsRequest.prototype.getWorkflowName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 10, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ListRunsRequest} returns this
 */
proto.workflow.ListRunsRequest.prototype.setWorkflowName = function(value) {
  return jspb.Message.setProto3StringField(this, 10, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.workflow.ListRunsResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.ListRunsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.ListRunsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.ListRunsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListRunsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
runsList: jspb.Message.toObjectList(msg.getRunsList(),
    proto.workflow.WorkflowRun.toObject, includeInstance),
total: jspb.Message.getFieldWithDefault(msg, 2, 0),
nextPageToken: jspb.Message.getFieldWithDefault(msg, 3, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.ListRunsResponse}
 */
proto.workflow.ListRunsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.ListRunsResponse;
  return proto.workflow.ListRunsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.ListRunsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.ListRunsResponse}
 */
proto.workflow.ListRunsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.workflow.WorkflowRun;
      reader.readMessage(value,proto.workflow.WorkflowRun.deserializeBinaryFromReader);
      msg.addRuns(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setTotal(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setNextPageToken(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.ListRunsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.ListRunsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.ListRunsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListRunsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRunsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.workflow.WorkflowRun.serializeBinaryToWriter
    );
  }
  f = message.getTotal();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getNextPageToken();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * repeated WorkflowRun runs = 1;
 * @return {!Array<!proto.workflow.WorkflowRun>}
 */
proto.workflow.ListRunsResponse.prototype.getRunsList = function() {
  return /** @type{!Array<!proto.workflow.WorkflowRun>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.workflow.WorkflowRun, 1));
};


/**
 * @param {!Array<!proto.workflow.WorkflowRun>} value
 * @return {!proto.workflow.ListRunsResponse} returns this
*/
proto.workflow.ListRunsResponse.prototype.setRunsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.workflow.WorkflowRun=} opt_value
 * @param {number=} opt_index
 * @return {!proto.workflow.WorkflowRun}
 */
proto.workflow.ListRunsResponse.prototype.addRuns = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.workflow.WorkflowRun, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.ListRunsResponse} returns this
 */
proto.workflow.ListRunsResponse.prototype.clearRunsList = function() {
  return this.setRunsList([]);
};


/**
 * optional int32 total = 2;
 * @return {number}
 */
proto.workflow.ListRunsResponse.prototype.getTotal = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.ListRunsResponse} returns this
 */
proto.workflow.ListRunsResponse.prototype.setTotal = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string next_page_token = 3;
 * @return {string}
 */
proto.workflow.ListRunsResponse.prototype.getNextPageToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ListRunsResponse} returns this
 */
proto.workflow.ListRunsResponse.prototype.setNextPageToken = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.GetRunEventsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.GetRunEventsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.GetRunEventsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.GetRunEventsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
runId: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.GetRunEventsRequest}
 */
proto.workflow.GetRunEventsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.GetRunEventsRequest;
  return proto.workflow.GetRunEventsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.GetRunEventsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.GetRunEventsRequest}
 */
proto.workflow.GetRunEventsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setRunId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.GetRunEventsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.GetRunEventsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.GetRunEventsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.GetRunEventsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getRunId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.GetRunEventsRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.GetRunEventsRequest} returns this
 */
proto.workflow.GetRunEventsRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string run_id = 2;
 * @return {string}
 */
proto.workflow.GetRunEventsRequest.prototype.getRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.GetRunEventsRequest} returns this
 */
proto.workflow.GetRunEventsRequest.prototype.setRunId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.workflow.GetRunEventsResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.GetRunEventsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.GetRunEventsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.GetRunEventsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.GetRunEventsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
eventsList: jspb.Message.toObjectList(msg.getEventsList(),
    proto.workflow.WorkflowEvent.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.GetRunEventsResponse}
 */
proto.workflow.GetRunEventsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.GetRunEventsResponse;
  return proto.workflow.GetRunEventsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.GetRunEventsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.GetRunEventsResponse}
 */
proto.workflow.GetRunEventsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.workflow.WorkflowEvent;
      reader.readMessage(value,proto.workflow.WorkflowEvent.deserializeBinaryFromReader);
      msg.addEvents(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.GetRunEventsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.GetRunEventsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.GetRunEventsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.GetRunEventsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getEventsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.workflow.WorkflowEvent.serializeBinaryToWriter
    );
  }
};


/**
 * repeated WorkflowEvent events = 1;
 * @return {!Array<!proto.workflow.WorkflowEvent>}
 */
proto.workflow.GetRunEventsResponse.prototype.getEventsList = function() {
  return /** @type{!Array<!proto.workflow.WorkflowEvent>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.workflow.WorkflowEvent, 1));
};


/**
 * @param {!Array<!proto.workflow.WorkflowEvent>} value
 * @return {!proto.workflow.GetRunEventsResponse} returns this
*/
proto.workflow.GetRunEventsResponse.prototype.setEventsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.workflow.WorkflowEvent=} opt_value
 * @param {number=} opt_index
 * @return {!proto.workflow.WorkflowEvent}
 */
proto.workflow.GetRunEventsResponse.prototype.addEvents = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.workflow.WorkflowEvent, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.GetRunEventsResponse} returns this
 */
proto.workflow.GetRunEventsResponse.prototype.clearEventsList = function() {
  return this.setEventsList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.GetCurrentRunsForNodeRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.GetCurrentRunsForNodeRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.GetCurrentRunsForNodeRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.GetCurrentRunsForNodeRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
nodeId: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.GetCurrentRunsForNodeRequest}
 */
proto.workflow.GetCurrentRunsForNodeRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.GetCurrentRunsForNodeRequest;
  return proto.workflow.GetCurrentRunsForNodeRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.GetCurrentRunsForNodeRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.GetCurrentRunsForNodeRequest}
 */
proto.workflow.GetCurrentRunsForNodeRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.GetCurrentRunsForNodeRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.GetCurrentRunsForNodeRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.GetCurrentRunsForNodeRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.GetCurrentRunsForNodeRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.GetCurrentRunsForNodeRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.GetCurrentRunsForNodeRequest} returns this
 */
proto.workflow.GetCurrentRunsForNodeRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string node_id = 2;
 * @return {string}
 */
proto.workflow.GetCurrentRunsForNodeRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.GetCurrentRunsForNodeRequest} returns this
 */
proto.workflow.GetCurrentRunsForNodeRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.GetComponentHistoryRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.GetComponentHistoryRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.GetComponentHistoryRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.GetComponentHistoryRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
componentName: jspb.Message.getFieldWithDefault(msg, 2, ""),
limit: jspb.Message.getFieldWithDefault(msg, 3, 0),
pageToken: jspb.Message.getFieldWithDefault(msg, 4, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.GetComponentHistoryRequest}
 */
proto.workflow.GetComponentHistoryRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.GetComponentHistoryRequest;
  return proto.workflow.GetComponentHistoryRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.GetComponentHistoryRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.GetComponentHistoryRequest}
 */
proto.workflow.GetComponentHistoryRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setComponentName(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setLimit(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setPageToken(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.GetComponentHistoryRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.GetComponentHistoryRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.GetComponentHistoryRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.GetComponentHistoryRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getComponentName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getLimit();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
  f = message.getPageToken();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.GetComponentHistoryRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.GetComponentHistoryRequest} returns this
 */
proto.workflow.GetComponentHistoryRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string component_name = 2;
 * @return {string}
 */
proto.workflow.GetComponentHistoryRequest.prototype.getComponentName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.GetComponentHistoryRequest} returns this
 */
proto.workflow.GetComponentHistoryRequest.prototype.setComponentName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int32 limit = 3;
 * @return {number}
 */
proto.workflow.GetComponentHistoryRequest.prototype.getLimit = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.GetComponentHistoryRequest} returns this
 */
proto.workflow.GetComponentHistoryRequest.prototype.setLimit = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional string page_token = 4;
 * @return {string}
 */
proto.workflow.GetComponentHistoryRequest.prototype.getPageToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.GetComponentHistoryRequest} returns this
 */
proto.workflow.GetComponentHistoryRequest.prototype.setPageToken = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.GetWorkflowGraphRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.GetWorkflowGraphRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.GetWorkflowGraphRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.GetWorkflowGraphRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
runId: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.GetWorkflowGraphRequest}
 */
proto.workflow.GetWorkflowGraphRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.GetWorkflowGraphRequest;
  return proto.workflow.GetWorkflowGraphRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.GetWorkflowGraphRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.GetWorkflowGraphRequest}
 */
proto.workflow.GetWorkflowGraphRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setRunId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.GetWorkflowGraphRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.GetWorkflowGraphRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.GetWorkflowGraphRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.GetWorkflowGraphRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getRunId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.GetWorkflowGraphRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.GetWorkflowGraphRequest} returns this
 */
proto.workflow.GetWorkflowGraphRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string run_id = 2;
 * @return {string}
 */
proto.workflow.GetWorkflowGraphRequest.prototype.getRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.GetWorkflowGraphRequest} returns this
 */
proto.workflow.GetWorkflowGraphRequest.prototype.setRunId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.WatchRunRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.WatchRunRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.WatchRunRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WatchRunRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
runId: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.WatchRunRequest}
 */
proto.workflow.WatchRunRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.WatchRunRequest;
  return proto.workflow.WatchRunRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.WatchRunRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.WatchRunRequest}
 */
proto.workflow.WatchRunRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setRunId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.WatchRunRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.WatchRunRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.WatchRunRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WatchRunRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getRunId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.WatchRunRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WatchRunRequest} returns this
 */
proto.workflow.WatchRunRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string run_id = 2;
 * @return {string}
 */
proto.workflow.WatchRunRequest.prototype.getRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WatchRunRequest} returns this
 */
proto.workflow.WatchRunRequest.prototype.setRunId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.WatchNodeRunsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.WatchNodeRunsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.WatchNodeRunsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WatchNodeRunsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
nodeId: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.WatchNodeRunsRequest}
 */
proto.workflow.WatchNodeRunsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.WatchNodeRunsRequest;
  return proto.workflow.WatchNodeRunsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.WatchNodeRunsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.WatchNodeRunsRequest}
 */
proto.workflow.WatchNodeRunsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.WatchNodeRunsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.WatchNodeRunsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.WatchNodeRunsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WatchNodeRunsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.WatchNodeRunsRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WatchNodeRunsRequest} returns this
 */
proto.workflow.WatchNodeRunsRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string node_id = 2;
 * @return {string}
 */
proto.workflow.WatchNodeRunsRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WatchNodeRunsRequest} returns this
 */
proto.workflow.WatchNodeRunsRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.WorkflowEventEnvelope.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.WorkflowEventEnvelope.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.WorkflowEventEnvelope} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowEventEnvelope.toObject = function(includeInstance, msg) {
  var f, obj = {
runId: jspb.Message.getFieldWithDefault(msg, 1, ""),
context: (f = msg.getContext()) && proto.workflow.WorkflowContext.toObject(includeInstance, f),
event: (f = msg.getEvent()) && proto.workflow.WorkflowEvent.toObject(includeInstance, f),
runStatus: jspb.Message.getFieldWithDefault(msg, 4, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.WorkflowEventEnvelope}
 */
proto.workflow.WorkflowEventEnvelope.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.WorkflowEventEnvelope;
  return proto.workflow.WorkflowEventEnvelope.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.WorkflowEventEnvelope} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.WorkflowEventEnvelope}
 */
proto.workflow.WorkflowEventEnvelope.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRunId(value);
      break;
    case 2:
      var value = new proto.workflow.WorkflowContext;
      reader.readMessage(value,proto.workflow.WorkflowContext.deserializeBinaryFromReader);
      msg.setContext(value);
      break;
    case 3:
      var value = new proto.workflow.WorkflowEvent;
      reader.readMessage(value,proto.workflow.WorkflowEvent.deserializeBinaryFromReader);
      msg.setEvent(value);
      break;
    case 4:
      var value = /** @type {!proto.workflow.RunStatus} */ (reader.readEnum());
      msg.setRunStatus(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.WorkflowEventEnvelope.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.WorkflowEventEnvelope.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.WorkflowEventEnvelope} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowEventEnvelope.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRunId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getContext();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.workflow.WorkflowContext.serializeBinaryToWriter
    );
  }
  f = message.getEvent();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.workflow.WorkflowEvent.serializeBinaryToWriter
    );
  }
  f = message.getRunStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      4,
      f
    );
  }
};


/**
 * optional string run_id = 1;
 * @return {string}
 */
proto.workflow.WorkflowEventEnvelope.prototype.getRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowEventEnvelope} returns this
 */
proto.workflow.WorkflowEventEnvelope.prototype.setRunId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional WorkflowContext context = 2;
 * @return {?proto.workflow.WorkflowContext}
 */
proto.workflow.WorkflowEventEnvelope.prototype.getContext = function() {
  return /** @type{?proto.workflow.WorkflowContext} */ (
    jspb.Message.getWrapperField(this, proto.workflow.WorkflowContext, 2));
};


/**
 * @param {?proto.workflow.WorkflowContext|undefined} value
 * @return {!proto.workflow.WorkflowEventEnvelope} returns this
*/
proto.workflow.WorkflowEventEnvelope.prototype.setContext = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowEventEnvelope} returns this
 */
proto.workflow.WorkflowEventEnvelope.prototype.clearContext = function() {
  return this.setContext(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowEventEnvelope.prototype.hasContext = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional WorkflowEvent event = 3;
 * @return {?proto.workflow.WorkflowEvent}
 */
proto.workflow.WorkflowEventEnvelope.prototype.getEvent = function() {
  return /** @type{?proto.workflow.WorkflowEvent} */ (
    jspb.Message.getWrapperField(this, proto.workflow.WorkflowEvent, 3));
};


/**
 * @param {?proto.workflow.WorkflowEvent|undefined} value
 * @return {!proto.workflow.WorkflowEventEnvelope} returns this
*/
proto.workflow.WorkflowEventEnvelope.prototype.setEvent = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowEventEnvelope} returns this
 */
proto.workflow.WorkflowEventEnvelope.prototype.clearEvent = function() {
  return this.setEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowEventEnvelope.prototype.hasEvent = function() {
  return jspb.Message.getField(this, 3) != null;
};


/**
 * optional RunStatus run_status = 4;
 * @return {!proto.workflow.RunStatus}
 */
proto.workflow.WorkflowEventEnvelope.prototype.getRunStatus = function() {
  return /** @type {!proto.workflow.RunStatus} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {!proto.workflow.RunStatus} value
 * @return {!proto.workflow.WorkflowEventEnvelope} returns this
 */
proto.workflow.WorkflowEventEnvelope.prototype.setRunStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 4, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.RetryRunRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.RetryRunRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.RetryRunRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.RetryRunRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
runId: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.RetryRunRequest}
 */
proto.workflow.RetryRunRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.RetryRunRequest;
  return proto.workflow.RetryRunRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.RetryRunRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.RetryRunRequest}
 */
proto.workflow.RetryRunRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setRunId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.RetryRunRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.RetryRunRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.RetryRunRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.RetryRunRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getRunId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.RetryRunRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RetryRunRequest} returns this
 */
proto.workflow.RetryRunRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string run_id = 2;
 * @return {string}
 */
proto.workflow.RetryRunRequest.prototype.getRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RetryRunRequest} returns this
 */
proto.workflow.RetryRunRequest.prototype.setRunId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.CancelRunRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.CancelRunRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.CancelRunRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.CancelRunRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
runId: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.CancelRunRequest}
 */
proto.workflow.CancelRunRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.CancelRunRequest;
  return proto.workflow.CancelRunRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.CancelRunRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.CancelRunRequest}
 */
proto.workflow.CancelRunRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setRunId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.CancelRunRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.CancelRunRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.CancelRunRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.CancelRunRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getRunId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.CancelRunRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.CancelRunRequest} returns this
 */
proto.workflow.CancelRunRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string run_id = 2;
 * @return {string}
 */
proto.workflow.CancelRunRequest.prototype.getRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.CancelRunRequest} returns this
 */
proto.workflow.CancelRunRequest.prototype.setRunId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.AcknowledgeRunRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.AcknowledgeRunRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.AcknowledgeRunRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.AcknowledgeRunRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
runId: jspb.Message.getFieldWithDefault(msg, 2, ""),
acknowledgedBy: jspb.Message.getFieldWithDefault(msg, 3, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.AcknowledgeRunRequest}
 */
proto.workflow.AcknowledgeRunRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.AcknowledgeRunRequest;
  return proto.workflow.AcknowledgeRunRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.AcknowledgeRunRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.AcknowledgeRunRequest}
 */
proto.workflow.AcknowledgeRunRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setRunId(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setAcknowledgedBy(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.AcknowledgeRunRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.AcknowledgeRunRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.AcknowledgeRunRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.AcknowledgeRunRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getRunId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getAcknowledgedBy();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.AcknowledgeRunRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.AcknowledgeRunRequest} returns this
 */
proto.workflow.AcknowledgeRunRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string run_id = 2;
 * @return {string}
 */
proto.workflow.AcknowledgeRunRequest.prototype.getRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.AcknowledgeRunRequest} returns this
 */
proto.workflow.AcknowledgeRunRequest.prototype.setRunId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string acknowledged_by = 3;
 * @return {string}
 */
proto.workflow.AcknowledgeRunRequest.prototype.getAcknowledgedBy = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.AcknowledgeRunRequest} returns this
 */
proto.workflow.AcknowledgeRunRequest.prototype.setAcknowledgedBy = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.DiagnoseRunRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.DiagnoseRunRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.DiagnoseRunRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.DiagnoseRunRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
runId: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.DiagnoseRunRequest}
 */
proto.workflow.DiagnoseRunRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.DiagnoseRunRequest;
  return proto.workflow.DiagnoseRunRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.DiagnoseRunRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.DiagnoseRunRequest}
 */
proto.workflow.DiagnoseRunRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setRunId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.DiagnoseRunRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.DiagnoseRunRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.DiagnoseRunRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.DiagnoseRunRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getRunId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.DiagnoseRunRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.DiagnoseRunRequest} returns this
 */
proto.workflow.DiagnoseRunRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string run_id = 2;
 * @return {string}
 */
proto.workflow.DiagnoseRunRequest.prototype.getRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.DiagnoseRunRequest} returns this
 */
proto.workflow.DiagnoseRunRequest.prototype.setRunId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.workflow.DiagnoseRunResponse.repeatedFields_ = [3];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.DiagnoseRunResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.DiagnoseRunResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.DiagnoseRunResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.DiagnoseRunResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
diagnosis: jspb.Message.getFieldWithDefault(msg, 1, ""),
confidence: jspb.Message.getFieldWithDefault(msg, 2, ""),
relatedRunIdsList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f,
suggestedAction: jspb.Message.getFieldWithDefault(msg, 4, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.DiagnoseRunResponse}
 */
proto.workflow.DiagnoseRunResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.DiagnoseRunResponse;
  return proto.workflow.DiagnoseRunResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.DiagnoseRunResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.DiagnoseRunResponse}
 */
proto.workflow.DiagnoseRunResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setDiagnosis(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setConfidence(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.addRelatedRunIds(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setSuggestedAction(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.DiagnoseRunResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.DiagnoseRunResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.DiagnoseRunResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.DiagnoseRunResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDiagnosis();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getConfidence();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getRelatedRunIdsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      3,
      f
    );
  }
  f = message.getSuggestedAction();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * optional string diagnosis = 1;
 * @return {string}
 */
proto.workflow.DiagnoseRunResponse.prototype.getDiagnosis = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.DiagnoseRunResponse} returns this
 */
proto.workflow.DiagnoseRunResponse.prototype.setDiagnosis = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string confidence = 2;
 * @return {string}
 */
proto.workflow.DiagnoseRunResponse.prototype.getConfidence = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.DiagnoseRunResponse} returns this
 */
proto.workflow.DiagnoseRunResponse.prototype.setConfidence = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * repeated string related_run_ids = 3;
 * @return {!Array<string>}
 */
proto.workflow.DiagnoseRunResponse.prototype.getRelatedRunIdsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.workflow.DiagnoseRunResponse} returns this
 */
proto.workflow.DiagnoseRunResponse.prototype.setRelatedRunIdsList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.workflow.DiagnoseRunResponse} returns this
 */
proto.workflow.DiagnoseRunResponse.prototype.addRelatedRunIds = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.DiagnoseRunResponse} returns this
 */
proto.workflow.DiagnoseRunResponse.prototype.clearRelatedRunIdsList = function() {
  return this.setRelatedRunIdsList([]);
};


/**
 * optional string suggested_action = 4;
 * @return {string}
 */
proto.workflow.DiagnoseRunResponse.prototype.getSuggestedAction = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.DiagnoseRunResponse} returns this
 */
proto.workflow.DiagnoseRunResponse.prototype.setSuggestedAction = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.WorkflowRunSummary.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.WorkflowRunSummary.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.WorkflowRunSummary} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowRunSummary.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
workflowName: jspb.Message.getFieldWithDefault(msg, 2, ""),
totalRuns: jspb.Message.getFieldWithDefault(msg, 3, 0),
successRuns: jspb.Message.getFieldWithDefault(msg, 4, 0),
failureRuns: jspb.Message.getFieldWithDefault(msg, 5, 0),
lastRunId: jspb.Message.getFieldWithDefault(msg, 6, ""),
lastRunStatus: jspb.Message.getFieldWithDefault(msg, 7, 0),
lastStartedAt: (f = msg.getLastStartedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
lastFinishedAt: (f = msg.getLastFinishedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
lastDurationMs: jspb.Message.getFieldWithDefault(msg, 10, 0),
lastSuccessId: jspb.Message.getFieldWithDefault(msg, 11, ""),
lastSuccessAt: (f = msg.getLastSuccessAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
lastFailureId: jspb.Message.getFieldWithDefault(msg, 13, ""),
lastFailureAt: (f = msg.getLastFailureAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
lastFailureReason: jspb.Message.getFieldWithDefault(msg, 15, ""),
updatedAt: (f = msg.getUpdatedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.WorkflowRunSummary}
 */
proto.workflow.WorkflowRunSummary.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.WorkflowRunSummary;
  return proto.workflow.WorkflowRunSummary.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.WorkflowRunSummary} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.WorkflowRunSummary}
 */
proto.workflow.WorkflowRunSummary.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setWorkflowName(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setTotalRuns(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setSuccessRuns(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setFailureRuns(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setLastRunId(value);
      break;
    case 7:
      var value = /** @type {!proto.workflow.RunStatus} */ (reader.readEnum());
      msg.setLastRunStatus(value);
      break;
    case 8:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setLastStartedAt(value);
      break;
    case 9:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setLastFinishedAt(value);
      break;
    case 10:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setLastDurationMs(value);
      break;
    case 11:
      var value = /** @type {string} */ (reader.readString());
      msg.setLastSuccessId(value);
      break;
    case 12:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setLastSuccessAt(value);
      break;
    case 13:
      var value = /** @type {string} */ (reader.readString());
      msg.setLastFailureId(value);
      break;
    case 14:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setLastFailureAt(value);
      break;
    case 15:
      var value = /** @type {string} */ (reader.readString());
      msg.setLastFailureReason(value);
      break;
    case 16:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setUpdatedAt(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.WorkflowRunSummary.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.WorkflowRunSummary.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.WorkflowRunSummary} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowRunSummary.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getWorkflowName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getTotalRuns();
  if (f !== 0) {
    writer.writeInt64(
      3,
      f
    );
  }
  f = message.getSuccessRuns();
  if (f !== 0) {
    writer.writeInt64(
      4,
      f
    );
  }
  f = message.getFailureRuns();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
  f = message.getLastRunId();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getLastRunStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      7,
      f
    );
  }
  f = message.getLastStartedAt();
  if (f != null) {
    writer.writeMessage(
      8,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getLastFinishedAt();
  if (f != null) {
    writer.writeMessage(
      9,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getLastDurationMs();
  if (f !== 0) {
    writer.writeInt64(
      10,
      f
    );
  }
  f = message.getLastSuccessId();
  if (f.length > 0) {
    writer.writeString(
      11,
      f
    );
  }
  f = message.getLastSuccessAt();
  if (f != null) {
    writer.writeMessage(
      12,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getLastFailureId();
  if (f.length > 0) {
    writer.writeString(
      13,
      f
    );
  }
  f = message.getLastFailureAt();
  if (f != null) {
    writer.writeMessage(
      14,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getLastFailureReason();
  if (f.length > 0) {
    writer.writeString(
      15,
      f
    );
  }
  f = message.getUpdatedAt();
  if (f != null) {
    writer.writeMessage(
      16,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.WorkflowRunSummary.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowRunSummary} returns this
 */
proto.workflow.WorkflowRunSummary.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string workflow_name = 2;
 * @return {string}
 */
proto.workflow.WorkflowRunSummary.prototype.getWorkflowName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowRunSummary} returns this
 */
proto.workflow.WorkflowRunSummary.prototype.setWorkflowName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int64 total_runs = 3;
 * @return {number}
 */
proto.workflow.WorkflowRunSummary.prototype.getTotalRuns = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.WorkflowRunSummary} returns this
 */
proto.workflow.WorkflowRunSummary.prototype.setTotalRuns = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional int64 success_runs = 4;
 * @return {number}
 */
proto.workflow.WorkflowRunSummary.prototype.getSuccessRuns = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.WorkflowRunSummary} returns this
 */
proto.workflow.WorkflowRunSummary.prototype.setSuccessRuns = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional int64 failure_runs = 5;
 * @return {number}
 */
proto.workflow.WorkflowRunSummary.prototype.getFailureRuns = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.WorkflowRunSummary} returns this
 */
proto.workflow.WorkflowRunSummary.prototype.setFailureRuns = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional string last_run_id = 6;
 * @return {string}
 */
proto.workflow.WorkflowRunSummary.prototype.getLastRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowRunSummary} returns this
 */
proto.workflow.WorkflowRunSummary.prototype.setLastRunId = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional RunStatus last_run_status = 7;
 * @return {!proto.workflow.RunStatus}
 */
proto.workflow.WorkflowRunSummary.prototype.getLastRunStatus = function() {
  return /** @type {!proto.workflow.RunStatus} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/**
 * @param {!proto.workflow.RunStatus} value
 * @return {!proto.workflow.WorkflowRunSummary} returns this
 */
proto.workflow.WorkflowRunSummary.prototype.setLastRunStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 7, value);
};


/**
 * optional google.protobuf.Timestamp last_started_at = 8;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.WorkflowRunSummary.prototype.getLastStartedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 8));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.WorkflowRunSummary} returns this
*/
proto.workflow.WorkflowRunSummary.prototype.setLastStartedAt = function(value) {
  return jspb.Message.setWrapperField(this, 8, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowRunSummary} returns this
 */
proto.workflow.WorkflowRunSummary.prototype.clearLastStartedAt = function() {
  return this.setLastStartedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowRunSummary.prototype.hasLastStartedAt = function() {
  return jspb.Message.getField(this, 8) != null;
};


/**
 * optional google.protobuf.Timestamp last_finished_at = 9;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.WorkflowRunSummary.prototype.getLastFinishedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 9));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.WorkflowRunSummary} returns this
*/
proto.workflow.WorkflowRunSummary.prototype.setLastFinishedAt = function(value) {
  return jspb.Message.setWrapperField(this, 9, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowRunSummary} returns this
 */
proto.workflow.WorkflowRunSummary.prototype.clearLastFinishedAt = function() {
  return this.setLastFinishedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowRunSummary.prototype.hasLastFinishedAt = function() {
  return jspb.Message.getField(this, 9) != null;
};


/**
 * optional int64 last_duration_ms = 10;
 * @return {number}
 */
proto.workflow.WorkflowRunSummary.prototype.getLastDurationMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 10, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.WorkflowRunSummary} returns this
 */
proto.workflow.WorkflowRunSummary.prototype.setLastDurationMs = function(value) {
  return jspb.Message.setProto3IntField(this, 10, value);
};


/**
 * optional string last_success_id = 11;
 * @return {string}
 */
proto.workflow.WorkflowRunSummary.prototype.getLastSuccessId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 11, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowRunSummary} returns this
 */
proto.workflow.WorkflowRunSummary.prototype.setLastSuccessId = function(value) {
  return jspb.Message.setProto3StringField(this, 11, value);
};


/**
 * optional google.protobuf.Timestamp last_success_at = 12;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.WorkflowRunSummary.prototype.getLastSuccessAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 12));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.WorkflowRunSummary} returns this
*/
proto.workflow.WorkflowRunSummary.prototype.setLastSuccessAt = function(value) {
  return jspb.Message.setWrapperField(this, 12, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowRunSummary} returns this
 */
proto.workflow.WorkflowRunSummary.prototype.clearLastSuccessAt = function() {
  return this.setLastSuccessAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowRunSummary.prototype.hasLastSuccessAt = function() {
  return jspb.Message.getField(this, 12) != null;
};


/**
 * optional string last_failure_id = 13;
 * @return {string}
 */
proto.workflow.WorkflowRunSummary.prototype.getLastFailureId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 13, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowRunSummary} returns this
 */
proto.workflow.WorkflowRunSummary.prototype.setLastFailureId = function(value) {
  return jspb.Message.setProto3StringField(this, 13, value);
};


/**
 * optional google.protobuf.Timestamp last_failure_at = 14;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.WorkflowRunSummary.prototype.getLastFailureAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 14));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.WorkflowRunSummary} returns this
*/
proto.workflow.WorkflowRunSummary.prototype.setLastFailureAt = function(value) {
  return jspb.Message.setWrapperField(this, 14, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowRunSummary} returns this
 */
proto.workflow.WorkflowRunSummary.prototype.clearLastFailureAt = function() {
  return this.setLastFailureAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowRunSummary.prototype.hasLastFailureAt = function() {
  return jspb.Message.getField(this, 14) != null;
};


/**
 * optional string last_failure_reason = 15;
 * @return {string}
 */
proto.workflow.WorkflowRunSummary.prototype.getLastFailureReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 15, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowRunSummary} returns this
 */
proto.workflow.WorkflowRunSummary.prototype.setLastFailureReason = function(value) {
  return jspb.Message.setProto3StringField(this, 15, value);
};


/**
 * optional google.protobuf.Timestamp updated_at = 16;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.WorkflowRunSummary.prototype.getUpdatedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 16));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.WorkflowRunSummary} returns this
*/
proto.workflow.WorkflowRunSummary.prototype.setUpdatedAt = function(value) {
  return jspb.Message.setWrapperField(this, 16, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowRunSummary} returns this
 */
proto.workflow.WorkflowRunSummary.prototype.clearUpdatedAt = function() {
  return this.setUpdatedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowRunSummary.prototype.hasUpdatedAt = function() {
  return jspb.Message.getField(this, 16) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.RecordOutcomeRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.RecordOutcomeRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.RecordOutcomeRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.RecordOutcomeRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
workflowName: jspb.Message.getFieldWithDefault(msg, 2, ""),
runId: jspb.Message.getFieldWithDefault(msg, 3, ""),
status: jspb.Message.getFieldWithDefault(msg, 4, 0),
startedAt: (f = msg.getStartedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
finishedAt: (f = msg.getFinishedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
durationMs: jspb.Message.getFieldWithDefault(msg, 7, 0),
failureReason: jspb.Message.getFieldWithDefault(msg, 8, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.RecordOutcomeRequest}
 */
proto.workflow.RecordOutcomeRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.RecordOutcomeRequest;
  return proto.workflow.RecordOutcomeRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.RecordOutcomeRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.RecordOutcomeRequest}
 */
proto.workflow.RecordOutcomeRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setWorkflowName(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setRunId(value);
      break;
    case 4:
      var value = /** @type {!proto.workflow.RunStatus} */ (reader.readEnum());
      msg.setStatus(value);
      break;
    case 5:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setStartedAt(value);
      break;
    case 6:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setFinishedAt(value);
      break;
    case 7:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setDurationMs(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setFailureReason(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.RecordOutcomeRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.RecordOutcomeRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.RecordOutcomeRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.RecordOutcomeRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getWorkflowName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getRunId();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      4,
      f
    );
  }
  f = message.getStartedAt();
  if (f != null) {
    writer.writeMessage(
      5,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getFinishedAt();
  if (f != null) {
    writer.writeMessage(
      6,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getDurationMs();
  if (f !== 0) {
    writer.writeInt64(
      7,
      f
    );
  }
  f = message.getFailureReason();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.RecordOutcomeRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordOutcomeRequest} returns this
 */
proto.workflow.RecordOutcomeRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string workflow_name = 2;
 * @return {string}
 */
proto.workflow.RecordOutcomeRequest.prototype.getWorkflowName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordOutcomeRequest} returns this
 */
proto.workflow.RecordOutcomeRequest.prototype.setWorkflowName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string run_id = 3;
 * @return {string}
 */
proto.workflow.RecordOutcomeRequest.prototype.getRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordOutcomeRequest} returns this
 */
proto.workflow.RecordOutcomeRequest.prototype.setRunId = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional RunStatus status = 4;
 * @return {!proto.workflow.RunStatus}
 */
proto.workflow.RecordOutcomeRequest.prototype.getStatus = function() {
  return /** @type {!proto.workflow.RunStatus} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {!proto.workflow.RunStatus} value
 * @return {!proto.workflow.RecordOutcomeRequest} returns this
 */
proto.workflow.RecordOutcomeRequest.prototype.setStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 4, value);
};


/**
 * optional google.protobuf.Timestamp started_at = 5;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.RecordOutcomeRequest.prototype.getStartedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 5));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.RecordOutcomeRequest} returns this
*/
proto.workflow.RecordOutcomeRequest.prototype.setStartedAt = function(value) {
  return jspb.Message.setWrapperField(this, 5, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.RecordOutcomeRequest} returns this
 */
proto.workflow.RecordOutcomeRequest.prototype.clearStartedAt = function() {
  return this.setStartedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.RecordOutcomeRequest.prototype.hasStartedAt = function() {
  return jspb.Message.getField(this, 5) != null;
};


/**
 * optional google.protobuf.Timestamp finished_at = 6;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.RecordOutcomeRequest.prototype.getFinishedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 6));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.RecordOutcomeRequest} returns this
*/
proto.workflow.RecordOutcomeRequest.prototype.setFinishedAt = function(value) {
  return jspb.Message.setWrapperField(this, 6, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.RecordOutcomeRequest} returns this
 */
proto.workflow.RecordOutcomeRequest.prototype.clearFinishedAt = function() {
  return this.setFinishedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.RecordOutcomeRequest.prototype.hasFinishedAt = function() {
  return jspb.Message.getField(this, 6) != null;
};


/**
 * optional int64 duration_ms = 7;
 * @return {number}
 */
proto.workflow.RecordOutcomeRequest.prototype.getDurationMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.RecordOutcomeRequest} returns this
 */
proto.workflow.RecordOutcomeRequest.prototype.setDurationMs = function(value) {
  return jspb.Message.setProto3IntField(this, 7, value);
};


/**
 * optional string failure_reason = 8;
 * @return {string}
 */
proto.workflow.RecordOutcomeRequest.prototype.getFailureReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordOutcomeRequest} returns this
 */
proto.workflow.RecordOutcomeRequest.prototype.setFailureReason = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.ListWorkflowSummariesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.ListWorkflowSummariesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.ListWorkflowSummariesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListWorkflowSummariesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
workflowName: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.ListWorkflowSummariesRequest}
 */
proto.workflow.ListWorkflowSummariesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.ListWorkflowSummariesRequest;
  return proto.workflow.ListWorkflowSummariesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.ListWorkflowSummariesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.ListWorkflowSummariesRequest}
 */
proto.workflow.ListWorkflowSummariesRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setWorkflowName(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.ListWorkflowSummariesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.ListWorkflowSummariesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.ListWorkflowSummariesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListWorkflowSummariesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getWorkflowName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.ListWorkflowSummariesRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ListWorkflowSummariesRequest} returns this
 */
proto.workflow.ListWorkflowSummariesRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string workflow_name = 2;
 * @return {string}
 */
proto.workflow.ListWorkflowSummariesRequest.prototype.getWorkflowName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ListWorkflowSummariesRequest} returns this
 */
proto.workflow.ListWorkflowSummariesRequest.prototype.setWorkflowName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.workflow.ListWorkflowSummariesResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.ListWorkflowSummariesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.ListWorkflowSummariesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.ListWorkflowSummariesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListWorkflowSummariesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
summariesList: jspb.Message.toObjectList(msg.getSummariesList(),
    proto.workflow.WorkflowRunSummary.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.ListWorkflowSummariesResponse}
 */
proto.workflow.ListWorkflowSummariesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.ListWorkflowSummariesResponse;
  return proto.workflow.ListWorkflowSummariesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.ListWorkflowSummariesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.ListWorkflowSummariesResponse}
 */
proto.workflow.ListWorkflowSummariesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.workflow.WorkflowRunSummary;
      reader.readMessage(value,proto.workflow.WorkflowRunSummary.deserializeBinaryFromReader);
      msg.addSummaries(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.ListWorkflowSummariesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.ListWorkflowSummariesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.ListWorkflowSummariesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListWorkflowSummariesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSummariesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.workflow.WorkflowRunSummary.serializeBinaryToWriter
    );
  }
};


/**
 * repeated WorkflowRunSummary summaries = 1;
 * @return {!Array<!proto.workflow.WorkflowRunSummary>}
 */
proto.workflow.ListWorkflowSummariesResponse.prototype.getSummariesList = function() {
  return /** @type{!Array<!proto.workflow.WorkflowRunSummary>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.workflow.WorkflowRunSummary, 1));
};


/**
 * @param {!Array<!proto.workflow.WorkflowRunSummary>} value
 * @return {!proto.workflow.ListWorkflowSummariesResponse} returns this
*/
proto.workflow.ListWorkflowSummariesResponse.prototype.setSummariesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.workflow.WorkflowRunSummary=} opt_value
 * @param {number=} opt_index
 * @return {!proto.workflow.WorkflowRunSummary}
 */
proto.workflow.ListWorkflowSummariesResponse.prototype.addSummaries = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.workflow.WorkflowRunSummary, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.ListWorkflowSummariesResponse} returns this
 */
proto.workflow.ListWorkflowSummariesResponse.prototype.clearSummariesList = function() {
  return this.setSummariesList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.WorkflowStepOutcome.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.WorkflowStepOutcome.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.WorkflowStepOutcome} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowStepOutcome.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
workflowName: jspb.Message.getFieldWithDefault(msg, 2, ""),
stepId: jspb.Message.getFieldWithDefault(msg, 3, ""),
totalExecutions: jspb.Message.getFieldWithDefault(msg, 4, 0),
successCount: jspb.Message.getFieldWithDefault(msg, 5, 0),
failureCount: jspb.Message.getFieldWithDefault(msg, 6, 0),
skippedCount: jspb.Message.getFieldWithDefault(msg, 7, 0),
lastStatus: jspb.Message.getFieldWithDefault(msg, 8, 0),
lastStartedAt: (f = msg.getLastStartedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
lastFinishedAt: (f = msg.getLastFinishedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
lastDurationMs: jspb.Message.getFieldWithDefault(msg, 11, 0),
lastErrorCode: jspb.Message.getFieldWithDefault(msg, 12, ""),
lastErrorMessage: jspb.Message.getFieldWithDefault(msg, 13, ""),
firstSeenAt: (f = msg.getFirstSeenAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
updatedAt: (f = msg.getUpdatedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.WorkflowStepOutcome}
 */
proto.workflow.WorkflowStepOutcome.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.WorkflowStepOutcome;
  return proto.workflow.WorkflowStepOutcome.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.WorkflowStepOutcome} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.WorkflowStepOutcome}
 */
proto.workflow.WorkflowStepOutcome.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setWorkflowName(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setStepId(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setTotalExecutions(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setSuccessCount(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setFailureCount(value);
      break;
    case 7:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setSkippedCount(value);
      break;
    case 8:
      var value = /** @type {!proto.workflow.StepStatus} */ (reader.readEnum());
      msg.setLastStatus(value);
      break;
    case 9:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setLastStartedAt(value);
      break;
    case 10:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setLastFinishedAt(value);
      break;
    case 11:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setLastDurationMs(value);
      break;
    case 12:
      var value = /** @type {string} */ (reader.readString());
      msg.setLastErrorCode(value);
      break;
    case 13:
      var value = /** @type {string} */ (reader.readString());
      msg.setLastErrorMessage(value);
      break;
    case 14:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setFirstSeenAt(value);
      break;
    case 15:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setUpdatedAt(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.WorkflowStepOutcome.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.WorkflowStepOutcome.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.WorkflowStepOutcome} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowStepOutcome.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getWorkflowName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getStepId();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getTotalExecutions();
  if (f !== 0) {
    writer.writeInt64(
      4,
      f
    );
  }
  f = message.getSuccessCount();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
  f = message.getFailureCount();
  if (f !== 0) {
    writer.writeInt64(
      6,
      f
    );
  }
  f = message.getSkippedCount();
  if (f !== 0) {
    writer.writeInt64(
      7,
      f
    );
  }
  f = message.getLastStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      8,
      f
    );
  }
  f = message.getLastStartedAt();
  if (f != null) {
    writer.writeMessage(
      9,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getLastFinishedAt();
  if (f != null) {
    writer.writeMessage(
      10,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getLastDurationMs();
  if (f !== 0) {
    writer.writeInt64(
      11,
      f
    );
  }
  f = message.getLastErrorCode();
  if (f.length > 0) {
    writer.writeString(
      12,
      f
    );
  }
  f = message.getLastErrorMessage();
  if (f.length > 0) {
    writer.writeString(
      13,
      f
    );
  }
  f = message.getFirstSeenAt();
  if (f != null) {
    writer.writeMessage(
      14,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getUpdatedAt();
  if (f != null) {
    writer.writeMessage(
      15,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.WorkflowStepOutcome.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowStepOutcome} returns this
 */
proto.workflow.WorkflowStepOutcome.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string workflow_name = 2;
 * @return {string}
 */
proto.workflow.WorkflowStepOutcome.prototype.getWorkflowName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowStepOutcome} returns this
 */
proto.workflow.WorkflowStepOutcome.prototype.setWorkflowName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string step_id = 3;
 * @return {string}
 */
proto.workflow.WorkflowStepOutcome.prototype.getStepId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowStepOutcome} returns this
 */
proto.workflow.WorkflowStepOutcome.prototype.setStepId = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional int64 total_executions = 4;
 * @return {number}
 */
proto.workflow.WorkflowStepOutcome.prototype.getTotalExecutions = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.WorkflowStepOutcome} returns this
 */
proto.workflow.WorkflowStepOutcome.prototype.setTotalExecutions = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional int64 success_count = 5;
 * @return {number}
 */
proto.workflow.WorkflowStepOutcome.prototype.getSuccessCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.WorkflowStepOutcome} returns this
 */
proto.workflow.WorkflowStepOutcome.prototype.setSuccessCount = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional int64 failure_count = 6;
 * @return {number}
 */
proto.workflow.WorkflowStepOutcome.prototype.getFailureCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.WorkflowStepOutcome} returns this
 */
proto.workflow.WorkflowStepOutcome.prototype.setFailureCount = function(value) {
  return jspb.Message.setProto3IntField(this, 6, value);
};


/**
 * optional int64 skipped_count = 7;
 * @return {number}
 */
proto.workflow.WorkflowStepOutcome.prototype.getSkippedCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.WorkflowStepOutcome} returns this
 */
proto.workflow.WorkflowStepOutcome.prototype.setSkippedCount = function(value) {
  return jspb.Message.setProto3IntField(this, 7, value);
};


/**
 * optional StepStatus last_status = 8;
 * @return {!proto.workflow.StepStatus}
 */
proto.workflow.WorkflowStepOutcome.prototype.getLastStatus = function() {
  return /** @type {!proto.workflow.StepStatus} */ (jspb.Message.getFieldWithDefault(this, 8, 0));
};


/**
 * @param {!proto.workflow.StepStatus} value
 * @return {!proto.workflow.WorkflowStepOutcome} returns this
 */
proto.workflow.WorkflowStepOutcome.prototype.setLastStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 8, value);
};


/**
 * optional google.protobuf.Timestamp last_started_at = 9;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.WorkflowStepOutcome.prototype.getLastStartedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 9));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.WorkflowStepOutcome} returns this
*/
proto.workflow.WorkflowStepOutcome.prototype.setLastStartedAt = function(value) {
  return jspb.Message.setWrapperField(this, 9, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowStepOutcome} returns this
 */
proto.workflow.WorkflowStepOutcome.prototype.clearLastStartedAt = function() {
  return this.setLastStartedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowStepOutcome.prototype.hasLastStartedAt = function() {
  return jspb.Message.getField(this, 9) != null;
};


/**
 * optional google.protobuf.Timestamp last_finished_at = 10;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.WorkflowStepOutcome.prototype.getLastFinishedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 10));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.WorkflowStepOutcome} returns this
*/
proto.workflow.WorkflowStepOutcome.prototype.setLastFinishedAt = function(value) {
  return jspb.Message.setWrapperField(this, 10, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowStepOutcome} returns this
 */
proto.workflow.WorkflowStepOutcome.prototype.clearLastFinishedAt = function() {
  return this.setLastFinishedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowStepOutcome.prototype.hasLastFinishedAt = function() {
  return jspb.Message.getField(this, 10) != null;
};


/**
 * optional int64 last_duration_ms = 11;
 * @return {number}
 */
proto.workflow.WorkflowStepOutcome.prototype.getLastDurationMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 11, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.WorkflowStepOutcome} returns this
 */
proto.workflow.WorkflowStepOutcome.prototype.setLastDurationMs = function(value) {
  return jspb.Message.setProto3IntField(this, 11, value);
};


/**
 * optional string last_error_code = 12;
 * @return {string}
 */
proto.workflow.WorkflowStepOutcome.prototype.getLastErrorCode = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 12, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowStepOutcome} returns this
 */
proto.workflow.WorkflowStepOutcome.prototype.setLastErrorCode = function(value) {
  return jspb.Message.setProto3StringField(this, 12, value);
};


/**
 * optional string last_error_message = 13;
 * @return {string}
 */
proto.workflow.WorkflowStepOutcome.prototype.getLastErrorMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 13, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowStepOutcome} returns this
 */
proto.workflow.WorkflowStepOutcome.prototype.setLastErrorMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 13, value);
};


/**
 * optional google.protobuf.Timestamp first_seen_at = 14;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.WorkflowStepOutcome.prototype.getFirstSeenAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 14));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.WorkflowStepOutcome} returns this
*/
proto.workflow.WorkflowStepOutcome.prototype.setFirstSeenAt = function(value) {
  return jspb.Message.setWrapperField(this, 14, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowStepOutcome} returns this
 */
proto.workflow.WorkflowStepOutcome.prototype.clearFirstSeenAt = function() {
  return this.setFirstSeenAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowStepOutcome.prototype.hasFirstSeenAt = function() {
  return jspb.Message.getField(this, 14) != null;
};


/**
 * optional google.protobuf.Timestamp updated_at = 15;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.WorkflowStepOutcome.prototype.getUpdatedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 15));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.WorkflowStepOutcome} returns this
*/
proto.workflow.WorkflowStepOutcome.prototype.setUpdatedAt = function(value) {
  return jspb.Message.setWrapperField(this, 15, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.WorkflowStepOutcome} returns this
 */
proto.workflow.WorkflowStepOutcome.prototype.clearUpdatedAt = function() {
  return this.setUpdatedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.WorkflowStepOutcome.prototype.hasUpdatedAt = function() {
  return jspb.Message.getField(this, 15) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.RecordStepOutcomeRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.RecordStepOutcomeRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.RecordStepOutcomeRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.RecordStepOutcomeRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
workflowName: jspb.Message.getFieldWithDefault(msg, 2, ""),
stepId: jspb.Message.getFieldWithDefault(msg, 3, ""),
status: jspb.Message.getFieldWithDefault(msg, 4, 0),
startedAt: (f = msg.getStartedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
finishedAt: (f = msg.getFinishedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
durationMs: jspb.Message.getFieldWithDefault(msg, 7, 0),
errorCode: jspb.Message.getFieldWithDefault(msg, 8, ""),
errorMessage: jspb.Message.getFieldWithDefault(msg, 9, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.RecordStepOutcomeRequest}
 */
proto.workflow.RecordStepOutcomeRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.RecordStepOutcomeRequest;
  return proto.workflow.RecordStepOutcomeRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.RecordStepOutcomeRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.RecordStepOutcomeRequest}
 */
proto.workflow.RecordStepOutcomeRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setWorkflowName(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setStepId(value);
      break;
    case 4:
      var value = /** @type {!proto.workflow.StepStatus} */ (reader.readEnum());
      msg.setStatus(value);
      break;
    case 5:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setStartedAt(value);
      break;
    case 6:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setFinishedAt(value);
      break;
    case 7:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setDurationMs(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setErrorCode(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setErrorMessage(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.RecordStepOutcomeRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.RecordStepOutcomeRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.RecordStepOutcomeRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.RecordStepOutcomeRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getWorkflowName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getStepId();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      4,
      f
    );
  }
  f = message.getStartedAt();
  if (f != null) {
    writer.writeMessage(
      5,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getFinishedAt();
  if (f != null) {
    writer.writeMessage(
      6,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getDurationMs();
  if (f !== 0) {
    writer.writeInt64(
      7,
      f
    );
  }
  f = message.getErrorCode();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
  f = message.getErrorMessage();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.RecordStepOutcomeRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordStepOutcomeRequest} returns this
 */
proto.workflow.RecordStepOutcomeRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string workflow_name = 2;
 * @return {string}
 */
proto.workflow.RecordStepOutcomeRequest.prototype.getWorkflowName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordStepOutcomeRequest} returns this
 */
proto.workflow.RecordStepOutcomeRequest.prototype.setWorkflowName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string step_id = 3;
 * @return {string}
 */
proto.workflow.RecordStepOutcomeRequest.prototype.getStepId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordStepOutcomeRequest} returns this
 */
proto.workflow.RecordStepOutcomeRequest.prototype.setStepId = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional StepStatus status = 4;
 * @return {!proto.workflow.StepStatus}
 */
proto.workflow.RecordStepOutcomeRequest.prototype.getStatus = function() {
  return /** @type {!proto.workflow.StepStatus} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {!proto.workflow.StepStatus} value
 * @return {!proto.workflow.RecordStepOutcomeRequest} returns this
 */
proto.workflow.RecordStepOutcomeRequest.prototype.setStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 4, value);
};


/**
 * optional google.protobuf.Timestamp started_at = 5;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.RecordStepOutcomeRequest.prototype.getStartedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 5));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.RecordStepOutcomeRequest} returns this
*/
proto.workflow.RecordStepOutcomeRequest.prototype.setStartedAt = function(value) {
  return jspb.Message.setWrapperField(this, 5, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.RecordStepOutcomeRequest} returns this
 */
proto.workflow.RecordStepOutcomeRequest.prototype.clearStartedAt = function() {
  return this.setStartedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.RecordStepOutcomeRequest.prototype.hasStartedAt = function() {
  return jspb.Message.getField(this, 5) != null;
};


/**
 * optional google.protobuf.Timestamp finished_at = 6;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.RecordStepOutcomeRequest.prototype.getFinishedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 6));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.RecordStepOutcomeRequest} returns this
*/
proto.workflow.RecordStepOutcomeRequest.prototype.setFinishedAt = function(value) {
  return jspb.Message.setWrapperField(this, 6, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.RecordStepOutcomeRequest} returns this
 */
proto.workflow.RecordStepOutcomeRequest.prototype.clearFinishedAt = function() {
  return this.setFinishedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.RecordStepOutcomeRequest.prototype.hasFinishedAt = function() {
  return jspb.Message.getField(this, 6) != null;
};


/**
 * optional int64 duration_ms = 7;
 * @return {number}
 */
proto.workflow.RecordStepOutcomeRequest.prototype.getDurationMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.RecordStepOutcomeRequest} returns this
 */
proto.workflow.RecordStepOutcomeRequest.prototype.setDurationMs = function(value) {
  return jspb.Message.setProto3IntField(this, 7, value);
};


/**
 * optional string error_code = 8;
 * @return {string}
 */
proto.workflow.RecordStepOutcomeRequest.prototype.getErrorCode = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordStepOutcomeRequest} returns this
 */
proto.workflow.RecordStepOutcomeRequest.prototype.setErrorCode = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};


/**
 * optional string error_message = 9;
 * @return {string}
 */
proto.workflow.RecordStepOutcomeRequest.prototype.getErrorMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordStepOutcomeRequest} returns this
 */
proto.workflow.RecordStepOutcomeRequest.prototype.setErrorMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.ListStepOutcomesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.ListStepOutcomesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.ListStepOutcomesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListStepOutcomesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
workflowName: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.ListStepOutcomesRequest}
 */
proto.workflow.ListStepOutcomesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.ListStepOutcomesRequest;
  return proto.workflow.ListStepOutcomesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.ListStepOutcomesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.ListStepOutcomesRequest}
 */
proto.workflow.ListStepOutcomesRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setWorkflowName(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.ListStepOutcomesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.ListStepOutcomesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.ListStepOutcomesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListStepOutcomesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getWorkflowName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.ListStepOutcomesRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ListStepOutcomesRequest} returns this
 */
proto.workflow.ListStepOutcomesRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string workflow_name = 2;
 * @return {string}
 */
proto.workflow.ListStepOutcomesRequest.prototype.getWorkflowName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ListStepOutcomesRequest} returns this
 */
proto.workflow.ListStepOutcomesRequest.prototype.setWorkflowName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.workflow.ListStepOutcomesResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.ListStepOutcomesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.ListStepOutcomesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.ListStepOutcomesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListStepOutcomesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
outcomesList: jspb.Message.toObjectList(msg.getOutcomesList(),
    proto.workflow.WorkflowStepOutcome.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.ListStepOutcomesResponse}
 */
proto.workflow.ListStepOutcomesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.ListStepOutcomesResponse;
  return proto.workflow.ListStepOutcomesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.ListStepOutcomesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.ListStepOutcomesResponse}
 */
proto.workflow.ListStepOutcomesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.workflow.WorkflowStepOutcome;
      reader.readMessage(value,proto.workflow.WorkflowStepOutcome.deserializeBinaryFromReader);
      msg.addOutcomes(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.ListStepOutcomesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.ListStepOutcomesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.ListStepOutcomesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListStepOutcomesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOutcomesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.workflow.WorkflowStepOutcome.serializeBinaryToWriter
    );
  }
};


/**
 * repeated WorkflowStepOutcome outcomes = 1;
 * @return {!Array<!proto.workflow.WorkflowStepOutcome>}
 */
proto.workflow.ListStepOutcomesResponse.prototype.getOutcomesList = function() {
  return /** @type{!Array<!proto.workflow.WorkflowStepOutcome>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.workflow.WorkflowStepOutcome, 1));
};


/**
 * @param {!Array<!proto.workflow.WorkflowStepOutcome>} value
 * @return {!proto.workflow.ListStepOutcomesResponse} returns this
*/
proto.workflow.ListStepOutcomesResponse.prototype.setOutcomesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.workflow.WorkflowStepOutcome=} opt_value
 * @param {number=} opt_index
 * @return {!proto.workflow.WorkflowStepOutcome}
 */
proto.workflow.ListStepOutcomesResponse.prototype.addOutcomes = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.workflow.WorkflowStepOutcome, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.ListStepOutcomesResponse} returns this
 */
proto.workflow.ListStepOutcomesResponse.prototype.clearOutcomesList = function() {
  return this.setOutcomesList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.PhaseTransitionEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.PhaseTransitionEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.PhaseTransitionEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.PhaseTransitionEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
resourceType: jspb.Message.getFieldWithDefault(msg, 2, ""),
resourceName: jspb.Message.getFieldWithDefault(msg, 3, ""),
eventAt: (f = msg.getEventAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
eventId: jspb.Message.getFieldWithDefault(msg, 5, ""),
fromPhase: jspb.Message.getFieldWithDefault(msg, 6, ""),
toPhase: jspb.Message.getFieldWithDefault(msg, 7, ""),
reason: jspb.Message.getFieldWithDefault(msg, 8, ""),
caller: jspb.Message.getFieldWithDefault(msg, 9, ""),
blocked: jspb.Message.getBooleanFieldWithDefault(msg, 10, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.PhaseTransitionEvent}
 */
proto.workflow.PhaseTransitionEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.PhaseTransitionEvent;
  return proto.workflow.PhaseTransitionEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.PhaseTransitionEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.PhaseTransitionEvent}
 */
proto.workflow.PhaseTransitionEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setResourceType(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setResourceName(value);
      break;
    case 4:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setEventAt(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setEventId(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setFromPhase(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setToPhase(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setReason(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setCaller(value);
      break;
    case 10:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setBlocked(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.PhaseTransitionEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.PhaseTransitionEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.PhaseTransitionEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.PhaseTransitionEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getResourceType();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getResourceName();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getEventAt();
  if (f != null) {
    writer.writeMessage(
      4,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getEventId();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getFromPhase();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getToPhase();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getReason();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
  f = message.getCaller();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
  f = message.getBlocked();
  if (f) {
    writer.writeBool(
      10,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.PhaseTransitionEvent.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.PhaseTransitionEvent} returns this
 */
proto.workflow.PhaseTransitionEvent.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string resource_type = 2;
 * @return {string}
 */
proto.workflow.PhaseTransitionEvent.prototype.getResourceType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.PhaseTransitionEvent} returns this
 */
proto.workflow.PhaseTransitionEvent.prototype.setResourceType = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string resource_name = 3;
 * @return {string}
 */
proto.workflow.PhaseTransitionEvent.prototype.getResourceName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.PhaseTransitionEvent} returns this
 */
proto.workflow.PhaseTransitionEvent.prototype.setResourceName = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional google.protobuf.Timestamp event_at = 4;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.PhaseTransitionEvent.prototype.getEventAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 4));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.PhaseTransitionEvent} returns this
*/
proto.workflow.PhaseTransitionEvent.prototype.setEventAt = function(value) {
  return jspb.Message.setWrapperField(this, 4, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.PhaseTransitionEvent} returns this
 */
proto.workflow.PhaseTransitionEvent.prototype.clearEventAt = function() {
  return this.setEventAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.PhaseTransitionEvent.prototype.hasEventAt = function() {
  return jspb.Message.getField(this, 4) != null;
};


/**
 * optional string event_id = 5;
 * @return {string}
 */
proto.workflow.PhaseTransitionEvent.prototype.getEventId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.PhaseTransitionEvent} returns this
 */
proto.workflow.PhaseTransitionEvent.prototype.setEventId = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string from_phase = 6;
 * @return {string}
 */
proto.workflow.PhaseTransitionEvent.prototype.getFromPhase = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.PhaseTransitionEvent} returns this
 */
proto.workflow.PhaseTransitionEvent.prototype.setFromPhase = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string to_phase = 7;
 * @return {string}
 */
proto.workflow.PhaseTransitionEvent.prototype.getToPhase = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.PhaseTransitionEvent} returns this
 */
proto.workflow.PhaseTransitionEvent.prototype.setToPhase = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional string reason = 8;
 * @return {string}
 */
proto.workflow.PhaseTransitionEvent.prototype.getReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.PhaseTransitionEvent} returns this
 */
proto.workflow.PhaseTransitionEvent.prototype.setReason = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};


/**
 * optional string caller = 9;
 * @return {string}
 */
proto.workflow.PhaseTransitionEvent.prototype.getCaller = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.PhaseTransitionEvent} returns this
 */
proto.workflow.PhaseTransitionEvent.prototype.setCaller = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};


/**
 * optional bool blocked = 10;
 * @return {boolean}
 */
proto.workflow.PhaseTransitionEvent.prototype.getBlocked = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 10, false));
};


/**
 * @param {boolean} value
 * @return {!proto.workflow.PhaseTransitionEvent} returns this
 */
proto.workflow.PhaseTransitionEvent.prototype.setBlocked = function(value) {
  return jspb.Message.setProto3BooleanField(this, 10, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.RecordPhaseTransitionRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.RecordPhaseTransitionRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.RecordPhaseTransitionRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.RecordPhaseTransitionRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
resourceType: jspb.Message.getFieldWithDefault(msg, 2, ""),
resourceName: jspb.Message.getFieldWithDefault(msg, 3, ""),
fromPhase: jspb.Message.getFieldWithDefault(msg, 4, ""),
toPhase: jspb.Message.getFieldWithDefault(msg, 5, ""),
reason: jspb.Message.getFieldWithDefault(msg, 6, ""),
caller: jspb.Message.getFieldWithDefault(msg, 7, ""),
blocked: jspb.Message.getBooleanFieldWithDefault(msg, 8, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.RecordPhaseTransitionRequest}
 */
proto.workflow.RecordPhaseTransitionRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.RecordPhaseTransitionRequest;
  return proto.workflow.RecordPhaseTransitionRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.RecordPhaseTransitionRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.RecordPhaseTransitionRequest}
 */
proto.workflow.RecordPhaseTransitionRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setResourceType(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setResourceName(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setFromPhase(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setToPhase(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setReason(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setCaller(value);
      break;
    case 8:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setBlocked(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.RecordPhaseTransitionRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.RecordPhaseTransitionRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.RecordPhaseTransitionRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.RecordPhaseTransitionRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getResourceType();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getResourceName();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getFromPhase();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getToPhase();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getReason();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getCaller();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getBlocked();
  if (f) {
    writer.writeBool(
      8,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.RecordPhaseTransitionRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordPhaseTransitionRequest} returns this
 */
proto.workflow.RecordPhaseTransitionRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string resource_type = 2;
 * @return {string}
 */
proto.workflow.RecordPhaseTransitionRequest.prototype.getResourceType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordPhaseTransitionRequest} returns this
 */
proto.workflow.RecordPhaseTransitionRequest.prototype.setResourceType = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string resource_name = 3;
 * @return {string}
 */
proto.workflow.RecordPhaseTransitionRequest.prototype.getResourceName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordPhaseTransitionRequest} returns this
 */
proto.workflow.RecordPhaseTransitionRequest.prototype.setResourceName = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string from_phase = 4;
 * @return {string}
 */
proto.workflow.RecordPhaseTransitionRequest.prototype.getFromPhase = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordPhaseTransitionRequest} returns this
 */
proto.workflow.RecordPhaseTransitionRequest.prototype.setFromPhase = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string to_phase = 5;
 * @return {string}
 */
proto.workflow.RecordPhaseTransitionRequest.prototype.getToPhase = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordPhaseTransitionRequest} returns this
 */
proto.workflow.RecordPhaseTransitionRequest.prototype.setToPhase = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string reason = 6;
 * @return {string}
 */
proto.workflow.RecordPhaseTransitionRequest.prototype.getReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordPhaseTransitionRequest} returns this
 */
proto.workflow.RecordPhaseTransitionRequest.prototype.setReason = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string caller = 7;
 * @return {string}
 */
proto.workflow.RecordPhaseTransitionRequest.prototype.getCaller = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordPhaseTransitionRequest} returns this
 */
proto.workflow.RecordPhaseTransitionRequest.prototype.setCaller = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional bool blocked = 8;
 * @return {boolean}
 */
proto.workflow.RecordPhaseTransitionRequest.prototype.getBlocked = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 8, false));
};


/**
 * @param {boolean} value
 * @return {!proto.workflow.RecordPhaseTransitionRequest} returns this
 */
proto.workflow.RecordPhaseTransitionRequest.prototype.setBlocked = function(value) {
  return jspb.Message.setProto3BooleanField(this, 8, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.ListPhaseTransitionsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.ListPhaseTransitionsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.ListPhaseTransitionsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListPhaseTransitionsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
resourceType: jspb.Message.getFieldWithDefault(msg, 2, ""),
resourceName: jspb.Message.getFieldWithDefault(msg, 3, ""),
limit: jspb.Message.getFieldWithDefault(msg, 4, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.ListPhaseTransitionsRequest}
 */
proto.workflow.ListPhaseTransitionsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.ListPhaseTransitionsRequest;
  return proto.workflow.ListPhaseTransitionsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.ListPhaseTransitionsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.ListPhaseTransitionsRequest}
 */
proto.workflow.ListPhaseTransitionsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setResourceType(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setResourceName(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setLimit(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.ListPhaseTransitionsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.ListPhaseTransitionsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.ListPhaseTransitionsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListPhaseTransitionsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getResourceType();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getResourceName();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getLimit();
  if (f !== 0) {
    writer.writeInt32(
      4,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.ListPhaseTransitionsRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ListPhaseTransitionsRequest} returns this
 */
proto.workflow.ListPhaseTransitionsRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string resource_type = 2;
 * @return {string}
 */
proto.workflow.ListPhaseTransitionsRequest.prototype.getResourceType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ListPhaseTransitionsRequest} returns this
 */
proto.workflow.ListPhaseTransitionsRequest.prototype.setResourceType = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string resource_name = 3;
 * @return {string}
 */
proto.workflow.ListPhaseTransitionsRequest.prototype.getResourceName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ListPhaseTransitionsRequest} returns this
 */
proto.workflow.ListPhaseTransitionsRequest.prototype.setResourceName = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional int32 limit = 4;
 * @return {number}
 */
proto.workflow.ListPhaseTransitionsRequest.prototype.getLimit = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.ListPhaseTransitionsRequest} returns this
 */
proto.workflow.ListPhaseTransitionsRequest.prototype.setLimit = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.workflow.ListPhaseTransitionsResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.ListPhaseTransitionsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.ListPhaseTransitionsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.ListPhaseTransitionsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListPhaseTransitionsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
eventsList: jspb.Message.toObjectList(msg.getEventsList(),
    proto.workflow.PhaseTransitionEvent.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.ListPhaseTransitionsResponse}
 */
proto.workflow.ListPhaseTransitionsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.ListPhaseTransitionsResponse;
  return proto.workflow.ListPhaseTransitionsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.ListPhaseTransitionsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.ListPhaseTransitionsResponse}
 */
proto.workflow.ListPhaseTransitionsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.workflow.PhaseTransitionEvent;
      reader.readMessage(value,proto.workflow.PhaseTransitionEvent.deserializeBinaryFromReader);
      msg.addEvents(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.ListPhaseTransitionsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.ListPhaseTransitionsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.ListPhaseTransitionsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListPhaseTransitionsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getEventsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.workflow.PhaseTransitionEvent.serializeBinaryToWriter
    );
  }
};


/**
 * repeated PhaseTransitionEvent events = 1;
 * @return {!Array<!proto.workflow.PhaseTransitionEvent>}
 */
proto.workflow.ListPhaseTransitionsResponse.prototype.getEventsList = function() {
  return /** @type{!Array<!proto.workflow.PhaseTransitionEvent>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.workflow.PhaseTransitionEvent, 1));
};


/**
 * @param {!Array<!proto.workflow.PhaseTransitionEvent>} value
 * @return {!proto.workflow.ListPhaseTransitionsResponse} returns this
*/
proto.workflow.ListPhaseTransitionsResponse.prototype.setEventsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.workflow.PhaseTransitionEvent=} opt_value
 * @param {number=} opt_index
 * @return {!proto.workflow.PhaseTransitionEvent}
 */
proto.workflow.ListPhaseTransitionsResponse.prototype.addEvents = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.workflow.PhaseTransitionEvent, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.ListPhaseTransitionsResponse} returns this
 */
proto.workflow.ListPhaseTransitionsResponse.prototype.clearEventsList = function() {
  return this.setEventsList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.DriftUnresolved.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.DriftUnresolved.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.DriftUnresolved} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.DriftUnresolved.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
driftType: jspb.Message.getFieldWithDefault(msg, 2, ""),
entityRef: jspb.Message.getFieldWithDefault(msg, 3, ""),
consecutiveCycles: jspb.Message.getFieldWithDefault(msg, 4, 0),
firstObservedAt: (f = msg.getFirstObservedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
lastObservedAt: (f = msg.getLastObservedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
chosenWorkflow: jspb.Message.getFieldWithDefault(msg, 7, ""),
lastRemediationId: jspb.Message.getFieldWithDefault(msg, 8, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.DriftUnresolved}
 */
proto.workflow.DriftUnresolved.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.DriftUnresolved;
  return proto.workflow.DriftUnresolved.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.DriftUnresolved} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.DriftUnresolved}
 */
proto.workflow.DriftUnresolved.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setDriftType(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setEntityRef(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setConsecutiveCycles(value);
      break;
    case 5:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setFirstObservedAt(value);
      break;
    case 6:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setLastObservedAt(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setChosenWorkflow(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setLastRemediationId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.DriftUnresolved.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.DriftUnresolved.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.DriftUnresolved} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.DriftUnresolved.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getDriftType();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getEntityRef();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getConsecutiveCycles();
  if (f !== 0) {
    writer.writeInt32(
      4,
      f
    );
  }
  f = message.getFirstObservedAt();
  if (f != null) {
    writer.writeMessage(
      5,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getLastObservedAt();
  if (f != null) {
    writer.writeMessage(
      6,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getChosenWorkflow();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getLastRemediationId();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.DriftUnresolved.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.DriftUnresolved} returns this
 */
proto.workflow.DriftUnresolved.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string drift_type = 2;
 * @return {string}
 */
proto.workflow.DriftUnresolved.prototype.getDriftType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.DriftUnresolved} returns this
 */
proto.workflow.DriftUnresolved.prototype.setDriftType = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string entity_ref = 3;
 * @return {string}
 */
proto.workflow.DriftUnresolved.prototype.getEntityRef = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.DriftUnresolved} returns this
 */
proto.workflow.DriftUnresolved.prototype.setEntityRef = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional int32 consecutive_cycles = 4;
 * @return {number}
 */
proto.workflow.DriftUnresolved.prototype.getConsecutiveCycles = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.DriftUnresolved} returns this
 */
proto.workflow.DriftUnresolved.prototype.setConsecutiveCycles = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional google.protobuf.Timestamp first_observed_at = 5;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.DriftUnresolved.prototype.getFirstObservedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 5));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.DriftUnresolved} returns this
*/
proto.workflow.DriftUnresolved.prototype.setFirstObservedAt = function(value) {
  return jspb.Message.setWrapperField(this, 5, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.DriftUnresolved} returns this
 */
proto.workflow.DriftUnresolved.prototype.clearFirstObservedAt = function() {
  return this.setFirstObservedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.DriftUnresolved.prototype.hasFirstObservedAt = function() {
  return jspb.Message.getField(this, 5) != null;
};


/**
 * optional google.protobuf.Timestamp last_observed_at = 6;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.DriftUnresolved.prototype.getLastObservedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 6));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.DriftUnresolved} returns this
*/
proto.workflow.DriftUnresolved.prototype.setLastObservedAt = function(value) {
  return jspb.Message.setWrapperField(this, 6, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.DriftUnresolved} returns this
 */
proto.workflow.DriftUnresolved.prototype.clearLastObservedAt = function() {
  return this.setLastObservedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.DriftUnresolved.prototype.hasLastObservedAt = function() {
  return jspb.Message.getField(this, 6) != null;
};


/**
 * optional string chosen_workflow = 7;
 * @return {string}
 */
proto.workflow.DriftUnresolved.prototype.getChosenWorkflow = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.DriftUnresolved} returns this
 */
proto.workflow.DriftUnresolved.prototype.setChosenWorkflow = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional string last_remediation_id = 8;
 * @return {string}
 */
proto.workflow.DriftUnresolved.prototype.getLastRemediationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.DriftUnresolved} returns this
 */
proto.workflow.DriftUnresolved.prototype.setLastRemediationId = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.RecordDriftObservationRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.RecordDriftObservationRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.RecordDriftObservationRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.RecordDriftObservationRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
driftType: jspb.Message.getFieldWithDefault(msg, 2, ""),
entityRef: jspb.Message.getFieldWithDefault(msg, 3, ""),
chosenWorkflow: jspb.Message.getFieldWithDefault(msg, 4, ""),
remediationId: jspb.Message.getFieldWithDefault(msg, 5, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.RecordDriftObservationRequest}
 */
proto.workflow.RecordDriftObservationRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.RecordDriftObservationRequest;
  return proto.workflow.RecordDriftObservationRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.RecordDriftObservationRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.RecordDriftObservationRequest}
 */
proto.workflow.RecordDriftObservationRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setDriftType(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setEntityRef(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setChosenWorkflow(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setRemediationId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.RecordDriftObservationRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.RecordDriftObservationRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.RecordDriftObservationRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.RecordDriftObservationRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getDriftType();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getEntityRef();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getChosenWorkflow();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getRemediationId();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.RecordDriftObservationRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordDriftObservationRequest} returns this
 */
proto.workflow.RecordDriftObservationRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string drift_type = 2;
 * @return {string}
 */
proto.workflow.RecordDriftObservationRequest.prototype.getDriftType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordDriftObservationRequest} returns this
 */
proto.workflow.RecordDriftObservationRequest.prototype.setDriftType = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string entity_ref = 3;
 * @return {string}
 */
proto.workflow.RecordDriftObservationRequest.prototype.getEntityRef = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordDriftObservationRequest} returns this
 */
proto.workflow.RecordDriftObservationRequest.prototype.setEntityRef = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string chosen_workflow = 4;
 * @return {string}
 */
proto.workflow.RecordDriftObservationRequest.prototype.getChosenWorkflow = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordDriftObservationRequest} returns this
 */
proto.workflow.RecordDriftObservationRequest.prototype.setChosenWorkflow = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string remediation_id = 5;
 * @return {string}
 */
proto.workflow.RecordDriftObservationRequest.prototype.getRemediationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RecordDriftObservationRequest} returns this
 */
proto.workflow.RecordDriftObservationRequest.prototype.setRemediationId = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.ClearDriftObservationRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.ClearDriftObservationRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.ClearDriftObservationRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ClearDriftObservationRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
driftType: jspb.Message.getFieldWithDefault(msg, 2, ""),
entityRef: jspb.Message.getFieldWithDefault(msg, 3, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.ClearDriftObservationRequest}
 */
proto.workflow.ClearDriftObservationRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.ClearDriftObservationRequest;
  return proto.workflow.ClearDriftObservationRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.ClearDriftObservationRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.ClearDriftObservationRequest}
 */
proto.workflow.ClearDriftObservationRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setDriftType(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setEntityRef(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.ClearDriftObservationRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.ClearDriftObservationRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.ClearDriftObservationRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ClearDriftObservationRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getDriftType();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getEntityRef();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.ClearDriftObservationRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ClearDriftObservationRequest} returns this
 */
proto.workflow.ClearDriftObservationRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string drift_type = 2;
 * @return {string}
 */
proto.workflow.ClearDriftObservationRequest.prototype.getDriftType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ClearDriftObservationRequest} returns this
 */
proto.workflow.ClearDriftObservationRequest.prototype.setDriftType = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string entity_ref = 3;
 * @return {string}
 */
proto.workflow.ClearDriftObservationRequest.prototype.getEntityRef = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ClearDriftObservationRequest} returns this
 */
proto.workflow.ClearDriftObservationRequest.prototype.setEntityRef = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.ListDriftUnresolvedRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.ListDriftUnresolvedRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.ListDriftUnresolvedRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListDriftUnresolvedRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
driftType: jspb.Message.getFieldWithDefault(msg, 2, ""),
minCycles: jspb.Message.getFieldWithDefault(msg, 3, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.ListDriftUnresolvedRequest}
 */
proto.workflow.ListDriftUnresolvedRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.ListDriftUnresolvedRequest;
  return proto.workflow.ListDriftUnresolvedRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.ListDriftUnresolvedRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.ListDriftUnresolvedRequest}
 */
proto.workflow.ListDriftUnresolvedRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setDriftType(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setMinCycles(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.ListDriftUnresolvedRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.ListDriftUnresolvedRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.ListDriftUnresolvedRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListDriftUnresolvedRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getDriftType();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getMinCycles();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.ListDriftUnresolvedRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ListDriftUnresolvedRequest} returns this
 */
proto.workflow.ListDriftUnresolvedRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string drift_type = 2;
 * @return {string}
 */
proto.workflow.ListDriftUnresolvedRequest.prototype.getDriftType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ListDriftUnresolvedRequest} returns this
 */
proto.workflow.ListDriftUnresolvedRequest.prototype.setDriftType = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int32 min_cycles = 3;
 * @return {number}
 */
proto.workflow.ListDriftUnresolvedRequest.prototype.getMinCycles = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.ListDriftUnresolvedRequest} returns this
 */
proto.workflow.ListDriftUnresolvedRequest.prototype.setMinCycles = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.workflow.ListDriftUnresolvedResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.ListDriftUnresolvedResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.ListDriftUnresolvedResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.ListDriftUnresolvedResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListDriftUnresolvedResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
itemsList: jspb.Message.toObjectList(msg.getItemsList(),
    proto.workflow.DriftUnresolved.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.ListDriftUnresolvedResponse}
 */
proto.workflow.ListDriftUnresolvedResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.ListDriftUnresolvedResponse;
  return proto.workflow.ListDriftUnresolvedResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.ListDriftUnresolvedResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.ListDriftUnresolvedResponse}
 */
proto.workflow.ListDriftUnresolvedResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.workflow.DriftUnresolved;
      reader.readMessage(value,proto.workflow.DriftUnresolved.deserializeBinaryFromReader);
      msg.addItems(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.ListDriftUnresolvedResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.ListDriftUnresolvedResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.ListDriftUnresolvedResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListDriftUnresolvedResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getItemsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.workflow.DriftUnresolved.serializeBinaryToWriter
    );
  }
};


/**
 * repeated DriftUnresolved items = 1;
 * @return {!Array<!proto.workflow.DriftUnresolved>}
 */
proto.workflow.ListDriftUnresolvedResponse.prototype.getItemsList = function() {
  return /** @type{!Array<!proto.workflow.DriftUnresolved>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.workflow.DriftUnresolved, 1));
};


/**
 * @param {!Array<!proto.workflow.DriftUnresolved>} value
 * @return {!proto.workflow.ListDriftUnresolvedResponse} returns this
*/
proto.workflow.ListDriftUnresolvedResponse.prototype.setItemsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.workflow.DriftUnresolved=} opt_value
 * @param {number=} opt_index
 * @return {!proto.workflow.DriftUnresolved}
 */
proto.workflow.ListDriftUnresolvedResponse.prototype.addItems = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.workflow.DriftUnresolved, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.ListDriftUnresolvedResponse} returns this
 */
proto.workflow.ListDriftUnresolvedResponse.prototype.clearItemsList = function() {
  return this.setItemsList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.EvidenceItem.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.EvidenceItem.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.EvidenceItem} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.EvidenceItem.toObject = function(includeInstance, msg) {
  var f, obj = {
id: jspb.Message.getFieldWithDefault(msg, 1, ""),
provenance: jspb.Message.getFieldWithDefault(msg, 2, 0),
source: jspb.Message.getFieldWithDefault(msg, 3, ""),
summary: jspb.Message.getFieldWithDefault(msg, 4, ""),
factsMap: (f = msg.getFactsMap()) ? f.toObject(includeInstance, undefined) : [],
observedAt: (f = msg.getObservedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.EvidenceItem}
 */
proto.workflow.EvidenceItem.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.EvidenceItem;
  return proto.workflow.EvidenceItem.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.EvidenceItem} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.EvidenceItem}
 */
proto.workflow.EvidenceItem.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    case 2:
      var value = /** @type {!proto.workflow.Provenance} */ (reader.readEnum());
      msg.setProvenance(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setSource(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setSummary(value);
      break;
    case 5:
      var value = msg.getFactsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
      break;
    case 6:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setObservedAt(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.EvidenceItem.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.EvidenceItem.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.EvidenceItem} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.EvidenceItem.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getProvenance();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
  f = message.getSource();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getSummary();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getFactsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(5, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
  f = message.getObservedAt();
  if (f != null) {
    writer.writeMessage(
      6,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.workflow.EvidenceItem.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.EvidenceItem} returns this
 */
proto.workflow.EvidenceItem.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional Provenance provenance = 2;
 * @return {!proto.workflow.Provenance}
 */
proto.workflow.EvidenceItem.prototype.getProvenance = function() {
  return /** @type {!proto.workflow.Provenance} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.workflow.Provenance} value
 * @return {!proto.workflow.EvidenceItem} returns this
 */
proto.workflow.EvidenceItem.prototype.setProvenance = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional string source = 3;
 * @return {string}
 */
proto.workflow.EvidenceItem.prototype.getSource = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.EvidenceItem} returns this
 */
proto.workflow.EvidenceItem.prototype.setSource = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string summary = 4;
 * @return {string}
 */
proto.workflow.EvidenceItem.prototype.getSummary = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.EvidenceItem} returns this
 */
proto.workflow.EvidenceItem.prototype.setSummary = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * map<string, string> facts = 5;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.workflow.EvidenceItem.prototype.getFactsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 5, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.workflow.EvidenceItem} returns this
 */
proto.workflow.EvidenceItem.prototype.clearFactsMap = function() {
  this.getFactsMap().clear();
  return this;
};


/**
 * optional google.protobuf.Timestamp observed_at = 6;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.EvidenceItem.prototype.getObservedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 6));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.EvidenceItem} returns this
*/
proto.workflow.EvidenceItem.prototype.setObservedAt = function(value) {
  return jspb.Message.setWrapperField(this, 6, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.EvidenceItem} returns this
 */
proto.workflow.EvidenceItem.prototype.clearObservedAt = function() {
  return this.setObservedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.EvidenceItem.prototype.hasObservedAt = function() {
  return jspb.Message.getField(this, 6) != null;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.workflow.DiagnosisItem.repeatedFields_ = [5];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.DiagnosisItem.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.DiagnosisItem.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.DiagnosisItem} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.DiagnosisItem.toObject = function(includeInstance, msg) {
  var f, obj = {
id: jspb.Message.getFieldWithDefault(msg, 1, ""),
source: jspb.Message.getFieldWithDefault(msg, 2, ""),
invariantId: jspb.Message.getFieldWithDefault(msg, 3, ""),
summary: jspb.Message.getFieldWithDefault(msg, 4, ""),
citedEvidenceIdsList: (f = jspb.Message.getRepeatedField(msg, 5)) == null ? undefined : f,
severity: jspb.Message.getFieldWithDefault(msg, 6, 0),
diagnosedAt: (f = msg.getDiagnosedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.DiagnosisItem}
 */
proto.workflow.DiagnosisItem.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.DiagnosisItem;
  return proto.workflow.DiagnosisItem.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.DiagnosisItem} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.DiagnosisItem}
 */
proto.workflow.DiagnosisItem.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setSource(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setInvariantId(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setSummary(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.addCitedEvidenceIds(value);
      break;
    case 6:
      var value = /** @type {!proto.workflow.IncidentSeverity} */ (reader.readEnum());
      msg.setSeverity(value);
      break;
    case 7:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setDiagnosedAt(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.DiagnosisItem.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.DiagnosisItem.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.DiagnosisItem} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.DiagnosisItem.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getSource();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getInvariantId();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getSummary();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getCitedEvidenceIdsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      5,
      f
    );
  }
  f = message.getSeverity();
  if (f !== 0.0) {
    writer.writeEnum(
      6,
      f
    );
  }
  f = message.getDiagnosedAt();
  if (f != null) {
    writer.writeMessage(
      7,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.workflow.DiagnosisItem.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.DiagnosisItem} returns this
 */
proto.workflow.DiagnosisItem.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string source = 2;
 * @return {string}
 */
proto.workflow.DiagnosisItem.prototype.getSource = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.DiagnosisItem} returns this
 */
proto.workflow.DiagnosisItem.prototype.setSource = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string invariant_id = 3;
 * @return {string}
 */
proto.workflow.DiagnosisItem.prototype.getInvariantId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.DiagnosisItem} returns this
 */
proto.workflow.DiagnosisItem.prototype.setInvariantId = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string summary = 4;
 * @return {string}
 */
proto.workflow.DiagnosisItem.prototype.getSummary = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.DiagnosisItem} returns this
 */
proto.workflow.DiagnosisItem.prototype.setSummary = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * repeated string cited_evidence_ids = 5;
 * @return {!Array<string>}
 */
proto.workflow.DiagnosisItem.prototype.getCitedEvidenceIdsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 5));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.workflow.DiagnosisItem} returns this
 */
proto.workflow.DiagnosisItem.prototype.setCitedEvidenceIdsList = function(value) {
  return jspb.Message.setField(this, 5, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.workflow.DiagnosisItem} returns this
 */
proto.workflow.DiagnosisItem.prototype.addCitedEvidenceIds = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 5, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.DiagnosisItem} returns this
 */
proto.workflow.DiagnosisItem.prototype.clearCitedEvidenceIdsList = function() {
  return this.setCitedEvidenceIdsList([]);
};


/**
 * optional IncidentSeverity severity = 6;
 * @return {!proto.workflow.IncidentSeverity}
 */
proto.workflow.DiagnosisItem.prototype.getSeverity = function() {
  return /** @type {!proto.workflow.IncidentSeverity} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {!proto.workflow.IncidentSeverity} value
 * @return {!proto.workflow.DiagnosisItem} returns this
 */
proto.workflow.DiagnosisItem.prototype.setSeverity = function(value) {
  return jspb.Message.setProto3EnumField(this, 6, value);
};


/**
 * optional google.protobuf.Timestamp diagnosed_at = 7;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.DiagnosisItem.prototype.getDiagnosedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 7));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.DiagnosisItem} returns this
*/
proto.workflow.DiagnosisItem.prototype.setDiagnosedAt = function(value) {
  return jspb.Message.setWrapperField(this, 7, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.DiagnosisItem} returns this
 */
proto.workflow.DiagnosisItem.prototype.clearDiagnosedAt = function() {
  return this.setDiagnosedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.DiagnosisItem.prototype.hasDiagnosedAt = function() {
  return jspb.Message.getField(this, 7) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.CodePatch.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.CodePatch.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.CodePatch} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.CodePatch.toObject = function(includeInstance, msg) {
  var f, obj = {
filePath: jspb.Message.getFieldWithDefault(msg, 1, ""),
line: jspb.Message.getFieldWithDefault(msg, 2, 0),
oldText: jspb.Message.getFieldWithDefault(msg, 3, ""),
newText: jspb.Message.getFieldWithDefault(msg, 4, ""),
repository: jspb.Message.getFieldWithDefault(msg, 5, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.CodePatch}
 */
proto.workflow.CodePatch.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.CodePatch;
  return proto.workflow.CodePatch.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.CodePatch} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.CodePatch}
 */
proto.workflow.CodePatch.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setFilePath(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setLine(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setOldText(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setNewText(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setRepository(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.CodePatch.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.CodePatch.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.CodePatch} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.CodePatch.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getFilePath();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getLine();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getOldText();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getNewText();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getRepository();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
};


/**
 * optional string file_path = 1;
 * @return {string}
 */
proto.workflow.CodePatch.prototype.getFilePath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.CodePatch} returns this
 */
proto.workflow.CodePatch.prototype.setFilePath = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int32 line = 2;
 * @return {number}
 */
proto.workflow.CodePatch.prototype.getLine = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.CodePatch} returns this
 */
proto.workflow.CodePatch.prototype.setLine = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string old_text = 3;
 * @return {string}
 */
proto.workflow.CodePatch.prototype.getOldText = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.CodePatch} returns this
 */
proto.workflow.CodePatch.prototype.setOldText = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string new_text = 4;
 * @return {string}
 */
proto.workflow.CodePatch.prototype.getNewText = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.CodePatch} returns this
 */
proto.workflow.CodePatch.prototype.setNewText = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string repository = 5;
 * @return {string}
 */
proto.workflow.CodePatch.prototype.getRepository = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.CodePatch} returns this
 */
proto.workflow.CodePatch.prototype.setRepository = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.ConfigPatch.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.ConfigPatch.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.ConfigPatch} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ConfigPatch.toObject = function(includeInstance, msg) {
  var f, obj = {
targetPath: jspb.Message.getFieldWithDefault(msg, 1, ""),
oldValue: jspb.Message.getFieldWithDefault(msg, 2, ""),
newValue: jspb.Message.getFieldWithDefault(msg, 3, ""),
format: jspb.Message.getFieldWithDefault(msg, 4, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.ConfigPatch}
 */
proto.workflow.ConfigPatch.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.ConfigPatch;
  return proto.workflow.ConfigPatch.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.ConfigPatch} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.ConfigPatch}
 */
proto.workflow.ConfigPatch.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setTargetPath(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setOldValue(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setNewValue(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setFormat(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.ConfigPatch.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.ConfigPatch.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.ConfigPatch} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ConfigPatch.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTargetPath();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOldValue();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getNewValue();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getFormat();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * optional string target_path = 1;
 * @return {string}
 */
proto.workflow.ConfigPatch.prototype.getTargetPath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ConfigPatch} returns this
 */
proto.workflow.ConfigPatch.prototype.setTargetPath = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string old_value = 2;
 * @return {string}
 */
proto.workflow.ConfigPatch.prototype.getOldValue = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ConfigPatch} returns this
 */
proto.workflow.ConfigPatch.prototype.setOldValue = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string new_value = 3;
 * @return {string}
 */
proto.workflow.ConfigPatch.prototype.getNewValue = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ConfigPatch} returns this
 */
proto.workflow.ConfigPatch.prototype.setNewValue = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string format = 4;
 * @return {string}
 */
proto.workflow.ConfigPatch.prototype.getFormat = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ConfigPatch} returns this
 */
proto.workflow.ConfigPatch.prototype.setFormat = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.workflow.CommandList.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.CommandList.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.CommandList.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.CommandList} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.CommandList.toObject = function(includeInstance, msg) {
  var f, obj = {
commandsList: (f = jspb.Message.getRepeatedField(msg, 1)) == null ? undefined : f,
targetHost: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.CommandList}
 */
proto.workflow.CommandList.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.CommandList;
  return proto.workflow.CommandList.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.CommandList} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.CommandList}
 */
proto.workflow.CommandList.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.addCommands(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setTargetHost(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.CommandList.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.CommandList.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.CommandList} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.CommandList.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getCommandsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      1,
      f
    );
  }
  f = message.getTargetHost();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * repeated string commands = 1;
 * @return {!Array<string>}
 */
proto.workflow.CommandList.prototype.getCommandsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 1));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.workflow.CommandList} returns this
 */
proto.workflow.CommandList.prototype.setCommandsList = function(value) {
  return jspb.Message.setField(this, 1, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.workflow.CommandList} returns this
 */
proto.workflow.CommandList.prototype.addCommands = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 1, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.CommandList} returns this
 */
proto.workflow.CommandList.prototype.clearCommandsList = function() {
  return this.setCommandsList([]);
};


/**
 * optional string target_host = 2;
 * @return {string}
 */
proto.workflow.CommandList.prototype.getTargetHost = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.CommandList} returns this
 */
proto.workflow.CommandList.prototype.setTargetHost = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.workflow.RestartAction.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.RestartAction.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.RestartAction.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.RestartAction} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.RestartAction.toObject = function(includeInstance, msg) {
  var f, obj = {
unitNamesList: (f = jspb.Message.getRepeatedField(msg, 1)) == null ? undefined : f,
targetHost: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.RestartAction}
 */
proto.workflow.RestartAction.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.RestartAction;
  return proto.workflow.RestartAction.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.RestartAction} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.RestartAction}
 */
proto.workflow.RestartAction.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.addUnitNames(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setTargetHost(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.RestartAction.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.RestartAction.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.RestartAction} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.RestartAction.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUnitNamesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      1,
      f
    );
  }
  f = message.getTargetHost();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * repeated string unit_names = 1;
 * @return {!Array<string>}
 */
proto.workflow.RestartAction.prototype.getUnitNamesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 1));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.workflow.RestartAction} returns this
 */
proto.workflow.RestartAction.prototype.setUnitNamesList = function(value) {
  return jspb.Message.setField(this, 1, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.workflow.RestartAction} returns this
 */
proto.workflow.RestartAction.prototype.addUnitNames = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 1, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.RestartAction} returns this
 */
proto.workflow.RestartAction.prototype.clearUnitNamesList = function() {
  return this.setUnitNamesList([]);
};


/**
 * optional string target_host = 2;
 * @return {string}
 */
proto.workflow.RestartAction.prototype.getTargetHost = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.RestartAction} returns this
 */
proto.workflow.RestartAction.prototype.setTargetHost = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.workflow.ProposedFix.repeatedFields_ = [6,7];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.ProposedFix.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.ProposedFix.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.ProposedFix} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ProposedFix.toObject = function(includeInstance, msg) {
  var f, obj = {
id: jspb.Message.getFieldWithDefault(msg, 1, ""),
proposer: jspb.Message.getFieldWithDefault(msg, 2, ""),
summary: jspb.Message.getFieldWithDefault(msg, 3, ""),
confidence: jspb.Message.getFieldWithDefault(msg, 4, ""),
reasoning: jspb.Message.getFieldWithDefault(msg, 5, ""),
citedEvidenceIdsList: (f = jspb.Message.getRepeatedField(msg, 6)) == null ? undefined : f,
citedDiagnosisIdsList: (f = jspb.Message.getRepeatedField(msg, 7)) == null ? undefined : f,
codePatch: (f = msg.getCodePatch()) && proto.workflow.CodePatch.toObject(includeInstance, f),
configPatch: (f = msg.getConfigPatch()) && proto.workflow.ConfigPatch.toObject(includeInstance, f),
commandList: (f = msg.getCommandList()) && proto.workflow.CommandList.toObject(includeInstance, f),
restartAction: (f = msg.getRestartAction()) && proto.workflow.RestartAction.toObject(includeInstance, f),
status: jspb.Message.getFieldWithDefault(msg, 12, 0),
appliedBy: jspb.Message.getFieldWithDefault(msg, 13, ""),
appliedAt: (f = msg.getAppliedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
applicationResult: jspb.Message.getFieldWithDefault(msg, 15, ""),
proposedAt: (f = msg.getProposedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
targetIncidentId: jspb.Message.getFieldWithDefault(msg, 17, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.ProposedFix}
 */
proto.workflow.ProposedFix.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.ProposedFix;
  return proto.workflow.ProposedFix.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.ProposedFix} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.ProposedFix}
 */
proto.workflow.ProposedFix.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setProposer(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setSummary(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setConfidence(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setReasoning(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.addCitedEvidenceIds(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.addCitedDiagnosisIds(value);
      break;
    case 8:
      var value = new proto.workflow.CodePatch;
      reader.readMessage(value,proto.workflow.CodePatch.deserializeBinaryFromReader);
      msg.setCodePatch(value);
      break;
    case 9:
      var value = new proto.workflow.ConfigPatch;
      reader.readMessage(value,proto.workflow.ConfigPatch.deserializeBinaryFromReader);
      msg.setConfigPatch(value);
      break;
    case 10:
      var value = new proto.workflow.CommandList;
      reader.readMessage(value,proto.workflow.CommandList.deserializeBinaryFromReader);
      msg.setCommandList(value);
      break;
    case 11:
      var value = new proto.workflow.RestartAction;
      reader.readMessage(value,proto.workflow.RestartAction.deserializeBinaryFromReader);
      msg.setRestartAction(value);
      break;
    case 12:
      var value = /** @type {!proto.workflow.FixStatus} */ (reader.readEnum());
      msg.setStatus(value);
      break;
    case 13:
      var value = /** @type {string} */ (reader.readString());
      msg.setAppliedBy(value);
      break;
    case 14:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setAppliedAt(value);
      break;
    case 15:
      var value = /** @type {string} */ (reader.readString());
      msg.setApplicationResult(value);
      break;
    case 16:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setProposedAt(value);
      break;
    case 17:
      var value = /** @type {string} */ (reader.readString());
      msg.setTargetIncidentId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.ProposedFix.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.ProposedFix.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.ProposedFix} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ProposedFix.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getProposer();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getSummary();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getConfidence();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getReasoning();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getCitedEvidenceIdsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      6,
      f
    );
  }
  f = message.getCitedDiagnosisIdsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      7,
      f
    );
  }
  f = message.getCodePatch();
  if (f != null) {
    writer.writeMessage(
      8,
      f,
      proto.workflow.CodePatch.serializeBinaryToWriter
    );
  }
  f = message.getConfigPatch();
  if (f != null) {
    writer.writeMessage(
      9,
      f,
      proto.workflow.ConfigPatch.serializeBinaryToWriter
    );
  }
  f = message.getCommandList();
  if (f != null) {
    writer.writeMessage(
      10,
      f,
      proto.workflow.CommandList.serializeBinaryToWriter
    );
  }
  f = message.getRestartAction();
  if (f != null) {
    writer.writeMessage(
      11,
      f,
      proto.workflow.RestartAction.serializeBinaryToWriter
    );
  }
  f = message.getStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      12,
      f
    );
  }
  f = message.getAppliedBy();
  if (f.length > 0) {
    writer.writeString(
      13,
      f
    );
  }
  f = message.getAppliedAt();
  if (f != null) {
    writer.writeMessage(
      14,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getApplicationResult();
  if (f.length > 0) {
    writer.writeString(
      15,
      f
    );
  }
  f = message.getProposedAt();
  if (f != null) {
    writer.writeMessage(
      16,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getTargetIncidentId();
  if (f.length > 0) {
    writer.writeString(
      17,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.workflow.ProposedFix.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ProposedFix} returns this
 */
proto.workflow.ProposedFix.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string proposer = 2;
 * @return {string}
 */
proto.workflow.ProposedFix.prototype.getProposer = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ProposedFix} returns this
 */
proto.workflow.ProposedFix.prototype.setProposer = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string summary = 3;
 * @return {string}
 */
proto.workflow.ProposedFix.prototype.getSummary = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ProposedFix} returns this
 */
proto.workflow.ProposedFix.prototype.setSummary = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string confidence = 4;
 * @return {string}
 */
proto.workflow.ProposedFix.prototype.getConfidence = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ProposedFix} returns this
 */
proto.workflow.ProposedFix.prototype.setConfidence = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string reasoning = 5;
 * @return {string}
 */
proto.workflow.ProposedFix.prototype.getReasoning = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ProposedFix} returns this
 */
proto.workflow.ProposedFix.prototype.setReasoning = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * repeated string cited_evidence_ids = 6;
 * @return {!Array<string>}
 */
proto.workflow.ProposedFix.prototype.getCitedEvidenceIdsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 6));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.workflow.ProposedFix} returns this
 */
proto.workflow.ProposedFix.prototype.setCitedEvidenceIdsList = function(value) {
  return jspb.Message.setField(this, 6, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.workflow.ProposedFix} returns this
 */
proto.workflow.ProposedFix.prototype.addCitedEvidenceIds = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 6, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.ProposedFix} returns this
 */
proto.workflow.ProposedFix.prototype.clearCitedEvidenceIdsList = function() {
  return this.setCitedEvidenceIdsList([]);
};


/**
 * repeated string cited_diagnosis_ids = 7;
 * @return {!Array<string>}
 */
proto.workflow.ProposedFix.prototype.getCitedDiagnosisIdsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 7));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.workflow.ProposedFix} returns this
 */
proto.workflow.ProposedFix.prototype.setCitedDiagnosisIdsList = function(value) {
  return jspb.Message.setField(this, 7, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.workflow.ProposedFix} returns this
 */
proto.workflow.ProposedFix.prototype.addCitedDiagnosisIds = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 7, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.ProposedFix} returns this
 */
proto.workflow.ProposedFix.prototype.clearCitedDiagnosisIdsList = function() {
  return this.setCitedDiagnosisIdsList([]);
};


/**
 * optional CodePatch code_patch = 8;
 * @return {?proto.workflow.CodePatch}
 */
proto.workflow.ProposedFix.prototype.getCodePatch = function() {
  return /** @type{?proto.workflow.CodePatch} */ (
    jspb.Message.getWrapperField(this, proto.workflow.CodePatch, 8));
};


/**
 * @param {?proto.workflow.CodePatch|undefined} value
 * @return {!proto.workflow.ProposedFix} returns this
*/
proto.workflow.ProposedFix.prototype.setCodePatch = function(value) {
  return jspb.Message.setWrapperField(this, 8, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.ProposedFix} returns this
 */
proto.workflow.ProposedFix.prototype.clearCodePatch = function() {
  return this.setCodePatch(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.ProposedFix.prototype.hasCodePatch = function() {
  return jspb.Message.getField(this, 8) != null;
};


/**
 * optional ConfigPatch config_patch = 9;
 * @return {?proto.workflow.ConfigPatch}
 */
proto.workflow.ProposedFix.prototype.getConfigPatch = function() {
  return /** @type{?proto.workflow.ConfigPatch} */ (
    jspb.Message.getWrapperField(this, proto.workflow.ConfigPatch, 9));
};


/**
 * @param {?proto.workflow.ConfigPatch|undefined} value
 * @return {!proto.workflow.ProposedFix} returns this
*/
proto.workflow.ProposedFix.prototype.setConfigPatch = function(value) {
  return jspb.Message.setWrapperField(this, 9, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.ProposedFix} returns this
 */
proto.workflow.ProposedFix.prototype.clearConfigPatch = function() {
  return this.setConfigPatch(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.ProposedFix.prototype.hasConfigPatch = function() {
  return jspb.Message.getField(this, 9) != null;
};


/**
 * optional CommandList command_list = 10;
 * @return {?proto.workflow.CommandList}
 */
proto.workflow.ProposedFix.prototype.getCommandList = function() {
  return /** @type{?proto.workflow.CommandList} */ (
    jspb.Message.getWrapperField(this, proto.workflow.CommandList, 10));
};


/**
 * @param {?proto.workflow.CommandList|undefined} value
 * @return {!proto.workflow.ProposedFix} returns this
*/
proto.workflow.ProposedFix.prototype.setCommandList = function(value) {
  return jspb.Message.setWrapperField(this, 10, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.ProposedFix} returns this
 */
proto.workflow.ProposedFix.prototype.clearCommandList = function() {
  return this.setCommandList(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.ProposedFix.prototype.hasCommandList = function() {
  return jspb.Message.getField(this, 10) != null;
};


/**
 * optional RestartAction restart_action = 11;
 * @return {?proto.workflow.RestartAction}
 */
proto.workflow.ProposedFix.prototype.getRestartAction = function() {
  return /** @type{?proto.workflow.RestartAction} */ (
    jspb.Message.getWrapperField(this, proto.workflow.RestartAction, 11));
};


/**
 * @param {?proto.workflow.RestartAction|undefined} value
 * @return {!proto.workflow.ProposedFix} returns this
*/
proto.workflow.ProposedFix.prototype.setRestartAction = function(value) {
  return jspb.Message.setWrapperField(this, 11, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.ProposedFix} returns this
 */
proto.workflow.ProposedFix.prototype.clearRestartAction = function() {
  return this.setRestartAction(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.ProposedFix.prototype.hasRestartAction = function() {
  return jspb.Message.getField(this, 11) != null;
};


/**
 * optional FixStatus status = 12;
 * @return {!proto.workflow.FixStatus}
 */
proto.workflow.ProposedFix.prototype.getStatus = function() {
  return /** @type {!proto.workflow.FixStatus} */ (jspb.Message.getFieldWithDefault(this, 12, 0));
};


/**
 * @param {!proto.workflow.FixStatus} value
 * @return {!proto.workflow.ProposedFix} returns this
 */
proto.workflow.ProposedFix.prototype.setStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 12, value);
};


/**
 * optional string applied_by = 13;
 * @return {string}
 */
proto.workflow.ProposedFix.prototype.getAppliedBy = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 13, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ProposedFix} returns this
 */
proto.workflow.ProposedFix.prototype.setAppliedBy = function(value) {
  return jspb.Message.setProto3StringField(this, 13, value);
};


/**
 * optional google.protobuf.Timestamp applied_at = 14;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.ProposedFix.prototype.getAppliedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 14));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.ProposedFix} returns this
*/
proto.workflow.ProposedFix.prototype.setAppliedAt = function(value) {
  return jspb.Message.setWrapperField(this, 14, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.ProposedFix} returns this
 */
proto.workflow.ProposedFix.prototype.clearAppliedAt = function() {
  return this.setAppliedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.ProposedFix.prototype.hasAppliedAt = function() {
  return jspb.Message.getField(this, 14) != null;
};


/**
 * optional string application_result = 15;
 * @return {string}
 */
proto.workflow.ProposedFix.prototype.getApplicationResult = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 15, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ProposedFix} returns this
 */
proto.workflow.ProposedFix.prototype.setApplicationResult = function(value) {
  return jspb.Message.setProto3StringField(this, 15, value);
};


/**
 * optional google.protobuf.Timestamp proposed_at = 16;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.ProposedFix.prototype.getProposedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 16));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.ProposedFix} returns this
*/
proto.workflow.ProposedFix.prototype.setProposedAt = function(value) {
  return jspb.Message.setWrapperField(this, 16, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.ProposedFix} returns this
 */
proto.workflow.ProposedFix.prototype.clearProposedAt = function() {
  return this.setProposedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.ProposedFix.prototype.hasProposedAt = function() {
  return jspb.Message.getField(this, 16) != null;
};


/**
 * optional string target_incident_id = 17;
 * @return {string}
 */
proto.workflow.ProposedFix.prototype.getTargetIncidentId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 17, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ProposedFix} returns this
 */
proto.workflow.ProposedFix.prototype.setTargetIncidentId = function(value) {
  return jspb.Message.setProto3StringField(this, 17, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.workflow.Incident.repeatedFields_ = [11,12,13,20];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.Incident.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.Incident.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.Incident} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.Incident.toObject = function(includeInstance, msg) {
  var f, obj = {
id: jspb.Message.getFieldWithDefault(msg, 1, ""),
clusterId: jspb.Message.getFieldWithDefault(msg, 2, ""),
category: jspb.Message.getFieldWithDefault(msg, 3, ""),
signature: jspb.Message.getFieldWithDefault(msg, 4, ""),
status: jspb.Message.getFieldWithDefault(msg, 5, 0),
severity: jspb.Message.getFieldWithDefault(msg, 6, 0),
headline: jspb.Message.getFieldWithDefault(msg, 7, ""),
occurrenceCount: jspb.Message.getFieldWithDefault(msg, 8, 0),
firstSeenAt: (f = msg.getFirstSeenAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
lastSeenAt: (f = msg.getLastSeenAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
evidenceList: jspb.Message.toObjectList(msg.getEvidenceList(),
    proto.workflow.EvidenceItem.toObject, includeInstance),
diagnosesList: jspb.Message.toObjectList(msg.getDiagnosesList(),
    proto.workflow.DiagnosisItem.toObject, includeInstance),
proposedFixesList: jspb.Message.toObjectList(msg.getProposedFixesList(),
    proto.workflow.ProposedFix.toObject, includeInstance),
acknowledged: jspb.Message.getBooleanFieldWithDefault(msg, 14, false),
acknowledgedBy: jspb.Message.getFieldWithDefault(msg, 15, ""),
acknowledgedAt: (f = msg.getAcknowledgedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
assignedTo: jspb.Message.getFieldWithDefault(msg, 17, ""),
entityRef: jspb.Message.getFieldWithDefault(msg, 18, ""),
entityType: jspb.Message.getFieldWithDefault(msg, 19, ""),
relatedIncidentIdsList: (f = jspb.Message.getRepeatedField(msg, 20)) == null ? undefined : f
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.Incident}
 */
proto.workflow.Incident.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.Incident;
  return proto.workflow.Incident.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.Incident} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.Incident}
 */
proto.workflow.Incident.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setCategory(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setSignature(value);
      break;
    case 5:
      var value = /** @type {!proto.workflow.IncidentStatus} */ (reader.readEnum());
      msg.setStatus(value);
      break;
    case 6:
      var value = /** @type {!proto.workflow.IncidentSeverity} */ (reader.readEnum());
      msg.setSeverity(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setHeadline(value);
      break;
    case 8:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setOccurrenceCount(value);
      break;
    case 9:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setFirstSeenAt(value);
      break;
    case 10:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setLastSeenAt(value);
      break;
    case 11:
      var value = new proto.workflow.EvidenceItem;
      reader.readMessage(value,proto.workflow.EvidenceItem.deserializeBinaryFromReader);
      msg.addEvidence(value);
      break;
    case 12:
      var value = new proto.workflow.DiagnosisItem;
      reader.readMessage(value,proto.workflow.DiagnosisItem.deserializeBinaryFromReader);
      msg.addDiagnoses(value);
      break;
    case 13:
      var value = new proto.workflow.ProposedFix;
      reader.readMessage(value,proto.workflow.ProposedFix.deserializeBinaryFromReader);
      msg.addProposedFixes(value);
      break;
    case 14:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setAcknowledged(value);
      break;
    case 15:
      var value = /** @type {string} */ (reader.readString());
      msg.setAcknowledgedBy(value);
      break;
    case 16:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setAcknowledgedAt(value);
      break;
    case 17:
      var value = /** @type {string} */ (reader.readString());
      msg.setAssignedTo(value);
      break;
    case 18:
      var value = /** @type {string} */ (reader.readString());
      msg.setEntityRef(value);
      break;
    case 19:
      var value = /** @type {string} */ (reader.readString());
      msg.setEntityType(value);
      break;
    case 20:
      var value = /** @type {string} */ (reader.readString());
      msg.addRelatedIncidentIds(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.Incident.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.Incident.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.Incident} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.Incident.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getCategory();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getSignature();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      5,
      f
    );
  }
  f = message.getSeverity();
  if (f !== 0.0) {
    writer.writeEnum(
      6,
      f
    );
  }
  f = message.getHeadline();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getOccurrenceCount();
  if (f !== 0) {
    writer.writeInt32(
      8,
      f
    );
  }
  f = message.getFirstSeenAt();
  if (f != null) {
    writer.writeMessage(
      9,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getLastSeenAt();
  if (f != null) {
    writer.writeMessage(
      10,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getEvidenceList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      11,
      f,
      proto.workflow.EvidenceItem.serializeBinaryToWriter
    );
  }
  f = message.getDiagnosesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      12,
      f,
      proto.workflow.DiagnosisItem.serializeBinaryToWriter
    );
  }
  f = message.getProposedFixesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      13,
      f,
      proto.workflow.ProposedFix.serializeBinaryToWriter
    );
  }
  f = message.getAcknowledged();
  if (f) {
    writer.writeBool(
      14,
      f
    );
  }
  f = message.getAcknowledgedBy();
  if (f.length > 0) {
    writer.writeString(
      15,
      f
    );
  }
  f = message.getAcknowledgedAt();
  if (f != null) {
    writer.writeMessage(
      16,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getAssignedTo();
  if (f.length > 0) {
    writer.writeString(
      17,
      f
    );
  }
  f = message.getEntityRef();
  if (f.length > 0) {
    writer.writeString(
      18,
      f
    );
  }
  f = message.getEntityType();
  if (f.length > 0) {
    writer.writeString(
      19,
      f
    );
  }
  f = message.getRelatedIncidentIdsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      20,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.workflow.Incident.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string cluster_id = 2;
 * @return {string}
 */
proto.workflow.Incident.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string category = 3;
 * @return {string}
 */
proto.workflow.Incident.prototype.getCategory = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.setCategory = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string signature = 4;
 * @return {string}
 */
proto.workflow.Incident.prototype.getSignature = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.setSignature = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional IncidentStatus status = 5;
 * @return {!proto.workflow.IncidentStatus}
 */
proto.workflow.Incident.prototype.getStatus = function() {
  return /** @type {!proto.workflow.IncidentStatus} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {!proto.workflow.IncidentStatus} value
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.setStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 5, value);
};


/**
 * optional IncidentSeverity severity = 6;
 * @return {!proto.workflow.IncidentSeverity}
 */
proto.workflow.Incident.prototype.getSeverity = function() {
  return /** @type {!proto.workflow.IncidentSeverity} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {!proto.workflow.IncidentSeverity} value
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.setSeverity = function(value) {
  return jspb.Message.setProto3EnumField(this, 6, value);
};


/**
 * optional string headline = 7;
 * @return {string}
 */
proto.workflow.Incident.prototype.getHeadline = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.setHeadline = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional int32 occurrence_count = 8;
 * @return {number}
 */
proto.workflow.Incident.prototype.getOccurrenceCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 8, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.setOccurrenceCount = function(value) {
  return jspb.Message.setProto3IntField(this, 8, value);
};


/**
 * optional google.protobuf.Timestamp first_seen_at = 9;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.Incident.prototype.getFirstSeenAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 9));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.Incident} returns this
*/
proto.workflow.Incident.prototype.setFirstSeenAt = function(value) {
  return jspb.Message.setWrapperField(this, 9, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.clearFirstSeenAt = function() {
  return this.setFirstSeenAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.Incident.prototype.hasFirstSeenAt = function() {
  return jspb.Message.getField(this, 9) != null;
};


/**
 * optional google.protobuf.Timestamp last_seen_at = 10;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.Incident.prototype.getLastSeenAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 10));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.Incident} returns this
*/
proto.workflow.Incident.prototype.setLastSeenAt = function(value) {
  return jspb.Message.setWrapperField(this, 10, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.clearLastSeenAt = function() {
  return this.setLastSeenAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.Incident.prototype.hasLastSeenAt = function() {
  return jspb.Message.getField(this, 10) != null;
};


/**
 * repeated EvidenceItem evidence = 11;
 * @return {!Array<!proto.workflow.EvidenceItem>}
 */
proto.workflow.Incident.prototype.getEvidenceList = function() {
  return /** @type{!Array<!proto.workflow.EvidenceItem>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.workflow.EvidenceItem, 11));
};


/**
 * @param {!Array<!proto.workflow.EvidenceItem>} value
 * @return {!proto.workflow.Incident} returns this
*/
proto.workflow.Incident.prototype.setEvidenceList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 11, value);
};


/**
 * @param {!proto.workflow.EvidenceItem=} opt_value
 * @param {number=} opt_index
 * @return {!proto.workflow.EvidenceItem}
 */
proto.workflow.Incident.prototype.addEvidence = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 11, opt_value, proto.workflow.EvidenceItem, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.clearEvidenceList = function() {
  return this.setEvidenceList([]);
};


/**
 * repeated DiagnosisItem diagnoses = 12;
 * @return {!Array<!proto.workflow.DiagnosisItem>}
 */
proto.workflow.Incident.prototype.getDiagnosesList = function() {
  return /** @type{!Array<!proto.workflow.DiagnosisItem>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.workflow.DiagnosisItem, 12));
};


/**
 * @param {!Array<!proto.workflow.DiagnosisItem>} value
 * @return {!proto.workflow.Incident} returns this
*/
proto.workflow.Incident.prototype.setDiagnosesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 12, value);
};


/**
 * @param {!proto.workflow.DiagnosisItem=} opt_value
 * @param {number=} opt_index
 * @return {!proto.workflow.DiagnosisItem}
 */
proto.workflow.Incident.prototype.addDiagnoses = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 12, opt_value, proto.workflow.DiagnosisItem, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.clearDiagnosesList = function() {
  return this.setDiagnosesList([]);
};


/**
 * repeated ProposedFix proposed_fixes = 13;
 * @return {!Array<!proto.workflow.ProposedFix>}
 */
proto.workflow.Incident.prototype.getProposedFixesList = function() {
  return /** @type{!Array<!proto.workflow.ProposedFix>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.workflow.ProposedFix, 13));
};


/**
 * @param {!Array<!proto.workflow.ProposedFix>} value
 * @return {!proto.workflow.Incident} returns this
*/
proto.workflow.Incident.prototype.setProposedFixesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 13, value);
};


/**
 * @param {!proto.workflow.ProposedFix=} opt_value
 * @param {number=} opt_index
 * @return {!proto.workflow.ProposedFix}
 */
proto.workflow.Incident.prototype.addProposedFixes = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 13, opt_value, proto.workflow.ProposedFix, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.clearProposedFixesList = function() {
  return this.setProposedFixesList([]);
};


/**
 * optional bool acknowledged = 14;
 * @return {boolean}
 */
proto.workflow.Incident.prototype.getAcknowledged = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 14, false));
};


/**
 * @param {boolean} value
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.setAcknowledged = function(value) {
  return jspb.Message.setProto3BooleanField(this, 14, value);
};


/**
 * optional string acknowledged_by = 15;
 * @return {string}
 */
proto.workflow.Incident.prototype.getAcknowledgedBy = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 15, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.setAcknowledgedBy = function(value) {
  return jspb.Message.setProto3StringField(this, 15, value);
};


/**
 * optional google.protobuf.Timestamp acknowledged_at = 16;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.workflow.Incident.prototype.getAcknowledgedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 16));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.workflow.Incident} returns this
*/
proto.workflow.Incident.prototype.setAcknowledgedAt = function(value) {
  return jspb.Message.setWrapperField(this, 16, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.clearAcknowledgedAt = function() {
  return this.setAcknowledgedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.Incident.prototype.hasAcknowledgedAt = function() {
  return jspb.Message.getField(this, 16) != null;
};


/**
 * optional string assigned_to = 17;
 * @return {string}
 */
proto.workflow.Incident.prototype.getAssignedTo = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 17, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.setAssignedTo = function(value) {
  return jspb.Message.setProto3StringField(this, 17, value);
};


/**
 * optional string entity_ref = 18;
 * @return {string}
 */
proto.workflow.Incident.prototype.getEntityRef = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 18, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.setEntityRef = function(value) {
  return jspb.Message.setProto3StringField(this, 18, value);
};


/**
 * optional string entity_type = 19;
 * @return {string}
 */
proto.workflow.Incident.prototype.getEntityType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 19, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.setEntityType = function(value) {
  return jspb.Message.setProto3StringField(this, 19, value);
};


/**
 * repeated string related_incident_ids = 20;
 * @return {!Array<string>}
 */
proto.workflow.Incident.prototype.getRelatedIncidentIdsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 20));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.setRelatedIncidentIdsList = function(value) {
  return jspb.Message.setField(this, 20, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.addRelatedIncidentIds = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 20, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.Incident} returns this
 */
proto.workflow.Incident.prototype.clearRelatedIncidentIdsList = function() {
  return this.setRelatedIncidentIdsList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.IncidentAction.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.IncidentAction.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.IncidentAction} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.IncidentAction.toObject = function(includeInstance, msg) {
  var f, obj = {
incidentId: jspb.Message.getFieldWithDefault(msg, 1, ""),
action: jspb.Message.getFieldWithDefault(msg, 2, ""),
actor: jspb.Message.getFieldWithDefault(msg, 3, ""),
fixId: jspb.Message.getFieldWithDefault(msg, 4, ""),
comment: jspb.Message.getFieldWithDefault(msg, 5, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.IncidentAction}
 */
proto.workflow.IncidentAction.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.IncidentAction;
  return proto.workflow.IncidentAction.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.IncidentAction} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.IncidentAction}
 */
proto.workflow.IncidentAction.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setIncidentId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setAction(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setActor(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setFixId(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setComment(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.IncidentAction.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.IncidentAction.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.IncidentAction} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.IncidentAction.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getIncidentId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getAction();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getActor();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getFixId();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getComment();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
};


/**
 * optional string incident_id = 1;
 * @return {string}
 */
proto.workflow.IncidentAction.prototype.getIncidentId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.IncidentAction} returns this
 */
proto.workflow.IncidentAction.prototype.setIncidentId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string action = 2;
 * @return {string}
 */
proto.workflow.IncidentAction.prototype.getAction = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.IncidentAction} returns this
 */
proto.workflow.IncidentAction.prototype.setAction = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string actor = 3;
 * @return {string}
 */
proto.workflow.IncidentAction.prototype.getActor = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.IncidentAction} returns this
 */
proto.workflow.IncidentAction.prototype.setActor = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string fix_id = 4;
 * @return {string}
 */
proto.workflow.IncidentAction.prototype.getFixId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.IncidentAction} returns this
 */
proto.workflow.IncidentAction.prototype.setFixId = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string comment = 5;
 * @return {string}
 */
proto.workflow.IncidentAction.prototype.getComment = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.IncidentAction} returns this
 */
proto.workflow.IncidentAction.prototype.setComment = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.ListIncidentsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.ListIncidentsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.ListIncidentsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListIncidentsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
status: jspb.Message.getFieldWithDefault(msg, 2, 0),
limit: jspb.Message.getFieldWithDefault(msg, 3, 0),
pageToken: jspb.Message.getFieldWithDefault(msg, 4, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.ListIncidentsRequest}
 */
proto.workflow.ListIncidentsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.ListIncidentsRequest;
  return proto.workflow.ListIncidentsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.ListIncidentsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.ListIncidentsRequest}
 */
proto.workflow.ListIncidentsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {!proto.workflow.IncidentStatus} */ (reader.readEnum());
      msg.setStatus(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setLimit(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setPageToken(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.ListIncidentsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.ListIncidentsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.ListIncidentsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListIncidentsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
  f = message.getLimit();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
  f = message.getPageToken();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.ListIncidentsRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ListIncidentsRequest} returns this
 */
proto.workflow.ListIncidentsRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional IncidentStatus status = 2;
 * @return {!proto.workflow.IncidentStatus}
 */
proto.workflow.ListIncidentsRequest.prototype.getStatus = function() {
  return /** @type {!proto.workflow.IncidentStatus} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.workflow.IncidentStatus} value
 * @return {!proto.workflow.ListIncidentsRequest} returns this
 */
proto.workflow.ListIncidentsRequest.prototype.setStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional int32 limit = 3;
 * @return {number}
 */
proto.workflow.ListIncidentsRequest.prototype.getLimit = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.workflow.ListIncidentsRequest} returns this
 */
proto.workflow.ListIncidentsRequest.prototype.setLimit = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional string page_token = 4;
 * @return {string}
 */
proto.workflow.ListIncidentsRequest.prototype.getPageToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ListIncidentsRequest} returns this
 */
proto.workflow.ListIncidentsRequest.prototype.setPageToken = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.workflow.ListIncidentsResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.ListIncidentsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.ListIncidentsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.ListIncidentsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListIncidentsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
incidentsList: jspb.Message.toObjectList(msg.getIncidentsList(),
    proto.workflow.Incident.toObject, includeInstance),
nextPageToken: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.ListIncidentsResponse}
 */
proto.workflow.ListIncidentsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.ListIncidentsResponse;
  return proto.workflow.ListIncidentsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.ListIncidentsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.ListIncidentsResponse}
 */
proto.workflow.ListIncidentsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.workflow.Incident;
      reader.readMessage(value,proto.workflow.Incident.deserializeBinaryFromReader);
      msg.addIncidents(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setNextPageToken(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.ListIncidentsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.ListIncidentsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.ListIncidentsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListIncidentsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getIncidentsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.workflow.Incident.serializeBinaryToWriter
    );
  }
  f = message.getNextPageToken();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * repeated Incident incidents = 1;
 * @return {!Array<!proto.workflow.Incident>}
 */
proto.workflow.ListIncidentsResponse.prototype.getIncidentsList = function() {
  return /** @type{!Array<!proto.workflow.Incident>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.workflow.Incident, 1));
};


/**
 * @param {!Array<!proto.workflow.Incident>} value
 * @return {!proto.workflow.ListIncidentsResponse} returns this
*/
proto.workflow.ListIncidentsResponse.prototype.setIncidentsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.workflow.Incident=} opt_value
 * @param {number=} opt_index
 * @return {!proto.workflow.Incident}
 */
proto.workflow.ListIncidentsResponse.prototype.addIncidents = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.workflow.Incident, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.ListIncidentsResponse} returns this
 */
proto.workflow.ListIncidentsResponse.prototype.clearIncidentsList = function() {
  return this.setIncidentsList([]);
};


/**
 * optional string next_page_token = 2;
 * @return {string}
 */
proto.workflow.ListIncidentsResponse.prototype.getNextPageToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.ListIncidentsResponse} returns this
 */
proto.workflow.ListIncidentsResponse.prototype.setNextPageToken = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.GetIncidentRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.GetIncidentRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.GetIncidentRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.GetIncidentRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
incidentId: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.GetIncidentRequest}
 */
proto.workflow.GetIncidentRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.GetIncidentRequest;
  return proto.workflow.GetIncidentRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.GetIncidentRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.GetIncidentRequest}
 */
proto.workflow.GetIncidentRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIncidentId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.GetIncidentRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.GetIncidentRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.GetIncidentRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.GetIncidentRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIncidentId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.GetIncidentRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.GetIncidentRequest} returns this
 */
proto.workflow.GetIncidentRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string incident_id = 2;
 * @return {string}
 */
proto.workflow.GetIncidentRequest.prototype.getIncidentId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.GetIncidentRequest} returns this
 */
proto.workflow.GetIncidentRequest.prototype.setIncidentId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.SubmitProposedFixRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.SubmitProposedFixRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.SubmitProposedFixRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.SubmitProposedFixRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
incidentId: jspb.Message.getFieldWithDefault(msg, 2, ""),
fix: (f = msg.getFix()) && proto.workflow.ProposedFix.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.SubmitProposedFixRequest}
 */
proto.workflow.SubmitProposedFixRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.SubmitProposedFixRequest;
  return proto.workflow.SubmitProposedFixRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.SubmitProposedFixRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.SubmitProposedFixRequest}
 */
proto.workflow.SubmitProposedFixRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIncidentId(value);
      break;
    case 3:
      var value = new proto.workflow.ProposedFix;
      reader.readMessage(value,proto.workflow.ProposedFix.deserializeBinaryFromReader);
      msg.setFix(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.SubmitProposedFixRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.SubmitProposedFixRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.SubmitProposedFixRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.SubmitProposedFixRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIncidentId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getFix();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.workflow.ProposedFix.serializeBinaryToWriter
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.workflow.SubmitProposedFixRequest.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.SubmitProposedFixRequest} returns this
 */
proto.workflow.SubmitProposedFixRequest.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string incident_id = 2;
 * @return {string}
 */
proto.workflow.SubmitProposedFixRequest.prototype.getIncidentId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.SubmitProposedFixRequest} returns this
 */
proto.workflow.SubmitProposedFixRequest.prototype.setIncidentId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional ProposedFix fix = 3;
 * @return {?proto.workflow.ProposedFix}
 */
proto.workflow.SubmitProposedFixRequest.prototype.getFix = function() {
  return /** @type{?proto.workflow.ProposedFix} */ (
    jspb.Message.getWrapperField(this, proto.workflow.ProposedFix, 3));
};


/**
 * @param {?proto.workflow.ProposedFix|undefined} value
 * @return {!proto.workflow.SubmitProposedFixRequest} returns this
*/
proto.workflow.SubmitProposedFixRequest.prototype.setFix = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.workflow.SubmitProposedFixRequest} returns this
 */
proto.workflow.SubmitProposedFixRequest.prototype.clearFix = function() {
  return this.setFix(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.workflow.SubmitProposedFixRequest.prototype.hasFix = function() {
  return jspb.Message.getField(this, 3) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.ListWorkflowDefinitionsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.ListWorkflowDefinitionsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.ListWorkflowDefinitionsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListWorkflowDefinitionsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.ListWorkflowDefinitionsRequest}
 */
proto.workflow.ListWorkflowDefinitionsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.ListWorkflowDefinitionsRequest;
  return proto.workflow.ListWorkflowDefinitionsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.ListWorkflowDefinitionsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.ListWorkflowDefinitionsRequest}
 */
proto.workflow.ListWorkflowDefinitionsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.ListWorkflowDefinitionsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.ListWorkflowDefinitionsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.ListWorkflowDefinitionsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListWorkflowDefinitionsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.WorkflowDefinitionSummary.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.WorkflowDefinitionSummary.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.WorkflowDefinitionSummary} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowDefinitionSummary.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, ""),
displayName: jspb.Message.getFieldWithDefault(msg, 2, ""),
description: jspb.Message.getFieldWithDefault(msg, 3, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.WorkflowDefinitionSummary}
 */
proto.workflow.WorkflowDefinitionSummary.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.WorkflowDefinitionSummary;
  return proto.workflow.WorkflowDefinitionSummary.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.WorkflowDefinitionSummary} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.WorkflowDefinitionSummary}
 */
proto.workflow.WorkflowDefinitionSummary.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setDisplayName(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setDescription(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.WorkflowDefinitionSummary.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.WorkflowDefinitionSummary.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.WorkflowDefinitionSummary} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.WorkflowDefinitionSummary.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getDisplayName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getDescription();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.workflow.WorkflowDefinitionSummary.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowDefinitionSummary} returns this
 */
proto.workflow.WorkflowDefinitionSummary.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string display_name = 2;
 * @return {string}
 */
proto.workflow.WorkflowDefinitionSummary.prototype.getDisplayName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowDefinitionSummary} returns this
 */
proto.workflow.WorkflowDefinitionSummary.prototype.setDisplayName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string description = 3;
 * @return {string}
 */
proto.workflow.WorkflowDefinitionSummary.prototype.getDescription = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.WorkflowDefinitionSummary} returns this
 */
proto.workflow.WorkflowDefinitionSummary.prototype.setDescription = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.workflow.ListWorkflowDefinitionsResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.ListWorkflowDefinitionsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.ListWorkflowDefinitionsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.ListWorkflowDefinitionsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListWorkflowDefinitionsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
definitionsList: jspb.Message.toObjectList(msg.getDefinitionsList(),
    proto.workflow.WorkflowDefinitionSummary.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.ListWorkflowDefinitionsResponse}
 */
proto.workflow.ListWorkflowDefinitionsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.ListWorkflowDefinitionsResponse;
  return proto.workflow.ListWorkflowDefinitionsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.ListWorkflowDefinitionsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.ListWorkflowDefinitionsResponse}
 */
proto.workflow.ListWorkflowDefinitionsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.workflow.WorkflowDefinitionSummary;
      reader.readMessage(value,proto.workflow.WorkflowDefinitionSummary.deserializeBinaryFromReader);
      msg.addDefinitions(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.ListWorkflowDefinitionsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.ListWorkflowDefinitionsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.ListWorkflowDefinitionsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.ListWorkflowDefinitionsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDefinitionsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.workflow.WorkflowDefinitionSummary.serializeBinaryToWriter
    );
  }
};


/**
 * repeated WorkflowDefinitionSummary definitions = 1;
 * @return {!Array<!proto.workflow.WorkflowDefinitionSummary>}
 */
proto.workflow.ListWorkflowDefinitionsResponse.prototype.getDefinitionsList = function() {
  return /** @type{!Array<!proto.workflow.WorkflowDefinitionSummary>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.workflow.WorkflowDefinitionSummary, 1));
};


/**
 * @param {!Array<!proto.workflow.WorkflowDefinitionSummary>} value
 * @return {!proto.workflow.ListWorkflowDefinitionsResponse} returns this
*/
proto.workflow.ListWorkflowDefinitionsResponse.prototype.setDefinitionsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.workflow.WorkflowDefinitionSummary=} opt_value
 * @param {number=} opt_index
 * @return {!proto.workflow.WorkflowDefinitionSummary}
 */
proto.workflow.ListWorkflowDefinitionsResponse.prototype.addDefinitions = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.workflow.WorkflowDefinitionSummary, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.workflow.ListWorkflowDefinitionsResponse} returns this
 */
proto.workflow.ListWorkflowDefinitionsResponse.prototype.clearDefinitionsList = function() {
  return this.setDefinitionsList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.GetWorkflowDefinitionRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.GetWorkflowDefinitionRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.GetWorkflowDefinitionRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.GetWorkflowDefinitionRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.GetWorkflowDefinitionRequest}
 */
proto.workflow.GetWorkflowDefinitionRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.GetWorkflowDefinitionRequest;
  return proto.workflow.GetWorkflowDefinitionRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.GetWorkflowDefinitionRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.GetWorkflowDefinitionRequest}
 */
proto.workflow.GetWorkflowDefinitionRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.GetWorkflowDefinitionRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.GetWorkflowDefinitionRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.GetWorkflowDefinitionRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.GetWorkflowDefinitionRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.workflow.GetWorkflowDefinitionRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.GetWorkflowDefinitionRequest} returns this
 */
proto.workflow.GetWorkflowDefinitionRequest.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.workflow.GetWorkflowDefinitionResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.workflow.GetWorkflowDefinitionResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.workflow.GetWorkflowDefinitionResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.GetWorkflowDefinitionResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, ""),
yamlContent: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.workflow.GetWorkflowDefinitionResponse}
 */
proto.workflow.GetWorkflowDefinitionResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.workflow.GetWorkflowDefinitionResponse;
  return proto.workflow.GetWorkflowDefinitionResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.workflow.GetWorkflowDefinitionResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.workflow.GetWorkflowDefinitionResponse}
 */
proto.workflow.GetWorkflowDefinitionResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setYamlContent(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.workflow.GetWorkflowDefinitionResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.workflow.GetWorkflowDefinitionResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.workflow.GetWorkflowDefinitionResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.workflow.GetWorkflowDefinitionResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getYamlContent();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.workflow.GetWorkflowDefinitionResponse.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.GetWorkflowDefinitionResponse} returns this
 */
proto.workflow.GetWorkflowDefinitionResponse.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string yaml_content = 2;
 * @return {string}
 */
proto.workflow.GetWorkflowDefinitionResponse.prototype.getYamlContent = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.workflow.GetWorkflowDefinitionResponse} returns this
 */
proto.workflow.GetWorkflowDefinitionResponse.prototype.setYamlContent = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * @enum {number}
 */
proto.workflow.WorkflowActor = {
  ACTOR_UNKNOWN: 0,
  ACTOR_CLUSTER_CONTROLLER: 1,
  ACTOR_REPOSITORY: 2,
  ACTOR_NODE_AGENT: 3,
  ACTOR_INSTALLER: 4,
  ACTOR_RUNTIME: 5,
  ACTOR_OPERATOR: 6,
  ACTOR_AI_DIAGNOSER: 7,
  ACTOR_AI_EXECUTOR: 8
};

/**
 * @enum {number}
 */
proto.workflow.WorkflowPhaseKind = {
  PHASE_UNKNOWN: 0,
  PHASE_DECISION: 1,
  PHASE_FETCH: 4,
  PHASE_INSTALL: 5,
  PHASE_CONFIGURE: 6,
  PHASE_START: 7,
  PHASE_VERIFY: 8,
  PHASE_PUBLISH: 9,
  PHASE_COMPLETE: 10
};

/**
 * @enum {number}
 */
proto.workflow.RunStatus = {
  RUN_STATUS_UNKNOWN: 0,
  RUN_STATUS_PENDING: 1,
  RUN_STATUS_EXECUTING: 5,
  RUN_STATUS_BLOCKED: 6,
  RUN_STATUS_RETRYING: 7,
  RUN_STATUS_SUCCEEDED: 8,
  RUN_STATUS_FAILED: 9,
  RUN_STATUS_CANCELED: 10,
  RUN_STATUS_ROLLED_BACK: 11,
  RUN_STATUS_SUPERSEDED: 12
};

/**
 * @enum {number}
 */
proto.workflow.StepStatus = {
  STEP_STATUS_UNKNOWN: 0,
  STEP_STATUS_PENDING: 1,
  STEP_STATUS_RUNNING: 2,
  STEP_STATUS_SUCCEEDED: 3,
  STEP_STATUS_FAILED: 4,
  STEP_STATUS_SKIPPED: 5,
  STEP_STATUS_BLOCKED: 6
};

/**
 * @enum {number}
 */
proto.workflow.FailureClass = {
  FAILURE_CLASS_UNKNOWN: 0,
  FAILURE_CLASS_CONFIG: 1,
  FAILURE_CLASS_PACKAGE: 2,
  FAILURE_CLASS_DEPENDENCY: 3,
  FAILURE_CLASS_NETWORK: 4,
  FAILURE_CLASS_REPOSITORY: 5,
  FAILURE_CLASS_SYSTEMD: 6,
  FAILURE_CLASS_VALIDATION: 7
};

/**
 * @enum {number}
 */
proto.workflow.ComponentKind = {
  COMPONENT_KIND_UNKNOWN: 0,
  COMPONENT_KIND_INFRASTRUCTURE: 1,
  COMPONENT_KIND_SERVICE: 2,
  COMPONENT_KIND_CONFIG_ONLY: 3
};

/**
 * @enum {number}
 */
proto.workflow.TriggerReason = {
  TRIGGER_REASON_UNKNOWN: 0,
  TRIGGER_REASON_DESIRED_DRIFT: 1,
  TRIGGER_REASON_BOOTSTRAP: 2,
  TRIGGER_REASON_RETRY: 3,
  TRIGGER_REASON_MANUAL: 4,
  TRIGGER_REASON_DEPENDENCY_UNBLOCKED: 5,
  TRIGGER_REASON_UPGRADE: 6,
  TRIGGER_REASON_REPAIR: 7
};

/**
 * @enum {number}
 */
proto.workflow.ArtifactKind = {
  ARTIFACT_KIND_UNKNOWN: 0,
  ARTIFACT_KIND_RELEASE: 1,
  ARTIFACT_KIND_PACKAGE: 3,
  ARTIFACT_KIND_MANIFEST: 4,
  ARTIFACT_KIND_SPEC: 5,
  ARTIFACT_KIND_SCRIPT: 6,
  ARTIFACT_KIND_UNIT: 7,
  ARTIFACT_KIND_CONFIG_FILE: 8,
  ARTIFACT_KIND_ETCD_KEY: 9,
  ARTIFACT_KIND_LOG: 10
};

/**
 * @enum {number}
 */
proto.workflow.IncidentStatus = {
  INCIDENT_STATUS_UNKNOWN: 0,
  INCIDENT_STATUS_OPEN: 1,
  INCIDENT_STATUS_RESOLVING: 2,
  INCIDENT_STATUS_RESOLVED: 3,
  INCIDENT_STATUS_ACKED: 4
};

/**
 * @enum {number}
 */
proto.workflow.IncidentSeverity = {
  INCIDENT_SEVERITY_UNKNOWN: 0,
  INCIDENT_SEVERITY_INFO: 1,
  INCIDENT_SEVERITY_WARN: 2,
  INCIDENT_SEVERITY_ERROR: 3,
  INCIDENT_SEVERITY_CRITICAL: 4
};

/**
 * @enum {number}
 */
proto.workflow.Provenance = {
  PROVENANCE_UNKNOWN: 0,
  PROVENANCE_OBSERVED: 1,
  PROVENANCE_CORRELATED: 2,
  PROVENANCE_DIAGNOSED: 3,
  PROVENANCE_AI_PROPOSED: 4
};

/**
 * @enum {number}
 */
proto.workflow.FixStatus = {
  FIX_STATUS_UNKNOWN: 0,
  FIX_STATUS_PROPOSED: 1,
  FIX_STATUS_APPROVED: 2,
  FIX_STATUS_APPLIED: 3,
  FIX_STATUS_REJECTED: 4,
  FIX_STATUS_FAILED: 5
};

goog.object.extend(exports, proto.workflow);
