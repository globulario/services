/**
 * @fileoverview gRPC-Web generated client stub for log
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.log = require('./log_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.log.LogServiceClient =
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
proto.log.LogServicePromiseClient =
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
 *   !proto.log.LogRqst,
 *   !proto.log.LogRsp>}
 */
const methodDescriptor_LogService_Log = new grpc.web.MethodDescriptor(
  '/log.LogService/Log',
  grpc.web.MethodType.UNARY,
  proto.log.LogRqst,
  proto.log.LogRsp,
  /**
   * @param {!proto.log.LogRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.log.LogRsp.deserializeBinary
);


/**
 * @param {!proto.log.LogRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.log.LogRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.log.LogRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.log.LogServiceClient.prototype.log =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/log.LogService/Log',
      request,
      metadata || {},
      methodDescriptor_LogService_Log,
      callback);
};


/**
 * @param {!proto.log.LogRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.log.LogRsp>}
 *     Promise that resolves to the response
 */
proto.log.LogServicePromiseClient.prototype.log =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/log.LogService/Log',
      request,
      metadata || {},
      methodDescriptor_LogService_Log);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.log.GetLogRqst,
 *   !proto.log.GetLogRsp>}
 */
const methodDescriptor_LogService_GetLog = new grpc.web.MethodDescriptor(
  '/log.LogService/GetLog',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.log.GetLogRqst,
  proto.log.GetLogRsp,
  /**
   * @param {!proto.log.GetLogRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.log.GetLogRsp.deserializeBinary
);


/**
 * @param {!proto.log.GetLogRqst} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.log.GetLogRsp>}
 *     The XHR Node Readable Stream
 */
proto.log.LogServiceClient.prototype.getLog =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/log.LogService/GetLog',
      request,
      metadata || {},
      methodDescriptor_LogService_GetLog);
};


/**
 * @param {!proto.log.GetLogRqst} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.log.GetLogRsp>}
 *     The XHR Node Readable Stream
 */
proto.log.LogServicePromiseClient.prototype.getLog =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/log.LogService/GetLog',
      request,
      metadata || {},
      methodDescriptor_LogService_GetLog);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.log.DeleteLogRqst,
 *   !proto.log.DeleteLogRsp>}
 */
const methodDescriptor_LogService_DeleteLog = new grpc.web.MethodDescriptor(
  '/log.LogService/DeleteLog',
  grpc.web.MethodType.UNARY,
  proto.log.DeleteLogRqst,
  proto.log.DeleteLogRsp,
  /**
   * @param {!proto.log.DeleteLogRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.log.DeleteLogRsp.deserializeBinary
);


/**
 * @param {!proto.log.DeleteLogRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.log.DeleteLogRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.log.DeleteLogRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.log.LogServiceClient.prototype.deleteLog =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/log.LogService/DeleteLog',
      request,
      metadata || {},
      methodDescriptor_LogService_DeleteLog,
      callback);
};


/**
 * @param {!proto.log.DeleteLogRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.log.DeleteLogRsp>}
 *     Promise that resolves to the response
 */
proto.log.LogServicePromiseClient.prototype.deleteLog =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/log.LogService/DeleteLog',
      request,
      metadata || {},
      methodDescriptor_LogService_DeleteLog);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.log.ClearAllLogRqst,
 *   !proto.log.ClearAllLogRsp>}
 */
const methodDescriptor_LogService_ClearAllLog = new grpc.web.MethodDescriptor(
  '/log.LogService/ClearAllLog',
  grpc.web.MethodType.UNARY,
  proto.log.ClearAllLogRqst,
  proto.log.ClearAllLogRsp,
  /**
   * @param {!proto.log.ClearAllLogRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.log.ClearAllLogRsp.deserializeBinary
);


/**
 * @param {!proto.log.ClearAllLogRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.log.ClearAllLogRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.log.ClearAllLogRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.log.LogServiceClient.prototype.clearAllLog =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/log.LogService/ClearAllLog',
      request,
      metadata || {},
      methodDescriptor_LogService_ClearAllLog,
      callback);
};


/**
 * @param {!proto.log.ClearAllLogRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.log.ClearAllLogRsp>}
 *     Promise that resolves to the response
 */
proto.log.LogServicePromiseClient.prototype.clearAllLog =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/log.LogService/ClearAllLog',
      request,
      metadata || {},
      methodDescriptor_LogService_ClearAllLog);
};


module.exports = proto.log;

