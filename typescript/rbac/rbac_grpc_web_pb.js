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

const proto = {};
proto.rbac = require('./rbac_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.rbac.RbacServiceClient =
    function(hostname, credentials, options) {
  if (!options) options = {};
  options['format'] = 'text';

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
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.rbac.RbacServicePromiseClient =
    function(hostname, credentials, options) {
  if (!options) options = {};
  options['format'] = 'text';

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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.rbac.SetResourcePermissionsRqst,
 *   !proto.rbac.SetResourcePermissionsRqst>}
 */
const methodInfo_RbacService_SetResourcePermissions = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.rbac.SetResourcePermissionsRqst)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.rbac.DeleteResourcePermissionsRqst,
 *   !proto.rbac.DeleteResourcePermissionsRqst>}
 */
const methodInfo_RbacService_DeleteResourcePermissions = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.rbac.DeleteResourcePermissionsRqst)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.rbac.DeleteResourcePermissionRqst,
 *   !proto.rbac.DeleteResourcePermissionRqst>}
 */
const methodInfo_RbacService_DeleteResourcePermission = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.rbac.DeleteResourcePermissionRqst)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.rbac.GetResourcePermissionRqst,
 *   !proto.rbac.GetResourcePermissionRsp>}
 */
const methodInfo_RbacService_GetResourcePermission = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.rbac.GetResourcePermissionRsp)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.rbac.SetResourcePermissionRqst,
 *   !proto.rbac.SetResourcePermissionRsp>}
 */
const methodInfo_RbacService_SetResourcePermission = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.rbac.SetResourcePermissionRsp)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.rbac.GetResourcePermissionsRqst,
 *   !proto.rbac.GetResourcePermissionsRsp>}
 */
const methodInfo_RbacService_GetResourcePermissions = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.rbac.GetResourcePermissionsRsp)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.rbac.AddResourceOwnerRqst,
 *   !proto.rbac.AddResourceOwnerRsp>}
 */
const methodInfo_RbacService_AddResourceOwner = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.rbac.AddResourceOwnerRsp)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.rbac.RemoveResourceOwnerRqst,
 *   !proto.rbac.RemoveResourceOwnerRsp>}
 */
const methodInfo_RbacService_RemoveResourceOwner = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.rbac.RemoveResourceOwnerRsp)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.rbac.DeleteAllAccessRqst,
 *   !proto.rbac.DeleteAllAccessRsp>}
 */
const methodInfo_RbacService_DeleteAllAccess = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.rbac.DeleteAllAccessRsp)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.rbac.ValidateAccessRqst,
 *   !proto.rbac.ValidateAccessRsp>}
 */
const methodInfo_RbacService_ValidateAccess = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.rbac.ValidateAccessRsp)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.rbac.GetActionResourceInfosRqst,
 *   !proto.rbac.GetActionResourceInfosRsp>}
 */
const methodInfo_RbacService_GetActionResourceInfos = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.rbac.GetActionResourceInfosRsp)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.rbac.ValidateActionRqst,
 *   !proto.rbac.ValidateActionRsp>}
 */
const methodInfo_RbacService_ValidateAction = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.rbac.ValidateActionRsp)}
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
 * @param {?Object<string, string>} metadata User defined
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


module.exports = proto.rbac;

