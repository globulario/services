// source: clustercontroller.proto
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
var plan_pb = require('./plan_pb.js');
goog.object.extend(proto, plan_pb);
goog.exportSymbol('proto.clustercontroller.ApplyNodePlanRequest', null, global);
goog.exportSymbol('proto.clustercontroller.ApplyNodePlanResponse', null, global);
goog.exportSymbol('proto.clustercontroller.ApplyNodePlanV1Request', null, global);
goog.exportSymbol('proto.clustercontroller.ApplyNodePlanV1Response', null, global);
goog.exportSymbol('proto.clustercontroller.ApproveJoinRequest', null, global);
goog.exportSymbol('proto.clustercontroller.ApproveJoinResponse', null, global);
goog.exportSymbol('proto.clustercontroller.ArtifactKind', null, global);
goog.exportSymbol('proto.clustercontroller.ArtifactRef', null, global);
goog.exportSymbol('proto.clustercontroller.ClusterInfo', null, global);
goog.exportSymbol('proto.clustercontroller.ClusterNetworkSpec', null, global);
goog.exportSymbol('proto.clustercontroller.CompleteOperationRequest', null, global);
goog.exportSymbol('proto.clustercontroller.CompleteOperationResponse', null, global);
goog.exportSymbol('proto.clustercontroller.CreateJoinTokenRequest', null, global);
goog.exportSymbol('proto.clustercontroller.CreateJoinTokenResponse', null, global);
goog.exportSymbol('proto.clustercontroller.DesiredNetwork', null, global);
goog.exportSymbol('proto.clustercontroller.GetClusterHealthRequest', null, global);
goog.exportSymbol('proto.clustercontroller.GetClusterHealthResponse', null, global);
goog.exportSymbol('proto.clustercontroller.GetClusterHealthV1Request', null, global);
goog.exportSymbol('proto.clustercontroller.GetClusterHealthV1Response', null, global);
goog.exportSymbol('proto.clustercontroller.GetJoinRequestStatusRequest', null, global);
goog.exportSymbol('proto.clustercontroller.GetJoinRequestStatusResponse', null, global);
goog.exportSymbol('proto.clustercontroller.GetNodePlanRequest', null, global);
goog.exportSymbol('proto.clustercontroller.GetNodePlanResponse', null, global);
goog.exportSymbol('proto.clustercontroller.GetNodePlanV1Request', null, global);
goog.exportSymbol('proto.clustercontroller.GetNodePlanV1Response', null, global);
goog.exportSymbol('proto.clustercontroller.JoinRequestRecord', null, global);
goog.exportSymbol('proto.clustercontroller.ListJoinRequestsRequest', null, global);
goog.exportSymbol('proto.clustercontroller.ListJoinRequestsResponse', null, global);
goog.exportSymbol('proto.clustercontroller.ListNodesRequest', null, global);
goog.exportSymbol('proto.clustercontroller.ListNodesResponse', null, global);
goog.exportSymbol('proto.clustercontroller.NodeHealth', null, global);
goog.exportSymbol('proto.clustercontroller.NodeHealthStatus', null, global);
goog.exportSymbol('proto.clustercontroller.NodeIdentity', null, global);
goog.exportSymbol('proto.clustercontroller.NodePlan', null, global);
goog.exportSymbol('proto.clustercontroller.NodeRecord', null, global);
goog.exportSymbol('proto.clustercontroller.NodeStatus', null, global);
goog.exportSymbol('proto.clustercontroller.NodeUnitStatus', null, global);
goog.exportSymbol('proto.clustercontroller.OperationEvent', null, global);
goog.exportSymbol('proto.clustercontroller.OperationPhase', null, global);
goog.exportSymbol('proto.clustercontroller.ReconcileNodeV1Request', null, global);
goog.exportSymbol('proto.clustercontroller.ReconcileNodeV1Response', null, global);
goog.exportSymbol('proto.clustercontroller.RejectJoinRequest', null, global);
goog.exportSymbol('proto.clustercontroller.RejectJoinResponse', null, global);
goog.exportSymbol('proto.clustercontroller.RemoveNodeRequest', null, global);
goog.exportSymbol('proto.clustercontroller.RemoveNodeResponse', null, global);
goog.exportSymbol('proto.clustercontroller.ReportNodeStatusRequest', null, global);
goog.exportSymbol('proto.clustercontroller.ReportNodeStatusResponse', null, global);
goog.exportSymbol('proto.clustercontroller.RequestJoinRequest', null, global);
goog.exportSymbol('proto.clustercontroller.RequestJoinResponse', null, global);
goog.exportSymbol('proto.clustercontroller.ServiceSummary', null, global);
goog.exportSymbol('proto.clustercontroller.SetNodeProfilesRequest', null, global);
goog.exportSymbol('proto.clustercontroller.SetNodeProfilesResponse', null, global);
goog.exportSymbol('proto.clustercontroller.StartApplyRequest', null, global);
goog.exportSymbol('proto.clustercontroller.StartApplyResponse', null, global);
goog.exportSymbol('proto.clustercontroller.UnitAction', null, global);
goog.exportSymbol('proto.clustercontroller.UpdateClusterNetworkRequest', null, global);
goog.exportSymbol('proto.clustercontroller.UpdateClusterNetworkResponse', null, global);
goog.exportSymbol('proto.clustercontroller.UpgradeGlobularRequest', null, global);
goog.exportSymbol('proto.clustercontroller.UpgradeGlobularResponse', null, global);
goog.exportSymbol('proto.clustercontroller.WatchNodePlanStatusV1Request', null, global);
goog.exportSymbol('proto.clustercontroller.WatchOperationsRequest', null, global);
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.ClusterInfo = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.ClusterInfo, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.ClusterInfo.displayName = 'proto.clustercontroller.ClusterInfo';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.ClusterNetworkSpec = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.clustercontroller.ClusterNetworkSpec.repeatedFields_, null);
};
goog.inherits(proto.clustercontroller.ClusterNetworkSpec, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.ClusterNetworkSpec.displayName = 'proto.clustercontroller.ClusterNetworkSpec';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.NodeIdentity = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.clustercontroller.NodeIdentity.repeatedFields_, null);
};
goog.inherits(proto.clustercontroller.NodeIdentity, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.NodeIdentity.displayName = 'proto.clustercontroller.NodeIdentity';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.NodeRecord = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.clustercontroller.NodeRecord.repeatedFields_, null);
};
goog.inherits(proto.clustercontroller.NodeRecord, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.NodeRecord.displayName = 'proto.clustercontroller.NodeRecord';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.CreateJoinTokenRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.CreateJoinTokenRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.CreateJoinTokenRequest.displayName = 'proto.clustercontroller.CreateJoinTokenRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.CreateJoinTokenResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.CreateJoinTokenResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.CreateJoinTokenResponse.displayName = 'proto.clustercontroller.CreateJoinTokenResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.RequestJoinRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.RequestJoinRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.RequestJoinRequest.displayName = 'proto.clustercontroller.RequestJoinRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.RequestJoinResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.RequestJoinResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.RequestJoinResponse.displayName = 'proto.clustercontroller.RequestJoinResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.GetJoinRequestStatusRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.GetJoinRequestStatusRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.GetJoinRequestStatusRequest.displayName = 'proto.clustercontroller.GetJoinRequestStatusRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.GetJoinRequestStatusResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.clustercontroller.GetJoinRequestStatusResponse.repeatedFields_, null);
};
goog.inherits(proto.clustercontroller.GetJoinRequestStatusResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.GetJoinRequestStatusResponse.displayName = 'proto.clustercontroller.GetJoinRequestStatusResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.JoinRequestRecord = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.clustercontroller.JoinRequestRecord.repeatedFields_, null);
};
goog.inherits(proto.clustercontroller.JoinRequestRecord, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.JoinRequestRecord.displayName = 'proto.clustercontroller.JoinRequestRecord';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.ListJoinRequestsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.ListJoinRequestsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.ListJoinRequestsRequest.displayName = 'proto.clustercontroller.ListJoinRequestsRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.ListJoinRequestsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.clustercontroller.ListJoinRequestsResponse.repeatedFields_, null);
};
goog.inherits(proto.clustercontroller.ListJoinRequestsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.ListJoinRequestsResponse.displayName = 'proto.clustercontroller.ListJoinRequestsResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.ApproveJoinRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.clustercontroller.ApproveJoinRequest.repeatedFields_, null);
};
goog.inherits(proto.clustercontroller.ApproveJoinRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.ApproveJoinRequest.displayName = 'proto.clustercontroller.ApproveJoinRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.ApproveJoinResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.ApproveJoinResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.ApproveJoinResponse.displayName = 'proto.clustercontroller.ApproveJoinResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.RejectJoinRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.RejectJoinRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.RejectJoinRequest.displayName = 'proto.clustercontroller.RejectJoinRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.RejectJoinResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.RejectJoinResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.RejectJoinResponse.displayName = 'proto.clustercontroller.RejectJoinResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.ListNodesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.ListNodesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.ListNodesRequest.displayName = 'proto.clustercontroller.ListNodesRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.ListNodesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.clustercontroller.ListNodesResponse.repeatedFields_, null);
};
goog.inherits(proto.clustercontroller.ListNodesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.ListNodesResponse.displayName = 'proto.clustercontroller.ListNodesResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.SetNodeProfilesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.clustercontroller.SetNodeProfilesRequest.repeatedFields_, null);
};
goog.inherits(proto.clustercontroller.SetNodeProfilesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.SetNodeProfilesRequest.displayName = 'proto.clustercontroller.SetNodeProfilesRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.SetNodeProfilesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.SetNodeProfilesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.SetNodeProfilesResponse.displayName = 'proto.clustercontroller.SetNodeProfilesResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.RemoveNodeRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.RemoveNodeRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.RemoveNodeRequest.displayName = 'proto.clustercontroller.RemoveNodeRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.RemoveNodeResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.RemoveNodeResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.RemoveNodeResponse.displayName = 'proto.clustercontroller.RemoveNodeResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.GetClusterHealthRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.GetClusterHealthRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.GetClusterHealthRequest.displayName = 'proto.clustercontroller.GetClusterHealthRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.GetClusterHealthResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.clustercontroller.GetClusterHealthResponse.repeatedFields_, null);
};
goog.inherits(proto.clustercontroller.GetClusterHealthResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.GetClusterHealthResponse.displayName = 'proto.clustercontroller.GetClusterHealthResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.NodeHealthStatus = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.NodeHealthStatus, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.NodeHealthStatus.displayName = 'proto.clustercontroller.NodeHealthStatus';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.UpdateClusterNetworkRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.UpdateClusterNetworkRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.UpdateClusterNetworkRequest.displayName = 'proto.clustercontroller.UpdateClusterNetworkRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.UpdateClusterNetworkResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.UpdateClusterNetworkResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.UpdateClusterNetworkResponse.displayName = 'proto.clustercontroller.UpdateClusterNetworkResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.ApplyNodePlanRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.ApplyNodePlanRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.ApplyNodePlanRequest.displayName = 'proto.clustercontroller.ApplyNodePlanRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.ApplyNodePlanResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.ApplyNodePlanResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.ApplyNodePlanResponse.displayName = 'proto.clustercontroller.ApplyNodePlanResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.ApplyNodePlanV1Request = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.ApplyNodePlanV1Request, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.ApplyNodePlanV1Request.displayName = 'proto.clustercontroller.ApplyNodePlanV1Request';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.ApplyNodePlanV1Response = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.ApplyNodePlanV1Response, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.ApplyNodePlanV1Response.displayName = 'proto.clustercontroller.ApplyNodePlanV1Response';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.ArtifactRef = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.ArtifactRef, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.ArtifactRef.displayName = 'proto.clustercontroller.ArtifactRef';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.UnitAction = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.UnitAction, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.UnitAction.displayName = 'proto.clustercontroller.UnitAction';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.NodePlan = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.clustercontroller.NodePlan.repeatedFields_, null);
};
goog.inherits(proto.clustercontroller.NodePlan, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.NodePlan.displayName = 'proto.clustercontroller.NodePlan';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.UpgradeGlobularRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.UpgradeGlobularRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.UpgradeGlobularRequest.displayName = 'proto.clustercontroller.UpgradeGlobularRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.UpgradeGlobularResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.UpgradeGlobularResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.UpgradeGlobularResponse.displayName = 'proto.clustercontroller.UpgradeGlobularResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.GetNodePlanRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.GetNodePlanRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.GetNodePlanRequest.displayName = 'proto.clustercontroller.GetNodePlanRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.GetNodePlanResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.GetNodePlanResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.GetNodePlanResponse.displayName = 'proto.clustercontroller.GetNodePlanResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.GetNodePlanV1Request = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.GetNodePlanV1Request, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.GetNodePlanV1Request.displayName = 'proto.clustercontroller.GetNodePlanV1Request';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.GetNodePlanV1Response = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.GetNodePlanV1Response, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.GetNodePlanV1Response.displayName = 'proto.clustercontroller.GetNodePlanV1Response';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.ReconcileNodeV1Request = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.ReconcileNodeV1Request, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.ReconcileNodeV1Request.displayName = 'proto.clustercontroller.ReconcileNodeV1Request';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.ReconcileNodeV1Response = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.ReconcileNodeV1Response, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.ReconcileNodeV1Response.displayName = 'proto.clustercontroller.ReconcileNodeV1Response';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.WatchNodePlanStatusV1Request = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.WatchNodePlanStatusV1Request, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.WatchNodePlanStatusV1Request.displayName = 'proto.clustercontroller.WatchNodePlanStatusV1Request';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.StartApplyRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.StartApplyRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.StartApplyRequest.displayName = 'proto.clustercontroller.StartApplyRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.StartApplyResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.StartApplyResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.StartApplyResponse.displayName = 'proto.clustercontroller.StartApplyResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.OperationEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.OperationEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.OperationEvent.displayName = 'proto.clustercontroller.OperationEvent';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.CompleteOperationRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.CompleteOperationRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.CompleteOperationRequest.displayName = 'proto.clustercontroller.CompleteOperationRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.CompleteOperationResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.CompleteOperationResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.CompleteOperationResponse.displayName = 'proto.clustercontroller.CompleteOperationResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.NodeUnitStatus = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.NodeUnitStatus, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.NodeUnitStatus.displayName = 'proto.clustercontroller.NodeUnitStatus';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.NodeStatus = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.clustercontroller.NodeStatus.repeatedFields_, null);
};
goog.inherits(proto.clustercontroller.NodeStatus, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.NodeStatus.displayName = 'proto.clustercontroller.NodeStatus';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.ReportNodeStatusRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.ReportNodeStatusRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.ReportNodeStatusRequest.displayName = 'proto.clustercontroller.ReportNodeStatusRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.ReportNodeStatusResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.ReportNodeStatusResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.ReportNodeStatusResponse.displayName = 'proto.clustercontroller.ReportNodeStatusResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.WatchOperationsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.WatchOperationsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.WatchOperationsRequest.displayName = 'proto.clustercontroller.WatchOperationsRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.DesiredNetwork = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.clustercontroller.DesiredNetwork.repeatedFields_, null);
};
goog.inherits(proto.clustercontroller.DesiredNetwork, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.DesiredNetwork.displayName = 'proto.clustercontroller.DesiredNetwork';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.GetClusterHealthV1Request = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.GetClusterHealthV1Request, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.GetClusterHealthV1Request.displayName = 'proto.clustercontroller.GetClusterHealthV1Request';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.NodeHealth = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.NodeHealth, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.NodeHealth.displayName = 'proto.clustercontroller.NodeHealth';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.ServiceSummary = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.clustercontroller.ServiceSummary, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.ServiceSummary.displayName = 'proto.clustercontroller.ServiceSummary';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.clustercontroller.GetClusterHealthV1Response = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.clustercontroller.GetClusterHealthV1Response.repeatedFields_, null);
};
goog.inherits(proto.clustercontroller.GetClusterHealthV1Response, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.clustercontroller.GetClusterHealthV1Response.displayName = 'proto.clustercontroller.GetClusterHealthV1Response';
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
proto.clustercontroller.ClusterInfo.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.ClusterInfo.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.ClusterInfo} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ClusterInfo.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.ClusterInfo}
 */
proto.clustercontroller.ClusterInfo.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.ClusterInfo;
  return proto.clustercontroller.ClusterInfo.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.ClusterInfo} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.ClusterInfo}
 */
proto.clustercontroller.ClusterInfo.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.ClusterInfo.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.ClusterInfo.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.ClusterInfo} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ClusterInfo.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.ClusterInfo.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.ClusterInfo} returns this
 */
proto.clustercontroller.ClusterInfo.prototype.setClusterId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string cluster_domain = 2;
 * @return {string}
 */
proto.clustercontroller.ClusterInfo.prototype.getClusterDomain = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.ClusterInfo} returns this
 */
proto.clustercontroller.ClusterInfo.prototype.setClusterDomain = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional google.protobuf.Timestamp created_at = 3;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.clustercontroller.ClusterInfo.prototype.getCreatedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 3));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.clustercontroller.ClusterInfo} returns this
*/
proto.clustercontroller.ClusterInfo.prototype.setCreatedAt = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.clustercontroller.ClusterInfo} returns this
 */
proto.clustercontroller.ClusterInfo.prototype.clearCreatedAt = function() {
  return this.setCreatedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.clustercontroller.ClusterInfo.prototype.hasCreatedAt = function() {
  return jspb.Message.getField(this, 3) != null;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.clustercontroller.ClusterNetworkSpec.repeatedFields_ = [5];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.clustercontroller.ClusterNetworkSpec.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.ClusterNetworkSpec.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.ClusterNetworkSpec} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ClusterNetworkSpec.toObject = function(includeInstance, msg) {
  var f, obj = {
clusterDomain: jspb.Message.getFieldWithDefault(msg, 1, ""),
protocol: jspb.Message.getFieldWithDefault(msg, 2, ""),
portHttp: jspb.Message.getFieldWithDefault(msg, 3, 0),
portHttps: jspb.Message.getFieldWithDefault(msg, 4, 0),
alternateDomainsList: (f = jspb.Message.getRepeatedField(msg, 5)) == null ? undefined : f,
acmeEnabled: jspb.Message.getBooleanFieldWithDefault(msg, 6, false),
adminEmail: jspb.Message.getFieldWithDefault(msg, 7, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.clustercontroller.ClusterNetworkSpec}
 */
proto.clustercontroller.ClusterNetworkSpec.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.ClusterNetworkSpec;
  return proto.clustercontroller.ClusterNetworkSpec.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.ClusterNetworkSpec} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.ClusterNetworkSpec}
 */
proto.clustercontroller.ClusterNetworkSpec.deserializeBinaryFromReader = function(msg, reader) {
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
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.clustercontroller.ClusterNetworkSpec.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.ClusterNetworkSpec.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.ClusterNetworkSpec} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ClusterNetworkSpec.serializeBinaryToWriter = function(message, writer) {
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
};


/**
 * optional string cluster_domain = 1;
 * @return {string}
 */
proto.clustercontroller.ClusterNetworkSpec.prototype.getClusterDomain = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.ClusterNetworkSpec} returns this
 */
proto.clustercontroller.ClusterNetworkSpec.prototype.setClusterDomain = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string protocol = 2;
 * @return {string}
 */
proto.clustercontroller.ClusterNetworkSpec.prototype.getProtocol = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.ClusterNetworkSpec} returns this
 */
proto.clustercontroller.ClusterNetworkSpec.prototype.setProtocol = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional uint32 port_http = 3;
 * @return {number}
 */
proto.clustercontroller.ClusterNetworkSpec.prototype.getPortHttp = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.clustercontroller.ClusterNetworkSpec} returns this
 */
proto.clustercontroller.ClusterNetworkSpec.prototype.setPortHttp = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional uint32 port_https = 4;
 * @return {number}
 */
proto.clustercontroller.ClusterNetworkSpec.prototype.getPortHttps = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.clustercontroller.ClusterNetworkSpec} returns this
 */
proto.clustercontroller.ClusterNetworkSpec.prototype.setPortHttps = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * repeated string alternate_domains = 5;
 * @return {!Array<string>}
 */
proto.clustercontroller.ClusterNetworkSpec.prototype.getAlternateDomainsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 5));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.clustercontroller.ClusterNetworkSpec} returns this
 */
proto.clustercontroller.ClusterNetworkSpec.prototype.setAlternateDomainsList = function(value) {
  return jspb.Message.setField(this, 5, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.clustercontroller.ClusterNetworkSpec} returns this
 */
proto.clustercontroller.ClusterNetworkSpec.prototype.addAlternateDomains = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 5, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.clustercontroller.ClusterNetworkSpec} returns this
 */
proto.clustercontroller.ClusterNetworkSpec.prototype.clearAlternateDomainsList = function() {
  return this.setAlternateDomainsList([]);
};


/**
 * optional bool acme_enabled = 6;
 * @return {boolean}
 */
proto.clustercontroller.ClusterNetworkSpec.prototype.getAcmeEnabled = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 6, false));
};


/**
 * @param {boolean} value
 * @return {!proto.clustercontroller.ClusterNetworkSpec} returns this
 */
proto.clustercontroller.ClusterNetworkSpec.prototype.setAcmeEnabled = function(value) {
  return jspb.Message.setProto3BooleanField(this, 6, value);
};


/**
 * optional string admin_email = 7;
 * @return {string}
 */
proto.clustercontroller.ClusterNetworkSpec.prototype.getAdminEmail = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.ClusterNetworkSpec} returns this
 */
proto.clustercontroller.ClusterNetworkSpec.prototype.setAdminEmail = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.clustercontroller.NodeIdentity.repeatedFields_ = [3];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.clustercontroller.NodeIdentity.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.NodeIdentity.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.NodeIdentity} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.NodeIdentity.toObject = function(includeInstance, msg) {
  var f, obj = {
hostname: jspb.Message.getFieldWithDefault(msg, 1, ""),
domain: jspb.Message.getFieldWithDefault(msg, 2, ""),
ipsList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f,
os: jspb.Message.getFieldWithDefault(msg, 4, ""),
arch: jspb.Message.getFieldWithDefault(msg, 5, ""),
agentVersion: jspb.Message.getFieldWithDefault(msg, 6, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.clustercontroller.NodeIdentity}
 */
proto.clustercontroller.NodeIdentity.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.NodeIdentity;
  return proto.clustercontroller.NodeIdentity.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.NodeIdentity} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.NodeIdentity}
 */
proto.clustercontroller.NodeIdentity.deserializeBinaryFromReader = function(msg, reader) {
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
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.clustercontroller.NodeIdentity.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.NodeIdentity.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.NodeIdentity} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.NodeIdentity.serializeBinaryToWriter = function(message, writer) {
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
};


/**
 * optional string hostname = 1;
 * @return {string}
 */
proto.clustercontroller.NodeIdentity.prototype.getHostname = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeIdentity} returns this
 */
proto.clustercontroller.NodeIdentity.prototype.setHostname = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string domain = 2;
 * @return {string}
 */
proto.clustercontroller.NodeIdentity.prototype.getDomain = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeIdentity} returns this
 */
proto.clustercontroller.NodeIdentity.prototype.setDomain = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * repeated string ips = 3;
 * @return {!Array<string>}
 */
proto.clustercontroller.NodeIdentity.prototype.getIpsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.clustercontroller.NodeIdentity} returns this
 */
proto.clustercontroller.NodeIdentity.prototype.setIpsList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.clustercontroller.NodeIdentity} returns this
 */
proto.clustercontroller.NodeIdentity.prototype.addIps = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.clustercontroller.NodeIdentity} returns this
 */
proto.clustercontroller.NodeIdentity.prototype.clearIpsList = function() {
  return this.setIpsList([]);
};


/**
 * optional string os = 4;
 * @return {string}
 */
proto.clustercontroller.NodeIdentity.prototype.getOs = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeIdentity} returns this
 */
proto.clustercontroller.NodeIdentity.prototype.setOs = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string arch = 5;
 * @return {string}
 */
proto.clustercontroller.NodeIdentity.prototype.getArch = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeIdentity} returns this
 */
proto.clustercontroller.NodeIdentity.prototype.setArch = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string agent_version = 6;
 * @return {string}
 */
proto.clustercontroller.NodeIdentity.prototype.getAgentVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeIdentity} returns this
 */
proto.clustercontroller.NodeIdentity.prototype.setAgentVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.clustercontroller.NodeRecord.repeatedFields_ = [5];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.clustercontroller.NodeRecord.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.NodeRecord.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.NodeRecord} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.NodeRecord.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
identity: (f = msg.getIdentity()) && proto.clustercontroller.NodeIdentity.toObject(includeInstance, f),
lastSeen: (f = msg.getLastSeen()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
status: jspb.Message.getFieldWithDefault(msg, 4, ""),
profilesList: (f = jspb.Message.getRepeatedField(msg, 5)) == null ? undefined : f,
metadataMap: (f = msg.getMetadataMap()) ? f.toObject(includeInstance, undefined) : [],
agentEndpoint: jspb.Message.getFieldWithDefault(msg, 7, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.clustercontroller.NodeRecord}
 */
proto.clustercontroller.NodeRecord.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.NodeRecord;
  return proto.clustercontroller.NodeRecord.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.NodeRecord} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.NodeRecord}
 */
proto.clustercontroller.NodeRecord.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = new proto.clustercontroller.NodeIdentity;
      reader.readMessage(value,proto.clustercontroller.NodeIdentity.deserializeBinaryFromReader);
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
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.clustercontroller.NodeRecord.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.NodeRecord.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.NodeRecord} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.NodeRecord.serializeBinaryToWriter = function(message, writer) {
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
      proto.clustercontroller.NodeIdentity.serializeBinaryToWriter
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
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.clustercontroller.NodeRecord.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeRecord} returns this
 */
proto.clustercontroller.NodeRecord.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional NodeIdentity identity = 2;
 * @return {?proto.clustercontroller.NodeIdentity}
 */
proto.clustercontroller.NodeRecord.prototype.getIdentity = function() {
  return /** @type{?proto.clustercontroller.NodeIdentity} */ (
    jspb.Message.getWrapperField(this, proto.clustercontroller.NodeIdentity, 2));
};


/**
 * @param {?proto.clustercontroller.NodeIdentity|undefined} value
 * @return {!proto.clustercontroller.NodeRecord} returns this
*/
proto.clustercontroller.NodeRecord.prototype.setIdentity = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.clustercontroller.NodeRecord} returns this
 */
proto.clustercontroller.NodeRecord.prototype.clearIdentity = function() {
  return this.setIdentity(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.clustercontroller.NodeRecord.prototype.hasIdentity = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional google.protobuf.Timestamp last_seen = 3;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.clustercontroller.NodeRecord.prototype.getLastSeen = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 3));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.clustercontroller.NodeRecord} returns this
*/
proto.clustercontroller.NodeRecord.prototype.setLastSeen = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.clustercontroller.NodeRecord} returns this
 */
proto.clustercontroller.NodeRecord.prototype.clearLastSeen = function() {
  return this.setLastSeen(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.clustercontroller.NodeRecord.prototype.hasLastSeen = function() {
  return jspb.Message.getField(this, 3) != null;
};


/**
 * optional string status = 4;
 * @return {string}
 */
proto.clustercontroller.NodeRecord.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeRecord} returns this
 */
proto.clustercontroller.NodeRecord.prototype.setStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * repeated string profiles = 5;
 * @return {!Array<string>}
 */
proto.clustercontroller.NodeRecord.prototype.getProfilesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 5));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.clustercontroller.NodeRecord} returns this
 */
proto.clustercontroller.NodeRecord.prototype.setProfilesList = function(value) {
  return jspb.Message.setField(this, 5, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.clustercontroller.NodeRecord} returns this
 */
proto.clustercontroller.NodeRecord.prototype.addProfiles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 5, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.clustercontroller.NodeRecord} returns this
 */
proto.clustercontroller.NodeRecord.prototype.clearProfilesList = function() {
  return this.setProfilesList([]);
};


/**
 * map<string, string> metadata = 6;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.clustercontroller.NodeRecord.prototype.getMetadataMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 6, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.clustercontroller.NodeRecord} returns this
 */
proto.clustercontroller.NodeRecord.prototype.clearMetadataMap = function() {
  this.getMetadataMap().clear();
  return this;
};


/**
 * optional string agent_endpoint = 7;
 * @return {string}
 */
proto.clustercontroller.NodeRecord.prototype.getAgentEndpoint = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeRecord} returns this
 */
proto.clustercontroller.NodeRecord.prototype.setAgentEndpoint = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
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
proto.clustercontroller.CreateJoinTokenRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.CreateJoinTokenRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.CreateJoinTokenRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.CreateJoinTokenRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.CreateJoinTokenRequest}
 */
proto.clustercontroller.CreateJoinTokenRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.CreateJoinTokenRequest;
  return proto.clustercontroller.CreateJoinTokenRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.CreateJoinTokenRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.CreateJoinTokenRequest}
 */
proto.clustercontroller.CreateJoinTokenRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.CreateJoinTokenRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.CreateJoinTokenRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.CreateJoinTokenRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.CreateJoinTokenRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.CreateJoinTokenRequest.prototype.getExpiresAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 1));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.clustercontroller.CreateJoinTokenRequest} returns this
*/
proto.clustercontroller.CreateJoinTokenRequest.prototype.setExpiresAt = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.clustercontroller.CreateJoinTokenRequest} returns this
 */
proto.clustercontroller.CreateJoinTokenRequest.prototype.clearExpiresAt = function() {
  return this.setExpiresAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.clustercontroller.CreateJoinTokenRequest.prototype.hasExpiresAt = function() {
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
proto.clustercontroller.CreateJoinTokenResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.CreateJoinTokenResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.CreateJoinTokenResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.CreateJoinTokenResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.CreateJoinTokenResponse}
 */
proto.clustercontroller.CreateJoinTokenResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.CreateJoinTokenResponse;
  return proto.clustercontroller.CreateJoinTokenResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.CreateJoinTokenResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.CreateJoinTokenResponse}
 */
proto.clustercontroller.CreateJoinTokenResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.CreateJoinTokenResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.CreateJoinTokenResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.CreateJoinTokenResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.CreateJoinTokenResponse.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.CreateJoinTokenResponse.prototype.getJoinToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.CreateJoinTokenResponse} returns this
 */
proto.clustercontroller.CreateJoinTokenResponse.prototype.setJoinToken = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional google.protobuf.Timestamp expires_at = 2;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.clustercontroller.CreateJoinTokenResponse.prototype.getExpiresAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 2));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.clustercontroller.CreateJoinTokenResponse} returns this
*/
proto.clustercontroller.CreateJoinTokenResponse.prototype.setExpiresAt = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.clustercontroller.CreateJoinTokenResponse} returns this
 */
proto.clustercontroller.CreateJoinTokenResponse.prototype.clearExpiresAt = function() {
  return this.setExpiresAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.clustercontroller.CreateJoinTokenResponse.prototype.hasExpiresAt = function() {
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
proto.clustercontroller.RequestJoinRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.RequestJoinRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.RequestJoinRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.RequestJoinRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
joinToken: jspb.Message.getFieldWithDefault(msg, 1, ""),
identity: (f = msg.getIdentity()) && proto.clustercontroller.NodeIdentity.toObject(includeInstance, f),
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
 * @return {!proto.clustercontroller.RequestJoinRequest}
 */
proto.clustercontroller.RequestJoinRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.RequestJoinRequest;
  return proto.clustercontroller.RequestJoinRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.RequestJoinRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.RequestJoinRequest}
 */
proto.clustercontroller.RequestJoinRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = new proto.clustercontroller.NodeIdentity;
      reader.readMessage(value,proto.clustercontroller.NodeIdentity.deserializeBinaryFromReader);
      msg.setIdentity(value);
      break;
    case 3:
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
proto.clustercontroller.RequestJoinRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.RequestJoinRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.RequestJoinRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.RequestJoinRequest.serializeBinaryToWriter = function(message, writer) {
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
      proto.clustercontroller.NodeIdentity.serializeBinaryToWriter
    );
  }
  f = message.getLabelsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(3, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
};


/**
 * optional string join_token = 1;
 * @return {string}
 */
proto.clustercontroller.RequestJoinRequest.prototype.getJoinToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.RequestJoinRequest} returns this
 */
proto.clustercontroller.RequestJoinRequest.prototype.setJoinToken = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional NodeIdentity identity = 2;
 * @return {?proto.clustercontroller.NodeIdentity}
 */
proto.clustercontroller.RequestJoinRequest.prototype.getIdentity = function() {
  return /** @type{?proto.clustercontroller.NodeIdentity} */ (
    jspb.Message.getWrapperField(this, proto.clustercontroller.NodeIdentity, 2));
};


/**
 * @param {?proto.clustercontroller.NodeIdentity|undefined} value
 * @return {!proto.clustercontroller.RequestJoinRequest} returns this
*/
proto.clustercontroller.RequestJoinRequest.prototype.setIdentity = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.clustercontroller.RequestJoinRequest} returns this
 */
proto.clustercontroller.RequestJoinRequest.prototype.clearIdentity = function() {
  return this.setIdentity(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.clustercontroller.RequestJoinRequest.prototype.hasIdentity = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * map<string, string> labels = 3;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.clustercontroller.RequestJoinRequest.prototype.getLabelsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 3, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.clustercontroller.RequestJoinRequest} returns this
 */
proto.clustercontroller.RequestJoinRequest.prototype.clearLabelsMap = function() {
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
proto.clustercontroller.RequestJoinResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.RequestJoinResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.RequestJoinResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.RequestJoinResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.RequestJoinResponse}
 */
proto.clustercontroller.RequestJoinResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.RequestJoinResponse;
  return proto.clustercontroller.RequestJoinResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.RequestJoinResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.RequestJoinResponse}
 */
proto.clustercontroller.RequestJoinResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.RequestJoinResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.RequestJoinResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.RequestJoinResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.RequestJoinResponse.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.RequestJoinResponse.prototype.getRequestId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.RequestJoinResponse} returns this
 */
proto.clustercontroller.RequestJoinResponse.prototype.setRequestId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string status = 2;
 * @return {string}
 */
proto.clustercontroller.RequestJoinResponse.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.RequestJoinResponse} returns this
 */
proto.clustercontroller.RequestJoinResponse.prototype.setStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string message = 3;
 * @return {string}
 */
proto.clustercontroller.RequestJoinResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.RequestJoinResponse} returns this
 */
proto.clustercontroller.RequestJoinResponse.prototype.setMessage = function(value) {
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
proto.clustercontroller.GetJoinRequestStatusRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.GetJoinRequestStatusRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.GetJoinRequestStatusRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.GetJoinRequestStatusRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.GetJoinRequestStatusRequest}
 */
proto.clustercontroller.GetJoinRequestStatusRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.GetJoinRequestStatusRequest;
  return proto.clustercontroller.GetJoinRequestStatusRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.GetJoinRequestStatusRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.GetJoinRequestStatusRequest}
 */
proto.clustercontroller.GetJoinRequestStatusRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.GetJoinRequestStatusRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.GetJoinRequestStatusRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.GetJoinRequestStatusRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.GetJoinRequestStatusRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.GetJoinRequestStatusRequest.prototype.getRequestId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.GetJoinRequestStatusRequest} returns this
 */
proto.clustercontroller.GetJoinRequestStatusRequest.prototype.setRequestId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.clustercontroller.GetJoinRequestStatusResponse.repeatedFields_ = [3];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.clustercontroller.GetJoinRequestStatusResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.GetJoinRequestStatusResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.GetJoinRequestStatusResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.GetJoinRequestStatusResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
status: jspb.Message.getFieldWithDefault(msg, 1, ""),
nodeId: jspb.Message.getFieldWithDefault(msg, 2, ""),
profilesList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f,
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
 * @return {!proto.clustercontroller.GetJoinRequestStatusResponse}
 */
proto.clustercontroller.GetJoinRequestStatusResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.GetJoinRequestStatusResponse;
  return proto.clustercontroller.GetJoinRequestStatusResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.GetJoinRequestStatusResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.GetJoinRequestStatusResponse}
 */
proto.clustercontroller.GetJoinRequestStatusResponse.deserializeBinaryFromReader = function(msg, reader) {
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
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.clustercontroller.GetJoinRequestStatusResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.GetJoinRequestStatusResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.GetJoinRequestStatusResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.GetJoinRequestStatusResponse.serializeBinaryToWriter = function(message, writer) {
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
};


/**
 * optional string status = 1;
 * @return {string}
 */
proto.clustercontroller.GetJoinRequestStatusResponse.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.GetJoinRequestStatusResponse} returns this
 */
proto.clustercontroller.GetJoinRequestStatusResponse.prototype.setStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string node_id = 2;
 * @return {string}
 */
proto.clustercontroller.GetJoinRequestStatusResponse.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.GetJoinRequestStatusResponse} returns this
 */
proto.clustercontroller.GetJoinRequestStatusResponse.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * repeated string profiles = 3;
 * @return {!Array<string>}
 */
proto.clustercontroller.GetJoinRequestStatusResponse.prototype.getProfilesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.clustercontroller.GetJoinRequestStatusResponse} returns this
 */
proto.clustercontroller.GetJoinRequestStatusResponse.prototype.setProfilesList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.clustercontroller.GetJoinRequestStatusResponse} returns this
 */
proto.clustercontroller.GetJoinRequestStatusResponse.prototype.addProfiles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.clustercontroller.GetJoinRequestStatusResponse} returns this
 */
proto.clustercontroller.GetJoinRequestStatusResponse.prototype.clearProfilesList = function() {
  return this.setProfilesList([]);
};


/**
 * optional string message = 4;
 * @return {string}
 */
proto.clustercontroller.GetJoinRequestStatusResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.GetJoinRequestStatusResponse} returns this
 */
proto.clustercontroller.GetJoinRequestStatusResponse.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.clustercontroller.JoinRequestRecord.repeatedFields_ = [5];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.clustercontroller.JoinRequestRecord.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.JoinRequestRecord.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.JoinRequestRecord} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.JoinRequestRecord.toObject = function(includeInstance, msg) {
  var f, obj = {
requestId: jspb.Message.getFieldWithDefault(msg, 1, ""),
identity: (f = msg.getIdentity()) && proto.clustercontroller.NodeIdentity.toObject(includeInstance, f),
status: jspb.Message.getFieldWithDefault(msg, 3, ""),
message: jspb.Message.getFieldWithDefault(msg, 4, ""),
profilesList: (f = jspb.Message.getRepeatedField(msg, 5)) == null ? undefined : f,
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
 * @return {!proto.clustercontroller.JoinRequestRecord}
 */
proto.clustercontroller.JoinRequestRecord.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.JoinRequestRecord;
  return proto.clustercontroller.JoinRequestRecord.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.JoinRequestRecord} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.JoinRequestRecord}
 */
proto.clustercontroller.JoinRequestRecord.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = new proto.clustercontroller.NodeIdentity;
      reader.readMessage(value,proto.clustercontroller.NodeIdentity.deserializeBinaryFromReader);
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
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.clustercontroller.JoinRequestRecord.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.JoinRequestRecord.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.JoinRequestRecord} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.JoinRequestRecord.serializeBinaryToWriter = function(message, writer) {
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
      proto.clustercontroller.NodeIdentity.serializeBinaryToWriter
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
};


/**
 * optional string request_id = 1;
 * @return {string}
 */
proto.clustercontroller.JoinRequestRecord.prototype.getRequestId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.JoinRequestRecord} returns this
 */
proto.clustercontroller.JoinRequestRecord.prototype.setRequestId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional NodeIdentity identity = 2;
 * @return {?proto.clustercontroller.NodeIdentity}
 */
proto.clustercontroller.JoinRequestRecord.prototype.getIdentity = function() {
  return /** @type{?proto.clustercontroller.NodeIdentity} */ (
    jspb.Message.getWrapperField(this, proto.clustercontroller.NodeIdentity, 2));
};


/**
 * @param {?proto.clustercontroller.NodeIdentity|undefined} value
 * @return {!proto.clustercontroller.JoinRequestRecord} returns this
*/
proto.clustercontroller.JoinRequestRecord.prototype.setIdentity = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.clustercontroller.JoinRequestRecord} returns this
 */
proto.clustercontroller.JoinRequestRecord.prototype.clearIdentity = function() {
  return this.setIdentity(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.clustercontroller.JoinRequestRecord.prototype.hasIdentity = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional string status = 3;
 * @return {string}
 */
proto.clustercontroller.JoinRequestRecord.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.JoinRequestRecord} returns this
 */
proto.clustercontroller.JoinRequestRecord.prototype.setStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string message = 4;
 * @return {string}
 */
proto.clustercontroller.JoinRequestRecord.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.JoinRequestRecord} returns this
 */
proto.clustercontroller.JoinRequestRecord.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * repeated string profiles = 5;
 * @return {!Array<string>}
 */
proto.clustercontroller.JoinRequestRecord.prototype.getProfilesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 5));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.clustercontroller.JoinRequestRecord} returns this
 */
proto.clustercontroller.JoinRequestRecord.prototype.setProfilesList = function(value) {
  return jspb.Message.setField(this, 5, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.clustercontroller.JoinRequestRecord} returns this
 */
proto.clustercontroller.JoinRequestRecord.prototype.addProfiles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 5, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.clustercontroller.JoinRequestRecord} returns this
 */
proto.clustercontroller.JoinRequestRecord.prototype.clearProfilesList = function() {
  return this.setProfilesList([]);
};


/**
 * map<string, string> metadata = 6;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.clustercontroller.JoinRequestRecord.prototype.getMetadataMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 6, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.clustercontroller.JoinRequestRecord} returns this
 */
proto.clustercontroller.JoinRequestRecord.prototype.clearMetadataMap = function() {
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
proto.clustercontroller.ListJoinRequestsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.ListJoinRequestsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.ListJoinRequestsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ListJoinRequestsRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.ListJoinRequestsRequest}
 */
proto.clustercontroller.ListJoinRequestsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.ListJoinRequestsRequest;
  return proto.clustercontroller.ListJoinRequestsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.ListJoinRequestsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.ListJoinRequestsRequest}
 */
proto.clustercontroller.ListJoinRequestsRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.ListJoinRequestsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.ListJoinRequestsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.ListJoinRequestsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ListJoinRequestsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.clustercontroller.ListJoinRequestsResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.clustercontroller.ListJoinRequestsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.ListJoinRequestsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.ListJoinRequestsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ListJoinRequestsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
pendingList: jspb.Message.toObjectList(msg.getPendingList(),
    proto.clustercontroller.JoinRequestRecord.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.clustercontroller.ListJoinRequestsResponse}
 */
proto.clustercontroller.ListJoinRequestsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.ListJoinRequestsResponse;
  return proto.clustercontroller.ListJoinRequestsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.ListJoinRequestsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.ListJoinRequestsResponse}
 */
proto.clustercontroller.ListJoinRequestsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.clustercontroller.JoinRequestRecord;
      reader.readMessage(value,proto.clustercontroller.JoinRequestRecord.deserializeBinaryFromReader);
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
proto.clustercontroller.ListJoinRequestsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.ListJoinRequestsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.ListJoinRequestsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ListJoinRequestsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPendingList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.clustercontroller.JoinRequestRecord.serializeBinaryToWriter
    );
  }
};


/**
 * repeated JoinRequestRecord pending = 1;
 * @return {!Array<!proto.clustercontroller.JoinRequestRecord>}
 */
proto.clustercontroller.ListJoinRequestsResponse.prototype.getPendingList = function() {
  return /** @type{!Array<!proto.clustercontroller.JoinRequestRecord>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.clustercontroller.JoinRequestRecord, 1));
};


/**
 * @param {!Array<!proto.clustercontroller.JoinRequestRecord>} value
 * @return {!proto.clustercontroller.ListJoinRequestsResponse} returns this
*/
proto.clustercontroller.ListJoinRequestsResponse.prototype.setPendingList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.clustercontroller.JoinRequestRecord=} opt_value
 * @param {number=} opt_index
 * @return {!proto.clustercontroller.JoinRequestRecord}
 */
proto.clustercontroller.ListJoinRequestsResponse.prototype.addPending = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.clustercontroller.JoinRequestRecord, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.clustercontroller.ListJoinRequestsResponse} returns this
 */
proto.clustercontroller.ListJoinRequestsResponse.prototype.clearPendingList = function() {
  return this.setPendingList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.clustercontroller.ApproveJoinRequest.repeatedFields_ = [3];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.clustercontroller.ApproveJoinRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.ApproveJoinRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.ApproveJoinRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ApproveJoinRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.ApproveJoinRequest}
 */
proto.clustercontroller.ApproveJoinRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.ApproveJoinRequest;
  return proto.clustercontroller.ApproveJoinRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.ApproveJoinRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.ApproveJoinRequest}
 */
proto.clustercontroller.ApproveJoinRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.ApproveJoinRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.ApproveJoinRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.ApproveJoinRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ApproveJoinRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.ApproveJoinRequest.prototype.getRequestId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.ApproveJoinRequest} returns this
 */
proto.clustercontroller.ApproveJoinRequest.prototype.setRequestId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string node_id = 2;
 * @return {string}
 */
proto.clustercontroller.ApproveJoinRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.ApproveJoinRequest} returns this
 */
proto.clustercontroller.ApproveJoinRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * repeated string profiles = 3;
 * @return {!Array<string>}
 */
proto.clustercontroller.ApproveJoinRequest.prototype.getProfilesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.clustercontroller.ApproveJoinRequest} returns this
 */
proto.clustercontroller.ApproveJoinRequest.prototype.setProfilesList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.clustercontroller.ApproveJoinRequest} returns this
 */
proto.clustercontroller.ApproveJoinRequest.prototype.addProfiles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.clustercontroller.ApproveJoinRequest} returns this
 */
proto.clustercontroller.ApproveJoinRequest.prototype.clearProfilesList = function() {
  return this.setProfilesList([]);
};


/**
 * map<string, string> metadata = 4;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.clustercontroller.ApproveJoinRequest.prototype.getMetadataMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 4, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.clustercontroller.ApproveJoinRequest} returns this
 */
proto.clustercontroller.ApproveJoinRequest.prototype.clearMetadataMap = function() {
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
proto.clustercontroller.ApproveJoinResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.ApproveJoinResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.ApproveJoinResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ApproveJoinResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.ApproveJoinResponse}
 */
proto.clustercontroller.ApproveJoinResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.ApproveJoinResponse;
  return proto.clustercontroller.ApproveJoinResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.ApproveJoinResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.ApproveJoinResponse}
 */
proto.clustercontroller.ApproveJoinResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.ApproveJoinResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.ApproveJoinResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.ApproveJoinResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ApproveJoinResponse.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.ApproveJoinResponse.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.ApproveJoinResponse} returns this
 */
proto.clustercontroller.ApproveJoinResponse.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string message = 2;
 * @return {string}
 */
proto.clustercontroller.ApproveJoinResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.ApproveJoinResponse} returns this
 */
proto.clustercontroller.ApproveJoinResponse.prototype.setMessage = function(value) {
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
proto.clustercontroller.RejectJoinRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.RejectJoinRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.RejectJoinRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.RejectJoinRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.RejectJoinRequest}
 */
proto.clustercontroller.RejectJoinRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.RejectJoinRequest;
  return proto.clustercontroller.RejectJoinRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.RejectJoinRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.RejectJoinRequest}
 */
proto.clustercontroller.RejectJoinRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.RejectJoinRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.RejectJoinRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.RejectJoinRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.RejectJoinRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.RejectJoinRequest.prototype.getRequestId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.RejectJoinRequest} returns this
 */
proto.clustercontroller.RejectJoinRequest.prototype.setRequestId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string node_id = 2;
 * @return {string}
 */
proto.clustercontroller.RejectJoinRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.RejectJoinRequest} returns this
 */
proto.clustercontroller.RejectJoinRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string reason = 3;
 * @return {string}
 */
proto.clustercontroller.RejectJoinRequest.prototype.getReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.RejectJoinRequest} returns this
 */
proto.clustercontroller.RejectJoinRequest.prototype.setReason = function(value) {
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
proto.clustercontroller.RejectJoinResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.RejectJoinResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.RejectJoinResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.RejectJoinResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.RejectJoinResponse}
 */
proto.clustercontroller.RejectJoinResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.RejectJoinResponse;
  return proto.clustercontroller.RejectJoinResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.RejectJoinResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.RejectJoinResponse}
 */
proto.clustercontroller.RejectJoinResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.RejectJoinResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.RejectJoinResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.RejectJoinResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.RejectJoinResponse.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.RejectJoinResponse.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.RejectJoinResponse} returns this
 */
proto.clustercontroller.RejectJoinResponse.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string message = 2;
 * @return {string}
 */
proto.clustercontroller.RejectJoinResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.RejectJoinResponse} returns this
 */
proto.clustercontroller.RejectJoinResponse.prototype.setMessage = function(value) {
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
proto.clustercontroller.ListNodesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.ListNodesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.ListNodesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ListNodesRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.ListNodesRequest}
 */
proto.clustercontroller.ListNodesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.ListNodesRequest;
  return proto.clustercontroller.ListNodesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.ListNodesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.ListNodesRequest}
 */
proto.clustercontroller.ListNodesRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.ListNodesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.ListNodesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.ListNodesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ListNodesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.clustercontroller.ListNodesResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.clustercontroller.ListNodesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.ListNodesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.ListNodesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ListNodesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
nodesList: jspb.Message.toObjectList(msg.getNodesList(),
    proto.clustercontroller.NodeRecord.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.clustercontroller.ListNodesResponse}
 */
proto.clustercontroller.ListNodesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.ListNodesResponse;
  return proto.clustercontroller.ListNodesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.ListNodesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.ListNodesResponse}
 */
proto.clustercontroller.ListNodesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.clustercontroller.NodeRecord;
      reader.readMessage(value,proto.clustercontroller.NodeRecord.deserializeBinaryFromReader);
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
proto.clustercontroller.ListNodesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.ListNodesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.ListNodesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ListNodesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.clustercontroller.NodeRecord.serializeBinaryToWriter
    );
  }
};


/**
 * repeated NodeRecord nodes = 1;
 * @return {!Array<!proto.clustercontroller.NodeRecord>}
 */
proto.clustercontroller.ListNodesResponse.prototype.getNodesList = function() {
  return /** @type{!Array<!proto.clustercontroller.NodeRecord>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.clustercontroller.NodeRecord, 1));
};


/**
 * @param {!Array<!proto.clustercontroller.NodeRecord>} value
 * @return {!proto.clustercontroller.ListNodesResponse} returns this
*/
proto.clustercontroller.ListNodesResponse.prototype.setNodesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.clustercontroller.NodeRecord=} opt_value
 * @param {number=} opt_index
 * @return {!proto.clustercontroller.NodeRecord}
 */
proto.clustercontroller.ListNodesResponse.prototype.addNodes = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.clustercontroller.NodeRecord, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.clustercontroller.ListNodesResponse} returns this
 */
proto.clustercontroller.ListNodesResponse.prototype.clearNodesList = function() {
  return this.setNodesList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.clustercontroller.SetNodeProfilesRequest.repeatedFields_ = [2];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.clustercontroller.SetNodeProfilesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.SetNodeProfilesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.SetNodeProfilesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.SetNodeProfilesRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.SetNodeProfilesRequest}
 */
proto.clustercontroller.SetNodeProfilesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.SetNodeProfilesRequest;
  return proto.clustercontroller.SetNodeProfilesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.SetNodeProfilesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.SetNodeProfilesRequest}
 */
proto.clustercontroller.SetNodeProfilesRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.SetNodeProfilesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.SetNodeProfilesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.SetNodeProfilesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.SetNodeProfilesRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.SetNodeProfilesRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.SetNodeProfilesRequest} returns this
 */
proto.clustercontroller.SetNodeProfilesRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * repeated string profiles = 2;
 * @return {!Array<string>}
 */
proto.clustercontroller.SetNodeProfilesRequest.prototype.getProfilesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.clustercontroller.SetNodeProfilesRequest} returns this
 */
proto.clustercontroller.SetNodeProfilesRequest.prototype.setProfilesList = function(value) {
  return jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.clustercontroller.SetNodeProfilesRequest} returns this
 */
proto.clustercontroller.SetNodeProfilesRequest.prototype.addProfiles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.clustercontroller.SetNodeProfilesRequest} returns this
 */
proto.clustercontroller.SetNodeProfilesRequest.prototype.clearProfilesList = function() {
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
proto.clustercontroller.SetNodeProfilesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.SetNodeProfilesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.SetNodeProfilesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.SetNodeProfilesResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.SetNodeProfilesResponse}
 */
proto.clustercontroller.SetNodeProfilesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.SetNodeProfilesResponse;
  return proto.clustercontroller.SetNodeProfilesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.SetNodeProfilesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.SetNodeProfilesResponse}
 */
proto.clustercontroller.SetNodeProfilesResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.SetNodeProfilesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.SetNodeProfilesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.SetNodeProfilesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.SetNodeProfilesResponse.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.SetNodeProfilesResponse.prototype.getOperationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.SetNodeProfilesResponse} returns this
 */
proto.clustercontroller.SetNodeProfilesResponse.prototype.setOperationId = function(value) {
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
proto.clustercontroller.RemoveNodeRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.RemoveNodeRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.RemoveNodeRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.RemoveNodeRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.RemoveNodeRequest}
 */
proto.clustercontroller.RemoveNodeRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.RemoveNodeRequest;
  return proto.clustercontroller.RemoveNodeRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.RemoveNodeRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.RemoveNodeRequest}
 */
proto.clustercontroller.RemoveNodeRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.RemoveNodeRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.RemoveNodeRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.RemoveNodeRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.RemoveNodeRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.RemoveNodeRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.RemoveNodeRequest} returns this
 */
proto.clustercontroller.RemoveNodeRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bool force = 2;
 * @return {boolean}
 */
proto.clustercontroller.RemoveNodeRequest.prototype.getForce = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.clustercontroller.RemoveNodeRequest} returns this
 */
proto.clustercontroller.RemoveNodeRequest.prototype.setForce = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * optional bool drain = 3;
 * @return {boolean}
 */
proto.clustercontroller.RemoveNodeRequest.prototype.getDrain = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.clustercontroller.RemoveNodeRequest} returns this
 */
proto.clustercontroller.RemoveNodeRequest.prototype.setDrain = function(value) {
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
proto.clustercontroller.RemoveNodeResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.RemoveNodeResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.RemoveNodeResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.RemoveNodeResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.RemoveNodeResponse}
 */
proto.clustercontroller.RemoveNodeResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.RemoveNodeResponse;
  return proto.clustercontroller.RemoveNodeResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.RemoveNodeResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.RemoveNodeResponse}
 */
proto.clustercontroller.RemoveNodeResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.RemoveNodeResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.RemoveNodeResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.RemoveNodeResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.RemoveNodeResponse.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.RemoveNodeResponse.prototype.getOperationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.RemoveNodeResponse} returns this
 */
proto.clustercontroller.RemoveNodeResponse.prototype.setOperationId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string message = 2;
 * @return {string}
 */
proto.clustercontroller.RemoveNodeResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.RemoveNodeResponse} returns this
 */
proto.clustercontroller.RemoveNodeResponse.prototype.setMessage = function(value) {
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
proto.clustercontroller.GetClusterHealthRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.GetClusterHealthRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.GetClusterHealthRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.GetClusterHealthRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.GetClusterHealthRequest}
 */
proto.clustercontroller.GetClusterHealthRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.GetClusterHealthRequest;
  return proto.clustercontroller.GetClusterHealthRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.GetClusterHealthRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.GetClusterHealthRequest}
 */
proto.clustercontroller.GetClusterHealthRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.GetClusterHealthRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.GetClusterHealthRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.GetClusterHealthRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.GetClusterHealthRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.clustercontroller.GetClusterHealthResponse.repeatedFields_ = [6];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.clustercontroller.GetClusterHealthResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.GetClusterHealthResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.GetClusterHealthResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.GetClusterHealthResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
status: jspb.Message.getFieldWithDefault(msg, 1, ""),
totalNodes: jspb.Message.getFieldWithDefault(msg, 2, 0),
healthyNodes: jspb.Message.getFieldWithDefault(msg, 3, 0),
unhealthyNodes: jspb.Message.getFieldWithDefault(msg, 4, 0),
unknownNodes: jspb.Message.getFieldWithDefault(msg, 5, 0),
nodeHealthList: jspb.Message.toObjectList(msg.getNodeHealthList(),
    proto.clustercontroller.NodeHealthStatus.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.clustercontroller.GetClusterHealthResponse}
 */
proto.clustercontroller.GetClusterHealthResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.GetClusterHealthResponse;
  return proto.clustercontroller.GetClusterHealthResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.GetClusterHealthResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.GetClusterHealthResponse}
 */
proto.clustercontroller.GetClusterHealthResponse.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = new proto.clustercontroller.NodeHealthStatus;
      reader.readMessage(value,proto.clustercontroller.NodeHealthStatus.deserializeBinaryFromReader);
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
proto.clustercontroller.GetClusterHealthResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.GetClusterHealthResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.GetClusterHealthResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.GetClusterHealthResponse.serializeBinaryToWriter = function(message, writer) {
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
      proto.clustercontroller.NodeHealthStatus.serializeBinaryToWriter
    );
  }
};


/**
 * optional string status = 1;
 * @return {string}
 */
proto.clustercontroller.GetClusterHealthResponse.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.GetClusterHealthResponse} returns this
 */
proto.clustercontroller.GetClusterHealthResponse.prototype.setStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int32 total_nodes = 2;
 * @return {number}
 */
proto.clustercontroller.GetClusterHealthResponse.prototype.getTotalNodes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.clustercontroller.GetClusterHealthResponse} returns this
 */
proto.clustercontroller.GetClusterHealthResponse.prototype.setTotalNodes = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional int32 healthy_nodes = 3;
 * @return {number}
 */
proto.clustercontroller.GetClusterHealthResponse.prototype.getHealthyNodes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.clustercontroller.GetClusterHealthResponse} returns this
 */
proto.clustercontroller.GetClusterHealthResponse.prototype.setHealthyNodes = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional int32 unhealthy_nodes = 4;
 * @return {number}
 */
proto.clustercontroller.GetClusterHealthResponse.prototype.getUnhealthyNodes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.clustercontroller.GetClusterHealthResponse} returns this
 */
proto.clustercontroller.GetClusterHealthResponse.prototype.setUnhealthyNodes = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional int32 unknown_nodes = 5;
 * @return {number}
 */
proto.clustercontroller.GetClusterHealthResponse.prototype.getUnknownNodes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.clustercontroller.GetClusterHealthResponse} returns this
 */
proto.clustercontroller.GetClusterHealthResponse.prototype.setUnknownNodes = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * repeated NodeHealthStatus node_health = 6;
 * @return {!Array<!proto.clustercontroller.NodeHealthStatus>}
 */
proto.clustercontroller.GetClusterHealthResponse.prototype.getNodeHealthList = function() {
  return /** @type{!Array<!proto.clustercontroller.NodeHealthStatus>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.clustercontroller.NodeHealthStatus, 6));
};


/**
 * @param {!Array<!proto.clustercontroller.NodeHealthStatus>} value
 * @return {!proto.clustercontroller.GetClusterHealthResponse} returns this
*/
proto.clustercontroller.GetClusterHealthResponse.prototype.setNodeHealthList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 6, value);
};


/**
 * @param {!proto.clustercontroller.NodeHealthStatus=} opt_value
 * @param {number=} opt_index
 * @return {!proto.clustercontroller.NodeHealthStatus}
 */
proto.clustercontroller.GetClusterHealthResponse.prototype.addNodeHealth = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 6, opt_value, proto.clustercontroller.NodeHealthStatus, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.clustercontroller.GetClusterHealthResponse} returns this
 */
proto.clustercontroller.GetClusterHealthResponse.prototype.clearNodeHealthList = function() {
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
proto.clustercontroller.NodeHealthStatus.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.NodeHealthStatus.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.NodeHealthStatus} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.NodeHealthStatus.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.NodeHealthStatus}
 */
proto.clustercontroller.NodeHealthStatus.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.NodeHealthStatus;
  return proto.clustercontroller.NodeHealthStatus.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.NodeHealthStatus} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.NodeHealthStatus}
 */
proto.clustercontroller.NodeHealthStatus.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.NodeHealthStatus.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.NodeHealthStatus.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.NodeHealthStatus} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.NodeHealthStatus.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.NodeHealthStatus.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeHealthStatus} returns this
 */
proto.clustercontroller.NodeHealthStatus.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string hostname = 2;
 * @return {string}
 */
proto.clustercontroller.NodeHealthStatus.prototype.getHostname = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeHealthStatus} returns this
 */
proto.clustercontroller.NodeHealthStatus.prototype.setHostname = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string status = 3;
 * @return {string}
 */
proto.clustercontroller.NodeHealthStatus.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeHealthStatus} returns this
 */
proto.clustercontroller.NodeHealthStatus.prototype.setStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string last_error = 4;
 * @return {string}
 */
proto.clustercontroller.NodeHealthStatus.prototype.getLastError = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeHealthStatus} returns this
 */
proto.clustercontroller.NodeHealthStatus.prototype.setLastError = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional google.protobuf.Timestamp last_seen = 5;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.clustercontroller.NodeHealthStatus.prototype.getLastSeen = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 5));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.clustercontroller.NodeHealthStatus} returns this
*/
proto.clustercontroller.NodeHealthStatus.prototype.setLastSeen = function(value) {
  return jspb.Message.setWrapperField(this, 5, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.clustercontroller.NodeHealthStatus} returns this
 */
proto.clustercontroller.NodeHealthStatus.prototype.clearLastSeen = function() {
  return this.setLastSeen(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.clustercontroller.NodeHealthStatus.prototype.hasLastSeen = function() {
  return jspb.Message.getField(this, 5) != null;
};


/**
 * optional int32 failed_checks = 6;
 * @return {number}
 */
proto.clustercontroller.NodeHealthStatus.prototype.getFailedChecks = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.clustercontroller.NodeHealthStatus} returns this
 */
proto.clustercontroller.NodeHealthStatus.prototype.setFailedChecks = function(value) {
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
proto.clustercontroller.UpdateClusterNetworkRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.UpdateClusterNetworkRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.UpdateClusterNetworkRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.UpdateClusterNetworkRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
spec: (f = msg.getSpec()) && proto.clustercontroller.ClusterNetworkSpec.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.clustercontroller.UpdateClusterNetworkRequest}
 */
proto.clustercontroller.UpdateClusterNetworkRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.UpdateClusterNetworkRequest;
  return proto.clustercontroller.UpdateClusterNetworkRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.UpdateClusterNetworkRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.UpdateClusterNetworkRequest}
 */
proto.clustercontroller.UpdateClusterNetworkRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.clustercontroller.ClusterNetworkSpec;
      reader.readMessage(value,proto.clustercontroller.ClusterNetworkSpec.deserializeBinaryFromReader);
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
proto.clustercontroller.UpdateClusterNetworkRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.UpdateClusterNetworkRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.UpdateClusterNetworkRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.UpdateClusterNetworkRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSpec();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.clustercontroller.ClusterNetworkSpec.serializeBinaryToWriter
    );
  }
};


/**
 * optional ClusterNetworkSpec spec = 1;
 * @return {?proto.clustercontroller.ClusterNetworkSpec}
 */
proto.clustercontroller.UpdateClusterNetworkRequest.prototype.getSpec = function() {
  return /** @type{?proto.clustercontroller.ClusterNetworkSpec} */ (
    jspb.Message.getWrapperField(this, proto.clustercontroller.ClusterNetworkSpec, 1));
};


/**
 * @param {?proto.clustercontroller.ClusterNetworkSpec|undefined} value
 * @return {!proto.clustercontroller.UpdateClusterNetworkRequest} returns this
*/
proto.clustercontroller.UpdateClusterNetworkRequest.prototype.setSpec = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.clustercontroller.UpdateClusterNetworkRequest} returns this
 */
proto.clustercontroller.UpdateClusterNetworkRequest.prototype.clearSpec = function() {
  return this.setSpec(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.clustercontroller.UpdateClusterNetworkRequest.prototype.hasSpec = function() {
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
proto.clustercontroller.UpdateClusterNetworkResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.UpdateClusterNetworkResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.UpdateClusterNetworkResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.UpdateClusterNetworkResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.UpdateClusterNetworkResponse}
 */
proto.clustercontroller.UpdateClusterNetworkResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.UpdateClusterNetworkResponse;
  return proto.clustercontroller.UpdateClusterNetworkResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.UpdateClusterNetworkResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.UpdateClusterNetworkResponse}
 */
proto.clustercontroller.UpdateClusterNetworkResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.UpdateClusterNetworkResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.UpdateClusterNetworkResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.UpdateClusterNetworkResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.UpdateClusterNetworkResponse.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.UpdateClusterNetworkResponse.prototype.getGeneration = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {number} value
 * @return {!proto.clustercontroller.UpdateClusterNetworkResponse} returns this
 */
proto.clustercontroller.UpdateClusterNetworkResponse.prototype.setGeneration = function(value) {
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
proto.clustercontroller.ApplyNodePlanRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.ApplyNodePlanRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.ApplyNodePlanRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ApplyNodePlanRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.ApplyNodePlanRequest}
 */
proto.clustercontroller.ApplyNodePlanRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.ApplyNodePlanRequest;
  return proto.clustercontroller.ApplyNodePlanRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.ApplyNodePlanRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.ApplyNodePlanRequest}
 */
proto.clustercontroller.ApplyNodePlanRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.ApplyNodePlanRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.ApplyNodePlanRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.ApplyNodePlanRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ApplyNodePlanRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.ApplyNodePlanRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.ApplyNodePlanRequest} returns this
 */
proto.clustercontroller.ApplyNodePlanRequest.prototype.setNodeId = function(value) {
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
proto.clustercontroller.ApplyNodePlanResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.ApplyNodePlanResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.ApplyNodePlanResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ApplyNodePlanResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.ApplyNodePlanResponse}
 */
proto.clustercontroller.ApplyNodePlanResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.ApplyNodePlanResponse;
  return proto.clustercontroller.ApplyNodePlanResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.ApplyNodePlanResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.ApplyNodePlanResponse}
 */
proto.clustercontroller.ApplyNodePlanResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.ApplyNodePlanResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.ApplyNodePlanResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.ApplyNodePlanResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ApplyNodePlanResponse.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.ApplyNodePlanResponse.prototype.getOperationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.ApplyNodePlanResponse} returns this
 */
proto.clustercontroller.ApplyNodePlanResponse.prototype.setOperationId = function(value) {
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
proto.clustercontroller.ApplyNodePlanV1Request.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.ApplyNodePlanV1Request.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.ApplyNodePlanV1Request} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ApplyNodePlanV1Request.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
plan: (f = msg.getPlan()) && plan_pb.NodePlan.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.clustercontroller.ApplyNodePlanV1Request}
 */
proto.clustercontroller.ApplyNodePlanV1Request.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.ApplyNodePlanV1Request;
  return proto.clustercontroller.ApplyNodePlanV1Request.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.ApplyNodePlanV1Request} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.ApplyNodePlanV1Request}
 */
proto.clustercontroller.ApplyNodePlanV1Request.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = new plan_pb.NodePlan;
      reader.readMessage(value,plan_pb.NodePlan.deserializeBinaryFromReader);
      msg.setPlan(value);
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
proto.clustercontroller.ApplyNodePlanV1Request.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.ApplyNodePlanV1Request.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.ApplyNodePlanV1Request} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ApplyNodePlanV1Request.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getPlan();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      plan_pb.NodePlan.serializeBinaryToWriter
    );
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.clustercontroller.ApplyNodePlanV1Request.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.ApplyNodePlanV1Request} returns this
 */
proto.clustercontroller.ApplyNodePlanV1Request.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional globular.plan.v1.NodePlan plan = 2;
 * @return {?proto.globular.plan.v1.NodePlan}
 */
proto.clustercontroller.ApplyNodePlanV1Request.prototype.getPlan = function() {
  return /** @type{?proto.globular.plan.v1.NodePlan} */ (
    jspb.Message.getWrapperField(this, plan_pb.NodePlan, 2));
};


/**
 * @param {?proto.globular.plan.v1.NodePlan|undefined} value
 * @return {!proto.clustercontroller.ApplyNodePlanV1Request} returns this
*/
proto.clustercontroller.ApplyNodePlanV1Request.prototype.setPlan = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.clustercontroller.ApplyNodePlanV1Request} returns this
 */
proto.clustercontroller.ApplyNodePlanV1Request.prototype.clearPlan = function() {
  return this.setPlan(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.clustercontroller.ApplyNodePlanV1Request.prototype.hasPlan = function() {
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
proto.clustercontroller.ApplyNodePlanV1Response.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.ApplyNodePlanV1Response.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.ApplyNodePlanV1Response} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ApplyNodePlanV1Response.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.ApplyNodePlanV1Response}
 */
proto.clustercontroller.ApplyNodePlanV1Response.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.ApplyNodePlanV1Response;
  return proto.clustercontroller.ApplyNodePlanV1Response.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.ApplyNodePlanV1Response} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.ApplyNodePlanV1Response}
 */
proto.clustercontroller.ApplyNodePlanV1Response.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.ApplyNodePlanV1Response.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.ApplyNodePlanV1Response.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.ApplyNodePlanV1Response} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ApplyNodePlanV1Response.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.ApplyNodePlanV1Response.prototype.getOperationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.ApplyNodePlanV1Response} returns this
 */
proto.clustercontroller.ApplyNodePlanV1Response.prototype.setOperationId = function(value) {
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
proto.clustercontroller.ArtifactRef.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.ArtifactRef.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.ArtifactRef} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ArtifactRef.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.ArtifactRef}
 */
proto.clustercontroller.ArtifactRef.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.ArtifactRef;
  return proto.clustercontroller.ArtifactRef.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.ArtifactRef} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.ArtifactRef}
 */
proto.clustercontroller.ArtifactRef.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.clustercontroller.ArtifactKind} */ (reader.readEnum());
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
proto.clustercontroller.ArtifactRef.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.ArtifactRef.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.ArtifactRef} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ArtifactRef.serializeBinaryToWriter = function(message, writer) {
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
 * @return {!proto.clustercontroller.ArtifactKind}
 */
proto.clustercontroller.ArtifactRef.prototype.getKind = function() {
  return /** @type {!proto.clustercontroller.ArtifactKind} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {!proto.clustercontroller.ArtifactKind} value
 * @return {!proto.clustercontroller.ArtifactRef} returns this
 */
proto.clustercontroller.ArtifactRef.prototype.setKind = function(value) {
  return jspb.Message.setProto3EnumField(this, 1, value);
};


/**
 * optional string name = 2;
 * @return {string}
 */
proto.clustercontroller.ArtifactRef.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.ArtifactRef} returns this
 */
proto.clustercontroller.ArtifactRef.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string publisher = 3;
 * @return {string}
 */
proto.clustercontroller.ArtifactRef.prototype.getPublisher = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.ArtifactRef} returns this
 */
proto.clustercontroller.ArtifactRef.prototype.setPublisher = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string version = 4;
 * @return {string}
 */
proto.clustercontroller.ArtifactRef.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.ArtifactRef} returns this
 */
proto.clustercontroller.ArtifactRef.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string discovery_id = 5;
 * @return {string}
 */
proto.clustercontroller.ArtifactRef.prototype.getDiscoveryId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.ArtifactRef} returns this
 */
proto.clustercontroller.ArtifactRef.prototype.setDiscoveryId = function(value) {
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
proto.clustercontroller.UnitAction.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.UnitAction.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.UnitAction} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.UnitAction.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.UnitAction}
 */
proto.clustercontroller.UnitAction.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.UnitAction;
  return proto.clustercontroller.UnitAction.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.UnitAction} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.UnitAction}
 */
proto.clustercontroller.UnitAction.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.UnitAction.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.UnitAction.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.UnitAction} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.UnitAction.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.UnitAction.prototype.getUnitName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.UnitAction} returns this
 */
proto.clustercontroller.UnitAction.prototype.setUnitName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string action = 2;
 * @return {string}
 */
proto.clustercontroller.UnitAction.prototype.getAction = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.UnitAction} returns this
 */
proto.clustercontroller.UnitAction.prototype.setAction = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.clustercontroller.NodePlan.repeatedFields_ = [2,3,4];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.clustercontroller.NodePlan.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.NodePlan.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.NodePlan} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.NodePlan.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
profilesList: (f = jspb.Message.getRepeatedField(msg, 2)) == null ? undefined : f,
ensureInstalledList: jspb.Message.toObjectList(msg.getEnsureInstalledList(),
    proto.clustercontroller.ArtifactRef.toObject, includeInstance),
unitActionsList: jspb.Message.toObjectList(msg.getUnitActionsList(),
    proto.clustercontroller.UnitAction.toObject, includeInstance),
renderedConfigMap: (f = msg.getRenderedConfigMap()) ? f.toObject(includeInstance, undefined) : []
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.clustercontroller.NodePlan}
 */
proto.clustercontroller.NodePlan.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.NodePlan;
  return proto.clustercontroller.NodePlan.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.NodePlan} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.NodePlan}
 */
proto.clustercontroller.NodePlan.deserializeBinaryFromReader = function(msg, reader) {
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
    case 3:
      var value = new proto.clustercontroller.ArtifactRef;
      reader.readMessage(value,proto.clustercontroller.ArtifactRef.deserializeBinaryFromReader);
      msg.addEnsureInstalled(value);
      break;
    case 4:
      var value = new proto.clustercontroller.UnitAction;
      reader.readMessage(value,proto.clustercontroller.UnitAction.deserializeBinaryFromReader);
      msg.addUnitActions(value);
      break;
    case 5:
      var value = msg.getRenderedConfigMap();
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
proto.clustercontroller.NodePlan.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.NodePlan.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.NodePlan} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.NodePlan.serializeBinaryToWriter = function(message, writer) {
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
  f = message.getEnsureInstalledList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      3,
      f,
      proto.clustercontroller.ArtifactRef.serializeBinaryToWriter
    );
  }
  f = message.getUnitActionsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      4,
      f,
      proto.clustercontroller.UnitAction.serializeBinaryToWriter
    );
  }
  f = message.getRenderedConfigMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(5, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.clustercontroller.NodePlan.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodePlan} returns this
 */
proto.clustercontroller.NodePlan.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * repeated string profiles = 2;
 * @return {!Array<string>}
 */
proto.clustercontroller.NodePlan.prototype.getProfilesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.clustercontroller.NodePlan} returns this
 */
proto.clustercontroller.NodePlan.prototype.setProfilesList = function(value) {
  return jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.clustercontroller.NodePlan} returns this
 */
proto.clustercontroller.NodePlan.prototype.addProfiles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.clustercontroller.NodePlan} returns this
 */
proto.clustercontroller.NodePlan.prototype.clearProfilesList = function() {
  return this.setProfilesList([]);
};


/**
 * repeated ArtifactRef ensure_installed = 3;
 * @return {!Array<!proto.clustercontroller.ArtifactRef>}
 */
proto.clustercontroller.NodePlan.prototype.getEnsureInstalledList = function() {
  return /** @type{!Array<!proto.clustercontroller.ArtifactRef>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.clustercontroller.ArtifactRef, 3));
};


/**
 * @param {!Array<!proto.clustercontroller.ArtifactRef>} value
 * @return {!proto.clustercontroller.NodePlan} returns this
*/
proto.clustercontroller.NodePlan.prototype.setEnsureInstalledList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 3, value);
};


/**
 * @param {!proto.clustercontroller.ArtifactRef=} opt_value
 * @param {number=} opt_index
 * @return {!proto.clustercontroller.ArtifactRef}
 */
proto.clustercontroller.NodePlan.prototype.addEnsureInstalled = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 3, opt_value, proto.clustercontroller.ArtifactRef, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.clustercontroller.NodePlan} returns this
 */
proto.clustercontroller.NodePlan.prototype.clearEnsureInstalledList = function() {
  return this.setEnsureInstalledList([]);
};


/**
 * repeated UnitAction unit_actions = 4;
 * @return {!Array<!proto.clustercontroller.UnitAction>}
 */
proto.clustercontroller.NodePlan.prototype.getUnitActionsList = function() {
  return /** @type{!Array<!proto.clustercontroller.UnitAction>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.clustercontroller.UnitAction, 4));
};


/**
 * @param {!Array<!proto.clustercontroller.UnitAction>} value
 * @return {!proto.clustercontroller.NodePlan} returns this
*/
proto.clustercontroller.NodePlan.prototype.setUnitActionsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 4, value);
};


/**
 * @param {!proto.clustercontroller.UnitAction=} opt_value
 * @param {number=} opt_index
 * @return {!proto.clustercontroller.UnitAction}
 */
proto.clustercontroller.NodePlan.prototype.addUnitActions = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 4, opt_value, proto.clustercontroller.UnitAction, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.clustercontroller.NodePlan} returns this
 */
proto.clustercontroller.NodePlan.prototype.clearUnitActionsList = function() {
  return this.setUnitActionsList([]);
};


/**
 * map<string, string> rendered_config = 5;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.clustercontroller.NodePlan.prototype.getRenderedConfigMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 5, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.clustercontroller.NodePlan} returns this
 */
proto.clustercontroller.NodePlan.prototype.clearRenderedConfigMap = function() {
  this.getRenderedConfigMap().clear();
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
proto.clustercontroller.UpgradeGlobularRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.UpgradeGlobularRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.UpgradeGlobularRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.UpgradeGlobularRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.UpgradeGlobularRequest}
 */
proto.clustercontroller.UpgradeGlobularRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.UpgradeGlobularRequest;
  return proto.clustercontroller.UpgradeGlobularRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.UpgradeGlobularRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.UpgradeGlobularRequest}
 */
proto.clustercontroller.UpgradeGlobularRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.UpgradeGlobularRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.UpgradeGlobularRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.UpgradeGlobularRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.UpgradeGlobularRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.UpgradeGlobularRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.UpgradeGlobularRequest} returns this
 */
proto.clustercontroller.UpgradeGlobularRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string platform = 2;
 * @return {string}
 */
proto.clustercontroller.UpgradeGlobularRequest.prototype.getPlatform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.UpgradeGlobularRequest} returns this
 */
proto.clustercontroller.UpgradeGlobularRequest.prototype.setPlatform = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional bytes artifact = 3;
 * @return {string}
 */
proto.clustercontroller.UpgradeGlobularRequest.prototype.getArtifact = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * optional bytes artifact = 3;
 * This is a type-conversion wrapper around `getArtifact()`
 * @return {string}
 */
proto.clustercontroller.UpgradeGlobularRequest.prototype.getArtifact_asB64 = function() {
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
proto.clustercontroller.UpgradeGlobularRequest.prototype.getArtifact_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getArtifact()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.clustercontroller.UpgradeGlobularRequest} returns this
 */
proto.clustercontroller.UpgradeGlobularRequest.prototype.setArtifact = function(value) {
  return jspb.Message.setProto3BytesField(this, 3, value);
};


/**
 * optional string sha256 = 4;
 * @return {string}
 */
proto.clustercontroller.UpgradeGlobularRequest.prototype.getSha256 = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.UpgradeGlobularRequest} returns this
 */
proto.clustercontroller.UpgradeGlobularRequest.prototype.setSha256 = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string target_path = 5;
 * @return {string}
 */
proto.clustercontroller.UpgradeGlobularRequest.prototype.getTargetPath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.UpgradeGlobularRequest} returns this
 */
proto.clustercontroller.UpgradeGlobularRequest.prototype.setTargetPath = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional uint32 probe_port = 6;
 * @return {number}
 */
proto.clustercontroller.UpgradeGlobularRequest.prototype.getProbePort = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.clustercontroller.UpgradeGlobularRequest} returns this
 */
proto.clustercontroller.UpgradeGlobularRequest.prototype.setProbePort = function(value) {
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
proto.clustercontroller.UpgradeGlobularResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.UpgradeGlobularResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.UpgradeGlobularResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.UpgradeGlobularResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
planId: jspb.Message.getFieldWithDefault(msg, 1, ""),
generation: jspb.Message.getFieldWithDefault(msg, 2, 0),
terminalState: jspb.Message.getFieldWithDefault(msg, 3, ""),
errorStepId: jspb.Message.getFieldWithDefault(msg, 4, ""),
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
 * @return {!proto.clustercontroller.UpgradeGlobularResponse}
 */
proto.clustercontroller.UpgradeGlobularResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.UpgradeGlobularResponse;
  return proto.clustercontroller.UpgradeGlobularResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.UpgradeGlobularResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.UpgradeGlobularResponse}
 */
proto.clustercontroller.UpgradeGlobularResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlanId(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readUint64());
      msg.setGeneration(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setTerminalState(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setErrorStepId(value);
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
proto.clustercontroller.UpgradeGlobularResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.UpgradeGlobularResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.UpgradeGlobularResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.UpgradeGlobularResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPlanId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getGeneration();
  if (f !== 0) {
    writer.writeUint64(
      2,
      f
    );
  }
  f = message.getTerminalState();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getErrorStepId();
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
 * optional string plan_id = 1;
 * @return {string}
 */
proto.clustercontroller.UpgradeGlobularResponse.prototype.getPlanId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.UpgradeGlobularResponse} returns this
 */
proto.clustercontroller.UpgradeGlobularResponse.prototype.setPlanId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional uint64 generation = 2;
 * @return {number}
 */
proto.clustercontroller.UpgradeGlobularResponse.prototype.getGeneration = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.clustercontroller.UpgradeGlobularResponse} returns this
 */
proto.clustercontroller.UpgradeGlobularResponse.prototype.setGeneration = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string terminal_state = 3;
 * @return {string}
 */
proto.clustercontroller.UpgradeGlobularResponse.prototype.getTerminalState = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.UpgradeGlobularResponse} returns this
 */
proto.clustercontroller.UpgradeGlobularResponse.prototype.setTerminalState = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string error_step_id = 4;
 * @return {string}
 */
proto.clustercontroller.UpgradeGlobularResponse.prototype.getErrorStepId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.UpgradeGlobularResponse} returns this
 */
proto.clustercontroller.UpgradeGlobularResponse.prototype.setErrorStepId = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string error_message = 5;
 * @return {string}
 */
proto.clustercontroller.UpgradeGlobularResponse.prototype.getErrorMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.UpgradeGlobularResponse} returns this
 */
proto.clustercontroller.UpgradeGlobularResponse.prototype.setErrorMessage = function(value) {
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
proto.clustercontroller.GetNodePlanRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.GetNodePlanRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.GetNodePlanRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.GetNodePlanRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.GetNodePlanRequest}
 */
proto.clustercontroller.GetNodePlanRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.GetNodePlanRequest;
  return proto.clustercontroller.GetNodePlanRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.GetNodePlanRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.GetNodePlanRequest}
 */
proto.clustercontroller.GetNodePlanRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.GetNodePlanRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.GetNodePlanRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.GetNodePlanRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.GetNodePlanRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.GetNodePlanRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.GetNodePlanRequest} returns this
 */
proto.clustercontroller.GetNodePlanRequest.prototype.setNodeId = function(value) {
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
proto.clustercontroller.GetNodePlanResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.GetNodePlanResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.GetNodePlanResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.GetNodePlanResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
plan: (f = msg.getPlan()) && proto.clustercontroller.NodePlan.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.clustercontroller.GetNodePlanResponse}
 */
proto.clustercontroller.GetNodePlanResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.GetNodePlanResponse;
  return proto.clustercontroller.GetNodePlanResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.GetNodePlanResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.GetNodePlanResponse}
 */
proto.clustercontroller.GetNodePlanResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.clustercontroller.NodePlan;
      reader.readMessage(value,proto.clustercontroller.NodePlan.deserializeBinaryFromReader);
      msg.setPlan(value);
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
proto.clustercontroller.GetNodePlanResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.GetNodePlanResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.GetNodePlanResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.GetNodePlanResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPlan();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.clustercontroller.NodePlan.serializeBinaryToWriter
    );
  }
};


/**
 * optional NodePlan plan = 1;
 * @return {?proto.clustercontroller.NodePlan}
 */
proto.clustercontroller.GetNodePlanResponse.prototype.getPlan = function() {
  return /** @type{?proto.clustercontroller.NodePlan} */ (
    jspb.Message.getWrapperField(this, proto.clustercontroller.NodePlan, 1));
};


/**
 * @param {?proto.clustercontroller.NodePlan|undefined} value
 * @return {!proto.clustercontroller.GetNodePlanResponse} returns this
*/
proto.clustercontroller.GetNodePlanResponse.prototype.setPlan = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.clustercontroller.GetNodePlanResponse} returns this
 */
proto.clustercontroller.GetNodePlanResponse.prototype.clearPlan = function() {
  return this.setPlan(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.clustercontroller.GetNodePlanResponse.prototype.hasPlan = function() {
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
proto.clustercontroller.GetNodePlanV1Request.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.GetNodePlanV1Request.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.GetNodePlanV1Request} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.GetNodePlanV1Request.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.GetNodePlanV1Request}
 */
proto.clustercontroller.GetNodePlanV1Request.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.GetNodePlanV1Request;
  return proto.clustercontroller.GetNodePlanV1Request.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.GetNodePlanV1Request} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.GetNodePlanV1Request}
 */
proto.clustercontroller.GetNodePlanV1Request.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.GetNodePlanV1Request.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.GetNodePlanV1Request.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.GetNodePlanV1Request} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.GetNodePlanV1Request.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.GetNodePlanV1Request.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.GetNodePlanV1Request} returns this
 */
proto.clustercontroller.GetNodePlanV1Request.prototype.setNodeId = function(value) {
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
proto.clustercontroller.GetNodePlanV1Response.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.GetNodePlanV1Response.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.GetNodePlanV1Response} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.GetNodePlanV1Response.toObject = function(includeInstance, msg) {
  var f, obj = {
plan: (f = msg.getPlan()) && plan_pb.NodePlan.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.clustercontroller.GetNodePlanV1Response}
 */
proto.clustercontroller.GetNodePlanV1Response.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.GetNodePlanV1Response;
  return proto.clustercontroller.GetNodePlanV1Response.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.GetNodePlanV1Response} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.GetNodePlanV1Response}
 */
proto.clustercontroller.GetNodePlanV1Response.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new plan_pb.NodePlan;
      reader.readMessage(value,plan_pb.NodePlan.deserializeBinaryFromReader);
      msg.setPlan(value);
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
proto.clustercontroller.GetNodePlanV1Response.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.GetNodePlanV1Response.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.GetNodePlanV1Response} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.GetNodePlanV1Response.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPlan();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      plan_pb.NodePlan.serializeBinaryToWriter
    );
  }
};


/**
 * optional globular.plan.v1.NodePlan plan = 1;
 * @return {?proto.globular.plan.v1.NodePlan}
 */
proto.clustercontroller.GetNodePlanV1Response.prototype.getPlan = function() {
  return /** @type{?proto.globular.plan.v1.NodePlan} */ (
    jspb.Message.getWrapperField(this, plan_pb.NodePlan, 1));
};


/**
 * @param {?proto.globular.plan.v1.NodePlan|undefined} value
 * @return {!proto.clustercontroller.GetNodePlanV1Response} returns this
*/
proto.clustercontroller.GetNodePlanV1Response.prototype.setPlan = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.clustercontroller.GetNodePlanV1Response} returns this
 */
proto.clustercontroller.GetNodePlanV1Response.prototype.clearPlan = function() {
  return this.setPlan(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.clustercontroller.GetNodePlanV1Response.prototype.hasPlan = function() {
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
proto.clustercontroller.ReconcileNodeV1Request.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.ReconcileNodeV1Request.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.ReconcileNodeV1Request} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ReconcileNodeV1Request.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.ReconcileNodeV1Request}
 */
proto.clustercontroller.ReconcileNodeV1Request.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.ReconcileNodeV1Request;
  return proto.clustercontroller.ReconcileNodeV1Request.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.ReconcileNodeV1Request} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.ReconcileNodeV1Request}
 */
proto.clustercontroller.ReconcileNodeV1Request.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.ReconcileNodeV1Request.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.ReconcileNodeV1Request.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.ReconcileNodeV1Request} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ReconcileNodeV1Request.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.ReconcileNodeV1Request.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.ReconcileNodeV1Request} returns this
 */
proto.clustercontroller.ReconcileNodeV1Request.prototype.setNodeId = function(value) {
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
proto.clustercontroller.ReconcileNodeV1Response.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.ReconcileNodeV1Response.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.ReconcileNodeV1Response} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ReconcileNodeV1Response.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.ReconcileNodeV1Response}
 */
proto.clustercontroller.ReconcileNodeV1Response.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.ReconcileNodeV1Response;
  return proto.clustercontroller.ReconcileNodeV1Response.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.ReconcileNodeV1Response} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.ReconcileNodeV1Response}
 */
proto.clustercontroller.ReconcileNodeV1Response.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.ReconcileNodeV1Response.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.ReconcileNodeV1Response.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.ReconcileNodeV1Response} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ReconcileNodeV1Response.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.WatchNodePlanStatusV1Request.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.WatchNodePlanStatusV1Request.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.WatchNodePlanStatusV1Request} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.WatchNodePlanStatusV1Request.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.WatchNodePlanStatusV1Request}
 */
proto.clustercontroller.WatchNodePlanStatusV1Request.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.WatchNodePlanStatusV1Request;
  return proto.clustercontroller.WatchNodePlanStatusV1Request.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.WatchNodePlanStatusV1Request} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.WatchNodePlanStatusV1Request}
 */
proto.clustercontroller.WatchNodePlanStatusV1Request.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.WatchNodePlanStatusV1Request.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.WatchNodePlanStatusV1Request.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.WatchNodePlanStatusV1Request} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.WatchNodePlanStatusV1Request.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.WatchNodePlanStatusV1Request.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.WatchNodePlanStatusV1Request} returns this
 */
proto.clustercontroller.WatchNodePlanStatusV1Request.prototype.setNodeId = function(value) {
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
proto.clustercontroller.StartApplyRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.StartApplyRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.StartApplyRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.StartApplyRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.StartApplyRequest}
 */
proto.clustercontroller.StartApplyRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.StartApplyRequest;
  return proto.clustercontroller.StartApplyRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.StartApplyRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.StartApplyRequest}
 */
proto.clustercontroller.StartApplyRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.StartApplyRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.StartApplyRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.StartApplyRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.StartApplyRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.StartApplyRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.StartApplyRequest} returns this
 */
proto.clustercontroller.StartApplyRequest.prototype.setNodeId = function(value) {
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
proto.clustercontroller.StartApplyResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.StartApplyResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.StartApplyResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.StartApplyResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.StartApplyResponse}
 */
proto.clustercontroller.StartApplyResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.StartApplyResponse;
  return proto.clustercontroller.StartApplyResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.StartApplyResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.StartApplyResponse}
 */
proto.clustercontroller.StartApplyResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.StartApplyResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.StartApplyResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.StartApplyResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.StartApplyResponse.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.StartApplyResponse.prototype.getOperationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.StartApplyResponse} returns this
 */
proto.clustercontroller.StartApplyResponse.prototype.setOperationId = function(value) {
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
proto.clustercontroller.OperationEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.OperationEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.OperationEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.OperationEvent.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.OperationEvent}
 */
proto.clustercontroller.OperationEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.OperationEvent;
  return proto.clustercontroller.OperationEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.OperationEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.OperationEvent}
 */
proto.clustercontroller.OperationEvent.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {!proto.clustercontroller.OperationPhase} */ (reader.readEnum());
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
proto.clustercontroller.OperationEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.OperationEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.OperationEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.OperationEvent.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.OperationEvent.prototype.getOperationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.OperationEvent} returns this
 */
proto.clustercontroller.OperationEvent.prototype.setOperationId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string node_id = 2;
 * @return {string}
 */
proto.clustercontroller.OperationEvent.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.OperationEvent} returns this
 */
proto.clustercontroller.OperationEvent.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional OperationPhase phase = 3;
 * @return {!proto.clustercontroller.OperationPhase}
 */
proto.clustercontroller.OperationEvent.prototype.getPhase = function() {
  return /** @type {!proto.clustercontroller.OperationPhase} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.clustercontroller.OperationPhase} value
 * @return {!proto.clustercontroller.OperationEvent} returns this
 */
proto.clustercontroller.OperationEvent.prototype.setPhase = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
};


/**
 * optional string message = 4;
 * @return {string}
 */
proto.clustercontroller.OperationEvent.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.OperationEvent} returns this
 */
proto.clustercontroller.OperationEvent.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional int32 percent = 5;
 * @return {number}
 */
proto.clustercontroller.OperationEvent.prototype.getPercent = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.clustercontroller.OperationEvent} returns this
 */
proto.clustercontroller.OperationEvent.prototype.setPercent = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional bool done = 6;
 * @return {boolean}
 */
proto.clustercontroller.OperationEvent.prototype.getDone = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 6, false));
};


/**
 * @param {boolean} value
 * @return {!proto.clustercontroller.OperationEvent} returns this
 */
proto.clustercontroller.OperationEvent.prototype.setDone = function(value) {
  return jspb.Message.setProto3BooleanField(this, 6, value);
};


/**
 * optional string error = 7;
 * @return {string}
 */
proto.clustercontroller.OperationEvent.prototype.getError = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.OperationEvent} returns this
 */
proto.clustercontroller.OperationEvent.prototype.setError = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional google.protobuf.Timestamp ts = 8;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.clustercontroller.OperationEvent.prototype.getTs = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 8));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.clustercontroller.OperationEvent} returns this
*/
proto.clustercontroller.OperationEvent.prototype.setTs = function(value) {
  return jspb.Message.setWrapperField(this, 8, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.clustercontroller.OperationEvent} returns this
 */
proto.clustercontroller.OperationEvent.prototype.clearTs = function() {
  return this.setTs(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.clustercontroller.OperationEvent.prototype.hasTs = function() {
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
proto.clustercontroller.CompleteOperationRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.CompleteOperationRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.CompleteOperationRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.CompleteOperationRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.CompleteOperationRequest}
 */
proto.clustercontroller.CompleteOperationRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.CompleteOperationRequest;
  return proto.clustercontroller.CompleteOperationRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.CompleteOperationRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.CompleteOperationRequest}
 */
proto.clustercontroller.CompleteOperationRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.CompleteOperationRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.CompleteOperationRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.CompleteOperationRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.CompleteOperationRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.CompleteOperationRequest.prototype.getOperationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.CompleteOperationRequest} returns this
 */
proto.clustercontroller.CompleteOperationRequest.prototype.setOperationId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string node_id = 2;
 * @return {string}
 */
proto.clustercontroller.CompleteOperationRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.CompleteOperationRequest} returns this
 */
proto.clustercontroller.CompleteOperationRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional bool success = 3;
 * @return {boolean}
 */
proto.clustercontroller.CompleteOperationRequest.prototype.getSuccess = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.clustercontroller.CompleteOperationRequest} returns this
 */
proto.clustercontroller.CompleteOperationRequest.prototype.setSuccess = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * optional string message = 4;
 * @return {string}
 */
proto.clustercontroller.CompleteOperationRequest.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.CompleteOperationRequest} returns this
 */
proto.clustercontroller.CompleteOperationRequest.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string error = 5;
 * @return {string}
 */
proto.clustercontroller.CompleteOperationRequest.prototype.getError = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.CompleteOperationRequest} returns this
 */
proto.clustercontroller.CompleteOperationRequest.prototype.setError = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional int32 percent = 6;
 * @return {number}
 */
proto.clustercontroller.CompleteOperationRequest.prototype.getPercent = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.clustercontroller.CompleteOperationRequest} returns this
 */
proto.clustercontroller.CompleteOperationRequest.prototype.setPercent = function(value) {
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
proto.clustercontroller.CompleteOperationResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.CompleteOperationResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.CompleteOperationResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.CompleteOperationResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.CompleteOperationResponse}
 */
proto.clustercontroller.CompleteOperationResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.CompleteOperationResponse;
  return proto.clustercontroller.CompleteOperationResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.CompleteOperationResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.CompleteOperationResponse}
 */
proto.clustercontroller.CompleteOperationResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.CompleteOperationResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.CompleteOperationResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.CompleteOperationResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.CompleteOperationResponse.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.CompleteOperationResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.CompleteOperationResponse} returns this
 */
proto.clustercontroller.CompleteOperationResponse.prototype.setMessage = function(value) {
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
proto.clustercontroller.NodeUnitStatus.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.NodeUnitStatus.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.NodeUnitStatus} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.NodeUnitStatus.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.NodeUnitStatus}
 */
proto.clustercontroller.NodeUnitStatus.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.NodeUnitStatus;
  return proto.clustercontroller.NodeUnitStatus.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.NodeUnitStatus} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.NodeUnitStatus}
 */
proto.clustercontroller.NodeUnitStatus.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.NodeUnitStatus.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.NodeUnitStatus.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.NodeUnitStatus} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.NodeUnitStatus.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.NodeUnitStatus.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeUnitStatus} returns this
 */
proto.clustercontroller.NodeUnitStatus.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string state = 2;
 * @return {string}
 */
proto.clustercontroller.NodeUnitStatus.prototype.getState = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeUnitStatus} returns this
 */
proto.clustercontroller.NodeUnitStatus.prototype.setState = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string details = 3;
 * @return {string}
 */
proto.clustercontroller.NodeUnitStatus.prototype.getDetails = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeUnitStatus} returns this
 */
proto.clustercontroller.NodeUnitStatus.prototype.setDetails = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.clustercontroller.NodeStatus.repeatedFields_ = [3,4];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.clustercontroller.NodeStatus.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.NodeStatus.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.NodeStatus} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.NodeStatus.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
identity: (f = msg.getIdentity()) && proto.clustercontroller.NodeIdentity.toObject(includeInstance, f),
ipsList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f,
unitsList: jspb.Message.toObjectList(msg.getUnitsList(),
    proto.clustercontroller.NodeUnitStatus.toObject, includeInstance),
lastError: jspb.Message.getFieldWithDefault(msg, 5, ""),
reportedAt: (f = msg.getReportedAt()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
agentEndpoint: jspb.Message.getFieldWithDefault(msg, 7, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.clustercontroller.NodeStatus}
 */
proto.clustercontroller.NodeStatus.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.NodeStatus;
  return proto.clustercontroller.NodeStatus.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.NodeStatus} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.NodeStatus}
 */
proto.clustercontroller.NodeStatus.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = new proto.clustercontroller.NodeIdentity;
      reader.readMessage(value,proto.clustercontroller.NodeIdentity.deserializeBinaryFromReader);
      msg.setIdentity(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.addIps(value);
      break;
    case 4:
      var value = new proto.clustercontroller.NodeUnitStatus;
      reader.readMessage(value,proto.clustercontroller.NodeUnitStatus.deserializeBinaryFromReader);
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
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.clustercontroller.NodeStatus.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.NodeStatus.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.NodeStatus} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.NodeStatus.serializeBinaryToWriter = function(message, writer) {
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
      proto.clustercontroller.NodeIdentity.serializeBinaryToWriter
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
      proto.clustercontroller.NodeUnitStatus.serializeBinaryToWriter
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
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.clustercontroller.NodeStatus.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeStatus} returns this
 */
proto.clustercontroller.NodeStatus.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional NodeIdentity identity = 2;
 * @return {?proto.clustercontroller.NodeIdentity}
 */
proto.clustercontroller.NodeStatus.prototype.getIdentity = function() {
  return /** @type{?proto.clustercontroller.NodeIdentity} */ (
    jspb.Message.getWrapperField(this, proto.clustercontroller.NodeIdentity, 2));
};


/**
 * @param {?proto.clustercontroller.NodeIdentity|undefined} value
 * @return {!proto.clustercontroller.NodeStatus} returns this
*/
proto.clustercontroller.NodeStatus.prototype.setIdentity = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.clustercontroller.NodeStatus} returns this
 */
proto.clustercontroller.NodeStatus.prototype.clearIdentity = function() {
  return this.setIdentity(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.clustercontroller.NodeStatus.prototype.hasIdentity = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * repeated string ips = 3;
 * @return {!Array<string>}
 */
proto.clustercontroller.NodeStatus.prototype.getIpsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.clustercontroller.NodeStatus} returns this
 */
proto.clustercontroller.NodeStatus.prototype.setIpsList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.clustercontroller.NodeStatus} returns this
 */
proto.clustercontroller.NodeStatus.prototype.addIps = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.clustercontroller.NodeStatus} returns this
 */
proto.clustercontroller.NodeStatus.prototype.clearIpsList = function() {
  return this.setIpsList([]);
};


/**
 * repeated NodeUnitStatus units = 4;
 * @return {!Array<!proto.clustercontroller.NodeUnitStatus>}
 */
proto.clustercontroller.NodeStatus.prototype.getUnitsList = function() {
  return /** @type{!Array<!proto.clustercontroller.NodeUnitStatus>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.clustercontroller.NodeUnitStatus, 4));
};


/**
 * @param {!Array<!proto.clustercontroller.NodeUnitStatus>} value
 * @return {!proto.clustercontroller.NodeStatus} returns this
*/
proto.clustercontroller.NodeStatus.prototype.setUnitsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 4, value);
};


/**
 * @param {!proto.clustercontroller.NodeUnitStatus=} opt_value
 * @param {number=} opt_index
 * @return {!proto.clustercontroller.NodeUnitStatus}
 */
proto.clustercontroller.NodeStatus.prototype.addUnits = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 4, opt_value, proto.clustercontroller.NodeUnitStatus, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.clustercontroller.NodeStatus} returns this
 */
proto.clustercontroller.NodeStatus.prototype.clearUnitsList = function() {
  return this.setUnitsList([]);
};


/**
 * optional string last_error = 5;
 * @return {string}
 */
proto.clustercontroller.NodeStatus.prototype.getLastError = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeStatus} returns this
 */
proto.clustercontroller.NodeStatus.prototype.setLastError = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional google.protobuf.Timestamp reported_at = 6;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.clustercontroller.NodeStatus.prototype.getReportedAt = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 6));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.clustercontroller.NodeStatus} returns this
*/
proto.clustercontroller.NodeStatus.prototype.setReportedAt = function(value) {
  return jspb.Message.setWrapperField(this, 6, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.clustercontroller.NodeStatus} returns this
 */
proto.clustercontroller.NodeStatus.prototype.clearReportedAt = function() {
  return this.setReportedAt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.clustercontroller.NodeStatus.prototype.hasReportedAt = function() {
  return jspb.Message.getField(this, 6) != null;
};


/**
 * optional string agent_endpoint = 7;
 * @return {string}
 */
proto.clustercontroller.NodeStatus.prototype.getAgentEndpoint = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeStatus} returns this
 */
proto.clustercontroller.NodeStatus.prototype.setAgentEndpoint = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
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
proto.clustercontroller.ReportNodeStatusRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.ReportNodeStatusRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.ReportNodeStatusRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ReportNodeStatusRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
status: (f = msg.getStatus()) && proto.clustercontroller.NodeStatus.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.clustercontroller.ReportNodeStatusRequest}
 */
proto.clustercontroller.ReportNodeStatusRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.ReportNodeStatusRequest;
  return proto.clustercontroller.ReportNodeStatusRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.ReportNodeStatusRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.ReportNodeStatusRequest}
 */
proto.clustercontroller.ReportNodeStatusRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.clustercontroller.NodeStatus;
      reader.readMessage(value,proto.clustercontroller.NodeStatus.deserializeBinaryFromReader);
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
proto.clustercontroller.ReportNodeStatusRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.ReportNodeStatusRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.ReportNodeStatusRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ReportNodeStatusRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.clustercontroller.NodeStatus.serializeBinaryToWriter
    );
  }
};


/**
 * optional NodeStatus status = 1;
 * @return {?proto.clustercontroller.NodeStatus}
 */
proto.clustercontroller.ReportNodeStatusRequest.prototype.getStatus = function() {
  return /** @type{?proto.clustercontroller.NodeStatus} */ (
    jspb.Message.getWrapperField(this, proto.clustercontroller.NodeStatus, 1));
};


/**
 * @param {?proto.clustercontroller.NodeStatus|undefined} value
 * @return {!proto.clustercontroller.ReportNodeStatusRequest} returns this
*/
proto.clustercontroller.ReportNodeStatusRequest.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.clustercontroller.ReportNodeStatusRequest} returns this
 */
proto.clustercontroller.ReportNodeStatusRequest.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.clustercontroller.ReportNodeStatusRequest.prototype.hasStatus = function() {
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
proto.clustercontroller.ReportNodeStatusResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.ReportNodeStatusResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.ReportNodeStatusResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ReportNodeStatusResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.ReportNodeStatusResponse}
 */
proto.clustercontroller.ReportNodeStatusResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.ReportNodeStatusResponse;
  return proto.clustercontroller.ReportNodeStatusResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.ReportNodeStatusResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.ReportNodeStatusResponse}
 */
proto.clustercontroller.ReportNodeStatusResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.ReportNodeStatusResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.ReportNodeStatusResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.ReportNodeStatusResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ReportNodeStatusResponse.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.ReportNodeStatusResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.ReportNodeStatusResponse} returns this
 */
proto.clustercontroller.ReportNodeStatusResponse.prototype.setMessage = function(value) {
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
proto.clustercontroller.WatchOperationsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.WatchOperationsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.WatchOperationsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.WatchOperationsRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.WatchOperationsRequest}
 */
proto.clustercontroller.WatchOperationsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.WatchOperationsRequest;
  return proto.clustercontroller.WatchOperationsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.WatchOperationsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.WatchOperationsRequest}
 */
proto.clustercontroller.WatchOperationsRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.WatchOperationsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.WatchOperationsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.WatchOperationsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.WatchOperationsRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.WatchOperationsRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.WatchOperationsRequest} returns this
 */
proto.clustercontroller.WatchOperationsRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string operation_id = 2;
 * @return {string}
 */
proto.clustercontroller.WatchOperationsRequest.prototype.getOperationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.WatchOperationsRequest} returns this
 */
proto.clustercontroller.WatchOperationsRequest.prototype.setOperationId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.clustercontroller.DesiredNetwork.repeatedFields_ = [7];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.clustercontroller.DesiredNetwork.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.DesiredNetwork.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.DesiredNetwork} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.DesiredNetwork.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.DesiredNetwork}
 */
proto.clustercontroller.DesiredNetwork.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.DesiredNetwork;
  return proto.clustercontroller.DesiredNetwork.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.DesiredNetwork} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.DesiredNetwork}
 */
proto.clustercontroller.DesiredNetwork.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.DesiredNetwork.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.DesiredNetwork.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.DesiredNetwork} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.DesiredNetwork.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.DesiredNetwork.prototype.getDomain = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.DesiredNetwork} returns this
 */
proto.clustercontroller.DesiredNetwork.prototype.setDomain = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string protocol = 2;
 * @return {string}
 */
proto.clustercontroller.DesiredNetwork.prototype.getProtocol = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.DesiredNetwork} returns this
 */
proto.clustercontroller.DesiredNetwork.prototype.setProtocol = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional uint32 port_http = 3;
 * @return {number}
 */
proto.clustercontroller.DesiredNetwork.prototype.getPortHttp = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.clustercontroller.DesiredNetwork} returns this
 */
proto.clustercontroller.DesiredNetwork.prototype.setPortHttp = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional uint32 port_https = 4;
 * @return {number}
 */
proto.clustercontroller.DesiredNetwork.prototype.getPortHttps = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.clustercontroller.DesiredNetwork} returns this
 */
proto.clustercontroller.DesiredNetwork.prototype.setPortHttps = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional bool acme_enabled = 5;
 * @return {boolean}
 */
proto.clustercontroller.DesiredNetwork.prototype.getAcmeEnabled = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 5, false));
};


/**
 * @param {boolean} value
 * @return {!proto.clustercontroller.DesiredNetwork} returns this
 */
proto.clustercontroller.DesiredNetwork.prototype.setAcmeEnabled = function(value) {
  return jspb.Message.setProto3BooleanField(this, 5, value);
};


/**
 * optional string admin_email = 6;
 * @return {string}
 */
proto.clustercontroller.DesiredNetwork.prototype.getAdminEmail = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.DesiredNetwork} returns this
 */
proto.clustercontroller.DesiredNetwork.prototype.setAdminEmail = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * repeated string alternate_domains = 7;
 * @return {!Array<string>}
 */
proto.clustercontroller.DesiredNetwork.prototype.getAlternateDomainsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 7));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.clustercontroller.DesiredNetwork} returns this
 */
proto.clustercontroller.DesiredNetwork.prototype.setAlternateDomainsList = function(value) {
  return jspb.Message.setField(this, 7, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.clustercontroller.DesiredNetwork} returns this
 */
proto.clustercontroller.DesiredNetwork.prototype.addAlternateDomains = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 7, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.clustercontroller.DesiredNetwork} returns this
 */
proto.clustercontroller.DesiredNetwork.prototype.clearAlternateDomainsList = function() {
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
proto.clustercontroller.GetClusterHealthV1Request.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.GetClusterHealthV1Request.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.GetClusterHealthV1Request} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.GetClusterHealthV1Request.toObject = function(includeInstance, msg) {
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
 * @return {!proto.clustercontroller.GetClusterHealthV1Request}
 */
proto.clustercontroller.GetClusterHealthV1Request.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.GetClusterHealthV1Request;
  return proto.clustercontroller.GetClusterHealthV1Request.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.GetClusterHealthV1Request} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.GetClusterHealthV1Request}
 */
proto.clustercontroller.GetClusterHealthV1Request.deserializeBinaryFromReader = function(msg, reader) {
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
proto.clustercontroller.GetClusterHealthV1Request.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.GetClusterHealthV1Request.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.GetClusterHealthV1Request} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.GetClusterHealthV1Request.serializeBinaryToWriter = function(message, writer) {
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
proto.clustercontroller.GetClusterHealthV1Request.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.GetClusterHealthV1Request} returns this
 */
proto.clustercontroller.GetClusterHealthV1Request.prototype.setClusterId = function(value) {
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
proto.clustercontroller.NodeHealth.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.NodeHealth.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.NodeHealth} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.NodeHealth.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
desiredNetworkHash: jspb.Message.getFieldWithDefault(msg, 2, ""),
appliedNetworkHash: jspb.Message.getFieldWithDefault(msg, 3, ""),
desiredServicesHash: jspb.Message.getFieldWithDefault(msg, 4, ""),
appliedServicesHash: jspb.Message.getFieldWithDefault(msg, 5, ""),
currentPlanId: jspb.Message.getFieldWithDefault(msg, 6, ""),
currentPlanGeneration: jspb.Message.getFieldWithDefault(msg, 7, 0),
currentPlanPhase: jspb.Message.getFieldWithDefault(msg, 8, ""),
lastError: jspb.Message.getFieldWithDefault(msg, 9, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.clustercontroller.NodeHealth}
 */
proto.clustercontroller.NodeHealth.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.NodeHealth;
  return proto.clustercontroller.NodeHealth.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.NodeHealth} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.NodeHealth}
 */
proto.clustercontroller.NodeHealth.deserializeBinaryFromReader = function(msg, reader) {
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
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setCurrentPlanId(value);
      break;
    case 7:
      var value = /** @type {number} */ (reader.readUint64());
      msg.setCurrentPlanGeneration(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setCurrentPlanPhase(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setLastError(value);
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
proto.clustercontroller.NodeHealth.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.NodeHealth.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.NodeHealth} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.NodeHealth.serializeBinaryToWriter = function(message, writer) {
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
  f = message.getCurrentPlanId();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getCurrentPlanGeneration();
  if (f !== 0) {
    writer.writeUint64(
      7,
      f
    );
  }
  f = message.getCurrentPlanPhase();
  if (f.length > 0) {
    writer.writeString(
      8,
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
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.clustercontroller.NodeHealth.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeHealth} returns this
 */
proto.clustercontroller.NodeHealth.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string desired_network_hash = 2;
 * @return {string}
 */
proto.clustercontroller.NodeHealth.prototype.getDesiredNetworkHash = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeHealth} returns this
 */
proto.clustercontroller.NodeHealth.prototype.setDesiredNetworkHash = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string applied_network_hash = 3;
 * @return {string}
 */
proto.clustercontroller.NodeHealth.prototype.getAppliedNetworkHash = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeHealth} returns this
 */
proto.clustercontroller.NodeHealth.prototype.setAppliedNetworkHash = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string desired_services_hash = 4;
 * @return {string}
 */
proto.clustercontroller.NodeHealth.prototype.getDesiredServicesHash = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeHealth} returns this
 */
proto.clustercontroller.NodeHealth.prototype.setDesiredServicesHash = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string applied_services_hash = 5;
 * @return {string}
 */
proto.clustercontroller.NodeHealth.prototype.getAppliedServicesHash = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeHealth} returns this
 */
proto.clustercontroller.NodeHealth.prototype.setAppliedServicesHash = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string current_plan_id = 6;
 * @return {string}
 */
proto.clustercontroller.NodeHealth.prototype.getCurrentPlanId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeHealth} returns this
 */
proto.clustercontroller.NodeHealth.prototype.setCurrentPlanId = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional uint64 current_plan_generation = 7;
 * @return {number}
 */
proto.clustercontroller.NodeHealth.prototype.getCurrentPlanGeneration = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/**
 * @param {number} value
 * @return {!proto.clustercontroller.NodeHealth} returns this
 */
proto.clustercontroller.NodeHealth.prototype.setCurrentPlanGeneration = function(value) {
  return jspb.Message.setProto3IntField(this, 7, value);
};


/**
 * optional string current_plan_phase = 8;
 * @return {string}
 */
proto.clustercontroller.NodeHealth.prototype.getCurrentPlanPhase = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeHealth} returns this
 */
proto.clustercontroller.NodeHealth.prototype.setCurrentPlanPhase = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};


/**
 * optional string last_error = 9;
 * @return {string}
 */
proto.clustercontroller.NodeHealth.prototype.getLastError = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.NodeHealth} returns this
 */
proto.clustercontroller.NodeHealth.prototype.setLastError = function(value) {
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
proto.clustercontroller.ServiceSummary.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.ServiceSummary.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.ServiceSummary} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ServiceSummary.toObject = function(includeInstance, msg) {
  var f, obj = {
serviceName: jspb.Message.getFieldWithDefault(msg, 1, ""),
desiredVersion: jspb.Message.getFieldWithDefault(msg, 2, ""),
nodesAtDesired: jspb.Message.getFieldWithDefault(msg, 3, 0),
nodesTotal: jspb.Message.getFieldWithDefault(msg, 4, 0),
upgrading: jspb.Message.getFieldWithDefault(msg, 5, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.clustercontroller.ServiceSummary}
 */
proto.clustercontroller.ServiceSummary.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.ServiceSummary;
  return proto.clustercontroller.ServiceSummary.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.ServiceSummary} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.ServiceSummary}
 */
proto.clustercontroller.ServiceSummary.deserializeBinaryFromReader = function(msg, reader) {
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
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.clustercontroller.ServiceSummary.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.ServiceSummary.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.ServiceSummary} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.ServiceSummary.serializeBinaryToWriter = function(message, writer) {
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
};


/**
 * optional string service_name = 1;
 * @return {string}
 */
proto.clustercontroller.ServiceSummary.prototype.getServiceName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.ServiceSummary} returns this
 */
proto.clustercontroller.ServiceSummary.prototype.setServiceName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string desired_version = 2;
 * @return {string}
 */
proto.clustercontroller.ServiceSummary.prototype.getDesiredVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.clustercontroller.ServiceSummary} returns this
 */
proto.clustercontroller.ServiceSummary.prototype.setDesiredVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int32 nodes_at_desired = 3;
 * @return {number}
 */
proto.clustercontroller.ServiceSummary.prototype.getNodesAtDesired = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.clustercontroller.ServiceSummary} returns this
 */
proto.clustercontroller.ServiceSummary.prototype.setNodesAtDesired = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional int32 nodes_total = 4;
 * @return {number}
 */
proto.clustercontroller.ServiceSummary.prototype.getNodesTotal = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.clustercontroller.ServiceSummary} returns this
 */
proto.clustercontroller.ServiceSummary.prototype.setNodesTotal = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional int32 upgrading = 5;
 * @return {number}
 */
proto.clustercontroller.ServiceSummary.prototype.getUpgrading = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.clustercontroller.ServiceSummary} returns this
 */
proto.clustercontroller.ServiceSummary.prototype.setUpgrading = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.clustercontroller.GetClusterHealthV1Response.repeatedFields_ = [1,2];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.clustercontroller.GetClusterHealthV1Response.prototype.toObject = function(opt_includeInstance) {
  return proto.clustercontroller.GetClusterHealthV1Response.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.clustercontroller.GetClusterHealthV1Response} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.GetClusterHealthV1Response.toObject = function(includeInstance, msg) {
  var f, obj = {
nodesList: jspb.Message.toObjectList(msg.getNodesList(),
    proto.clustercontroller.NodeHealth.toObject, includeInstance),
servicesList: jspb.Message.toObjectList(msg.getServicesList(),
    proto.clustercontroller.ServiceSummary.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.clustercontroller.GetClusterHealthV1Response}
 */
proto.clustercontroller.GetClusterHealthV1Response.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.clustercontroller.GetClusterHealthV1Response;
  return proto.clustercontroller.GetClusterHealthV1Response.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.clustercontroller.GetClusterHealthV1Response} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.clustercontroller.GetClusterHealthV1Response}
 */
proto.clustercontroller.GetClusterHealthV1Response.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.clustercontroller.NodeHealth;
      reader.readMessage(value,proto.clustercontroller.NodeHealth.deserializeBinaryFromReader);
      msg.addNodes(value);
      break;
    case 2:
      var value = new proto.clustercontroller.ServiceSummary;
      reader.readMessage(value,proto.clustercontroller.ServiceSummary.deserializeBinaryFromReader);
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
proto.clustercontroller.GetClusterHealthV1Response.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.clustercontroller.GetClusterHealthV1Response.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.clustercontroller.GetClusterHealthV1Response} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.clustercontroller.GetClusterHealthV1Response.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.clustercontroller.NodeHealth.serializeBinaryToWriter
    );
  }
  f = message.getServicesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      2,
      f,
      proto.clustercontroller.ServiceSummary.serializeBinaryToWriter
    );
  }
};


/**
 * repeated NodeHealth nodes = 1;
 * @return {!Array<!proto.clustercontroller.NodeHealth>}
 */
proto.clustercontroller.GetClusterHealthV1Response.prototype.getNodesList = function() {
  return /** @type{!Array<!proto.clustercontroller.NodeHealth>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.clustercontroller.NodeHealth, 1));
};


/**
 * @param {!Array<!proto.clustercontroller.NodeHealth>} value
 * @return {!proto.clustercontroller.GetClusterHealthV1Response} returns this
*/
proto.clustercontroller.GetClusterHealthV1Response.prototype.setNodesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.clustercontroller.NodeHealth=} opt_value
 * @param {number=} opt_index
 * @return {!proto.clustercontroller.NodeHealth}
 */
proto.clustercontroller.GetClusterHealthV1Response.prototype.addNodes = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.clustercontroller.NodeHealth, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.clustercontroller.GetClusterHealthV1Response} returns this
 */
proto.clustercontroller.GetClusterHealthV1Response.prototype.clearNodesList = function() {
  return this.setNodesList([]);
};


/**
 * repeated ServiceSummary services = 2;
 * @return {!Array<!proto.clustercontroller.ServiceSummary>}
 */
proto.clustercontroller.GetClusterHealthV1Response.prototype.getServicesList = function() {
  return /** @type{!Array<!proto.clustercontroller.ServiceSummary>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.clustercontroller.ServiceSummary, 2));
};


/**
 * @param {!Array<!proto.clustercontroller.ServiceSummary>} value
 * @return {!proto.clustercontroller.GetClusterHealthV1Response} returns this
*/
proto.clustercontroller.GetClusterHealthV1Response.prototype.setServicesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 2, value);
};


/**
 * @param {!proto.clustercontroller.ServiceSummary=} opt_value
 * @param {number=} opt_index
 * @return {!proto.clustercontroller.ServiceSummary}
 */
proto.clustercontroller.GetClusterHealthV1Response.prototype.addServices = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 2, opt_value, proto.clustercontroller.ServiceSummary, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.clustercontroller.GetClusterHealthV1Response} returns this
 */
proto.clustercontroller.GetClusterHealthV1Response.prototype.clearServicesList = function() {
  return this.setServicesList([]);
};


/**
 * @enum {number}
 */
proto.clustercontroller.ArtifactKind = {
  ARTIFACT_KIND_UNSPECIFIED: 0,
  ARTIFACT_SERVICE: 1,
  ARTIFACT_APPLICATION: 2,
  ARTIFACT_SUBSYSTEM: 3
};

/**
 * @enum {number}
 */
proto.clustercontroller.OperationPhase = {
  OP_PHASE_UNSPECIFIED: 0,
  OP_QUEUED: 1,
  OP_RUNNING: 2,
  OP_SUCCEEDED: 3,
  OP_FAILED: 4
};

goog.object.extend(exports, proto.clustercontroller);
