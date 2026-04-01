// source: cluster_controller.proto
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
goog.exportSymbol('proto.cluster_controller.AffectedNodeDiff', null, global);
goog.exportSymbol('proto.cluster_controller.ApproveJoinRequest', null, global);
goog.exportSymbol('proto.cluster_controller.ApproveJoinResponse', null, global);
goog.exportSymbol('proto.cluster_controller.ArtifactKind', null, global);
goog.exportSymbol('proto.cluster_controller.ArtifactRef', null, global);
goog.exportSymbol('proto.cluster_controller.ClusterInfo', null, global);
goog.exportSymbol('proto.cluster_controller.ClusterNetworkSpec', null, global);
goog.exportSymbol('proto.cluster_controller.CompleteOperationRequest', null, global);
goog.exportSymbol('proto.cluster_controller.CompleteOperationResponse', null, global);
goog.exportSymbol('proto.cluster_controller.ConfigFileDiff', null, global);
goog.exportSymbol('proto.cluster_controller.CreateJoinTokenRequest', null, global);
goog.exportSymbol('proto.cluster_controller.CreateJoinTokenResponse', null, global);
goog.exportSymbol('proto.cluster_controller.DesiredNetwork', null, global);
goog.exportSymbol('proto.cluster_controller.DesiredService', null, global);
goog.exportSymbol('proto.cluster_controller.DesiredServicesDelta', null, global);
goog.exportSymbol('proto.cluster_controller.DesiredState', null, global);
goog.exportSymbol('proto.cluster_controller.DomainMigration', null, global);
goog.exportSymbol('proto.cluster_controller.DomainMigration.MigrationState', null, global);
goog.exportSymbol('proto.cluster_controller.ExternalDNSConfig', null, global);
goog.exportSymbol('proto.cluster_controller.GetClusterHealthRequest', null, global);
goog.exportSymbol('proto.cluster_controller.GetClusterHealthResponse', null, global);
goog.exportSymbol('proto.cluster_controller.GetClusterHealthV1Request', null, global);
goog.exportSymbol('proto.cluster_controller.GetClusterHealthV1Response', null, global);
goog.exportSymbol('proto.cluster_controller.GetJoinRequestStatusRequest', null, global);
goog.exportSymbol('proto.cluster_controller.GetJoinRequestStatusResponse', null, global);
goog.exportSymbol('proto.cluster_controller.GetNodeHealthDetailV1Request', null, global);
goog.exportSymbol('proto.cluster_controller.GetNodeHealthDetailV1Response', null, global);
goog.exportSymbol('proto.cluster_controller.InstallPolicy', null, global);
goog.exportSymbol('proto.cluster_controller.JoinRequestRecord', null, global);
goog.exportSymbol('proto.cluster_controller.ListJoinRequestsRequest', null, global);
goog.exportSymbol('proto.cluster_controller.ListJoinRequestsResponse', null, global);
goog.exportSymbol('proto.cluster_controller.ListNodesRequest', null, global);
goog.exportSymbol('proto.cluster_controller.ListNodesResponse', null, global);
goog.exportSymbol('proto.cluster_controller.NodeCapabilities', null, global);
goog.exportSymbol('proto.cluster_controller.NodeChange', null, global);
goog.exportSymbol('proto.cluster_controller.NodeHealth', null, global);
goog.exportSymbol('proto.cluster_controller.NodeHealthCheck', null, global);
goog.exportSymbol('proto.cluster_controller.NodeHealthStatus', null, global);
goog.exportSymbol('proto.cluster_controller.NodeIdentity', null, global);
goog.exportSymbol('proto.cluster_controller.NodeRecord', null, global);
goog.exportSymbol('proto.cluster_controller.NodeStatus', null, global);
goog.exportSymbol('proto.cluster_controller.NodeUnitStatus', null, global);
goog.exportSymbol('proto.cluster_controller.OperationEvent', null, global);
goog.exportSymbol('proto.cluster_controller.OperationPhase', null, global);
goog.exportSymbol('proto.cluster_controller.PreviewNodeProfilesRequest', null, global);
goog.exportSymbol('proto.cluster_controller.PreviewNodeProfilesResponse', null, global);
goog.exportSymbol('proto.cluster_controller.RejectJoinRequest', null, global);
goog.exportSymbol('proto.cluster_controller.RejectJoinResponse', null, global);
goog.exportSymbol('proto.cluster_controller.RemoveDesiredServiceRequest', null, global);
goog.exportSymbol('proto.cluster_controller.RemoveNodeRequest', null, global);
goog.exportSymbol('proto.cluster_controller.RemoveNodeResponse', null, global);
goog.exportSymbol('proto.cluster_controller.ReportNodeStatusRequest', null, global);
goog.exportSymbol('proto.cluster_controller.ReportNodeStatusResponse', null, global);
goog.exportSymbol('proto.cluster_controller.RequestJoinRequest', null, global);
goog.exportSymbol('proto.cluster_controller.RequestJoinResponse', null, global);
goog.exportSymbol('proto.cluster_controller.SeedDesiredStateRequest', null, global);
goog.exportSymbol('proto.cluster_controller.SeedDesiredStateRequest.Mode', null, global);
goog.exportSymbol('proto.cluster_controller.ServiceChangePreview', null, global);
goog.exportSymbol('proto.cluster_controller.ServiceSummary', null, global);
goog.exportSymbol('proto.cluster_controller.SetNodeProfilesRequest', null, global);
goog.exportSymbol('proto.cluster_controller.SetNodeProfilesResponse', null, global);
goog.exportSymbol('proto.cluster_controller.StartApplyRequest', null, global);
goog.exportSymbol('proto.cluster_controller.StartApplyResponse', null, global);
goog.exportSymbol('proto.cluster_controller.UnitAction', null, global);
goog.exportSymbol('proto.cluster_controller.UpdateClusterNetworkRequest', null, global);
goog.exportSymbol('proto.cluster_controller.UpdateClusterNetworkResponse', null, global);
goog.exportSymbol('proto.cluster_controller.UpgradeGlobularRequest', null, global);
goog.exportSymbol('proto.cluster_controller.UpgradeGlobularResponse', null, global);
goog.exportSymbol('proto.cluster_controller.UpsertDesiredServiceRequest', null, global);
goog.exportSymbol('proto.cluster_controller.ValidateArtifactRequest', null, global);
goog.exportSymbol('proto.cluster_controller.ValidationIssue', null, global);
goog.exportSymbol('proto.cluster_controller.ValidationIssue.Severity', null, global);
goog.exportSymbol('proto.cluster_controller.ValidationReport', null, global);
goog.exportSymbol('proto.cluster_controller.WatchOperationsRequest', null, global);
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
proto.cluster_controller.ClusterInfo = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.ClusterInfo, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.ClusterInfo.displayName = 'proto.cluster_controller.ClusterInfo';
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
proto.cluster_controller.ClusterNetworkSpec = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.ClusterNetworkSpec.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.ClusterNetworkSpec, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.ClusterNetworkSpec.displayName = 'proto.cluster_controller.ClusterNetworkSpec';
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
proto.cluster_controller.DomainMigration = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.DomainMigration, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.DomainMigration.displayName = 'proto.cluster_controller.DomainMigration';
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
proto.cluster_controller.ExternalDNSConfig = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.ExternalDNSConfig.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.ExternalDNSConfig, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.ExternalDNSConfig.displayName = 'proto.cluster_controller.ExternalDNSConfig';
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
proto.cluster_controller.NodeIdentity = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.NodeIdentity.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.NodeIdentity, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.NodeIdentity.displayName = 'proto.cluster_controller.NodeIdentity';
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
proto.cluster_controller.NodeCapabilities = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.NodeCapabilities, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.NodeCapabilities.displayName = 'proto.cluster_controller.NodeCapabilities';
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
proto.cluster_controller.NodeRecord = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.NodeRecord.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.NodeRecord, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.NodeRecord.displayName = 'proto.cluster_controller.NodeRecord';
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
proto.cluster_controller.CreateJoinTokenRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.CreateJoinTokenRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.CreateJoinTokenRequest.displayName = 'proto.cluster_controller.CreateJoinTokenRequest';
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
proto.cluster_controller.CreateJoinTokenResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.CreateJoinTokenResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.CreateJoinTokenResponse.displayName = 'proto.cluster_controller.CreateJoinTokenResponse';
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
proto.cluster_controller.RequestJoinRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.RequestJoinRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.RequestJoinRequest.displayName = 'proto.cluster_controller.RequestJoinRequest';
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
proto.cluster_controller.RequestJoinResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.RequestJoinResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.RequestJoinResponse.displayName = 'proto.cluster_controller.RequestJoinResponse';
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
proto.cluster_controller.GetJoinRequestStatusRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.GetJoinRequestStatusRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.GetJoinRequestStatusRequest.displayName = 'proto.cluster_controller.GetJoinRequestStatusRequest';
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
proto.cluster_controller.GetJoinRequestStatusResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.GetJoinRequestStatusResponse.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.GetJoinRequestStatusResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.GetJoinRequestStatusResponse.displayName = 'proto.cluster_controller.GetJoinRequestStatusResponse';
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
proto.cluster_controller.JoinRequestRecord = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.JoinRequestRecord.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.JoinRequestRecord, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.JoinRequestRecord.displayName = 'proto.cluster_controller.JoinRequestRecord';
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
proto.cluster_controller.ListJoinRequestsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.ListJoinRequestsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.ListJoinRequestsRequest.displayName = 'proto.cluster_controller.ListJoinRequestsRequest';
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
proto.cluster_controller.ListJoinRequestsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.ListJoinRequestsResponse.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.ListJoinRequestsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.ListJoinRequestsResponse.displayName = 'proto.cluster_controller.ListJoinRequestsResponse';
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
proto.cluster_controller.ApproveJoinRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.ApproveJoinRequest.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.ApproveJoinRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.ApproveJoinRequest.displayName = 'proto.cluster_controller.ApproveJoinRequest';
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
proto.cluster_controller.ApproveJoinResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.ApproveJoinResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.ApproveJoinResponse.displayName = 'proto.cluster_controller.ApproveJoinResponse';
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
proto.cluster_controller.RejectJoinRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.RejectJoinRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.RejectJoinRequest.displayName = 'proto.cluster_controller.RejectJoinRequest';
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
proto.cluster_controller.RejectJoinResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.RejectJoinResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.RejectJoinResponse.displayName = 'proto.cluster_controller.RejectJoinResponse';
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
proto.cluster_controller.ListNodesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.ListNodesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.ListNodesRequest.displayName = 'proto.cluster_controller.ListNodesRequest';
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
proto.cluster_controller.ListNodesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.ListNodesResponse.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.ListNodesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.ListNodesResponse.displayName = 'proto.cluster_controller.ListNodesResponse';
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
proto.cluster_controller.SetNodeProfilesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.SetNodeProfilesRequest.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.SetNodeProfilesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.SetNodeProfilesRequest.displayName = 'proto.cluster_controller.SetNodeProfilesRequest';
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
proto.cluster_controller.SetNodeProfilesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.SetNodeProfilesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.SetNodeProfilesResponse.displayName = 'proto.cluster_controller.SetNodeProfilesResponse';
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
proto.cluster_controller.RemoveNodeRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.RemoveNodeRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.RemoveNodeRequest.displayName = 'proto.cluster_controller.RemoveNodeRequest';
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
proto.cluster_controller.RemoveNodeResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.RemoveNodeResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.RemoveNodeResponse.displayName = 'proto.cluster_controller.RemoveNodeResponse';
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
proto.cluster_controller.GetClusterHealthRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.GetClusterHealthRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.GetClusterHealthRequest.displayName = 'proto.cluster_controller.GetClusterHealthRequest';
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
proto.cluster_controller.GetClusterHealthResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.GetClusterHealthResponse.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.GetClusterHealthResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.GetClusterHealthResponse.displayName = 'proto.cluster_controller.GetClusterHealthResponse';
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
proto.cluster_controller.NodeHealthStatus = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.NodeHealthStatus, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.NodeHealthStatus.displayName = 'proto.cluster_controller.NodeHealthStatus';
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
proto.cluster_controller.UpdateClusterNetworkRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.UpdateClusterNetworkRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.UpdateClusterNetworkRequest.displayName = 'proto.cluster_controller.UpdateClusterNetworkRequest';
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
proto.cluster_controller.UpdateClusterNetworkResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.UpdateClusterNetworkResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.UpdateClusterNetworkResponse.displayName = 'proto.cluster_controller.UpdateClusterNetworkResponse';
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
proto.cluster_controller.ArtifactRef = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.ArtifactRef, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.ArtifactRef.displayName = 'proto.cluster_controller.ArtifactRef';
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
proto.cluster_controller.UnitAction = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.UnitAction, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.UnitAction.displayName = 'proto.cluster_controller.UnitAction';
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
proto.cluster_controller.UpgradeGlobularRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.UpgradeGlobularRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.UpgradeGlobularRequest.displayName = 'proto.cluster_controller.UpgradeGlobularRequest';
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
proto.cluster_controller.UpgradeGlobularResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.UpgradeGlobularResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.UpgradeGlobularResponse.displayName = 'proto.cluster_controller.UpgradeGlobularResponse';
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
proto.cluster_controller.StartApplyRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.StartApplyRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.StartApplyRequest.displayName = 'proto.cluster_controller.StartApplyRequest';
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
proto.cluster_controller.StartApplyResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.StartApplyResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.StartApplyResponse.displayName = 'proto.cluster_controller.StartApplyResponse';
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
proto.cluster_controller.OperationEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.OperationEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.OperationEvent.displayName = 'proto.cluster_controller.OperationEvent';
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
proto.cluster_controller.CompleteOperationRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.CompleteOperationRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.CompleteOperationRequest.displayName = 'proto.cluster_controller.CompleteOperationRequest';
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
proto.cluster_controller.CompleteOperationResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.CompleteOperationResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.CompleteOperationResponse.displayName = 'proto.cluster_controller.CompleteOperationResponse';
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
proto.cluster_controller.NodeUnitStatus = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.NodeUnitStatus, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.NodeUnitStatus.displayName = 'proto.cluster_controller.NodeUnitStatus';
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
proto.cluster_controller.NodeStatus = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.NodeStatus.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.NodeStatus, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.NodeStatus.displayName = 'proto.cluster_controller.NodeStatus';
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
proto.cluster_controller.ReportNodeStatusRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.ReportNodeStatusRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.ReportNodeStatusRequest.displayName = 'proto.cluster_controller.ReportNodeStatusRequest';
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
proto.cluster_controller.ReportNodeStatusResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.ReportNodeStatusResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.ReportNodeStatusResponse.displayName = 'proto.cluster_controller.ReportNodeStatusResponse';
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
proto.cluster_controller.WatchOperationsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.WatchOperationsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.WatchOperationsRequest.displayName = 'proto.cluster_controller.WatchOperationsRequest';
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
proto.cluster_controller.DesiredNetwork = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.DesiredNetwork.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.DesiredNetwork, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.DesiredNetwork.displayName = 'proto.cluster_controller.DesiredNetwork';
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
proto.cluster_controller.GetClusterHealthV1Request = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.GetClusterHealthV1Request, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.GetClusterHealthV1Request.displayName = 'proto.cluster_controller.GetClusterHealthV1Request';
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
proto.cluster_controller.NodeHealth = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.NodeHealth, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.NodeHealth.displayName = 'proto.cluster_controller.NodeHealth';
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
proto.cluster_controller.ServiceSummary = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.ServiceSummary, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.ServiceSummary.displayName = 'proto.cluster_controller.ServiceSummary';
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
proto.cluster_controller.GetClusterHealthV1Response = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.GetClusterHealthV1Response.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.GetClusterHealthV1Response, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.GetClusterHealthV1Response.displayName = 'proto.cluster_controller.GetClusterHealthV1Response';
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
proto.cluster_controller.NodeHealthCheck = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.NodeHealthCheck, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.NodeHealthCheck.displayName = 'proto.cluster_controller.NodeHealthCheck';
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
proto.cluster_controller.GetNodeHealthDetailV1Request = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.GetNodeHealthDetailV1Request, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.GetNodeHealthDetailV1Request.displayName = 'proto.cluster_controller.GetNodeHealthDetailV1Request';
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
proto.cluster_controller.GetNodeHealthDetailV1Response = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.GetNodeHealthDetailV1Response.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.GetNodeHealthDetailV1Response, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.GetNodeHealthDetailV1Response.displayName = 'proto.cluster_controller.GetNodeHealthDetailV1Response';
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
proto.cluster_controller.PreviewNodeProfilesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.PreviewNodeProfilesRequest.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.PreviewNodeProfilesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.PreviewNodeProfilesRequest.displayName = 'proto.cluster_controller.PreviewNodeProfilesRequest';
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
proto.cluster_controller.ConfigFileDiff = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.ConfigFileDiff, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.ConfigFileDiff.displayName = 'proto.cluster_controller.ConfigFileDiff';
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
proto.cluster_controller.AffectedNodeDiff = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.AffectedNodeDiff.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.AffectedNodeDiff, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.AffectedNodeDiff.displayName = 'proto.cluster_controller.AffectedNodeDiff';
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
proto.cluster_controller.PreviewNodeProfilesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.PreviewNodeProfilesResponse.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.PreviewNodeProfilesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.PreviewNodeProfilesResponse.displayName = 'proto.cluster_controller.PreviewNodeProfilesResponse';
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
proto.cluster_controller.DesiredService = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.DesiredService, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.DesiredService.displayName = 'proto.cluster_controller.DesiredService';
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
proto.cluster_controller.DesiredState = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.DesiredState.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.DesiredState, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.DesiredState.displayName = 'proto.cluster_controller.DesiredState';
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
proto.cluster_controller.UpsertDesiredServiceRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.UpsertDesiredServiceRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.UpsertDesiredServiceRequest.displayName = 'proto.cluster_controller.UpsertDesiredServiceRequest';
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
proto.cluster_controller.RemoveDesiredServiceRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.RemoveDesiredServiceRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.RemoveDesiredServiceRequest.displayName = 'proto.cluster_controller.RemoveDesiredServiceRequest';
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
proto.cluster_controller.SeedDesiredStateRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.SeedDesiredStateRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.SeedDesiredStateRequest.displayName = 'proto.cluster_controller.SeedDesiredStateRequest';
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
proto.cluster_controller.ValidateArtifactRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.ValidateArtifactRequest.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.ValidateArtifactRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.ValidateArtifactRequest.displayName = 'proto.cluster_controller.ValidateArtifactRequest';
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
proto.cluster_controller.ValidationIssue = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.cluster_controller.ValidationIssue, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.ValidationIssue.displayName = 'proto.cluster_controller.ValidationIssue';
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
proto.cluster_controller.ValidationReport = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.ValidationReport.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.ValidationReport, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.ValidationReport.displayName = 'proto.cluster_controller.ValidationReport';
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
proto.cluster_controller.DesiredServicesDelta = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.DesiredServicesDelta.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.DesiredServicesDelta, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.DesiredServicesDelta.displayName = 'proto.cluster_controller.DesiredServicesDelta';
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
proto.cluster_controller.NodeChange = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.NodeChange.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.NodeChange, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.NodeChange.displayName = 'proto.cluster_controller.NodeChange';
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
proto.cluster_controller.ServiceChangePreview = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.ServiceChangePreview.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.ServiceChangePreview, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.ServiceChangePreview.displayName = 'proto.cluster_controller.ServiceChangePreview';
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
proto.cluster_controller.InstallPolicy = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.cluster_controller.InstallPolicy.repeatedFields_, null);
};
goog.inherits(proto.cluster_controller.InstallPolicy, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.cluster_controller.InstallPolicy.displayName = 'proto.cluster_controller.InstallPolicy';
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
proto.cluster_controller.ClusterInfo.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.ClusterInfo.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.ClusterInfo} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ClusterInfo.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, ""),
clusterDomain: jspb.Message.getFieldWithDefault(msg, 2, ""),
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
 * @return {!proto.cluster_controller.ClusterInfo}
 */
proto.cluster_controller.ClusterInfo.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.ClusterInfo;
  return proto.cluster_controller.ClusterInfo.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.ClusterInfo} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.ClusterInfo}
 */
proto.cluster_controller.ClusterInfo.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setClusterDomain(value);
      break;
    case 3:
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
proto.cluster_controller.ClusterInfo.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.ClusterInfo.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.ClusterInfo} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ClusterInfo.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getClusterDomain();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getCreatedAt();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.cluster_controller.ClusterInfo.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ClusterInfo} returns this
 */
proto.cluster_controller.ClusterInfo.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string cluster_domain = 2;
 * @return {string}
 */
proto.cluster_controller.ClusterInfo.prototype.getClusterDomain = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ClusterInfo} returns this
 */
proto.cluster_controller.ClusterInfo.prototype.setClusterDomain = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional google.protobuf.Timestamp created_at = 3;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.cluster_controller.ClusterInfo.prototype.getCreatedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 3));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.cluster_controller.ClusterInfo} returns this
*/
proto.cluster_controller.ClusterInfo.prototype.setCreatedAt = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.cluster_controller.ClusterInfo} returns this
 */
proto.cluster_controller.ClusterInfo.prototype.clearCreatedAt = function() {
  return this.setCreatedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.cluster_controller.ClusterInfo.prototype.hasCreatedAt = function() {
  return jspb.Message.getField(this, 3) != null;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.ClusterNetworkSpec.repeatedFields_ = [5,10];



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
proto.cluster_controller.ClusterNetworkSpec.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.ClusterNetworkSpec.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.ClusterNetworkSpec} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ClusterNetworkSpec.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterDomain: jspb.Message.getFieldWithDefault(msg, 1, ""),
protocol: jspb.Message.getFieldWithDefault(msg, 2, ""),
portHttp: jspb.Message.getFieldWithDefault(msg, 3, 0),
portHttps: jspb.Message.getFieldWithDefault(msg, 4, 0),
alternateDomainsList: (f = jspb.Message.getRepeatedField(msg, 5)) == null ? undefined : f,
acmeEnabled: jspb.Message.getBooleanFieldWithDefault(msg, 6, false),
adminEmail: jspb.Message.getFieldWithDefault(msg, 7, ""),
gatewayFqdn: jspb.Message.getFieldWithDefault(msg, 8, ""),
dnsEndpoint: jspb.Message.getFieldWithDefault(msg, 9, ""),
dnsNameserversList: (f = jspb.Message.getRepeatedField(msg, 10)) == null ? undefined : f,
dnsTtl: jspb.Message.getFieldWithDefault(msg, 11, 0),
externalDns: (f = msg.getExternalDns()) && proto.cluster_controller.ExternalDNSConfig.toObject(includeInstance, f),
domainMigration: (f = msg.getDomainMigration()) && proto.cluster_controller.DomainMigration.toObject(includeInstance, f)
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
 * @return {!proto.cluster_controller.ClusterNetworkSpec}
 */
proto.cluster_controller.ClusterNetworkSpec.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.ClusterNetworkSpec;
  return proto.cluster_controller.ClusterNetworkSpec.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.ClusterNetworkSpec} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.ClusterNetworkSpec}
 */
proto.cluster_controller.ClusterNetworkSpec.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setProtocol(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setPortHttp(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setPortHttps(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.addAlternateDomains(value);
      break;
    case 6:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setAcmeEnabled(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setAdminEmail(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setGatewayFqdn(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setDnsEndpoint(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.addDnsNameservers(value);
      break;
    case 11:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setDnsTtl(value);
      break;
    case 12:
      var value = new proto.cluster_controller.ExternalDNSConfig;
      reader.readMessage(value,proto.cluster_controller.ExternalDNSConfig.deserializeBinaryFromReader);
      msg.setExternalDns(value);
      break;
    case 13:
      var value = new proto.cluster_controller.DomainMigration;
      reader.readMessage(value,proto.cluster_controller.DomainMigration.deserializeBinaryFromReader);
      msg.setDomainMigration(value);
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
proto.cluster_controller.ClusterNetworkSpec.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.ClusterNetworkSpec.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.ClusterNetworkSpec} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ClusterNetworkSpec.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterDomain();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getProtocol();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getPortHttp();
  if (f !== 0) {
    writer.writeUint32(
      3,
      f
    );
  }
  f = message.getPortHttps();
  if (f !== 0) {
    writer.writeUint32(
      4,
      f
    );
  }
  f = message.getAlternateDomainsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      5,
      f
    );
  }
  f = message.getAcmeEnabled();
  if (f) {
    writer.writeBool(
      6,
      f
    );
  }
  f = message.getAdminEmail();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getGatewayFqdn();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
  f = message.getDnsEndpoint();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
  f = message.getDnsNameserversList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      10,
      f
    );
  }
  f = message.getDnsTtl();
  if (f !== 0) {
    writer.writeUint32(
      11,
      f
    );
  }
  f = message.getExternalDns();
  if (f != null) {
    writer.writeMessage(
      12,
      f,
      proto.cluster_controller.ExternalDNSConfig.serializeBinaryToWriter
    );
  }
  f = message.getDomainMigration();
  if (f != null) {
    writer.writeMessage(
      13,
      f,
      proto.cluster_controller.DomainMigration.serializeBinaryToWriter
    );
  }
};


/**
 * optional string cluster_domain = 1;
 * @return {string}
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.getClusterDomain = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ClusterNetworkSpec} returns this
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.setClusterDomain = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string protocol = 2;
 * @return {string}
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.getProtocol = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ClusterNetworkSpec} returns this
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.setProtocol = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional uint32 port_http = 3;
 * @return {number}
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.getPortHttp = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.ClusterNetworkSpec} returns this
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.setPortHttp = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional uint32 port_https = 4;
 * @return {number}
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.getPortHttps = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.ClusterNetworkSpec} returns this
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.setPortHttps = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * repeated string alternate_domains = 5;
 * @return {!Array<string>}
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.getAlternateDomainsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 5));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.ClusterNetworkSpec} returns this
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.setAlternateDomainsList = function(value) {
  return jspb.Message.setField(this, 5, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.ClusterNetworkSpec} returns this
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.addAlternateDomains = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 5, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.ClusterNetworkSpec} returns this
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.clearAlternateDomainsList = function() {
  return this.setAlternateDomainsList([]);
};


/**
 * optional bool acme_enabled = 6;
 * @return {boolean}
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.getAcmeEnabled = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 6, false));
};


/**
 * @param {boolean} value
 * @return {!proto.cluster_controller.ClusterNetworkSpec} returns this
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.setAcmeEnabled = function(value) {
  return jspb.Message.setProto3BooleanField(this, 6, value);
};


/**
 * optional string admin_email = 7;
 * @return {string}
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.getAdminEmail = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ClusterNetworkSpec} returns this
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.setAdminEmail = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional string gateway_fqdn = 8;
 * @return {string}
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.getGatewayFqdn = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ClusterNetworkSpec} returns this
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.setGatewayFqdn = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};


/**
 * optional string dns_endpoint = 9;
 * @return {string}
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.getDnsEndpoint = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ClusterNetworkSpec} returns this
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.setDnsEndpoint = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};


/**
 * repeated string dns_nameservers = 10;
 * @return {!Array<string>}
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.getDnsNameserversList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 10));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.ClusterNetworkSpec} returns this
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.setDnsNameserversList = function(value) {
  return jspb.Message.setField(this, 10, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.ClusterNetworkSpec} returns this
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.addDnsNameservers = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 10, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.ClusterNetworkSpec} returns this
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.clearDnsNameserversList = function() {
  return this.setDnsNameserversList([]);
};


/**
 * optional uint32 dns_ttl = 11;
 * @return {number}
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.getDnsTtl = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 11, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.ClusterNetworkSpec} returns this
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.setDnsTtl = function(value) {
  return jspb.Message.setProto3IntField(this, 11, value);
};


/**
 * optional ExternalDNSConfig external_dns = 12;
 * @return {?proto.cluster_controller.ExternalDNSConfig}
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.getExternalDns = function() {
  return /** @type{?proto.cluster_controller.ExternalDNSConfig} */ (
    jspb.Message.getWrapperField(this, proto.cluster_controller.ExternalDNSConfig, 12));
};


/**
 * @param {?proto.cluster_controller.ExternalDNSConfig|undefined} value
 * @return {!proto.cluster_controller.ClusterNetworkSpec} returns this
*/
proto.cluster_controller.ClusterNetworkSpec.prototype.setExternalDns = function(value) {
  return jspb.Message.setWrapperField(this, 12, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.cluster_controller.ClusterNetworkSpec} returns this
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.clearExternalDns = function() {
  return this.setExternalDns(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.hasExternalDns = function() {
  return jspb.Message.getField(this, 12) != null;
};


/**
 * optional DomainMigration domain_migration = 13;
 * @return {?proto.cluster_controller.DomainMigration}
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.getDomainMigration = function() {
  return /** @type{?proto.cluster_controller.DomainMigration} */ (
    jspb.Message.getWrapperField(this, proto.cluster_controller.DomainMigration, 13));
};


/**
 * @param {?proto.cluster_controller.DomainMigration|undefined} value
 * @return {!proto.cluster_controller.ClusterNetworkSpec} returns this
*/
proto.cluster_controller.ClusterNetworkSpec.prototype.setDomainMigration = function(value) {
  return jspb.Message.setWrapperField(this, 13, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.cluster_controller.ClusterNetworkSpec} returns this
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.clearDomainMigration = function() {
  return this.setDomainMigration(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.cluster_controller.ClusterNetworkSpec.prototype.hasDomainMigration = function() {
  return jspb.Message.getField(this, 13) != null;
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
proto.cluster_controller.DomainMigration.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.DomainMigration.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.DomainMigration} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.DomainMigration.toObject = function(includeInstance, msg) {
  var f, obj = {
oldDomain: jspb.Message.getFieldWithDefault(msg, 1, ""),
newDomain: jspb.Message.getFieldWithDefault(msg, 2, ""),
state: jspb.Message.getFieldWithDefault(msg, 3, 0),
startedAt: jspb.Message.getFieldWithDefault(msg, 4, 0),
gracePeriodSeconds: jspb.Message.getFieldWithDefault(msg, 5, 0)
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
 * @return {!proto.cluster_controller.DomainMigration}
 */
proto.cluster_controller.DomainMigration.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.DomainMigration;
  return proto.cluster_controller.DomainMigration.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.DomainMigration} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.DomainMigration}
 */
proto.cluster_controller.DomainMigration.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setOldDomain(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setNewDomain(value);
      break;
    case 3:
      var value = /** @type {!proto.cluster_controller.DomainMigration.MigrationState} */ (reader.readEnum());
      msg.setState(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setStartedAt(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setGracePeriodSeconds(value);
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
proto.cluster_controller.DomainMigration.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.DomainMigration.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.DomainMigration} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.DomainMigration.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOldDomain();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getNewDomain();
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
  f = message.getStartedAt();
  if (f !== 0) {
    writer.writeInt64(
      4,
      f
    );
  }
  f = message.getGracePeriodSeconds();
  if (f !== 0) {
    writer.writeUint32(
      5,
      f
    );
  }
};


/**
 * @enum {number}
 */
proto.cluster_controller.DomainMigration.MigrationState = {
  MIGRATION_NOT_STARTED: 0,
  MIGRATION_IN_PROGRESS: 1,
  MIGRATION_COMPLETED: 2
};

/**
 * optional string old_domain = 1;
 * @return {string}
 */
proto.cluster_controller.DomainMigration.prototype.getOldDomain = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.DomainMigration} returns this
 */
proto.cluster_controller.DomainMigration.prototype.setOldDomain = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string new_domain = 2;
 * @return {string}
 */
proto.cluster_controller.DomainMigration.prototype.getNewDomain = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.DomainMigration} returns this
 */
proto.cluster_controller.DomainMigration.prototype.setNewDomain = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional MigrationState state = 3;
 * @return {!proto.cluster_controller.DomainMigration.MigrationState}
 */
proto.cluster_controller.DomainMigration.prototype.getState = function() {
  return /** @type {!proto.cluster_controller.DomainMigration.MigrationState} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.cluster_controller.DomainMigration.MigrationState} value
 * @return {!proto.cluster_controller.DomainMigration} returns this
 */
proto.cluster_controller.DomainMigration.prototype.setState = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
};


/**
 * optional int64 started_at = 4;
 * @return {number}
 */
proto.cluster_controller.DomainMigration.prototype.getStartedAt = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.DomainMigration} returns this
 */
proto.cluster_controller.DomainMigration.prototype.setStartedAt = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional uint32 grace_period_seconds = 5;
 * @return {number}
 */
proto.cluster_controller.DomainMigration.prototype.getGracePeriodSeconds = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.DomainMigration} returns this
 */
proto.cluster_controller.DomainMigration.prototype.setGracePeriodSeconds = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.ExternalDNSConfig.repeatedFields_ = [4];



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
proto.cluster_controller.ExternalDNSConfig.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.ExternalDNSConfig.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.ExternalDNSConfig} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ExternalDNSConfig.toObject = function(includeInstance, msg) {
  var f, obj = {
enabled: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
provider: jspb.Message.getFieldWithDefault(msg, 2, ""),
domain: jspb.Message.getFieldWithDefault(msg, 3, ""),
publishList: (f = jspb.Message.getRepeatedField(msg, 4)) == null ? undefined : f,
ttl: jspb.Message.getFieldWithDefault(msg, 5, 0),
allowPrivateIps: jspb.Message.getBooleanFieldWithDefault(msg, 6, false),
providerConfigMap: (f = msg.getProviderConfigMap()) ? f.toObject(includeInstance, undefined) : []
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
 * @return {!proto.cluster_controller.ExternalDNSConfig}
 */
proto.cluster_controller.ExternalDNSConfig.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.ExternalDNSConfig;
  return proto.cluster_controller.ExternalDNSConfig.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.ExternalDNSConfig} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.ExternalDNSConfig}
 */
proto.cluster_controller.ExternalDNSConfig.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setEnabled(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setProvider(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setDomain(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.addPublish(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setTtl(value);
      break;
    case 6:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setAllowPrivateIps(value);
      break;
    case 7:
      var value = msg.getProviderConfigMap();
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
proto.cluster_controller.ExternalDNSConfig.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.ExternalDNSConfig.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.ExternalDNSConfig} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ExternalDNSConfig.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getEnabled();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getProvider();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getDomain();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getPublishList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      4,
      f
    );
  }
  f = message.getTtl();
  if (f !== 0) {
    writer.writeUint32(
      5,
      f
    );
  }
  f = message.getAllowPrivateIps();
  if (f) {
    writer.writeBool(
      6,
      f
    );
  }
  f = message.getProviderConfigMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(7, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
};


/**
 * optional bool enabled = 1;
 * @return {boolean}
 */
proto.cluster_controller.ExternalDNSConfig.prototype.getEnabled = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.cluster_controller.ExternalDNSConfig} returns this
 */
proto.cluster_controller.ExternalDNSConfig.prototype.setEnabled = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional string provider = 2;
 * @return {string}
 */
proto.cluster_controller.ExternalDNSConfig.prototype.getProvider = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ExternalDNSConfig} returns this
 */
proto.cluster_controller.ExternalDNSConfig.prototype.setProvider = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string domain = 3;
 * @return {string}
 */
proto.cluster_controller.ExternalDNSConfig.prototype.getDomain = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ExternalDNSConfig} returns this
 */
proto.cluster_controller.ExternalDNSConfig.prototype.setDomain = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * repeated string publish = 4;
 * @return {!Array<string>}
 */
proto.cluster_controller.ExternalDNSConfig.prototype.getPublishList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 4));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.ExternalDNSConfig} returns this
 */
proto.cluster_controller.ExternalDNSConfig.prototype.setPublishList = function(value) {
  return jspb.Message.setField(this, 4, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.ExternalDNSConfig} returns this
 */
proto.cluster_controller.ExternalDNSConfig.prototype.addPublish = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 4, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.ExternalDNSConfig} returns this
 */
proto.cluster_controller.ExternalDNSConfig.prototype.clearPublishList = function() {
  return this.setPublishList([]);
};


/**
 * optional uint32 ttl = 5;
 * @return {number}
 */
proto.cluster_controller.ExternalDNSConfig.prototype.getTtl = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.ExternalDNSConfig} returns this
 */
proto.cluster_controller.ExternalDNSConfig.prototype.setTtl = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional bool allow_private_ips = 6;
 * @return {boolean}
 */
proto.cluster_controller.ExternalDNSConfig.prototype.getAllowPrivateIps = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 6, false));
};


/**
 * @param {boolean} value
 * @return {!proto.cluster_controller.ExternalDNSConfig} returns this
 */
proto.cluster_controller.ExternalDNSConfig.prototype.setAllowPrivateIps = function(value) {
  return jspb.Message.setProto3BooleanField(this, 6, value);
};


/**
 * map<string, string> provider_config = 7;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.cluster_controller.ExternalDNSConfig.prototype.getProviderConfigMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 7, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.cluster_controller.ExternalDNSConfig} returns this
 */
proto.cluster_controller.ExternalDNSConfig.prototype.clearProviderConfigMap = function() {
  this.getProviderConfigMap().clear();
  return this;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.NodeIdentity.repeatedFields_ = [3];



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
proto.cluster_controller.NodeIdentity.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.NodeIdentity.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.NodeIdentity} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.NodeIdentity.toObject = function(includeInstance, msg) {
  var f, obj = {
hostname: jspb.Message.getFieldWithDefault(msg, 1, ""),
domain: jspb.Message.getFieldWithDefault(msg, 2, ""),
ipsList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f,
os: jspb.Message.getFieldWithDefault(msg, 4, ""),
arch: jspb.Message.getFieldWithDefault(msg, 5, ""),
agentVersion: jspb.Message.getFieldWithDefault(msg, 6, ""),
nodeName: jspb.Message.getFieldWithDefault(msg, 7, ""),
advertiseIp: jspb.Message.getFieldWithDefault(msg, 8, ""),
advertiseFqdn: jspb.Message.getFieldWithDefault(msg, 9, "")
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
 * @return {!proto.cluster_controller.NodeIdentity}
 */
proto.cluster_controller.NodeIdentity.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.NodeIdentity;
  return proto.cluster_controller.NodeIdentity.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.NodeIdentity} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.NodeIdentity}
 */
proto.cluster_controller.NodeIdentity.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setHostname(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setDomain(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.addIps(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setOs(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setArch(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setAgentVersion(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeName(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setAdvertiseIp(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setAdvertiseFqdn(value);
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
proto.cluster_controller.NodeIdentity.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.NodeIdentity.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.NodeIdentity} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.NodeIdentity.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getHostname();
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
  f = message.getIpsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      3,
      f
    );
  }
  f = message.getOs();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getArch();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getAgentVersion();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getNodeName();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getAdvertiseIp();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
  f = message.getAdvertiseFqdn();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
};


/**
 * optional string hostname = 1;
 * @return {string}
 */
proto.cluster_controller.NodeIdentity.prototype.getHostname = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeIdentity} returns this
 */
proto.cluster_controller.NodeIdentity.prototype.setHostname = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string domain = 2;
 * @return {string}
 */
proto.cluster_controller.NodeIdentity.prototype.getDomain = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeIdentity} returns this
 */
proto.cluster_controller.NodeIdentity.prototype.setDomain = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * repeated string ips = 3;
 * @return {!Array<string>}
 */
proto.cluster_controller.NodeIdentity.prototype.getIpsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.NodeIdentity} returns this
 */
proto.cluster_controller.NodeIdentity.prototype.setIpsList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.NodeIdentity} returns this
 */
proto.cluster_controller.NodeIdentity.prototype.addIps = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.NodeIdentity} returns this
 */
proto.cluster_controller.NodeIdentity.prototype.clearIpsList = function() {
  return this.setIpsList([]);
};


/**
 * optional string os = 4;
 * @return {string}
 */
proto.cluster_controller.NodeIdentity.prototype.getOs = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeIdentity} returns this
 */
proto.cluster_controller.NodeIdentity.prototype.setOs = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string arch = 5;
 * @return {string}
 */
proto.cluster_controller.NodeIdentity.prototype.getArch = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeIdentity} returns this
 */
proto.cluster_controller.NodeIdentity.prototype.setArch = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string agent_version = 6;
 * @return {string}
 */
proto.cluster_controller.NodeIdentity.prototype.getAgentVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeIdentity} returns this
 */
proto.cluster_controller.NodeIdentity.prototype.setAgentVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string node_name = 7;
 * @return {string}
 */
proto.cluster_controller.NodeIdentity.prototype.getNodeName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeIdentity} returns this
 */
proto.cluster_controller.NodeIdentity.prototype.setNodeName = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional string advertise_ip = 8;
 * @return {string}
 */
proto.cluster_controller.NodeIdentity.prototype.getAdvertiseIp = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeIdentity} returns this
 */
proto.cluster_controller.NodeIdentity.prototype.setAdvertiseIp = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};


/**
 * optional string advertise_fqdn = 9;
 * @return {string}
 */
proto.cluster_controller.NodeIdentity.prototype.getAdvertiseFqdn = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeIdentity} returns this
 */
proto.cluster_controller.NodeIdentity.prototype.setAdvertiseFqdn = function(value) {
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
proto.cluster_controller.NodeCapabilities.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.NodeCapabilities.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.NodeCapabilities} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.NodeCapabilities.toObject = function(includeInstance, msg) {
  var f, obj = {
cpuCount: jspb.Message.getFieldWithDefault(msg, 1, 0),
ramBytes: jspb.Message.getFieldWithDefault(msg, 2, 0),
diskBytes: jspb.Message.getFieldWithDefault(msg, 3, 0),
diskFreeBytes: jspb.Message.getFieldWithDefault(msg, 4, 0),
canApplyPrivileged: jspb.Message.getBooleanFieldWithDefault(msg, 5, false),
privilegeReason: jspb.Message.getFieldWithDefault(msg, 6, "")
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
 * @return {!proto.cluster_controller.NodeCapabilities}
 */
proto.cluster_controller.NodeCapabilities.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.NodeCapabilities;
  return proto.cluster_controller.NodeCapabilities.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.NodeCapabilities} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.NodeCapabilities}
 */
proto.cluster_controller.NodeCapabilities.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setCpuCount(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readUint64());
      msg.setRamBytes(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readUint64());
      msg.setDiskBytes(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readUint64());
      msg.setDiskFreeBytes(value);
      break;
    case 5:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setCanApplyPrivileged(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setPrivilegeReason(value);
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
proto.cluster_controller.NodeCapabilities.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.NodeCapabilities.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.NodeCapabilities} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.NodeCapabilities.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getCpuCount();
  if (f !== 0) {
    writer.writeUint32(
      1,
      f
    );
  }
  f = message.getRamBytes();
  if (f !== 0) {
    writer.writeUint64(
      2,
      f
    );
  }
  f = message.getDiskBytes();
  if (f !== 0) {
    writer.writeUint64(
      3,
      f
    );
  }
  f = message.getDiskFreeBytes();
  if (f !== 0) {
    writer.writeUint64(
      4,
      f
    );
  }
  f = message.getCanApplyPrivileged();
  if (f) {
    writer.writeBool(
      5,
      f
    );
  }
  f = message.getPrivilegeReason();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
};


/**
 * optional uint32 cpu_count = 1;
 * @return {number}
 */
proto.cluster_controller.NodeCapabilities.prototype.getCpuCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.NodeCapabilities} returns this
 */
proto.cluster_controller.NodeCapabilities.prototype.setCpuCount = function(value) {
  return jspb.Message.setProto3IntField(this, 1, value);
};


/**
 * optional uint64 ram_bytes = 2;
 * @return {number}
 */
proto.cluster_controller.NodeCapabilities.prototype.getRamBytes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.NodeCapabilities} returns this
 */
proto.cluster_controller.NodeCapabilities.prototype.setRamBytes = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional uint64 disk_bytes = 3;
 * @return {number}
 */
proto.cluster_controller.NodeCapabilities.prototype.getDiskBytes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.NodeCapabilities} returns this
 */
proto.cluster_controller.NodeCapabilities.prototype.setDiskBytes = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional uint64 disk_free_bytes = 4;
 * @return {number}
 */
proto.cluster_controller.NodeCapabilities.prototype.getDiskFreeBytes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.NodeCapabilities} returns this
 */
proto.cluster_controller.NodeCapabilities.prototype.setDiskFreeBytes = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional bool can_apply_privileged = 5;
 * @return {boolean}
 */
proto.cluster_controller.NodeCapabilities.prototype.getCanApplyPrivileged = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 5, false));
};


/**
 * @param {boolean} value
 * @return {!proto.cluster_controller.NodeCapabilities} returns this
 */
proto.cluster_controller.NodeCapabilities.prototype.setCanApplyPrivileged = function(value) {
  return jspb.Message.setProto3BooleanField(this, 5, value);
};


/**
 * optional string privilege_reason = 6;
 * @return {string}
 */
proto.cluster_controller.NodeCapabilities.prototype.getPrivilegeReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeCapabilities} returns this
 */
proto.cluster_controller.NodeCapabilities.prototype.setPrivilegeReason = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.NodeRecord.repeatedFields_ = [5];



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
proto.cluster_controller.NodeRecord.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.NodeRecord.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.NodeRecord} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.NodeRecord.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
identity: (f = msg.getIdentity()) && proto.cluster_controller.NodeIdentity.toObject(includeInstance, f),
lastSeen: (f = msg.getLastSeen()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
status: jspb.Message.getFieldWithDefault(msg, 4, ""),
profilesList: (f = jspb.Message.getRepeatedField(msg, 5)) == null ? undefined : f,
metadataMap: (f = msg.getMetadataMap()) ? f.toObject(includeInstance, undefined) : [],
agentEndpoint: jspb.Message.getFieldWithDefault(msg, 7, ""),
advertiseFqdn: jspb.Message.getFieldWithDefault(msg, 8, ""),
capabilities: (f = msg.getCapabilities()) && proto.cluster_controller.NodeCapabilities.toObject(includeInstance, f)
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
 * @return {!proto.cluster_controller.NodeRecord}
 */
proto.cluster_controller.NodeRecord.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.NodeRecord;
  return proto.cluster_controller.NodeRecord.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.NodeRecord} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.NodeRecord}
 */
proto.cluster_controller.NodeRecord.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = new proto.cluster_controller.NodeIdentity;
      reader.readMessage(value,proto.cluster_controller.NodeIdentity.deserializeBinaryFromReader);
      msg.setIdentity(value);
      break;
    case 3:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setLastSeen(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setStatus(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.addProfiles(value);
      break;
    case 6:
      var value = msg.getMetadataMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setAgentEndpoint(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setAdvertiseFqdn(value);
      break;
    case 9:
      var value = new proto.cluster_controller.NodeCapabilities;
      reader.readMessage(value,proto.cluster_controller.NodeCapabilities.deserializeBinaryFromReader);
      msg.setCapabilities(value);
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
proto.cluster_controller.NodeRecord.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.NodeRecord.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.NodeRecord} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.NodeRecord.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIdentity();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.cluster_controller.NodeIdentity.serializeBinaryToWriter
    );
  }
  f = message.getLastSeen();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getStatus();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getProfilesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      5,
      f
    );
  }
  f = message.getMetadataMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(6, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
  f = message.getAgentEndpoint();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getAdvertiseFqdn();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
  f = message.getCapabilities();
  if (f != null) {
    writer.writeMessage(
      9,
      f,
      proto.cluster_controller.NodeCapabilities.serializeBinaryToWriter
    );
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.cluster_controller.NodeRecord.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeRecord} returns this
 */
proto.cluster_controller.NodeRecord.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional NodeIdentity identity = 2;
 * @return {?proto.cluster_controller.NodeIdentity}
 */
proto.cluster_controller.NodeRecord.prototype.getIdentity = function() {
  return /** @type{?proto.cluster_controller.NodeIdentity} */ (
    jspb.Message.getWrapperField(this, proto.cluster_controller.NodeIdentity, 2));
};


/**
 * @param {?proto.cluster_controller.NodeIdentity|undefined} value
 * @return {!proto.cluster_controller.NodeRecord} returns this
*/
proto.cluster_controller.NodeRecord.prototype.setIdentity = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.cluster_controller.NodeRecord} returns this
 */
proto.cluster_controller.NodeRecord.prototype.clearIdentity = function() {
  return this.setIdentity(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.cluster_controller.NodeRecord.prototype.hasIdentity = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional google.protobuf.Timestamp last_seen = 3;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.cluster_controller.NodeRecord.prototype.getLastSeen = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 3));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.cluster_controller.NodeRecord} returns this
*/
proto.cluster_controller.NodeRecord.prototype.setLastSeen = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.cluster_controller.NodeRecord} returns this
 */
proto.cluster_controller.NodeRecord.prototype.clearLastSeen = function() {
  return this.setLastSeen(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.cluster_controller.NodeRecord.prototype.hasLastSeen = function() {
  return jspb.Message.getField(this, 3) != null;
};


/**
 * optional string status = 4;
 * @return {string}
 */
proto.cluster_controller.NodeRecord.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeRecord} returns this
 */
proto.cluster_controller.NodeRecord.prototype.setStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * repeated string profiles = 5;
 * @return {!Array<string>}
 */
proto.cluster_controller.NodeRecord.prototype.getProfilesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 5));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.NodeRecord} returns this
 */
proto.cluster_controller.NodeRecord.prototype.setProfilesList = function(value) {
  return jspb.Message.setField(this, 5, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.NodeRecord} returns this
 */
proto.cluster_controller.NodeRecord.prototype.addProfiles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 5, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.NodeRecord} returns this
 */
proto.cluster_controller.NodeRecord.prototype.clearProfilesList = function() {
  return this.setProfilesList([]);
};


/**
 * map<string, string> metadata = 6;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.cluster_controller.NodeRecord.prototype.getMetadataMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 6, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.cluster_controller.NodeRecord} returns this
 */
proto.cluster_controller.NodeRecord.prototype.clearMetadataMap = function() {
  this.getMetadataMap().clear();
  return this;
};


/**
 * optional string agent_endpoint = 7;
 * @return {string}
 */
proto.cluster_controller.NodeRecord.prototype.getAgentEndpoint = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeRecord} returns this
 */
proto.cluster_controller.NodeRecord.prototype.setAgentEndpoint = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional string advertise_fqdn = 8;
 * @return {string}
 */
proto.cluster_controller.NodeRecord.prototype.getAdvertiseFqdn = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeRecord} returns this
 */
proto.cluster_controller.NodeRecord.prototype.setAdvertiseFqdn = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};


/**
 * optional NodeCapabilities capabilities = 9;
 * @return {?proto.cluster_controller.NodeCapabilities}
 */
proto.cluster_controller.NodeRecord.prototype.getCapabilities = function() {
  return /** @type{?proto.cluster_controller.NodeCapabilities} */ (
    jspb.Message.getWrapperField(this, proto.cluster_controller.NodeCapabilities, 9));
};


/**
 * @param {?proto.cluster_controller.NodeCapabilities|undefined} value
 * @return {!proto.cluster_controller.NodeRecord} returns this
*/
proto.cluster_controller.NodeRecord.prototype.setCapabilities = function(value) {
  return jspb.Message.setWrapperField(this, 9, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.cluster_controller.NodeRecord} returns this
 */
proto.cluster_controller.NodeRecord.prototype.clearCapabilities = function() {
  return this.setCapabilities(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.cluster_controller.NodeRecord.prototype.hasCapabilities = function() {
  return jspb.Message.getField(this, 9) != null;
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
proto.cluster_controller.CreateJoinTokenRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.CreateJoinTokenRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.CreateJoinTokenRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.CreateJoinTokenRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
expiresAt: (f = msg.getExpiresAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f)
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
 * @return {!proto.cluster_controller.CreateJoinTokenRequest}
 */
proto.cluster_controller.CreateJoinTokenRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.CreateJoinTokenRequest;
  return proto.cluster_controller.CreateJoinTokenRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.CreateJoinTokenRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.CreateJoinTokenRequest}
 */
proto.cluster_controller.CreateJoinTokenRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setExpiresAt(value);
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
proto.cluster_controller.CreateJoinTokenRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.CreateJoinTokenRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.CreateJoinTokenRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.CreateJoinTokenRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getExpiresAt();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
};


/**
 * optional google.protobuf.Timestamp expires_at = 1;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.cluster_controller.CreateJoinTokenRequest.prototype.getExpiresAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 1));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.cluster_controller.CreateJoinTokenRequest} returns this
*/
proto.cluster_controller.CreateJoinTokenRequest.prototype.setExpiresAt = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.cluster_controller.CreateJoinTokenRequest} returns this
 */
proto.cluster_controller.CreateJoinTokenRequest.prototype.clearExpiresAt = function() {
  return this.setExpiresAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.cluster_controller.CreateJoinTokenRequest.prototype.hasExpiresAt = function() {
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
proto.cluster_controller.CreateJoinTokenResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.CreateJoinTokenResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.CreateJoinTokenResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.CreateJoinTokenResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
joinToken: jspb.Message.getFieldWithDefault(msg, 1, ""),
expiresAt: (f = msg.getExpiresAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f)
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
 * @return {!proto.cluster_controller.CreateJoinTokenResponse}
 */
proto.cluster_controller.CreateJoinTokenResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.CreateJoinTokenResponse;
  return proto.cluster_controller.CreateJoinTokenResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.CreateJoinTokenResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.CreateJoinTokenResponse}
 */
proto.cluster_controller.CreateJoinTokenResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setJoinToken(value);
      break;
    case 2:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setExpiresAt(value);
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
proto.cluster_controller.CreateJoinTokenResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.CreateJoinTokenResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.CreateJoinTokenResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.CreateJoinTokenResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getJoinToken();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getExpiresAt();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
};


/**
 * optional string join_token = 1;
 * @return {string}
 */
proto.cluster_controller.CreateJoinTokenResponse.prototype.getJoinToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.CreateJoinTokenResponse} returns this
 */
proto.cluster_controller.CreateJoinTokenResponse.prototype.setJoinToken = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional google.protobuf.Timestamp expires_at = 2;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.cluster_controller.CreateJoinTokenResponse.prototype.getExpiresAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 2));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.cluster_controller.CreateJoinTokenResponse} returns this
*/
proto.cluster_controller.CreateJoinTokenResponse.prototype.setExpiresAt = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.cluster_controller.CreateJoinTokenResponse} returns this
 */
proto.cluster_controller.CreateJoinTokenResponse.prototype.clearExpiresAt = function() {
  return this.setExpiresAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.cluster_controller.CreateJoinTokenResponse.prototype.hasExpiresAt = function() {
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
proto.cluster_controller.RequestJoinRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.RequestJoinRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.RequestJoinRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.RequestJoinRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
joinToken: jspb.Message.getFieldWithDefault(msg, 1, ""),
identity: (f = msg.getIdentity()) && proto.cluster_controller.NodeIdentity.toObject(includeInstance, f),
labelsMap: (f = msg.getLabelsMap()) ? f.toObject(includeInstance, undefined) : [],
capabilities: (f = msg.getCapabilities()) && proto.cluster_controller.NodeCapabilities.toObject(includeInstance, f)
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
 * @return {!proto.cluster_controller.RequestJoinRequest}
 */
proto.cluster_controller.RequestJoinRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.RequestJoinRequest;
  return proto.cluster_controller.RequestJoinRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.RequestJoinRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.RequestJoinRequest}
 */
proto.cluster_controller.RequestJoinRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setJoinToken(value);
      break;
    case 2:
      var value = new proto.cluster_controller.NodeIdentity;
      reader.readMessage(value,proto.cluster_controller.NodeIdentity.deserializeBinaryFromReader);
      msg.setIdentity(value);
      break;
    case 3:
      var value = msg.getLabelsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
      break;
    case 4:
      var value = new proto.cluster_controller.NodeCapabilities;
      reader.readMessage(value,proto.cluster_controller.NodeCapabilities.deserializeBinaryFromReader);
      msg.setCapabilities(value);
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
proto.cluster_controller.RequestJoinRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.RequestJoinRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.RequestJoinRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.RequestJoinRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getJoinToken();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIdentity();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.cluster_controller.NodeIdentity.serializeBinaryToWriter
    );
  }
  f = message.getLabelsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(3, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
  f = message.getCapabilities();
  if (f != null) {
    writer.writeMessage(
      4,
      f,
      proto.cluster_controller.NodeCapabilities.serializeBinaryToWriter
    );
  }
};


/**
 * optional string join_token = 1;
 * @return {string}
 */
proto.cluster_controller.RequestJoinRequest.prototype.getJoinToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.RequestJoinRequest} returns this
 */
proto.cluster_controller.RequestJoinRequest.prototype.setJoinToken = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional NodeIdentity identity = 2;
 * @return {?proto.cluster_controller.NodeIdentity}
 */
proto.cluster_controller.RequestJoinRequest.prototype.getIdentity = function() {
  return /** @type{?proto.cluster_controller.NodeIdentity} */ (
    jspb.Message.getWrapperField(this, proto.cluster_controller.NodeIdentity, 2));
};


/**
 * @param {?proto.cluster_controller.NodeIdentity|undefined} value
 * @return {!proto.cluster_controller.RequestJoinRequest} returns this
*/
proto.cluster_controller.RequestJoinRequest.prototype.setIdentity = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.cluster_controller.RequestJoinRequest} returns this
 */
proto.cluster_controller.RequestJoinRequest.prototype.clearIdentity = function() {
  return this.setIdentity(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.cluster_controller.RequestJoinRequest.prototype.hasIdentity = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * map<string, string> labels = 3;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.cluster_controller.RequestJoinRequest.prototype.getLabelsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 3, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.cluster_controller.RequestJoinRequest} returns this
 */
proto.cluster_controller.RequestJoinRequest.prototype.clearLabelsMap = function() {
  this.getLabelsMap().clear();
  return this;
};


/**
 * optional NodeCapabilities capabilities = 4;
 * @return {?proto.cluster_controller.NodeCapabilities}
 */
proto.cluster_controller.RequestJoinRequest.prototype.getCapabilities = function() {
  return /** @type{?proto.cluster_controller.NodeCapabilities} */ (
    jspb.Message.getWrapperField(this, proto.cluster_controller.NodeCapabilities, 4));
};


/**
 * @param {?proto.cluster_controller.NodeCapabilities|undefined} value
 * @return {!proto.cluster_controller.RequestJoinRequest} returns this
*/
proto.cluster_controller.RequestJoinRequest.prototype.setCapabilities = function(value) {
  return jspb.Message.setWrapperField(this, 4, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.cluster_controller.RequestJoinRequest} returns this
 */
proto.cluster_controller.RequestJoinRequest.prototype.clearCapabilities = function() {
  return this.setCapabilities(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.cluster_controller.RequestJoinRequest.prototype.hasCapabilities = function() {
  return jspb.Message.getField(this, 4) != null;
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
proto.cluster_controller.RequestJoinResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.RequestJoinResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.RequestJoinResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.RequestJoinResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
requestId: jspb.Message.getFieldWithDefault(msg, 1, ""),
status: jspb.Message.getFieldWithDefault(msg, 2, ""),
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
 * @return {!proto.cluster_controller.RequestJoinResponse}
 */
proto.cluster_controller.RequestJoinResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.RequestJoinResponse;
  return proto.cluster_controller.RequestJoinResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.RequestJoinResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.RequestJoinResponse}
 */
proto.cluster_controller.RequestJoinResponse.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setStatus(value);
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
proto.cluster_controller.RequestJoinResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.RequestJoinResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.RequestJoinResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.RequestJoinResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRequestId();
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
  f = message.getMessage();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string request_id = 1;
 * @return {string}
 */
proto.cluster_controller.RequestJoinResponse.prototype.getRequestId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.RequestJoinResponse} returns this
 */
proto.cluster_controller.RequestJoinResponse.prototype.setRequestId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string status = 2;
 * @return {string}
 */
proto.cluster_controller.RequestJoinResponse.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.RequestJoinResponse} returns this
 */
proto.cluster_controller.RequestJoinResponse.prototype.setStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string message = 3;
 * @return {string}
 */
proto.cluster_controller.RequestJoinResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.RequestJoinResponse} returns this
 */
proto.cluster_controller.RequestJoinResponse.prototype.setMessage = function(value) {
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
proto.cluster_controller.GetJoinRequestStatusRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.GetJoinRequestStatusRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.GetJoinRequestStatusRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.GetJoinRequestStatusRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
requestId: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.cluster_controller.GetJoinRequestStatusRequest}
 */
proto.cluster_controller.GetJoinRequestStatusRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.GetJoinRequestStatusRequest;
  return proto.cluster_controller.GetJoinRequestStatusRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.GetJoinRequestStatusRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.GetJoinRequestStatusRequest}
 */
proto.cluster_controller.GetJoinRequestStatusRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.cluster_controller.GetJoinRequestStatusRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.GetJoinRequestStatusRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.GetJoinRequestStatusRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.GetJoinRequestStatusRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRequestId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string request_id = 1;
 * @return {string}
 */
proto.cluster_controller.GetJoinRequestStatusRequest.prototype.getRequestId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.GetJoinRequestStatusRequest} returns this
 */
proto.cluster_controller.GetJoinRequestStatusRequest.prototype.setRequestId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.GetJoinRequestStatusResponse.repeatedFields_ = [3];



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
proto.cluster_controller.GetJoinRequestStatusResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.GetJoinRequestStatusResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.GetJoinRequestStatusResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.GetJoinRequestStatusResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
status: jspb.Message.getFieldWithDefault(msg, 1, ""),
nodeId: jspb.Message.getFieldWithDefault(msg, 2, ""),
profilesList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f,
message: jspb.Message.getFieldWithDefault(msg, 4, ""),
nodeToken: jspb.Message.getFieldWithDefault(msg, 5, ""),
nodePrincipal: jspb.Message.getFieldWithDefault(msg, 6, "")
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
 * @return {!proto.cluster_controller.GetJoinRequestStatusResponse}
 */
proto.cluster_controller.GetJoinRequestStatusResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.GetJoinRequestStatusResponse;
  return proto.cluster_controller.GetJoinRequestStatusResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.GetJoinRequestStatusResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.GetJoinRequestStatusResponse}
 */
proto.cluster_controller.GetJoinRequestStatusResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setStatus(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeId(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.addProfiles(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeToken(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodePrincipal(value);
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
proto.cluster_controller.GetJoinRequestStatusResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.GetJoinRequestStatusResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.GetJoinRequestStatusResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.GetJoinRequestStatusResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getStatus();
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
  f = message.getProfilesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
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
  f = message.getNodeToken();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getNodePrincipal();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
};


/**
 * optional string status = 1;
 * @return {string}
 */
proto.cluster_controller.GetJoinRequestStatusResponse.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.GetJoinRequestStatusResponse} returns this
 */
proto.cluster_controller.GetJoinRequestStatusResponse.prototype.setStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string node_id = 2;
 * @return {string}
 */
proto.cluster_controller.GetJoinRequestStatusResponse.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.GetJoinRequestStatusResponse} returns this
 */
proto.cluster_controller.GetJoinRequestStatusResponse.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * repeated string profiles = 3;
 * @return {!Array<string>}
 */
proto.cluster_controller.GetJoinRequestStatusResponse.prototype.getProfilesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.GetJoinRequestStatusResponse} returns this
 */
proto.cluster_controller.GetJoinRequestStatusResponse.prototype.setProfilesList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.GetJoinRequestStatusResponse} returns this
 */
proto.cluster_controller.GetJoinRequestStatusResponse.prototype.addProfiles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.GetJoinRequestStatusResponse} returns this
 */
proto.cluster_controller.GetJoinRequestStatusResponse.prototype.clearProfilesList = function() {
  return this.setProfilesList([]);
};


/**
 * optional string message = 4;
 * @return {string}
 */
proto.cluster_controller.GetJoinRequestStatusResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.GetJoinRequestStatusResponse} returns this
 */
proto.cluster_controller.GetJoinRequestStatusResponse.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string node_token = 5;
 * @return {string}
 */
proto.cluster_controller.GetJoinRequestStatusResponse.prototype.getNodeToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.GetJoinRequestStatusResponse} returns this
 */
proto.cluster_controller.GetJoinRequestStatusResponse.prototype.setNodeToken = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string node_principal = 6;
 * @return {string}
 */
proto.cluster_controller.GetJoinRequestStatusResponse.prototype.getNodePrincipal = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.GetJoinRequestStatusResponse} returns this
 */
proto.cluster_controller.GetJoinRequestStatusResponse.prototype.setNodePrincipal = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.JoinRequestRecord.repeatedFields_ = [5,8];



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
proto.cluster_controller.JoinRequestRecord.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.JoinRequestRecord.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.JoinRequestRecord} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.JoinRequestRecord.toObject = function(includeInstance, msg) {
  var f, obj = {
requestId: jspb.Message.getFieldWithDefault(msg, 1, ""),
identity: (f = msg.getIdentity()) && proto.cluster_controller.NodeIdentity.toObject(includeInstance, f),
status: jspb.Message.getFieldWithDefault(msg, 3, ""),
message: jspb.Message.getFieldWithDefault(msg, 4, ""),
profilesList: (f = jspb.Message.getRepeatedField(msg, 5)) == null ? undefined : f,
metadataMap: (f = msg.getMetadataMap()) ? f.toObject(includeInstance, undefined) : [],
capabilities: (f = msg.getCapabilities()) && proto.cluster_controller.NodeCapabilities.toObject(includeInstance, f),
suggestedProfilesList: (f = jspb.Message.getRepeatedField(msg, 8)) == null ? undefined : f
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
 * @return {!proto.cluster_controller.JoinRequestRecord}
 */
proto.cluster_controller.JoinRequestRecord.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.JoinRequestRecord;
  return proto.cluster_controller.JoinRequestRecord.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.JoinRequestRecord} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.JoinRequestRecord}
 */
proto.cluster_controller.JoinRequestRecord.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = new proto.cluster_controller.NodeIdentity;
      reader.readMessage(value,proto.cluster_controller.NodeIdentity.deserializeBinaryFromReader);
      msg.setIdentity(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setStatus(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.addProfiles(value);
      break;
    case 6:
      var value = msg.getMetadataMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
      break;
    case 7:
      var value = new proto.cluster_controller.NodeCapabilities;
      reader.readMessage(value,proto.cluster_controller.NodeCapabilities.deserializeBinaryFromReader);
      msg.setCapabilities(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.addSuggestedProfiles(value);
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
proto.cluster_controller.JoinRequestRecord.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.JoinRequestRecord.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.JoinRequestRecord} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.JoinRequestRecord.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRequestId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIdentity();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.cluster_controller.NodeIdentity.serializeBinaryToWriter
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
  f = message.getProfilesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      5,
      f
    );
  }
  f = message.getMetadataMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(6, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
  f = message.getCapabilities();
  if (f != null) {
    writer.writeMessage(
      7,
      f,
      proto.cluster_controller.NodeCapabilities.serializeBinaryToWriter
    );
  }
  f = message.getSuggestedProfilesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      8,
      f
    );
  }
};


/**
 * optional string request_id = 1;
 * @return {string}
 */
proto.cluster_controller.JoinRequestRecord.prototype.getRequestId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.JoinRequestRecord} returns this
 */
proto.cluster_controller.JoinRequestRecord.prototype.setRequestId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional NodeIdentity identity = 2;
 * @return {?proto.cluster_controller.NodeIdentity}
 */
proto.cluster_controller.JoinRequestRecord.prototype.getIdentity = function() {
  return /** @type{?proto.cluster_controller.NodeIdentity} */ (
    jspb.Message.getWrapperField(this, proto.cluster_controller.NodeIdentity, 2));
};


/**
 * @param {?proto.cluster_controller.NodeIdentity|undefined} value
 * @return {!proto.cluster_controller.JoinRequestRecord} returns this
*/
proto.cluster_controller.JoinRequestRecord.prototype.setIdentity = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.cluster_controller.JoinRequestRecord} returns this
 */
proto.cluster_controller.JoinRequestRecord.prototype.clearIdentity = function() {
  return this.setIdentity(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.cluster_controller.JoinRequestRecord.prototype.hasIdentity = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional string status = 3;
 * @return {string}
 */
proto.cluster_controller.JoinRequestRecord.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.JoinRequestRecord} returns this
 */
proto.cluster_controller.JoinRequestRecord.prototype.setStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string message = 4;
 * @return {string}
 */
proto.cluster_controller.JoinRequestRecord.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.JoinRequestRecord} returns this
 */
proto.cluster_controller.JoinRequestRecord.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * repeated string profiles = 5;
 * @return {!Array<string>}
 */
proto.cluster_controller.JoinRequestRecord.prototype.getProfilesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 5));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.JoinRequestRecord} returns this
 */
proto.cluster_controller.JoinRequestRecord.prototype.setProfilesList = function(value) {
  return jspb.Message.setField(this, 5, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.JoinRequestRecord} returns this
 */
proto.cluster_controller.JoinRequestRecord.prototype.addProfiles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 5, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.JoinRequestRecord} returns this
 */
proto.cluster_controller.JoinRequestRecord.prototype.clearProfilesList = function() {
  return this.setProfilesList([]);
};


/**
 * map<string, string> metadata = 6;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.cluster_controller.JoinRequestRecord.prototype.getMetadataMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 6, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.cluster_controller.JoinRequestRecord} returns this
 */
proto.cluster_controller.JoinRequestRecord.prototype.clearMetadataMap = function() {
  this.getMetadataMap().clear();
  return this;
};


/**
 * optional NodeCapabilities capabilities = 7;
 * @return {?proto.cluster_controller.NodeCapabilities}
 */
proto.cluster_controller.JoinRequestRecord.prototype.getCapabilities = function() {
  return /** @type{?proto.cluster_controller.NodeCapabilities} */ (
    jspb.Message.getWrapperField(this, proto.cluster_controller.NodeCapabilities, 7));
};


/**
 * @param {?proto.cluster_controller.NodeCapabilities|undefined} value
 * @return {!proto.cluster_controller.JoinRequestRecord} returns this
*/
proto.cluster_controller.JoinRequestRecord.prototype.setCapabilities = function(value) {
  return jspb.Message.setWrapperField(this, 7, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.cluster_controller.JoinRequestRecord} returns this
 */
proto.cluster_controller.JoinRequestRecord.prototype.clearCapabilities = function() {
  return this.setCapabilities(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.cluster_controller.JoinRequestRecord.prototype.hasCapabilities = function() {
  return jspb.Message.getField(this, 7) != null;
};


/**
 * repeated string suggested_profiles = 8;
 * @return {!Array<string>}
 */
proto.cluster_controller.JoinRequestRecord.prototype.getSuggestedProfilesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 8));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.JoinRequestRecord} returns this
 */
proto.cluster_controller.JoinRequestRecord.prototype.setSuggestedProfilesList = function(value) {
  return jspb.Message.setField(this, 8, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.JoinRequestRecord} returns this
 */
proto.cluster_controller.JoinRequestRecord.prototype.addSuggestedProfiles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 8, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.JoinRequestRecord} returns this
 */
proto.cluster_controller.JoinRequestRecord.prototype.clearSuggestedProfilesList = function() {
  return this.setSuggestedProfilesList([]);
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
proto.cluster_controller.ListJoinRequestsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.ListJoinRequestsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.ListJoinRequestsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ListJoinRequestsRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.cluster_controller.ListJoinRequestsRequest}
 */
proto.cluster_controller.ListJoinRequestsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.ListJoinRequestsRequest;
  return proto.cluster_controller.ListJoinRequestsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.ListJoinRequestsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.ListJoinRequestsRequest}
 */
proto.cluster_controller.ListJoinRequestsRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.cluster_controller.ListJoinRequestsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.ListJoinRequestsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.ListJoinRequestsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ListJoinRequestsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.ListJoinRequestsResponse.repeatedFields_ = [1];



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
proto.cluster_controller.ListJoinRequestsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.ListJoinRequestsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.ListJoinRequestsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ListJoinRequestsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
pendingList: jspb.Message.toObjectList(msg.getPendingList(),
    proto.cluster_controller.JoinRequestRecord.toObject, includeInstance)
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
 * @return {!proto.cluster_controller.ListJoinRequestsResponse}
 */
proto.cluster_controller.ListJoinRequestsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.ListJoinRequestsResponse;
  return proto.cluster_controller.ListJoinRequestsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.ListJoinRequestsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.ListJoinRequestsResponse}
 */
proto.cluster_controller.ListJoinRequestsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.cluster_controller.JoinRequestRecord;
      reader.readMessage(value,proto.cluster_controller.JoinRequestRecord.deserializeBinaryFromReader);
      msg.addPending(value);
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
proto.cluster_controller.ListJoinRequestsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.ListJoinRequestsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.ListJoinRequestsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ListJoinRequestsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPendingList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.cluster_controller.JoinRequestRecord.serializeBinaryToWriter
    );
  }
};


/**
 * repeated JoinRequestRecord pending = 1;
 * @return {!Array<!proto.cluster_controller.JoinRequestRecord>}
 */
proto.cluster_controller.ListJoinRequestsResponse.prototype.getPendingList = function() {
  return /** @type{!Array<!proto.cluster_controller.JoinRequestRecord>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.cluster_controller.JoinRequestRecord, 1));
};


/**
 * @param {!Array<!proto.cluster_controller.JoinRequestRecord>} value
 * @return {!proto.cluster_controller.ListJoinRequestsResponse} returns this
*/
proto.cluster_controller.ListJoinRequestsResponse.prototype.setPendingList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.cluster_controller.JoinRequestRecord=} opt_value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.JoinRequestRecord}
 */
proto.cluster_controller.ListJoinRequestsResponse.prototype.addPending = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.cluster_controller.JoinRequestRecord, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.ListJoinRequestsResponse} returns this
 */
proto.cluster_controller.ListJoinRequestsResponse.prototype.clearPendingList = function() {
  return this.setPendingList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.ApproveJoinRequest.repeatedFields_ = [3];



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
proto.cluster_controller.ApproveJoinRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.ApproveJoinRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.ApproveJoinRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ApproveJoinRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
requestId: jspb.Message.getFieldWithDefault(msg, 1, ""),
nodeId: jspb.Message.getFieldWithDefault(msg, 2, ""),
profilesList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f,
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
 * @return {!proto.cluster_controller.ApproveJoinRequest}
 */
proto.cluster_controller.ApproveJoinRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.ApproveJoinRequest;
  return proto.cluster_controller.ApproveJoinRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.ApproveJoinRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.ApproveJoinRequest}
 */
proto.cluster_controller.ApproveJoinRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.addProfiles(value);
      break;
    case 4:
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
proto.cluster_controller.ApproveJoinRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.ApproveJoinRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.ApproveJoinRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ApproveJoinRequest.serializeBinaryToWriter = function(message, writer) {
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
  f = message.getProfilesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      3,
      f
    );
  }
  f = message.getMetadataMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(4, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
};


/**
 * optional string request_id = 1;
 * @return {string}
 */
proto.cluster_controller.ApproveJoinRequest.prototype.getRequestId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ApproveJoinRequest} returns this
 */
proto.cluster_controller.ApproveJoinRequest.prototype.setRequestId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string node_id = 2;
 * @return {string}
 */
proto.cluster_controller.ApproveJoinRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ApproveJoinRequest} returns this
 */
proto.cluster_controller.ApproveJoinRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * repeated string profiles = 3;
 * @return {!Array<string>}
 */
proto.cluster_controller.ApproveJoinRequest.prototype.getProfilesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.ApproveJoinRequest} returns this
 */
proto.cluster_controller.ApproveJoinRequest.prototype.setProfilesList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.ApproveJoinRequest} returns this
 */
proto.cluster_controller.ApproveJoinRequest.prototype.addProfiles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.ApproveJoinRequest} returns this
 */
proto.cluster_controller.ApproveJoinRequest.prototype.clearProfilesList = function() {
  return this.setProfilesList([]);
};


/**
 * map<string, string> metadata = 4;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.cluster_controller.ApproveJoinRequest.prototype.getMetadataMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 4, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.cluster_controller.ApproveJoinRequest} returns this
 */
proto.cluster_controller.ApproveJoinRequest.prototype.clearMetadataMap = function() {
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
proto.cluster_controller.ApproveJoinResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.ApproveJoinResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.ApproveJoinResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ApproveJoinResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
message: jspb.Message.getFieldWithDefault(msg, 2, ""),
nodeToken: jspb.Message.getFieldWithDefault(msg, 3, ""),
nodePrincipal: jspb.Message.getFieldWithDefault(msg, 4, "")
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
 * @return {!proto.cluster_controller.ApproveJoinResponse}
 */
proto.cluster_controller.ApproveJoinResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.ApproveJoinResponse;
  return proto.cluster_controller.ApproveJoinResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.ApproveJoinResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.ApproveJoinResponse}
 */
proto.cluster_controller.ApproveJoinResponse.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setMessage(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeToken(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodePrincipal(value);
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
proto.cluster_controller.ApproveJoinResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.ApproveJoinResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.ApproveJoinResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ApproveJoinResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
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
  f = message.getNodeToken();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getNodePrincipal();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.cluster_controller.ApproveJoinResponse.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ApproveJoinResponse} returns this
 */
proto.cluster_controller.ApproveJoinResponse.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string message = 2;
 * @return {string}
 */
proto.cluster_controller.ApproveJoinResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ApproveJoinResponse} returns this
 */
proto.cluster_controller.ApproveJoinResponse.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string node_token = 3;
 * @return {string}
 */
proto.cluster_controller.ApproveJoinResponse.prototype.getNodeToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ApproveJoinResponse} returns this
 */
proto.cluster_controller.ApproveJoinResponse.prototype.setNodeToken = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string node_principal = 4;
 * @return {string}
 */
proto.cluster_controller.ApproveJoinResponse.prototype.getNodePrincipal = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ApproveJoinResponse} returns this
 */
proto.cluster_controller.ApproveJoinResponse.prototype.setNodePrincipal = function(value) {
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
proto.cluster_controller.RejectJoinRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.RejectJoinRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.RejectJoinRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.RejectJoinRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
requestId: jspb.Message.getFieldWithDefault(msg, 1, ""),
nodeId: jspb.Message.getFieldWithDefault(msg, 2, ""),
reason: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.cluster_controller.RejectJoinRequest}
 */
proto.cluster_controller.RejectJoinRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.RejectJoinRequest;
  return proto.cluster_controller.RejectJoinRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.RejectJoinRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.RejectJoinRequest}
 */
proto.cluster_controller.RejectJoinRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.cluster_controller.RejectJoinRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.RejectJoinRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.RejectJoinRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.RejectJoinRequest.serializeBinaryToWriter = function(message, writer) {
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
  f = message.getReason();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string request_id = 1;
 * @return {string}
 */
proto.cluster_controller.RejectJoinRequest.prototype.getRequestId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.RejectJoinRequest} returns this
 */
proto.cluster_controller.RejectJoinRequest.prototype.setRequestId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string node_id = 2;
 * @return {string}
 */
proto.cluster_controller.RejectJoinRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.RejectJoinRequest} returns this
 */
proto.cluster_controller.RejectJoinRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string reason = 3;
 * @return {string}
 */
proto.cluster_controller.RejectJoinRequest.prototype.getReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.RejectJoinRequest} returns this
 */
proto.cluster_controller.RejectJoinRequest.prototype.setReason = function(value) {
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
proto.cluster_controller.RejectJoinResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.RejectJoinResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.RejectJoinResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.RejectJoinResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
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
 * @return {!proto.cluster_controller.RejectJoinResponse}
 */
proto.cluster_controller.RejectJoinResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.RejectJoinResponse;
  return proto.cluster_controller.RejectJoinResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.RejectJoinResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.RejectJoinResponse}
 */
proto.cluster_controller.RejectJoinResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.cluster_controller.RejectJoinResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.RejectJoinResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.RejectJoinResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.RejectJoinResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
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
 * optional string node_id = 1;
 * @return {string}
 */
proto.cluster_controller.RejectJoinResponse.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.RejectJoinResponse} returns this
 */
proto.cluster_controller.RejectJoinResponse.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string message = 2;
 * @return {string}
 */
proto.cluster_controller.RejectJoinResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.RejectJoinResponse} returns this
 */
proto.cluster_controller.RejectJoinResponse.prototype.setMessage = function(value) {
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
proto.cluster_controller.ListNodesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.ListNodesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.ListNodesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ListNodesRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.cluster_controller.ListNodesRequest}
 */
proto.cluster_controller.ListNodesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.ListNodesRequest;
  return proto.cluster_controller.ListNodesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.ListNodesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.ListNodesRequest}
 */
proto.cluster_controller.ListNodesRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.cluster_controller.ListNodesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.ListNodesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.ListNodesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ListNodesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.ListNodesResponse.repeatedFields_ = [1];



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
proto.cluster_controller.ListNodesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.ListNodesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.ListNodesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ListNodesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
nodesList: jspb.Message.toObjectList(msg.getNodesList(),
    proto.cluster_controller.NodeRecord.toObject, includeInstance)
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
 * @return {!proto.cluster_controller.ListNodesResponse}
 */
proto.cluster_controller.ListNodesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.ListNodesResponse;
  return proto.cluster_controller.ListNodesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.ListNodesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.ListNodesResponse}
 */
proto.cluster_controller.ListNodesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.cluster_controller.NodeRecord;
      reader.readMessage(value,proto.cluster_controller.NodeRecord.deserializeBinaryFromReader);
      msg.addNodes(value);
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
proto.cluster_controller.ListNodesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.ListNodesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.ListNodesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ListNodesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.cluster_controller.NodeRecord.serializeBinaryToWriter
    );
  }
};


/**
 * repeated NodeRecord nodes = 1;
 * @return {!Array<!proto.cluster_controller.NodeRecord>}
 */
proto.cluster_controller.ListNodesResponse.prototype.getNodesList = function() {
  return /** @type{!Array<!proto.cluster_controller.NodeRecord>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.cluster_controller.NodeRecord, 1));
};


/**
 * @param {!Array<!proto.cluster_controller.NodeRecord>} value
 * @return {!proto.cluster_controller.ListNodesResponse} returns this
*/
proto.cluster_controller.ListNodesResponse.prototype.setNodesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.cluster_controller.NodeRecord=} opt_value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.NodeRecord}
 */
proto.cluster_controller.ListNodesResponse.prototype.addNodes = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.cluster_controller.NodeRecord, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.ListNodesResponse} returns this
 */
proto.cluster_controller.ListNodesResponse.prototype.clearNodesList = function() {
  return this.setNodesList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.SetNodeProfilesRequest.repeatedFields_ = [2];



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
proto.cluster_controller.SetNodeProfilesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.SetNodeProfilesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.SetNodeProfilesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.SetNodeProfilesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
profilesList: (f = jspb.Message.getRepeatedField(msg, 2)) == null ? undefined : f
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
 * @return {!proto.cluster_controller.SetNodeProfilesRequest}
 */
proto.cluster_controller.SetNodeProfilesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.SetNodeProfilesRequest;
  return proto.cluster_controller.SetNodeProfilesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.SetNodeProfilesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.SetNodeProfilesRequest}
 */
proto.cluster_controller.SetNodeProfilesRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.cluster_controller.SetNodeProfilesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.SetNodeProfilesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.SetNodeProfilesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.SetNodeProfilesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getProfilesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      2,
      f
    );
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.cluster_controller.SetNodeProfilesRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.SetNodeProfilesRequest} returns this
 */
proto.cluster_controller.SetNodeProfilesRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * repeated string profiles = 2;
 * @return {!Array<string>}
 */
proto.cluster_controller.SetNodeProfilesRequest.prototype.getProfilesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.SetNodeProfilesRequest} returns this
 */
proto.cluster_controller.SetNodeProfilesRequest.prototype.setProfilesList = function(value) {
  return jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.SetNodeProfilesRequest} returns this
 */
proto.cluster_controller.SetNodeProfilesRequest.prototype.addProfiles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.SetNodeProfilesRequest} returns this
 */
proto.cluster_controller.SetNodeProfilesRequest.prototype.clearProfilesList = function() {
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
proto.cluster_controller.SetNodeProfilesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.SetNodeProfilesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.SetNodeProfilesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.SetNodeProfilesResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.cluster_controller.SetNodeProfilesResponse}
 */
proto.cluster_controller.SetNodeProfilesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.SetNodeProfilesResponse;
  return proto.cluster_controller.SetNodeProfilesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.SetNodeProfilesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.SetNodeProfilesResponse}
 */
proto.cluster_controller.SetNodeProfilesResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.cluster_controller.SetNodeProfilesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.SetNodeProfilesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.SetNodeProfilesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.SetNodeProfilesResponse.serializeBinaryToWriter = function(message, writer) {
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
proto.cluster_controller.SetNodeProfilesResponse.prototype.getOperationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.SetNodeProfilesResponse} returns this
 */
proto.cluster_controller.SetNodeProfilesResponse.prototype.setOperationId = function(value) {
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
proto.cluster_controller.RemoveNodeRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.RemoveNodeRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.RemoveNodeRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.RemoveNodeRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
force: jspb.Message.getBooleanFieldWithDefault(msg, 2, false),
drain: jspb.Message.getBooleanFieldWithDefault(msg, 3, false)
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
 * @return {!proto.cluster_controller.RemoveNodeRequest}
 */
proto.cluster_controller.RemoveNodeRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.RemoveNodeRequest;
  return proto.cluster_controller.RemoveNodeRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.RemoveNodeRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.RemoveNodeRequest}
 */
proto.cluster_controller.RemoveNodeRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setForce(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setDrain(value);
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
proto.cluster_controller.RemoveNodeRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.RemoveNodeRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.RemoveNodeRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.RemoveNodeRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
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
  f = message.getDrain();
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
proto.cluster_controller.RemoveNodeRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.RemoveNodeRequest} returns this
 */
proto.cluster_controller.RemoveNodeRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bool force = 2;
 * @return {boolean}
 */
proto.cluster_controller.RemoveNodeRequest.prototype.getForce = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.cluster_controller.RemoveNodeRequest} returns this
 */
proto.cluster_controller.RemoveNodeRequest.prototype.setForce = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * optional bool drain = 3;
 * @return {boolean}
 */
proto.cluster_controller.RemoveNodeRequest.prototype.getDrain = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.cluster_controller.RemoveNodeRequest} returns this
 */
proto.cluster_controller.RemoveNodeRequest.prototype.setDrain = function(value) {
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
proto.cluster_controller.RemoveNodeResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.RemoveNodeResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.RemoveNodeResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.RemoveNodeResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
operationId: jspb.Message.getFieldWithDefault(msg, 1, ""),
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
 * @return {!proto.cluster_controller.RemoveNodeResponse}
 */
proto.cluster_controller.RemoveNodeResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.RemoveNodeResponse;
  return proto.cluster_controller.RemoveNodeResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.RemoveNodeResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.RemoveNodeResponse}
 */
proto.cluster_controller.RemoveNodeResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.cluster_controller.RemoveNodeResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.RemoveNodeResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.RemoveNodeResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.RemoveNodeResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOperationId();
  if (f.length > 0) {
    writer.writeString(
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
 * optional string operation_id = 1;
 * @return {string}
 */
proto.cluster_controller.RemoveNodeResponse.prototype.getOperationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.RemoveNodeResponse} returns this
 */
proto.cluster_controller.RemoveNodeResponse.prototype.setOperationId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string message = 2;
 * @return {string}
 */
proto.cluster_controller.RemoveNodeResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.RemoveNodeResponse} returns this
 */
proto.cluster_controller.RemoveNodeResponse.prototype.setMessage = function(value) {
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
proto.cluster_controller.GetClusterHealthRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.GetClusterHealthRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.GetClusterHealthRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.GetClusterHealthRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.cluster_controller.GetClusterHealthRequest}
 */
proto.cluster_controller.GetClusterHealthRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.GetClusterHealthRequest;
  return proto.cluster_controller.GetClusterHealthRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.GetClusterHealthRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.GetClusterHealthRequest}
 */
proto.cluster_controller.GetClusterHealthRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.cluster_controller.GetClusterHealthRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.GetClusterHealthRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.GetClusterHealthRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.GetClusterHealthRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.GetClusterHealthResponse.repeatedFields_ = [6];



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
proto.cluster_controller.GetClusterHealthResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.GetClusterHealthResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.GetClusterHealthResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.GetClusterHealthResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
status: jspb.Message.getFieldWithDefault(msg, 1, ""),
totalNodes: jspb.Message.getFieldWithDefault(msg, 2, 0),
healthyNodes: jspb.Message.getFieldWithDefault(msg, 3, 0),
unhealthyNodes: jspb.Message.getFieldWithDefault(msg, 4, 0),
unknownNodes: jspb.Message.getFieldWithDefault(msg, 5, 0),
nodeHealthList: jspb.Message.toObjectList(msg.getNodeHealthList(),
    proto.cluster_controller.NodeHealthStatus.toObject, includeInstance)
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
 * @return {!proto.cluster_controller.GetClusterHealthResponse}
 */
proto.cluster_controller.GetClusterHealthResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.GetClusterHealthResponse;
  return proto.cluster_controller.GetClusterHealthResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.GetClusterHealthResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.GetClusterHealthResponse}
 */
proto.cluster_controller.GetClusterHealthResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setStatus(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setTotalNodes(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setHealthyNodes(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setUnhealthyNodes(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setUnknownNodes(value);
      break;
    case 6:
      var value = new proto.cluster_controller.NodeHealthStatus;
      reader.readMessage(value,proto.cluster_controller.NodeHealthStatus.deserializeBinaryFromReader);
      msg.addNodeHealth(value);
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
proto.cluster_controller.GetClusterHealthResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.GetClusterHealthResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.GetClusterHealthResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.GetClusterHealthResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getStatus();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getTotalNodes();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getHealthyNodes();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
  f = message.getUnhealthyNodes();
  if (f !== 0) {
    writer.writeInt32(
      4,
      f
    );
  }
  f = message.getUnknownNodes();
  if (f !== 0) {
    writer.writeInt32(
      5,
      f
    );
  }
  f = message.getNodeHealthList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      6,
      f,
      proto.cluster_controller.NodeHealthStatus.serializeBinaryToWriter
    );
  }
};


/**
 * optional string status = 1;
 * @return {string}
 */
proto.cluster_controller.GetClusterHealthResponse.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.GetClusterHealthResponse} returns this
 */
proto.cluster_controller.GetClusterHealthResponse.prototype.setStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int32 total_nodes = 2;
 * @return {number}
 */
proto.cluster_controller.GetClusterHealthResponse.prototype.getTotalNodes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.GetClusterHealthResponse} returns this
 */
proto.cluster_controller.GetClusterHealthResponse.prototype.setTotalNodes = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional int32 healthy_nodes = 3;
 * @return {number}
 */
proto.cluster_controller.GetClusterHealthResponse.prototype.getHealthyNodes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.GetClusterHealthResponse} returns this
 */
proto.cluster_controller.GetClusterHealthResponse.prototype.setHealthyNodes = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional int32 unhealthy_nodes = 4;
 * @return {number}
 */
proto.cluster_controller.GetClusterHealthResponse.prototype.getUnhealthyNodes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.GetClusterHealthResponse} returns this
 */
proto.cluster_controller.GetClusterHealthResponse.prototype.setUnhealthyNodes = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional int32 unknown_nodes = 5;
 * @return {number}
 */
proto.cluster_controller.GetClusterHealthResponse.prototype.getUnknownNodes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.GetClusterHealthResponse} returns this
 */
proto.cluster_controller.GetClusterHealthResponse.prototype.setUnknownNodes = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * repeated NodeHealthStatus node_health = 6;
 * @return {!Array<!proto.cluster_controller.NodeHealthStatus>}
 */
proto.cluster_controller.GetClusterHealthResponse.prototype.getNodeHealthList = function() {
  return /** @type{!Array<!proto.cluster_controller.NodeHealthStatus>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.cluster_controller.NodeHealthStatus, 6));
};


/**
 * @param {!Array<!proto.cluster_controller.NodeHealthStatus>} value
 * @return {!proto.cluster_controller.GetClusterHealthResponse} returns this
*/
proto.cluster_controller.GetClusterHealthResponse.prototype.setNodeHealthList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 6, value);
};


/**
 * @param {!proto.cluster_controller.NodeHealthStatus=} opt_value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.NodeHealthStatus}
 */
proto.cluster_controller.GetClusterHealthResponse.prototype.addNodeHealth = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 6, opt_value, proto.cluster_controller.NodeHealthStatus, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.GetClusterHealthResponse} returns this
 */
proto.cluster_controller.GetClusterHealthResponse.prototype.clearNodeHealthList = function() {
  return this.setNodeHealthList([]);
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
proto.cluster_controller.NodeHealthStatus.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.NodeHealthStatus.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.NodeHealthStatus} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.NodeHealthStatus.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
hostname: jspb.Message.getFieldWithDefault(msg, 2, ""),
status: jspb.Message.getFieldWithDefault(msg, 3, ""),
lastError: jspb.Message.getFieldWithDefault(msg, 4, ""),
lastSeen: (f = msg.getLastSeen()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
failedChecks: jspb.Message.getFieldWithDefault(msg, 6, 0)
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
 * @return {!proto.cluster_controller.NodeHealthStatus}
 */
proto.cluster_controller.NodeHealthStatus.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.NodeHealthStatus;
  return proto.cluster_controller.NodeHealthStatus.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.NodeHealthStatus} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.NodeHealthStatus}
 */
proto.cluster_controller.NodeHealthStatus.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setStatus(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setLastError(value);
      break;
    case 5:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setLastSeen(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setFailedChecks(value);
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
proto.cluster_controller.NodeHealthStatus.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.NodeHealthStatus.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.NodeHealthStatus} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.NodeHealthStatus.serializeBinaryToWriter = function(message, writer) {
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
  f = message.getStatus();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getLastError();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getLastSeen();
  if (f != null) {
    writer.writeMessage(
      5,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getFailedChecks();
  if (f !== 0) {
    writer.writeInt32(
      6,
      f
    );
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.cluster_controller.NodeHealthStatus.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeHealthStatus} returns this
 */
proto.cluster_controller.NodeHealthStatus.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string hostname = 2;
 * @return {string}
 */
proto.cluster_controller.NodeHealthStatus.prototype.getHostname = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeHealthStatus} returns this
 */
proto.cluster_controller.NodeHealthStatus.prototype.setHostname = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string status = 3;
 * @return {string}
 */
proto.cluster_controller.NodeHealthStatus.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeHealthStatus} returns this
 */
proto.cluster_controller.NodeHealthStatus.prototype.setStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string last_error = 4;
 * @return {string}
 */
proto.cluster_controller.NodeHealthStatus.prototype.getLastError = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeHealthStatus} returns this
 */
proto.cluster_controller.NodeHealthStatus.prototype.setLastError = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional google.protobuf.Timestamp last_seen = 5;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.cluster_controller.NodeHealthStatus.prototype.getLastSeen = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 5));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.cluster_controller.NodeHealthStatus} returns this
*/
proto.cluster_controller.NodeHealthStatus.prototype.setLastSeen = function(value) {
  return jspb.Message.setWrapperField(this, 5, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.cluster_controller.NodeHealthStatus} returns this
 */
proto.cluster_controller.NodeHealthStatus.prototype.clearLastSeen = function() {
  return this.setLastSeen(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.cluster_controller.NodeHealthStatus.prototype.hasLastSeen = function() {
  return jspb.Message.getField(this, 5) != null;
};


/**
 * optional int32 failed_checks = 6;
 * @return {number}
 */
proto.cluster_controller.NodeHealthStatus.prototype.getFailedChecks = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.NodeHealthStatus} returns this
 */
proto.cluster_controller.NodeHealthStatus.prototype.setFailedChecks = function(value) {
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
proto.cluster_controller.UpdateClusterNetworkRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.UpdateClusterNetworkRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.UpdateClusterNetworkRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.UpdateClusterNetworkRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
spec: (f = msg.getSpec()) && proto.cluster_controller.ClusterNetworkSpec.toObject(includeInstance, f)
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
 * @return {!proto.cluster_controller.UpdateClusterNetworkRequest}
 */
proto.cluster_controller.UpdateClusterNetworkRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.UpdateClusterNetworkRequest;
  return proto.cluster_controller.UpdateClusterNetworkRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.UpdateClusterNetworkRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.UpdateClusterNetworkRequest}
 */
proto.cluster_controller.UpdateClusterNetworkRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.cluster_controller.ClusterNetworkSpec;
      reader.readMessage(value,proto.cluster_controller.ClusterNetworkSpec.deserializeBinaryFromReader);
      msg.setSpec(value);
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
proto.cluster_controller.UpdateClusterNetworkRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.UpdateClusterNetworkRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.UpdateClusterNetworkRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.UpdateClusterNetworkRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSpec();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.cluster_controller.ClusterNetworkSpec.serializeBinaryToWriter
    );
  }
};


/**
 * optional ClusterNetworkSpec spec = 1;
 * @return {?proto.cluster_controller.ClusterNetworkSpec}
 */
proto.cluster_controller.UpdateClusterNetworkRequest.prototype.getSpec = function() {
  return /** @type{?proto.cluster_controller.ClusterNetworkSpec} */ (
    jspb.Message.getWrapperField(this, proto.cluster_controller.ClusterNetworkSpec, 1));
};


/**
 * @param {?proto.cluster_controller.ClusterNetworkSpec|undefined} value
 * @return {!proto.cluster_controller.UpdateClusterNetworkRequest} returns this
*/
proto.cluster_controller.UpdateClusterNetworkRequest.prototype.setSpec = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.cluster_controller.UpdateClusterNetworkRequest} returns this
 */
proto.cluster_controller.UpdateClusterNetworkRequest.prototype.clearSpec = function() {
  return this.setSpec(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.cluster_controller.UpdateClusterNetworkRequest.prototype.hasSpec = function() {
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
proto.cluster_controller.UpdateClusterNetworkResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.UpdateClusterNetworkResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.UpdateClusterNetworkResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.UpdateClusterNetworkResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
generation: jspb.Message.getFieldWithDefault(msg, 1, 0)
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
 * @return {!proto.cluster_controller.UpdateClusterNetworkResponse}
 */
proto.cluster_controller.UpdateClusterNetworkResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.UpdateClusterNetworkResponse;
  return proto.cluster_controller.UpdateClusterNetworkResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.UpdateClusterNetworkResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.UpdateClusterNetworkResponse}
 */
proto.cluster_controller.UpdateClusterNetworkResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readUint64());
      msg.setGeneration(value);
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
proto.cluster_controller.UpdateClusterNetworkResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.UpdateClusterNetworkResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.UpdateClusterNetworkResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.UpdateClusterNetworkResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getGeneration();
  if (f !== 0) {
    writer.writeUint64(
      1,
      f
    );
  }
};


/**
 * optional uint64 generation = 1;
 * @return {number}
 */
proto.cluster_controller.UpdateClusterNetworkResponse.prototype.getGeneration = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.UpdateClusterNetworkResponse} returns this
 */
proto.cluster_controller.UpdateClusterNetworkResponse.prototype.setGeneration = function(value) {
  return jspb.Message.setProto3IntField(this, 1, value);
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
proto.cluster_controller.ArtifactRef.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.ArtifactRef.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.ArtifactRef} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ArtifactRef.toObject = function(includeInstance, msg) {
  var f, obj = {
kind: jspb.Message.getFieldWithDefault(msg, 1, 0),
name: jspb.Message.getFieldWithDefault(msg, 2, ""),
publisher: jspb.Message.getFieldWithDefault(msg, 3, ""),
version: jspb.Message.getFieldWithDefault(msg, 4, ""),
discoveryId: jspb.Message.getFieldWithDefault(msg, 5, "")
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
 * @return {!proto.cluster_controller.ArtifactRef}
 */
proto.cluster_controller.ArtifactRef.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.ArtifactRef;
  return proto.cluster_controller.ArtifactRef.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.ArtifactRef} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.ArtifactRef}
 */
proto.cluster_controller.ArtifactRef.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.cluster_controller.ArtifactKind} */ (reader.readEnum());
      msg.setKind(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisher(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setVersion(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setDiscoveryId(value);
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
proto.cluster_controller.ArtifactRef.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.ArtifactRef.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.ArtifactRef} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ArtifactRef.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getKind();
  if (f !== 0.0) {
    writer.writeEnum(
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
  f = message.getPublisher();
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
  f = message.getDiscoveryId();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
};


/**
 * optional ArtifactKind kind = 1;
 * @return {!proto.cluster_controller.ArtifactKind}
 */
proto.cluster_controller.ArtifactRef.prototype.getKind = function() {
  return /** @type {!proto.cluster_controller.ArtifactKind} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {!proto.cluster_controller.ArtifactKind} value
 * @return {!proto.cluster_controller.ArtifactRef} returns this
 */
proto.cluster_controller.ArtifactRef.prototype.setKind = function(value) {
  return jspb.Message.setProto3EnumField(this, 1, value);
};


/**
 * optional string name = 2;
 * @return {string}
 */
proto.cluster_controller.ArtifactRef.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ArtifactRef} returns this
 */
proto.cluster_controller.ArtifactRef.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string publisher = 3;
 * @return {string}
 */
proto.cluster_controller.ArtifactRef.prototype.getPublisher = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ArtifactRef} returns this
 */
proto.cluster_controller.ArtifactRef.prototype.setPublisher = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string version = 4;
 * @return {string}
 */
proto.cluster_controller.ArtifactRef.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ArtifactRef} returns this
 */
proto.cluster_controller.ArtifactRef.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string discovery_id = 5;
 * @return {string}
 */
proto.cluster_controller.ArtifactRef.prototype.getDiscoveryId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ArtifactRef} returns this
 */
proto.cluster_controller.ArtifactRef.prototype.setDiscoveryId = function(value) {
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
proto.cluster_controller.UnitAction.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.UnitAction.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.UnitAction} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.UnitAction.toObject = function(includeInstance, msg) {
  var f, obj = {
unitName: jspb.Message.getFieldWithDefault(msg, 1, ""),
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
 * @return {!proto.cluster_controller.UnitAction}
 */
proto.cluster_controller.UnitAction.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.UnitAction;
  return proto.cluster_controller.UnitAction.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.UnitAction} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.UnitAction}
 */
proto.cluster_controller.UnitAction.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUnitName(value);
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
proto.cluster_controller.UnitAction.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.UnitAction.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.UnitAction} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.UnitAction.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUnitName();
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
 * optional string unit_name = 1;
 * @return {string}
 */
proto.cluster_controller.UnitAction.prototype.getUnitName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.UnitAction} returns this
 */
proto.cluster_controller.UnitAction.prototype.setUnitName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string action = 2;
 * @return {string}
 */
proto.cluster_controller.UnitAction.prototype.getAction = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.UnitAction} returns this
 */
proto.cluster_controller.UnitAction.prototype.setAction = function(value) {
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
proto.cluster_controller.UpgradeGlobularRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.UpgradeGlobularRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.UpgradeGlobularRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.UpgradeGlobularRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
platform: jspb.Message.getFieldWithDefault(msg, 2, ""),
artifact: msg.getArtifact_asB64(),
sha256: jspb.Message.getFieldWithDefault(msg, 4, ""),
targetPath: jspb.Message.getFieldWithDefault(msg, 5, ""),
probePort: jspb.Message.getFieldWithDefault(msg, 6, 0)
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
 * @return {!proto.cluster_controller.UpgradeGlobularRequest}
 */
proto.cluster_controller.UpgradeGlobularRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.UpgradeGlobularRequest;
  return proto.cluster_controller.UpgradeGlobularRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.UpgradeGlobularRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.UpgradeGlobularRequest}
 */
proto.cluster_controller.UpgradeGlobularRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setPlatform(value);
      break;
    case 3:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setArtifact(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setSha256(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setTargetPath(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setProbePort(value);
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
proto.cluster_controller.UpgradeGlobularRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.UpgradeGlobularRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.UpgradeGlobularRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.UpgradeGlobularRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getPlatform();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getArtifact_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      3,
      f
    );
  }
  f = message.getSha256();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getTargetPath();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getProbePort();
  if (f !== 0) {
    writer.writeUint32(
      6,
      f
    );
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.cluster_controller.UpgradeGlobularRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.UpgradeGlobularRequest} returns this
 */
proto.cluster_controller.UpgradeGlobularRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string platform = 2;
 * @return {string}
 */
proto.cluster_controller.UpgradeGlobularRequest.prototype.getPlatform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.UpgradeGlobularRequest} returns this
 */
proto.cluster_controller.UpgradeGlobularRequest.prototype.setPlatform = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional bytes artifact = 3;
 * @return {string}
 */
proto.cluster_controller.UpgradeGlobularRequest.prototype.getArtifact = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * optional bytes artifact = 3;
 * This is a type-conversion wrapper around `getArtifact()`
 * @return {string}
 */
proto.cluster_controller.UpgradeGlobularRequest.prototype.getArtifact_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getArtifact()));
};


/**
 * optional bytes artifact = 3;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getArtifact()`
 * @return {!Uint8Array}
 */
proto.cluster_controller.UpgradeGlobularRequest.prototype.getArtifact_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getArtifact()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.cluster_controller.UpgradeGlobularRequest} returns this
 */
proto.cluster_controller.UpgradeGlobularRequest.prototype.setArtifact = function(value) {
  return jspb.Message.setProto3BytesField(this, 3, value);
};


/**
 * optional string sha256 = 4;
 * @return {string}
 */
proto.cluster_controller.UpgradeGlobularRequest.prototype.getSha256 = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.UpgradeGlobularRequest} returns this
 */
proto.cluster_controller.UpgradeGlobularRequest.prototype.setSha256 = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string target_path = 5;
 * @return {string}
 */
proto.cluster_controller.UpgradeGlobularRequest.prototype.getTargetPath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.UpgradeGlobularRequest} returns this
 */
proto.cluster_controller.UpgradeGlobularRequest.prototype.setTargetPath = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional uint32 probe_port = 6;
 * @return {number}
 */
proto.cluster_controller.UpgradeGlobularRequest.prototype.getProbePort = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.UpgradeGlobularRequest} returns this
 */
proto.cluster_controller.UpgradeGlobularRequest.prototype.setProbePort = function(value) {
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
proto.cluster_controller.UpgradeGlobularResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.UpgradeGlobularResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.UpgradeGlobularResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.UpgradeGlobularResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
upgradeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
terminalState: jspb.Message.getFieldWithDefault(msg, 2, ""),
errorMessage: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.cluster_controller.UpgradeGlobularResponse}
 */
proto.cluster_controller.UpgradeGlobularResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.UpgradeGlobularResponse;
  return proto.cluster_controller.UpgradeGlobularResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.UpgradeGlobularResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.UpgradeGlobularResponse}
 */
proto.cluster_controller.UpgradeGlobularResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUpgradeId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setTerminalState(value);
      break;
    case 3:
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
proto.cluster_controller.UpgradeGlobularResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.UpgradeGlobularResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.UpgradeGlobularResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.UpgradeGlobularResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUpgradeId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getTerminalState();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getErrorMessage();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string upgrade_id = 1;
 * @return {string}
 */
proto.cluster_controller.UpgradeGlobularResponse.prototype.getUpgradeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.UpgradeGlobularResponse} returns this
 */
proto.cluster_controller.UpgradeGlobularResponse.prototype.setUpgradeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string terminal_state = 2;
 * @return {string}
 */
proto.cluster_controller.UpgradeGlobularResponse.prototype.getTerminalState = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.UpgradeGlobularResponse} returns this
 */
proto.cluster_controller.UpgradeGlobularResponse.prototype.setTerminalState = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string error_message = 3;
 * @return {string}
 */
proto.cluster_controller.UpgradeGlobularResponse.prototype.getErrorMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.UpgradeGlobularResponse} returns this
 */
proto.cluster_controller.UpgradeGlobularResponse.prototype.setErrorMessage = function(value) {
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
proto.cluster_controller.StartApplyRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.StartApplyRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.StartApplyRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.StartApplyRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.cluster_controller.StartApplyRequest}
 */
proto.cluster_controller.StartApplyRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.StartApplyRequest;
  return proto.cluster_controller.StartApplyRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.StartApplyRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.StartApplyRequest}
 */
proto.cluster_controller.StartApplyRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.cluster_controller.StartApplyRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.StartApplyRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.StartApplyRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.StartApplyRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.cluster_controller.StartApplyRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.StartApplyRequest} returns this
 */
proto.cluster_controller.StartApplyRequest.prototype.setNodeId = function(value) {
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
proto.cluster_controller.StartApplyResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.StartApplyResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.StartApplyResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.StartApplyResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.cluster_controller.StartApplyResponse}
 */
proto.cluster_controller.StartApplyResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.StartApplyResponse;
  return proto.cluster_controller.StartApplyResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.StartApplyResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.StartApplyResponse}
 */
proto.cluster_controller.StartApplyResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.cluster_controller.StartApplyResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.StartApplyResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.StartApplyResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.StartApplyResponse.serializeBinaryToWriter = function(message, writer) {
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
proto.cluster_controller.StartApplyResponse.prototype.getOperationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.StartApplyResponse} returns this
 */
proto.cluster_controller.StartApplyResponse.prototype.setOperationId = function(value) {
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
proto.cluster_controller.OperationEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.OperationEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.OperationEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.OperationEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
operationId: jspb.Message.getFieldWithDefault(msg, 1, ""),
nodeId: jspb.Message.getFieldWithDefault(msg, 2, ""),
phase: jspb.Message.getFieldWithDefault(msg, 3, 0),
message: jspb.Message.getFieldWithDefault(msg, 4, ""),
percent: jspb.Message.getFieldWithDefault(msg, 5, 0),
done: jspb.Message.getBooleanFieldWithDefault(msg, 6, false),
error: jspb.Message.getFieldWithDefault(msg, 7, ""),
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
 * @return {!proto.cluster_controller.OperationEvent}
 */
proto.cluster_controller.OperationEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.OperationEvent;
  return proto.cluster_controller.OperationEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.OperationEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.OperationEvent}
 */
proto.cluster_controller.OperationEvent.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setNodeId(value);
      break;
    case 3:
      var value = /** @type {!proto.cluster_controller.OperationPhase} */ (reader.readEnum());
      msg.setPhase(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setPercent(value);
      break;
    case 6:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setDone(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setError(value);
      break;
    case 8:
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
proto.cluster_controller.OperationEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.OperationEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.OperationEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.OperationEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOperationId();
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
  f = message.getPhase();
  if (f !== 0.0) {
    writer.writeEnum(
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
  f = message.getPercent();
  if (f !== 0) {
    writer.writeInt32(
      5,
      f
    );
  }
  f = message.getDone();
  if (f) {
    writer.writeBool(
      6,
      f
    );
  }
  f = message.getError();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getTs();
  if (f != null) {
    writer.writeMessage(
      8,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
};


/**
 * optional string operation_id = 1;
 * @return {string}
 */
proto.cluster_controller.OperationEvent.prototype.getOperationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.OperationEvent} returns this
 */
proto.cluster_controller.OperationEvent.prototype.setOperationId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string node_id = 2;
 * @return {string}
 */
proto.cluster_controller.OperationEvent.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.OperationEvent} returns this
 */
proto.cluster_controller.OperationEvent.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional OperationPhase phase = 3;
 * @return {!proto.cluster_controller.OperationPhase}
 */
proto.cluster_controller.OperationEvent.prototype.getPhase = function() {
  return /** @type {!proto.cluster_controller.OperationPhase} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.cluster_controller.OperationPhase} value
 * @return {!proto.cluster_controller.OperationEvent} returns this
 */
proto.cluster_controller.OperationEvent.prototype.setPhase = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
};


/**
 * optional string message = 4;
 * @return {string}
 */
proto.cluster_controller.OperationEvent.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.OperationEvent} returns this
 */
proto.cluster_controller.OperationEvent.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional int32 percent = 5;
 * @return {number}
 */
proto.cluster_controller.OperationEvent.prototype.getPercent = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.OperationEvent} returns this
 */
proto.cluster_controller.OperationEvent.prototype.setPercent = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional bool done = 6;
 * @return {boolean}
 */
proto.cluster_controller.OperationEvent.prototype.getDone = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 6, false));
};


/**
 * @param {boolean} value
 * @return {!proto.cluster_controller.OperationEvent} returns this
 */
proto.cluster_controller.OperationEvent.prototype.setDone = function(value) {
  return jspb.Message.setProto3BooleanField(this, 6, value);
};


/**
 * optional string error = 7;
 * @return {string}
 */
proto.cluster_controller.OperationEvent.prototype.getError = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.OperationEvent} returns this
 */
proto.cluster_controller.OperationEvent.prototype.setError = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional google.protobuf.Timestamp ts = 8;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.cluster_controller.OperationEvent.prototype.getTs = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 8));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.cluster_controller.OperationEvent} returns this
*/
proto.cluster_controller.OperationEvent.prototype.setTs = function(value) {
  return jspb.Message.setWrapperField(this, 8, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.cluster_controller.OperationEvent} returns this
 */
proto.cluster_controller.OperationEvent.prototype.clearTs = function() {
  return this.setTs(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.cluster_controller.OperationEvent.prototype.hasTs = function() {
  return jspb.Message.getField(this, 8) != null;
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
proto.cluster_controller.CompleteOperationRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.CompleteOperationRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.CompleteOperationRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.CompleteOperationRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
operationId: jspb.Message.getFieldWithDefault(msg, 1, ""),
nodeId: jspb.Message.getFieldWithDefault(msg, 2, ""),
success: jspb.Message.getBooleanFieldWithDefault(msg, 3, false),
message: jspb.Message.getFieldWithDefault(msg, 4, ""),
error: jspb.Message.getFieldWithDefault(msg, 5, ""),
percent: jspb.Message.getFieldWithDefault(msg, 6, 0)
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
 * @return {!proto.cluster_controller.CompleteOperationRequest}
 */
proto.cluster_controller.CompleteOperationRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.CompleteOperationRequest;
  return proto.cluster_controller.CompleteOperationRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.CompleteOperationRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.CompleteOperationRequest}
 */
proto.cluster_controller.CompleteOperationRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setNodeId(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setSuccess(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setError(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setPercent(value);
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
proto.cluster_controller.CompleteOperationRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.CompleteOperationRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.CompleteOperationRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.CompleteOperationRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOperationId();
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
  f = message.getSuccess();
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
  f = message.getError();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getPercent();
  if (f !== 0) {
    writer.writeInt32(
      6,
      f
    );
  }
};


/**
 * optional string operation_id = 1;
 * @return {string}
 */
proto.cluster_controller.CompleteOperationRequest.prototype.getOperationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.CompleteOperationRequest} returns this
 */
proto.cluster_controller.CompleteOperationRequest.prototype.setOperationId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string node_id = 2;
 * @return {string}
 */
proto.cluster_controller.CompleteOperationRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.CompleteOperationRequest} returns this
 */
proto.cluster_controller.CompleteOperationRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional bool success = 3;
 * @return {boolean}
 */
proto.cluster_controller.CompleteOperationRequest.prototype.getSuccess = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.cluster_controller.CompleteOperationRequest} returns this
 */
proto.cluster_controller.CompleteOperationRequest.prototype.setSuccess = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * optional string message = 4;
 * @return {string}
 */
proto.cluster_controller.CompleteOperationRequest.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.CompleteOperationRequest} returns this
 */
proto.cluster_controller.CompleteOperationRequest.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string error = 5;
 * @return {string}
 */
proto.cluster_controller.CompleteOperationRequest.prototype.getError = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.CompleteOperationRequest} returns this
 */
proto.cluster_controller.CompleteOperationRequest.prototype.setError = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional int32 percent = 6;
 * @return {number}
 */
proto.cluster_controller.CompleteOperationRequest.prototype.getPercent = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.CompleteOperationRequest} returns this
 */
proto.cluster_controller.CompleteOperationRequest.prototype.setPercent = function(value) {
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
proto.cluster_controller.CompleteOperationResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.CompleteOperationResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.CompleteOperationResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.CompleteOperationResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
message: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.cluster_controller.CompleteOperationResponse}
 */
proto.cluster_controller.CompleteOperationResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.CompleteOperationResponse;
  return proto.cluster_controller.CompleteOperationResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.CompleteOperationResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.CompleteOperationResponse}
 */
proto.cluster_controller.CompleteOperationResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
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
proto.cluster_controller.CompleteOperationResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.CompleteOperationResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.CompleteOperationResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.CompleteOperationResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMessage();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string message = 1;
 * @return {string}
 */
proto.cluster_controller.CompleteOperationResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.CompleteOperationResponse} returns this
 */
proto.cluster_controller.CompleteOperationResponse.prototype.setMessage = function(value) {
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
proto.cluster_controller.NodeUnitStatus.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.NodeUnitStatus.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.NodeUnitStatus} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.NodeUnitStatus.toObject = function(includeInstance, msg) {
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
 * @return {!proto.cluster_controller.NodeUnitStatus}
 */
proto.cluster_controller.NodeUnitStatus.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.NodeUnitStatus;
  return proto.cluster_controller.NodeUnitStatus.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.NodeUnitStatus} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.NodeUnitStatus}
 */
proto.cluster_controller.NodeUnitStatus.deserializeBinaryFromReader = function(msg, reader) {
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
proto.cluster_controller.NodeUnitStatus.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.NodeUnitStatus.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.NodeUnitStatus} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.NodeUnitStatus.serializeBinaryToWriter = function(message, writer) {
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
proto.cluster_controller.NodeUnitStatus.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeUnitStatus} returns this
 */
proto.cluster_controller.NodeUnitStatus.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string state = 2;
 * @return {string}
 */
proto.cluster_controller.NodeUnitStatus.prototype.getState = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeUnitStatus} returns this
 */
proto.cluster_controller.NodeUnitStatus.prototype.setState = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string details = 3;
 * @return {string}
 */
proto.cluster_controller.NodeUnitStatus.prototype.getDetails = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeUnitStatus} returns this
 */
proto.cluster_controller.NodeUnitStatus.prototype.setDetails = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.NodeStatus.repeatedFields_ = [3,4,10];



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
proto.cluster_controller.NodeStatus.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.NodeStatus.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.NodeStatus} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.NodeStatus.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
identity: (f = msg.getIdentity()) && proto.cluster_controller.NodeIdentity.toObject(includeInstance, f),
ipsList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f,
unitsList: jspb.Message.toObjectList(msg.getUnitsList(),
    proto.cluster_controller.NodeUnitStatus.toObject, includeInstance),
lastError: jspb.Message.getFieldWithDefault(msg, 5, ""),
reportedAt: (f = msg.getReportedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
agentEndpoint: jspb.Message.getFieldWithDefault(msg, 7, ""),
appliedServicesHash: jspb.Message.getFieldWithDefault(msg, 8, ""),
installedVersionsMap: (f = msg.getInstalledVersionsMap()) ? f.toObject(includeInstance, undefined) : [],
installedUnitFilesList: (f = jspb.Message.getRepeatedField(msg, 10)) == null ? undefined : f,
inventoryComplete: jspb.Message.getBooleanFieldWithDefault(msg, 11, false),
capabilities: (f = msg.getCapabilities()) && proto.cluster_controller.NodeCapabilities.toObject(includeInstance, f)
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
 * @return {!proto.cluster_controller.NodeStatus}
 */
proto.cluster_controller.NodeStatus.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.NodeStatus;
  return proto.cluster_controller.NodeStatus.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.NodeStatus} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.NodeStatus}
 */
proto.cluster_controller.NodeStatus.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = new proto.cluster_controller.NodeIdentity;
      reader.readMessage(value,proto.cluster_controller.NodeIdentity.deserializeBinaryFromReader);
      msg.setIdentity(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.addIps(value);
      break;
    case 4:
      var value = new proto.cluster_controller.NodeUnitStatus;
      reader.readMessage(value,proto.cluster_controller.NodeUnitStatus.deserializeBinaryFromReader);
      msg.addUnits(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setLastError(value);
      break;
    case 6:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setReportedAt(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setAgentEndpoint(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setAppliedServicesHash(value);
      break;
    case 9:
      var value = msg.getInstalledVersionsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.addInstalledUnitFiles(value);
      break;
    case 11:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setInventoryComplete(value);
      break;
    case 12:
      var value = new proto.cluster_controller.NodeCapabilities;
      reader.readMessage(value,proto.cluster_controller.NodeCapabilities.deserializeBinaryFromReader);
      msg.setCapabilities(value);
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
proto.cluster_controller.NodeStatus.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.NodeStatus.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.NodeStatus} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.NodeStatus.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIdentity();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.cluster_controller.NodeIdentity.serializeBinaryToWriter
    );
  }
  f = message.getIpsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      3,
      f
    );
  }
  f = message.getUnitsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      4,
      f,
      proto.cluster_controller.NodeUnitStatus.serializeBinaryToWriter
    );
  }
  f = message.getLastError();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getReportedAt();
  if (f != null) {
    writer.writeMessage(
      6,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getAgentEndpoint();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getAppliedServicesHash();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
  f = message.getInstalledVersionsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(9, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
  f = message.getInstalledUnitFilesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      10,
      f
    );
  }
  f = message.getInventoryComplete();
  if (f) {
    writer.writeBool(
      11,
      f
    );
  }
  f = message.getCapabilities();
  if (f != null) {
    writer.writeMessage(
      12,
      f,
      proto.cluster_controller.NodeCapabilities.serializeBinaryToWriter
    );
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.cluster_controller.NodeStatus.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeStatus} returns this
 */
proto.cluster_controller.NodeStatus.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional NodeIdentity identity = 2;
 * @return {?proto.cluster_controller.NodeIdentity}
 */
proto.cluster_controller.NodeStatus.prototype.getIdentity = function() {
  return /** @type{?proto.cluster_controller.NodeIdentity} */ (
    jspb.Message.getWrapperField(this, proto.cluster_controller.NodeIdentity, 2));
};


/**
 * @param {?proto.cluster_controller.NodeIdentity|undefined} value
 * @return {!proto.cluster_controller.NodeStatus} returns this
*/
proto.cluster_controller.NodeStatus.prototype.setIdentity = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.cluster_controller.NodeStatus} returns this
 */
proto.cluster_controller.NodeStatus.prototype.clearIdentity = function() {
  return this.setIdentity(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.cluster_controller.NodeStatus.prototype.hasIdentity = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * repeated string ips = 3;
 * @return {!Array<string>}
 */
proto.cluster_controller.NodeStatus.prototype.getIpsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.NodeStatus} returns this
 */
proto.cluster_controller.NodeStatus.prototype.setIpsList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.NodeStatus} returns this
 */
proto.cluster_controller.NodeStatus.prototype.addIps = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.NodeStatus} returns this
 */
proto.cluster_controller.NodeStatus.prototype.clearIpsList = function() {
  return this.setIpsList([]);
};


/**
 * repeated NodeUnitStatus units = 4;
 * @return {!Array<!proto.cluster_controller.NodeUnitStatus>}
 */
proto.cluster_controller.NodeStatus.prototype.getUnitsList = function() {
  return /** @type{!Array<!proto.cluster_controller.NodeUnitStatus>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.cluster_controller.NodeUnitStatus, 4));
};


/**
 * @param {!Array<!proto.cluster_controller.NodeUnitStatus>} value
 * @return {!proto.cluster_controller.NodeStatus} returns this
*/
proto.cluster_controller.NodeStatus.prototype.setUnitsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 4, value);
};


/**
 * @param {!proto.cluster_controller.NodeUnitStatus=} opt_value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.NodeUnitStatus}
 */
proto.cluster_controller.NodeStatus.prototype.addUnits = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 4, opt_value, proto.cluster_controller.NodeUnitStatus, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.NodeStatus} returns this
 */
proto.cluster_controller.NodeStatus.prototype.clearUnitsList = function() {
  return this.setUnitsList([]);
};


/**
 * optional string last_error = 5;
 * @return {string}
 */
proto.cluster_controller.NodeStatus.prototype.getLastError = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeStatus} returns this
 */
proto.cluster_controller.NodeStatus.prototype.setLastError = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional google.protobuf.Timestamp reported_at = 6;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.cluster_controller.NodeStatus.prototype.getReportedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 6));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.cluster_controller.NodeStatus} returns this
*/
proto.cluster_controller.NodeStatus.prototype.setReportedAt = function(value) {
  return jspb.Message.setWrapperField(this, 6, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.cluster_controller.NodeStatus} returns this
 */
proto.cluster_controller.NodeStatus.prototype.clearReportedAt = function() {
  return this.setReportedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.cluster_controller.NodeStatus.prototype.hasReportedAt = function() {
  return jspb.Message.getField(this, 6) != null;
};


/**
 * optional string agent_endpoint = 7;
 * @return {string}
 */
proto.cluster_controller.NodeStatus.prototype.getAgentEndpoint = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeStatus} returns this
 */
proto.cluster_controller.NodeStatus.prototype.setAgentEndpoint = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional string applied_services_hash = 8;
 * @return {string}
 */
proto.cluster_controller.NodeStatus.prototype.getAppliedServicesHash = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeStatus} returns this
 */
proto.cluster_controller.NodeStatus.prototype.setAppliedServicesHash = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};


/**
 * map<string, string> installed_versions = 9;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.cluster_controller.NodeStatus.prototype.getInstalledVersionsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 9, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.cluster_controller.NodeStatus} returns this
 */
proto.cluster_controller.NodeStatus.prototype.clearInstalledVersionsMap = function() {
  this.getInstalledVersionsMap().clear();
  return this;
};


/**
 * repeated string installed_unit_files = 10;
 * @return {!Array<string>}
 */
proto.cluster_controller.NodeStatus.prototype.getInstalledUnitFilesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 10));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.NodeStatus} returns this
 */
proto.cluster_controller.NodeStatus.prototype.setInstalledUnitFilesList = function(value) {
  return jspb.Message.setField(this, 10, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.NodeStatus} returns this
 */
proto.cluster_controller.NodeStatus.prototype.addInstalledUnitFiles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 10, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.NodeStatus} returns this
 */
proto.cluster_controller.NodeStatus.prototype.clearInstalledUnitFilesList = function() {
  return this.setInstalledUnitFilesList([]);
};


/**
 * optional bool inventory_complete = 11;
 * @return {boolean}
 */
proto.cluster_controller.NodeStatus.prototype.getInventoryComplete = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 11, false));
};


/**
 * @param {boolean} value
 * @return {!proto.cluster_controller.NodeStatus} returns this
 */
proto.cluster_controller.NodeStatus.prototype.setInventoryComplete = function(value) {
  return jspb.Message.setProto3BooleanField(this, 11, value);
};


/**
 * optional NodeCapabilities capabilities = 12;
 * @return {?proto.cluster_controller.NodeCapabilities}
 */
proto.cluster_controller.NodeStatus.prototype.getCapabilities = function() {
  return /** @type{?proto.cluster_controller.NodeCapabilities} */ (
    jspb.Message.getWrapperField(this, proto.cluster_controller.NodeCapabilities, 12));
};


/**
 * @param {?proto.cluster_controller.NodeCapabilities|undefined} value
 * @return {!proto.cluster_controller.NodeStatus} returns this
*/
proto.cluster_controller.NodeStatus.prototype.setCapabilities = function(value) {
  return jspb.Message.setWrapperField(this, 12, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.cluster_controller.NodeStatus} returns this
 */
proto.cluster_controller.NodeStatus.prototype.clearCapabilities = function() {
  return this.setCapabilities(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.cluster_controller.NodeStatus.prototype.hasCapabilities = function() {
  return jspb.Message.getField(this, 12) != null;
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
proto.cluster_controller.ReportNodeStatusRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.ReportNodeStatusRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.ReportNodeStatusRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ReportNodeStatusRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
status: (f = msg.getStatus()) && proto.cluster_controller.NodeStatus.toObject(includeInstance, f)
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
 * @return {!proto.cluster_controller.ReportNodeStatusRequest}
 */
proto.cluster_controller.ReportNodeStatusRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.ReportNodeStatusRequest;
  return proto.cluster_controller.ReportNodeStatusRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.ReportNodeStatusRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.ReportNodeStatusRequest}
 */
proto.cluster_controller.ReportNodeStatusRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.cluster_controller.NodeStatus;
      reader.readMessage(value,proto.cluster_controller.NodeStatus.deserializeBinaryFromReader);
      msg.setStatus(value);
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
proto.cluster_controller.ReportNodeStatusRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.ReportNodeStatusRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.ReportNodeStatusRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ReportNodeStatusRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.cluster_controller.NodeStatus.serializeBinaryToWriter
    );
  }
};


/**
 * optional NodeStatus status = 1;
 * @return {?proto.cluster_controller.NodeStatus}
 */
proto.cluster_controller.ReportNodeStatusRequest.prototype.getStatus = function() {
  return /** @type{?proto.cluster_controller.NodeStatus} */ (
    jspb.Message.getWrapperField(this, proto.cluster_controller.NodeStatus, 1));
};


/**
 * @param {?proto.cluster_controller.NodeStatus|undefined} value
 * @return {!proto.cluster_controller.ReportNodeStatusRequest} returns this
*/
proto.cluster_controller.ReportNodeStatusRequest.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.cluster_controller.ReportNodeStatusRequest} returns this
 */
proto.cluster_controller.ReportNodeStatusRequest.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.cluster_controller.ReportNodeStatusRequest.prototype.hasStatus = function() {
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
proto.cluster_controller.ReportNodeStatusResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.ReportNodeStatusResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.ReportNodeStatusResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ReportNodeStatusResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
message: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.cluster_controller.ReportNodeStatusResponse}
 */
proto.cluster_controller.ReportNodeStatusResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.ReportNodeStatusResponse;
  return proto.cluster_controller.ReportNodeStatusResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.ReportNodeStatusResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.ReportNodeStatusResponse}
 */
proto.cluster_controller.ReportNodeStatusResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
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
proto.cluster_controller.ReportNodeStatusResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.ReportNodeStatusResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.ReportNodeStatusResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ReportNodeStatusResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMessage();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string message = 1;
 * @return {string}
 */
proto.cluster_controller.ReportNodeStatusResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ReportNodeStatusResponse} returns this
 */
proto.cluster_controller.ReportNodeStatusResponse.prototype.setMessage = function(value) {
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
proto.cluster_controller.WatchOperationsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.WatchOperationsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.WatchOperationsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.WatchOperationsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
operationId: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.cluster_controller.WatchOperationsRequest}
 */
proto.cluster_controller.WatchOperationsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.WatchOperationsRequest;
  return proto.cluster_controller.WatchOperationsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.WatchOperationsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.WatchOperationsRequest}
 */
proto.cluster_controller.WatchOperationsRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.cluster_controller.WatchOperationsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.WatchOperationsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.WatchOperationsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.WatchOperationsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOperationId();
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
proto.cluster_controller.WatchOperationsRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.WatchOperationsRequest} returns this
 */
proto.cluster_controller.WatchOperationsRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string operation_id = 2;
 * @return {string}
 */
proto.cluster_controller.WatchOperationsRequest.prototype.getOperationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.WatchOperationsRequest} returns this
 */
proto.cluster_controller.WatchOperationsRequest.prototype.setOperationId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.DesiredNetwork.repeatedFields_ = [7];



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
proto.cluster_controller.DesiredNetwork.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.DesiredNetwork.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.DesiredNetwork} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.DesiredNetwork.toObject = function(includeInstance, msg) {
  var f, obj = {
domain: jspb.Message.getFieldWithDefault(msg, 1, ""),
protocol: jspb.Message.getFieldWithDefault(msg, 2, ""),
portHttp: jspb.Message.getFieldWithDefault(msg, 3, 0),
portHttps: jspb.Message.getFieldWithDefault(msg, 4, 0),
acmeEnabled: jspb.Message.getBooleanFieldWithDefault(msg, 5, false),
adminEmail: jspb.Message.getFieldWithDefault(msg, 6, ""),
alternateDomainsList: (f = jspb.Message.getRepeatedField(msg, 7)) == null ? undefined : f
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
 * @return {!proto.cluster_controller.DesiredNetwork}
 */
proto.cluster_controller.DesiredNetwork.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.DesiredNetwork;
  return proto.cluster_controller.DesiredNetwork.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.DesiredNetwork} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.DesiredNetwork}
 */
proto.cluster_controller.DesiredNetwork.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setDomain(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setProtocol(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setPortHttp(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setPortHttps(value);
      break;
    case 5:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setAcmeEnabled(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setAdminEmail(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.addAlternateDomains(value);
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
proto.cluster_controller.DesiredNetwork.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.DesiredNetwork.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.DesiredNetwork} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.DesiredNetwork.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDomain();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getProtocol();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getPortHttp();
  if (f !== 0) {
    writer.writeUint32(
      3,
      f
    );
  }
  f = message.getPortHttps();
  if (f !== 0) {
    writer.writeUint32(
      4,
      f
    );
  }
  f = message.getAcmeEnabled();
  if (f) {
    writer.writeBool(
      5,
      f
    );
  }
  f = message.getAdminEmail();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getAlternateDomainsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      7,
      f
    );
  }
};


/**
 * optional string domain = 1;
 * @return {string}
 */
proto.cluster_controller.DesiredNetwork.prototype.getDomain = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.DesiredNetwork} returns this
 */
proto.cluster_controller.DesiredNetwork.prototype.setDomain = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string protocol = 2;
 * @return {string}
 */
proto.cluster_controller.DesiredNetwork.prototype.getProtocol = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.DesiredNetwork} returns this
 */
proto.cluster_controller.DesiredNetwork.prototype.setProtocol = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional uint32 port_http = 3;
 * @return {number}
 */
proto.cluster_controller.DesiredNetwork.prototype.getPortHttp = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.DesiredNetwork} returns this
 */
proto.cluster_controller.DesiredNetwork.prototype.setPortHttp = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional uint32 port_https = 4;
 * @return {number}
 */
proto.cluster_controller.DesiredNetwork.prototype.getPortHttps = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.DesiredNetwork} returns this
 */
proto.cluster_controller.DesiredNetwork.prototype.setPortHttps = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional bool acme_enabled = 5;
 * @return {boolean}
 */
proto.cluster_controller.DesiredNetwork.prototype.getAcmeEnabled = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 5, false));
};


/**
 * @param {boolean} value
 * @return {!proto.cluster_controller.DesiredNetwork} returns this
 */
proto.cluster_controller.DesiredNetwork.prototype.setAcmeEnabled = function(value) {
  return jspb.Message.setProto3BooleanField(this, 5, value);
};


/**
 * optional string admin_email = 6;
 * @return {string}
 */
proto.cluster_controller.DesiredNetwork.prototype.getAdminEmail = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.DesiredNetwork} returns this
 */
proto.cluster_controller.DesiredNetwork.prototype.setAdminEmail = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * repeated string alternate_domains = 7;
 * @return {!Array<string>}
 */
proto.cluster_controller.DesiredNetwork.prototype.getAlternateDomainsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 7));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.DesiredNetwork} returns this
 */
proto.cluster_controller.DesiredNetwork.prototype.setAlternateDomainsList = function(value) {
  return jspb.Message.setField(this, 7, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.DesiredNetwork} returns this
 */
proto.cluster_controller.DesiredNetwork.prototype.addAlternateDomains = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 7, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.DesiredNetwork} returns this
 */
proto.cluster_controller.DesiredNetwork.prototype.clearAlternateDomainsList = function() {
  return this.setAlternateDomainsList([]);
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
proto.cluster_controller.GetClusterHealthV1Request.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.GetClusterHealthV1Request.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.GetClusterHealthV1Request} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.GetClusterHealthV1Request.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterId: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.cluster_controller.GetClusterHealthV1Request}
 */
proto.cluster_controller.GetClusterHealthV1Request.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.GetClusterHealthV1Request;
  return proto.cluster_controller.GetClusterHealthV1Request.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.GetClusterHealthV1Request} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.GetClusterHealthV1Request}
 */
proto.cluster_controller.GetClusterHealthV1Request.deserializeBinaryFromReader = function(msg, reader) {
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
proto.cluster_controller.GetClusterHealthV1Request.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.GetClusterHealthV1Request.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.GetClusterHealthV1Request} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.GetClusterHealthV1Request.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string cluster_id = 1;
 * @return {string}
 */
proto.cluster_controller.GetClusterHealthV1Request.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.GetClusterHealthV1Request} returns this
 */
proto.cluster_controller.GetClusterHealthV1Request.prototype.setClusterId = function(value) {
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
proto.cluster_controller.NodeHealth.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.NodeHealth.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.NodeHealth} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.NodeHealth.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
desiredNetworkHash: jspb.Message.getFieldWithDefault(msg, 2, ""),
appliedNetworkHash: jspb.Message.getFieldWithDefault(msg, 3, ""),
desiredServicesHash: jspb.Message.getFieldWithDefault(msg, 4, ""),
appliedServicesHash: jspb.Message.getFieldWithDefault(msg, 5, ""),
lastError: jspb.Message.getFieldWithDefault(msg, 9, ""),
canApplyPrivileged: jspb.Message.getBooleanFieldWithDefault(msg, 10, false),
installedVersionsMap: (f = msg.getInstalledVersionsMap()) ? f.toObject(includeInstance, undefined) : []
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
 * @return {!proto.cluster_controller.NodeHealth}
 */
proto.cluster_controller.NodeHealth.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.NodeHealth;
  return proto.cluster_controller.NodeHealth.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.NodeHealth} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.NodeHealth}
 */
proto.cluster_controller.NodeHealth.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setDesiredNetworkHash(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setAppliedNetworkHash(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setDesiredServicesHash(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setAppliedServicesHash(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setLastError(value);
      break;
    case 10:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setCanApplyPrivileged(value);
      break;
    case 11:
      var value = msg.getInstalledVersionsMap();
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
proto.cluster_controller.NodeHealth.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.NodeHealth.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.NodeHealth} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.NodeHealth.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getDesiredNetworkHash();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getAppliedNetworkHash();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getDesiredServicesHash();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getAppliedServicesHash();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getLastError();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
  f = message.getCanApplyPrivileged();
  if (f) {
    writer.writeBool(
      10,
      f
    );
  }
  f = message.getInstalledVersionsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(11, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.cluster_controller.NodeHealth.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeHealth} returns this
 */
proto.cluster_controller.NodeHealth.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string desired_network_hash = 2;
 * @return {string}
 */
proto.cluster_controller.NodeHealth.prototype.getDesiredNetworkHash = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeHealth} returns this
 */
proto.cluster_controller.NodeHealth.prototype.setDesiredNetworkHash = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string applied_network_hash = 3;
 * @return {string}
 */
proto.cluster_controller.NodeHealth.prototype.getAppliedNetworkHash = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeHealth} returns this
 */
proto.cluster_controller.NodeHealth.prototype.setAppliedNetworkHash = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string desired_services_hash = 4;
 * @return {string}
 */
proto.cluster_controller.NodeHealth.prototype.getDesiredServicesHash = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeHealth} returns this
 */
proto.cluster_controller.NodeHealth.prototype.setDesiredServicesHash = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string applied_services_hash = 5;
 * @return {string}
 */
proto.cluster_controller.NodeHealth.prototype.getAppliedServicesHash = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeHealth} returns this
 */
proto.cluster_controller.NodeHealth.prototype.setAppliedServicesHash = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string last_error = 9;
 * @return {string}
 */
proto.cluster_controller.NodeHealth.prototype.getLastError = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeHealth} returns this
 */
proto.cluster_controller.NodeHealth.prototype.setLastError = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};


/**
 * optional bool can_apply_privileged = 10;
 * @return {boolean}
 */
proto.cluster_controller.NodeHealth.prototype.getCanApplyPrivileged = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 10, false));
};


/**
 * @param {boolean} value
 * @return {!proto.cluster_controller.NodeHealth} returns this
 */
proto.cluster_controller.NodeHealth.prototype.setCanApplyPrivileged = function(value) {
  return jspb.Message.setProto3BooleanField(this, 10, value);
};


/**
 * map<string, string> installed_versions = 11;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.cluster_controller.NodeHealth.prototype.getInstalledVersionsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 11, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.cluster_controller.NodeHealth} returns this
 */
proto.cluster_controller.NodeHealth.prototype.clearInstalledVersionsMap = function() {
  this.getInstalledVersionsMap().clear();
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
proto.cluster_controller.ServiceSummary.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.ServiceSummary.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.ServiceSummary} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ServiceSummary.toObject = function(includeInstance, msg) {
  var f, obj = {
serviceName: jspb.Message.getFieldWithDefault(msg, 1, ""),
desiredVersion: jspb.Message.getFieldWithDefault(msg, 2, ""),
nodesAtDesired: jspb.Message.getFieldWithDefault(msg, 3, 0),
nodesTotal: jspb.Message.getFieldWithDefault(msg, 4, 0),
upgrading: jspb.Message.getFieldWithDefault(msg, 5, 0),
kind: jspb.Message.getFieldWithDefault(msg, 6, "")
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
 * @return {!proto.cluster_controller.ServiceSummary}
 */
proto.cluster_controller.ServiceSummary.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.ServiceSummary;
  return proto.cluster_controller.ServiceSummary.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.ServiceSummary} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.ServiceSummary}
 */
proto.cluster_controller.ServiceSummary.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setDesiredVersion(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setNodesAtDesired(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setNodesTotal(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setUpgrading(value);
      break;
    case 6:
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
proto.cluster_controller.ServiceSummary.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.ServiceSummary.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.ServiceSummary} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ServiceSummary.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getServiceName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getDesiredVersion();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getNodesAtDesired();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
  f = message.getNodesTotal();
  if (f !== 0) {
    writer.writeInt32(
      4,
      f
    );
  }
  f = message.getUpgrading();
  if (f !== 0) {
    writer.writeInt32(
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
};


/**
 * optional string service_name = 1;
 * @return {string}
 */
proto.cluster_controller.ServiceSummary.prototype.getServiceName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ServiceSummary} returns this
 */
proto.cluster_controller.ServiceSummary.prototype.setServiceName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string desired_version = 2;
 * @return {string}
 */
proto.cluster_controller.ServiceSummary.prototype.getDesiredVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ServiceSummary} returns this
 */
proto.cluster_controller.ServiceSummary.prototype.setDesiredVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int32 nodes_at_desired = 3;
 * @return {number}
 */
proto.cluster_controller.ServiceSummary.prototype.getNodesAtDesired = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.ServiceSummary} returns this
 */
proto.cluster_controller.ServiceSummary.prototype.setNodesAtDesired = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional int32 nodes_total = 4;
 * @return {number}
 */
proto.cluster_controller.ServiceSummary.prototype.getNodesTotal = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.ServiceSummary} returns this
 */
proto.cluster_controller.ServiceSummary.prototype.setNodesTotal = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional int32 upgrading = 5;
 * @return {number}
 */
proto.cluster_controller.ServiceSummary.prototype.getUpgrading = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.ServiceSummary} returns this
 */
proto.cluster_controller.ServiceSummary.prototype.setUpgrading = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional string kind = 6;
 * @return {string}
 */
proto.cluster_controller.ServiceSummary.prototype.getKind = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ServiceSummary} returns this
 */
proto.cluster_controller.ServiceSummary.prototype.setKind = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.GetClusterHealthV1Response.repeatedFields_ = [1,2];



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
proto.cluster_controller.GetClusterHealthV1Response.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.GetClusterHealthV1Response.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.GetClusterHealthV1Response} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.GetClusterHealthV1Response.toObject = function(includeInstance, msg) {
  var f, obj = {
nodesList: jspb.Message.toObjectList(msg.getNodesList(),
    proto.cluster_controller.NodeHealth.toObject, includeInstance),
servicesList: jspb.Message.toObjectList(msg.getServicesList(),
    proto.cluster_controller.ServiceSummary.toObject, includeInstance)
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
 * @return {!proto.cluster_controller.GetClusterHealthV1Response}
 */
proto.cluster_controller.GetClusterHealthV1Response.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.GetClusterHealthV1Response;
  return proto.cluster_controller.GetClusterHealthV1Response.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.GetClusterHealthV1Response} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.GetClusterHealthV1Response}
 */
proto.cluster_controller.GetClusterHealthV1Response.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.cluster_controller.NodeHealth;
      reader.readMessage(value,proto.cluster_controller.NodeHealth.deserializeBinaryFromReader);
      msg.addNodes(value);
      break;
    case 2:
      var value = new proto.cluster_controller.ServiceSummary;
      reader.readMessage(value,proto.cluster_controller.ServiceSummary.deserializeBinaryFromReader);
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
proto.cluster_controller.GetClusterHealthV1Response.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.GetClusterHealthV1Response.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.GetClusterHealthV1Response} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.GetClusterHealthV1Response.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.cluster_controller.NodeHealth.serializeBinaryToWriter
    );
  }
  f = message.getServicesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      2,
      f,
      proto.cluster_controller.ServiceSummary.serializeBinaryToWriter
    );
  }
};


/**
 * repeated NodeHealth nodes = 1;
 * @return {!Array<!proto.cluster_controller.NodeHealth>}
 */
proto.cluster_controller.GetClusterHealthV1Response.prototype.getNodesList = function() {
  return /** @type{!Array<!proto.cluster_controller.NodeHealth>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.cluster_controller.NodeHealth, 1));
};


/**
 * @param {!Array<!proto.cluster_controller.NodeHealth>} value
 * @return {!proto.cluster_controller.GetClusterHealthV1Response} returns this
*/
proto.cluster_controller.GetClusterHealthV1Response.prototype.setNodesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.cluster_controller.NodeHealth=} opt_value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.NodeHealth}
 */
proto.cluster_controller.GetClusterHealthV1Response.prototype.addNodes = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.cluster_controller.NodeHealth, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.GetClusterHealthV1Response} returns this
 */
proto.cluster_controller.GetClusterHealthV1Response.prototype.clearNodesList = function() {
  return this.setNodesList([]);
};


/**
 * repeated ServiceSummary services = 2;
 * @return {!Array<!proto.cluster_controller.ServiceSummary>}
 */
proto.cluster_controller.GetClusterHealthV1Response.prototype.getServicesList = function() {
  return /** @type{!Array<!proto.cluster_controller.ServiceSummary>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.cluster_controller.ServiceSummary, 2));
};


/**
 * @param {!Array<!proto.cluster_controller.ServiceSummary>} value
 * @return {!proto.cluster_controller.GetClusterHealthV1Response} returns this
*/
proto.cluster_controller.GetClusterHealthV1Response.prototype.setServicesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 2, value);
};


/**
 * @param {!proto.cluster_controller.ServiceSummary=} opt_value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.ServiceSummary}
 */
proto.cluster_controller.GetClusterHealthV1Response.prototype.addServices = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 2, opt_value, proto.cluster_controller.ServiceSummary, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.GetClusterHealthV1Response} returns this
 */
proto.cluster_controller.GetClusterHealthV1Response.prototype.clearServicesList = function() {
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
proto.cluster_controller.NodeHealthCheck.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.NodeHealthCheck.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.NodeHealthCheck} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.NodeHealthCheck.toObject = function(includeInstance, msg) {
  var f, obj = {
subsystem: jspb.Message.getFieldWithDefault(msg, 1, ""),
ok: jspb.Message.getBooleanFieldWithDefault(msg, 2, false),
reason: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.cluster_controller.NodeHealthCheck}
 */
proto.cluster_controller.NodeHealthCheck.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.NodeHealthCheck;
  return proto.cluster_controller.NodeHealthCheck.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.NodeHealthCheck} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.NodeHealthCheck}
 */
proto.cluster_controller.NodeHealthCheck.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setSubsystem(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setOk(value);
      break;
    case 3:
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
proto.cluster_controller.NodeHealthCheck.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.NodeHealthCheck.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.NodeHealthCheck} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.NodeHealthCheck.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSubsystem();
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
  f = message.getReason();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string subsystem = 1;
 * @return {string}
 */
proto.cluster_controller.NodeHealthCheck.prototype.getSubsystem = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeHealthCheck} returns this
 */
proto.cluster_controller.NodeHealthCheck.prototype.setSubsystem = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bool ok = 2;
 * @return {boolean}
 */
proto.cluster_controller.NodeHealthCheck.prototype.getOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.cluster_controller.NodeHealthCheck} returns this
 */
proto.cluster_controller.NodeHealthCheck.prototype.setOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * optional string reason = 3;
 * @return {string}
 */
proto.cluster_controller.NodeHealthCheck.prototype.getReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeHealthCheck} returns this
 */
proto.cluster_controller.NodeHealthCheck.prototype.setReason = function(value) {
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
proto.cluster_controller.GetNodeHealthDetailV1Request.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.GetNodeHealthDetailV1Request.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.GetNodeHealthDetailV1Request} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.GetNodeHealthDetailV1Request.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.cluster_controller.GetNodeHealthDetailV1Request}
 */
proto.cluster_controller.GetNodeHealthDetailV1Request.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.GetNodeHealthDetailV1Request;
  return proto.cluster_controller.GetNodeHealthDetailV1Request.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.GetNodeHealthDetailV1Request} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.GetNodeHealthDetailV1Request}
 */
proto.cluster_controller.GetNodeHealthDetailV1Request.deserializeBinaryFromReader = function(msg, reader) {
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
proto.cluster_controller.GetNodeHealthDetailV1Request.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.GetNodeHealthDetailV1Request.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.GetNodeHealthDetailV1Request} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.GetNodeHealthDetailV1Request.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.cluster_controller.GetNodeHealthDetailV1Request.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.GetNodeHealthDetailV1Request} returns this
 */
proto.cluster_controller.GetNodeHealthDetailV1Request.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.repeatedFields_ = [4];



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
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.GetNodeHealthDetailV1Response.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.GetNodeHealthDetailV1Response} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
overallStatus: jspb.Message.getFieldWithDefault(msg, 2, ""),
healthy: jspb.Message.getBooleanFieldWithDefault(msg, 3, false),
checksList: jspb.Message.toObjectList(msg.getChecksList(),
    proto.cluster_controller.NodeHealthCheck.toObject, includeInstance),
lastError: jspb.Message.getFieldWithDefault(msg, 5, ""),
canApplyPrivileged: jspb.Message.getBooleanFieldWithDefault(msg, 6, false),
inventoryComplete: jspb.Message.getBooleanFieldWithDefault(msg, 7, false),
lastSeen: (f = msg.getLastSeen()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
privilegeReason: jspb.Message.getFieldWithDefault(msg, 9, "")
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
 * @return {!proto.cluster_controller.GetNodeHealthDetailV1Response}
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.GetNodeHealthDetailV1Response;
  return proto.cluster_controller.GetNodeHealthDetailV1Response.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.GetNodeHealthDetailV1Response} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.GetNodeHealthDetailV1Response}
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setOverallStatus(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setHealthy(value);
      break;
    case 4:
      var value = new proto.cluster_controller.NodeHealthCheck;
      reader.readMessage(value,proto.cluster_controller.NodeHealthCheck.deserializeBinaryFromReader);
      msg.addChecks(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setLastError(value);
      break;
    case 6:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setCanApplyPrivileged(value);
      break;
    case 7:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setInventoryComplete(value);
      break;
    case 8:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setLastSeen(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setPrivilegeReason(value);
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
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.GetNodeHealthDetailV1Response.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.GetNodeHealthDetailV1Response} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOverallStatus();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getHealthy();
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
      proto.cluster_controller.NodeHealthCheck.serializeBinaryToWriter
    );
  }
  f = message.getLastError();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getCanApplyPrivileged();
  if (f) {
    writer.writeBool(
      6,
      f
    );
  }
  f = message.getInventoryComplete();
  if (f) {
    writer.writeBool(
      7,
      f
    );
  }
  f = message.getLastSeen();
  if (f != null) {
    writer.writeMessage(
      8,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getPrivilegeReason();
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
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.GetNodeHealthDetailV1Response} returns this
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string overall_status = 2;
 * @return {string}
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.getOverallStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.GetNodeHealthDetailV1Response} returns this
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.setOverallStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional bool healthy = 3;
 * @return {boolean}
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.getHealthy = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.cluster_controller.GetNodeHealthDetailV1Response} returns this
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.setHealthy = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * repeated NodeHealthCheck checks = 4;
 * @return {!Array<!proto.cluster_controller.NodeHealthCheck>}
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.getChecksList = function() {
  return /** @type{!Array<!proto.cluster_controller.NodeHealthCheck>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.cluster_controller.NodeHealthCheck, 4));
};


/**
 * @param {!Array<!proto.cluster_controller.NodeHealthCheck>} value
 * @return {!proto.cluster_controller.GetNodeHealthDetailV1Response} returns this
*/
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.setChecksList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 4, value);
};


/**
 * @param {!proto.cluster_controller.NodeHealthCheck=} opt_value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.NodeHealthCheck}
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.addChecks = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 4, opt_value, proto.cluster_controller.NodeHealthCheck, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.GetNodeHealthDetailV1Response} returns this
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.clearChecksList = function() {
  return this.setChecksList([]);
};


/**
 * optional string last_error = 5;
 * @return {string}
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.getLastError = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.GetNodeHealthDetailV1Response} returns this
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.setLastError = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional bool can_apply_privileged = 6;
 * @return {boolean}
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.getCanApplyPrivileged = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 6, false));
};


/**
 * @param {boolean} value
 * @return {!proto.cluster_controller.GetNodeHealthDetailV1Response} returns this
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.setCanApplyPrivileged = function(value) {
  return jspb.Message.setProto3BooleanField(this, 6, value);
};


/**
 * optional bool inventory_complete = 7;
 * @return {boolean}
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.getInventoryComplete = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 7, false));
};


/**
 * @param {boolean} value
 * @return {!proto.cluster_controller.GetNodeHealthDetailV1Response} returns this
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.setInventoryComplete = function(value) {
  return jspb.Message.setProto3BooleanField(this, 7, value);
};


/**
 * optional google.protobuf.Timestamp last_seen = 8;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.getLastSeen = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 8));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.cluster_controller.GetNodeHealthDetailV1Response} returns this
*/
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.setLastSeen = function(value) {
  return jspb.Message.setWrapperField(this, 8, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.cluster_controller.GetNodeHealthDetailV1Response} returns this
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.clearLastSeen = function() {
  return this.setLastSeen(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.hasLastSeen = function() {
  return jspb.Message.getField(this, 8) != null;
};


/**
 * optional string privilege_reason = 9;
 * @return {string}
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.getPrivilegeReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.GetNodeHealthDetailV1Response} returns this
 */
proto.cluster_controller.GetNodeHealthDetailV1Response.prototype.setPrivilegeReason = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.PreviewNodeProfilesRequest.repeatedFields_ = [2];



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
proto.cluster_controller.PreviewNodeProfilesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.PreviewNodeProfilesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.PreviewNodeProfilesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.PreviewNodeProfilesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
profilesList: (f = jspb.Message.getRepeatedField(msg, 2)) == null ? undefined : f
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
 * @return {!proto.cluster_controller.PreviewNodeProfilesRequest}
 */
proto.cluster_controller.PreviewNodeProfilesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.PreviewNodeProfilesRequest;
  return proto.cluster_controller.PreviewNodeProfilesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.PreviewNodeProfilesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.PreviewNodeProfilesRequest}
 */
proto.cluster_controller.PreviewNodeProfilesRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.cluster_controller.PreviewNodeProfilesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.PreviewNodeProfilesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.PreviewNodeProfilesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.PreviewNodeProfilesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getProfilesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      2,
      f
    );
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.cluster_controller.PreviewNodeProfilesRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.PreviewNodeProfilesRequest} returns this
 */
proto.cluster_controller.PreviewNodeProfilesRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * repeated string profiles = 2;
 * @return {!Array<string>}
 */
proto.cluster_controller.PreviewNodeProfilesRequest.prototype.getProfilesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.PreviewNodeProfilesRequest} returns this
 */
proto.cluster_controller.PreviewNodeProfilesRequest.prototype.setProfilesList = function(value) {
  return jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.PreviewNodeProfilesRequest} returns this
 */
proto.cluster_controller.PreviewNodeProfilesRequest.prototype.addProfiles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.PreviewNodeProfilesRequest} returns this
 */
proto.cluster_controller.PreviewNodeProfilesRequest.prototype.clearProfilesList = function() {
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
proto.cluster_controller.ConfigFileDiff.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.ConfigFileDiff.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.ConfigFileDiff} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ConfigFileDiff.toObject = function(includeInstance, msg) {
  var f, obj = {
path: jspb.Message.getFieldWithDefault(msg, 1, ""),
oldHash: jspb.Message.getFieldWithDefault(msg, 2, ""),
newHash: jspb.Message.getFieldWithDefault(msg, 3, ""),
changed: jspb.Message.getBooleanFieldWithDefault(msg, 4, false)
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
 * @return {!proto.cluster_controller.ConfigFileDiff}
 */
proto.cluster_controller.ConfigFileDiff.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.ConfigFileDiff;
  return proto.cluster_controller.ConfigFileDiff.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.ConfigFileDiff} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.ConfigFileDiff}
 */
proto.cluster_controller.ConfigFileDiff.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPath(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setOldHash(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setNewHash(value);
      break;
    case 4:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setChanged(value);
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
proto.cluster_controller.ConfigFileDiff.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.ConfigFileDiff.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.ConfigFileDiff} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ConfigFileDiff.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPath();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOldHash();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getNewHash();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getChanged();
  if (f) {
    writer.writeBool(
      4,
      f
    );
  }
};


/**
 * optional string path = 1;
 * @return {string}
 */
proto.cluster_controller.ConfigFileDiff.prototype.getPath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ConfigFileDiff} returns this
 */
proto.cluster_controller.ConfigFileDiff.prototype.setPath = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string old_hash = 2;
 * @return {string}
 */
proto.cluster_controller.ConfigFileDiff.prototype.getOldHash = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ConfigFileDiff} returns this
 */
proto.cluster_controller.ConfigFileDiff.prototype.setOldHash = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string new_hash = 3;
 * @return {string}
 */
proto.cluster_controller.ConfigFileDiff.prototype.getNewHash = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ConfigFileDiff} returns this
 */
proto.cluster_controller.ConfigFileDiff.prototype.setNewHash = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional bool changed = 4;
 * @return {boolean}
 */
proto.cluster_controller.ConfigFileDiff.prototype.getChanged = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 4, false));
};


/**
 * @param {boolean} value
 * @return {!proto.cluster_controller.ConfigFileDiff} returns this
 */
proto.cluster_controller.ConfigFileDiff.prototype.setChanged = function(value) {
  return jspb.Message.setProto3BooleanField(this, 4, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.AffectedNodeDiff.repeatedFields_ = [2];



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
proto.cluster_controller.AffectedNodeDiff.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.AffectedNodeDiff.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.AffectedNodeDiff} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.AffectedNodeDiff.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
configDiffList: jspb.Message.toObjectList(msg.getConfigDiffList(),
    proto.cluster_controller.ConfigFileDiff.toObject, includeInstance)
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
 * @return {!proto.cluster_controller.AffectedNodeDiff}
 */
proto.cluster_controller.AffectedNodeDiff.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.AffectedNodeDiff;
  return proto.cluster_controller.AffectedNodeDiff.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.AffectedNodeDiff} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.AffectedNodeDiff}
 */
proto.cluster_controller.AffectedNodeDiff.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = new proto.cluster_controller.ConfigFileDiff;
      reader.readMessage(value,proto.cluster_controller.ConfigFileDiff.deserializeBinaryFromReader);
      msg.addConfigDiff(value);
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
proto.cluster_controller.AffectedNodeDiff.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.AffectedNodeDiff.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.AffectedNodeDiff} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.AffectedNodeDiff.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getConfigDiffList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      2,
      f,
      proto.cluster_controller.ConfigFileDiff.serializeBinaryToWriter
    );
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.cluster_controller.AffectedNodeDiff.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.AffectedNodeDiff} returns this
 */
proto.cluster_controller.AffectedNodeDiff.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * repeated ConfigFileDiff config_diff = 2;
 * @return {!Array<!proto.cluster_controller.ConfigFileDiff>}
 */
proto.cluster_controller.AffectedNodeDiff.prototype.getConfigDiffList = function() {
  return /** @type{!Array<!proto.cluster_controller.ConfigFileDiff>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.cluster_controller.ConfigFileDiff, 2));
};


/**
 * @param {!Array<!proto.cluster_controller.ConfigFileDiff>} value
 * @return {!proto.cluster_controller.AffectedNodeDiff} returns this
*/
proto.cluster_controller.AffectedNodeDiff.prototype.setConfigDiffList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 2, value);
};


/**
 * @param {!proto.cluster_controller.ConfigFileDiff=} opt_value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.ConfigFileDiff}
 */
proto.cluster_controller.AffectedNodeDiff.prototype.addConfigDiff = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 2, opt_value, proto.cluster_controller.ConfigFileDiff, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.AffectedNodeDiff} returns this
 */
proto.cluster_controller.AffectedNodeDiff.prototype.clearConfigDiffList = function() {
  return this.setConfigDiffList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.PreviewNodeProfilesResponse.repeatedFields_ = [1,2,3,4,5];



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
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.PreviewNodeProfilesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.PreviewNodeProfilesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.PreviewNodeProfilesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
normalizedProfilesList: (f = jspb.Message.getRepeatedField(msg, 1)) == null ? undefined : f,
unitDiffList: jspb.Message.toObjectList(msg.getUnitDiffList(),
    proto.cluster_controller.UnitAction.toObject, includeInstance),
configDiffList: jspb.Message.toObjectList(msg.getConfigDiffList(),
    proto.cluster_controller.ConfigFileDiff.toObject, includeInstance),
restartUnitsList: (f = jspb.Message.getRepeatedField(msg, 4)) == null ? undefined : f,
affectedNodesList: jspb.Message.toObjectList(msg.getAffectedNodesList(),
    proto.cluster_controller.AffectedNodeDiff.toObject, includeInstance)
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
 * @return {!proto.cluster_controller.PreviewNodeProfilesResponse}
 */
proto.cluster_controller.PreviewNodeProfilesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.PreviewNodeProfilesResponse;
  return proto.cluster_controller.PreviewNodeProfilesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.PreviewNodeProfilesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.PreviewNodeProfilesResponse}
 */
proto.cluster_controller.PreviewNodeProfilesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.addNormalizedProfiles(value);
      break;
    case 2:
      var value = new proto.cluster_controller.UnitAction;
      reader.readMessage(value,proto.cluster_controller.UnitAction.deserializeBinaryFromReader);
      msg.addUnitDiff(value);
      break;
    case 3:
      var value = new proto.cluster_controller.ConfigFileDiff;
      reader.readMessage(value,proto.cluster_controller.ConfigFileDiff.deserializeBinaryFromReader);
      msg.addConfigDiff(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.addRestartUnits(value);
      break;
    case 5:
      var value = new proto.cluster_controller.AffectedNodeDiff;
      reader.readMessage(value,proto.cluster_controller.AffectedNodeDiff.deserializeBinaryFromReader);
      msg.addAffectedNodes(value);
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
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.PreviewNodeProfilesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.PreviewNodeProfilesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.PreviewNodeProfilesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNormalizedProfilesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      1,
      f
    );
  }
  f = message.getUnitDiffList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      2,
      f,
      proto.cluster_controller.UnitAction.serializeBinaryToWriter
    );
  }
  f = message.getConfigDiffList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      3,
      f,
      proto.cluster_controller.ConfigFileDiff.serializeBinaryToWriter
    );
  }
  f = message.getRestartUnitsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      4,
      f
    );
  }
  f = message.getAffectedNodesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      5,
      f,
      proto.cluster_controller.AffectedNodeDiff.serializeBinaryToWriter
    );
  }
};


/**
 * repeated string normalized_profiles = 1;
 * @return {!Array<string>}
 */
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.getNormalizedProfilesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 1));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.PreviewNodeProfilesResponse} returns this
 */
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.setNormalizedProfilesList = function(value) {
  return jspb.Message.setField(this, 1, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.PreviewNodeProfilesResponse} returns this
 */
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.addNormalizedProfiles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 1, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.PreviewNodeProfilesResponse} returns this
 */
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.clearNormalizedProfilesList = function() {
  return this.setNormalizedProfilesList([]);
};


/**
 * repeated UnitAction unit_diff = 2;
 * @return {!Array<!proto.cluster_controller.UnitAction>}
 */
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.getUnitDiffList = function() {
  return /** @type{!Array<!proto.cluster_controller.UnitAction>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.cluster_controller.UnitAction, 2));
};


/**
 * @param {!Array<!proto.cluster_controller.UnitAction>} value
 * @return {!proto.cluster_controller.PreviewNodeProfilesResponse} returns this
*/
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.setUnitDiffList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 2, value);
};


/**
 * @param {!proto.cluster_controller.UnitAction=} opt_value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.UnitAction}
 */
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.addUnitDiff = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 2, opt_value, proto.cluster_controller.UnitAction, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.PreviewNodeProfilesResponse} returns this
 */
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.clearUnitDiffList = function() {
  return this.setUnitDiffList([]);
};


/**
 * repeated ConfigFileDiff config_diff = 3;
 * @return {!Array<!proto.cluster_controller.ConfigFileDiff>}
 */
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.getConfigDiffList = function() {
  return /** @type{!Array<!proto.cluster_controller.ConfigFileDiff>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.cluster_controller.ConfigFileDiff, 3));
};


/**
 * @param {!Array<!proto.cluster_controller.ConfigFileDiff>} value
 * @return {!proto.cluster_controller.PreviewNodeProfilesResponse} returns this
*/
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.setConfigDiffList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 3, value);
};


/**
 * @param {!proto.cluster_controller.ConfigFileDiff=} opt_value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.ConfigFileDiff}
 */
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.addConfigDiff = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 3, opt_value, proto.cluster_controller.ConfigFileDiff, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.PreviewNodeProfilesResponse} returns this
 */
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.clearConfigDiffList = function() {
  return this.setConfigDiffList([]);
};


/**
 * repeated string restart_units = 4;
 * @return {!Array<string>}
 */
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.getRestartUnitsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 4));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.PreviewNodeProfilesResponse} returns this
 */
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.setRestartUnitsList = function(value) {
  return jspb.Message.setField(this, 4, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.PreviewNodeProfilesResponse} returns this
 */
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.addRestartUnits = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 4, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.PreviewNodeProfilesResponse} returns this
 */
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.clearRestartUnitsList = function() {
  return this.setRestartUnitsList([]);
};


/**
 * repeated AffectedNodeDiff affected_nodes = 5;
 * @return {!Array<!proto.cluster_controller.AffectedNodeDiff>}
 */
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.getAffectedNodesList = function() {
  return /** @type{!Array<!proto.cluster_controller.AffectedNodeDiff>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.cluster_controller.AffectedNodeDiff, 5));
};


/**
 * @param {!Array<!proto.cluster_controller.AffectedNodeDiff>} value
 * @return {!proto.cluster_controller.PreviewNodeProfilesResponse} returns this
*/
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.setAffectedNodesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 5, value);
};


/**
 * @param {!proto.cluster_controller.AffectedNodeDiff=} opt_value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.AffectedNodeDiff}
 */
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.addAffectedNodes = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 5, opt_value, proto.cluster_controller.AffectedNodeDiff, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.PreviewNodeProfilesResponse} returns this
 */
proto.cluster_controller.PreviewNodeProfilesResponse.prototype.clearAffectedNodesList = function() {
  return this.setAffectedNodesList([]);
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
proto.cluster_controller.DesiredService.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.DesiredService.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.DesiredService} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.DesiredService.toObject = function(includeInstance, msg) {
  var f, obj = {
serviceId: jspb.Message.getFieldWithDefault(msg, 1, ""),
version: jspb.Message.getFieldWithDefault(msg, 2, ""),
platform: jspb.Message.getFieldWithDefault(msg, 3, ""),
buildNumber: jspb.Message.getFieldWithDefault(msg, 4, 0),
status: jspb.Message.getFieldWithDefault(msg, 5, "")
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
 * @return {!proto.cluster_controller.DesiredService}
 */
proto.cluster_controller.DesiredService.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.DesiredService;
  return proto.cluster_controller.DesiredService.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.DesiredService} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.DesiredService}
 */
proto.cluster_controller.DesiredService.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setServiceId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setVersion(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlatform(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setStatus(value);
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
proto.cluster_controller.DesiredService.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.DesiredService.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.DesiredService} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.DesiredService.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getServiceId();
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
  f = message.getPlatform();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
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
};


/**
 * optional string service_id = 1;
 * @return {string}
 */
proto.cluster_controller.DesiredService.prototype.getServiceId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.DesiredService} returns this
 */
proto.cluster_controller.DesiredService.prototype.setServiceId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string version = 2;
 * @return {string}
 */
proto.cluster_controller.DesiredService.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.DesiredService} returns this
 */
proto.cluster_controller.DesiredService.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string platform = 3;
 * @return {string}
 */
proto.cluster_controller.DesiredService.prototype.getPlatform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.DesiredService} returns this
 */
proto.cluster_controller.DesiredService.prototype.setPlatform = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional int64 build_number = 4;
 * @return {number}
 */
proto.cluster_controller.DesiredService.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.cluster_controller.DesiredService} returns this
 */
proto.cluster_controller.DesiredService.prototype.setBuildNumber = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional string status = 5;
 * @return {string}
 */
proto.cluster_controller.DesiredService.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.DesiredService} returns this
 */
proto.cluster_controller.DesiredService.prototype.setStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.DesiredState.repeatedFields_ = [1];



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
proto.cluster_controller.DesiredState.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.DesiredState.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.DesiredState} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.DesiredState.toObject = function(includeInstance, msg) {
  var f, obj = {
servicesList: jspb.Message.toObjectList(msg.getServicesList(),
    proto.cluster_controller.DesiredService.toObject, includeInstance),
revision: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.cluster_controller.DesiredState}
 */
proto.cluster_controller.DesiredState.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.DesiredState;
  return proto.cluster_controller.DesiredState.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.DesiredState} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.DesiredState}
 */
proto.cluster_controller.DesiredState.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.cluster_controller.DesiredService;
      reader.readMessage(value,proto.cluster_controller.DesiredService.deserializeBinaryFromReader);
      msg.addServices(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setRevision(value);
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
proto.cluster_controller.DesiredState.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.DesiredState.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.DesiredState} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.DesiredState.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getServicesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.cluster_controller.DesiredService.serializeBinaryToWriter
    );
  }
  f = message.getRevision();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * repeated DesiredService services = 1;
 * @return {!Array<!proto.cluster_controller.DesiredService>}
 */
proto.cluster_controller.DesiredState.prototype.getServicesList = function() {
  return /** @type{!Array<!proto.cluster_controller.DesiredService>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.cluster_controller.DesiredService, 1));
};


/**
 * @param {!Array<!proto.cluster_controller.DesiredService>} value
 * @return {!proto.cluster_controller.DesiredState} returns this
*/
proto.cluster_controller.DesiredState.prototype.setServicesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.cluster_controller.DesiredService=} opt_value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.DesiredService}
 */
proto.cluster_controller.DesiredState.prototype.addServices = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.cluster_controller.DesiredService, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.DesiredState} returns this
 */
proto.cluster_controller.DesiredState.prototype.clearServicesList = function() {
  return this.setServicesList([]);
};


/**
 * optional string revision = 2;
 * @return {string}
 */
proto.cluster_controller.DesiredState.prototype.getRevision = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.DesiredState} returns this
 */
proto.cluster_controller.DesiredState.prototype.setRevision = function(value) {
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
proto.cluster_controller.UpsertDesiredServiceRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.UpsertDesiredServiceRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.UpsertDesiredServiceRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.UpsertDesiredServiceRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
service: (f = msg.getService()) && proto.cluster_controller.DesiredService.toObject(includeInstance, f)
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
 * @return {!proto.cluster_controller.UpsertDesiredServiceRequest}
 */
proto.cluster_controller.UpsertDesiredServiceRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.UpsertDesiredServiceRequest;
  return proto.cluster_controller.UpsertDesiredServiceRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.UpsertDesiredServiceRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.UpsertDesiredServiceRequest}
 */
proto.cluster_controller.UpsertDesiredServiceRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.cluster_controller.DesiredService;
      reader.readMessage(value,proto.cluster_controller.DesiredService.deserializeBinaryFromReader);
      msg.setService(value);
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
proto.cluster_controller.UpsertDesiredServiceRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.UpsertDesiredServiceRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.UpsertDesiredServiceRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.UpsertDesiredServiceRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getService();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.cluster_controller.DesiredService.serializeBinaryToWriter
    );
  }
};


/**
 * optional DesiredService service = 1;
 * @return {?proto.cluster_controller.DesiredService}
 */
proto.cluster_controller.UpsertDesiredServiceRequest.prototype.getService = function() {
  return /** @type{?proto.cluster_controller.DesiredService} */ (
    jspb.Message.getWrapperField(this, proto.cluster_controller.DesiredService, 1));
};


/**
 * @param {?proto.cluster_controller.DesiredService|undefined} value
 * @return {!proto.cluster_controller.UpsertDesiredServiceRequest} returns this
*/
proto.cluster_controller.UpsertDesiredServiceRequest.prototype.setService = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.cluster_controller.UpsertDesiredServiceRequest} returns this
 */
proto.cluster_controller.UpsertDesiredServiceRequest.prototype.clearService = function() {
  return this.setService(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.cluster_controller.UpsertDesiredServiceRequest.prototype.hasService = function() {
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
proto.cluster_controller.RemoveDesiredServiceRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.RemoveDesiredServiceRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.RemoveDesiredServiceRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.RemoveDesiredServiceRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
serviceId: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.cluster_controller.RemoveDesiredServiceRequest}
 */
proto.cluster_controller.RemoveDesiredServiceRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.RemoveDesiredServiceRequest;
  return proto.cluster_controller.RemoveDesiredServiceRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.RemoveDesiredServiceRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.RemoveDesiredServiceRequest}
 */
proto.cluster_controller.RemoveDesiredServiceRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setServiceId(value);
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
proto.cluster_controller.RemoveDesiredServiceRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.RemoveDesiredServiceRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.RemoveDesiredServiceRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.RemoveDesiredServiceRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getServiceId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string service_id = 1;
 * @return {string}
 */
proto.cluster_controller.RemoveDesiredServiceRequest.prototype.getServiceId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.RemoveDesiredServiceRequest} returns this
 */
proto.cluster_controller.RemoveDesiredServiceRequest.prototype.setServiceId = function(value) {
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
proto.cluster_controller.SeedDesiredStateRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.SeedDesiredStateRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.SeedDesiredStateRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.SeedDesiredStateRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
mode: jspb.Message.getFieldWithDefault(msg, 1, 0)
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
 * @return {!proto.cluster_controller.SeedDesiredStateRequest}
 */
proto.cluster_controller.SeedDesiredStateRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.SeedDesiredStateRequest;
  return proto.cluster_controller.SeedDesiredStateRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.SeedDesiredStateRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.SeedDesiredStateRequest}
 */
proto.cluster_controller.SeedDesiredStateRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.cluster_controller.SeedDesiredStateRequest.Mode} */ (reader.readEnum());
      msg.setMode(value);
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
proto.cluster_controller.SeedDesiredStateRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.SeedDesiredStateRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.SeedDesiredStateRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.SeedDesiredStateRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMode();
  if (f !== 0.0) {
    writer.writeEnum(
      1,
      f
    );
  }
};


/**
 * @enum {number}
 */
proto.cluster_controller.SeedDesiredStateRequest.Mode = {
  DEFAULT_CORE_PROFILE: 0,
  IMPORT_FROM_INSTALLED: 1
};

/**
 * optional Mode mode = 1;
 * @return {!proto.cluster_controller.SeedDesiredStateRequest.Mode}
 */
proto.cluster_controller.SeedDesiredStateRequest.prototype.getMode = function() {
  return /** @type {!proto.cluster_controller.SeedDesiredStateRequest.Mode} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {!proto.cluster_controller.SeedDesiredStateRequest.Mode} value
 * @return {!proto.cluster_controller.SeedDesiredStateRequest} returns this
 */
proto.cluster_controller.SeedDesiredStateRequest.prototype.setMode = function(value) {
  return jspb.Message.setProto3EnumField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.ValidateArtifactRequest.repeatedFields_ = [3];



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
proto.cluster_controller.ValidateArtifactRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.ValidateArtifactRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.ValidateArtifactRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ValidateArtifactRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
serviceId: jspb.Message.getFieldWithDefault(msg, 1, ""),
version: jspb.Message.getFieldWithDefault(msg, 2, ""),
targetNodeIdsList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f
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
 * @return {!proto.cluster_controller.ValidateArtifactRequest}
 */
proto.cluster_controller.ValidateArtifactRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.ValidateArtifactRequest;
  return proto.cluster_controller.ValidateArtifactRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.ValidateArtifactRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.ValidateArtifactRequest}
 */
proto.cluster_controller.ValidateArtifactRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setServiceId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setVersion(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.addTargetNodeIds(value);
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
proto.cluster_controller.ValidateArtifactRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.ValidateArtifactRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.ValidateArtifactRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ValidateArtifactRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getServiceId();
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
  f = message.getTargetNodeIdsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      3,
      f
    );
  }
};


/**
 * optional string service_id = 1;
 * @return {string}
 */
proto.cluster_controller.ValidateArtifactRequest.prototype.getServiceId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ValidateArtifactRequest} returns this
 */
proto.cluster_controller.ValidateArtifactRequest.prototype.setServiceId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string version = 2;
 * @return {string}
 */
proto.cluster_controller.ValidateArtifactRequest.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ValidateArtifactRequest} returns this
 */
proto.cluster_controller.ValidateArtifactRequest.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * repeated string target_node_ids = 3;
 * @return {!Array<string>}
 */
proto.cluster_controller.ValidateArtifactRequest.prototype.getTargetNodeIdsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.ValidateArtifactRequest} returns this
 */
proto.cluster_controller.ValidateArtifactRequest.prototype.setTargetNodeIdsList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.ValidateArtifactRequest} returns this
 */
proto.cluster_controller.ValidateArtifactRequest.prototype.addTargetNodeIds = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.ValidateArtifactRequest} returns this
 */
proto.cluster_controller.ValidateArtifactRequest.prototype.clearTargetNodeIdsList = function() {
  return this.setTargetNodeIdsList([]);
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
proto.cluster_controller.ValidationIssue.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.ValidationIssue.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.ValidationIssue} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ValidationIssue.toObject = function(includeInstance, msg) {
  var f, obj = {
severity: jspb.Message.getFieldWithDefault(msg, 1, 0),
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
 * @return {!proto.cluster_controller.ValidationIssue}
 */
proto.cluster_controller.ValidationIssue.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.ValidationIssue;
  return proto.cluster_controller.ValidationIssue.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.ValidationIssue} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.ValidationIssue}
 */
proto.cluster_controller.ValidationIssue.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.cluster_controller.ValidationIssue.Severity} */ (reader.readEnum());
      msg.setSeverity(value);
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
proto.cluster_controller.ValidationIssue.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.ValidationIssue.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.ValidationIssue} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ValidationIssue.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSeverity();
  if (f !== 0.0) {
    writer.writeEnum(
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
 * @enum {number}
 */
proto.cluster_controller.ValidationIssue.Severity = {
  ERROR: 0,
  WARNING: 1
};

/**
 * optional Severity severity = 1;
 * @return {!proto.cluster_controller.ValidationIssue.Severity}
 */
proto.cluster_controller.ValidationIssue.prototype.getSeverity = function() {
  return /** @type {!proto.cluster_controller.ValidationIssue.Severity} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {!proto.cluster_controller.ValidationIssue.Severity} value
 * @return {!proto.cluster_controller.ValidationIssue} returns this
 */
proto.cluster_controller.ValidationIssue.prototype.setSeverity = function(value) {
  return jspb.Message.setProto3EnumField(this, 1, value);
};


/**
 * optional string message = 2;
 * @return {string}
 */
proto.cluster_controller.ValidationIssue.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ValidationIssue} returns this
 */
proto.cluster_controller.ValidationIssue.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.ValidationReport.repeatedFields_ = [4];



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
proto.cluster_controller.ValidationReport.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.ValidationReport.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.ValidationReport} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ValidationReport.toObject = function(includeInstance, msg) {
  var f, obj = {
checksumOk: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
signatureStatus: jspb.Message.getFieldWithDefault(msg, 2, ""),
platformOk: jspb.Message.getBooleanFieldWithDefault(msg, 3, false),
issuesList: jspb.Message.toObjectList(msg.getIssuesList(),
    proto.cluster_controller.ValidationIssue.toObject, includeInstance)
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
 * @return {!proto.cluster_controller.ValidationReport}
 */
proto.cluster_controller.ValidationReport.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.ValidationReport;
  return proto.cluster_controller.ValidationReport.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.ValidationReport} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.ValidationReport}
 */
proto.cluster_controller.ValidationReport.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setChecksumOk(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setSignatureStatus(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setPlatformOk(value);
      break;
    case 4:
      var value = new proto.cluster_controller.ValidationIssue;
      reader.readMessage(value,proto.cluster_controller.ValidationIssue.deserializeBinaryFromReader);
      msg.addIssues(value);
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
proto.cluster_controller.ValidationReport.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.ValidationReport.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.ValidationReport} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ValidationReport.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getChecksumOk();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getSignatureStatus();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getPlatformOk();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
  f = message.getIssuesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      4,
      f,
      proto.cluster_controller.ValidationIssue.serializeBinaryToWriter
    );
  }
};


/**
 * optional bool checksum_ok = 1;
 * @return {boolean}
 */
proto.cluster_controller.ValidationReport.prototype.getChecksumOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.cluster_controller.ValidationReport} returns this
 */
proto.cluster_controller.ValidationReport.prototype.setChecksumOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional string signature_status = 2;
 * @return {string}
 */
proto.cluster_controller.ValidationReport.prototype.getSignatureStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.ValidationReport} returns this
 */
proto.cluster_controller.ValidationReport.prototype.setSignatureStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional bool platform_ok = 3;
 * @return {boolean}
 */
proto.cluster_controller.ValidationReport.prototype.getPlatformOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.cluster_controller.ValidationReport} returns this
 */
proto.cluster_controller.ValidationReport.prototype.setPlatformOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * repeated ValidationIssue issues = 4;
 * @return {!Array<!proto.cluster_controller.ValidationIssue>}
 */
proto.cluster_controller.ValidationReport.prototype.getIssuesList = function() {
  return /** @type{!Array<!proto.cluster_controller.ValidationIssue>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.cluster_controller.ValidationIssue, 4));
};


/**
 * @param {!Array<!proto.cluster_controller.ValidationIssue>} value
 * @return {!proto.cluster_controller.ValidationReport} returns this
*/
proto.cluster_controller.ValidationReport.prototype.setIssuesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 4, value);
};


/**
 * @param {!proto.cluster_controller.ValidationIssue=} opt_value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.ValidationIssue}
 */
proto.cluster_controller.ValidationReport.prototype.addIssues = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 4, opt_value, proto.cluster_controller.ValidationIssue, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.ValidationReport} returns this
 */
proto.cluster_controller.ValidationReport.prototype.clearIssuesList = function() {
  return this.setIssuesList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.DesiredServicesDelta.repeatedFields_ = [1,2];



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
proto.cluster_controller.DesiredServicesDelta.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.DesiredServicesDelta.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.DesiredServicesDelta} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.DesiredServicesDelta.toObject = function(includeInstance, msg) {
  var f, obj = {
upsertsList: jspb.Message.toObjectList(msg.getUpsertsList(),
    proto.cluster_controller.DesiredService.toObject, includeInstance),
removalsList: (f = jspb.Message.getRepeatedField(msg, 2)) == null ? undefined : f
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
 * @return {!proto.cluster_controller.DesiredServicesDelta}
 */
proto.cluster_controller.DesiredServicesDelta.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.DesiredServicesDelta;
  return proto.cluster_controller.DesiredServicesDelta.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.DesiredServicesDelta} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.DesiredServicesDelta}
 */
proto.cluster_controller.DesiredServicesDelta.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.cluster_controller.DesiredService;
      reader.readMessage(value,proto.cluster_controller.DesiredService.deserializeBinaryFromReader);
      msg.addUpserts(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.addRemovals(value);
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
proto.cluster_controller.DesiredServicesDelta.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.DesiredServicesDelta.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.DesiredServicesDelta} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.DesiredServicesDelta.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUpsertsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.cluster_controller.DesiredService.serializeBinaryToWriter
    );
  }
  f = message.getRemovalsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      2,
      f
    );
  }
};


/**
 * repeated DesiredService upserts = 1;
 * @return {!Array<!proto.cluster_controller.DesiredService>}
 */
proto.cluster_controller.DesiredServicesDelta.prototype.getUpsertsList = function() {
  return /** @type{!Array<!proto.cluster_controller.DesiredService>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.cluster_controller.DesiredService, 1));
};


/**
 * @param {!Array<!proto.cluster_controller.DesiredService>} value
 * @return {!proto.cluster_controller.DesiredServicesDelta} returns this
*/
proto.cluster_controller.DesiredServicesDelta.prototype.setUpsertsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.cluster_controller.DesiredService=} opt_value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.DesiredService}
 */
proto.cluster_controller.DesiredServicesDelta.prototype.addUpserts = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.cluster_controller.DesiredService, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.DesiredServicesDelta} returns this
 */
proto.cluster_controller.DesiredServicesDelta.prototype.clearUpsertsList = function() {
  return this.setUpsertsList([]);
};


/**
 * repeated string removals = 2;
 * @return {!Array<string>}
 */
proto.cluster_controller.DesiredServicesDelta.prototype.getRemovalsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.DesiredServicesDelta} returns this
 */
proto.cluster_controller.DesiredServicesDelta.prototype.setRemovalsList = function(value) {
  return jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.DesiredServicesDelta} returns this
 */
proto.cluster_controller.DesiredServicesDelta.prototype.addRemovals = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.DesiredServicesDelta} returns this
 */
proto.cluster_controller.DesiredServicesDelta.prototype.clearRemovalsList = function() {
  return this.setRemovalsList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.NodeChange.repeatedFields_ = [2,3,4];



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
proto.cluster_controller.NodeChange.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.NodeChange.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.NodeChange} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.NodeChange.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
willInstallList: (f = jspb.Message.getRepeatedField(msg, 2)) == null ? undefined : f,
willRemoveList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f,
warningsList: (f = jspb.Message.getRepeatedField(msg, 4)) == null ? undefined : f
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
 * @return {!proto.cluster_controller.NodeChange}
 */
proto.cluster_controller.NodeChange.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.NodeChange;
  return proto.cluster_controller.NodeChange.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.NodeChange} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.NodeChange}
 */
proto.cluster_controller.NodeChange.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.addWillInstall(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.addWillRemove(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
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
proto.cluster_controller.NodeChange.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.NodeChange.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.NodeChange} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.NodeChange.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getWillInstallList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      2,
      f
    );
  }
  f = message.getWillRemoveList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      3,
      f
    );
  }
  f = message.getWarningsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      4,
      f
    );
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.cluster_controller.NodeChange.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.NodeChange} returns this
 */
proto.cluster_controller.NodeChange.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * repeated string will_install = 2;
 * @return {!Array<string>}
 */
proto.cluster_controller.NodeChange.prototype.getWillInstallList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.NodeChange} returns this
 */
proto.cluster_controller.NodeChange.prototype.setWillInstallList = function(value) {
  return jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.NodeChange} returns this
 */
proto.cluster_controller.NodeChange.prototype.addWillInstall = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.NodeChange} returns this
 */
proto.cluster_controller.NodeChange.prototype.clearWillInstallList = function() {
  return this.setWillInstallList([]);
};


/**
 * repeated string will_remove = 3;
 * @return {!Array<string>}
 */
proto.cluster_controller.NodeChange.prototype.getWillRemoveList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.NodeChange} returns this
 */
proto.cluster_controller.NodeChange.prototype.setWillRemoveList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.NodeChange} returns this
 */
proto.cluster_controller.NodeChange.prototype.addWillRemove = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.NodeChange} returns this
 */
proto.cluster_controller.NodeChange.prototype.clearWillRemoveList = function() {
  return this.setWillRemoveList([]);
};


/**
 * repeated string warnings = 4;
 * @return {!Array<string>}
 */
proto.cluster_controller.NodeChange.prototype.getWarningsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 4));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.NodeChange} returns this
 */
proto.cluster_controller.NodeChange.prototype.setWarningsList = function(value) {
  return jspb.Message.setField(this, 4, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.NodeChange} returns this
 */
proto.cluster_controller.NodeChange.prototype.addWarnings = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 4, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.NodeChange} returns this
 */
proto.cluster_controller.NodeChange.prototype.clearWarningsList = function() {
  return this.setWarningsList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.ServiceChangePreview.repeatedFields_ = [1,2];



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
proto.cluster_controller.ServiceChangePreview.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.ServiceChangePreview.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.ServiceChangePreview} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ServiceChangePreview.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeChangesList: jspb.Message.toObjectList(msg.getNodeChangesList(),
    proto.cluster_controller.NodeChange.toObject, includeInstance),
blockingIssuesList: jspb.Message.toObjectList(msg.getBlockingIssuesList(),
    proto.cluster_controller.ValidationIssue.toObject, includeInstance)
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
 * @return {!proto.cluster_controller.ServiceChangePreview}
 */
proto.cluster_controller.ServiceChangePreview.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.ServiceChangePreview;
  return proto.cluster_controller.ServiceChangePreview.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.ServiceChangePreview} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.ServiceChangePreview}
 */
proto.cluster_controller.ServiceChangePreview.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.cluster_controller.NodeChange;
      reader.readMessage(value,proto.cluster_controller.NodeChange.deserializeBinaryFromReader);
      msg.addNodeChanges(value);
      break;
    case 2:
      var value = new proto.cluster_controller.ValidationIssue;
      reader.readMessage(value,proto.cluster_controller.ValidationIssue.deserializeBinaryFromReader);
      msg.addBlockingIssues(value);
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
proto.cluster_controller.ServiceChangePreview.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.ServiceChangePreview.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.ServiceChangePreview} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.ServiceChangePreview.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeChangesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.cluster_controller.NodeChange.serializeBinaryToWriter
    );
  }
  f = message.getBlockingIssuesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      2,
      f,
      proto.cluster_controller.ValidationIssue.serializeBinaryToWriter
    );
  }
};


/**
 * repeated NodeChange node_changes = 1;
 * @return {!Array<!proto.cluster_controller.NodeChange>}
 */
proto.cluster_controller.ServiceChangePreview.prototype.getNodeChangesList = function() {
  return /** @type{!Array<!proto.cluster_controller.NodeChange>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.cluster_controller.NodeChange, 1));
};


/**
 * @param {!Array<!proto.cluster_controller.NodeChange>} value
 * @return {!proto.cluster_controller.ServiceChangePreview} returns this
*/
proto.cluster_controller.ServiceChangePreview.prototype.setNodeChangesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.cluster_controller.NodeChange=} opt_value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.NodeChange}
 */
proto.cluster_controller.ServiceChangePreview.prototype.addNodeChanges = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.cluster_controller.NodeChange, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.ServiceChangePreview} returns this
 */
proto.cluster_controller.ServiceChangePreview.prototype.clearNodeChangesList = function() {
  return this.setNodeChangesList([]);
};


/**
 * repeated ValidationIssue blocking_issues = 2;
 * @return {!Array<!proto.cluster_controller.ValidationIssue>}
 */
proto.cluster_controller.ServiceChangePreview.prototype.getBlockingIssuesList = function() {
  return /** @type{!Array<!proto.cluster_controller.ValidationIssue>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.cluster_controller.ValidationIssue, 2));
};


/**
 * @param {!Array<!proto.cluster_controller.ValidationIssue>} value
 * @return {!proto.cluster_controller.ServiceChangePreview} returns this
*/
proto.cluster_controller.ServiceChangePreview.prototype.setBlockingIssuesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 2, value);
};


/**
 * @param {!proto.cluster_controller.ValidationIssue=} opt_value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.ValidationIssue}
 */
proto.cluster_controller.ServiceChangePreview.prototype.addBlockingIssues = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 2, opt_value, proto.cluster_controller.ValidationIssue, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.ServiceChangePreview} returns this
 */
proto.cluster_controller.ServiceChangePreview.prototype.clearBlockingIssuesList = function() {
  return this.setBlockingIssuesList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.cluster_controller.InstallPolicy.repeatedFields_ = [3,4];



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
proto.cluster_controller.InstallPolicy.prototype.toObject = function(opt_includeInstance) {
  return proto.cluster_controller.InstallPolicy.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.cluster_controller.InstallPolicy} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.InstallPolicy.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, ""),
verifiedPublishersOnly: jspb.Message.getBooleanFieldWithDefault(msg, 2, false),
allowedNamespacesList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f,
blockedNamespacesList: (f = jspb.Message.getRepeatedField(msg, 4)) == null ? undefined : f,
blockDeprecated: jspb.Message.getBooleanFieldWithDefault(msg, 5, false),
blockYanked: jspb.Message.getBooleanFieldWithDefault(msg, 6, false)
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
 * @return {!proto.cluster_controller.InstallPolicy}
 */
proto.cluster_controller.InstallPolicy.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.cluster_controller.InstallPolicy;
  return proto.cluster_controller.InstallPolicy.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.cluster_controller.InstallPolicy} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.cluster_controller.InstallPolicy}
 */
proto.cluster_controller.InstallPolicy.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setVerifiedPublishersOnly(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.addAllowedNamespaces(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.addBlockedNamespaces(value);
      break;
    case 5:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setBlockDeprecated(value);
      break;
    case 6:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setBlockYanked(value);
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
proto.cluster_controller.InstallPolicy.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.cluster_controller.InstallPolicy.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.cluster_controller.InstallPolicy} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.cluster_controller.InstallPolicy.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getVerifiedPublishersOnly();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
  f = message.getAllowedNamespacesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      3,
      f
    );
  }
  f = message.getBlockedNamespacesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      4,
      f
    );
  }
  f = message.getBlockDeprecated();
  if (f) {
    writer.writeBool(
      5,
      f
    );
  }
  f = message.getBlockYanked();
  if (f) {
    writer.writeBool(
      6,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.cluster_controller.InstallPolicy.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.cluster_controller.InstallPolicy} returns this
 */
proto.cluster_controller.InstallPolicy.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bool verified_publishers_only = 2;
 * @return {boolean}
 */
proto.cluster_controller.InstallPolicy.prototype.getVerifiedPublishersOnly = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.cluster_controller.InstallPolicy} returns this
 */
proto.cluster_controller.InstallPolicy.prototype.setVerifiedPublishersOnly = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * repeated string allowed_namespaces = 3;
 * @return {!Array<string>}
 */
proto.cluster_controller.InstallPolicy.prototype.getAllowedNamespacesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.InstallPolicy} returns this
 */
proto.cluster_controller.InstallPolicy.prototype.setAllowedNamespacesList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.InstallPolicy} returns this
 */
proto.cluster_controller.InstallPolicy.prototype.addAllowedNamespaces = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.InstallPolicy} returns this
 */
proto.cluster_controller.InstallPolicy.prototype.clearAllowedNamespacesList = function() {
  return this.setAllowedNamespacesList([]);
};


/**
 * repeated string blocked_namespaces = 4;
 * @return {!Array<string>}
 */
proto.cluster_controller.InstallPolicy.prototype.getBlockedNamespacesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 4));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.cluster_controller.InstallPolicy} returns this
 */
proto.cluster_controller.InstallPolicy.prototype.setBlockedNamespacesList = function(value) {
  return jspb.Message.setField(this, 4, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.cluster_controller.InstallPolicy} returns this
 */
proto.cluster_controller.InstallPolicy.prototype.addBlockedNamespaces = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 4, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.cluster_controller.InstallPolicy} returns this
 */
proto.cluster_controller.InstallPolicy.prototype.clearBlockedNamespacesList = function() {
  return this.setBlockedNamespacesList([]);
};


/**
 * optional bool block_deprecated = 5;
 * @return {boolean}
 */
proto.cluster_controller.InstallPolicy.prototype.getBlockDeprecated = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 5, false));
};


/**
 * @param {boolean} value
 * @return {!proto.cluster_controller.InstallPolicy} returns this
 */
proto.cluster_controller.InstallPolicy.prototype.setBlockDeprecated = function(value) {
  return jspb.Message.setProto3BooleanField(this, 5, value);
};


/**
 * optional bool block_yanked = 6;
 * @return {boolean}
 */
proto.cluster_controller.InstallPolicy.prototype.getBlockYanked = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 6, false));
};


/**
 * @param {boolean} value
 * @return {!proto.cluster_controller.InstallPolicy} returns this
 */
proto.cluster_controller.InstallPolicy.prototype.setBlockYanked = function(value) {
  return jspb.Message.setProto3BooleanField(this, 6, value);
};


/**
 * @enum {number}
 */
proto.cluster_controller.ArtifactKind = {
  ARTIFACT_KIND_UNSPECIFIED: 0,
  ARTIFACT_SERVICE: 1,
  ARTIFACT_APPLICATION: 2,
  ARTIFACT_SUBSYSTEM: 3
};

/**
 * @enum {number}
 */
proto.cluster_controller.OperationPhase = {
  OP_PHASE_UNSPECIFIED: 0,
  OP_QUEUED: 1,
  OP_RUNNING: 2,
  OP_SUCCEEDED: 3,
  OP_FAILED: 4
};

goog.object.extend(exports, proto.cluster_controller);
