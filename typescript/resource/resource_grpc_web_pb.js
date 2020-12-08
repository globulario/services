/**
 * @fileoverview gRPC-Web generated client stub for resource
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.resource = require('./resource_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.resource.ResourceServiceClient =
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
proto.resource.ResourceServicePromiseClient =
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
 *   !proto.resource.GetAllActionsRqst,
 *   !proto.resource.GetAllActionsRsp>}
 */
const methodDescriptor_ResourceService_GetAllActions = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetAllActions',
  grpc.web.MethodType.UNARY,
  proto.resource.GetAllActionsRqst,
  proto.resource.GetAllActionsRsp,
  /**
   * @param {!proto.resource.GetAllActionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetAllActionsRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetAllActionsRqst,
 *   !proto.resource.GetAllActionsRsp>}
 */
const methodInfo_ResourceService_GetAllActions = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetAllActionsRsp,
  /**
   * @param {!proto.resource.GetAllActionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetAllActionsRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetAllActionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.GetAllActionsRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetAllActionsRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getAllActions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/GetAllActions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetAllActions,
      callback);
};


/**
 * @param {!proto.resource.GetAllActionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.GetAllActionsRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.getAllActions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/GetAllActions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetAllActions);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.ValidateTokenRqst,
 *   !proto.resource.ValidateTokenRsp>}
 */
const methodDescriptor_ResourceService_ValidateToken = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/ValidateToken',
  grpc.web.MethodType.UNARY,
  proto.resource.ValidateTokenRqst,
  proto.resource.ValidateTokenRsp,
  /**
   * @param {!proto.resource.ValidateTokenRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ValidateTokenRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.ValidateTokenRqst,
 *   !proto.resource.ValidateTokenRsp>}
 */
const methodInfo_ResourceService_ValidateToken = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.ValidateTokenRsp,
  /**
   * @param {!proto.resource.ValidateTokenRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ValidateTokenRsp.deserializeBinary
);


/**
 * @param {!proto.resource.ValidateTokenRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.ValidateTokenRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.ValidateTokenRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.validateToken =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/ValidateToken',
      request,
      metadata || {},
      methodDescriptor_ResourceService_ValidateToken,
      callback);
};


/**
 * @param {!proto.resource.ValidateTokenRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.ValidateTokenRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.validateToken =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/ValidateToken',
      request,
      metadata || {},
      methodDescriptor_ResourceService_ValidateToken);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.RefreshTokenRqst,
 *   !proto.resource.RefreshTokenRsp>}
 */
const methodDescriptor_ResourceService_RefreshToken = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/RefreshToken',
  grpc.web.MethodType.UNARY,
  proto.resource.RefreshTokenRqst,
  proto.resource.RefreshTokenRsp,
  /**
   * @param {!proto.resource.RefreshTokenRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RefreshTokenRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.RefreshTokenRqst,
 *   !proto.resource.RefreshTokenRsp>}
 */
const methodInfo_ResourceService_RefreshToken = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.RefreshTokenRsp,
  /**
   * @param {!proto.resource.RefreshTokenRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RefreshTokenRsp.deserializeBinary
);


/**
 * @param {!proto.resource.RefreshTokenRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.RefreshTokenRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.RefreshTokenRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.refreshToken =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/RefreshToken',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RefreshToken,
      callback);
};


/**
 * @param {!proto.resource.RefreshTokenRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.RefreshTokenRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.refreshToken =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/RefreshToken',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RefreshToken);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.AuthenticateRqst,
 *   !proto.resource.AuthenticateRsp>}
 */
const methodDescriptor_ResourceService_Authenticate = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/Authenticate',
  grpc.web.MethodType.UNARY,
  proto.resource.AuthenticateRqst,
  proto.resource.AuthenticateRsp,
  /**
   * @param {!proto.resource.AuthenticateRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AuthenticateRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.AuthenticateRqst,
 *   !proto.resource.AuthenticateRsp>}
 */
const methodInfo_ResourceService_Authenticate = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.AuthenticateRsp,
  /**
   * @param {!proto.resource.AuthenticateRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AuthenticateRsp.deserializeBinary
);


/**
 * @param {!proto.resource.AuthenticateRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.AuthenticateRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.AuthenticateRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.authenticate =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/Authenticate',
      request,
      metadata || {},
      methodDescriptor_ResourceService_Authenticate,
      callback);
};


/**
 * @param {!proto.resource.AuthenticateRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.AuthenticateRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.authenticate =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/Authenticate',
      request,
      metadata || {},
      methodDescriptor_ResourceService_Authenticate);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.SynchronizeLdapRqst,
 *   !proto.resource.SynchronizeLdapRsp>}
 */
const methodDescriptor_ResourceService_SynchronizeLdap = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/SynchronizeLdap',
  grpc.web.MethodType.UNARY,
  proto.resource.SynchronizeLdapRqst,
  proto.resource.SynchronizeLdapRsp,
  /**
   * @param {!proto.resource.SynchronizeLdapRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SynchronizeLdapRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.SynchronizeLdapRqst,
 *   !proto.resource.SynchronizeLdapRsp>}
 */
const methodInfo_ResourceService_SynchronizeLdap = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.SynchronizeLdapRsp,
  /**
   * @param {!proto.resource.SynchronizeLdapRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SynchronizeLdapRsp.deserializeBinary
);


/**
 * @param {!proto.resource.SynchronizeLdapRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.SynchronizeLdapRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.SynchronizeLdapRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.synchronizeLdap =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/SynchronizeLdap',
      request,
      metadata || {},
      methodDescriptor_ResourceService_SynchronizeLdap,
      callback);
};


/**
 * @param {!proto.resource.SynchronizeLdapRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.SynchronizeLdapRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.synchronizeLdap =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/SynchronizeLdap',
      request,
      metadata || {},
      methodDescriptor_ResourceService_SynchronizeLdap);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.CreateOrganizationRqst,
 *   !proto.resource.CreateOrganizationRsp>}
 */
const methodDescriptor_ResourceService_CreateOrganization = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/CreateOrganization',
  grpc.web.MethodType.UNARY,
  proto.resource.CreateOrganizationRqst,
  proto.resource.CreateOrganizationRsp,
  /**
   * @param {!proto.resource.CreateOrganizationRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.CreateOrganizationRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.CreateOrganizationRqst,
 *   !proto.resource.CreateOrganizationRsp>}
 */
const methodInfo_ResourceService_CreateOrganization = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.CreateOrganizationRsp,
  /**
   * @param {!proto.resource.CreateOrganizationRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.CreateOrganizationRsp.deserializeBinary
);


/**
 * @param {!proto.resource.CreateOrganizationRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.CreateOrganizationRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.CreateOrganizationRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.createOrganization =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/CreateOrganization',
      request,
      metadata || {},
      methodDescriptor_ResourceService_CreateOrganization,
      callback);
};


/**
 * @param {!proto.resource.CreateOrganizationRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.CreateOrganizationRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.createOrganization =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/CreateOrganization',
      request,
      metadata || {},
      methodDescriptor_ResourceService_CreateOrganization);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetOrganizationsRqst,
 *   !proto.resource.GetOrganizationsRsp>}
 */
const methodDescriptor_ResourceService_GetOrganizations = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetOrganizations',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.resource.GetOrganizationsRqst,
  proto.resource.GetOrganizationsRsp,
  /**
   * @param {!proto.resource.GetOrganizationsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetOrganizationsRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetOrganizationsRqst,
 *   !proto.resource.GetOrganizationsRsp>}
 */
const methodInfo_ResourceService_GetOrganizations = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetOrganizationsRsp,
  /**
   * @param {!proto.resource.GetOrganizationsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetOrganizationsRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetOrganizationsRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetOrganizationsRsp>}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getOrganizations =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.ResourceService/GetOrganizations',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetOrganizations);
};


/**
 * @param {!proto.resource.GetOrganizationsRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetOrganizationsRsp>}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServicePromiseClient.prototype.getOrganizations =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.ResourceService/GetOrganizations',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetOrganizations);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.DeleteOrganizationRqst,
 *   !proto.resource.DeleteOrganizationRsp>}
 */
const methodDescriptor_ResourceService_DeleteOrganization = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/DeleteOrganization',
  grpc.web.MethodType.UNARY,
  proto.resource.DeleteOrganizationRqst,
  proto.resource.DeleteOrganizationRsp,
  /**
   * @param {!proto.resource.DeleteOrganizationRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteOrganizationRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.DeleteOrganizationRqst,
 *   !proto.resource.DeleteOrganizationRsp>}
 */
const methodInfo_ResourceService_DeleteOrganization = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.DeleteOrganizationRsp,
  /**
   * @param {!proto.resource.DeleteOrganizationRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteOrganizationRsp.deserializeBinary
);


/**
 * @param {!proto.resource.DeleteOrganizationRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.DeleteOrganizationRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.DeleteOrganizationRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.deleteOrganization =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/DeleteOrganization',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteOrganization,
      callback);
};


/**
 * @param {!proto.resource.DeleteOrganizationRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.DeleteOrganizationRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.deleteOrganization =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/DeleteOrganization',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteOrganization);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.AddOrganizationAccountRqst,
 *   !proto.resource.AddOrganizationAccountRsp>}
 */
const methodDescriptor_ResourceService_AddOrganizationAccount = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/AddOrganizationAccount',
  grpc.web.MethodType.UNARY,
  proto.resource.AddOrganizationAccountRqst,
  proto.resource.AddOrganizationAccountRsp,
  /**
   * @param {!proto.resource.AddOrganizationAccountRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddOrganizationAccountRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.AddOrganizationAccountRqst,
 *   !proto.resource.AddOrganizationAccountRsp>}
 */
const methodInfo_ResourceService_AddOrganizationAccount = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.AddOrganizationAccountRsp,
  /**
   * @param {!proto.resource.AddOrganizationAccountRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddOrganizationAccountRsp.deserializeBinary
);


/**
 * @param {!proto.resource.AddOrganizationAccountRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.AddOrganizationAccountRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.AddOrganizationAccountRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.addOrganizationAccount =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/AddOrganizationAccount',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddOrganizationAccount,
      callback);
};


/**
 * @param {!proto.resource.AddOrganizationAccountRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.AddOrganizationAccountRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.addOrganizationAccount =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/AddOrganizationAccount',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddOrganizationAccount);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.AddOrganizationGroupRqst,
 *   !proto.resource.AddOrganizationGroupRsp>}
 */
const methodDescriptor_ResourceService_AddOrganizationGroup = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/AddOrganizationGroup',
  grpc.web.MethodType.UNARY,
  proto.resource.AddOrganizationGroupRqst,
  proto.resource.AddOrganizationGroupRsp,
  /**
   * @param {!proto.resource.AddOrganizationGroupRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddOrganizationGroupRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.AddOrganizationGroupRqst,
 *   !proto.resource.AddOrganizationGroupRsp>}
 */
const methodInfo_ResourceService_AddOrganizationGroup = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.AddOrganizationGroupRsp,
  /**
   * @param {!proto.resource.AddOrganizationGroupRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddOrganizationGroupRsp.deserializeBinary
);


/**
 * @param {!proto.resource.AddOrganizationGroupRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.AddOrganizationGroupRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.AddOrganizationGroupRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.addOrganizationGroup =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/AddOrganizationGroup',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddOrganizationGroup,
      callback);
};


/**
 * @param {!proto.resource.AddOrganizationGroupRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.AddOrganizationGroupRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.addOrganizationGroup =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/AddOrganizationGroup',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddOrganizationGroup);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.AddOrganizationRoleRqst,
 *   !proto.resource.AddOrganizationRoleRsp>}
 */
const methodDescriptor_ResourceService_AddOrganizationRole = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/AddOrganizationRole',
  grpc.web.MethodType.UNARY,
  proto.resource.AddOrganizationRoleRqst,
  proto.resource.AddOrganizationRoleRsp,
  /**
   * @param {!proto.resource.AddOrganizationRoleRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddOrganizationRoleRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.AddOrganizationRoleRqst,
 *   !proto.resource.AddOrganizationRoleRsp>}
 */
const methodInfo_ResourceService_AddOrganizationRole = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.AddOrganizationRoleRsp,
  /**
   * @param {!proto.resource.AddOrganizationRoleRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddOrganizationRoleRsp.deserializeBinary
);


/**
 * @param {!proto.resource.AddOrganizationRoleRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.AddOrganizationRoleRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.AddOrganizationRoleRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.addOrganizationRole =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/AddOrganizationRole',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddOrganizationRole,
      callback);
};


/**
 * @param {!proto.resource.AddOrganizationRoleRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.AddOrganizationRoleRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.addOrganizationRole =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/AddOrganizationRole',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddOrganizationRole);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.AddOrganizationApplicationRqst,
 *   !proto.resource.AddOrganizationApplicationRsp>}
 */
const methodDescriptor_ResourceService_AddOrganizationApplication = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/AddOrganizationApplication',
  grpc.web.MethodType.UNARY,
  proto.resource.AddOrganizationApplicationRqst,
  proto.resource.AddOrganizationApplicationRsp,
  /**
   * @param {!proto.resource.AddOrganizationApplicationRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddOrganizationApplicationRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.AddOrganizationApplicationRqst,
 *   !proto.resource.AddOrganizationApplicationRsp>}
 */
const methodInfo_ResourceService_AddOrganizationApplication = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.AddOrganizationApplicationRsp,
  /**
   * @param {!proto.resource.AddOrganizationApplicationRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddOrganizationApplicationRsp.deserializeBinary
);


/**
 * @param {!proto.resource.AddOrganizationApplicationRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.AddOrganizationApplicationRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.AddOrganizationApplicationRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.addOrganizationApplication =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/AddOrganizationApplication',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddOrganizationApplication,
      callback);
};


/**
 * @param {!proto.resource.AddOrganizationApplicationRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.AddOrganizationApplicationRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.addOrganizationApplication =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/AddOrganizationApplication',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddOrganizationApplication);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.RemoveOrganizationAccountRqst,
 *   !proto.resource.RemoveOrganizationAccountRsp>}
 */
const methodDescriptor_ResourceService_RemoveOrganizationAccount = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/RemoveOrganizationAccount',
  grpc.web.MethodType.UNARY,
  proto.resource.RemoveOrganizationAccountRqst,
  proto.resource.RemoveOrganizationAccountRsp,
  /**
   * @param {!proto.resource.RemoveOrganizationAccountRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveOrganizationAccountRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.RemoveOrganizationAccountRqst,
 *   !proto.resource.RemoveOrganizationAccountRsp>}
 */
const methodInfo_ResourceService_RemoveOrganizationAccount = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.RemoveOrganizationAccountRsp,
  /**
   * @param {!proto.resource.RemoveOrganizationAccountRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveOrganizationAccountRsp.deserializeBinary
);


/**
 * @param {!proto.resource.RemoveOrganizationAccountRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.RemoveOrganizationAccountRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.RemoveOrganizationAccountRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.removeOrganizationAccount =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/RemoveOrganizationAccount',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveOrganizationAccount,
      callback);
};


/**
 * @param {!proto.resource.RemoveOrganizationAccountRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.RemoveOrganizationAccountRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.removeOrganizationAccount =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/RemoveOrganizationAccount',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveOrganizationAccount);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.RemoveOrganizationGroupRqst,
 *   !proto.resource.RemoveOrganizationGroupRsp>}
 */
const methodDescriptor_ResourceService_RemoveOrganizationGroup = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/RemoveOrganizationGroup',
  grpc.web.MethodType.UNARY,
  proto.resource.RemoveOrganizationGroupRqst,
  proto.resource.RemoveOrganizationGroupRsp,
  /**
   * @param {!proto.resource.RemoveOrganizationGroupRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveOrganizationGroupRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.RemoveOrganizationGroupRqst,
 *   !proto.resource.RemoveOrganizationGroupRsp>}
 */
const methodInfo_ResourceService_RemoveOrganizationGroup = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.RemoveOrganizationGroupRsp,
  /**
   * @param {!proto.resource.RemoveOrganizationGroupRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveOrganizationGroupRsp.deserializeBinary
);


/**
 * @param {!proto.resource.RemoveOrganizationGroupRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.RemoveOrganizationGroupRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.RemoveOrganizationGroupRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.removeOrganizationGroup =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/RemoveOrganizationGroup',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveOrganizationGroup,
      callback);
};


/**
 * @param {!proto.resource.RemoveOrganizationGroupRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.RemoveOrganizationGroupRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.removeOrganizationGroup =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/RemoveOrganizationGroup',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveOrganizationGroup);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.RemoveOrganizationRoleRqst,
 *   !proto.resource.RemoveOrganizationRoleRsp>}
 */
const methodDescriptor_ResourceService_RemoveOrganizationRole = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/RemoveOrganizationRole',
  grpc.web.MethodType.UNARY,
  proto.resource.RemoveOrganizationRoleRqst,
  proto.resource.RemoveOrganizationRoleRsp,
  /**
   * @param {!proto.resource.RemoveOrganizationRoleRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveOrganizationRoleRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.RemoveOrganizationRoleRqst,
 *   !proto.resource.RemoveOrganizationRoleRsp>}
 */
const methodInfo_ResourceService_RemoveOrganizationRole = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.RemoveOrganizationRoleRsp,
  /**
   * @param {!proto.resource.RemoveOrganizationRoleRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveOrganizationRoleRsp.deserializeBinary
);


/**
 * @param {!proto.resource.RemoveOrganizationRoleRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.RemoveOrganizationRoleRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.RemoveOrganizationRoleRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.removeOrganizationRole =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/RemoveOrganizationRole',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveOrganizationRole,
      callback);
};


/**
 * @param {!proto.resource.RemoveOrganizationRoleRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.RemoveOrganizationRoleRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.removeOrganizationRole =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/RemoveOrganizationRole',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveOrganizationRole);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.RemoveOrganizationApplicationRqst,
 *   !proto.resource.RemoveOrganizationApplicationRsp>}
 */
const methodDescriptor_ResourceService_RemoveOrganizationApplication = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/RemoveOrganizationApplication',
  grpc.web.MethodType.UNARY,
  proto.resource.RemoveOrganizationApplicationRqst,
  proto.resource.RemoveOrganizationApplicationRsp,
  /**
   * @param {!proto.resource.RemoveOrganizationApplicationRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveOrganizationApplicationRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.RemoveOrganizationApplicationRqst,
 *   !proto.resource.RemoveOrganizationApplicationRsp>}
 */
const methodInfo_ResourceService_RemoveOrganizationApplication = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.RemoveOrganizationApplicationRsp,
  /**
   * @param {!proto.resource.RemoveOrganizationApplicationRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveOrganizationApplicationRsp.deserializeBinary
);


/**
 * @param {!proto.resource.RemoveOrganizationApplicationRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.RemoveOrganizationApplicationRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.RemoveOrganizationApplicationRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.removeOrganizationApplication =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/RemoveOrganizationApplication',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveOrganizationApplication,
      callback);
};


/**
 * @param {!proto.resource.RemoveOrganizationApplicationRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.RemoveOrganizationApplicationRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.removeOrganizationApplication =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/RemoveOrganizationApplication',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveOrganizationApplication);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.CreateGroupRqst,
 *   !proto.resource.CreateGroupRsp>}
 */
const methodDescriptor_ResourceService_CreateGroup = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/CreateGroup',
  grpc.web.MethodType.UNARY,
  proto.resource.CreateGroupRqst,
  proto.resource.CreateGroupRsp,
  /**
   * @param {!proto.resource.CreateGroupRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.CreateGroupRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.CreateGroupRqst,
 *   !proto.resource.CreateGroupRsp>}
 */
const methodInfo_ResourceService_CreateGroup = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.CreateGroupRsp,
  /**
   * @param {!proto.resource.CreateGroupRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.CreateGroupRsp.deserializeBinary
);


/**
 * @param {!proto.resource.CreateGroupRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.CreateGroupRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.CreateGroupRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.createGroup =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/CreateGroup',
      request,
      metadata || {},
      methodDescriptor_ResourceService_CreateGroup,
      callback);
};


/**
 * @param {!proto.resource.CreateGroupRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.CreateGroupRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.createGroup =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/CreateGroup',
      request,
      metadata || {},
      methodDescriptor_ResourceService_CreateGroup);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetGroupsRqst,
 *   !proto.resource.GetGroupsRsp>}
 */
const methodDescriptor_ResourceService_GetGroups = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetGroups',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.resource.GetGroupsRqst,
  proto.resource.GetGroupsRsp,
  /**
   * @param {!proto.resource.GetGroupsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetGroupsRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetGroupsRqst,
 *   !proto.resource.GetGroupsRsp>}
 */
const methodInfo_ResourceService_GetGroups = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetGroupsRsp,
  /**
   * @param {!proto.resource.GetGroupsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetGroupsRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetGroupsRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetGroupsRsp>}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getGroups =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.ResourceService/GetGroups',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetGroups);
};


/**
 * @param {!proto.resource.GetGroupsRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetGroupsRsp>}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServicePromiseClient.prototype.getGroups =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.ResourceService/GetGroups',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetGroups);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.DeleteGroupRqst,
 *   !proto.resource.DeleteGroupRsp>}
 */
const methodDescriptor_ResourceService_DeleteGroup = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/DeleteGroup',
  grpc.web.MethodType.UNARY,
  proto.resource.DeleteGroupRqst,
  proto.resource.DeleteGroupRsp,
  /**
   * @param {!proto.resource.DeleteGroupRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteGroupRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.DeleteGroupRqst,
 *   !proto.resource.DeleteGroupRsp>}
 */
const methodInfo_ResourceService_DeleteGroup = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.DeleteGroupRsp,
  /**
   * @param {!proto.resource.DeleteGroupRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteGroupRsp.deserializeBinary
);


/**
 * @param {!proto.resource.DeleteGroupRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.DeleteGroupRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.DeleteGroupRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.deleteGroup =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/DeleteGroup',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteGroup,
      callback);
};


/**
 * @param {!proto.resource.DeleteGroupRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.DeleteGroupRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.deleteGroup =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/DeleteGroup',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteGroup);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.AddGroupMemberAccountRqst,
 *   !proto.resource.AddGroupMemberAccountRsp>}
 */
const methodDescriptor_ResourceService_AddGroupMemberAccount = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/AddGroupMemberAccount',
  grpc.web.MethodType.UNARY,
  proto.resource.AddGroupMemberAccountRqst,
  proto.resource.AddGroupMemberAccountRsp,
  /**
   * @param {!proto.resource.AddGroupMemberAccountRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddGroupMemberAccountRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.AddGroupMemberAccountRqst,
 *   !proto.resource.AddGroupMemberAccountRsp>}
 */
const methodInfo_ResourceService_AddGroupMemberAccount = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.AddGroupMemberAccountRsp,
  /**
   * @param {!proto.resource.AddGroupMemberAccountRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddGroupMemberAccountRsp.deserializeBinary
);


/**
 * @param {!proto.resource.AddGroupMemberAccountRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.AddGroupMemberAccountRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.AddGroupMemberAccountRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.addGroupMemberAccount =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/AddGroupMemberAccount',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddGroupMemberAccount,
      callback);
};


/**
 * @param {!proto.resource.AddGroupMemberAccountRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.AddGroupMemberAccountRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.addGroupMemberAccount =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/AddGroupMemberAccount',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddGroupMemberAccount);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.RemoveGroupMemberAccountRqst,
 *   !proto.resource.RemoveGroupMemberAccountRsp>}
 */
const methodDescriptor_ResourceService_RemoveGroupMemberAccount = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/RemoveGroupMemberAccount',
  grpc.web.MethodType.UNARY,
  proto.resource.RemoveGroupMemberAccountRqst,
  proto.resource.RemoveGroupMemberAccountRsp,
  /**
   * @param {!proto.resource.RemoveGroupMemberAccountRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveGroupMemberAccountRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.RemoveGroupMemberAccountRqst,
 *   !proto.resource.RemoveGroupMemberAccountRsp>}
 */
const methodInfo_ResourceService_RemoveGroupMemberAccount = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.RemoveGroupMemberAccountRsp,
  /**
   * @param {!proto.resource.RemoveGroupMemberAccountRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveGroupMemberAccountRsp.deserializeBinary
);


/**
 * @param {!proto.resource.RemoveGroupMemberAccountRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.RemoveGroupMemberAccountRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.RemoveGroupMemberAccountRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.removeGroupMemberAccount =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/RemoveGroupMemberAccount',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveGroupMemberAccount,
      callback);
};


/**
 * @param {!proto.resource.RemoveGroupMemberAccountRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.RemoveGroupMemberAccountRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.removeGroupMemberAccount =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/RemoveGroupMemberAccount',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveGroupMemberAccount);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.RegisterAccountRqst,
 *   !proto.resource.RegisterAccountRsp>}
 */
const methodDescriptor_ResourceService_RegisterAccount = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/RegisterAccount',
  grpc.web.MethodType.UNARY,
  proto.resource.RegisterAccountRqst,
  proto.resource.RegisterAccountRsp,
  /**
   * @param {!proto.resource.RegisterAccountRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RegisterAccountRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.RegisterAccountRqst,
 *   !proto.resource.RegisterAccountRsp>}
 */
const methodInfo_ResourceService_RegisterAccount = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.RegisterAccountRsp,
  /**
   * @param {!proto.resource.RegisterAccountRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RegisterAccountRsp.deserializeBinary
);


/**
 * @param {!proto.resource.RegisterAccountRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.RegisterAccountRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.RegisterAccountRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.registerAccount =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/RegisterAccount',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RegisterAccount,
      callback);
};


/**
 * @param {!proto.resource.RegisterAccountRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.RegisterAccountRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.registerAccount =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/RegisterAccount',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RegisterAccount);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.DeleteAccountRqst,
 *   !proto.resource.DeleteAccountRsp>}
 */
const methodDescriptor_ResourceService_DeleteAccount = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/DeleteAccount',
  grpc.web.MethodType.UNARY,
  proto.resource.DeleteAccountRqst,
  proto.resource.DeleteAccountRsp,
  /**
   * @param {!proto.resource.DeleteAccountRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteAccountRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.DeleteAccountRqst,
 *   !proto.resource.DeleteAccountRsp>}
 */
const methodInfo_ResourceService_DeleteAccount = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.DeleteAccountRsp,
  /**
   * @param {!proto.resource.DeleteAccountRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteAccountRsp.deserializeBinary
);


/**
 * @param {!proto.resource.DeleteAccountRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.DeleteAccountRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.DeleteAccountRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.deleteAccount =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/DeleteAccount',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteAccount,
      callback);
};


/**
 * @param {!proto.resource.DeleteAccountRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.DeleteAccountRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.deleteAccount =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/DeleteAccount',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteAccount);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.AddAccountRoleRqst,
 *   !proto.resource.AddAccountRoleRsp>}
 */
const methodDescriptor_ResourceService_AddAccountRole = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/AddAccountRole',
  grpc.web.MethodType.UNARY,
  proto.resource.AddAccountRoleRqst,
  proto.resource.AddAccountRoleRsp,
  /**
   * @param {!proto.resource.AddAccountRoleRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddAccountRoleRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.AddAccountRoleRqst,
 *   !proto.resource.AddAccountRoleRsp>}
 */
const methodInfo_ResourceService_AddAccountRole = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.AddAccountRoleRsp,
  /**
   * @param {!proto.resource.AddAccountRoleRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddAccountRoleRsp.deserializeBinary
);


/**
 * @param {!proto.resource.AddAccountRoleRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.AddAccountRoleRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.AddAccountRoleRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.addAccountRole =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/AddAccountRole',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddAccountRole,
      callback);
};


/**
 * @param {!proto.resource.AddAccountRoleRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.AddAccountRoleRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.addAccountRole =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/AddAccountRole',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddAccountRole);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.RemoveAccountRoleRqst,
 *   !proto.resource.RemoveAccountRoleRsp>}
 */
const methodDescriptor_ResourceService_RemoveAccountRole = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/RemoveAccountRole',
  grpc.web.MethodType.UNARY,
  proto.resource.RemoveAccountRoleRqst,
  proto.resource.RemoveAccountRoleRsp,
  /**
   * @param {!proto.resource.RemoveAccountRoleRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveAccountRoleRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.RemoveAccountRoleRqst,
 *   !proto.resource.RemoveAccountRoleRsp>}
 */
const methodInfo_ResourceService_RemoveAccountRole = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.RemoveAccountRoleRsp,
  /**
   * @param {!proto.resource.RemoveAccountRoleRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveAccountRoleRsp.deserializeBinary
);


/**
 * @param {!proto.resource.RemoveAccountRoleRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.RemoveAccountRoleRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.RemoveAccountRoleRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.removeAccountRole =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/RemoveAccountRole',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveAccountRole,
      callback);
};


/**
 * @param {!proto.resource.RemoveAccountRoleRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.RemoveAccountRoleRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.removeAccountRole =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/RemoveAccountRole',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveAccountRole);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.CreateRoleRqst,
 *   !proto.resource.CreateRoleRsp>}
 */
const methodDescriptor_ResourceService_CreateRole = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/CreateRole',
  grpc.web.MethodType.UNARY,
  proto.resource.CreateRoleRqst,
  proto.resource.CreateRoleRsp,
  /**
   * @param {!proto.resource.CreateRoleRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.CreateRoleRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.CreateRoleRqst,
 *   !proto.resource.CreateRoleRsp>}
 */
const methodInfo_ResourceService_CreateRole = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.CreateRoleRsp,
  /**
   * @param {!proto.resource.CreateRoleRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.CreateRoleRsp.deserializeBinary
);


/**
 * @param {!proto.resource.CreateRoleRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.CreateRoleRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.CreateRoleRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.createRole =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/CreateRole',
      request,
      metadata || {},
      methodDescriptor_ResourceService_CreateRole,
      callback);
};


/**
 * @param {!proto.resource.CreateRoleRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.CreateRoleRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.createRole =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/CreateRole',
      request,
      metadata || {},
      methodDescriptor_ResourceService_CreateRole);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.DeleteRoleRqst,
 *   !proto.resource.DeleteRoleRsp>}
 */
const methodDescriptor_ResourceService_DeleteRole = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/DeleteRole',
  grpc.web.MethodType.UNARY,
  proto.resource.DeleteRoleRqst,
  proto.resource.DeleteRoleRsp,
  /**
   * @param {!proto.resource.DeleteRoleRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteRoleRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.DeleteRoleRqst,
 *   !proto.resource.DeleteRoleRsp>}
 */
const methodInfo_ResourceService_DeleteRole = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.DeleteRoleRsp,
  /**
   * @param {!proto.resource.DeleteRoleRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteRoleRsp.deserializeBinary
);


/**
 * @param {!proto.resource.DeleteRoleRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.DeleteRoleRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.DeleteRoleRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.deleteRole =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/DeleteRole',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteRole,
      callback);
};


/**
 * @param {!proto.resource.DeleteRoleRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.DeleteRoleRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.deleteRole =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/DeleteRole',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteRole);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.AddRoleActionRqst,
 *   !proto.resource.AddRoleActionRsp>}
 */
const methodDescriptor_ResourceService_AddRoleAction = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/AddRoleAction',
  grpc.web.MethodType.UNARY,
  proto.resource.AddRoleActionRqst,
  proto.resource.AddRoleActionRsp,
  /**
   * @param {!proto.resource.AddRoleActionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddRoleActionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.AddRoleActionRqst,
 *   !proto.resource.AddRoleActionRsp>}
 */
const methodInfo_ResourceService_AddRoleAction = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.AddRoleActionRsp,
  /**
   * @param {!proto.resource.AddRoleActionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddRoleActionRsp.deserializeBinary
);


/**
 * @param {!proto.resource.AddRoleActionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.AddRoleActionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.AddRoleActionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.addRoleAction =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/AddRoleAction',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddRoleAction,
      callback);
};


/**
 * @param {!proto.resource.AddRoleActionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.AddRoleActionRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.addRoleAction =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/AddRoleAction',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddRoleAction);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.RemoveRoleActionRqst,
 *   !proto.resource.RemoveRoleActionRsp>}
 */
const methodDescriptor_ResourceService_RemoveRoleAction = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/RemoveRoleAction',
  grpc.web.MethodType.UNARY,
  proto.resource.RemoveRoleActionRqst,
  proto.resource.RemoveRoleActionRsp,
  /**
   * @param {!proto.resource.RemoveRoleActionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveRoleActionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.RemoveRoleActionRqst,
 *   !proto.resource.RemoveRoleActionRsp>}
 */
const methodInfo_ResourceService_RemoveRoleAction = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.RemoveRoleActionRsp,
  /**
   * @param {!proto.resource.RemoveRoleActionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveRoleActionRsp.deserializeBinary
);


/**
 * @param {!proto.resource.RemoveRoleActionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.RemoveRoleActionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.RemoveRoleActionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.removeRoleAction =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/RemoveRoleAction',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveRoleAction,
      callback);
};


/**
 * @param {!proto.resource.RemoveRoleActionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.RemoveRoleActionRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.removeRoleAction =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/RemoveRoleAction',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveRoleAction);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetAllApplicationsInfoRqst,
 *   !proto.resource.GetAllApplicationsInfoRsp>}
 */
const methodDescriptor_ResourceService_GetAllApplicationsInfo = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetAllApplicationsInfo',
  grpc.web.MethodType.UNARY,
  proto.resource.GetAllApplicationsInfoRqst,
  proto.resource.GetAllApplicationsInfoRsp,
  /**
   * @param {!proto.resource.GetAllApplicationsInfoRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetAllApplicationsInfoRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetAllApplicationsInfoRqst,
 *   !proto.resource.GetAllApplicationsInfoRsp>}
 */
const methodInfo_ResourceService_GetAllApplicationsInfo = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetAllApplicationsInfoRsp,
  /**
   * @param {!proto.resource.GetAllApplicationsInfoRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetAllApplicationsInfoRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetAllApplicationsInfoRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.GetAllApplicationsInfoRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetAllApplicationsInfoRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getAllApplicationsInfo =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/GetAllApplicationsInfo',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetAllApplicationsInfo,
      callback);
};


/**
 * @param {!proto.resource.GetAllApplicationsInfoRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.GetAllApplicationsInfoRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.getAllApplicationsInfo =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/GetAllApplicationsInfo',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetAllApplicationsInfo);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.DeleteApplicationRqst,
 *   !proto.resource.DeleteApplicationRsp>}
 */
const methodDescriptor_ResourceService_DeleteApplication = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/DeleteApplication',
  grpc.web.MethodType.UNARY,
  proto.resource.DeleteApplicationRqst,
  proto.resource.DeleteApplicationRsp,
  /**
   * @param {!proto.resource.DeleteApplicationRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteApplicationRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.DeleteApplicationRqst,
 *   !proto.resource.DeleteApplicationRsp>}
 */
const methodInfo_ResourceService_DeleteApplication = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.DeleteApplicationRsp,
  /**
   * @param {!proto.resource.DeleteApplicationRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteApplicationRsp.deserializeBinary
);


/**
 * @param {!proto.resource.DeleteApplicationRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.DeleteApplicationRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.DeleteApplicationRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.deleteApplication =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/DeleteApplication',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteApplication,
      callback);
};


/**
 * @param {!proto.resource.DeleteApplicationRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.DeleteApplicationRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.deleteApplication =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/DeleteApplication',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteApplication);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.AddApplicationActionRqst,
 *   !proto.resource.AddApplicationActionRsp>}
 */
const methodDescriptor_ResourceService_AddApplicationAction = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/AddApplicationAction',
  grpc.web.MethodType.UNARY,
  proto.resource.AddApplicationActionRqst,
  proto.resource.AddApplicationActionRsp,
  /**
   * @param {!proto.resource.AddApplicationActionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddApplicationActionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.AddApplicationActionRqst,
 *   !proto.resource.AddApplicationActionRsp>}
 */
const methodInfo_ResourceService_AddApplicationAction = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.AddApplicationActionRsp,
  /**
   * @param {!proto.resource.AddApplicationActionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddApplicationActionRsp.deserializeBinary
);


/**
 * @param {!proto.resource.AddApplicationActionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.AddApplicationActionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.AddApplicationActionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.addApplicationAction =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/AddApplicationAction',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddApplicationAction,
      callback);
};


/**
 * @param {!proto.resource.AddApplicationActionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.AddApplicationActionRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.addApplicationAction =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/AddApplicationAction',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddApplicationAction);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.RemoveApplicationActionRqst,
 *   !proto.resource.RemoveApplicationActionRsp>}
 */
const methodDescriptor_ResourceService_RemoveApplicationAction = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/RemoveApplicationAction',
  grpc.web.MethodType.UNARY,
  proto.resource.RemoveApplicationActionRqst,
  proto.resource.RemoveApplicationActionRsp,
  /**
   * @param {!proto.resource.RemoveApplicationActionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveApplicationActionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.RemoveApplicationActionRqst,
 *   !proto.resource.RemoveApplicationActionRsp>}
 */
const methodInfo_ResourceService_RemoveApplicationAction = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.RemoveApplicationActionRsp,
  /**
   * @param {!proto.resource.RemoveApplicationActionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveApplicationActionRsp.deserializeBinary
);


/**
 * @param {!proto.resource.RemoveApplicationActionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.RemoveApplicationActionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.RemoveApplicationActionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.removeApplicationAction =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/RemoveApplicationAction',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveApplicationAction,
      callback);
};


/**
 * @param {!proto.resource.RemoveApplicationActionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.RemoveApplicationActionRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.removeApplicationAction =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/RemoveApplicationAction',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveApplicationAction);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.RegisterPeerRqst,
 *   !proto.resource.RegisterPeerRsp>}
 */
const methodDescriptor_ResourceService_RegisterPeer = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/RegisterPeer',
  grpc.web.MethodType.UNARY,
  proto.resource.RegisterPeerRqst,
  proto.resource.RegisterPeerRsp,
  /**
   * @param {!proto.resource.RegisterPeerRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RegisterPeerRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.RegisterPeerRqst,
 *   !proto.resource.RegisterPeerRsp>}
 */
const methodInfo_ResourceService_RegisterPeer = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.RegisterPeerRsp,
  /**
   * @param {!proto.resource.RegisterPeerRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RegisterPeerRsp.deserializeBinary
);


/**
 * @param {!proto.resource.RegisterPeerRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.RegisterPeerRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.RegisterPeerRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.registerPeer =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/RegisterPeer',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RegisterPeer,
      callback);
};


/**
 * @param {!proto.resource.RegisterPeerRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.RegisterPeerRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.registerPeer =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/RegisterPeer',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RegisterPeer);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetPeersRqst,
 *   !proto.resource.GetPeersRsp>}
 */
const methodDescriptor_ResourceService_GetPeers = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetPeers',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.resource.GetPeersRqst,
  proto.resource.GetPeersRsp,
  /**
   * @param {!proto.resource.GetPeersRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetPeersRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetPeersRqst,
 *   !proto.resource.GetPeersRsp>}
 */
const methodInfo_ResourceService_GetPeers = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetPeersRsp,
  /**
   * @param {!proto.resource.GetPeersRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetPeersRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetPeersRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetPeersRsp>}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getPeers =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.ResourceService/GetPeers',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetPeers);
};


/**
 * @param {!proto.resource.GetPeersRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetPeersRsp>}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServicePromiseClient.prototype.getPeers =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.ResourceService/GetPeers',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetPeers);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.DeletePeerRqst,
 *   !proto.resource.DeletePeerRsp>}
 */
const methodDescriptor_ResourceService_DeletePeer = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/DeletePeer',
  grpc.web.MethodType.UNARY,
  proto.resource.DeletePeerRqst,
  proto.resource.DeletePeerRsp,
  /**
   * @param {!proto.resource.DeletePeerRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeletePeerRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.DeletePeerRqst,
 *   !proto.resource.DeletePeerRsp>}
 */
const methodInfo_ResourceService_DeletePeer = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.DeletePeerRsp,
  /**
   * @param {!proto.resource.DeletePeerRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeletePeerRsp.deserializeBinary
);


/**
 * @param {!proto.resource.DeletePeerRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.DeletePeerRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.DeletePeerRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.deletePeer =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/DeletePeer',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeletePeer,
      callback);
};


/**
 * @param {!proto.resource.DeletePeerRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.DeletePeerRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.deletePeer =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/DeletePeer',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeletePeer);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.AddPeerActionRqst,
 *   !proto.resource.AddPeerActionRsp>}
 */
const methodDescriptor_ResourceService_AddPeerAction = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/AddPeerAction',
  grpc.web.MethodType.UNARY,
  proto.resource.AddPeerActionRqst,
  proto.resource.AddPeerActionRsp,
  /**
   * @param {!proto.resource.AddPeerActionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddPeerActionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.AddPeerActionRqst,
 *   !proto.resource.AddPeerActionRsp>}
 */
const methodInfo_ResourceService_AddPeerAction = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.AddPeerActionRsp,
  /**
   * @param {!proto.resource.AddPeerActionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddPeerActionRsp.deserializeBinary
);


/**
 * @param {!proto.resource.AddPeerActionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.AddPeerActionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.AddPeerActionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.addPeerAction =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/AddPeerAction',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddPeerAction,
      callback);
};


/**
 * @param {!proto.resource.AddPeerActionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.AddPeerActionRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.addPeerAction =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/AddPeerAction',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddPeerAction);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.RemovePeerActionRqst,
 *   !proto.resource.RemovePeerActionRsp>}
 */
const methodDescriptor_ResourceService_RemovePeerAction = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/RemovePeerAction',
  grpc.web.MethodType.UNARY,
  proto.resource.RemovePeerActionRqst,
  proto.resource.RemovePeerActionRsp,
  /**
   * @param {!proto.resource.RemovePeerActionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemovePeerActionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.RemovePeerActionRqst,
 *   !proto.resource.RemovePeerActionRsp>}
 */
const methodInfo_ResourceService_RemovePeerAction = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.RemovePeerActionRsp,
  /**
   * @param {!proto.resource.RemovePeerActionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemovePeerActionRsp.deserializeBinary
);


/**
 * @param {!proto.resource.RemovePeerActionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.RemovePeerActionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.RemovePeerActionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.removePeerAction =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/RemovePeerAction',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemovePeerAction,
      callback);
};


/**
 * @param {!proto.resource.RemovePeerActionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.RemovePeerActionRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.removePeerAction =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/RemovePeerAction',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemovePeerAction);
};


/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.resource.RbacServiceClient =
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
proto.resource.RbacServicePromiseClient =
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
 *   !proto.resource.SetActionResourcesPermissionRqst,
 *   !proto.resource.SetActionResourcesPermissionRsp>}
 */
const methodDescriptor_RbacService_SetActionResourcesPermission = new grpc.web.MethodDescriptor(
  '/resource.RbacService/SetActionResourcesPermission',
  grpc.web.MethodType.UNARY,
  proto.resource.SetActionResourcesPermissionRqst,
  proto.resource.SetActionResourcesPermissionRsp,
  /**
   * @param {!proto.resource.SetActionResourcesPermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetActionResourcesPermissionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.SetActionResourcesPermissionRqst,
 *   !proto.resource.SetActionResourcesPermissionRsp>}
 */
const methodInfo_RbacService_SetActionResourcesPermission = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.SetActionResourcesPermissionRsp,
  /**
   * @param {!proto.resource.SetActionResourcesPermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetActionResourcesPermissionRsp.deserializeBinary
);


/**
 * @param {!proto.resource.SetActionResourcesPermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.SetActionResourcesPermissionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.SetActionResourcesPermissionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.RbacServiceClient.prototype.setActionResourcesPermission =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.RbacService/SetActionResourcesPermission',
      request,
      metadata || {},
      methodDescriptor_RbacService_SetActionResourcesPermission,
      callback);
};


/**
 * @param {!proto.resource.SetActionResourcesPermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.SetActionResourcesPermissionRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.RbacServicePromiseClient.prototype.setActionResourcesPermission =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.RbacService/SetActionResourcesPermission',
      request,
      metadata || {},
      methodDescriptor_RbacService_SetActionResourcesPermission);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetActionResourcesPermissionRqst,
 *   !proto.resource.GetActionResourcesPermissionRsp>}
 */
const methodDescriptor_RbacService_GetActionResourcesPermission = new grpc.web.MethodDescriptor(
  '/resource.RbacService/GetActionResourcesPermission',
  grpc.web.MethodType.UNARY,
  proto.resource.GetActionResourcesPermissionRqst,
  proto.resource.GetActionResourcesPermissionRsp,
  /**
   * @param {!proto.resource.GetActionResourcesPermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetActionResourcesPermissionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetActionResourcesPermissionRqst,
 *   !proto.resource.GetActionResourcesPermissionRsp>}
 */
const methodInfo_RbacService_GetActionResourcesPermission = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetActionResourcesPermissionRsp,
  /**
   * @param {!proto.resource.GetActionResourcesPermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetActionResourcesPermissionRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetActionResourcesPermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.GetActionResourcesPermissionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetActionResourcesPermissionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.RbacServiceClient.prototype.getActionResourcesPermission =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.RbacService/GetActionResourcesPermission',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetActionResourcesPermission,
      callback);
};


/**
 * @param {!proto.resource.GetActionResourcesPermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.GetActionResourcesPermissionRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.RbacServicePromiseClient.prototype.getActionResourcesPermission =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.RbacService/GetActionResourcesPermission',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetActionResourcesPermission);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.SetResourcePermissionsRqst,
 *   !proto.resource.SetResourcePermissionsRqst>}
 */
const methodDescriptor_RbacService_SetResourcePermissions = new grpc.web.MethodDescriptor(
  '/resource.RbacService/SetResourcePermissions',
  grpc.web.MethodType.UNARY,
  proto.resource.SetResourcePermissionsRqst,
  proto.resource.SetResourcePermissionsRqst,
  /**
   * @param {!proto.resource.SetResourcePermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetResourcePermissionsRqst.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.SetResourcePermissionsRqst,
 *   !proto.resource.SetResourcePermissionsRqst>}
 */
const methodInfo_RbacService_SetResourcePermissions = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.SetResourcePermissionsRqst,
  /**
   * @param {!proto.resource.SetResourcePermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetResourcePermissionsRqst.deserializeBinary
);


/**
 * @param {!proto.resource.SetResourcePermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.SetResourcePermissionsRqst)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.SetResourcePermissionsRqst>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.RbacServiceClient.prototype.setResourcePermissions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.RbacService/SetResourcePermissions',
      request,
      metadata || {},
      methodDescriptor_RbacService_SetResourcePermissions,
      callback);
};


/**
 * @param {!proto.resource.SetResourcePermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.SetResourcePermissionsRqst>}
 *     A native promise that resolves to the response
 */
proto.resource.RbacServicePromiseClient.prototype.setResourcePermissions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.RbacService/SetResourcePermissions',
      request,
      metadata || {},
      methodDescriptor_RbacService_SetResourcePermissions);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.DeleteResourcePermissionsRqst,
 *   !proto.resource.DeleteResourcePermissionsRqst>}
 */
const methodDescriptor_RbacService_DeleteResourcePermissions = new grpc.web.MethodDescriptor(
  '/resource.RbacService/DeleteResourcePermissions',
  grpc.web.MethodType.UNARY,
  proto.resource.DeleteResourcePermissionsRqst,
  proto.resource.DeleteResourcePermissionsRqst,
  /**
   * @param {!proto.resource.DeleteResourcePermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteResourcePermissionsRqst.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.DeleteResourcePermissionsRqst,
 *   !proto.resource.DeleteResourcePermissionsRqst>}
 */
const methodInfo_RbacService_DeleteResourcePermissions = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.DeleteResourcePermissionsRqst,
  /**
   * @param {!proto.resource.DeleteResourcePermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteResourcePermissionsRqst.deserializeBinary
);


/**
 * @param {!proto.resource.DeleteResourcePermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.DeleteResourcePermissionsRqst)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.DeleteResourcePermissionsRqst>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.RbacServiceClient.prototype.deleteResourcePermissions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.RbacService/DeleteResourcePermissions',
      request,
      metadata || {},
      methodDescriptor_RbacService_DeleteResourcePermissions,
      callback);
};


/**
 * @param {!proto.resource.DeleteResourcePermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.DeleteResourcePermissionsRqst>}
 *     A native promise that resolves to the response
 */
proto.resource.RbacServicePromiseClient.prototype.deleteResourcePermissions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.RbacService/DeleteResourcePermissions',
      request,
      metadata || {},
      methodDescriptor_RbacService_DeleteResourcePermissions);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.DeleteResourcePermissionRqst,
 *   !proto.resource.DeleteResourcePermissionRqst>}
 */
const methodDescriptor_RbacService_DeleteResourcePermission = new grpc.web.MethodDescriptor(
  '/resource.RbacService/DeleteResourcePermission',
  grpc.web.MethodType.UNARY,
  proto.resource.DeleteResourcePermissionRqst,
  proto.resource.DeleteResourcePermissionRqst,
  /**
   * @param {!proto.resource.DeleteResourcePermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteResourcePermissionRqst.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.DeleteResourcePermissionRqst,
 *   !proto.resource.DeleteResourcePermissionRqst>}
 */
const methodInfo_RbacService_DeleteResourcePermission = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.DeleteResourcePermissionRqst,
  /**
   * @param {!proto.resource.DeleteResourcePermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteResourcePermissionRqst.deserializeBinary
);


/**
 * @param {!proto.resource.DeleteResourcePermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.DeleteResourcePermissionRqst)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.DeleteResourcePermissionRqst>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.RbacServiceClient.prototype.deleteResourcePermission =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.RbacService/DeleteResourcePermission',
      request,
      metadata || {},
      methodDescriptor_RbacService_DeleteResourcePermission,
      callback);
};


/**
 * @param {!proto.resource.DeleteResourcePermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.DeleteResourcePermissionRqst>}
 *     A native promise that resolves to the response
 */
proto.resource.RbacServicePromiseClient.prototype.deleteResourcePermission =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.RbacService/DeleteResourcePermission',
      request,
      metadata || {},
      methodDescriptor_RbacService_DeleteResourcePermission);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.SetResourcePermissionRqst,
 *   !proto.resource.SetResourcePermissionRsp>}
 */
const methodDescriptor_RbacService_SetResourcePermission = new grpc.web.MethodDescriptor(
  '/resource.RbacService/SetResourcePermission',
  grpc.web.MethodType.UNARY,
  proto.resource.SetResourcePermissionRqst,
  proto.resource.SetResourcePermissionRsp,
  /**
   * @param {!proto.resource.SetResourcePermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetResourcePermissionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.SetResourcePermissionRqst,
 *   !proto.resource.SetResourcePermissionRsp>}
 */
const methodInfo_RbacService_SetResourcePermission = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.SetResourcePermissionRsp,
  /**
   * @param {!proto.resource.SetResourcePermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetResourcePermissionRsp.deserializeBinary
);


/**
 * @param {!proto.resource.SetResourcePermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.SetResourcePermissionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.SetResourcePermissionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.RbacServiceClient.prototype.setResourcePermission =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.RbacService/SetResourcePermission',
      request,
      metadata || {},
      methodDescriptor_RbacService_SetResourcePermission,
      callback);
};


/**
 * @param {!proto.resource.SetResourcePermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.SetResourcePermissionRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.RbacServicePromiseClient.prototype.setResourcePermission =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.RbacService/SetResourcePermission',
      request,
      metadata || {},
      methodDescriptor_RbacService_SetResourcePermission);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetResourcePermissionRqst,
 *   !proto.resource.GetResourcePermissionRsp>}
 */
const methodDescriptor_RbacService_GetResourcePermission = new grpc.web.MethodDescriptor(
  '/resource.RbacService/GetResourcePermission',
  grpc.web.MethodType.UNARY,
  proto.resource.GetResourcePermissionRqst,
  proto.resource.GetResourcePermissionRsp,
  /**
   * @param {!proto.resource.GetResourcePermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetResourcePermissionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetResourcePermissionRqst,
 *   !proto.resource.GetResourcePermissionRsp>}
 */
const methodInfo_RbacService_GetResourcePermission = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetResourcePermissionRsp,
  /**
   * @param {!proto.resource.GetResourcePermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetResourcePermissionRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetResourcePermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.GetResourcePermissionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetResourcePermissionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.RbacServiceClient.prototype.getResourcePermission =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.RbacService/GetResourcePermission',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetResourcePermission,
      callback);
};


/**
 * @param {!proto.resource.GetResourcePermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.GetResourcePermissionRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.RbacServicePromiseClient.prototype.getResourcePermission =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.RbacService/GetResourcePermission',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetResourcePermission);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetResourcePermissionsRqst,
 *   !proto.resource.GetResourcePermissionsRsp>}
 */
const methodDescriptor_RbacService_GetResourcePermissions = new grpc.web.MethodDescriptor(
  '/resource.RbacService/GetResourcePermissions',
  grpc.web.MethodType.UNARY,
  proto.resource.GetResourcePermissionsRqst,
  proto.resource.GetResourcePermissionsRsp,
  /**
   * @param {!proto.resource.GetResourcePermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetResourcePermissionsRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetResourcePermissionsRqst,
 *   !proto.resource.GetResourcePermissionsRsp>}
 */
const methodInfo_RbacService_GetResourcePermissions = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetResourcePermissionsRsp,
  /**
   * @param {!proto.resource.GetResourcePermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetResourcePermissionsRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetResourcePermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.GetResourcePermissionsRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetResourcePermissionsRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.RbacServiceClient.prototype.getResourcePermissions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.RbacService/GetResourcePermissions',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetResourcePermissions,
      callback);
};


/**
 * @param {!proto.resource.GetResourcePermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.GetResourcePermissionsRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.RbacServicePromiseClient.prototype.getResourcePermissions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.RbacService/GetResourcePermissions',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetResourcePermissions);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.AddResourceOwnerRqst,
 *   !proto.resource.AddResourceOwnerRsp>}
 */
const methodDescriptor_RbacService_AddResourceOwner = new grpc.web.MethodDescriptor(
  '/resource.RbacService/AddResourceOwner',
  grpc.web.MethodType.UNARY,
  proto.resource.AddResourceOwnerRqst,
  proto.resource.AddResourceOwnerRsp,
  /**
   * @param {!proto.resource.AddResourceOwnerRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddResourceOwnerRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.AddResourceOwnerRqst,
 *   !proto.resource.AddResourceOwnerRsp>}
 */
const methodInfo_RbacService_AddResourceOwner = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.AddResourceOwnerRsp,
  /**
   * @param {!proto.resource.AddResourceOwnerRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddResourceOwnerRsp.deserializeBinary
);


/**
 * @param {!proto.resource.AddResourceOwnerRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.AddResourceOwnerRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.AddResourceOwnerRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.RbacServiceClient.prototype.addResourceOwner =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.RbacService/AddResourceOwner',
      request,
      metadata || {},
      methodDescriptor_RbacService_AddResourceOwner,
      callback);
};


/**
 * @param {!proto.resource.AddResourceOwnerRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.AddResourceOwnerRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.RbacServicePromiseClient.prototype.addResourceOwner =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.RbacService/AddResourceOwner',
      request,
      metadata || {},
      methodDescriptor_RbacService_AddResourceOwner);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.AddResourceOwnerRqst,
 *   !proto.resource.AddResourceOwnerRsp>}
 */
const methodDescriptor_RbacService_RemoveResourceOwner = new grpc.web.MethodDescriptor(
  '/resource.RbacService/RemoveResourceOwner',
  grpc.web.MethodType.UNARY,
  proto.resource.AddResourceOwnerRqst,
  proto.resource.AddResourceOwnerRsp,
  /**
   * @param {!proto.resource.AddResourceOwnerRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddResourceOwnerRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.AddResourceOwnerRqst,
 *   !proto.resource.AddResourceOwnerRsp>}
 */
const methodInfo_RbacService_RemoveResourceOwner = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.AddResourceOwnerRsp,
  /**
   * @param {!proto.resource.AddResourceOwnerRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddResourceOwnerRsp.deserializeBinary
);


/**
 * @param {!proto.resource.AddResourceOwnerRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.AddResourceOwnerRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.AddResourceOwnerRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.RbacServiceClient.prototype.removeResourceOwner =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.RbacService/RemoveResourceOwner',
      request,
      metadata || {},
      methodDescriptor_RbacService_RemoveResourceOwner,
      callback);
};


/**
 * @param {!proto.resource.AddResourceOwnerRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.AddResourceOwnerRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.RbacServicePromiseClient.prototype.removeResourceOwner =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.RbacService/RemoveResourceOwner',
      request,
      metadata || {},
      methodDescriptor_RbacService_RemoveResourceOwner);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.DeleteAllAccessRqst,
 *   !proto.resource.DeleteAllAccessRsp>}
 */
const methodDescriptor_RbacService_DeleteAllAccess = new grpc.web.MethodDescriptor(
  '/resource.RbacService/DeleteAllAccess',
  grpc.web.MethodType.UNARY,
  proto.resource.DeleteAllAccessRqst,
  proto.resource.DeleteAllAccessRsp,
  /**
   * @param {!proto.resource.DeleteAllAccessRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteAllAccessRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.DeleteAllAccessRqst,
 *   !proto.resource.DeleteAllAccessRsp>}
 */
const methodInfo_RbacService_DeleteAllAccess = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.DeleteAllAccessRsp,
  /**
   * @param {!proto.resource.DeleteAllAccessRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteAllAccessRsp.deserializeBinary
);


/**
 * @param {!proto.resource.DeleteAllAccessRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.DeleteAllAccessRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.DeleteAllAccessRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.RbacServiceClient.prototype.deleteAllAccess =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.RbacService/DeleteAllAccess',
      request,
      metadata || {},
      methodDescriptor_RbacService_DeleteAllAccess,
      callback);
};


/**
 * @param {!proto.resource.DeleteAllAccessRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.DeleteAllAccessRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.RbacServicePromiseClient.prototype.deleteAllAccess =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.RbacService/DeleteAllAccess',
      request,
      metadata || {},
      methodDescriptor_RbacService_DeleteAllAccess);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.ValidateAccessRqst,
 *   !proto.resource.ValidateAccessRsp>}
 */
const methodDescriptor_RbacService_ValidateAccess = new grpc.web.MethodDescriptor(
  '/resource.RbacService/ValidateAccess',
  grpc.web.MethodType.UNARY,
  proto.resource.ValidateAccessRqst,
  proto.resource.ValidateAccessRsp,
  /**
   * @param {!proto.resource.ValidateAccessRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ValidateAccessRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.ValidateAccessRqst,
 *   !proto.resource.ValidateAccessRsp>}
 */
const methodInfo_RbacService_ValidateAccess = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.ValidateAccessRsp,
  /**
   * @param {!proto.resource.ValidateAccessRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ValidateAccessRsp.deserializeBinary
);


/**
 * @param {!proto.resource.ValidateAccessRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.ValidateAccessRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.ValidateAccessRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.RbacServiceClient.prototype.validateAccess =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.RbacService/ValidateAccess',
      request,
      metadata || {},
      methodDescriptor_RbacService_ValidateAccess,
      callback);
};


/**
 * @param {!proto.resource.ValidateAccessRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.ValidateAccessRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.RbacServicePromiseClient.prototype.validateAccess =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.RbacService/ValidateAccess',
      request,
      metadata || {},
      methodDescriptor_RbacService_ValidateAccess);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetAccessesRqst,
 *   !proto.resource.GetAccessesRsp>}
 */
const methodDescriptor_RbacService_GetAccesses = new grpc.web.MethodDescriptor(
  '/resource.RbacService/GetAccesses',
  grpc.web.MethodType.UNARY,
  proto.resource.GetAccessesRqst,
  proto.resource.GetAccessesRsp,
  /**
   * @param {!proto.resource.GetAccessesRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetAccessesRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetAccessesRqst,
 *   !proto.resource.GetAccessesRsp>}
 */
const methodInfo_RbacService_GetAccesses = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetAccessesRsp,
  /**
   * @param {!proto.resource.GetAccessesRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetAccessesRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetAccessesRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.GetAccessesRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetAccessesRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.RbacServiceClient.prototype.getAccesses =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.RbacService/GetAccesses',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetAccesses,
      callback);
};


/**
 * @param {!proto.resource.GetAccessesRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.GetAccessesRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.RbacServicePromiseClient.prototype.getAccesses =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.RbacService/GetAccesses',
      request,
      metadata || {},
      methodDescriptor_RbacService_GetAccesses);
};


/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.resource.LogServiceClient =
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
proto.resource.LogServicePromiseClient =
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
 *   !proto.resource.LogRqst,
 *   !proto.resource.LogRsp>}
 */
const methodDescriptor_LogService_Log = new grpc.web.MethodDescriptor(
  '/resource.LogService/Log',
  grpc.web.MethodType.UNARY,
  proto.resource.LogRqst,
  proto.resource.LogRsp,
  /**
   * @param {!proto.resource.LogRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.LogRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.LogRqst,
 *   !proto.resource.LogRsp>}
 */
const methodInfo_LogService_Log = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.LogRsp,
  /**
   * @param {!proto.resource.LogRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.LogRsp.deserializeBinary
);


/**
 * @param {!proto.resource.LogRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.LogRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.LogRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.LogServiceClient.prototype.log =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.LogService/Log',
      request,
      metadata || {},
      methodDescriptor_LogService_Log,
      callback);
};


/**
 * @param {!proto.resource.LogRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.LogRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.LogServicePromiseClient.prototype.log =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.LogService/Log',
      request,
      metadata || {},
      methodDescriptor_LogService_Log);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetLogRqst,
 *   !proto.resource.GetLogRsp>}
 */
const methodDescriptor_LogService_GetLog = new grpc.web.MethodDescriptor(
  '/resource.LogService/GetLog',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.resource.GetLogRqst,
  proto.resource.GetLogRsp,
  /**
   * @param {!proto.resource.GetLogRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetLogRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetLogRqst,
 *   !proto.resource.GetLogRsp>}
 */
const methodInfo_LogService_GetLog = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetLogRsp,
  /**
   * @param {!proto.resource.GetLogRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetLogRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetLogRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetLogRsp>}
 *     The XHR Node Readable Stream
 */
proto.resource.LogServiceClient.prototype.getLog =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.LogService/GetLog',
      request,
      metadata || {},
      methodDescriptor_LogService_GetLog);
};


/**
 * @param {!proto.resource.GetLogRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetLogRsp>}
 *     The XHR Node Readable Stream
 */
proto.resource.LogServicePromiseClient.prototype.getLog =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.LogService/GetLog',
      request,
      metadata || {},
      methodDescriptor_LogService_GetLog);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.DeleteLogRqst,
 *   !proto.resource.DeleteLogRsp>}
 */
const methodDescriptor_LogService_DeleteLog = new grpc.web.MethodDescriptor(
  '/resource.LogService/DeleteLog',
  grpc.web.MethodType.UNARY,
  proto.resource.DeleteLogRqst,
  proto.resource.DeleteLogRsp,
  /**
   * @param {!proto.resource.DeleteLogRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteLogRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.DeleteLogRqst,
 *   !proto.resource.DeleteLogRsp>}
 */
const methodInfo_LogService_DeleteLog = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.DeleteLogRsp,
  /**
   * @param {!proto.resource.DeleteLogRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteLogRsp.deserializeBinary
);


/**
 * @param {!proto.resource.DeleteLogRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.DeleteLogRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.DeleteLogRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.LogServiceClient.prototype.deleteLog =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.LogService/DeleteLog',
      request,
      metadata || {},
      methodDescriptor_LogService_DeleteLog,
      callback);
};


/**
 * @param {!proto.resource.DeleteLogRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.DeleteLogRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.LogServicePromiseClient.prototype.deleteLog =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.LogService/DeleteLog',
      request,
      metadata || {},
      methodDescriptor_LogService_DeleteLog);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.ClearAllLogRqst,
 *   !proto.resource.ClearAllLogRsp>}
 */
const methodDescriptor_LogService_ClearAllLog = new grpc.web.MethodDescriptor(
  '/resource.LogService/ClearAllLog',
  grpc.web.MethodType.UNARY,
  proto.resource.ClearAllLogRqst,
  proto.resource.ClearAllLogRsp,
  /**
   * @param {!proto.resource.ClearAllLogRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ClearAllLogRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.ClearAllLogRqst,
 *   !proto.resource.ClearAllLogRsp>}
 */
const methodInfo_LogService_ClearAllLog = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.ClearAllLogRsp,
  /**
   * @param {!proto.resource.ClearAllLogRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ClearAllLogRsp.deserializeBinary
);


/**
 * @param {!proto.resource.ClearAllLogRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.ClearAllLogRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.ClearAllLogRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.LogServiceClient.prototype.clearAllLog =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.LogService/ClearAllLog',
      request,
      metadata || {},
      methodDescriptor_LogService_ClearAllLog,
      callback);
};


/**
 * @param {!proto.resource.ClearAllLogRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.ClearAllLogRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.LogServicePromiseClient.prototype.clearAllLog =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.LogService/ClearAllLog',
      request,
      metadata || {},
      methodDescriptor_LogService_ClearAllLog);
};


module.exports = proto.resource;

