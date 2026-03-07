// source: backup_manager.proto
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

goog.exportSymbol('proto.backup_manager.BackupArtifact', null, global);
goog.exportSymbol('proto.backup_manager.BackupDestination', null, global);
goog.exportSymbol('proto.backup_manager.BackupDestinationType', null, global);
goog.exportSymbol('proto.backup_manager.BackupJob', null, global);
goog.exportSymbol('proto.backup_manager.BackupJobState', null, global);
goog.exportSymbol('proto.backup_manager.BackupJobType', null, global);
goog.exportSymbol('proto.backup_manager.BackupMode', null, global);
goog.exportSymbol('proto.backup_manager.BackupPlan', null, global);
goog.exportSymbol('proto.backup_manager.BackupProviderResult', null, global);
goog.exportSymbol('proto.backup_manager.BackupProviderSpec', null, global);
goog.exportSymbol('proto.backup_manager.BackupProviderType', null, global);
goog.exportSymbol('proto.backup_manager.BackupScope', null, global);
goog.exportSymbol('proto.backup_manager.BackupSeverity', null, global);
goog.exportSymbol('proto.backup_manager.CancelBackupJobRequest', null, global);
goog.exportSymbol('proto.backup_manager.CancelBackupJobResponse', null, global);
goog.exportSymbol('proto.backup_manager.ClusterInfo', null, global);
goog.exportSymbol('proto.backup_manager.CreateMinioBucketRequest', null, global);
goog.exportSymbol('proto.backup_manager.CreateMinioBucketResponse', null, global);
goog.exportSymbol('proto.backup_manager.DeleteBackupJobRequest', null, global);
goog.exportSymbol('proto.backup_manager.DeleteBackupJobResponse', null, global);
goog.exportSymbol('proto.backup_manager.DeleteBackupRequest', null, global);
goog.exportSymbol('proto.backup_manager.DeleteBackupResponse', null, global);
goog.exportSymbol('proto.backup_manager.DeleteMinioBucketRequest', null, global);
goog.exportSymbol('proto.backup_manager.DeleteMinioBucketResponse', null, global);
goog.exportSymbol('proto.backup_manager.DeleteResult', null, global);
goog.exportSymbol('proto.backup_manager.DemoteBackupRequest', null, global);
goog.exportSymbol('proto.backup_manager.DemoteBackupResponse', null, global);
goog.exportSymbol('proto.backup_manager.FinalizeBackupHookRequest', null, global);
goog.exportSymbol('proto.backup_manager.FinalizeBackupHookResponse', null, global);
goog.exportSymbol('proto.backup_manager.GetBackupJobRequest', null, global);
goog.exportSymbol('proto.backup_manager.GetBackupJobResponse', null, global);
goog.exportSymbol('proto.backup_manager.GetBackupRequest', null, global);
goog.exportSymbol('proto.backup_manager.GetBackupResponse', null, global);
goog.exportSymbol('proto.backup_manager.GetRetentionStatusRequest', null, global);
goog.exportSymbol('proto.backup_manager.GetRetentionStatusResponse', null, global);
goog.exportSymbol('proto.backup_manager.HookResult', null, global);
goog.exportSymbol('proto.backup_manager.HookSummary', null, global);
goog.exportSymbol('proto.backup_manager.ListBackupJobsRequest', null, global);
goog.exportSymbol('proto.backup_manager.ListBackupJobsResponse', null, global);
goog.exportSymbol('proto.backup_manager.ListBackupsRequest', null, global);
goog.exportSymbol('proto.backup_manager.ListBackupsResponse', null, global);
goog.exportSymbol('proto.backup_manager.ListMinioBucketsRequest', null, global);
goog.exportSymbol('proto.backup_manager.ListMinioBucketsResponse', null, global);
goog.exportSymbol('proto.backup_manager.MinioBucketInfo', null, global);
goog.exportSymbol('proto.backup_manager.PreflightCheckRequest', null, global);
goog.exportSymbol('proto.backup_manager.PreflightCheckResponse', null, global);
goog.exportSymbol('proto.backup_manager.PrepareBackupHookRequest', null, global);
goog.exportSymbol('proto.backup_manager.PrepareBackupHookResponse', null, global);
goog.exportSymbol('proto.backup_manager.PromoteBackupRequest', null, global);
goog.exportSymbol('proto.backup_manager.PromoteBackupResponse', null, global);
goog.exportSymbol('proto.backup_manager.QualityState', null, global);
goog.exportSymbol('proto.backup_manager.ReplicationResult', null, global);
goog.exportSymbol('proto.backup_manager.ReplicationValidation', null, global);
goog.exportSymbol('proto.backup_manager.RestoreBackupRequest', null, global);
goog.exportSymbol('proto.backup_manager.RestoreBackupResponse', null, global);
goog.exportSymbol('proto.backup_manager.RestorePlanRequest', null, global);
goog.exportSymbol('proto.backup_manager.RestorePlanResponse', null, global);
goog.exportSymbol('proto.backup_manager.RestoreStep', null, global);
goog.exportSymbol('proto.backup_manager.RestoreTestCheck', null, global);
goog.exportSymbol('proto.backup_manager.RestoreTestLevel', null, global);
goog.exportSymbol('proto.backup_manager.RestoreTestReport', null, global);
goog.exportSymbol('proto.backup_manager.RetentionPolicy', null, global);
goog.exportSymbol('proto.backup_manager.RunBackupRequest', null, global);
goog.exportSymbol('proto.backup_manager.RunBackupResponse', null, global);
goog.exportSymbol('proto.backup_manager.RunRestoreTestRequest', null, global);
goog.exportSymbol('proto.backup_manager.RunRestoreTestResponse', null, global);
goog.exportSymbol('proto.backup_manager.RunRetentionRequest', null, global);
goog.exportSymbol('proto.backup_manager.RunRetentionResponse', null, global);
goog.exportSymbol('proto.backup_manager.SkippedProvider', null, global);
goog.exportSymbol('proto.backup_manager.StopRequest', null, global);
goog.exportSymbol('proto.backup_manager.StopResponse', null, global);
goog.exportSymbol('proto.backup_manager.ToolCheck', null, global);
goog.exportSymbol('proto.backup_manager.ValidateBackupRequest', null, global);
goog.exportSymbol('proto.backup_manager.ValidateBackupResponse', null, global);
goog.exportSymbol('proto.backup_manager.ValidationIssue', null, global);
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
proto.backup_manager.BackupScope = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.backup_manager.BackupScope.repeatedFields_, null);
};
goog.inherits(proto.backup_manager.BackupScope, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.BackupScope.displayName = 'proto.backup_manager.BackupScope';
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
proto.backup_manager.ClusterInfo = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.ClusterInfo, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.ClusterInfo.displayName = 'proto.backup_manager.ClusterInfo';
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
proto.backup_manager.HookResult = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.HookResult, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.HookResult.displayName = 'proto.backup_manager.HookResult';
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
proto.backup_manager.HookSummary = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.backup_manager.HookSummary.repeatedFields_, null);
};
goog.inherits(proto.backup_manager.HookSummary, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.HookSummary.displayName = 'proto.backup_manager.HookSummary';
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
proto.backup_manager.BackupDestination = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.BackupDestination, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.BackupDestination.displayName = 'proto.backup_manager.BackupDestination';
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
proto.backup_manager.BackupProviderSpec = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.BackupProviderSpec, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.BackupProviderSpec.displayName = 'proto.backup_manager.BackupProviderSpec';
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
proto.backup_manager.BackupPlan = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.backup_manager.BackupPlan.repeatedFields_, null);
};
goog.inherits(proto.backup_manager.BackupPlan, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.BackupPlan.displayName = 'proto.backup_manager.BackupPlan';
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
proto.backup_manager.ReplicationResult = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.ReplicationResult, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.ReplicationResult.displayName = 'proto.backup_manager.ReplicationResult';
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
proto.backup_manager.ReplicationValidation = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.backup_manager.ReplicationValidation.repeatedFields_, null);
};
goog.inherits(proto.backup_manager.ReplicationValidation, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.ReplicationValidation.displayName = 'proto.backup_manager.ReplicationValidation';
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
proto.backup_manager.BackupProviderResult = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.backup_manager.BackupProviderResult.repeatedFields_, null);
};
goog.inherits(proto.backup_manager.BackupProviderResult, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.BackupProviderResult.displayName = 'proto.backup_manager.BackupProviderResult';
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
proto.backup_manager.BackupJob = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.backup_manager.BackupJob.repeatedFields_, null);
};
goog.inherits(proto.backup_manager.BackupJob, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.BackupJob.displayName = 'proto.backup_manager.BackupJob';
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
proto.backup_manager.BackupArtifact = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.backup_manager.BackupArtifact.repeatedFields_, null);
};
goog.inherits(proto.backup_manager.BackupArtifact, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.BackupArtifact.displayName = 'proto.backup_manager.BackupArtifact';
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
proto.backup_manager.RetentionPolicy = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.RetentionPolicy, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.RetentionPolicy.displayName = 'proto.backup_manager.RetentionPolicy';
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
proto.backup_manager.RunBackupRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.RunBackupRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.RunBackupRequest.displayName = 'proto.backup_manager.RunBackupRequest';
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
proto.backup_manager.RunBackupResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.RunBackupResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.RunBackupResponse.displayName = 'proto.backup_manager.RunBackupResponse';
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
proto.backup_manager.GetBackupJobRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.GetBackupJobRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.GetBackupJobRequest.displayName = 'proto.backup_manager.GetBackupJobRequest';
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
proto.backup_manager.GetBackupJobResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.GetBackupJobResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.GetBackupJobResponse.displayName = 'proto.backup_manager.GetBackupJobResponse';
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
proto.backup_manager.ListBackupJobsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.ListBackupJobsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.ListBackupJobsRequest.displayName = 'proto.backup_manager.ListBackupJobsRequest';
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
proto.backup_manager.ListBackupJobsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.backup_manager.ListBackupJobsResponse.repeatedFields_, null);
};
goog.inherits(proto.backup_manager.ListBackupJobsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.ListBackupJobsResponse.displayName = 'proto.backup_manager.ListBackupJobsResponse';
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
proto.backup_manager.ListBackupsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.ListBackupsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.ListBackupsRequest.displayName = 'proto.backup_manager.ListBackupsRequest';
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
proto.backup_manager.ListBackupsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.backup_manager.ListBackupsResponse.repeatedFields_, null);
};
goog.inherits(proto.backup_manager.ListBackupsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.ListBackupsResponse.displayName = 'proto.backup_manager.ListBackupsResponse';
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
proto.backup_manager.GetBackupRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.GetBackupRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.GetBackupRequest.displayName = 'proto.backup_manager.GetBackupRequest';
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
proto.backup_manager.GetBackupResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.GetBackupResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.GetBackupResponse.displayName = 'proto.backup_manager.GetBackupResponse';
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
proto.backup_manager.DeleteBackupRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.DeleteBackupRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.DeleteBackupRequest.displayName = 'proto.backup_manager.DeleteBackupRequest';
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
proto.backup_manager.DeleteBackupResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.backup_manager.DeleteBackupResponse.repeatedFields_, null);
};
goog.inherits(proto.backup_manager.DeleteBackupResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.DeleteBackupResponse.displayName = 'proto.backup_manager.DeleteBackupResponse';
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
proto.backup_manager.DeleteResult = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.DeleteResult, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.DeleteResult.displayName = 'proto.backup_manager.DeleteResult';
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
proto.backup_manager.ValidateBackupRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.ValidateBackupRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.ValidateBackupRequest.displayName = 'proto.backup_manager.ValidateBackupRequest';
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
proto.backup_manager.ValidateBackupResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.backup_manager.ValidateBackupResponse.repeatedFields_, null);
};
goog.inherits(proto.backup_manager.ValidateBackupResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.ValidateBackupResponse.displayName = 'proto.backup_manager.ValidateBackupResponse';
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
proto.backup_manager.ValidationIssue = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.ValidationIssue, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.ValidationIssue.displayName = 'proto.backup_manager.ValidationIssue';
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
proto.backup_manager.RestorePlanRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.RestorePlanRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.RestorePlanRequest.displayName = 'proto.backup_manager.RestorePlanRequest';
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
proto.backup_manager.RestorePlanResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.backup_manager.RestorePlanResponse.repeatedFields_, null);
};
goog.inherits(proto.backup_manager.RestorePlanResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.RestorePlanResponse.displayName = 'proto.backup_manager.RestorePlanResponse';
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
proto.backup_manager.RestoreStep = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.RestoreStep, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.RestoreStep.displayName = 'proto.backup_manager.RestoreStep';
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
proto.backup_manager.RestoreBackupRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.RestoreBackupRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.RestoreBackupRequest.displayName = 'proto.backup_manager.RestoreBackupRequest';
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
proto.backup_manager.RestoreBackupResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.backup_manager.RestoreBackupResponse.repeatedFields_, null);
};
goog.inherits(proto.backup_manager.RestoreBackupResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.RestoreBackupResponse.displayName = 'proto.backup_manager.RestoreBackupResponse';
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
proto.backup_manager.CancelBackupJobRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.CancelBackupJobRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.CancelBackupJobRequest.displayName = 'proto.backup_manager.CancelBackupJobRequest';
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
proto.backup_manager.CancelBackupJobResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.CancelBackupJobResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.CancelBackupJobResponse.displayName = 'proto.backup_manager.CancelBackupJobResponse';
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
proto.backup_manager.DeleteBackupJobRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.DeleteBackupJobRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.DeleteBackupJobRequest.displayName = 'proto.backup_manager.DeleteBackupJobRequest';
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
proto.backup_manager.DeleteBackupJobResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.DeleteBackupJobResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.DeleteBackupJobResponse.displayName = 'proto.backup_manager.DeleteBackupJobResponse';
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
proto.backup_manager.RunRetentionRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.RunRetentionRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.RunRetentionRequest.displayName = 'proto.backup_manager.RunRetentionRequest';
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
proto.backup_manager.RunRetentionResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.backup_manager.RunRetentionResponse.repeatedFields_, null);
};
goog.inherits(proto.backup_manager.RunRetentionResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.RunRetentionResponse.displayName = 'proto.backup_manager.RunRetentionResponse';
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
proto.backup_manager.GetRetentionStatusRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.GetRetentionStatusRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.GetRetentionStatusRequest.displayName = 'proto.backup_manager.GetRetentionStatusRequest';
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
proto.backup_manager.GetRetentionStatusResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.GetRetentionStatusResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.GetRetentionStatusResponse.displayName = 'proto.backup_manager.GetRetentionStatusResponse';
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
proto.backup_manager.PreflightCheckRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.PreflightCheckRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.PreflightCheckRequest.displayName = 'proto.backup_manager.PreflightCheckRequest';
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
proto.backup_manager.PreflightCheckResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.backup_manager.PreflightCheckResponse.repeatedFields_, null);
};
goog.inherits(proto.backup_manager.PreflightCheckResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.PreflightCheckResponse.displayName = 'proto.backup_manager.PreflightCheckResponse';
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
proto.backup_manager.ToolCheck = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.ToolCheck, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.ToolCheck.displayName = 'proto.backup_manager.ToolCheck';
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
proto.backup_manager.SkippedProvider = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.SkippedProvider, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.SkippedProvider.displayName = 'proto.backup_manager.SkippedProvider';
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
proto.backup_manager.RunRestoreTestRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.RunRestoreTestRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.RunRestoreTestRequest.displayName = 'proto.backup_manager.RunRestoreTestRequest';
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
proto.backup_manager.RunRestoreTestResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.RunRestoreTestResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.RunRestoreTestResponse.displayName = 'proto.backup_manager.RunRestoreTestResponse';
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
proto.backup_manager.RestoreTestReport = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.backup_manager.RestoreTestReport.repeatedFields_, null);
};
goog.inherits(proto.backup_manager.RestoreTestReport, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.RestoreTestReport.displayName = 'proto.backup_manager.RestoreTestReport';
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
proto.backup_manager.RestoreTestCheck = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.RestoreTestCheck, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.RestoreTestCheck.displayName = 'proto.backup_manager.RestoreTestCheck';
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
proto.backup_manager.PromoteBackupRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.PromoteBackupRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.PromoteBackupRequest.displayName = 'proto.backup_manager.PromoteBackupRequest';
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
proto.backup_manager.PromoteBackupResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.PromoteBackupResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.PromoteBackupResponse.displayName = 'proto.backup_manager.PromoteBackupResponse';
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
proto.backup_manager.DemoteBackupRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.DemoteBackupRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.DemoteBackupRequest.displayName = 'proto.backup_manager.DemoteBackupRequest';
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
proto.backup_manager.DemoteBackupResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.DemoteBackupResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.DemoteBackupResponse.displayName = 'proto.backup_manager.DemoteBackupResponse';
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
proto.backup_manager.PrepareBackupHookRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.PrepareBackupHookRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.PrepareBackupHookRequest.displayName = 'proto.backup_manager.PrepareBackupHookRequest';
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
proto.backup_manager.PrepareBackupHookResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.PrepareBackupHookResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.PrepareBackupHookResponse.displayName = 'proto.backup_manager.PrepareBackupHookResponse';
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
proto.backup_manager.FinalizeBackupHookRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.FinalizeBackupHookRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.FinalizeBackupHookRequest.displayName = 'proto.backup_manager.FinalizeBackupHookRequest';
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
proto.backup_manager.FinalizeBackupHookResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.FinalizeBackupHookResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.FinalizeBackupHookResponse.displayName = 'proto.backup_manager.FinalizeBackupHookResponse';
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
proto.backup_manager.MinioBucketInfo = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.MinioBucketInfo, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.MinioBucketInfo.displayName = 'proto.backup_manager.MinioBucketInfo';
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
proto.backup_manager.ListMinioBucketsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.ListMinioBucketsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.ListMinioBucketsRequest.displayName = 'proto.backup_manager.ListMinioBucketsRequest';
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
proto.backup_manager.ListMinioBucketsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.backup_manager.ListMinioBucketsResponse.repeatedFields_, null);
};
goog.inherits(proto.backup_manager.ListMinioBucketsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.ListMinioBucketsResponse.displayName = 'proto.backup_manager.ListMinioBucketsResponse';
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
proto.backup_manager.CreateMinioBucketRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.CreateMinioBucketRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.CreateMinioBucketRequest.displayName = 'proto.backup_manager.CreateMinioBucketRequest';
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
proto.backup_manager.CreateMinioBucketResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.CreateMinioBucketResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.CreateMinioBucketResponse.displayName = 'proto.backup_manager.CreateMinioBucketResponse';
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
proto.backup_manager.DeleteMinioBucketRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.DeleteMinioBucketRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.DeleteMinioBucketRequest.displayName = 'proto.backup_manager.DeleteMinioBucketRequest';
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
proto.backup_manager.DeleteMinioBucketResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.DeleteMinioBucketResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.DeleteMinioBucketResponse.displayName = 'proto.backup_manager.DeleteMinioBucketResponse';
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
proto.backup_manager.StopRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.StopRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.StopRequest.displayName = 'proto.backup_manager.StopRequest';
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
proto.backup_manager.StopResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.backup_manager.StopResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.backup_manager.StopResponse.displayName = 'proto.backup_manager.StopResponse';
}

/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.backup_manager.BackupScope.repeatedFields_ = [1,2];



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
proto.backup_manager.BackupScope.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.BackupScope.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.BackupScope} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.BackupScope.toObject = function(includeInstance, msg) {
  var f, obj = {
providersList: (f = jspb.Message.getRepeatedField(msg, 1)) == null ? undefined : f,
servicesList: (f = jspb.Message.getRepeatedField(msg, 2)) == null ? undefined : f
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
 * @return {!proto.backup_manager.BackupScope}
 */
proto.backup_manager.BackupScope.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.BackupScope;
  return proto.backup_manager.BackupScope.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.BackupScope} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.BackupScope}
 */
proto.backup_manager.BackupScope.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.addProviders(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.addServices(value);
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
proto.backup_manager.BackupScope.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.BackupScope.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.BackupScope} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.BackupScope.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getProvidersList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      1,
      f
    );
  }
  f = message.getServicesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      2,
      f
    );
  }
};


/**
 * repeated string providers = 1;
 * @return {!Array<string>}
 */
proto.backup_manager.BackupScope.prototype.getProvidersList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 1));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.backup_manager.BackupScope} returns this
 */
proto.backup_manager.BackupScope.prototype.setProvidersList = function(value) {
  return jspb.Message.setField(this, 1, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.BackupScope} returns this
 */
proto.backup_manager.BackupScope.prototype.addProviders = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 1, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.BackupScope} returns this
 */
proto.backup_manager.BackupScope.prototype.clearProvidersList = function() {
  return this.setProvidersList([]);
};


/**
 * repeated string services = 2;
 * @return {!Array<string>}
 */
proto.backup_manager.BackupScope.prototype.getServicesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.backup_manager.BackupScope} returns this
 */
proto.backup_manager.BackupScope.prototype.setServicesList = function(value) {
  return jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.BackupScope} returns this
 */
proto.backup_manager.BackupScope.prototype.addServices = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.BackupScope} returns this
 */
proto.backup_manager.BackupScope.prototype.clearServicesList = function() {
  return this.setServicesList([]);
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
proto.backup_manager.ClusterInfo.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.ClusterInfo.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.ClusterInfo} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ClusterInfo.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
domain: jspb.Message.getFieldWithDefault(msg, 2, ""),
nodeId: jspb.Message.getFieldWithDefault(msg, 3, ""),
topologyHash: jspb.Message.getFieldWithDefault(msg, 4, "")
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
 * @return {!proto.backup_manager.ClusterInfo}
 */
proto.backup_manager.ClusterInfo.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.ClusterInfo;
  return proto.backup_manager.ClusterInfo.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.ClusterInfo} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.ClusterInfo}
 */
proto.backup_manager.ClusterInfo.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setDomain(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeId(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setTopologyHash(value);
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
proto.backup_manager.ClusterInfo.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.ClusterInfo.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.ClusterInfo} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ClusterInfo.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getDomain();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getTopologyHash();
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
proto.backup_manager.ClusterInfo.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.ClusterInfo} returns this
 */
proto.backup_manager.ClusterInfo.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string domain = 2;
 * @return {string}
 */
proto.backup_manager.ClusterInfo.prototype.getDomain = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.ClusterInfo} returns this
 */
proto.backup_manager.ClusterInfo.prototype.setDomain = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string node_id = 3;
 * @return {string}
 */
proto.backup_manager.ClusterInfo.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.ClusterInfo} returns this
 */
proto.backup_manager.ClusterInfo.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string topology_hash = 4;
 * @return {string}
 */
proto.backup_manager.ClusterInfo.prototype.getTopologyHash = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.ClusterInfo} returns this
 */
proto.backup_manager.ClusterInfo.prototype.setTopologyHash = function(value) {
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
proto.backup_manager.HookResult.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.HookResult.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.HookResult} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.HookResult.toObject = function(includeInstance, msg) {
  var f, obj = {
serviceName: jspb.Message.getFieldWithDefault(msg, 1, ""),
ok: jspb.Message.getBooleanFieldWithDefault(msg, 2, false),
message: jspb.Message.getFieldWithDefault(msg, 3, ""),
detailsMap: (f = msg.getDetailsMap()) ? f.toObject(includeInstance, undefined) : [],
durationMs: jspb.Message.getFieldWithDefault(msg, 5, 0)
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
 * @return {!proto.backup_manager.HookResult}
 */
proto.backup_manager.HookResult.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.HookResult;
  return proto.backup_manager.HookResult.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.HookResult} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.HookResult}
 */
proto.backup_manager.HookResult.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setServiceName(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setOk(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
      break;
    case 4:
      var value = msg.getDetailsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
      break;
    case 5:
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
proto.backup_manager.HookResult.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.HookResult.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.HookResult} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.HookResult.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getServiceName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOk();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
  f = message.getMessage();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getDetailsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(4, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
  f = message.getDurationMs();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
};


/**
 * optional string service_name = 1;
 * @return {string}
 */
proto.backup_manager.HookResult.prototype.getServiceName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.HookResult} returns this
 */
proto.backup_manager.HookResult.prototype.setServiceName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bool ok = 2;
 * @return {boolean}
 */
proto.backup_manager.HookResult.prototype.getOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.HookResult} returns this
 */
proto.backup_manager.HookResult.prototype.setOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * optional string message = 3;
 * @return {string}
 */
proto.backup_manager.HookResult.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.HookResult} returns this
 */
proto.backup_manager.HookResult.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * map<string, string> details = 4;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.backup_manager.HookResult.prototype.getDetailsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 4, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.backup_manager.HookResult} returns this
 */
proto.backup_manager.HookResult.prototype.clearDetailsMap = function() {
  this.getDetailsMap().clear();
  return this;
};


/**
 * optional int64 duration_ms = 5;
 * @return {number}
 */
proto.backup_manager.HookResult.prototype.getDurationMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.HookResult} returns this
 */
proto.backup_manager.HookResult.prototype.setDurationMs = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.backup_manager.HookSummary.repeatedFields_ = [1,2];



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
proto.backup_manager.HookSummary.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.HookSummary.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.HookSummary} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.HookSummary.toObject = function(includeInstance, msg) {
  var f, obj = {
prepareList: jspb.Message.toObjectList(msg.getPrepareList(),
    proto.backup_manager.HookResult.toObject, includeInstance),
finalizeList: jspb.Message.toObjectList(msg.getFinalizeList(),
    proto.backup_manager.HookResult.toObject, includeInstance)
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
 * @return {!proto.backup_manager.HookSummary}
 */
proto.backup_manager.HookSummary.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.HookSummary;
  return proto.backup_manager.HookSummary.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.HookSummary} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.HookSummary}
 */
proto.backup_manager.HookSummary.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.backup_manager.HookResult;
      reader.readMessage(value,proto.backup_manager.HookResult.deserializeBinaryFromReader);
      msg.addPrepare(value);
      break;
    case 2:
      var value = new proto.backup_manager.HookResult;
      reader.readMessage(value,proto.backup_manager.HookResult.deserializeBinaryFromReader);
      msg.addFinalize(value);
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
proto.backup_manager.HookSummary.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.HookSummary.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.HookSummary} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.HookSummary.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPrepareList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.backup_manager.HookResult.serializeBinaryToWriter
    );
  }
  f = message.getFinalizeList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      2,
      f,
      proto.backup_manager.HookResult.serializeBinaryToWriter
    );
  }
};


/**
 * repeated HookResult prepare = 1;
 * @return {!Array<!proto.backup_manager.HookResult>}
 */
proto.backup_manager.HookSummary.prototype.getPrepareList = function() {
  return /** @type{!Array<!proto.backup_manager.HookResult>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.HookResult, 1));
};


/**
 * @param {!Array<!proto.backup_manager.HookResult>} value
 * @return {!proto.backup_manager.HookSummary} returns this
*/
proto.backup_manager.HookSummary.prototype.setPrepareList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.backup_manager.HookResult=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.HookResult}
 */
proto.backup_manager.HookSummary.prototype.addPrepare = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.backup_manager.HookResult, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.HookSummary} returns this
 */
proto.backup_manager.HookSummary.prototype.clearPrepareList = function() {
  return this.setPrepareList([]);
};


/**
 * repeated HookResult finalize = 2;
 * @return {!Array<!proto.backup_manager.HookResult>}
 */
proto.backup_manager.HookSummary.prototype.getFinalizeList = function() {
  return /** @type{!Array<!proto.backup_manager.HookResult>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.HookResult, 2));
};


/**
 * @param {!Array<!proto.backup_manager.HookResult>} value
 * @return {!proto.backup_manager.HookSummary} returns this
*/
proto.backup_manager.HookSummary.prototype.setFinalizeList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 2, value);
};


/**
 * @param {!proto.backup_manager.HookResult=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.HookResult}
 */
proto.backup_manager.HookSummary.prototype.addFinalize = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 2, opt_value, proto.backup_manager.HookResult, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.HookSummary} returns this
 */
proto.backup_manager.HookSummary.prototype.clearFinalizeList = function() {
  return this.setFinalizeList([]);
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
proto.backup_manager.BackupDestination.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.BackupDestination.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.BackupDestination} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.BackupDestination.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, ""),
type: jspb.Message.getFieldWithDefault(msg, 2, 0),
path: jspb.Message.getFieldWithDefault(msg, 3, ""),
optionsMap: (f = msg.getOptionsMap()) ? f.toObject(includeInstance, undefined) : [],
primary: jspb.Message.getBooleanFieldWithDefault(msg, 5, false)
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
 * @return {!proto.backup_manager.BackupDestination}
 */
proto.backup_manager.BackupDestination.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.BackupDestination;
  return proto.backup_manager.BackupDestination.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.BackupDestination} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.BackupDestination}
 */
proto.backup_manager.BackupDestination.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {!proto.backup_manager.BackupDestinationType} */ (reader.readEnum());
      msg.setType(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setPath(value);
      break;
    case 4:
      var value = msg.getOptionsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
      break;
    case 5:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setPrimary(value);
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
proto.backup_manager.BackupDestination.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.BackupDestination.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.BackupDestination} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.BackupDestination.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getType();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
  f = message.getPath();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getOptionsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(4, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
  f = message.getPrimary();
  if (f) {
    writer.writeBool(
      5,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.backup_manager.BackupDestination.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.BackupDestination} returns this
 */
proto.backup_manager.BackupDestination.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional BackupDestinationType type = 2;
 * @return {!proto.backup_manager.BackupDestinationType}
 */
proto.backup_manager.BackupDestination.prototype.getType = function() {
  return /** @type {!proto.backup_manager.BackupDestinationType} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.backup_manager.BackupDestinationType} value
 * @return {!proto.backup_manager.BackupDestination} returns this
 */
proto.backup_manager.BackupDestination.prototype.setType = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional string path = 3;
 * @return {string}
 */
proto.backup_manager.BackupDestination.prototype.getPath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.BackupDestination} returns this
 */
proto.backup_manager.BackupDestination.prototype.setPath = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * map<string, string> options = 4;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.backup_manager.BackupDestination.prototype.getOptionsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 4, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.backup_manager.BackupDestination} returns this
 */
proto.backup_manager.BackupDestination.prototype.clearOptionsMap = function() {
  this.getOptionsMap().clear();
  return this;
};


/**
 * optional bool primary = 5;
 * @return {boolean}
 */
proto.backup_manager.BackupDestination.prototype.getPrimary = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 5, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.BackupDestination} returns this
 */
proto.backup_manager.BackupDestination.prototype.setPrimary = function(value) {
  return jspb.Message.setProto3BooleanField(this, 5, value);
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
proto.backup_manager.BackupProviderSpec.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.BackupProviderSpec.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.BackupProviderSpec} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.BackupProviderSpec.toObject = function(includeInstance, msg) {
  var f, obj = {
type: jspb.Message.getFieldWithDefault(msg, 1, 0),
enabled: jspb.Message.getBooleanFieldWithDefault(msg, 2, false),
optionsMap: (f = msg.getOptionsMap()) ? f.toObject(includeInstance, undefined) : [],
timeoutSeconds: jspb.Message.getFieldWithDefault(msg, 4, 0)
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
 * @return {!proto.backup_manager.BackupProviderSpec}
 */
proto.backup_manager.BackupProviderSpec.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.BackupProviderSpec;
  return proto.backup_manager.BackupProviderSpec.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.BackupProviderSpec} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.BackupProviderSpec}
 */
proto.backup_manager.BackupProviderSpec.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.backup_manager.BackupProviderType} */ (reader.readEnum());
      msg.setType(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setEnabled(value);
      break;
    case 3:
      var value = msg.getOptionsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
      break;
    case 4:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setTimeoutSeconds(value);
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
proto.backup_manager.BackupProviderSpec.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.BackupProviderSpec.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.BackupProviderSpec} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.BackupProviderSpec.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getType();
  if (f !== 0.0) {
    writer.writeEnum(
      1,
      f
    );
  }
  f = message.getEnabled();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
  f = message.getOptionsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(3, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
  f = message.getTimeoutSeconds();
  if (f !== 0) {
    writer.writeUint32(
      4,
      f
    );
  }
};


/**
 * optional BackupProviderType type = 1;
 * @return {!proto.backup_manager.BackupProviderType}
 */
proto.backup_manager.BackupProviderSpec.prototype.getType = function() {
  return /** @type {!proto.backup_manager.BackupProviderType} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {!proto.backup_manager.BackupProviderType} value
 * @return {!proto.backup_manager.BackupProviderSpec} returns this
 */
proto.backup_manager.BackupProviderSpec.prototype.setType = function(value) {
  return jspb.Message.setProto3EnumField(this, 1, value);
};


/**
 * optional bool enabled = 2;
 * @return {boolean}
 */
proto.backup_manager.BackupProviderSpec.prototype.getEnabled = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.BackupProviderSpec} returns this
 */
proto.backup_manager.BackupProviderSpec.prototype.setEnabled = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * map<string, string> options = 3;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.backup_manager.BackupProviderSpec.prototype.getOptionsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 3, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.backup_manager.BackupProviderSpec} returns this
 */
proto.backup_manager.BackupProviderSpec.prototype.clearOptionsMap = function() {
  this.getOptionsMap().clear();
  return this;
};


/**
 * optional uint32 timeout_seconds = 4;
 * @return {number}
 */
proto.backup_manager.BackupProviderSpec.prototype.getTimeoutSeconds = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.BackupProviderSpec} returns this
 */
proto.backup_manager.BackupProviderSpec.prototype.setTimeoutSeconds = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.backup_manager.BackupPlan.repeatedFields_ = [2,4];



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
proto.backup_manager.BackupPlan.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.BackupPlan.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.BackupPlan} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.BackupPlan.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, ""),
providersList: jspb.Message.toObjectList(msg.getProvidersList(),
    proto.backup_manager.BackupProviderSpec.toObject, includeInstance),
destination: jspb.Message.getFieldWithDefault(msg, 3, ""),
destinationsList: jspb.Message.toObjectList(msg.getDestinationsList(),
    proto.backup_manager.BackupDestination.toObject, includeInstance)
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
 * @return {!proto.backup_manager.BackupPlan}
 */
proto.backup_manager.BackupPlan.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.BackupPlan;
  return proto.backup_manager.BackupPlan.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.BackupPlan} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.BackupPlan}
 */
proto.backup_manager.BackupPlan.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = new proto.backup_manager.BackupProviderSpec;
      reader.readMessage(value,proto.backup_manager.BackupProviderSpec.deserializeBinaryFromReader);
      msg.addProviders(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setDestination(value);
      break;
    case 4:
      var value = new proto.backup_manager.BackupDestination;
      reader.readMessage(value,proto.backup_manager.BackupDestination.deserializeBinaryFromReader);
      msg.addDestinations(value);
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
proto.backup_manager.BackupPlan.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.BackupPlan.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.BackupPlan} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.BackupPlan.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getProvidersList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      2,
      f,
      proto.backup_manager.BackupProviderSpec.serializeBinaryToWriter
    );
  }
  f = message.getDestination();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getDestinationsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      4,
      f,
      proto.backup_manager.BackupDestination.serializeBinaryToWriter
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.backup_manager.BackupPlan.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.BackupPlan} returns this
 */
proto.backup_manager.BackupPlan.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * repeated BackupProviderSpec providers = 2;
 * @return {!Array<!proto.backup_manager.BackupProviderSpec>}
 */
proto.backup_manager.BackupPlan.prototype.getProvidersList = function() {
  return /** @type{!Array<!proto.backup_manager.BackupProviderSpec>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.BackupProviderSpec, 2));
};


/**
 * @param {!Array<!proto.backup_manager.BackupProviderSpec>} value
 * @return {!proto.backup_manager.BackupPlan} returns this
*/
proto.backup_manager.BackupPlan.prototype.setProvidersList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 2, value);
};


/**
 * @param {!proto.backup_manager.BackupProviderSpec=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.BackupProviderSpec}
 */
proto.backup_manager.BackupPlan.prototype.addProviders = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 2, opt_value, proto.backup_manager.BackupProviderSpec, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.BackupPlan} returns this
 */
proto.backup_manager.BackupPlan.prototype.clearProvidersList = function() {
  return this.setProvidersList([]);
};


/**
 * optional string destination = 3;
 * @return {string}
 */
proto.backup_manager.BackupPlan.prototype.getDestination = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.BackupPlan} returns this
 */
proto.backup_manager.BackupPlan.prototype.setDestination = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * repeated BackupDestination destinations = 4;
 * @return {!Array<!proto.backup_manager.BackupDestination>}
 */
proto.backup_manager.BackupPlan.prototype.getDestinationsList = function() {
  return /** @type{!Array<!proto.backup_manager.BackupDestination>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.BackupDestination, 4));
};


/**
 * @param {!Array<!proto.backup_manager.BackupDestination>} value
 * @return {!proto.backup_manager.BackupPlan} returns this
*/
proto.backup_manager.BackupPlan.prototype.setDestinationsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 4, value);
};


/**
 * @param {!proto.backup_manager.BackupDestination=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.BackupDestination}
 */
proto.backup_manager.BackupPlan.prototype.addDestinations = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 4, opt_value, proto.backup_manager.BackupDestination, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.BackupPlan} returns this
 */
proto.backup_manager.BackupPlan.prototype.clearDestinationsList = function() {
  return this.setDestinationsList([]);
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
proto.backup_manager.ReplicationResult.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.ReplicationResult.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.ReplicationResult} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ReplicationResult.toObject = function(includeInstance, msg) {
  var f, obj = {
destinationName: jspb.Message.getFieldWithDefault(msg, 1, ""),
destinationType: jspb.Message.getFieldWithDefault(msg, 2, 0),
destinationPath: jspb.Message.getFieldWithDefault(msg, 3, ""),
state: jspb.Message.getFieldWithDefault(msg, 4, 0),
errorMessage: jspb.Message.getFieldWithDefault(msg, 5, ""),
bytesWritten: jspb.Message.getFieldWithDefault(msg, 6, 0),
startedUnixMs: jspb.Message.getFieldWithDefault(msg, 7, 0),
finishedUnixMs: jspb.Message.getFieldWithDefault(msg, 8, 0)
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
 * @return {!proto.backup_manager.ReplicationResult}
 */
proto.backup_manager.ReplicationResult.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.ReplicationResult;
  return proto.backup_manager.ReplicationResult.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.ReplicationResult} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.ReplicationResult}
 */
proto.backup_manager.ReplicationResult.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setDestinationName(value);
      break;
    case 2:
      var value = /** @type {!proto.backup_manager.BackupDestinationType} */ (reader.readEnum());
      msg.setDestinationType(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setDestinationPath(value);
      break;
    case 4:
      var value = /** @type {!proto.backup_manager.BackupJobState} */ (reader.readEnum());
      msg.setState(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setErrorMessage(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readUint64());
      msg.setBytesWritten(value);
      break;
    case 7:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setStartedUnixMs(value);
      break;
    case 8:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setFinishedUnixMs(value);
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
proto.backup_manager.ReplicationResult.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.ReplicationResult.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.ReplicationResult} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ReplicationResult.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDestinationName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getDestinationType();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
  f = message.getDestinationPath();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getState();
  if (f !== 0.0) {
    writer.writeEnum(
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
  f = message.getBytesWritten();
  if (f !== 0) {
    writer.writeUint64(
      6,
      f
    );
  }
  f = message.getStartedUnixMs();
  if (f !== 0) {
    writer.writeInt64(
      7,
      f
    );
  }
  f = message.getFinishedUnixMs();
  if (f !== 0) {
    writer.writeInt64(
      8,
      f
    );
  }
};


/**
 * optional string destination_name = 1;
 * @return {string}
 */
proto.backup_manager.ReplicationResult.prototype.getDestinationName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.ReplicationResult} returns this
 */
proto.backup_manager.ReplicationResult.prototype.setDestinationName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional BackupDestinationType destination_type = 2;
 * @return {!proto.backup_manager.BackupDestinationType}
 */
proto.backup_manager.ReplicationResult.prototype.getDestinationType = function() {
  return /** @type {!proto.backup_manager.BackupDestinationType} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.backup_manager.BackupDestinationType} value
 * @return {!proto.backup_manager.ReplicationResult} returns this
 */
proto.backup_manager.ReplicationResult.prototype.setDestinationType = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional string destination_path = 3;
 * @return {string}
 */
proto.backup_manager.ReplicationResult.prototype.getDestinationPath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.ReplicationResult} returns this
 */
proto.backup_manager.ReplicationResult.prototype.setDestinationPath = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional BackupJobState state = 4;
 * @return {!proto.backup_manager.BackupJobState}
 */
proto.backup_manager.ReplicationResult.prototype.getState = function() {
  return /** @type {!proto.backup_manager.BackupJobState} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {!proto.backup_manager.BackupJobState} value
 * @return {!proto.backup_manager.ReplicationResult} returns this
 */
proto.backup_manager.ReplicationResult.prototype.setState = function(value) {
  return jspb.Message.setProto3EnumField(this, 4, value);
};


/**
 * optional string error_message = 5;
 * @return {string}
 */
proto.backup_manager.ReplicationResult.prototype.getErrorMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.ReplicationResult} returns this
 */
proto.backup_manager.ReplicationResult.prototype.setErrorMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional uint64 bytes_written = 6;
 * @return {number}
 */
proto.backup_manager.ReplicationResult.prototype.getBytesWritten = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.ReplicationResult} returns this
 */
proto.backup_manager.ReplicationResult.prototype.setBytesWritten = function(value) {
  return jspb.Message.setProto3IntField(this, 6, value);
};


/**
 * optional int64 started_unix_ms = 7;
 * @return {number}
 */
proto.backup_manager.ReplicationResult.prototype.getStartedUnixMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.ReplicationResult} returns this
 */
proto.backup_manager.ReplicationResult.prototype.setStartedUnixMs = function(value) {
  return jspb.Message.setProto3IntField(this, 7, value);
};


/**
 * optional int64 finished_unix_ms = 8;
 * @return {number}
 */
proto.backup_manager.ReplicationResult.prototype.getFinishedUnixMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 8, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.ReplicationResult} returns this
 */
proto.backup_manager.ReplicationResult.prototype.setFinishedUnixMs = function(value) {
  return jspb.Message.setProto3IntField(this, 8, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.backup_manager.ReplicationValidation.repeatedFields_ = [3];



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
proto.backup_manager.ReplicationValidation.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.ReplicationValidation.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.ReplicationValidation} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ReplicationValidation.toObject = function(includeInstance, msg) {
  var f, obj = {
destinationName: jspb.Message.getFieldWithDefault(msg, 1, ""),
ok: jspb.Message.getBooleanFieldWithDefault(msg, 2, false),
missingFilesList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f,
errorMessage: jspb.Message.getFieldWithDefault(msg, 4, "")
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
 * @return {!proto.backup_manager.ReplicationValidation}
 */
proto.backup_manager.ReplicationValidation.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.ReplicationValidation;
  return proto.backup_manager.ReplicationValidation.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.ReplicationValidation} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.ReplicationValidation}
 */
proto.backup_manager.ReplicationValidation.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setDestinationName(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setOk(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.addMissingFiles(value);
      break;
    case 4:
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
proto.backup_manager.ReplicationValidation.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.ReplicationValidation.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.ReplicationValidation} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ReplicationValidation.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDestinationName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOk();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
  f = message.getMissingFilesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      3,
      f
    );
  }
  f = message.getErrorMessage();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * optional string destination_name = 1;
 * @return {string}
 */
proto.backup_manager.ReplicationValidation.prototype.getDestinationName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.ReplicationValidation} returns this
 */
proto.backup_manager.ReplicationValidation.prototype.setDestinationName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bool ok = 2;
 * @return {boolean}
 */
proto.backup_manager.ReplicationValidation.prototype.getOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.ReplicationValidation} returns this
 */
proto.backup_manager.ReplicationValidation.prototype.setOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * repeated string missing_files = 3;
 * @return {!Array<string>}
 */
proto.backup_manager.ReplicationValidation.prototype.getMissingFilesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.backup_manager.ReplicationValidation} returns this
 */
proto.backup_manager.ReplicationValidation.prototype.setMissingFilesList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.ReplicationValidation} returns this
 */
proto.backup_manager.ReplicationValidation.prototype.addMissingFiles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.ReplicationValidation} returns this
 */
proto.backup_manager.ReplicationValidation.prototype.clearMissingFilesList = function() {
  return this.setMissingFilesList([]);
};


/**
 * optional string error_message = 4;
 * @return {string}
 */
proto.backup_manager.ReplicationValidation.prototype.getErrorMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.ReplicationValidation} returns this
 */
proto.backup_manager.ReplicationValidation.prototype.setErrorMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.backup_manager.BackupProviderResult.repeatedFields_ = [11,12];



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
proto.backup_manager.BackupProviderResult.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.BackupProviderResult.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.BackupProviderResult} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.BackupProviderResult.toObject = function(includeInstance, msg) {
  var f, obj = {
type: jspb.Message.getFieldWithDefault(msg, 1, 0),
enabled: jspb.Message.getBooleanFieldWithDefault(msg, 2, false),
state: jspb.Message.getFieldWithDefault(msg, 3, 0),
severity: jspb.Message.getFieldWithDefault(msg, 4, 0),
summary: jspb.Message.getFieldWithDefault(msg, 5, ""),
outputsMap: (f = msg.getOutputsMap()) ? f.toObject(includeInstance, undefined) : [],
errorMessage: jspb.Message.getFieldWithDefault(msg, 7, ""),
startedUnixMs: jspb.Message.getFieldWithDefault(msg, 8, 0),
finishedUnixMs: jspb.Message.getFieldWithDefault(msg, 9, 0),
bytesWritten: jspb.Message.getFieldWithDefault(msg, 10, 0),
payloadFilesList: (f = jspb.Message.getRepeatedField(msg, 11)) == null ? undefined : f,
outputFilesList: (f = jspb.Message.getRepeatedField(msg, 12)) == null ? undefined : f,
restoreInputsMap: (f = msg.getRestoreInputsMap()) ? f.toObject(includeInstance, undefined) : []
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
 * @return {!proto.backup_manager.BackupProviderResult}
 */
proto.backup_manager.BackupProviderResult.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.BackupProviderResult;
  return proto.backup_manager.BackupProviderResult.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.BackupProviderResult} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.BackupProviderResult}
 */
proto.backup_manager.BackupProviderResult.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.backup_manager.BackupProviderType} */ (reader.readEnum());
      msg.setType(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setEnabled(value);
      break;
    case 3:
      var value = /** @type {!proto.backup_manager.BackupJobState} */ (reader.readEnum());
      msg.setState(value);
      break;
    case 4:
      var value = /** @type {!proto.backup_manager.BackupSeverity} */ (reader.readEnum());
      msg.setSeverity(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setSummary(value);
      break;
    case 6:
      var value = msg.getOutputsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setErrorMessage(value);
      break;
    case 8:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setStartedUnixMs(value);
      break;
    case 9:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setFinishedUnixMs(value);
      break;
    case 10:
      var value = /** @type {number} */ (reader.readUint64());
      msg.setBytesWritten(value);
      break;
    case 11:
      var value = /** @type {string} */ (reader.readString());
      msg.addPayloadFiles(value);
      break;
    case 12:
      var value = /** @type {string} */ (reader.readString());
      msg.addOutputFiles(value);
      break;
    case 13:
      var value = msg.getRestoreInputsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
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
proto.backup_manager.BackupProviderResult.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.BackupProviderResult.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.BackupProviderResult} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.BackupProviderResult.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getType();
  if (f !== 0.0) {
    writer.writeEnum(
      1,
      f
    );
  }
  f = message.getEnabled();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
  f = message.getState();
  if (f !== 0.0) {
    writer.writeEnum(
      3,
      f
    );
  }
  f = message.getSeverity();
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
  f = message.getOutputsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(6, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
  f = message.getErrorMessage();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getStartedUnixMs();
  if (f !== 0) {
    writer.writeInt64(
      8,
      f
    );
  }
  f = message.getFinishedUnixMs();
  if (f !== 0) {
    writer.writeInt64(
      9,
      f
    );
  }
  f = message.getBytesWritten();
  if (f !== 0) {
    writer.writeUint64(
      10,
      f
    );
  }
  f = message.getPayloadFilesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      11,
      f
    );
  }
  f = message.getOutputFilesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      12,
      f
    );
  }
  f = message.getRestoreInputsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(13, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
};


/**
 * optional BackupProviderType type = 1;
 * @return {!proto.backup_manager.BackupProviderType}
 */
proto.backup_manager.BackupProviderResult.prototype.getType = function() {
  return /** @type {!proto.backup_manager.BackupProviderType} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {!proto.backup_manager.BackupProviderType} value
 * @return {!proto.backup_manager.BackupProviderResult} returns this
 */
proto.backup_manager.BackupProviderResult.prototype.setType = function(value) {
  return jspb.Message.setProto3EnumField(this, 1, value);
};


/**
 * optional bool enabled = 2;
 * @return {boolean}
 */
proto.backup_manager.BackupProviderResult.prototype.getEnabled = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.BackupProviderResult} returns this
 */
proto.backup_manager.BackupProviderResult.prototype.setEnabled = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * optional BackupJobState state = 3;
 * @return {!proto.backup_manager.BackupJobState}
 */
proto.backup_manager.BackupProviderResult.prototype.getState = function() {
  return /** @type {!proto.backup_manager.BackupJobState} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.backup_manager.BackupJobState} value
 * @return {!proto.backup_manager.BackupProviderResult} returns this
 */
proto.backup_manager.BackupProviderResult.prototype.setState = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
};


/**
 * optional BackupSeverity severity = 4;
 * @return {!proto.backup_manager.BackupSeverity}
 */
proto.backup_manager.BackupProviderResult.prototype.getSeverity = function() {
  return /** @type {!proto.backup_manager.BackupSeverity} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {!proto.backup_manager.BackupSeverity} value
 * @return {!proto.backup_manager.BackupProviderResult} returns this
 */
proto.backup_manager.BackupProviderResult.prototype.setSeverity = function(value) {
  return jspb.Message.setProto3EnumField(this, 4, value);
};


/**
 * optional string summary = 5;
 * @return {string}
 */
proto.backup_manager.BackupProviderResult.prototype.getSummary = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.BackupProviderResult} returns this
 */
proto.backup_manager.BackupProviderResult.prototype.setSummary = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * map<string, string> outputs = 6;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.backup_manager.BackupProviderResult.prototype.getOutputsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 6, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.backup_manager.BackupProviderResult} returns this
 */
proto.backup_manager.BackupProviderResult.prototype.clearOutputsMap = function() {
  this.getOutputsMap().clear();
  return this;
};


/**
 * optional string error_message = 7;
 * @return {string}
 */
proto.backup_manager.BackupProviderResult.prototype.getErrorMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.BackupProviderResult} returns this
 */
proto.backup_manager.BackupProviderResult.prototype.setErrorMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional int64 started_unix_ms = 8;
 * @return {number}
 */
proto.backup_manager.BackupProviderResult.prototype.getStartedUnixMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 8, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.BackupProviderResult} returns this
 */
proto.backup_manager.BackupProviderResult.prototype.setStartedUnixMs = function(value) {
  return jspb.Message.setProto3IntField(this, 8, value);
};


/**
 * optional int64 finished_unix_ms = 9;
 * @return {number}
 */
proto.backup_manager.BackupProviderResult.prototype.getFinishedUnixMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 9, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.BackupProviderResult} returns this
 */
proto.backup_manager.BackupProviderResult.prototype.setFinishedUnixMs = function(value) {
  return jspb.Message.setProto3IntField(this, 9, value);
};


/**
 * optional uint64 bytes_written = 10;
 * @return {number}
 */
proto.backup_manager.BackupProviderResult.prototype.getBytesWritten = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 10, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.BackupProviderResult} returns this
 */
proto.backup_manager.BackupProviderResult.prototype.setBytesWritten = function(value) {
  return jspb.Message.setProto3IntField(this, 10, value);
};


/**
 * repeated string payload_files = 11;
 * @return {!Array<string>}
 */
proto.backup_manager.BackupProviderResult.prototype.getPayloadFilesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 11));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.backup_manager.BackupProviderResult} returns this
 */
proto.backup_manager.BackupProviderResult.prototype.setPayloadFilesList = function(value) {
  return jspb.Message.setField(this, 11, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.BackupProviderResult} returns this
 */
proto.backup_manager.BackupProviderResult.prototype.addPayloadFiles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 11, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.BackupProviderResult} returns this
 */
proto.backup_manager.BackupProviderResult.prototype.clearPayloadFilesList = function() {
  return this.setPayloadFilesList([]);
};


/**
 * repeated string output_files = 12;
 * @return {!Array<string>}
 */
proto.backup_manager.BackupProviderResult.prototype.getOutputFilesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 12));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.backup_manager.BackupProviderResult} returns this
 */
proto.backup_manager.BackupProviderResult.prototype.setOutputFilesList = function(value) {
  return jspb.Message.setField(this, 12, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.BackupProviderResult} returns this
 */
proto.backup_manager.BackupProviderResult.prototype.addOutputFiles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 12, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.BackupProviderResult} returns this
 */
proto.backup_manager.BackupProviderResult.prototype.clearOutputFilesList = function() {
  return this.setOutputFilesList([]);
};


/**
 * map<string, string> restore_inputs = 13;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.backup_manager.BackupProviderResult.prototype.getRestoreInputsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 13, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.backup_manager.BackupProviderResult} returns this
 */
proto.backup_manager.BackupProviderResult.prototype.clearRestoreInputsMap = function() {
  this.getRestoreInputsMap().clear();
  return this;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.backup_manager.BackupJob.repeatedFields_ = [8,11];



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
proto.backup_manager.BackupJob.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.BackupJob.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.BackupJob} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.BackupJob.toObject = function(includeInstance, msg) {
  var f, obj = {
jobId: jspb.Message.getFieldWithDefault(msg, 1, ""),
planName: jspb.Message.getFieldWithDefault(msg, 2, ""),
state: jspb.Message.getFieldWithDefault(msg, 3, 0),
createdUnixMs: jspb.Message.getFieldWithDefault(msg, 4, 0),
startedUnixMs: jspb.Message.getFieldWithDefault(msg, 5, 0),
finishedUnixMs: jspb.Message.getFieldWithDefault(msg, 6, 0),
plan: (f = msg.getPlan()) && proto.backup_manager.BackupPlan.toObject(includeInstance, f),
resultsList: jspb.Message.toObjectList(msg.getResultsList(),
    proto.backup_manager.BackupProviderResult.toObject, includeInstance),
backupId: jspb.Message.getFieldWithDefault(msg, 9, ""),
message: jspb.Message.getFieldWithDefault(msg, 10, ""),
replicationsList: jspb.Message.toObjectList(msg.getReplicationsList(),
    proto.backup_manager.ReplicationResult.toObject, includeInstance),
jobType: jspb.Message.getFieldWithDefault(msg, 12, 0)
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
 * @return {!proto.backup_manager.BackupJob}
 */
proto.backup_manager.BackupJob.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.BackupJob;
  return proto.backup_manager.BackupJob.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.BackupJob} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.BackupJob}
 */
proto.backup_manager.BackupJob.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setJobId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlanName(value);
      break;
    case 3:
      var value = /** @type {!proto.backup_manager.BackupJobState} */ (reader.readEnum());
      msg.setState(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setCreatedUnixMs(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setStartedUnixMs(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setFinishedUnixMs(value);
      break;
    case 7:
      var value = new proto.backup_manager.BackupPlan;
      reader.readMessage(value,proto.backup_manager.BackupPlan.deserializeBinaryFromReader);
      msg.setPlan(value);
      break;
    case 8:
      var value = new proto.backup_manager.BackupProviderResult;
      reader.readMessage(value,proto.backup_manager.BackupProviderResult.deserializeBinaryFromReader);
      msg.addResults(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setBackupId(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
      break;
    case 11:
      var value = new proto.backup_manager.ReplicationResult;
      reader.readMessage(value,proto.backup_manager.ReplicationResult.deserializeBinaryFromReader);
      msg.addReplications(value);
      break;
    case 12:
      var value = /** @type {!proto.backup_manager.BackupJobType} */ (reader.readEnum());
      msg.setJobType(value);
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
proto.backup_manager.BackupJob.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.BackupJob.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.BackupJob} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.BackupJob.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getJobId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getPlanName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getState();
  if (f !== 0.0) {
    writer.writeEnum(
      3,
      f
    );
  }
  f = message.getCreatedUnixMs();
  if (f !== 0) {
    writer.writeInt64(
      4,
      f
    );
  }
  f = message.getStartedUnixMs();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
  f = message.getFinishedUnixMs();
  if (f !== 0) {
    writer.writeInt64(
      6,
      f
    );
  }
  f = message.getPlan();
  if (f != null) {
    writer.writeMessage(
      7,
      f,
      proto.backup_manager.BackupPlan.serializeBinaryToWriter
    );
  }
  f = message.getResultsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      8,
      f,
      proto.backup_manager.BackupProviderResult.serializeBinaryToWriter
    );
  }
  f = message.getBackupId();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
  f = message.getMessage();
  if (f.length > 0) {
    writer.writeString(
      10,
      f
    );
  }
  f = message.getReplicationsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      11,
      f,
      proto.backup_manager.ReplicationResult.serializeBinaryToWriter
    );
  }
  f = message.getJobType();
  if (f !== 0.0) {
    writer.writeEnum(
      12,
      f
    );
  }
};


/**
 * optional string job_id = 1;
 * @return {string}
 */
proto.backup_manager.BackupJob.prototype.getJobId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.BackupJob} returns this
 */
proto.backup_manager.BackupJob.prototype.setJobId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string plan_name = 2;
 * @return {string}
 */
proto.backup_manager.BackupJob.prototype.getPlanName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.BackupJob} returns this
 */
proto.backup_manager.BackupJob.prototype.setPlanName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional BackupJobState state = 3;
 * @return {!proto.backup_manager.BackupJobState}
 */
proto.backup_manager.BackupJob.prototype.getState = function() {
  return /** @type {!proto.backup_manager.BackupJobState} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.backup_manager.BackupJobState} value
 * @return {!proto.backup_manager.BackupJob} returns this
 */
proto.backup_manager.BackupJob.prototype.setState = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
};


/**
 * optional int64 created_unix_ms = 4;
 * @return {number}
 */
proto.backup_manager.BackupJob.prototype.getCreatedUnixMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.BackupJob} returns this
 */
proto.backup_manager.BackupJob.prototype.setCreatedUnixMs = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional int64 started_unix_ms = 5;
 * @return {number}
 */
proto.backup_manager.BackupJob.prototype.getStartedUnixMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.BackupJob} returns this
 */
proto.backup_manager.BackupJob.prototype.setStartedUnixMs = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional int64 finished_unix_ms = 6;
 * @return {number}
 */
proto.backup_manager.BackupJob.prototype.getFinishedUnixMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.BackupJob} returns this
 */
proto.backup_manager.BackupJob.prototype.setFinishedUnixMs = function(value) {
  return jspb.Message.setProto3IntField(this, 6, value);
};


/**
 * optional BackupPlan plan = 7;
 * @return {?proto.backup_manager.BackupPlan}
 */
proto.backup_manager.BackupJob.prototype.getPlan = function() {
  return /** @type{?proto.backup_manager.BackupPlan} */ (
    jspb.Message.getWrapperField(this, proto.backup_manager.BackupPlan, 7));
};


/**
 * @param {?proto.backup_manager.BackupPlan|undefined} value
 * @return {!proto.backup_manager.BackupJob} returns this
*/
proto.backup_manager.BackupJob.prototype.setPlan = function(value) {
  return jspb.Message.setWrapperField(this, 7, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.backup_manager.BackupJob} returns this
 */
proto.backup_manager.BackupJob.prototype.clearPlan = function() {
  return this.setPlan(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.backup_manager.BackupJob.prototype.hasPlan = function() {
  return jspb.Message.getField(this, 7) != null;
};


/**
 * repeated BackupProviderResult results = 8;
 * @return {!Array<!proto.backup_manager.BackupProviderResult>}
 */
proto.backup_manager.BackupJob.prototype.getResultsList = function() {
  return /** @type{!Array<!proto.backup_manager.BackupProviderResult>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.BackupProviderResult, 8));
};


/**
 * @param {!Array<!proto.backup_manager.BackupProviderResult>} value
 * @return {!proto.backup_manager.BackupJob} returns this
*/
proto.backup_manager.BackupJob.prototype.setResultsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 8, value);
};


/**
 * @param {!proto.backup_manager.BackupProviderResult=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.BackupProviderResult}
 */
proto.backup_manager.BackupJob.prototype.addResults = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 8, opt_value, proto.backup_manager.BackupProviderResult, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.BackupJob} returns this
 */
proto.backup_manager.BackupJob.prototype.clearResultsList = function() {
  return this.setResultsList([]);
};


/**
 * optional string backup_id = 9;
 * @return {string}
 */
proto.backup_manager.BackupJob.prototype.getBackupId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.BackupJob} returns this
 */
proto.backup_manager.BackupJob.prototype.setBackupId = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};


/**
 * optional string message = 10;
 * @return {string}
 */
proto.backup_manager.BackupJob.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 10, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.BackupJob} returns this
 */
proto.backup_manager.BackupJob.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 10, value);
};


/**
 * repeated ReplicationResult replications = 11;
 * @return {!Array<!proto.backup_manager.ReplicationResult>}
 */
proto.backup_manager.BackupJob.prototype.getReplicationsList = function() {
  return /** @type{!Array<!proto.backup_manager.ReplicationResult>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.ReplicationResult, 11));
};


/**
 * @param {!Array<!proto.backup_manager.ReplicationResult>} value
 * @return {!proto.backup_manager.BackupJob} returns this
*/
proto.backup_manager.BackupJob.prototype.setReplicationsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 11, value);
};


/**
 * @param {!proto.backup_manager.ReplicationResult=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.ReplicationResult}
 */
proto.backup_manager.BackupJob.prototype.addReplications = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 11, opt_value, proto.backup_manager.ReplicationResult, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.BackupJob} returns this
 */
proto.backup_manager.BackupJob.prototype.clearReplicationsList = function() {
  return this.setReplicationsList([]);
};


/**
 * optional BackupJobType job_type = 12;
 * @return {!proto.backup_manager.BackupJobType}
 */
proto.backup_manager.BackupJob.prototype.getJobType = function() {
  return /** @type {!proto.backup_manager.BackupJobType} */ (jspb.Message.getFieldWithDefault(this, 12, 0));
};


/**
 * @param {!proto.backup_manager.BackupJobType} value
 * @return {!proto.backup_manager.BackupJob} returns this
 */
proto.backup_manager.BackupJob.prototype.setJobType = function(value) {
  return jspb.Message.setProto3EnumField(this, 12, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.backup_manager.BackupArtifact.repeatedFields_ = [8,11,12,20];



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
proto.backup_manager.BackupArtifact.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.BackupArtifact.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.BackupArtifact} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.BackupArtifact.toObject = function(includeInstance, msg) {
  var f, obj = {
backupId: jspb.Message.getFieldWithDefault(msg, 1, ""),
createdUnixMs: jspb.Message.getFieldWithDefault(msg, 2, 0),
location: jspb.Message.getFieldWithDefault(msg, 3, ""),
planName: jspb.Message.getFieldWithDefault(msg, 4, ""),
clusterId: jspb.Message.getFieldWithDefault(msg, 5, ""),
domain: jspb.Message.getFieldWithDefault(msg, 6, ""),
createdBy: jspb.Message.getFieldWithDefault(msg, 7, ""),
providerResultsList: jspb.Message.toObjectList(msg.getProviderResultsList(),
    proto.backup_manager.BackupProviderResult.toObject, includeInstance),
manifestSha256: jspb.Message.getFieldWithDefault(msg, 9, ""),
totalBytes: jspb.Message.getFieldWithDefault(msg, 10, 0),
locationsList: (f = jspb.Message.getRepeatedField(msg, 11)) == null ? undefined : f,
replicationsList: jspb.Message.toObjectList(msg.getReplicationsList(),
    proto.backup_manager.ReplicationResult.toObject, includeInstance),
schemaVersion: jspb.Message.getFieldWithDefault(msg, 13, 0),
mode: jspb.Message.getFieldWithDefault(msg, 14, 0),
scope: (f = msg.getScope()) && proto.backup_manager.BackupScope.toObject(includeInstance, f),
labelsMap: (f = msg.getLabelsMap()) ? f.toObject(includeInstance, undefined) : [],
qualityState: jspb.Message.getFieldWithDefault(msg, 17, 0),
cluster: (f = msg.getCluster()) && proto.backup_manager.ClusterInfo.toObject(includeInstance, f),
hooks: (f = msg.getHooks()) && proto.backup_manager.HookSummary.toObject(includeInstance, f),
skippedProvidersList: jspb.Message.toObjectList(msg.getSkippedProvidersList(),
    proto.backup_manager.SkippedProvider.toObject, includeInstance)
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
 * @return {!proto.backup_manager.BackupArtifact}
 */
proto.backup_manager.BackupArtifact.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.BackupArtifact;
  return proto.backup_manager.BackupArtifact.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.BackupArtifact} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.BackupArtifact}
 */
proto.backup_manager.BackupArtifact.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setBackupId(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setCreatedUnixMs(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setLocation(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlanName(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterId(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setDomain(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setCreatedBy(value);
      break;
    case 8:
      var value = new proto.backup_manager.BackupProviderResult;
      reader.readMessage(value,proto.backup_manager.BackupProviderResult.deserializeBinaryFromReader);
      msg.addProviderResults(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setManifestSha256(value);
      break;
    case 10:
      var value = /** @type {number} */ (reader.readUint64());
      msg.setTotalBytes(value);
      break;
    case 11:
      var value = /** @type {string} */ (reader.readString());
      msg.addLocations(value);
      break;
    case 12:
      var value = new proto.backup_manager.ReplicationResult;
      reader.readMessage(value,proto.backup_manager.ReplicationResult.deserializeBinaryFromReader);
      msg.addReplications(value);
      break;
    case 13:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setSchemaVersion(value);
      break;
    case 14:
      var value = /** @type {!proto.backup_manager.BackupMode} */ (reader.readEnum());
      msg.setMode(value);
      break;
    case 15:
      var value = new proto.backup_manager.BackupScope;
      reader.readMessage(value,proto.backup_manager.BackupScope.deserializeBinaryFromReader);
      msg.setScope(value);
      break;
    case 16:
      var value = msg.getLabelsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
      break;
    case 17:
      var value = /** @type {!proto.backup_manager.QualityState} */ (reader.readEnum());
      msg.setQualityState(value);
      break;
    case 18:
      var value = new proto.backup_manager.ClusterInfo;
      reader.readMessage(value,proto.backup_manager.ClusterInfo.deserializeBinaryFromReader);
      msg.setCluster(value);
      break;
    case 19:
      var value = new proto.backup_manager.HookSummary;
      reader.readMessage(value,proto.backup_manager.HookSummary.deserializeBinaryFromReader);
      msg.setHooks(value);
      break;
    case 20:
      var value = new proto.backup_manager.SkippedProvider;
      reader.readMessage(value,proto.backup_manager.SkippedProvider.deserializeBinaryFromReader);
      msg.addSkippedProviders(value);
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
proto.backup_manager.BackupArtifact.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.BackupArtifact.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.BackupArtifact} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.BackupArtifact.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBackupId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getCreatedUnixMs();
  if (f !== 0) {
    writer.writeInt64(
      2,
      f
    );
  }
  f = message.getLocation();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getPlanName();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getDomain();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getCreatedBy();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getProviderResultsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      8,
      f,
      proto.backup_manager.BackupProviderResult.serializeBinaryToWriter
    );
  }
  f = message.getManifestSha256();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
  f = message.getTotalBytes();
  if (f !== 0) {
    writer.writeUint64(
      10,
      f
    );
  }
  f = message.getLocationsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      11,
      f
    );
  }
  f = message.getReplicationsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      12,
      f,
      proto.backup_manager.ReplicationResult.serializeBinaryToWriter
    );
  }
  f = message.getSchemaVersion();
  if (f !== 0) {
    writer.writeUint32(
      13,
      f
    );
  }
  f = message.getMode();
  if (f !== 0.0) {
    writer.writeEnum(
      14,
      f
    );
  }
  f = message.getScope();
  if (f != null) {
    writer.writeMessage(
      15,
      f,
      proto.backup_manager.BackupScope.serializeBinaryToWriter
    );
  }
  f = message.getLabelsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(16, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
  f = message.getQualityState();
  if (f !== 0.0) {
    writer.writeEnum(
      17,
      f
    );
  }
  f = message.getCluster();
  if (f != null) {
    writer.writeMessage(
      18,
      f,
      proto.backup_manager.ClusterInfo.serializeBinaryToWriter
    );
  }
  f = message.getHooks();
  if (f != null) {
    writer.writeMessage(
      19,
      f,
      proto.backup_manager.HookSummary.serializeBinaryToWriter
    );
  }
  f = message.getSkippedProvidersList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      20,
      f,
      proto.backup_manager.SkippedProvider.serializeBinaryToWriter
    );
  }
};


/**
 * optional string backup_id = 1;
 * @return {string}
 */
proto.backup_manager.BackupArtifact.prototype.getBackupId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.setBackupId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int64 created_unix_ms = 2;
 * @return {number}
 */
proto.backup_manager.BackupArtifact.prototype.getCreatedUnixMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.setCreatedUnixMs = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string location = 3;
 * @return {string}
 */
proto.backup_manager.BackupArtifact.prototype.getLocation = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.setLocation = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string plan_name = 4;
 * @return {string}
 */
proto.backup_manager.BackupArtifact.prototype.getPlanName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.setPlanName = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string cluster_id = 5;
 * @return {string}
 */
proto.backup_manager.BackupArtifact.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string domain = 6;
 * @return {string}
 */
proto.backup_manager.BackupArtifact.prototype.getDomain = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.setDomain = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string created_by = 7;
 * @return {string}
 */
proto.backup_manager.BackupArtifact.prototype.getCreatedBy = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.setCreatedBy = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * repeated BackupProviderResult provider_results = 8;
 * @return {!Array<!proto.backup_manager.BackupProviderResult>}
 */
proto.backup_manager.BackupArtifact.prototype.getProviderResultsList = function() {
  return /** @type{!Array<!proto.backup_manager.BackupProviderResult>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.BackupProviderResult, 8));
};


/**
 * @param {!Array<!proto.backup_manager.BackupProviderResult>} value
 * @return {!proto.backup_manager.BackupArtifact} returns this
*/
proto.backup_manager.BackupArtifact.prototype.setProviderResultsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 8, value);
};


/**
 * @param {!proto.backup_manager.BackupProviderResult=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.BackupProviderResult}
 */
proto.backup_manager.BackupArtifact.prototype.addProviderResults = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 8, opt_value, proto.backup_manager.BackupProviderResult, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.clearProviderResultsList = function() {
  return this.setProviderResultsList([]);
};


/**
 * optional string manifest_sha256 = 9;
 * @return {string}
 */
proto.backup_manager.BackupArtifact.prototype.getManifestSha256 = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.setManifestSha256 = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};


/**
 * optional uint64 total_bytes = 10;
 * @return {number}
 */
proto.backup_manager.BackupArtifact.prototype.getTotalBytes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 10, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.setTotalBytes = function(value) {
  return jspb.Message.setProto3IntField(this, 10, value);
};


/**
 * repeated string locations = 11;
 * @return {!Array<string>}
 */
proto.backup_manager.BackupArtifact.prototype.getLocationsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 11));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.setLocationsList = function(value) {
  return jspb.Message.setField(this, 11, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.addLocations = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 11, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.clearLocationsList = function() {
  return this.setLocationsList([]);
};


/**
 * repeated ReplicationResult replications = 12;
 * @return {!Array<!proto.backup_manager.ReplicationResult>}
 */
proto.backup_manager.BackupArtifact.prototype.getReplicationsList = function() {
  return /** @type{!Array<!proto.backup_manager.ReplicationResult>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.ReplicationResult, 12));
};


/**
 * @param {!Array<!proto.backup_manager.ReplicationResult>} value
 * @return {!proto.backup_manager.BackupArtifact} returns this
*/
proto.backup_manager.BackupArtifact.prototype.setReplicationsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 12, value);
};


/**
 * @param {!proto.backup_manager.ReplicationResult=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.ReplicationResult}
 */
proto.backup_manager.BackupArtifact.prototype.addReplications = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 12, opt_value, proto.backup_manager.ReplicationResult, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.clearReplicationsList = function() {
  return this.setReplicationsList([]);
};


/**
 * optional uint32 schema_version = 13;
 * @return {number}
 */
proto.backup_manager.BackupArtifact.prototype.getSchemaVersion = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 13, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.setSchemaVersion = function(value) {
  return jspb.Message.setProto3IntField(this, 13, value);
};


/**
 * optional BackupMode mode = 14;
 * @return {!proto.backup_manager.BackupMode}
 */
proto.backup_manager.BackupArtifact.prototype.getMode = function() {
  return /** @type {!proto.backup_manager.BackupMode} */ (jspb.Message.getFieldWithDefault(this, 14, 0));
};


/**
 * @param {!proto.backup_manager.BackupMode} value
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.setMode = function(value) {
  return jspb.Message.setProto3EnumField(this, 14, value);
};


/**
 * optional BackupScope scope = 15;
 * @return {?proto.backup_manager.BackupScope}
 */
proto.backup_manager.BackupArtifact.prototype.getScope = function() {
  return /** @type{?proto.backup_manager.BackupScope} */ (
    jspb.Message.getWrapperField(this, proto.backup_manager.BackupScope, 15));
};


/**
 * @param {?proto.backup_manager.BackupScope|undefined} value
 * @return {!proto.backup_manager.BackupArtifact} returns this
*/
proto.backup_manager.BackupArtifact.prototype.setScope = function(value) {
  return jspb.Message.setWrapperField(this, 15, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.clearScope = function() {
  return this.setScope(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.backup_manager.BackupArtifact.prototype.hasScope = function() {
  return jspb.Message.getField(this, 15) != null;
};


/**
 * map<string, string> labels = 16;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.backup_manager.BackupArtifact.prototype.getLabelsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 16, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.clearLabelsMap = function() {
  this.getLabelsMap().clear();
  return this;
};


/**
 * optional QualityState quality_state = 17;
 * @return {!proto.backup_manager.QualityState}
 */
proto.backup_manager.BackupArtifact.prototype.getQualityState = function() {
  return /** @type {!proto.backup_manager.QualityState} */ (jspb.Message.getFieldWithDefault(this, 17, 0));
};


/**
 * @param {!proto.backup_manager.QualityState} value
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.setQualityState = function(value) {
  return jspb.Message.setProto3EnumField(this, 17, value);
};


/**
 * optional ClusterInfo cluster = 18;
 * @return {?proto.backup_manager.ClusterInfo}
 */
proto.backup_manager.BackupArtifact.prototype.getCluster = function() {
  return /** @type{?proto.backup_manager.ClusterInfo} */ (
    jspb.Message.getWrapperField(this, proto.backup_manager.ClusterInfo, 18));
};


/**
 * @param {?proto.backup_manager.ClusterInfo|undefined} value
 * @return {!proto.backup_manager.BackupArtifact} returns this
*/
proto.backup_manager.BackupArtifact.prototype.setCluster = function(value) {
  return jspb.Message.setWrapperField(this, 18, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.clearCluster = function() {
  return this.setCluster(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.backup_manager.BackupArtifact.prototype.hasCluster = function() {
  return jspb.Message.getField(this, 18) != null;
};


/**
 * optional HookSummary hooks = 19;
 * @return {?proto.backup_manager.HookSummary}
 */
proto.backup_manager.BackupArtifact.prototype.getHooks = function() {
  return /** @type{?proto.backup_manager.HookSummary} */ (
    jspb.Message.getWrapperField(this, proto.backup_manager.HookSummary, 19));
};


/**
 * @param {?proto.backup_manager.HookSummary|undefined} value
 * @return {!proto.backup_manager.BackupArtifact} returns this
*/
proto.backup_manager.BackupArtifact.prototype.setHooks = function(value) {
  return jspb.Message.setWrapperField(this, 19, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.clearHooks = function() {
  return this.setHooks(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.backup_manager.BackupArtifact.prototype.hasHooks = function() {
  return jspb.Message.getField(this, 19) != null;
};


/**
 * repeated SkippedProvider skipped_providers = 20;
 * @return {!Array<!proto.backup_manager.SkippedProvider>}
 */
proto.backup_manager.BackupArtifact.prototype.getSkippedProvidersList = function() {
  return /** @type{!Array<!proto.backup_manager.SkippedProvider>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.SkippedProvider, 20));
};


/**
 * @param {!Array<!proto.backup_manager.SkippedProvider>} value
 * @return {!proto.backup_manager.BackupArtifact} returns this
*/
proto.backup_manager.BackupArtifact.prototype.setSkippedProvidersList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 20, value);
};


/**
 * @param {!proto.backup_manager.SkippedProvider=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.SkippedProvider}
 */
proto.backup_manager.BackupArtifact.prototype.addSkippedProviders = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 20, opt_value, proto.backup_manager.SkippedProvider, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.BackupArtifact} returns this
 */
proto.backup_manager.BackupArtifact.prototype.clearSkippedProvidersList = function() {
  return this.setSkippedProvidersList([]);
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
proto.backup_manager.RetentionPolicy.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.RetentionPolicy.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.RetentionPolicy} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RetentionPolicy.toObject = function(includeInstance, msg) {
  var f, obj = {
keepLastN: jspb.Message.getFieldWithDefault(msg, 1, 0),
keepDays: jspb.Message.getFieldWithDefault(msg, 2, 0),
maxTotalBytes: jspb.Message.getFieldWithDefault(msg, 3, 0)
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
 * @return {!proto.backup_manager.RetentionPolicy}
 */
proto.backup_manager.RetentionPolicy.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.RetentionPolicy;
  return proto.backup_manager.RetentionPolicy.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.RetentionPolicy} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.RetentionPolicy}
 */
proto.backup_manager.RetentionPolicy.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setKeepLastN(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setKeepDays(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readUint64());
      msg.setMaxTotalBytes(value);
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
proto.backup_manager.RetentionPolicy.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.RetentionPolicy.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.RetentionPolicy} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RetentionPolicy.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getKeepLastN();
  if (f !== 0) {
    writer.writeUint32(
      1,
      f
    );
  }
  f = message.getKeepDays();
  if (f !== 0) {
    writer.writeUint32(
      2,
      f
    );
  }
  f = message.getMaxTotalBytes();
  if (f !== 0) {
    writer.writeUint64(
      3,
      f
    );
  }
};


/**
 * optional uint32 keep_last_n = 1;
 * @return {number}
 */
proto.backup_manager.RetentionPolicy.prototype.getKeepLastN = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.RetentionPolicy} returns this
 */
proto.backup_manager.RetentionPolicy.prototype.setKeepLastN = function(value) {
  return jspb.Message.setProto3IntField(this, 1, value);
};


/**
 * optional uint32 keep_days = 2;
 * @return {number}
 */
proto.backup_manager.RetentionPolicy.prototype.getKeepDays = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.RetentionPolicy} returns this
 */
proto.backup_manager.RetentionPolicy.prototype.setKeepDays = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional uint64 max_total_bytes = 3;
 * @return {number}
 */
proto.backup_manager.RetentionPolicy.prototype.getMaxTotalBytes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.RetentionPolicy} returns this
 */
proto.backup_manager.RetentionPolicy.prototype.setMaxTotalBytes = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
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
proto.backup_manager.RunBackupRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.RunBackupRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.RunBackupRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RunBackupRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
plan: (f = msg.getPlan()) && proto.backup_manager.BackupPlan.toObject(includeInstance, f),
requestId: jspb.Message.getFieldWithDefault(msg, 2, ""),
failIfRunning: jspb.Message.getBooleanFieldWithDefault(msg, 3, false),
mode: jspb.Message.getFieldWithDefault(msg, 4, 0),
scope: (f = msg.getScope()) && proto.backup_manager.BackupScope.toObject(includeInstance, f),
labelsMap: (f = msg.getLabelsMap()) ? f.toObject(includeInstance, undefined) : []
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
 * @return {!proto.backup_manager.RunBackupRequest}
 */
proto.backup_manager.RunBackupRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.RunBackupRequest;
  return proto.backup_manager.RunBackupRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.RunBackupRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.RunBackupRequest}
 */
proto.backup_manager.RunBackupRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.backup_manager.BackupPlan;
      reader.readMessage(value,proto.backup_manager.BackupPlan.deserializeBinaryFromReader);
      msg.setPlan(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setRequestId(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setFailIfRunning(value);
      break;
    case 4:
      var value = /** @type {!proto.backup_manager.BackupMode} */ (reader.readEnum());
      msg.setMode(value);
      break;
    case 5:
      var value = new proto.backup_manager.BackupScope;
      reader.readMessage(value,proto.backup_manager.BackupScope.deserializeBinaryFromReader);
      msg.setScope(value);
      break;
    case 6:
      var value = msg.getLabelsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
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
proto.backup_manager.RunBackupRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.RunBackupRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.RunBackupRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RunBackupRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPlan();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.backup_manager.BackupPlan.serializeBinaryToWriter
    );
  }
  f = message.getRequestId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getFailIfRunning();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
  f = message.getMode();
  if (f !== 0.0) {
    writer.writeEnum(
      4,
      f
    );
  }
  f = message.getScope();
  if (f != null) {
    writer.writeMessage(
      5,
      f,
      proto.backup_manager.BackupScope.serializeBinaryToWriter
    );
  }
  f = message.getLabelsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(6, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
};


/**
 * optional BackupPlan plan = 1;
 * @return {?proto.backup_manager.BackupPlan}
 */
proto.backup_manager.RunBackupRequest.prototype.getPlan = function() {
  return /** @type{?proto.backup_manager.BackupPlan} */ (
    jspb.Message.getWrapperField(this, proto.backup_manager.BackupPlan, 1));
};


/**
 * @param {?proto.backup_manager.BackupPlan|undefined} value
 * @return {!proto.backup_manager.RunBackupRequest} returns this
*/
proto.backup_manager.RunBackupRequest.prototype.setPlan = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.backup_manager.RunBackupRequest} returns this
 */
proto.backup_manager.RunBackupRequest.prototype.clearPlan = function() {
  return this.setPlan(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.backup_manager.RunBackupRequest.prototype.hasPlan = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string request_id = 2;
 * @return {string}
 */
proto.backup_manager.RunBackupRequest.prototype.getRequestId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.RunBackupRequest} returns this
 */
proto.backup_manager.RunBackupRequest.prototype.setRequestId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional bool fail_if_running = 3;
 * @return {boolean}
 */
proto.backup_manager.RunBackupRequest.prototype.getFailIfRunning = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.RunBackupRequest} returns this
 */
proto.backup_manager.RunBackupRequest.prototype.setFailIfRunning = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * optional BackupMode mode = 4;
 * @return {!proto.backup_manager.BackupMode}
 */
proto.backup_manager.RunBackupRequest.prototype.getMode = function() {
  return /** @type {!proto.backup_manager.BackupMode} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {!proto.backup_manager.BackupMode} value
 * @return {!proto.backup_manager.RunBackupRequest} returns this
 */
proto.backup_manager.RunBackupRequest.prototype.setMode = function(value) {
  return jspb.Message.setProto3EnumField(this, 4, value);
};


/**
 * optional BackupScope scope = 5;
 * @return {?proto.backup_manager.BackupScope}
 */
proto.backup_manager.RunBackupRequest.prototype.getScope = function() {
  return /** @type{?proto.backup_manager.BackupScope} */ (
    jspb.Message.getWrapperField(this, proto.backup_manager.BackupScope, 5));
};


/**
 * @param {?proto.backup_manager.BackupScope|undefined} value
 * @return {!proto.backup_manager.RunBackupRequest} returns this
*/
proto.backup_manager.RunBackupRequest.prototype.setScope = function(value) {
  return jspb.Message.setWrapperField(this, 5, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.backup_manager.RunBackupRequest} returns this
 */
proto.backup_manager.RunBackupRequest.prototype.clearScope = function() {
  return this.setScope(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.backup_manager.RunBackupRequest.prototype.hasScope = function() {
  return jspb.Message.getField(this, 5) != null;
};


/**
 * map<string, string> labels = 6;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.backup_manager.RunBackupRequest.prototype.getLabelsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 6, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.backup_manager.RunBackupRequest} returns this
 */
proto.backup_manager.RunBackupRequest.prototype.clearLabelsMap = function() {
  this.getLabelsMap().clear();
  return this;
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
proto.backup_manager.RunBackupResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.RunBackupResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.RunBackupResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RunBackupResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
jobId: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.backup_manager.RunBackupResponse}
 */
proto.backup_manager.RunBackupResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.RunBackupResponse;
  return proto.backup_manager.RunBackupResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.RunBackupResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.RunBackupResponse}
 */
proto.backup_manager.RunBackupResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setJobId(value);
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
proto.backup_manager.RunBackupResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.RunBackupResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.RunBackupResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RunBackupResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getJobId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string job_id = 1;
 * @return {string}
 */
proto.backup_manager.RunBackupResponse.prototype.getJobId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.RunBackupResponse} returns this
 */
proto.backup_manager.RunBackupResponse.prototype.setJobId = function(value) {
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
proto.backup_manager.GetBackupJobRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.GetBackupJobRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.GetBackupJobRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.GetBackupJobRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
jobId: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.backup_manager.GetBackupJobRequest}
 */
proto.backup_manager.GetBackupJobRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.GetBackupJobRequest;
  return proto.backup_manager.GetBackupJobRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.GetBackupJobRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.GetBackupJobRequest}
 */
proto.backup_manager.GetBackupJobRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setJobId(value);
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
proto.backup_manager.GetBackupJobRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.GetBackupJobRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.GetBackupJobRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.GetBackupJobRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getJobId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string job_id = 1;
 * @return {string}
 */
proto.backup_manager.GetBackupJobRequest.prototype.getJobId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.GetBackupJobRequest} returns this
 */
proto.backup_manager.GetBackupJobRequest.prototype.setJobId = function(value) {
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
proto.backup_manager.GetBackupJobResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.GetBackupJobResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.GetBackupJobResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.GetBackupJobResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
job: (f = msg.getJob()) && proto.backup_manager.BackupJob.toObject(includeInstance, f)
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
 * @return {!proto.backup_manager.GetBackupJobResponse}
 */
proto.backup_manager.GetBackupJobResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.GetBackupJobResponse;
  return proto.backup_manager.GetBackupJobResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.GetBackupJobResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.GetBackupJobResponse}
 */
proto.backup_manager.GetBackupJobResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.backup_manager.BackupJob;
      reader.readMessage(value,proto.backup_manager.BackupJob.deserializeBinaryFromReader);
      msg.setJob(value);
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
proto.backup_manager.GetBackupJobResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.GetBackupJobResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.GetBackupJobResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.GetBackupJobResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getJob();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.backup_manager.BackupJob.serializeBinaryToWriter
    );
  }
};


/**
 * optional BackupJob job = 1;
 * @return {?proto.backup_manager.BackupJob}
 */
proto.backup_manager.GetBackupJobResponse.prototype.getJob = function() {
  return /** @type{?proto.backup_manager.BackupJob} */ (
    jspb.Message.getWrapperField(this, proto.backup_manager.BackupJob, 1));
};


/**
 * @param {?proto.backup_manager.BackupJob|undefined} value
 * @return {!proto.backup_manager.GetBackupJobResponse} returns this
*/
proto.backup_manager.GetBackupJobResponse.prototype.setJob = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.backup_manager.GetBackupJobResponse} returns this
 */
proto.backup_manager.GetBackupJobResponse.prototype.clearJob = function() {
  return this.setJob(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.backup_manager.GetBackupJobResponse.prototype.hasJob = function() {
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
proto.backup_manager.ListBackupJobsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.ListBackupJobsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.ListBackupJobsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ListBackupJobsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
limit: jspb.Message.getFieldWithDefault(msg, 1, 0),
offset: jspb.Message.getFieldWithDefault(msg, 2, 0),
state: jspb.Message.getFieldWithDefault(msg, 3, 0),
planName: jspb.Message.getFieldWithDefault(msg, 4, "")
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
 * @return {!proto.backup_manager.ListBackupJobsRequest}
 */
proto.backup_manager.ListBackupJobsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.ListBackupJobsRequest;
  return proto.backup_manager.ListBackupJobsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.ListBackupJobsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.ListBackupJobsRequest}
 */
proto.backup_manager.ListBackupJobsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setLimit(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setOffset(value);
      break;
    case 3:
      var value = /** @type {!proto.backup_manager.BackupJobState} */ (reader.readEnum());
      msg.setState(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlanName(value);
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
proto.backup_manager.ListBackupJobsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.ListBackupJobsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.ListBackupJobsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ListBackupJobsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getLimit();
  if (f !== 0) {
    writer.writeUint32(
      1,
      f
    );
  }
  f = message.getOffset();
  if (f !== 0) {
    writer.writeUint32(
      2,
      f
    );
  }
  f = message.getState();
  if (f !== 0.0) {
    writer.writeEnum(
      3,
      f
    );
  }
  f = message.getPlanName();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * optional uint32 limit = 1;
 * @return {number}
 */
proto.backup_manager.ListBackupJobsRequest.prototype.getLimit = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.ListBackupJobsRequest} returns this
 */
proto.backup_manager.ListBackupJobsRequest.prototype.setLimit = function(value) {
  return jspb.Message.setProto3IntField(this, 1, value);
};


/**
 * optional uint32 offset = 2;
 * @return {number}
 */
proto.backup_manager.ListBackupJobsRequest.prototype.getOffset = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.ListBackupJobsRequest} returns this
 */
proto.backup_manager.ListBackupJobsRequest.prototype.setOffset = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional BackupJobState state = 3;
 * @return {!proto.backup_manager.BackupJobState}
 */
proto.backup_manager.ListBackupJobsRequest.prototype.getState = function() {
  return /** @type {!proto.backup_manager.BackupJobState} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.backup_manager.BackupJobState} value
 * @return {!proto.backup_manager.ListBackupJobsRequest} returns this
 */
proto.backup_manager.ListBackupJobsRequest.prototype.setState = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
};


/**
 * optional string plan_name = 4;
 * @return {string}
 */
proto.backup_manager.ListBackupJobsRequest.prototype.getPlanName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.ListBackupJobsRequest} returns this
 */
proto.backup_manager.ListBackupJobsRequest.prototype.setPlanName = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.backup_manager.ListBackupJobsResponse.repeatedFields_ = [1];



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
proto.backup_manager.ListBackupJobsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.ListBackupJobsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.ListBackupJobsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ListBackupJobsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
jobsList: jspb.Message.toObjectList(msg.getJobsList(),
    proto.backup_manager.BackupJob.toObject, includeInstance),
total: jspb.Message.getFieldWithDefault(msg, 2, 0)
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
 * @return {!proto.backup_manager.ListBackupJobsResponse}
 */
proto.backup_manager.ListBackupJobsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.ListBackupJobsResponse;
  return proto.backup_manager.ListBackupJobsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.ListBackupJobsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.ListBackupJobsResponse}
 */
proto.backup_manager.ListBackupJobsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.backup_manager.BackupJob;
      reader.readMessage(value,proto.backup_manager.BackupJob.deserializeBinaryFromReader);
      msg.addJobs(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setTotal(value);
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
proto.backup_manager.ListBackupJobsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.ListBackupJobsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.ListBackupJobsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ListBackupJobsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getJobsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.backup_manager.BackupJob.serializeBinaryToWriter
    );
  }
  f = message.getTotal();
  if (f !== 0) {
    writer.writeUint32(
      2,
      f
    );
  }
};


/**
 * repeated BackupJob jobs = 1;
 * @return {!Array<!proto.backup_manager.BackupJob>}
 */
proto.backup_manager.ListBackupJobsResponse.prototype.getJobsList = function() {
  return /** @type{!Array<!proto.backup_manager.BackupJob>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.BackupJob, 1));
};


/**
 * @param {!Array<!proto.backup_manager.BackupJob>} value
 * @return {!proto.backup_manager.ListBackupJobsResponse} returns this
*/
proto.backup_manager.ListBackupJobsResponse.prototype.setJobsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.backup_manager.BackupJob=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.BackupJob}
 */
proto.backup_manager.ListBackupJobsResponse.prototype.addJobs = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.backup_manager.BackupJob, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.ListBackupJobsResponse} returns this
 */
proto.backup_manager.ListBackupJobsResponse.prototype.clearJobsList = function() {
  return this.setJobsList([]);
};


/**
 * optional uint32 total = 2;
 * @return {number}
 */
proto.backup_manager.ListBackupJobsResponse.prototype.getTotal = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.ListBackupJobsResponse} returns this
 */
proto.backup_manager.ListBackupJobsResponse.prototype.setTotal = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
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
proto.backup_manager.ListBackupsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.ListBackupsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.ListBackupsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ListBackupsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
limit: jspb.Message.getFieldWithDefault(msg, 1, 0),
offset: jspb.Message.getFieldWithDefault(msg, 2, 0),
planName: jspb.Message.getFieldWithDefault(msg, 3, ""),
mode: jspb.Message.getFieldWithDefault(msg, 4, 0),
qualityState: jspb.Message.getFieldWithDefault(msg, 5, 0)
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
 * @return {!proto.backup_manager.ListBackupsRequest}
 */
proto.backup_manager.ListBackupsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.ListBackupsRequest;
  return proto.backup_manager.ListBackupsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.ListBackupsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.ListBackupsRequest}
 */
proto.backup_manager.ListBackupsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setLimit(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setOffset(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlanName(value);
      break;
    case 4:
      var value = /** @type {!proto.backup_manager.BackupMode} */ (reader.readEnum());
      msg.setMode(value);
      break;
    case 5:
      var value = /** @type {!proto.backup_manager.QualityState} */ (reader.readEnum());
      msg.setQualityState(value);
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
proto.backup_manager.ListBackupsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.ListBackupsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.ListBackupsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ListBackupsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getLimit();
  if (f !== 0) {
    writer.writeUint32(
      1,
      f
    );
  }
  f = message.getOffset();
  if (f !== 0) {
    writer.writeUint32(
      2,
      f
    );
  }
  f = message.getPlanName();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getMode();
  if (f !== 0.0) {
    writer.writeEnum(
      4,
      f
    );
  }
  f = message.getQualityState();
  if (f !== 0.0) {
    writer.writeEnum(
      5,
      f
    );
  }
};


/**
 * optional uint32 limit = 1;
 * @return {number}
 */
proto.backup_manager.ListBackupsRequest.prototype.getLimit = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.ListBackupsRequest} returns this
 */
proto.backup_manager.ListBackupsRequest.prototype.setLimit = function(value) {
  return jspb.Message.setProto3IntField(this, 1, value);
};


/**
 * optional uint32 offset = 2;
 * @return {number}
 */
proto.backup_manager.ListBackupsRequest.prototype.getOffset = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.ListBackupsRequest} returns this
 */
proto.backup_manager.ListBackupsRequest.prototype.setOffset = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string plan_name = 3;
 * @return {string}
 */
proto.backup_manager.ListBackupsRequest.prototype.getPlanName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.ListBackupsRequest} returns this
 */
proto.backup_manager.ListBackupsRequest.prototype.setPlanName = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional BackupMode mode = 4;
 * @return {!proto.backup_manager.BackupMode}
 */
proto.backup_manager.ListBackupsRequest.prototype.getMode = function() {
  return /** @type {!proto.backup_manager.BackupMode} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {!proto.backup_manager.BackupMode} value
 * @return {!proto.backup_manager.ListBackupsRequest} returns this
 */
proto.backup_manager.ListBackupsRequest.prototype.setMode = function(value) {
  return jspb.Message.setProto3EnumField(this, 4, value);
};


/**
 * optional QualityState quality_state = 5;
 * @return {!proto.backup_manager.QualityState}
 */
proto.backup_manager.ListBackupsRequest.prototype.getQualityState = function() {
  return /** @type {!proto.backup_manager.QualityState} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {!proto.backup_manager.QualityState} value
 * @return {!proto.backup_manager.ListBackupsRequest} returns this
 */
proto.backup_manager.ListBackupsRequest.prototype.setQualityState = function(value) {
  return jspb.Message.setProto3EnumField(this, 5, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.backup_manager.ListBackupsResponse.repeatedFields_ = [1];



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
proto.backup_manager.ListBackupsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.ListBackupsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.ListBackupsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ListBackupsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
backupsList: jspb.Message.toObjectList(msg.getBackupsList(),
    proto.backup_manager.BackupArtifact.toObject, includeInstance),
total: jspb.Message.getFieldWithDefault(msg, 2, 0)
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
 * @return {!proto.backup_manager.ListBackupsResponse}
 */
proto.backup_manager.ListBackupsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.ListBackupsResponse;
  return proto.backup_manager.ListBackupsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.ListBackupsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.ListBackupsResponse}
 */
proto.backup_manager.ListBackupsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.backup_manager.BackupArtifact;
      reader.readMessage(value,proto.backup_manager.BackupArtifact.deserializeBinaryFromReader);
      msg.addBackups(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setTotal(value);
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
proto.backup_manager.ListBackupsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.ListBackupsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.ListBackupsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ListBackupsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBackupsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.backup_manager.BackupArtifact.serializeBinaryToWriter
    );
  }
  f = message.getTotal();
  if (f !== 0) {
    writer.writeUint32(
      2,
      f
    );
  }
};


/**
 * repeated BackupArtifact backups = 1;
 * @return {!Array<!proto.backup_manager.BackupArtifact>}
 */
proto.backup_manager.ListBackupsResponse.prototype.getBackupsList = function() {
  return /** @type{!Array<!proto.backup_manager.BackupArtifact>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.BackupArtifact, 1));
};


/**
 * @param {!Array<!proto.backup_manager.BackupArtifact>} value
 * @return {!proto.backup_manager.ListBackupsResponse} returns this
*/
proto.backup_manager.ListBackupsResponse.prototype.setBackupsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.backup_manager.BackupArtifact=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.BackupArtifact}
 */
proto.backup_manager.ListBackupsResponse.prototype.addBackups = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.backup_manager.BackupArtifact, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.ListBackupsResponse} returns this
 */
proto.backup_manager.ListBackupsResponse.prototype.clearBackupsList = function() {
  return this.setBackupsList([]);
};


/**
 * optional uint32 total = 2;
 * @return {number}
 */
proto.backup_manager.ListBackupsResponse.prototype.getTotal = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.ListBackupsResponse} returns this
 */
proto.backup_manager.ListBackupsResponse.prototype.setTotal = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
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
proto.backup_manager.GetBackupRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.GetBackupRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.GetBackupRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.GetBackupRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
backupId: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.backup_manager.GetBackupRequest}
 */
proto.backup_manager.GetBackupRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.GetBackupRequest;
  return proto.backup_manager.GetBackupRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.GetBackupRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.GetBackupRequest}
 */
proto.backup_manager.GetBackupRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setBackupId(value);
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
proto.backup_manager.GetBackupRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.GetBackupRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.GetBackupRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.GetBackupRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBackupId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string backup_id = 1;
 * @return {string}
 */
proto.backup_manager.GetBackupRequest.prototype.getBackupId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.GetBackupRequest} returns this
 */
proto.backup_manager.GetBackupRequest.prototype.setBackupId = function(value) {
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
proto.backup_manager.GetBackupResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.GetBackupResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.GetBackupResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.GetBackupResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
backup: (f = msg.getBackup()) && proto.backup_manager.BackupArtifact.toObject(includeInstance, f)
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
 * @return {!proto.backup_manager.GetBackupResponse}
 */
proto.backup_manager.GetBackupResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.GetBackupResponse;
  return proto.backup_manager.GetBackupResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.GetBackupResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.GetBackupResponse}
 */
proto.backup_manager.GetBackupResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.backup_manager.BackupArtifact;
      reader.readMessage(value,proto.backup_manager.BackupArtifact.deserializeBinaryFromReader);
      msg.setBackup(value);
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
proto.backup_manager.GetBackupResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.GetBackupResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.GetBackupResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.GetBackupResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBackup();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.backup_manager.BackupArtifact.serializeBinaryToWriter
    );
  }
};


/**
 * optional BackupArtifact backup = 1;
 * @return {?proto.backup_manager.BackupArtifact}
 */
proto.backup_manager.GetBackupResponse.prototype.getBackup = function() {
  return /** @type{?proto.backup_manager.BackupArtifact} */ (
    jspb.Message.getWrapperField(this, proto.backup_manager.BackupArtifact, 1));
};


/**
 * @param {?proto.backup_manager.BackupArtifact|undefined} value
 * @return {!proto.backup_manager.GetBackupResponse} returns this
*/
proto.backup_manager.GetBackupResponse.prototype.setBackup = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.backup_manager.GetBackupResponse} returns this
 */
proto.backup_manager.GetBackupResponse.prototype.clearBackup = function() {
  return this.setBackup(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.backup_manager.GetBackupResponse.prototype.hasBackup = function() {
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
proto.backup_manager.DeleteBackupRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.DeleteBackupRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.DeleteBackupRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.DeleteBackupRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
backupId: jspb.Message.getFieldWithDefault(msg, 1, ""),
deleteProviderArtifacts: jspb.Message.getBooleanFieldWithDefault(msg, 2, false)
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
 * @return {!proto.backup_manager.DeleteBackupRequest}
 */
proto.backup_manager.DeleteBackupRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.DeleteBackupRequest;
  return proto.backup_manager.DeleteBackupRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.DeleteBackupRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.DeleteBackupRequest}
 */
proto.backup_manager.DeleteBackupRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setBackupId(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setDeleteProviderArtifacts(value);
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
proto.backup_manager.DeleteBackupRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.DeleteBackupRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.DeleteBackupRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.DeleteBackupRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBackupId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getDeleteProviderArtifacts();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
};


/**
 * optional string backup_id = 1;
 * @return {string}
 */
proto.backup_manager.DeleteBackupRequest.prototype.getBackupId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.DeleteBackupRequest} returns this
 */
proto.backup_manager.DeleteBackupRequest.prototype.setBackupId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bool delete_provider_artifacts = 2;
 * @return {boolean}
 */
proto.backup_manager.DeleteBackupRequest.prototype.getDeleteProviderArtifacts = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.DeleteBackupRequest} returns this
 */
proto.backup_manager.DeleteBackupRequest.prototype.setDeleteProviderArtifacts = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.backup_manager.DeleteBackupResponse.repeatedFields_ = [3,4];



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
proto.backup_manager.DeleteBackupResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.DeleteBackupResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.DeleteBackupResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.DeleteBackupResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
deleted: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
message: jspb.Message.getFieldWithDefault(msg, 2, ""),
providerResultsList: jspb.Message.toObjectList(msg.getProviderResultsList(),
    proto.backup_manager.DeleteResult.toObject, includeInstance),
replicationResultsList: jspb.Message.toObjectList(msg.getReplicationResultsList(),
    proto.backup_manager.DeleteResult.toObject, includeInstance)
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
 * @return {!proto.backup_manager.DeleteBackupResponse}
 */
proto.backup_manager.DeleteBackupResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.DeleteBackupResponse;
  return proto.backup_manager.DeleteBackupResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.DeleteBackupResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.DeleteBackupResponse}
 */
proto.backup_manager.DeleteBackupResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setDeleted(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
      break;
    case 3:
      var value = new proto.backup_manager.DeleteResult;
      reader.readMessage(value,proto.backup_manager.DeleteResult.deserializeBinaryFromReader);
      msg.addProviderResults(value);
      break;
    case 4:
      var value = new proto.backup_manager.DeleteResult;
      reader.readMessage(value,proto.backup_manager.DeleteResult.deserializeBinaryFromReader);
      msg.addReplicationResults(value);
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
proto.backup_manager.DeleteBackupResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.DeleteBackupResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.DeleteBackupResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.DeleteBackupResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDeleted();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getMessage();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getProviderResultsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      3,
      f,
      proto.backup_manager.DeleteResult.serializeBinaryToWriter
    );
  }
  f = message.getReplicationResultsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      4,
      f,
      proto.backup_manager.DeleteResult.serializeBinaryToWriter
    );
  }
};


/**
 * optional bool deleted = 1;
 * @return {boolean}
 */
proto.backup_manager.DeleteBackupResponse.prototype.getDeleted = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.DeleteBackupResponse} returns this
 */
proto.backup_manager.DeleteBackupResponse.prototype.setDeleted = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional string message = 2;
 * @return {string}
 */
proto.backup_manager.DeleteBackupResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.DeleteBackupResponse} returns this
 */
proto.backup_manager.DeleteBackupResponse.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * repeated DeleteResult provider_results = 3;
 * @return {!Array<!proto.backup_manager.DeleteResult>}
 */
proto.backup_manager.DeleteBackupResponse.prototype.getProviderResultsList = function() {
  return /** @type{!Array<!proto.backup_manager.DeleteResult>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.DeleteResult, 3));
};


/**
 * @param {!Array<!proto.backup_manager.DeleteResult>} value
 * @return {!proto.backup_manager.DeleteBackupResponse} returns this
*/
proto.backup_manager.DeleteBackupResponse.prototype.setProviderResultsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 3, value);
};


/**
 * @param {!proto.backup_manager.DeleteResult=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.DeleteResult}
 */
proto.backup_manager.DeleteBackupResponse.prototype.addProviderResults = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 3, opt_value, proto.backup_manager.DeleteResult, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.DeleteBackupResponse} returns this
 */
proto.backup_manager.DeleteBackupResponse.prototype.clearProviderResultsList = function() {
  return this.setProviderResultsList([]);
};


/**
 * repeated DeleteResult replication_results = 4;
 * @return {!Array<!proto.backup_manager.DeleteResult>}
 */
proto.backup_manager.DeleteBackupResponse.prototype.getReplicationResultsList = function() {
  return /** @type{!Array<!proto.backup_manager.DeleteResult>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.DeleteResult, 4));
};


/**
 * @param {!Array<!proto.backup_manager.DeleteResult>} value
 * @return {!proto.backup_manager.DeleteBackupResponse} returns this
*/
proto.backup_manager.DeleteBackupResponse.prototype.setReplicationResultsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 4, value);
};


/**
 * @param {!proto.backup_manager.DeleteResult=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.DeleteResult}
 */
proto.backup_manager.DeleteBackupResponse.prototype.addReplicationResults = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 4, opt_value, proto.backup_manager.DeleteResult, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.DeleteBackupResponse} returns this
 */
proto.backup_manager.DeleteBackupResponse.prototype.clearReplicationResultsList = function() {
  return this.setReplicationResultsList([]);
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
proto.backup_manager.DeleteResult.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.DeleteResult.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.DeleteResult} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.DeleteResult.toObject = function(includeInstance, msg) {
  var f, obj = {
target: jspb.Message.getFieldWithDefault(msg, 1, ""),
ok: jspb.Message.getBooleanFieldWithDefault(msg, 2, false),
message: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.backup_manager.DeleteResult}
 */
proto.backup_manager.DeleteResult.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.DeleteResult;
  return proto.backup_manager.DeleteResult.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.DeleteResult} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.DeleteResult}
 */
proto.backup_manager.DeleteResult.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setTarget(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setOk(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
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
proto.backup_manager.DeleteResult.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.DeleteResult.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.DeleteResult} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.DeleteResult.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTarget();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOk();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
  f = message.getMessage();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string target = 1;
 * @return {string}
 */
proto.backup_manager.DeleteResult.prototype.getTarget = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.DeleteResult} returns this
 */
proto.backup_manager.DeleteResult.prototype.setTarget = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bool ok = 2;
 * @return {boolean}
 */
proto.backup_manager.DeleteResult.prototype.getOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.DeleteResult} returns this
 */
proto.backup_manager.DeleteResult.prototype.setOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * optional string message = 3;
 * @return {string}
 */
proto.backup_manager.DeleteResult.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.DeleteResult} returns this
 */
proto.backup_manager.DeleteResult.prototype.setMessage = function(value) {
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
proto.backup_manager.ValidateBackupRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.ValidateBackupRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.ValidateBackupRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ValidateBackupRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
backupId: jspb.Message.getFieldWithDefault(msg, 1, ""),
deep: jspb.Message.getBooleanFieldWithDefault(msg, 2, false)
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
 * @return {!proto.backup_manager.ValidateBackupRequest}
 */
proto.backup_manager.ValidateBackupRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.ValidateBackupRequest;
  return proto.backup_manager.ValidateBackupRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.ValidateBackupRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.ValidateBackupRequest}
 */
proto.backup_manager.ValidateBackupRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setBackupId(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setDeep(value);
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
proto.backup_manager.ValidateBackupRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.ValidateBackupRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.ValidateBackupRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ValidateBackupRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBackupId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getDeep();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
};


/**
 * optional string backup_id = 1;
 * @return {string}
 */
proto.backup_manager.ValidateBackupRequest.prototype.getBackupId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.ValidateBackupRequest} returns this
 */
proto.backup_manager.ValidateBackupRequest.prototype.setBackupId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bool deep = 2;
 * @return {boolean}
 */
proto.backup_manager.ValidateBackupRequest.prototype.getDeep = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.ValidateBackupRequest} returns this
 */
proto.backup_manager.ValidateBackupRequest.prototype.setDeep = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.backup_manager.ValidateBackupResponse.repeatedFields_ = [2,3];



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
proto.backup_manager.ValidateBackupResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.ValidateBackupResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.ValidateBackupResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ValidateBackupResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
valid: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
issuesList: jspb.Message.toObjectList(msg.getIssuesList(),
    proto.backup_manager.ValidationIssue.toObject, includeInstance),
replicationChecksList: jspb.Message.toObjectList(msg.getReplicationChecksList(),
    proto.backup_manager.ReplicationValidation.toObject, includeInstance)
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
 * @return {!proto.backup_manager.ValidateBackupResponse}
 */
proto.backup_manager.ValidateBackupResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.ValidateBackupResponse;
  return proto.backup_manager.ValidateBackupResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.ValidateBackupResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.ValidateBackupResponse}
 */
proto.backup_manager.ValidateBackupResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setValid(value);
      break;
    case 2:
      var value = new proto.backup_manager.ValidationIssue;
      reader.readMessage(value,proto.backup_manager.ValidationIssue.deserializeBinaryFromReader);
      msg.addIssues(value);
      break;
    case 3:
      var value = new proto.backup_manager.ReplicationValidation;
      reader.readMessage(value,proto.backup_manager.ReplicationValidation.deserializeBinaryFromReader);
      msg.addReplicationChecks(value);
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
proto.backup_manager.ValidateBackupResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.ValidateBackupResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.ValidateBackupResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ValidateBackupResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getValid();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getIssuesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      2,
      f,
      proto.backup_manager.ValidationIssue.serializeBinaryToWriter
    );
  }
  f = message.getReplicationChecksList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      3,
      f,
      proto.backup_manager.ReplicationValidation.serializeBinaryToWriter
    );
  }
};


/**
 * optional bool valid = 1;
 * @return {boolean}
 */
proto.backup_manager.ValidateBackupResponse.prototype.getValid = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.ValidateBackupResponse} returns this
 */
proto.backup_manager.ValidateBackupResponse.prototype.setValid = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * repeated ValidationIssue issues = 2;
 * @return {!Array<!proto.backup_manager.ValidationIssue>}
 */
proto.backup_manager.ValidateBackupResponse.prototype.getIssuesList = function() {
  return /** @type{!Array<!proto.backup_manager.ValidationIssue>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.ValidationIssue, 2));
};


/**
 * @param {!Array<!proto.backup_manager.ValidationIssue>} value
 * @return {!proto.backup_manager.ValidateBackupResponse} returns this
*/
proto.backup_manager.ValidateBackupResponse.prototype.setIssuesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 2, value);
};


/**
 * @param {!proto.backup_manager.ValidationIssue=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.ValidationIssue}
 */
proto.backup_manager.ValidateBackupResponse.prototype.addIssues = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 2, opt_value, proto.backup_manager.ValidationIssue, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.ValidateBackupResponse} returns this
 */
proto.backup_manager.ValidateBackupResponse.prototype.clearIssuesList = function() {
  return this.setIssuesList([]);
};


/**
 * repeated ReplicationValidation replication_checks = 3;
 * @return {!Array<!proto.backup_manager.ReplicationValidation>}
 */
proto.backup_manager.ValidateBackupResponse.prototype.getReplicationChecksList = function() {
  return /** @type{!Array<!proto.backup_manager.ReplicationValidation>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.ReplicationValidation, 3));
};


/**
 * @param {!Array<!proto.backup_manager.ReplicationValidation>} value
 * @return {!proto.backup_manager.ValidateBackupResponse} returns this
*/
proto.backup_manager.ValidateBackupResponse.prototype.setReplicationChecksList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 3, value);
};


/**
 * @param {!proto.backup_manager.ReplicationValidation=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.ReplicationValidation}
 */
proto.backup_manager.ValidateBackupResponse.prototype.addReplicationChecks = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 3, opt_value, proto.backup_manager.ReplicationValidation, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.ValidateBackupResponse} returns this
 */
proto.backup_manager.ValidateBackupResponse.prototype.clearReplicationChecksList = function() {
  return this.setReplicationChecksList([]);
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
proto.backup_manager.ValidationIssue.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.ValidationIssue.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.ValidationIssue} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ValidationIssue.toObject = function(includeInstance, msg) {
  var f, obj = {
severity: jspb.Message.getFieldWithDefault(msg, 1, 0),
code: jspb.Message.getFieldWithDefault(msg, 2, ""),
message: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.backup_manager.ValidationIssue}
 */
proto.backup_manager.ValidationIssue.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.ValidationIssue;
  return proto.backup_manager.ValidationIssue.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.ValidationIssue} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.ValidationIssue}
 */
proto.backup_manager.ValidationIssue.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.backup_manager.BackupSeverity} */ (reader.readEnum());
      msg.setSeverity(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setCode(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
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
proto.backup_manager.ValidationIssue.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.ValidationIssue.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.ValidationIssue} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ValidationIssue.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSeverity();
  if (f !== 0.0) {
    writer.writeEnum(
      1,
      f
    );
  }
  f = message.getCode();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getMessage();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional BackupSeverity severity = 1;
 * @return {!proto.backup_manager.BackupSeverity}
 */
proto.backup_manager.ValidationIssue.prototype.getSeverity = function() {
  return /** @type {!proto.backup_manager.BackupSeverity} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {!proto.backup_manager.BackupSeverity} value
 * @return {!proto.backup_manager.ValidationIssue} returns this
 */
proto.backup_manager.ValidationIssue.prototype.setSeverity = function(value) {
  return jspb.Message.setProto3EnumField(this, 1, value);
};


/**
 * optional string code = 2;
 * @return {string}
 */
proto.backup_manager.ValidationIssue.prototype.getCode = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.ValidationIssue} returns this
 */
proto.backup_manager.ValidationIssue.prototype.setCode = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string message = 3;
 * @return {string}
 */
proto.backup_manager.ValidationIssue.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.ValidationIssue} returns this
 */
proto.backup_manager.ValidationIssue.prototype.setMessage = function(value) {
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
proto.backup_manager.RestorePlanRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.RestorePlanRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.RestorePlanRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RestorePlanRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
backupId: jspb.Message.getFieldWithDefault(msg, 1, ""),
includeEtcd: jspb.Message.getBooleanFieldWithDefault(msg, 2, false),
includeConfig: jspb.Message.getBooleanFieldWithDefault(msg, 3, false),
includeMinio: jspb.Message.getBooleanFieldWithDefault(msg, 4, false),
includeScylla: jspb.Message.getBooleanFieldWithDefault(msg, 5, false)
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
 * @return {!proto.backup_manager.RestorePlanRequest}
 */
proto.backup_manager.RestorePlanRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.RestorePlanRequest;
  return proto.backup_manager.RestorePlanRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.RestorePlanRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.RestorePlanRequest}
 */
proto.backup_manager.RestorePlanRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setBackupId(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIncludeEtcd(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIncludeConfig(value);
      break;
    case 4:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIncludeMinio(value);
      break;
    case 5:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIncludeScylla(value);
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
proto.backup_manager.RestorePlanRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.RestorePlanRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.RestorePlanRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RestorePlanRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBackupId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIncludeEtcd();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
  f = message.getIncludeConfig();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
  f = message.getIncludeMinio();
  if (f) {
    writer.writeBool(
      4,
      f
    );
  }
  f = message.getIncludeScylla();
  if (f) {
    writer.writeBool(
      5,
      f
    );
  }
};


/**
 * optional string backup_id = 1;
 * @return {string}
 */
proto.backup_manager.RestorePlanRequest.prototype.getBackupId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.RestorePlanRequest} returns this
 */
proto.backup_manager.RestorePlanRequest.prototype.setBackupId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bool include_etcd = 2;
 * @return {boolean}
 */
proto.backup_manager.RestorePlanRequest.prototype.getIncludeEtcd = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.RestorePlanRequest} returns this
 */
proto.backup_manager.RestorePlanRequest.prototype.setIncludeEtcd = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * optional bool include_config = 3;
 * @return {boolean}
 */
proto.backup_manager.RestorePlanRequest.prototype.getIncludeConfig = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.RestorePlanRequest} returns this
 */
proto.backup_manager.RestorePlanRequest.prototype.setIncludeConfig = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * optional bool include_minio = 4;
 * @return {boolean}
 */
proto.backup_manager.RestorePlanRequest.prototype.getIncludeMinio = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 4, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.RestorePlanRequest} returns this
 */
proto.backup_manager.RestorePlanRequest.prototype.setIncludeMinio = function(value) {
  return jspb.Message.setProto3BooleanField(this, 4, value);
};


/**
 * optional bool include_scylla = 5;
 * @return {boolean}
 */
proto.backup_manager.RestorePlanRequest.prototype.getIncludeScylla = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 5, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.RestorePlanRequest} returns this
 */
proto.backup_manager.RestorePlanRequest.prototype.setIncludeScylla = function(value) {
  return jspb.Message.setProto3BooleanField(this, 5, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.backup_manager.RestorePlanResponse.repeatedFields_ = [2,3];



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
proto.backup_manager.RestorePlanResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.RestorePlanResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.RestorePlanResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RestorePlanResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
backupId: jspb.Message.getFieldWithDefault(msg, 1, ""),
stepsList: jspb.Message.toObjectList(msg.getStepsList(),
    proto.backup_manager.RestoreStep.toObject, includeInstance),
warningsList: jspb.Message.toObjectList(msg.getWarningsList(),
    proto.backup_manager.ValidationIssue.toObject, includeInstance),
confirmationToken: jspb.Message.getFieldWithDefault(msg, 4, "")
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
 * @return {!proto.backup_manager.RestorePlanResponse}
 */
proto.backup_manager.RestorePlanResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.RestorePlanResponse;
  return proto.backup_manager.RestorePlanResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.RestorePlanResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.RestorePlanResponse}
 */
proto.backup_manager.RestorePlanResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setBackupId(value);
      break;
    case 2:
      var value = new proto.backup_manager.RestoreStep;
      reader.readMessage(value,proto.backup_manager.RestoreStep.deserializeBinaryFromReader);
      msg.addSteps(value);
      break;
    case 3:
      var value = new proto.backup_manager.ValidationIssue;
      reader.readMessage(value,proto.backup_manager.ValidationIssue.deserializeBinaryFromReader);
      msg.addWarnings(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setConfirmationToken(value);
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
proto.backup_manager.RestorePlanResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.RestorePlanResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.RestorePlanResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RestorePlanResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBackupId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getStepsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      2,
      f,
      proto.backup_manager.RestoreStep.serializeBinaryToWriter
    );
  }
  f = message.getWarningsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      3,
      f,
      proto.backup_manager.ValidationIssue.serializeBinaryToWriter
    );
  }
  f = message.getConfirmationToken();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * optional string backup_id = 1;
 * @return {string}
 */
proto.backup_manager.RestorePlanResponse.prototype.getBackupId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.RestorePlanResponse} returns this
 */
proto.backup_manager.RestorePlanResponse.prototype.setBackupId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * repeated RestoreStep steps = 2;
 * @return {!Array<!proto.backup_manager.RestoreStep>}
 */
proto.backup_manager.RestorePlanResponse.prototype.getStepsList = function() {
  return /** @type{!Array<!proto.backup_manager.RestoreStep>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.RestoreStep, 2));
};


/**
 * @param {!Array<!proto.backup_manager.RestoreStep>} value
 * @return {!proto.backup_manager.RestorePlanResponse} returns this
*/
proto.backup_manager.RestorePlanResponse.prototype.setStepsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 2, value);
};


/**
 * @param {!proto.backup_manager.RestoreStep=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.RestoreStep}
 */
proto.backup_manager.RestorePlanResponse.prototype.addSteps = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 2, opt_value, proto.backup_manager.RestoreStep, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.RestorePlanResponse} returns this
 */
proto.backup_manager.RestorePlanResponse.prototype.clearStepsList = function() {
  return this.setStepsList([]);
};


/**
 * repeated ValidationIssue warnings = 3;
 * @return {!Array<!proto.backup_manager.ValidationIssue>}
 */
proto.backup_manager.RestorePlanResponse.prototype.getWarningsList = function() {
  return /** @type{!Array<!proto.backup_manager.ValidationIssue>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.ValidationIssue, 3));
};


/**
 * @param {!Array<!proto.backup_manager.ValidationIssue>} value
 * @return {!proto.backup_manager.RestorePlanResponse} returns this
*/
proto.backup_manager.RestorePlanResponse.prototype.setWarningsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 3, value);
};


/**
 * @param {!proto.backup_manager.ValidationIssue=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.ValidationIssue}
 */
proto.backup_manager.RestorePlanResponse.prototype.addWarnings = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 3, opt_value, proto.backup_manager.ValidationIssue, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.RestorePlanResponse} returns this
 */
proto.backup_manager.RestorePlanResponse.prototype.clearWarningsList = function() {
  return this.setWarningsList([]);
};


/**
 * optional string confirmation_token = 4;
 * @return {string}
 */
proto.backup_manager.RestorePlanResponse.prototype.getConfirmationToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.RestorePlanResponse} returns this
 */
proto.backup_manager.RestorePlanResponse.prototype.setConfirmationToken = function(value) {
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
proto.backup_manager.RestoreStep.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.RestoreStep.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.RestoreStep} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RestoreStep.toObject = function(includeInstance, msg) {
  var f, obj = {
order: jspb.Message.getFieldWithDefault(msg, 1, 0),
title: jspb.Message.getFieldWithDefault(msg, 2, ""),
details: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.backup_manager.RestoreStep}
 */
proto.backup_manager.RestoreStep.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.RestoreStep;
  return proto.backup_manager.RestoreStep.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.RestoreStep} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.RestoreStep}
 */
proto.backup_manager.RestoreStep.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setOrder(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setTitle(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setDetails(value);
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
proto.backup_manager.RestoreStep.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.RestoreStep.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.RestoreStep} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RestoreStep.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOrder();
  if (f !== 0) {
    writer.writeUint32(
      1,
      f
    );
  }
  f = message.getTitle();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getDetails();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional uint32 order = 1;
 * @return {number}
 */
proto.backup_manager.RestoreStep.prototype.getOrder = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.RestoreStep} returns this
 */
proto.backup_manager.RestoreStep.prototype.setOrder = function(value) {
  return jspb.Message.setProto3IntField(this, 1, value);
};


/**
 * optional string title = 2;
 * @return {string}
 */
proto.backup_manager.RestoreStep.prototype.getTitle = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.RestoreStep} returns this
 */
proto.backup_manager.RestoreStep.prototype.setTitle = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string details = 3;
 * @return {string}
 */
proto.backup_manager.RestoreStep.prototype.getDetails = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.RestoreStep} returns this
 */
proto.backup_manager.RestoreStep.prototype.setDetails = function(value) {
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
proto.backup_manager.RestoreBackupRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.RestoreBackupRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.RestoreBackupRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RestoreBackupRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
backupId: jspb.Message.getFieldWithDefault(msg, 1, ""),
includeEtcd: jspb.Message.getBooleanFieldWithDefault(msg, 2, false),
includeConfig: jspb.Message.getBooleanFieldWithDefault(msg, 3, false),
includeMinio: jspb.Message.getBooleanFieldWithDefault(msg, 4, false),
includeScylla: jspb.Message.getBooleanFieldWithDefault(msg, 5, false),
dryRun: jspb.Message.getBooleanFieldWithDefault(msg, 6, false),
force: jspb.Message.getBooleanFieldWithDefault(msg, 7, false),
confirmationToken: jspb.Message.getFieldWithDefault(msg, 8, ""),
targetNode: jspb.Message.getFieldWithDefault(msg, 9, "")
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
 * @return {!proto.backup_manager.RestoreBackupRequest}
 */
proto.backup_manager.RestoreBackupRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.RestoreBackupRequest;
  return proto.backup_manager.RestoreBackupRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.RestoreBackupRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.RestoreBackupRequest}
 */
proto.backup_manager.RestoreBackupRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setBackupId(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIncludeEtcd(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIncludeConfig(value);
      break;
    case 4:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIncludeMinio(value);
      break;
    case 5:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIncludeScylla(value);
      break;
    case 6:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setDryRun(value);
      break;
    case 7:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setForce(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setConfirmationToken(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setTargetNode(value);
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
proto.backup_manager.RestoreBackupRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.RestoreBackupRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.RestoreBackupRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RestoreBackupRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBackupId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIncludeEtcd();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
  f = message.getIncludeConfig();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
  f = message.getIncludeMinio();
  if (f) {
    writer.writeBool(
      4,
      f
    );
  }
  f = message.getIncludeScylla();
  if (f) {
    writer.writeBool(
      5,
      f
    );
  }
  f = message.getDryRun();
  if (f) {
    writer.writeBool(
      6,
      f
    );
  }
  f = message.getForce();
  if (f) {
    writer.writeBool(
      7,
      f
    );
  }
  f = message.getConfirmationToken();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
  f = message.getTargetNode();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
};


/**
 * optional string backup_id = 1;
 * @return {string}
 */
proto.backup_manager.RestoreBackupRequest.prototype.getBackupId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.RestoreBackupRequest} returns this
 */
proto.backup_manager.RestoreBackupRequest.prototype.setBackupId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bool include_etcd = 2;
 * @return {boolean}
 */
proto.backup_manager.RestoreBackupRequest.prototype.getIncludeEtcd = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.RestoreBackupRequest} returns this
 */
proto.backup_manager.RestoreBackupRequest.prototype.setIncludeEtcd = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * optional bool include_config = 3;
 * @return {boolean}
 */
proto.backup_manager.RestoreBackupRequest.prototype.getIncludeConfig = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.RestoreBackupRequest} returns this
 */
proto.backup_manager.RestoreBackupRequest.prototype.setIncludeConfig = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * optional bool include_minio = 4;
 * @return {boolean}
 */
proto.backup_manager.RestoreBackupRequest.prototype.getIncludeMinio = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 4, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.RestoreBackupRequest} returns this
 */
proto.backup_manager.RestoreBackupRequest.prototype.setIncludeMinio = function(value) {
  return jspb.Message.setProto3BooleanField(this, 4, value);
};


/**
 * optional bool include_scylla = 5;
 * @return {boolean}
 */
proto.backup_manager.RestoreBackupRequest.prototype.getIncludeScylla = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 5, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.RestoreBackupRequest} returns this
 */
proto.backup_manager.RestoreBackupRequest.prototype.setIncludeScylla = function(value) {
  return jspb.Message.setProto3BooleanField(this, 5, value);
};


/**
 * optional bool dry_run = 6;
 * @return {boolean}
 */
proto.backup_manager.RestoreBackupRequest.prototype.getDryRun = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 6, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.RestoreBackupRequest} returns this
 */
proto.backup_manager.RestoreBackupRequest.prototype.setDryRun = function(value) {
  return jspb.Message.setProto3BooleanField(this, 6, value);
};


/**
 * optional bool force = 7;
 * @return {boolean}
 */
proto.backup_manager.RestoreBackupRequest.prototype.getForce = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 7, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.RestoreBackupRequest} returns this
 */
proto.backup_manager.RestoreBackupRequest.prototype.setForce = function(value) {
  return jspb.Message.setProto3BooleanField(this, 7, value);
};


/**
 * optional string confirmation_token = 8;
 * @return {string}
 */
proto.backup_manager.RestoreBackupRequest.prototype.getConfirmationToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.RestoreBackupRequest} returns this
 */
proto.backup_manager.RestoreBackupRequest.prototype.setConfirmationToken = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};


/**
 * optional string target_node = 9;
 * @return {string}
 */
proto.backup_manager.RestoreBackupRequest.prototype.getTargetNode = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.RestoreBackupRequest} returns this
 */
proto.backup_manager.RestoreBackupRequest.prototype.setTargetNode = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.backup_manager.RestoreBackupResponse.repeatedFields_ = [3,4];



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
proto.backup_manager.RestoreBackupResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.RestoreBackupResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.RestoreBackupResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RestoreBackupResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
jobId: jspb.Message.getFieldWithDefault(msg, 1, ""),
dryRun: jspb.Message.getBooleanFieldWithDefault(msg, 2, false),
stepsList: jspb.Message.toObjectList(msg.getStepsList(),
    proto.backup_manager.RestoreStep.toObject, includeInstance),
warningsList: jspb.Message.toObjectList(msg.getWarningsList(),
    proto.backup_manager.ValidationIssue.toObject, includeInstance)
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
 * @return {!proto.backup_manager.RestoreBackupResponse}
 */
proto.backup_manager.RestoreBackupResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.RestoreBackupResponse;
  return proto.backup_manager.RestoreBackupResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.RestoreBackupResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.RestoreBackupResponse}
 */
proto.backup_manager.RestoreBackupResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setJobId(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setDryRun(value);
      break;
    case 3:
      var value = new proto.backup_manager.RestoreStep;
      reader.readMessage(value,proto.backup_manager.RestoreStep.deserializeBinaryFromReader);
      msg.addSteps(value);
      break;
    case 4:
      var value = new proto.backup_manager.ValidationIssue;
      reader.readMessage(value,proto.backup_manager.ValidationIssue.deserializeBinaryFromReader);
      msg.addWarnings(value);
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
proto.backup_manager.RestoreBackupResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.RestoreBackupResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.RestoreBackupResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RestoreBackupResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getJobId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getDryRun();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
  f = message.getStepsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      3,
      f,
      proto.backup_manager.RestoreStep.serializeBinaryToWriter
    );
  }
  f = message.getWarningsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      4,
      f,
      proto.backup_manager.ValidationIssue.serializeBinaryToWriter
    );
  }
};


/**
 * optional string job_id = 1;
 * @return {string}
 */
proto.backup_manager.RestoreBackupResponse.prototype.getJobId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.RestoreBackupResponse} returns this
 */
proto.backup_manager.RestoreBackupResponse.prototype.setJobId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bool dry_run = 2;
 * @return {boolean}
 */
proto.backup_manager.RestoreBackupResponse.prototype.getDryRun = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.RestoreBackupResponse} returns this
 */
proto.backup_manager.RestoreBackupResponse.prototype.setDryRun = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * repeated RestoreStep steps = 3;
 * @return {!Array<!proto.backup_manager.RestoreStep>}
 */
proto.backup_manager.RestoreBackupResponse.prototype.getStepsList = function() {
  return /** @type{!Array<!proto.backup_manager.RestoreStep>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.RestoreStep, 3));
};


/**
 * @param {!Array<!proto.backup_manager.RestoreStep>} value
 * @return {!proto.backup_manager.RestoreBackupResponse} returns this
*/
proto.backup_manager.RestoreBackupResponse.prototype.setStepsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 3, value);
};


/**
 * @param {!proto.backup_manager.RestoreStep=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.RestoreStep}
 */
proto.backup_manager.RestoreBackupResponse.prototype.addSteps = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 3, opt_value, proto.backup_manager.RestoreStep, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.RestoreBackupResponse} returns this
 */
proto.backup_manager.RestoreBackupResponse.prototype.clearStepsList = function() {
  return this.setStepsList([]);
};


/**
 * repeated ValidationIssue warnings = 4;
 * @return {!Array<!proto.backup_manager.ValidationIssue>}
 */
proto.backup_manager.RestoreBackupResponse.prototype.getWarningsList = function() {
  return /** @type{!Array<!proto.backup_manager.ValidationIssue>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.ValidationIssue, 4));
};


/**
 * @param {!Array<!proto.backup_manager.ValidationIssue>} value
 * @return {!proto.backup_manager.RestoreBackupResponse} returns this
*/
proto.backup_manager.RestoreBackupResponse.prototype.setWarningsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 4, value);
};


/**
 * @param {!proto.backup_manager.ValidationIssue=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.ValidationIssue}
 */
proto.backup_manager.RestoreBackupResponse.prototype.addWarnings = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 4, opt_value, proto.backup_manager.ValidationIssue, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.RestoreBackupResponse} returns this
 */
proto.backup_manager.RestoreBackupResponse.prototype.clearWarningsList = function() {
  return this.setWarningsList([]);
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
proto.backup_manager.CancelBackupJobRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.CancelBackupJobRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.CancelBackupJobRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.CancelBackupJobRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
jobId: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.backup_manager.CancelBackupJobRequest}
 */
proto.backup_manager.CancelBackupJobRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.CancelBackupJobRequest;
  return proto.backup_manager.CancelBackupJobRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.CancelBackupJobRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.CancelBackupJobRequest}
 */
proto.backup_manager.CancelBackupJobRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setJobId(value);
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
proto.backup_manager.CancelBackupJobRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.CancelBackupJobRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.CancelBackupJobRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.CancelBackupJobRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getJobId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string job_id = 1;
 * @return {string}
 */
proto.backup_manager.CancelBackupJobRequest.prototype.getJobId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.CancelBackupJobRequest} returns this
 */
proto.backup_manager.CancelBackupJobRequest.prototype.setJobId = function(value) {
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
proto.backup_manager.CancelBackupJobResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.CancelBackupJobResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.CancelBackupJobResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.CancelBackupJobResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
canceled: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
message: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.backup_manager.CancelBackupJobResponse}
 */
proto.backup_manager.CancelBackupJobResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.CancelBackupJobResponse;
  return proto.backup_manager.CancelBackupJobResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.CancelBackupJobResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.CancelBackupJobResponse}
 */
proto.backup_manager.CancelBackupJobResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setCanceled(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
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
proto.backup_manager.CancelBackupJobResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.CancelBackupJobResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.CancelBackupJobResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.CancelBackupJobResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getCanceled();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getMessage();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional bool canceled = 1;
 * @return {boolean}
 */
proto.backup_manager.CancelBackupJobResponse.prototype.getCanceled = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.CancelBackupJobResponse} returns this
 */
proto.backup_manager.CancelBackupJobResponse.prototype.setCanceled = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional string message = 2;
 * @return {string}
 */
proto.backup_manager.CancelBackupJobResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.CancelBackupJobResponse} returns this
 */
proto.backup_manager.CancelBackupJobResponse.prototype.setMessage = function(value) {
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
proto.backup_manager.DeleteBackupJobRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.DeleteBackupJobRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.DeleteBackupJobRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.DeleteBackupJobRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
jobId: jspb.Message.getFieldWithDefault(msg, 1, ""),
deleteArtifacts: jspb.Message.getBooleanFieldWithDefault(msg, 2, false)
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
 * @return {!proto.backup_manager.DeleteBackupJobRequest}
 */
proto.backup_manager.DeleteBackupJobRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.DeleteBackupJobRequest;
  return proto.backup_manager.DeleteBackupJobRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.DeleteBackupJobRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.DeleteBackupJobRequest}
 */
proto.backup_manager.DeleteBackupJobRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setJobId(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setDeleteArtifacts(value);
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
proto.backup_manager.DeleteBackupJobRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.DeleteBackupJobRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.DeleteBackupJobRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.DeleteBackupJobRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getJobId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getDeleteArtifacts();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
};


/**
 * optional string job_id = 1;
 * @return {string}
 */
proto.backup_manager.DeleteBackupJobRequest.prototype.getJobId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.DeleteBackupJobRequest} returns this
 */
proto.backup_manager.DeleteBackupJobRequest.prototype.setJobId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bool delete_artifacts = 2;
 * @return {boolean}
 */
proto.backup_manager.DeleteBackupJobRequest.prototype.getDeleteArtifacts = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.DeleteBackupJobRequest} returns this
 */
proto.backup_manager.DeleteBackupJobRequest.prototype.setDeleteArtifacts = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
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
proto.backup_manager.DeleteBackupJobResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.DeleteBackupJobResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.DeleteBackupJobResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.DeleteBackupJobResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
deleted: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
message: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.backup_manager.DeleteBackupJobResponse}
 */
proto.backup_manager.DeleteBackupJobResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.DeleteBackupJobResponse;
  return proto.backup_manager.DeleteBackupJobResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.DeleteBackupJobResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.DeleteBackupJobResponse}
 */
proto.backup_manager.DeleteBackupJobResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setDeleted(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
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
proto.backup_manager.DeleteBackupJobResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.DeleteBackupJobResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.DeleteBackupJobResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.DeleteBackupJobResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDeleted();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getMessage();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional bool deleted = 1;
 * @return {boolean}
 */
proto.backup_manager.DeleteBackupJobResponse.prototype.getDeleted = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.DeleteBackupJobResponse} returns this
 */
proto.backup_manager.DeleteBackupJobResponse.prototype.setDeleted = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional string message = 2;
 * @return {string}
 */
proto.backup_manager.DeleteBackupJobResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.DeleteBackupJobResponse} returns this
 */
proto.backup_manager.DeleteBackupJobResponse.prototype.setMessage = function(value) {
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
proto.backup_manager.RunRetentionRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.RunRetentionRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.RunRetentionRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RunRetentionRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
dryRun: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.backup_manager.RunRetentionRequest}
 */
proto.backup_manager.RunRetentionRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.RunRetentionRequest;
  return proto.backup_manager.RunRetentionRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.RunRetentionRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.RunRetentionRequest}
 */
proto.backup_manager.RunRetentionRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setDryRun(value);
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
proto.backup_manager.RunRetentionRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.RunRetentionRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.RunRetentionRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RunRetentionRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDryRun();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool dry_run = 1;
 * @return {boolean}
 */
proto.backup_manager.RunRetentionRequest.prototype.getDryRun = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.RunRetentionRequest} returns this
 */
proto.backup_manager.RunRetentionRequest.prototype.setDryRun = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.backup_manager.RunRetentionResponse.repeatedFields_ = [1,2];



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
proto.backup_manager.RunRetentionResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.RunRetentionResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.RunRetentionResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RunRetentionResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
deletedBackupIdsList: (f = jspb.Message.getRepeatedField(msg, 1)) == null ? undefined : f,
keptBackupIdsList: (f = jspb.Message.getRepeatedField(msg, 2)) == null ? undefined : f,
dryRun: jspb.Message.getBooleanFieldWithDefault(msg, 3, false),
message: jspb.Message.getFieldWithDefault(msg, 4, "")
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
 * @return {!proto.backup_manager.RunRetentionResponse}
 */
proto.backup_manager.RunRetentionResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.RunRetentionResponse;
  return proto.backup_manager.RunRetentionResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.RunRetentionResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.RunRetentionResponse}
 */
proto.backup_manager.RunRetentionResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.addDeletedBackupIds(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.addKeptBackupIds(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setDryRun(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
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
proto.backup_manager.RunRetentionResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.RunRetentionResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.RunRetentionResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RunRetentionResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDeletedBackupIdsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      1,
      f
    );
  }
  f = message.getKeptBackupIdsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      2,
      f
    );
  }
  f = message.getDryRun();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
  f = message.getMessage();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * repeated string deleted_backup_ids = 1;
 * @return {!Array<string>}
 */
proto.backup_manager.RunRetentionResponse.prototype.getDeletedBackupIdsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 1));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.backup_manager.RunRetentionResponse} returns this
 */
proto.backup_manager.RunRetentionResponse.prototype.setDeletedBackupIdsList = function(value) {
  return jspb.Message.setField(this, 1, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.RunRetentionResponse} returns this
 */
proto.backup_manager.RunRetentionResponse.prototype.addDeletedBackupIds = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 1, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.RunRetentionResponse} returns this
 */
proto.backup_manager.RunRetentionResponse.prototype.clearDeletedBackupIdsList = function() {
  return this.setDeletedBackupIdsList([]);
};


/**
 * repeated string kept_backup_ids = 2;
 * @return {!Array<string>}
 */
proto.backup_manager.RunRetentionResponse.prototype.getKeptBackupIdsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.backup_manager.RunRetentionResponse} returns this
 */
proto.backup_manager.RunRetentionResponse.prototype.setKeptBackupIdsList = function(value) {
  return jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.RunRetentionResponse} returns this
 */
proto.backup_manager.RunRetentionResponse.prototype.addKeptBackupIds = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.RunRetentionResponse} returns this
 */
proto.backup_manager.RunRetentionResponse.prototype.clearKeptBackupIdsList = function() {
  return this.setKeptBackupIdsList([]);
};


/**
 * optional bool dry_run = 3;
 * @return {boolean}
 */
proto.backup_manager.RunRetentionResponse.prototype.getDryRun = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.RunRetentionResponse} returns this
 */
proto.backup_manager.RunRetentionResponse.prototype.setDryRun = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * optional string message = 4;
 * @return {string}
 */
proto.backup_manager.RunRetentionResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.RunRetentionResponse} returns this
 */
proto.backup_manager.RunRetentionResponse.prototype.setMessage = function(value) {
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
proto.backup_manager.GetRetentionStatusRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.GetRetentionStatusRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.GetRetentionStatusRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.GetRetentionStatusRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.backup_manager.GetRetentionStatusRequest}
 */
proto.backup_manager.GetRetentionStatusRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.GetRetentionStatusRequest;
  return proto.backup_manager.GetRetentionStatusRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.GetRetentionStatusRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.GetRetentionStatusRequest}
 */
proto.backup_manager.GetRetentionStatusRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.backup_manager.GetRetentionStatusRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.GetRetentionStatusRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.GetRetentionStatusRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.GetRetentionStatusRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.backup_manager.GetRetentionStatusResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.GetRetentionStatusResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.GetRetentionStatusResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.GetRetentionStatusResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
policy: (f = msg.getPolicy()) && proto.backup_manager.RetentionPolicy.toObject(includeInstance, f),
currentBackupCount: jspb.Message.getFieldWithDefault(msg, 2, 0),
currentTotalBytes: jspb.Message.getFieldWithDefault(msg, 3, 0),
oldestBackupUnixMs: jspb.Message.getFieldWithDefault(msg, 4, 0),
newestBackupUnixMs: jspb.Message.getFieldWithDefault(msg, 5, 0)
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
 * @return {!proto.backup_manager.GetRetentionStatusResponse}
 */
proto.backup_manager.GetRetentionStatusResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.GetRetentionStatusResponse;
  return proto.backup_manager.GetRetentionStatusResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.GetRetentionStatusResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.GetRetentionStatusResponse}
 */
proto.backup_manager.GetRetentionStatusResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.backup_manager.RetentionPolicy;
      reader.readMessage(value,proto.backup_manager.RetentionPolicy.deserializeBinaryFromReader);
      msg.setPolicy(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setCurrentBackupCount(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readUint64());
      msg.setCurrentTotalBytes(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setOldestBackupUnixMs(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setNewestBackupUnixMs(value);
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
proto.backup_manager.GetRetentionStatusResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.GetRetentionStatusResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.GetRetentionStatusResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.GetRetentionStatusResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPolicy();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.backup_manager.RetentionPolicy.serializeBinaryToWriter
    );
  }
  f = message.getCurrentBackupCount();
  if (f !== 0) {
    writer.writeUint32(
      2,
      f
    );
  }
  f = message.getCurrentTotalBytes();
  if (f !== 0) {
    writer.writeUint64(
      3,
      f
    );
  }
  f = message.getOldestBackupUnixMs();
  if (f !== 0) {
    writer.writeInt64(
      4,
      f
    );
  }
  f = message.getNewestBackupUnixMs();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
};


/**
 * optional RetentionPolicy policy = 1;
 * @return {?proto.backup_manager.RetentionPolicy}
 */
proto.backup_manager.GetRetentionStatusResponse.prototype.getPolicy = function() {
  return /** @type{?proto.backup_manager.RetentionPolicy} */ (
    jspb.Message.getWrapperField(this, proto.backup_manager.RetentionPolicy, 1));
};


/**
 * @param {?proto.backup_manager.RetentionPolicy|undefined} value
 * @return {!proto.backup_manager.GetRetentionStatusResponse} returns this
*/
proto.backup_manager.GetRetentionStatusResponse.prototype.setPolicy = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.backup_manager.GetRetentionStatusResponse} returns this
 */
proto.backup_manager.GetRetentionStatusResponse.prototype.clearPolicy = function() {
  return this.setPolicy(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.backup_manager.GetRetentionStatusResponse.prototype.hasPolicy = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional uint32 current_backup_count = 2;
 * @return {number}
 */
proto.backup_manager.GetRetentionStatusResponse.prototype.getCurrentBackupCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.GetRetentionStatusResponse} returns this
 */
proto.backup_manager.GetRetentionStatusResponse.prototype.setCurrentBackupCount = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional uint64 current_total_bytes = 3;
 * @return {number}
 */
proto.backup_manager.GetRetentionStatusResponse.prototype.getCurrentTotalBytes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.GetRetentionStatusResponse} returns this
 */
proto.backup_manager.GetRetentionStatusResponse.prototype.setCurrentTotalBytes = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional int64 oldest_backup_unix_ms = 4;
 * @return {number}
 */
proto.backup_manager.GetRetentionStatusResponse.prototype.getOldestBackupUnixMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.GetRetentionStatusResponse} returns this
 */
proto.backup_manager.GetRetentionStatusResponse.prototype.setOldestBackupUnixMs = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional int64 newest_backup_unix_ms = 5;
 * @return {number}
 */
proto.backup_manager.GetRetentionStatusResponse.prototype.getNewestBackupUnixMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.GetRetentionStatusResponse} returns this
 */
proto.backup_manager.GetRetentionStatusResponse.prototype.setNewestBackupUnixMs = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
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
proto.backup_manager.PreflightCheckRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.PreflightCheckRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.PreflightCheckRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.PreflightCheckRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.backup_manager.PreflightCheckRequest}
 */
proto.backup_manager.PreflightCheckRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.PreflightCheckRequest;
  return proto.backup_manager.PreflightCheckRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.PreflightCheckRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.PreflightCheckRequest}
 */
proto.backup_manager.PreflightCheckRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.backup_manager.PreflightCheckRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.PreflightCheckRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.PreflightCheckRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.PreflightCheckRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.backup_manager.PreflightCheckResponse.repeatedFields_ = [1];



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
proto.backup_manager.PreflightCheckResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.PreflightCheckResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.PreflightCheckResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.PreflightCheckResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
toolsList: jspb.Message.toObjectList(msg.getToolsList(),
    proto.backup_manager.ToolCheck.toObject, includeInstance),
allOk: jspb.Message.getBooleanFieldWithDefault(msg, 2, false)
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
 * @return {!proto.backup_manager.PreflightCheckResponse}
 */
proto.backup_manager.PreflightCheckResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.PreflightCheckResponse;
  return proto.backup_manager.PreflightCheckResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.PreflightCheckResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.PreflightCheckResponse}
 */
proto.backup_manager.PreflightCheckResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.backup_manager.ToolCheck;
      reader.readMessage(value,proto.backup_manager.ToolCheck.deserializeBinaryFromReader);
      msg.addTools(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setAllOk(value);
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
proto.backup_manager.PreflightCheckResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.PreflightCheckResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.PreflightCheckResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.PreflightCheckResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getToolsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.backup_manager.ToolCheck.serializeBinaryToWriter
    );
  }
  f = message.getAllOk();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
};


/**
 * repeated ToolCheck tools = 1;
 * @return {!Array<!proto.backup_manager.ToolCheck>}
 */
proto.backup_manager.PreflightCheckResponse.prototype.getToolsList = function() {
  return /** @type{!Array<!proto.backup_manager.ToolCheck>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.ToolCheck, 1));
};


/**
 * @param {!Array<!proto.backup_manager.ToolCheck>} value
 * @return {!proto.backup_manager.PreflightCheckResponse} returns this
*/
proto.backup_manager.PreflightCheckResponse.prototype.setToolsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.backup_manager.ToolCheck=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.ToolCheck}
 */
proto.backup_manager.PreflightCheckResponse.prototype.addTools = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.backup_manager.ToolCheck, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.PreflightCheckResponse} returns this
 */
proto.backup_manager.PreflightCheckResponse.prototype.clearToolsList = function() {
  return this.setToolsList([]);
};


/**
 * optional bool all_ok = 2;
 * @return {boolean}
 */
proto.backup_manager.PreflightCheckResponse.prototype.getAllOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.PreflightCheckResponse} returns this
 */
proto.backup_manager.PreflightCheckResponse.prototype.setAllOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
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
proto.backup_manager.ToolCheck.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.ToolCheck.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.ToolCheck} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ToolCheck.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, ""),
available: jspb.Message.getBooleanFieldWithDefault(msg, 2, false),
version: jspb.Message.getFieldWithDefault(msg, 3, ""),
path: jspb.Message.getFieldWithDefault(msg, 4, ""),
errorMessage: jspb.Message.getFieldWithDefault(msg, 5, "")
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
 * @return {!proto.backup_manager.ToolCheck}
 */
proto.backup_manager.ToolCheck.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.ToolCheck;
  return proto.backup_manager.ToolCheck.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.ToolCheck} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.ToolCheck}
 */
proto.backup_manager.ToolCheck.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setAvailable(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setVersion(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setPath(value);
      break;
    case 5:
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
proto.backup_manager.ToolCheck.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.ToolCheck.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.ToolCheck} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ToolCheck.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getAvailable();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
  f = message.getVersion();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getPath();
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
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.backup_manager.ToolCheck.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.ToolCheck} returns this
 */
proto.backup_manager.ToolCheck.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bool available = 2;
 * @return {boolean}
 */
proto.backup_manager.ToolCheck.prototype.getAvailable = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.ToolCheck} returns this
 */
proto.backup_manager.ToolCheck.prototype.setAvailable = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * optional string version = 3;
 * @return {string}
 */
proto.backup_manager.ToolCheck.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.ToolCheck} returns this
 */
proto.backup_manager.ToolCheck.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string path = 4;
 * @return {string}
 */
proto.backup_manager.ToolCheck.prototype.getPath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.ToolCheck} returns this
 */
proto.backup_manager.ToolCheck.prototype.setPath = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string error_message = 5;
 * @return {string}
 */
proto.backup_manager.ToolCheck.prototype.getErrorMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.ToolCheck} returns this
 */
proto.backup_manager.ToolCheck.prototype.setErrorMessage = function(value) {
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
proto.backup_manager.SkippedProvider.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.SkippedProvider.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.SkippedProvider} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.SkippedProvider.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, ""),
reason: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.backup_manager.SkippedProvider}
 */
proto.backup_manager.SkippedProvider.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.SkippedProvider;
  return proto.backup_manager.SkippedProvider.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.SkippedProvider} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.SkippedProvider}
 */
proto.backup_manager.SkippedProvider.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setReason(value);
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
proto.backup_manager.SkippedProvider.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.SkippedProvider.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.SkippedProvider} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.SkippedProvider.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getReason();
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
proto.backup_manager.SkippedProvider.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.SkippedProvider} returns this
 */
proto.backup_manager.SkippedProvider.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string reason = 2;
 * @return {string}
 */
proto.backup_manager.SkippedProvider.prototype.getReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.SkippedProvider} returns this
 */
proto.backup_manager.SkippedProvider.prototype.setReason = function(value) {
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
proto.backup_manager.RunRestoreTestRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.RunRestoreTestRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.RunRestoreTestRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RunRestoreTestRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
backupId: jspb.Message.getFieldWithDefault(msg, 1, ""),
level: jspb.Message.getFieldWithDefault(msg, 2, 0),
targetRoot: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.backup_manager.RunRestoreTestRequest}
 */
proto.backup_manager.RunRestoreTestRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.RunRestoreTestRequest;
  return proto.backup_manager.RunRestoreTestRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.RunRestoreTestRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.RunRestoreTestRequest}
 */
proto.backup_manager.RunRestoreTestRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setBackupId(value);
      break;
    case 2:
      var value = /** @type {!proto.backup_manager.RestoreTestLevel} */ (reader.readEnum());
      msg.setLevel(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setTargetRoot(value);
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
proto.backup_manager.RunRestoreTestRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.RunRestoreTestRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.RunRestoreTestRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RunRestoreTestRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBackupId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getLevel();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
  f = message.getTargetRoot();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string backup_id = 1;
 * @return {string}
 */
proto.backup_manager.RunRestoreTestRequest.prototype.getBackupId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.RunRestoreTestRequest} returns this
 */
proto.backup_manager.RunRestoreTestRequest.prototype.setBackupId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional RestoreTestLevel level = 2;
 * @return {!proto.backup_manager.RestoreTestLevel}
 */
proto.backup_manager.RunRestoreTestRequest.prototype.getLevel = function() {
  return /** @type {!proto.backup_manager.RestoreTestLevel} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.backup_manager.RestoreTestLevel} value
 * @return {!proto.backup_manager.RunRestoreTestRequest} returns this
 */
proto.backup_manager.RunRestoreTestRequest.prototype.setLevel = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional string target_root = 3;
 * @return {string}
 */
proto.backup_manager.RunRestoreTestRequest.prototype.getTargetRoot = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.RunRestoreTestRequest} returns this
 */
proto.backup_manager.RunRestoreTestRequest.prototype.setTargetRoot = function(value) {
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
proto.backup_manager.RunRestoreTestResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.RunRestoreTestResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.RunRestoreTestResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RunRestoreTestResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
jobId: jspb.Message.getFieldWithDefault(msg, 1, ""),
backupId: jspb.Message.getFieldWithDefault(msg, 2, ""),
level: jspb.Message.getFieldWithDefault(msg, 3, 0)
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
 * @return {!proto.backup_manager.RunRestoreTestResponse}
 */
proto.backup_manager.RunRestoreTestResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.RunRestoreTestResponse;
  return proto.backup_manager.RunRestoreTestResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.RunRestoreTestResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.RunRestoreTestResponse}
 */
proto.backup_manager.RunRestoreTestResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setJobId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setBackupId(value);
      break;
    case 3:
      var value = /** @type {!proto.backup_manager.RestoreTestLevel} */ (reader.readEnum());
      msg.setLevel(value);
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
proto.backup_manager.RunRestoreTestResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.RunRestoreTestResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.RunRestoreTestResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RunRestoreTestResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getJobId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getBackupId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getLevel();
  if (f !== 0.0) {
    writer.writeEnum(
      3,
      f
    );
  }
};


/**
 * optional string job_id = 1;
 * @return {string}
 */
proto.backup_manager.RunRestoreTestResponse.prototype.getJobId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.RunRestoreTestResponse} returns this
 */
proto.backup_manager.RunRestoreTestResponse.prototype.setJobId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string backup_id = 2;
 * @return {string}
 */
proto.backup_manager.RunRestoreTestResponse.prototype.getBackupId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.RunRestoreTestResponse} returns this
 */
proto.backup_manager.RunRestoreTestResponse.prototype.setBackupId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional RestoreTestLevel level = 3;
 * @return {!proto.backup_manager.RestoreTestLevel}
 */
proto.backup_manager.RunRestoreTestResponse.prototype.getLevel = function() {
  return /** @type {!proto.backup_manager.RestoreTestLevel} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.backup_manager.RestoreTestLevel} value
 * @return {!proto.backup_manager.RunRestoreTestResponse} returns this
 */
proto.backup_manager.RunRestoreTestResponse.prototype.setLevel = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.backup_manager.RestoreTestReport.repeatedFields_ = [4];



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
proto.backup_manager.RestoreTestReport.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.RestoreTestReport.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.RestoreTestReport} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RestoreTestReport.toObject = function(includeInstance, msg) {
  var f, obj = {
backupId: jspb.Message.getFieldWithDefault(msg, 1, ""),
level: jspb.Message.getFieldWithDefault(msg, 2, 0),
passed: jspb.Message.getBooleanFieldWithDefault(msg, 3, false),
checksList: jspb.Message.toObjectList(msg.getChecksList(),
    proto.backup_manager.RestoreTestCheck.toObject, includeInstance),
startedUnixMs: jspb.Message.getFieldWithDefault(msg, 5, 0),
finishedUnixMs: jspb.Message.getFieldWithDefault(msg, 6, 0)
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
 * @return {!proto.backup_manager.RestoreTestReport}
 */
proto.backup_manager.RestoreTestReport.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.RestoreTestReport;
  return proto.backup_manager.RestoreTestReport.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.RestoreTestReport} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.RestoreTestReport}
 */
proto.backup_manager.RestoreTestReport.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setBackupId(value);
      break;
    case 2:
      var value = /** @type {!proto.backup_manager.RestoreTestLevel} */ (reader.readEnum());
      msg.setLevel(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setPassed(value);
      break;
    case 4:
      var value = new proto.backup_manager.RestoreTestCheck;
      reader.readMessage(value,proto.backup_manager.RestoreTestCheck.deserializeBinaryFromReader);
      msg.addChecks(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setStartedUnixMs(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setFinishedUnixMs(value);
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
proto.backup_manager.RestoreTestReport.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.RestoreTestReport.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.RestoreTestReport} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RestoreTestReport.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBackupId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getLevel();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
  f = message.getPassed();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
  f = message.getChecksList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      4,
      f,
      proto.backup_manager.RestoreTestCheck.serializeBinaryToWriter
    );
  }
  f = message.getStartedUnixMs();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
  f = message.getFinishedUnixMs();
  if (f !== 0) {
    writer.writeInt64(
      6,
      f
    );
  }
};


/**
 * optional string backup_id = 1;
 * @return {string}
 */
proto.backup_manager.RestoreTestReport.prototype.getBackupId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.RestoreTestReport} returns this
 */
proto.backup_manager.RestoreTestReport.prototype.setBackupId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional RestoreTestLevel level = 2;
 * @return {!proto.backup_manager.RestoreTestLevel}
 */
proto.backup_manager.RestoreTestReport.prototype.getLevel = function() {
  return /** @type {!proto.backup_manager.RestoreTestLevel} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.backup_manager.RestoreTestLevel} value
 * @return {!proto.backup_manager.RestoreTestReport} returns this
 */
proto.backup_manager.RestoreTestReport.prototype.setLevel = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional bool passed = 3;
 * @return {boolean}
 */
proto.backup_manager.RestoreTestReport.prototype.getPassed = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.RestoreTestReport} returns this
 */
proto.backup_manager.RestoreTestReport.prototype.setPassed = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * repeated RestoreTestCheck checks = 4;
 * @return {!Array<!proto.backup_manager.RestoreTestCheck>}
 */
proto.backup_manager.RestoreTestReport.prototype.getChecksList = function() {
  return /** @type{!Array<!proto.backup_manager.RestoreTestCheck>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.RestoreTestCheck, 4));
};


/**
 * @param {!Array<!proto.backup_manager.RestoreTestCheck>} value
 * @return {!proto.backup_manager.RestoreTestReport} returns this
*/
proto.backup_manager.RestoreTestReport.prototype.setChecksList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 4, value);
};


/**
 * @param {!proto.backup_manager.RestoreTestCheck=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.RestoreTestCheck}
 */
proto.backup_manager.RestoreTestReport.prototype.addChecks = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 4, opt_value, proto.backup_manager.RestoreTestCheck, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.RestoreTestReport} returns this
 */
proto.backup_manager.RestoreTestReport.prototype.clearChecksList = function() {
  return this.setChecksList([]);
};


/**
 * optional int64 started_unix_ms = 5;
 * @return {number}
 */
proto.backup_manager.RestoreTestReport.prototype.getStartedUnixMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.RestoreTestReport} returns this
 */
proto.backup_manager.RestoreTestReport.prototype.setStartedUnixMs = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional int64 finished_unix_ms = 6;
 * @return {number}
 */
proto.backup_manager.RestoreTestReport.prototype.getFinishedUnixMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.RestoreTestReport} returns this
 */
proto.backup_manager.RestoreTestReport.prototype.setFinishedUnixMs = function(value) {
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
proto.backup_manager.RestoreTestCheck.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.RestoreTestCheck.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.RestoreTestCheck} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RestoreTestCheck.toObject = function(includeInstance, msg) {
  var f, obj = {
provider: jspb.Message.getFieldWithDefault(msg, 1, ""),
ok: jspb.Message.getBooleanFieldWithDefault(msg, 2, false),
summary: jspb.Message.getFieldWithDefault(msg, 3, ""),
errorMessage: jspb.Message.getFieldWithDefault(msg, 4, "")
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
 * @return {!proto.backup_manager.RestoreTestCheck}
 */
proto.backup_manager.RestoreTestCheck.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.RestoreTestCheck;
  return proto.backup_manager.RestoreTestCheck.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.RestoreTestCheck} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.RestoreTestCheck}
 */
proto.backup_manager.RestoreTestCheck.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setProvider(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setOk(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setSummary(value);
      break;
    case 4:
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
proto.backup_manager.RestoreTestCheck.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.RestoreTestCheck.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.RestoreTestCheck} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.RestoreTestCheck.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getProvider();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOk();
  if (f) {
    writer.writeBool(
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
  f = message.getErrorMessage();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * optional string provider = 1;
 * @return {string}
 */
proto.backup_manager.RestoreTestCheck.prototype.getProvider = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.RestoreTestCheck} returns this
 */
proto.backup_manager.RestoreTestCheck.prototype.setProvider = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bool ok = 2;
 * @return {boolean}
 */
proto.backup_manager.RestoreTestCheck.prototype.getOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.RestoreTestCheck} returns this
 */
proto.backup_manager.RestoreTestCheck.prototype.setOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * optional string summary = 3;
 * @return {string}
 */
proto.backup_manager.RestoreTestCheck.prototype.getSummary = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.RestoreTestCheck} returns this
 */
proto.backup_manager.RestoreTestCheck.prototype.setSummary = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string error_message = 4;
 * @return {string}
 */
proto.backup_manager.RestoreTestCheck.prototype.getErrorMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.RestoreTestCheck} returns this
 */
proto.backup_manager.RestoreTestCheck.prototype.setErrorMessage = function(value) {
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
proto.backup_manager.PromoteBackupRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.PromoteBackupRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.PromoteBackupRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.PromoteBackupRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
backupId: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.backup_manager.PromoteBackupRequest}
 */
proto.backup_manager.PromoteBackupRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.PromoteBackupRequest;
  return proto.backup_manager.PromoteBackupRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.PromoteBackupRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.PromoteBackupRequest}
 */
proto.backup_manager.PromoteBackupRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setBackupId(value);
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
proto.backup_manager.PromoteBackupRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.PromoteBackupRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.PromoteBackupRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.PromoteBackupRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBackupId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string backup_id = 1;
 * @return {string}
 */
proto.backup_manager.PromoteBackupRequest.prototype.getBackupId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.PromoteBackupRequest} returns this
 */
proto.backup_manager.PromoteBackupRequest.prototype.setBackupId = function(value) {
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
proto.backup_manager.PromoteBackupResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.PromoteBackupResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.PromoteBackupResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.PromoteBackupResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
ok: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
qualityState: jspb.Message.getFieldWithDefault(msg, 2, 0),
message: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.backup_manager.PromoteBackupResponse}
 */
proto.backup_manager.PromoteBackupResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.PromoteBackupResponse;
  return proto.backup_manager.PromoteBackupResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.PromoteBackupResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.PromoteBackupResponse}
 */
proto.backup_manager.PromoteBackupResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setOk(value);
      break;
    case 2:
      var value = /** @type {!proto.backup_manager.QualityState} */ (reader.readEnum());
      msg.setQualityState(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
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
proto.backup_manager.PromoteBackupResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.PromoteBackupResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.PromoteBackupResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.PromoteBackupResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOk();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getQualityState();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
  f = message.getMessage();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional bool ok = 1;
 * @return {boolean}
 */
proto.backup_manager.PromoteBackupResponse.prototype.getOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.PromoteBackupResponse} returns this
 */
proto.backup_manager.PromoteBackupResponse.prototype.setOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional QualityState quality_state = 2;
 * @return {!proto.backup_manager.QualityState}
 */
proto.backup_manager.PromoteBackupResponse.prototype.getQualityState = function() {
  return /** @type {!proto.backup_manager.QualityState} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.backup_manager.QualityState} value
 * @return {!proto.backup_manager.PromoteBackupResponse} returns this
 */
proto.backup_manager.PromoteBackupResponse.prototype.setQualityState = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional string message = 3;
 * @return {string}
 */
proto.backup_manager.PromoteBackupResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.PromoteBackupResponse} returns this
 */
proto.backup_manager.PromoteBackupResponse.prototype.setMessage = function(value) {
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
proto.backup_manager.DemoteBackupRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.DemoteBackupRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.DemoteBackupRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.DemoteBackupRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
backupId: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.backup_manager.DemoteBackupRequest}
 */
proto.backup_manager.DemoteBackupRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.DemoteBackupRequest;
  return proto.backup_manager.DemoteBackupRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.DemoteBackupRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.DemoteBackupRequest}
 */
proto.backup_manager.DemoteBackupRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setBackupId(value);
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
proto.backup_manager.DemoteBackupRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.DemoteBackupRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.DemoteBackupRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.DemoteBackupRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBackupId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string backup_id = 1;
 * @return {string}
 */
proto.backup_manager.DemoteBackupRequest.prototype.getBackupId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.DemoteBackupRequest} returns this
 */
proto.backup_manager.DemoteBackupRequest.prototype.setBackupId = function(value) {
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
proto.backup_manager.DemoteBackupResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.DemoteBackupResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.DemoteBackupResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.DemoteBackupResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
ok: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
qualityState: jspb.Message.getFieldWithDefault(msg, 2, 0),
message: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.backup_manager.DemoteBackupResponse}
 */
proto.backup_manager.DemoteBackupResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.DemoteBackupResponse;
  return proto.backup_manager.DemoteBackupResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.DemoteBackupResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.DemoteBackupResponse}
 */
proto.backup_manager.DemoteBackupResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setOk(value);
      break;
    case 2:
      var value = /** @type {!proto.backup_manager.QualityState} */ (reader.readEnum());
      msg.setQualityState(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
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
proto.backup_manager.DemoteBackupResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.DemoteBackupResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.DemoteBackupResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.DemoteBackupResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOk();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getQualityState();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
  f = message.getMessage();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional bool ok = 1;
 * @return {boolean}
 */
proto.backup_manager.DemoteBackupResponse.prototype.getOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.DemoteBackupResponse} returns this
 */
proto.backup_manager.DemoteBackupResponse.prototype.setOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional QualityState quality_state = 2;
 * @return {!proto.backup_manager.QualityState}
 */
proto.backup_manager.DemoteBackupResponse.prototype.getQualityState = function() {
  return /** @type {!proto.backup_manager.QualityState} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.backup_manager.QualityState} value
 * @return {!proto.backup_manager.DemoteBackupResponse} returns this
 */
proto.backup_manager.DemoteBackupResponse.prototype.setQualityState = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional string message = 3;
 * @return {string}
 */
proto.backup_manager.DemoteBackupResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.DemoteBackupResponse} returns this
 */
proto.backup_manager.DemoteBackupResponse.prototype.setMessage = function(value) {
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
proto.backup_manager.PrepareBackupHookRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.PrepareBackupHookRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.PrepareBackupHookRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.PrepareBackupHookRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
backupId: jspb.Message.getFieldWithDefault(msg, 1, ""),
mode: jspb.Message.getFieldWithDefault(msg, 2, 0),
scope: (f = msg.getScope()) && proto.backup_manager.BackupScope.toObject(includeInstance, f),
labelsMap: (f = msg.getLabelsMap()) ? f.toObject(includeInstance, undefined) : [],
timeoutSeconds: jspb.Message.getFieldWithDefault(msg, 5, 0)
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
 * @return {!proto.backup_manager.PrepareBackupHookRequest}
 */
proto.backup_manager.PrepareBackupHookRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.PrepareBackupHookRequest;
  return proto.backup_manager.PrepareBackupHookRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.PrepareBackupHookRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.PrepareBackupHookRequest}
 */
proto.backup_manager.PrepareBackupHookRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setBackupId(value);
      break;
    case 2:
      var value = /** @type {!proto.backup_manager.BackupMode} */ (reader.readEnum());
      msg.setMode(value);
      break;
    case 3:
      var value = new proto.backup_manager.BackupScope;
      reader.readMessage(value,proto.backup_manager.BackupScope.deserializeBinaryFromReader);
      msg.setScope(value);
      break;
    case 4:
      var value = msg.getLabelsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setTimeoutSeconds(value);
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
proto.backup_manager.PrepareBackupHookRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.PrepareBackupHookRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.PrepareBackupHookRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.PrepareBackupHookRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBackupId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getMode();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
  f = message.getScope();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.backup_manager.BackupScope.serializeBinaryToWriter
    );
  }
  f = message.getLabelsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(4, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
  f = message.getTimeoutSeconds();
  if (f !== 0) {
    writer.writeInt32(
      5,
      f
    );
  }
};


/**
 * optional string backup_id = 1;
 * @return {string}
 */
proto.backup_manager.PrepareBackupHookRequest.prototype.getBackupId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.PrepareBackupHookRequest} returns this
 */
proto.backup_manager.PrepareBackupHookRequest.prototype.setBackupId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional BackupMode mode = 2;
 * @return {!proto.backup_manager.BackupMode}
 */
proto.backup_manager.PrepareBackupHookRequest.prototype.getMode = function() {
  return /** @type {!proto.backup_manager.BackupMode} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.backup_manager.BackupMode} value
 * @return {!proto.backup_manager.PrepareBackupHookRequest} returns this
 */
proto.backup_manager.PrepareBackupHookRequest.prototype.setMode = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional BackupScope scope = 3;
 * @return {?proto.backup_manager.BackupScope}
 */
proto.backup_manager.PrepareBackupHookRequest.prototype.getScope = function() {
  return /** @type{?proto.backup_manager.BackupScope} */ (
    jspb.Message.getWrapperField(this, proto.backup_manager.BackupScope, 3));
};


/**
 * @param {?proto.backup_manager.BackupScope|undefined} value
 * @return {!proto.backup_manager.PrepareBackupHookRequest} returns this
*/
proto.backup_manager.PrepareBackupHookRequest.prototype.setScope = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.backup_manager.PrepareBackupHookRequest} returns this
 */
proto.backup_manager.PrepareBackupHookRequest.prototype.clearScope = function() {
  return this.setScope(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.backup_manager.PrepareBackupHookRequest.prototype.hasScope = function() {
  return jspb.Message.getField(this, 3) != null;
};


/**
 * map<string, string> labels = 4;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.backup_manager.PrepareBackupHookRequest.prototype.getLabelsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 4, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.backup_manager.PrepareBackupHookRequest} returns this
 */
proto.backup_manager.PrepareBackupHookRequest.prototype.clearLabelsMap = function() {
  this.getLabelsMap().clear();
  return this;
};


/**
 * optional int32 timeout_seconds = 5;
 * @return {number}
 */
proto.backup_manager.PrepareBackupHookRequest.prototype.getTimeoutSeconds = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.PrepareBackupHookRequest} returns this
 */
proto.backup_manager.PrepareBackupHookRequest.prototype.setTimeoutSeconds = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
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
proto.backup_manager.PrepareBackupHookResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.PrepareBackupHookResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.PrepareBackupHookResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.PrepareBackupHookResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
ok: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
message: jspb.Message.getFieldWithDefault(msg, 2, ""),
detailsMap: (f = msg.getDetailsMap()) ? f.toObject(includeInstance, undefined) : []
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
 * @return {!proto.backup_manager.PrepareBackupHookResponse}
 */
proto.backup_manager.PrepareBackupHookResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.PrepareBackupHookResponse;
  return proto.backup_manager.PrepareBackupHookResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.PrepareBackupHookResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.PrepareBackupHookResponse}
 */
proto.backup_manager.PrepareBackupHookResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setOk(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
      break;
    case 3:
      var value = msg.getDetailsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
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
proto.backup_manager.PrepareBackupHookResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.PrepareBackupHookResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.PrepareBackupHookResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.PrepareBackupHookResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOk();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getMessage();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getDetailsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(3, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
};


/**
 * optional bool ok = 1;
 * @return {boolean}
 */
proto.backup_manager.PrepareBackupHookResponse.prototype.getOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.PrepareBackupHookResponse} returns this
 */
proto.backup_manager.PrepareBackupHookResponse.prototype.setOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional string message = 2;
 * @return {string}
 */
proto.backup_manager.PrepareBackupHookResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.PrepareBackupHookResponse} returns this
 */
proto.backup_manager.PrepareBackupHookResponse.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * map<string, string> details = 3;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.backup_manager.PrepareBackupHookResponse.prototype.getDetailsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 3, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.backup_manager.PrepareBackupHookResponse} returns this
 */
proto.backup_manager.PrepareBackupHookResponse.prototype.clearDetailsMap = function() {
  this.getDetailsMap().clear();
  return this;
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
proto.backup_manager.FinalizeBackupHookRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.FinalizeBackupHookRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.FinalizeBackupHookRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.FinalizeBackupHookRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
backupId: jspb.Message.getFieldWithDefault(msg, 1, ""),
mode: jspb.Message.getFieldWithDefault(msg, 2, 0),
scope: (f = msg.getScope()) && proto.backup_manager.BackupScope.toObject(includeInstance, f),
labelsMap: (f = msg.getLabelsMap()) ? f.toObject(includeInstance, undefined) : [],
backupSucceeded: jspb.Message.getBooleanFieldWithDefault(msg, 5, false)
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
 * @return {!proto.backup_manager.FinalizeBackupHookRequest}
 */
proto.backup_manager.FinalizeBackupHookRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.FinalizeBackupHookRequest;
  return proto.backup_manager.FinalizeBackupHookRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.FinalizeBackupHookRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.FinalizeBackupHookRequest}
 */
proto.backup_manager.FinalizeBackupHookRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setBackupId(value);
      break;
    case 2:
      var value = /** @type {!proto.backup_manager.BackupMode} */ (reader.readEnum());
      msg.setMode(value);
      break;
    case 3:
      var value = new proto.backup_manager.BackupScope;
      reader.readMessage(value,proto.backup_manager.BackupScope.deserializeBinaryFromReader);
      msg.setScope(value);
      break;
    case 4:
      var value = msg.getLabelsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
      break;
    case 5:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setBackupSucceeded(value);
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
proto.backup_manager.FinalizeBackupHookRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.FinalizeBackupHookRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.FinalizeBackupHookRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.FinalizeBackupHookRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBackupId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getMode();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
  f = message.getScope();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.backup_manager.BackupScope.serializeBinaryToWriter
    );
  }
  f = message.getLabelsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(4, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
  f = message.getBackupSucceeded();
  if (f) {
    writer.writeBool(
      5,
      f
    );
  }
};


/**
 * optional string backup_id = 1;
 * @return {string}
 */
proto.backup_manager.FinalizeBackupHookRequest.prototype.getBackupId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.FinalizeBackupHookRequest} returns this
 */
proto.backup_manager.FinalizeBackupHookRequest.prototype.setBackupId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional BackupMode mode = 2;
 * @return {!proto.backup_manager.BackupMode}
 */
proto.backup_manager.FinalizeBackupHookRequest.prototype.getMode = function() {
  return /** @type {!proto.backup_manager.BackupMode} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.backup_manager.BackupMode} value
 * @return {!proto.backup_manager.FinalizeBackupHookRequest} returns this
 */
proto.backup_manager.FinalizeBackupHookRequest.prototype.setMode = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional BackupScope scope = 3;
 * @return {?proto.backup_manager.BackupScope}
 */
proto.backup_manager.FinalizeBackupHookRequest.prototype.getScope = function() {
  return /** @type{?proto.backup_manager.BackupScope} */ (
    jspb.Message.getWrapperField(this, proto.backup_manager.BackupScope, 3));
};


/**
 * @param {?proto.backup_manager.BackupScope|undefined} value
 * @return {!proto.backup_manager.FinalizeBackupHookRequest} returns this
*/
proto.backup_manager.FinalizeBackupHookRequest.prototype.setScope = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.backup_manager.FinalizeBackupHookRequest} returns this
 */
proto.backup_manager.FinalizeBackupHookRequest.prototype.clearScope = function() {
  return this.setScope(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.backup_manager.FinalizeBackupHookRequest.prototype.hasScope = function() {
  return jspb.Message.getField(this, 3) != null;
};


/**
 * map<string, string> labels = 4;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.backup_manager.FinalizeBackupHookRequest.prototype.getLabelsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 4, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.backup_manager.FinalizeBackupHookRequest} returns this
 */
proto.backup_manager.FinalizeBackupHookRequest.prototype.clearLabelsMap = function() {
  this.getLabelsMap().clear();
  return this;
};


/**
 * optional bool backup_succeeded = 5;
 * @return {boolean}
 */
proto.backup_manager.FinalizeBackupHookRequest.prototype.getBackupSucceeded = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 5, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.FinalizeBackupHookRequest} returns this
 */
proto.backup_manager.FinalizeBackupHookRequest.prototype.setBackupSucceeded = function(value) {
  return jspb.Message.setProto3BooleanField(this, 5, value);
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
proto.backup_manager.FinalizeBackupHookResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.FinalizeBackupHookResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.FinalizeBackupHookResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.FinalizeBackupHookResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
ok: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
message: jspb.Message.getFieldWithDefault(msg, 2, ""),
detailsMap: (f = msg.getDetailsMap()) ? f.toObject(includeInstance, undefined) : []
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
 * @return {!proto.backup_manager.FinalizeBackupHookResponse}
 */
proto.backup_manager.FinalizeBackupHookResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.FinalizeBackupHookResponse;
  return proto.backup_manager.FinalizeBackupHookResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.FinalizeBackupHookResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.FinalizeBackupHookResponse}
 */
proto.backup_manager.FinalizeBackupHookResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setOk(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
      break;
    case 3:
      var value = msg.getDetailsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
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
proto.backup_manager.FinalizeBackupHookResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.FinalizeBackupHookResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.FinalizeBackupHookResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.FinalizeBackupHookResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOk();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getMessage();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getDetailsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(3, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
};


/**
 * optional bool ok = 1;
 * @return {boolean}
 */
proto.backup_manager.FinalizeBackupHookResponse.prototype.getOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.FinalizeBackupHookResponse} returns this
 */
proto.backup_manager.FinalizeBackupHookResponse.prototype.setOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional string message = 2;
 * @return {string}
 */
proto.backup_manager.FinalizeBackupHookResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.FinalizeBackupHookResponse} returns this
 */
proto.backup_manager.FinalizeBackupHookResponse.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * map<string, string> details = 3;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.backup_manager.FinalizeBackupHookResponse.prototype.getDetailsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 3, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.backup_manager.FinalizeBackupHookResponse} returns this
 */
proto.backup_manager.FinalizeBackupHookResponse.prototype.clearDetailsMap = function() {
  this.getDetailsMap().clear();
  return this;
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
proto.backup_manager.MinioBucketInfo.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.MinioBucketInfo.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.MinioBucketInfo} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.MinioBucketInfo.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, ""),
creationDate: jspb.Message.getFieldWithDefault(msg, 2, ""),
sizeBytes: jspb.Message.getFieldWithDefault(msg, 3, 0),
objectCount: jspb.Message.getFieldWithDefault(msg, 4, 0)
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
 * @return {!proto.backup_manager.MinioBucketInfo}
 */
proto.backup_manager.MinioBucketInfo.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.MinioBucketInfo;
  return proto.backup_manager.MinioBucketInfo.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.MinioBucketInfo} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.MinioBucketInfo}
 */
proto.backup_manager.MinioBucketInfo.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setCreationDate(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readUint64());
      msg.setSizeBytes(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readUint64());
      msg.setObjectCount(value);
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
proto.backup_manager.MinioBucketInfo.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.MinioBucketInfo.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.MinioBucketInfo} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.MinioBucketInfo.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getCreationDate();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getSizeBytes();
  if (f !== 0) {
    writer.writeUint64(
      3,
      f
    );
  }
  f = message.getObjectCount();
  if (f !== 0) {
    writer.writeUint64(
      4,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.backup_manager.MinioBucketInfo.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.MinioBucketInfo} returns this
 */
proto.backup_manager.MinioBucketInfo.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string creation_date = 2;
 * @return {string}
 */
proto.backup_manager.MinioBucketInfo.prototype.getCreationDate = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.MinioBucketInfo} returns this
 */
proto.backup_manager.MinioBucketInfo.prototype.setCreationDate = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional uint64 size_bytes = 3;
 * @return {number}
 */
proto.backup_manager.MinioBucketInfo.prototype.getSizeBytes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.MinioBucketInfo} returns this
 */
proto.backup_manager.MinioBucketInfo.prototype.setSizeBytes = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional uint64 object_count = 4;
 * @return {number}
 */
proto.backup_manager.MinioBucketInfo.prototype.getObjectCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.backup_manager.MinioBucketInfo} returns this
 */
proto.backup_manager.MinioBucketInfo.prototype.setObjectCount = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
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
proto.backup_manager.ListMinioBucketsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.ListMinioBucketsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.ListMinioBucketsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ListMinioBucketsRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.backup_manager.ListMinioBucketsRequest}
 */
proto.backup_manager.ListMinioBucketsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.ListMinioBucketsRequest;
  return proto.backup_manager.ListMinioBucketsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.ListMinioBucketsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.ListMinioBucketsRequest}
 */
proto.backup_manager.ListMinioBucketsRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.backup_manager.ListMinioBucketsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.ListMinioBucketsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.ListMinioBucketsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ListMinioBucketsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.backup_manager.ListMinioBucketsResponse.repeatedFields_ = [1];



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
proto.backup_manager.ListMinioBucketsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.ListMinioBucketsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.ListMinioBucketsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ListMinioBucketsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
bucketsList: jspb.Message.toObjectList(msg.getBucketsList(),
    proto.backup_manager.MinioBucketInfo.toObject, includeInstance),
endpoint: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.backup_manager.ListMinioBucketsResponse}
 */
proto.backup_manager.ListMinioBucketsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.ListMinioBucketsResponse;
  return proto.backup_manager.ListMinioBucketsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.ListMinioBucketsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.ListMinioBucketsResponse}
 */
proto.backup_manager.ListMinioBucketsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.backup_manager.MinioBucketInfo;
      reader.readMessage(value,proto.backup_manager.MinioBucketInfo.deserializeBinaryFromReader);
      msg.addBuckets(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setEndpoint(value);
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
proto.backup_manager.ListMinioBucketsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.ListMinioBucketsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.ListMinioBucketsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.ListMinioBucketsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBucketsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.backup_manager.MinioBucketInfo.serializeBinaryToWriter
    );
  }
  f = message.getEndpoint();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * repeated MinioBucketInfo buckets = 1;
 * @return {!Array<!proto.backup_manager.MinioBucketInfo>}
 */
proto.backup_manager.ListMinioBucketsResponse.prototype.getBucketsList = function() {
  return /** @type{!Array<!proto.backup_manager.MinioBucketInfo>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.backup_manager.MinioBucketInfo, 1));
};


/**
 * @param {!Array<!proto.backup_manager.MinioBucketInfo>} value
 * @return {!proto.backup_manager.ListMinioBucketsResponse} returns this
*/
proto.backup_manager.ListMinioBucketsResponse.prototype.setBucketsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.backup_manager.MinioBucketInfo=} opt_value
 * @param {number=} opt_index
 * @return {!proto.backup_manager.MinioBucketInfo}
 */
proto.backup_manager.ListMinioBucketsResponse.prototype.addBuckets = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.backup_manager.MinioBucketInfo, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.backup_manager.ListMinioBucketsResponse} returns this
 */
proto.backup_manager.ListMinioBucketsResponse.prototype.clearBucketsList = function() {
  return this.setBucketsList([]);
};


/**
 * optional string endpoint = 2;
 * @return {string}
 */
proto.backup_manager.ListMinioBucketsResponse.prototype.getEndpoint = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.ListMinioBucketsResponse} returns this
 */
proto.backup_manager.ListMinioBucketsResponse.prototype.setEndpoint = function(value) {
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
proto.backup_manager.CreateMinioBucketRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.CreateMinioBucketRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.CreateMinioBucketRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.CreateMinioBucketRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, ""),
setAsBackupDestination: jspb.Message.getBooleanFieldWithDefault(msg, 2, false),
setAsScyllaLocation: jspb.Message.getBooleanFieldWithDefault(msg, 3, false)
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
 * @return {!proto.backup_manager.CreateMinioBucketRequest}
 */
proto.backup_manager.CreateMinioBucketRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.CreateMinioBucketRequest;
  return proto.backup_manager.CreateMinioBucketRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.CreateMinioBucketRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.CreateMinioBucketRequest}
 */
proto.backup_manager.CreateMinioBucketRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setSetAsBackupDestination(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setSetAsScyllaLocation(value);
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
proto.backup_manager.CreateMinioBucketRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.CreateMinioBucketRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.CreateMinioBucketRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.CreateMinioBucketRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getSetAsBackupDestination();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
  f = message.getSetAsScyllaLocation();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.backup_manager.CreateMinioBucketRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.CreateMinioBucketRequest} returns this
 */
proto.backup_manager.CreateMinioBucketRequest.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bool set_as_backup_destination = 2;
 * @return {boolean}
 */
proto.backup_manager.CreateMinioBucketRequest.prototype.getSetAsBackupDestination = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.CreateMinioBucketRequest} returns this
 */
proto.backup_manager.CreateMinioBucketRequest.prototype.setSetAsBackupDestination = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * optional bool set_as_scylla_location = 3;
 * @return {boolean}
 */
proto.backup_manager.CreateMinioBucketRequest.prototype.getSetAsScyllaLocation = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.CreateMinioBucketRequest} returns this
 */
proto.backup_manager.CreateMinioBucketRequest.prototype.setSetAsScyllaLocation = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
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
proto.backup_manager.CreateMinioBucketResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.CreateMinioBucketResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.CreateMinioBucketResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.CreateMinioBucketResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
ok: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
message: jspb.Message.getFieldWithDefault(msg, 2, ""),
bucketName: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.backup_manager.CreateMinioBucketResponse}
 */
proto.backup_manager.CreateMinioBucketResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.CreateMinioBucketResponse;
  return proto.backup_manager.CreateMinioBucketResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.CreateMinioBucketResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.CreateMinioBucketResponse}
 */
proto.backup_manager.CreateMinioBucketResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setOk(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setBucketName(value);
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
proto.backup_manager.CreateMinioBucketResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.CreateMinioBucketResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.CreateMinioBucketResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.CreateMinioBucketResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOk();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getMessage();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getBucketName();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional bool ok = 1;
 * @return {boolean}
 */
proto.backup_manager.CreateMinioBucketResponse.prototype.getOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.CreateMinioBucketResponse} returns this
 */
proto.backup_manager.CreateMinioBucketResponse.prototype.setOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional string message = 2;
 * @return {string}
 */
proto.backup_manager.CreateMinioBucketResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.CreateMinioBucketResponse} returns this
 */
proto.backup_manager.CreateMinioBucketResponse.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string bucket_name = 3;
 * @return {string}
 */
proto.backup_manager.CreateMinioBucketResponse.prototype.getBucketName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.CreateMinioBucketResponse} returns this
 */
proto.backup_manager.CreateMinioBucketResponse.prototype.setBucketName = function(value) {
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
proto.backup_manager.DeleteMinioBucketRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.DeleteMinioBucketRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.DeleteMinioBucketRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.DeleteMinioBucketRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, ""),
force: jspb.Message.getBooleanFieldWithDefault(msg, 2, false)
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
 * @return {!proto.backup_manager.DeleteMinioBucketRequest}
 */
proto.backup_manager.DeleteMinioBucketRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.DeleteMinioBucketRequest;
  return proto.backup_manager.DeleteMinioBucketRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.DeleteMinioBucketRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.DeleteMinioBucketRequest}
 */
proto.backup_manager.DeleteMinioBucketRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setForce(value);
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
proto.backup_manager.DeleteMinioBucketRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.DeleteMinioBucketRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.DeleteMinioBucketRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.DeleteMinioBucketRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getForce();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.backup_manager.DeleteMinioBucketRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.DeleteMinioBucketRequest} returns this
 */
proto.backup_manager.DeleteMinioBucketRequest.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bool force = 2;
 * @return {boolean}
 */
proto.backup_manager.DeleteMinioBucketRequest.prototype.getForce = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.DeleteMinioBucketRequest} returns this
 */
proto.backup_manager.DeleteMinioBucketRequest.prototype.setForce = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
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
proto.backup_manager.DeleteMinioBucketResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.DeleteMinioBucketResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.DeleteMinioBucketResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.DeleteMinioBucketResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
ok: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
message: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.backup_manager.DeleteMinioBucketResponse}
 */
proto.backup_manager.DeleteMinioBucketResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.DeleteMinioBucketResponse;
  return proto.backup_manager.DeleteMinioBucketResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.DeleteMinioBucketResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.DeleteMinioBucketResponse}
 */
proto.backup_manager.DeleteMinioBucketResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setOk(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
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
proto.backup_manager.DeleteMinioBucketResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.DeleteMinioBucketResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.DeleteMinioBucketResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.DeleteMinioBucketResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOk();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getMessage();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional bool ok = 1;
 * @return {boolean}
 */
proto.backup_manager.DeleteMinioBucketResponse.prototype.getOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.backup_manager.DeleteMinioBucketResponse} returns this
 */
proto.backup_manager.DeleteMinioBucketResponse.prototype.setOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional string message = 2;
 * @return {string}
 */
proto.backup_manager.DeleteMinioBucketResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.backup_manager.DeleteMinioBucketResponse} returns this
 */
proto.backup_manager.DeleteMinioBucketResponse.prototype.setMessage = function(value) {
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
proto.backup_manager.StopRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.StopRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.StopRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.StopRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.backup_manager.StopRequest}
 */
proto.backup_manager.StopRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.StopRequest;
  return proto.backup_manager.StopRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.StopRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.StopRequest}
 */
proto.backup_manager.StopRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.backup_manager.StopRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.StopRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.StopRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.StopRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.backup_manager.StopResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.backup_manager.StopResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.backup_manager.StopResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.StopResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.backup_manager.StopResponse}
 */
proto.backup_manager.StopResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.backup_manager.StopResponse;
  return proto.backup_manager.StopResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.backup_manager.StopResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.backup_manager.StopResponse}
 */
proto.backup_manager.StopResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.backup_manager.StopResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.backup_manager.StopResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.backup_manager.StopResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.backup_manager.StopResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};


/**
 * @enum {number}
 */
proto.backup_manager.BackupProviderType = {
  BACKUP_PROVIDER_TYPE_UNSPECIFIED: 0,
  BACKUP_PROVIDER_ETCD: 1,
  BACKUP_PROVIDER_RESTIC: 2,
  BACKUP_PROVIDER_MINIO: 3,
  BACKUP_PROVIDER_SCYLLA: 4
};

/**
 * @enum {number}
 */
proto.backup_manager.BackupJobState = {
  BACKUP_JOB_STATE_UNSPECIFIED: 0,
  BACKUP_JOB_QUEUED: 1,
  BACKUP_JOB_RUNNING: 2,
  BACKUP_JOB_SUCCEEDED: 3,
  BACKUP_JOB_FAILED: 4,
  BACKUP_JOB_CANCELED: 5
};

/**
 * @enum {number}
 */
proto.backup_manager.BackupJobType = {
  BACKUP_JOB_TYPE_UNSPECIFIED: 0,
  BACKUP_JOB_TYPE_BACKUP: 1,
  BACKUP_JOB_TYPE_RESTORE: 2,
  BACKUP_JOB_TYPE_RETENTION: 3
};

/**
 * @enum {number}
 */
proto.backup_manager.BackupSeverity = {
  BACKUP_SEVERITY_UNSPECIFIED: 0,
  BACKUP_SEVERITY_INFO: 1,
  BACKUP_SEVERITY_WARN: 2,
  BACKUP_SEVERITY_ERROR: 3
};

/**
 * @enum {number}
 */
proto.backup_manager.BackupDestinationType = {
  BACKUP_DESTINATION_TYPE_UNSPECIFIED: 0,
  BACKUP_DESTINATION_LOCAL: 1,
  BACKUP_DESTINATION_MINIO: 2,
  BACKUP_DESTINATION_NFS: 3,
  BACKUP_DESTINATION_S3: 4,
  BACKUP_DESTINATION_RCLONE: 5
};

/**
 * @enum {number}
 */
proto.backup_manager.BackupMode = {
  BACKUP_MODE_UNSPECIFIED: 0,
  BACKUP_MODE_SERVICE: 1,
  BACKUP_MODE_CLUSTER: 2
};

/**
 * @enum {number}
 */
proto.backup_manager.QualityState = {
  QUALITY_STATE_UNSPECIFIED: 0,
  QUALITY_UNVERIFIED: 1,
  QUALITY_VALIDATED: 2,
  QUALITY_RESTORE_TESTED: 3,
  QUALITY_PROMOTED: 4
};

/**
 * @enum {number}
 */
proto.backup_manager.RestoreTestLevel = {
  RESTORE_TEST_LEVEL_UNSPECIFIED: 0,
  RESTORE_TEST_LIGHT: 1,
  RESTORE_TEST_HEAVY: 2
};

goog.object.extend(exports, proto.backup_manager);
