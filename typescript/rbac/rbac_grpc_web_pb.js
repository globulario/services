/**
 * @fileoverview gRPC-Web generated client stub for rbac
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');


var google_protobuf_struct_pb = require('google-protobuf/google/protobuf/struct_pb.js')
const proto = {};
proto.rbac = require('./rbac_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.rbac.RbacServiceClient =
    function(hostname, credentials, options) {
  if (!options) options = {};
  options.format = 'text';

  /**
   * @private @const {!grpc.web.GrpcWebClientBase} The client
   */
  this.client_ = new grpc.web.GrpcWebClientBase(options);

  /**
   * @private @const {string} The hostname
   */
  this.hostname_ = hostname;

};


/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.rbac.RbacServicePromiseClient =
    function(hostname, credentials, options) {
  if (!options) options = {};
  options.format = 'text';

  /**
   * @private @const {!grpc.web.GrpcWebClientBase} The client
   */
  this.client_ = new grpc.web.GrpcWebClientBase(options);

  /**
   * @private @const {string} The hostname
   */
  this.hostname_ = hostname;

};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.SetResourcePermissionsRqst,
 *   !proto.rbac.SetResourcePermissionsRqst>}
 */
const methodDescriptor_RbacService_SetResourcePermissions = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/SetResourcePermissions',
  grpc.web.MethodType.UNARY,
  proto.rbac.SetResourcePermissionsRqst,
  proto.rbac.SetResourcePermissionsRqst,
  /**
   * @param {!proto.rbac.SetResourcePermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.SetResourcePermissionsRqst.deserializeBinary
);


/**
 * @param {!proto.rbac.SetResourcePermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.SetResourcePermissionsRqst)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.SetResourcePermissionsRqst>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.setResourcePermissions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/SetResourcePermissions',
      request,
      metadata || {},
      methodDescriptor_RbacService_SetResourcePermissions,
      callback);
};


/**
 * @param {!proto.rbac.SetResourcePermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.SetResourcePermissionsRqst>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.setResourcePermissions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/SetResourcePermissions',
      request,
      metadata || {},
      methodDescriptor_RbacService_SetResourcePermissions);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.DeleteResourcePermissionsRqst,
 *   !proto.rbac.DeleteResourcePermissionsRqst>}
 */
const methodDescriptor_RbacService_DeleteResourcePermissions = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/DeleteResourcePermissions',
  grpc.web.MethodType.UNARY,
  proto.rbac.DeleteResourcePermissionsRqst,
  proto.rbac.DeleteResourcePermissionsRqst,
  /**
   * @param {!proto.rbac.DeleteResourcePermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.DeleteResourcePermissionsRqst.deserializeBinary
);


/**
 * @param {!proto.rbac.DeleteResourcePermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.DeleteResourcePermissionsRqst)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.DeleteResourcePermissionsRqst>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.deleteResourcePermissions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/DeleteResourcePermissions',
      request,
      metadata || {},
      methodDescriptor_RbacService_DeleteResourcePermissions,
      callback);
};


/**
 * @param {!proto.rbac.DeleteResourcePermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.DeleteResourcePermissionsRqst>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.deleteResourcePermissions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/DeleteResourcePermissions',
      request,
      metadata || {},
      methodDescriptor_RbacService_DeleteResourcePermissions);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.DeleteResourcePermissionRqst,
 *   !proto.rbac.DeleteResourcePermissionRqst>}
 */
const methodDescriptor_RbacService_DeleteResourcePermission = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/DeleteResourcePermission',
  grpc.web.MethodType.UNARY,
  proto.rbac.DeleteResourcePermissionRqst,
  proto.rbac.DeleteResourcePermissionRqst,
  /**
   * @param {!proto.rbac.DeleteResourcePermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.DeleteResourcePermissionRqst.deserializeBinary
);


/**
 * @param {!proto.rbac.DeleteResourcePermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.DeleteResourcePermissionRqst)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.DeleteResourcePermissionRqst>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.deleteResourcePermission =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/DeleteResourcePermission',
      request,
      metadata || {},
      methodDescriptor_RbacService_DeleteResourcePermission,
      callback);
};


/**
 * @param {!proto.rbac.DeleteResourcePermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.DeleteResourcePermissionRqst>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.deleteResourcePermission =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/DeleteResourcePermission',
      request,
      metadata || {},
      methodDescriptor_RbacService_DeleteResourcePermission);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.GetResourcePermissionRqst,
 *   !proto.rbac.GetResourcePermissionRsp>}
 */
const methodDescriptor_RbacService_GetResourcePermission = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/GetResourcePermission',
  grpc.web.MethodType.UNARY,
  proto.rbac.GetResourcePermissionRqst,
  proto.rbac.GetResourcePermissionRsp,
  /**
   * @param {!proto.rbac.GetResourcePermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.GetResourcePermissionRsp.deserializeBinary
);


/**
 * @param {!proto.rbac.GetResourcePermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.GetResourcePermissionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.GetResourcePermissionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.getResourcePermission =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/GetResourcePermission',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetResourcePermission,
      callback);
};


/**
 * @param {!proto.rbac.GetResourcePermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.GetResourcePermissionRsp>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.getResourcePermission =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/GetResourcePermission',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetResourcePermission);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.SetResourcePermissionRqst,
 *   !proto.rbac.SetResourcePermissionRsp>}
 */
const methodDescriptor_RbacService_SetResourcePermission = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/SetResourcePermission',
  grpc.web.MethodType.UNARY,
  proto.rbac.SetResourcePermissionRqst,
  proto.rbac.SetResourcePermissionRsp,
  /**
   * @param {!proto.rbac.SetResourcePermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.SetResourcePermissionRsp.deserializeBinary
);


/**
 * @param {!proto.rbac.SetResourcePermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.SetResourcePermissionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.SetResourcePermissionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.setResourcePermission =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/SetResourcePermission',
      request,
      metadata || {},
      methodDescriptor_RbacService_SetResourcePermission,
      callback);
};


/**
 * @param {!proto.rbac.SetResourcePermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.SetResourcePermissionRsp>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.setResourcePermission =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/SetResourcePermission',
      request,
      metadata || {},
      methodDescriptor_RbacService_SetResourcePermission);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.GetResourcePermissionsRqst,
 *   !proto.rbac.GetResourcePermissionsRsp>}
 */
const methodDescriptor_RbacService_GetResourcePermissions = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/GetResourcePermissions',
  grpc.web.MethodType.UNARY,
  proto.rbac.GetResourcePermissionsRqst,
  proto.rbac.GetResourcePermissionsRsp,
  /**
   * @param {!proto.rbac.GetResourcePermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.GetResourcePermissionsRsp.deserializeBinary
);


/**
 * @param {!proto.rbac.GetResourcePermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.GetResourcePermissionsRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.GetResourcePermissionsRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.getResourcePermissions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/GetResourcePermissions',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetResourcePermissions,
      callback);
};


/**
 * @param {!proto.rbac.GetResourcePermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.GetResourcePermissionsRsp>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.getResourcePermissions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/GetResourcePermissions',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetResourcePermissions);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.GetResourcePermissionsByResourceTypeRqst,
 *   !proto.rbac.GetResourcePermissionsByResourceTypeRsp>}
 */
const methodDescriptor_RbacService_GetResourcePermissionsByResourceType = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/GetResourcePermissionsByResourceType',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.rbac.GetResourcePermissionsByResourceTypeRqst,
  proto.rbac.GetResourcePermissionsByResourceTypeRsp,
  /**
   * @param {!proto.rbac.GetResourcePermissionsByResourceTypeRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.GetResourcePermissionsByResourceTypeRsp.deserializeBinary
);


/**
 * @param {!proto.rbac.GetResourcePermissionsByResourceTypeRqst} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.GetResourcePermissionsByResourceTypeRsp>}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.getResourcePermissionsByResourceType =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/rbac.RbacService/GetResourcePermissionsByResourceType',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetResourcePermissionsByResourceType);
};


/**
 * @param {!proto.rbac.GetResourcePermissionsByResourceTypeRqst} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.GetResourcePermissionsByResourceTypeRsp>}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServicePromiseClient.prototype.getResourcePermissionsByResourceType =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/rbac.RbacService/GetResourcePermissionsByResourceType',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetResourcePermissionsByResourceType);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.GetResourcePermissionsBySubjectRqst,
 *   !proto.rbac.GetResourcePermissionsBySubjectRsp>}
 */
const methodDescriptor_RbacService_GetResourcePermissionsBySubject = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/GetResourcePermissionsBySubject',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.rbac.GetResourcePermissionsBySubjectRqst,
  proto.rbac.GetResourcePermissionsBySubjectRsp,
  /**
   * @param {!proto.rbac.GetResourcePermissionsBySubjectRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.GetResourcePermissionsBySubjectRsp.deserializeBinary
);


/**
 * @param {!proto.rbac.GetResourcePermissionsBySubjectRqst} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.GetResourcePermissionsBySubjectRsp>}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.getResourcePermissionsBySubject =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/rbac.RbacService/GetResourcePermissionsBySubject',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetResourcePermissionsBySubject);
};


/**
 * @param {!proto.rbac.GetResourcePermissionsBySubjectRqst} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.GetResourcePermissionsBySubjectRsp>}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServicePromiseClient.prototype.getResourcePermissionsBySubject =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/rbac.RbacService/GetResourcePermissionsBySubject',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetResourcePermissionsBySubject);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.AddResourceOwnerRqst,
 *   !proto.rbac.AddResourceOwnerRsp>}
 */
const methodDescriptor_RbacService_AddResourceOwner = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/AddResourceOwner',
  grpc.web.MethodType.UNARY,
  proto.rbac.AddResourceOwnerRqst,
  proto.rbac.AddResourceOwnerRsp,
  /**
   * @param {!proto.rbac.AddResourceOwnerRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.AddResourceOwnerRsp.deserializeBinary
);


/**
 * @param {!proto.rbac.AddResourceOwnerRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.AddResourceOwnerRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.AddResourceOwnerRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.addResourceOwner =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/AddResourceOwner',
      request,
      metadata || {},
      methodDescriptor_RbacService_AddResourceOwner,
      callback);
};


/**
 * @param {!proto.rbac.AddResourceOwnerRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.AddResourceOwnerRsp>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.addResourceOwner =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/AddResourceOwner',
      request,
      metadata || {},
      methodDescriptor_RbacService_AddResourceOwner);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.RemoveResourceOwnerRqst,
 *   !proto.rbac.RemoveResourceOwnerRsp>}
 */
const methodDescriptor_RbacService_RemoveResourceOwner = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/RemoveResourceOwner',
  grpc.web.MethodType.UNARY,
  proto.rbac.RemoveResourceOwnerRqst,
  proto.rbac.RemoveResourceOwnerRsp,
  /**
   * @param {!proto.rbac.RemoveResourceOwnerRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.RemoveResourceOwnerRsp.deserializeBinary
);


/**
 * @param {!proto.rbac.RemoveResourceOwnerRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.RemoveResourceOwnerRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.RemoveResourceOwnerRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.removeResourceOwner =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/RemoveResourceOwner',
      request,
      metadata || {},
      methodDescriptor_RbacService_RemoveResourceOwner,
      callback);
};


/**
 * @param {!proto.rbac.RemoveResourceOwnerRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.RemoveResourceOwnerRsp>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.removeResourceOwner =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/RemoveResourceOwner',
      request,
      metadata || {},
      methodDescriptor_RbacService_RemoveResourceOwner);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.DeleteAllAccessRqst,
 *   !proto.rbac.DeleteAllAccessRsp>}
 */
const methodDescriptor_RbacService_DeleteAllAccess = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/DeleteAllAccess',
  grpc.web.MethodType.UNARY,
  proto.rbac.DeleteAllAccessRqst,
  proto.rbac.DeleteAllAccessRsp,
  /**
   * @param {!proto.rbac.DeleteAllAccessRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.DeleteAllAccessRsp.deserializeBinary
);


/**
 * @param {!proto.rbac.DeleteAllAccessRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.DeleteAllAccessRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.DeleteAllAccessRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.deleteAllAccess =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/DeleteAllAccess',
      request,
      metadata || {},
      methodDescriptor_RbacService_DeleteAllAccess,
      callback);
};


/**
 * @param {!proto.rbac.DeleteAllAccessRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.DeleteAllAccessRsp>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.deleteAllAccess =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/DeleteAllAccess',
      request,
      metadata || {},
      methodDescriptor_RbacService_DeleteAllAccess);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.ValidateAccessRqst,
 *   !proto.rbac.ValidateAccessRsp>}
 */
const methodDescriptor_RbacService_ValidateAccess = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/ValidateAccess',
  grpc.web.MethodType.UNARY,
  proto.rbac.ValidateAccessRqst,
  proto.rbac.ValidateAccessRsp,
  /**
   * @param {!proto.rbac.ValidateAccessRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.ValidateAccessRsp.deserializeBinary
);


/**
 * @param {!proto.rbac.ValidateAccessRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.ValidateAccessRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.ValidateAccessRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.validateAccess =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/ValidateAccess',
      request,
      metadata || {},
      methodDescriptor_RbacService_ValidateAccess,
      callback);
};


/**
 * @param {!proto.rbac.ValidateAccessRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.ValidateAccessRsp>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.validateAccess =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/ValidateAccess',
      request,
      metadata || {},
      methodDescriptor_RbacService_ValidateAccess);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.SetActionResourcesPermissionsRqst,
 *   !proto.rbac.SetActionResourcesPermissionsRsp>}
 */
const methodDescriptor_RbacService_SetActionResourcesPermissions = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/SetActionResourcesPermissions',
  grpc.web.MethodType.UNARY,
  proto.rbac.SetActionResourcesPermissionsRqst,
  proto.rbac.SetActionResourcesPermissionsRsp,
  /**
   * @param {!proto.rbac.SetActionResourcesPermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.SetActionResourcesPermissionsRsp.deserializeBinary
);


/**
 * @param {!proto.rbac.SetActionResourcesPermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.SetActionResourcesPermissionsRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.SetActionResourcesPermissionsRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.setActionResourcesPermissions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/SetActionResourcesPermissions',
      request,
      metadata || {},
      methodDescriptor_RbacService_SetActionResourcesPermissions,
      callback);
};


/**
 * @param {!proto.rbac.SetActionResourcesPermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.SetActionResourcesPermissionsRsp>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.setActionResourcesPermissions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/SetActionResourcesPermissions',
      request,
      metadata || {},
      methodDescriptor_RbacService_SetActionResourcesPermissions);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.GetActionResourceInfosRqst,
 *   !proto.rbac.GetActionResourceInfosRsp>}
 */
const methodDescriptor_RbacService_GetActionResourceInfos = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/GetActionResourceInfos',
  grpc.web.MethodType.UNARY,
  proto.rbac.GetActionResourceInfosRqst,
  proto.rbac.GetActionResourceInfosRsp,
  /**
   * @param {!proto.rbac.GetActionResourceInfosRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.GetActionResourceInfosRsp.deserializeBinary
);


/**
 * @param {!proto.rbac.GetActionResourceInfosRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.GetActionResourceInfosRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.GetActionResourceInfosRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.getActionResourceInfos =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/GetActionResourceInfos',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetActionResourceInfos,
      callback);
};


/**
 * @param {!proto.rbac.GetActionResourceInfosRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.GetActionResourceInfosRsp>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.getActionResourceInfos =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/GetActionResourceInfos',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetActionResourceInfos);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.ValidateActionRqst,
 *   !proto.rbac.ValidateActionRsp>}
 */
const methodDescriptor_RbacService_ValidateAction = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/ValidateAction',
  grpc.web.MethodType.UNARY,
  proto.rbac.ValidateActionRqst,
  proto.rbac.ValidateActionRsp,
  /**
   * @param {!proto.rbac.ValidateActionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.ValidateActionRsp.deserializeBinary
);


/**
 * @param {!proto.rbac.ValidateActionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.ValidateActionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.ValidateActionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.validateAction =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/ValidateAction',
      request,
      metadata || {},
      methodDescriptor_RbacService_ValidateAction,
      callback);
};


/**
 * @param {!proto.rbac.ValidateActionRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.ValidateActionRsp>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.validateAction =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/ValidateAction',
      request,
      metadata || {},
      methodDescriptor_RbacService_ValidateAction);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.ValidateSubjectSpaceRqst,
 *   !proto.rbac.ValidateSubjectSpaceRsp>}
 */
const methodDescriptor_RbacService_ValidateSubjectSpace = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/ValidateSubjectSpace',
  grpc.web.MethodType.UNARY,
  proto.rbac.ValidateSubjectSpaceRqst,
  proto.rbac.ValidateSubjectSpaceRsp,
  /**
   * @param {!proto.rbac.ValidateSubjectSpaceRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.ValidateSubjectSpaceRsp.deserializeBinary
);


/**
 * @param {!proto.rbac.ValidateSubjectSpaceRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.ValidateSubjectSpaceRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.ValidateSubjectSpaceRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.validateSubjectSpace =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/ValidateSubjectSpace',
      request,
      metadata || {},
      methodDescriptor_RbacService_ValidateSubjectSpace,
      callback);
};


/**
 * @param {!proto.rbac.ValidateSubjectSpaceRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.ValidateSubjectSpaceRsp>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.validateSubjectSpace =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/ValidateSubjectSpace',
      request,
      metadata || {},
      methodDescriptor_RbacService_ValidateSubjectSpace);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.GetSubjectAvailableSpaceRqst,
 *   !proto.rbac.GetSubjectAvailableSpaceRsp>}
 */
const methodDescriptor_RbacService_GetSubjectAvailableSpace = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/GetSubjectAvailableSpace',
  grpc.web.MethodType.UNARY,
  proto.rbac.GetSubjectAvailableSpaceRqst,
  proto.rbac.GetSubjectAvailableSpaceRsp,
  /**
   * @param {!proto.rbac.GetSubjectAvailableSpaceRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.GetSubjectAvailableSpaceRsp.deserializeBinary
);


/**
 * @param {!proto.rbac.GetSubjectAvailableSpaceRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.GetSubjectAvailableSpaceRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.GetSubjectAvailableSpaceRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.getSubjectAvailableSpace =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/GetSubjectAvailableSpace',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetSubjectAvailableSpace,
      callback);
};


/**
 * @param {!proto.rbac.GetSubjectAvailableSpaceRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.GetSubjectAvailableSpaceRsp>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.getSubjectAvailableSpace =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/GetSubjectAvailableSpace',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetSubjectAvailableSpace);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.GetSubjectAllocatedSpaceRqst,
 *   !proto.rbac.GetSubjectAllocatedSpaceRsp>}
 */
const methodDescriptor_RbacService_GetSubjectAllocatedSpace = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/GetSubjectAllocatedSpace',
  grpc.web.MethodType.UNARY,
  proto.rbac.GetSubjectAllocatedSpaceRqst,
  proto.rbac.GetSubjectAllocatedSpaceRsp,
  /**
   * @param {!proto.rbac.GetSubjectAllocatedSpaceRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.GetSubjectAllocatedSpaceRsp.deserializeBinary
);


/**
 * @param {!proto.rbac.GetSubjectAllocatedSpaceRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.GetSubjectAllocatedSpaceRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.GetSubjectAllocatedSpaceRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.getSubjectAllocatedSpace =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/GetSubjectAllocatedSpace',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetSubjectAllocatedSpace,
      callback);
};


/**
 * @param {!proto.rbac.GetSubjectAllocatedSpaceRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.GetSubjectAllocatedSpaceRsp>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.getSubjectAllocatedSpace =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/GetSubjectAllocatedSpace',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetSubjectAllocatedSpace);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.SetSubjectAllocatedSpaceRqst,
 *   !proto.rbac.SetSubjectAllocatedSpaceRsp>}
 */
const methodDescriptor_RbacService_SetSubjectAllocatedSpace = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/SetSubjectAllocatedSpace',
  grpc.web.MethodType.UNARY,
  proto.rbac.SetSubjectAllocatedSpaceRqst,
  proto.rbac.SetSubjectAllocatedSpaceRsp,
  /**
   * @param {!proto.rbac.SetSubjectAllocatedSpaceRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.SetSubjectAllocatedSpaceRsp.deserializeBinary
);


/**
 * @param {!proto.rbac.SetSubjectAllocatedSpaceRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.SetSubjectAllocatedSpaceRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.SetSubjectAllocatedSpaceRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.setSubjectAllocatedSpace =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/SetSubjectAllocatedSpace',
      request,
      metadata || {},
      methodDescriptor_RbacService_SetSubjectAllocatedSpace,
      callback);
};


/**
 * @param {!proto.rbac.SetSubjectAllocatedSpaceRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.SetSubjectAllocatedSpaceRsp>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.setSubjectAllocatedSpace =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/SetSubjectAllocatedSpace',
      request,
      metadata || {},
      methodDescriptor_RbacService_SetSubjectAllocatedSpace);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.ShareResourceRqst,
 *   !proto.rbac.ShareResourceRsp>}
 */
const methodDescriptor_RbacService_ShareResource = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/ShareResource',
  grpc.web.MethodType.UNARY,
  proto.rbac.ShareResourceRqst,
  proto.rbac.ShareResourceRsp,
  /**
   * @param {!proto.rbac.ShareResourceRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.ShareResourceRsp.deserializeBinary
);


/**
 * @param {!proto.rbac.ShareResourceRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.ShareResourceRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.ShareResourceRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.shareResource =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/ShareResource',
      request,
      metadata || {},
      methodDescriptor_RbacService_ShareResource,
      callback);
};


/**
 * @param {!proto.rbac.ShareResourceRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.ShareResourceRsp>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.shareResource =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/ShareResource',
      request,
      metadata || {},
      methodDescriptor_RbacService_ShareResource);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.UnshareResourceRqst,
 *   !proto.rbac.UnshareResourceRsp>}
 */
const methodDescriptor_RbacService_UshareResource = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/UshareResource',
  grpc.web.MethodType.UNARY,
  proto.rbac.UnshareResourceRqst,
  proto.rbac.UnshareResourceRsp,
  /**
   * @param {!proto.rbac.UnshareResourceRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.UnshareResourceRsp.deserializeBinary
);


/**
 * @param {!proto.rbac.UnshareResourceRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.UnshareResourceRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.UnshareResourceRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.ushareResource =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/UshareResource',
      request,
      metadata || {},
      methodDescriptor_RbacService_UshareResource,
      callback);
};


/**
 * @param {!proto.rbac.UnshareResourceRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.UnshareResourceRsp>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.ushareResource =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/UshareResource',
      request,
      metadata || {},
      methodDescriptor_RbacService_UshareResource);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.GetSharedResourceRqst,
 *   !proto.rbac.GetSharedResourceRsp>}
 */
const methodDescriptor_RbacService_GetSharedResource = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/GetSharedResource',
  grpc.web.MethodType.UNARY,
  proto.rbac.GetSharedResourceRqst,
  proto.rbac.GetSharedResourceRsp,
  /**
   * @param {!proto.rbac.GetSharedResourceRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.GetSharedResourceRsp.deserializeBinary
);


/**
 * @param {!proto.rbac.GetSharedResourceRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.GetSharedResourceRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.GetSharedResourceRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.getSharedResource =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/GetSharedResource',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetSharedResource,
      callback);
};


/**
 * @param {!proto.rbac.GetSharedResourceRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.GetSharedResourceRsp>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.getSharedResource =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/GetSharedResource',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetSharedResource);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.RemoveSubjectFromShareRqst,
 *   !proto.rbac.RemoveSubjectFromShareRsp>}
 */
const methodDescriptor_RbacService_RemoveSubjectFromShare = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/RemoveSubjectFromShare',
  grpc.web.MethodType.UNARY,
  proto.rbac.RemoveSubjectFromShareRqst,
  proto.rbac.RemoveSubjectFromShareRsp,
  /**
   * @param {!proto.rbac.RemoveSubjectFromShareRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.RemoveSubjectFromShareRsp.deserializeBinary
);


/**
 * @param {!proto.rbac.RemoveSubjectFromShareRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.RemoveSubjectFromShareRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.RemoveSubjectFromShareRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.removeSubjectFromShare =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/RemoveSubjectFromShare',
      request,
      metadata || {},
      methodDescriptor_RbacService_RemoveSubjectFromShare,
      callback);
};


/**
 * @param {!proto.rbac.RemoveSubjectFromShareRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.RemoveSubjectFromShareRsp>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.removeSubjectFromShare =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/RemoveSubjectFromShare',
      request,
      metadata || {},
      methodDescriptor_RbacService_RemoveSubjectFromShare);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.rbac.DeleteSubjectShareRqst,
 *   !proto.rbac.DeleteSubjectShareRsp>}
 */
const methodDescriptor_RbacService_DeleteSubjectShare = new grpc.web.MethodDescriptor(
  '/rbac.RbacService/DeleteSubjectShare',
  grpc.web.MethodType.UNARY,
  proto.rbac.DeleteSubjectShareRqst,
  proto.rbac.DeleteSubjectShareRsp,
  /**
   * @param {!proto.rbac.DeleteSubjectShareRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.rbac.DeleteSubjectShareRsp.deserializeBinary
);


/**
 * @param {!proto.rbac.DeleteSubjectShareRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.rbac.DeleteSubjectShareRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.rbac.DeleteSubjectShareRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.rbac.RbacServiceClient.prototype.deleteSubjectShare =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/rbac.RbacService/DeleteSubjectShare',
      request,
      metadata || {},
      methodDescriptor_RbacService_DeleteSubjectShare,
      callback);
};


/**
 * @param {!proto.rbac.DeleteSubjectShareRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.rbac.DeleteSubjectShareRsp>}
 *     Promise that resolves to the response
 */
proto.rbac.RbacServicePromiseClient.prototype.deleteSubjectShare =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/rbac.RbacService/DeleteSubjectShare',
      request,
      metadata || {},
      methodDescriptor_RbacService_DeleteSubjectShare);
};


module.exports = proto.rbac;

