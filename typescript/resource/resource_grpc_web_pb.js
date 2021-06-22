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
 *     Promise that resolves to the response
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
 *     Promise that resolves to the response
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
 *     Promise that resolves to the response
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
 *     Promise that resolves to the response
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
 *     Promise that resolves to the response
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
 *     Promise that resolves to the response
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
 *     Promise that resolves to the response
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
 *     Promise that resolves to the response
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
 *     Promise that resolves to the response
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
 *     Promise that resolves to the response
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
 *     Promise that resolves to the response
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
 *   !proto.resource.UpdateGroupRqst,
 *   !proto.resource.UpdateGroupRsp>}
 */
const methodDescriptor_ResourceService_UpdateGroup = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/UpdateGroup',
  grpc.web.MethodType.UNARY,
  proto.resource.UpdateGroupRqst,
  proto.resource.UpdateGroupRsp,
  /**
   * @param {!proto.resource.UpdateGroupRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.UpdateGroupRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.UpdateGroupRqst,
 *   !proto.resource.UpdateGroupRsp>}
 */
const methodInfo_ResourceService_UpdateGroup = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.UpdateGroupRsp,
  /**
   * @param {!proto.resource.UpdateGroupRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.UpdateGroupRsp.deserializeBinary
);


/**
 * @param {!proto.resource.UpdateGroupRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.UpdateGroupRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.UpdateGroupRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.updateGroup =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/UpdateGroup',
      request,
      metadata || {},
      methodDescriptor_ResourceService_UpdateGroup,
      callback);
};


/**
 * @param {!proto.resource.UpdateGroupRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.UpdateGroupRsp>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.updateGroup =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/UpdateGroup',
      request,
      metadata || {},
      methodDescriptor_ResourceService_UpdateGroup);
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
 *     Promise that resolves to the response
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
 *     Promise that resolves to the response
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
 *     Promise that resolves to the response
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
 *     Promise that resolves to the response
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
 *     Promise that resolves to the response
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
 *   !proto.resource.GetAccountRqst,
 *   !proto.resource.GetAccountRsp>}
 */
const methodDescriptor_ResourceService_GetAccount = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetAccount',
  grpc.web.MethodType.UNARY,
  proto.resource.GetAccountRqst,
  proto.resource.GetAccountRsp,
  /**
   * @param {!proto.resource.GetAccountRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetAccountRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetAccountRqst,
 *   !proto.resource.GetAccountRsp>}
 */
const methodInfo_ResourceService_GetAccount = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetAccountRsp,
  /**
   * @param {!proto.resource.GetAccountRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetAccountRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetAccountRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.GetAccountRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetAccountRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getAccount =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/GetAccount',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetAccount,
      callback);
};


/**
 * @param {!proto.resource.GetAccountRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.GetAccountRsp>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.getAccount =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/GetAccount',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetAccount);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.SetAccountPasswordRqst,
 *   !proto.resource.SetAccountPasswordRsp>}
 */
const methodDescriptor_ResourceService_SetAccountPassword = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/SetAccountPassword',
  grpc.web.MethodType.UNARY,
  proto.resource.SetAccountPasswordRqst,
  proto.resource.SetAccountPasswordRsp,
  /**
   * @param {!proto.resource.SetAccountPasswordRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetAccountPasswordRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.SetAccountPasswordRqst,
 *   !proto.resource.SetAccountPasswordRsp>}
 */
const methodInfo_ResourceService_SetAccountPassword = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.SetAccountPasswordRsp,
  /**
   * @param {!proto.resource.SetAccountPasswordRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetAccountPasswordRsp.deserializeBinary
);


/**
 * @param {!proto.resource.SetAccountPasswordRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.SetAccountPasswordRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.SetAccountPasswordRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.setAccountPassword =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/SetAccountPassword',
      request,
      metadata || {},
      methodDescriptor_ResourceService_SetAccountPassword,
      callback);
};


/**
 * @param {!proto.resource.SetAccountPasswordRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.SetAccountPasswordRsp>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.setAccountPassword =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/SetAccountPassword',
      request,
      metadata || {},
      methodDescriptor_ResourceService_SetAccountPassword);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetAccountsRqst,
 *   !proto.resource.GetAccountsRsp>}
 */
const methodDescriptor_ResourceService_GetAccounts = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetAccounts',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.resource.GetAccountsRqst,
  proto.resource.GetAccountsRsp,
  /**
   * @param {!proto.resource.GetAccountsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetAccountsRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetAccountsRqst,
 *   !proto.resource.GetAccountsRsp>}
 */
const methodInfo_ResourceService_GetAccounts = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetAccountsRsp,
  /**
   * @param {!proto.resource.GetAccountsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetAccountsRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetAccountsRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetAccountsRsp>}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getAccounts =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.ResourceService/GetAccounts',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetAccounts);
};


/**
 * @param {!proto.resource.GetAccountsRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetAccountsRsp>}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServicePromiseClient.prototype.getAccounts =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.ResourceService/GetAccounts',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetAccounts);
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
 *     Promise that resolves to the response
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
 *     Promise that resolves to the response
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
 *   !proto.resource.SetAccountContactRqst,
 *   !proto.resource.SetAccountContactRsp>}
 */
const methodDescriptor_ResourceService_SetAccountContact = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/SetAccountContact',
  grpc.web.MethodType.UNARY,
  proto.resource.SetAccountContactRqst,
  proto.resource.SetAccountContactRsp,
  /**
   * @param {!proto.resource.SetAccountContactRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetAccountContactRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.SetAccountContactRqst,
 *   !proto.resource.SetAccountContactRsp>}
 */
const methodInfo_ResourceService_SetAccountContact = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.SetAccountContactRsp,
  /**
   * @param {!proto.resource.SetAccountContactRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetAccountContactRsp.deserializeBinary
);


/**
 * @param {!proto.resource.SetAccountContactRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.SetAccountContactRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.SetAccountContactRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.setAccountContact =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/SetAccountContact',
      request,
      metadata || {},
      methodDescriptor_ResourceService_SetAccountContact,
      callback);
};


/**
 * @param {!proto.resource.SetAccountContactRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.SetAccountContactRsp>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.setAccountContact =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/SetAccountContact',
      request,
      metadata || {},
      methodDescriptor_ResourceService_SetAccountContact);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.SetEmailRequest,
 *   !proto.resource.SetEmailResponse>}
 */
const methodDescriptor_ResourceService_SetEmail = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/SetEmail',
  grpc.web.MethodType.UNARY,
  proto.resource.SetEmailRequest,
  proto.resource.SetEmailResponse,
  /**
   * @param {!proto.resource.SetEmailRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetEmailResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.SetEmailRequest,
 *   !proto.resource.SetEmailResponse>}
 */
const methodInfo_ResourceService_SetEmail = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.SetEmailResponse,
  /**
   * @param {!proto.resource.SetEmailRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetEmailResponse.deserializeBinary
);


/**
 * @param {!proto.resource.SetEmailRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.SetEmailResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.SetEmailResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.setEmail =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/SetEmail',
      request,
      metadata || {},
      methodDescriptor_ResourceService_SetEmail,
      callback);
};


/**
 * @param {!proto.resource.SetEmailRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.SetEmailResponse>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.setEmail =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/SetEmail',
      request,
      metadata || {},
      methodDescriptor_ResourceService_SetEmail);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.IsOrgnanizationMemberRqst,
 *   !proto.resource.IsOrgnanizationMemberRsp>}
 */
const methodDescriptor_ResourceService_IsOrgnanizationMember = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/IsOrgnanizationMember',
  grpc.web.MethodType.UNARY,
  proto.resource.IsOrgnanizationMemberRqst,
  proto.resource.IsOrgnanizationMemberRsp,
  /**
   * @param {!proto.resource.IsOrgnanizationMemberRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.IsOrgnanizationMemberRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.IsOrgnanizationMemberRqst,
 *   !proto.resource.IsOrgnanizationMemberRsp>}
 */
const methodInfo_ResourceService_IsOrgnanizationMember = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.IsOrgnanizationMemberRsp,
  /**
   * @param {!proto.resource.IsOrgnanizationMemberRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.IsOrgnanizationMemberRsp.deserializeBinary
);


/**
 * @param {!proto.resource.IsOrgnanizationMemberRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.IsOrgnanizationMemberRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.IsOrgnanizationMemberRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.isOrgnanizationMember =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/IsOrgnanizationMember',
      request,
      metadata || {},
      methodDescriptor_ResourceService_IsOrgnanizationMember,
      callback);
};


/**
 * @param {!proto.resource.IsOrgnanizationMemberRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.IsOrgnanizationMemberRsp>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.isOrgnanizationMember =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/IsOrgnanizationMember',
      request,
      metadata || {},
      methodDescriptor_ResourceService_IsOrgnanizationMember);
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
 *     Promise that resolves to the response
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
 *   !proto.resource.GetRolesRqst,
 *   !proto.resource.GetRolesRsp>}
 */
const methodDescriptor_ResourceService_GetRoles = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetRoles',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.resource.GetRolesRqst,
  proto.resource.GetRolesRsp,
  /**
   * @param {!proto.resource.GetRolesRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetRolesRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetRolesRqst,
 *   !proto.resource.GetRolesRsp>}
 */
const methodInfo_ResourceService_GetRoles = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetRolesRsp,
  /**
   * @param {!proto.resource.GetRolesRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetRolesRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetRolesRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetRolesRsp>}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getRoles =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.ResourceService/GetRoles',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetRoles);
};


/**
 * @param {!proto.resource.GetRolesRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetRolesRsp>}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServicePromiseClient.prototype.getRoles =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.ResourceService/GetRoles',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetRoles);
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
 *     Promise that resolves to the response
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
 *   !proto.resource.AddRoleActionsRqst,
 *   !proto.resource.AddRoleActionsRsp>}
 */
const methodDescriptor_ResourceService_AddRoleActions = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/AddRoleActions',
  grpc.web.MethodType.UNARY,
  proto.resource.AddRoleActionsRqst,
  proto.resource.AddRoleActionsRsp,
  /**
   * @param {!proto.resource.AddRoleActionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddRoleActionsRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.AddRoleActionsRqst,
 *   !proto.resource.AddRoleActionsRsp>}
 */
const methodInfo_ResourceService_AddRoleActions = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.AddRoleActionsRsp,
  /**
   * @param {!proto.resource.AddRoleActionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddRoleActionsRsp.deserializeBinary
);


/**
 * @param {!proto.resource.AddRoleActionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.AddRoleActionsRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.AddRoleActionsRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.addRoleActions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/AddRoleActions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddRoleActions,
      callback);
};


/**
 * @param {!proto.resource.AddRoleActionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.AddRoleActionsRsp>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.addRoleActions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/AddRoleActions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddRoleActions);
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
 *     Promise that resolves to the response
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
 *   !proto.resource.RemoveRolesActionRqst,
 *   !proto.resource.RemoveRolesActionRsp>}
 */
const methodDescriptor_ResourceService_RemoveRolesAction = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/RemoveRolesAction',
  grpc.web.MethodType.UNARY,
  proto.resource.RemoveRolesActionRqst,
  proto.resource.RemoveRolesActionRsp,
  /**
   * @param {!proto.resource.RemoveRolesActionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveRolesActionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.RemoveRolesActionRqst,
 *   !proto.resource.RemoveRolesActionRsp>}
 */
const methodInfo_ResourceService_RemoveRolesAction = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.RemoveRolesActionRsp,
  /**
   * @param {!proto.resource.RemoveRolesActionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveRolesActionRsp.deserializeBinary
);


/**
 * @param {!proto.resource.RemoveRolesActionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.RemoveRolesActionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.RemoveRolesActionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.removeRolesAction =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/RemoveRolesAction',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveRolesAction,
      callback);
};


/**
 * @param {!proto.resource.RemoveRolesActionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.RemoveRolesActionRsp>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.removeRolesAction =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/RemoveRolesAction',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveRolesAction);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.CreateApplicationRqst,
 *   !proto.resource.CreateApplicationRsp>}
 */
const methodDescriptor_ResourceService_CreateApplication = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/CreateApplication',
  grpc.web.MethodType.UNARY,
  proto.resource.CreateApplicationRqst,
  proto.resource.CreateApplicationRsp,
  /**
   * @param {!proto.resource.CreateApplicationRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.CreateApplicationRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.CreateApplicationRqst,
 *   !proto.resource.CreateApplicationRsp>}
 */
const methodInfo_ResourceService_CreateApplication = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.CreateApplicationRsp,
  /**
   * @param {!proto.resource.CreateApplicationRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.CreateApplicationRsp.deserializeBinary
);


/**
 * @param {!proto.resource.CreateApplicationRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.CreateApplicationRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.CreateApplicationRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.createApplication =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/CreateApplication',
      request,
      metadata || {},
      methodDescriptor_ResourceService_CreateApplication,
      callback);
};


/**
 * @param {!proto.resource.CreateApplicationRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.CreateApplicationRsp>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.createApplication =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/CreateApplication',
      request,
      metadata || {},
      methodDescriptor_ResourceService_CreateApplication);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.UpdateApplicationRqst,
 *   !proto.resource.UpdateApplicationRsp>}
 */
const methodDescriptor_ResourceService_UpdateApplication = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/UpdateApplication',
  grpc.web.MethodType.UNARY,
  proto.resource.UpdateApplicationRqst,
  proto.resource.UpdateApplicationRsp,
  /**
   * @param {!proto.resource.UpdateApplicationRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.UpdateApplicationRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.UpdateApplicationRqst,
 *   !proto.resource.UpdateApplicationRsp>}
 */
const methodInfo_ResourceService_UpdateApplication = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.UpdateApplicationRsp,
  /**
   * @param {!proto.resource.UpdateApplicationRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.UpdateApplicationRsp.deserializeBinary
);


/**
 * @param {!proto.resource.UpdateApplicationRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.UpdateApplicationRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.UpdateApplicationRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.updateApplication =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/UpdateApplication',
      request,
      metadata || {},
      methodDescriptor_ResourceService_UpdateApplication,
      callback);
};


/**
 * @param {!proto.resource.UpdateApplicationRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.UpdateApplicationRsp>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.updateApplication =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/UpdateApplication',
      request,
      metadata || {},
      methodDescriptor_ResourceService_UpdateApplication);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetApplicationsRqst,
 *   !proto.resource.GetApplicationsRsp>}
 */
const methodDescriptor_ResourceService_GetApplications = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetApplications',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.resource.GetApplicationsRqst,
  proto.resource.GetApplicationsRsp,
  /**
   * @param {!proto.resource.GetApplicationsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetApplicationsRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetApplicationsRqst,
 *   !proto.resource.GetApplicationsRsp>}
 */
const methodInfo_ResourceService_GetApplications = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetApplicationsRsp,
  /**
   * @param {!proto.resource.GetApplicationsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetApplicationsRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetApplicationsRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetApplicationsRsp>}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getApplications =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.ResourceService/GetApplications',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetApplications);
};


/**
 * @param {!proto.resource.GetApplicationsRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetApplicationsRsp>}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServicePromiseClient.prototype.getApplications =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.ResourceService/GetApplications',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetApplications);
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
 *     Promise that resolves to the response
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
 *   !proto.resource.AddApplicationActionsRqst,
 *   !proto.resource.AddApplicationActionsRsp>}
 */
const methodDescriptor_ResourceService_AddApplicationActions = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/AddApplicationActions',
  grpc.web.MethodType.UNARY,
  proto.resource.AddApplicationActionsRqst,
  proto.resource.AddApplicationActionsRsp,
  /**
   * @param {!proto.resource.AddApplicationActionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddApplicationActionsRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.AddApplicationActionsRqst,
 *   !proto.resource.AddApplicationActionsRsp>}
 */
const methodInfo_ResourceService_AddApplicationActions = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.AddApplicationActionsRsp,
  /**
   * @param {!proto.resource.AddApplicationActionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddApplicationActionsRsp.deserializeBinary
);


/**
 * @param {!proto.resource.AddApplicationActionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.AddApplicationActionsRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.AddApplicationActionsRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.addApplicationActions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/AddApplicationActions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddApplicationActions,
      callback);
};


/**
 * @param {!proto.resource.AddApplicationActionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.AddApplicationActionsRsp>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.addApplicationActions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/AddApplicationActions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddApplicationActions);
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
 *     Promise that resolves to the response
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
 *   !proto.resource.RemoveApplicationsActionRqst,
 *   !proto.resource.RemoveApplicationsActionRsp>}
 */
const methodDescriptor_ResourceService_RemoveApplicationsAction = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/RemoveApplicationsAction',
  grpc.web.MethodType.UNARY,
  proto.resource.RemoveApplicationsActionRqst,
  proto.resource.RemoveApplicationsActionRsp,
  /**
   * @param {!proto.resource.RemoveApplicationsActionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveApplicationsActionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.RemoveApplicationsActionRqst,
 *   !proto.resource.RemoveApplicationsActionRsp>}
 */
const methodInfo_ResourceService_RemoveApplicationsAction = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.RemoveApplicationsActionRsp,
  /**
   * @param {!proto.resource.RemoveApplicationsActionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveApplicationsActionRsp.deserializeBinary
);


/**
 * @param {!proto.resource.RemoveApplicationsActionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.RemoveApplicationsActionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.RemoveApplicationsActionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.removeApplicationsAction =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/RemoveApplicationsAction',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveApplicationsAction,
      callback);
};


/**
 * @param {!proto.resource.RemoveApplicationsActionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.RemoveApplicationsActionRsp>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.removeApplicationsAction =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/RemoveApplicationsAction',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveApplicationsAction);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetApplicationVersionRqst,
 *   !proto.resource.GetApplicationVersionRsp>}
 */
const methodDescriptor_ResourceService_GetApplicationVersion = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetApplicationVersion',
  grpc.web.MethodType.UNARY,
  proto.resource.GetApplicationVersionRqst,
  proto.resource.GetApplicationVersionRsp,
  /**
   * @param {!proto.resource.GetApplicationVersionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetApplicationVersionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetApplicationVersionRqst,
 *   !proto.resource.GetApplicationVersionRsp>}
 */
const methodInfo_ResourceService_GetApplicationVersion = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetApplicationVersionRsp,
  /**
   * @param {!proto.resource.GetApplicationVersionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetApplicationVersionRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetApplicationVersionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.GetApplicationVersionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetApplicationVersionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getApplicationVersion =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/GetApplicationVersion',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetApplicationVersion,
      callback);
};


/**
 * @param {!proto.resource.GetApplicationVersionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.GetApplicationVersionRsp>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.getApplicationVersion =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/GetApplicationVersion',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetApplicationVersion);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetApplicationAliasRqst,
 *   !proto.resource.GetApplicationAliasRsp>}
 */
const methodDescriptor_ResourceService_GetApplicationAlias = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetApplicationAlias',
  grpc.web.MethodType.UNARY,
  proto.resource.GetApplicationAliasRqst,
  proto.resource.GetApplicationAliasRsp,
  /**
   * @param {!proto.resource.GetApplicationAliasRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetApplicationAliasRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetApplicationAliasRqst,
 *   !proto.resource.GetApplicationAliasRsp>}
 */
const methodInfo_ResourceService_GetApplicationAlias = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetApplicationAliasRsp,
  /**
   * @param {!proto.resource.GetApplicationAliasRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetApplicationAliasRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetApplicationAliasRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.GetApplicationAliasRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetApplicationAliasRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getApplicationAlias =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/GetApplicationAlias',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetApplicationAlias,
      callback);
};


/**
 * @param {!proto.resource.GetApplicationAliasRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.GetApplicationAliasRsp>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.getApplicationAlias =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/GetApplicationAlias',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetApplicationAlias);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetApplicationIconRqst,
 *   !proto.resource.GetApplicationIconRsp>}
 */
const methodDescriptor_ResourceService_GetApplicationIcon = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetApplicationIcon',
  grpc.web.MethodType.UNARY,
  proto.resource.GetApplicationIconRqst,
  proto.resource.GetApplicationIconRsp,
  /**
   * @param {!proto.resource.GetApplicationIconRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetApplicationIconRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetApplicationIconRqst,
 *   !proto.resource.GetApplicationIconRsp>}
 */
const methodInfo_ResourceService_GetApplicationIcon = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetApplicationIconRsp,
  /**
   * @param {!proto.resource.GetApplicationIconRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetApplicationIconRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetApplicationIconRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.GetApplicationIconRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetApplicationIconRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getApplicationIcon =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/GetApplicationIcon',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetApplicationIcon,
      callback);
};


/**
 * @param {!proto.resource.GetApplicationIconRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.GetApplicationIconRsp>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.getApplicationIcon =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/GetApplicationIcon',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetApplicationIcon);
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
 *     Promise that resolves to the response
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
 *     Promise that resolves to the response
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
 *   !proto.resource.AddPeerActionsRqst,
 *   !proto.resource.AddPeerActionsRsp>}
 */
const methodDescriptor_ResourceService_AddPeerActions = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/AddPeerActions',
  grpc.web.MethodType.UNARY,
  proto.resource.AddPeerActionsRqst,
  proto.resource.AddPeerActionsRsp,
  /**
   * @param {!proto.resource.AddPeerActionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddPeerActionsRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.AddPeerActionsRqst,
 *   !proto.resource.AddPeerActionsRsp>}
 */
const methodInfo_ResourceService_AddPeerActions = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.AddPeerActionsRsp,
  /**
   * @param {!proto.resource.AddPeerActionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.AddPeerActionsRsp.deserializeBinary
);


/**
 * @param {!proto.resource.AddPeerActionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.AddPeerActionsRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.AddPeerActionsRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.addPeerActions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/AddPeerActions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddPeerActions,
      callback);
};


/**
 * @param {!proto.resource.AddPeerActionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.AddPeerActionsRsp>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.addPeerActions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/AddPeerActions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_AddPeerActions);
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
 *     Promise that resolves to the response
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
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.RemovePeersActionRqst,
 *   !proto.resource.RemovePeersActionRsp>}
 */
const methodDescriptor_ResourceService_RemovePeersAction = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/RemovePeersAction',
  grpc.web.MethodType.UNARY,
  proto.resource.RemovePeersActionRqst,
  proto.resource.RemovePeersActionRsp,
  /**
   * @param {!proto.resource.RemovePeersActionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemovePeersActionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.RemovePeersActionRqst,
 *   !proto.resource.RemovePeersActionRsp>}
 */
const methodInfo_ResourceService_RemovePeersAction = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.RemovePeersActionRsp,
  /**
   * @param {!proto.resource.RemovePeersActionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemovePeersActionRsp.deserializeBinary
);


/**
 * @param {!proto.resource.RemovePeersActionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.RemovePeersActionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.RemovePeersActionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.removePeersAction =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/RemovePeersAction',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemovePeersAction,
      callback);
};


/**
 * @param {!proto.resource.RemovePeersActionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.RemovePeersActionRsp>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.removePeersAction =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/RemovePeersAction',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemovePeersAction);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.CreateNotificationRqst,
 *   !proto.resource.CreateNotificationRsp>}
 */
const methodDescriptor_ResourceService_CreateNotification = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/CreateNotification',
  grpc.web.MethodType.UNARY,
  proto.resource.CreateNotificationRqst,
  proto.resource.CreateNotificationRsp,
  /**
   * @param {!proto.resource.CreateNotificationRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.CreateNotificationRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.CreateNotificationRqst,
 *   !proto.resource.CreateNotificationRsp>}
 */
const methodInfo_ResourceService_CreateNotification = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.CreateNotificationRsp,
  /**
   * @param {!proto.resource.CreateNotificationRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.CreateNotificationRsp.deserializeBinary
);


/**
 * @param {!proto.resource.CreateNotificationRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.CreateNotificationRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.CreateNotificationRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.createNotification =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/CreateNotification',
      request,
      metadata || {},
      methodDescriptor_ResourceService_CreateNotification,
      callback);
};


/**
 * @param {!proto.resource.CreateNotificationRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.CreateNotificationRsp>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.createNotification =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/CreateNotification',
      request,
      metadata || {},
      methodDescriptor_ResourceService_CreateNotification);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetNotificationsRqst,
 *   !proto.resource.GetNotificationsRsp>}
 */
const methodDescriptor_ResourceService_GetNotifications = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetNotifications',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.resource.GetNotificationsRqst,
  proto.resource.GetNotificationsRsp,
  /**
   * @param {!proto.resource.GetNotificationsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetNotificationsRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetNotificationsRqst,
 *   !proto.resource.GetNotificationsRsp>}
 */
const methodInfo_ResourceService_GetNotifications = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetNotificationsRsp,
  /**
   * @param {!proto.resource.GetNotificationsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetNotificationsRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetNotificationsRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetNotificationsRsp>}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getNotifications =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.ResourceService/GetNotifications',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetNotifications);
};


/**
 * @param {!proto.resource.GetNotificationsRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetNotificationsRsp>}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServicePromiseClient.prototype.getNotifications =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.ResourceService/GetNotifications',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetNotifications);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.DeleteNotificationRqst,
 *   !proto.resource.DeleteNotificationRsp>}
 */
const methodDescriptor_ResourceService_DeleteNotification = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/DeleteNotification',
  grpc.web.MethodType.UNARY,
  proto.resource.DeleteNotificationRqst,
  proto.resource.DeleteNotificationRsp,
  /**
   * @param {!proto.resource.DeleteNotificationRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteNotificationRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.DeleteNotificationRqst,
 *   !proto.resource.DeleteNotificationRsp>}
 */
const methodInfo_ResourceService_DeleteNotification = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.DeleteNotificationRsp,
  /**
   * @param {!proto.resource.DeleteNotificationRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteNotificationRsp.deserializeBinary
);


/**
 * @param {!proto.resource.DeleteNotificationRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.DeleteNotificationRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.DeleteNotificationRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.deleteNotification =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/DeleteNotification',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteNotification,
      callback);
};


/**
 * @param {!proto.resource.DeleteNotificationRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.DeleteNotificationRsp>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.deleteNotification =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/DeleteNotification',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteNotification);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.ClearAllNotificationsRqst,
 *   !proto.resource.ClearAllNotificationsRsp>}
 */
const methodDescriptor_ResourceService_ClearAllNotifications = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/ClearAllNotifications',
  grpc.web.MethodType.UNARY,
  proto.resource.ClearAllNotificationsRqst,
  proto.resource.ClearAllNotificationsRsp,
  /**
   * @param {!proto.resource.ClearAllNotificationsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ClearAllNotificationsRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.ClearAllNotificationsRqst,
 *   !proto.resource.ClearAllNotificationsRsp>}
 */
const methodInfo_ResourceService_ClearAllNotifications = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.ClearAllNotificationsRsp,
  /**
   * @param {!proto.resource.ClearAllNotificationsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ClearAllNotificationsRsp.deserializeBinary
);


/**
 * @param {!proto.resource.ClearAllNotificationsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.ClearAllNotificationsRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.ClearAllNotificationsRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.clearAllNotifications =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/ClearAllNotifications',
      request,
      metadata || {},
      methodDescriptor_ResourceService_ClearAllNotifications,
      callback);
};


/**
 * @param {!proto.resource.ClearAllNotificationsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.ClearAllNotificationsRsp>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.clearAllNotifications =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/ClearAllNotifications',
      request,
      metadata || {},
      methodDescriptor_ResourceService_ClearAllNotifications);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.ClearNotificationsByTypeRqst,
 *   !proto.resource.ClearNotificationsByTypeRsp>}
 */
const methodDescriptor_ResourceService_ClearNotificationsByType = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/ClearNotificationsByType',
  grpc.web.MethodType.UNARY,
  proto.resource.ClearNotificationsByTypeRqst,
  proto.resource.ClearNotificationsByTypeRsp,
  /**
   * @param {!proto.resource.ClearNotificationsByTypeRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ClearNotificationsByTypeRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.ClearNotificationsByTypeRqst,
 *   !proto.resource.ClearNotificationsByTypeRsp>}
 */
const methodInfo_ResourceService_ClearNotificationsByType = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.ClearNotificationsByTypeRsp,
  /**
   * @param {!proto.resource.ClearNotificationsByTypeRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ClearNotificationsByTypeRsp.deserializeBinary
);


/**
 * @param {!proto.resource.ClearNotificationsByTypeRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.ClearNotificationsByTypeRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.ClearNotificationsByTypeRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.clearNotificationsByType =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/ClearNotificationsByType',
      request,
      metadata || {},
      methodDescriptor_ResourceService_ClearNotificationsByType,
      callback);
};


/**
 * @param {!proto.resource.ClearNotificationsByTypeRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.ClearNotificationsByTypeRsp>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.clearNotificationsByType =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/ClearNotificationsByType',
      request,
      metadata || {},
      methodDescriptor_ResourceService_ClearNotificationsByType);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.FindPackagesDescriptorRequest,
 *   !proto.resource.FindPackagesDescriptorResponse>}
 */
const methodDescriptor_ResourceService_FindPackages = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/FindPackages',
  grpc.web.MethodType.UNARY,
  proto.resource.FindPackagesDescriptorRequest,
  proto.resource.FindPackagesDescriptorResponse,
  /**
   * @param {!proto.resource.FindPackagesDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.FindPackagesDescriptorResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.FindPackagesDescriptorRequest,
 *   !proto.resource.FindPackagesDescriptorResponse>}
 */
const methodInfo_ResourceService_FindPackages = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.FindPackagesDescriptorResponse,
  /**
   * @param {!proto.resource.FindPackagesDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.FindPackagesDescriptorResponse.deserializeBinary
);


/**
 * @param {!proto.resource.FindPackagesDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.FindPackagesDescriptorResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.FindPackagesDescriptorResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.findPackages =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/FindPackages',
      request,
      metadata || {},
      methodDescriptor_ResourceService_FindPackages,
      callback);
};


/**
 * @param {!proto.resource.FindPackagesDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.FindPackagesDescriptorResponse>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.findPackages =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/FindPackages',
      request,
      metadata || {},
      methodDescriptor_ResourceService_FindPackages);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetPackageDescriptorRequest,
 *   !proto.resource.GetPackageDescriptorResponse>}
 */
const methodDescriptor_ResourceService_GetPackageDescriptor = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetPackageDescriptor',
  grpc.web.MethodType.UNARY,
  proto.resource.GetPackageDescriptorRequest,
  proto.resource.GetPackageDescriptorResponse,
  /**
   * @param {!proto.resource.GetPackageDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetPackageDescriptorResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetPackageDescriptorRequest,
 *   !proto.resource.GetPackageDescriptorResponse>}
 */
const methodInfo_ResourceService_GetPackageDescriptor = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetPackageDescriptorResponse,
  /**
   * @param {!proto.resource.GetPackageDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetPackageDescriptorResponse.deserializeBinary
);


/**
 * @param {!proto.resource.GetPackageDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.GetPackageDescriptorResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetPackageDescriptorResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getPackageDescriptor =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/GetPackageDescriptor',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetPackageDescriptor,
      callback);
};


/**
 * @param {!proto.resource.GetPackageDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.GetPackageDescriptorResponse>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.getPackageDescriptor =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/GetPackageDescriptor',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetPackageDescriptor);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetPackagesDescriptorRequest,
 *   !proto.resource.GetPackagesDescriptorResponse>}
 */
const methodDescriptor_ResourceService_GetPackagesDescriptor = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetPackagesDescriptor',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.resource.GetPackagesDescriptorRequest,
  proto.resource.GetPackagesDescriptorResponse,
  /**
   * @param {!proto.resource.GetPackagesDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetPackagesDescriptorResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetPackagesDescriptorRequest,
 *   !proto.resource.GetPackagesDescriptorResponse>}
 */
const methodInfo_ResourceService_GetPackagesDescriptor = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetPackagesDescriptorResponse,
  /**
   * @param {!proto.resource.GetPackagesDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetPackagesDescriptorResponse.deserializeBinary
);


/**
 * @param {!proto.resource.GetPackagesDescriptorRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetPackagesDescriptorResponse>}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getPackagesDescriptor =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.ResourceService/GetPackagesDescriptor',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetPackagesDescriptor);
};


/**
 * @param {!proto.resource.GetPackagesDescriptorRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetPackagesDescriptorResponse>}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServicePromiseClient.prototype.getPackagesDescriptor =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.ResourceService/GetPackagesDescriptor',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetPackagesDescriptor);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.SetPackageDescriptorRequest,
 *   !proto.resource.SetPackageDescriptorResponse>}
 */
const methodDescriptor_ResourceService_SetPackageDescriptor = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/SetPackageDescriptor',
  grpc.web.MethodType.UNARY,
  proto.resource.SetPackageDescriptorRequest,
  proto.resource.SetPackageDescriptorResponse,
  /**
   * @param {!proto.resource.SetPackageDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetPackageDescriptorResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.SetPackageDescriptorRequest,
 *   !proto.resource.SetPackageDescriptorResponse>}
 */
const methodInfo_ResourceService_SetPackageDescriptor = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.SetPackageDescriptorResponse,
  /**
   * @param {!proto.resource.SetPackageDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetPackageDescriptorResponse.deserializeBinary
);


/**
 * @param {!proto.resource.SetPackageDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.SetPackageDescriptorResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.SetPackageDescriptorResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.setPackageDescriptor =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/SetPackageDescriptor',
      request,
      metadata || {},
      methodDescriptor_ResourceService_SetPackageDescriptor,
      callback);
};


/**
 * @param {!proto.resource.SetPackageDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.SetPackageDescriptorResponse>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.setPackageDescriptor =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/SetPackageDescriptor',
      request,
      metadata || {},
      methodDescriptor_ResourceService_SetPackageDescriptor);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.SetPackageBundleRequest,
 *   !proto.resource.SetPackageBundleResponse>}
 */
const methodDescriptor_ResourceService_SetPackageBundle = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/SetPackageBundle',
  grpc.web.MethodType.UNARY,
  proto.resource.SetPackageBundleRequest,
  proto.resource.SetPackageBundleResponse,
  /**
   * @param {!proto.resource.SetPackageBundleRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetPackageBundleResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.SetPackageBundleRequest,
 *   !proto.resource.SetPackageBundleResponse>}
 */
const methodInfo_ResourceService_SetPackageBundle = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.SetPackageBundleResponse,
  /**
   * @param {!proto.resource.SetPackageBundleRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetPackageBundleResponse.deserializeBinary
);


/**
 * @param {!proto.resource.SetPackageBundleRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.SetPackageBundleResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.SetPackageBundleResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.setPackageBundle =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/SetPackageBundle',
      request,
      metadata || {},
      methodDescriptor_ResourceService_SetPackageBundle,
      callback);
};


/**
 * @param {!proto.resource.SetPackageBundleRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.SetPackageBundleResponse>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.setPackageBundle =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/SetPackageBundle',
      request,
      metadata || {},
      methodDescriptor_ResourceService_SetPackageBundle);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetPackageBundleChecksumRequest,
 *   !proto.resource.GetPackageBundleChecksumResponse>}
 */
const methodDescriptor_ResourceService_GetPackageBundleChecksum = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetPackageBundleChecksum',
  grpc.web.MethodType.UNARY,
  proto.resource.GetPackageBundleChecksumRequest,
  proto.resource.GetPackageBundleChecksumResponse,
  /**
   * @param {!proto.resource.GetPackageBundleChecksumRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetPackageBundleChecksumResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetPackageBundleChecksumRequest,
 *   !proto.resource.GetPackageBundleChecksumResponse>}
 */
const methodInfo_ResourceService_GetPackageBundleChecksum = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetPackageBundleChecksumResponse,
  /**
   * @param {!proto.resource.GetPackageBundleChecksumRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetPackageBundleChecksumResponse.deserializeBinary
);


/**
 * @param {!proto.resource.GetPackageBundleChecksumRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.GetPackageBundleChecksumResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetPackageBundleChecksumResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getPackageBundleChecksum =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/GetPackageBundleChecksum',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetPackageBundleChecksum,
      callback);
};


/**
 * @param {!proto.resource.GetPackageBundleChecksumRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.GetPackageBundleChecksumResponse>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.getPackageBundleChecksum =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/GetPackageBundleChecksum',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetPackageBundleChecksum);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.UpdateSessionRequest,
 *   !proto.resource.UpdateSessionResponse>}
 */
const methodDescriptor_ResourceService_UpdateSession = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/UpdateSession',
  grpc.web.MethodType.UNARY,
  proto.resource.UpdateSessionRequest,
  proto.resource.UpdateSessionResponse,
  /**
   * @param {!proto.resource.UpdateSessionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.UpdateSessionResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.UpdateSessionRequest,
 *   !proto.resource.UpdateSessionResponse>}
 */
const methodInfo_ResourceService_UpdateSession = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.UpdateSessionResponse,
  /**
   * @param {!proto.resource.UpdateSessionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.UpdateSessionResponse.deserializeBinary
);


/**
 * @param {!proto.resource.UpdateSessionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.UpdateSessionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.UpdateSessionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.updateSession =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/UpdateSession',
      request,
      metadata || {},
      methodDescriptor_ResourceService_UpdateSession,
      callback);
};


/**
 * @param {!proto.resource.UpdateSessionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.UpdateSessionResponse>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.updateSession =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/UpdateSession',
      request,
      metadata || {},
      methodDescriptor_ResourceService_UpdateSession);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetSessionsRequest,
 *   !proto.resource.GetSessionsResponse>}
 */
const methodDescriptor_ResourceService_GetSessions = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetSessions',
  grpc.web.MethodType.UNARY,
  proto.resource.GetSessionsRequest,
  proto.resource.GetSessionsResponse,
  /**
   * @param {!proto.resource.GetSessionsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetSessionsResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetSessionsRequest,
 *   !proto.resource.GetSessionsResponse>}
 */
const methodInfo_ResourceService_GetSessions = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetSessionsResponse,
  /**
   * @param {!proto.resource.GetSessionsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetSessionsResponse.deserializeBinary
);


/**
 * @param {!proto.resource.GetSessionsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.GetSessionsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetSessionsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getSessions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/GetSessions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetSessions,
      callback);
};


/**
 * @param {!proto.resource.GetSessionsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.GetSessionsResponse>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.getSessions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/GetSessions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetSessions);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.RemoveSessionRequest,
 *   !proto.resource.RemoveSessionResponse>}
 */
const methodDescriptor_ResourceService_RemoveSession = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/RemoveSession',
  grpc.web.MethodType.UNARY,
  proto.resource.RemoveSessionRequest,
  proto.resource.RemoveSessionResponse,
  /**
   * @param {!proto.resource.RemoveSessionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveSessionResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.RemoveSessionRequest,
 *   !proto.resource.RemoveSessionResponse>}
 */
const methodInfo_ResourceService_RemoveSession = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.RemoveSessionResponse,
  /**
   * @param {!proto.resource.RemoveSessionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveSessionResponse.deserializeBinary
);


/**
 * @param {!proto.resource.RemoveSessionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.RemoveSessionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.RemoveSessionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.removeSession =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/RemoveSession',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveSession,
      callback);
};


/**
 * @param {!proto.resource.RemoveSessionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.RemoveSessionResponse>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.removeSession =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/RemoveSession',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveSession);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetSessionRequest,
 *   !proto.resource.GetSessionResponse>}
 */
const methodDescriptor_ResourceService_GetSession = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetSession',
  grpc.web.MethodType.UNARY,
  proto.resource.GetSessionRequest,
  proto.resource.GetSessionResponse,
  /**
   * @param {!proto.resource.GetSessionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetSessionResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetSessionRequest,
 *   !proto.resource.GetSessionResponse>}
 */
const methodInfo_ResourceService_GetSession = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetSessionResponse,
  /**
   * @param {!proto.resource.GetSessionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetSessionResponse.deserializeBinary
);


/**
 * @param {!proto.resource.GetSessionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.GetSessionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetSessionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getSession =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/GetSession',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetSession,
      callback);
};


/**
 * @param {!proto.resource.GetSessionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.GetSessionResponse>}
 *     Promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.getSession =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/GetSession',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetSession);
};


module.exports = proto.resource;

