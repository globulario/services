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
 *   !proto.resource.GetPermissionsRqst,
 *   !proto.resource.GetPermissionsRsp>}
 */
const methodDescriptor_ResourceService_GetPermissions = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetPermissions',
  grpc.web.MethodType.UNARY,
  proto.resource.GetPermissionsRqst,
  proto.resource.GetPermissionsRsp,
  /**
   * @param {!proto.resource.GetPermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetPermissionsRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetPermissionsRqst,
 *   !proto.resource.GetPermissionsRsp>}
 */
const methodInfo_ResourceService_GetPermissions = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetPermissionsRsp,
  /**
   * @param {!proto.resource.GetPermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetPermissionsRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetPermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.GetPermissionsRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetPermissionsRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getPermissions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/GetPermissions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetPermissions,
      callback);
};


/**
 * @param {!proto.resource.GetPermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.GetPermissionsRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.getPermissions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/GetPermissions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetPermissions);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.SetPermissionRqst,
 *   !proto.resource.SetPermissionRsp>}
 */
const methodDescriptor_ResourceService_SetPermission = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/SetPermission',
  grpc.web.MethodType.UNARY,
  proto.resource.SetPermissionRqst,
  proto.resource.SetPermissionRsp,
  /**
   * @param {!proto.resource.SetPermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetPermissionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.SetPermissionRqst,
 *   !proto.resource.SetPermissionRsp>}
 */
const methodInfo_ResourceService_SetPermission = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.SetPermissionRsp,
  /**
   * @param {!proto.resource.SetPermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetPermissionRsp.deserializeBinary
);


/**
 * @param {!proto.resource.SetPermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.SetPermissionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.SetPermissionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.setPermission =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/SetPermission',
      request,
      metadata || {},
      methodDescriptor_ResourceService_SetPermission,
      callback);
};


/**
 * @param {!proto.resource.SetPermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.SetPermissionRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.setPermission =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/SetPermission',
      request,
      metadata || {},
      methodDescriptor_ResourceService_SetPermission);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.DeletePermissionsRqst,
 *   !proto.resource.DeletePermissionsRsp>}
 */
const methodDescriptor_ResourceService_DeletePermissions = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/DeletePermissions',
  grpc.web.MethodType.UNARY,
  proto.resource.DeletePermissionsRqst,
  proto.resource.DeletePermissionsRsp,
  /**
   * @param {!proto.resource.DeletePermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeletePermissionsRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.DeletePermissionsRqst,
 *   !proto.resource.DeletePermissionsRsp>}
 */
const methodInfo_ResourceService_DeletePermissions = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.DeletePermissionsRsp,
  /**
   * @param {!proto.resource.DeletePermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeletePermissionsRsp.deserializeBinary
);


/**
 * @param {!proto.resource.DeletePermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.DeletePermissionsRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.DeletePermissionsRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.deletePermissions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/DeletePermissions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeletePermissions,
      callback);
};


/**
 * @param {!proto.resource.DeletePermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.DeletePermissionsRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.deletePermissions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/DeletePermissions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeletePermissions);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.SetResourceOwnerRqst,
 *   !proto.resource.SetResourceOwnerRsp>}
 */
const methodDescriptor_ResourceService_SetResourceOwner = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/SetResourceOwner',
  grpc.web.MethodType.UNARY,
  proto.resource.SetResourceOwnerRqst,
  proto.resource.SetResourceOwnerRsp,
  /**
   * @param {!proto.resource.SetResourceOwnerRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetResourceOwnerRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.SetResourceOwnerRqst,
 *   !proto.resource.SetResourceOwnerRsp>}
 */
const methodInfo_ResourceService_SetResourceOwner = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.SetResourceOwnerRsp,
  /**
   * @param {!proto.resource.SetResourceOwnerRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetResourceOwnerRsp.deserializeBinary
);


/**
 * @param {!proto.resource.SetResourceOwnerRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.SetResourceOwnerRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.SetResourceOwnerRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.setResourceOwner =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/SetResourceOwner',
      request,
      metadata || {},
      methodDescriptor_ResourceService_SetResourceOwner,
      callback);
};


/**
 * @param {!proto.resource.SetResourceOwnerRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.SetResourceOwnerRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.setResourceOwner =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/SetResourceOwner',
      request,
      metadata || {},
      methodDescriptor_ResourceService_SetResourceOwner);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetResourceOwnersRqst,
 *   !proto.resource.GetResourceOwnersRsp>}
 */
const methodDescriptor_ResourceService_GetResourceOwners = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetResourceOwners',
  grpc.web.MethodType.UNARY,
  proto.resource.GetResourceOwnersRqst,
  proto.resource.GetResourceOwnersRsp,
  /**
   * @param {!proto.resource.GetResourceOwnersRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetResourceOwnersRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetResourceOwnersRqst,
 *   !proto.resource.GetResourceOwnersRsp>}
 */
const methodInfo_ResourceService_GetResourceOwners = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetResourceOwnersRsp,
  /**
   * @param {!proto.resource.GetResourceOwnersRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetResourceOwnersRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetResourceOwnersRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.GetResourceOwnersRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetResourceOwnersRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getResourceOwners =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/GetResourceOwners',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetResourceOwners,
      callback);
};


/**
 * @param {!proto.resource.GetResourceOwnersRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.GetResourceOwnersRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.getResourceOwners =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/GetResourceOwners',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetResourceOwners);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.DeleteResourceOwnerRqst,
 *   !proto.resource.DeleteResourceOwnerRsp>}
 */
const methodDescriptor_ResourceService_DeleteResourceOwner = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/DeleteResourceOwner',
  grpc.web.MethodType.UNARY,
  proto.resource.DeleteResourceOwnerRqst,
  proto.resource.DeleteResourceOwnerRsp,
  /**
   * @param {!proto.resource.DeleteResourceOwnerRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteResourceOwnerRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.DeleteResourceOwnerRqst,
 *   !proto.resource.DeleteResourceOwnerRsp>}
 */
const methodInfo_ResourceService_DeleteResourceOwner = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.DeleteResourceOwnerRsp,
  /**
   * @param {!proto.resource.DeleteResourceOwnerRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteResourceOwnerRsp.deserializeBinary
);


/**
 * @param {!proto.resource.DeleteResourceOwnerRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.DeleteResourceOwnerRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.DeleteResourceOwnerRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.deleteResourceOwner =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/DeleteResourceOwner',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteResourceOwner,
      callback);
};


/**
 * @param {!proto.resource.DeleteResourceOwnerRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.DeleteResourceOwnerRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.deleteResourceOwner =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/DeleteResourceOwner',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteResourceOwner);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.DeleteResourceOwnersRqst,
 *   !proto.resource.DeleteResourceOwnersRsp>}
 */
const methodDescriptor_ResourceService_DeleteResourceOwners = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/DeleteResourceOwners',
  grpc.web.MethodType.UNARY,
  proto.resource.DeleteResourceOwnersRqst,
  proto.resource.DeleteResourceOwnersRsp,
  /**
   * @param {!proto.resource.DeleteResourceOwnersRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteResourceOwnersRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.DeleteResourceOwnersRqst,
 *   !proto.resource.DeleteResourceOwnersRsp>}
 */
const methodInfo_ResourceService_DeleteResourceOwners = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.DeleteResourceOwnersRsp,
  /**
   * @param {!proto.resource.DeleteResourceOwnersRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteResourceOwnersRsp.deserializeBinary
);


/**
 * @param {!proto.resource.DeleteResourceOwnersRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.DeleteResourceOwnersRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.DeleteResourceOwnersRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.deleteResourceOwners =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/DeleteResourceOwners',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteResourceOwners,
      callback);
};


/**
 * @param {!proto.resource.DeleteResourceOwnersRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.DeleteResourceOwnersRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.deleteResourceOwners =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/DeleteResourceOwners',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteResourceOwners);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetAllFilesInfoRqst,
 *   !proto.resource.GetAllFilesInfoRsp>}
 */
const methodDescriptor_ResourceService_GetAllFilesInfo = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetAllFilesInfo',
  grpc.web.MethodType.UNARY,
  proto.resource.GetAllFilesInfoRqst,
  proto.resource.GetAllFilesInfoRsp,
  /**
   * @param {!proto.resource.GetAllFilesInfoRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetAllFilesInfoRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetAllFilesInfoRqst,
 *   !proto.resource.GetAllFilesInfoRsp>}
 */
const methodInfo_ResourceService_GetAllFilesInfo = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetAllFilesInfoRsp,
  /**
   * @param {!proto.resource.GetAllFilesInfoRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetAllFilesInfoRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetAllFilesInfoRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.GetAllFilesInfoRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetAllFilesInfoRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getAllFilesInfo =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/GetAllFilesInfo',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetAllFilesInfo,
      callback);
};


/**
 * @param {!proto.resource.GetAllFilesInfoRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.GetAllFilesInfoRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.getAllFilesInfo =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/GetAllFilesInfo',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetAllFilesInfo);
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
 *   !proto.resource.ValidateUserResourceAccessRqst,
 *   !proto.resource.ValidateUserResourceAccessRsp>}
 */
const methodDescriptor_ResourceService_ValidateUserResourceAccess = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/ValidateUserResourceAccess',
  grpc.web.MethodType.UNARY,
  proto.resource.ValidateUserResourceAccessRqst,
  proto.resource.ValidateUserResourceAccessRsp,
  /**
   * @param {!proto.resource.ValidateUserResourceAccessRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ValidateUserResourceAccessRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.ValidateUserResourceAccessRqst,
 *   !proto.resource.ValidateUserResourceAccessRsp>}
 */
const methodInfo_ResourceService_ValidateUserResourceAccess = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.ValidateUserResourceAccessRsp,
  /**
   * @param {!proto.resource.ValidateUserResourceAccessRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ValidateUserResourceAccessRsp.deserializeBinary
);


/**
 * @param {!proto.resource.ValidateUserResourceAccessRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.ValidateUserResourceAccessRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.ValidateUserResourceAccessRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.validateUserResourceAccess =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/ValidateUserResourceAccess',
      request,
      metadata || {},
      methodDescriptor_ResourceService_ValidateUserResourceAccess,
      callback);
};


/**
 * @param {!proto.resource.ValidateUserResourceAccessRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.ValidateUserResourceAccessRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.validateUserResourceAccess =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/ValidateUserResourceAccess',
      request,
      metadata || {},
      methodDescriptor_ResourceService_ValidateUserResourceAccess);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.ValidateApplicationResourceAccessRqst,
 *   !proto.resource.ValidateApplicationResourceAccessRsp>}
 */
const methodDescriptor_ResourceService_ValidateApplicationResourceAccess = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/ValidateApplicationResourceAccess',
  grpc.web.MethodType.UNARY,
  proto.resource.ValidateApplicationResourceAccessRqst,
  proto.resource.ValidateApplicationResourceAccessRsp,
  /**
   * @param {!proto.resource.ValidateApplicationResourceAccessRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ValidateApplicationResourceAccessRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.ValidateApplicationResourceAccessRqst,
 *   !proto.resource.ValidateApplicationResourceAccessRsp>}
 */
const methodInfo_ResourceService_ValidateApplicationResourceAccess = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.ValidateApplicationResourceAccessRsp,
  /**
   * @param {!proto.resource.ValidateApplicationResourceAccessRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ValidateApplicationResourceAccessRsp.deserializeBinary
);


/**
 * @param {!proto.resource.ValidateApplicationResourceAccessRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.ValidateApplicationResourceAccessRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.ValidateApplicationResourceAccessRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.validateApplicationResourceAccess =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/ValidateApplicationResourceAccess',
      request,
      metadata || {},
      methodDescriptor_ResourceService_ValidateApplicationResourceAccess,
      callback);
};


/**
 * @param {!proto.resource.ValidateApplicationResourceAccessRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.ValidateApplicationResourceAccessRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.validateApplicationResourceAccess =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/ValidateApplicationResourceAccess',
      request,
      metadata || {},
      methodDescriptor_ResourceService_ValidateApplicationResourceAccess);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.ValidateUserAccessRqst,
 *   !proto.resource.ValidateUserAccessRsp>}
 */
const methodDescriptor_ResourceService_ValidateUserAccess = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/ValidateUserAccess',
  grpc.web.MethodType.UNARY,
  proto.resource.ValidateUserAccessRqst,
  proto.resource.ValidateUserAccessRsp,
  /**
   * @param {!proto.resource.ValidateUserAccessRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ValidateUserAccessRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.ValidateUserAccessRqst,
 *   !proto.resource.ValidateUserAccessRsp>}
 */
const methodInfo_ResourceService_ValidateUserAccess = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.ValidateUserAccessRsp,
  /**
   * @param {!proto.resource.ValidateUserAccessRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ValidateUserAccessRsp.deserializeBinary
);


/**
 * @param {!proto.resource.ValidateUserAccessRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.ValidateUserAccessRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.ValidateUserAccessRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.validateUserAccess =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/ValidateUserAccess',
      request,
      metadata || {},
      methodDescriptor_ResourceService_ValidateUserAccess,
      callback);
};


/**
 * @param {!proto.resource.ValidateUserAccessRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.ValidateUserAccessRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.validateUserAccess =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/ValidateUserAccess',
      request,
      metadata || {},
      methodDescriptor_ResourceService_ValidateUserAccess);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.ValidateApplicationAccessRqst,
 *   !proto.resource.ValidateApplicationAccessRsp>}
 */
const methodDescriptor_ResourceService_ValidateApplicationAccess = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/ValidateApplicationAccess',
  grpc.web.MethodType.UNARY,
  proto.resource.ValidateApplicationAccessRqst,
  proto.resource.ValidateApplicationAccessRsp,
  /**
   * @param {!proto.resource.ValidateApplicationAccessRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ValidateApplicationAccessRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.ValidateApplicationAccessRqst,
 *   !proto.resource.ValidateApplicationAccessRsp>}
 */
const methodInfo_ResourceService_ValidateApplicationAccess = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.ValidateApplicationAccessRsp,
  /**
   * @param {!proto.resource.ValidateApplicationAccessRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ValidateApplicationAccessRsp.deserializeBinary
);


/**
 * @param {!proto.resource.ValidateApplicationAccessRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.ValidateApplicationAccessRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.ValidateApplicationAccessRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.validateApplicationAccess =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/ValidateApplicationAccess',
      request,
      metadata || {},
      methodDescriptor_ResourceService_ValidateApplicationAccess,
      callback);
};


/**
 * @param {!proto.resource.ValidateApplicationAccessRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.ValidateApplicationAccessRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.validateApplicationAccess =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/ValidateApplicationAccess',
      request,
      metadata || {},
      methodDescriptor_ResourceService_ValidateApplicationAccess);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.ValidatePeerAccessRqst,
 *   !proto.resource.ValidatePeerAccessRsp>}
 */
const methodDescriptor_ResourceService_ValidatePeerAccess = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/ValidatePeerAccess',
  grpc.web.MethodType.UNARY,
  proto.resource.ValidatePeerAccessRqst,
  proto.resource.ValidatePeerAccessRsp,
  /**
   * @param {!proto.resource.ValidatePeerAccessRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ValidatePeerAccessRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.ValidatePeerAccessRqst,
 *   !proto.resource.ValidatePeerAccessRsp>}
 */
const methodInfo_ResourceService_ValidatePeerAccess = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.ValidatePeerAccessRsp,
  /**
   * @param {!proto.resource.ValidatePeerAccessRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ValidatePeerAccessRsp.deserializeBinary
);


/**
 * @param {!proto.resource.ValidatePeerAccessRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.ValidatePeerAccessRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.ValidatePeerAccessRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.validatePeerAccess =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/ValidatePeerAccess',
      request,
      metadata || {},
      methodDescriptor_ResourceService_ValidatePeerAccess,
      callback);
};


/**
 * @param {!proto.resource.ValidatePeerAccessRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.ValidatePeerAccessRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.validatePeerAccess =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/ValidatePeerAccess',
      request,
      metadata || {},
      methodDescriptor_ResourceService_ValidatePeerAccess);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.ValidatePeerResourceAccessRqst,
 *   !proto.resource.ValidatePeerResourceAccessRsp>}
 */
const methodDescriptor_ResourceService_ValidatePeerResourceAccess = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/ValidatePeerResourceAccess',
  grpc.web.MethodType.UNARY,
  proto.resource.ValidatePeerResourceAccessRqst,
  proto.resource.ValidatePeerResourceAccessRsp,
  /**
   * @param {!proto.resource.ValidatePeerResourceAccessRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ValidatePeerResourceAccessRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.ValidatePeerResourceAccessRqst,
 *   !proto.resource.ValidatePeerResourceAccessRsp>}
 */
const methodInfo_ResourceService_ValidatePeerResourceAccess = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.ValidatePeerResourceAccessRsp,
  /**
   * @param {!proto.resource.ValidatePeerResourceAccessRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.ValidatePeerResourceAccessRsp.deserializeBinary
);


/**
 * @param {!proto.resource.ValidatePeerResourceAccessRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.ValidatePeerResourceAccessRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.ValidatePeerResourceAccessRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.validatePeerResourceAccess =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/ValidatePeerResourceAccess',
      request,
      metadata || {},
      methodDescriptor_ResourceService_ValidatePeerResourceAccess,
      callback);
};


/**
 * @param {!proto.resource.ValidatePeerResourceAccessRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.ValidatePeerResourceAccessRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.validatePeerResourceAccess =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/ValidatePeerResourceAccess',
      request,
      metadata || {},
      methodDescriptor_ResourceService_ValidatePeerResourceAccess);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.DeleteAccountPermissionsRqst,
 *   !proto.resource.DeleteAccountPermissionsRsp>}
 */
const methodDescriptor_ResourceService_DeleteAccountPermissions = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/DeleteAccountPermissions',
  grpc.web.MethodType.UNARY,
  proto.resource.DeleteAccountPermissionsRqst,
  proto.resource.DeleteAccountPermissionsRsp,
  /**
   * @param {!proto.resource.DeleteAccountPermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteAccountPermissionsRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.DeleteAccountPermissionsRqst,
 *   !proto.resource.DeleteAccountPermissionsRsp>}
 */
const methodInfo_ResourceService_DeleteAccountPermissions = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.DeleteAccountPermissionsRsp,
  /**
   * @param {!proto.resource.DeleteAccountPermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteAccountPermissionsRsp.deserializeBinary
);


/**
 * @param {!proto.resource.DeleteAccountPermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.DeleteAccountPermissionsRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.DeleteAccountPermissionsRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.deleteAccountPermissions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/DeleteAccountPermissions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteAccountPermissions,
      callback);
};


/**
 * @param {!proto.resource.DeleteAccountPermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.DeleteAccountPermissionsRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.deleteAccountPermissions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/DeleteAccountPermissions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteAccountPermissions);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.DeleteRolePermissionsRqst,
 *   !proto.resource.DeleteRolePermissionsRsp>}
 */
const methodDescriptor_ResourceService_DeleteRolePermissions = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/DeleteRolePermissions',
  grpc.web.MethodType.UNARY,
  proto.resource.DeleteRolePermissionsRqst,
  proto.resource.DeleteRolePermissionsRsp,
  /**
   * @param {!proto.resource.DeleteRolePermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteRolePermissionsRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.DeleteRolePermissionsRqst,
 *   !proto.resource.DeleteRolePermissionsRsp>}
 */
const methodInfo_ResourceService_DeleteRolePermissions = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.DeleteRolePermissionsRsp,
  /**
   * @param {!proto.resource.DeleteRolePermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteRolePermissionsRsp.deserializeBinary
);


/**
 * @param {!proto.resource.DeleteRolePermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.DeleteRolePermissionsRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.DeleteRolePermissionsRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.deleteRolePermissions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/DeleteRolePermissions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteRolePermissions,
      callback);
};


/**
 * @param {!proto.resource.DeleteRolePermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.DeleteRolePermissionsRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.deleteRolePermissions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/DeleteRolePermissions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteRolePermissions);
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
 *   !proto.resource.LogRqst,
 *   !proto.resource.LogRsp>}
 */
const methodDescriptor_ResourceService_Log = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/Log',
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
const methodInfo_ResourceService_Log = new grpc.web.AbstractClientBase.MethodInfo(
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
proto.resource.ResourceServiceClient.prototype.log =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/Log',
      request,
      metadata || {},
      methodDescriptor_ResourceService_Log,
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
proto.resource.ResourceServicePromiseClient.prototype.log =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/Log',
      request,
      metadata || {},
      methodDescriptor_ResourceService_Log);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetLogRqst,
 *   !proto.resource.GetLogRsp>}
 */
const methodDescriptor_ResourceService_GetLog = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetLog',
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
const methodInfo_ResourceService_GetLog = new grpc.web.AbstractClientBase.MethodInfo(
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
proto.resource.ResourceServiceClient.prototype.getLog =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.ResourceService/GetLog',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetLog);
};


/**
 * @param {!proto.resource.GetLogRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetLogRsp>}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServicePromiseClient.prototype.getLog =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.ResourceService/GetLog',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetLog);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.DeleteLogRqst,
 *   !proto.resource.DeleteLogRsp>}
 */
const methodDescriptor_ResourceService_DeleteLog = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/DeleteLog',
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
const methodInfo_ResourceService_DeleteLog = new grpc.web.AbstractClientBase.MethodInfo(
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
proto.resource.ResourceServiceClient.prototype.deleteLog =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/DeleteLog',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteLog,
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
proto.resource.ResourceServicePromiseClient.prototype.deleteLog =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/DeleteLog',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteLog);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.ClearAllLogRqst,
 *   !proto.resource.ClearAllLogRsp>}
 */
const methodDescriptor_ResourceService_ClearAllLog = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/ClearAllLog',
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
const methodInfo_ResourceService_ClearAllLog = new grpc.web.AbstractClientBase.MethodInfo(
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
proto.resource.ResourceServiceClient.prototype.clearAllLog =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/ClearAllLog',
      request,
      metadata || {},
      methodDescriptor_ResourceService_ClearAllLog,
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
proto.resource.ResourceServicePromiseClient.prototype.clearAllLog =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/ClearAllLog',
      request,
      metadata || {},
      methodDescriptor_ResourceService_ClearAllLog);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetResourcesRqst,
 *   !proto.resource.GetResourcesRsp>}
 */
const methodDescriptor_ResourceService_GetResources = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetResources',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.resource.GetResourcesRqst,
  proto.resource.GetResourcesRsp,
  /**
   * @param {!proto.resource.GetResourcesRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetResourcesRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetResourcesRqst,
 *   !proto.resource.GetResourcesRsp>}
 */
const methodInfo_ResourceService_GetResources = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetResourcesRsp,
  /**
   * @param {!proto.resource.GetResourcesRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetResourcesRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetResourcesRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetResourcesRsp>}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getResources =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.ResourceService/GetResources',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetResources);
};


/**
 * @param {!proto.resource.GetResourcesRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetResourcesRsp>}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServicePromiseClient.prototype.getResources =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/resource.ResourceService/GetResources',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetResources);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.SetResourceRqst,
 *   !proto.resource.SetResourceRsp>}
 */
const methodDescriptor_ResourceService_SetResource = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/SetResource',
  grpc.web.MethodType.UNARY,
  proto.resource.SetResourceRqst,
  proto.resource.SetResourceRsp,
  /**
   * @param {!proto.resource.SetResourceRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetResourceRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.SetResourceRqst,
 *   !proto.resource.SetResourceRsp>}
 */
const methodInfo_ResourceService_SetResource = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.SetResourceRsp,
  /**
   * @param {!proto.resource.SetResourceRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetResourceRsp.deserializeBinary
);


/**
 * @param {!proto.resource.SetResourceRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.SetResourceRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.SetResourceRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.setResource =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/SetResource',
      request,
      metadata || {},
      methodDescriptor_ResourceService_SetResource,
      callback);
};


/**
 * @param {!proto.resource.SetResourceRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.SetResourceRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.setResource =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/SetResource',
      request,
      metadata || {},
      methodDescriptor_ResourceService_SetResource);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.RemoveResourceRqst,
 *   !proto.resource.RemoveResourceRsp>}
 */
const methodDescriptor_ResourceService_RemoveResource = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/RemoveResource',
  grpc.web.MethodType.UNARY,
  proto.resource.RemoveResourceRqst,
  proto.resource.RemoveResourceRsp,
  /**
   * @param {!proto.resource.RemoveResourceRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveResourceRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.RemoveResourceRqst,
 *   !proto.resource.RemoveResourceRsp>}
 */
const methodInfo_ResourceService_RemoveResource = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.RemoveResourceRsp,
  /**
   * @param {!proto.resource.RemoveResourceRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveResourceRsp.deserializeBinary
);


/**
 * @param {!proto.resource.RemoveResourceRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.RemoveResourceRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.RemoveResourceRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.removeResource =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/RemoveResource',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveResource,
      callback);
};


/**
 * @param {!proto.resource.RemoveResourceRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.RemoveResourceRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.removeResource =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/RemoveResource',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveResource);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.SetActionPermissionRqst,
 *   !proto.resource.SetActionPermissionRsp>}
 */
const methodDescriptor_ResourceService_SetActionPermission = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/SetActionPermission',
  grpc.web.MethodType.UNARY,
  proto.resource.SetActionPermissionRqst,
  proto.resource.SetActionPermissionRsp,
  /**
   * @param {!proto.resource.SetActionPermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetActionPermissionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.SetActionPermissionRqst,
 *   !proto.resource.SetActionPermissionRsp>}
 */
const methodInfo_ResourceService_SetActionPermission = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.SetActionPermissionRsp,
  /**
   * @param {!proto.resource.SetActionPermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.SetActionPermissionRsp.deserializeBinary
);


/**
 * @param {!proto.resource.SetActionPermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.SetActionPermissionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.SetActionPermissionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.setActionPermission =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/SetActionPermission',
      request,
      metadata || {},
      methodDescriptor_ResourceService_SetActionPermission,
      callback);
};


/**
 * @param {!proto.resource.SetActionPermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.SetActionPermissionRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.setActionPermission =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/SetActionPermission',
      request,
      metadata || {},
      methodDescriptor_ResourceService_SetActionPermission);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.RemoveActionPermissionRqst,
 *   !proto.resource.RemoveActionPermissionRsp>}
 */
const methodDescriptor_ResourceService_RemoveActionPermission = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/RemoveActionPermission',
  grpc.web.MethodType.UNARY,
  proto.resource.RemoveActionPermissionRqst,
  proto.resource.RemoveActionPermissionRsp,
  /**
   * @param {!proto.resource.RemoveActionPermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveActionPermissionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.RemoveActionPermissionRqst,
 *   !proto.resource.RemoveActionPermissionRsp>}
 */
const methodInfo_ResourceService_RemoveActionPermission = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.RemoveActionPermissionRsp,
  /**
   * @param {!proto.resource.RemoveActionPermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RemoveActionPermissionRsp.deserializeBinary
);


/**
 * @param {!proto.resource.RemoveActionPermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.RemoveActionPermissionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.RemoveActionPermissionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.removeActionPermission =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/RemoveActionPermission',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveActionPermission,
      callback);
};


/**
 * @param {!proto.resource.RemoveActionPermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.RemoveActionPermissionRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.removeActionPermission =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/RemoveActionPermission',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RemoveActionPermission);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.GetActionPermissionRqst,
 *   !proto.resource.GetActionPermissionRsp>}
 */
const methodDescriptor_ResourceService_GetActionPermission = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/GetActionPermission',
  grpc.web.MethodType.UNARY,
  proto.resource.GetActionPermissionRqst,
  proto.resource.GetActionPermissionRsp,
  /**
   * @param {!proto.resource.GetActionPermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetActionPermissionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.GetActionPermissionRqst,
 *   !proto.resource.GetActionPermissionRsp>}
 */
const methodInfo_ResourceService_GetActionPermission = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.GetActionPermissionRsp,
  /**
   * @param {!proto.resource.GetActionPermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.GetActionPermissionRsp.deserializeBinary
);


/**
 * @param {!proto.resource.GetActionPermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.GetActionPermissionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.GetActionPermissionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.getActionPermission =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/GetActionPermission',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetActionPermission,
      callback);
};


/**
 * @param {!proto.resource.GetActionPermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.GetActionPermissionRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.getActionPermission =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/GetActionPermission',
      request,
      metadata || {},
      methodDescriptor_ResourceService_GetActionPermission);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.CreateDirPermissionsRqst,
 *   !proto.resource.CreateDirPermissionsRsp>}
 */
const methodDescriptor_ResourceService_CreateDirPermissions = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/CreateDirPermissions',
  grpc.web.MethodType.UNARY,
  proto.resource.CreateDirPermissionsRqst,
  proto.resource.CreateDirPermissionsRsp,
  /**
   * @param {!proto.resource.CreateDirPermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.CreateDirPermissionsRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.CreateDirPermissionsRqst,
 *   !proto.resource.CreateDirPermissionsRsp>}
 */
const methodInfo_ResourceService_CreateDirPermissions = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.CreateDirPermissionsRsp,
  /**
   * @param {!proto.resource.CreateDirPermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.CreateDirPermissionsRsp.deserializeBinary
);


/**
 * @param {!proto.resource.CreateDirPermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.CreateDirPermissionsRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.CreateDirPermissionsRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.createDirPermissions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/CreateDirPermissions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_CreateDirPermissions,
      callback);
};


/**
 * @param {!proto.resource.CreateDirPermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.CreateDirPermissionsRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.createDirPermissions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/CreateDirPermissions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_CreateDirPermissions);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.RenameFilePermissionRqst,
 *   !proto.resource.RenameFilePermissionRsp>}
 */
const methodDescriptor_ResourceService_RenameFilePermission = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/RenameFilePermission',
  grpc.web.MethodType.UNARY,
  proto.resource.RenameFilePermissionRqst,
  proto.resource.RenameFilePermissionRsp,
  /**
   * @param {!proto.resource.RenameFilePermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RenameFilePermissionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.RenameFilePermissionRqst,
 *   !proto.resource.RenameFilePermissionRsp>}
 */
const methodInfo_ResourceService_RenameFilePermission = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.RenameFilePermissionRsp,
  /**
   * @param {!proto.resource.RenameFilePermissionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.RenameFilePermissionRsp.deserializeBinary
);


/**
 * @param {!proto.resource.RenameFilePermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.RenameFilePermissionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.RenameFilePermissionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.renameFilePermission =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/RenameFilePermission',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RenameFilePermission,
      callback);
};


/**
 * @param {!proto.resource.RenameFilePermissionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.RenameFilePermissionRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.renameFilePermission =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/RenameFilePermission',
      request,
      metadata || {},
      methodDescriptor_ResourceService_RenameFilePermission);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.DeleteDirPermissionsRqst,
 *   !proto.resource.DeleteDirPermissionsRsp>}
 */
const methodDescriptor_ResourceService_DeleteDirPermissions = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/DeleteDirPermissions',
  grpc.web.MethodType.UNARY,
  proto.resource.DeleteDirPermissionsRqst,
  proto.resource.DeleteDirPermissionsRsp,
  /**
   * @param {!proto.resource.DeleteDirPermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteDirPermissionsRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.DeleteDirPermissionsRqst,
 *   !proto.resource.DeleteDirPermissionsRsp>}
 */
const methodInfo_ResourceService_DeleteDirPermissions = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.DeleteDirPermissionsRsp,
  /**
   * @param {!proto.resource.DeleteDirPermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteDirPermissionsRsp.deserializeBinary
);


/**
 * @param {!proto.resource.DeleteDirPermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.DeleteDirPermissionsRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.DeleteDirPermissionsRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.deleteDirPermissions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/DeleteDirPermissions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteDirPermissions,
      callback);
};


/**
 * @param {!proto.resource.DeleteDirPermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.DeleteDirPermissionsRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.deleteDirPermissions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/DeleteDirPermissions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteDirPermissions);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.resource.DeleteFilePermissionsRqst,
 *   !proto.resource.DeleteFilePermissionsRsp>}
 */
const methodDescriptor_ResourceService_DeleteFilePermissions = new grpc.web.MethodDescriptor(
  '/resource.ResourceService/DeleteFilePermissions',
  grpc.web.MethodType.UNARY,
  proto.resource.DeleteFilePermissionsRqst,
  proto.resource.DeleteFilePermissionsRsp,
  /**
   * @param {!proto.resource.DeleteFilePermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteFilePermissionsRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.resource.DeleteFilePermissionsRqst,
 *   !proto.resource.DeleteFilePermissionsRsp>}
 */
const methodInfo_ResourceService_DeleteFilePermissions = new grpc.web.AbstractClientBase.MethodInfo(
  proto.resource.DeleteFilePermissionsRsp,
  /**
   * @param {!proto.resource.DeleteFilePermissionsRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.resource.DeleteFilePermissionsRsp.deserializeBinary
);


/**
 * @param {!proto.resource.DeleteFilePermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.resource.DeleteFilePermissionsRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.resource.DeleteFilePermissionsRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.resource.ResourceServiceClient.prototype.deleteFilePermissions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/resource.ResourceService/DeleteFilePermissions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteFilePermissions,
      callback);
};


/**
 * @param {!proto.resource.DeleteFilePermissionsRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.resource.DeleteFilePermissionsRsp>}
 *     A native promise that resolves to the response
 */
proto.resource.ResourceServicePromiseClient.prototype.deleteFilePermissions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/resource.ResourceService/DeleteFilePermissions',
      request,
      metadata || {},
      methodDescriptor_ResourceService_DeleteFilePermissions);
};


module.exports = proto.resource;

