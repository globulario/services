/**
 * @fileoverview gRPC-Web generated client stub for event
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.event = require('./event_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.event.EventServiceClient =
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
proto.event.EventServicePromiseClient =
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
 *   !proto.event.StopRequest,
 *   !proto.event.StopResponse>}
 */
const methodDescriptor_EventService_Stop = new grpc.web.MethodDescriptor(
  '/event.EventService/Stop',
  grpc.web.MethodType.UNARY,
  proto.event.StopRequest,
  proto.event.StopResponse,
  /**
   * @param {!proto.event.StopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.event.StopResponse.deserializeBinary
);


/**
 * @param {!proto.event.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.event.StopResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.event.StopResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.event.EventServiceClient.prototype.stop =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/event.EventService/Stop',
      request,
      metadata || {},
      methodDescriptor_EventService_Stop,
      callback);
};


/**
 * @param {!proto.event.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.event.StopResponse>}
 *     Promise that resolves to the response
 */
proto.event.EventServicePromiseClient.prototype.stop =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/event.EventService/Stop',
      request,
      metadata || {},
      methodDescriptor_EventService_Stop);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.event.OnEventRequest,
 *   !proto.event.OnEventResponse>}
 */
const methodDescriptor_EventService_OnEvent = new grpc.web.MethodDescriptor(
  '/event.EventService/OnEvent',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.event.OnEventRequest,
  proto.event.OnEventResponse,
  /**
   * @param {!proto.event.OnEventRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.event.OnEventResponse.deserializeBinary
);


/**
 * @param {!proto.event.OnEventRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.event.OnEventResponse>}
 *     The XHR Node Readable Stream
 */
proto.event.EventServiceClient.prototype.onEvent =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/event.EventService/OnEvent',
      request,
      metadata || {},
      methodDescriptor_EventService_OnEvent);
};


/**
 * @param {!proto.event.OnEventRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.event.OnEventResponse>}
 *     The XHR Node Readable Stream
 */
proto.event.EventServicePromiseClient.prototype.onEvent =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/event.EventService/OnEvent',
      request,
      metadata || {},
      methodDescriptor_EventService_OnEvent);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.event.QuitRequest,
 *   !proto.event.QuitResponse>}
 */
const methodDescriptor_EventService_Quit = new grpc.web.MethodDescriptor(
  '/event.EventService/Quit',
  grpc.web.MethodType.UNARY,
  proto.event.QuitRequest,
  proto.event.QuitResponse,
  /**
   * @param {!proto.event.QuitRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.event.QuitResponse.deserializeBinary
);


/**
 * @param {!proto.event.QuitRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.event.QuitResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.event.QuitResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.event.EventServiceClient.prototype.quit =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/event.EventService/Quit',
      request,
      metadata || {},
      methodDescriptor_EventService_Quit,
      callback);
};


/**
 * @param {!proto.event.QuitRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.event.QuitResponse>}
 *     Promise that resolves to the response
 */
proto.event.EventServicePromiseClient.prototype.quit =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/event.EventService/Quit',
      request,
      metadata || {},
      methodDescriptor_EventService_Quit);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.event.SubscribeRequest,
 *   !proto.event.SubscribeResponse>}
 */
const methodDescriptor_EventService_Subscribe = new grpc.web.MethodDescriptor(
  '/event.EventService/Subscribe',
  grpc.web.MethodType.UNARY,
  proto.event.SubscribeRequest,
  proto.event.SubscribeResponse,
  /**
   * @param {!proto.event.SubscribeRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.event.SubscribeResponse.deserializeBinary
);


/**
 * @param {!proto.event.SubscribeRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.event.SubscribeResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.event.SubscribeResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.event.EventServiceClient.prototype.subscribe =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/event.EventService/Subscribe',
      request,
      metadata || {},
      methodDescriptor_EventService_Subscribe,
      callback);
};


/**
 * @param {!proto.event.SubscribeRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.event.SubscribeResponse>}
 *     Promise that resolves to the response
 */
proto.event.EventServicePromiseClient.prototype.subscribe =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/event.EventService/Subscribe',
      request,
      metadata || {},
      methodDescriptor_EventService_Subscribe);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.event.UnSubscribeRequest,
 *   !proto.event.UnSubscribeResponse>}
 */
const methodDescriptor_EventService_UnSubscribe = new grpc.web.MethodDescriptor(
  '/event.EventService/UnSubscribe',
  grpc.web.MethodType.UNARY,
  proto.event.UnSubscribeRequest,
  proto.event.UnSubscribeResponse,
  /**
   * @param {!proto.event.UnSubscribeRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.event.UnSubscribeResponse.deserializeBinary
);


/**
 * @param {!proto.event.UnSubscribeRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.event.UnSubscribeResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.event.UnSubscribeResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.event.EventServiceClient.prototype.unSubscribe =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/event.EventService/UnSubscribe',
      request,
      metadata || {},
      methodDescriptor_EventService_UnSubscribe,
      callback);
};


/**
 * @param {!proto.event.UnSubscribeRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.event.UnSubscribeResponse>}
 *     Promise that resolves to the response
 */
proto.event.EventServicePromiseClient.prototype.unSubscribe =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/event.EventService/UnSubscribe',
      request,
      metadata || {},
      methodDescriptor_EventService_UnSubscribe);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.event.PublishRequest,
 *   !proto.event.PublishResponse>}
 */
const methodDescriptor_EventService_Publish = new grpc.web.MethodDescriptor(
  '/event.EventService/Publish',
  grpc.web.MethodType.UNARY,
  proto.event.PublishRequest,
  proto.event.PublishResponse,
  /**
   * @param {!proto.event.PublishRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.event.PublishResponse.deserializeBinary
);


/**
 * @param {!proto.event.PublishRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.event.PublishResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.event.PublishResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.event.EventServiceClient.prototype.publish =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/event.EventService/Publish',
      request,
      metadata || {},
      methodDescriptor_EventService_Publish,
      callback);
};


/**
 * @param {!proto.event.PublishRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.event.PublishResponse>}
 *     Promise that resolves to the response
 */
proto.event.EventServicePromiseClient.prototype.publish =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/event.EventService/Publish',
      request,
      metadata || {},
      methodDescriptor_EventService_Publish);
};


module.exports = proto.event;

