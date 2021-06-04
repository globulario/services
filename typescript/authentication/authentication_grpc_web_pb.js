/**
 * @fileoverview gRPC-Web generated client stub for authentication
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.authentication = require('./authentication_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.authentication.AuthenticationServiceClient =
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
proto.authentication.AuthenticationServicePromiseClient =
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
 *   !proto.authentication.ValidateTokenRqst,
 *   !proto.authentication.ValidateTokenRsp>}
 */
const methodDescriptor_AuthenticationService_ValidateToken = new grpc.web.MethodDescriptor(
  '/authentication.AuthenticationService/ValidateToken',
  grpc.web.MethodType.UNARY,
  proto.authentication.ValidateTokenRqst,
  proto.authentication.ValidateTokenRsp,
  /**
   * @param {!proto.authentication.ValidateTokenRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.authentication.ValidateTokenRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.authentication.ValidateTokenRqst,
 *   !proto.authentication.ValidateTokenRsp>}
 */
const methodInfo_AuthenticationService_ValidateToken = new grpc.web.AbstractClientBase.MethodInfo(
  proto.authentication.ValidateTokenRsp,
  /**
   * @param {!proto.authentication.ValidateTokenRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.authentication.ValidateTokenRsp.deserializeBinary
);


/**
 * @param {!proto.authentication.ValidateTokenRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.authentication.ValidateTokenRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.authentication.ValidateTokenRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.authentication.AuthenticationServiceClient.prototype.validateToken =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/authentication.AuthenticationService/ValidateToken',
      request,
      metadata || {},
      methodDescriptor_AuthenticationService_ValidateToken,
      callback);
};


/**
 * @param {!proto.authentication.ValidateTokenRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.authentication.ValidateTokenRsp>}
 *     Promise that resolves to the response
 */
proto.authentication.AuthenticationServicePromiseClient.prototype.validateToken =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/authentication.AuthenticationService/ValidateToken',
      request,
      metadata || {},
      methodDescriptor_AuthenticationService_ValidateToken);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.authentication.RefreshTokenRqst,
 *   !proto.authentication.RefreshTokenRsp>}
 */
const methodDescriptor_AuthenticationService_RefreshToken = new grpc.web.MethodDescriptor(
  '/authentication.AuthenticationService/RefreshToken',
  grpc.web.MethodType.UNARY,
  proto.authentication.RefreshTokenRqst,
  proto.authentication.RefreshTokenRsp,
  /**
   * @param {!proto.authentication.RefreshTokenRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.authentication.RefreshTokenRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.authentication.RefreshTokenRqst,
 *   !proto.authentication.RefreshTokenRsp>}
 */
const methodInfo_AuthenticationService_RefreshToken = new grpc.web.AbstractClientBase.MethodInfo(
  proto.authentication.RefreshTokenRsp,
  /**
   * @param {!proto.authentication.RefreshTokenRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.authentication.RefreshTokenRsp.deserializeBinary
);


/**
 * @param {!proto.authentication.RefreshTokenRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.authentication.RefreshTokenRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.authentication.RefreshTokenRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.authentication.AuthenticationServiceClient.prototype.refreshToken =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/authentication.AuthenticationService/RefreshToken',
      request,
      metadata || {},
      methodDescriptor_AuthenticationService_RefreshToken,
      callback);
};


/**
 * @param {!proto.authentication.RefreshTokenRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.authentication.RefreshTokenRsp>}
 *     Promise that resolves to the response
 */
proto.authentication.AuthenticationServicePromiseClient.prototype.refreshToken =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/authentication.AuthenticationService/RefreshToken',
      request,
      metadata || {},
      methodDescriptor_AuthenticationService_RefreshToken);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.authentication.SetPasswordRequest,
 *   !proto.authentication.SetPasswordResponse>}
 */
const methodDescriptor_AuthenticationService_SetPassword = new grpc.web.MethodDescriptor(
  '/authentication.AuthenticationService/SetPassword',
  grpc.web.MethodType.UNARY,
  proto.authentication.SetPasswordRequest,
  proto.authentication.SetPasswordResponse,
  /**
   * @param {!proto.authentication.SetPasswordRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.authentication.SetPasswordResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.authentication.SetPasswordRequest,
 *   !proto.authentication.SetPasswordResponse>}
 */
const methodInfo_AuthenticationService_SetPassword = new grpc.web.AbstractClientBase.MethodInfo(
  proto.authentication.SetPasswordResponse,
  /**
   * @param {!proto.authentication.SetPasswordRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.authentication.SetPasswordResponse.deserializeBinary
);


/**
 * @param {!proto.authentication.SetPasswordRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.authentication.SetPasswordResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.authentication.SetPasswordResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.authentication.AuthenticationServiceClient.prototype.setPassword =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/authentication.AuthenticationService/SetPassword',
      request,
      metadata || {},
      methodDescriptor_AuthenticationService_SetPassword,
      callback);
};


/**
 * @param {!proto.authentication.SetPasswordRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.authentication.SetPasswordResponse>}
 *     Promise that resolves to the response
 */
proto.authentication.AuthenticationServicePromiseClient.prototype.setPassword =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/authentication.AuthenticationService/SetPassword',
      request,
      metadata || {},
      methodDescriptor_AuthenticationService_SetPassword);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.authentication.SetRootPasswordRequest,
 *   !proto.authentication.SetRootPasswordResponse>}
 */
const methodDescriptor_AuthenticationService_SetRootPassword = new grpc.web.MethodDescriptor(
  '/authentication.AuthenticationService/SetRootPassword',
  grpc.web.MethodType.UNARY,
  proto.authentication.SetRootPasswordRequest,
  proto.authentication.SetRootPasswordResponse,
  /**
   * @param {!proto.authentication.SetRootPasswordRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.authentication.SetRootPasswordResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.authentication.SetRootPasswordRequest,
 *   !proto.authentication.SetRootPasswordResponse>}
 */
const methodInfo_AuthenticationService_SetRootPassword = new grpc.web.AbstractClientBase.MethodInfo(
  proto.authentication.SetRootPasswordResponse,
  /**
   * @param {!proto.authentication.SetRootPasswordRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.authentication.SetRootPasswordResponse.deserializeBinary
);


/**
 * @param {!proto.authentication.SetRootPasswordRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.authentication.SetRootPasswordResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.authentication.SetRootPasswordResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.authentication.AuthenticationServiceClient.prototype.setRootPassword =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/authentication.AuthenticationService/SetRootPassword',
      request,
      metadata || {},
      methodDescriptor_AuthenticationService_SetRootPassword,
      callback);
};


/**
 * @param {!proto.authentication.SetRootPasswordRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.authentication.SetRootPasswordResponse>}
 *     Promise that resolves to the response
 */
proto.authentication.AuthenticationServicePromiseClient.prototype.setRootPassword =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/authentication.AuthenticationService/SetRootPassword',
      request,
      metadata || {},
      methodDescriptor_AuthenticationService_SetRootPassword);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.authentication.SetRootEmailRequest,
 *   !proto.authentication.SetRootEmailResponse>}
 */
const methodDescriptor_AuthenticationService_SetRootEmail = new grpc.web.MethodDescriptor(
  '/authentication.AuthenticationService/SetRootEmail',
  grpc.web.MethodType.UNARY,
  proto.authentication.SetRootEmailRequest,
  proto.authentication.SetRootEmailResponse,
  /**
   * @param {!proto.authentication.SetRootEmailRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.authentication.SetRootEmailResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.authentication.SetRootEmailRequest,
 *   !proto.authentication.SetRootEmailResponse>}
 */
const methodInfo_AuthenticationService_SetRootEmail = new grpc.web.AbstractClientBase.MethodInfo(
  proto.authentication.SetRootEmailResponse,
  /**
   * @param {!proto.authentication.SetRootEmailRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.authentication.SetRootEmailResponse.deserializeBinary
);


/**
 * @param {!proto.authentication.SetRootEmailRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.authentication.SetRootEmailResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.authentication.SetRootEmailResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.authentication.AuthenticationServiceClient.prototype.setRootEmail =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/authentication.AuthenticationService/SetRootEmail',
      request,
      metadata || {},
      methodDescriptor_AuthenticationService_SetRootEmail,
      callback);
};


/**
 * @param {!proto.authentication.SetRootEmailRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.authentication.SetRootEmailResponse>}
 *     Promise that resolves to the response
 */
proto.authentication.AuthenticationServicePromiseClient.prototype.setRootEmail =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/authentication.AuthenticationService/SetRootEmail',
      request,
      metadata || {},
      methodDescriptor_AuthenticationService_SetRootEmail);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.authentication.AuthenticateRqst,
 *   !proto.authentication.AuthenticateRsp>}
 */
const methodDescriptor_AuthenticationService_Authenticate = new grpc.web.MethodDescriptor(
  '/authentication.AuthenticationService/Authenticate',
  grpc.web.MethodType.UNARY,
  proto.authentication.AuthenticateRqst,
  proto.authentication.AuthenticateRsp,
  /**
   * @param {!proto.authentication.AuthenticateRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.authentication.AuthenticateRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.authentication.AuthenticateRqst,
 *   !proto.authentication.AuthenticateRsp>}
 */
const methodInfo_AuthenticationService_Authenticate = new grpc.web.AbstractClientBase.MethodInfo(
  proto.authentication.AuthenticateRsp,
  /**
   * @param {!proto.authentication.AuthenticateRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.authentication.AuthenticateRsp.deserializeBinary
);


/**
 * @param {!proto.authentication.AuthenticateRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.authentication.AuthenticateRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.authentication.AuthenticateRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.authentication.AuthenticationServiceClient.prototype.authenticate =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/authentication.AuthenticationService/Authenticate',
      request,
      metadata || {},
      methodDescriptor_AuthenticationService_Authenticate,
      callback);
};


/**
 * @param {!proto.authentication.AuthenticateRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.authentication.AuthenticateRsp>}
 *     Promise that resolves to the response
 */
proto.authentication.AuthenticationServicePromiseClient.prototype.authenticate =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/authentication.AuthenticationService/Authenticate',
      request,
      metadata || {},
      methodDescriptor_AuthenticationService_Authenticate);
};


module.exports = proto.authentication;

