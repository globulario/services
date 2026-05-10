// source: repository.proto
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

var resource_pb = require('./resource_pb.js');
goog.object.extend(proto, resource_pb);
goog.exportSymbol('proto.repository.AllocateUploadRequest', null, global);
goog.exportSymbol('proto.repository.AllocateUploadResponse', null, global);
goog.exportSymbol('proto.repository.ApplicationDetail', null, global);
goog.exportSymbol('proto.repository.ArchiveUnreachableArtifactsRequest', null, global);
goog.exportSymbol('proto.repository.ArchiveUnreachableArtifactsResponse', null, global);
goog.exportSymbol('proto.repository.ArchivedArtifactRecord', null, global);
goog.exportSymbol('proto.repository.ArtifactChannel', null, global);
goog.exportSymbol('proto.repository.ArtifactDependencyRef', null, global);
goog.exportSymbol('proto.repository.ArtifactKind', null, global);
goog.exportSymbol('proto.repository.ArtifactManifest', null, global);
goog.exportSymbol('proto.repository.ArtifactManifest.TypeDetailCase', null, global);
goog.exportSymbol('proto.repository.ArtifactRef', null, global);
goog.exportSymbol('proto.repository.ArtifactSignature', null, global);
goog.exportSymbol('proto.repository.ArtifactVerifyStatus', null, global);
goog.exportSymbol('proto.repository.BundleSummary', null, global);
goog.exportSymbol('proto.repository.CapabilityHealthProto', null, global);
goog.exportSymbol('proto.repository.ConfigKind', null, global);
goog.exportSymbol('proto.repository.ConfigReceiptAction', null, global);
goog.exportSymbol('proto.repository.DeleteArtifactRequest', null, global);
goog.exportSymbol('proto.repository.DeleteArtifactResponse', null, global);
goog.exportSymbol('proto.repository.DependencyHealthProto', null, global);
goog.exportSymbol('proto.repository.DescribePackageRequest', null, global);
goog.exportSymbol('proto.repository.DescribePackageResponse', null, global);
goog.exportSymbol('proto.repository.DesiredInfo', null, global);
goog.exportSymbol('proto.repository.DownloadArtifactRequest', null, global);
goog.exportSymbol('proto.repository.DownloadArtifactResponse', null, global);
goog.exportSymbol('proto.repository.DownloadBundleRequest', null, global);
goog.exportSymbol('proto.repository.DownloadBundleResponse', null, global);
goog.exportSymbol('proto.repository.ExplainArtifactRequest', null, global);
goog.exportSymbol('proto.repository.ExplainArtifactResponse', null, global);
goog.exportSymbol('proto.repository.GetArtifactManifestRequest', null, global);
goog.exportSymbol('proto.repository.GetArtifactManifestResponse', null, global);
goog.exportSymbol('proto.repository.GetArtifactVersionsRequest', null, global);
goog.exportSymbol('proto.repository.GetArtifactVersionsResponse', null, global);
goog.exportSymbol('proto.repository.GetNamespaceRequest', null, global);
goog.exportSymbol('proto.repository.GetNamespaceResponse', null, global);
goog.exportSymbol('proto.repository.GetRepositoryStatusRequest', null, global);
goog.exportSymbol('proto.repository.GetRepositoryStatusResponse', null, global);
goog.exportSymbol('proto.repository.ImportProvisionalRequest', null, global);
goog.exportSymbol('proto.repository.ImportProvisionalResponse', null, global);
goog.exportSymbol('proto.repository.InfrastructureDetail', null, global);
goog.exportSymbol('proto.repository.InstalledPackageRevision', null, global);
goog.exportSymbol('proto.repository.ListArtifactSignaturesRequest', null, global);
goog.exportSymbol('proto.repository.ListArtifactSignaturesResponse', null, global);
goog.exportSymbol('proto.repository.ListArtifactsRequest', null, global);
goog.exportSymbol('proto.repository.ListArtifactsResponse', null, global);
goog.exportSymbol('proto.repository.ListBundlesRequest', null, global);
goog.exportSymbol('proto.repository.ListBundlesResponse', null, global);
goog.exportSymbol('proto.repository.ListConfigReceiptsRequest', null, global);
goog.exportSymbol('proto.repository.ListConfigReceiptsResponse', null, global);
goog.exportSymbol('proto.repository.ListInstalledRevisionsRequest', null, global);
goog.exportSymbol('proto.repository.ListInstalledRevisionsResponse', null, global);
goog.exportSymbol('proto.repository.ListRepositoryFindingsRequest', null, global);
goog.exportSymbol('proto.repository.ListRepositoryFindingsResponse', null, global);
goog.exportSymbol('proto.repository.ListRollbackCandidatesRequest', null, global);
goog.exportSymbol('proto.repository.ListRollbackCandidatesResponse', null, global);
goog.exportSymbol('proto.repository.ListTrustedPublishersRequest', null, global);
goog.exportSymbol('proto.repository.ListTrustedPublishersResponse', null, global);
goog.exportSymbol('proto.repository.ListUpstreamsRequest', null, global);
goog.exportSymbol('proto.repository.ListUpstreamsResponse', null, global);
goog.exportSymbol('proto.repository.MergeStrategy', null, global);
goog.exportSymbol('proto.repository.NamespaceInfo', null, global);
goog.exportSymbol('proto.repository.NodeInstallation', null, global);
goog.exportSymbol('proto.repository.PackageConfigFile', null, global);
goog.exportSymbol('proto.repository.PackageConfigReceipt', null, global);
goog.exportSymbol('proto.repository.PackageInfo', null, global);
goog.exportSymbol('proto.repository.PromoteArtifactRequest', null, global);
goog.exportSymbol('proto.repository.PromoteArtifactResponse', null, global);
goog.exportSymbol('proto.repository.ProvenanceRecord', null, global);
goog.exportSymbol('proto.repository.PublishState', null, global);
goog.exportSymbol('proto.repository.RecordConfigReceiptRequest', null, global);
goog.exportSymbol('proto.repository.RecordConfigReceiptResponse', null, global);
goog.exportSymbol('proto.repository.RecordInstalledRevisionRequest', null, global);
goog.exportSymbol('proto.repository.RecordInstalledRevisionResponse', null, global);
goog.exportSymbol('proto.repository.RegisterArtifactSignatureRequest', null, global);
goog.exportSymbol('proto.repository.RegisterArtifactSignatureResponse', null, global);
goog.exportSymbol('proto.repository.RegisterUpstreamRequest', null, global);
goog.exportSymbol('proto.repository.RegisterUpstreamResponse', null, global);
goog.exportSymbol('proto.repository.RemoveUpstreamRequest', null, global);
goog.exportSymbol('proto.repository.RemoveUpstreamResponse', null, global);
goog.exportSymbol('proto.repository.RepairArtifactRequest', null, global);
goog.exportSymbol('proto.repository.RepairArtifactResponse', null, global);
goog.exportSymbol('proto.repository.RepositoryFinding', null, global);
goog.exportSymbol('proto.repository.RepositoryFindingKind', null, global);
goog.exportSymbol('proto.repository.RepositoryFindingSeverity', null, global);
goog.exportSymbol('proto.repository.ResolveArtifactRequest', null, global);
goog.exportSymbol('proto.repository.ResolveArtifactResponse', null, global);
goog.exportSymbol('proto.repository.ResolveByEntrypointChecksumRequest', null, global);
goog.exportSymbol('proto.repository.ResolveByEntrypointChecksumResponse', null, global);
goog.exportSymbol('proto.repository.RevokePublisherKeyRequest', null, global);
goog.exportSymbol('proto.repository.RevokePublisherKeyResponse', null, global);
goog.exportSymbol('proto.repository.RollbackCandidate', null, global);
goog.exportSymbol('proto.repository.RollbackEligibility', null, global);
goog.exportSymbol('proto.repository.SearchArtifactsRequest', null, global);
goog.exportSymbol('proto.repository.SearchArtifactsResponse', null, global);
goog.exportSymbol('proto.repository.ServiceDetail', null, global);
goog.exportSymbol('proto.repository.SetArtifactStateRequest', null, global);
goog.exportSymbol('proto.repository.SetArtifactStateResponse', null, global);
goog.exportSymbol('proto.repository.SignaturePolicy', null, global);
goog.exportSymbol('proto.repository.SignatureStatus', null, global);
goog.exportSymbol('proto.repository.SyncFromUpstreamRequest', null, global);
goog.exportSymbol('proto.repository.SyncFromUpstreamResponse', null, global);
goog.exportSymbol('proto.repository.TrustPublisherRequest', null, global);
goog.exportSymbol('proto.repository.TrustPublisherResponse', null, global);
goog.exportSymbol('proto.repository.TrustState', null, global);
goog.exportSymbol('proto.repository.TrustedPublisher', null, global);
goog.exportSymbol('proto.repository.UpdateArtifactBinaryHeader', null, global);
goog.exportSymbol('proto.repository.UpdateArtifactBinaryRequest', null, global);
goog.exportSymbol('proto.repository.UpdateArtifactBinaryRequest.PayloadCase', null, global);
goog.exportSymbol('proto.repository.UpdateArtifactBinaryResponse', null, global);
goog.exportSymbol('proto.repository.UploadArtifactRequest', null, global);
goog.exportSymbol('proto.repository.UploadArtifactResponse', null, global);
goog.exportSymbol('proto.repository.UploadBundleRequest', null, global);
goog.exportSymbol('proto.repository.UploadBundleResponse', null, global);
goog.exportSymbol('proto.repository.UpstreamImportRecord', null, global);
goog.exportSymbol('proto.repository.UpstreamSource', null, global);
goog.exportSymbol('proto.repository.UpstreamSourceType', null, global);
goog.exportSymbol('proto.repository.UpstreamSyncResult', null, global);
goog.exportSymbol('proto.repository.UpstreamSyncStatus', null, global);
goog.exportSymbol('proto.repository.VerifyArtifactRequest', null, global);
goog.exportSymbol('proto.repository.VerifyArtifactResponse', null, global);
goog.exportSymbol('proto.repository.VerifyArtifactSignatureRequest', null, global);
goog.exportSymbol('proto.repository.VerifyArtifactSignatureResponse', null, global);
goog.exportSymbol('proto.repository.VersionIntent', null, global);
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ArtifactRef = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ArtifactRef, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ArtifactRef.displayName = 'proto.repository.ArtifactRef';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ArtifactDependencyRef = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ArtifactDependencyRef, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ArtifactDependencyRef.displayName = 'proto.repository.ArtifactDependencyRef';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ArtifactManifest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.ArtifactManifest.repeatedFields_, proto.repository.ArtifactManifest.oneofGroups_);
};
goog.inherits(proto.repository.ArtifactManifest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ArtifactManifest.displayName = 'proto.repository.ArtifactManifest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ServiceDetail = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.ServiceDetail.repeatedFields_, null);
};
goog.inherits(proto.repository.ServiceDetail, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ServiceDetail.displayName = 'proto.repository.ServiceDetail';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ApplicationDetail = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.ApplicationDetail.repeatedFields_, null);
};
goog.inherits(proto.repository.ApplicationDetail, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ApplicationDetail.displayName = 'proto.repository.ApplicationDetail';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.InfrastructureDetail = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.InfrastructureDetail.repeatedFields_, null);
};
goog.inherits(proto.repository.InfrastructureDetail, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.InfrastructureDetail.displayName = 'proto.repository.InfrastructureDetail';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ProvenanceRecord = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ProvenanceRecord, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ProvenanceRecord.displayName = 'proto.repository.ProvenanceRecord';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.SetArtifactStateRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.SetArtifactStateRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.SetArtifactStateRequest.displayName = 'proto.repository.SetArtifactStateRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.SetArtifactStateResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.SetArtifactStateResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.SetArtifactStateResponse.displayName = 'proto.repository.SetArtifactStateResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.GetNamespaceRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.GetNamespaceRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.GetNamespaceRequest.displayName = 'proto.repository.GetNamespaceRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.NamespaceInfo = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.NamespaceInfo.repeatedFields_, null);
};
goog.inherits(proto.repository.NamespaceInfo, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.NamespaceInfo.displayName = 'proto.repository.NamespaceInfo';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.GetNamespaceResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.GetNamespaceResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.GetNamespaceResponse.displayName = 'proto.repository.GetNamespaceResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ListArtifactsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ListArtifactsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ListArtifactsRequest.displayName = 'proto.repository.ListArtifactsRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ListArtifactsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.ListArtifactsResponse.repeatedFields_, null);
};
goog.inherits(proto.repository.ListArtifactsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ListArtifactsResponse.displayName = 'proto.repository.ListArtifactsResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.UploadArtifactRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.UploadArtifactRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.UploadArtifactRequest.displayName = 'proto.repository.UploadArtifactRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.UploadArtifactResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.UploadArtifactResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.UploadArtifactResponse.displayName = 'proto.repository.UploadArtifactResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.DownloadArtifactRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.DownloadArtifactRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.DownloadArtifactRequest.displayName = 'proto.repository.DownloadArtifactRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.DownloadArtifactResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.DownloadArtifactResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.DownloadArtifactResponse.displayName = 'proto.repository.DownloadArtifactResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.GetArtifactManifestRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.GetArtifactManifestRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.GetArtifactManifestRequest.displayName = 'proto.repository.GetArtifactManifestRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.GetArtifactManifestResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.GetArtifactManifestResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.GetArtifactManifestResponse.displayName = 'proto.repository.GetArtifactManifestResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.UploadBundleRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.UploadBundleRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.UploadBundleRequest.displayName = 'proto.repository.UploadBundleRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.UploadBundleResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.UploadBundleResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.UploadBundleResponse.displayName = 'proto.repository.UploadBundleResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.DownloadBundleRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.DownloadBundleRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.DownloadBundleRequest.displayName = 'proto.repository.DownloadBundleRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.DownloadBundleResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.DownloadBundleResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.DownloadBundleResponse.displayName = 'proto.repository.DownloadBundleResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.BundleSummary = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.BundleSummary, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.BundleSummary.displayName = 'proto.repository.BundleSummary';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ListBundlesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ListBundlesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ListBundlesRequest.displayName = 'proto.repository.ListBundlesRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ListBundlesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.ListBundlesResponse.repeatedFields_, null);
};
goog.inherits(proto.repository.ListBundlesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ListBundlesResponse.displayName = 'proto.repository.ListBundlesResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.SearchArtifactsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.SearchArtifactsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.SearchArtifactsRequest.displayName = 'proto.repository.SearchArtifactsRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.SearchArtifactsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.SearchArtifactsResponse.repeatedFields_, null);
};
goog.inherits(proto.repository.SearchArtifactsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.SearchArtifactsResponse.displayName = 'proto.repository.SearchArtifactsResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.GetArtifactVersionsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.GetArtifactVersionsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.GetArtifactVersionsRequest.displayName = 'proto.repository.GetArtifactVersionsRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.GetArtifactVersionsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.GetArtifactVersionsResponse.repeatedFields_, null);
};
goog.inherits(proto.repository.GetArtifactVersionsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.GetArtifactVersionsResponse.displayName = 'proto.repository.GetArtifactVersionsResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.DeleteArtifactRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.DeleteArtifactRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.DeleteArtifactRequest.displayName = 'proto.repository.DeleteArtifactRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.DeleteArtifactResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.DeleteArtifactResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.DeleteArtifactResponse.displayName = 'proto.repository.DeleteArtifactResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.PromoteArtifactRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.PromoteArtifactRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.PromoteArtifactRequest.displayName = 'proto.repository.PromoteArtifactRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.PromoteArtifactResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.PromoteArtifactResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.PromoteArtifactResponse.displayName = 'proto.repository.PromoteArtifactResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.DescribePackageRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.DescribePackageRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.DescribePackageRequest.displayName = 'proto.repository.DescribePackageRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.NodeInstallation = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.NodeInstallation, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.NodeInstallation.displayName = 'proto.repository.NodeInstallation';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.DesiredInfo = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.DesiredInfo, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.DesiredInfo.displayName = 'proto.repository.DesiredInfo';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.PackageInfo = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.PackageInfo.repeatedFields_, null);
};
goog.inherits(proto.repository.PackageInfo, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.PackageInfo.displayName = 'proto.repository.PackageInfo';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.DescribePackageResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.DescribePackageResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.DescribePackageResponse.displayName = 'proto.repository.DescribePackageResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.VerifyArtifactRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.VerifyArtifactRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.VerifyArtifactRequest.displayName = 'proto.repository.VerifyArtifactRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.VerifyArtifactResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.VerifyArtifactResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.VerifyArtifactResponse.displayName = 'proto.repository.VerifyArtifactResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.RepairArtifactRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.RepairArtifactRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.RepairArtifactRequest.displayName = 'proto.repository.RepairArtifactRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.RepairArtifactResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.RepairArtifactResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.RepairArtifactResponse.displayName = 'proto.repository.RepairArtifactResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ExplainArtifactRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ExplainArtifactRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ExplainArtifactRequest.displayName = 'proto.repository.ExplainArtifactRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ExplainArtifactResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.ExplainArtifactResponse.repeatedFields_, null);
};
goog.inherits(proto.repository.ExplainArtifactResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ExplainArtifactResponse.displayName = 'proto.repository.ExplainArtifactResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ResolveArtifactRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ResolveArtifactRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ResolveArtifactRequest.displayName = 'proto.repository.ResolveArtifactRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ResolveArtifactResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ResolveArtifactResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ResolveArtifactResponse.displayName = 'proto.repository.ResolveArtifactResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ResolveByEntrypointChecksumRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ResolveByEntrypointChecksumRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ResolveByEntrypointChecksumRequest.displayName = 'proto.repository.ResolveByEntrypointChecksumRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ResolveByEntrypointChecksumResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ResolveByEntrypointChecksumResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ResolveByEntrypointChecksumResponse.displayName = 'proto.repository.ResolveByEntrypointChecksumResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.UpdateArtifactBinaryRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.repository.UpdateArtifactBinaryRequest.oneofGroups_);
};
goog.inherits(proto.repository.UpdateArtifactBinaryRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.UpdateArtifactBinaryRequest.displayName = 'proto.repository.UpdateArtifactBinaryRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.UpdateArtifactBinaryHeader = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.UpdateArtifactBinaryHeader, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.UpdateArtifactBinaryHeader.displayName = 'proto.repository.UpdateArtifactBinaryHeader';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.UpdateArtifactBinaryResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.UpdateArtifactBinaryResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.UpdateArtifactBinaryResponse.displayName = 'proto.repository.UpdateArtifactBinaryResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.AllocateUploadRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.AllocateUploadRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.AllocateUploadRequest.displayName = 'proto.repository.AllocateUploadRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.AllocateUploadResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.AllocateUploadResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.AllocateUploadResponse.displayName = 'proto.repository.AllocateUploadResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ImportProvisionalRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ImportProvisionalRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ImportProvisionalRequest.displayName = 'proto.repository.ImportProvisionalRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ImportProvisionalResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ImportProvisionalResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ImportProvisionalResponse.displayName = 'proto.repository.ImportProvisionalResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.UpstreamSource = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.UpstreamSource.repeatedFields_, null);
};
goog.inherits(proto.repository.UpstreamSource, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.UpstreamSource.displayName = 'proto.repository.UpstreamSource';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.RegisterUpstreamRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.RegisterUpstreamRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.RegisterUpstreamRequest.displayName = 'proto.repository.RegisterUpstreamRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.RegisterUpstreamResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.RegisterUpstreamResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.RegisterUpstreamResponse.displayName = 'proto.repository.RegisterUpstreamResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ListUpstreamsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ListUpstreamsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ListUpstreamsRequest.displayName = 'proto.repository.ListUpstreamsRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ListUpstreamsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.ListUpstreamsResponse.repeatedFields_, null);
};
goog.inherits(proto.repository.ListUpstreamsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ListUpstreamsResponse.displayName = 'proto.repository.ListUpstreamsResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.RemoveUpstreamRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.RemoveUpstreamRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.RemoveUpstreamRequest.displayName = 'proto.repository.RemoveUpstreamRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.RemoveUpstreamResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.RemoveUpstreamResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.RemoveUpstreamResponse.displayName = 'proto.repository.RemoveUpstreamResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.UpstreamSyncResult = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.UpstreamSyncResult, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.UpstreamSyncResult.displayName = 'proto.repository.UpstreamSyncResult';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.SyncFromUpstreamRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.SyncFromUpstreamRequest.repeatedFields_, null);
};
goog.inherits(proto.repository.SyncFromUpstreamRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.SyncFromUpstreamRequest.displayName = 'proto.repository.SyncFromUpstreamRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.SyncFromUpstreamResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.SyncFromUpstreamResponse.repeatedFields_, null);
};
goog.inherits(proto.repository.SyncFromUpstreamResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.SyncFromUpstreamResponse.displayName = 'proto.repository.SyncFromUpstreamResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.UpstreamImportRecord = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.UpstreamImportRecord, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.UpstreamImportRecord.displayName = 'proto.repository.UpstreamImportRecord';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ArchiveUnreachableArtifactsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ArchiveUnreachableArtifactsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ArchiveUnreachableArtifactsRequest.displayName = 'proto.repository.ArchiveUnreachableArtifactsRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ArchivedArtifactRecord = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ArchivedArtifactRecord, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ArchivedArtifactRecord.displayName = 'proto.repository.ArchivedArtifactRecord';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ArchiveUnreachableArtifactsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.ArchiveUnreachableArtifactsResponse.repeatedFields_, null);
};
goog.inherits(proto.repository.ArchiveUnreachableArtifactsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ArchiveUnreachableArtifactsResponse.displayName = 'proto.repository.ArchiveUnreachableArtifactsResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.PackageConfigFile = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.PackageConfigFile, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.PackageConfigFile.displayName = 'proto.repository.PackageConfigFile';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.TrustedPublisher = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.TrustedPublisher, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.TrustedPublisher.displayName = 'proto.repository.TrustedPublisher';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ArtifactSignature = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ArtifactSignature, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ArtifactSignature.displayName = 'proto.repository.ArtifactSignature';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.TrustPublisherRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.TrustPublisherRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.TrustPublisherRequest.displayName = 'proto.repository.TrustPublisherRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.TrustPublisherResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.TrustPublisherResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.TrustPublisherResponse.displayName = 'proto.repository.TrustPublisherResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.RevokePublisherKeyRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.RevokePublisherKeyRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.RevokePublisherKeyRequest.displayName = 'proto.repository.RevokePublisherKeyRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.RevokePublisherKeyResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.RevokePublisherKeyResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.RevokePublisherKeyResponse.displayName = 'proto.repository.RevokePublisherKeyResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ListTrustedPublishersRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ListTrustedPublishersRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ListTrustedPublishersRequest.displayName = 'proto.repository.ListTrustedPublishersRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ListTrustedPublishersResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.ListTrustedPublishersResponse.repeatedFields_, null);
};
goog.inherits(proto.repository.ListTrustedPublishersResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ListTrustedPublishersResponse.displayName = 'proto.repository.ListTrustedPublishersResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.RegisterArtifactSignatureRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.RegisterArtifactSignatureRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.RegisterArtifactSignatureRequest.displayName = 'proto.repository.RegisterArtifactSignatureRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.RegisterArtifactSignatureResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.RegisterArtifactSignatureResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.RegisterArtifactSignatureResponse.displayName = 'proto.repository.RegisterArtifactSignatureResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.VerifyArtifactSignatureRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.VerifyArtifactSignatureRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.VerifyArtifactSignatureRequest.displayName = 'proto.repository.VerifyArtifactSignatureRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.VerifyArtifactSignatureResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.VerifyArtifactSignatureResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.VerifyArtifactSignatureResponse.displayName = 'proto.repository.VerifyArtifactSignatureResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ListArtifactSignaturesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ListArtifactSignaturesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ListArtifactSignaturesRequest.displayName = 'proto.repository.ListArtifactSignaturesRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ListArtifactSignaturesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.ListArtifactSignaturesResponse.repeatedFields_, null);
};
goog.inherits(proto.repository.ListArtifactSignaturesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ListArtifactSignaturesResponse.displayName = 'proto.repository.ListArtifactSignaturesResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.InstalledPackageRevision = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.InstalledPackageRevision, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.InstalledPackageRevision.displayName = 'proto.repository.InstalledPackageRevision';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.RecordInstalledRevisionRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.RecordInstalledRevisionRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.RecordInstalledRevisionRequest.displayName = 'proto.repository.RecordInstalledRevisionRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.RecordInstalledRevisionResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.RecordInstalledRevisionResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.RecordInstalledRevisionResponse.displayName = 'proto.repository.RecordInstalledRevisionResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ListInstalledRevisionsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ListInstalledRevisionsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ListInstalledRevisionsRequest.displayName = 'proto.repository.ListInstalledRevisionsRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ListInstalledRevisionsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.ListInstalledRevisionsResponse.repeatedFields_, null);
};
goog.inherits(proto.repository.ListInstalledRevisionsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ListInstalledRevisionsResponse.displayName = 'proto.repository.ListInstalledRevisionsResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.RollbackEligibility = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.RollbackEligibility, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.RollbackEligibility.displayName = 'proto.repository.RollbackEligibility';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.RollbackCandidate = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.RollbackCandidate, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.RollbackCandidate.displayName = 'proto.repository.RollbackCandidate';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ListRollbackCandidatesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ListRollbackCandidatesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ListRollbackCandidatesRequest.displayName = 'proto.repository.ListRollbackCandidatesRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ListRollbackCandidatesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.ListRollbackCandidatesResponse.repeatedFields_, null);
};
goog.inherits(proto.repository.ListRollbackCandidatesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ListRollbackCandidatesResponse.displayName = 'proto.repository.ListRollbackCandidatesResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.SignaturePolicy = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.SignaturePolicy.repeatedFields_, null);
};
goog.inherits(proto.repository.SignaturePolicy, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.SignaturePolicy.displayName = 'proto.repository.SignaturePolicy';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.PackageConfigReceipt = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.PackageConfigReceipt, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.PackageConfigReceipt.displayName = 'proto.repository.PackageConfigReceipt';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.RecordConfigReceiptRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.RecordConfigReceiptRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.RecordConfigReceiptRequest.displayName = 'proto.repository.RecordConfigReceiptRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.RecordConfigReceiptResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.RecordConfigReceiptResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.RecordConfigReceiptResponse.displayName = 'proto.repository.RecordConfigReceiptResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ListConfigReceiptsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ListConfigReceiptsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ListConfigReceiptsRequest.displayName = 'proto.repository.ListConfigReceiptsRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ListConfigReceiptsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.ListConfigReceiptsResponse.repeatedFields_, null);
};
goog.inherits(proto.repository.ListConfigReceiptsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ListConfigReceiptsResponse.displayName = 'proto.repository.ListConfigReceiptsResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.RepositoryFinding = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.RepositoryFinding, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.RepositoryFinding.displayName = 'proto.repository.RepositoryFinding';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ListRepositoryFindingsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.ListRepositoryFindingsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ListRepositoryFindingsRequest.displayName = 'proto.repository.ListRepositoryFindingsRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.ListRepositoryFindingsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.ListRepositoryFindingsResponse.repeatedFields_, null);
};
goog.inherits(proto.repository.ListRepositoryFindingsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.ListRepositoryFindingsResponse.displayName = 'proto.repository.ListRepositoryFindingsResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.DependencyHealthProto = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.DependencyHealthProto.repeatedFields_, null);
};
goog.inherits(proto.repository.DependencyHealthProto, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.DependencyHealthProto.displayName = 'proto.repository.DependencyHealthProto';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.CapabilityHealthProto = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.CapabilityHealthProto, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.CapabilityHealthProto.displayName = 'proto.repository.CapabilityHealthProto';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.GetRepositoryStatusRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.repository.GetRepositoryStatusRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.GetRepositoryStatusRequest.displayName = 'proto.repository.GetRepositoryStatusRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.repository.GetRepositoryStatusResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.repository.GetRepositoryStatusResponse.repeatedFields_, null);
};
goog.inherits(proto.repository.GetRepositoryStatusResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.repository.GetRepositoryStatusResponse.displayName = 'proto.repository.GetRepositoryStatusResponse';
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
proto.repository.ArtifactRef.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ArtifactRef.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ArtifactRef} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ArtifactRef.toObject = function(includeInstance, msg) {
  var f, obj = {
publisherId: jspb.Message.getFieldWithDefault(msg, 1, ""),
name: jspb.Message.getFieldWithDefault(msg, 2, ""),
version: jspb.Message.getFieldWithDefault(msg, 3, ""),
platform: jspb.Message.getFieldWithDefault(msg, 4, ""),
kind: jspb.Message.getFieldWithDefault(msg, 5, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ArtifactRef}
 */
proto.repository.ArtifactRef.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ArtifactRef;
  return proto.repository.ArtifactRef.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ArtifactRef} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ArtifactRef}
 */
proto.repository.ArtifactRef.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisherId(value);
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
      msg.setPlatform(value);
      break;
    case 5:
      var value = /** @type {!proto.repository.ArtifactKind} */ (reader.readEnum());
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
proto.repository.ArtifactRef.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ArtifactRef.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ArtifactRef} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ArtifactRef.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublisherId();
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
  f = message.getPlatform();
  if (f.length > 0) {
    writer.writeString(
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
};


/**
 * optional string publisher_id = 1;
 * @return {string}
 */
proto.repository.ArtifactRef.prototype.getPublisherId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactRef} returns this
 */
proto.repository.ArtifactRef.prototype.setPublisherId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string name = 2;
 * @return {string}
 */
proto.repository.ArtifactRef.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactRef} returns this
 */
proto.repository.ArtifactRef.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string version = 3;
 * @return {string}
 */
proto.repository.ArtifactRef.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactRef} returns this
 */
proto.repository.ArtifactRef.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string platform = 4;
 * @return {string}
 */
proto.repository.ArtifactRef.prototype.getPlatform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactRef} returns this
 */
proto.repository.ArtifactRef.prototype.setPlatform = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional ArtifactKind kind = 5;
 * @return {!proto.repository.ArtifactKind}
 */
proto.repository.ArtifactRef.prototype.getKind = function() {
  return /** @type {!proto.repository.ArtifactKind} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {!proto.repository.ArtifactKind} value
 * @return {!proto.repository.ArtifactRef} returns this
 */
proto.repository.ArtifactRef.prototype.setKind = function(value) {
  return jspb.Message.setProto3EnumField(this, 5, value);
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
proto.repository.ArtifactDependencyRef.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ArtifactDependencyRef.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ArtifactDependencyRef} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ArtifactDependencyRef.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, ""),
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
 * @return {!proto.repository.ArtifactDependencyRef}
 */
proto.repository.ArtifactDependencyRef.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ArtifactDependencyRef;
  return proto.repository.ArtifactDependencyRef.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ArtifactDependencyRef} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ArtifactDependencyRef}
 */
proto.repository.ArtifactDependencyRef.deserializeBinaryFromReader = function(msg, reader) {
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
proto.repository.ArtifactDependencyRef.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ArtifactDependencyRef.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ArtifactDependencyRef} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ArtifactDependencyRef.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
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
 * optional string name = 1;
 * @return {string}
 */
proto.repository.ArtifactDependencyRef.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactDependencyRef} returns this
 */
proto.repository.ArtifactDependencyRef.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string publisher_id = 2;
 * @return {string}
 */
proto.repository.ArtifactDependencyRef.prototype.getPublisherId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactDependencyRef} returns this
 */
proto.repository.ArtifactDependencyRef.prototype.setPublisherId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.ArtifactManifest.repeatedFields_ = [10,11,13,15,50,55,56,59,60,70];

/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.repository.ArtifactManifest.oneofGroups_ = [[30,31,32]];

/**
 * @enum {number}
 */
proto.repository.ArtifactManifest.TypeDetailCase = {
  TYPE_DETAIL_NOT_SET: 0,
  SERVICE_DETAIL: 30,
  APPLICATION_DETAIL: 31,
  INFRASTRUCTURE_DETAIL: 32
};

/**
 * @return {proto.repository.ArtifactManifest.TypeDetailCase}
 */
proto.repository.ArtifactManifest.prototype.getTypeDetailCase = function() {
  return /** @type {proto.repository.ArtifactManifest.TypeDetailCase} */(jspb.Message.computeOneofCase(this, proto.repository.ArtifactManifest.oneofGroups_[0]));
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
proto.repository.ArtifactManifest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ArtifactManifest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ArtifactManifest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ArtifactManifest.toObject = function(includeInstance, msg) {
  var f, obj = {
ref: (f = msg.getRef()) && proto.repository.ArtifactRef.toObject(includeInstance, f),
checksum: jspb.Message.getFieldWithDefault(msg, 2, ""),
sizeBytes: jspb.Message.getFieldWithDefault(msg, 3, 0),
modifiedUnix: jspb.Message.getFieldWithDefault(msg, 4, 0),
buildNumber: jspb.Message.getFieldWithDefault(msg, 5, 0),
providesList: (f = jspb.Message.getRepeatedField(msg, 10)) == null ? undefined : f,
requiresList: (f = jspb.Message.getRepeatedField(msg, 11)) == null ? undefined : f,
defaultsMap: (f = msg.getDefaultsMap()) ? f.toObject(includeInstance, undefined) : [],
entrypointsList: (f = jspb.Message.getRepeatedField(msg, 13)) == null ? undefined : f,
description: jspb.Message.getFieldWithDefault(msg, 14, ""),
keywordsList: (f = jspb.Message.getRepeatedField(msg, 15)) == null ? undefined : f,
icon: jspb.Message.getFieldWithDefault(msg, 16, ""),
alias: jspb.Message.getFieldWithDefault(msg, 17, ""),
license: jspb.Message.getFieldWithDefault(msg, 18, ""),
minGlobularVersion: jspb.Message.getFieldWithDefault(msg, 19, ""),
publishedUnix: jspb.Message.getFieldWithDefault(msg, 20, 0),
buildCommit: jspb.Message.getFieldWithDefault(msg, 21, ""),
buildTimestampUnix: jspb.Message.getFieldWithDefault(msg, 22, 0),
buildSource: jspb.Message.getFieldWithDefault(msg, 23, ""),
buildNotes: jspb.Message.getFieldWithDefault(msg, 24, ""),
serviceDetail: (f = msg.getServiceDetail()) && proto.repository.ServiceDetail.toObject(includeInstance, f),
applicationDetail: (f = msg.getApplicationDetail()) && proto.repository.ApplicationDetail.toObject(includeInstance, f),
infrastructureDetail: (f = msg.getInfrastructureDetail()) && proto.repository.InfrastructureDetail.toObject(includeInstance, f),
profilesList: (f = jspb.Message.getRepeatedField(msg, 50)) == null ? undefined : f,
priority: jspb.Message.getFieldWithDefault(msg, 51, 0),
installMode: jspb.Message.getFieldWithDefault(msg, 52, ""),
managedUnit: jspb.Message.getBooleanFieldWithDefault(msg, 53, false),
systemdUnit: jspb.Message.getFieldWithDefault(msg, 54, ""),
runtimeLocalDependenciesList: (f = jspb.Message.getRepeatedField(msg, 55)) == null ? undefined : f,
installDependenciesList: (f = jspb.Message.getRepeatedField(msg, 56)) == null ? undefined : f,
healthCheckUnit: jspb.Message.getFieldWithDefault(msg, 57, ""),
healthCheckPort: jspb.Message.getFieldWithDefault(msg, 58, 0),
hardDepsList: jspb.Message.toObjectList(msg.getHardDepsList(),
    proto.repository.ArtifactDependencyRef.toObject, includeInstance),
runtimeUsesList: (f = jspb.Message.getRepeatedField(msg, 60)) == null ? undefined : f,
publishState: jspb.Message.getFieldWithDefault(msg, 40, 0),
provenance: (f = msg.getProvenance()) && proto.repository.ProvenanceRecord.toObject(includeInstance, f),
buildId: jspb.Message.getFieldWithDefault(msg, 42, ""),
provisional: jspb.Message.getBooleanFieldWithDefault(msg, 43, false),
upstreamImport: (f = msg.getUpstreamImport()) && proto.repository.UpstreamImportRecord.toObject(includeInstance, f),
entrypointChecksum: jspb.Message.getFieldWithDefault(msg, 44, ""),
channel: jspb.Message.getFieldWithDefault(msg, 45, 0),
configsList: jspb.Message.toObjectList(msg.getConfigsList(),
    proto.repository.PackageConfigFile.toObject, includeInstance),
signatureKeyId: jspb.Message.getFieldWithDefault(msg, 71, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ArtifactManifest}
 */
proto.repository.ArtifactManifest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ArtifactManifest;
  return proto.repository.ArtifactManifest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ArtifactManifest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ArtifactManifest}
 */
proto.repository.ArtifactManifest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactRef;
      reader.readMessage(value,proto.repository.ArtifactRef.deserializeBinaryFromReader);
      msg.setRef(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setChecksum(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setSizeBytes(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setModifiedUnix(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.addProvides(value);
      break;
    case 11:
      var value = /** @type {string} */ (reader.readString());
      msg.addRequires(value);
      break;
    case 12:
      var value = msg.getDefaultsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
      break;
    case 13:
      var value = /** @type {string} */ (reader.readString());
      msg.addEntrypoints(value);
      break;
    case 14:
      var value = /** @type {string} */ (reader.readString());
      msg.setDescription(value);
      break;
    case 15:
      var value = /** @type {string} */ (reader.readString());
      msg.addKeywords(value);
      break;
    case 16:
      var value = /** @type {string} */ (reader.readString());
      msg.setIcon(value);
      break;
    case 17:
      var value = /** @type {string} */ (reader.readString());
      msg.setAlias(value);
      break;
    case 18:
      var value = /** @type {string} */ (reader.readString());
      msg.setLicense(value);
      break;
    case 19:
      var value = /** @type {string} */ (reader.readString());
      msg.setMinGlobularVersion(value);
      break;
    case 20:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setPublishedUnix(value);
      break;
    case 21:
      var value = /** @type {string} */ (reader.readString());
      msg.setBuildCommit(value);
      break;
    case 22:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildTimestampUnix(value);
      break;
    case 23:
      var value = /** @type {string} */ (reader.readString());
      msg.setBuildSource(value);
      break;
    case 24:
      var value = /** @type {string} */ (reader.readString());
      msg.setBuildNotes(value);
      break;
    case 30:
      var value = new proto.repository.ServiceDetail;
      reader.readMessage(value,proto.repository.ServiceDetail.deserializeBinaryFromReader);
      msg.setServiceDetail(value);
      break;
    case 31:
      var value = new proto.repository.ApplicationDetail;
      reader.readMessage(value,proto.repository.ApplicationDetail.deserializeBinaryFromReader);
      msg.setApplicationDetail(value);
      break;
    case 32:
      var value = new proto.repository.InfrastructureDetail;
      reader.readMessage(value,proto.repository.InfrastructureDetail.deserializeBinaryFromReader);
      msg.setInfrastructureDetail(value);
      break;
    case 50:
      var value = /** @type {string} */ (reader.readString());
      msg.addProfiles(value);
      break;
    case 51:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setPriority(value);
      break;
    case 52:
      var value = /** @type {string} */ (reader.readString());
      msg.setInstallMode(value);
      break;
    case 53:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setManagedUnit(value);
      break;
    case 54:
      var value = /** @type {string} */ (reader.readString());
      msg.setSystemdUnit(value);
      break;
    case 55:
      var value = /** @type {string} */ (reader.readString());
      msg.addRuntimeLocalDependencies(value);
      break;
    case 56:
      var value = /** @type {string} */ (reader.readString());
      msg.addInstallDependencies(value);
      break;
    case 57:
      var value = /** @type {string} */ (reader.readString());
      msg.setHealthCheckUnit(value);
      break;
    case 58:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setHealthCheckPort(value);
      break;
    case 59:
      var value = new proto.repository.ArtifactDependencyRef;
      reader.readMessage(value,proto.repository.ArtifactDependencyRef.deserializeBinaryFromReader);
      msg.addHardDeps(value);
      break;
    case 60:
      var value = /** @type {string} */ (reader.readString());
      msg.addRuntimeUses(value);
      break;
    case 40:
      var value = /** @type {!proto.repository.PublishState} */ (reader.readEnum());
      msg.setPublishState(value);
      break;
    case 41:
      var value = new proto.repository.ProvenanceRecord;
      reader.readMessage(value,proto.repository.ProvenanceRecord.deserializeBinaryFromReader);
      msg.setProvenance(value);
      break;
    case 42:
      var value = /** @type {string} */ (reader.readString());
      msg.setBuildId(value);
      break;
    case 43:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setProvisional(value);
      break;
    case 61:
      var value = new proto.repository.UpstreamImportRecord;
      reader.readMessage(value,proto.repository.UpstreamImportRecord.deserializeBinaryFromReader);
      msg.setUpstreamImport(value);
      break;
    case 44:
      var value = /** @type {string} */ (reader.readString());
      msg.setEntrypointChecksum(value);
      break;
    case 45:
      var value = /** @type {!proto.repository.ArtifactChannel} */ (reader.readEnum());
      msg.setChannel(value);
      break;
    case 70:
      var value = new proto.repository.PackageConfigFile;
      reader.readMessage(value,proto.repository.PackageConfigFile.deserializeBinaryFromReader);
      msg.addConfigs(value);
      break;
    case 71:
      var value = /** @type {string} */ (reader.readString());
      msg.setSignatureKeyId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.ArtifactManifest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ArtifactManifest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ArtifactManifest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ArtifactManifest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRef();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.ArtifactRef.serializeBinaryToWriter
    );
  }
  f = message.getChecksum();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getSizeBytes();
  if (f !== 0) {
    writer.writeInt64(
      3,
      f
    );
  }
  f = message.getModifiedUnix();
  if (f !== 0) {
    writer.writeInt64(
      4,
      f
    );
  }
  f = message.getBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
  f = message.getProvidesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      10,
      f
    );
  }
  f = message.getRequiresList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      11,
      f
    );
  }
  f = message.getDefaultsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(12, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
  f = message.getEntrypointsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      13,
      f
    );
  }
  f = message.getDescription();
  if (f.length > 0) {
    writer.writeString(
      14,
      f
    );
  }
  f = message.getKeywordsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      15,
      f
    );
  }
  f = message.getIcon();
  if (f.length > 0) {
    writer.writeString(
      16,
      f
    );
  }
  f = message.getAlias();
  if (f.length > 0) {
    writer.writeString(
      17,
      f
    );
  }
  f = message.getLicense();
  if (f.length > 0) {
    writer.writeString(
      18,
      f
    );
  }
  f = message.getMinGlobularVersion();
  if (f.length > 0) {
    writer.writeString(
      19,
      f
    );
  }
  f = message.getPublishedUnix();
  if (f !== 0) {
    writer.writeInt64(
      20,
      f
    );
  }
  f = message.getBuildCommit();
  if (f.length > 0) {
    writer.writeString(
      21,
      f
    );
  }
  f = message.getBuildTimestampUnix();
  if (f !== 0) {
    writer.writeInt64(
      22,
      f
    );
  }
  f = message.getBuildSource();
  if (f.length > 0) {
    writer.writeString(
      23,
      f
    );
  }
  f = message.getBuildNotes();
  if (f.length > 0) {
    writer.writeString(
      24,
      f
    );
  }
  f = message.getServiceDetail();
  if (f != null) {
    writer.writeMessage(
      30,
      f,
      proto.repository.ServiceDetail.serializeBinaryToWriter
    );
  }
  f = message.getApplicationDetail();
  if (f != null) {
    writer.writeMessage(
      31,
      f,
      proto.repository.ApplicationDetail.serializeBinaryToWriter
    );
  }
  f = message.getInfrastructureDetail();
  if (f != null) {
    writer.writeMessage(
      32,
      f,
      proto.repository.InfrastructureDetail.serializeBinaryToWriter
    );
  }
  f = message.getProfilesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      50,
      f
    );
  }
  f = message.getPriority();
  if (f !== 0) {
    writer.writeInt32(
      51,
      f
    );
  }
  f = message.getInstallMode();
  if (f.length > 0) {
    writer.writeString(
      52,
      f
    );
  }
  f = message.getManagedUnit();
  if (f) {
    writer.writeBool(
      53,
      f
    );
  }
  f = message.getSystemdUnit();
  if (f.length > 0) {
    writer.writeString(
      54,
      f
    );
  }
  f = message.getRuntimeLocalDependenciesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      55,
      f
    );
  }
  f = message.getInstallDependenciesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      56,
      f
    );
  }
  f = message.getHealthCheckUnit();
  if (f.length > 0) {
    writer.writeString(
      57,
      f
    );
  }
  f = message.getHealthCheckPort();
  if (f !== 0) {
    writer.writeInt32(
      58,
      f
    );
  }
  f = message.getHardDepsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      59,
      f,
      proto.repository.ArtifactDependencyRef.serializeBinaryToWriter
    );
  }
  f = message.getRuntimeUsesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      60,
      f
    );
  }
  f = message.getPublishState();
  if (f !== 0.0) {
    writer.writeEnum(
      40,
      f
    );
  }
  f = message.getProvenance();
  if (f != null) {
    writer.writeMessage(
      41,
      f,
      proto.repository.ProvenanceRecord.serializeBinaryToWriter
    );
  }
  f = message.getBuildId();
  if (f.length > 0) {
    writer.writeString(
      42,
      f
    );
  }
  f = message.getProvisional();
  if (f) {
    writer.writeBool(
      43,
      f
    );
  }
  f = message.getUpstreamImport();
  if (f != null) {
    writer.writeMessage(
      61,
      f,
      proto.repository.UpstreamImportRecord.serializeBinaryToWriter
    );
  }
  f = message.getEntrypointChecksum();
  if (f.length > 0) {
    writer.writeString(
      44,
      f
    );
  }
  f = message.getChannel();
  if (f !== 0.0) {
    writer.writeEnum(
      45,
      f
    );
  }
  f = message.getConfigsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      70,
      f,
      proto.repository.PackageConfigFile.serializeBinaryToWriter
    );
  }
  f = message.getSignatureKeyId();
  if (f.length > 0) {
    writer.writeString(
      71,
      f
    );
  }
};


/**
 * optional ArtifactRef ref = 1;
 * @return {?proto.repository.ArtifactRef}
 */
proto.repository.ArtifactManifest.prototype.getRef = function() {
  return /** @type{?proto.repository.ArtifactRef} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactRef, 1));
};


/**
 * @param {?proto.repository.ArtifactRef|undefined} value
 * @return {!proto.repository.ArtifactManifest} returns this
*/
proto.repository.ArtifactManifest.prototype.setRef = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.clearRef = function() {
  return this.setRef(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.ArtifactManifest.prototype.hasRef = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string checksum = 2;
 * @return {string}
 */
proto.repository.ArtifactManifest.prototype.getChecksum = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setChecksum = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int64 size_bytes = 3;
 * @return {number}
 */
proto.repository.ArtifactManifest.prototype.getSizeBytes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setSizeBytes = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional int64 modified_unix = 4;
 * @return {number}
 */
proto.repository.ArtifactManifest.prototype.getModifiedUnix = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setModifiedUnix = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional int64 build_number = 5;
 * @return {number}
 */
proto.repository.ArtifactManifest.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setBuildNumber = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * repeated string provides = 10;
 * @return {!Array<string>}
 */
proto.repository.ArtifactManifest.prototype.getProvidesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 10));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setProvidesList = function(value) {
  return jspb.Message.setField(this, 10, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.addProvides = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 10, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.clearProvidesList = function() {
  return this.setProvidesList([]);
};


/**
 * repeated string requires = 11;
 * @return {!Array<string>}
 */
proto.repository.ArtifactManifest.prototype.getRequiresList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 11));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setRequiresList = function(value) {
  return jspb.Message.setField(this, 11, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.addRequires = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 11, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.clearRequiresList = function() {
  return this.setRequiresList([]);
};


/**
 * map<string, string> defaults = 12;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.repository.ArtifactManifest.prototype.getDefaultsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 12, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.clearDefaultsMap = function() {
  this.getDefaultsMap().clear();
  return this;
};


/**
 * repeated string entrypoints = 13;
 * @return {!Array<string>}
 */
proto.repository.ArtifactManifest.prototype.getEntrypointsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 13));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setEntrypointsList = function(value) {
  return jspb.Message.setField(this, 13, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.addEntrypoints = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 13, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.clearEntrypointsList = function() {
  return this.setEntrypointsList([]);
};


/**
 * optional string description = 14;
 * @return {string}
 */
proto.repository.ArtifactManifest.prototype.getDescription = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 14, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setDescription = function(value) {
  return jspb.Message.setProto3StringField(this, 14, value);
};


/**
 * repeated string keywords = 15;
 * @return {!Array<string>}
 */
proto.repository.ArtifactManifest.prototype.getKeywordsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 15));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setKeywordsList = function(value) {
  return jspb.Message.setField(this, 15, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.addKeywords = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 15, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.clearKeywordsList = function() {
  return this.setKeywordsList([]);
};


/**
 * optional string icon = 16;
 * @return {string}
 */
proto.repository.ArtifactManifest.prototype.getIcon = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 16, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setIcon = function(value) {
  return jspb.Message.setProto3StringField(this, 16, value);
};


/**
 * optional string alias = 17;
 * @return {string}
 */
proto.repository.ArtifactManifest.prototype.getAlias = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 17, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setAlias = function(value) {
  return jspb.Message.setProto3StringField(this, 17, value);
};


/**
 * optional string license = 18;
 * @return {string}
 */
proto.repository.ArtifactManifest.prototype.getLicense = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 18, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setLicense = function(value) {
  return jspb.Message.setProto3StringField(this, 18, value);
};


/**
 * optional string min_globular_version = 19;
 * @return {string}
 */
proto.repository.ArtifactManifest.prototype.getMinGlobularVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 19, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setMinGlobularVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 19, value);
};


/**
 * optional int64 published_unix = 20;
 * @return {number}
 */
proto.repository.ArtifactManifest.prototype.getPublishedUnix = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 20, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setPublishedUnix = function(value) {
  return jspb.Message.setProto3IntField(this, 20, value);
};


/**
 * optional string build_commit = 21;
 * @return {string}
 */
proto.repository.ArtifactManifest.prototype.getBuildCommit = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 21, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setBuildCommit = function(value) {
  return jspb.Message.setProto3StringField(this, 21, value);
};


/**
 * optional int64 build_timestamp_unix = 22;
 * @return {number}
 */
proto.repository.ArtifactManifest.prototype.getBuildTimestampUnix = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 22, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setBuildTimestampUnix = function(value) {
  return jspb.Message.setProto3IntField(this, 22, value);
};


/**
 * optional string build_source = 23;
 * @return {string}
 */
proto.repository.ArtifactManifest.prototype.getBuildSource = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 23, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setBuildSource = function(value) {
  return jspb.Message.setProto3StringField(this, 23, value);
};


/**
 * optional string build_notes = 24;
 * @return {string}
 */
proto.repository.ArtifactManifest.prototype.getBuildNotes = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 24, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setBuildNotes = function(value) {
  return jspb.Message.setProto3StringField(this, 24, value);
};


/**
 * optional ServiceDetail service_detail = 30;
 * @return {?proto.repository.ServiceDetail}
 */
proto.repository.ArtifactManifest.prototype.getServiceDetail = function() {
  return /** @type{?proto.repository.ServiceDetail} */ (
    jspb.Message.getWrapperField(this, proto.repository.ServiceDetail, 30));
};


/**
 * @param {?proto.repository.ServiceDetail|undefined} value
 * @return {!proto.repository.ArtifactManifest} returns this
*/
proto.repository.ArtifactManifest.prototype.setServiceDetail = function(value) {
  return jspb.Message.setOneofWrapperField(this, 30, proto.repository.ArtifactManifest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.clearServiceDetail = function() {
  return this.setServiceDetail(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.ArtifactManifest.prototype.hasServiceDetail = function() {
  return jspb.Message.getField(this, 30) != null;
};


/**
 * optional ApplicationDetail application_detail = 31;
 * @return {?proto.repository.ApplicationDetail}
 */
proto.repository.ArtifactManifest.prototype.getApplicationDetail = function() {
  return /** @type{?proto.repository.ApplicationDetail} */ (
    jspb.Message.getWrapperField(this, proto.repository.ApplicationDetail, 31));
};


/**
 * @param {?proto.repository.ApplicationDetail|undefined} value
 * @return {!proto.repository.ArtifactManifest} returns this
*/
proto.repository.ArtifactManifest.prototype.setApplicationDetail = function(value) {
  return jspb.Message.setOneofWrapperField(this, 31, proto.repository.ArtifactManifest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.clearApplicationDetail = function() {
  return this.setApplicationDetail(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.ArtifactManifest.prototype.hasApplicationDetail = function() {
  return jspb.Message.getField(this, 31) != null;
};


/**
 * optional InfrastructureDetail infrastructure_detail = 32;
 * @return {?proto.repository.InfrastructureDetail}
 */
proto.repository.ArtifactManifest.prototype.getInfrastructureDetail = function() {
  return /** @type{?proto.repository.InfrastructureDetail} */ (
    jspb.Message.getWrapperField(this, proto.repository.InfrastructureDetail, 32));
};


/**
 * @param {?proto.repository.InfrastructureDetail|undefined} value
 * @return {!proto.repository.ArtifactManifest} returns this
*/
proto.repository.ArtifactManifest.prototype.setInfrastructureDetail = function(value) {
  return jspb.Message.setOneofWrapperField(this, 32, proto.repository.ArtifactManifest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.clearInfrastructureDetail = function() {
  return this.setInfrastructureDetail(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.ArtifactManifest.prototype.hasInfrastructureDetail = function() {
  return jspb.Message.getField(this, 32) != null;
};


/**
 * repeated string profiles = 50;
 * @return {!Array<string>}
 */
proto.repository.ArtifactManifest.prototype.getProfilesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 50));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setProfilesList = function(value) {
  return jspb.Message.setField(this, 50, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.addProfiles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 50, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.clearProfilesList = function() {
  return this.setProfilesList([]);
};


/**
 * optional int32 priority = 51;
 * @return {number}
 */
proto.repository.ArtifactManifest.prototype.getPriority = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 51, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setPriority = function(value) {
  return jspb.Message.setProto3IntField(this, 51, value);
};


/**
 * optional string install_mode = 52;
 * @return {string}
 */
proto.repository.ArtifactManifest.prototype.getInstallMode = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 52, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setInstallMode = function(value) {
  return jspb.Message.setProto3StringField(this, 52, value);
};


/**
 * optional bool managed_unit = 53;
 * @return {boolean}
 */
proto.repository.ArtifactManifest.prototype.getManagedUnit = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 53, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setManagedUnit = function(value) {
  return jspb.Message.setProto3BooleanField(this, 53, value);
};


/**
 * optional string systemd_unit = 54;
 * @return {string}
 */
proto.repository.ArtifactManifest.prototype.getSystemdUnit = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 54, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setSystemdUnit = function(value) {
  return jspb.Message.setProto3StringField(this, 54, value);
};


/**
 * repeated string runtime_local_dependencies = 55;
 * @return {!Array<string>}
 */
proto.repository.ArtifactManifest.prototype.getRuntimeLocalDependenciesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 55));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setRuntimeLocalDependenciesList = function(value) {
  return jspb.Message.setField(this, 55, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.addRuntimeLocalDependencies = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 55, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.clearRuntimeLocalDependenciesList = function() {
  return this.setRuntimeLocalDependenciesList([]);
};


/**
 * repeated string install_dependencies = 56;
 * @return {!Array<string>}
 */
proto.repository.ArtifactManifest.prototype.getInstallDependenciesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 56));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setInstallDependenciesList = function(value) {
  return jspb.Message.setField(this, 56, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.addInstallDependencies = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 56, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.clearInstallDependenciesList = function() {
  return this.setInstallDependenciesList([]);
};


/**
 * optional string health_check_unit = 57;
 * @return {string}
 */
proto.repository.ArtifactManifest.prototype.getHealthCheckUnit = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 57, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setHealthCheckUnit = function(value) {
  return jspb.Message.setProto3StringField(this, 57, value);
};


/**
 * optional int32 health_check_port = 58;
 * @return {number}
 */
proto.repository.ArtifactManifest.prototype.getHealthCheckPort = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 58, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setHealthCheckPort = function(value) {
  return jspb.Message.setProto3IntField(this, 58, value);
};


/**
 * repeated ArtifactDependencyRef hard_deps = 59;
 * @return {!Array<!proto.repository.ArtifactDependencyRef>}
 */
proto.repository.ArtifactManifest.prototype.getHardDepsList = function() {
  return /** @type{!Array<!proto.repository.ArtifactDependencyRef>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.repository.ArtifactDependencyRef, 59));
};


/**
 * @param {!Array<!proto.repository.ArtifactDependencyRef>} value
 * @return {!proto.repository.ArtifactManifest} returns this
*/
proto.repository.ArtifactManifest.prototype.setHardDepsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 59, value);
};


/**
 * @param {!proto.repository.ArtifactDependencyRef=} opt_value
 * @param {number=} opt_index
 * @return {!proto.repository.ArtifactDependencyRef}
 */
proto.repository.ArtifactManifest.prototype.addHardDeps = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 59, opt_value, proto.repository.ArtifactDependencyRef, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.clearHardDepsList = function() {
  return this.setHardDepsList([]);
};


/**
 * repeated string runtime_uses = 60;
 * @return {!Array<string>}
 */
proto.repository.ArtifactManifest.prototype.getRuntimeUsesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 60));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setRuntimeUsesList = function(value) {
  return jspb.Message.setField(this, 60, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.addRuntimeUses = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 60, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.clearRuntimeUsesList = function() {
  return this.setRuntimeUsesList([]);
};


/**
 * optional PublishState publish_state = 40;
 * @return {!proto.repository.PublishState}
 */
proto.repository.ArtifactManifest.prototype.getPublishState = function() {
  return /** @type {!proto.repository.PublishState} */ (jspb.Message.getFieldWithDefault(this, 40, 0));
};


/**
 * @param {!proto.repository.PublishState} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setPublishState = function(value) {
  return jspb.Message.setProto3EnumField(this, 40, value);
};


/**
 * optional ProvenanceRecord provenance = 41;
 * @return {?proto.repository.ProvenanceRecord}
 */
proto.repository.ArtifactManifest.prototype.getProvenance = function() {
  return /** @type{?proto.repository.ProvenanceRecord} */ (
    jspb.Message.getWrapperField(this, proto.repository.ProvenanceRecord, 41));
};


/**
 * @param {?proto.repository.ProvenanceRecord|undefined} value
 * @return {!proto.repository.ArtifactManifest} returns this
*/
proto.repository.ArtifactManifest.prototype.setProvenance = function(value) {
  return jspb.Message.setWrapperField(this, 41, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.clearProvenance = function() {
  return this.setProvenance(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.ArtifactManifest.prototype.hasProvenance = function() {
  return jspb.Message.getField(this, 41) != null;
};


/**
 * optional string build_id = 42;
 * @return {string}
 */
proto.repository.ArtifactManifest.prototype.getBuildId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 42, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setBuildId = function(value) {
  return jspb.Message.setProto3StringField(this, 42, value);
};


/**
 * optional bool provisional = 43;
 * @return {boolean}
 */
proto.repository.ArtifactManifest.prototype.getProvisional = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 43, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setProvisional = function(value) {
  return jspb.Message.setProto3BooleanField(this, 43, value);
};


/**
 * optional UpstreamImportRecord upstream_import = 61;
 * @return {?proto.repository.UpstreamImportRecord}
 */
proto.repository.ArtifactManifest.prototype.getUpstreamImport = function() {
  return /** @type{?proto.repository.UpstreamImportRecord} */ (
    jspb.Message.getWrapperField(this, proto.repository.UpstreamImportRecord, 61));
};


/**
 * @param {?proto.repository.UpstreamImportRecord|undefined} value
 * @return {!proto.repository.ArtifactManifest} returns this
*/
proto.repository.ArtifactManifest.prototype.setUpstreamImport = function(value) {
  return jspb.Message.setWrapperField(this, 61, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.clearUpstreamImport = function() {
  return this.setUpstreamImport(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.ArtifactManifest.prototype.hasUpstreamImport = function() {
  return jspb.Message.getField(this, 61) != null;
};


/**
 * optional string entrypoint_checksum = 44;
 * @return {string}
 */
proto.repository.ArtifactManifest.prototype.getEntrypointChecksum = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 44, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setEntrypointChecksum = function(value) {
  return jspb.Message.setProto3StringField(this, 44, value);
};


/**
 * optional ArtifactChannel channel = 45;
 * @return {!proto.repository.ArtifactChannel}
 */
proto.repository.ArtifactManifest.prototype.getChannel = function() {
  return /** @type {!proto.repository.ArtifactChannel} */ (jspb.Message.getFieldWithDefault(this, 45, 0));
};


/**
 * @param {!proto.repository.ArtifactChannel} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setChannel = function(value) {
  return jspb.Message.setProto3EnumField(this, 45, value);
};


/**
 * repeated PackageConfigFile configs = 70;
 * @return {!Array<!proto.repository.PackageConfigFile>}
 */
proto.repository.ArtifactManifest.prototype.getConfigsList = function() {
  return /** @type{!Array<!proto.repository.PackageConfigFile>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.repository.PackageConfigFile, 70));
};


/**
 * @param {!Array<!proto.repository.PackageConfigFile>} value
 * @return {!proto.repository.ArtifactManifest} returns this
*/
proto.repository.ArtifactManifest.prototype.setConfigsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 70, value);
};


/**
 * @param {!proto.repository.PackageConfigFile=} opt_value
 * @param {number=} opt_index
 * @return {!proto.repository.PackageConfigFile}
 */
proto.repository.ArtifactManifest.prototype.addConfigs = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 70, opt_value, proto.repository.PackageConfigFile, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.clearConfigsList = function() {
  return this.setConfigsList([]);
};


/**
 * optional string signature_key_id = 71;
 * @return {string}
 */
proto.repository.ArtifactManifest.prototype.getSignatureKeyId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 71, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactManifest} returns this
 */
proto.repository.ArtifactManifest.prototype.setSignatureKeyId = function(value) {
  return jspb.Message.setProto3StringField(this, 71, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.ServiceDetail.repeatedFields_ = [5];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.ServiceDetail.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ServiceDetail.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ServiceDetail} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ServiceDetail.toObject = function(includeInstance, msg) {
  var f, obj = {
protoFile: jspb.Message.getFieldWithDefault(msg, 1, ""),
grpcServiceName: jspb.Message.getFieldWithDefault(msg, 2, ""),
defaultPort: jspb.Message.getFieldWithDefault(msg, 3, 0),
systemdUnit: jspb.Message.getFieldWithDefault(msg, 4, ""),
serviceDependenciesList: (f = jspb.Message.getRepeatedField(msg, 5)) == null ? undefined : f
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ServiceDetail}
 */
proto.repository.ServiceDetail.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ServiceDetail;
  return proto.repository.ServiceDetail.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ServiceDetail} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ServiceDetail}
 */
proto.repository.ServiceDetail.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setProtoFile(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setGrpcServiceName(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setDefaultPort(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setSystemdUnit(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.addServiceDependencies(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.ServiceDetail.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ServiceDetail.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ServiceDetail} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ServiceDetail.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getProtoFile();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getGrpcServiceName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getDefaultPort();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
  f = message.getSystemdUnit();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getServiceDependenciesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      5,
      f
    );
  }
};


/**
 * optional string proto_file = 1;
 * @return {string}
 */
proto.repository.ServiceDetail.prototype.getProtoFile = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ServiceDetail} returns this
 */
proto.repository.ServiceDetail.prototype.setProtoFile = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string grpc_service_name = 2;
 * @return {string}
 */
proto.repository.ServiceDetail.prototype.getGrpcServiceName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ServiceDetail} returns this
 */
proto.repository.ServiceDetail.prototype.setGrpcServiceName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int32 default_port = 3;
 * @return {number}
 */
proto.repository.ServiceDetail.prototype.getDefaultPort = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ServiceDetail} returns this
 */
proto.repository.ServiceDetail.prototype.setDefaultPort = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional string systemd_unit = 4;
 * @return {string}
 */
proto.repository.ServiceDetail.prototype.getSystemdUnit = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ServiceDetail} returns this
 */
proto.repository.ServiceDetail.prototype.setSystemdUnit = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * repeated string service_dependencies = 5;
 * @return {!Array<string>}
 */
proto.repository.ServiceDetail.prototype.getServiceDependenciesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 5));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.ServiceDetail} returns this
 */
proto.repository.ServiceDetail.prototype.setServiceDependenciesList = function(value) {
  return jspb.Message.setField(this, 5, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.ServiceDetail} returns this
 */
proto.repository.ServiceDetail.prototype.addServiceDependencies = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 5, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ServiceDetail} returns this
 */
proto.repository.ServiceDetail.prototype.clearServiceDependenciesList = function() {
  return this.setServiceDependenciesList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.ApplicationDetail.repeatedFields_ = [3,4,5,7];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.ApplicationDetail.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ApplicationDetail.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ApplicationDetail} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ApplicationDetail.toObject = function(includeInstance, msg) {
  var f, obj = {
route: jspb.Message.getFieldWithDefault(msg, 1, ""),
indexFile: jspb.Message.getFieldWithDefault(msg, 2, ""),
actionsList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f,
rolesList: (f = jspb.Message.getRepeatedField(msg, 4)) == null ? undefined : f,
groupsList: (f = jspb.Message.getRepeatedField(msg, 5)) == null ? undefined : f,
setAsDefault: jspb.Message.getBooleanFieldWithDefault(msg, 6, false),
requiredServicesList: (f = jspb.Message.getRepeatedField(msg, 7)) == null ? undefined : f,
appConfigMap: (f = msg.getAppConfigMap()) ? f.toObject(includeInstance, undefined) : []
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ApplicationDetail}
 */
proto.repository.ApplicationDetail.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ApplicationDetail;
  return proto.repository.ApplicationDetail.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ApplicationDetail} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ApplicationDetail}
 */
proto.repository.ApplicationDetail.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRoute(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexFile(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.addActions(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.addRoles(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.addGroups(value);
      break;
    case 6:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setSetAsDefault(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.addRequiredServices(value);
      break;
    case 8:
      var value = msg.getAppConfigMap();
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
proto.repository.ApplicationDetail.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ApplicationDetail.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ApplicationDetail} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ApplicationDetail.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRoute();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIndexFile();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getActionsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      3,
      f
    );
  }
  f = message.getRolesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      4,
      f
    );
  }
  f = message.getGroupsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      5,
      f
    );
  }
  f = message.getSetAsDefault();
  if (f) {
    writer.writeBool(
      6,
      f
    );
  }
  f = message.getRequiredServicesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      7,
      f
    );
  }
  f = message.getAppConfigMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(8, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
};


/**
 * optional string route = 1;
 * @return {string}
 */
proto.repository.ApplicationDetail.prototype.getRoute = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ApplicationDetail} returns this
 */
proto.repository.ApplicationDetail.prototype.setRoute = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string index_file = 2;
 * @return {string}
 */
proto.repository.ApplicationDetail.prototype.getIndexFile = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ApplicationDetail} returns this
 */
proto.repository.ApplicationDetail.prototype.setIndexFile = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * repeated string actions = 3;
 * @return {!Array<string>}
 */
proto.repository.ApplicationDetail.prototype.getActionsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.ApplicationDetail} returns this
 */
proto.repository.ApplicationDetail.prototype.setActionsList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.ApplicationDetail} returns this
 */
proto.repository.ApplicationDetail.prototype.addActions = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ApplicationDetail} returns this
 */
proto.repository.ApplicationDetail.prototype.clearActionsList = function() {
  return this.setActionsList([]);
};


/**
 * repeated string roles = 4;
 * @return {!Array<string>}
 */
proto.repository.ApplicationDetail.prototype.getRolesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 4));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.ApplicationDetail} returns this
 */
proto.repository.ApplicationDetail.prototype.setRolesList = function(value) {
  return jspb.Message.setField(this, 4, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.ApplicationDetail} returns this
 */
proto.repository.ApplicationDetail.prototype.addRoles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 4, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ApplicationDetail} returns this
 */
proto.repository.ApplicationDetail.prototype.clearRolesList = function() {
  return this.setRolesList([]);
};


/**
 * repeated string groups = 5;
 * @return {!Array<string>}
 */
proto.repository.ApplicationDetail.prototype.getGroupsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 5));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.ApplicationDetail} returns this
 */
proto.repository.ApplicationDetail.prototype.setGroupsList = function(value) {
  return jspb.Message.setField(this, 5, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.ApplicationDetail} returns this
 */
proto.repository.ApplicationDetail.prototype.addGroups = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 5, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ApplicationDetail} returns this
 */
proto.repository.ApplicationDetail.prototype.clearGroupsList = function() {
  return this.setGroupsList([]);
};


/**
 * optional bool set_as_default = 6;
 * @return {boolean}
 */
proto.repository.ApplicationDetail.prototype.getSetAsDefault = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 6, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.ApplicationDetail} returns this
 */
proto.repository.ApplicationDetail.prototype.setSetAsDefault = function(value) {
  return jspb.Message.setProto3BooleanField(this, 6, value);
};


/**
 * repeated string required_services = 7;
 * @return {!Array<string>}
 */
proto.repository.ApplicationDetail.prototype.getRequiredServicesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 7));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.ApplicationDetail} returns this
 */
proto.repository.ApplicationDetail.prototype.setRequiredServicesList = function(value) {
  return jspb.Message.setField(this, 7, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.ApplicationDetail} returns this
 */
proto.repository.ApplicationDetail.prototype.addRequiredServices = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 7, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ApplicationDetail} returns this
 */
proto.repository.ApplicationDetail.prototype.clearRequiredServicesList = function() {
  return this.setRequiredServicesList([]);
};


/**
 * map<string, string> app_config = 8;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.repository.ApplicationDetail.prototype.getAppConfigMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 8, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.repository.ApplicationDetail} returns this
 */
proto.repository.ApplicationDetail.prototype.clearAppConfigMap = function() {
  this.getAppConfigMap().clear();
  return this;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.InfrastructureDetail.repeatedFields_ = [3,6];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.InfrastructureDetail.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.InfrastructureDetail.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.InfrastructureDetail} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.InfrastructureDetail.toObject = function(includeInstance, msg) {
  var f, obj = {
component: jspb.Message.getFieldWithDefault(msg, 1, ""),
configTemplate: jspb.Message.getFieldWithDefault(msg, 2, ""),
dataDirsList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f,
healthEndpoint: jspb.Message.getFieldWithDefault(msg, 4, ""),
upgradeStrategy: jspb.Message.getFieldWithDefault(msg, 5, ""),
requiredPrivilegesList: (f = jspb.Message.getRepeatedField(msg, 6)) == null ? undefined : f
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.InfrastructureDetail}
 */
proto.repository.InfrastructureDetail.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.InfrastructureDetail;
  return proto.repository.InfrastructureDetail.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.InfrastructureDetail} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.InfrastructureDetail}
 */
proto.repository.InfrastructureDetail.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setComponent(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setConfigTemplate(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.addDataDirs(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setHealthEndpoint(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setUpgradeStrategy(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.addRequiredPrivileges(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.InfrastructureDetail.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.InfrastructureDetail.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.InfrastructureDetail} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.InfrastructureDetail.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getComponent();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getConfigTemplate();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getDataDirsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      3,
      f
    );
  }
  f = message.getHealthEndpoint();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getUpgradeStrategy();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getRequiredPrivilegesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      6,
      f
    );
  }
};


/**
 * optional string component = 1;
 * @return {string}
 */
proto.repository.InfrastructureDetail.prototype.getComponent = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.InfrastructureDetail} returns this
 */
proto.repository.InfrastructureDetail.prototype.setComponent = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string config_template = 2;
 * @return {string}
 */
proto.repository.InfrastructureDetail.prototype.getConfigTemplate = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.InfrastructureDetail} returns this
 */
proto.repository.InfrastructureDetail.prototype.setConfigTemplate = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * repeated string data_dirs = 3;
 * @return {!Array<string>}
 */
proto.repository.InfrastructureDetail.prototype.getDataDirsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.InfrastructureDetail} returns this
 */
proto.repository.InfrastructureDetail.prototype.setDataDirsList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.InfrastructureDetail} returns this
 */
proto.repository.InfrastructureDetail.prototype.addDataDirs = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.InfrastructureDetail} returns this
 */
proto.repository.InfrastructureDetail.prototype.clearDataDirsList = function() {
  return this.setDataDirsList([]);
};


/**
 * optional string health_endpoint = 4;
 * @return {string}
 */
proto.repository.InfrastructureDetail.prototype.getHealthEndpoint = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.InfrastructureDetail} returns this
 */
proto.repository.InfrastructureDetail.prototype.setHealthEndpoint = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string upgrade_strategy = 5;
 * @return {string}
 */
proto.repository.InfrastructureDetail.prototype.getUpgradeStrategy = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.InfrastructureDetail} returns this
 */
proto.repository.InfrastructureDetail.prototype.setUpgradeStrategy = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * repeated string required_privileges = 6;
 * @return {!Array<string>}
 */
proto.repository.InfrastructureDetail.prototype.getRequiredPrivilegesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 6));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.InfrastructureDetail} returns this
 */
proto.repository.InfrastructureDetail.prototype.setRequiredPrivilegesList = function(value) {
  return jspb.Message.setField(this, 6, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.InfrastructureDetail} returns this
 */
proto.repository.InfrastructureDetail.prototype.addRequiredPrivileges = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 6, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.InfrastructureDetail} returns this
 */
proto.repository.InfrastructureDetail.prototype.clearRequiredPrivilegesList = function() {
  return this.setRequiredPrivilegesList([]);
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
proto.repository.ProvenanceRecord.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ProvenanceRecord.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ProvenanceRecord} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ProvenanceRecord.toObject = function(includeInstance, msg) {
  var f, obj = {
subject: jspb.Message.getFieldWithDefault(msg, 1, ""),
principalType: jspb.Message.getFieldWithDefault(msg, 2, ""),
authMethod: jspb.Message.getFieldWithDefault(msg, 3, ""),
sourceIp: jspb.Message.getFieldWithDefault(msg, 4, ""),
buildCommit: jspb.Message.getFieldWithDefault(msg, 5, ""),
buildSource: jspb.Message.getFieldWithDefault(msg, 6, ""),
timestampUnix: jspb.Message.getFieldWithDefault(msg, 7, 0),
clusterId: jspb.Message.getFieldWithDefault(msg, 8, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ProvenanceRecord}
 */
proto.repository.ProvenanceRecord.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ProvenanceRecord;
  return proto.repository.ProvenanceRecord.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ProvenanceRecord} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ProvenanceRecord}
 */
proto.repository.ProvenanceRecord.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setPrincipalType(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setAuthMethod(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setSourceIp(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setBuildCommit(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setBuildSource(value);
      break;
    case 7:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setTimestampUnix(value);
      break;
    case 8:
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
proto.repository.ProvenanceRecord.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ProvenanceRecord.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ProvenanceRecord} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ProvenanceRecord.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSubject();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getPrincipalType();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getAuthMethod();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getSourceIp();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getBuildCommit();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getBuildSource();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getTimestampUnix();
  if (f !== 0) {
    writer.writeInt64(
      7,
      f
    );
  }
  f = message.getClusterId();
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
proto.repository.ProvenanceRecord.prototype.getSubject = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ProvenanceRecord} returns this
 */
proto.repository.ProvenanceRecord.prototype.setSubject = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string principal_type = 2;
 * @return {string}
 */
proto.repository.ProvenanceRecord.prototype.getPrincipalType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ProvenanceRecord} returns this
 */
proto.repository.ProvenanceRecord.prototype.setPrincipalType = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string auth_method = 3;
 * @return {string}
 */
proto.repository.ProvenanceRecord.prototype.getAuthMethod = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ProvenanceRecord} returns this
 */
proto.repository.ProvenanceRecord.prototype.setAuthMethod = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string source_ip = 4;
 * @return {string}
 */
proto.repository.ProvenanceRecord.prototype.getSourceIp = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ProvenanceRecord} returns this
 */
proto.repository.ProvenanceRecord.prototype.setSourceIp = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string build_commit = 5;
 * @return {string}
 */
proto.repository.ProvenanceRecord.prototype.getBuildCommit = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ProvenanceRecord} returns this
 */
proto.repository.ProvenanceRecord.prototype.setBuildCommit = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string build_source = 6;
 * @return {string}
 */
proto.repository.ProvenanceRecord.prototype.getBuildSource = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ProvenanceRecord} returns this
 */
proto.repository.ProvenanceRecord.prototype.setBuildSource = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional int64 timestamp_unix = 7;
 * @return {number}
 */
proto.repository.ProvenanceRecord.prototype.getTimestampUnix = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ProvenanceRecord} returns this
 */
proto.repository.ProvenanceRecord.prototype.setTimestampUnix = function(value) {
  return jspb.Message.setProto3IntField(this, 7, value);
};


/**
 * optional string cluster_id = 8;
 * @return {string}
 */
proto.repository.ProvenanceRecord.prototype.getClusterId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ProvenanceRecord} returns this
 */
proto.repository.ProvenanceRecord.prototype.setClusterId = function(value) {
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
proto.repository.SetArtifactStateRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.SetArtifactStateRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.SetArtifactStateRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.SetArtifactStateRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
ref: (f = msg.getRef()) && proto.repository.ArtifactRef.toObject(includeInstance, f),
buildNumber: jspb.Message.getFieldWithDefault(msg, 2, 0),
targetState: jspb.Message.getFieldWithDefault(msg, 3, 0),
reason: jspb.Message.getFieldWithDefault(msg, 4, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.SetArtifactStateRequest}
 */
proto.repository.SetArtifactStateRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.SetArtifactStateRequest;
  return proto.repository.SetArtifactStateRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.SetArtifactStateRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.SetArtifactStateRequest}
 */
proto.repository.SetArtifactStateRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactRef;
      reader.readMessage(value,proto.repository.ArtifactRef.deserializeBinaryFromReader);
      msg.setRef(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    case 3:
      var value = /** @type {!proto.repository.PublishState} */ (reader.readEnum());
      msg.setTargetState(value);
      break;
    case 4:
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
proto.repository.SetArtifactStateRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.SetArtifactStateRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.SetArtifactStateRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.SetArtifactStateRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRef();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.ArtifactRef.serializeBinaryToWriter
    );
  }
  f = message.getBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
      2,
      f
    );
  }
  f = message.getTargetState();
  if (f !== 0.0) {
    writer.writeEnum(
      3,
      f
    );
  }
  f = message.getReason();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * optional ArtifactRef ref = 1;
 * @return {?proto.repository.ArtifactRef}
 */
proto.repository.SetArtifactStateRequest.prototype.getRef = function() {
  return /** @type{?proto.repository.ArtifactRef} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactRef, 1));
};


/**
 * @param {?proto.repository.ArtifactRef|undefined} value
 * @return {!proto.repository.SetArtifactStateRequest} returns this
*/
proto.repository.SetArtifactStateRequest.prototype.setRef = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.SetArtifactStateRequest} returns this
 */
proto.repository.SetArtifactStateRequest.prototype.clearRef = function() {
  return this.setRef(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.SetArtifactStateRequest.prototype.hasRef = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional int64 build_number = 2;
 * @return {number}
 */
proto.repository.SetArtifactStateRequest.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.SetArtifactStateRequest} returns this
 */
proto.repository.SetArtifactStateRequest.prototype.setBuildNumber = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional PublishState target_state = 3;
 * @return {!proto.repository.PublishState}
 */
proto.repository.SetArtifactStateRequest.prototype.getTargetState = function() {
  return /** @type {!proto.repository.PublishState} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.repository.PublishState} value
 * @return {!proto.repository.SetArtifactStateRequest} returns this
 */
proto.repository.SetArtifactStateRequest.prototype.setTargetState = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
};


/**
 * optional string reason = 4;
 * @return {string}
 */
proto.repository.SetArtifactStateRequest.prototype.getReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.SetArtifactStateRequest} returns this
 */
proto.repository.SetArtifactStateRequest.prototype.setReason = function(value) {
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
proto.repository.SetArtifactStateResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.SetArtifactStateResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.SetArtifactStateResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.SetArtifactStateResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
previousState: jspb.Message.getFieldWithDefault(msg, 1, 0),
currentState: jspb.Message.getFieldWithDefault(msg, 2, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.SetArtifactStateResponse}
 */
proto.repository.SetArtifactStateResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.SetArtifactStateResponse;
  return proto.repository.SetArtifactStateResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.SetArtifactStateResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.SetArtifactStateResponse}
 */
proto.repository.SetArtifactStateResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.repository.PublishState} */ (reader.readEnum());
      msg.setPreviousState(value);
      break;
    case 2:
      var value = /** @type {!proto.repository.PublishState} */ (reader.readEnum());
      msg.setCurrentState(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.SetArtifactStateResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.SetArtifactStateResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.SetArtifactStateResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.SetArtifactStateResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPreviousState();
  if (f !== 0.0) {
    writer.writeEnum(
      1,
      f
    );
  }
  f = message.getCurrentState();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
};


/**
 * optional PublishState previous_state = 1;
 * @return {!proto.repository.PublishState}
 */
proto.repository.SetArtifactStateResponse.prototype.getPreviousState = function() {
  return /** @type {!proto.repository.PublishState} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {!proto.repository.PublishState} value
 * @return {!proto.repository.SetArtifactStateResponse} returns this
 */
proto.repository.SetArtifactStateResponse.prototype.setPreviousState = function(value) {
  return jspb.Message.setProto3EnumField(this, 1, value);
};


/**
 * optional PublishState current_state = 2;
 * @return {!proto.repository.PublishState}
 */
proto.repository.SetArtifactStateResponse.prototype.getCurrentState = function() {
  return /** @type {!proto.repository.PublishState} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.repository.PublishState} value
 * @return {!proto.repository.SetArtifactStateResponse} returns this
 */
proto.repository.SetArtifactStateResponse.prototype.setCurrentState = function(value) {
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
proto.repository.GetNamespaceRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.GetNamespaceRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.GetNamespaceRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.GetNamespaceRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
namespaceId: jspb.Message.getFieldWithDefault(msg, 1, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.GetNamespaceRequest}
 */
proto.repository.GetNamespaceRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.GetNamespaceRequest;
  return proto.repository.GetNamespaceRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.GetNamespaceRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.GetNamespaceRequest}
 */
proto.repository.GetNamespaceRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setNamespaceId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.GetNamespaceRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.GetNamespaceRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.GetNamespaceRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.GetNamespaceRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNamespaceId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string namespace_id = 1;
 * @return {string}
 */
proto.repository.GetNamespaceRequest.prototype.getNamespaceId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.GetNamespaceRequest} returns this
 */
proto.repository.GetNamespaceRequest.prototype.setNamespaceId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.NamespaceInfo.repeatedFields_ = [2,3];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.NamespaceInfo.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.NamespaceInfo.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.NamespaceInfo} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.NamespaceInfo.toObject = function(includeInstance, msg) {
  var f, obj = {
namespaceId: jspb.Message.getFieldWithDefault(msg, 1, ""),
ownersList: (f = jspb.Message.getRepeatedField(msg, 2)) == null ? undefined : f,
permittedList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.NamespaceInfo}
 */
proto.repository.NamespaceInfo.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.NamespaceInfo;
  return proto.repository.NamespaceInfo.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.NamespaceInfo} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.NamespaceInfo}
 */
proto.repository.NamespaceInfo.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setNamespaceId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.addOwners(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.addPermitted(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.NamespaceInfo.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.NamespaceInfo.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.NamespaceInfo} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.NamespaceInfo.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNamespaceId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOwnersList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      2,
      f
    );
  }
  f = message.getPermittedList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      3,
      f
    );
  }
};


/**
 * optional string namespace_id = 1;
 * @return {string}
 */
proto.repository.NamespaceInfo.prototype.getNamespaceId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.NamespaceInfo} returns this
 */
proto.repository.NamespaceInfo.prototype.setNamespaceId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * repeated string owners = 2;
 * @return {!Array<string>}
 */
proto.repository.NamespaceInfo.prototype.getOwnersList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.NamespaceInfo} returns this
 */
proto.repository.NamespaceInfo.prototype.setOwnersList = function(value) {
  return jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.NamespaceInfo} returns this
 */
proto.repository.NamespaceInfo.prototype.addOwners = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.NamespaceInfo} returns this
 */
proto.repository.NamespaceInfo.prototype.clearOwnersList = function() {
  return this.setOwnersList([]);
};


/**
 * repeated string permitted = 3;
 * @return {!Array<string>}
 */
proto.repository.NamespaceInfo.prototype.getPermittedList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.NamespaceInfo} returns this
 */
proto.repository.NamespaceInfo.prototype.setPermittedList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.NamespaceInfo} returns this
 */
proto.repository.NamespaceInfo.prototype.addPermitted = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.NamespaceInfo} returns this
 */
proto.repository.NamespaceInfo.prototype.clearPermittedList = function() {
  return this.setPermittedList([]);
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
proto.repository.GetNamespaceResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.GetNamespaceResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.GetNamespaceResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.GetNamespaceResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
namespace: (f = msg.getNamespace()) && proto.repository.NamespaceInfo.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.GetNamespaceResponse}
 */
proto.repository.GetNamespaceResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.GetNamespaceResponse;
  return proto.repository.GetNamespaceResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.GetNamespaceResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.GetNamespaceResponse}
 */
proto.repository.GetNamespaceResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.NamespaceInfo;
      reader.readMessage(value,proto.repository.NamespaceInfo.deserializeBinaryFromReader);
      msg.setNamespace(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.GetNamespaceResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.GetNamespaceResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.GetNamespaceResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.GetNamespaceResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNamespace();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.NamespaceInfo.serializeBinaryToWriter
    );
  }
};


/**
 * optional NamespaceInfo namespace = 1;
 * @return {?proto.repository.NamespaceInfo}
 */
proto.repository.GetNamespaceResponse.prototype.getNamespace = function() {
  return /** @type{?proto.repository.NamespaceInfo} */ (
    jspb.Message.getWrapperField(this, proto.repository.NamespaceInfo, 1));
};


/**
 * @param {?proto.repository.NamespaceInfo|undefined} value
 * @return {!proto.repository.GetNamespaceResponse} returns this
*/
proto.repository.GetNamespaceResponse.prototype.setNamespace = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.GetNamespaceResponse} returns this
 */
proto.repository.GetNamespaceResponse.prototype.clearNamespace = function() {
  return this.setNamespace(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.GetNamespaceResponse.prototype.hasNamespace = function() {
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
proto.repository.ListArtifactsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ListArtifactsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ListArtifactsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListArtifactsRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.repository.ListArtifactsRequest}
 */
proto.repository.ListArtifactsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ListArtifactsRequest;
  return proto.repository.ListArtifactsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ListArtifactsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ListArtifactsRequest}
 */
proto.repository.ListArtifactsRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.repository.ListArtifactsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ListArtifactsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ListArtifactsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListArtifactsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.ListArtifactsResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.ListArtifactsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ListArtifactsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ListArtifactsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListArtifactsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
artifactsList: jspb.Message.toObjectList(msg.getArtifactsList(),
    proto.repository.ArtifactManifest.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ListArtifactsResponse}
 */
proto.repository.ListArtifactsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ListArtifactsResponse;
  return proto.repository.ListArtifactsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ListArtifactsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ListArtifactsResponse}
 */
proto.repository.ListArtifactsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactManifest;
      reader.readMessage(value,proto.repository.ArtifactManifest.deserializeBinaryFromReader);
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
proto.repository.ListArtifactsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ListArtifactsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ListArtifactsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListArtifactsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getArtifactsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.repository.ArtifactManifest.serializeBinaryToWriter
    );
  }
};


/**
 * repeated ArtifactManifest artifacts = 1;
 * @return {!Array<!proto.repository.ArtifactManifest>}
 */
proto.repository.ListArtifactsResponse.prototype.getArtifactsList = function() {
  return /** @type{!Array<!proto.repository.ArtifactManifest>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.repository.ArtifactManifest, 1));
};


/**
 * @param {!Array<!proto.repository.ArtifactManifest>} value
 * @return {!proto.repository.ListArtifactsResponse} returns this
*/
proto.repository.ListArtifactsResponse.prototype.setArtifactsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.repository.ArtifactManifest=} opt_value
 * @param {number=} opt_index
 * @return {!proto.repository.ArtifactManifest}
 */
proto.repository.ListArtifactsResponse.prototype.addArtifacts = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.repository.ArtifactManifest, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ListArtifactsResponse} returns this
 */
proto.repository.ListArtifactsResponse.prototype.clearArtifactsList = function() {
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
proto.repository.UploadArtifactRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.UploadArtifactRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.UploadArtifactRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.UploadArtifactRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
user: jspb.Message.getFieldWithDefault(msg, 1, ""),
organization: jspb.Message.getFieldWithDefault(msg, 2, ""),
ref: (f = msg.getRef()) && proto.repository.ArtifactRef.toObject(includeInstance, f),
data: msg.getData_asB64(),
buildNumber: jspb.Message.getFieldWithDefault(msg, 5, 0),
reservationId: jspb.Message.getFieldWithDefault(msg, 6, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.UploadArtifactRequest}
 */
proto.repository.UploadArtifactRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.UploadArtifactRequest;
  return proto.repository.UploadArtifactRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.UploadArtifactRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.UploadArtifactRequest}
 */
proto.repository.UploadArtifactRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUser(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setOrganization(value);
      break;
    case 3:
      var value = new proto.repository.ArtifactRef;
      reader.readMessage(value,proto.repository.ArtifactRef.deserializeBinaryFromReader);
      msg.setRef(value);
      break;
    case 4:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setData(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setReservationId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.UploadArtifactRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.UploadArtifactRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.UploadArtifactRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.UploadArtifactRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUser();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOrganization();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getRef();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.repository.ArtifactRef.serializeBinaryToWriter
    );
  }
  f = message.getData_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      4,
      f
    );
  }
  f = message.getBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
  f = message.getReservationId();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
};


/**
 * optional string user = 1;
 * @return {string}
 */
proto.repository.UploadArtifactRequest.prototype.getUser = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UploadArtifactRequest} returns this
 */
proto.repository.UploadArtifactRequest.prototype.setUser = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string organization = 2;
 * @return {string}
 */
proto.repository.UploadArtifactRequest.prototype.getOrganization = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UploadArtifactRequest} returns this
 */
proto.repository.UploadArtifactRequest.prototype.setOrganization = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional ArtifactRef ref = 3;
 * @return {?proto.repository.ArtifactRef}
 */
proto.repository.UploadArtifactRequest.prototype.getRef = function() {
  return /** @type{?proto.repository.ArtifactRef} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactRef, 3));
};


/**
 * @param {?proto.repository.ArtifactRef|undefined} value
 * @return {!proto.repository.UploadArtifactRequest} returns this
*/
proto.repository.UploadArtifactRequest.prototype.setRef = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.UploadArtifactRequest} returns this
 */
proto.repository.UploadArtifactRequest.prototype.clearRef = function() {
  return this.setRef(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.UploadArtifactRequest.prototype.hasRef = function() {
  return jspb.Message.getField(this, 3) != null;
};


/**
 * optional bytes data = 4;
 * @return {string}
 */
proto.repository.UploadArtifactRequest.prototype.getData = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * optional bytes data = 4;
 * This is a type-conversion wrapper around `getData()`
 * @return {string}
 */
proto.repository.UploadArtifactRequest.prototype.getData_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getData()));
};


/**
 * optional bytes data = 4;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getData()`
 * @return {!Uint8Array}
 */
proto.repository.UploadArtifactRequest.prototype.getData_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getData()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.repository.UploadArtifactRequest} returns this
 */
proto.repository.UploadArtifactRequest.prototype.setData = function(value) {
  return jspb.Message.setProto3BytesField(this, 4, value);
};


/**
 * optional int64 build_number = 5;
 * @return {number}
 */
proto.repository.UploadArtifactRequest.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.UploadArtifactRequest} returns this
 */
proto.repository.UploadArtifactRequest.prototype.setBuildNumber = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional string reservation_id = 6;
 * @return {string}
 */
proto.repository.UploadArtifactRequest.prototype.getReservationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UploadArtifactRequest} returns this
 */
proto.repository.UploadArtifactRequest.prototype.setReservationId = function(value) {
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
proto.repository.UploadArtifactResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.UploadArtifactResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.UploadArtifactResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.UploadArtifactResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
buildId: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.UploadArtifactResponse}
 */
proto.repository.UploadArtifactResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.UploadArtifactResponse;
  return proto.repository.UploadArtifactResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.UploadArtifactResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.UploadArtifactResponse}
 */
proto.repository.UploadArtifactResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
      break;
    case 2:
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
proto.repository.UploadArtifactResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.UploadArtifactResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.UploadArtifactResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.UploadArtifactResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getBuildId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.repository.UploadArtifactResponse.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.UploadArtifactResponse} returns this
 */
proto.repository.UploadArtifactResponse.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional string build_id = 2;
 * @return {string}
 */
proto.repository.UploadArtifactResponse.prototype.getBuildId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UploadArtifactResponse} returns this
 */
proto.repository.UploadArtifactResponse.prototype.setBuildId = function(value) {
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
proto.repository.DownloadArtifactRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.DownloadArtifactRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.DownloadArtifactRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.DownloadArtifactRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
ref: (f = msg.getRef()) && proto.repository.ArtifactRef.toObject(includeInstance, f),
buildNumber: jspb.Message.getFieldWithDefault(msg, 2, 0),
allowUpstreamFallback: jspb.Message.getBooleanFieldWithDefault(msg, 3, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.DownloadArtifactRequest}
 */
proto.repository.DownloadArtifactRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.DownloadArtifactRequest;
  return proto.repository.DownloadArtifactRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.DownloadArtifactRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.DownloadArtifactRequest}
 */
proto.repository.DownloadArtifactRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactRef;
      reader.readMessage(value,proto.repository.ArtifactRef.deserializeBinaryFromReader);
      msg.setRef(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setAllowUpstreamFallback(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.DownloadArtifactRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.DownloadArtifactRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.DownloadArtifactRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.DownloadArtifactRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRef();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.ArtifactRef.serializeBinaryToWriter
    );
  }
  f = message.getBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
      2,
      f
    );
  }
  f = message.getAllowUpstreamFallback();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
};


/**
 * optional ArtifactRef ref = 1;
 * @return {?proto.repository.ArtifactRef}
 */
proto.repository.DownloadArtifactRequest.prototype.getRef = function() {
  return /** @type{?proto.repository.ArtifactRef} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactRef, 1));
};


/**
 * @param {?proto.repository.ArtifactRef|undefined} value
 * @return {!proto.repository.DownloadArtifactRequest} returns this
*/
proto.repository.DownloadArtifactRequest.prototype.setRef = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.DownloadArtifactRequest} returns this
 */
proto.repository.DownloadArtifactRequest.prototype.clearRef = function() {
  return this.setRef(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.DownloadArtifactRequest.prototype.hasRef = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional int64 build_number = 2;
 * @return {number}
 */
proto.repository.DownloadArtifactRequest.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.DownloadArtifactRequest} returns this
 */
proto.repository.DownloadArtifactRequest.prototype.setBuildNumber = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional bool allow_upstream_fallback = 3;
 * @return {boolean}
 */
proto.repository.DownloadArtifactRequest.prototype.getAllowUpstreamFallback = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.DownloadArtifactRequest} returns this
 */
proto.repository.DownloadArtifactRequest.prototype.setAllowUpstreamFallback = function(value) {
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
proto.repository.DownloadArtifactResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.DownloadArtifactResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.DownloadArtifactResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.DownloadArtifactResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
data: msg.getData_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.DownloadArtifactResponse}
 */
proto.repository.DownloadArtifactResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.DownloadArtifactResponse;
  return proto.repository.DownloadArtifactResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.DownloadArtifactResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.DownloadArtifactResponse}
 */
proto.repository.DownloadArtifactResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setData(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.DownloadArtifactResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.DownloadArtifactResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.DownloadArtifactResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.DownloadArtifactResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getData_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
};


/**
 * optional bytes data = 1;
 * @return {string}
 */
proto.repository.DownloadArtifactResponse.prototype.getData = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes data = 1;
 * This is a type-conversion wrapper around `getData()`
 * @return {string}
 */
proto.repository.DownloadArtifactResponse.prototype.getData_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getData()));
};


/**
 * optional bytes data = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getData()`
 * @return {!Uint8Array}
 */
proto.repository.DownloadArtifactResponse.prototype.getData_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getData()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.repository.DownloadArtifactResponse} returns this
 */
proto.repository.DownloadArtifactResponse.prototype.setData = function(value) {
  return jspb.Message.setProto3BytesField(this, 1, value);
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
proto.repository.GetArtifactManifestRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.GetArtifactManifestRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.GetArtifactManifestRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.GetArtifactManifestRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
ref: (f = msg.getRef()) && proto.repository.ArtifactRef.toObject(includeInstance, f),
buildNumber: jspb.Message.getFieldWithDefault(msg, 2, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.GetArtifactManifestRequest}
 */
proto.repository.GetArtifactManifestRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.GetArtifactManifestRequest;
  return proto.repository.GetArtifactManifestRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.GetArtifactManifestRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.GetArtifactManifestRequest}
 */
proto.repository.GetArtifactManifestRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactRef;
      reader.readMessage(value,proto.repository.ArtifactRef.deserializeBinaryFromReader);
      msg.setRef(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.GetArtifactManifestRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.GetArtifactManifestRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.GetArtifactManifestRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.GetArtifactManifestRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRef();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.ArtifactRef.serializeBinaryToWriter
    );
  }
  f = message.getBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
      2,
      f
    );
  }
};


/**
 * optional ArtifactRef ref = 1;
 * @return {?proto.repository.ArtifactRef}
 */
proto.repository.GetArtifactManifestRequest.prototype.getRef = function() {
  return /** @type{?proto.repository.ArtifactRef} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactRef, 1));
};


/**
 * @param {?proto.repository.ArtifactRef|undefined} value
 * @return {!proto.repository.GetArtifactManifestRequest} returns this
*/
proto.repository.GetArtifactManifestRequest.prototype.setRef = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.GetArtifactManifestRequest} returns this
 */
proto.repository.GetArtifactManifestRequest.prototype.clearRef = function() {
  return this.setRef(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.GetArtifactManifestRequest.prototype.hasRef = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional int64 build_number = 2;
 * @return {number}
 */
proto.repository.GetArtifactManifestRequest.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.GetArtifactManifestRequest} returns this
 */
proto.repository.GetArtifactManifestRequest.prototype.setBuildNumber = function(value) {
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
proto.repository.GetArtifactManifestResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.GetArtifactManifestResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.GetArtifactManifestResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.GetArtifactManifestResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
manifest: (f = msg.getManifest()) && proto.repository.ArtifactManifest.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.GetArtifactManifestResponse}
 */
proto.repository.GetArtifactManifestResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.GetArtifactManifestResponse;
  return proto.repository.GetArtifactManifestResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.GetArtifactManifestResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.GetArtifactManifestResponse}
 */
proto.repository.GetArtifactManifestResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactManifest;
      reader.readMessage(value,proto.repository.ArtifactManifest.deserializeBinaryFromReader);
      msg.setManifest(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.GetArtifactManifestResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.GetArtifactManifestResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.GetArtifactManifestResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.GetArtifactManifestResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getManifest();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.ArtifactManifest.serializeBinaryToWriter
    );
  }
};


/**
 * optional ArtifactManifest manifest = 1;
 * @return {?proto.repository.ArtifactManifest}
 */
proto.repository.GetArtifactManifestResponse.prototype.getManifest = function() {
  return /** @type{?proto.repository.ArtifactManifest} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactManifest, 1));
};


/**
 * @param {?proto.repository.ArtifactManifest|undefined} value
 * @return {!proto.repository.GetArtifactManifestResponse} returns this
*/
proto.repository.GetArtifactManifestResponse.prototype.setManifest = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.GetArtifactManifestResponse} returns this
 */
proto.repository.GetArtifactManifestResponse.prototype.clearManifest = function() {
  return this.setManifest(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.GetArtifactManifestResponse.prototype.hasManifest = function() {
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
proto.repository.UploadBundleRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.UploadBundleRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.UploadBundleRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.UploadBundleRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
user: jspb.Message.getFieldWithDefault(msg, 1, ""),
organization: jspb.Message.getFieldWithDefault(msg, 2, ""),
data: msg.getData_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.UploadBundleRequest}
 */
proto.repository.UploadBundleRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.UploadBundleRequest;
  return proto.repository.UploadBundleRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.UploadBundleRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.UploadBundleRequest}
 */
proto.repository.UploadBundleRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUser(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setOrganization(value);
      break;
    case 3:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setData(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.UploadBundleRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.UploadBundleRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.UploadBundleRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.UploadBundleRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUser();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOrganization();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getData_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      3,
      f
    );
  }
};


/**
 * optional string user = 1;
 * @return {string}
 */
proto.repository.UploadBundleRequest.prototype.getUser = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UploadBundleRequest} returns this
 */
proto.repository.UploadBundleRequest.prototype.setUser = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string organization = 2;
 * @return {string}
 */
proto.repository.UploadBundleRequest.prototype.getOrganization = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UploadBundleRequest} returns this
 */
proto.repository.UploadBundleRequest.prototype.setOrganization = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional bytes data = 3;
 * @return {string}
 */
proto.repository.UploadBundleRequest.prototype.getData = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * optional bytes data = 3;
 * This is a type-conversion wrapper around `getData()`
 * @return {string}
 */
proto.repository.UploadBundleRequest.prototype.getData_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getData()));
};


/**
 * optional bytes data = 3;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getData()`
 * @return {!Uint8Array}
 */
proto.repository.UploadBundleRequest.prototype.getData_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getData()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.repository.UploadBundleRequest} returns this
 */
proto.repository.UploadBundleRequest.prototype.setData = function(value) {
  return jspb.Message.setProto3BytesField(this, 3, value);
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
proto.repository.UploadBundleResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.UploadBundleResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.UploadBundleResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.UploadBundleResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.UploadBundleResponse}
 */
proto.repository.UploadBundleResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.UploadBundleResponse;
  return proto.repository.UploadBundleResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.UploadBundleResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.UploadBundleResponse}
 */
proto.repository.UploadBundleResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
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
proto.repository.UploadBundleResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.UploadBundleResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.UploadBundleResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.UploadBundleResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.repository.UploadBundleResponse.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.UploadBundleResponse} returns this
 */
proto.repository.UploadBundleResponse.prototype.setResult = function(value) {
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
proto.repository.DownloadBundleRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.DownloadBundleRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.DownloadBundleRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.DownloadBundleRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
descriptor: (f = msg.getDescriptor()) && resource_pb.PackageDescriptor.toObject(includeInstance, f),
platform: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.DownloadBundleRequest}
 */
proto.repository.DownloadBundleRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.DownloadBundleRequest;
  return proto.repository.DownloadBundleRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.DownloadBundleRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.DownloadBundleRequest}
 */
proto.repository.DownloadBundleRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new resource_pb.PackageDescriptor;
      reader.readMessage(value,resource_pb.PackageDescriptor.deserializeBinaryFromReader);
      msg.setDescriptor(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlatform(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.DownloadBundleRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.DownloadBundleRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.DownloadBundleRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.DownloadBundleRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDescriptor();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      resource_pb.PackageDescriptor.serializeBinaryToWriter
    );
  }
  f = message.getPlatform();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional resource.PackageDescriptor descriptor = 1;
 * @return {?proto.resource.PackageDescriptor}
 */
proto.repository.DownloadBundleRequest.prototype.getDescriptor = function() {
  return /** @type{?proto.resource.PackageDescriptor} */ (
    jspb.Message.getWrapperField(this, resource_pb.PackageDescriptor, 1));
};


/**
 * @param {?proto.resource.PackageDescriptor|undefined} value
 * @return {!proto.repository.DownloadBundleRequest} returns this
*/
proto.repository.DownloadBundleRequest.prototype.setDescriptor = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.DownloadBundleRequest} returns this
 */
proto.repository.DownloadBundleRequest.prototype.clearDescriptor = function() {
  return this.setDescriptor(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.DownloadBundleRequest.prototype.hasDescriptor = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string platform = 2;
 * @return {string}
 */
proto.repository.DownloadBundleRequest.prototype.getPlatform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.DownloadBundleRequest} returns this
 */
proto.repository.DownloadBundleRequest.prototype.setPlatform = function(value) {
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
proto.repository.DownloadBundleResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.DownloadBundleResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.DownloadBundleResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.DownloadBundleResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
data: msg.getData_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.DownloadBundleResponse}
 */
proto.repository.DownloadBundleResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.DownloadBundleResponse;
  return proto.repository.DownloadBundleResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.DownloadBundleResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.DownloadBundleResponse}
 */
proto.repository.DownloadBundleResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setData(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.DownloadBundleResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.DownloadBundleResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.DownloadBundleResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.DownloadBundleResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getData_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
};


/**
 * optional bytes data = 1;
 * @return {string}
 */
proto.repository.DownloadBundleResponse.prototype.getData = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes data = 1;
 * This is a type-conversion wrapper around `getData()`
 * @return {string}
 */
proto.repository.DownloadBundleResponse.prototype.getData_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getData()));
};


/**
 * optional bytes data = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getData()`
 * @return {!Uint8Array}
 */
proto.repository.DownloadBundleResponse.prototype.getData_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getData()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.repository.DownloadBundleResponse} returns this
 */
proto.repository.DownloadBundleResponse.prototype.setData = function(value) {
  return jspb.Message.setProto3BytesField(this, 1, value);
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
proto.repository.BundleSummary.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.BundleSummary.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.BundleSummary} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.BundleSummary.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, ""),
version: jspb.Message.getFieldWithDefault(msg, 2, ""),
platform: jspb.Message.getFieldWithDefault(msg, 3, ""),
publisherId: jspb.Message.getFieldWithDefault(msg, 4, ""),
serviceId: jspb.Message.getFieldWithDefault(msg, 5, ""),
sizeBytes: jspb.Message.getFieldWithDefault(msg, 6, 0),
publishedUnix: jspb.Message.getFieldWithDefault(msg, 7, 0),
sha256: jspb.Message.getFieldWithDefault(msg, 8, ""),
buildNumber: jspb.Message.getFieldWithDefault(msg, 9, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.BundleSummary}
 */
proto.repository.BundleSummary.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.BundleSummary;
  return proto.repository.BundleSummary.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.BundleSummary} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.BundleSummary}
 */
proto.repository.BundleSummary.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {string} */ (reader.readString());
      msg.setPlatform(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisherId(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setServiceId(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setSizeBytes(value);
      break;
    case 7:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setPublishedUnix(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setSha256(value);
      break;
    case 9:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.BundleSummary.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.BundleSummary.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.BundleSummary} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.BundleSummary.serializeBinaryToWriter = function(message, writer) {
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
  f = message.getPlatform();
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
  f = message.getServiceId();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getSizeBytes();
  if (f !== 0) {
    writer.writeInt64(
      6,
      f
    );
  }
  f = message.getPublishedUnix();
  if (f !== 0) {
    writer.writeInt64(
      7,
      f
    );
  }
  f = message.getSha256();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
  f = message.getBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
      9,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.repository.BundleSummary.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.BundleSummary} returns this
 */
proto.repository.BundleSummary.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string version = 2;
 * @return {string}
 */
proto.repository.BundleSummary.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.BundleSummary} returns this
 */
proto.repository.BundleSummary.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string platform = 3;
 * @return {string}
 */
proto.repository.BundleSummary.prototype.getPlatform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.BundleSummary} returns this
 */
proto.repository.BundleSummary.prototype.setPlatform = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string publisher_id = 4;
 * @return {string}
 */
proto.repository.BundleSummary.prototype.getPublisherId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.BundleSummary} returns this
 */
proto.repository.BundleSummary.prototype.setPublisherId = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string service_id = 5;
 * @return {string}
 */
proto.repository.BundleSummary.prototype.getServiceId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.BundleSummary} returns this
 */
proto.repository.BundleSummary.prototype.setServiceId = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional int64 size_bytes = 6;
 * @return {number}
 */
proto.repository.BundleSummary.prototype.getSizeBytes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.BundleSummary} returns this
 */
proto.repository.BundleSummary.prototype.setSizeBytes = function(value) {
  return jspb.Message.setProto3IntField(this, 6, value);
};


/**
 * optional int64 published_unix = 7;
 * @return {number}
 */
proto.repository.BundleSummary.prototype.getPublishedUnix = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.BundleSummary} returns this
 */
proto.repository.BundleSummary.prototype.setPublishedUnix = function(value) {
  return jspb.Message.setProto3IntField(this, 7, value);
};


/**
 * optional string sha256 = 8;
 * @return {string}
 */
proto.repository.BundleSummary.prototype.getSha256 = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.BundleSummary} returns this
 */
proto.repository.BundleSummary.prototype.setSha256 = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};


/**
 * optional int64 build_number = 9;
 * @return {number}
 */
proto.repository.BundleSummary.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 9, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.BundleSummary} returns this
 */
proto.repository.BundleSummary.prototype.setBuildNumber = function(value) {
  return jspb.Message.setProto3IntField(this, 9, value);
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
proto.repository.ListBundlesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ListBundlesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ListBundlesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListBundlesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
prefix: jspb.Message.getFieldWithDefault(msg, 1, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ListBundlesRequest}
 */
proto.repository.ListBundlesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ListBundlesRequest;
  return proto.repository.ListBundlesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ListBundlesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ListBundlesRequest}
 */
proto.repository.ListBundlesRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPrefix(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.ListBundlesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ListBundlesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ListBundlesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListBundlesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPrefix();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string prefix = 1;
 * @return {string}
 */
proto.repository.ListBundlesRequest.prototype.getPrefix = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ListBundlesRequest} returns this
 */
proto.repository.ListBundlesRequest.prototype.setPrefix = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.ListBundlesResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.ListBundlesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ListBundlesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ListBundlesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListBundlesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
bundlesList: jspb.Message.toObjectList(msg.getBundlesList(),
    proto.repository.BundleSummary.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ListBundlesResponse}
 */
proto.repository.ListBundlesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ListBundlesResponse;
  return proto.repository.ListBundlesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ListBundlesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ListBundlesResponse}
 */
proto.repository.ListBundlesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.BundleSummary;
      reader.readMessage(value,proto.repository.BundleSummary.deserializeBinaryFromReader);
      msg.addBundles(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.ListBundlesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ListBundlesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ListBundlesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListBundlesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBundlesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.repository.BundleSummary.serializeBinaryToWriter
    );
  }
};


/**
 * repeated BundleSummary bundles = 1;
 * @return {!Array<!proto.repository.BundleSummary>}
 */
proto.repository.ListBundlesResponse.prototype.getBundlesList = function() {
  return /** @type{!Array<!proto.repository.BundleSummary>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.repository.BundleSummary, 1));
};


/**
 * @param {!Array<!proto.repository.BundleSummary>} value
 * @return {!proto.repository.ListBundlesResponse} returns this
*/
proto.repository.ListBundlesResponse.prototype.setBundlesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.repository.BundleSummary=} opt_value
 * @param {number=} opt_index
 * @return {!proto.repository.BundleSummary}
 */
proto.repository.ListBundlesResponse.prototype.addBundles = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.repository.BundleSummary, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ListBundlesResponse} returns this
 */
proto.repository.ListBundlesResponse.prototype.clearBundlesList = function() {
  return this.setBundlesList([]);
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
proto.repository.SearchArtifactsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.SearchArtifactsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.SearchArtifactsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.SearchArtifactsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
query: jspb.Message.getFieldWithDefault(msg, 1, ""),
kind: jspb.Message.getFieldWithDefault(msg, 2, 0),
publisherId: jspb.Message.getFieldWithDefault(msg, 3, ""),
platform: jspb.Message.getFieldWithDefault(msg, 4, ""),
pageSize: jspb.Message.getFieldWithDefault(msg, 5, 0),
pageToken: jspb.Message.getFieldWithDefault(msg, 6, ""),
channel: jspb.Message.getFieldWithDefault(msg, 7, 0),
includeAllChannels: jspb.Message.getBooleanFieldWithDefault(msg, 8, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.SearchArtifactsRequest}
 */
proto.repository.SearchArtifactsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.SearchArtifactsRequest;
  return proto.repository.SearchArtifactsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.SearchArtifactsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.SearchArtifactsRequest}
 */
proto.repository.SearchArtifactsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setQuery(value);
      break;
    case 2:
      var value = /** @type {!proto.repository.ArtifactKind} */ (reader.readEnum());
      msg.setKind(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisherId(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlatform(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setPageSize(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setPageToken(value);
      break;
    case 7:
      var value = /** @type {!proto.repository.ArtifactChannel} */ (reader.readEnum());
      msg.setChannel(value);
      break;
    case 8:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIncludeAllChannels(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.SearchArtifactsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.SearchArtifactsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.SearchArtifactsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.SearchArtifactsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getQuery();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getKind();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
  f = message.getPublisherId();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getPlatform();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getPageSize();
  if (f !== 0) {
    writer.writeInt32(
      5,
      f
    );
  }
  f = message.getPageToken();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getChannel();
  if (f !== 0.0) {
    writer.writeEnum(
      7,
      f
    );
  }
  f = message.getIncludeAllChannels();
  if (f) {
    writer.writeBool(
      8,
      f
    );
  }
};


/**
 * optional string query = 1;
 * @return {string}
 */
proto.repository.SearchArtifactsRequest.prototype.getQuery = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.SearchArtifactsRequest} returns this
 */
proto.repository.SearchArtifactsRequest.prototype.setQuery = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional ArtifactKind kind = 2;
 * @return {!proto.repository.ArtifactKind}
 */
proto.repository.SearchArtifactsRequest.prototype.getKind = function() {
  return /** @type {!proto.repository.ArtifactKind} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.repository.ArtifactKind} value
 * @return {!proto.repository.SearchArtifactsRequest} returns this
 */
proto.repository.SearchArtifactsRequest.prototype.setKind = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional string publisher_id = 3;
 * @return {string}
 */
proto.repository.SearchArtifactsRequest.prototype.getPublisherId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.SearchArtifactsRequest} returns this
 */
proto.repository.SearchArtifactsRequest.prototype.setPublisherId = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string platform = 4;
 * @return {string}
 */
proto.repository.SearchArtifactsRequest.prototype.getPlatform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.SearchArtifactsRequest} returns this
 */
proto.repository.SearchArtifactsRequest.prototype.setPlatform = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional int32 page_size = 5;
 * @return {number}
 */
proto.repository.SearchArtifactsRequest.prototype.getPageSize = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.SearchArtifactsRequest} returns this
 */
proto.repository.SearchArtifactsRequest.prototype.setPageSize = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional string page_token = 6;
 * @return {string}
 */
proto.repository.SearchArtifactsRequest.prototype.getPageToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.SearchArtifactsRequest} returns this
 */
proto.repository.SearchArtifactsRequest.prototype.setPageToken = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional ArtifactChannel channel = 7;
 * @return {!proto.repository.ArtifactChannel}
 */
proto.repository.SearchArtifactsRequest.prototype.getChannel = function() {
  return /** @type {!proto.repository.ArtifactChannel} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/**
 * @param {!proto.repository.ArtifactChannel} value
 * @return {!proto.repository.SearchArtifactsRequest} returns this
 */
proto.repository.SearchArtifactsRequest.prototype.setChannel = function(value) {
  return jspb.Message.setProto3EnumField(this, 7, value);
};


/**
 * optional bool include_all_channels = 8;
 * @return {boolean}
 */
proto.repository.SearchArtifactsRequest.prototype.getIncludeAllChannels = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 8, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.SearchArtifactsRequest} returns this
 */
proto.repository.SearchArtifactsRequest.prototype.setIncludeAllChannels = function(value) {
  return jspb.Message.setProto3BooleanField(this, 8, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.SearchArtifactsResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.SearchArtifactsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.SearchArtifactsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.SearchArtifactsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.SearchArtifactsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
artifactsList: jspb.Message.toObjectList(msg.getArtifactsList(),
    proto.repository.ArtifactManifest.toObject, includeInstance),
nextPageToken: jspb.Message.getFieldWithDefault(msg, 2, ""),
totalCount: jspb.Message.getFieldWithDefault(msg, 3, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.SearchArtifactsResponse}
 */
proto.repository.SearchArtifactsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.SearchArtifactsResponse;
  return proto.repository.SearchArtifactsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.SearchArtifactsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.SearchArtifactsResponse}
 */
proto.repository.SearchArtifactsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactManifest;
      reader.readMessage(value,proto.repository.ArtifactManifest.deserializeBinaryFromReader);
      msg.addArtifacts(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setNextPageToken(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setTotalCount(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.SearchArtifactsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.SearchArtifactsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.SearchArtifactsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.SearchArtifactsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getArtifactsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.repository.ArtifactManifest.serializeBinaryToWriter
    );
  }
  f = message.getNextPageToken();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getTotalCount();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
};


/**
 * repeated ArtifactManifest artifacts = 1;
 * @return {!Array<!proto.repository.ArtifactManifest>}
 */
proto.repository.SearchArtifactsResponse.prototype.getArtifactsList = function() {
  return /** @type{!Array<!proto.repository.ArtifactManifest>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.repository.ArtifactManifest, 1));
};


/**
 * @param {!Array<!proto.repository.ArtifactManifest>} value
 * @return {!proto.repository.SearchArtifactsResponse} returns this
*/
proto.repository.SearchArtifactsResponse.prototype.setArtifactsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.repository.ArtifactManifest=} opt_value
 * @param {number=} opt_index
 * @return {!proto.repository.ArtifactManifest}
 */
proto.repository.SearchArtifactsResponse.prototype.addArtifacts = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.repository.ArtifactManifest, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.SearchArtifactsResponse} returns this
 */
proto.repository.SearchArtifactsResponse.prototype.clearArtifactsList = function() {
  return this.setArtifactsList([]);
};


/**
 * optional string next_page_token = 2;
 * @return {string}
 */
proto.repository.SearchArtifactsResponse.prototype.getNextPageToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.SearchArtifactsResponse} returns this
 */
proto.repository.SearchArtifactsResponse.prototype.setNextPageToken = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int32 total_count = 3;
 * @return {number}
 */
proto.repository.SearchArtifactsResponse.prototype.getTotalCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.SearchArtifactsResponse} returns this
 */
proto.repository.SearchArtifactsResponse.prototype.setTotalCount = function(value) {
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
proto.repository.GetArtifactVersionsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.GetArtifactVersionsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.GetArtifactVersionsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.GetArtifactVersionsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
publisherId: jspb.Message.getFieldWithDefault(msg, 1, ""),
name: jspb.Message.getFieldWithDefault(msg, 2, ""),
platform: jspb.Message.getFieldWithDefault(msg, 3, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.GetArtifactVersionsRequest}
 */
proto.repository.GetArtifactVersionsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.GetArtifactVersionsRequest;
  return proto.repository.GetArtifactVersionsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.GetArtifactVersionsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.GetArtifactVersionsRequest}
 */
proto.repository.GetArtifactVersionsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisherId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlatform(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.GetArtifactVersionsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.GetArtifactVersionsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.GetArtifactVersionsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.GetArtifactVersionsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublisherId();
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
  f = message.getPlatform();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string publisher_id = 1;
 * @return {string}
 */
proto.repository.GetArtifactVersionsRequest.prototype.getPublisherId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.GetArtifactVersionsRequest} returns this
 */
proto.repository.GetArtifactVersionsRequest.prototype.setPublisherId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string name = 2;
 * @return {string}
 */
proto.repository.GetArtifactVersionsRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.GetArtifactVersionsRequest} returns this
 */
proto.repository.GetArtifactVersionsRequest.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string platform = 3;
 * @return {string}
 */
proto.repository.GetArtifactVersionsRequest.prototype.getPlatform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.GetArtifactVersionsRequest} returns this
 */
proto.repository.GetArtifactVersionsRequest.prototype.setPlatform = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.GetArtifactVersionsResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.GetArtifactVersionsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.GetArtifactVersionsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.GetArtifactVersionsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.GetArtifactVersionsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
versionsList: jspb.Message.toObjectList(msg.getVersionsList(),
    proto.repository.ArtifactManifest.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.GetArtifactVersionsResponse}
 */
proto.repository.GetArtifactVersionsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.GetArtifactVersionsResponse;
  return proto.repository.GetArtifactVersionsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.GetArtifactVersionsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.GetArtifactVersionsResponse}
 */
proto.repository.GetArtifactVersionsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactManifest;
      reader.readMessage(value,proto.repository.ArtifactManifest.deserializeBinaryFromReader);
      msg.addVersions(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.GetArtifactVersionsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.GetArtifactVersionsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.GetArtifactVersionsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.GetArtifactVersionsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getVersionsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.repository.ArtifactManifest.serializeBinaryToWriter
    );
  }
};


/**
 * repeated ArtifactManifest versions = 1;
 * @return {!Array<!proto.repository.ArtifactManifest>}
 */
proto.repository.GetArtifactVersionsResponse.prototype.getVersionsList = function() {
  return /** @type{!Array<!proto.repository.ArtifactManifest>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.repository.ArtifactManifest, 1));
};


/**
 * @param {!Array<!proto.repository.ArtifactManifest>} value
 * @return {!proto.repository.GetArtifactVersionsResponse} returns this
*/
proto.repository.GetArtifactVersionsResponse.prototype.setVersionsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.repository.ArtifactManifest=} opt_value
 * @param {number=} opt_index
 * @return {!proto.repository.ArtifactManifest}
 */
proto.repository.GetArtifactVersionsResponse.prototype.addVersions = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.repository.ArtifactManifest, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.GetArtifactVersionsResponse} returns this
 */
proto.repository.GetArtifactVersionsResponse.prototype.clearVersionsList = function() {
  return this.setVersionsList([]);
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
proto.repository.DeleteArtifactRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.DeleteArtifactRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.DeleteArtifactRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.DeleteArtifactRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
ref: (f = msg.getRef()) && proto.repository.ArtifactRef.toObject(includeInstance, f),
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
 * @return {!proto.repository.DeleteArtifactRequest}
 */
proto.repository.DeleteArtifactRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.DeleteArtifactRequest;
  return proto.repository.DeleteArtifactRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.DeleteArtifactRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.DeleteArtifactRequest}
 */
proto.repository.DeleteArtifactRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactRef;
      reader.readMessage(value,proto.repository.ArtifactRef.deserializeBinaryFromReader);
      msg.setRef(value);
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
proto.repository.DeleteArtifactRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.DeleteArtifactRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.DeleteArtifactRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.DeleteArtifactRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRef();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.ArtifactRef.serializeBinaryToWriter
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
 * optional ArtifactRef ref = 1;
 * @return {?proto.repository.ArtifactRef}
 */
proto.repository.DeleteArtifactRequest.prototype.getRef = function() {
  return /** @type{?proto.repository.ArtifactRef} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactRef, 1));
};


/**
 * @param {?proto.repository.ArtifactRef|undefined} value
 * @return {!proto.repository.DeleteArtifactRequest} returns this
*/
proto.repository.DeleteArtifactRequest.prototype.setRef = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.DeleteArtifactRequest} returns this
 */
proto.repository.DeleteArtifactRequest.prototype.clearRef = function() {
  return this.setRef(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.DeleteArtifactRequest.prototype.hasRef = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional bool force = 2;
 * @return {boolean}
 */
proto.repository.DeleteArtifactRequest.prototype.getForce = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.DeleteArtifactRequest} returns this
 */
proto.repository.DeleteArtifactRequest.prototype.setForce = function(value) {
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
proto.repository.DeleteArtifactResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.DeleteArtifactResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.DeleteArtifactResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.DeleteArtifactResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
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
 * @return {!proto.repository.DeleteArtifactResponse}
 */
proto.repository.DeleteArtifactResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.DeleteArtifactResponse;
  return proto.repository.DeleteArtifactResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.DeleteArtifactResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.DeleteArtifactResponse}
 */
proto.repository.DeleteArtifactResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.repository.DeleteArtifactResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.DeleteArtifactResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.DeleteArtifactResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.DeleteArtifactResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
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
 * optional bool result = 1;
 * @return {boolean}
 */
proto.repository.DeleteArtifactResponse.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.DeleteArtifactResponse} returns this
 */
proto.repository.DeleteArtifactResponse.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional string message = 2;
 * @return {string}
 */
proto.repository.DeleteArtifactResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.DeleteArtifactResponse} returns this
 */
proto.repository.DeleteArtifactResponse.prototype.setMessage = function(value) {
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
proto.repository.PromoteArtifactRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.PromoteArtifactRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.PromoteArtifactRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.PromoteArtifactRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
ref: (f = msg.getRef()) && proto.repository.ArtifactRef.toObject(includeInstance, f),
buildNumber: jspb.Message.getFieldWithDefault(msg, 2, 0),
targetState: jspb.Message.getFieldWithDefault(msg, 3, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.PromoteArtifactRequest}
 */
proto.repository.PromoteArtifactRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.PromoteArtifactRequest;
  return proto.repository.PromoteArtifactRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.PromoteArtifactRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.PromoteArtifactRequest}
 */
proto.repository.PromoteArtifactRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactRef;
      reader.readMessage(value,proto.repository.ArtifactRef.deserializeBinaryFromReader);
      msg.setRef(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    case 3:
      var value = /** @type {!proto.repository.PublishState} */ (reader.readEnum());
      msg.setTargetState(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.PromoteArtifactRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.PromoteArtifactRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.PromoteArtifactRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.PromoteArtifactRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRef();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.ArtifactRef.serializeBinaryToWriter
    );
  }
  f = message.getBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
      2,
      f
    );
  }
  f = message.getTargetState();
  if (f !== 0.0) {
    writer.writeEnum(
      3,
      f
    );
  }
};


/**
 * optional ArtifactRef ref = 1;
 * @return {?proto.repository.ArtifactRef}
 */
proto.repository.PromoteArtifactRequest.prototype.getRef = function() {
  return /** @type{?proto.repository.ArtifactRef} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactRef, 1));
};


/**
 * @param {?proto.repository.ArtifactRef|undefined} value
 * @return {!proto.repository.PromoteArtifactRequest} returns this
*/
proto.repository.PromoteArtifactRequest.prototype.setRef = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.PromoteArtifactRequest} returns this
 */
proto.repository.PromoteArtifactRequest.prototype.clearRef = function() {
  return this.setRef(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.PromoteArtifactRequest.prototype.hasRef = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional int64 build_number = 2;
 * @return {number}
 */
proto.repository.PromoteArtifactRequest.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.PromoteArtifactRequest} returns this
 */
proto.repository.PromoteArtifactRequest.prototype.setBuildNumber = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional PublishState target_state = 3;
 * @return {!proto.repository.PublishState}
 */
proto.repository.PromoteArtifactRequest.prototype.getTargetState = function() {
  return /** @type {!proto.repository.PublishState} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.repository.PublishState} value
 * @return {!proto.repository.PromoteArtifactRequest} returns this
 */
proto.repository.PromoteArtifactRequest.prototype.setTargetState = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
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
proto.repository.PromoteArtifactResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.PromoteArtifactResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.PromoteArtifactResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.PromoteArtifactResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
previousState: jspb.Message.getFieldWithDefault(msg, 2, 0),
currentState: jspb.Message.getFieldWithDefault(msg, 3, 0),
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
 * @return {!proto.repository.PromoteArtifactResponse}
 */
proto.repository.PromoteArtifactResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.PromoteArtifactResponse;
  return proto.repository.PromoteArtifactResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.PromoteArtifactResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.PromoteArtifactResponse}
 */
proto.repository.PromoteArtifactResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
      break;
    case 2:
      var value = /** @type {!proto.repository.PublishState} */ (reader.readEnum());
      msg.setPreviousState(value);
      break;
    case 3:
      var value = /** @type {!proto.repository.PublishState} */ (reader.readEnum());
      msg.setCurrentState(value);
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
proto.repository.PromoteArtifactResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.PromoteArtifactResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.PromoteArtifactResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.PromoteArtifactResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getPreviousState();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
  f = message.getCurrentState();
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
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.repository.PromoteArtifactResponse.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.PromoteArtifactResponse} returns this
 */
proto.repository.PromoteArtifactResponse.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional PublishState previous_state = 2;
 * @return {!proto.repository.PublishState}
 */
proto.repository.PromoteArtifactResponse.prototype.getPreviousState = function() {
  return /** @type {!proto.repository.PublishState} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.repository.PublishState} value
 * @return {!proto.repository.PromoteArtifactResponse} returns this
 */
proto.repository.PromoteArtifactResponse.prototype.setPreviousState = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional PublishState current_state = 3;
 * @return {!proto.repository.PublishState}
 */
proto.repository.PromoteArtifactResponse.prototype.getCurrentState = function() {
  return /** @type {!proto.repository.PublishState} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.repository.PublishState} value
 * @return {!proto.repository.PromoteArtifactResponse} returns this
 */
proto.repository.PromoteArtifactResponse.prototype.setCurrentState = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
};


/**
 * optional string message = 4;
 * @return {string}
 */
proto.repository.PromoteArtifactResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.PromoteArtifactResponse} returns this
 */
proto.repository.PromoteArtifactResponse.prototype.setMessage = function(value) {
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
proto.repository.DescribePackageRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.DescribePackageRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.DescribePackageRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.DescribePackageRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, ""),
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
 * @return {!proto.repository.DescribePackageRequest}
 */
proto.repository.DescribePackageRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.DescribePackageRequest;
  return proto.repository.DescribePackageRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.DescribePackageRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.DescribePackageRequest}
 */
proto.repository.DescribePackageRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.repository.DescribePackageRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.DescribePackageRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.DescribePackageRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.DescribePackageRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
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
 * optional string name = 1;
 * @return {string}
 */
proto.repository.DescribePackageRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.DescribePackageRequest} returns this
 */
proto.repository.DescribePackageRequest.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string publisher_id = 2;
 * @return {string}
 */
proto.repository.DescribePackageRequest.prototype.getPublisherId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.DescribePackageRequest} returns this
 */
proto.repository.DescribePackageRequest.prototype.setPublisherId = function(value) {
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
proto.repository.NodeInstallation.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.NodeInstallation.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.NodeInstallation} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.NodeInstallation.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
version: jspb.Message.getFieldWithDefault(msg, 2, ""),
status: jspb.Message.getFieldWithDefault(msg, 3, ""),
checksum: jspb.Message.getFieldWithDefault(msg, 4, ""),
installedAt: jspb.Message.getFieldWithDefault(msg, 5, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.NodeInstallation}
 */
proto.repository.NodeInstallation.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.NodeInstallation;
  return proto.repository.NodeInstallation.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.NodeInstallation} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.NodeInstallation}
 */
proto.repository.NodeInstallation.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setVersion(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setStatus(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setChecksum(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setInstalledAt(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.NodeInstallation.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.NodeInstallation.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.NodeInstallation} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.NodeInstallation.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
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
  f = message.getStatus();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getChecksum();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getInstalledAt();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.repository.NodeInstallation.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.NodeInstallation} returns this
 */
proto.repository.NodeInstallation.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string version = 2;
 * @return {string}
 */
proto.repository.NodeInstallation.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.NodeInstallation} returns this
 */
proto.repository.NodeInstallation.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string status = 3;
 * @return {string}
 */
proto.repository.NodeInstallation.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.NodeInstallation} returns this
 */
proto.repository.NodeInstallation.prototype.setStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string checksum = 4;
 * @return {string}
 */
proto.repository.NodeInstallation.prototype.getChecksum = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.NodeInstallation} returns this
 */
proto.repository.NodeInstallation.prototype.setChecksum = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional int64 installed_at = 5;
 * @return {number}
 */
proto.repository.NodeInstallation.prototype.getInstalledAt = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.NodeInstallation} returns this
 */
proto.repository.NodeInstallation.prototype.setInstalledAt = function(value) {
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
proto.repository.DesiredInfo.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.DesiredInfo.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.DesiredInfo} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.DesiredInfo.toObject = function(includeInstance, msg) {
  var f, obj = {
version: jspb.Message.getFieldWithDefault(msg, 1, ""),
generation: jspb.Message.getFieldWithDefault(msg, 2, 0),
publisher: jspb.Message.getFieldWithDefault(msg, 3, ""),
present: jspb.Message.getBooleanFieldWithDefault(msg, 4, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.DesiredInfo}
 */
proto.repository.DesiredInfo.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.DesiredInfo;
  return proto.repository.DesiredInfo.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.DesiredInfo} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.DesiredInfo}
 */
proto.repository.DesiredInfo.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setVersion(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setGeneration(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisher(value);
      break;
    case 4:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setPresent(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.DesiredInfo.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.DesiredInfo.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.DesiredInfo} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.DesiredInfo.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getVersion();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getGeneration();
  if (f !== 0) {
    writer.writeInt64(
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
  f = message.getPresent();
  if (f) {
    writer.writeBool(
      4,
      f
    );
  }
};


/**
 * optional string version = 1;
 * @return {string}
 */
proto.repository.DesiredInfo.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.DesiredInfo} returns this
 */
proto.repository.DesiredInfo.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int64 generation = 2;
 * @return {number}
 */
proto.repository.DesiredInfo.prototype.getGeneration = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.DesiredInfo} returns this
 */
proto.repository.DesiredInfo.prototype.setGeneration = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string publisher = 3;
 * @return {string}
 */
proto.repository.DesiredInfo.prototype.getPublisher = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.DesiredInfo} returns this
 */
proto.repository.DesiredInfo.prototype.setPublisher = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional bool present = 4;
 * @return {boolean}
 */
proto.repository.DesiredInfo.prototype.getPresent = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 4, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.DesiredInfo} returns this
 */
proto.repository.DesiredInfo.prototype.setPresent = function(value) {
  return jspb.Message.setProto3BooleanField(this, 4, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.PackageInfo.repeatedFields_ = [4,7,8];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.PackageInfo.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.PackageInfo.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.PackageInfo} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.PackageInfo.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, ""),
kind: jspb.Message.getFieldWithDefault(msg, 2, 0),
publisher: jspb.Message.getFieldWithDefault(msg, 3, ""),
versionsList: (f = jspb.Message.getRepeatedField(msg, 4)) == null ? undefined : f,
latestVersion: jspb.Message.getFieldWithDefault(msg, 5, ""),
desired: (f = msg.getDesired()) && proto.repository.DesiredInfo.toObject(includeInstance, f),
installedOnList: jspb.Message.toObjectList(msg.getInstalledOnList(),
    proto.repository.NodeInstallation.toObject, includeInstance),
failingOnList: jspb.Message.toObjectList(msg.getFailingOnList(),
    proto.repository.NodeInstallation.toObject, includeInstance),
source: jspb.Message.getFieldWithDefault(msg, 9, ""),
observedAt: jspb.Message.getFieldWithDefault(msg, 10, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.PackageInfo}
 */
proto.repository.PackageInfo.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.PackageInfo;
  return proto.repository.PackageInfo.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.PackageInfo} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.PackageInfo}
 */
proto.repository.PackageInfo.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {!proto.repository.ArtifactKind} */ (reader.readEnum());
      msg.setKind(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisher(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.addVersions(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setLatestVersion(value);
      break;
    case 6:
      var value = new proto.repository.DesiredInfo;
      reader.readMessage(value,proto.repository.DesiredInfo.deserializeBinaryFromReader);
      msg.setDesired(value);
      break;
    case 7:
      var value = new proto.repository.NodeInstallation;
      reader.readMessage(value,proto.repository.NodeInstallation.deserializeBinaryFromReader);
      msg.addInstalledOn(value);
      break;
    case 8:
      var value = new proto.repository.NodeInstallation;
      reader.readMessage(value,proto.repository.NodeInstallation.deserializeBinaryFromReader);
      msg.addFailingOn(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setSource(value);
      break;
    case 10:
      var value = /** @type {number} */ (reader.readInt64());
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
proto.repository.PackageInfo.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.PackageInfo.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.PackageInfo} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.PackageInfo.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getKind();
  if (f !== 0.0) {
    writer.writeEnum(
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
  f = message.getVersionsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      4,
      f
    );
  }
  f = message.getLatestVersion();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getDesired();
  if (f != null) {
    writer.writeMessage(
      6,
      f,
      proto.repository.DesiredInfo.serializeBinaryToWriter
    );
  }
  f = message.getInstalledOnList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      7,
      f,
      proto.repository.NodeInstallation.serializeBinaryToWriter
    );
  }
  f = message.getFailingOnList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      8,
      f,
      proto.repository.NodeInstallation.serializeBinaryToWriter
    );
  }
  f = message.getSource();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
  f = message.getObservedAt();
  if (f !== 0) {
    writer.writeInt64(
      10,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.repository.PackageInfo.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.PackageInfo} returns this
 */
proto.repository.PackageInfo.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional ArtifactKind kind = 2;
 * @return {!proto.repository.ArtifactKind}
 */
proto.repository.PackageInfo.prototype.getKind = function() {
  return /** @type {!proto.repository.ArtifactKind} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.repository.ArtifactKind} value
 * @return {!proto.repository.PackageInfo} returns this
 */
proto.repository.PackageInfo.prototype.setKind = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional string publisher = 3;
 * @return {string}
 */
proto.repository.PackageInfo.prototype.getPublisher = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.PackageInfo} returns this
 */
proto.repository.PackageInfo.prototype.setPublisher = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * repeated string versions = 4;
 * @return {!Array<string>}
 */
proto.repository.PackageInfo.prototype.getVersionsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 4));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.PackageInfo} returns this
 */
proto.repository.PackageInfo.prototype.setVersionsList = function(value) {
  return jspb.Message.setField(this, 4, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.PackageInfo} returns this
 */
proto.repository.PackageInfo.prototype.addVersions = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 4, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.PackageInfo} returns this
 */
proto.repository.PackageInfo.prototype.clearVersionsList = function() {
  return this.setVersionsList([]);
};


/**
 * optional string latest_version = 5;
 * @return {string}
 */
proto.repository.PackageInfo.prototype.getLatestVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.PackageInfo} returns this
 */
proto.repository.PackageInfo.prototype.setLatestVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional DesiredInfo desired = 6;
 * @return {?proto.repository.DesiredInfo}
 */
proto.repository.PackageInfo.prototype.getDesired = function() {
  return /** @type{?proto.repository.DesiredInfo} */ (
    jspb.Message.getWrapperField(this, proto.repository.DesiredInfo, 6));
};


/**
 * @param {?proto.repository.DesiredInfo|undefined} value
 * @return {!proto.repository.PackageInfo} returns this
*/
proto.repository.PackageInfo.prototype.setDesired = function(value) {
  return jspb.Message.setWrapperField(this, 6, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.PackageInfo} returns this
 */
proto.repository.PackageInfo.prototype.clearDesired = function() {
  return this.setDesired(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.PackageInfo.prototype.hasDesired = function() {
  return jspb.Message.getField(this, 6) != null;
};


/**
 * repeated NodeInstallation installed_on = 7;
 * @return {!Array<!proto.repository.NodeInstallation>}
 */
proto.repository.PackageInfo.prototype.getInstalledOnList = function() {
  return /** @type{!Array<!proto.repository.NodeInstallation>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.repository.NodeInstallation, 7));
};


/**
 * @param {!Array<!proto.repository.NodeInstallation>} value
 * @return {!proto.repository.PackageInfo} returns this
*/
proto.repository.PackageInfo.prototype.setInstalledOnList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 7, value);
};


/**
 * @param {!proto.repository.NodeInstallation=} opt_value
 * @param {number=} opt_index
 * @return {!proto.repository.NodeInstallation}
 */
proto.repository.PackageInfo.prototype.addInstalledOn = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 7, opt_value, proto.repository.NodeInstallation, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.PackageInfo} returns this
 */
proto.repository.PackageInfo.prototype.clearInstalledOnList = function() {
  return this.setInstalledOnList([]);
};


/**
 * repeated NodeInstallation failing_on = 8;
 * @return {!Array<!proto.repository.NodeInstallation>}
 */
proto.repository.PackageInfo.prototype.getFailingOnList = function() {
  return /** @type{!Array<!proto.repository.NodeInstallation>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.repository.NodeInstallation, 8));
};


/**
 * @param {!Array<!proto.repository.NodeInstallation>} value
 * @return {!proto.repository.PackageInfo} returns this
*/
proto.repository.PackageInfo.prototype.setFailingOnList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 8, value);
};


/**
 * @param {!proto.repository.NodeInstallation=} opt_value
 * @param {number=} opt_index
 * @return {!proto.repository.NodeInstallation}
 */
proto.repository.PackageInfo.prototype.addFailingOn = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 8, opt_value, proto.repository.NodeInstallation, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.PackageInfo} returns this
 */
proto.repository.PackageInfo.prototype.clearFailingOnList = function() {
  return this.setFailingOnList([]);
};


/**
 * optional string source = 9;
 * @return {string}
 */
proto.repository.PackageInfo.prototype.getSource = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.PackageInfo} returns this
 */
proto.repository.PackageInfo.prototype.setSource = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};


/**
 * optional int64 observed_at = 10;
 * @return {number}
 */
proto.repository.PackageInfo.prototype.getObservedAt = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 10, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.PackageInfo} returns this
 */
proto.repository.PackageInfo.prototype.setObservedAt = function(value) {
  return jspb.Message.setProto3IntField(this, 10, value);
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
proto.repository.DescribePackageResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.DescribePackageResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.DescribePackageResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.DescribePackageResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
info: (f = msg.getInfo()) && proto.repository.PackageInfo.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.DescribePackageResponse}
 */
proto.repository.DescribePackageResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.DescribePackageResponse;
  return proto.repository.DescribePackageResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.DescribePackageResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.DescribePackageResponse}
 */
proto.repository.DescribePackageResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.PackageInfo;
      reader.readMessage(value,proto.repository.PackageInfo.deserializeBinaryFromReader);
      msg.setInfo(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.DescribePackageResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.DescribePackageResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.DescribePackageResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.DescribePackageResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getInfo();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.PackageInfo.serializeBinaryToWriter
    );
  }
};


/**
 * optional PackageInfo info = 1;
 * @return {?proto.repository.PackageInfo}
 */
proto.repository.DescribePackageResponse.prototype.getInfo = function() {
  return /** @type{?proto.repository.PackageInfo} */ (
    jspb.Message.getWrapperField(this, proto.repository.PackageInfo, 1));
};


/**
 * @param {?proto.repository.PackageInfo|undefined} value
 * @return {!proto.repository.DescribePackageResponse} returns this
*/
proto.repository.DescribePackageResponse.prototype.setInfo = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.DescribePackageResponse} returns this
 */
proto.repository.DescribePackageResponse.prototype.clearInfo = function() {
  return this.setInfo(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.DescribePackageResponse.prototype.hasInfo = function() {
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
proto.repository.VerifyArtifactRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.VerifyArtifactRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.VerifyArtifactRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.VerifyArtifactRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
ref: (f = msg.getRef()) && proto.repository.ArtifactRef.toObject(includeInstance, f),
buildNumber: jspb.Message.getFieldWithDefault(msg, 2, 0),
buildId: jspb.Message.getFieldWithDefault(msg, 3, ""),
verifyDigest: jspb.Message.getBooleanFieldWithDefault(msg, 4, false),
verifySignature: jspb.Message.getBooleanFieldWithDefault(msg, 5, false),
includeLedger: jspb.Message.getBooleanFieldWithDefault(msg, 6, false),
includeManifest: jspb.Message.getBooleanFieldWithDefault(msg, 7, false),
includeBlob: jspb.Message.getBooleanFieldWithDefault(msg, 8, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.VerifyArtifactRequest}
 */
proto.repository.VerifyArtifactRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.VerifyArtifactRequest;
  return proto.repository.VerifyArtifactRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.VerifyArtifactRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.VerifyArtifactRequest}
 */
proto.repository.VerifyArtifactRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactRef;
      reader.readMessage(value,proto.repository.ArtifactRef.deserializeBinaryFromReader);
      msg.setRef(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setBuildId(value);
      break;
    case 4:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setVerifyDigest(value);
      break;
    case 5:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setVerifySignature(value);
      break;
    case 6:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIncludeLedger(value);
      break;
    case 7:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIncludeManifest(value);
      break;
    case 8:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIncludeBlob(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.VerifyArtifactRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.VerifyArtifactRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.VerifyArtifactRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.VerifyArtifactRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRef();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.ArtifactRef.serializeBinaryToWriter
    );
  }
  f = message.getBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
      2,
      f
    );
  }
  f = message.getBuildId();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getVerifyDigest();
  if (f) {
    writer.writeBool(
      4,
      f
    );
  }
  f = message.getVerifySignature();
  if (f) {
    writer.writeBool(
      5,
      f
    );
  }
  f = message.getIncludeLedger();
  if (f) {
    writer.writeBool(
      6,
      f
    );
  }
  f = message.getIncludeManifest();
  if (f) {
    writer.writeBool(
      7,
      f
    );
  }
  f = message.getIncludeBlob();
  if (f) {
    writer.writeBool(
      8,
      f
    );
  }
};


/**
 * optional ArtifactRef ref = 1;
 * @return {?proto.repository.ArtifactRef}
 */
proto.repository.VerifyArtifactRequest.prototype.getRef = function() {
  return /** @type{?proto.repository.ArtifactRef} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactRef, 1));
};


/**
 * @param {?proto.repository.ArtifactRef|undefined} value
 * @return {!proto.repository.VerifyArtifactRequest} returns this
*/
proto.repository.VerifyArtifactRequest.prototype.setRef = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.VerifyArtifactRequest} returns this
 */
proto.repository.VerifyArtifactRequest.prototype.clearRef = function() {
  return this.setRef(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.VerifyArtifactRequest.prototype.hasRef = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional int64 build_number = 2;
 * @return {number}
 */
proto.repository.VerifyArtifactRequest.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.VerifyArtifactRequest} returns this
 */
proto.repository.VerifyArtifactRequest.prototype.setBuildNumber = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string build_id = 3;
 * @return {string}
 */
proto.repository.VerifyArtifactRequest.prototype.getBuildId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.VerifyArtifactRequest} returns this
 */
proto.repository.VerifyArtifactRequest.prototype.setBuildId = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional bool verify_digest = 4;
 * @return {boolean}
 */
proto.repository.VerifyArtifactRequest.prototype.getVerifyDigest = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 4, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.VerifyArtifactRequest} returns this
 */
proto.repository.VerifyArtifactRequest.prototype.setVerifyDigest = function(value) {
  return jspb.Message.setProto3BooleanField(this, 4, value);
};


/**
 * optional bool verify_signature = 5;
 * @return {boolean}
 */
proto.repository.VerifyArtifactRequest.prototype.getVerifySignature = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 5, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.VerifyArtifactRequest} returns this
 */
proto.repository.VerifyArtifactRequest.prototype.setVerifySignature = function(value) {
  return jspb.Message.setProto3BooleanField(this, 5, value);
};


/**
 * optional bool include_ledger = 6;
 * @return {boolean}
 */
proto.repository.VerifyArtifactRequest.prototype.getIncludeLedger = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 6, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.VerifyArtifactRequest} returns this
 */
proto.repository.VerifyArtifactRequest.prototype.setIncludeLedger = function(value) {
  return jspb.Message.setProto3BooleanField(this, 6, value);
};


/**
 * optional bool include_manifest = 7;
 * @return {boolean}
 */
proto.repository.VerifyArtifactRequest.prototype.getIncludeManifest = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 7, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.VerifyArtifactRequest} returns this
 */
proto.repository.VerifyArtifactRequest.prototype.setIncludeManifest = function(value) {
  return jspb.Message.setProto3BooleanField(this, 7, value);
};


/**
 * optional bool include_blob = 8;
 * @return {boolean}
 */
proto.repository.VerifyArtifactRequest.prototype.getIncludeBlob = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 8, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.VerifyArtifactRequest} returns this
 */
proto.repository.VerifyArtifactRequest.prototype.setIncludeBlob = function(value) {
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
proto.repository.VerifyArtifactResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.VerifyArtifactResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.VerifyArtifactResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.VerifyArtifactResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
ref: (f = msg.getRef()) && proto.repository.ArtifactRef.toObject(includeInstance, f),
artifactKey: jspb.Message.getFieldWithDefault(msg, 2, ""),
artifactState: jspb.Message.getFieldWithDefault(msg, 3, ""),
publishState: jspb.Message.getFieldWithDefault(msg, 4, 0),
installable: jspb.Message.getBooleanFieldWithDefault(msg, 5, false),
status: jspb.Message.getFieldWithDefault(msg, 6, 0),
reason: jspb.Message.getFieldWithDefault(msg, 7, ""),
blobKey: jspb.Message.getFieldWithDefault(msg, 8, ""),
expectedSize: jspb.Message.getFieldWithDefault(msg, 9, 0),
actualSize: jspb.Message.getFieldWithDefault(msg, 10, 0),
expectedDigest: jspb.Message.getFieldWithDefault(msg, 11, ""),
actualDigest: jspb.Message.getFieldWithDefault(msg, 12, ""),
signatureStatus: jspb.Message.getFieldWithDefault(msg, 13, ""),
provenanceStatus: jspb.Message.getFieldWithDefault(msg, 14, ""),
repairable: jspb.Message.getBooleanFieldWithDefault(msg, 15, false),
recommendedAction: jspb.Message.getFieldWithDefault(msg, 16, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.VerifyArtifactResponse}
 */
proto.repository.VerifyArtifactResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.VerifyArtifactResponse;
  return proto.repository.VerifyArtifactResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.VerifyArtifactResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.VerifyArtifactResponse}
 */
proto.repository.VerifyArtifactResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactRef;
      reader.readMessage(value,proto.repository.ArtifactRef.deserializeBinaryFromReader);
      msg.setRef(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setArtifactKey(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setArtifactState(value);
      break;
    case 4:
      var value = /** @type {!proto.repository.PublishState} */ (reader.readEnum());
      msg.setPublishState(value);
      break;
    case 5:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setInstallable(value);
      break;
    case 6:
      var value = /** @type {!proto.repository.ArtifactVerifyStatus} */ (reader.readEnum());
      msg.setStatus(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setReason(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setBlobKey(value);
      break;
    case 9:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setExpectedSize(value);
      break;
    case 10:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setActualSize(value);
      break;
    case 11:
      var value = /** @type {string} */ (reader.readString());
      msg.setExpectedDigest(value);
      break;
    case 12:
      var value = /** @type {string} */ (reader.readString());
      msg.setActualDigest(value);
      break;
    case 13:
      var value = /** @type {string} */ (reader.readString());
      msg.setSignatureStatus(value);
      break;
    case 14:
      var value = /** @type {string} */ (reader.readString());
      msg.setProvenanceStatus(value);
      break;
    case 15:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setRepairable(value);
      break;
    case 16:
      var value = /** @type {string} */ (reader.readString());
      msg.setRecommendedAction(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.VerifyArtifactResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.VerifyArtifactResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.VerifyArtifactResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.VerifyArtifactResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRef();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.ArtifactRef.serializeBinaryToWriter
    );
  }
  f = message.getArtifactKey();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getArtifactState();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getPublishState();
  if (f !== 0.0) {
    writer.writeEnum(
      4,
      f
    );
  }
  f = message.getInstallable();
  if (f) {
    writer.writeBool(
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
  f = message.getReason();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getBlobKey();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
  f = message.getExpectedSize();
  if (f !== 0) {
    writer.writeInt64(
      9,
      f
    );
  }
  f = message.getActualSize();
  if (f !== 0) {
    writer.writeInt64(
      10,
      f
    );
  }
  f = message.getExpectedDigest();
  if (f.length > 0) {
    writer.writeString(
      11,
      f
    );
  }
  f = message.getActualDigest();
  if (f.length > 0) {
    writer.writeString(
      12,
      f
    );
  }
  f = message.getSignatureStatus();
  if (f.length > 0) {
    writer.writeString(
      13,
      f
    );
  }
  f = message.getProvenanceStatus();
  if (f.length > 0) {
    writer.writeString(
      14,
      f
    );
  }
  f = message.getRepairable();
  if (f) {
    writer.writeBool(
      15,
      f
    );
  }
  f = message.getRecommendedAction();
  if (f.length > 0) {
    writer.writeString(
      16,
      f
    );
  }
};


/**
 * optional ArtifactRef ref = 1;
 * @return {?proto.repository.ArtifactRef}
 */
proto.repository.VerifyArtifactResponse.prototype.getRef = function() {
  return /** @type{?proto.repository.ArtifactRef} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactRef, 1));
};


/**
 * @param {?proto.repository.ArtifactRef|undefined} value
 * @return {!proto.repository.VerifyArtifactResponse} returns this
*/
proto.repository.VerifyArtifactResponse.prototype.setRef = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.VerifyArtifactResponse} returns this
 */
proto.repository.VerifyArtifactResponse.prototype.clearRef = function() {
  return this.setRef(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.VerifyArtifactResponse.prototype.hasRef = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string artifact_key = 2;
 * @return {string}
 */
proto.repository.VerifyArtifactResponse.prototype.getArtifactKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.VerifyArtifactResponse} returns this
 */
proto.repository.VerifyArtifactResponse.prototype.setArtifactKey = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string artifact_state = 3;
 * @return {string}
 */
proto.repository.VerifyArtifactResponse.prototype.getArtifactState = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.VerifyArtifactResponse} returns this
 */
proto.repository.VerifyArtifactResponse.prototype.setArtifactState = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional PublishState publish_state = 4;
 * @return {!proto.repository.PublishState}
 */
proto.repository.VerifyArtifactResponse.prototype.getPublishState = function() {
  return /** @type {!proto.repository.PublishState} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {!proto.repository.PublishState} value
 * @return {!proto.repository.VerifyArtifactResponse} returns this
 */
proto.repository.VerifyArtifactResponse.prototype.setPublishState = function(value) {
  return jspb.Message.setProto3EnumField(this, 4, value);
};


/**
 * optional bool installable = 5;
 * @return {boolean}
 */
proto.repository.VerifyArtifactResponse.prototype.getInstallable = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 5, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.VerifyArtifactResponse} returns this
 */
proto.repository.VerifyArtifactResponse.prototype.setInstallable = function(value) {
  return jspb.Message.setProto3BooleanField(this, 5, value);
};


/**
 * optional ArtifactVerifyStatus status = 6;
 * @return {!proto.repository.ArtifactVerifyStatus}
 */
proto.repository.VerifyArtifactResponse.prototype.getStatus = function() {
  return /** @type {!proto.repository.ArtifactVerifyStatus} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {!proto.repository.ArtifactVerifyStatus} value
 * @return {!proto.repository.VerifyArtifactResponse} returns this
 */
proto.repository.VerifyArtifactResponse.prototype.setStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 6, value);
};


/**
 * optional string reason = 7;
 * @return {string}
 */
proto.repository.VerifyArtifactResponse.prototype.getReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.VerifyArtifactResponse} returns this
 */
proto.repository.VerifyArtifactResponse.prototype.setReason = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional string blob_key = 8;
 * @return {string}
 */
proto.repository.VerifyArtifactResponse.prototype.getBlobKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.VerifyArtifactResponse} returns this
 */
proto.repository.VerifyArtifactResponse.prototype.setBlobKey = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};


/**
 * optional int64 expected_size = 9;
 * @return {number}
 */
proto.repository.VerifyArtifactResponse.prototype.getExpectedSize = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 9, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.VerifyArtifactResponse} returns this
 */
proto.repository.VerifyArtifactResponse.prototype.setExpectedSize = function(value) {
  return jspb.Message.setProto3IntField(this, 9, value);
};


/**
 * optional int64 actual_size = 10;
 * @return {number}
 */
proto.repository.VerifyArtifactResponse.prototype.getActualSize = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 10, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.VerifyArtifactResponse} returns this
 */
proto.repository.VerifyArtifactResponse.prototype.setActualSize = function(value) {
  return jspb.Message.setProto3IntField(this, 10, value);
};


/**
 * optional string expected_digest = 11;
 * @return {string}
 */
proto.repository.VerifyArtifactResponse.prototype.getExpectedDigest = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 11, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.VerifyArtifactResponse} returns this
 */
proto.repository.VerifyArtifactResponse.prototype.setExpectedDigest = function(value) {
  return jspb.Message.setProto3StringField(this, 11, value);
};


/**
 * optional string actual_digest = 12;
 * @return {string}
 */
proto.repository.VerifyArtifactResponse.prototype.getActualDigest = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 12, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.VerifyArtifactResponse} returns this
 */
proto.repository.VerifyArtifactResponse.prototype.setActualDigest = function(value) {
  return jspb.Message.setProto3StringField(this, 12, value);
};


/**
 * optional string signature_status = 13;
 * @return {string}
 */
proto.repository.VerifyArtifactResponse.prototype.getSignatureStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 13, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.VerifyArtifactResponse} returns this
 */
proto.repository.VerifyArtifactResponse.prototype.setSignatureStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 13, value);
};


/**
 * optional string provenance_status = 14;
 * @return {string}
 */
proto.repository.VerifyArtifactResponse.prototype.getProvenanceStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 14, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.VerifyArtifactResponse} returns this
 */
proto.repository.VerifyArtifactResponse.prototype.setProvenanceStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 14, value);
};


/**
 * optional bool repairable = 15;
 * @return {boolean}
 */
proto.repository.VerifyArtifactResponse.prototype.getRepairable = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 15, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.VerifyArtifactResponse} returns this
 */
proto.repository.VerifyArtifactResponse.prototype.setRepairable = function(value) {
  return jspb.Message.setProto3BooleanField(this, 15, value);
};


/**
 * optional string recommended_action = 16;
 * @return {string}
 */
proto.repository.VerifyArtifactResponse.prototype.getRecommendedAction = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 16, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.VerifyArtifactResponse} returns this
 */
proto.repository.VerifyArtifactResponse.prototype.setRecommendedAction = function(value) {
  return jspb.Message.setProto3StringField(this, 16, value);
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
proto.repository.RepairArtifactRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.RepairArtifactRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.RepairArtifactRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RepairArtifactRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
ref: (f = msg.getRef()) && proto.repository.ArtifactRef.toObject(includeInstance, f),
buildNumber: jspb.Message.getFieldWithDefault(msg, 2, 0),
dryRun: jspb.Message.getBooleanFieldWithDefault(msg, 3, false),
force: jspb.Message.getBooleanFieldWithDefault(msg, 4, false),
allowQuarantineOverride: jspb.Message.getBooleanFieldWithDefault(msg, 5, false),
operatorSubject: jspb.Message.getFieldWithDefault(msg, 6, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.RepairArtifactRequest}
 */
proto.repository.RepairArtifactRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.RepairArtifactRequest;
  return proto.repository.RepairArtifactRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.RepairArtifactRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.RepairArtifactRequest}
 */
proto.repository.RepairArtifactRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactRef;
      reader.readMessage(value,proto.repository.ArtifactRef.deserializeBinaryFromReader);
      msg.setRef(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setDryRun(value);
      break;
    case 4:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setForce(value);
      break;
    case 5:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setAllowQuarantineOverride(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setOperatorSubject(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.RepairArtifactRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.RepairArtifactRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.RepairArtifactRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RepairArtifactRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRef();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.ArtifactRef.serializeBinaryToWriter
    );
  }
  f = message.getBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
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
  f = message.getForce();
  if (f) {
    writer.writeBool(
      4,
      f
    );
  }
  f = message.getAllowQuarantineOverride();
  if (f) {
    writer.writeBool(
      5,
      f
    );
  }
  f = message.getOperatorSubject();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
};


/**
 * optional ArtifactRef ref = 1;
 * @return {?proto.repository.ArtifactRef}
 */
proto.repository.RepairArtifactRequest.prototype.getRef = function() {
  return /** @type{?proto.repository.ArtifactRef} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactRef, 1));
};


/**
 * @param {?proto.repository.ArtifactRef|undefined} value
 * @return {!proto.repository.RepairArtifactRequest} returns this
*/
proto.repository.RepairArtifactRequest.prototype.setRef = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.RepairArtifactRequest} returns this
 */
proto.repository.RepairArtifactRequest.prototype.clearRef = function() {
  return this.setRef(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.RepairArtifactRequest.prototype.hasRef = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional int64 build_number = 2;
 * @return {number}
 */
proto.repository.RepairArtifactRequest.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.RepairArtifactRequest} returns this
 */
proto.repository.RepairArtifactRequest.prototype.setBuildNumber = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional bool dry_run = 3;
 * @return {boolean}
 */
proto.repository.RepairArtifactRequest.prototype.getDryRun = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.RepairArtifactRequest} returns this
 */
proto.repository.RepairArtifactRequest.prototype.setDryRun = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * optional bool force = 4;
 * @return {boolean}
 */
proto.repository.RepairArtifactRequest.prototype.getForce = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 4, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.RepairArtifactRequest} returns this
 */
proto.repository.RepairArtifactRequest.prototype.setForce = function(value) {
  return jspb.Message.setProto3BooleanField(this, 4, value);
};


/**
 * optional bool allow_quarantine_override = 5;
 * @return {boolean}
 */
proto.repository.RepairArtifactRequest.prototype.getAllowQuarantineOverride = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 5, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.RepairArtifactRequest} returns this
 */
proto.repository.RepairArtifactRequest.prototype.setAllowQuarantineOverride = function(value) {
  return jspb.Message.setProto3BooleanField(this, 5, value);
};


/**
 * optional string operator_subject = 6;
 * @return {string}
 */
proto.repository.RepairArtifactRequest.prototype.getOperatorSubject = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RepairArtifactRequest} returns this
 */
proto.repository.RepairArtifactRequest.prototype.setOperatorSubject = function(value) {
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
proto.repository.RepairArtifactResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.RepairArtifactResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.RepairArtifactResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RepairArtifactResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
ref: (f = msg.getRef()) && proto.repository.ArtifactRef.toObject(includeInstance, f),
artifactKey: jspb.Message.getFieldWithDefault(msg, 2, ""),
action: jspb.Message.getFieldWithDefault(msg, 3, ""),
detail: jspb.Message.getFieldWithDefault(msg, 4, ""),
artifactStateBefore: jspb.Message.getFieldWithDefault(msg, 5, ""),
artifactStateAfter: jspb.Message.getFieldWithDefault(msg, 6, ""),
workflowRunId: jspb.Message.getFieldWithDefault(msg, 7, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.RepairArtifactResponse}
 */
proto.repository.RepairArtifactResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.RepairArtifactResponse;
  return proto.repository.RepairArtifactResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.RepairArtifactResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.RepairArtifactResponse}
 */
proto.repository.RepairArtifactResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactRef;
      reader.readMessage(value,proto.repository.ArtifactRef.deserializeBinaryFromReader);
      msg.setRef(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setArtifactKey(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setAction(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setDetail(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setArtifactStateBefore(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setArtifactStateAfter(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setWorkflowRunId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.RepairArtifactResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.RepairArtifactResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.RepairArtifactResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RepairArtifactResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRef();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.ArtifactRef.serializeBinaryToWriter
    );
  }
  f = message.getArtifactKey();
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
  f = message.getDetail();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getArtifactStateBefore();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getArtifactStateAfter();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getWorkflowRunId();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
};


/**
 * optional ArtifactRef ref = 1;
 * @return {?proto.repository.ArtifactRef}
 */
proto.repository.RepairArtifactResponse.prototype.getRef = function() {
  return /** @type{?proto.repository.ArtifactRef} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactRef, 1));
};


/**
 * @param {?proto.repository.ArtifactRef|undefined} value
 * @return {!proto.repository.RepairArtifactResponse} returns this
*/
proto.repository.RepairArtifactResponse.prototype.setRef = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.RepairArtifactResponse} returns this
 */
proto.repository.RepairArtifactResponse.prototype.clearRef = function() {
  return this.setRef(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.RepairArtifactResponse.prototype.hasRef = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string artifact_key = 2;
 * @return {string}
 */
proto.repository.RepairArtifactResponse.prototype.getArtifactKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RepairArtifactResponse} returns this
 */
proto.repository.RepairArtifactResponse.prototype.setArtifactKey = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string action = 3;
 * @return {string}
 */
proto.repository.RepairArtifactResponse.prototype.getAction = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RepairArtifactResponse} returns this
 */
proto.repository.RepairArtifactResponse.prototype.setAction = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string detail = 4;
 * @return {string}
 */
proto.repository.RepairArtifactResponse.prototype.getDetail = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RepairArtifactResponse} returns this
 */
proto.repository.RepairArtifactResponse.prototype.setDetail = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string artifact_state_before = 5;
 * @return {string}
 */
proto.repository.RepairArtifactResponse.prototype.getArtifactStateBefore = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RepairArtifactResponse} returns this
 */
proto.repository.RepairArtifactResponse.prototype.setArtifactStateBefore = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string artifact_state_after = 6;
 * @return {string}
 */
proto.repository.RepairArtifactResponse.prototype.getArtifactStateAfter = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RepairArtifactResponse} returns this
 */
proto.repository.RepairArtifactResponse.prototype.setArtifactStateAfter = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string workflow_run_id = 7;
 * @return {string}
 */
proto.repository.RepairArtifactResponse.prototype.getWorkflowRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RepairArtifactResponse} returns this
 */
proto.repository.RepairArtifactResponse.prototype.setWorkflowRunId = function(value) {
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
proto.repository.ExplainArtifactRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ExplainArtifactRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ExplainArtifactRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ExplainArtifactRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
ref: (f = msg.getRef()) && proto.repository.ArtifactRef.toObject(includeInstance, f),
buildNumber: jspb.Message.getFieldWithDefault(msg, 2, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ExplainArtifactRequest}
 */
proto.repository.ExplainArtifactRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ExplainArtifactRequest;
  return proto.repository.ExplainArtifactRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ExplainArtifactRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ExplainArtifactRequest}
 */
proto.repository.ExplainArtifactRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactRef;
      reader.readMessage(value,proto.repository.ArtifactRef.deserializeBinaryFromReader);
      msg.setRef(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.ExplainArtifactRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ExplainArtifactRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ExplainArtifactRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ExplainArtifactRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRef();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.ArtifactRef.serializeBinaryToWriter
    );
  }
  f = message.getBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
      2,
      f
    );
  }
};


/**
 * optional ArtifactRef ref = 1;
 * @return {?proto.repository.ArtifactRef}
 */
proto.repository.ExplainArtifactRequest.prototype.getRef = function() {
  return /** @type{?proto.repository.ArtifactRef} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactRef, 1));
};


/**
 * @param {?proto.repository.ArtifactRef|undefined} value
 * @return {!proto.repository.ExplainArtifactRequest} returns this
*/
proto.repository.ExplainArtifactRequest.prototype.setRef = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.ExplainArtifactRequest} returns this
 */
proto.repository.ExplainArtifactRequest.prototype.clearRef = function() {
  return this.setRef(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.ExplainArtifactRequest.prototype.hasRef = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional int64 build_number = 2;
 * @return {number}
 */
proto.repository.ExplainArtifactRequest.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ExplainArtifactRequest} returns this
 */
proto.repository.ExplainArtifactRequest.prototype.setBuildNumber = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.ExplainArtifactResponse.repeatedFields_ = [19];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.ExplainArtifactResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ExplainArtifactResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ExplainArtifactResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ExplainArtifactResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
ref: (f = msg.getRef()) && proto.repository.ArtifactRef.toObject(includeInstance, f),
artifactKey: jspb.Message.getFieldWithDefault(msg, 2, ""),
artifactState: jspb.Message.getFieldWithDefault(msg, 3, ""),
publishState: jspb.Message.getFieldWithDefault(msg, 4, 0),
blobKey: jspb.Message.getFieldWithDefault(msg, 5, ""),
blobPresent: jspb.Message.getBooleanFieldWithDefault(msg, 6, false),
expectedSize: jspb.Message.getFieldWithDefault(msg, 7, 0),
actualSize: jspb.Message.getFieldWithDefault(msg, 8, 0),
expectedDigest: jspb.Message.getFieldWithDefault(msg, 9, ""),
actualDigest: jspb.Message.getFieldWithDefault(msg, 10, ""),
ledgerPresent: jspb.Message.getBooleanFieldWithDefault(msg, 11, false),
manifestPresent: jspb.Message.getBooleanFieldWithDefault(msg, 12, false),
signatureStatus: jspb.Message.getFieldWithDefault(msg, 13, ""),
installable: jspb.Message.getBooleanFieldWithDefault(msg, 14, false),
recommendedAction: jspb.Message.getFieldWithDefault(msg, 15, ""),
relatedWorkflowRunId: jspb.Message.getFieldWithDefault(msg, 16, ""),
verifyStatus: jspb.Message.getFieldWithDefault(msg, 17, 0),
detail: jspb.Message.getFieldWithDefault(msg, 18, ""),
sourceAvailabilityList: (f = jspb.Message.getRepeatedField(msg, 19)) == null ? undefined : f,
repairable: jspb.Message.getBooleanFieldWithDefault(msg, 20, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ExplainArtifactResponse}
 */
proto.repository.ExplainArtifactResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ExplainArtifactResponse;
  return proto.repository.ExplainArtifactResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ExplainArtifactResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ExplainArtifactResponse}
 */
proto.repository.ExplainArtifactResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactRef;
      reader.readMessage(value,proto.repository.ArtifactRef.deserializeBinaryFromReader);
      msg.setRef(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setArtifactKey(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setArtifactState(value);
      break;
    case 4:
      var value = /** @type {!proto.repository.PublishState} */ (reader.readEnum());
      msg.setPublishState(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setBlobKey(value);
      break;
    case 6:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setBlobPresent(value);
      break;
    case 7:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setExpectedSize(value);
      break;
    case 8:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setActualSize(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setExpectedDigest(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.setActualDigest(value);
      break;
    case 11:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setLedgerPresent(value);
      break;
    case 12:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setManifestPresent(value);
      break;
    case 13:
      var value = /** @type {string} */ (reader.readString());
      msg.setSignatureStatus(value);
      break;
    case 14:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setInstallable(value);
      break;
    case 15:
      var value = /** @type {string} */ (reader.readString());
      msg.setRecommendedAction(value);
      break;
    case 16:
      var value = /** @type {string} */ (reader.readString());
      msg.setRelatedWorkflowRunId(value);
      break;
    case 17:
      var value = /** @type {!proto.repository.ArtifactVerifyStatus} */ (reader.readEnum());
      msg.setVerifyStatus(value);
      break;
    case 18:
      var value = /** @type {string} */ (reader.readString());
      msg.setDetail(value);
      break;
    case 19:
      var value = /** @type {string} */ (reader.readString());
      msg.addSourceAvailability(value);
      break;
    case 20:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setRepairable(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.ExplainArtifactResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ExplainArtifactResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ExplainArtifactResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ExplainArtifactResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRef();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.ArtifactRef.serializeBinaryToWriter
    );
  }
  f = message.getArtifactKey();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getArtifactState();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getPublishState();
  if (f !== 0.0) {
    writer.writeEnum(
      4,
      f
    );
  }
  f = message.getBlobKey();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getBlobPresent();
  if (f) {
    writer.writeBool(
      6,
      f
    );
  }
  f = message.getExpectedSize();
  if (f !== 0) {
    writer.writeInt64(
      7,
      f
    );
  }
  f = message.getActualSize();
  if (f !== 0) {
    writer.writeInt64(
      8,
      f
    );
  }
  f = message.getExpectedDigest();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
  f = message.getActualDigest();
  if (f.length > 0) {
    writer.writeString(
      10,
      f
    );
  }
  f = message.getLedgerPresent();
  if (f) {
    writer.writeBool(
      11,
      f
    );
  }
  f = message.getManifestPresent();
  if (f) {
    writer.writeBool(
      12,
      f
    );
  }
  f = message.getSignatureStatus();
  if (f.length > 0) {
    writer.writeString(
      13,
      f
    );
  }
  f = message.getInstallable();
  if (f) {
    writer.writeBool(
      14,
      f
    );
  }
  f = message.getRecommendedAction();
  if (f.length > 0) {
    writer.writeString(
      15,
      f
    );
  }
  f = message.getRelatedWorkflowRunId();
  if (f.length > 0) {
    writer.writeString(
      16,
      f
    );
  }
  f = message.getVerifyStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      17,
      f
    );
  }
  f = message.getDetail();
  if (f.length > 0) {
    writer.writeString(
      18,
      f
    );
  }
  f = message.getSourceAvailabilityList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      19,
      f
    );
  }
  f = message.getRepairable();
  if (f) {
    writer.writeBool(
      20,
      f
    );
  }
};


/**
 * optional ArtifactRef ref = 1;
 * @return {?proto.repository.ArtifactRef}
 */
proto.repository.ExplainArtifactResponse.prototype.getRef = function() {
  return /** @type{?proto.repository.ArtifactRef} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactRef, 1));
};


/**
 * @param {?proto.repository.ArtifactRef|undefined} value
 * @return {!proto.repository.ExplainArtifactResponse} returns this
*/
proto.repository.ExplainArtifactResponse.prototype.setRef = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.clearRef = function() {
  return this.setRef(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.ExplainArtifactResponse.prototype.hasRef = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string artifact_key = 2;
 * @return {string}
 */
proto.repository.ExplainArtifactResponse.prototype.getArtifactKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.setArtifactKey = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string artifact_state = 3;
 * @return {string}
 */
proto.repository.ExplainArtifactResponse.prototype.getArtifactState = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.setArtifactState = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional PublishState publish_state = 4;
 * @return {!proto.repository.PublishState}
 */
proto.repository.ExplainArtifactResponse.prototype.getPublishState = function() {
  return /** @type {!proto.repository.PublishState} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {!proto.repository.PublishState} value
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.setPublishState = function(value) {
  return jspb.Message.setProto3EnumField(this, 4, value);
};


/**
 * optional string blob_key = 5;
 * @return {string}
 */
proto.repository.ExplainArtifactResponse.prototype.getBlobKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.setBlobKey = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional bool blob_present = 6;
 * @return {boolean}
 */
proto.repository.ExplainArtifactResponse.prototype.getBlobPresent = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 6, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.setBlobPresent = function(value) {
  return jspb.Message.setProto3BooleanField(this, 6, value);
};


/**
 * optional int64 expected_size = 7;
 * @return {number}
 */
proto.repository.ExplainArtifactResponse.prototype.getExpectedSize = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.setExpectedSize = function(value) {
  return jspb.Message.setProto3IntField(this, 7, value);
};


/**
 * optional int64 actual_size = 8;
 * @return {number}
 */
proto.repository.ExplainArtifactResponse.prototype.getActualSize = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 8, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.setActualSize = function(value) {
  return jspb.Message.setProto3IntField(this, 8, value);
};


/**
 * optional string expected_digest = 9;
 * @return {string}
 */
proto.repository.ExplainArtifactResponse.prototype.getExpectedDigest = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.setExpectedDigest = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};


/**
 * optional string actual_digest = 10;
 * @return {string}
 */
proto.repository.ExplainArtifactResponse.prototype.getActualDigest = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 10, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.setActualDigest = function(value) {
  return jspb.Message.setProto3StringField(this, 10, value);
};


/**
 * optional bool ledger_present = 11;
 * @return {boolean}
 */
proto.repository.ExplainArtifactResponse.prototype.getLedgerPresent = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 11, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.setLedgerPresent = function(value) {
  return jspb.Message.setProto3BooleanField(this, 11, value);
};


/**
 * optional bool manifest_present = 12;
 * @return {boolean}
 */
proto.repository.ExplainArtifactResponse.prototype.getManifestPresent = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 12, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.setManifestPresent = function(value) {
  return jspb.Message.setProto3BooleanField(this, 12, value);
};


/**
 * optional string signature_status = 13;
 * @return {string}
 */
proto.repository.ExplainArtifactResponse.prototype.getSignatureStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 13, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.setSignatureStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 13, value);
};


/**
 * optional bool installable = 14;
 * @return {boolean}
 */
proto.repository.ExplainArtifactResponse.prototype.getInstallable = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 14, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.setInstallable = function(value) {
  return jspb.Message.setProto3BooleanField(this, 14, value);
};


/**
 * optional string recommended_action = 15;
 * @return {string}
 */
proto.repository.ExplainArtifactResponse.prototype.getRecommendedAction = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 15, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.setRecommendedAction = function(value) {
  return jspb.Message.setProto3StringField(this, 15, value);
};


/**
 * optional string related_workflow_run_id = 16;
 * @return {string}
 */
proto.repository.ExplainArtifactResponse.prototype.getRelatedWorkflowRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 16, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.setRelatedWorkflowRunId = function(value) {
  return jspb.Message.setProto3StringField(this, 16, value);
};


/**
 * optional ArtifactVerifyStatus verify_status = 17;
 * @return {!proto.repository.ArtifactVerifyStatus}
 */
proto.repository.ExplainArtifactResponse.prototype.getVerifyStatus = function() {
  return /** @type {!proto.repository.ArtifactVerifyStatus} */ (jspb.Message.getFieldWithDefault(this, 17, 0));
};


/**
 * @param {!proto.repository.ArtifactVerifyStatus} value
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.setVerifyStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 17, value);
};


/**
 * optional string detail = 18;
 * @return {string}
 */
proto.repository.ExplainArtifactResponse.prototype.getDetail = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 18, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.setDetail = function(value) {
  return jspb.Message.setProto3StringField(this, 18, value);
};


/**
 * repeated string source_availability = 19;
 * @return {!Array<string>}
 */
proto.repository.ExplainArtifactResponse.prototype.getSourceAvailabilityList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 19));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.setSourceAvailabilityList = function(value) {
  return jspb.Message.setField(this, 19, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.addSourceAvailability = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 19, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.clearSourceAvailabilityList = function() {
  return this.setSourceAvailabilityList([]);
};


/**
 * optional bool repairable = 20;
 * @return {boolean}
 */
proto.repository.ExplainArtifactResponse.prototype.getRepairable = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 20, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.ExplainArtifactResponse} returns this
 */
proto.repository.ExplainArtifactResponse.prototype.setRepairable = function(value) {
  return jspb.Message.setProto3BooleanField(this, 20, value);
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
proto.repository.ResolveArtifactRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ResolveArtifactRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ResolveArtifactRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ResolveArtifactRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
publisherId: jspb.Message.getFieldWithDefault(msg, 1, ""),
name: jspb.Message.getFieldWithDefault(msg, 2, ""),
kind: jspb.Message.getFieldWithDefault(msg, 3, 0),
platform: jspb.Message.getFieldWithDefault(msg, 4, ""),
channel: jspb.Message.getFieldWithDefault(msg, 5, 0),
version: jspb.Message.getFieldWithDefault(msg, 6, ""),
buildId: jspb.Message.getFieldWithDefault(msg, 7, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ResolveArtifactRequest}
 */
proto.repository.ResolveArtifactRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ResolveArtifactRequest;
  return proto.repository.ResolveArtifactRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ResolveArtifactRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ResolveArtifactRequest}
 */
proto.repository.ResolveArtifactRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisherId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    case 3:
      var value = /** @type {!proto.repository.ArtifactKind} */ (reader.readEnum());
      msg.setKind(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlatform(value);
      break;
    case 5:
      var value = /** @type {!proto.repository.ArtifactChannel} */ (reader.readEnum());
      msg.setChannel(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setVersion(value);
      break;
    case 7:
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
proto.repository.ResolveArtifactRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ResolveArtifactRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ResolveArtifactRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ResolveArtifactRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublisherId();
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
  f = message.getKind();
  if (f !== 0.0) {
    writer.writeEnum(
      3,
      f
    );
  }
  f = message.getPlatform();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getChannel();
  if (f !== 0.0) {
    writer.writeEnum(
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
  f = message.getBuildId();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
};


/**
 * optional string publisher_id = 1;
 * @return {string}
 */
proto.repository.ResolveArtifactRequest.prototype.getPublisherId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ResolveArtifactRequest} returns this
 */
proto.repository.ResolveArtifactRequest.prototype.setPublisherId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string name = 2;
 * @return {string}
 */
proto.repository.ResolveArtifactRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ResolveArtifactRequest} returns this
 */
proto.repository.ResolveArtifactRequest.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional ArtifactKind kind = 3;
 * @return {!proto.repository.ArtifactKind}
 */
proto.repository.ResolveArtifactRequest.prototype.getKind = function() {
  return /** @type {!proto.repository.ArtifactKind} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.repository.ArtifactKind} value
 * @return {!proto.repository.ResolveArtifactRequest} returns this
 */
proto.repository.ResolveArtifactRequest.prototype.setKind = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
};


/**
 * optional string platform = 4;
 * @return {string}
 */
proto.repository.ResolveArtifactRequest.prototype.getPlatform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ResolveArtifactRequest} returns this
 */
proto.repository.ResolveArtifactRequest.prototype.setPlatform = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional ArtifactChannel channel = 5;
 * @return {!proto.repository.ArtifactChannel}
 */
proto.repository.ResolveArtifactRequest.prototype.getChannel = function() {
  return /** @type {!proto.repository.ArtifactChannel} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {!proto.repository.ArtifactChannel} value
 * @return {!proto.repository.ResolveArtifactRequest} returns this
 */
proto.repository.ResolveArtifactRequest.prototype.setChannel = function(value) {
  return jspb.Message.setProto3EnumField(this, 5, value);
};


/**
 * optional string version = 6;
 * @return {string}
 */
proto.repository.ResolveArtifactRequest.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ResolveArtifactRequest} returns this
 */
proto.repository.ResolveArtifactRequest.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string build_id = 7;
 * @return {string}
 */
proto.repository.ResolveArtifactRequest.prototype.getBuildId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ResolveArtifactRequest} returns this
 */
proto.repository.ResolveArtifactRequest.prototype.setBuildId = function(value) {
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
proto.repository.ResolveArtifactResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ResolveArtifactResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ResolveArtifactResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ResolveArtifactResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
manifest: (f = msg.getManifest()) && proto.repository.ArtifactManifest.toObject(includeInstance, f),
resolutionSource: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ResolveArtifactResponse}
 */
proto.repository.ResolveArtifactResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ResolveArtifactResponse;
  return proto.repository.ResolveArtifactResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ResolveArtifactResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ResolveArtifactResponse}
 */
proto.repository.ResolveArtifactResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactManifest;
      reader.readMessage(value,proto.repository.ArtifactManifest.deserializeBinaryFromReader);
      msg.setManifest(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setResolutionSource(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.ResolveArtifactResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ResolveArtifactResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ResolveArtifactResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ResolveArtifactResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getManifest();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.ArtifactManifest.serializeBinaryToWriter
    );
  }
  f = message.getResolutionSource();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional ArtifactManifest manifest = 1;
 * @return {?proto.repository.ArtifactManifest}
 */
proto.repository.ResolveArtifactResponse.prototype.getManifest = function() {
  return /** @type{?proto.repository.ArtifactManifest} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactManifest, 1));
};


/**
 * @param {?proto.repository.ArtifactManifest|undefined} value
 * @return {!proto.repository.ResolveArtifactResponse} returns this
*/
proto.repository.ResolveArtifactResponse.prototype.setManifest = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.ResolveArtifactResponse} returns this
 */
proto.repository.ResolveArtifactResponse.prototype.clearManifest = function() {
  return this.setManifest(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.ResolveArtifactResponse.prototype.hasManifest = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string resolution_source = 2;
 * @return {string}
 */
proto.repository.ResolveArtifactResponse.prototype.getResolutionSource = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ResolveArtifactResponse} returns this
 */
proto.repository.ResolveArtifactResponse.prototype.setResolutionSource = function(value) {
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
proto.repository.ResolveByEntrypointChecksumRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ResolveByEntrypointChecksumRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ResolveByEntrypointChecksumRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ResolveByEntrypointChecksumRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
checksum: jspb.Message.getFieldWithDefault(msg, 1, ""),
platform: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ResolveByEntrypointChecksumRequest}
 */
proto.repository.ResolveByEntrypointChecksumRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ResolveByEntrypointChecksumRequest;
  return proto.repository.ResolveByEntrypointChecksumRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ResolveByEntrypointChecksumRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ResolveByEntrypointChecksumRequest}
 */
proto.repository.ResolveByEntrypointChecksumRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setChecksum(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlatform(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.ResolveByEntrypointChecksumRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ResolveByEntrypointChecksumRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ResolveByEntrypointChecksumRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ResolveByEntrypointChecksumRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getChecksum();
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
};


/**
 * optional string checksum = 1;
 * @return {string}
 */
proto.repository.ResolveByEntrypointChecksumRequest.prototype.getChecksum = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ResolveByEntrypointChecksumRequest} returns this
 */
proto.repository.ResolveByEntrypointChecksumRequest.prototype.setChecksum = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string platform = 2;
 * @return {string}
 */
proto.repository.ResolveByEntrypointChecksumRequest.prototype.getPlatform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ResolveByEntrypointChecksumRequest} returns this
 */
proto.repository.ResolveByEntrypointChecksumRequest.prototype.setPlatform = function(value) {
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
proto.repository.ResolveByEntrypointChecksumResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ResolveByEntrypointChecksumResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ResolveByEntrypointChecksumResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ResolveByEntrypointChecksumResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
manifest: (f = msg.getManifest()) && proto.repository.ArtifactManifest.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ResolveByEntrypointChecksumResponse}
 */
proto.repository.ResolveByEntrypointChecksumResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ResolveByEntrypointChecksumResponse;
  return proto.repository.ResolveByEntrypointChecksumResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ResolveByEntrypointChecksumResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ResolveByEntrypointChecksumResponse}
 */
proto.repository.ResolveByEntrypointChecksumResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactManifest;
      reader.readMessage(value,proto.repository.ArtifactManifest.deserializeBinaryFromReader);
      msg.setManifest(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.ResolveByEntrypointChecksumResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ResolveByEntrypointChecksumResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ResolveByEntrypointChecksumResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ResolveByEntrypointChecksumResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getManifest();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.ArtifactManifest.serializeBinaryToWriter
    );
  }
};


/**
 * optional ArtifactManifest manifest = 1;
 * @return {?proto.repository.ArtifactManifest}
 */
proto.repository.ResolveByEntrypointChecksumResponse.prototype.getManifest = function() {
  return /** @type{?proto.repository.ArtifactManifest} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactManifest, 1));
};


/**
 * @param {?proto.repository.ArtifactManifest|undefined} value
 * @return {!proto.repository.ResolveByEntrypointChecksumResponse} returns this
*/
proto.repository.ResolveByEntrypointChecksumResponse.prototype.setManifest = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.ResolveByEntrypointChecksumResponse} returns this
 */
proto.repository.ResolveByEntrypointChecksumResponse.prototype.clearManifest = function() {
  return this.setManifest(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.ResolveByEntrypointChecksumResponse.prototype.hasManifest = function() {
  return jspb.Message.getField(this, 1) != null;
};



/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.repository.UpdateArtifactBinaryRequest.oneofGroups_ = [[1,2]];

/**
 * @enum {number}
 */
proto.repository.UpdateArtifactBinaryRequest.PayloadCase = {
  PAYLOAD_NOT_SET: 0,
  HEADER: 1,
  CHUNK: 2
};

/**
 * @return {proto.repository.UpdateArtifactBinaryRequest.PayloadCase}
 */
proto.repository.UpdateArtifactBinaryRequest.prototype.getPayloadCase = function() {
  return /** @type {proto.repository.UpdateArtifactBinaryRequest.PayloadCase} */(jspb.Message.computeOneofCase(this, proto.repository.UpdateArtifactBinaryRequest.oneofGroups_[0]));
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
proto.repository.UpdateArtifactBinaryRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.UpdateArtifactBinaryRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.UpdateArtifactBinaryRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.UpdateArtifactBinaryRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
header: (f = msg.getHeader()) && proto.repository.UpdateArtifactBinaryHeader.toObject(includeInstance, f),
chunk: msg.getChunk_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.UpdateArtifactBinaryRequest}
 */
proto.repository.UpdateArtifactBinaryRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.UpdateArtifactBinaryRequest;
  return proto.repository.UpdateArtifactBinaryRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.UpdateArtifactBinaryRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.UpdateArtifactBinaryRequest}
 */
proto.repository.UpdateArtifactBinaryRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.UpdateArtifactBinaryHeader;
      reader.readMessage(value,proto.repository.UpdateArtifactBinaryHeader.deserializeBinaryFromReader);
      msg.setHeader(value);
      break;
    case 2:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setChunk(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.UpdateArtifactBinaryRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.UpdateArtifactBinaryRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.UpdateArtifactBinaryRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.UpdateArtifactBinaryRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getHeader();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.UpdateArtifactBinaryHeader.serializeBinaryToWriter
    );
  }
  f = /** @type {!(string|Uint8Array)} */ (jspb.Message.getField(message, 2));
  if (f != null) {
    writer.writeBytes(
      2,
      f
    );
  }
};


/**
 * optional UpdateArtifactBinaryHeader header = 1;
 * @return {?proto.repository.UpdateArtifactBinaryHeader}
 */
proto.repository.UpdateArtifactBinaryRequest.prototype.getHeader = function() {
  return /** @type{?proto.repository.UpdateArtifactBinaryHeader} */ (
    jspb.Message.getWrapperField(this, proto.repository.UpdateArtifactBinaryHeader, 1));
};


/**
 * @param {?proto.repository.UpdateArtifactBinaryHeader|undefined} value
 * @return {!proto.repository.UpdateArtifactBinaryRequest} returns this
*/
proto.repository.UpdateArtifactBinaryRequest.prototype.setHeader = function(value) {
  return jspb.Message.setOneofWrapperField(this, 1, proto.repository.UpdateArtifactBinaryRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.UpdateArtifactBinaryRequest} returns this
 */
proto.repository.UpdateArtifactBinaryRequest.prototype.clearHeader = function() {
  return this.setHeader(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.UpdateArtifactBinaryRequest.prototype.hasHeader = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional bytes chunk = 2;
 * @return {string}
 */
proto.repository.UpdateArtifactBinaryRequest.prototype.getChunk = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * optional bytes chunk = 2;
 * This is a type-conversion wrapper around `getChunk()`
 * @return {string}
 */
proto.repository.UpdateArtifactBinaryRequest.prototype.getChunk_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getChunk()));
};


/**
 * optional bytes chunk = 2;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getChunk()`
 * @return {!Uint8Array}
 */
proto.repository.UpdateArtifactBinaryRequest.prototype.getChunk_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getChunk()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.repository.UpdateArtifactBinaryRequest} returns this
 */
proto.repository.UpdateArtifactBinaryRequest.prototype.setChunk = function(value) {
  return jspb.Message.setOneofField(this, 2, proto.repository.UpdateArtifactBinaryRequest.oneofGroups_[0], value);
};


/**
 * Clears the field making it undefined.
 * @return {!proto.repository.UpdateArtifactBinaryRequest} returns this
 */
proto.repository.UpdateArtifactBinaryRequest.prototype.clearChunk = function() {
  return jspb.Message.setOneofField(this, 2, proto.repository.UpdateArtifactBinaryRequest.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.UpdateArtifactBinaryRequest.prototype.hasChunk = function() {
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
proto.repository.UpdateArtifactBinaryHeader.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.UpdateArtifactBinaryHeader.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.UpdateArtifactBinaryHeader} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.UpdateArtifactBinaryHeader.toObject = function(includeInstance, msg) {
  var f, obj = {
ref: (f = msg.getRef()) && proto.repository.ArtifactRef.toObject(includeInstance, f),
checksum: jspb.Message.getFieldWithDefault(msg, 2, ""),
sizeBytes: jspb.Message.getFieldWithDefault(msg, 3, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.UpdateArtifactBinaryHeader}
 */
proto.repository.UpdateArtifactBinaryHeader.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.UpdateArtifactBinaryHeader;
  return proto.repository.UpdateArtifactBinaryHeader.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.UpdateArtifactBinaryHeader} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.UpdateArtifactBinaryHeader}
 */
proto.repository.UpdateArtifactBinaryHeader.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactRef;
      reader.readMessage(value,proto.repository.ArtifactRef.deserializeBinaryFromReader);
      msg.setRef(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setChecksum(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setSizeBytes(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.UpdateArtifactBinaryHeader.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.UpdateArtifactBinaryHeader.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.UpdateArtifactBinaryHeader} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.UpdateArtifactBinaryHeader.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRef();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.ArtifactRef.serializeBinaryToWriter
    );
  }
  f = message.getChecksum();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getSizeBytes();
  if (f !== 0) {
    writer.writeInt64(
      3,
      f
    );
  }
};


/**
 * optional ArtifactRef ref = 1;
 * @return {?proto.repository.ArtifactRef}
 */
proto.repository.UpdateArtifactBinaryHeader.prototype.getRef = function() {
  return /** @type{?proto.repository.ArtifactRef} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactRef, 1));
};


/**
 * @param {?proto.repository.ArtifactRef|undefined} value
 * @return {!proto.repository.UpdateArtifactBinaryHeader} returns this
*/
proto.repository.UpdateArtifactBinaryHeader.prototype.setRef = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.UpdateArtifactBinaryHeader} returns this
 */
proto.repository.UpdateArtifactBinaryHeader.prototype.clearRef = function() {
  return this.setRef(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.UpdateArtifactBinaryHeader.prototype.hasRef = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string checksum = 2;
 * @return {string}
 */
proto.repository.UpdateArtifactBinaryHeader.prototype.getChecksum = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpdateArtifactBinaryHeader} returns this
 */
proto.repository.UpdateArtifactBinaryHeader.prototype.setChecksum = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int64 size_bytes = 3;
 * @return {number}
 */
proto.repository.UpdateArtifactBinaryHeader.prototype.getSizeBytes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.UpdateArtifactBinaryHeader} returns this
 */
proto.repository.UpdateArtifactBinaryHeader.prototype.setSizeBytes = function(value) {
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
proto.repository.UpdateArtifactBinaryResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.UpdateArtifactBinaryResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.UpdateArtifactBinaryResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.UpdateArtifactBinaryResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
buildNumber: jspb.Message.getFieldWithDefault(msg, 1, 0),
checksum: jspb.Message.getFieldWithDefault(msg, 2, ""),
status: jspb.Message.getFieldWithDefault(msg, 3, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.UpdateArtifactBinaryResponse}
 */
proto.repository.UpdateArtifactBinaryResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.UpdateArtifactBinaryResponse;
  return proto.repository.UpdateArtifactBinaryResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.UpdateArtifactBinaryResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.UpdateArtifactBinaryResponse}
 */
proto.repository.UpdateArtifactBinaryResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setChecksum(value);
      break;
    case 3:
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
proto.repository.UpdateArtifactBinaryResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.UpdateArtifactBinaryResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.UpdateArtifactBinaryResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.UpdateArtifactBinaryResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
      1,
      f
    );
  }
  f = message.getChecksum();
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
};


/**
 * optional int64 build_number = 1;
 * @return {number}
 */
proto.repository.UpdateArtifactBinaryResponse.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.UpdateArtifactBinaryResponse} returns this
 */
proto.repository.UpdateArtifactBinaryResponse.prototype.setBuildNumber = function(value) {
  return jspb.Message.setProto3IntField(this, 1, value);
};


/**
 * optional string checksum = 2;
 * @return {string}
 */
proto.repository.UpdateArtifactBinaryResponse.prototype.getChecksum = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpdateArtifactBinaryResponse} returns this
 */
proto.repository.UpdateArtifactBinaryResponse.prototype.setChecksum = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string status = 3;
 * @return {string}
 */
proto.repository.UpdateArtifactBinaryResponse.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpdateArtifactBinaryResponse} returns this
 */
proto.repository.UpdateArtifactBinaryResponse.prototype.setStatus = function(value) {
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
proto.repository.AllocateUploadRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.AllocateUploadRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.AllocateUploadRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.AllocateUploadRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
publisherId: jspb.Message.getFieldWithDefault(msg, 1, ""),
name: jspb.Message.getFieldWithDefault(msg, 2, ""),
platform: jspb.Message.getFieldWithDefault(msg, 3, ""),
intent: jspb.Message.getFieldWithDefault(msg, 4, 0),
exactVersion: jspb.Message.getFieldWithDefault(msg, 5, ""),
channel: jspb.Message.getFieldWithDefault(msg, 6, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.AllocateUploadRequest}
 */
proto.repository.AllocateUploadRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.AllocateUploadRequest;
  return proto.repository.AllocateUploadRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.AllocateUploadRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.AllocateUploadRequest}
 */
proto.repository.AllocateUploadRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisherId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlatform(value);
      break;
    case 4:
      var value = /** @type {!proto.repository.VersionIntent} */ (reader.readEnum());
      msg.setIntent(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setExactVersion(value);
      break;
    case 6:
      var value = /** @type {!proto.repository.ArtifactChannel} */ (reader.readEnum());
      msg.setChannel(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.AllocateUploadRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.AllocateUploadRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.AllocateUploadRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.AllocateUploadRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublisherId();
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
  f = message.getPlatform();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getIntent();
  if (f !== 0.0) {
    writer.writeEnum(
      4,
      f
    );
  }
  f = message.getExactVersion();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getChannel();
  if (f !== 0.0) {
    writer.writeEnum(
      6,
      f
    );
  }
};


/**
 * optional string publisher_id = 1;
 * @return {string}
 */
proto.repository.AllocateUploadRequest.prototype.getPublisherId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.AllocateUploadRequest} returns this
 */
proto.repository.AllocateUploadRequest.prototype.setPublisherId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string name = 2;
 * @return {string}
 */
proto.repository.AllocateUploadRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.AllocateUploadRequest} returns this
 */
proto.repository.AllocateUploadRequest.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string platform = 3;
 * @return {string}
 */
proto.repository.AllocateUploadRequest.prototype.getPlatform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.AllocateUploadRequest} returns this
 */
proto.repository.AllocateUploadRequest.prototype.setPlatform = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional VersionIntent intent = 4;
 * @return {!proto.repository.VersionIntent}
 */
proto.repository.AllocateUploadRequest.prototype.getIntent = function() {
  return /** @type {!proto.repository.VersionIntent} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {!proto.repository.VersionIntent} value
 * @return {!proto.repository.AllocateUploadRequest} returns this
 */
proto.repository.AllocateUploadRequest.prototype.setIntent = function(value) {
  return jspb.Message.setProto3EnumField(this, 4, value);
};


/**
 * optional string exact_version = 5;
 * @return {string}
 */
proto.repository.AllocateUploadRequest.prototype.getExactVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.AllocateUploadRequest} returns this
 */
proto.repository.AllocateUploadRequest.prototype.setExactVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional ArtifactChannel channel = 6;
 * @return {!proto.repository.ArtifactChannel}
 */
proto.repository.AllocateUploadRequest.prototype.getChannel = function() {
  return /** @type {!proto.repository.ArtifactChannel} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {!proto.repository.ArtifactChannel} value
 * @return {!proto.repository.AllocateUploadRequest} returns this
 */
proto.repository.AllocateUploadRequest.prototype.setChannel = function(value) {
  return jspb.Message.setProto3EnumField(this, 6, value);
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
proto.repository.AllocateUploadResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.AllocateUploadResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.AllocateUploadResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.AllocateUploadResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
version: jspb.Message.getFieldWithDefault(msg, 1, ""),
reservationId: jspb.Message.getFieldWithDefault(msg, 2, ""),
buildId: jspb.Message.getFieldWithDefault(msg, 3, ""),
buildNumber: jspb.Message.getFieldWithDefault(msg, 4, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.AllocateUploadResponse}
 */
proto.repository.AllocateUploadResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.AllocateUploadResponse;
  return proto.repository.AllocateUploadResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.AllocateUploadResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.AllocateUploadResponse}
 */
proto.repository.AllocateUploadResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setVersion(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setReservationId(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setBuildId(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.AllocateUploadResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.AllocateUploadResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.AllocateUploadResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.AllocateUploadResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getVersion();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getReservationId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getBuildId();
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
};


/**
 * optional string version = 1;
 * @return {string}
 */
proto.repository.AllocateUploadResponse.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.AllocateUploadResponse} returns this
 */
proto.repository.AllocateUploadResponse.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string reservation_id = 2;
 * @return {string}
 */
proto.repository.AllocateUploadResponse.prototype.getReservationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.AllocateUploadResponse} returns this
 */
proto.repository.AllocateUploadResponse.prototype.setReservationId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string build_id = 3;
 * @return {string}
 */
proto.repository.AllocateUploadResponse.prototype.getBuildId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.AllocateUploadResponse} returns this
 */
proto.repository.AllocateUploadResponse.prototype.setBuildId = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional int64 build_number = 4;
 * @return {number}
 */
proto.repository.AllocateUploadResponse.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.AllocateUploadResponse} returns this
 */
proto.repository.AllocateUploadResponse.prototype.setBuildNumber = function(value) {
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
proto.repository.ImportProvisionalRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ImportProvisionalRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ImportProvisionalRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ImportProvisionalRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
publisherId: jspb.Message.getFieldWithDefault(msg, 1, ""),
name: jspb.Message.getFieldWithDefault(msg, 2, ""),
version: jspb.Message.getFieldWithDefault(msg, 3, ""),
platform: jspb.Message.getFieldWithDefault(msg, 4, ""),
digest: jspb.Message.getFieldWithDefault(msg, 5, ""),
provisionalBuildId: jspb.Message.getFieldWithDefault(msg, 6, ""),
data: msg.getData_asB64(),
kind: jspb.Message.getFieldWithDefault(msg, 8, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ImportProvisionalRequest}
 */
proto.repository.ImportProvisionalRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ImportProvisionalRequest;
  return proto.repository.ImportProvisionalRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ImportProvisionalRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ImportProvisionalRequest}
 */
proto.repository.ImportProvisionalRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisherId(value);
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
      msg.setPlatform(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setDigest(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setProvisionalBuildId(value);
      break;
    case 7:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setData(value);
      break;
    case 8:
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
proto.repository.ImportProvisionalRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ImportProvisionalRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ImportProvisionalRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ImportProvisionalRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublisherId();
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
  f = message.getPlatform();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getDigest();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getProvisionalBuildId();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getData_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      7,
      f
    );
  }
  f = message.getKind();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
};


/**
 * optional string publisher_id = 1;
 * @return {string}
 */
proto.repository.ImportProvisionalRequest.prototype.getPublisherId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ImportProvisionalRequest} returns this
 */
proto.repository.ImportProvisionalRequest.prototype.setPublisherId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string name = 2;
 * @return {string}
 */
proto.repository.ImportProvisionalRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ImportProvisionalRequest} returns this
 */
proto.repository.ImportProvisionalRequest.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string version = 3;
 * @return {string}
 */
proto.repository.ImportProvisionalRequest.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ImportProvisionalRequest} returns this
 */
proto.repository.ImportProvisionalRequest.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string platform = 4;
 * @return {string}
 */
proto.repository.ImportProvisionalRequest.prototype.getPlatform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ImportProvisionalRequest} returns this
 */
proto.repository.ImportProvisionalRequest.prototype.setPlatform = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string digest = 5;
 * @return {string}
 */
proto.repository.ImportProvisionalRequest.prototype.getDigest = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ImportProvisionalRequest} returns this
 */
proto.repository.ImportProvisionalRequest.prototype.setDigest = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string provisional_build_id = 6;
 * @return {string}
 */
proto.repository.ImportProvisionalRequest.prototype.getProvisionalBuildId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ImportProvisionalRequest} returns this
 */
proto.repository.ImportProvisionalRequest.prototype.setProvisionalBuildId = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional bytes data = 7;
 * @return {string}
 */
proto.repository.ImportProvisionalRequest.prototype.getData = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * optional bytes data = 7;
 * This is a type-conversion wrapper around `getData()`
 * @return {string}
 */
proto.repository.ImportProvisionalRequest.prototype.getData_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getData()));
};


/**
 * optional bytes data = 7;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getData()`
 * @return {!Uint8Array}
 */
proto.repository.ImportProvisionalRequest.prototype.getData_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getData()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.repository.ImportProvisionalRequest} returns this
 */
proto.repository.ImportProvisionalRequest.prototype.setData = function(value) {
  return jspb.Message.setProto3BytesField(this, 7, value);
};


/**
 * optional string kind = 8;
 * @return {string}
 */
proto.repository.ImportProvisionalRequest.prototype.getKind = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ImportProvisionalRequest} returns this
 */
proto.repository.ImportProvisionalRequest.prototype.setKind = function(value) {
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
proto.repository.ImportProvisionalResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ImportProvisionalResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ImportProvisionalResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ImportProvisionalResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
ok: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
confirmedBuildId: jspb.Message.getFieldWithDefault(msg, 2, ""),
confirmedVersion: jspb.Message.getFieldWithDefault(msg, 3, ""),
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
 * @return {!proto.repository.ImportProvisionalResponse}
 */
proto.repository.ImportProvisionalResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ImportProvisionalResponse;
  return proto.repository.ImportProvisionalResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ImportProvisionalResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ImportProvisionalResponse}
 */
proto.repository.ImportProvisionalResponse.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setConfirmedBuildId(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setConfirmedVersion(value);
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
proto.repository.ImportProvisionalResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ImportProvisionalResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ImportProvisionalResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ImportProvisionalResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOk();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getConfirmedBuildId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getConfirmedVersion();
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
proto.repository.ImportProvisionalResponse.prototype.getOk = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.ImportProvisionalResponse} returns this
 */
proto.repository.ImportProvisionalResponse.prototype.setOk = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional string confirmed_build_id = 2;
 * @return {string}
 */
proto.repository.ImportProvisionalResponse.prototype.getConfirmedBuildId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ImportProvisionalResponse} returns this
 */
proto.repository.ImportProvisionalResponse.prototype.setConfirmedBuildId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string confirmed_version = 3;
 * @return {string}
 */
proto.repository.ImportProvisionalResponse.prototype.getConfirmedVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ImportProvisionalResponse} returns this
 */
proto.repository.ImportProvisionalResponse.prototype.setConfirmedVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string state = 4;
 * @return {string}
 */
proto.repository.ImportProvisionalResponse.prototype.getState = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ImportProvisionalResponse} returns this
 */
proto.repository.ImportProvisionalResponse.prototype.setState = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string message = 5;
 * @return {string}
 */
proto.repository.ImportProvisionalResponse.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ImportProvisionalResponse} returns this
 */
proto.repository.ImportProvisionalResponse.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.UpstreamSource.repeatedFields_ = [11,12,13];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.UpstreamSource.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.UpstreamSource.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.UpstreamSource} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.UpstreamSource.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, ""),
type: jspb.Message.getFieldWithDefault(msg, 2, 0),
indexUrl: jspb.Message.getFieldWithDefault(msg, 3, ""),
channel: jspb.Message.getFieldWithDefault(msg, 4, ""),
platform: jspb.Message.getFieldWithDefault(msg, 5, ""),
enabled: jspb.Message.getBooleanFieldWithDefault(msg, 6, false),
lastSyncedTag: jspb.Message.getFieldWithDefault(msg, 7, ""),
credentialsRef: jspb.Message.getFieldWithDefault(msg, 8, ""),
defaultPublisherId: jspb.Message.getFieldWithDefault(msg, 10, ""),
allowedPublishersList: (f = jspb.Message.getRepeatedField(msg, 11)) == null ? undefined : f,
allowedKindsList: (f = jspb.Message.getRepeatedField(msg, 12)) == null ? undefined : f,
allowedChannelsList: (f = jspb.Message.getRepeatedField(msg, 13)) == null ? undefined : f,
requireChecksum: jspb.Message.getBooleanFieldWithDefault(msg, 14, false),
trustPolicy: jspb.Message.getFieldWithDefault(msg, 15, ""),
lastSyncUnix: jspb.Message.getFieldWithDefault(msg, 16, 0),
lastSyncStatus: jspb.Message.getFieldWithDefault(msg, 17, ""),
lastSyncError: jspb.Message.getFieldWithDefault(msg, 18, ""),
repoUrl: jspb.Message.getFieldWithDefault(msg, 20, ""),
includePrereleases: jspb.Message.getBooleanFieldWithDefault(msg, 21, false),
owner: jspb.Message.getFieldWithDefault(msg, 30, ""),
repo: jspb.Message.getFieldWithDefault(msg, 31, ""),
branch: jspb.Message.getFieldWithDefault(msg, 32, ""),
indexPathTemplate: jspb.Message.getFieldWithDefault(msg, 33, ""),
artifactBaseUrl: jspb.Message.getFieldWithDefault(msg, 34, ""),
localRoot: jspb.Message.getFieldWithDefault(msg, 35, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.UpstreamSource}
 */
proto.repository.UpstreamSource.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.UpstreamSource;
  return proto.repository.UpstreamSource.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.UpstreamSource} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.UpstreamSource}
 */
proto.repository.UpstreamSource.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {!proto.repository.UpstreamSourceType} */ (reader.readEnum());
      msg.setType(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexUrl(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setChannel(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlatform(value);
      break;
    case 6:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setEnabled(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setLastSyncedTag(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setCredentialsRef(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.setDefaultPublisherId(value);
      break;
    case 11:
      var value = /** @type {string} */ (reader.readString());
      msg.addAllowedPublishers(value);
      break;
    case 12:
      var value = /** @type {string} */ (reader.readString());
      msg.addAllowedKinds(value);
      break;
    case 13:
      var value = /** @type {string} */ (reader.readString());
      msg.addAllowedChannels(value);
      break;
    case 14:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setRequireChecksum(value);
      break;
    case 15:
      var value = /** @type {string} */ (reader.readString());
      msg.setTrustPolicy(value);
      break;
    case 16:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setLastSyncUnix(value);
      break;
    case 17:
      var value = /** @type {string} */ (reader.readString());
      msg.setLastSyncStatus(value);
      break;
    case 18:
      var value = /** @type {string} */ (reader.readString());
      msg.setLastSyncError(value);
      break;
    case 20:
      var value = /** @type {string} */ (reader.readString());
      msg.setRepoUrl(value);
      break;
    case 21:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIncludePrereleases(value);
      break;
    case 30:
      var value = /** @type {string} */ (reader.readString());
      msg.setOwner(value);
      break;
    case 31:
      var value = /** @type {string} */ (reader.readString());
      msg.setRepo(value);
      break;
    case 32:
      var value = /** @type {string} */ (reader.readString());
      msg.setBranch(value);
      break;
    case 33:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexPathTemplate(value);
      break;
    case 34:
      var value = /** @type {string} */ (reader.readString());
      msg.setArtifactBaseUrl(value);
      break;
    case 35:
      var value = /** @type {string} */ (reader.readString());
      msg.setLocalRoot(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.UpstreamSource.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.UpstreamSource.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.UpstreamSource} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.UpstreamSource.serializeBinaryToWriter = function(message, writer) {
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
  f = message.getIndexUrl();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getChannel();
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
  f = message.getEnabled();
  if (f) {
    writer.writeBool(
      6,
      f
    );
  }
  f = message.getLastSyncedTag();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getCredentialsRef();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
  f = message.getDefaultPublisherId();
  if (f.length > 0) {
    writer.writeString(
      10,
      f
    );
  }
  f = message.getAllowedPublishersList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      11,
      f
    );
  }
  f = message.getAllowedKindsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      12,
      f
    );
  }
  f = message.getAllowedChannelsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      13,
      f
    );
  }
  f = message.getRequireChecksum();
  if (f) {
    writer.writeBool(
      14,
      f
    );
  }
  f = message.getTrustPolicy();
  if (f.length > 0) {
    writer.writeString(
      15,
      f
    );
  }
  f = message.getLastSyncUnix();
  if (f !== 0) {
    writer.writeInt64(
      16,
      f
    );
  }
  f = message.getLastSyncStatus();
  if (f.length > 0) {
    writer.writeString(
      17,
      f
    );
  }
  f = message.getLastSyncError();
  if (f.length > 0) {
    writer.writeString(
      18,
      f
    );
  }
  f = message.getRepoUrl();
  if (f.length > 0) {
    writer.writeString(
      20,
      f
    );
  }
  f = message.getIncludePrereleases();
  if (f) {
    writer.writeBool(
      21,
      f
    );
  }
  f = message.getOwner();
  if (f.length > 0) {
    writer.writeString(
      30,
      f
    );
  }
  f = message.getRepo();
  if (f.length > 0) {
    writer.writeString(
      31,
      f
    );
  }
  f = message.getBranch();
  if (f.length > 0) {
    writer.writeString(
      32,
      f
    );
  }
  f = message.getIndexPathTemplate();
  if (f.length > 0) {
    writer.writeString(
      33,
      f
    );
  }
  f = message.getArtifactBaseUrl();
  if (f.length > 0) {
    writer.writeString(
      34,
      f
    );
  }
  f = message.getLocalRoot();
  if (f.length > 0) {
    writer.writeString(
      35,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.repository.UpstreamSource.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional UpstreamSourceType type = 2;
 * @return {!proto.repository.UpstreamSourceType}
 */
proto.repository.UpstreamSource.prototype.getType = function() {
  return /** @type {!proto.repository.UpstreamSourceType} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.repository.UpstreamSourceType} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setType = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional string index_url = 3;
 * @return {string}
 */
proto.repository.UpstreamSource.prototype.getIndexUrl = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setIndexUrl = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string channel = 4;
 * @return {string}
 */
proto.repository.UpstreamSource.prototype.getChannel = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setChannel = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string platform = 5;
 * @return {string}
 */
proto.repository.UpstreamSource.prototype.getPlatform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setPlatform = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional bool enabled = 6;
 * @return {boolean}
 */
proto.repository.UpstreamSource.prototype.getEnabled = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 6, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setEnabled = function(value) {
  return jspb.Message.setProto3BooleanField(this, 6, value);
};


/**
 * optional string last_synced_tag = 7;
 * @return {string}
 */
proto.repository.UpstreamSource.prototype.getLastSyncedTag = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setLastSyncedTag = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional string credentials_ref = 8;
 * @return {string}
 */
proto.repository.UpstreamSource.prototype.getCredentialsRef = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setCredentialsRef = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};


/**
 * optional string default_publisher_id = 10;
 * @return {string}
 */
proto.repository.UpstreamSource.prototype.getDefaultPublisherId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 10, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setDefaultPublisherId = function(value) {
  return jspb.Message.setProto3StringField(this, 10, value);
};


/**
 * repeated string allowed_publishers = 11;
 * @return {!Array<string>}
 */
proto.repository.UpstreamSource.prototype.getAllowedPublishersList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 11));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setAllowedPublishersList = function(value) {
  return jspb.Message.setField(this, 11, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.addAllowedPublishers = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 11, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.clearAllowedPublishersList = function() {
  return this.setAllowedPublishersList([]);
};


/**
 * repeated string allowed_kinds = 12;
 * @return {!Array<string>}
 */
proto.repository.UpstreamSource.prototype.getAllowedKindsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 12));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setAllowedKindsList = function(value) {
  return jspb.Message.setField(this, 12, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.addAllowedKinds = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 12, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.clearAllowedKindsList = function() {
  return this.setAllowedKindsList([]);
};


/**
 * repeated string allowed_channels = 13;
 * @return {!Array<string>}
 */
proto.repository.UpstreamSource.prototype.getAllowedChannelsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 13));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setAllowedChannelsList = function(value) {
  return jspb.Message.setField(this, 13, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.addAllowedChannels = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 13, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.clearAllowedChannelsList = function() {
  return this.setAllowedChannelsList([]);
};


/**
 * optional bool require_checksum = 14;
 * @return {boolean}
 */
proto.repository.UpstreamSource.prototype.getRequireChecksum = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 14, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setRequireChecksum = function(value) {
  return jspb.Message.setProto3BooleanField(this, 14, value);
};


/**
 * optional string trust_policy = 15;
 * @return {string}
 */
proto.repository.UpstreamSource.prototype.getTrustPolicy = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 15, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setTrustPolicy = function(value) {
  return jspb.Message.setProto3StringField(this, 15, value);
};


/**
 * optional int64 last_sync_unix = 16;
 * @return {number}
 */
proto.repository.UpstreamSource.prototype.getLastSyncUnix = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 16, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setLastSyncUnix = function(value) {
  return jspb.Message.setProto3IntField(this, 16, value);
};


/**
 * optional string last_sync_status = 17;
 * @return {string}
 */
proto.repository.UpstreamSource.prototype.getLastSyncStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 17, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setLastSyncStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 17, value);
};


/**
 * optional string last_sync_error = 18;
 * @return {string}
 */
proto.repository.UpstreamSource.prototype.getLastSyncError = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 18, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setLastSyncError = function(value) {
  return jspb.Message.setProto3StringField(this, 18, value);
};


/**
 * optional string repo_url = 20;
 * @return {string}
 */
proto.repository.UpstreamSource.prototype.getRepoUrl = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 20, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setRepoUrl = function(value) {
  return jspb.Message.setProto3StringField(this, 20, value);
};


/**
 * optional bool include_prereleases = 21;
 * @return {boolean}
 */
proto.repository.UpstreamSource.prototype.getIncludePrereleases = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 21, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setIncludePrereleases = function(value) {
  return jspb.Message.setProto3BooleanField(this, 21, value);
};


/**
 * optional string owner = 30;
 * @return {string}
 */
proto.repository.UpstreamSource.prototype.getOwner = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 30, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setOwner = function(value) {
  return jspb.Message.setProto3StringField(this, 30, value);
};


/**
 * optional string repo = 31;
 * @return {string}
 */
proto.repository.UpstreamSource.prototype.getRepo = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 31, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setRepo = function(value) {
  return jspb.Message.setProto3StringField(this, 31, value);
};


/**
 * optional string branch = 32;
 * @return {string}
 */
proto.repository.UpstreamSource.prototype.getBranch = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 32, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setBranch = function(value) {
  return jspb.Message.setProto3StringField(this, 32, value);
};


/**
 * optional string index_path_template = 33;
 * @return {string}
 */
proto.repository.UpstreamSource.prototype.getIndexPathTemplate = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 33, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setIndexPathTemplate = function(value) {
  return jspb.Message.setProto3StringField(this, 33, value);
};


/**
 * optional string artifact_base_url = 34;
 * @return {string}
 */
proto.repository.UpstreamSource.prototype.getArtifactBaseUrl = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 34, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setArtifactBaseUrl = function(value) {
  return jspb.Message.setProto3StringField(this, 34, value);
};


/**
 * optional string local_root = 35;
 * @return {string}
 */
proto.repository.UpstreamSource.prototype.getLocalRoot = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 35, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSource} returns this
 */
proto.repository.UpstreamSource.prototype.setLocalRoot = function(value) {
  return jspb.Message.setProto3StringField(this, 35, value);
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
proto.repository.RegisterUpstreamRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.RegisterUpstreamRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.RegisterUpstreamRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RegisterUpstreamRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
source: (f = msg.getSource()) && proto.repository.UpstreamSource.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.RegisterUpstreamRequest}
 */
proto.repository.RegisterUpstreamRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.RegisterUpstreamRequest;
  return proto.repository.RegisterUpstreamRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.RegisterUpstreamRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.RegisterUpstreamRequest}
 */
proto.repository.RegisterUpstreamRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.UpstreamSource;
      reader.readMessage(value,proto.repository.UpstreamSource.deserializeBinaryFromReader);
      msg.setSource(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.RegisterUpstreamRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.RegisterUpstreamRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.RegisterUpstreamRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RegisterUpstreamRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSource();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.UpstreamSource.serializeBinaryToWriter
    );
  }
};


/**
 * optional UpstreamSource source = 1;
 * @return {?proto.repository.UpstreamSource}
 */
proto.repository.RegisterUpstreamRequest.prototype.getSource = function() {
  return /** @type{?proto.repository.UpstreamSource} */ (
    jspb.Message.getWrapperField(this, proto.repository.UpstreamSource, 1));
};


/**
 * @param {?proto.repository.UpstreamSource|undefined} value
 * @return {!proto.repository.RegisterUpstreamRequest} returns this
*/
proto.repository.RegisterUpstreamRequest.prototype.setSource = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.RegisterUpstreamRequest} returns this
 */
proto.repository.RegisterUpstreamRequest.prototype.clearSource = function() {
  return this.setSource(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.RegisterUpstreamRequest.prototype.hasSource = function() {
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
proto.repository.RegisterUpstreamResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.RegisterUpstreamResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.RegisterUpstreamResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RegisterUpstreamResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
source: (f = msg.getSource()) && proto.repository.UpstreamSource.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.RegisterUpstreamResponse}
 */
proto.repository.RegisterUpstreamResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.RegisterUpstreamResponse;
  return proto.repository.RegisterUpstreamResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.RegisterUpstreamResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.RegisterUpstreamResponse}
 */
proto.repository.RegisterUpstreamResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.UpstreamSource;
      reader.readMessage(value,proto.repository.UpstreamSource.deserializeBinaryFromReader);
      msg.setSource(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.RegisterUpstreamResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.RegisterUpstreamResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.RegisterUpstreamResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RegisterUpstreamResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSource();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.UpstreamSource.serializeBinaryToWriter
    );
  }
};


/**
 * optional UpstreamSource source = 1;
 * @return {?proto.repository.UpstreamSource}
 */
proto.repository.RegisterUpstreamResponse.prototype.getSource = function() {
  return /** @type{?proto.repository.UpstreamSource} */ (
    jspb.Message.getWrapperField(this, proto.repository.UpstreamSource, 1));
};


/**
 * @param {?proto.repository.UpstreamSource|undefined} value
 * @return {!proto.repository.RegisterUpstreamResponse} returns this
*/
proto.repository.RegisterUpstreamResponse.prototype.setSource = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.RegisterUpstreamResponse} returns this
 */
proto.repository.RegisterUpstreamResponse.prototype.clearSource = function() {
  return this.setSource(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.RegisterUpstreamResponse.prototype.hasSource = function() {
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
proto.repository.ListUpstreamsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ListUpstreamsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ListUpstreamsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListUpstreamsRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.repository.ListUpstreamsRequest}
 */
proto.repository.ListUpstreamsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ListUpstreamsRequest;
  return proto.repository.ListUpstreamsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ListUpstreamsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ListUpstreamsRequest}
 */
proto.repository.ListUpstreamsRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.repository.ListUpstreamsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ListUpstreamsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ListUpstreamsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListUpstreamsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.ListUpstreamsResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.ListUpstreamsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ListUpstreamsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ListUpstreamsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListUpstreamsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
sourcesList: jspb.Message.toObjectList(msg.getSourcesList(),
    proto.repository.UpstreamSource.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ListUpstreamsResponse}
 */
proto.repository.ListUpstreamsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ListUpstreamsResponse;
  return proto.repository.ListUpstreamsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ListUpstreamsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ListUpstreamsResponse}
 */
proto.repository.ListUpstreamsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.UpstreamSource;
      reader.readMessage(value,proto.repository.UpstreamSource.deserializeBinaryFromReader);
      msg.addSources(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.ListUpstreamsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ListUpstreamsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ListUpstreamsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListUpstreamsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSourcesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.repository.UpstreamSource.serializeBinaryToWriter
    );
  }
};


/**
 * repeated UpstreamSource sources = 1;
 * @return {!Array<!proto.repository.UpstreamSource>}
 */
proto.repository.ListUpstreamsResponse.prototype.getSourcesList = function() {
  return /** @type{!Array<!proto.repository.UpstreamSource>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.repository.UpstreamSource, 1));
};


/**
 * @param {!Array<!proto.repository.UpstreamSource>} value
 * @return {!proto.repository.ListUpstreamsResponse} returns this
*/
proto.repository.ListUpstreamsResponse.prototype.setSourcesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.repository.UpstreamSource=} opt_value
 * @param {number=} opt_index
 * @return {!proto.repository.UpstreamSource}
 */
proto.repository.ListUpstreamsResponse.prototype.addSources = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.repository.UpstreamSource, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ListUpstreamsResponse} returns this
 */
proto.repository.ListUpstreamsResponse.prototype.clearSourcesList = function() {
  return this.setSourcesList([]);
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
proto.repository.RemoveUpstreamRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.RemoveUpstreamRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.RemoveUpstreamRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RemoveUpstreamRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.repository.RemoveUpstreamRequest}
 */
proto.repository.RemoveUpstreamRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.RemoveUpstreamRequest;
  return proto.repository.RemoveUpstreamRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.RemoveUpstreamRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.RemoveUpstreamRequest}
 */
proto.repository.RemoveUpstreamRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.repository.RemoveUpstreamRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.RemoveUpstreamRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.RemoveUpstreamRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RemoveUpstreamRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.repository.RemoveUpstreamRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RemoveUpstreamRequest} returns this
 */
proto.repository.RemoveUpstreamRequest.prototype.setName = function(value) {
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
proto.repository.RemoveUpstreamResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.RemoveUpstreamResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.RemoveUpstreamResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RemoveUpstreamResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.repository.RemoveUpstreamResponse}
 */
proto.repository.RemoveUpstreamResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.RemoveUpstreamResponse;
  return proto.repository.RemoveUpstreamResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.RemoveUpstreamResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.RemoveUpstreamResponse}
 */
proto.repository.RemoveUpstreamResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.repository.RemoveUpstreamResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.RemoveUpstreamResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.RemoveUpstreamResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RemoveUpstreamResponse.serializeBinaryToWriter = function(message, writer) {
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
proto.repository.UpstreamSyncResult.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.UpstreamSyncResult.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.UpstreamSyncResult} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.UpstreamSyncResult.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, ""),
version: jspb.Message.getFieldWithDefault(msg, 2, ""),
buildId: jspb.Message.getFieldWithDefault(msg, 3, ""),
platform: jspb.Message.getFieldWithDefault(msg, 4, ""),
packageDigest: jspb.Message.getFieldWithDefault(msg, 5, ""),
status: jspb.Message.getFieldWithDefault(msg, 6, 0),
detail: jspb.Message.getFieldWithDefault(msg, 7, ""),
publisher: jspb.Message.getFieldWithDefault(msg, 10, ""),
kind: jspb.Message.getFieldWithDefault(msg, 11, ""),
channel: jspb.Message.getFieldWithDefault(msg, 12, ""),
buildNumber: jspb.Message.getFieldWithDefault(msg, 13, 0),
checksumPresent: jspb.Message.getBooleanFieldWithDefault(msg, 14, false),
localVersion: jspb.Message.getFieldWithDefault(msg, 15, ""),
localBuildNumber: jspb.Message.getFieldWithDefault(msg, 16, 0),
action: jspb.Message.getFieldWithDefault(msg, 17, ""),
blockedReason: jspb.Message.getFieldWithDefault(msg, 18, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.UpstreamSyncResult}
 */
proto.repository.UpstreamSyncResult.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.UpstreamSyncResult;
  return proto.repository.UpstreamSyncResult.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.UpstreamSyncResult} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.UpstreamSyncResult}
 */
proto.repository.UpstreamSyncResult.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {string} */ (reader.readString());
      msg.setBuildId(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlatform(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setPackageDigest(value);
      break;
    case 6:
      var value = /** @type {!proto.repository.UpstreamSyncStatus} */ (reader.readEnum());
      msg.setStatus(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setDetail(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisher(value);
      break;
    case 11:
      var value = /** @type {string} */ (reader.readString());
      msg.setKind(value);
      break;
    case 12:
      var value = /** @type {string} */ (reader.readString());
      msg.setChannel(value);
      break;
    case 13:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    case 14:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setChecksumPresent(value);
      break;
    case 15:
      var value = /** @type {string} */ (reader.readString());
      msg.setLocalVersion(value);
      break;
    case 16:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setLocalBuildNumber(value);
      break;
    case 17:
      var value = /** @type {string} */ (reader.readString());
      msg.setAction(value);
      break;
    case 18:
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
proto.repository.UpstreamSyncResult.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.UpstreamSyncResult.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.UpstreamSyncResult} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.UpstreamSyncResult.serializeBinaryToWriter = function(message, writer) {
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
  f = message.getBuildId();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getPlatform();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getPackageDigest();
  if (f.length > 0) {
    writer.writeString(
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
  f = message.getDetail();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getPublisher();
  if (f.length > 0) {
    writer.writeString(
      10,
      f
    );
  }
  f = message.getKind();
  if (f.length > 0) {
    writer.writeString(
      11,
      f
    );
  }
  f = message.getChannel();
  if (f.length > 0) {
    writer.writeString(
      12,
      f
    );
  }
  f = message.getBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
      13,
      f
    );
  }
  f = message.getChecksumPresent();
  if (f) {
    writer.writeBool(
      14,
      f
    );
  }
  f = message.getLocalVersion();
  if (f.length > 0) {
    writer.writeString(
      15,
      f
    );
  }
  f = message.getLocalBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
      16,
      f
    );
  }
  f = message.getAction();
  if (f.length > 0) {
    writer.writeString(
      17,
      f
    );
  }
  f = message.getBlockedReason();
  if (f.length > 0) {
    writer.writeString(
      18,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.repository.UpstreamSyncResult.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSyncResult} returns this
 */
proto.repository.UpstreamSyncResult.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string version = 2;
 * @return {string}
 */
proto.repository.UpstreamSyncResult.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSyncResult} returns this
 */
proto.repository.UpstreamSyncResult.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string build_id = 3;
 * @return {string}
 */
proto.repository.UpstreamSyncResult.prototype.getBuildId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSyncResult} returns this
 */
proto.repository.UpstreamSyncResult.prototype.setBuildId = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string platform = 4;
 * @return {string}
 */
proto.repository.UpstreamSyncResult.prototype.getPlatform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSyncResult} returns this
 */
proto.repository.UpstreamSyncResult.prototype.setPlatform = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string package_digest = 5;
 * @return {string}
 */
proto.repository.UpstreamSyncResult.prototype.getPackageDigest = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSyncResult} returns this
 */
proto.repository.UpstreamSyncResult.prototype.setPackageDigest = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional UpstreamSyncStatus status = 6;
 * @return {!proto.repository.UpstreamSyncStatus}
 */
proto.repository.UpstreamSyncResult.prototype.getStatus = function() {
  return /** @type {!proto.repository.UpstreamSyncStatus} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {!proto.repository.UpstreamSyncStatus} value
 * @return {!proto.repository.UpstreamSyncResult} returns this
 */
proto.repository.UpstreamSyncResult.prototype.setStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 6, value);
};


/**
 * optional string detail = 7;
 * @return {string}
 */
proto.repository.UpstreamSyncResult.prototype.getDetail = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSyncResult} returns this
 */
proto.repository.UpstreamSyncResult.prototype.setDetail = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional string publisher = 10;
 * @return {string}
 */
proto.repository.UpstreamSyncResult.prototype.getPublisher = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 10, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSyncResult} returns this
 */
proto.repository.UpstreamSyncResult.prototype.setPublisher = function(value) {
  return jspb.Message.setProto3StringField(this, 10, value);
};


/**
 * optional string kind = 11;
 * @return {string}
 */
proto.repository.UpstreamSyncResult.prototype.getKind = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 11, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSyncResult} returns this
 */
proto.repository.UpstreamSyncResult.prototype.setKind = function(value) {
  return jspb.Message.setProto3StringField(this, 11, value);
};


/**
 * optional string channel = 12;
 * @return {string}
 */
proto.repository.UpstreamSyncResult.prototype.getChannel = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 12, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSyncResult} returns this
 */
proto.repository.UpstreamSyncResult.prototype.setChannel = function(value) {
  return jspb.Message.setProto3StringField(this, 12, value);
};


/**
 * optional int64 build_number = 13;
 * @return {number}
 */
proto.repository.UpstreamSyncResult.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 13, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.UpstreamSyncResult} returns this
 */
proto.repository.UpstreamSyncResult.prototype.setBuildNumber = function(value) {
  return jspb.Message.setProto3IntField(this, 13, value);
};


/**
 * optional bool checksum_present = 14;
 * @return {boolean}
 */
proto.repository.UpstreamSyncResult.prototype.getChecksumPresent = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 14, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.UpstreamSyncResult} returns this
 */
proto.repository.UpstreamSyncResult.prototype.setChecksumPresent = function(value) {
  return jspb.Message.setProto3BooleanField(this, 14, value);
};


/**
 * optional string local_version = 15;
 * @return {string}
 */
proto.repository.UpstreamSyncResult.prototype.getLocalVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 15, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSyncResult} returns this
 */
proto.repository.UpstreamSyncResult.prototype.setLocalVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 15, value);
};


/**
 * optional int64 local_build_number = 16;
 * @return {number}
 */
proto.repository.UpstreamSyncResult.prototype.getLocalBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 16, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.UpstreamSyncResult} returns this
 */
proto.repository.UpstreamSyncResult.prototype.setLocalBuildNumber = function(value) {
  return jspb.Message.setProto3IntField(this, 16, value);
};


/**
 * optional string action = 17;
 * @return {string}
 */
proto.repository.UpstreamSyncResult.prototype.getAction = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 17, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSyncResult} returns this
 */
proto.repository.UpstreamSyncResult.prototype.setAction = function(value) {
  return jspb.Message.setProto3StringField(this, 17, value);
};


/**
 * optional string blocked_reason = 18;
 * @return {string}
 */
proto.repository.UpstreamSyncResult.prototype.getBlockedReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 18, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamSyncResult} returns this
 */
proto.repository.UpstreamSyncResult.prototype.setBlockedReason = function(value) {
  return jspb.Message.setProto3StringField(this, 18, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.SyncFromUpstreamRequest.repeatedFields_ = [4];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.SyncFromUpstreamRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.SyncFromUpstreamRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.SyncFromUpstreamRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.SyncFromUpstreamRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
sourceName: jspb.Message.getFieldWithDefault(msg, 1, ""),
releaseTag: jspb.Message.getFieldWithDefault(msg, 2, ""),
dryRun: jspb.Message.getBooleanFieldWithDefault(msg, 3, false),
onlyList: (f = jspb.Message.getRepeatedField(msg, 4)) == null ? undefined : f,
resolveLatest: jspb.Message.getBooleanFieldWithDefault(msg, 5, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.SyncFromUpstreamRequest}
 */
proto.repository.SyncFromUpstreamRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.SyncFromUpstreamRequest;
  return proto.repository.SyncFromUpstreamRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.SyncFromUpstreamRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.SyncFromUpstreamRequest}
 */
proto.repository.SyncFromUpstreamRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setSourceName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setReleaseTag(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setDryRun(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.addOnly(value);
      break;
    case 5:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResolveLatest(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.SyncFromUpstreamRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.SyncFromUpstreamRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.SyncFromUpstreamRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.SyncFromUpstreamRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSourceName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getReleaseTag();
  if (f.length > 0) {
    writer.writeString(
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
  f = message.getOnlyList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      4,
      f
    );
  }
  f = message.getResolveLatest();
  if (f) {
    writer.writeBool(
      5,
      f
    );
  }
};


/**
 * optional string source_name = 1;
 * @return {string}
 */
proto.repository.SyncFromUpstreamRequest.prototype.getSourceName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.SyncFromUpstreamRequest} returns this
 */
proto.repository.SyncFromUpstreamRequest.prototype.setSourceName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string release_tag = 2;
 * @return {string}
 */
proto.repository.SyncFromUpstreamRequest.prototype.getReleaseTag = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.SyncFromUpstreamRequest} returns this
 */
proto.repository.SyncFromUpstreamRequest.prototype.setReleaseTag = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional bool dry_run = 3;
 * @return {boolean}
 */
proto.repository.SyncFromUpstreamRequest.prototype.getDryRun = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.SyncFromUpstreamRequest} returns this
 */
proto.repository.SyncFromUpstreamRequest.prototype.setDryRun = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * repeated string only = 4;
 * @return {!Array<string>}
 */
proto.repository.SyncFromUpstreamRequest.prototype.getOnlyList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 4));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.SyncFromUpstreamRequest} returns this
 */
proto.repository.SyncFromUpstreamRequest.prototype.setOnlyList = function(value) {
  return jspb.Message.setField(this, 4, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.SyncFromUpstreamRequest} returns this
 */
proto.repository.SyncFromUpstreamRequest.prototype.addOnly = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 4, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.SyncFromUpstreamRequest} returns this
 */
proto.repository.SyncFromUpstreamRequest.prototype.clearOnlyList = function() {
  return this.setOnlyList([]);
};


/**
 * optional bool resolve_latest = 5;
 * @return {boolean}
 */
proto.repository.SyncFromUpstreamRequest.prototype.getResolveLatest = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 5, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.SyncFromUpstreamRequest} returns this
 */
proto.repository.SyncFromUpstreamRequest.prototype.setResolveLatest = function(value) {
  return jspb.Message.setProto3BooleanField(this, 5, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.SyncFromUpstreamResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.SyncFromUpstreamResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.SyncFromUpstreamResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.SyncFromUpstreamResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.SyncFromUpstreamResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
resultsList: jspb.Message.toObjectList(msg.getResultsList(),
    proto.repository.UpstreamSyncResult.toObject, includeInstance),
imported: jspb.Message.getFieldWithDefault(msg, 2, 0),
skipped: jspb.Message.getFieldWithDefault(msg, 3, 0),
rejected: jspb.Message.getFieldWithDefault(msg, 4, 0),
failed: jspb.Message.getFieldWithDefault(msg, 5, 0),
dryRun: jspb.Message.getBooleanFieldWithDefault(msg, 6, false),
resolvedTag: jspb.Message.getFieldWithDefault(msg, 7, ""),
sourceName: jspb.Message.getFieldWithDefault(msg, 8, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.SyncFromUpstreamResponse}
 */
proto.repository.SyncFromUpstreamResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.SyncFromUpstreamResponse;
  return proto.repository.SyncFromUpstreamResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.SyncFromUpstreamResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.SyncFromUpstreamResponse}
 */
proto.repository.SyncFromUpstreamResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.UpstreamSyncResult;
      reader.readMessage(value,proto.repository.UpstreamSyncResult.deserializeBinaryFromReader);
      msg.addResults(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setImported(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setSkipped(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setRejected(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setFailed(value);
      break;
    case 6:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setDryRun(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setResolvedTag(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setSourceName(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.SyncFromUpstreamResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.SyncFromUpstreamResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.SyncFromUpstreamResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.SyncFromUpstreamResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResultsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.repository.UpstreamSyncResult.serializeBinaryToWriter
    );
  }
  f = message.getImported();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getSkipped();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
  f = message.getRejected();
  if (f !== 0) {
    writer.writeInt32(
      4,
      f
    );
  }
  f = message.getFailed();
  if (f !== 0) {
    writer.writeInt32(
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
  f = message.getResolvedTag();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getSourceName();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
};


/**
 * repeated UpstreamSyncResult results = 1;
 * @return {!Array<!proto.repository.UpstreamSyncResult>}
 */
proto.repository.SyncFromUpstreamResponse.prototype.getResultsList = function() {
  return /** @type{!Array<!proto.repository.UpstreamSyncResult>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.repository.UpstreamSyncResult, 1));
};


/**
 * @param {!Array<!proto.repository.UpstreamSyncResult>} value
 * @return {!proto.repository.SyncFromUpstreamResponse} returns this
*/
proto.repository.SyncFromUpstreamResponse.prototype.setResultsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.repository.UpstreamSyncResult=} opt_value
 * @param {number=} opt_index
 * @return {!proto.repository.UpstreamSyncResult}
 */
proto.repository.SyncFromUpstreamResponse.prototype.addResults = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.repository.UpstreamSyncResult, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.SyncFromUpstreamResponse} returns this
 */
proto.repository.SyncFromUpstreamResponse.prototype.clearResultsList = function() {
  return this.setResultsList([]);
};


/**
 * optional int32 imported = 2;
 * @return {number}
 */
proto.repository.SyncFromUpstreamResponse.prototype.getImported = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.SyncFromUpstreamResponse} returns this
 */
proto.repository.SyncFromUpstreamResponse.prototype.setImported = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional int32 skipped = 3;
 * @return {number}
 */
proto.repository.SyncFromUpstreamResponse.prototype.getSkipped = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.SyncFromUpstreamResponse} returns this
 */
proto.repository.SyncFromUpstreamResponse.prototype.setSkipped = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional int32 rejected = 4;
 * @return {number}
 */
proto.repository.SyncFromUpstreamResponse.prototype.getRejected = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.SyncFromUpstreamResponse} returns this
 */
proto.repository.SyncFromUpstreamResponse.prototype.setRejected = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional int32 failed = 5;
 * @return {number}
 */
proto.repository.SyncFromUpstreamResponse.prototype.getFailed = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.SyncFromUpstreamResponse} returns this
 */
proto.repository.SyncFromUpstreamResponse.prototype.setFailed = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional bool dry_run = 6;
 * @return {boolean}
 */
proto.repository.SyncFromUpstreamResponse.prototype.getDryRun = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 6, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.SyncFromUpstreamResponse} returns this
 */
proto.repository.SyncFromUpstreamResponse.prototype.setDryRun = function(value) {
  return jspb.Message.setProto3BooleanField(this, 6, value);
};


/**
 * optional string resolved_tag = 7;
 * @return {string}
 */
proto.repository.SyncFromUpstreamResponse.prototype.getResolvedTag = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.SyncFromUpstreamResponse} returns this
 */
proto.repository.SyncFromUpstreamResponse.prototype.setResolvedTag = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional string source_name = 8;
 * @return {string}
 */
proto.repository.SyncFromUpstreamResponse.prototype.getSourceName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.SyncFromUpstreamResponse} returns this
 */
proto.repository.SyncFromUpstreamResponse.prototype.setSourceName = function(value) {
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
proto.repository.UpstreamImportRecord.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.UpstreamImportRecord.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.UpstreamImportRecord} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.UpstreamImportRecord.toObject = function(includeInstance, msg) {
  var f, obj = {
sourceName: jspb.Message.getFieldWithDefault(msg, 1, ""),
releaseTag: jspb.Message.getFieldWithDefault(msg, 2, ""),
assetUrl: jspb.Message.getFieldWithDefault(msg, 3, ""),
indexUrl: jspb.Message.getFieldWithDefault(msg, 4, ""),
importedAt: jspb.Message.getFieldWithDefault(msg, 5, 0),
publisher: jspb.Message.getFieldWithDefault(msg, 6, ""),
kind: jspb.Message.getFieldWithDefault(msg, 7, ""),
channel: jspb.Message.getFieldWithDefault(msg, 8, ""),
buildNumber: jspb.Message.getFieldWithDefault(msg, 9, 0),
checksum: jspb.Message.getFieldWithDefault(msg, 10, ""),
originRelease: jspb.Message.getFieldWithDefault(msg, 11, ""),
changedInRelease: jspb.Message.getBooleanFieldWithDefault(msg, 12, false),
platformRelease: jspb.Message.getFieldWithDefault(msg, 13, ""),
packageContractDigest: jspb.Message.getFieldWithDefault(msg, 14, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.UpstreamImportRecord}
 */
proto.repository.UpstreamImportRecord.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.UpstreamImportRecord;
  return proto.repository.UpstreamImportRecord.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.UpstreamImportRecord} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.UpstreamImportRecord}
 */
proto.repository.UpstreamImportRecord.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setSourceName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setReleaseTag(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setAssetUrl(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexUrl(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setImportedAt(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisher(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setKind(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setChannel(value);
      break;
    case 9:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.setChecksum(value);
      break;
    case 11:
      var value = /** @type {string} */ (reader.readString());
      msg.setOriginRelease(value);
      break;
    case 12:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setChangedInRelease(value);
      break;
    case 13:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlatformRelease(value);
      break;
    case 14:
      var value = /** @type {string} */ (reader.readString());
      msg.setPackageContractDigest(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.UpstreamImportRecord.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.UpstreamImportRecord.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.UpstreamImportRecord} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.UpstreamImportRecord.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSourceName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getReleaseTag();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getAssetUrl();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getIndexUrl();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getImportedAt();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
  f = message.getPublisher();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getKind();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getChannel();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
  f = message.getBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
      9,
      f
    );
  }
  f = message.getChecksum();
  if (f.length > 0) {
    writer.writeString(
      10,
      f
    );
  }
  f = message.getOriginRelease();
  if (f.length > 0) {
    writer.writeString(
      11,
      f
    );
  }
  f = message.getChangedInRelease();
  if (f) {
    writer.writeBool(
      12,
      f
    );
  }
  f = message.getPlatformRelease();
  if (f.length > 0) {
    writer.writeString(
      13,
      f
    );
  }
  f = message.getPackageContractDigest();
  if (f.length > 0) {
    writer.writeString(
      14,
      f
    );
  }
};


/**
 * optional string source_name = 1;
 * @return {string}
 */
proto.repository.UpstreamImportRecord.prototype.getSourceName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamImportRecord} returns this
 */
proto.repository.UpstreamImportRecord.prototype.setSourceName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string release_tag = 2;
 * @return {string}
 */
proto.repository.UpstreamImportRecord.prototype.getReleaseTag = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamImportRecord} returns this
 */
proto.repository.UpstreamImportRecord.prototype.setReleaseTag = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string asset_url = 3;
 * @return {string}
 */
proto.repository.UpstreamImportRecord.prototype.getAssetUrl = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamImportRecord} returns this
 */
proto.repository.UpstreamImportRecord.prototype.setAssetUrl = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string index_url = 4;
 * @return {string}
 */
proto.repository.UpstreamImportRecord.prototype.getIndexUrl = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamImportRecord} returns this
 */
proto.repository.UpstreamImportRecord.prototype.setIndexUrl = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional int64 imported_at = 5;
 * @return {number}
 */
proto.repository.UpstreamImportRecord.prototype.getImportedAt = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.UpstreamImportRecord} returns this
 */
proto.repository.UpstreamImportRecord.prototype.setImportedAt = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional string publisher = 6;
 * @return {string}
 */
proto.repository.UpstreamImportRecord.prototype.getPublisher = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamImportRecord} returns this
 */
proto.repository.UpstreamImportRecord.prototype.setPublisher = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string kind = 7;
 * @return {string}
 */
proto.repository.UpstreamImportRecord.prototype.getKind = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamImportRecord} returns this
 */
proto.repository.UpstreamImportRecord.prototype.setKind = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional string channel = 8;
 * @return {string}
 */
proto.repository.UpstreamImportRecord.prototype.getChannel = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamImportRecord} returns this
 */
proto.repository.UpstreamImportRecord.prototype.setChannel = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};


/**
 * optional int64 build_number = 9;
 * @return {number}
 */
proto.repository.UpstreamImportRecord.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 9, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.UpstreamImportRecord} returns this
 */
proto.repository.UpstreamImportRecord.prototype.setBuildNumber = function(value) {
  return jspb.Message.setProto3IntField(this, 9, value);
};


/**
 * optional string checksum = 10;
 * @return {string}
 */
proto.repository.UpstreamImportRecord.prototype.getChecksum = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 10, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamImportRecord} returns this
 */
proto.repository.UpstreamImportRecord.prototype.setChecksum = function(value) {
  return jspb.Message.setProto3StringField(this, 10, value);
};


/**
 * optional string origin_release = 11;
 * @return {string}
 */
proto.repository.UpstreamImportRecord.prototype.getOriginRelease = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 11, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamImportRecord} returns this
 */
proto.repository.UpstreamImportRecord.prototype.setOriginRelease = function(value) {
  return jspb.Message.setProto3StringField(this, 11, value);
};


/**
 * optional bool changed_in_release = 12;
 * @return {boolean}
 */
proto.repository.UpstreamImportRecord.prototype.getChangedInRelease = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 12, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.UpstreamImportRecord} returns this
 */
proto.repository.UpstreamImportRecord.prototype.setChangedInRelease = function(value) {
  return jspb.Message.setProto3BooleanField(this, 12, value);
};


/**
 * optional string platform_release = 13;
 * @return {string}
 */
proto.repository.UpstreamImportRecord.prototype.getPlatformRelease = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 13, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamImportRecord} returns this
 */
proto.repository.UpstreamImportRecord.prototype.setPlatformRelease = function(value) {
  return jspb.Message.setProto3StringField(this, 13, value);
};


/**
 * optional string package_contract_digest = 14;
 * @return {string}
 */
proto.repository.UpstreamImportRecord.prototype.getPackageContractDigest = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 14, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.UpstreamImportRecord} returns this
 */
proto.repository.UpstreamImportRecord.prototype.setPackageContractDigest = function(value) {
  return jspb.Message.setProto3StringField(this, 14, value);
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
proto.repository.ArchiveUnreachableArtifactsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ArchiveUnreachableArtifactsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ArchiveUnreachableArtifactsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ArchiveUnreachableArtifactsRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.repository.ArchiveUnreachableArtifactsRequest}
 */
proto.repository.ArchiveUnreachableArtifactsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ArchiveUnreachableArtifactsRequest;
  return proto.repository.ArchiveUnreachableArtifactsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ArchiveUnreachableArtifactsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ArchiveUnreachableArtifactsRequest}
 */
proto.repository.ArchiveUnreachableArtifactsRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.repository.ArchiveUnreachableArtifactsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ArchiveUnreachableArtifactsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ArchiveUnreachableArtifactsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ArchiveUnreachableArtifactsRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.repository.ArchiveUnreachableArtifactsRequest.prototype.getDryRun = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.ArchiveUnreachableArtifactsRequest} returns this
 */
proto.repository.ArchiveUnreachableArtifactsRequest.prototype.setDryRun = function(value) {
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
proto.repository.ArchivedArtifactRecord.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ArchivedArtifactRecord.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ArchivedArtifactRecord} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ArchivedArtifactRecord.toObject = function(includeInstance, msg) {
  var f, obj = {
key: jspb.Message.getFieldWithDefault(msg, 1, ""),
buildId: jspb.Message.getFieldWithDefault(msg, 2, ""),
name: jspb.Message.getFieldWithDefault(msg, 3, ""),
version: jspb.Message.getFieldWithDefault(msg, 4, ""),
publisher: jspb.Message.getFieldWithDefault(msg, 5, ""),
reason: jspb.Message.getFieldWithDefault(msg, 6, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ArchivedArtifactRecord}
 */
proto.repository.ArchivedArtifactRecord.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ArchivedArtifactRecord;
  return proto.repository.ArchivedArtifactRecord.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ArchivedArtifactRecord} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ArchivedArtifactRecord}
 */
proto.repository.ArchivedArtifactRecord.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setKey(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setBuildId(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setVersion(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisher(value);
      break;
    case 6:
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
proto.repository.ArchivedArtifactRecord.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ArchivedArtifactRecord.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ArchivedArtifactRecord} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ArchivedArtifactRecord.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getKey();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getBuildId();
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
  f = message.getVersion();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getPublisher();
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
};


/**
 * optional string key = 1;
 * @return {string}
 */
proto.repository.ArchivedArtifactRecord.prototype.getKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArchivedArtifactRecord} returns this
 */
proto.repository.ArchivedArtifactRecord.prototype.setKey = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string build_id = 2;
 * @return {string}
 */
proto.repository.ArchivedArtifactRecord.prototype.getBuildId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArchivedArtifactRecord} returns this
 */
proto.repository.ArchivedArtifactRecord.prototype.setBuildId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string name = 3;
 * @return {string}
 */
proto.repository.ArchivedArtifactRecord.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArchivedArtifactRecord} returns this
 */
proto.repository.ArchivedArtifactRecord.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string version = 4;
 * @return {string}
 */
proto.repository.ArchivedArtifactRecord.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArchivedArtifactRecord} returns this
 */
proto.repository.ArchivedArtifactRecord.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string publisher = 5;
 * @return {string}
 */
proto.repository.ArchivedArtifactRecord.prototype.getPublisher = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArchivedArtifactRecord} returns this
 */
proto.repository.ArchivedArtifactRecord.prototype.setPublisher = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string reason = 6;
 * @return {string}
 */
proto.repository.ArchivedArtifactRecord.prototype.getReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArchivedArtifactRecord} returns this
 */
proto.repository.ArchivedArtifactRecord.prototype.setReason = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.ArchiveUnreachableArtifactsResponse.repeatedFields_ = [4];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.ArchiveUnreachableArtifactsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ArchiveUnreachableArtifactsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ArchiveUnreachableArtifactsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ArchiveUnreachableArtifactsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
archivedCount: jspb.Message.getFieldWithDefault(msg, 1, 0),
skippedCount: jspb.Message.getFieldWithDefault(msg, 2, 0),
protectedCount: jspb.Message.getFieldWithDefault(msg, 3, 0),
archivedList: jspb.Message.toObjectList(msg.getArchivedList(),
    proto.repository.ArchivedArtifactRecord.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ArchiveUnreachableArtifactsResponse}
 */
proto.repository.ArchiveUnreachableArtifactsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ArchiveUnreachableArtifactsResponse;
  return proto.repository.ArchiveUnreachableArtifactsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ArchiveUnreachableArtifactsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ArchiveUnreachableArtifactsResponse}
 */
proto.repository.ArchiveUnreachableArtifactsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setArchivedCount(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setSkippedCount(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setProtectedCount(value);
      break;
    case 4:
      var value = new proto.repository.ArchivedArtifactRecord;
      reader.readMessage(value,proto.repository.ArchivedArtifactRecord.deserializeBinaryFromReader);
      msg.addArchived(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.ArchiveUnreachableArtifactsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ArchiveUnreachableArtifactsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ArchiveUnreachableArtifactsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ArchiveUnreachableArtifactsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getArchivedCount();
  if (f !== 0) {
    writer.writeInt32(
      1,
      f
    );
  }
  f = message.getSkippedCount();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getProtectedCount();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
  f = message.getArchivedList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      4,
      f,
      proto.repository.ArchivedArtifactRecord.serializeBinaryToWriter
    );
  }
};


/**
 * optional int32 archived_count = 1;
 * @return {number}
 */
proto.repository.ArchiveUnreachableArtifactsResponse.prototype.getArchivedCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ArchiveUnreachableArtifactsResponse} returns this
 */
proto.repository.ArchiveUnreachableArtifactsResponse.prototype.setArchivedCount = function(value) {
  return jspb.Message.setProto3IntField(this, 1, value);
};


/**
 * optional int32 skipped_count = 2;
 * @return {number}
 */
proto.repository.ArchiveUnreachableArtifactsResponse.prototype.getSkippedCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ArchiveUnreachableArtifactsResponse} returns this
 */
proto.repository.ArchiveUnreachableArtifactsResponse.prototype.setSkippedCount = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional int32 protected_count = 3;
 * @return {number}
 */
proto.repository.ArchiveUnreachableArtifactsResponse.prototype.getProtectedCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ArchiveUnreachableArtifactsResponse} returns this
 */
proto.repository.ArchiveUnreachableArtifactsResponse.prototype.setProtectedCount = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * repeated ArchivedArtifactRecord archived = 4;
 * @return {!Array<!proto.repository.ArchivedArtifactRecord>}
 */
proto.repository.ArchiveUnreachableArtifactsResponse.prototype.getArchivedList = function() {
  return /** @type{!Array<!proto.repository.ArchivedArtifactRecord>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.repository.ArchivedArtifactRecord, 4));
};


/**
 * @param {!Array<!proto.repository.ArchivedArtifactRecord>} value
 * @return {!proto.repository.ArchiveUnreachableArtifactsResponse} returns this
*/
proto.repository.ArchiveUnreachableArtifactsResponse.prototype.setArchivedList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 4, value);
};


/**
 * @param {!proto.repository.ArchivedArtifactRecord=} opt_value
 * @param {number=} opt_index
 * @return {!proto.repository.ArchivedArtifactRecord}
 */
proto.repository.ArchiveUnreachableArtifactsResponse.prototype.addArchived = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 4, opt_value, proto.repository.ArchivedArtifactRecord, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ArchiveUnreachableArtifactsResponse} returns this
 */
proto.repository.ArchiveUnreachableArtifactsResponse.prototype.clearArchivedList = function() {
  return this.setArchivedList([]);
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
proto.repository.PackageConfigFile.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.PackageConfigFile.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.PackageConfigFile} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.PackageConfigFile.toObject = function(includeInstance, msg) {
  var f, obj = {
path: jspb.Message.getFieldWithDefault(msg, 1, ""),
configKind: jspb.Message.getFieldWithDefault(msg, 2, 0),
ownerPackage: jspb.Message.getFieldWithDefault(msg, 3, ""),
checksumAtInstall: jspb.Message.getFieldWithDefault(msg, 4, ""),
currentChecksum: jspb.Message.getFieldWithDefault(msg, 5, ""),
lastModifiedUnix: jspb.Message.getFieldWithDefault(msg, 6, 0),
mergeStrategy: jspb.Message.getFieldWithDefault(msg, 7, 0),
preserveOnUpgrade: jspb.Message.getBooleanFieldWithDefault(msg, 8, false),
restoreOnRollback: jspb.Message.getBooleanFieldWithDefault(msg, 9, false),
sensitive: jspb.Message.getBooleanFieldWithDefault(msg, 10, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.PackageConfigFile}
 */
proto.repository.PackageConfigFile.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.PackageConfigFile;
  return proto.repository.PackageConfigFile.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.PackageConfigFile} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.PackageConfigFile}
 */
proto.repository.PackageConfigFile.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {!proto.repository.ConfigKind} */ (reader.readEnum());
      msg.setConfigKind(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setOwnerPackage(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setChecksumAtInstall(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setCurrentChecksum(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setLastModifiedUnix(value);
      break;
    case 7:
      var value = /** @type {!proto.repository.MergeStrategy} */ (reader.readEnum());
      msg.setMergeStrategy(value);
      break;
    case 8:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setPreserveOnUpgrade(value);
      break;
    case 9:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setRestoreOnRollback(value);
      break;
    case 10:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setSensitive(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.PackageConfigFile.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.PackageConfigFile.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.PackageConfigFile} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.PackageConfigFile.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPath();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getConfigKind();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
  f = message.getOwnerPackage();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getChecksumAtInstall();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getCurrentChecksum();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getLastModifiedUnix();
  if (f !== 0) {
    writer.writeInt64(
      6,
      f
    );
  }
  f = message.getMergeStrategy();
  if (f !== 0.0) {
    writer.writeEnum(
      7,
      f
    );
  }
  f = message.getPreserveOnUpgrade();
  if (f) {
    writer.writeBool(
      8,
      f
    );
  }
  f = message.getRestoreOnRollback();
  if (f) {
    writer.writeBool(
      9,
      f
    );
  }
  f = message.getSensitive();
  if (f) {
    writer.writeBool(
      10,
      f
    );
  }
};


/**
 * optional string path = 1;
 * @return {string}
 */
proto.repository.PackageConfigFile.prototype.getPath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.PackageConfigFile} returns this
 */
proto.repository.PackageConfigFile.prototype.setPath = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional ConfigKind config_kind = 2;
 * @return {!proto.repository.ConfigKind}
 */
proto.repository.PackageConfigFile.prototype.getConfigKind = function() {
  return /** @type {!proto.repository.ConfigKind} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.repository.ConfigKind} value
 * @return {!proto.repository.PackageConfigFile} returns this
 */
proto.repository.PackageConfigFile.prototype.setConfigKind = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional string owner_package = 3;
 * @return {string}
 */
proto.repository.PackageConfigFile.prototype.getOwnerPackage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.PackageConfigFile} returns this
 */
proto.repository.PackageConfigFile.prototype.setOwnerPackage = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string checksum_at_install = 4;
 * @return {string}
 */
proto.repository.PackageConfigFile.prototype.getChecksumAtInstall = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.PackageConfigFile} returns this
 */
proto.repository.PackageConfigFile.prototype.setChecksumAtInstall = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string current_checksum = 5;
 * @return {string}
 */
proto.repository.PackageConfigFile.prototype.getCurrentChecksum = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.PackageConfigFile} returns this
 */
proto.repository.PackageConfigFile.prototype.setCurrentChecksum = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional int64 last_modified_unix = 6;
 * @return {number}
 */
proto.repository.PackageConfigFile.prototype.getLastModifiedUnix = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.PackageConfigFile} returns this
 */
proto.repository.PackageConfigFile.prototype.setLastModifiedUnix = function(value) {
  return jspb.Message.setProto3IntField(this, 6, value);
};


/**
 * optional MergeStrategy merge_strategy = 7;
 * @return {!proto.repository.MergeStrategy}
 */
proto.repository.PackageConfigFile.prototype.getMergeStrategy = function() {
  return /** @type {!proto.repository.MergeStrategy} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/**
 * @param {!proto.repository.MergeStrategy} value
 * @return {!proto.repository.PackageConfigFile} returns this
 */
proto.repository.PackageConfigFile.prototype.setMergeStrategy = function(value) {
  return jspb.Message.setProto3EnumField(this, 7, value);
};


/**
 * optional bool preserve_on_upgrade = 8;
 * @return {boolean}
 */
proto.repository.PackageConfigFile.prototype.getPreserveOnUpgrade = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 8, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.PackageConfigFile} returns this
 */
proto.repository.PackageConfigFile.prototype.setPreserveOnUpgrade = function(value) {
  return jspb.Message.setProto3BooleanField(this, 8, value);
};


/**
 * optional bool restore_on_rollback = 9;
 * @return {boolean}
 */
proto.repository.PackageConfigFile.prototype.getRestoreOnRollback = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 9, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.PackageConfigFile} returns this
 */
proto.repository.PackageConfigFile.prototype.setRestoreOnRollback = function(value) {
  return jspb.Message.setProto3BooleanField(this, 9, value);
};


/**
 * optional bool sensitive = 10;
 * @return {boolean}
 */
proto.repository.PackageConfigFile.prototype.getSensitive = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 10, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.PackageConfigFile} returns this
 */
proto.repository.PackageConfigFile.prototype.setSensitive = function(value) {
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
proto.repository.TrustedPublisher.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.TrustedPublisher.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.TrustedPublisher} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.TrustedPublisher.toObject = function(includeInstance, msg) {
  var f, obj = {
publisherId: jspb.Message.getFieldWithDefault(msg, 1, ""),
publicKeyId: jspb.Message.getFieldWithDefault(msg, 2, ""),
publicKeyPem: msg.getPublicKeyPem_asB64(),
trustState: jspb.Message.getFieldWithDefault(msg, 4, 0),
validFromUnix: jspb.Message.getFieldWithDefault(msg, 5, 0),
validUntilUnix: jspb.Message.getFieldWithDefault(msg, 6, 0),
createdBy: jspb.Message.getFieldWithDefault(msg, 7, ""),
createdUnix: jspb.Message.getFieldWithDefault(msg, 8, 0),
algorithm: jspb.Message.getFieldWithDefault(msg, 9, ""),
notes: jspb.Message.getFieldWithDefault(msg, 10, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.TrustedPublisher}
 */
proto.repository.TrustedPublisher.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.TrustedPublisher;
  return proto.repository.TrustedPublisher.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.TrustedPublisher} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.TrustedPublisher}
 */
proto.repository.TrustedPublisher.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisherId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublicKeyId(value);
      break;
    case 3:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setPublicKeyPem(value);
      break;
    case 4:
      var value = /** @type {!proto.repository.TrustState} */ (reader.readEnum());
      msg.setTrustState(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setValidFromUnix(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setValidUntilUnix(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setCreatedBy(value);
      break;
    case 8:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setCreatedUnix(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setAlgorithm(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.setNotes(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.TrustedPublisher.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.TrustedPublisher.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.TrustedPublisher} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.TrustedPublisher.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublisherId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getPublicKeyId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getPublicKeyPem_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      3,
      f
    );
  }
  f = message.getTrustState();
  if (f !== 0.0) {
    writer.writeEnum(
      4,
      f
    );
  }
  f = message.getValidFromUnix();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
  f = message.getValidUntilUnix();
  if (f !== 0) {
    writer.writeInt64(
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
  f = message.getCreatedUnix();
  if (f !== 0) {
    writer.writeInt64(
      8,
      f
    );
  }
  f = message.getAlgorithm();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
  f = message.getNotes();
  if (f.length > 0) {
    writer.writeString(
      10,
      f
    );
  }
};


/**
 * optional string publisher_id = 1;
 * @return {string}
 */
proto.repository.TrustedPublisher.prototype.getPublisherId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.TrustedPublisher} returns this
 */
proto.repository.TrustedPublisher.prototype.setPublisherId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string public_key_id = 2;
 * @return {string}
 */
proto.repository.TrustedPublisher.prototype.getPublicKeyId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.TrustedPublisher} returns this
 */
proto.repository.TrustedPublisher.prototype.setPublicKeyId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional bytes public_key_pem = 3;
 * @return {string}
 */
proto.repository.TrustedPublisher.prototype.getPublicKeyPem = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * optional bytes public_key_pem = 3;
 * This is a type-conversion wrapper around `getPublicKeyPem()`
 * @return {string}
 */
proto.repository.TrustedPublisher.prototype.getPublicKeyPem_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getPublicKeyPem()));
};


/**
 * optional bytes public_key_pem = 3;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getPublicKeyPem()`
 * @return {!Uint8Array}
 */
proto.repository.TrustedPublisher.prototype.getPublicKeyPem_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getPublicKeyPem()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.repository.TrustedPublisher} returns this
 */
proto.repository.TrustedPublisher.prototype.setPublicKeyPem = function(value) {
  return jspb.Message.setProto3BytesField(this, 3, value);
};


/**
 * optional TrustState trust_state = 4;
 * @return {!proto.repository.TrustState}
 */
proto.repository.TrustedPublisher.prototype.getTrustState = function() {
  return /** @type {!proto.repository.TrustState} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {!proto.repository.TrustState} value
 * @return {!proto.repository.TrustedPublisher} returns this
 */
proto.repository.TrustedPublisher.prototype.setTrustState = function(value) {
  return jspb.Message.setProto3EnumField(this, 4, value);
};


/**
 * optional int64 valid_from_unix = 5;
 * @return {number}
 */
proto.repository.TrustedPublisher.prototype.getValidFromUnix = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.TrustedPublisher} returns this
 */
proto.repository.TrustedPublisher.prototype.setValidFromUnix = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional int64 valid_until_unix = 6;
 * @return {number}
 */
proto.repository.TrustedPublisher.prototype.getValidUntilUnix = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.TrustedPublisher} returns this
 */
proto.repository.TrustedPublisher.prototype.setValidUntilUnix = function(value) {
  return jspb.Message.setProto3IntField(this, 6, value);
};


/**
 * optional string created_by = 7;
 * @return {string}
 */
proto.repository.TrustedPublisher.prototype.getCreatedBy = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.TrustedPublisher} returns this
 */
proto.repository.TrustedPublisher.prototype.setCreatedBy = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional int64 created_unix = 8;
 * @return {number}
 */
proto.repository.TrustedPublisher.prototype.getCreatedUnix = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 8, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.TrustedPublisher} returns this
 */
proto.repository.TrustedPublisher.prototype.setCreatedUnix = function(value) {
  return jspb.Message.setProto3IntField(this, 8, value);
};


/**
 * optional string algorithm = 9;
 * @return {string}
 */
proto.repository.TrustedPublisher.prototype.getAlgorithm = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.TrustedPublisher} returns this
 */
proto.repository.TrustedPublisher.prototype.setAlgorithm = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};


/**
 * optional string notes = 10;
 * @return {string}
 */
proto.repository.TrustedPublisher.prototype.getNotes = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 10, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.TrustedPublisher} returns this
 */
proto.repository.TrustedPublisher.prototype.setNotes = function(value) {
  return jspb.Message.setProto3StringField(this, 10, value);
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
proto.repository.ArtifactSignature.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ArtifactSignature.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ArtifactSignature} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ArtifactSignature.toObject = function(includeInstance, msg) {
  var f, obj = {
artifactKey: jspb.Message.getFieldWithDefault(msg, 1, ""),
digest: jspb.Message.getFieldWithDefault(msg, 2, ""),
algorithm: jspb.Message.getFieldWithDefault(msg, 3, ""),
signatureBytes: msg.getSignatureBytes_asB64(),
publicKeyId: jspb.Message.getFieldWithDefault(msg, 5, ""),
signedBy: jspb.Message.getFieldWithDefault(msg, 6, ""),
signedAtUnix: jspb.Message.getFieldWithDefault(msg, 7, 0),
provenanceRef: jspb.Message.getFieldWithDefault(msg, 8, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ArtifactSignature}
 */
proto.repository.ArtifactSignature.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ArtifactSignature;
  return proto.repository.ArtifactSignature.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ArtifactSignature} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ArtifactSignature}
 */
proto.repository.ArtifactSignature.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setArtifactKey(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setDigest(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setAlgorithm(value);
      break;
    case 4:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setSignatureBytes(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublicKeyId(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setSignedBy(value);
      break;
    case 7:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setSignedAtUnix(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setProvenanceRef(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.ArtifactSignature.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ArtifactSignature.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ArtifactSignature} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ArtifactSignature.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getArtifactKey();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getDigest();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getAlgorithm();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getSignatureBytes_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      4,
      f
    );
  }
  f = message.getPublicKeyId();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getSignedBy();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getSignedAtUnix();
  if (f !== 0) {
    writer.writeInt64(
      7,
      f
    );
  }
  f = message.getProvenanceRef();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
};


/**
 * optional string artifact_key = 1;
 * @return {string}
 */
proto.repository.ArtifactSignature.prototype.getArtifactKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactSignature} returns this
 */
proto.repository.ArtifactSignature.prototype.setArtifactKey = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string digest = 2;
 * @return {string}
 */
proto.repository.ArtifactSignature.prototype.getDigest = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactSignature} returns this
 */
proto.repository.ArtifactSignature.prototype.setDigest = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string algorithm = 3;
 * @return {string}
 */
proto.repository.ArtifactSignature.prototype.getAlgorithm = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactSignature} returns this
 */
proto.repository.ArtifactSignature.prototype.setAlgorithm = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional bytes signature_bytes = 4;
 * @return {string}
 */
proto.repository.ArtifactSignature.prototype.getSignatureBytes = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * optional bytes signature_bytes = 4;
 * This is a type-conversion wrapper around `getSignatureBytes()`
 * @return {string}
 */
proto.repository.ArtifactSignature.prototype.getSignatureBytes_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getSignatureBytes()));
};


/**
 * optional bytes signature_bytes = 4;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getSignatureBytes()`
 * @return {!Uint8Array}
 */
proto.repository.ArtifactSignature.prototype.getSignatureBytes_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getSignatureBytes()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.repository.ArtifactSignature} returns this
 */
proto.repository.ArtifactSignature.prototype.setSignatureBytes = function(value) {
  return jspb.Message.setProto3BytesField(this, 4, value);
};


/**
 * optional string public_key_id = 5;
 * @return {string}
 */
proto.repository.ArtifactSignature.prototype.getPublicKeyId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactSignature} returns this
 */
proto.repository.ArtifactSignature.prototype.setPublicKeyId = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string signed_by = 6;
 * @return {string}
 */
proto.repository.ArtifactSignature.prototype.getSignedBy = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactSignature} returns this
 */
proto.repository.ArtifactSignature.prototype.setSignedBy = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional int64 signed_at_unix = 7;
 * @return {number}
 */
proto.repository.ArtifactSignature.prototype.getSignedAtUnix = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ArtifactSignature} returns this
 */
proto.repository.ArtifactSignature.prototype.setSignedAtUnix = function(value) {
  return jspb.Message.setProto3IntField(this, 7, value);
};


/**
 * optional string provenance_ref = 8;
 * @return {string}
 */
proto.repository.ArtifactSignature.prototype.getProvenanceRef = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ArtifactSignature} returns this
 */
proto.repository.ArtifactSignature.prototype.setProvenanceRef = function(value) {
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
proto.repository.TrustPublisherRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.TrustPublisherRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.TrustPublisherRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.TrustPublisherRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
publisherId: jspb.Message.getFieldWithDefault(msg, 1, ""),
publicKeyId: jspb.Message.getFieldWithDefault(msg, 2, ""),
publicKeyPem: msg.getPublicKeyPem_asB64(),
algorithm: jspb.Message.getFieldWithDefault(msg, 4, ""),
validUntilUnix: jspb.Message.getFieldWithDefault(msg, 5, 0),
notes: jspb.Message.getFieldWithDefault(msg, 6, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.TrustPublisherRequest}
 */
proto.repository.TrustPublisherRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.TrustPublisherRequest;
  return proto.repository.TrustPublisherRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.TrustPublisherRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.TrustPublisherRequest}
 */
proto.repository.TrustPublisherRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisherId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublicKeyId(value);
      break;
    case 3:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setPublicKeyPem(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setAlgorithm(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setValidUntilUnix(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setNotes(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.TrustPublisherRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.TrustPublisherRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.TrustPublisherRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.TrustPublisherRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublisherId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getPublicKeyId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getPublicKeyPem_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      3,
      f
    );
  }
  f = message.getAlgorithm();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getValidUntilUnix();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
  f = message.getNotes();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
};


/**
 * optional string publisher_id = 1;
 * @return {string}
 */
proto.repository.TrustPublisherRequest.prototype.getPublisherId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.TrustPublisherRequest} returns this
 */
proto.repository.TrustPublisherRequest.prototype.setPublisherId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string public_key_id = 2;
 * @return {string}
 */
proto.repository.TrustPublisherRequest.prototype.getPublicKeyId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.TrustPublisherRequest} returns this
 */
proto.repository.TrustPublisherRequest.prototype.setPublicKeyId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional bytes public_key_pem = 3;
 * @return {string}
 */
proto.repository.TrustPublisherRequest.prototype.getPublicKeyPem = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * optional bytes public_key_pem = 3;
 * This is a type-conversion wrapper around `getPublicKeyPem()`
 * @return {string}
 */
proto.repository.TrustPublisherRequest.prototype.getPublicKeyPem_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getPublicKeyPem()));
};


/**
 * optional bytes public_key_pem = 3;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getPublicKeyPem()`
 * @return {!Uint8Array}
 */
proto.repository.TrustPublisherRequest.prototype.getPublicKeyPem_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getPublicKeyPem()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.repository.TrustPublisherRequest} returns this
 */
proto.repository.TrustPublisherRequest.prototype.setPublicKeyPem = function(value) {
  return jspb.Message.setProto3BytesField(this, 3, value);
};


/**
 * optional string algorithm = 4;
 * @return {string}
 */
proto.repository.TrustPublisherRequest.prototype.getAlgorithm = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.TrustPublisherRequest} returns this
 */
proto.repository.TrustPublisherRequest.prototype.setAlgorithm = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional int64 valid_until_unix = 5;
 * @return {number}
 */
proto.repository.TrustPublisherRequest.prototype.getValidUntilUnix = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.TrustPublisherRequest} returns this
 */
proto.repository.TrustPublisherRequest.prototype.setValidUntilUnix = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional string notes = 6;
 * @return {string}
 */
proto.repository.TrustPublisherRequest.prototype.getNotes = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.TrustPublisherRequest} returns this
 */
proto.repository.TrustPublisherRequest.prototype.setNotes = function(value) {
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
proto.repository.TrustPublisherResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.TrustPublisherResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.TrustPublisherResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.TrustPublisherResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
publisher: (f = msg.getPublisher()) && proto.repository.TrustedPublisher.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.TrustPublisherResponse}
 */
proto.repository.TrustPublisherResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.TrustPublisherResponse;
  return proto.repository.TrustPublisherResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.TrustPublisherResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.TrustPublisherResponse}
 */
proto.repository.TrustPublisherResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.TrustedPublisher;
      reader.readMessage(value,proto.repository.TrustedPublisher.deserializeBinaryFromReader);
      msg.setPublisher(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.TrustPublisherResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.TrustPublisherResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.TrustPublisherResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.TrustPublisherResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublisher();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.TrustedPublisher.serializeBinaryToWriter
    );
  }
};


/**
 * optional TrustedPublisher publisher = 1;
 * @return {?proto.repository.TrustedPublisher}
 */
proto.repository.TrustPublisherResponse.prototype.getPublisher = function() {
  return /** @type{?proto.repository.TrustedPublisher} */ (
    jspb.Message.getWrapperField(this, proto.repository.TrustedPublisher, 1));
};


/**
 * @param {?proto.repository.TrustedPublisher|undefined} value
 * @return {!proto.repository.TrustPublisherResponse} returns this
*/
proto.repository.TrustPublisherResponse.prototype.setPublisher = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.TrustPublisherResponse} returns this
 */
proto.repository.TrustPublisherResponse.prototype.clearPublisher = function() {
  return this.setPublisher(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.TrustPublisherResponse.prototype.hasPublisher = function() {
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
proto.repository.RevokePublisherKeyRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.RevokePublisherKeyRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.RevokePublisherKeyRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RevokePublisherKeyRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
publisherId: jspb.Message.getFieldWithDefault(msg, 1, ""),
publicKeyId: jspb.Message.getFieldWithDefault(msg, 2, ""),
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
 * @return {!proto.repository.RevokePublisherKeyRequest}
 */
proto.repository.RevokePublisherKeyRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.RevokePublisherKeyRequest;
  return proto.repository.RevokePublisherKeyRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.RevokePublisherKeyRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.RevokePublisherKeyRequest}
 */
proto.repository.RevokePublisherKeyRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisherId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublicKeyId(value);
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
proto.repository.RevokePublisherKeyRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.RevokePublisherKeyRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.RevokePublisherKeyRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RevokePublisherKeyRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublisherId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getPublicKeyId();
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
 * optional string publisher_id = 1;
 * @return {string}
 */
proto.repository.RevokePublisherKeyRequest.prototype.getPublisherId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RevokePublisherKeyRequest} returns this
 */
proto.repository.RevokePublisherKeyRequest.prototype.setPublisherId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string public_key_id = 2;
 * @return {string}
 */
proto.repository.RevokePublisherKeyRequest.prototype.getPublicKeyId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RevokePublisherKeyRequest} returns this
 */
proto.repository.RevokePublisherKeyRequest.prototype.setPublicKeyId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string reason = 3;
 * @return {string}
 */
proto.repository.RevokePublisherKeyRequest.prototype.getReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RevokePublisherKeyRequest} returns this
 */
proto.repository.RevokePublisherKeyRequest.prototype.setReason = function(value) {
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
proto.repository.RevokePublisherKeyResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.RevokePublisherKeyResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.RevokePublisherKeyResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RevokePublisherKeyResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
publisher: (f = msg.getPublisher()) && proto.repository.TrustedPublisher.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.RevokePublisherKeyResponse}
 */
proto.repository.RevokePublisherKeyResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.RevokePublisherKeyResponse;
  return proto.repository.RevokePublisherKeyResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.RevokePublisherKeyResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.RevokePublisherKeyResponse}
 */
proto.repository.RevokePublisherKeyResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.TrustedPublisher;
      reader.readMessage(value,proto.repository.TrustedPublisher.deserializeBinaryFromReader);
      msg.setPublisher(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.RevokePublisherKeyResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.RevokePublisherKeyResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.RevokePublisherKeyResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RevokePublisherKeyResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublisher();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.TrustedPublisher.serializeBinaryToWriter
    );
  }
};


/**
 * optional TrustedPublisher publisher = 1;
 * @return {?proto.repository.TrustedPublisher}
 */
proto.repository.RevokePublisherKeyResponse.prototype.getPublisher = function() {
  return /** @type{?proto.repository.TrustedPublisher} */ (
    jspb.Message.getWrapperField(this, proto.repository.TrustedPublisher, 1));
};


/**
 * @param {?proto.repository.TrustedPublisher|undefined} value
 * @return {!proto.repository.RevokePublisherKeyResponse} returns this
*/
proto.repository.RevokePublisherKeyResponse.prototype.setPublisher = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.RevokePublisherKeyResponse} returns this
 */
proto.repository.RevokePublisherKeyResponse.prototype.clearPublisher = function() {
  return this.setPublisher(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.RevokePublisherKeyResponse.prototype.hasPublisher = function() {
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
proto.repository.ListTrustedPublishersRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ListTrustedPublishersRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ListTrustedPublishersRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListTrustedPublishersRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
publisherId: jspb.Message.getFieldWithDefault(msg, 1, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ListTrustedPublishersRequest}
 */
proto.repository.ListTrustedPublishersRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ListTrustedPublishersRequest;
  return proto.repository.ListTrustedPublishersRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ListTrustedPublishersRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ListTrustedPublishersRequest}
 */
proto.repository.ListTrustedPublishersRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
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
proto.repository.ListTrustedPublishersRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ListTrustedPublishersRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ListTrustedPublishersRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListTrustedPublishersRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublisherId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string publisher_id = 1;
 * @return {string}
 */
proto.repository.ListTrustedPublishersRequest.prototype.getPublisherId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ListTrustedPublishersRequest} returns this
 */
proto.repository.ListTrustedPublishersRequest.prototype.setPublisherId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.ListTrustedPublishersResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.ListTrustedPublishersResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ListTrustedPublishersResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ListTrustedPublishersResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListTrustedPublishersResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
publishersList: jspb.Message.toObjectList(msg.getPublishersList(),
    proto.repository.TrustedPublisher.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ListTrustedPublishersResponse}
 */
proto.repository.ListTrustedPublishersResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ListTrustedPublishersResponse;
  return proto.repository.ListTrustedPublishersResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ListTrustedPublishersResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ListTrustedPublishersResponse}
 */
proto.repository.ListTrustedPublishersResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.TrustedPublisher;
      reader.readMessage(value,proto.repository.TrustedPublisher.deserializeBinaryFromReader);
      msg.addPublishers(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.ListTrustedPublishersResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ListTrustedPublishersResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ListTrustedPublishersResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListTrustedPublishersResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublishersList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.repository.TrustedPublisher.serializeBinaryToWriter
    );
  }
};


/**
 * repeated TrustedPublisher publishers = 1;
 * @return {!Array<!proto.repository.TrustedPublisher>}
 */
proto.repository.ListTrustedPublishersResponse.prototype.getPublishersList = function() {
  return /** @type{!Array<!proto.repository.TrustedPublisher>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.repository.TrustedPublisher, 1));
};


/**
 * @param {!Array<!proto.repository.TrustedPublisher>} value
 * @return {!proto.repository.ListTrustedPublishersResponse} returns this
*/
proto.repository.ListTrustedPublishersResponse.prototype.setPublishersList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.repository.TrustedPublisher=} opt_value
 * @param {number=} opt_index
 * @return {!proto.repository.TrustedPublisher}
 */
proto.repository.ListTrustedPublishersResponse.prototype.addPublishers = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.repository.TrustedPublisher, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ListTrustedPublishersResponse} returns this
 */
proto.repository.ListTrustedPublishersResponse.prototype.clearPublishersList = function() {
  return this.setPublishersList([]);
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
proto.repository.RegisterArtifactSignatureRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.RegisterArtifactSignatureRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.RegisterArtifactSignatureRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RegisterArtifactSignatureRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
ref: (f = msg.getRef()) && proto.repository.ArtifactRef.toObject(includeInstance, f),
buildNumber: jspb.Message.getFieldWithDefault(msg, 2, 0),
algorithm: jspb.Message.getFieldWithDefault(msg, 3, ""),
publicKeyId: jspb.Message.getFieldWithDefault(msg, 4, ""),
signatureBytes: msg.getSignatureBytes_asB64(),
provenanceRef: jspb.Message.getFieldWithDefault(msg, 6, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.RegisterArtifactSignatureRequest}
 */
proto.repository.RegisterArtifactSignatureRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.RegisterArtifactSignatureRequest;
  return proto.repository.RegisterArtifactSignatureRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.RegisterArtifactSignatureRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.RegisterArtifactSignatureRequest}
 */
proto.repository.RegisterArtifactSignatureRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactRef;
      reader.readMessage(value,proto.repository.ArtifactRef.deserializeBinaryFromReader);
      msg.setRef(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setAlgorithm(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublicKeyId(value);
      break;
    case 5:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setSignatureBytes(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setProvenanceRef(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.RegisterArtifactSignatureRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.RegisterArtifactSignatureRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.RegisterArtifactSignatureRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RegisterArtifactSignatureRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRef();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.ArtifactRef.serializeBinaryToWriter
    );
  }
  f = message.getBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
      2,
      f
    );
  }
  f = message.getAlgorithm();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getPublicKeyId();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getSignatureBytes_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      5,
      f
    );
  }
  f = message.getProvenanceRef();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
};


/**
 * optional ArtifactRef ref = 1;
 * @return {?proto.repository.ArtifactRef}
 */
proto.repository.RegisterArtifactSignatureRequest.prototype.getRef = function() {
  return /** @type{?proto.repository.ArtifactRef} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactRef, 1));
};


/**
 * @param {?proto.repository.ArtifactRef|undefined} value
 * @return {!proto.repository.RegisterArtifactSignatureRequest} returns this
*/
proto.repository.RegisterArtifactSignatureRequest.prototype.setRef = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.RegisterArtifactSignatureRequest} returns this
 */
proto.repository.RegisterArtifactSignatureRequest.prototype.clearRef = function() {
  return this.setRef(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.RegisterArtifactSignatureRequest.prototype.hasRef = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional int64 build_number = 2;
 * @return {number}
 */
proto.repository.RegisterArtifactSignatureRequest.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.RegisterArtifactSignatureRequest} returns this
 */
proto.repository.RegisterArtifactSignatureRequest.prototype.setBuildNumber = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string algorithm = 3;
 * @return {string}
 */
proto.repository.RegisterArtifactSignatureRequest.prototype.getAlgorithm = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RegisterArtifactSignatureRequest} returns this
 */
proto.repository.RegisterArtifactSignatureRequest.prototype.setAlgorithm = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string public_key_id = 4;
 * @return {string}
 */
proto.repository.RegisterArtifactSignatureRequest.prototype.getPublicKeyId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RegisterArtifactSignatureRequest} returns this
 */
proto.repository.RegisterArtifactSignatureRequest.prototype.setPublicKeyId = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional bytes signature_bytes = 5;
 * @return {string}
 */
proto.repository.RegisterArtifactSignatureRequest.prototype.getSignatureBytes = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * optional bytes signature_bytes = 5;
 * This is a type-conversion wrapper around `getSignatureBytes()`
 * @return {string}
 */
proto.repository.RegisterArtifactSignatureRequest.prototype.getSignatureBytes_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getSignatureBytes()));
};


/**
 * optional bytes signature_bytes = 5;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getSignatureBytes()`
 * @return {!Uint8Array}
 */
proto.repository.RegisterArtifactSignatureRequest.prototype.getSignatureBytes_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getSignatureBytes()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.repository.RegisterArtifactSignatureRequest} returns this
 */
proto.repository.RegisterArtifactSignatureRequest.prototype.setSignatureBytes = function(value) {
  return jspb.Message.setProto3BytesField(this, 5, value);
};


/**
 * optional string provenance_ref = 6;
 * @return {string}
 */
proto.repository.RegisterArtifactSignatureRequest.prototype.getProvenanceRef = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RegisterArtifactSignatureRequest} returns this
 */
proto.repository.RegisterArtifactSignatureRequest.prototype.setProvenanceRef = function(value) {
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
proto.repository.RegisterArtifactSignatureResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.RegisterArtifactSignatureResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.RegisterArtifactSignatureResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RegisterArtifactSignatureResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
signature: (f = msg.getSignature()) && proto.repository.ArtifactSignature.toObject(includeInstance, f),
status: jspb.Message.getFieldWithDefault(msg, 2, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.RegisterArtifactSignatureResponse}
 */
proto.repository.RegisterArtifactSignatureResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.RegisterArtifactSignatureResponse;
  return proto.repository.RegisterArtifactSignatureResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.RegisterArtifactSignatureResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.RegisterArtifactSignatureResponse}
 */
proto.repository.RegisterArtifactSignatureResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactSignature;
      reader.readMessage(value,proto.repository.ArtifactSignature.deserializeBinaryFromReader);
      msg.setSignature(value);
      break;
    case 2:
      var value = /** @type {!proto.repository.SignatureStatus} */ (reader.readEnum());
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
proto.repository.RegisterArtifactSignatureResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.RegisterArtifactSignatureResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.RegisterArtifactSignatureResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RegisterArtifactSignatureResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSignature();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.ArtifactSignature.serializeBinaryToWriter
    );
  }
  f = message.getStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
};


/**
 * optional ArtifactSignature signature = 1;
 * @return {?proto.repository.ArtifactSignature}
 */
proto.repository.RegisterArtifactSignatureResponse.prototype.getSignature = function() {
  return /** @type{?proto.repository.ArtifactSignature} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactSignature, 1));
};


/**
 * @param {?proto.repository.ArtifactSignature|undefined} value
 * @return {!proto.repository.RegisterArtifactSignatureResponse} returns this
*/
proto.repository.RegisterArtifactSignatureResponse.prototype.setSignature = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.RegisterArtifactSignatureResponse} returns this
 */
proto.repository.RegisterArtifactSignatureResponse.prototype.clearSignature = function() {
  return this.setSignature(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.RegisterArtifactSignatureResponse.prototype.hasSignature = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional SignatureStatus status = 2;
 * @return {!proto.repository.SignatureStatus}
 */
proto.repository.RegisterArtifactSignatureResponse.prototype.getStatus = function() {
  return /** @type {!proto.repository.SignatureStatus} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.repository.SignatureStatus} value
 * @return {!proto.repository.RegisterArtifactSignatureResponse} returns this
 */
proto.repository.RegisterArtifactSignatureResponse.prototype.setStatus = function(value) {
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
proto.repository.VerifyArtifactSignatureRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.VerifyArtifactSignatureRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.VerifyArtifactSignatureRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.VerifyArtifactSignatureRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
ref: (f = msg.getRef()) && proto.repository.ArtifactRef.toObject(includeInstance, f),
buildNumber: jspb.Message.getFieldWithDefault(msg, 2, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.VerifyArtifactSignatureRequest}
 */
proto.repository.VerifyArtifactSignatureRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.VerifyArtifactSignatureRequest;
  return proto.repository.VerifyArtifactSignatureRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.VerifyArtifactSignatureRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.VerifyArtifactSignatureRequest}
 */
proto.repository.VerifyArtifactSignatureRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactRef;
      reader.readMessage(value,proto.repository.ArtifactRef.deserializeBinaryFromReader);
      msg.setRef(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.VerifyArtifactSignatureRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.VerifyArtifactSignatureRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.VerifyArtifactSignatureRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.VerifyArtifactSignatureRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRef();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.ArtifactRef.serializeBinaryToWriter
    );
  }
  f = message.getBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
      2,
      f
    );
  }
};


/**
 * optional ArtifactRef ref = 1;
 * @return {?proto.repository.ArtifactRef}
 */
proto.repository.VerifyArtifactSignatureRequest.prototype.getRef = function() {
  return /** @type{?proto.repository.ArtifactRef} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactRef, 1));
};


/**
 * @param {?proto.repository.ArtifactRef|undefined} value
 * @return {!proto.repository.VerifyArtifactSignatureRequest} returns this
*/
proto.repository.VerifyArtifactSignatureRequest.prototype.setRef = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.VerifyArtifactSignatureRequest} returns this
 */
proto.repository.VerifyArtifactSignatureRequest.prototype.clearRef = function() {
  return this.setRef(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.VerifyArtifactSignatureRequest.prototype.hasRef = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional int64 build_number = 2;
 * @return {number}
 */
proto.repository.VerifyArtifactSignatureRequest.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.VerifyArtifactSignatureRequest} returns this
 */
proto.repository.VerifyArtifactSignatureRequest.prototype.setBuildNumber = function(value) {
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
proto.repository.VerifyArtifactSignatureResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.VerifyArtifactSignatureResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.VerifyArtifactSignatureResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.VerifyArtifactSignatureResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
status: jspb.Message.getFieldWithDefault(msg, 1, 0),
reason: jspb.Message.getFieldWithDefault(msg, 2, ""),
signature: (f = msg.getSignature()) && proto.repository.ArtifactSignature.toObject(includeInstance, f),
publisher: (f = msg.getPublisher()) && proto.repository.TrustedPublisher.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.VerifyArtifactSignatureResponse}
 */
proto.repository.VerifyArtifactSignatureResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.VerifyArtifactSignatureResponse;
  return proto.repository.VerifyArtifactSignatureResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.VerifyArtifactSignatureResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.VerifyArtifactSignatureResponse}
 */
proto.repository.VerifyArtifactSignatureResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.repository.SignatureStatus} */ (reader.readEnum());
      msg.setStatus(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setReason(value);
      break;
    case 3:
      var value = new proto.repository.ArtifactSignature;
      reader.readMessage(value,proto.repository.ArtifactSignature.deserializeBinaryFromReader);
      msg.setSignature(value);
      break;
    case 4:
      var value = new proto.repository.TrustedPublisher;
      reader.readMessage(value,proto.repository.TrustedPublisher.deserializeBinaryFromReader);
      msg.setPublisher(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.VerifyArtifactSignatureResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.VerifyArtifactSignatureResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.VerifyArtifactSignatureResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.VerifyArtifactSignatureResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getStatus();
  if (f !== 0.0) {
    writer.writeEnum(
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
  f = message.getSignature();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.repository.ArtifactSignature.serializeBinaryToWriter
    );
  }
  f = message.getPublisher();
  if (f != null) {
    writer.writeMessage(
      4,
      f,
      proto.repository.TrustedPublisher.serializeBinaryToWriter
    );
  }
};


/**
 * optional SignatureStatus status = 1;
 * @return {!proto.repository.SignatureStatus}
 */
proto.repository.VerifyArtifactSignatureResponse.prototype.getStatus = function() {
  return /** @type {!proto.repository.SignatureStatus} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {!proto.repository.SignatureStatus} value
 * @return {!proto.repository.VerifyArtifactSignatureResponse} returns this
 */
proto.repository.VerifyArtifactSignatureResponse.prototype.setStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 1, value);
};


/**
 * optional string reason = 2;
 * @return {string}
 */
proto.repository.VerifyArtifactSignatureResponse.prototype.getReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.VerifyArtifactSignatureResponse} returns this
 */
proto.repository.VerifyArtifactSignatureResponse.prototype.setReason = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional ArtifactSignature signature = 3;
 * @return {?proto.repository.ArtifactSignature}
 */
proto.repository.VerifyArtifactSignatureResponse.prototype.getSignature = function() {
  return /** @type{?proto.repository.ArtifactSignature} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactSignature, 3));
};


/**
 * @param {?proto.repository.ArtifactSignature|undefined} value
 * @return {!proto.repository.VerifyArtifactSignatureResponse} returns this
*/
proto.repository.VerifyArtifactSignatureResponse.prototype.setSignature = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.VerifyArtifactSignatureResponse} returns this
 */
proto.repository.VerifyArtifactSignatureResponse.prototype.clearSignature = function() {
  return this.setSignature(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.VerifyArtifactSignatureResponse.prototype.hasSignature = function() {
  return jspb.Message.getField(this, 3) != null;
};


/**
 * optional TrustedPublisher publisher = 4;
 * @return {?proto.repository.TrustedPublisher}
 */
proto.repository.VerifyArtifactSignatureResponse.prototype.getPublisher = function() {
  return /** @type{?proto.repository.TrustedPublisher} */ (
    jspb.Message.getWrapperField(this, proto.repository.TrustedPublisher, 4));
};


/**
 * @param {?proto.repository.TrustedPublisher|undefined} value
 * @return {!proto.repository.VerifyArtifactSignatureResponse} returns this
*/
proto.repository.VerifyArtifactSignatureResponse.prototype.setPublisher = function(value) {
  return jspb.Message.setWrapperField(this, 4, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.VerifyArtifactSignatureResponse} returns this
 */
proto.repository.VerifyArtifactSignatureResponse.prototype.clearPublisher = function() {
  return this.setPublisher(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.VerifyArtifactSignatureResponse.prototype.hasPublisher = function() {
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
proto.repository.ListArtifactSignaturesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ListArtifactSignaturesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ListArtifactSignaturesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListArtifactSignaturesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
ref: (f = msg.getRef()) && proto.repository.ArtifactRef.toObject(includeInstance, f),
buildNumber: jspb.Message.getFieldWithDefault(msg, 2, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ListArtifactSignaturesRequest}
 */
proto.repository.ListArtifactSignaturesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ListArtifactSignaturesRequest;
  return proto.repository.ListArtifactSignaturesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ListArtifactSignaturesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ListArtifactSignaturesRequest}
 */
proto.repository.ListArtifactSignaturesRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactRef;
      reader.readMessage(value,proto.repository.ArtifactRef.deserializeBinaryFromReader);
      msg.setRef(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.ListArtifactSignaturesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ListArtifactSignaturesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ListArtifactSignaturesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListArtifactSignaturesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRef();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.ArtifactRef.serializeBinaryToWriter
    );
  }
  f = message.getBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
      2,
      f
    );
  }
};


/**
 * optional ArtifactRef ref = 1;
 * @return {?proto.repository.ArtifactRef}
 */
proto.repository.ListArtifactSignaturesRequest.prototype.getRef = function() {
  return /** @type{?proto.repository.ArtifactRef} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactRef, 1));
};


/**
 * @param {?proto.repository.ArtifactRef|undefined} value
 * @return {!proto.repository.ListArtifactSignaturesRequest} returns this
*/
proto.repository.ListArtifactSignaturesRequest.prototype.setRef = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.ListArtifactSignaturesRequest} returns this
 */
proto.repository.ListArtifactSignaturesRequest.prototype.clearRef = function() {
  return this.setRef(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.ListArtifactSignaturesRequest.prototype.hasRef = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional int64 build_number = 2;
 * @return {number}
 */
proto.repository.ListArtifactSignaturesRequest.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ListArtifactSignaturesRequest} returns this
 */
proto.repository.ListArtifactSignaturesRequest.prototype.setBuildNumber = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.ListArtifactSignaturesResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.ListArtifactSignaturesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ListArtifactSignaturesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ListArtifactSignaturesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListArtifactSignaturesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
signaturesList: jspb.Message.toObjectList(msg.getSignaturesList(),
    proto.repository.ArtifactSignature.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ListArtifactSignaturesResponse}
 */
proto.repository.ListArtifactSignaturesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ListArtifactSignaturesResponse;
  return proto.repository.ListArtifactSignaturesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ListArtifactSignaturesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ListArtifactSignaturesResponse}
 */
proto.repository.ListArtifactSignaturesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactSignature;
      reader.readMessage(value,proto.repository.ArtifactSignature.deserializeBinaryFromReader);
      msg.addSignatures(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.ListArtifactSignaturesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ListArtifactSignaturesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ListArtifactSignaturesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListArtifactSignaturesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSignaturesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.repository.ArtifactSignature.serializeBinaryToWriter
    );
  }
};


/**
 * repeated ArtifactSignature signatures = 1;
 * @return {!Array<!proto.repository.ArtifactSignature>}
 */
proto.repository.ListArtifactSignaturesResponse.prototype.getSignaturesList = function() {
  return /** @type{!Array<!proto.repository.ArtifactSignature>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.repository.ArtifactSignature, 1));
};


/**
 * @param {!Array<!proto.repository.ArtifactSignature>} value
 * @return {!proto.repository.ListArtifactSignaturesResponse} returns this
*/
proto.repository.ListArtifactSignaturesResponse.prototype.setSignaturesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.repository.ArtifactSignature=} opt_value
 * @param {number=} opt_index
 * @return {!proto.repository.ArtifactSignature}
 */
proto.repository.ListArtifactSignaturesResponse.prototype.addSignatures = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.repository.ArtifactSignature, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ListArtifactSignaturesResponse} returns this
 */
proto.repository.ListArtifactSignaturesResponse.prototype.clearSignaturesList = function() {
  return this.setSignaturesList([]);
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
proto.repository.InstalledPackageRevision.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.InstalledPackageRevision.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.InstalledPackageRevision} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.InstalledPackageRevision.toObject = function(includeInstance, msg) {
  var f, obj = {
publisherId: jspb.Message.getFieldWithDefault(msg, 1, ""),
name: jspb.Message.getFieldWithDefault(msg, 2, ""),
kind: jspb.Message.getFieldWithDefault(msg, 3, 0),
version: jspb.Message.getFieldWithDefault(msg, 4, ""),
buildId: jspb.Message.getFieldWithDefault(msg, 5, ""),
buildNumber: jspb.Message.getFieldWithDefault(msg, 6, 0),
platform: jspb.Message.getFieldWithDefault(msg, 7, ""),
checksum: jspb.Message.getFieldWithDefault(msg, 8, ""),
installedAtUnix: jspb.Message.getFieldWithDefault(msg, 9, 0),
installedByWorkflowRunId: jspb.Message.getFieldWithDefault(msg, 10, ""),
nodeId: jspb.Message.getFieldWithDefault(msg, 11, ""),
previousRevisionId: jspb.Message.getFieldWithDefault(msg, 12, ""),
configSnapshotId: jspb.Message.getFieldWithDefault(msg, 13, ""),
serviceStatusBefore: jspb.Message.getFieldWithDefault(msg, 14, ""),
serviceStatusAfter: jspb.Message.getFieldWithDefault(msg, 15, ""),
revisionId: jspb.Message.getFieldWithDefault(msg, 16, ""),
action: jspb.Message.getFieldWithDefault(msg, 17, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.InstalledPackageRevision}
 */
proto.repository.InstalledPackageRevision.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.InstalledPackageRevision;
  return proto.repository.InstalledPackageRevision.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.InstalledPackageRevision} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.InstalledPackageRevision}
 */
proto.repository.InstalledPackageRevision.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisherId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    case 3:
      var value = /** @type {!proto.repository.ArtifactKind} */ (reader.readEnum());
      msg.setKind(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setVersion(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setBuildId(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlatform(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setChecksum(value);
      break;
    case 9:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setInstalledAtUnix(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.setInstalledByWorkflowRunId(value);
      break;
    case 11:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeId(value);
      break;
    case 12:
      var value = /** @type {string} */ (reader.readString());
      msg.setPreviousRevisionId(value);
      break;
    case 13:
      var value = /** @type {string} */ (reader.readString());
      msg.setConfigSnapshotId(value);
      break;
    case 14:
      var value = /** @type {string} */ (reader.readString());
      msg.setServiceStatusBefore(value);
      break;
    case 15:
      var value = /** @type {string} */ (reader.readString());
      msg.setServiceStatusAfter(value);
      break;
    case 16:
      var value = /** @type {string} */ (reader.readString());
      msg.setRevisionId(value);
      break;
    case 17:
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
proto.repository.InstalledPackageRevision.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.InstalledPackageRevision.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.InstalledPackageRevision} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.InstalledPackageRevision.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublisherId();
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
  f = message.getKind();
  if (f !== 0.0) {
    writer.writeEnum(
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
  f = message.getBuildId();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
      6,
      f
    );
  }
  f = message.getPlatform();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getChecksum();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
  f = message.getInstalledAtUnix();
  if (f !== 0) {
    writer.writeInt64(
      9,
      f
    );
  }
  f = message.getInstalledByWorkflowRunId();
  if (f.length > 0) {
    writer.writeString(
      10,
      f
    );
  }
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      11,
      f
    );
  }
  f = message.getPreviousRevisionId();
  if (f.length > 0) {
    writer.writeString(
      12,
      f
    );
  }
  f = message.getConfigSnapshotId();
  if (f.length > 0) {
    writer.writeString(
      13,
      f
    );
  }
  f = message.getServiceStatusBefore();
  if (f.length > 0) {
    writer.writeString(
      14,
      f
    );
  }
  f = message.getServiceStatusAfter();
  if (f.length > 0) {
    writer.writeString(
      15,
      f
    );
  }
  f = message.getRevisionId();
  if (f.length > 0) {
    writer.writeString(
      16,
      f
    );
  }
  f = message.getAction();
  if (f.length > 0) {
    writer.writeString(
      17,
      f
    );
  }
};


/**
 * optional string publisher_id = 1;
 * @return {string}
 */
proto.repository.InstalledPackageRevision.prototype.getPublisherId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.InstalledPackageRevision} returns this
 */
proto.repository.InstalledPackageRevision.prototype.setPublisherId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string name = 2;
 * @return {string}
 */
proto.repository.InstalledPackageRevision.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.InstalledPackageRevision} returns this
 */
proto.repository.InstalledPackageRevision.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional ArtifactKind kind = 3;
 * @return {!proto.repository.ArtifactKind}
 */
proto.repository.InstalledPackageRevision.prototype.getKind = function() {
  return /** @type {!proto.repository.ArtifactKind} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.repository.ArtifactKind} value
 * @return {!proto.repository.InstalledPackageRevision} returns this
 */
proto.repository.InstalledPackageRevision.prototype.setKind = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
};


/**
 * optional string version = 4;
 * @return {string}
 */
proto.repository.InstalledPackageRevision.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.InstalledPackageRevision} returns this
 */
proto.repository.InstalledPackageRevision.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string build_id = 5;
 * @return {string}
 */
proto.repository.InstalledPackageRevision.prototype.getBuildId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.InstalledPackageRevision} returns this
 */
proto.repository.InstalledPackageRevision.prototype.setBuildId = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional int64 build_number = 6;
 * @return {number}
 */
proto.repository.InstalledPackageRevision.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.InstalledPackageRevision} returns this
 */
proto.repository.InstalledPackageRevision.prototype.setBuildNumber = function(value) {
  return jspb.Message.setProto3IntField(this, 6, value);
};


/**
 * optional string platform = 7;
 * @return {string}
 */
proto.repository.InstalledPackageRevision.prototype.getPlatform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.InstalledPackageRevision} returns this
 */
proto.repository.InstalledPackageRevision.prototype.setPlatform = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional string checksum = 8;
 * @return {string}
 */
proto.repository.InstalledPackageRevision.prototype.getChecksum = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.InstalledPackageRevision} returns this
 */
proto.repository.InstalledPackageRevision.prototype.setChecksum = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};


/**
 * optional int64 installed_at_unix = 9;
 * @return {number}
 */
proto.repository.InstalledPackageRevision.prototype.getInstalledAtUnix = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 9, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.InstalledPackageRevision} returns this
 */
proto.repository.InstalledPackageRevision.prototype.setInstalledAtUnix = function(value) {
  return jspb.Message.setProto3IntField(this, 9, value);
};


/**
 * optional string installed_by_workflow_run_id = 10;
 * @return {string}
 */
proto.repository.InstalledPackageRevision.prototype.getInstalledByWorkflowRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 10, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.InstalledPackageRevision} returns this
 */
proto.repository.InstalledPackageRevision.prototype.setInstalledByWorkflowRunId = function(value) {
  return jspb.Message.setProto3StringField(this, 10, value);
};


/**
 * optional string node_id = 11;
 * @return {string}
 */
proto.repository.InstalledPackageRevision.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 11, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.InstalledPackageRevision} returns this
 */
proto.repository.InstalledPackageRevision.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 11, value);
};


/**
 * optional string previous_revision_id = 12;
 * @return {string}
 */
proto.repository.InstalledPackageRevision.prototype.getPreviousRevisionId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 12, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.InstalledPackageRevision} returns this
 */
proto.repository.InstalledPackageRevision.prototype.setPreviousRevisionId = function(value) {
  return jspb.Message.setProto3StringField(this, 12, value);
};


/**
 * optional string config_snapshot_id = 13;
 * @return {string}
 */
proto.repository.InstalledPackageRevision.prototype.getConfigSnapshotId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 13, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.InstalledPackageRevision} returns this
 */
proto.repository.InstalledPackageRevision.prototype.setConfigSnapshotId = function(value) {
  return jspb.Message.setProto3StringField(this, 13, value);
};


/**
 * optional string service_status_before = 14;
 * @return {string}
 */
proto.repository.InstalledPackageRevision.prototype.getServiceStatusBefore = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 14, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.InstalledPackageRevision} returns this
 */
proto.repository.InstalledPackageRevision.prototype.setServiceStatusBefore = function(value) {
  return jspb.Message.setProto3StringField(this, 14, value);
};


/**
 * optional string service_status_after = 15;
 * @return {string}
 */
proto.repository.InstalledPackageRevision.prototype.getServiceStatusAfter = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 15, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.InstalledPackageRevision} returns this
 */
proto.repository.InstalledPackageRevision.prototype.setServiceStatusAfter = function(value) {
  return jspb.Message.setProto3StringField(this, 15, value);
};


/**
 * optional string revision_id = 16;
 * @return {string}
 */
proto.repository.InstalledPackageRevision.prototype.getRevisionId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 16, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.InstalledPackageRevision} returns this
 */
proto.repository.InstalledPackageRevision.prototype.setRevisionId = function(value) {
  return jspb.Message.setProto3StringField(this, 16, value);
};


/**
 * optional string action = 17;
 * @return {string}
 */
proto.repository.InstalledPackageRevision.prototype.getAction = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 17, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.InstalledPackageRevision} returns this
 */
proto.repository.InstalledPackageRevision.prototype.setAction = function(value) {
  return jspb.Message.setProto3StringField(this, 17, value);
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
proto.repository.RecordInstalledRevisionRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.RecordInstalledRevisionRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.RecordInstalledRevisionRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RecordInstalledRevisionRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
revision: (f = msg.getRevision()) && proto.repository.InstalledPackageRevision.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.RecordInstalledRevisionRequest}
 */
proto.repository.RecordInstalledRevisionRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.RecordInstalledRevisionRequest;
  return proto.repository.RecordInstalledRevisionRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.RecordInstalledRevisionRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.RecordInstalledRevisionRequest}
 */
proto.repository.RecordInstalledRevisionRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.InstalledPackageRevision;
      reader.readMessage(value,proto.repository.InstalledPackageRevision.deserializeBinaryFromReader);
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
proto.repository.RecordInstalledRevisionRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.RecordInstalledRevisionRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.RecordInstalledRevisionRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RecordInstalledRevisionRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRevision();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.InstalledPackageRevision.serializeBinaryToWriter
    );
  }
};


/**
 * optional InstalledPackageRevision revision = 1;
 * @return {?proto.repository.InstalledPackageRevision}
 */
proto.repository.RecordInstalledRevisionRequest.prototype.getRevision = function() {
  return /** @type{?proto.repository.InstalledPackageRevision} */ (
    jspb.Message.getWrapperField(this, proto.repository.InstalledPackageRevision, 1));
};


/**
 * @param {?proto.repository.InstalledPackageRevision|undefined} value
 * @return {!proto.repository.RecordInstalledRevisionRequest} returns this
*/
proto.repository.RecordInstalledRevisionRequest.prototype.setRevision = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.RecordInstalledRevisionRequest} returns this
 */
proto.repository.RecordInstalledRevisionRequest.prototype.clearRevision = function() {
  return this.setRevision(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.RecordInstalledRevisionRequest.prototype.hasRevision = function() {
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
proto.repository.RecordInstalledRevisionResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.RecordInstalledRevisionResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.RecordInstalledRevisionResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RecordInstalledRevisionResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
revisionId: jspb.Message.getFieldWithDefault(msg, 1, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.RecordInstalledRevisionResponse}
 */
proto.repository.RecordInstalledRevisionResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.RecordInstalledRevisionResponse;
  return proto.repository.RecordInstalledRevisionResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.RecordInstalledRevisionResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.RecordInstalledRevisionResponse}
 */
proto.repository.RecordInstalledRevisionResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRevisionId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.RecordInstalledRevisionResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.RecordInstalledRevisionResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.RecordInstalledRevisionResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RecordInstalledRevisionResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRevisionId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string revision_id = 1;
 * @return {string}
 */
proto.repository.RecordInstalledRevisionResponse.prototype.getRevisionId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RecordInstalledRevisionResponse} returns this
 */
proto.repository.RecordInstalledRevisionResponse.prototype.setRevisionId = function(value) {
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
proto.repository.ListInstalledRevisionsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ListInstalledRevisionsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ListInstalledRevisionsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListInstalledRevisionsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
publisherId: jspb.Message.getFieldWithDefault(msg, 1, ""),
name: jspb.Message.getFieldWithDefault(msg, 2, ""),
kind: jspb.Message.getFieldWithDefault(msg, 3, 0),
platform: jspb.Message.getFieldWithDefault(msg, 4, ""),
nodeId: jspb.Message.getFieldWithDefault(msg, 5, ""),
limit: jspb.Message.getFieldWithDefault(msg, 6, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ListInstalledRevisionsRequest}
 */
proto.repository.ListInstalledRevisionsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ListInstalledRevisionsRequest;
  return proto.repository.ListInstalledRevisionsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ListInstalledRevisionsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ListInstalledRevisionsRequest}
 */
proto.repository.ListInstalledRevisionsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisherId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    case 3:
      var value = /** @type {!proto.repository.ArtifactKind} */ (reader.readEnum());
      msg.setKind(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlatform(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeId(value);
      break;
    case 6:
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
proto.repository.ListInstalledRevisionsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ListInstalledRevisionsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ListInstalledRevisionsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListInstalledRevisionsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublisherId();
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
  f = message.getKind();
  if (f !== 0.0) {
    writer.writeEnum(
      3,
      f
    );
  }
  f = message.getPlatform();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getNodeId();
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
};


/**
 * optional string publisher_id = 1;
 * @return {string}
 */
proto.repository.ListInstalledRevisionsRequest.prototype.getPublisherId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ListInstalledRevisionsRequest} returns this
 */
proto.repository.ListInstalledRevisionsRequest.prototype.setPublisherId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string name = 2;
 * @return {string}
 */
proto.repository.ListInstalledRevisionsRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ListInstalledRevisionsRequest} returns this
 */
proto.repository.ListInstalledRevisionsRequest.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional ArtifactKind kind = 3;
 * @return {!proto.repository.ArtifactKind}
 */
proto.repository.ListInstalledRevisionsRequest.prototype.getKind = function() {
  return /** @type {!proto.repository.ArtifactKind} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.repository.ArtifactKind} value
 * @return {!proto.repository.ListInstalledRevisionsRequest} returns this
 */
proto.repository.ListInstalledRevisionsRequest.prototype.setKind = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
};


/**
 * optional string platform = 4;
 * @return {string}
 */
proto.repository.ListInstalledRevisionsRequest.prototype.getPlatform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ListInstalledRevisionsRequest} returns this
 */
proto.repository.ListInstalledRevisionsRequest.prototype.setPlatform = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string node_id = 5;
 * @return {string}
 */
proto.repository.ListInstalledRevisionsRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ListInstalledRevisionsRequest} returns this
 */
proto.repository.ListInstalledRevisionsRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional int32 limit = 6;
 * @return {number}
 */
proto.repository.ListInstalledRevisionsRequest.prototype.getLimit = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ListInstalledRevisionsRequest} returns this
 */
proto.repository.ListInstalledRevisionsRequest.prototype.setLimit = function(value) {
  return jspb.Message.setProto3IntField(this, 6, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.ListInstalledRevisionsResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.ListInstalledRevisionsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ListInstalledRevisionsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ListInstalledRevisionsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListInstalledRevisionsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
revisionsList: jspb.Message.toObjectList(msg.getRevisionsList(),
    proto.repository.InstalledPackageRevision.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ListInstalledRevisionsResponse}
 */
proto.repository.ListInstalledRevisionsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ListInstalledRevisionsResponse;
  return proto.repository.ListInstalledRevisionsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ListInstalledRevisionsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ListInstalledRevisionsResponse}
 */
proto.repository.ListInstalledRevisionsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.InstalledPackageRevision;
      reader.readMessage(value,proto.repository.InstalledPackageRevision.deserializeBinaryFromReader);
      msg.addRevisions(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.ListInstalledRevisionsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ListInstalledRevisionsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ListInstalledRevisionsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListInstalledRevisionsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRevisionsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.repository.InstalledPackageRevision.serializeBinaryToWriter
    );
  }
};


/**
 * repeated InstalledPackageRevision revisions = 1;
 * @return {!Array<!proto.repository.InstalledPackageRevision>}
 */
proto.repository.ListInstalledRevisionsResponse.prototype.getRevisionsList = function() {
  return /** @type{!Array<!proto.repository.InstalledPackageRevision>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.repository.InstalledPackageRevision, 1));
};


/**
 * @param {!Array<!proto.repository.InstalledPackageRevision>} value
 * @return {!proto.repository.ListInstalledRevisionsResponse} returns this
*/
proto.repository.ListInstalledRevisionsResponse.prototype.setRevisionsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.repository.InstalledPackageRevision=} opt_value
 * @param {number=} opt_index
 * @return {!proto.repository.InstalledPackageRevision}
 */
proto.repository.ListInstalledRevisionsResponse.prototype.addRevisions = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.repository.InstalledPackageRevision, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ListInstalledRevisionsResponse} returns this
 */
proto.repository.ListInstalledRevisionsResponse.prototype.clearRevisionsList = function() {
  return this.setRevisionsList([]);
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
proto.repository.RollbackEligibility.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.RollbackEligibility.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.RollbackEligibility} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RollbackEligibility.toObject = function(includeInstance, msg) {
  var f, obj = {
eligible: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
reason: jspb.Message.getFieldWithDefault(msg, 2, ""),
verifyStatus: jspb.Message.getFieldWithDefault(msg, 3, 0),
signatureStatus: jspb.Message.getFieldWithDefault(msg, 4, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.RollbackEligibility}
 */
proto.repository.RollbackEligibility.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.RollbackEligibility;
  return proto.repository.RollbackEligibility.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.RollbackEligibility} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.RollbackEligibility}
 */
proto.repository.RollbackEligibility.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setEligible(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setReason(value);
      break;
    case 3:
      var value = /** @type {!proto.repository.ArtifactVerifyStatus} */ (reader.readEnum());
      msg.setVerifyStatus(value);
      break;
    case 4:
      var value = /** @type {!proto.repository.SignatureStatus} */ (reader.readEnum());
      msg.setSignatureStatus(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.RollbackEligibility.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.RollbackEligibility.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.RollbackEligibility} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RollbackEligibility.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getEligible();
  if (f) {
    writer.writeBool(
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
  f = message.getVerifyStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      3,
      f
    );
  }
  f = message.getSignatureStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      4,
      f
    );
  }
};


/**
 * optional bool eligible = 1;
 * @return {boolean}
 */
proto.repository.RollbackEligibility.prototype.getEligible = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.RollbackEligibility} returns this
 */
proto.repository.RollbackEligibility.prototype.setEligible = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional string reason = 2;
 * @return {string}
 */
proto.repository.RollbackEligibility.prototype.getReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RollbackEligibility} returns this
 */
proto.repository.RollbackEligibility.prototype.setReason = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional ArtifactVerifyStatus verify_status = 3;
 * @return {!proto.repository.ArtifactVerifyStatus}
 */
proto.repository.RollbackEligibility.prototype.getVerifyStatus = function() {
  return /** @type {!proto.repository.ArtifactVerifyStatus} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.repository.ArtifactVerifyStatus} value
 * @return {!proto.repository.RollbackEligibility} returns this
 */
proto.repository.RollbackEligibility.prototype.setVerifyStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
};


/**
 * optional SignatureStatus signature_status = 4;
 * @return {!proto.repository.SignatureStatus}
 */
proto.repository.RollbackEligibility.prototype.getSignatureStatus = function() {
  return /** @type {!proto.repository.SignatureStatus} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {!proto.repository.SignatureStatus} value
 * @return {!proto.repository.RollbackEligibility} returns this
 */
proto.repository.RollbackEligibility.prototype.setSignatureStatus = function(value) {
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
proto.repository.RollbackCandidate.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.RollbackCandidate.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.RollbackCandidate} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RollbackCandidate.toObject = function(includeInstance, msg) {
  var f, obj = {
revision: (f = msg.getRevision()) && proto.repository.InstalledPackageRevision.toObject(includeInstance, f),
targetRef: (f = msg.getTargetRef()) && proto.repository.ArtifactRef.toObject(includeInstance, f),
eligibility: (f = msg.getEligibility()) && proto.repository.RollbackEligibility.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.RollbackCandidate}
 */
proto.repository.RollbackCandidate.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.RollbackCandidate;
  return proto.repository.RollbackCandidate.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.RollbackCandidate} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.RollbackCandidate}
 */
proto.repository.RollbackCandidate.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.InstalledPackageRevision;
      reader.readMessage(value,proto.repository.InstalledPackageRevision.deserializeBinaryFromReader);
      msg.setRevision(value);
      break;
    case 2:
      var value = new proto.repository.ArtifactRef;
      reader.readMessage(value,proto.repository.ArtifactRef.deserializeBinaryFromReader);
      msg.setTargetRef(value);
      break;
    case 3:
      var value = new proto.repository.RollbackEligibility;
      reader.readMessage(value,proto.repository.RollbackEligibility.deserializeBinaryFromReader);
      msg.setEligibility(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.RollbackCandidate.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.RollbackCandidate.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.RollbackCandidate} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RollbackCandidate.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRevision();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.InstalledPackageRevision.serializeBinaryToWriter
    );
  }
  f = message.getTargetRef();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.repository.ArtifactRef.serializeBinaryToWriter
    );
  }
  f = message.getEligibility();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.repository.RollbackEligibility.serializeBinaryToWriter
    );
  }
};


/**
 * optional InstalledPackageRevision revision = 1;
 * @return {?proto.repository.InstalledPackageRevision}
 */
proto.repository.RollbackCandidate.prototype.getRevision = function() {
  return /** @type{?proto.repository.InstalledPackageRevision} */ (
    jspb.Message.getWrapperField(this, proto.repository.InstalledPackageRevision, 1));
};


/**
 * @param {?proto.repository.InstalledPackageRevision|undefined} value
 * @return {!proto.repository.RollbackCandidate} returns this
*/
proto.repository.RollbackCandidate.prototype.setRevision = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.RollbackCandidate} returns this
 */
proto.repository.RollbackCandidate.prototype.clearRevision = function() {
  return this.setRevision(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.RollbackCandidate.prototype.hasRevision = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional ArtifactRef target_ref = 2;
 * @return {?proto.repository.ArtifactRef}
 */
proto.repository.RollbackCandidate.prototype.getTargetRef = function() {
  return /** @type{?proto.repository.ArtifactRef} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactRef, 2));
};


/**
 * @param {?proto.repository.ArtifactRef|undefined} value
 * @return {!proto.repository.RollbackCandidate} returns this
*/
proto.repository.RollbackCandidate.prototype.setTargetRef = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.RollbackCandidate} returns this
 */
proto.repository.RollbackCandidate.prototype.clearTargetRef = function() {
  return this.setTargetRef(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.RollbackCandidate.prototype.hasTargetRef = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional RollbackEligibility eligibility = 3;
 * @return {?proto.repository.RollbackEligibility}
 */
proto.repository.RollbackCandidate.prototype.getEligibility = function() {
  return /** @type{?proto.repository.RollbackEligibility} */ (
    jspb.Message.getWrapperField(this, proto.repository.RollbackEligibility, 3));
};


/**
 * @param {?proto.repository.RollbackEligibility|undefined} value
 * @return {!proto.repository.RollbackCandidate} returns this
*/
proto.repository.RollbackCandidate.prototype.setEligibility = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.RollbackCandidate} returns this
 */
proto.repository.RollbackCandidate.prototype.clearEligibility = function() {
  return this.setEligibility(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.RollbackCandidate.prototype.hasEligibility = function() {
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
proto.repository.ListRollbackCandidatesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ListRollbackCandidatesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ListRollbackCandidatesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListRollbackCandidatesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
publisherId: jspb.Message.getFieldWithDefault(msg, 1, ""),
name: jspb.Message.getFieldWithDefault(msg, 2, ""),
kind: jspb.Message.getFieldWithDefault(msg, 3, 0),
platform: jspb.Message.getFieldWithDefault(msg, 4, ""),
nodeId: jspb.Message.getFieldWithDefault(msg, 5, ""),
limit: jspb.Message.getFieldWithDefault(msg, 6, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ListRollbackCandidatesRequest}
 */
proto.repository.ListRollbackCandidatesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ListRollbackCandidatesRequest;
  return proto.repository.ListRollbackCandidatesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ListRollbackCandidatesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ListRollbackCandidatesRequest}
 */
proto.repository.ListRollbackCandidatesRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisherId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    case 3:
      var value = /** @type {!proto.repository.ArtifactKind} */ (reader.readEnum());
      msg.setKind(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlatform(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeId(value);
      break;
    case 6:
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
proto.repository.ListRollbackCandidatesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ListRollbackCandidatesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ListRollbackCandidatesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListRollbackCandidatesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublisherId();
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
  f = message.getKind();
  if (f !== 0.0) {
    writer.writeEnum(
      3,
      f
    );
  }
  f = message.getPlatform();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getNodeId();
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
};


/**
 * optional string publisher_id = 1;
 * @return {string}
 */
proto.repository.ListRollbackCandidatesRequest.prototype.getPublisherId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ListRollbackCandidatesRequest} returns this
 */
proto.repository.ListRollbackCandidatesRequest.prototype.setPublisherId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string name = 2;
 * @return {string}
 */
proto.repository.ListRollbackCandidatesRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ListRollbackCandidatesRequest} returns this
 */
proto.repository.ListRollbackCandidatesRequest.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional ArtifactKind kind = 3;
 * @return {!proto.repository.ArtifactKind}
 */
proto.repository.ListRollbackCandidatesRequest.prototype.getKind = function() {
  return /** @type {!proto.repository.ArtifactKind} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.repository.ArtifactKind} value
 * @return {!proto.repository.ListRollbackCandidatesRequest} returns this
 */
proto.repository.ListRollbackCandidatesRequest.prototype.setKind = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
};


/**
 * optional string platform = 4;
 * @return {string}
 */
proto.repository.ListRollbackCandidatesRequest.prototype.getPlatform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ListRollbackCandidatesRequest} returns this
 */
proto.repository.ListRollbackCandidatesRequest.prototype.setPlatform = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string node_id = 5;
 * @return {string}
 */
proto.repository.ListRollbackCandidatesRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ListRollbackCandidatesRequest} returns this
 */
proto.repository.ListRollbackCandidatesRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional int32 limit = 6;
 * @return {number}
 */
proto.repository.ListRollbackCandidatesRequest.prototype.getLimit = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ListRollbackCandidatesRequest} returns this
 */
proto.repository.ListRollbackCandidatesRequest.prototype.setLimit = function(value) {
  return jspb.Message.setProto3IntField(this, 6, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.ListRollbackCandidatesResponse.repeatedFields_ = [2];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.ListRollbackCandidatesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ListRollbackCandidatesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ListRollbackCandidatesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListRollbackCandidatesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
currentRef: (f = msg.getCurrentRef()) && proto.repository.ArtifactRef.toObject(includeInstance, f),
candidatesList: jspb.Message.toObjectList(msg.getCandidatesList(),
    proto.repository.RollbackCandidate.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ListRollbackCandidatesResponse}
 */
proto.repository.ListRollbackCandidatesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ListRollbackCandidatesResponse;
  return proto.repository.ListRollbackCandidatesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ListRollbackCandidatesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ListRollbackCandidatesResponse}
 */
proto.repository.ListRollbackCandidatesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.ArtifactRef;
      reader.readMessage(value,proto.repository.ArtifactRef.deserializeBinaryFromReader);
      msg.setCurrentRef(value);
      break;
    case 2:
      var value = new proto.repository.RollbackCandidate;
      reader.readMessage(value,proto.repository.RollbackCandidate.deserializeBinaryFromReader);
      msg.addCandidates(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.ListRollbackCandidatesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ListRollbackCandidatesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ListRollbackCandidatesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListRollbackCandidatesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getCurrentRef();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.ArtifactRef.serializeBinaryToWriter
    );
  }
  f = message.getCandidatesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      2,
      f,
      proto.repository.RollbackCandidate.serializeBinaryToWriter
    );
  }
};


/**
 * optional ArtifactRef current_ref = 1;
 * @return {?proto.repository.ArtifactRef}
 */
proto.repository.ListRollbackCandidatesResponse.prototype.getCurrentRef = function() {
  return /** @type{?proto.repository.ArtifactRef} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactRef, 1));
};


/**
 * @param {?proto.repository.ArtifactRef|undefined} value
 * @return {!proto.repository.ListRollbackCandidatesResponse} returns this
*/
proto.repository.ListRollbackCandidatesResponse.prototype.setCurrentRef = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.ListRollbackCandidatesResponse} returns this
 */
proto.repository.ListRollbackCandidatesResponse.prototype.clearCurrentRef = function() {
  return this.setCurrentRef(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.ListRollbackCandidatesResponse.prototype.hasCurrentRef = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * repeated RollbackCandidate candidates = 2;
 * @return {!Array<!proto.repository.RollbackCandidate>}
 */
proto.repository.ListRollbackCandidatesResponse.prototype.getCandidatesList = function() {
  return /** @type{!Array<!proto.repository.RollbackCandidate>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.repository.RollbackCandidate, 2));
};


/**
 * @param {!Array<!proto.repository.RollbackCandidate>} value
 * @return {!proto.repository.ListRollbackCandidatesResponse} returns this
*/
proto.repository.ListRollbackCandidatesResponse.prototype.setCandidatesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 2, value);
};


/**
 * @param {!proto.repository.RollbackCandidate=} opt_value
 * @param {number=} opt_index
 * @return {!proto.repository.RollbackCandidate}
 */
proto.repository.ListRollbackCandidatesResponse.prototype.addCandidates = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 2, opt_value, proto.repository.RollbackCandidate, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ListRollbackCandidatesResponse} returns this
 */
proto.repository.ListRollbackCandidatesResponse.prototype.clearCandidatesList = function() {
  return this.setCandidatesList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.SignaturePolicy.repeatedFields_ = [4];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.SignaturePolicy.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.SignaturePolicy.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.SignaturePolicy} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.SignaturePolicy.toObject = function(includeInstance, msg) {
  var f, obj = {
requireSignaturesForCore: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
requireSignaturesForAll: jspb.Message.getBooleanFieldWithDefault(msg, 2, false),
allowUnsignedLocalDevelopment: jspb.Message.getBooleanFieldWithDefault(msg, 3, false),
trustedCorePublishersList: (f = jspb.Message.getRepeatedField(msg, 4)) == null ? undefined : f,
quarantineOnInvalidSignature: jspb.Message.getBooleanFieldWithDefault(msg, 5, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.SignaturePolicy}
 */
proto.repository.SignaturePolicy.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.SignaturePolicy;
  return proto.repository.SignaturePolicy.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.SignaturePolicy} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.SignaturePolicy}
 */
proto.repository.SignaturePolicy.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setRequireSignaturesForCore(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setRequireSignaturesForAll(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setAllowUnsignedLocalDevelopment(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.addTrustedCorePublishers(value);
      break;
    case 5:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setQuarantineOnInvalidSignature(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.SignaturePolicy.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.SignaturePolicy.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.SignaturePolicy} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.SignaturePolicy.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRequireSignaturesForCore();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getRequireSignaturesForAll();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
  f = message.getAllowUnsignedLocalDevelopment();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
  f = message.getTrustedCorePublishersList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      4,
      f
    );
  }
  f = message.getQuarantineOnInvalidSignature();
  if (f) {
    writer.writeBool(
      5,
      f
    );
  }
};


/**
 * optional bool require_signatures_for_core = 1;
 * @return {boolean}
 */
proto.repository.SignaturePolicy.prototype.getRequireSignaturesForCore = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.SignaturePolicy} returns this
 */
proto.repository.SignaturePolicy.prototype.setRequireSignaturesForCore = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional bool require_signatures_for_all = 2;
 * @return {boolean}
 */
proto.repository.SignaturePolicy.prototype.getRequireSignaturesForAll = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.SignaturePolicy} returns this
 */
proto.repository.SignaturePolicy.prototype.setRequireSignaturesForAll = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * optional bool allow_unsigned_local_development = 3;
 * @return {boolean}
 */
proto.repository.SignaturePolicy.prototype.getAllowUnsignedLocalDevelopment = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.SignaturePolicy} returns this
 */
proto.repository.SignaturePolicy.prototype.setAllowUnsignedLocalDevelopment = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * repeated string trusted_core_publishers = 4;
 * @return {!Array<string>}
 */
proto.repository.SignaturePolicy.prototype.getTrustedCorePublishersList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 4));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.SignaturePolicy} returns this
 */
proto.repository.SignaturePolicy.prototype.setTrustedCorePublishersList = function(value) {
  return jspb.Message.setField(this, 4, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.SignaturePolicy} returns this
 */
proto.repository.SignaturePolicy.prototype.addTrustedCorePublishers = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 4, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.SignaturePolicy} returns this
 */
proto.repository.SignaturePolicy.prototype.clearTrustedCorePublishersList = function() {
  return this.setTrustedCorePublishersList([]);
};


/**
 * optional bool quarantine_on_invalid_signature = 5;
 * @return {boolean}
 */
proto.repository.SignaturePolicy.prototype.getQuarantineOnInvalidSignature = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 5, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.SignaturePolicy} returns this
 */
proto.repository.SignaturePolicy.prototype.setQuarantineOnInvalidSignature = function(value) {
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
proto.repository.PackageConfigReceipt.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.PackageConfigReceipt.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.PackageConfigReceipt} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.PackageConfigReceipt.toObject = function(includeInstance, msg) {
  var f, obj = {
nodeId: jspb.Message.getFieldWithDefault(msg, 1, ""),
publisherId: jspb.Message.getFieldWithDefault(msg, 2, ""),
name: jspb.Message.getFieldWithDefault(msg, 3, ""),
platform: jspb.Message.getFieldWithDefault(msg, 4, ""),
buildNumber: jspb.Message.getFieldWithDefault(msg, 5, 0),
path: jspb.Message.getFieldWithDefault(msg, 6, ""),
configKind: jspb.Message.getFieldWithDefault(msg, 7, 0),
mergeStrategy: jspb.Message.getFieldWithDefault(msg, 8, 0),
checksumBefore: jspb.Message.getFieldWithDefault(msg, 9, ""),
checksumAfter: jspb.Message.getFieldWithDefault(msg, 10, ""),
action: jspb.Message.getFieldWithDefault(msg, 11, 0),
snapshotId: jspb.Message.getFieldWithDefault(msg, 12, ""),
workflowRunId: jspb.Message.getFieldWithDefault(msg, 13, ""),
timestampUnix: jspb.Message.getFieldWithDefault(msg, 14, 0),
reason: jspb.Message.getFieldWithDefault(msg, 15, ""),
sensitive: jspb.Message.getBooleanFieldWithDefault(msg, 16, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.PackageConfigReceipt}
 */
proto.repository.PackageConfigReceipt.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.PackageConfigReceipt;
  return proto.repository.PackageConfigReceipt.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.PackageConfigReceipt} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.PackageConfigReceipt}
 */
proto.repository.PackageConfigReceipt.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setPublisherId(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlatform(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setBuildNumber(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setPath(value);
      break;
    case 7:
      var value = /** @type {!proto.repository.ConfigKind} */ (reader.readEnum());
      msg.setConfigKind(value);
      break;
    case 8:
      var value = /** @type {!proto.repository.MergeStrategy} */ (reader.readEnum());
      msg.setMergeStrategy(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setChecksumBefore(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.setChecksumAfter(value);
      break;
    case 11:
      var value = /** @type {!proto.repository.ConfigReceiptAction} */ (reader.readEnum());
      msg.setAction(value);
      break;
    case 12:
      var value = /** @type {string} */ (reader.readString());
      msg.setSnapshotId(value);
      break;
    case 13:
      var value = /** @type {string} */ (reader.readString());
      msg.setWorkflowRunId(value);
      break;
    case 14:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setTimestampUnix(value);
      break;
    case 15:
      var value = /** @type {string} */ (reader.readString());
      msg.setReason(value);
      break;
    case 16:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setSensitive(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.PackageConfigReceipt.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.PackageConfigReceipt.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.PackageConfigReceipt} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.PackageConfigReceipt.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNodeId();
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
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getPlatform();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getBuildNumber();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
  f = message.getPath();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getConfigKind();
  if (f !== 0.0) {
    writer.writeEnum(
      7,
      f
    );
  }
  f = message.getMergeStrategy();
  if (f !== 0.0) {
    writer.writeEnum(
      8,
      f
    );
  }
  f = message.getChecksumBefore();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
  f = message.getChecksumAfter();
  if (f.length > 0) {
    writer.writeString(
      10,
      f
    );
  }
  f = message.getAction();
  if (f !== 0.0) {
    writer.writeEnum(
      11,
      f
    );
  }
  f = message.getSnapshotId();
  if (f.length > 0) {
    writer.writeString(
      12,
      f
    );
  }
  f = message.getWorkflowRunId();
  if (f.length > 0) {
    writer.writeString(
      13,
      f
    );
  }
  f = message.getTimestampUnix();
  if (f !== 0) {
    writer.writeInt64(
      14,
      f
    );
  }
  f = message.getReason();
  if (f.length > 0) {
    writer.writeString(
      15,
      f
    );
  }
  f = message.getSensitive();
  if (f) {
    writer.writeBool(
      16,
      f
    );
  }
};


/**
 * optional string node_id = 1;
 * @return {string}
 */
proto.repository.PackageConfigReceipt.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.PackageConfigReceipt} returns this
 */
proto.repository.PackageConfigReceipt.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string publisher_id = 2;
 * @return {string}
 */
proto.repository.PackageConfigReceipt.prototype.getPublisherId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.PackageConfigReceipt} returns this
 */
proto.repository.PackageConfigReceipt.prototype.setPublisherId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string name = 3;
 * @return {string}
 */
proto.repository.PackageConfigReceipt.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.PackageConfigReceipt} returns this
 */
proto.repository.PackageConfigReceipt.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string platform = 4;
 * @return {string}
 */
proto.repository.PackageConfigReceipt.prototype.getPlatform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.PackageConfigReceipt} returns this
 */
proto.repository.PackageConfigReceipt.prototype.setPlatform = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional int64 build_number = 5;
 * @return {number}
 */
proto.repository.PackageConfigReceipt.prototype.getBuildNumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.PackageConfigReceipt} returns this
 */
proto.repository.PackageConfigReceipt.prototype.setBuildNumber = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional string path = 6;
 * @return {string}
 */
proto.repository.PackageConfigReceipt.prototype.getPath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.PackageConfigReceipt} returns this
 */
proto.repository.PackageConfigReceipt.prototype.setPath = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional ConfigKind config_kind = 7;
 * @return {!proto.repository.ConfigKind}
 */
proto.repository.PackageConfigReceipt.prototype.getConfigKind = function() {
  return /** @type {!proto.repository.ConfigKind} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/**
 * @param {!proto.repository.ConfigKind} value
 * @return {!proto.repository.PackageConfigReceipt} returns this
 */
proto.repository.PackageConfigReceipt.prototype.setConfigKind = function(value) {
  return jspb.Message.setProto3EnumField(this, 7, value);
};


/**
 * optional MergeStrategy merge_strategy = 8;
 * @return {!proto.repository.MergeStrategy}
 */
proto.repository.PackageConfigReceipt.prototype.getMergeStrategy = function() {
  return /** @type {!proto.repository.MergeStrategy} */ (jspb.Message.getFieldWithDefault(this, 8, 0));
};


/**
 * @param {!proto.repository.MergeStrategy} value
 * @return {!proto.repository.PackageConfigReceipt} returns this
 */
proto.repository.PackageConfigReceipt.prototype.setMergeStrategy = function(value) {
  return jspb.Message.setProto3EnumField(this, 8, value);
};


/**
 * optional string checksum_before = 9;
 * @return {string}
 */
proto.repository.PackageConfigReceipt.prototype.getChecksumBefore = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.PackageConfigReceipt} returns this
 */
proto.repository.PackageConfigReceipt.prototype.setChecksumBefore = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};


/**
 * optional string checksum_after = 10;
 * @return {string}
 */
proto.repository.PackageConfigReceipt.prototype.getChecksumAfter = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 10, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.PackageConfigReceipt} returns this
 */
proto.repository.PackageConfigReceipt.prototype.setChecksumAfter = function(value) {
  return jspb.Message.setProto3StringField(this, 10, value);
};


/**
 * optional ConfigReceiptAction action = 11;
 * @return {!proto.repository.ConfigReceiptAction}
 */
proto.repository.PackageConfigReceipt.prototype.getAction = function() {
  return /** @type {!proto.repository.ConfigReceiptAction} */ (jspb.Message.getFieldWithDefault(this, 11, 0));
};


/**
 * @param {!proto.repository.ConfigReceiptAction} value
 * @return {!proto.repository.PackageConfigReceipt} returns this
 */
proto.repository.PackageConfigReceipt.prototype.setAction = function(value) {
  return jspb.Message.setProto3EnumField(this, 11, value);
};


/**
 * optional string snapshot_id = 12;
 * @return {string}
 */
proto.repository.PackageConfigReceipt.prototype.getSnapshotId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 12, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.PackageConfigReceipt} returns this
 */
proto.repository.PackageConfigReceipt.prototype.setSnapshotId = function(value) {
  return jspb.Message.setProto3StringField(this, 12, value);
};


/**
 * optional string workflow_run_id = 13;
 * @return {string}
 */
proto.repository.PackageConfigReceipt.prototype.getWorkflowRunId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 13, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.PackageConfigReceipt} returns this
 */
proto.repository.PackageConfigReceipt.prototype.setWorkflowRunId = function(value) {
  return jspb.Message.setProto3StringField(this, 13, value);
};


/**
 * optional int64 timestamp_unix = 14;
 * @return {number}
 */
proto.repository.PackageConfigReceipt.prototype.getTimestampUnix = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 14, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.PackageConfigReceipt} returns this
 */
proto.repository.PackageConfigReceipt.prototype.setTimestampUnix = function(value) {
  return jspb.Message.setProto3IntField(this, 14, value);
};


/**
 * optional string reason = 15;
 * @return {string}
 */
proto.repository.PackageConfigReceipt.prototype.getReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 15, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.PackageConfigReceipt} returns this
 */
proto.repository.PackageConfigReceipt.prototype.setReason = function(value) {
  return jspb.Message.setProto3StringField(this, 15, value);
};


/**
 * optional bool sensitive = 16;
 * @return {boolean}
 */
proto.repository.PackageConfigReceipt.prototype.getSensitive = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 16, false));
};


/**
 * @param {boolean} value
 * @return {!proto.repository.PackageConfigReceipt} returns this
 */
proto.repository.PackageConfigReceipt.prototype.setSensitive = function(value) {
  return jspb.Message.setProto3BooleanField(this, 16, value);
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
proto.repository.RecordConfigReceiptRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.RecordConfigReceiptRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.RecordConfigReceiptRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RecordConfigReceiptRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
receipt: (f = msg.getReceipt()) && proto.repository.PackageConfigReceipt.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.RecordConfigReceiptRequest}
 */
proto.repository.RecordConfigReceiptRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.RecordConfigReceiptRequest;
  return proto.repository.RecordConfigReceiptRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.RecordConfigReceiptRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.RecordConfigReceiptRequest}
 */
proto.repository.RecordConfigReceiptRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.PackageConfigReceipt;
      reader.readMessage(value,proto.repository.PackageConfigReceipt.deserializeBinaryFromReader);
      msg.setReceipt(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.RecordConfigReceiptRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.RecordConfigReceiptRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.RecordConfigReceiptRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RecordConfigReceiptRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getReceipt();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.repository.PackageConfigReceipt.serializeBinaryToWriter
    );
  }
};


/**
 * optional PackageConfigReceipt receipt = 1;
 * @return {?proto.repository.PackageConfigReceipt}
 */
proto.repository.RecordConfigReceiptRequest.prototype.getReceipt = function() {
  return /** @type{?proto.repository.PackageConfigReceipt} */ (
    jspb.Message.getWrapperField(this, proto.repository.PackageConfigReceipt, 1));
};


/**
 * @param {?proto.repository.PackageConfigReceipt|undefined} value
 * @return {!proto.repository.RecordConfigReceiptRequest} returns this
*/
proto.repository.RecordConfigReceiptRequest.prototype.setReceipt = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.RecordConfigReceiptRequest} returns this
 */
proto.repository.RecordConfigReceiptRequest.prototype.clearReceipt = function() {
  return this.setReceipt(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.RecordConfigReceiptRequest.prototype.hasReceipt = function() {
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
proto.repository.RecordConfigReceiptResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.RecordConfigReceiptResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.RecordConfigReceiptResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RecordConfigReceiptResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.repository.RecordConfigReceiptResponse}
 */
proto.repository.RecordConfigReceiptResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.RecordConfigReceiptResponse;
  return proto.repository.RecordConfigReceiptResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.RecordConfigReceiptResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.RecordConfigReceiptResponse}
 */
proto.repository.RecordConfigReceiptResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.repository.RecordConfigReceiptResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.RecordConfigReceiptResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.RecordConfigReceiptResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RecordConfigReceiptResponse.serializeBinaryToWriter = function(message, writer) {
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
proto.repository.ListConfigReceiptsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ListConfigReceiptsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ListConfigReceiptsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListConfigReceiptsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
publisherId: jspb.Message.getFieldWithDefault(msg, 1, ""),
name: jspb.Message.getFieldWithDefault(msg, 2, ""),
platform: jspb.Message.getFieldWithDefault(msg, 3, ""),
nodeId: jspb.Message.getFieldWithDefault(msg, 4, ""),
actionFilter: jspb.Message.getFieldWithDefault(msg, 5, 0),
limit: jspb.Message.getFieldWithDefault(msg, 6, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ListConfigReceiptsRequest}
 */
proto.repository.ListConfigReceiptsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ListConfigReceiptsRequest;
  return proto.repository.ListConfigReceiptsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ListConfigReceiptsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ListConfigReceiptsRequest}
 */
proto.repository.ListConfigReceiptsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisherId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlatform(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeId(value);
      break;
    case 5:
      var value = /** @type {!proto.repository.ConfigReceiptAction} */ (reader.readEnum());
      msg.setActionFilter(value);
      break;
    case 6:
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
proto.repository.ListConfigReceiptsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ListConfigReceiptsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ListConfigReceiptsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListConfigReceiptsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublisherId();
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
  f = message.getPlatform();
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
  f = message.getActionFilter();
  if (f !== 0.0) {
    writer.writeEnum(
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
};


/**
 * optional string publisher_id = 1;
 * @return {string}
 */
proto.repository.ListConfigReceiptsRequest.prototype.getPublisherId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ListConfigReceiptsRequest} returns this
 */
proto.repository.ListConfigReceiptsRequest.prototype.setPublisherId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string name = 2;
 * @return {string}
 */
proto.repository.ListConfigReceiptsRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ListConfigReceiptsRequest} returns this
 */
proto.repository.ListConfigReceiptsRequest.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string platform = 3;
 * @return {string}
 */
proto.repository.ListConfigReceiptsRequest.prototype.getPlatform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ListConfigReceiptsRequest} returns this
 */
proto.repository.ListConfigReceiptsRequest.prototype.setPlatform = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string node_id = 4;
 * @return {string}
 */
proto.repository.ListConfigReceiptsRequest.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.ListConfigReceiptsRequest} returns this
 */
proto.repository.ListConfigReceiptsRequest.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional ConfigReceiptAction action_filter = 5;
 * @return {!proto.repository.ConfigReceiptAction}
 */
proto.repository.ListConfigReceiptsRequest.prototype.getActionFilter = function() {
  return /** @type {!proto.repository.ConfigReceiptAction} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {!proto.repository.ConfigReceiptAction} value
 * @return {!proto.repository.ListConfigReceiptsRequest} returns this
 */
proto.repository.ListConfigReceiptsRequest.prototype.setActionFilter = function(value) {
  return jspb.Message.setProto3EnumField(this, 5, value);
};


/**
 * optional int32 limit = 6;
 * @return {number}
 */
proto.repository.ListConfigReceiptsRequest.prototype.getLimit = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ListConfigReceiptsRequest} returns this
 */
proto.repository.ListConfigReceiptsRequest.prototype.setLimit = function(value) {
  return jspb.Message.setProto3IntField(this, 6, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.ListConfigReceiptsResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.ListConfigReceiptsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ListConfigReceiptsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ListConfigReceiptsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListConfigReceiptsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
receiptsList: jspb.Message.toObjectList(msg.getReceiptsList(),
    proto.repository.PackageConfigReceipt.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ListConfigReceiptsResponse}
 */
proto.repository.ListConfigReceiptsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ListConfigReceiptsResponse;
  return proto.repository.ListConfigReceiptsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ListConfigReceiptsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ListConfigReceiptsResponse}
 */
proto.repository.ListConfigReceiptsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.PackageConfigReceipt;
      reader.readMessage(value,proto.repository.PackageConfigReceipt.deserializeBinaryFromReader);
      msg.addReceipts(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.ListConfigReceiptsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ListConfigReceiptsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ListConfigReceiptsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListConfigReceiptsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getReceiptsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.repository.PackageConfigReceipt.serializeBinaryToWriter
    );
  }
};


/**
 * repeated PackageConfigReceipt receipts = 1;
 * @return {!Array<!proto.repository.PackageConfigReceipt>}
 */
proto.repository.ListConfigReceiptsResponse.prototype.getReceiptsList = function() {
  return /** @type{!Array<!proto.repository.PackageConfigReceipt>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.repository.PackageConfigReceipt, 1));
};


/**
 * @param {!Array<!proto.repository.PackageConfigReceipt>} value
 * @return {!proto.repository.ListConfigReceiptsResponse} returns this
*/
proto.repository.ListConfigReceiptsResponse.prototype.setReceiptsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.repository.PackageConfigReceipt=} opt_value
 * @param {number=} opt_index
 * @return {!proto.repository.PackageConfigReceipt}
 */
proto.repository.ListConfigReceiptsResponse.prototype.addReceipts = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.repository.PackageConfigReceipt, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ListConfigReceiptsResponse} returns this
 */
proto.repository.ListConfigReceiptsResponse.prototype.clearReceiptsList = function() {
  return this.setReceiptsList([]);
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
proto.repository.RepositoryFinding.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.RepositoryFinding.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.RepositoryFinding} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RepositoryFinding.toObject = function(includeInstance, msg) {
  var f, obj = {
kind: jspb.Message.getFieldWithDefault(msg, 1, 0),
severity: jspb.Message.getFieldWithDefault(msg, 2, 0),
artifactKey: jspb.Message.getFieldWithDefault(msg, 3, ""),
ref: (f = msg.getRef()) && proto.repository.ArtifactRef.toObject(includeInstance, f),
nodeId: jspb.Message.getFieldWithDefault(msg, 5, ""),
currentState: jspb.Message.getFieldWithDefault(msg, 6, ""),
expectedState: jspb.Message.getFieldWithDefault(msg, 7, ""),
reason: jspb.Message.getFieldWithDefault(msg, 8, ""),
recommendedCommand: jspb.Message.getFieldWithDefault(msg, 9, ""),
evidenceMap: (f = msg.getEvidenceMap()) ? f.toObject(includeInstance, undefined) : [],
observedAtUnix: jspb.Message.getFieldWithDefault(msg, 11, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.RepositoryFinding}
 */
proto.repository.RepositoryFinding.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.RepositoryFinding;
  return proto.repository.RepositoryFinding.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.RepositoryFinding} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.RepositoryFinding}
 */
proto.repository.RepositoryFinding.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.repository.RepositoryFindingKind} */ (reader.readEnum());
      msg.setKind(value);
      break;
    case 2:
      var value = /** @type {!proto.repository.RepositoryFindingSeverity} */ (reader.readEnum());
      msg.setSeverity(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setArtifactKey(value);
      break;
    case 4:
      var value = new proto.repository.ArtifactRef;
      reader.readMessage(value,proto.repository.ArtifactRef.deserializeBinaryFromReader);
      msg.setRef(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setNodeId(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setCurrentState(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setExpectedState(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setReason(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setRecommendedCommand(value);
      break;
    case 10:
      var value = msg.getEvidenceMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readString, null, "", "");
         });
      break;
    case 11:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setObservedAtUnix(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.RepositoryFinding.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.RepositoryFinding.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.RepositoryFinding} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.RepositoryFinding.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getKind();
  if (f !== 0.0) {
    writer.writeEnum(
      1,
      f
    );
  }
  f = message.getSeverity();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
  f = message.getArtifactKey();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getRef();
  if (f != null) {
    writer.writeMessage(
      4,
      f,
      proto.repository.ArtifactRef.serializeBinaryToWriter
    );
  }
  f = message.getNodeId();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getCurrentState();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getExpectedState();
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
  f = message.getRecommendedCommand();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
  f = message.getEvidenceMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(10, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeString);
  }
  f = message.getObservedAtUnix();
  if (f !== 0) {
    writer.writeInt64(
      11,
      f
    );
  }
};


/**
 * optional RepositoryFindingKind kind = 1;
 * @return {!proto.repository.RepositoryFindingKind}
 */
proto.repository.RepositoryFinding.prototype.getKind = function() {
  return /** @type {!proto.repository.RepositoryFindingKind} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {!proto.repository.RepositoryFindingKind} value
 * @return {!proto.repository.RepositoryFinding} returns this
 */
proto.repository.RepositoryFinding.prototype.setKind = function(value) {
  return jspb.Message.setProto3EnumField(this, 1, value);
};


/**
 * optional RepositoryFindingSeverity severity = 2;
 * @return {!proto.repository.RepositoryFindingSeverity}
 */
proto.repository.RepositoryFinding.prototype.getSeverity = function() {
  return /** @type {!proto.repository.RepositoryFindingSeverity} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.repository.RepositoryFindingSeverity} value
 * @return {!proto.repository.RepositoryFinding} returns this
 */
proto.repository.RepositoryFinding.prototype.setSeverity = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional string artifact_key = 3;
 * @return {string}
 */
proto.repository.RepositoryFinding.prototype.getArtifactKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RepositoryFinding} returns this
 */
proto.repository.RepositoryFinding.prototype.setArtifactKey = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional ArtifactRef ref = 4;
 * @return {?proto.repository.ArtifactRef}
 */
proto.repository.RepositoryFinding.prototype.getRef = function() {
  return /** @type{?proto.repository.ArtifactRef} */ (
    jspb.Message.getWrapperField(this, proto.repository.ArtifactRef, 4));
};


/**
 * @param {?proto.repository.ArtifactRef|undefined} value
 * @return {!proto.repository.RepositoryFinding} returns this
*/
proto.repository.RepositoryFinding.prototype.setRef = function(value) {
  return jspb.Message.setWrapperField(this, 4, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.repository.RepositoryFinding} returns this
 */
proto.repository.RepositoryFinding.prototype.clearRef = function() {
  return this.setRef(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.repository.RepositoryFinding.prototype.hasRef = function() {
  return jspb.Message.getField(this, 4) != null;
};


/**
 * optional string node_id = 5;
 * @return {string}
 */
proto.repository.RepositoryFinding.prototype.getNodeId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RepositoryFinding} returns this
 */
proto.repository.RepositoryFinding.prototype.setNodeId = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string current_state = 6;
 * @return {string}
 */
proto.repository.RepositoryFinding.prototype.getCurrentState = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RepositoryFinding} returns this
 */
proto.repository.RepositoryFinding.prototype.setCurrentState = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string expected_state = 7;
 * @return {string}
 */
proto.repository.RepositoryFinding.prototype.getExpectedState = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RepositoryFinding} returns this
 */
proto.repository.RepositoryFinding.prototype.setExpectedState = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional string reason = 8;
 * @return {string}
 */
proto.repository.RepositoryFinding.prototype.getReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RepositoryFinding} returns this
 */
proto.repository.RepositoryFinding.prototype.setReason = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};


/**
 * optional string recommended_command = 9;
 * @return {string}
 */
proto.repository.RepositoryFinding.prototype.getRecommendedCommand = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.RepositoryFinding} returns this
 */
proto.repository.RepositoryFinding.prototype.setRecommendedCommand = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};


/**
 * map<string, string> evidence = 10;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,string>}
 */
proto.repository.RepositoryFinding.prototype.getEvidenceMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,string>} */ (
      jspb.Message.getMapField(this, 10, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.repository.RepositoryFinding} returns this
 */
proto.repository.RepositoryFinding.prototype.clearEvidenceMap = function() {
  this.getEvidenceMap().clear();
  return this;
};


/**
 * optional int64 observed_at_unix = 11;
 * @return {number}
 */
proto.repository.RepositoryFinding.prototype.getObservedAtUnix = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 11, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.RepositoryFinding} returns this
 */
proto.repository.RepositoryFinding.prototype.setObservedAtUnix = function(value) {
  return jspb.Message.setProto3IntField(this, 11, value);
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
proto.repository.ListRepositoryFindingsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ListRepositoryFindingsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ListRepositoryFindingsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListRepositoryFindingsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
kindFilter: jspb.Message.getFieldWithDefault(msg, 1, 0),
limit: jspb.Message.getFieldWithDefault(msg, 2, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ListRepositoryFindingsRequest}
 */
proto.repository.ListRepositoryFindingsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ListRepositoryFindingsRequest;
  return proto.repository.ListRepositoryFindingsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ListRepositoryFindingsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ListRepositoryFindingsRequest}
 */
proto.repository.ListRepositoryFindingsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.repository.RepositoryFindingKind} */ (reader.readEnum());
      msg.setKindFilter(value);
      break;
    case 2:
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
proto.repository.ListRepositoryFindingsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ListRepositoryFindingsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ListRepositoryFindingsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListRepositoryFindingsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getKindFilter();
  if (f !== 0.0) {
    writer.writeEnum(
      1,
      f
    );
  }
  f = message.getLimit();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
};


/**
 * optional RepositoryFindingKind kind_filter = 1;
 * @return {!proto.repository.RepositoryFindingKind}
 */
proto.repository.ListRepositoryFindingsRequest.prototype.getKindFilter = function() {
  return /** @type {!proto.repository.RepositoryFindingKind} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {!proto.repository.RepositoryFindingKind} value
 * @return {!proto.repository.ListRepositoryFindingsRequest} returns this
 */
proto.repository.ListRepositoryFindingsRequest.prototype.setKindFilter = function(value) {
  return jspb.Message.setProto3EnumField(this, 1, value);
};


/**
 * optional int32 limit = 2;
 * @return {number}
 */
proto.repository.ListRepositoryFindingsRequest.prototype.getLimit = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ListRepositoryFindingsRequest} returns this
 */
proto.repository.ListRepositoryFindingsRequest.prototype.setLimit = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.ListRepositoryFindingsResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.ListRepositoryFindingsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.ListRepositoryFindingsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.ListRepositoryFindingsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListRepositoryFindingsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
findingsList: jspb.Message.toObjectList(msg.getFindingsList(),
    proto.repository.RepositoryFinding.toObject, includeInstance),
generatedAtUnix: jspb.Message.getFieldWithDefault(msg, 2, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.ListRepositoryFindingsResponse}
 */
proto.repository.ListRepositoryFindingsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.ListRepositoryFindingsResponse;
  return proto.repository.ListRepositoryFindingsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.ListRepositoryFindingsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.ListRepositoryFindingsResponse}
 */
proto.repository.ListRepositoryFindingsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.repository.RepositoryFinding;
      reader.readMessage(value,proto.repository.RepositoryFinding.deserializeBinaryFromReader);
      msg.addFindings(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setGeneratedAtUnix(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.ListRepositoryFindingsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.ListRepositoryFindingsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.ListRepositoryFindingsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.ListRepositoryFindingsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getFindingsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.repository.RepositoryFinding.serializeBinaryToWriter
    );
  }
  f = message.getGeneratedAtUnix();
  if (f !== 0) {
    writer.writeInt64(
      2,
      f
    );
  }
};


/**
 * repeated RepositoryFinding findings = 1;
 * @return {!Array<!proto.repository.RepositoryFinding>}
 */
proto.repository.ListRepositoryFindingsResponse.prototype.getFindingsList = function() {
  return /** @type{!Array<!proto.repository.RepositoryFinding>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.repository.RepositoryFinding, 1));
};


/**
 * @param {!Array<!proto.repository.RepositoryFinding>} value
 * @return {!proto.repository.ListRepositoryFindingsResponse} returns this
*/
proto.repository.ListRepositoryFindingsResponse.prototype.setFindingsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.repository.RepositoryFinding=} opt_value
 * @param {number=} opt_index
 * @return {!proto.repository.RepositoryFinding}
 */
proto.repository.ListRepositoryFindingsResponse.prototype.addFindings = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.repository.RepositoryFinding, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.ListRepositoryFindingsResponse} returns this
 */
proto.repository.ListRepositoryFindingsResponse.prototype.clearFindingsList = function() {
  return this.setFindingsList([]);
};


/**
 * optional int64 generated_at_unix = 2;
 * @return {number}
 */
proto.repository.ListRepositoryFindingsResponse.prototype.getGeneratedAtUnix = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.ListRepositoryFindingsResponse} returns this
 */
proto.repository.ListRepositoryFindingsResponse.prototype.setGeneratedAtUnix = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.DependencyHealthProto.repeatedFields_ = [5];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.DependencyHealthProto.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.DependencyHealthProto.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.DependencyHealthProto} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.DependencyHealthProto.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, ""),
kind: jspb.Message.getFieldWithDefault(msg, 2, ""),
status: jspb.Message.getFieldWithDefault(msg, 3, ""),
reason: jspb.Message.getFieldWithDefault(msg, 4, ""),
affectsCapabilitiesList: (f = jspb.Message.getRepeatedField(msg, 5)) == null ? undefined : f
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.DependencyHealthProto}
 */
proto.repository.DependencyHealthProto.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.DependencyHealthProto;
  return proto.repository.DependencyHealthProto.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.DependencyHealthProto} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.DependencyHealthProto}
 */
proto.repository.DependencyHealthProto.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setKind(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setStatus(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setReason(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.addAffectsCapabilities(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.DependencyHealthProto.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.DependencyHealthProto.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.DependencyHealthProto} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.DependencyHealthProto.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
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
  f = message.getStatus();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getReason();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getAffectsCapabilitiesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      5,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.repository.DependencyHealthProto.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.DependencyHealthProto} returns this
 */
proto.repository.DependencyHealthProto.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string kind = 2;
 * @return {string}
 */
proto.repository.DependencyHealthProto.prototype.getKind = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.DependencyHealthProto} returns this
 */
proto.repository.DependencyHealthProto.prototype.setKind = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string status = 3;
 * @return {string}
 */
proto.repository.DependencyHealthProto.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.DependencyHealthProto} returns this
 */
proto.repository.DependencyHealthProto.prototype.setStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string reason = 4;
 * @return {string}
 */
proto.repository.DependencyHealthProto.prototype.getReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.DependencyHealthProto} returns this
 */
proto.repository.DependencyHealthProto.prototype.setReason = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * repeated string affects_capabilities = 5;
 * @return {!Array<string>}
 */
proto.repository.DependencyHealthProto.prototype.getAffectsCapabilitiesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 5));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.repository.DependencyHealthProto} returns this
 */
proto.repository.DependencyHealthProto.prototype.setAffectsCapabilitiesList = function(value) {
  return jspb.Message.setField(this, 5, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.repository.DependencyHealthProto} returns this
 */
proto.repository.DependencyHealthProto.prototype.addAffectsCapabilities = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 5, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.DependencyHealthProto} returns this
 */
proto.repository.DependencyHealthProto.prototype.clearAffectsCapabilitiesList = function() {
  return this.setAffectsCapabilitiesList([]);
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
proto.repository.CapabilityHealthProto.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.CapabilityHealthProto.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.CapabilityHealthProto} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.CapabilityHealthProto.toObject = function(includeInstance, msg) {
  var f, obj = {
name: jspb.Message.getFieldWithDefault(msg, 1, ""),
status: jspb.Message.getFieldWithDefault(msg, 2, ""),
mode: jspb.Message.getFieldWithDefault(msg, 3, ""),
reason: jspb.Message.getFieldWithDefault(msg, 4, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.CapabilityHealthProto}
 */
proto.repository.CapabilityHealthProto.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.CapabilityHealthProto;
  return proto.repository.CapabilityHealthProto.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.CapabilityHealthProto} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.CapabilityHealthProto}
 */
proto.repository.CapabilityHealthProto.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setStatus(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setMode(value);
      break;
    case 4:
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
proto.repository.CapabilityHealthProto.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.CapabilityHealthProto.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.CapabilityHealthProto} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.CapabilityHealthProto.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
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
  f = message.getMode();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getReason();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.repository.CapabilityHealthProto.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.CapabilityHealthProto} returns this
 */
proto.repository.CapabilityHealthProto.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string status = 2;
 * @return {string}
 */
proto.repository.CapabilityHealthProto.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.CapabilityHealthProto} returns this
 */
proto.repository.CapabilityHealthProto.prototype.setStatus = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string mode = 3;
 * @return {string}
 */
proto.repository.CapabilityHealthProto.prototype.getMode = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.CapabilityHealthProto} returns this
 */
proto.repository.CapabilityHealthProto.prototype.setMode = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string reason = 4;
 * @return {string}
 */
proto.repository.CapabilityHealthProto.prototype.getReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.CapabilityHealthProto} returns this
 */
proto.repository.CapabilityHealthProto.prototype.setReason = function(value) {
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
proto.repository.GetRepositoryStatusRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.GetRepositoryStatusRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.GetRepositoryStatusRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.GetRepositoryStatusRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.repository.GetRepositoryStatusRequest}
 */
proto.repository.GetRepositoryStatusRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.GetRepositoryStatusRequest;
  return proto.repository.GetRepositoryStatusRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.GetRepositoryStatusRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.GetRepositoryStatusRequest}
 */
proto.repository.GetRepositoryStatusRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.repository.GetRepositoryStatusRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.GetRepositoryStatusRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.GetRepositoryStatusRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.GetRepositoryStatusRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.repository.GetRepositoryStatusResponse.repeatedFields_ = [4,5];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.repository.GetRepositoryStatusResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.repository.GetRepositoryStatusResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.repository.GetRepositoryStatusResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.GetRepositoryStatusResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
service: jspb.Message.getFieldWithDefault(msg, 1, ""),
mode: jspb.Message.getFieldWithDefault(msg, 2, ""),
reason: jspb.Message.getFieldWithDefault(msg, 3, ""),
dependenciesList: jspb.Message.toObjectList(msg.getDependenciesList(),
    proto.repository.DependencyHealthProto.toObject, includeInstance),
capabilitiesList: jspb.Message.toObjectList(msg.getCapabilitiesList(),
    proto.repository.CapabilityHealthProto.toObject, includeInstance),
observedAtUnix: jspb.Message.getFieldWithDefault(msg, 6, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.repository.GetRepositoryStatusResponse}
 */
proto.repository.GetRepositoryStatusResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.repository.GetRepositoryStatusResponse;
  return proto.repository.GetRepositoryStatusResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.repository.GetRepositoryStatusResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.repository.GetRepositoryStatusResponse}
 */
proto.repository.GetRepositoryStatusResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setService(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setMode(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setReason(value);
      break;
    case 4:
      var value = new proto.repository.DependencyHealthProto;
      reader.readMessage(value,proto.repository.DependencyHealthProto.deserializeBinaryFromReader);
      msg.addDependencies(value);
      break;
    case 5:
      var value = new proto.repository.CapabilityHealthProto;
      reader.readMessage(value,proto.repository.CapabilityHealthProto.deserializeBinaryFromReader);
      msg.addCapabilities(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setObservedAtUnix(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.repository.GetRepositoryStatusResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.repository.GetRepositoryStatusResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.repository.GetRepositoryStatusResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.repository.GetRepositoryStatusResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getService();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getMode();
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
  f = message.getDependenciesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      4,
      f,
      proto.repository.DependencyHealthProto.serializeBinaryToWriter
    );
  }
  f = message.getCapabilitiesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      5,
      f,
      proto.repository.CapabilityHealthProto.serializeBinaryToWriter
    );
  }
  f = message.getObservedAtUnix();
  if (f !== 0) {
    writer.writeInt64(
      6,
      f
    );
  }
};


/**
 * optional string service = 1;
 * @return {string}
 */
proto.repository.GetRepositoryStatusResponse.prototype.getService = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.GetRepositoryStatusResponse} returns this
 */
proto.repository.GetRepositoryStatusResponse.prototype.setService = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string mode = 2;
 * @return {string}
 */
proto.repository.GetRepositoryStatusResponse.prototype.getMode = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.GetRepositoryStatusResponse} returns this
 */
proto.repository.GetRepositoryStatusResponse.prototype.setMode = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string reason = 3;
 * @return {string}
 */
proto.repository.GetRepositoryStatusResponse.prototype.getReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.repository.GetRepositoryStatusResponse} returns this
 */
proto.repository.GetRepositoryStatusResponse.prototype.setReason = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * repeated DependencyHealthProto dependencies = 4;
 * @return {!Array<!proto.repository.DependencyHealthProto>}
 */
proto.repository.GetRepositoryStatusResponse.prototype.getDependenciesList = function() {
  return /** @type{!Array<!proto.repository.DependencyHealthProto>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.repository.DependencyHealthProto, 4));
};


/**
 * @param {!Array<!proto.repository.DependencyHealthProto>} value
 * @return {!proto.repository.GetRepositoryStatusResponse} returns this
*/
proto.repository.GetRepositoryStatusResponse.prototype.setDependenciesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 4, value);
};


/**
 * @param {!proto.repository.DependencyHealthProto=} opt_value
 * @param {number=} opt_index
 * @return {!proto.repository.DependencyHealthProto}
 */
proto.repository.GetRepositoryStatusResponse.prototype.addDependencies = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 4, opt_value, proto.repository.DependencyHealthProto, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.GetRepositoryStatusResponse} returns this
 */
proto.repository.GetRepositoryStatusResponse.prototype.clearDependenciesList = function() {
  return this.setDependenciesList([]);
};


/**
 * repeated CapabilityHealthProto capabilities = 5;
 * @return {!Array<!proto.repository.CapabilityHealthProto>}
 */
proto.repository.GetRepositoryStatusResponse.prototype.getCapabilitiesList = function() {
  return /** @type{!Array<!proto.repository.CapabilityHealthProto>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.repository.CapabilityHealthProto, 5));
};


/**
 * @param {!Array<!proto.repository.CapabilityHealthProto>} value
 * @return {!proto.repository.GetRepositoryStatusResponse} returns this
*/
proto.repository.GetRepositoryStatusResponse.prototype.setCapabilitiesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 5, value);
};


/**
 * @param {!proto.repository.CapabilityHealthProto=} opt_value
 * @param {number=} opt_index
 * @return {!proto.repository.CapabilityHealthProto}
 */
proto.repository.GetRepositoryStatusResponse.prototype.addCapabilities = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 5, opt_value, proto.repository.CapabilityHealthProto, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.repository.GetRepositoryStatusResponse} returns this
 */
proto.repository.GetRepositoryStatusResponse.prototype.clearCapabilitiesList = function() {
  return this.setCapabilitiesList([]);
};


/**
 * optional int64 observed_at_unix = 6;
 * @return {number}
 */
proto.repository.GetRepositoryStatusResponse.prototype.getObservedAtUnix = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.repository.GetRepositoryStatusResponse} returns this
 */
proto.repository.GetRepositoryStatusResponse.prototype.setObservedAtUnix = function(value) {
  return jspb.Message.setProto3IntField(this, 6, value);
};


/**
 * @enum {number}
 */
proto.repository.ArtifactKind = {
  ARTIFACT_KIND_UNSPECIFIED: 0,
  SERVICE: 1,
  APPLICATION: 2,
  AGENT: 3,
  SUBSYSTEM: 4,
  INFRASTRUCTURE: 5,
  COMMAND: 6,
  AWARENESS_BUNDLE: 7
};

/**
 * @enum {number}
 */
proto.repository.ArtifactChannel = {
  CHANNEL_UNSET: 0,
  STABLE: 1,
  CANDIDATE: 2,
  CANARY: 3,
  DEV: 4,
  BOOTSTRAP: 5
};

/**
 * @enum {number}
 */
proto.repository.PublishState = {
  PUBLISH_STATE_UNSPECIFIED: 0,
  STAGING: 1,
  VERIFIED: 2,
  PUBLISHED: 3,
  FAILED: 4,
  ORPHANED: 5,
  DEPRECATED: 6,
  YANKED: 7,
  QUARANTINED: 8,
  REVOKED: 9,
  CORRUPTED: 10,
  ARCHIVED: 11
};

/**
 * @enum {number}
 */
proto.repository.ArtifactVerifyStatus = {
  ARTIFACT_VERIFY_STATUS_UNSPECIFIED: 0,
  ARTIFACT_VERIFY_OK: 1,
  ARTIFACT_VERIFY_BROKEN_MISSING_BLOB: 2,
  ARTIFACT_VERIFY_BROKEN_CHECKSUM_MISMATCH: 3,
  ARTIFACT_VERIFY_BROKEN_LEDGER_MISSING: 4,
  ARTIFACT_VERIFY_BROKEN_MANIFEST_MISSING: 5,
  ARTIFACT_VERIFY_BROKEN_SIGNATURE_MISSING: 6,
  ARTIFACT_VERIFY_BROKEN_SIGNATURE_INVALID: 7,
  ARTIFACT_VERIFY_BROKEN_PROVENANCE_INVALID: 8,
  ARTIFACT_VERIFY_QUARANTINED: 9,
  ARTIFACT_VERIFY_REVOKED: 10,
  ARTIFACT_VERIFY_INCONCLUSIVE: 11
};

/**
 * @enum {number}
 */
proto.repository.VersionIntent = {
  VERSION_INTENT_UNSPECIFIED: 0,
  BUMP_PATCH: 1,
  BUMP_MINOR: 2,
  BUMP_MAJOR: 3,
  EXACT: 4
};

/**
 * @enum {number}
 */
proto.repository.UpstreamSourceType = {
  UPSTREAM_TYPE_UNSPECIFIED: 0,
  GITHUB_RELEASE: 1,
  HTTP_INDEX: 2,
  GIT_INDEX: 3,
  LOCAL_DIR: 4
};

/**
 * @enum {number}
 */
proto.repository.UpstreamSyncStatus = {
  SYNC_IMPORTED: 0,
  SYNC_SKIPPED: 1,
  SYNC_REJECTED: 2,
  SYNC_FAILED: 3,
  SYNC_WOULD_IMPORT: 4,
  SYNC_WOULD_SKIP: 5,
  SYNC_WOULD_REJECT: 6,
  SYNC_WOULD_FAIL: 7
};

/**
 * @enum {number}
 */
proto.repository.MergeStrategy = {
  MERGE_STRATEGY_UNSPECIFIED: 0,
  MERGE_REPLACE: 1,
  MERGE_PRESERVE: 2,
  MERGE_THREE_WAY: 3,
  MERGE_TEMPLATE_RENDER: 4,
  MERGE_APPEND_ONLY: 5,
  MERGE_FAIL_ON_LOCAL_MODIFICATION: 6,
  MERGE_SECRET_EXTERNAL: 7
};

/**
 * @enum {number}
 */
proto.repository.ConfigKind = {
  CONFIG_KIND_UNSPECIFIED: 0,
  CONFIG_DEFAULT: 1,
  CONFIG_OPERATOR_OVERRIDE: 2,
  CONFIG_GENERATED: 3,
  CONFIG_SECRET: 4,
  CONFIG_RUNTIME_STATE: 5
};

/**
 * @enum {number}
 */
proto.repository.TrustState = {
  TRUST_STATE_UNSPECIFIED: 0,
  TRUST_TRUSTED: 1,
  TRUST_UNTRUSTED: 2,
  TRUST_REVOKED: 3,
  TRUST_EXPIRED: 4
};

/**
 * @enum {number}
 */
proto.repository.SignatureStatus = {
  SIGNATURE_STATUS_UNSPECIFIED: 0,
  SIGNATURE_OK: 1,
  SIGNATURE_MISSING: 2,
  SIGNATURE_INVALID: 3,
  SIGNATURE_UNTRUSTED_PUBLISHER: 4,
  SIGNATURE_REVOKED_KEY: 5,
  SIGNATURE_EXPIRED_KEY: 6,
  SIGNATURE_DIGEST_MISMATCH: 7,
  SIGNATURE_INCONCLUSIVE: 8
};

/**
 * @enum {number}
 */
proto.repository.ConfigReceiptAction = {
  CONFIG_RECEIPT_ACTION_UNSPECIFIED: 0,
  CONFIG_RECEIPT_PRESERVED: 1,
  CONFIG_RECEIPT_REPLACED: 2,
  CONFIG_RECEIPT_GENERATED: 3,
  CONFIG_RECEIPT_MERGED: 4,
  CONFIG_RECEIPT_CONFLICT: 5,
  CONFIG_RECEIPT_RESTORED: 6,
  CONFIG_RECEIPT_SKIPPED_SECRET: 7,
  CONFIG_RECEIPT_FAILED: 8
};

/**
 * @enum {number}
 */
proto.repository.RepositoryFindingKind = {
  REPOSITORY_FINDING_UNSPECIFIED: 0,
  REPO_FIND_PUBLISHED_MISSING_BLOB: 1,
  REPO_FIND_PUBLISHED_CHECKSUM_MISMATCH: 2,
  REPO_FIND_PUBLISHED_UNSIGNED_REQUIRED: 3,
  REPO_FIND_REVOKED_INSTALLABLE: 4,
  REPO_FIND_QUARANTINED_INSTALLABLE: 5,
  REPO_FIND_CONFIG_CONFLICT: 6,
  REPO_FIND_ROLLBACK_FAILED: 7,
  REPO_FIND_SCYLLA_DOWN_MODE_INCONSISTENT: 10,
  REPO_FIND_MINIO_BLOCKS_REPOSITORY: 11,
  REPO_FIND_SOURCE_CHAIN_UNAVAILABLE: 12,
  REPO_FIND_LOCAL_CACHE_CORRUPTION: 13
};

/**
 * @enum {number}
 */
proto.repository.RepositoryFindingSeverity = {
  REPO_FIND_SEVERITY_UNSPECIFIED: 0,
  REPO_FIND_INFO: 1,
  REPO_FIND_WARN: 2,
  REPO_FIND_ERROR: 3,
  REPO_FIND_CRITICAL: 4
};

goog.object.extend(exports, proto.repository);
