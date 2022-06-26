/**
 * @fileoverview gRPC-Web generated client stub for spc
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.spc = require('./spc_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.spc.SpcServiceClient =
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
proto.spc.SpcServicePromiseClient =
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
 *   !proto.spc.StopRequest,
 *   !proto.spc.StopResponse>}
 */
const methodDescriptor_SpcService_Stop = new grpc.web.MethodDescriptor(
  '/spc.SpcService/Stop',
  grpc.web.MethodType.UNARY,
  proto.spc.StopRequest,
  proto.spc.StopResponse,
  /**
   * @param {!proto.spc.StopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.spc.StopResponse.deserializeBinary
);


/**
 * @param {!proto.spc.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.spc.StopResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.spc.StopResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.spc.SpcServiceClient.prototype.stop =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/spc.SpcService/Stop',
      request,
      metadata || {},
      methodDescriptor_SpcService_Stop,
      callback);
};


/**
 * @param {!proto.spc.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.spc.StopResponse>}
 *     Promise that resolves to the response
 */
proto.spc.SpcServicePromiseClient.prototype.stop =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/spc.SpcService/Stop',
      request,
      metadata || {},
      methodDescriptor_SpcService_Stop);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.spc.CreateAnalyseRqst,
 *   !proto.spc.CreateAnalyseRsp>}
 */
const methodDescriptor_SpcService_CreateAnalyse = new grpc.web.MethodDescriptor(
  '/spc.SpcService/CreateAnalyse',
  grpc.web.MethodType.UNARY,
  proto.spc.CreateAnalyseRqst,
  proto.spc.CreateAnalyseRsp,
  /**
   * @param {!proto.spc.CreateAnalyseRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.spc.CreateAnalyseRsp.deserializeBinary
);


/**
 * @param {!proto.spc.CreateAnalyseRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.spc.CreateAnalyseRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.spc.CreateAnalyseRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.spc.SpcServiceClient.prototype.createAnalyse =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/spc.SpcService/CreateAnalyse',
      request,
      metadata || {},
      methodDescriptor_SpcService_CreateAnalyse,
      callback);
};


/**
 * @param {!proto.spc.CreateAnalyseRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.spc.CreateAnalyseRsp>}
 *     Promise that resolves to the response
 */
proto.spc.SpcServicePromiseClient.prototype.createAnalyse =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/spc.SpcService/CreateAnalyse',
      request,
      metadata || {},
      methodDescriptor_SpcService_CreateAnalyse);
};


module.exports = proto.spc;

