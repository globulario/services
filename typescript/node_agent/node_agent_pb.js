// source: node_agent.proto
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
var cluster_controller_pb = require('./cluster_controller_pb.js');
goog.object.extend(proto, cluster_controller_pb);
goog.exportSymbol('proto.node_agent.ApplyPackageReleaseRequest', null, global);
goog.exportSymbol('proto.node_agent.ApplyPackageReleaseResponse', null, global);
goog.exportSymbol('proto.node_agent.BackupProviderResult', null, global);
goog.exportSymbol('proto.node_agent.BackupProviderSpec', null, global);
goog.exportSymbol('proto.node_agent.BootstrapFirstNodeRequest', null, global);
goog.exportSymbol('proto.node_agent.BootstrapFirstNodeResponse', null, global);
goog.exportSymbol('proto.node_agent.CertificateInfo', null, global);
goog.exportSymbol('proto.node_agent.CleanupDiskJournalRequest', null, global);
goog.exportSymbol('proto.node_agent.CleanupDiskJournalResponse', null, global);
goog.exportSymbol('proto.node_agent.CollectBackupSecretsRequest', null, global);
goog.exportSymbol('proto.node_agent.CollectBackupSecretsResponse', null, global);
goog.exportSymbol('proto.node_agent.ControlServiceRequest', null, global);
goog.exportSymbol('proto.node_agent.ControlServiceResponse', null, global);
goog.exportSymbol('proto.node_agent.DeleteCacheArtifactRequest', null, global);
goog.exportSymbol('proto.node_agent.DeleteCacheArtifactResponse', null, global);
goog.exportSymbol('proto.node_agent.GetBackupTaskResultRequest', null, global);
goog.exportSymbol('proto.node_agent.GetBackupTaskResultResponse', null, global);
goog.exportSymbol('proto.node_agent.GetCertificateStatusRequest', null, global);
goog.exportSymbol('proto.node_agent.GetCertificateStatusResponse', null, global);
goog.exportSymbol('proto.node_agent.GetInfraProbeRequest', null, global);
goog.exportSymbol('proto.node_agent.GetInfraProbeResponse', null, global);
goog.exportSymbol('proto.node_agent.GetInstalledPackageRequest', null, global);
goog.exportSymbol('proto.node_agent.GetInstalledPackageResponse', null, global);
goog.exportSymbol('proto.node_agent.GetInventoryRequest', null, global);
goog.exportSymbol('proto.node_agent.GetInventoryResponse', null, global);
goog.exportSymbol('proto.node_agent.GetRestoreTaskResultRequest', null, global);
goog.exportSymbol('proto.node_agent.GetRestoreTaskResultResponse', null, global);
goog.exportSymbol('proto.node_agent.GetServiceLogsRequest', null, global);
goog.exportSymbol('proto.node_agent.GetServiceLogsResponse', null, global);
goog.exportSymbol('proto.node_agent.GetServiceRuntimeProofRequest', null, global);
goog.exportSymbol('proto.node_agent.GetServiceRuntimeProofResponse', null, global);
goog.exportSymbol('proto.node_agent.GetSubsystemHealthRequest', null, global);
goog.exportSymbol('proto.node_agent.GetSubsystemHealthResponse', null, global);
goog.exportSymbol('proto.node_agent.InstalledComponent', null, global);
goog.exportSymbol('proto.node_agent.InstalledPackage', null, global);
goog.exportSymbol('proto.node_agent.Inventory', null, global);
goog.exportSymbol('proto.node_agent.JoinClusterRequest', null, global);
goog.exportSymbol('proto.node_agent.JoinClusterResponse', null, global);
goog.exportSymbol('proto.node_agent.ListInstalledPackagesRequest', null, global);
goog.exportSymbol('proto.node_agent.ListInstalledPackagesResponse', null, global);
goog.exportSymbol('proto.node_agent.OperationEvent', null, global);
goog.exportSymbol('proto.node_agent.RestoreProviderSpec', null, global);
goog.exportSymbol('proto.node_agent.RotateNodeTokenRequest', null, global);
goog.exportSymbol('proto.node_agent.RotateNodeTokenResponse', null, global);
goog.exportSymbol('proto.node_agent.RunBackupProviderRequest', null, global);
goog.exportSymbol('proto.node_agent.RunBackupProviderResponse', null, global);
goog.exportSymbol('proto.node_agent.RunRestoreProviderRequest', null, global);
goog.exportSymbol('proto.node_agent.RunRestoreProviderResponse', null, global);
goog.exportSymbol('proto.node_agent.RunWorkflowRequest', null, global);
goog.exportSymbol('proto.node_agent.RunWorkflowResponse', null, global);
goog.exportSymbol('proto.node_agent.SearchServiceLogsRequest', null, global);
goog.exportSymbol('proto.node_agent.SearchServiceLogsResponse', null, global);
goog.exportSymbol('proto.node_agent.SecretFileEntry', null, global);
goog.exportSymbol('proto.node_agent.ServiceRuntimeProof', null, global);
goog.exportSymbol('proto.node_agent.SetInstalledPackageRequest', null, global);
goog.exportSymbol('proto.node_agent.SetInstalledPackageResponse', null, global);
goog.exportSymbol('proto.node_agent.SubsystemHealth', null, global);
goog.exportSymbol('proto.node_agent.SubsystemState', null, global);
goog.exportSymbol('proto.node_agent.UnitStatus', null, global);
goog.exportSymbol('proto.node_agent.VerifyPackageIntegrityRequest', null, global);
goog.exportSymbol('proto.node_agent.VerifyPackageIntegrityResponse', null, global);
goog.exportSymbol('proto.node_agent.WatchOperationRequest', null, global);
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
proto.node_agent.JoinClusterRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.JoinClusterRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.JoinClusterRequest.displayName = 'proto.node_agent.JoinClusterRequest';
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
proto.node_agent.JoinClusterResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.JoinClusterResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.JoinClusterResponse.displayName = 'proto.node_agent.JoinClusterResponse';
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
proto.node_agent.InstalledComponent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.InstalledComponent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.InstalledComponent.displayName = 'proto.node_agent.InstalledComponent';
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
proto.node_agent.InstalledPackage = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.InstalledPackage, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.InstalledPackage.displayName = 'proto.node_agent.InstalledPackage';
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
proto.node_agent.ListInstalledPackagesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.ListInstalledPackagesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.ListInstalledPackagesRequest.displayName = 'proto.node_agent.ListInstalledPackagesRequest';
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
proto.node_agent.ListInstalledPackagesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.node_agent.ListInstalledPackagesResponse.repeatedFields_, null);
};
goog.inherits(proto.node_agent.ListInstalledPackagesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.ListInstalledPackagesResponse.displayName = 'proto.node_agent.ListInstalledPackagesResponse';
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
proto.node_agent.GetInstalledPackageRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.GetInstalledPackageRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.GetInstalledPackageRequest.displayName = 'proto.node_agent.GetInstalledPackageRequest';
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
proto.node_agent.GetInstalledPackageResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.GetInstalledPackageResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.GetInstalledPackageResponse.displayName = 'proto.node_agent.GetInstalledPackageResponse';
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
proto.node_agent.SetInstalledPackageRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.SetInstalledPackageRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.SetInstalledPackageRequest.displayName = 'proto.node_agent.SetInstalledPackageRequest';
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
proto.node_agent.SetInstalledPackageResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.SetInstalledPackageResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.SetInstalledPackageResponse.displayName = 'proto.node_agent.SetInstalledPackageResponse';
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
proto.node_agent.UnitStatus = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.UnitStatus, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.UnitStatus.displayName = 'proto.node_agent.UnitStatus';
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
proto.node_agent.Inventory = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.node_agent.Inventory.repeatedFields_, null);
};
goog.inherits(proto.node_agent.Inventory, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.Inventory.displayName = 'proto.node_agent.Inventory';
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
proto.node_agent.GetInventoryRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.GetInventoryRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.GetInventoryRequest.displayName = 'proto.node_agent.GetInventoryRequest';
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
proto.node_agent.GetInventoryResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.GetInventoryResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.GetInventoryResponse.displayName = 'proto.node_agent.GetInventoryResponse';
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
proto.node_agent.WatchOperationRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.WatchOperationRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.WatchOperationRequest.displayName = 'proto.node_agent.WatchOperationRequest';
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
proto.node_agent.OperationEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.OperationEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.OperationEvent.displayName = 'proto.node_agent.OperationEvent';
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
proto.node_agent.BootstrapFirstNodeRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.node_agent.BootstrapFirstNodeRequest.repeatedFields_, null);
};
goog.inherits(proto.node_agent.BootstrapFirstNodeRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.BootstrapFirstNodeRequest.displayName = 'proto.node_agent.BootstrapFirstNodeRequest';
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
proto.node_agent.BootstrapFirstNodeResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.BootstrapFirstNodeResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.BootstrapFirstNodeResponse.displayName = 'proto.node_agent.BootstrapFirstNodeResponse';
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
proto.node_agent.BackupProviderSpec = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.BackupProviderSpec, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.BackupProviderSpec.displayName = 'proto.node_agent.BackupProviderSpec';
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
proto.node_agent.RunBackupProviderRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.RunBackupProviderRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.RunBackupProviderRequest.displayName = 'proto.node_agent.RunBackupProviderRequest';
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
proto.node_agent.RunBackupProviderResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.RunBackupProviderResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.RunBackupProviderResponse.displayName = 'proto.node_agent.RunBackupProviderResponse';
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
proto.node_agent.GetBackupTaskResultRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.GetBackupTaskResultRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.GetBackupTaskResultRequest.displayName = 'proto.node_agent.GetBackupTaskResultRequest';
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
proto.node_agent.BackupProviderResult = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.node_agent.BackupProviderResult.repeatedFields_, null);
};
goog.inherits(proto.node_agent.BackupProviderResult, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.BackupProviderResult.displayName = 'proto.node_agent.BackupProviderResult';
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
proto.node_agent.GetBackupTaskResultResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.GetBackupTaskResultResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.GetBackupTaskResultResponse.displayName = 'proto.node_agent.GetBackupTaskResultResponse';
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
proto.node_agent.RestoreProviderSpec = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.RestoreProviderSpec, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.RestoreProviderSpec.displayName = 'proto.node_agent.RestoreProviderSpec';
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
proto.node_agent.RunRestoreProviderRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.RunRestoreProviderRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.RunRestoreProviderRequest.displayName = 'proto.node_agent.RunRestoreProviderRequest';
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
proto.node_agent.RunRestoreProviderResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.RunRestoreProviderResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.RunRestoreProviderResponse.displayName = 'proto.node_agent.RunRestoreProviderResponse';
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
proto.node_agent.GetRestoreTaskResultRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.GetRestoreTaskResultRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.GetRestoreTaskResultRequest.displayName = 'proto.node_agent.GetRestoreTaskResultRequest';
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
proto.node_agent.GetRestoreTaskResultResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.GetRestoreTaskResultResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.GetRestoreTaskResultResponse.displayName = 'proto.node_agent.GetRestoreTaskResultResponse';
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
proto.node_agent.ServiceRuntimeProof = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.node_agent.ServiceRuntimeProof.repeatedFields_, null);
};
goog.inherits(proto.node_agent.ServiceRuntimeProof, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.ServiceRuntimeProof.displayName = 'proto.node_agent.ServiceRuntimeProof';
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
proto.node_agent.GetServiceRuntimeProofRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.GetServiceRuntimeProofRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.GetServiceRuntimeProofRequest.displayName = 'proto.node_agent.GetServiceRuntimeProofRequest';
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
proto.node_agent.GetServiceRuntimeProofResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.node_agent.GetServiceRuntimeProofResponse.repeatedFields_, null);
};
goog.inherits(proto.node_agent.GetServiceRuntimeProofResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.GetServiceRuntimeProofResponse.displayName = 'proto.node_agent.GetServiceRuntimeProofResponse';
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
proto.node_agent.VerifyPackageIntegrityRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.VerifyPackageIntegrityRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.VerifyPackageIntegrityRequest.displayName = 'proto.node_agent.VerifyPackageIntegrityRequest';
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
proto.node_agent.VerifyPackageIntegrityResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.VerifyPackageIntegrityResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.VerifyPackageIntegrityResponse.displayName = 'proto.node_agent.VerifyPackageIntegrityResponse';
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
proto.node_agent.RotateNodeTokenRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.RotateNodeTokenRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.RotateNodeTokenRequest.displayName = 'proto.node_agent.RotateNodeTokenRequest';
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
proto.node_agent.RotateNodeTokenResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.RotateNodeTokenResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.RotateNodeTokenResponse.displayName = 'proto.node_agent.RotateNodeTokenResponse';
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
proto.node_agent.GetServiceLogsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.GetServiceLogsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.GetServiceLogsRequest.displayName = 'proto.node_agent.GetServiceLogsRequest';
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
proto.node_agent.GetServiceLogsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.node_agent.GetServiceLogsResponse.repeatedFields_, null);
};
goog.inherits(proto.node_agent.GetServiceLogsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.GetServiceLogsResponse.displayName = 'proto.node_agent.GetServiceLogsResponse';
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
proto.node_agent.SearchServiceLogsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.SearchServiceLogsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.SearchServiceLogsRequest.displayName = 'proto.node_agent.SearchServiceLogsRequest';
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
proto.node_agent.SearchServiceLogsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.node_agent.SearchServiceLogsResponse.repeatedFields_, null);
};
goog.inherits(proto.node_agent.SearchServiceLogsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.SearchServiceLogsResponse.displayName = 'proto.node_agent.SearchServiceLogsResponse';
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
proto.node_agent.CertificateInfo = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.node_agent.CertificateInfo.repeatedFields_, null);
};
goog.inherits(proto.node_agent.CertificateInfo, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.CertificateInfo.displayName = 'proto.node_agent.CertificateInfo';
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
proto.node_agent.ControlServiceRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.ControlServiceRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.ControlServiceRequest.displayName = 'proto.node_agent.ControlServiceRequest';
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
proto.node_agent.ControlServiceResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.ControlServiceResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.ControlServiceResponse.displayName = 'proto.node_agent.ControlServiceResponse';
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
proto.node_agent.GetCertificateStatusRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.GetCertificateStatusRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.GetCertificateStatusRequest.displayName = 'proto.node_agent.GetCertificateStatusRequest';
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
proto.node_agent.GetCertificateStatusResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.GetCertificateStatusResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.GetCertificateStatusResponse.displayName = 'proto.node_agent.GetCertificateStatusResponse';
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
proto.node_agent.SubsystemHealth = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.SubsystemHealth, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.SubsystemHealth.displayName = 'proto.node_agent.SubsystemHealth';
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
proto.node_agent.GetSubsystemHealthRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.GetSubsystemHealthRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.GetSubsystemHealthRequest.displayName = 'proto.node_agent.GetSubsystemHealthRequest';
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
proto.node_agent.GetSubsystemHealthResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.node_agent.GetSubsystemHealthResponse.repeatedFields_, null);
};
goog.inherits(proto.node_agent.GetSubsystemHealthResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.GetSubsystemHealthResponse.displayName = 'proto.node_agent.GetSubsystemHealthResponse';
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
proto.node_agent.GetInfraProbeRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.GetInfraProbeRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.GetInfraProbeRequest.displayName = 'proto.node_agent.GetInfraProbeRequest';
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
proto.node_agent.GetInfraProbeResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.node_agent.GetInfraProbeResponse.repeatedFields_, null);
};
goog.inherits(proto.node_agent.GetInfraProbeResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.GetInfraProbeResponse.displayName = 'proto.node_agent.GetInfraProbeResponse';
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
proto.node_agent.RunWorkflowRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.RunWorkflowRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.RunWorkflowRequest.displayName = 'proto.node_agent.RunWorkflowRequest';
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
proto.node_agent.RunWorkflowResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.RunWorkflowResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.RunWorkflowResponse.displayName = 'proto.node_agent.RunWorkflowResponse';
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
proto.node_agent.ApplyPackageReleaseRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.ApplyPackageReleaseRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.ApplyPackageReleaseRequest.displayName = 'proto.node_agent.ApplyPackageReleaseRequest';
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
proto.node_agent.ApplyPackageReleaseResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.ApplyPackageReleaseResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.ApplyPackageReleaseResponse.displayName = 'proto.node_agent.ApplyPackageReleaseResponse';
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
proto.node_agent.DeleteCacheArtifactRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.DeleteCacheArtifactRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.DeleteCacheArtifactRequest.displayName = 'proto.node_agent.DeleteCacheArtifactRequest';
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
proto.node_agent.DeleteCacheArtifactResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.DeleteCacheArtifactResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.DeleteCacheArtifactResponse.displayName = 'proto.node_agent.DeleteCacheArtifactResponse';
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
proto.node_agent.CleanupDiskJournalRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.CleanupDiskJournalRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.CleanupDiskJournalRequest.displayName = 'proto.node_agent.CleanupDiskJournalRequest';
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
proto.node_agent.CleanupDiskJournalResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.CleanupDiskJournalResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.CleanupDiskJournalResponse.displayName = 'proto.node_agent.CleanupDiskJournalResponse';
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
proto.node_agent.CollectBackupSecretsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.CollectBackupSecretsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.CollectBackupSecretsRequest.displayName = 'proto.node_agent.CollectBackupSecretsRequest';
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
proto.node_agent.SecretFileEntry = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.node_agent.SecretFileEntry, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.SecretFileEntry.displayName = 'proto.node_agent.SecretFileEntry';
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
proto.node_agent.CollectBackupSecretsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.node_agent.CollectBackupSecretsResponse.repeatedFields_, null);
};
goog.inherits(proto.node_agent.CollectBackupSecretsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.node_agent.CollectBackupSecretsResponse.displayName = 'proto.node_agent.CollectBackupSecretsResponse';
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
proto.node_agent.JoinClusterRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.JoinClusterRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.JoinClusterRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.JoinClusterRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
controllerEndpoint: jspb.Message.getFieldWithDefault(msg, 1, ""),
joinToken: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.node_agent.JoinClusterRequest}
 */
proto.node_agent.JoinClusterRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.JoinClusterRequest;
  return proto.node_agent.JoinClusterRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.JoinClusterRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.JoinClusterRequest}
 */
proto.node_agent.JoinClusterRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setControllerEndpoint(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setJoinToken(value);
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
proto.node_agent.JoinClusterRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.JoinClusterRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.JoinClusterRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.JoinClusterRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getControllerEndpoint();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getJoinToken();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string controller_endpoint = 1;
 * @return {string}
 */
proto.node_agent.JoinClusterRequest.prototype.getControllerEndpoint = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.JoinClusterRequest} returns this
 */
proto.node_agent.JoinClusterRequest.prototype.setControllerEndpoint = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string join_token = 2;
 * @return {string}
 */
proto.node_agent.JoinClusterRequest.prototype.getJoinToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.JoinClusterRequest} returns this
 */
proto.node_agent.JoinClusterRequest.prototype.setJoinToken = function(value) {
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
proto.node_agent.JoinClusterResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.JoinClusterResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.JoinClusterResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.JoinClusterResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
requestId: jspb.Message.getFieldWithDefault(msg, 1, ""),
nodeId: jspb.Message.getFieldWithDefault(msg, 2, ""),
status: jspb.Message.getFieldWithDefault(msg, 3, ""),
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
 * @return {!proto.node_agent.JoinClusterResponse}
 */
proto.node_agent.JoinClusterResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.JoinClusterResponse;
  return proto.node_agent.JoinClusterResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.JoinClusterResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.JoinClusterResponse}
 */
proto.node_agent.JoinClusterResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRequestId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeId(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setStatus(value);
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
proto.node_agent.JoinClusterResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.JoinClusterResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.JoinClusterResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.JoinClusterResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRequestId();
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
  f = message.getStatus();
  if (f.length > 0) {
    writer.writeString(
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
 * optional string request_id = 1;
 * @return {string}
 */
proto.node_agent.JoinClusterResponse.prototype.getRequestId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.JoinClusterResponse} returns this
 */
proto.node_agent.JoinClusterResponse.prototype.setRequestId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string node_id = 2;
 * @return {string}
 */
proto.node_agent.JoinClusterResponse.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.JoinClusterResponse} returns this
 */
proto.node_agent.JoinClusterResponse.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string status = 3;
 * @return {string}
 */
proto.node_agent.JoinClusterResponse.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.JoinClusterResponse} returns this
 */
proto.node_agent.JoinClusterResponse.prototype.setStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string message = 4;
 * @return {string}
 */
proto.node_agent.JoinClusterResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.JoinClusterResponse} returns this
 */
proto.node_agent.JoinClusterResponse.prototype.setMessage = function(value) {
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
proto.node_agent.InstalledComponent.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.InstalledComponent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.InstalledComponent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.InstalledComponent.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, ""),
version: jspb.Message.getFieldWithDefault(msg, 2, ""),
installed: jspb.Message.getBooleanFieldWithDefault(msg, 3, false)
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
 * @return {!proto.node_agent.InstalledComponent}
 */
proto.node_agent.InstalledComponent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.InstalledComponent;
  return proto.node_agent.InstalledComponent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.InstalledComponent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.InstalledComponent}
 */
proto.node_agent.InstalledComponent.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setVersion(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setInstalled(value);
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
proto.node_agent.InstalledComponent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.InstalledComponent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.InstalledComponent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.InstalledComponent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getVersion();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getInstalled();
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
proto.node_agent.InstalledComponent.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.InstalledComponent} returns this
 */
proto.node_agent.InstalledComponent.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string version = 2;
 * @return {string}
 */
proto.node_agent.InstalledComponent.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.InstalledComponent} returns this
 */
proto.node_agent.InstalledComponent.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional bool installed = 3;
 * @return {boolean}
 */
proto.node_agent.InstalledComponent.prototype.getInstalled = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.InstalledComponent} returns this
 */
proto.node_agent.InstalledComponent.prototype.setInstalled = function(value) {
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
proto.node_agent.InstalledPackage.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.InstalledPackage.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.InstalledPackage} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.InstalledPackage.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
name: jspb.Message.getFieldWithDefault(msg, 2, ""),
version: jspb.Message.getFieldWithDefault(msg, 3, ""),
publisherId: jspb.Message.getFieldWithDefault(msg, 4, ""),
platform: jspb.Message.getFieldWithDefault(msg, 5, ""),
kind: jspb.Message.getFieldWithDefault(msg, 6, ""),
checksum: jspb.Message.getFieldWithDefault(msg, 7, ""),
installedUnix: jspb.Message.getFieldWithDefault(msg, 8, 0),
updatedUnix: jspb.Message.getFieldWithDefault(msg, 9, 0),
status: jspb.Message.getFieldWithDefault(msg, 10, ""),
operationId: jspb.Message.getFieldWithDefault(msg, 11, ""),
metadataMap: (f = msg.getMetadataMap()) ? f.toObject(includeInstance, undefined) : [],
buildNumber: jspb.Message.getFieldWithDefault(msg, 13, 0),
buildId: jspb.Message.getFieldWithDefault(msg, 14, ""),
provisional: jspb.Message.getBooleanFieldWithDefault(msg, 15, false)
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
 * @return {!proto.node_agent.InstalledPackage}
 */
proto.node_agent.InstalledPackage.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.InstalledPackage;
  return proto.node_agent.InstalledPackage.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.InstalledPackage} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.InstalledPackage}
 */
proto.node_agent.InstalledPackage.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setVersion(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisherId(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlatform(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setKind(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setChecksum(value);
      break;
    case 8:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setInstalledUnix(value);
      break;
    case 9:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setUpdatedUnix(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.setStatus(value);
      break;
    case 11:
      var value = /** @type {string} */ (reader.readString());
      msg.setOperationId(value);
      break;
    case 12:
      var value = msg.getMetadataMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
      break;
    case 13:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    case 14:
      var value = /** @type {string} */ (reader.readString());
      msg.setBuildId(value);
      break;
    case 15:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setProvisional(value);
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
proto.node_agent.InstalledPackage.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.InstalledPackage.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.InstalledPackage} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.InstalledPackage.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
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
  f = message.getPublisherId();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getPlatform();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getKind();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getChecksum();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getInstalledUnix();
  if (f !== 0) {
    writer.writeInt64(
      8,
      f
    );
  }
  f = message.getUpdatedUnix();
  if (f !== 0) {
    writer.writeInt64(
      9,
      f
    );
  }
  f = message.getStatus();
  if (f.length > 0) {
    writer.writeString(
      10,
      f
    );
  }
  f = message.getOperationId();
  if (f.length > 0) {
    writer.writeString(
      11,
      f
    );
  }
  f = message.getMetadataMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(12, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
  f = message.getBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
      13,
      f
    );
  }
  f = message.getBuildId();
  if (f.length > 0) {
    writer.writeString(
      14,
      f
    );
  }
  f = message.getProvisional();
  if (f) {
    writer.writeBool(
      15,
      f
    );
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.node_agent.InstalledPackage.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.InstalledPackage} returns this
 */
proto.node_agent.InstalledPackage.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string name = 2;
 * @return {string}
 */
proto.node_agent.InstalledPackage.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.InstalledPackage} returns this
 */
proto.node_agent.InstalledPackage.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string version = 3;
 * @return {string}
 */
proto.node_agent.InstalledPackage.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.InstalledPackage} returns this
 */
proto.node_agent.InstalledPackage.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string publisher_id = 4;
 * @return {string}
 */
proto.node_agent.InstalledPackage.prototype.getPublisherId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.InstalledPackage} returns this
 */
proto.node_agent.InstalledPackage.prototype.setPublisherId = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string platform = 5;
 * @return {string}
 */
proto.node_agent.InstalledPackage.prototype.getPlatform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.InstalledPackage} returns this
 */
proto.node_agent.InstalledPackage.prototype.setPlatform = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string kind = 6;
 * @return {string}
 */
proto.node_agent.InstalledPackage.prototype.getKind = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.InstalledPackage} returns this
 */
proto.node_agent.InstalledPackage.prototype.setKind = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string checksum = 7;
 * @return {string}
 */
proto.node_agent.InstalledPackage.prototype.getChecksum = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.InstalledPackage} returns this
 */
proto.node_agent.InstalledPackage.prototype.setChecksum = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional int64 installed_unix = 8;
 * @return {number}
 */
proto.node_agent.InstalledPackage.prototype.getInstalledUnix = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 8, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.InstalledPackage} returns this
 */
proto.node_agent.InstalledPackage.prototype.setInstalledUnix = function(value) {
  return jspb.Message.setProto3IntField(this, 8, value);
};


/**
 * optional int64 updated_unix = 9;
 * @return {number}
 */
proto.node_agent.InstalledPackage.prototype.getUpdatedUnix = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 9, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.InstalledPackage} returns this
 */
proto.node_agent.InstalledPackage.prototype.setUpdatedUnix = function(value) {
  return jspb.Message.setProto3IntField(this, 9, value);
};


/**
 * optional string status = 10;
 * @return {string}
 */
proto.node_agent.InstalledPackage.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 10, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.InstalledPackage} returns this
 */
proto.node_agent.InstalledPackage.prototype.setStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 10, value);
};


/**
 * optional string operation_id = 11;
 * @return {string}
 */
proto.node_agent.InstalledPackage.prototype.getOperationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 11, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.InstalledPackage} returns this
 */
proto.node_agent.InstalledPackage.prototype.setOperationId = function(value) {
  return jspb.Message.setProto3StringField(this, 11, value);
};


/**
 * map<string, string> metadata = 12;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.node_agent.InstalledPackage.prototype.getMetadataMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 12, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.node_agent.InstalledPackage} returns this
 */
proto.node_agent.InstalledPackage.prototype.clearMetadataMap = function() {
  this.getMetadataMap().clear();
  return this;
};


/**
 * optional int64 build_number = 13;
 * @return {number}
 */
proto.node_agent.InstalledPackage.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 13, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.InstalledPackage} returns this
 */
proto.node_agent.InstalledPackage.prototype.setBuildNumber = function(value) {
  return jspb.Message.setProto3IntField(this, 13, value);
};


/**
 * optional string build_id = 14;
 * @return {string}
 */
proto.node_agent.InstalledPackage.prototype.getBuildId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 14, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.InstalledPackage} returns this
 */
proto.node_agent.InstalledPackage.prototype.setBuildId = function(value) {
  return jspb.Message.setProto3StringField(this, 14, value);
};


/**
 * optional bool provisional = 15;
 * @return {boolean}
 */
proto.node_agent.InstalledPackage.prototype.getProvisional = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 15, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.InstalledPackage} returns this
 */
proto.node_agent.InstalledPackage.prototype.setProvisional = function(value) {
  return jspb.Message.setProto3BooleanField(this, 15, value);
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
proto.node_agent.ListInstalledPackagesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.ListInstalledPackagesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.ListInstalledPackagesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.ListInstalledPackagesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
kind: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.node_agent.ListInstalledPackagesRequest}
 */
proto.node_agent.ListInstalledPackagesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.ListInstalledPackagesRequest;
  return proto.node_agent.ListInstalledPackagesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.ListInstalledPackagesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.ListInstalledPackagesRequest}
 */
proto.node_agent.ListInstalledPackagesRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setKind(value);
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
proto.node_agent.ListInstalledPackagesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.ListInstalledPackagesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.ListInstalledPackagesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.ListInstalledPackagesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getKind();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.node_agent.ListInstalledPackagesRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ListInstalledPackagesRequest} returns this
 */
proto.node_agent.ListInstalledPackagesRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string kind = 2;
 * @return {string}
 */
proto.node_agent.ListInstalledPackagesRequest.prototype.getKind = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ListInstalledPackagesRequest} returns this
 */
proto.node_agent.ListInstalledPackagesRequest.prototype.setKind = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.node_agent.ListInstalledPackagesResponse.repeatedFields_ = [1];



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
proto.node_agent.ListInstalledPackagesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.ListInstalledPackagesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.ListInstalledPackagesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.ListInstalledPackagesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
packagesList: jspb.Message.toObjectList(msg.getPackagesList(),
    proto.node_agent.InstalledPackage.toObject, includeInstance)
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
 * @return {!proto.node_agent.ListInstalledPackagesResponse}
 */
proto.node_agent.ListInstalledPackagesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.ListInstalledPackagesResponse;
  return proto.node_agent.ListInstalledPackagesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.ListInstalledPackagesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.ListInstalledPackagesResponse}
 */
proto.node_agent.ListInstalledPackagesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.node_agent.InstalledPackage;
      reader.readMessage(value,proto.node_agent.InstalledPackage.deserializeBinaryFromReader);
      msg.addPackages(value);
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
proto.node_agent.ListInstalledPackagesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.ListInstalledPackagesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.ListInstalledPackagesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.ListInstalledPackagesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPackagesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.node_agent.InstalledPackage.serializeBinaryToWriter
    );
  }
};


/**
 * repeated InstalledPackage packages = 1;
 * @return {!Array<!proto.node_agent.InstalledPackage>}
 */
proto.node_agent.ListInstalledPackagesResponse.prototype.getPackagesList = function() {
  return /** @type{!Array<!proto.node_agent.InstalledPackage>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.node_agent.InstalledPackage, 1));
};


/**
 * @param {!Array<!proto.node_agent.InstalledPackage>} value
 * @return {!proto.node_agent.ListInstalledPackagesResponse} returns this
*/
proto.node_agent.ListInstalledPackagesResponse.prototype.setPackagesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.node_agent.InstalledPackage=} opt_value
 * @param {number=} opt_index
 * @return {!proto.node_agent.InstalledPackage}
 */
proto.node_agent.ListInstalledPackagesResponse.prototype.addPackages = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.node_agent.InstalledPackage, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.node_agent.ListInstalledPackagesResponse} returns this
 */
proto.node_agent.ListInstalledPackagesResponse.prototype.clearPackagesList = function() {
  return this.setPackagesList([]);
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
proto.node_agent.GetInstalledPackageRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.GetInstalledPackageRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.GetInstalledPackageRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetInstalledPackageRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
kind: jspb.Message.getFieldWithDefault(msg, 2, ""),
name: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.node_agent.GetInstalledPackageRequest}
 */
proto.node_agent.GetInstalledPackageRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.GetInstalledPackageRequest;
  return proto.node_agent.GetInstalledPackageRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.GetInstalledPackageRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.GetInstalledPackageRequest}
 */
proto.node_agent.GetInstalledPackageRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setKind(value);
      break;
    case 3:
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
proto.node_agent.GetInstalledPackageRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.GetInstalledPackageRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.GetInstalledPackageRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetInstalledPackageRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getKind();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.node_agent.GetInstalledPackageRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.GetInstalledPackageRequest} returns this
 */
proto.node_agent.GetInstalledPackageRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string kind = 2;
 * @return {string}
 */
proto.node_agent.GetInstalledPackageRequest.prototype.getKind = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.GetInstalledPackageRequest} returns this
 */
proto.node_agent.GetInstalledPackageRequest.prototype.setKind = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string name = 3;
 * @return {string}
 */
proto.node_agent.GetInstalledPackageRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.GetInstalledPackageRequest} returns this
 */
proto.node_agent.GetInstalledPackageRequest.prototype.setName = function(value) {
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
proto.node_agent.GetInstalledPackageResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.GetInstalledPackageResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.GetInstalledPackageResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetInstalledPackageResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
pb_package: (f = msg.getPackage()) && proto.node_agent.InstalledPackage.toObject(includeInstance, f)
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
 * @return {!proto.node_agent.GetInstalledPackageResponse}
 */
proto.node_agent.GetInstalledPackageResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.GetInstalledPackageResponse;
  return proto.node_agent.GetInstalledPackageResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.GetInstalledPackageResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.GetInstalledPackageResponse}
 */
proto.node_agent.GetInstalledPackageResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.node_agent.InstalledPackage;
      reader.readMessage(value,proto.node_agent.InstalledPackage.deserializeBinaryFromReader);
      msg.setPackage(value);
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
proto.node_agent.GetInstalledPackageResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.GetInstalledPackageResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.GetInstalledPackageResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetInstalledPackageResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPackage();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.node_agent.InstalledPackage.serializeBinaryToWriter
    );
  }
};


/**
 * optional InstalledPackage package = 1;
 * @return {?proto.node_agent.InstalledPackage}
 */
proto.node_agent.GetInstalledPackageResponse.prototype.getPackage = function() {
  return /** @type{?proto.node_agent.InstalledPackage} */ (
    jspb.Message.getWrapperField(this, proto.node_agent.InstalledPackage, 1));
};


/**
 * @param {?proto.node_agent.InstalledPackage|undefined} value
 * @return {!proto.node_agent.GetInstalledPackageResponse} returns this
*/
proto.node_agent.GetInstalledPackageResponse.prototype.setPackage = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.node_agent.GetInstalledPackageResponse} returns this
 */
proto.node_agent.GetInstalledPackageResponse.prototype.clearPackage = function() {
  return this.setPackage(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.node_agent.GetInstalledPackageResponse.prototype.hasPackage = function() {
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
proto.node_agent.SetInstalledPackageRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.SetInstalledPackageRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.SetInstalledPackageRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.SetInstalledPackageRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
pb_package: (f = msg.getPackage()) && proto.node_agent.InstalledPackage.toObject(includeInstance, f)
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
 * @return {!proto.node_agent.SetInstalledPackageRequest}
 */
proto.node_agent.SetInstalledPackageRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.SetInstalledPackageRequest;
  return proto.node_agent.SetInstalledPackageRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.SetInstalledPackageRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.SetInstalledPackageRequest}
 */
proto.node_agent.SetInstalledPackageRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.node_agent.InstalledPackage;
      reader.readMessage(value,proto.node_agent.InstalledPackage.deserializeBinaryFromReader);
      msg.setPackage(value);
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
proto.node_agent.SetInstalledPackageRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.SetInstalledPackageRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.SetInstalledPackageRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.SetInstalledPackageRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPackage();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.node_agent.InstalledPackage.serializeBinaryToWriter
    );
  }
};


/**
 * optional InstalledPackage package = 1;
 * @return {?proto.node_agent.InstalledPackage}
 */
proto.node_agent.SetInstalledPackageRequest.prototype.getPackage = function() {
  return /** @type{?proto.node_agent.InstalledPackage} */ (
    jspb.Message.getWrapperField(this, proto.node_agent.InstalledPackage, 1));
};


/**
 * @param {?proto.node_agent.InstalledPackage|undefined} value
 * @return {!proto.node_agent.SetInstalledPackageRequest} returns this
*/
proto.node_agent.SetInstalledPackageRequest.prototype.setPackage = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.node_agent.SetInstalledPackageRequest} returns this
 */
proto.node_agent.SetInstalledPackageRequest.prototype.clearPackage = function() {
  return this.setPackage(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.node_agent.SetInstalledPackageRequest.prototype.hasPackage = function() {
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
proto.node_agent.SetInstalledPackageResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.SetInstalledPackageResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.SetInstalledPackageResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.SetInstalledPackageResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.node_agent.SetInstalledPackageResponse}
 */
proto.node_agent.SetInstalledPackageResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.SetInstalledPackageResponse;
  return proto.node_agent.SetInstalledPackageResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.SetInstalledPackageResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.SetInstalledPackageResponse}
 */
proto.node_agent.SetInstalledPackageResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.node_agent.SetInstalledPackageResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.SetInstalledPackageResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.SetInstalledPackageResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.SetInstalledPackageResponse.serializeBinaryToWriter = function(message, writer) {
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
proto.node_agent.SetInstalledPackageResponse.prototype.getOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.SetInstalledPackageResponse} returns this
 */
proto.node_agent.SetInstalledPackageResponse.prototype.setOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional string message = 2;
 * @return {string}
 */
proto.node_agent.SetInstalledPackageResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.SetInstalledPackageResponse} returns this
 */
proto.node_agent.SetInstalledPackageResponse.prototype.setMessage = function(value) {
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
proto.node_agent.UnitStatus.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.UnitStatus.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.UnitStatus} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.UnitStatus.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, ""),
state: jspb.Message.getFieldWithDefault(msg, 2, ""),
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
 * @return {!proto.node_agent.UnitStatus}
 */
proto.node_agent.UnitStatus.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.UnitStatus;
  return proto.node_agent.UnitStatus.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.UnitStatus} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.UnitStatus}
 */
proto.node_agent.UnitStatus.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setState(value);
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
proto.node_agent.UnitStatus.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.UnitStatus.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.UnitStatus} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.UnitStatus.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getState();
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
 * optional string name = 1;
 * @return {string}
 */
proto.node_agent.UnitStatus.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.UnitStatus} returns this
 */
proto.node_agent.UnitStatus.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string state = 2;
 * @return {string}
 */
proto.node_agent.UnitStatus.prototype.getState = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.UnitStatus} returns this
 */
proto.node_agent.UnitStatus.prototype.setState = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string details = 3;
 * @return {string}
 */
proto.node_agent.UnitStatus.prototype.getDetails = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.UnitStatus} returns this
 */
proto.node_agent.UnitStatus.prototype.setDetails = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.node_agent.Inventory.repeatedFields_ = [3,4];



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
proto.node_agent.Inventory.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.Inventory.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.Inventory} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.Inventory.toObject = function(includeInstance, msg) {
  var f, obj = {
identity: (f = msg.getIdentity()) && cluster_controller_pb.NodeIdentity.toObject(includeInstance, f),
unixTime: (f = msg.getUnixTime()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
componentsList: jspb.Message.toObjectList(msg.getComponentsList(),
    proto.node_agent.InstalledComponent.toObject, includeInstance),
unitsList: jspb.Message.toObjectList(msg.getUnitsList(),
    proto.node_agent.UnitStatus.toObject, includeInstance)
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
 * @return {!proto.node_agent.Inventory}
 */
proto.node_agent.Inventory.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.Inventory;
  return proto.node_agent.Inventory.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.Inventory} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.Inventory}
 */
proto.node_agent.Inventory.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new cluster_controller_pb.NodeIdentity;
      reader.readMessage(value,cluster_controller_pb.NodeIdentity.deserializeBinaryFromReader);
      msg.setIdentity(value);
      break;
    case 2:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setUnixTime(value);
      break;
    case 3:
      var value = new proto.node_agent.InstalledComponent;
      reader.readMessage(value,proto.node_agent.InstalledComponent.deserializeBinaryFromReader);
      msg.addComponents(value);
      break;
    case 4:
      var value = new proto.node_agent.UnitStatus;
      reader.readMessage(value,proto.node_agent.UnitStatus.deserializeBinaryFromReader);
      msg.addUnits(value);
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
proto.node_agent.Inventory.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.Inventory.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.Inventory} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.Inventory.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getIdentity();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      cluster_controller_pb.NodeIdentity.serializeBinaryToWriter
    );
  }
  f = message.getUnixTime();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getComponentsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      3,
      f,
      proto.node_agent.InstalledComponent.serializeBinaryToWriter
    );
  }
  f = message.getUnitsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      4,
      f,
      proto.node_agent.UnitStatus.serializeBinaryToWriter
    );
  }
};


/**
 * optional cluster_controller.NodeIdentity identity = 1;
 * @return {?proto.cluster_controller.NodeIdentity}
 */
proto.node_agent.Inventory.prototype.getIdentity = function() {
  return /** @type{?proto.cluster_controller.NodeIdentity} */ (
    jspb.Message.getWrapperField(this, cluster_controller_pb.NodeIdentity, 1));
};


/**
 * @param {?proto.cluster_controller.NodeIdentity|undefined} value
 * @return {!proto.node_agent.Inventory} returns this
*/
proto.node_agent.Inventory.prototype.setIdentity = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.node_agent.Inventory} returns this
 */
proto.node_agent.Inventory.prototype.clearIdentity = function() {
  return this.setIdentity(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.node_agent.Inventory.prototype.hasIdentity = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional google.protobuf.Timestamp unix_time = 2;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.node_agent.Inventory.prototype.getUnixTime = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 2));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.node_agent.Inventory} returns this
*/
proto.node_agent.Inventory.prototype.setUnixTime = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.node_agent.Inventory} returns this
 */
proto.node_agent.Inventory.prototype.clearUnixTime = function() {
  return this.setUnixTime(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.node_agent.Inventory.prototype.hasUnixTime = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * repeated InstalledComponent components = 3;
 * @return {!Array<!proto.node_agent.InstalledComponent>}
 */
proto.node_agent.Inventory.prototype.getComponentsList = function() {
  return /** @type{!Array<!proto.node_agent.InstalledComponent>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.node_agent.InstalledComponent, 3));
};


/**
 * @param {!Array<!proto.node_agent.InstalledComponent>} value
 * @return {!proto.node_agent.Inventory} returns this
*/
proto.node_agent.Inventory.prototype.setComponentsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 3, value);
};


/**
 * @param {!proto.node_agent.InstalledComponent=} opt_value
 * @param {number=} opt_index
 * @return {!proto.node_agent.InstalledComponent}
 */
proto.node_agent.Inventory.prototype.addComponents = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 3, opt_value, proto.node_agent.InstalledComponent, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.node_agent.Inventory} returns this
 */
proto.node_agent.Inventory.prototype.clearComponentsList = function() {
  return this.setComponentsList([]);
};


/**
 * repeated UnitStatus units = 4;
 * @return {!Array<!proto.node_agent.UnitStatus>}
 */
proto.node_agent.Inventory.prototype.getUnitsList = function() {
  return /** @type{!Array<!proto.node_agent.UnitStatus>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.node_agent.UnitStatus, 4));
};


/**
 * @param {!Array<!proto.node_agent.UnitStatus>} value
 * @return {!proto.node_agent.Inventory} returns this
*/
proto.node_agent.Inventory.prototype.setUnitsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 4, value);
};


/**
 * @param {!proto.node_agent.UnitStatus=} opt_value
 * @param {number=} opt_index
 * @return {!proto.node_agent.UnitStatus}
 */
proto.node_agent.Inventory.prototype.addUnits = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 4, opt_value, proto.node_agent.UnitStatus, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.node_agent.Inventory} returns this
 */
proto.node_agent.Inventory.prototype.clearUnitsList = function() {
  return this.setUnitsList([]);
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
proto.node_agent.GetInventoryRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.GetInventoryRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.GetInventoryRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetInventoryRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.node_agent.GetInventoryRequest}
 */
proto.node_agent.GetInventoryRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.GetInventoryRequest;
  return proto.node_agent.GetInventoryRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.GetInventoryRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.GetInventoryRequest}
 */
proto.node_agent.GetInventoryRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.node_agent.GetInventoryRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.GetInventoryRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.GetInventoryRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetInventoryRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.node_agent.GetInventoryResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.GetInventoryResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.GetInventoryResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetInventoryResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
inventory: (f = msg.getInventory()) && proto.node_agent.Inventory.toObject(includeInstance, f)
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
 * @return {!proto.node_agent.GetInventoryResponse}
 */
proto.node_agent.GetInventoryResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.GetInventoryResponse;
  return proto.node_agent.GetInventoryResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.GetInventoryResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.GetInventoryResponse}
 */
proto.node_agent.GetInventoryResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.node_agent.Inventory;
      reader.readMessage(value,proto.node_agent.Inventory.deserializeBinaryFromReader);
      msg.setInventory(value);
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
proto.node_agent.GetInventoryResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.GetInventoryResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.GetInventoryResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetInventoryResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getInventory();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.node_agent.Inventory.serializeBinaryToWriter
    );
  }
};


/**
 * optional Inventory inventory = 1;
 * @return {?proto.node_agent.Inventory}
 */
proto.node_agent.GetInventoryResponse.prototype.getInventory = function() {
  return /** @type{?proto.node_agent.Inventory} */ (
    jspb.Message.getWrapperField(this, proto.node_agent.Inventory, 1));
};


/**
 * @param {?proto.node_agent.Inventory|undefined} value
 * @return {!proto.node_agent.GetInventoryResponse} returns this
*/
proto.node_agent.GetInventoryResponse.prototype.setInventory = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.node_agent.GetInventoryResponse} returns this
 */
proto.node_agent.GetInventoryResponse.prototype.clearInventory = function() {
  return this.setInventory(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.node_agent.GetInventoryResponse.prototype.hasInventory = function() {
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
proto.node_agent.WatchOperationRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.WatchOperationRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.WatchOperationRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.WatchOperationRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
operationId: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.node_agent.WatchOperationRequest}
 */
proto.node_agent.WatchOperationRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.WatchOperationRequest;
  return proto.node_agent.WatchOperationRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.WatchOperationRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.WatchOperationRequest}
 */
proto.node_agent.WatchOperationRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setOperationId(value);
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
proto.node_agent.WatchOperationRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.WatchOperationRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.WatchOperationRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.WatchOperationRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOperationId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string operation_id = 1;
 * @return {string}
 */
proto.node_agent.WatchOperationRequest.prototype.getOperationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.WatchOperationRequest} returns this
 */
proto.node_agent.WatchOperationRequest.prototype.setOperationId = function(value) {
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
proto.node_agent.OperationEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.OperationEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.OperationEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.OperationEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
operationId: jspb.Message.getFieldWithDefault(msg, 1, ""),
phase: jspb.Message.getFieldWithDefault(msg, 2, 0),
message: jspb.Message.getFieldWithDefault(msg, 3, ""),
percent: jspb.Message.getFieldWithDefault(msg, 4, 0),
done: jspb.Message.getBooleanFieldWithDefault(msg, 5, false),
error: jspb.Message.getFieldWithDefault(msg, 6, ""),
ts: (f = msg.getTs()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f)
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
 * @return {!proto.node_agent.OperationEvent}
 */
proto.node_agent.OperationEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.OperationEvent;
  return proto.node_agent.OperationEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.OperationEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.OperationEvent}
 */
proto.node_agent.OperationEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setOperationId(value);
      break;
    case 2:
      var value = /** @type {!proto.cluster_controller.OperationPhase} */ (reader.readEnum());
      msg.setPhase(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setPercent(value);
      break;
    case 5:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setDone(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setError(value);
      break;
    case 7:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setTs(value);
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
proto.node_agent.OperationEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.OperationEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.OperationEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.OperationEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOperationId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getPhase();
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
  f = message.getPercent();
  if (f !== 0) {
    writer.writeInt32(
      4,
      f
    );
  }
  f = message.getDone();
  if (f) {
    writer.writeBool(
      5,
      f
    );
  }
  f = message.getError();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getTs();
  if (f != null) {
    writer.writeMessage(
      7,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
};


/**
 * optional string operation_id = 1;
 * @return {string}
 */
proto.node_agent.OperationEvent.prototype.getOperationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.OperationEvent} returns this
 */
proto.node_agent.OperationEvent.prototype.setOperationId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional cluster_controller.OperationPhase phase = 2;
 * @return {!proto.cluster_controller.OperationPhase}
 */
proto.node_agent.OperationEvent.prototype.getPhase = function() {
  return /** @type {!proto.cluster_controller.OperationPhase} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.cluster_controller.OperationPhase} value
 * @return {!proto.node_agent.OperationEvent} returns this
 */
proto.node_agent.OperationEvent.prototype.setPhase = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional string message = 3;
 * @return {string}
 */
proto.node_agent.OperationEvent.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.OperationEvent} returns this
 */
proto.node_agent.OperationEvent.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional int32 percent = 4;
 * @return {number}
 */
proto.node_agent.OperationEvent.prototype.getPercent = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.OperationEvent} returns this
 */
proto.node_agent.OperationEvent.prototype.setPercent = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional bool done = 5;
 * @return {boolean}
 */
proto.node_agent.OperationEvent.prototype.getDone = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 5, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.OperationEvent} returns this
 */
proto.node_agent.OperationEvent.prototype.setDone = function(value) {
  return jspb.Message.setProto3BooleanField(this, 5, value);
};


/**
 * optional string error = 6;
 * @return {string}
 */
proto.node_agent.OperationEvent.prototype.getError = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.OperationEvent} returns this
 */
proto.node_agent.OperationEvent.prototype.setError = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional google.protobuf.Timestamp ts = 7;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.node_agent.OperationEvent.prototype.getTs = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 7));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.node_agent.OperationEvent} returns this
*/
proto.node_agent.OperationEvent.prototype.setTs = function(value) {
  return jspb.Message.setWrapperField(this, 7, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.node_agent.OperationEvent} returns this
 */
proto.node_agent.OperationEvent.prototype.clearTs = function() {
  return this.setTs(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.node_agent.OperationEvent.prototype.hasTs = function() {
  return jspb.Message.getField(this, 7) != null;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.node_agent.BootstrapFirstNodeRequest.repeatedFields_ = [3];



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
proto.node_agent.BootstrapFirstNodeRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.BootstrapFirstNodeRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.BootstrapFirstNodeRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.BootstrapFirstNodeRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterDomain: jspb.Message.getFieldWithDefault(msg, 1, ""),
controllerBind: jspb.Message.getFieldWithDefault(msg, 2, ""),
profilesList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f
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
 * @return {!proto.node_agent.BootstrapFirstNodeRequest}
 */
proto.node_agent.BootstrapFirstNodeRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.BootstrapFirstNodeRequest;
  return proto.node_agent.BootstrapFirstNodeRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.BootstrapFirstNodeRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.BootstrapFirstNodeRequest}
 */
proto.node_agent.BootstrapFirstNodeRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterDomain(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setControllerBind(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.addProfiles(value);
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
proto.node_agent.BootstrapFirstNodeRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.BootstrapFirstNodeRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.BootstrapFirstNodeRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.BootstrapFirstNodeRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterDomain();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getControllerBind();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getProfilesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      3,
      f
    );
  }
};


/**
 * optional string cluster_domain = 1;
 * @return {string}
 */
proto.node_agent.BootstrapFirstNodeRequest.prototype.getClusterDomain = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.BootstrapFirstNodeRequest} returns this
 */
proto.node_agent.BootstrapFirstNodeRequest.prototype.setClusterDomain = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string controller_bind = 2;
 * @return {string}
 */
proto.node_agent.BootstrapFirstNodeRequest.prototype.getControllerBind = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.BootstrapFirstNodeRequest} returns this
 */
proto.node_agent.BootstrapFirstNodeRequest.prototype.setControllerBind = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * repeated string profiles = 3;
 * @return {!Array<string>}
 */
proto.node_agent.BootstrapFirstNodeRequest.prototype.getProfilesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.node_agent.BootstrapFirstNodeRequest} returns this
 */
proto.node_agent.BootstrapFirstNodeRequest.prototype.setProfilesList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.node_agent.BootstrapFirstNodeRequest} returns this
 */
proto.node_agent.BootstrapFirstNodeRequest.prototype.addProfiles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.node_agent.BootstrapFirstNodeRequest} returns this
 */
proto.node_agent.BootstrapFirstNodeRequest.prototype.clearProfilesList = function() {
  return this.setProfilesList([]);
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
proto.node_agent.BootstrapFirstNodeResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.BootstrapFirstNodeResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.BootstrapFirstNodeResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.BootstrapFirstNodeResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
operationId: jspb.Message.getFieldWithDefault(msg, 1, ""),
joinToken: jspb.Message.getFieldWithDefault(msg, 2, ""),
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
 * @return {!proto.node_agent.BootstrapFirstNodeResponse}
 */
proto.node_agent.BootstrapFirstNodeResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.BootstrapFirstNodeResponse;
  return proto.node_agent.BootstrapFirstNodeResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.BootstrapFirstNodeResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.BootstrapFirstNodeResponse}
 */
proto.node_agent.BootstrapFirstNodeResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setOperationId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setJoinToken(value);
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
proto.node_agent.BootstrapFirstNodeResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.BootstrapFirstNodeResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.BootstrapFirstNodeResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.BootstrapFirstNodeResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOperationId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getJoinToken();
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
 * optional string operation_id = 1;
 * @return {string}
 */
proto.node_agent.BootstrapFirstNodeResponse.prototype.getOperationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.BootstrapFirstNodeResponse} returns this
 */
proto.node_agent.BootstrapFirstNodeResponse.prototype.setOperationId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string join_token = 2;
 * @return {string}
 */
proto.node_agent.BootstrapFirstNodeResponse.prototype.getJoinToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.BootstrapFirstNodeResponse} returns this
 */
proto.node_agent.BootstrapFirstNodeResponse.prototype.setJoinToken = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string message = 3;
 * @return {string}
 */
proto.node_agent.BootstrapFirstNodeResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.BootstrapFirstNodeResponse} returns this
 */
proto.node_agent.BootstrapFirstNodeResponse.prototype.setMessage = function(value) {
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
proto.node_agent.BackupProviderSpec.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.BackupProviderSpec.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.BackupProviderSpec} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.BackupProviderSpec.toObject = function(includeInstance, msg) {
  var f, obj = {
provider: jspb.Message.getFieldWithDefault(msg, 1, ""),
optionsMap: (f = msg.getOptionsMap()) ? f.toObject(includeInstance, undefined) : [],
timeoutSeconds: jspb.Message.getFieldWithDefault(msg, 3, 0)
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
 * @return {!proto.node_agent.BackupProviderSpec}
 */
proto.node_agent.BackupProviderSpec.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.BackupProviderSpec;
  return proto.node_agent.BackupProviderSpec.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.BackupProviderSpec} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.BackupProviderSpec}
 */
proto.node_agent.BackupProviderSpec.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = msg.getOptionsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
      break;
    case 3:
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
proto.node_agent.BackupProviderSpec.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.BackupProviderSpec.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.BackupProviderSpec} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.BackupProviderSpec.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getProvider();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOptionsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(2, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
  f = message.getTimeoutSeconds();
  if (f !== 0) {
    writer.writeUint32(
      3,
      f
    );
  }
};


/**
 * optional string provider = 1;
 * @return {string}
 */
proto.node_agent.BackupProviderSpec.prototype.getProvider = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.BackupProviderSpec} returns this
 */
proto.node_agent.BackupProviderSpec.prototype.setProvider = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * map<string, string> options = 2;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.node_agent.BackupProviderSpec.prototype.getOptionsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 2, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.node_agent.BackupProviderSpec} returns this
 */
proto.node_agent.BackupProviderSpec.prototype.clearOptionsMap = function() {
  this.getOptionsMap().clear();
  return this;
};


/**
 * optional uint32 timeout_seconds = 3;
 * @return {number}
 */
proto.node_agent.BackupProviderSpec.prototype.getTimeoutSeconds = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.BackupProviderSpec} returns this
 */
proto.node_agent.BackupProviderSpec.prototype.setTimeoutSeconds = function(value) {
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
proto.node_agent.RunBackupProviderRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.RunBackupProviderRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.RunBackupProviderRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.RunBackupProviderRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
backupId: jspb.Message.getFieldWithDefault(msg, 1, ""),
spec: (f = msg.getSpec()) && proto.node_agent.BackupProviderSpec.toObject(includeInstance, f),
nodeId: jspb.Message.getFieldWithDefault(msg, 3, ""),
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
 * @return {!proto.node_agent.RunBackupProviderRequest}
 */
proto.node_agent.RunBackupProviderRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.RunBackupProviderRequest;
  return proto.node_agent.RunBackupProviderRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.RunBackupProviderRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.RunBackupProviderRequest}
 */
proto.node_agent.RunBackupProviderRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = new proto.node_agent.BackupProviderSpec;
      reader.readMessage(value,proto.node_agent.BackupProviderSpec.deserializeBinaryFromReader);
      msg.setSpec(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeId(value);
      break;
    case 4:
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
proto.node_agent.RunBackupProviderRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.RunBackupProviderRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.RunBackupProviderRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.RunBackupProviderRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBackupId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getSpec();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.node_agent.BackupProviderSpec.serializeBinaryToWriter
    );
  }
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getLabelsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(4, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
};


/**
 * optional string backup_id = 1;
 * @return {string}
 */
proto.node_agent.RunBackupProviderRequest.prototype.getBackupId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.RunBackupProviderRequest} returns this
 */
proto.node_agent.RunBackupProviderRequest.prototype.setBackupId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional BackupProviderSpec spec = 2;
 * @return {?proto.node_agent.BackupProviderSpec}
 */
proto.node_agent.RunBackupProviderRequest.prototype.getSpec = function() {
  return /** @type{?proto.node_agent.BackupProviderSpec} */ (
    jspb.Message.getWrapperField(this, proto.node_agent.BackupProviderSpec, 2));
};


/**
 * @param {?proto.node_agent.BackupProviderSpec|undefined} value
 * @return {!proto.node_agent.RunBackupProviderRequest} returns this
*/
proto.node_agent.RunBackupProviderRequest.prototype.setSpec = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.node_agent.RunBackupProviderRequest} returns this
 */
proto.node_agent.RunBackupProviderRequest.prototype.clearSpec = function() {
  return this.setSpec(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.node_agent.RunBackupProviderRequest.prototype.hasSpec = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional string node_id = 3;
 * @return {string}
 */
proto.node_agent.RunBackupProviderRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.RunBackupProviderRequest} returns this
 */
proto.node_agent.RunBackupProviderRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * map<string, string> labels = 4;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.node_agent.RunBackupProviderRequest.prototype.getLabelsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 4, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.node_agent.RunBackupProviderRequest} returns this
 */
proto.node_agent.RunBackupProviderRequest.prototype.clearLabelsMap = function() {
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
proto.node_agent.RunBackupProviderResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.RunBackupProviderResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.RunBackupProviderResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.RunBackupProviderResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
taskId: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.node_agent.RunBackupProviderResponse}
 */
proto.node_agent.RunBackupProviderResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.RunBackupProviderResponse;
  return proto.node_agent.RunBackupProviderResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.RunBackupProviderResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.RunBackupProviderResponse}
 */
proto.node_agent.RunBackupProviderResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setTaskId(value);
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
proto.node_agent.RunBackupProviderResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.RunBackupProviderResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.RunBackupProviderResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.RunBackupProviderResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTaskId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string task_id = 1;
 * @return {string}
 */
proto.node_agent.RunBackupProviderResponse.prototype.getTaskId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.RunBackupProviderResponse} returns this
 */
proto.node_agent.RunBackupProviderResponse.prototype.setTaskId = function(value) {
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
proto.node_agent.GetBackupTaskResultRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.GetBackupTaskResultRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.GetBackupTaskResultRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetBackupTaskResultRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
taskId: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.node_agent.GetBackupTaskResultRequest}
 */
proto.node_agent.GetBackupTaskResultRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.GetBackupTaskResultRequest;
  return proto.node_agent.GetBackupTaskResultRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.GetBackupTaskResultRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.GetBackupTaskResultRequest}
 */
proto.node_agent.GetBackupTaskResultRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setTaskId(value);
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
proto.node_agent.GetBackupTaskResultRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.GetBackupTaskResultRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.GetBackupTaskResultRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetBackupTaskResultRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTaskId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string task_id = 1;
 * @return {string}
 */
proto.node_agent.GetBackupTaskResultRequest.prototype.getTaskId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.GetBackupTaskResultRequest} returns this
 */
proto.node_agent.GetBackupTaskResultRequest.prototype.setTaskId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.node_agent.BackupProviderResult.repeatedFields_ = [6];



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
proto.node_agent.BackupProviderResult.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.BackupProviderResult.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.BackupProviderResult} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.BackupProviderResult.toObject = function(includeInstance, msg) {
  var f, obj = {
provider: jspb.Message.getFieldWithDefault(msg, 1, ""),
ok: jspb.Message.getBooleanFieldWithDefault(msg, 2, false),
summary: jspb.Message.getFieldWithDefault(msg, 3, ""),
errorMessage: jspb.Message.getFieldWithDefault(msg, 4, ""),
outputsMap: (f = msg.getOutputsMap()) ? f.toObject(includeInstance, undefined) : [],
outputFilesList: (f = jspb.Message.getRepeatedField(msg, 6)) == null ? undefined : f,
startedUnixMs: jspb.Message.getFieldWithDefault(msg, 7, 0),
finishedUnixMs: jspb.Message.getFieldWithDefault(msg, 8, 0),
bytesWritten: jspb.Message.getFieldWithDefault(msg, 9, 0),
done: jspb.Message.getBooleanFieldWithDefault(msg, 10, false),
artifactsMap: (f = msg.getArtifactsMap()) ? f.toObject(includeInstance, undefined) : []
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
 * @return {!proto.node_agent.BackupProviderResult}
 */
proto.node_agent.BackupProviderResult.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.BackupProviderResult;
  return proto.node_agent.BackupProviderResult.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.BackupProviderResult} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.BackupProviderResult}
 */
proto.node_agent.BackupProviderResult.deserializeBinaryFromReader = function(msg, reader) {
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
    case 5:
      var value = msg.getOutputsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.addOutputFiles(value);
      break;
    case 7:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setStartedUnixMs(value);
      break;
    case 8:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setFinishedUnixMs(value);
      break;
    case 9:
      var value = /** @type {number} */ (reader.readUint64());
      msg.setBytesWritten(value);
      break;
    case 10:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setDone(value);
      break;
    case 11:
      var value = msg.getArtifactsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readBytes, null, "", "");
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
proto.node_agent.BackupProviderResult.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.BackupProviderResult.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.BackupProviderResult} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.BackupProviderResult.serializeBinaryToWriter = function(message, writer) {
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
  f = message.getOutputsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(5, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
  f = message.getOutputFilesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
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
  f = message.getBytesWritten();
  if (f !== 0) {
    writer.writeUint64(
      9,
      f
    );
  }
  f = message.getDone();
  if (f) {
    writer.writeBool(
      10,
      f
    );
  }
  f = message.getArtifactsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(11, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeBytes);
  }
};


/**
 * optional string provider = 1;
 * @return {string}
 */
proto.node_agent.BackupProviderResult.prototype.getProvider = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.BackupProviderResult} returns this
 */
proto.node_agent.BackupProviderResult.prototype.setProvider = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bool ok = 2;
 * @return {boolean}
 */
proto.node_agent.BackupProviderResult.prototype.getOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.BackupProviderResult} returns this
 */
proto.node_agent.BackupProviderResult.prototype.setOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * optional string summary = 3;
 * @return {string}
 */
proto.node_agent.BackupProviderResult.prototype.getSummary = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.BackupProviderResult} returns this
 */
proto.node_agent.BackupProviderResult.prototype.setSummary = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string error_message = 4;
 * @return {string}
 */
proto.node_agent.BackupProviderResult.prototype.getErrorMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.BackupProviderResult} returns this
 */
proto.node_agent.BackupProviderResult.prototype.setErrorMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * map<string, string> outputs = 5;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.node_agent.BackupProviderResult.prototype.getOutputsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 5, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.node_agent.BackupProviderResult} returns this
 */
proto.node_agent.BackupProviderResult.prototype.clearOutputsMap = function() {
  this.getOutputsMap().clear();
  return this;
};


/**
 * repeated string output_files = 6;
 * @return {!Array<string>}
 */
proto.node_agent.BackupProviderResult.prototype.getOutputFilesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 6));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.node_agent.BackupProviderResult} returns this
 */
proto.node_agent.BackupProviderResult.prototype.setOutputFilesList = function(value) {
  return jspb.Message.setField(this, 6, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.node_agent.BackupProviderResult} returns this
 */
proto.node_agent.BackupProviderResult.prototype.addOutputFiles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 6, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.node_agent.BackupProviderResult} returns this
 */
proto.node_agent.BackupProviderResult.prototype.clearOutputFilesList = function() {
  return this.setOutputFilesList([]);
};


/**
 * optional int64 started_unix_ms = 7;
 * @return {number}
 */
proto.node_agent.BackupProviderResult.prototype.getStartedUnixMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.BackupProviderResult} returns this
 */
proto.node_agent.BackupProviderResult.prototype.setStartedUnixMs = function(value) {
  return jspb.Message.setProto3IntField(this, 7, value);
};


/**
 * optional int64 finished_unix_ms = 8;
 * @return {number}
 */
proto.node_agent.BackupProviderResult.prototype.getFinishedUnixMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 8, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.BackupProviderResult} returns this
 */
proto.node_agent.BackupProviderResult.prototype.setFinishedUnixMs = function(value) {
  return jspb.Message.setProto3IntField(this, 8, value);
};


/**
 * optional uint64 bytes_written = 9;
 * @return {number}
 */
proto.node_agent.BackupProviderResult.prototype.getBytesWritten = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 9, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.BackupProviderResult} returns this
 */
proto.node_agent.BackupProviderResult.prototype.setBytesWritten = function(value) {
  return jspb.Message.setProto3IntField(this, 9, value);
};


/**
 * optional bool done = 10;
 * @return {boolean}
 */
proto.node_agent.BackupProviderResult.prototype.getDone = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 10, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.BackupProviderResult} returns this
 */
proto.node_agent.BackupProviderResult.prototype.setDone = function(value) {
  return jspb.Message.setProto3BooleanField(this, 10, value);
};


/**
 * map<string, bytes> artifacts = 11;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,!(string|Uint8Array)>}
 */
proto.node_agent.BackupProviderResult.prototype.getArtifactsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,!(string|Uint8Array)>} */ (
      jspb.Message.getMapField(this, 11, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.node_agent.BackupProviderResult} returns this
 */
proto.node_agent.BackupProviderResult.prototype.clearArtifactsMap = function() {
  this.getArtifactsMap().clear();
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
proto.node_agent.GetBackupTaskResultResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.GetBackupTaskResultResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.GetBackupTaskResultResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetBackupTaskResultResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
result: (f = msg.getResult()) && proto.node_agent.BackupProviderResult.toObject(includeInstance, f)
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
 * @return {!proto.node_agent.GetBackupTaskResultResponse}
 */
proto.node_agent.GetBackupTaskResultResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.GetBackupTaskResultResponse;
  return proto.node_agent.GetBackupTaskResultResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.GetBackupTaskResultResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.GetBackupTaskResultResponse}
 */
proto.node_agent.GetBackupTaskResultResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.node_agent.BackupProviderResult;
      reader.readMessage(value,proto.node_agent.BackupProviderResult.deserializeBinaryFromReader);
      msg.setResult(value);
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
proto.node_agent.GetBackupTaskResultResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.GetBackupTaskResultResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.GetBackupTaskResultResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetBackupTaskResultResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.node_agent.BackupProviderResult.serializeBinaryToWriter
    );
  }
};


/**
 * optional BackupProviderResult result = 1;
 * @return {?proto.node_agent.BackupProviderResult}
 */
proto.node_agent.GetBackupTaskResultResponse.prototype.getResult = function() {
  return /** @type{?proto.node_agent.BackupProviderResult} */ (
    jspb.Message.getWrapperField(this, proto.node_agent.BackupProviderResult, 1));
};


/**
 * @param {?proto.node_agent.BackupProviderResult|undefined} value
 * @return {!proto.node_agent.GetBackupTaskResultResponse} returns this
*/
proto.node_agent.GetBackupTaskResultResponse.prototype.setResult = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.node_agent.GetBackupTaskResultResponse} returns this
 */
proto.node_agent.GetBackupTaskResultResponse.prototype.clearResult = function() {
  return this.setResult(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.node_agent.GetBackupTaskResultResponse.prototype.hasResult = function() {
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
proto.node_agent.RestoreProviderSpec.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.RestoreProviderSpec.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.RestoreProviderSpec} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.RestoreProviderSpec.toObject = function(includeInstance, msg) {
  var f, obj = {
provider: jspb.Message.getFieldWithDefault(msg, 1, ""),
optionsMap: (f = msg.getOptionsMap()) ? f.toObject(includeInstance, undefined) : [],
timeoutSeconds: jspb.Message.getFieldWithDefault(msg, 3, 0),
force: jspb.Message.getBooleanFieldWithDefault(msg, 4, false)
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
 * @return {!proto.node_agent.RestoreProviderSpec}
 */
proto.node_agent.RestoreProviderSpec.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.RestoreProviderSpec;
  return proto.node_agent.RestoreProviderSpec.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.RestoreProviderSpec} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.RestoreProviderSpec}
 */
proto.node_agent.RestoreProviderSpec.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = msg.getOptionsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
      break;
    case 3:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setTimeoutSeconds(value);
      break;
    case 4:
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
proto.node_agent.RestoreProviderSpec.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.RestoreProviderSpec.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.RestoreProviderSpec} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.RestoreProviderSpec.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getProvider();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOptionsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(2, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
  f = message.getTimeoutSeconds();
  if (f !== 0) {
    writer.writeUint32(
      3,
      f
    );
  }
  f = message.getForce();
  if (f) {
    writer.writeBool(
      4,
      f
    );
  }
};


/**
 * optional string provider = 1;
 * @return {string}
 */
proto.node_agent.RestoreProviderSpec.prototype.getProvider = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.RestoreProviderSpec} returns this
 */
proto.node_agent.RestoreProviderSpec.prototype.setProvider = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * map<string, string> options = 2;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.node_agent.RestoreProviderSpec.prototype.getOptionsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 2, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.node_agent.RestoreProviderSpec} returns this
 */
proto.node_agent.RestoreProviderSpec.prototype.clearOptionsMap = function() {
  this.getOptionsMap().clear();
  return this;
};


/**
 * optional uint32 timeout_seconds = 3;
 * @return {number}
 */
proto.node_agent.RestoreProviderSpec.prototype.getTimeoutSeconds = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.RestoreProviderSpec} returns this
 */
proto.node_agent.RestoreProviderSpec.prototype.setTimeoutSeconds = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional bool force = 4;
 * @return {boolean}
 */
proto.node_agent.RestoreProviderSpec.prototype.getForce = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 4, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.RestoreProviderSpec} returns this
 */
proto.node_agent.RestoreProviderSpec.prototype.setForce = function(value) {
  return jspb.Message.setProto3BooleanField(this, 4, value);
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
proto.node_agent.RunRestoreProviderRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.RunRestoreProviderRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.RunRestoreProviderRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.RunRestoreProviderRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
backupId: jspb.Message.getFieldWithDefault(msg, 1, ""),
spec: (f = msg.getSpec()) && proto.node_agent.RestoreProviderSpec.toObject(includeInstance, f),
nodeId: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.node_agent.RunRestoreProviderRequest}
 */
proto.node_agent.RunRestoreProviderRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.RunRestoreProviderRequest;
  return proto.node_agent.RunRestoreProviderRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.RunRestoreProviderRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.RunRestoreProviderRequest}
 */
proto.node_agent.RunRestoreProviderRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = new proto.node_agent.RestoreProviderSpec;
      reader.readMessage(value,proto.node_agent.RestoreProviderSpec.deserializeBinaryFromReader);
      msg.setSpec(value);
      break;
    case 3:
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
proto.node_agent.RunRestoreProviderRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.RunRestoreProviderRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.RunRestoreProviderRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.RunRestoreProviderRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBackupId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getSpec();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.node_agent.RestoreProviderSpec.serializeBinaryToWriter
    );
  }
  f = message.getNodeId();
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
proto.node_agent.RunRestoreProviderRequest.prototype.getBackupId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.RunRestoreProviderRequest} returns this
 */
proto.node_agent.RunRestoreProviderRequest.prototype.setBackupId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional RestoreProviderSpec spec = 2;
 * @return {?proto.node_agent.RestoreProviderSpec}
 */
proto.node_agent.RunRestoreProviderRequest.prototype.getSpec = function() {
  return /** @type{?proto.node_agent.RestoreProviderSpec} */ (
    jspb.Message.getWrapperField(this, proto.node_agent.RestoreProviderSpec, 2));
};


/**
 * @param {?proto.node_agent.RestoreProviderSpec|undefined} value
 * @return {!proto.node_agent.RunRestoreProviderRequest} returns this
*/
proto.node_agent.RunRestoreProviderRequest.prototype.setSpec = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.node_agent.RunRestoreProviderRequest} returns this
 */
proto.node_agent.RunRestoreProviderRequest.prototype.clearSpec = function() {
  return this.setSpec(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.node_agent.RunRestoreProviderRequest.prototype.hasSpec = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional string node_id = 3;
 * @return {string}
 */
proto.node_agent.RunRestoreProviderRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.RunRestoreProviderRequest} returns this
 */
proto.node_agent.RunRestoreProviderRequest.prototype.setNodeId = function(value) {
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
proto.node_agent.RunRestoreProviderResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.RunRestoreProviderResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.RunRestoreProviderResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.RunRestoreProviderResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
taskId: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.node_agent.RunRestoreProviderResponse}
 */
proto.node_agent.RunRestoreProviderResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.RunRestoreProviderResponse;
  return proto.node_agent.RunRestoreProviderResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.RunRestoreProviderResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.RunRestoreProviderResponse}
 */
proto.node_agent.RunRestoreProviderResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setTaskId(value);
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
proto.node_agent.RunRestoreProviderResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.RunRestoreProviderResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.RunRestoreProviderResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.RunRestoreProviderResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTaskId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string task_id = 1;
 * @return {string}
 */
proto.node_agent.RunRestoreProviderResponse.prototype.getTaskId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.RunRestoreProviderResponse} returns this
 */
proto.node_agent.RunRestoreProviderResponse.prototype.setTaskId = function(value) {
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
proto.node_agent.GetRestoreTaskResultRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.GetRestoreTaskResultRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.GetRestoreTaskResultRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetRestoreTaskResultRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
taskId: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.node_agent.GetRestoreTaskResultRequest}
 */
proto.node_agent.GetRestoreTaskResultRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.GetRestoreTaskResultRequest;
  return proto.node_agent.GetRestoreTaskResultRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.GetRestoreTaskResultRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.GetRestoreTaskResultRequest}
 */
proto.node_agent.GetRestoreTaskResultRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setTaskId(value);
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
proto.node_agent.GetRestoreTaskResultRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.GetRestoreTaskResultRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.GetRestoreTaskResultRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetRestoreTaskResultRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTaskId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string task_id = 1;
 * @return {string}
 */
proto.node_agent.GetRestoreTaskResultRequest.prototype.getTaskId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.GetRestoreTaskResultRequest} returns this
 */
proto.node_agent.GetRestoreTaskResultRequest.prototype.setTaskId = function(value) {
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
proto.node_agent.GetRestoreTaskResultResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.GetRestoreTaskResultResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.GetRestoreTaskResultResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetRestoreTaskResultResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
result: (f = msg.getResult()) && proto.node_agent.BackupProviderResult.toObject(includeInstance, f)
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
 * @return {!proto.node_agent.GetRestoreTaskResultResponse}
 */
proto.node_agent.GetRestoreTaskResultResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.GetRestoreTaskResultResponse;
  return proto.node_agent.GetRestoreTaskResultResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.GetRestoreTaskResultResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.GetRestoreTaskResultResponse}
 */
proto.node_agent.GetRestoreTaskResultResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.node_agent.BackupProviderResult;
      reader.readMessage(value,proto.node_agent.BackupProviderResult.deserializeBinaryFromReader);
      msg.setResult(value);
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
proto.node_agent.GetRestoreTaskResultResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.GetRestoreTaskResultResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.GetRestoreTaskResultResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetRestoreTaskResultResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.node_agent.BackupProviderResult.serializeBinaryToWriter
    );
  }
};


/**
 * optional BackupProviderResult result = 1;
 * @return {?proto.node_agent.BackupProviderResult}
 */
proto.node_agent.GetRestoreTaskResultResponse.prototype.getResult = function() {
  return /** @type{?proto.node_agent.BackupProviderResult} */ (
    jspb.Message.getWrapperField(this, proto.node_agent.BackupProviderResult, 1));
};


/**
 * @param {?proto.node_agent.BackupProviderResult|undefined} value
 * @return {!proto.node_agent.GetRestoreTaskResultResponse} returns this
*/
proto.node_agent.GetRestoreTaskResultResponse.prototype.setResult = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.node_agent.GetRestoreTaskResultResponse} returns this
 */
proto.node_agent.GetRestoreTaskResultResponse.prototype.clearResult = function() {
  return this.setResult(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.node_agent.GetRestoreTaskResultResponse.prototype.hasResult = function() {
  return jspb.Message.getField(this, 1) != null;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.node_agent.ServiceRuntimeProof.repeatedFields_ = [41];



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
proto.node_agent.ServiceRuntimeProof.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.ServiceRuntimeProof.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.ServiceRuntimeProof} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.ServiceRuntimeProof.toObject = function(includeInstance, msg) {
  var f, obj = {
serviceName: jspb.Message.getFieldWithDefault(msg, 1, ""),
serviceId: jspb.Message.getFieldWithDefault(msg, 2, ""),
nodeId: jspb.Message.getFieldWithDefault(msg, 3, ""),
expectedBuildId: jspb.Message.getFieldWithDefault(msg, 10, ""),
expectedVersion: jspb.Message.getFieldWithDefault(msg, 11, ""),
installedPath: jspb.Message.getFieldWithDefault(msg, 12, ""),
installedSha256: jspb.Message.getFieldWithDefault(msg, 13, ""),
runningPid: jspb.Message.getFieldWithDefault(msg, 20, 0),
runningExePath: jspb.Message.getFieldWithDefault(msg, 21, ""),
runningExeSha256: jspb.Message.getFieldWithDefault(msg, 22, ""),
runtimeVersion: jspb.Message.getFieldWithDefault(msg, 23, ""),
runtimeBuildId: jspb.Message.getFieldWithDefault(msg, 24, ""),
processStartTime: (f = msg.getProcessStartTime()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
systemdActiveState: jspb.Message.getFieldWithDefault(msg, 30, ""),
systemdSubState: jspb.Message.getFieldWithDefault(msg, 31, ""),
systemdUnitPath: jspb.Message.getFieldWithDefault(msg, 32, ""),
systemdUnitSha256: jspb.Message.getFieldWithDefault(msg, 33, ""),
effectiveExecStart: jspb.Message.getFieldWithDefault(msg, 34, ""),
effectiveType: jspb.Message.getFieldWithDefault(msg, 35, ""),
checkedAt: (f = msg.getCheckedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
errorsList: (f = jspb.Message.getRepeatedField(msg, 41)) == null ? undefined : f
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
 * @return {!proto.node_agent.ServiceRuntimeProof}
 */
proto.node_agent.ServiceRuntimeProof.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.ServiceRuntimeProof;
  return proto.node_agent.ServiceRuntimeProof.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.ServiceRuntimeProof} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.ServiceRuntimeProof}
 */
proto.node_agent.ServiceRuntimeProof.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {string} */ (reader.readString());
      msg.setServiceId(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeId(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.setExpectedBuildId(value);
      break;
    case 11:
      var value = /** @type {string} */ (reader.readString());
      msg.setExpectedVersion(value);
      break;
    case 12:
      var value = /** @type {string} */ (reader.readString());
      msg.setInstalledPath(value);
      break;
    case 13:
      var value = /** @type {string} */ (reader.readString());
      msg.setInstalledSha256(value);
      break;
    case 20:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setRunningPid(value);
      break;
    case 21:
      var value = /** @type {string} */ (reader.readString());
      msg.setRunningExePath(value);
      break;
    case 22:
      var value = /** @type {string} */ (reader.readString());
      msg.setRunningExeSha256(value);
      break;
    case 23:
      var value = /** @type {string} */ (reader.readString());
      msg.setRuntimeVersion(value);
      break;
    case 24:
      var value = /** @type {string} */ (reader.readString());
      msg.setRuntimeBuildId(value);
      break;
    case 25:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setProcessStartTime(value);
      break;
    case 30:
      var value = /** @type {string} */ (reader.readString());
      msg.setSystemdActiveState(value);
      break;
    case 31:
      var value = /** @type {string} */ (reader.readString());
      msg.setSystemdSubState(value);
      break;
    case 32:
      var value = /** @type {string} */ (reader.readString());
      msg.setSystemdUnitPath(value);
      break;
    case 33:
      var value = /** @type {string} */ (reader.readString());
      msg.setSystemdUnitSha256(value);
      break;
    case 34:
      var value = /** @type {string} */ (reader.readString());
      msg.setEffectiveExecStart(value);
      break;
    case 35:
      var value = /** @type {string} */ (reader.readString());
      msg.setEffectiveType(value);
      break;
    case 40:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setCheckedAt(value);
      break;
    case 41:
      var value = /** @type {string} */ (reader.readString());
      msg.addErrors(value);
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
proto.node_agent.ServiceRuntimeProof.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.ServiceRuntimeProof.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.ServiceRuntimeProof} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.ServiceRuntimeProof.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getServiceName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getServiceId();
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
  f = message.getExpectedBuildId();
  if (f.length > 0) {
    writer.writeString(
      10,
      f
    );
  }
  f = message.getExpectedVersion();
  if (f.length > 0) {
    writer.writeString(
      11,
      f
    );
  }
  f = message.getInstalledPath();
  if (f.length > 0) {
    writer.writeString(
      12,
      f
    );
  }
  f = message.getInstalledSha256();
  if (f.length > 0) {
    writer.writeString(
      13,
      f
    );
  }
  f = message.getRunningPid();
  if (f !== 0) {
    writer.writeInt32(
      20,
      f
    );
  }
  f = message.getRunningExePath();
  if (f.length > 0) {
    writer.writeString(
      21,
      f
    );
  }
  f = message.getRunningExeSha256();
  if (f.length > 0) {
    writer.writeString(
      22,
      f
    );
  }
  f = message.getRuntimeVersion();
  if (f.length > 0) {
    writer.writeString(
      23,
      f
    );
  }
  f = message.getRuntimeBuildId();
  if (f.length > 0) {
    writer.writeString(
      24,
      f
    );
  }
  f = message.getProcessStartTime();
  if (f != null) {
    writer.writeMessage(
      25,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getSystemdActiveState();
  if (f.length > 0) {
    writer.writeString(
      30,
      f
    );
  }
  f = message.getSystemdSubState();
  if (f.length > 0) {
    writer.writeString(
      31,
      f
    );
  }
  f = message.getSystemdUnitPath();
  if (f.length > 0) {
    writer.writeString(
      32,
      f
    );
  }
  f = message.getSystemdUnitSha256();
  if (f.length > 0) {
    writer.writeString(
      33,
      f
    );
  }
  f = message.getEffectiveExecStart();
  if (f.length > 0) {
    writer.writeString(
      34,
      f
    );
  }
  f = message.getEffectiveType();
  if (f.length > 0) {
    writer.writeString(
      35,
      f
    );
  }
  f = message.getCheckedAt();
  if (f != null) {
    writer.writeMessage(
      40,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getErrorsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      41,
      f
    );
  }
};


/**
 * optional string service_name = 1;
 * @return {string}
 */
proto.node_agent.ServiceRuntimeProof.prototype.getServiceName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.setServiceName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string service_id = 2;
 * @return {string}
 */
proto.node_agent.ServiceRuntimeProof.prototype.getServiceId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.setServiceId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string node_id = 3;
 * @return {string}
 */
proto.node_agent.ServiceRuntimeProof.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string expected_build_id = 10;
 * @return {string}
 */
proto.node_agent.ServiceRuntimeProof.prototype.getExpectedBuildId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 10, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.setExpectedBuildId = function(value) {
  return jspb.Message.setProto3StringField(this, 10, value);
};


/**
 * optional string expected_version = 11;
 * @return {string}
 */
proto.node_agent.ServiceRuntimeProof.prototype.getExpectedVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 11, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.setExpectedVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 11, value);
};


/**
 * optional string installed_path = 12;
 * @return {string}
 */
proto.node_agent.ServiceRuntimeProof.prototype.getInstalledPath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 12, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.setInstalledPath = function(value) {
  return jspb.Message.setProto3StringField(this, 12, value);
};


/**
 * optional string installed_sha256 = 13;
 * @return {string}
 */
proto.node_agent.ServiceRuntimeProof.prototype.getInstalledSha256 = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 13, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.setInstalledSha256 = function(value) {
  return jspb.Message.setProto3StringField(this, 13, value);
};


/**
 * optional int32 running_pid = 20;
 * @return {number}
 */
proto.node_agent.ServiceRuntimeProof.prototype.getRunningPid = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 20, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.setRunningPid = function(value) {
  return jspb.Message.setProto3IntField(this, 20, value);
};


/**
 * optional string running_exe_path = 21;
 * @return {string}
 */
proto.node_agent.ServiceRuntimeProof.prototype.getRunningExePath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 21, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.setRunningExePath = function(value) {
  return jspb.Message.setProto3StringField(this, 21, value);
};


/**
 * optional string running_exe_sha256 = 22;
 * @return {string}
 */
proto.node_agent.ServiceRuntimeProof.prototype.getRunningExeSha256 = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 22, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.setRunningExeSha256 = function(value) {
  return jspb.Message.setProto3StringField(this, 22, value);
};


/**
 * optional string runtime_version = 23;
 * @return {string}
 */
proto.node_agent.ServiceRuntimeProof.prototype.getRuntimeVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 23, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.setRuntimeVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 23, value);
};


/**
 * optional string runtime_build_id = 24;
 * @return {string}
 */
proto.node_agent.ServiceRuntimeProof.prototype.getRuntimeBuildId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 24, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.setRuntimeBuildId = function(value) {
  return jspb.Message.setProto3StringField(this, 24, value);
};


/**
 * optional google.protobuf.Timestamp process_start_time = 25;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.node_agent.ServiceRuntimeProof.prototype.getProcessStartTime = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 25));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
*/
proto.node_agent.ServiceRuntimeProof.prototype.setProcessStartTime = function(value) {
  return jspb.Message.setWrapperField(this, 25, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.clearProcessStartTime = function() {
  return this.setProcessStartTime(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.node_agent.ServiceRuntimeProof.prototype.hasProcessStartTime = function() {
  return jspb.Message.getField(this, 25) != null;
};


/**
 * optional string systemd_active_state = 30;
 * @return {string}
 */
proto.node_agent.ServiceRuntimeProof.prototype.getSystemdActiveState = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 30, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.setSystemdActiveState = function(value) {
  return jspb.Message.setProto3StringField(this, 30, value);
};


/**
 * optional string systemd_sub_state = 31;
 * @return {string}
 */
proto.node_agent.ServiceRuntimeProof.prototype.getSystemdSubState = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 31, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.setSystemdSubState = function(value) {
  return jspb.Message.setProto3StringField(this, 31, value);
};


/**
 * optional string systemd_unit_path = 32;
 * @return {string}
 */
proto.node_agent.ServiceRuntimeProof.prototype.getSystemdUnitPath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 32, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.setSystemdUnitPath = function(value) {
  return jspb.Message.setProto3StringField(this, 32, value);
};


/**
 * optional string systemd_unit_sha256 = 33;
 * @return {string}
 */
proto.node_agent.ServiceRuntimeProof.prototype.getSystemdUnitSha256 = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 33, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.setSystemdUnitSha256 = function(value) {
  return jspb.Message.setProto3StringField(this, 33, value);
};


/**
 * optional string effective_exec_start = 34;
 * @return {string}
 */
proto.node_agent.ServiceRuntimeProof.prototype.getEffectiveExecStart = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 34, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.setEffectiveExecStart = function(value) {
  return jspb.Message.setProto3StringField(this, 34, value);
};


/**
 * optional string effective_type = 35;
 * @return {string}
 */
proto.node_agent.ServiceRuntimeProof.prototype.getEffectiveType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 35, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.setEffectiveType = function(value) {
  return jspb.Message.setProto3StringField(this, 35, value);
};


/**
 * optional google.protobuf.Timestamp checked_at = 40;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.node_agent.ServiceRuntimeProof.prototype.getCheckedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 40));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
*/
proto.node_agent.ServiceRuntimeProof.prototype.setCheckedAt = function(value) {
  return jspb.Message.setWrapperField(this, 40, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.clearCheckedAt = function() {
  return this.setCheckedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.node_agent.ServiceRuntimeProof.prototype.hasCheckedAt = function() {
  return jspb.Message.getField(this, 40) != null;
};


/**
 * repeated string errors = 41;
 * @return {!Array<string>}
 */
proto.node_agent.ServiceRuntimeProof.prototype.getErrorsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 41));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.setErrorsList = function(value) {
  return jspb.Message.setField(this, 41, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.addErrors = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 41, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.node_agent.ServiceRuntimeProof} returns this
 */
proto.node_agent.ServiceRuntimeProof.prototype.clearErrorsList = function() {
  return this.setErrorsList([]);
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
proto.node_agent.GetServiceRuntimeProofRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.GetServiceRuntimeProofRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.GetServiceRuntimeProofRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetServiceRuntimeProofRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
serviceName: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.node_agent.GetServiceRuntimeProofRequest}
 */
proto.node_agent.GetServiceRuntimeProofRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.GetServiceRuntimeProofRequest;
  return proto.node_agent.GetServiceRuntimeProofRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.GetServiceRuntimeProofRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.GetServiceRuntimeProofRequest}
 */
proto.node_agent.GetServiceRuntimeProofRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setServiceName(value);
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
proto.node_agent.GetServiceRuntimeProofRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.GetServiceRuntimeProofRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.GetServiceRuntimeProofRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetServiceRuntimeProofRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getServiceName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.node_agent.GetServiceRuntimeProofRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.GetServiceRuntimeProofRequest} returns this
 */
proto.node_agent.GetServiceRuntimeProofRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string service_name = 2;
 * @return {string}
 */
proto.node_agent.GetServiceRuntimeProofRequest.prototype.getServiceName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.GetServiceRuntimeProofRequest} returns this
 */
proto.node_agent.GetServiceRuntimeProofRequest.prototype.setServiceName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.node_agent.GetServiceRuntimeProofResponse.repeatedFields_ = [1];



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
proto.node_agent.GetServiceRuntimeProofResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.GetServiceRuntimeProofResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.GetServiceRuntimeProofResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetServiceRuntimeProofResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
proofsList: jspb.Message.toObjectList(msg.getProofsList(),
    proto.node_agent.ServiceRuntimeProof.toObject, includeInstance)
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
 * @return {!proto.node_agent.GetServiceRuntimeProofResponse}
 */
proto.node_agent.GetServiceRuntimeProofResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.GetServiceRuntimeProofResponse;
  return proto.node_agent.GetServiceRuntimeProofResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.GetServiceRuntimeProofResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.GetServiceRuntimeProofResponse}
 */
proto.node_agent.GetServiceRuntimeProofResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.node_agent.ServiceRuntimeProof;
      reader.readMessage(value,proto.node_agent.ServiceRuntimeProof.deserializeBinaryFromReader);
      msg.addProofs(value);
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
proto.node_agent.GetServiceRuntimeProofResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.GetServiceRuntimeProofResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.GetServiceRuntimeProofResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetServiceRuntimeProofResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getProofsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.node_agent.ServiceRuntimeProof.serializeBinaryToWriter
    );
  }
};


/**
 * repeated ServiceRuntimeProof proofs = 1;
 * @return {!Array<!proto.node_agent.ServiceRuntimeProof>}
 */
proto.node_agent.GetServiceRuntimeProofResponse.prototype.getProofsList = function() {
  return /** @type{!Array<!proto.node_agent.ServiceRuntimeProof>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.node_agent.ServiceRuntimeProof, 1));
};


/**
 * @param {!Array<!proto.node_agent.ServiceRuntimeProof>} value
 * @return {!proto.node_agent.GetServiceRuntimeProofResponse} returns this
*/
proto.node_agent.GetServiceRuntimeProofResponse.prototype.setProofsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.node_agent.ServiceRuntimeProof=} opt_value
 * @param {number=} opt_index
 * @return {!proto.node_agent.ServiceRuntimeProof}
 */
proto.node_agent.GetServiceRuntimeProofResponse.prototype.addProofs = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.node_agent.ServiceRuntimeProof, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.node_agent.GetServiceRuntimeProofResponse} returns this
 */
proto.node_agent.GetServiceRuntimeProofResponse.prototype.clearProofsList = function() {
  return this.setProofsList([]);
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
proto.node_agent.VerifyPackageIntegrityRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.VerifyPackageIntegrityRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.VerifyPackageIntegrityRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.VerifyPackageIntegrityRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
packageName: jspb.Message.getFieldWithDefault(msg, 1, ""),
kind: jspb.Message.getFieldWithDefault(msg, 2, ""),
repositoryAddr: jspb.Message.getFieldWithDefault(msg, 3, ""),
nodeId: jspb.Message.getFieldWithDefault(msg, 4, "")
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
 * @return {!proto.node_agent.VerifyPackageIntegrityRequest}
 */
proto.node_agent.VerifyPackageIntegrityRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.VerifyPackageIntegrityRequest;
  return proto.node_agent.VerifyPackageIntegrityRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.VerifyPackageIntegrityRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.VerifyPackageIntegrityRequest}
 */
proto.node_agent.VerifyPackageIntegrityRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPackageName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setKind(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setRepositoryAddr(value);
      break;
    case 4:
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
proto.node_agent.VerifyPackageIntegrityRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.VerifyPackageIntegrityRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.VerifyPackageIntegrityRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.VerifyPackageIntegrityRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPackageName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getKind();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getRepositoryAddr();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * optional string package_name = 1;
 * @return {string}
 */
proto.node_agent.VerifyPackageIntegrityRequest.prototype.getPackageName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.VerifyPackageIntegrityRequest} returns this
 */
proto.node_agent.VerifyPackageIntegrityRequest.prototype.setPackageName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string kind = 2;
 * @return {string}
 */
proto.node_agent.VerifyPackageIntegrityRequest.prototype.getKind = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.VerifyPackageIntegrityRequest} returns this
 */
proto.node_agent.VerifyPackageIntegrityRequest.prototype.setKind = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string repository_addr = 3;
 * @return {string}
 */
proto.node_agent.VerifyPackageIntegrityRequest.prototype.getRepositoryAddr = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.VerifyPackageIntegrityRequest} returns this
 */
proto.node_agent.VerifyPackageIntegrityRequest.prototype.setRepositoryAddr = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string node_id = 4;
 * @return {string}
 */
proto.node_agent.VerifyPackageIntegrityRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.VerifyPackageIntegrityRequest} returns this
 */
proto.node_agent.VerifyPackageIntegrityRequest.prototype.setNodeId = function(value) {
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
proto.node_agent.VerifyPackageIntegrityResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.VerifyPackageIntegrityResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.VerifyPackageIntegrityResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.VerifyPackageIntegrityResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
ok: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
reportJson: jspb.Message.getFieldWithDefault(msg, 2, ""),
findingCount: jspb.Message.getFieldWithDefault(msg, 3, 0),
checkedCount: jspb.Message.getFieldWithDefault(msg, 4, 0),
errorDetail: jspb.Message.getFieldWithDefault(msg, 5, "")
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
 * @return {!proto.node_agent.VerifyPackageIntegrityResponse}
 */
proto.node_agent.VerifyPackageIntegrityResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.VerifyPackageIntegrityResponse;
  return proto.node_agent.VerifyPackageIntegrityResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.VerifyPackageIntegrityResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.VerifyPackageIntegrityResponse}
 */
proto.node_agent.VerifyPackageIntegrityResponse.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setReportJson(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setFindingCount(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setCheckedCount(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setErrorDetail(value);
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
proto.node_agent.VerifyPackageIntegrityResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.VerifyPackageIntegrityResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.VerifyPackageIntegrityResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.VerifyPackageIntegrityResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOk();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getReportJson();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getFindingCount();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
  f = message.getCheckedCount();
  if (f !== 0) {
    writer.writeInt32(
      4,
      f
    );
  }
  f = message.getErrorDetail();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
};


/**
 * optional bool ok = 1;
 * @return {boolean}
 */
proto.node_agent.VerifyPackageIntegrityResponse.prototype.getOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.VerifyPackageIntegrityResponse} returns this
 */
proto.node_agent.VerifyPackageIntegrityResponse.prototype.setOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional string report_json = 2;
 * @return {string}
 */
proto.node_agent.VerifyPackageIntegrityResponse.prototype.getReportJson = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.VerifyPackageIntegrityResponse} returns this
 */
proto.node_agent.VerifyPackageIntegrityResponse.prototype.setReportJson = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int32 finding_count = 3;
 * @return {number}
 */
proto.node_agent.VerifyPackageIntegrityResponse.prototype.getFindingCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.VerifyPackageIntegrityResponse} returns this
 */
proto.node_agent.VerifyPackageIntegrityResponse.prototype.setFindingCount = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional int32 checked_count = 4;
 * @return {number}
 */
proto.node_agent.VerifyPackageIntegrityResponse.prototype.getCheckedCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.VerifyPackageIntegrityResponse} returns this
 */
proto.node_agent.VerifyPackageIntegrityResponse.prototype.setCheckedCount = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional string error_detail = 5;
 * @return {string}
 */
proto.node_agent.VerifyPackageIntegrityResponse.prototype.getErrorDetail = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.VerifyPackageIntegrityResponse} returns this
 */
proto.node_agent.VerifyPackageIntegrityResponse.prototype.setErrorDetail = function(value) {
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
proto.node_agent.RotateNodeTokenRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.RotateNodeTokenRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.RotateNodeTokenRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.RotateNodeTokenRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
newToken: jspb.Message.getFieldWithDefault(msg, 1, ""),
newPrincipal: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.node_agent.RotateNodeTokenRequest}
 */
proto.node_agent.RotateNodeTokenRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.RotateNodeTokenRequest;
  return proto.node_agent.RotateNodeTokenRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.RotateNodeTokenRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.RotateNodeTokenRequest}
 */
proto.node_agent.RotateNodeTokenRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setNewToken(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setNewPrincipal(value);
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
proto.node_agent.RotateNodeTokenRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.RotateNodeTokenRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.RotateNodeTokenRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.RotateNodeTokenRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNewToken();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getNewPrincipal();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string new_token = 1;
 * @return {string}
 */
proto.node_agent.RotateNodeTokenRequest.prototype.getNewToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.RotateNodeTokenRequest} returns this
 */
proto.node_agent.RotateNodeTokenRequest.prototype.setNewToken = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string new_principal = 2;
 * @return {string}
 */
proto.node_agent.RotateNodeTokenRequest.prototype.getNewPrincipal = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.RotateNodeTokenRequest} returns this
 */
proto.node_agent.RotateNodeTokenRequest.prototype.setNewPrincipal = function(value) {
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
proto.node_agent.RotateNodeTokenResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.RotateNodeTokenResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.RotateNodeTokenResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.RotateNodeTokenResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
ok: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.node_agent.RotateNodeTokenResponse}
 */
proto.node_agent.RotateNodeTokenResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.RotateNodeTokenResponse;
  return proto.node_agent.RotateNodeTokenResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.RotateNodeTokenResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.RotateNodeTokenResponse}
 */
proto.node_agent.RotateNodeTokenResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.node_agent.RotateNodeTokenResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.RotateNodeTokenResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.RotateNodeTokenResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.RotateNodeTokenResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOk();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool ok = 1;
 * @return {boolean}
 */
proto.node_agent.RotateNodeTokenResponse.prototype.getOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.RotateNodeTokenResponse} returns this
 */
proto.node_agent.RotateNodeTokenResponse.prototype.setOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.node_agent.GetServiceLogsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.GetServiceLogsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.GetServiceLogsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetServiceLogsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
unit: jspb.Message.getFieldWithDefault(msg, 1, ""),
lines: jspb.Message.getFieldWithDefault(msg, 2, 0),
priority: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.node_agent.GetServiceLogsRequest}
 */
proto.node_agent.GetServiceLogsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.GetServiceLogsRequest;
  return proto.node_agent.GetServiceLogsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.GetServiceLogsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.GetServiceLogsRequest}
 */
proto.node_agent.GetServiceLogsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUnit(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setLines(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setPriority(value);
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
proto.node_agent.GetServiceLogsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.GetServiceLogsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.GetServiceLogsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetServiceLogsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUnit();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getLines();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getPriority();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string unit = 1;
 * @return {string}
 */
proto.node_agent.GetServiceLogsRequest.prototype.getUnit = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.GetServiceLogsRequest} returns this
 */
proto.node_agent.GetServiceLogsRequest.prototype.setUnit = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int32 lines = 2;
 * @return {number}
 */
proto.node_agent.GetServiceLogsRequest.prototype.getLines = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.GetServiceLogsRequest} returns this
 */
proto.node_agent.GetServiceLogsRequest.prototype.setLines = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string priority = 3;
 * @return {string}
 */
proto.node_agent.GetServiceLogsRequest.prototype.getPriority = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.GetServiceLogsRequest} returns this
 */
proto.node_agent.GetServiceLogsRequest.prototype.setPriority = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.node_agent.GetServiceLogsResponse.repeatedFields_ = [3];



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
proto.node_agent.GetServiceLogsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.GetServiceLogsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.GetServiceLogsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetServiceLogsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
unit: jspb.Message.getFieldWithDefault(msg, 1, ""),
lineCount: jspb.Message.getFieldWithDefault(msg, 2, 0),
linesList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f
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
 * @return {!proto.node_agent.GetServiceLogsResponse}
 */
proto.node_agent.GetServiceLogsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.GetServiceLogsResponse;
  return proto.node_agent.GetServiceLogsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.GetServiceLogsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.GetServiceLogsResponse}
 */
proto.node_agent.GetServiceLogsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUnit(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setLineCount(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.addLines(value);
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
proto.node_agent.GetServiceLogsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.GetServiceLogsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.GetServiceLogsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetServiceLogsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUnit();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getLineCount();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getLinesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      3,
      f
    );
  }
};


/**
 * optional string unit = 1;
 * @return {string}
 */
proto.node_agent.GetServiceLogsResponse.prototype.getUnit = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.GetServiceLogsResponse} returns this
 */
proto.node_agent.GetServiceLogsResponse.prototype.setUnit = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int32 line_count = 2;
 * @return {number}
 */
proto.node_agent.GetServiceLogsResponse.prototype.getLineCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.GetServiceLogsResponse} returns this
 */
proto.node_agent.GetServiceLogsResponse.prototype.setLineCount = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * repeated string lines = 3;
 * @return {!Array<string>}
 */
proto.node_agent.GetServiceLogsResponse.prototype.getLinesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.node_agent.GetServiceLogsResponse} returns this
 */
proto.node_agent.GetServiceLogsResponse.prototype.setLinesList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.node_agent.GetServiceLogsResponse} returns this
 */
proto.node_agent.GetServiceLogsResponse.prototype.addLines = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.node_agent.GetServiceLogsResponse} returns this
 */
proto.node_agent.GetServiceLogsResponse.prototype.clearLinesList = function() {
  return this.setLinesList([]);
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
proto.node_agent.SearchServiceLogsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.SearchServiceLogsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.SearchServiceLogsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.SearchServiceLogsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
unit: jspb.Message.getFieldWithDefault(msg, 1, ""),
pattern: jspb.Message.getFieldWithDefault(msg, 2, ""),
since: jspb.Message.getFieldWithDefault(msg, 3, ""),
until: jspb.Message.getFieldWithDefault(msg, 4, ""),
priority: jspb.Message.getFieldWithDefault(msg, 5, ""),
limit: jspb.Message.getFieldWithDefault(msg, 6, 0),
caseSensitive: jspb.Message.getBooleanFieldWithDefault(msg, 7, false)
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
 * @return {!proto.node_agent.SearchServiceLogsRequest}
 */
proto.node_agent.SearchServiceLogsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.SearchServiceLogsRequest;
  return proto.node_agent.SearchServiceLogsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.SearchServiceLogsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.SearchServiceLogsRequest}
 */
proto.node_agent.SearchServiceLogsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUnit(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setPattern(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setSince(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setUntil(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setPriority(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setLimit(value);
      break;
    case 7:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setCaseSensitive(value);
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
proto.node_agent.SearchServiceLogsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.SearchServiceLogsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.SearchServiceLogsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.SearchServiceLogsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUnit();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getPattern();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getSince();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getUntil();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getPriority();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getLimit();
  if (f !== 0) {
    writer.writeInt32(
      6,
      f
    );
  }
  f = message.getCaseSensitive();
  if (f) {
    writer.writeBool(
      7,
      f
    );
  }
};


/**
 * optional string unit = 1;
 * @return {string}
 */
proto.node_agent.SearchServiceLogsRequest.prototype.getUnit = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.SearchServiceLogsRequest} returns this
 */
proto.node_agent.SearchServiceLogsRequest.prototype.setUnit = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string pattern = 2;
 * @return {string}
 */
proto.node_agent.SearchServiceLogsRequest.prototype.getPattern = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.SearchServiceLogsRequest} returns this
 */
proto.node_agent.SearchServiceLogsRequest.prototype.setPattern = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string since = 3;
 * @return {string}
 */
proto.node_agent.SearchServiceLogsRequest.prototype.getSince = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.SearchServiceLogsRequest} returns this
 */
proto.node_agent.SearchServiceLogsRequest.prototype.setSince = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string until = 4;
 * @return {string}
 */
proto.node_agent.SearchServiceLogsRequest.prototype.getUntil = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.SearchServiceLogsRequest} returns this
 */
proto.node_agent.SearchServiceLogsRequest.prototype.setUntil = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string priority = 5;
 * @return {string}
 */
proto.node_agent.SearchServiceLogsRequest.prototype.getPriority = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.SearchServiceLogsRequest} returns this
 */
proto.node_agent.SearchServiceLogsRequest.prototype.setPriority = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional int32 limit = 6;
 * @return {number}
 */
proto.node_agent.SearchServiceLogsRequest.prototype.getLimit = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.SearchServiceLogsRequest} returns this
 */
proto.node_agent.SearchServiceLogsRequest.prototype.setLimit = function(value) {
  return jspb.Message.setProto3IntField(this, 6, value);
};


/**
 * optional bool case_sensitive = 7;
 * @return {boolean}
 */
proto.node_agent.SearchServiceLogsRequest.prototype.getCaseSensitive = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 7, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.SearchServiceLogsRequest} returns this
 */
proto.node_agent.SearchServiceLogsRequest.prototype.setCaseSensitive = function(value) {
  return jspb.Message.setProto3BooleanField(this, 7, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.node_agent.SearchServiceLogsResponse.repeatedFields_ = [3];



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
proto.node_agent.SearchServiceLogsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.SearchServiceLogsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.SearchServiceLogsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.SearchServiceLogsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
unit: jspb.Message.getFieldWithDefault(msg, 1, ""),
matchCount: jspb.Message.getFieldWithDefault(msg, 2, 0),
linesList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f,
since: jspb.Message.getFieldWithDefault(msg, 4, ""),
until: jspb.Message.getFieldWithDefault(msg, 5, ""),
truncated: jspb.Message.getBooleanFieldWithDefault(msg, 6, false)
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
 * @return {!proto.node_agent.SearchServiceLogsResponse}
 */
proto.node_agent.SearchServiceLogsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.SearchServiceLogsResponse;
  return proto.node_agent.SearchServiceLogsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.SearchServiceLogsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.SearchServiceLogsResponse}
 */
proto.node_agent.SearchServiceLogsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUnit(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setMatchCount(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.addLines(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setSince(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setUntil(value);
      break;
    case 6:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setTruncated(value);
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
proto.node_agent.SearchServiceLogsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.SearchServiceLogsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.SearchServiceLogsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.SearchServiceLogsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUnit();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getMatchCount();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getLinesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      3,
      f
    );
  }
  f = message.getSince();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getUntil();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getTruncated();
  if (f) {
    writer.writeBool(
      6,
      f
    );
  }
};


/**
 * optional string unit = 1;
 * @return {string}
 */
proto.node_agent.SearchServiceLogsResponse.prototype.getUnit = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.SearchServiceLogsResponse} returns this
 */
proto.node_agent.SearchServiceLogsResponse.prototype.setUnit = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int32 match_count = 2;
 * @return {number}
 */
proto.node_agent.SearchServiceLogsResponse.prototype.getMatchCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.SearchServiceLogsResponse} returns this
 */
proto.node_agent.SearchServiceLogsResponse.prototype.setMatchCount = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * repeated string lines = 3;
 * @return {!Array<string>}
 */
proto.node_agent.SearchServiceLogsResponse.prototype.getLinesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.node_agent.SearchServiceLogsResponse} returns this
 */
proto.node_agent.SearchServiceLogsResponse.prototype.setLinesList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.node_agent.SearchServiceLogsResponse} returns this
 */
proto.node_agent.SearchServiceLogsResponse.prototype.addLines = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.node_agent.SearchServiceLogsResponse} returns this
 */
proto.node_agent.SearchServiceLogsResponse.prototype.clearLinesList = function() {
  return this.setLinesList([]);
};


/**
 * optional string since = 4;
 * @return {string}
 */
proto.node_agent.SearchServiceLogsResponse.prototype.getSince = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.SearchServiceLogsResponse} returns this
 */
proto.node_agent.SearchServiceLogsResponse.prototype.setSince = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string until = 5;
 * @return {string}
 */
proto.node_agent.SearchServiceLogsResponse.prototype.getUntil = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.SearchServiceLogsResponse} returns this
 */
proto.node_agent.SearchServiceLogsResponse.prototype.setUntil = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional bool truncated = 6;
 * @return {boolean}
 */
proto.node_agent.SearchServiceLogsResponse.prototype.getTruncated = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 6, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.SearchServiceLogsResponse} returns this
 */
proto.node_agent.SearchServiceLogsResponse.prototype.setTruncated = function(value) {
  return jspb.Message.setProto3BooleanField(this, 6, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.node_agent.CertificateInfo.repeatedFields_ = [3];



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
proto.node_agent.CertificateInfo.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.CertificateInfo.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.CertificateInfo} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.CertificateInfo.toObject = function(includeInstance, msg) {
  var f, obj = {
subject: jspb.Message.getFieldWithDefault(msg, 1, ""),
issuer: jspb.Message.getFieldWithDefault(msg, 2, ""),
sansList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f,
notBefore: jspb.Message.getFieldWithDefault(msg, 4, ""),
notAfter: jspb.Message.getFieldWithDefault(msg, 5, ""),
daysUntilExpiry: jspb.Message.getFieldWithDefault(msg, 6, 0),
chainValid: jspb.Message.getBooleanFieldWithDefault(msg, 7, false),
fingerprint: jspb.Message.getFieldWithDefault(msg, 8, "")
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
 * @return {!proto.node_agent.CertificateInfo}
 */
proto.node_agent.CertificateInfo.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.CertificateInfo;
  return proto.node_agent.CertificateInfo.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.CertificateInfo} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.CertificateInfo}
 */
proto.node_agent.CertificateInfo.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setSubject(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIssuer(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.addSans(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setNotBefore(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setNotAfter(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setDaysUntilExpiry(value);
      break;
    case 7:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setChainValid(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setFingerprint(value);
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
proto.node_agent.CertificateInfo.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.CertificateInfo.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.CertificateInfo} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.CertificateInfo.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSubject();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIssuer();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getSansList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      3,
      f
    );
  }
  f = message.getNotBefore();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getNotAfter();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getDaysUntilExpiry();
  if (f !== 0) {
    writer.writeInt32(
      6,
      f
    );
  }
  f = message.getChainValid();
  if (f) {
    writer.writeBool(
      7,
      f
    );
  }
  f = message.getFingerprint();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
};


/**
 * optional string subject = 1;
 * @return {string}
 */
proto.node_agent.CertificateInfo.prototype.getSubject = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.CertificateInfo} returns this
 */
proto.node_agent.CertificateInfo.prototype.setSubject = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string issuer = 2;
 * @return {string}
 */
proto.node_agent.CertificateInfo.prototype.getIssuer = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.CertificateInfo} returns this
 */
proto.node_agent.CertificateInfo.prototype.setIssuer = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * repeated string sans = 3;
 * @return {!Array<string>}
 */
proto.node_agent.CertificateInfo.prototype.getSansList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.node_agent.CertificateInfo} returns this
 */
proto.node_agent.CertificateInfo.prototype.setSansList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.node_agent.CertificateInfo} returns this
 */
proto.node_agent.CertificateInfo.prototype.addSans = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.node_agent.CertificateInfo} returns this
 */
proto.node_agent.CertificateInfo.prototype.clearSansList = function() {
  return this.setSansList([]);
};


/**
 * optional string not_before = 4;
 * @return {string}
 */
proto.node_agent.CertificateInfo.prototype.getNotBefore = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.CertificateInfo} returns this
 */
proto.node_agent.CertificateInfo.prototype.setNotBefore = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string not_after = 5;
 * @return {string}
 */
proto.node_agent.CertificateInfo.prototype.getNotAfter = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.CertificateInfo} returns this
 */
proto.node_agent.CertificateInfo.prototype.setNotAfter = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional int32 days_until_expiry = 6;
 * @return {number}
 */
proto.node_agent.CertificateInfo.prototype.getDaysUntilExpiry = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.CertificateInfo} returns this
 */
proto.node_agent.CertificateInfo.prototype.setDaysUntilExpiry = function(value) {
  return jspb.Message.setProto3IntField(this, 6, value);
};


/**
 * optional bool chain_valid = 7;
 * @return {boolean}
 */
proto.node_agent.CertificateInfo.prototype.getChainValid = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 7, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.CertificateInfo} returns this
 */
proto.node_agent.CertificateInfo.prototype.setChainValid = function(value) {
  return jspb.Message.setProto3BooleanField(this, 7, value);
};


/**
 * optional string fingerprint = 8;
 * @return {string}
 */
proto.node_agent.CertificateInfo.prototype.getFingerprint = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.CertificateInfo} returns this
 */
proto.node_agent.CertificateInfo.prototype.setFingerprint = function(value) {
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
proto.node_agent.ControlServiceRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.ControlServiceRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.ControlServiceRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.ControlServiceRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
unit: jspb.Message.getFieldWithDefault(msg, 1, ""),
action: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.node_agent.ControlServiceRequest}
 */
proto.node_agent.ControlServiceRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.ControlServiceRequest;
  return proto.node_agent.ControlServiceRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.ControlServiceRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.ControlServiceRequest}
 */
proto.node_agent.ControlServiceRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUnit(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setAction(value);
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
proto.node_agent.ControlServiceRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.ControlServiceRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.ControlServiceRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.ControlServiceRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUnit();
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
};


/**
 * optional string unit = 1;
 * @return {string}
 */
proto.node_agent.ControlServiceRequest.prototype.getUnit = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ControlServiceRequest} returns this
 */
proto.node_agent.ControlServiceRequest.prototype.setUnit = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string action = 2;
 * @return {string}
 */
proto.node_agent.ControlServiceRequest.prototype.getAction = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ControlServiceRequest} returns this
 */
proto.node_agent.ControlServiceRequest.prototype.setAction = function(value) {
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
proto.node_agent.ControlServiceResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.ControlServiceResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.ControlServiceResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.ControlServiceResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
ok: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
unit: jspb.Message.getFieldWithDefault(msg, 2, ""),
action: jspb.Message.getFieldWithDefault(msg, 3, ""),
state: jspb.Message.getFieldWithDefault(msg, 4, ""),
message: jspb.Message.getFieldWithDefault(msg, 5, "")
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
 * @return {!proto.node_agent.ControlServiceResponse}
 */
proto.node_agent.ControlServiceResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.ControlServiceResponse;
  return proto.node_agent.ControlServiceResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.ControlServiceResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.ControlServiceResponse}
 */
proto.node_agent.ControlServiceResponse.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setUnit(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setAction(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setState(value);
      break;
    case 5:
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
proto.node_agent.ControlServiceResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.ControlServiceResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.ControlServiceResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.ControlServiceResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOk();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getUnit();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getAction();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getState();
  if (f.length > 0) {
    writer.writeString(
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
};


/**
 * optional bool ok = 1;
 * @return {boolean}
 */
proto.node_agent.ControlServiceResponse.prototype.getOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.ControlServiceResponse} returns this
 */
proto.node_agent.ControlServiceResponse.prototype.setOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional string unit = 2;
 * @return {string}
 */
proto.node_agent.ControlServiceResponse.prototype.getUnit = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ControlServiceResponse} returns this
 */
proto.node_agent.ControlServiceResponse.prototype.setUnit = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string action = 3;
 * @return {string}
 */
proto.node_agent.ControlServiceResponse.prototype.getAction = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ControlServiceResponse} returns this
 */
proto.node_agent.ControlServiceResponse.prototype.setAction = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string state = 4;
 * @return {string}
 */
proto.node_agent.ControlServiceResponse.prototype.getState = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ControlServiceResponse} returns this
 */
proto.node_agent.ControlServiceResponse.prototype.setState = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string message = 5;
 * @return {string}
 */
proto.node_agent.ControlServiceResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ControlServiceResponse} returns this
 */
proto.node_agent.ControlServiceResponse.prototype.setMessage = function(value) {
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
proto.node_agent.GetCertificateStatusRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.GetCertificateStatusRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.GetCertificateStatusRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetCertificateStatusRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.node_agent.GetCertificateStatusRequest}
 */
proto.node_agent.GetCertificateStatusRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.GetCertificateStatusRequest;
  return proto.node_agent.GetCertificateStatusRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.GetCertificateStatusRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.GetCertificateStatusRequest}
 */
proto.node_agent.GetCertificateStatusRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.node_agent.GetCertificateStatusRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.GetCertificateStatusRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.GetCertificateStatusRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetCertificateStatusRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.node_agent.GetCertificateStatusResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.GetCertificateStatusResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.GetCertificateStatusResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetCertificateStatusResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
serverCert: (f = msg.getServerCert()) && proto.node_agent.CertificateInfo.toObject(includeInstance, f),
caCert: (f = msg.getCaCert()) && proto.node_agent.CertificateInfo.toObject(includeInstance, f)
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
 * @return {!proto.node_agent.GetCertificateStatusResponse}
 */
proto.node_agent.GetCertificateStatusResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.GetCertificateStatusResponse;
  return proto.node_agent.GetCertificateStatusResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.GetCertificateStatusResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.GetCertificateStatusResponse}
 */
proto.node_agent.GetCertificateStatusResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.node_agent.CertificateInfo;
      reader.readMessage(value,proto.node_agent.CertificateInfo.deserializeBinaryFromReader);
      msg.setServerCert(value);
      break;
    case 2:
      var value = new proto.node_agent.CertificateInfo;
      reader.readMessage(value,proto.node_agent.CertificateInfo.deserializeBinaryFromReader);
      msg.setCaCert(value);
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
proto.node_agent.GetCertificateStatusResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.GetCertificateStatusResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.GetCertificateStatusResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetCertificateStatusResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getServerCert();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.node_agent.CertificateInfo.serializeBinaryToWriter
    );
  }
  f = message.getCaCert();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.node_agent.CertificateInfo.serializeBinaryToWriter
    );
  }
};


/**
 * optional CertificateInfo server_cert = 1;
 * @return {?proto.node_agent.CertificateInfo}
 */
proto.node_agent.GetCertificateStatusResponse.prototype.getServerCert = function() {
  return /** @type{?proto.node_agent.CertificateInfo} */ (
    jspb.Message.getWrapperField(this, proto.node_agent.CertificateInfo, 1));
};


/**
 * @param {?proto.node_agent.CertificateInfo|undefined} value
 * @return {!proto.node_agent.GetCertificateStatusResponse} returns this
*/
proto.node_agent.GetCertificateStatusResponse.prototype.setServerCert = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.node_agent.GetCertificateStatusResponse} returns this
 */
proto.node_agent.GetCertificateStatusResponse.prototype.clearServerCert = function() {
  return this.setServerCert(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.node_agent.GetCertificateStatusResponse.prototype.hasServerCert = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional CertificateInfo ca_cert = 2;
 * @return {?proto.node_agent.CertificateInfo}
 */
proto.node_agent.GetCertificateStatusResponse.prototype.getCaCert = function() {
  return /** @type{?proto.node_agent.CertificateInfo} */ (
    jspb.Message.getWrapperField(this, proto.node_agent.CertificateInfo, 2));
};


/**
 * @param {?proto.node_agent.CertificateInfo|undefined} value
 * @return {!proto.node_agent.GetCertificateStatusResponse} returns this
*/
proto.node_agent.GetCertificateStatusResponse.prototype.setCaCert = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.node_agent.GetCertificateStatusResponse} returns this
 */
proto.node_agent.GetCertificateStatusResponse.prototype.clearCaCert = function() {
  return this.setCaCert(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.node_agent.GetCertificateStatusResponse.prototype.hasCaCert = function() {
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
proto.node_agent.SubsystemHealth.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.SubsystemHealth.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.SubsystemHealth} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.SubsystemHealth.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, ""),
state: jspb.Message.getFieldWithDefault(msg, 2, 0),
lastTick: (f = msg.getLastTick()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
lastError: jspb.Message.getFieldWithDefault(msg, 4, ""),
errorCount: jspb.Message.getFieldWithDefault(msg, 5, 0),
metadataMap: (f = msg.getMetadataMap()) ? f.toObject(includeInstance, undefined) : []
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
 * @return {!proto.node_agent.SubsystemHealth}
 */
proto.node_agent.SubsystemHealth.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.SubsystemHealth;
  return proto.node_agent.SubsystemHealth.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.SubsystemHealth} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.SubsystemHealth}
 */
proto.node_agent.SubsystemHealth.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {!proto.node_agent.SubsystemState} */ (reader.readEnum());
      msg.setState(value);
      break;
    case 3:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setLastTick(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setLastError(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setErrorCount(value);
      break;
    case 6:
      var value = msg.getMetadataMap();
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
proto.node_agent.SubsystemHealth.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.SubsystemHealth.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.SubsystemHealth} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.SubsystemHealth.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getState();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
  f = message.getLastTick();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getLastError();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getErrorCount();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
  f = message.getMetadataMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(6, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.node_agent.SubsystemHealth.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.SubsystemHealth} returns this
 */
proto.node_agent.SubsystemHealth.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional SubsystemState state = 2;
 * @return {!proto.node_agent.SubsystemState}
 */
proto.node_agent.SubsystemHealth.prototype.getState = function() {
  return /** @type {!proto.node_agent.SubsystemState} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.node_agent.SubsystemState} value
 * @return {!proto.node_agent.SubsystemHealth} returns this
 */
proto.node_agent.SubsystemHealth.prototype.setState = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional google.protobuf.Timestamp last_tick = 3;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.node_agent.SubsystemHealth.prototype.getLastTick = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 3));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.node_agent.SubsystemHealth} returns this
*/
proto.node_agent.SubsystemHealth.prototype.setLastTick = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.node_agent.SubsystemHealth} returns this
 */
proto.node_agent.SubsystemHealth.prototype.clearLastTick = function() {
  return this.setLastTick(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.node_agent.SubsystemHealth.prototype.hasLastTick = function() {
  return jspb.Message.getField(this, 3) != null;
};


/**
 * optional string last_error = 4;
 * @return {string}
 */
proto.node_agent.SubsystemHealth.prototype.getLastError = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.SubsystemHealth} returns this
 */
proto.node_agent.SubsystemHealth.prototype.setLastError = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional int64 error_count = 5;
 * @return {number}
 */
proto.node_agent.SubsystemHealth.prototype.getErrorCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.SubsystemHealth} returns this
 */
proto.node_agent.SubsystemHealth.prototype.setErrorCount = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * map<string, string> metadata = 6;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.node_agent.SubsystemHealth.prototype.getMetadataMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 6, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.node_agent.SubsystemHealth} returns this
 */
proto.node_agent.SubsystemHealth.prototype.clearMetadataMap = function() {
  this.getMetadataMap().clear();
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
proto.node_agent.GetSubsystemHealthRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.GetSubsystemHealthRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.GetSubsystemHealthRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetSubsystemHealthRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.node_agent.GetSubsystemHealthRequest}
 */
proto.node_agent.GetSubsystemHealthRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.GetSubsystemHealthRequest;
  return proto.node_agent.GetSubsystemHealthRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.GetSubsystemHealthRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.GetSubsystemHealthRequest}
 */
proto.node_agent.GetSubsystemHealthRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.node_agent.GetSubsystemHealthRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.GetSubsystemHealthRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.GetSubsystemHealthRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetSubsystemHealthRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.node_agent.GetSubsystemHealthResponse.repeatedFields_ = [1];



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
proto.node_agent.GetSubsystemHealthResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.GetSubsystemHealthResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.GetSubsystemHealthResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetSubsystemHealthResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
subsystemsList: jspb.Message.toObjectList(msg.getSubsystemsList(),
    proto.node_agent.SubsystemHealth.toObject, includeInstance),
overall: jspb.Message.getFieldWithDefault(msg, 2, 0)
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
 * @return {!proto.node_agent.GetSubsystemHealthResponse}
 */
proto.node_agent.GetSubsystemHealthResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.GetSubsystemHealthResponse;
  return proto.node_agent.GetSubsystemHealthResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.GetSubsystemHealthResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.GetSubsystemHealthResponse}
 */
proto.node_agent.GetSubsystemHealthResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.node_agent.SubsystemHealth;
      reader.readMessage(value,proto.node_agent.SubsystemHealth.deserializeBinaryFromReader);
      msg.addSubsystems(value);
      break;
    case 2:
      var value = /** @type {!proto.node_agent.SubsystemState} */ (reader.readEnum());
      msg.setOverall(value);
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
proto.node_agent.GetSubsystemHealthResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.GetSubsystemHealthResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.GetSubsystemHealthResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetSubsystemHealthResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSubsystemsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.node_agent.SubsystemHealth.serializeBinaryToWriter
    );
  }
  f = message.getOverall();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
};


/**
 * repeated SubsystemHealth subsystems = 1;
 * @return {!Array<!proto.node_agent.SubsystemHealth>}
 */
proto.node_agent.GetSubsystemHealthResponse.prototype.getSubsystemsList = function() {
  return /** @type{!Array<!proto.node_agent.SubsystemHealth>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.node_agent.SubsystemHealth, 1));
};


/**
 * @param {!Array<!proto.node_agent.SubsystemHealth>} value
 * @return {!proto.node_agent.GetSubsystemHealthResponse} returns this
*/
proto.node_agent.GetSubsystemHealthResponse.prototype.setSubsystemsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.node_agent.SubsystemHealth=} opt_value
 * @param {number=} opt_index
 * @return {!proto.node_agent.SubsystemHealth}
 */
proto.node_agent.GetSubsystemHealthResponse.prototype.addSubsystems = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.node_agent.SubsystemHealth, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.node_agent.GetSubsystemHealthResponse} returns this
 */
proto.node_agent.GetSubsystemHealthResponse.prototype.clearSubsystemsList = function() {
  return this.setSubsystemsList([]);
};


/**
 * optional SubsystemState overall = 2;
 * @return {!proto.node_agent.SubsystemState}
 */
proto.node_agent.GetSubsystemHealthResponse.prototype.getOverall = function() {
  return /** @type {!proto.node_agent.SubsystemState} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.node_agent.SubsystemState} value
 * @return {!proto.node_agent.GetSubsystemHealthResponse} returns this
 */
proto.node_agent.GetSubsystemHealthResponse.prototype.setOverall = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
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
proto.node_agent.GetInfraProbeRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.GetInfraProbeRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.GetInfraProbeRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetInfraProbeRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
component: jspb.Message.getFieldWithDefault(msg, 2, ""),
bypassCache: jspb.Message.getBooleanFieldWithDefault(msg, 3, false)
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
 * @return {!proto.node_agent.GetInfraProbeRequest}
 */
proto.node_agent.GetInfraProbeRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.GetInfraProbeRequest;
  return proto.node_agent.GetInfraProbeRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.GetInfraProbeRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.GetInfraProbeRequest}
 */
proto.node_agent.GetInfraProbeRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setComponent(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setBypassCache(value);
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
proto.node_agent.GetInfraProbeRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.GetInfraProbeRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.GetInfraProbeRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetInfraProbeRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getComponent();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getBypassCache();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.node_agent.GetInfraProbeRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.GetInfraProbeRequest} returns this
 */
proto.node_agent.GetInfraProbeRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string component = 2;
 * @return {string}
 */
proto.node_agent.GetInfraProbeRequest.prototype.getComponent = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.GetInfraProbeRequest} returns this
 */
proto.node_agent.GetInfraProbeRequest.prototype.setComponent = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional bool bypass_cache = 3;
 * @return {boolean}
 */
proto.node_agent.GetInfraProbeRequest.prototype.getBypassCache = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.GetInfraProbeRequest} returns this
 */
proto.node_agent.GetInfraProbeRequest.prototype.setBypassCache = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.node_agent.GetInfraProbeResponse.repeatedFields_ = [1];



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
proto.node_agent.GetInfraProbeResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.GetInfraProbeResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.GetInfraProbeResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetInfraProbeResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
resultsList: jspb.Message.toObjectList(msg.getResultsList(),
    cluster_controller_pb.InfraProbeResult.toObject, includeInstance)
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
 * @return {!proto.node_agent.GetInfraProbeResponse}
 */
proto.node_agent.GetInfraProbeResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.GetInfraProbeResponse;
  return proto.node_agent.GetInfraProbeResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.GetInfraProbeResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.GetInfraProbeResponse}
 */
proto.node_agent.GetInfraProbeResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new cluster_controller_pb.InfraProbeResult;
      reader.readMessage(value,cluster_controller_pb.InfraProbeResult.deserializeBinaryFromReader);
      msg.addResults(value);
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
proto.node_agent.GetInfraProbeResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.GetInfraProbeResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.GetInfraProbeResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.GetInfraProbeResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResultsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      cluster_controller_pb.InfraProbeResult.serializeBinaryToWriter
    );
  }
};


/**
 * repeated cluster_controller.InfraProbeResult results = 1;
 * @return {!Array<!proto.cluster_controller.InfraProbeResult>}
 */
proto.node_agent.GetInfraProbeResponse.prototype.getResultsList = function() {
  return /** @type{!Array<!proto.cluster_controller.InfraProbeResult>} */ (
    jspb.Message.getRepeatedWrapperField(this, cluster_controller_pb.InfraProbeResult, 1));
};


/**
 * @param {!Array<!proto.cluster_controller.InfraProbeResult>} value
 * @return {!proto.node_agent.GetInfraProbeResponse} returns this
*/
proto.node_agent.GetInfraProbeResponse.prototype.setResultsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.cluster_controller.InfraProbeResult=} opt_value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.InfraProbeResult}
 */
proto.node_agent.GetInfraProbeResponse.prototype.addResults = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.cluster_controller.InfraProbeResult, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.node_agent.GetInfraProbeResponse} returns this
 */
proto.node_agent.GetInfraProbeResponse.prototype.clearResultsList = function() {
  return this.setResultsList([]);
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
proto.node_agent.RunWorkflowRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.RunWorkflowRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.RunWorkflowRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.RunWorkflowRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
workflowName: jspb.Message.getFieldWithDefault(msg, 1, ""),
definitionPath: jspb.Message.getFieldWithDefault(msg, 2, ""),
inputsMap: (f = msg.getInputsMap()) ? f.toObject(includeInstance, undefined) : []
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
 * @return {!proto.node_agent.RunWorkflowRequest}
 */
proto.node_agent.RunWorkflowRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.RunWorkflowRequest;
  return proto.node_agent.RunWorkflowRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.RunWorkflowRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.RunWorkflowRequest}
 */
proto.node_agent.RunWorkflowRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setWorkflowName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setDefinitionPath(value);
      break;
    case 3:
      var value = msg.getInputsMap();
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
proto.node_agent.RunWorkflowRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.RunWorkflowRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.RunWorkflowRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.RunWorkflowRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getWorkflowName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getDefinitionPath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getInputsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(3, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
};


/**
 * optional string workflow_name = 1;
 * @return {string}
 */
proto.node_agent.RunWorkflowRequest.prototype.getWorkflowName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.RunWorkflowRequest} returns this
 */
proto.node_agent.RunWorkflowRequest.prototype.setWorkflowName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string definition_path = 2;
 * @return {string}
 */
proto.node_agent.RunWorkflowRequest.prototype.getDefinitionPath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.RunWorkflowRequest} returns this
 */
proto.node_agent.RunWorkflowRequest.prototype.setDefinitionPath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * map<string, string> inputs = 3;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.node_agent.RunWorkflowRequest.prototype.getInputsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 3, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.node_agent.RunWorkflowRequest} returns this
 */
proto.node_agent.RunWorkflowRequest.prototype.clearInputsMap = function() {
  this.getInputsMap().clear();
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
proto.node_agent.RunWorkflowResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.RunWorkflowResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.RunWorkflowResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.RunWorkflowResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
runId: jspb.Message.getFieldWithDefault(msg, 1, ""),
status: jspb.Message.getFieldWithDefault(msg, 2, ""),
stepsSucceeded: jspb.Message.getFieldWithDefault(msg, 3, 0),
stepsFailed: jspb.Message.getFieldWithDefault(msg, 4, 0),
stepsTotal: jspb.Message.getFieldWithDefault(msg, 5, 0),
error: jspb.Message.getFieldWithDefault(msg, 6, ""),
durationMs: jspb.Message.getFieldWithDefault(msg, 7, 0)
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
 * @return {!proto.node_agent.RunWorkflowResponse}
 */
proto.node_agent.RunWorkflowResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.RunWorkflowResponse;
  return proto.node_agent.RunWorkflowResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.RunWorkflowResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.RunWorkflowResponse}
 */
proto.node_agent.RunWorkflowResponse.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setStatus(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setStepsSucceeded(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setStepsFailed(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setStepsTotal(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setError(value);
      break;
    case 7:
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
proto.node_agent.RunWorkflowResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.RunWorkflowResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.RunWorkflowResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.RunWorkflowResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRunId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getStatus();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getStepsSucceeded();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
  f = message.getStepsFailed();
  if (f !== 0) {
    writer.writeInt32(
      4,
      f
    );
  }
  f = message.getStepsTotal();
  if (f !== 0) {
    writer.writeInt32(
      5,
      f
    );
  }
  f = message.getError();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getDurationMs();
  if (f !== 0) {
    writer.writeInt64(
      7,
      f
    );
  }
};


/**
 * optional string run_id = 1;
 * @return {string}
 */
proto.node_agent.RunWorkflowResponse.prototype.getRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.RunWorkflowResponse} returns this
 */
proto.node_agent.RunWorkflowResponse.prototype.setRunId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string status = 2;
 * @return {string}
 */
proto.node_agent.RunWorkflowResponse.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.RunWorkflowResponse} returns this
 */
proto.node_agent.RunWorkflowResponse.prototype.setStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int32 steps_succeeded = 3;
 * @return {number}
 */
proto.node_agent.RunWorkflowResponse.prototype.getStepsSucceeded = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.RunWorkflowResponse} returns this
 */
proto.node_agent.RunWorkflowResponse.prototype.setStepsSucceeded = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional int32 steps_failed = 4;
 * @return {number}
 */
proto.node_agent.RunWorkflowResponse.prototype.getStepsFailed = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.RunWorkflowResponse} returns this
 */
proto.node_agent.RunWorkflowResponse.prototype.setStepsFailed = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional int32 steps_total = 5;
 * @return {number}
 */
proto.node_agent.RunWorkflowResponse.prototype.getStepsTotal = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.RunWorkflowResponse} returns this
 */
proto.node_agent.RunWorkflowResponse.prototype.setStepsTotal = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional string error = 6;
 * @return {string}
 */
proto.node_agent.RunWorkflowResponse.prototype.getError = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.RunWorkflowResponse} returns this
 */
proto.node_agent.RunWorkflowResponse.prototype.setError = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional int64 duration_ms = 7;
 * @return {number}
 */
proto.node_agent.RunWorkflowResponse.prototype.getDurationMs = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.RunWorkflowResponse} returns this
 */
proto.node_agent.RunWorkflowResponse.prototype.setDurationMs = function(value) {
  return jspb.Message.setProto3IntField(this, 7, value);
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
proto.node_agent.ApplyPackageReleaseRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.ApplyPackageReleaseRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.ApplyPackageReleaseRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.ApplyPackageReleaseRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
packageName: jspb.Message.getFieldWithDefault(msg, 1, ""),
packageKind: jspb.Message.getFieldWithDefault(msg, 2, ""),
version: jspb.Message.getFieldWithDefault(msg, 3, ""),
publisher: jspb.Message.getFieldWithDefault(msg, 4, ""),
platform: jspb.Message.getFieldWithDefault(msg, 5, ""),
force: jspb.Message.getBooleanFieldWithDefault(msg, 6, false),
expectedSha256: jspb.Message.getFieldWithDefault(msg, 7, ""),
operationId: jspb.Message.getFieldWithDefault(msg, 8, ""),
repositoryAddr: jspb.Message.getFieldWithDefault(msg, 9, ""),
buildNumber: jspb.Message.getFieldWithDefault(msg, 10, 0),
buildId: jspb.Message.getFieldWithDefault(msg, 11, ""),
rollbackMode: jspb.Message.getBooleanFieldWithDefault(msg, 12, false),
rollbackReason: jspb.Message.getFieldWithDefault(msg, 13, ""),
workflowRunId: jspb.Message.getFieldWithDefault(msg, 14, ""),
targetRevisionId: jspb.Message.getFieldWithDefault(msg, 15, ""),
preserveConfigs: jspb.Message.getBooleanFieldWithDefault(msg, 16, false),
restoreConfigSnapshot: jspb.Message.getBooleanFieldWithDefault(msg, 17, false),
allowDowngrade: jspb.Message.getBooleanFieldWithDefault(msg, 18, false),
previousRevisionId: jspb.Message.getFieldWithDefault(msg, 19, "")
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
 * @return {!proto.node_agent.ApplyPackageReleaseRequest}
 */
proto.node_agent.ApplyPackageReleaseRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.ApplyPackageReleaseRequest;
  return proto.node_agent.ApplyPackageReleaseRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.ApplyPackageReleaseRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.ApplyPackageReleaseRequest}
 */
proto.node_agent.ApplyPackageReleaseRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPackageName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setPackageKind(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setVersion(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisher(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlatform(value);
      break;
    case 6:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setForce(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setExpectedSha256(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setOperationId(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setRepositoryAddr(value);
      break;
    case 10:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    case 11:
      var value = /** @type {string} */ (reader.readString());
      msg.setBuildId(value);
      break;
    case 12:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setRollbackMode(value);
      break;
    case 13:
      var value = /** @type {string} */ (reader.readString());
      msg.setRollbackReason(value);
      break;
    case 14:
      var value = /** @type {string} */ (reader.readString());
      msg.setWorkflowRunId(value);
      break;
    case 15:
      var value = /** @type {string} */ (reader.readString());
      msg.setTargetRevisionId(value);
      break;
    case 16:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setPreserveConfigs(value);
      break;
    case 17:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setRestoreConfigSnapshot(value);
      break;
    case 18:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setAllowDowngrade(value);
      break;
    case 19:
      var value = /** @type {string} */ (reader.readString());
      msg.setPreviousRevisionId(value);
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
proto.node_agent.ApplyPackageReleaseRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.ApplyPackageReleaseRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.ApplyPackageReleaseRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.ApplyPackageReleaseRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPackageName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getPackageKind();
  if (f.length > 0) {
    writer.writeString(
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
  f = message.getPublisher();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getPlatform();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getForce();
  if (f) {
    writer.writeBool(
      6,
      f
    );
  }
  f = message.getExpectedSha256();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getOperationId();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
  f = message.getRepositoryAddr();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
  f = message.getBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
      10,
      f
    );
  }
  f = message.getBuildId();
  if (f.length > 0) {
    writer.writeString(
      11,
      f
    );
  }
  f = message.getRollbackMode();
  if (f) {
    writer.writeBool(
      12,
      f
    );
  }
  f = message.getRollbackReason();
  if (f.length > 0) {
    writer.writeString(
      13,
      f
    );
  }
  f = message.getWorkflowRunId();
  if (f.length > 0) {
    writer.writeString(
      14,
      f
    );
  }
  f = message.getTargetRevisionId();
  if (f.length > 0) {
    writer.writeString(
      15,
      f
    );
  }
  f = message.getPreserveConfigs();
  if (f) {
    writer.writeBool(
      16,
      f
    );
  }
  f = message.getRestoreConfigSnapshot();
  if (f) {
    writer.writeBool(
      17,
      f
    );
  }
  f = message.getAllowDowngrade();
  if (f) {
    writer.writeBool(
      18,
      f
    );
  }
  f = message.getPreviousRevisionId();
  if (f.length > 0) {
    writer.writeString(
      19,
      f
    );
  }
};


/**
 * optional string package_name = 1;
 * @return {string}
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.getPackageName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ApplyPackageReleaseRequest} returns this
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.setPackageName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string package_kind = 2;
 * @return {string}
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.getPackageKind = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ApplyPackageReleaseRequest} returns this
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.setPackageKind = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string version = 3;
 * @return {string}
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ApplyPackageReleaseRequest} returns this
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string publisher = 4;
 * @return {string}
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.getPublisher = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ApplyPackageReleaseRequest} returns this
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.setPublisher = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string platform = 5;
 * @return {string}
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.getPlatform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ApplyPackageReleaseRequest} returns this
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.setPlatform = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional bool force = 6;
 * @return {boolean}
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.getForce = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 6, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.ApplyPackageReleaseRequest} returns this
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.setForce = function(value) {
  return jspb.Message.setProto3BooleanField(this, 6, value);
};


/**
 * optional string expected_sha256 = 7;
 * @return {string}
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.getExpectedSha256 = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ApplyPackageReleaseRequest} returns this
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.setExpectedSha256 = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional string operation_id = 8;
 * @return {string}
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.getOperationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ApplyPackageReleaseRequest} returns this
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.setOperationId = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};


/**
 * optional string repository_addr = 9;
 * @return {string}
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.getRepositoryAddr = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ApplyPackageReleaseRequest} returns this
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.setRepositoryAddr = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};


/**
 * optional int64 build_number = 10;
 * @return {number}
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 10, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.ApplyPackageReleaseRequest} returns this
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.setBuildNumber = function(value) {
  return jspb.Message.setProto3IntField(this, 10, value);
};


/**
 * optional string build_id = 11;
 * @return {string}
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.getBuildId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 11, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ApplyPackageReleaseRequest} returns this
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.setBuildId = function(value) {
  return jspb.Message.setProto3StringField(this, 11, value);
};


/**
 * optional bool rollback_mode = 12;
 * @return {boolean}
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.getRollbackMode = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 12, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.ApplyPackageReleaseRequest} returns this
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.setRollbackMode = function(value) {
  return jspb.Message.setProto3BooleanField(this, 12, value);
};


/**
 * optional string rollback_reason = 13;
 * @return {string}
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.getRollbackReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 13, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ApplyPackageReleaseRequest} returns this
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.setRollbackReason = function(value) {
  return jspb.Message.setProto3StringField(this, 13, value);
};


/**
 * optional string workflow_run_id = 14;
 * @return {string}
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.getWorkflowRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 14, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ApplyPackageReleaseRequest} returns this
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.setWorkflowRunId = function(value) {
  return jspb.Message.setProto3StringField(this, 14, value);
};


/**
 * optional string target_revision_id = 15;
 * @return {string}
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.getTargetRevisionId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 15, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ApplyPackageReleaseRequest} returns this
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.setTargetRevisionId = function(value) {
  return jspb.Message.setProto3StringField(this, 15, value);
};


/**
 * optional bool preserve_configs = 16;
 * @return {boolean}
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.getPreserveConfigs = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 16, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.ApplyPackageReleaseRequest} returns this
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.setPreserveConfigs = function(value) {
  return jspb.Message.setProto3BooleanField(this, 16, value);
};


/**
 * optional bool restore_config_snapshot = 17;
 * @return {boolean}
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.getRestoreConfigSnapshot = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 17, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.ApplyPackageReleaseRequest} returns this
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.setRestoreConfigSnapshot = function(value) {
  return jspb.Message.setProto3BooleanField(this, 17, value);
};


/**
 * optional bool allow_downgrade = 18;
 * @return {boolean}
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.getAllowDowngrade = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 18, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.ApplyPackageReleaseRequest} returns this
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.setAllowDowngrade = function(value) {
  return jspb.Message.setProto3BooleanField(this, 18, value);
};


/**
 * optional string previous_revision_id = 19;
 * @return {string}
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.getPreviousRevisionId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 19, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ApplyPackageReleaseRequest} returns this
 */
proto.node_agent.ApplyPackageReleaseRequest.prototype.setPreviousRevisionId = function(value) {
  return jspb.Message.setProto3StringField(this, 19, value);
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
proto.node_agent.ApplyPackageReleaseResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.ApplyPackageReleaseResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.ApplyPackageReleaseResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.ApplyPackageReleaseResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
ok: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
message: jspb.Message.getFieldWithDefault(msg, 2, ""),
packageName: jspb.Message.getFieldWithDefault(msg, 3, ""),
version: jspb.Message.getFieldWithDefault(msg, 4, ""),
status: jspb.Message.getFieldWithDefault(msg, 5, ""),
errorDetail: jspb.Message.getFieldWithDefault(msg, 6, ""),
checksum: jspb.Message.getFieldWithDefault(msg, 7, ""),
operationId: jspb.Message.getFieldWithDefault(msg, 8, ""),
buildId: jspb.Message.getFieldWithDefault(msg, 9, "")
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
 * @return {!proto.node_agent.ApplyPackageReleaseResponse}
 */
proto.node_agent.ApplyPackageReleaseResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.ApplyPackageReleaseResponse;
  return proto.node_agent.ApplyPackageReleaseResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.ApplyPackageReleaseResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.ApplyPackageReleaseResponse}
 */
proto.node_agent.ApplyPackageReleaseResponse.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setPackageName(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setVersion(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setStatus(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setErrorDetail(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setChecksum(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setOperationId(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setBuildId(value);
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
proto.node_agent.ApplyPackageReleaseResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.ApplyPackageReleaseResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.ApplyPackageReleaseResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.ApplyPackageReleaseResponse.serializeBinaryToWriter = function(message, writer) {
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
  f = message.getPackageName();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getVersion();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getStatus();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getErrorDetail();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getChecksum();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getOperationId();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
  f = message.getBuildId();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
};


/**
 * optional bool ok = 1;
 * @return {boolean}
 */
proto.node_agent.ApplyPackageReleaseResponse.prototype.getOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.ApplyPackageReleaseResponse} returns this
 */
proto.node_agent.ApplyPackageReleaseResponse.prototype.setOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional string message = 2;
 * @return {string}
 */
proto.node_agent.ApplyPackageReleaseResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ApplyPackageReleaseResponse} returns this
 */
proto.node_agent.ApplyPackageReleaseResponse.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string package_name = 3;
 * @return {string}
 */
proto.node_agent.ApplyPackageReleaseResponse.prototype.getPackageName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ApplyPackageReleaseResponse} returns this
 */
proto.node_agent.ApplyPackageReleaseResponse.prototype.setPackageName = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string version = 4;
 * @return {string}
 */
proto.node_agent.ApplyPackageReleaseResponse.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ApplyPackageReleaseResponse} returns this
 */
proto.node_agent.ApplyPackageReleaseResponse.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string status = 5;
 * @return {string}
 */
proto.node_agent.ApplyPackageReleaseResponse.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ApplyPackageReleaseResponse} returns this
 */
proto.node_agent.ApplyPackageReleaseResponse.prototype.setStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string error_detail = 6;
 * @return {string}
 */
proto.node_agent.ApplyPackageReleaseResponse.prototype.getErrorDetail = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ApplyPackageReleaseResponse} returns this
 */
proto.node_agent.ApplyPackageReleaseResponse.prototype.setErrorDetail = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string checksum = 7;
 * @return {string}
 */
proto.node_agent.ApplyPackageReleaseResponse.prototype.getChecksum = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ApplyPackageReleaseResponse} returns this
 */
proto.node_agent.ApplyPackageReleaseResponse.prototype.setChecksum = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional string operation_id = 8;
 * @return {string}
 */
proto.node_agent.ApplyPackageReleaseResponse.prototype.getOperationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ApplyPackageReleaseResponse} returns this
 */
proto.node_agent.ApplyPackageReleaseResponse.prototype.setOperationId = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};


/**
 * optional string build_id = 9;
 * @return {string}
 */
proto.node_agent.ApplyPackageReleaseResponse.prototype.getBuildId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.ApplyPackageReleaseResponse} returns this
 */
proto.node_agent.ApplyPackageReleaseResponse.prototype.setBuildId = function(value) {
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
proto.node_agent.DeleteCacheArtifactRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.DeleteCacheArtifactRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.DeleteCacheArtifactRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.DeleteCacheArtifactRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
packageName: jspb.Message.getFieldWithDefault(msg, 1, ""),
publisherId: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.node_agent.DeleteCacheArtifactRequest}
 */
proto.node_agent.DeleteCacheArtifactRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.DeleteCacheArtifactRequest;
  return proto.node_agent.DeleteCacheArtifactRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.DeleteCacheArtifactRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.DeleteCacheArtifactRequest}
 */
proto.node_agent.DeleteCacheArtifactRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPackageName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisherId(value);
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
proto.node_agent.DeleteCacheArtifactRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.DeleteCacheArtifactRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.DeleteCacheArtifactRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.DeleteCacheArtifactRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPackageName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getPublisherId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string package_name = 1;
 * @return {string}
 */
proto.node_agent.DeleteCacheArtifactRequest.prototype.getPackageName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.DeleteCacheArtifactRequest} returns this
 */
proto.node_agent.DeleteCacheArtifactRequest.prototype.setPackageName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string publisher_id = 2;
 * @return {string}
 */
proto.node_agent.DeleteCacheArtifactRequest.prototype.getPublisherId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.DeleteCacheArtifactRequest} returns this
 */
proto.node_agent.DeleteCacheArtifactRequest.prototype.setPublisherId = function(value) {
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
proto.node_agent.DeleteCacheArtifactResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.DeleteCacheArtifactResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.DeleteCacheArtifactResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.DeleteCacheArtifactResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
ok: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
message: jspb.Message.getFieldWithDefault(msg, 2, ""),
path: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.node_agent.DeleteCacheArtifactResponse}
 */
proto.node_agent.DeleteCacheArtifactResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.DeleteCacheArtifactResponse;
  return proto.node_agent.DeleteCacheArtifactResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.DeleteCacheArtifactResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.DeleteCacheArtifactResponse}
 */
proto.node_agent.DeleteCacheArtifactResponse.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setPath(value);
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
proto.node_agent.DeleteCacheArtifactResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.DeleteCacheArtifactResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.DeleteCacheArtifactResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.DeleteCacheArtifactResponse.serializeBinaryToWriter = function(message, writer) {
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
  f = message.getPath();
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
proto.node_agent.DeleteCacheArtifactResponse.prototype.getOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.DeleteCacheArtifactResponse} returns this
 */
proto.node_agent.DeleteCacheArtifactResponse.prototype.setOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional string message = 2;
 * @return {string}
 */
proto.node_agent.DeleteCacheArtifactResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.DeleteCacheArtifactResponse} returns this
 */
proto.node_agent.DeleteCacheArtifactResponse.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string path = 3;
 * @return {string}
 */
proto.node_agent.DeleteCacheArtifactResponse.prototype.getPath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.DeleteCacheArtifactResponse} returns this
 */
proto.node_agent.DeleteCacheArtifactResponse.prototype.setPath = function(value) {
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
proto.node_agent.CleanupDiskJournalRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.CleanupDiskJournalRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.CleanupDiskJournalRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.CleanupDiskJournalRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
maxAgeDays: jspb.Message.getFieldWithDefault(msg, 1, 0),
targetSizeMb: jspb.Message.getFieldWithDefault(msg, 2, 0),
dryRun: jspb.Message.getBooleanFieldWithDefault(msg, 3, false)
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
 * @return {!proto.node_agent.CleanupDiskJournalRequest}
 */
proto.node_agent.CleanupDiskJournalRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.CleanupDiskJournalRequest;
  return proto.node_agent.CleanupDiskJournalRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.CleanupDiskJournalRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.CleanupDiskJournalRequest}
 */
proto.node_agent.CleanupDiskJournalRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setMaxAgeDays(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readUint64());
      msg.setTargetSizeMb(value);
      break;
    case 3:
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
proto.node_agent.CleanupDiskJournalRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.CleanupDiskJournalRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.CleanupDiskJournalRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.CleanupDiskJournalRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMaxAgeDays();
  if (f !== 0) {
    writer.writeInt32(
      1,
      f
    );
  }
  f = message.getTargetSizeMb();
  if (f !== 0) {
    writer.writeUint64(
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
};


/**
 * optional int32 max_age_days = 1;
 * @return {number}
 */
proto.node_agent.CleanupDiskJournalRequest.prototype.getMaxAgeDays = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.CleanupDiskJournalRequest} returns this
 */
proto.node_agent.CleanupDiskJournalRequest.prototype.setMaxAgeDays = function(value) {
  return jspb.Message.setProto3IntField(this, 1, value);
};


/**
 * optional uint64 target_size_mb = 2;
 * @return {number}
 */
proto.node_agent.CleanupDiskJournalRequest.prototype.getTargetSizeMb = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.CleanupDiskJournalRequest} returns this
 */
proto.node_agent.CleanupDiskJournalRequest.prototype.setTargetSizeMb = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional bool dry_run = 3;
 * @return {boolean}
 */
proto.node_agent.CleanupDiskJournalRequest.prototype.getDryRun = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.CleanupDiskJournalRequest} returns this
 */
proto.node_agent.CleanupDiskJournalRequest.prototype.setDryRun = function(value) {
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
proto.node_agent.CleanupDiskJournalResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.CleanupDiskJournalResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.CleanupDiskJournalResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.CleanupDiskJournalResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
ok: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
freedBytes: jspb.Message.getFieldWithDefault(msg, 2, 0),
message: jspb.Message.getFieldWithDefault(msg, 3, ""),
error: jspb.Message.getFieldWithDefault(msg, 4, "")
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
 * @return {!proto.node_agent.CleanupDiskJournalResponse}
 */
proto.node_agent.CleanupDiskJournalResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.CleanupDiskJournalResponse;
  return proto.node_agent.CleanupDiskJournalResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.CleanupDiskJournalResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.CleanupDiskJournalResponse}
 */
proto.node_agent.CleanupDiskJournalResponse.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {number} */ (reader.readUint64());
      msg.setFreedBytes(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setError(value);
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
proto.node_agent.CleanupDiskJournalResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.CleanupDiskJournalResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.CleanupDiskJournalResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.CleanupDiskJournalResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOk();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getFreedBytes();
  if (f !== 0) {
    writer.writeUint64(
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
  f = message.getError();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * optional bool ok = 1;
 * @return {boolean}
 */
proto.node_agent.CleanupDiskJournalResponse.prototype.getOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.CleanupDiskJournalResponse} returns this
 */
proto.node_agent.CleanupDiskJournalResponse.prototype.setOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional uint64 freed_bytes = 2;
 * @return {number}
 */
proto.node_agent.CleanupDiskJournalResponse.prototype.getFreedBytes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.CleanupDiskJournalResponse} returns this
 */
proto.node_agent.CleanupDiskJournalResponse.prototype.setFreedBytes = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string message = 3;
 * @return {string}
 */
proto.node_agent.CleanupDiskJournalResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.CleanupDiskJournalResponse} returns this
 */
proto.node_agent.CleanupDiskJournalResponse.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string error = 4;
 * @return {string}
 */
proto.node_agent.CleanupDiskJournalResponse.prototype.getError = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.CleanupDiskJournalResponse} returns this
 */
proto.node_agent.CleanupDiskJournalResponse.prototype.setError = function(value) {
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
proto.node_agent.CollectBackupSecretsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.CollectBackupSecretsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.CollectBackupSecretsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.CollectBackupSecretsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
capsuleDir: jspb.Message.getFieldWithDefault(msg, 1, ""),
backupId: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.node_agent.CollectBackupSecretsRequest}
 */
proto.node_agent.CollectBackupSecretsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.CollectBackupSecretsRequest;
  return proto.node_agent.CollectBackupSecretsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.CollectBackupSecretsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.CollectBackupSecretsRequest}
 */
proto.node_agent.CollectBackupSecretsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setCapsuleDir(value);
      break;
    case 2:
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
proto.node_agent.CollectBackupSecretsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.CollectBackupSecretsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.CollectBackupSecretsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.CollectBackupSecretsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getCapsuleDir();
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
};


/**
 * optional string capsule_dir = 1;
 * @return {string}
 */
proto.node_agent.CollectBackupSecretsRequest.prototype.getCapsuleDir = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.CollectBackupSecretsRequest} returns this
 */
proto.node_agent.CollectBackupSecretsRequest.prototype.setCapsuleDir = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string backup_id = 2;
 * @return {string}
 */
proto.node_agent.CollectBackupSecretsRequest.prototype.getBackupId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.CollectBackupSecretsRequest} returns this
 */
proto.node_agent.CollectBackupSecretsRequest.prototype.setBackupId = function(value) {
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
proto.node_agent.SecretFileEntry.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.SecretFileEntry.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.SecretFileEntry} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.SecretFileEntry.toObject = function(includeInstance, msg) {
  var f, obj = {
originalPath: jspb.Message.getFieldWithDefault(msg, 1, ""),
capsuleRelpath: jspb.Message.getFieldWithDefault(msg, 2, ""),
modeOctal: jspb.Message.getFieldWithDefault(msg, 3, ""),
owner: jspb.Message.getFieldWithDefault(msg, 4, ""),
group: jspb.Message.getFieldWithDefault(msg, 5, ""),
sizeBytes: jspb.Message.getFieldWithDefault(msg, 6, 0),
sha256: jspb.Message.getFieldWithDefault(msg, 7, ""),
required: jspb.Message.getBooleanFieldWithDefault(msg, 8, false),
optionalWhenAbsent: jspb.Message.getBooleanFieldWithDefault(msg, 9, false),
found: jspb.Message.getBooleanFieldWithDefault(msg, 10, false),
reason: jspb.Message.getFieldWithDefault(msg, 11, ""),
producedBy: jspb.Message.getFieldWithDefault(msg, 12, "")
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
 * @return {!proto.node_agent.SecretFileEntry}
 */
proto.node_agent.SecretFileEntry.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.SecretFileEntry;
  return proto.node_agent.SecretFileEntry.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.SecretFileEntry} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.SecretFileEntry}
 */
proto.node_agent.SecretFileEntry.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setOriginalPath(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setCapsuleRelpath(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setModeOctal(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setOwner(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setGroup(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readUint64());
      msg.setSizeBytes(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setSha256(value);
      break;
    case 8:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setRequired(value);
      break;
    case 9:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setOptionalWhenAbsent(value);
      break;
    case 10:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setFound(value);
      break;
    case 11:
      var value = /** @type {string} */ (reader.readString());
      msg.setReason(value);
      break;
    case 12:
      var value = /** @type {string} */ (reader.readString());
      msg.setProducedBy(value);
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
proto.node_agent.SecretFileEntry.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.SecretFileEntry.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.SecretFileEntry} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.SecretFileEntry.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOriginalPath();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getCapsuleRelpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getModeOctal();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getOwner();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getGroup();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getSizeBytes();
  if (f !== 0) {
    writer.writeUint64(
      6,
      f
    );
  }
  f = message.getSha256();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getRequired();
  if (f) {
    writer.writeBool(
      8,
      f
    );
  }
  f = message.getOptionalWhenAbsent();
  if (f) {
    writer.writeBool(
      9,
      f
    );
  }
  f = message.getFound();
  if (f) {
    writer.writeBool(
      10,
      f
    );
  }
  f = message.getReason();
  if (f.length > 0) {
    writer.writeString(
      11,
      f
    );
  }
  f = message.getProducedBy();
  if (f.length > 0) {
    writer.writeString(
      12,
      f
    );
  }
};


/**
 * optional string original_path = 1;
 * @return {string}
 */
proto.node_agent.SecretFileEntry.prototype.getOriginalPath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.SecretFileEntry} returns this
 */
proto.node_agent.SecretFileEntry.prototype.setOriginalPath = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string capsule_relpath = 2;
 * @return {string}
 */
proto.node_agent.SecretFileEntry.prototype.getCapsuleRelpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.SecretFileEntry} returns this
 */
proto.node_agent.SecretFileEntry.prototype.setCapsuleRelpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string mode_octal = 3;
 * @return {string}
 */
proto.node_agent.SecretFileEntry.prototype.getModeOctal = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.SecretFileEntry} returns this
 */
proto.node_agent.SecretFileEntry.prototype.setModeOctal = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string owner = 4;
 * @return {string}
 */
proto.node_agent.SecretFileEntry.prototype.getOwner = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.SecretFileEntry} returns this
 */
proto.node_agent.SecretFileEntry.prototype.setOwner = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string group = 5;
 * @return {string}
 */
proto.node_agent.SecretFileEntry.prototype.getGroup = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.SecretFileEntry} returns this
 */
proto.node_agent.SecretFileEntry.prototype.setGroup = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional uint64 size_bytes = 6;
 * @return {number}
 */
proto.node_agent.SecretFileEntry.prototype.getSizeBytes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.node_agent.SecretFileEntry} returns this
 */
proto.node_agent.SecretFileEntry.prototype.setSizeBytes = function(value) {
  return jspb.Message.setProto3IntField(this, 6, value);
};


/**
 * optional string sha256 = 7;
 * @return {string}
 */
proto.node_agent.SecretFileEntry.prototype.getSha256 = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.SecretFileEntry} returns this
 */
proto.node_agent.SecretFileEntry.prototype.setSha256 = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional bool required = 8;
 * @return {boolean}
 */
proto.node_agent.SecretFileEntry.prototype.getRequired = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 8, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.SecretFileEntry} returns this
 */
proto.node_agent.SecretFileEntry.prototype.setRequired = function(value) {
  return jspb.Message.setProto3BooleanField(this, 8, value);
};


/**
 * optional bool optional_when_absent = 9;
 * @return {boolean}
 */
proto.node_agent.SecretFileEntry.prototype.getOptionalWhenAbsent = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 9, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.SecretFileEntry} returns this
 */
proto.node_agent.SecretFileEntry.prototype.setOptionalWhenAbsent = function(value) {
  return jspb.Message.setProto3BooleanField(this, 9, value);
};


/**
 * optional bool found = 10;
 * @return {boolean}
 */
proto.node_agent.SecretFileEntry.prototype.getFound = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 10, false));
};


/**
 * @param {boolean} value
 * @return {!proto.node_agent.SecretFileEntry} returns this
 */
proto.node_agent.SecretFileEntry.prototype.setFound = function(value) {
  return jspb.Message.setProto3BooleanField(this, 10, value);
};


/**
 * optional string reason = 11;
 * @return {string}
 */
proto.node_agent.SecretFileEntry.prototype.getReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 11, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.SecretFileEntry} returns this
 */
proto.node_agent.SecretFileEntry.prototype.setReason = function(value) {
  return jspb.Message.setProto3StringField(this, 11, value);
};


/**
 * optional string produced_by = 12;
 * @return {string}
 */
proto.node_agent.SecretFileEntry.prototype.getProducedBy = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 12, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.SecretFileEntry} returns this
 */
proto.node_agent.SecretFileEntry.prototype.setProducedBy = function(value) {
  return jspb.Message.setProto3StringField(this, 12, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.node_agent.CollectBackupSecretsResponse.repeatedFields_ = [6,7,8];



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
proto.node_agent.CollectBackupSecretsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.node_agent.CollectBackupSecretsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.node_agent.CollectBackupSecretsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.CollectBackupSecretsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
hostname: jspb.Message.getFieldWithDefault(msg, 2, ""),
primaryIp: jspb.Message.getFieldWithDefault(msg, 3, ""),
nodeAgentVersion: jspb.Message.getFieldWithDefault(msg, 4, ""),
collectedAtUnix: jspb.Message.getFieldWithDefault(msg, 5, ""),
entriesList: jspb.Message.toObjectList(msg.getEntriesList(),
    proto.node_agent.SecretFileEntry.toObject, includeInstance),
missingRequiredList: (f = jspb.Message.getRepeatedField(msg, 7)) == null ? undefined : f,
missingOptionalList: (f = jspb.Message.getRepeatedField(msg, 8)) == null ? undefined : f,
perNodeManifest: jspb.Message.getFieldWithDefault(msg, 9, "")
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
 * @return {!proto.node_agent.CollectBackupSecretsResponse}
 */
proto.node_agent.CollectBackupSecretsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.node_agent.CollectBackupSecretsResponse;
  return proto.node_agent.CollectBackupSecretsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.node_agent.CollectBackupSecretsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.node_agent.CollectBackupSecretsResponse}
 */
proto.node_agent.CollectBackupSecretsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setHostname(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setPrimaryIp(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeAgentVersion(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setCollectedAtUnix(value);
      break;
    case 6:
      var value = new proto.node_agent.SecretFileEntry;
      reader.readMessage(value,proto.node_agent.SecretFileEntry.deserializeBinaryFromReader);
      msg.addEntries(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.addMissingRequired(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.addMissingOptional(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setPerNodeManifest(value);
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
proto.node_agent.CollectBackupSecretsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.node_agent.CollectBackupSecretsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.node_agent.CollectBackupSecretsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.node_agent.CollectBackupSecretsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getHostname();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getPrimaryIp();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getNodeAgentVersion();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getCollectedAtUnix();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getEntriesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      6,
      f,
      proto.node_agent.SecretFileEntry.serializeBinaryToWriter
    );
  }
  f = message.getMissingRequiredList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      7,
      f
    );
  }
  f = message.getMissingOptionalList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      8,
      f
    );
  }
  f = message.getPerNodeManifest();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.CollectBackupSecretsResponse} returns this
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string hostname = 2;
 * @return {string}
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.getHostname = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.CollectBackupSecretsResponse} returns this
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.setHostname = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string primary_ip = 3;
 * @return {string}
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.getPrimaryIp = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.CollectBackupSecretsResponse} returns this
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.setPrimaryIp = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string node_agent_version = 4;
 * @return {string}
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.getNodeAgentVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.CollectBackupSecretsResponse} returns this
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.setNodeAgentVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string collected_at_unix = 5;
 * @return {string}
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.getCollectedAtUnix = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.CollectBackupSecretsResponse} returns this
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.setCollectedAtUnix = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * repeated SecretFileEntry entries = 6;
 * @return {!Array<!proto.node_agent.SecretFileEntry>}
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.getEntriesList = function() {
  return /** @type{!Array<!proto.node_agent.SecretFileEntry>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.node_agent.SecretFileEntry, 6));
};


/**
 * @param {!Array<!proto.node_agent.SecretFileEntry>} value
 * @return {!proto.node_agent.CollectBackupSecretsResponse} returns this
*/
proto.node_agent.CollectBackupSecretsResponse.prototype.setEntriesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 6, value);
};


/**
 * @param {!proto.node_agent.SecretFileEntry=} opt_value
 * @param {number=} opt_index
 * @return {!proto.node_agent.SecretFileEntry}
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.addEntries = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 6, opt_value, proto.node_agent.SecretFileEntry, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.node_agent.CollectBackupSecretsResponse} returns this
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.clearEntriesList = function() {
  return this.setEntriesList([]);
};


/**
 * repeated string missing_required = 7;
 * @return {!Array<string>}
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.getMissingRequiredList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 7));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.node_agent.CollectBackupSecretsResponse} returns this
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.setMissingRequiredList = function(value) {
  return jspb.Message.setField(this, 7, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.node_agent.CollectBackupSecretsResponse} returns this
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.addMissingRequired = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 7, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.node_agent.CollectBackupSecretsResponse} returns this
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.clearMissingRequiredList = function() {
  return this.setMissingRequiredList([]);
};


/**
 * repeated string missing_optional = 8;
 * @return {!Array<string>}
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.getMissingOptionalList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 8));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.node_agent.CollectBackupSecretsResponse} returns this
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.setMissingOptionalList = function(value) {
  return jspb.Message.setField(this, 8, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.node_agent.CollectBackupSecretsResponse} returns this
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.addMissingOptional = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 8, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.node_agent.CollectBackupSecretsResponse} returns this
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.clearMissingOptionalList = function() {
  return this.setMissingOptionalList([]);
};


/**
 * optional string per_node_manifest = 9;
 * @return {string}
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.getPerNodeManifest = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.node_agent.CollectBackupSecretsResponse} returns this
 */
proto.node_agent.CollectBackupSecretsResponse.prototype.setPerNodeManifest = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};


/**
 * @enum {number}
 */
proto.node_agent.SubsystemState = {
  SUBSYSTEM_STATE_UNSPECIFIED: 0,
  SUBSYSTEM_STATE_HEALTHY: 1,
  SUBSYSTEM_STATE_DEGRADED: 2,
  SUBSYSTEM_STATE_FAILED: 3,
  SUBSYSTEM_STATE_STARTING: 4,
  SUBSYSTEM_STATE_STOPPED: 5
};

goog.object.extend(exports, proto.node_agent);
