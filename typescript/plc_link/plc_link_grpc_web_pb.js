/**
 * @fileoverview gRPC-Web generated client stub for plc_link
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.plc_link = require('./plc_link_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.plc_link.PlcLinkServiceClient =
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
proto.plc_link.PlcLinkServicePromiseClient =
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
 *   !proto.plc_link.StopRequest,
 *   !proto.plc_link.StopResponse>}
 */
const methodDescriptor_PlcLinkService_Stop = new grpc.web.MethodDescriptor(
  '/plc_link.PlcLinkService/Stop',
  grpc.web.MethodType.UNARY,
  proto.plc_link.StopRequest,
  proto.plc_link.StopResponse,
  /**
   * @param {!proto.plc_link.StopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc_link.StopResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.plc_link.StopRequest,
 *   !proto.plc_link.StopResponse>}
 */
const methodInfo_PlcLinkService_Stop = new grpc.web.AbstractClientBase.MethodInfo(
  proto.plc_link.StopResponse,
  /**
   * @param {!proto.plc_link.StopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc_link.StopResponse.deserializeBinary
);


/**
 * @param {!proto.plc_link.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.plc_link.StopResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.plc_link.StopResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.plc_link.PlcLinkServiceClient.prototype.stop =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/plc_link.PlcLinkService/Stop',
      request,
      metadata || {},
      methodDescriptor_PlcLinkService_Stop,
      callback);
};


/**
 * @param {!proto.plc_link.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.plc_link.StopResponse>}
 *     A native promise that resolves to the response
 */
proto.plc_link.PlcLinkServicePromiseClient.prototype.stop =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/plc_link.PlcLinkService/Stop',
      request,
      metadata || {},
      methodDescriptor_PlcLinkService_Stop);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.plc_link.LinkRqst,
 *   !proto.plc_link.LinkRsp>}
 */
const methodDescriptor_PlcLinkService_Link = new grpc.web.MethodDescriptor(
  '/plc_link.PlcLinkService/Link',
  grpc.web.MethodType.UNARY,
  proto.plc_link.LinkRqst,
  proto.plc_link.LinkRsp,
  /**
   * @param {!proto.plc_link.LinkRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc_link.LinkRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.plc_link.LinkRqst,
 *   !proto.plc_link.LinkRsp>}
 */
const methodInfo_PlcLinkService_Link = new grpc.web.AbstractClientBase.MethodInfo(
  proto.plc_link.LinkRsp,
  /**
   * @param {!proto.plc_link.LinkRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc_link.LinkRsp.deserializeBinary
);


/**
 * @param {!proto.plc_link.LinkRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.plc_link.LinkRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.plc_link.LinkRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.plc_link.PlcLinkServiceClient.prototype.link =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/plc_link.PlcLinkService/Link',
      request,
      metadata || {},
      methodDescriptor_PlcLinkService_Link,
      callback);
};


/**
 * @param {!proto.plc_link.LinkRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.plc_link.LinkRsp>}
 *     A native promise that resolves to the response
 */
proto.plc_link.PlcLinkServicePromiseClient.prototype.link =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/plc_link.PlcLinkService/Link',
      request,
      metadata || {},
      methodDescriptor_PlcLinkService_Link);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.plc_link.UnLinkRqst,
 *   !proto.plc_link.UnLinkRsp>}
 */
const methodDescriptor_PlcLinkService_UnLink = new grpc.web.MethodDescriptor(
  '/plc_link.PlcLinkService/UnLink',
  grpc.web.MethodType.UNARY,
  proto.plc_link.UnLinkRqst,
  proto.plc_link.UnLinkRsp,
  /**
   * @param {!proto.plc_link.UnLinkRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc_link.UnLinkRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.plc_link.UnLinkRqst,
 *   !proto.plc_link.UnLinkRsp>}
 */
const methodInfo_PlcLinkService_UnLink = new grpc.web.AbstractClientBase.MethodInfo(
  proto.plc_link.UnLinkRsp,
  /**
   * @param {!proto.plc_link.UnLinkRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc_link.UnLinkRsp.deserializeBinary
);


/**
 * @param {!proto.plc_link.UnLinkRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.plc_link.UnLinkRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.plc_link.UnLinkRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.plc_link.PlcLinkServiceClient.prototype.unLink =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/plc_link.PlcLinkService/UnLink',
      request,
      metadata || {},
      methodDescriptor_PlcLinkService_UnLink,
      callback);
};


/**
 * @param {!proto.plc_link.UnLinkRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.plc_link.UnLinkRsp>}
 *     A native promise that resolves to the response
 */
proto.plc_link.PlcLinkServicePromiseClient.prototype.unLink =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/plc_link.PlcLinkService/UnLink',
      request,
      metadata || {},
      methodDescriptor_PlcLinkService_UnLink);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.plc_link.SuspendRqst,
 *   !proto.plc_link.SuspendRsp>}
 */
const methodDescriptor_PlcLinkService_Suspend = new grpc.web.MethodDescriptor(
  '/plc_link.PlcLinkService/Suspend',
  grpc.web.MethodType.UNARY,
  proto.plc_link.SuspendRqst,
  proto.plc_link.SuspendRsp,
  /**
   * @param {!proto.plc_link.SuspendRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc_link.SuspendRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.plc_link.SuspendRqst,
 *   !proto.plc_link.SuspendRsp>}
 */
const methodInfo_PlcLinkService_Suspend = new grpc.web.AbstractClientBase.MethodInfo(
  proto.plc_link.SuspendRsp,
  /**
   * @param {!proto.plc_link.SuspendRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc_link.SuspendRsp.deserializeBinary
);


/**
 * @param {!proto.plc_link.SuspendRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.plc_link.SuspendRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.plc_link.SuspendRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.plc_link.PlcLinkServiceClient.prototype.suspend =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/plc_link.PlcLinkService/Suspend',
      request,
      metadata || {},
      methodDescriptor_PlcLinkService_Suspend,
      callback);
};


/**
 * @param {!proto.plc_link.SuspendRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.plc_link.SuspendRsp>}
 *     A native promise that resolves to the response
 */
proto.plc_link.PlcLinkServicePromiseClient.prototype.suspend =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/plc_link.PlcLinkService/Suspend',
      request,
      metadata || {},
      methodDescriptor_PlcLinkService_Suspend);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.plc_link.ResumeRqst,
 *   !proto.plc_link.ResumeRsp>}
 */
const methodDescriptor_PlcLinkService_Resume = new grpc.web.MethodDescriptor(
  '/plc_link.PlcLinkService/Resume',
  grpc.web.MethodType.UNARY,
  proto.plc_link.ResumeRqst,
  proto.plc_link.ResumeRsp,
  /**
   * @param {!proto.plc_link.ResumeRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc_link.ResumeRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.plc_link.ResumeRqst,
 *   !proto.plc_link.ResumeRsp>}
 */
const methodInfo_PlcLinkService_Resume = new grpc.web.AbstractClientBase.MethodInfo(
  proto.plc_link.ResumeRsp,
  /**
   * @param {!proto.plc_link.ResumeRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc_link.ResumeRsp.deserializeBinary
);


/**
 * @param {!proto.plc_link.ResumeRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.plc_link.ResumeRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.plc_link.ResumeRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.plc_link.PlcLinkServiceClient.prototype.resume =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/plc_link.PlcLinkService/Resume',
      request,
      metadata || {},
      methodDescriptor_PlcLinkService_Resume,
      callback);
};


/**
 * @param {!proto.plc_link.ResumeRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.plc_link.ResumeRsp>}
 *     A native promise that resolves to the response
 */
proto.plc_link.PlcLinkServicePromiseClient.prototype.resume =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/plc_link.PlcLinkService/Resume',
      request,
      metadata || {},
      methodDescriptor_PlcLinkService_Resume);
};


module.exports = proto.plc_link;

