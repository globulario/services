/**
 * @fileoverview gRPC-Web generated client stub for sql
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.sql = require('./sql_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.sql.SqlServiceClient =
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
proto.sql.SqlServicePromiseClient =
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
 *   !proto.sql.StopRequest,
 *   !proto.sql.StopResponse>}
 */
const methodDescriptor_SqlService_Stop = new grpc.web.MethodDescriptor(
  '/sql.SqlService/Stop',
  grpc.web.MethodType.UNARY,
  proto.sql.StopRequest,
  proto.sql.StopResponse,
  /**
   * @param {!proto.sql.StopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.sql.StopResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.sql.StopRequest,
 *   !proto.sql.StopResponse>}
 */
const methodInfo_SqlService_Stop = new grpc.web.AbstractClientBase.MethodInfo(
  proto.sql.StopResponse,
  /**
   * @param {!proto.sql.StopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.sql.StopResponse.deserializeBinary
);


/**
 * @param {!proto.sql.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.sql.StopResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.sql.StopResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.sql.SqlServiceClient.prototype.stop =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/sql.SqlService/Stop',
      request,
      metadata || {},
      methodDescriptor_SqlService_Stop,
      callback);
};


/**
 * @param {!proto.sql.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.sql.StopResponse>}
 *     Promise that resolves to the response
 */
proto.sql.SqlServicePromiseClient.prototype.stop =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/sql.SqlService/Stop',
      request,
      metadata || {},
      methodDescriptor_SqlService_Stop);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.sql.CreateConnectionRqst,
 *   !proto.sql.CreateConnectionRsp>}
 */
const methodDescriptor_SqlService_CreateConnection = new grpc.web.MethodDescriptor(
  '/sql.SqlService/CreateConnection',
  grpc.web.MethodType.UNARY,
  proto.sql.CreateConnectionRqst,
  proto.sql.CreateConnectionRsp,
  /**
   * @param {!proto.sql.CreateConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.sql.CreateConnectionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.sql.CreateConnectionRqst,
 *   !proto.sql.CreateConnectionRsp>}
 */
const methodInfo_SqlService_CreateConnection = new grpc.web.AbstractClientBase.MethodInfo(
  proto.sql.CreateConnectionRsp,
  /**
   * @param {!proto.sql.CreateConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.sql.CreateConnectionRsp.deserializeBinary
);


/**
 * @param {!proto.sql.CreateConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.sql.CreateConnectionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.sql.CreateConnectionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.sql.SqlServiceClient.prototype.createConnection =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/sql.SqlService/CreateConnection',
      request,
      metadata || {},
      methodDescriptor_SqlService_CreateConnection,
      callback);
};


/**
 * @param {!proto.sql.CreateConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.sql.CreateConnectionRsp>}
 *     Promise that resolves to the response
 */
proto.sql.SqlServicePromiseClient.prototype.createConnection =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/sql.SqlService/CreateConnection',
      request,
      metadata || {},
      methodDescriptor_SqlService_CreateConnection);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.sql.DeleteConnectionRqst,
 *   !proto.sql.DeleteConnectionRsp>}
 */
const methodDescriptor_SqlService_DeleteConnection = new grpc.web.MethodDescriptor(
  '/sql.SqlService/DeleteConnection',
  grpc.web.MethodType.UNARY,
  proto.sql.DeleteConnectionRqst,
  proto.sql.DeleteConnectionRsp,
  /**
   * @param {!proto.sql.DeleteConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.sql.DeleteConnectionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.sql.DeleteConnectionRqst,
 *   !proto.sql.DeleteConnectionRsp>}
 */
const methodInfo_SqlService_DeleteConnection = new grpc.web.AbstractClientBase.MethodInfo(
  proto.sql.DeleteConnectionRsp,
  /**
   * @param {!proto.sql.DeleteConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.sql.DeleteConnectionRsp.deserializeBinary
);


/**
 * @param {!proto.sql.DeleteConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.sql.DeleteConnectionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.sql.DeleteConnectionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.sql.SqlServiceClient.prototype.deleteConnection =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/sql.SqlService/DeleteConnection',
      request,
      metadata || {},
      methodDescriptor_SqlService_DeleteConnection,
      callback);
};


/**
 * @param {!proto.sql.DeleteConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.sql.DeleteConnectionRsp>}
 *     Promise that resolves to the response
 */
proto.sql.SqlServicePromiseClient.prototype.deleteConnection =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/sql.SqlService/DeleteConnection',
      request,
      metadata || {},
      methodDescriptor_SqlService_DeleteConnection);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.sql.PingConnectionRqst,
 *   !proto.sql.PingConnectionRsp>}
 */
const methodDescriptor_SqlService_Ping = new grpc.web.MethodDescriptor(
  '/sql.SqlService/Ping',
  grpc.web.MethodType.UNARY,
  proto.sql.PingConnectionRqst,
  proto.sql.PingConnectionRsp,
  /**
   * @param {!proto.sql.PingConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.sql.PingConnectionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.sql.PingConnectionRqst,
 *   !proto.sql.PingConnectionRsp>}
 */
const methodInfo_SqlService_Ping = new grpc.web.AbstractClientBase.MethodInfo(
  proto.sql.PingConnectionRsp,
  /**
   * @param {!proto.sql.PingConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.sql.PingConnectionRsp.deserializeBinary
);


/**
 * @param {!proto.sql.PingConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.sql.PingConnectionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.sql.PingConnectionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.sql.SqlServiceClient.prototype.ping =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/sql.SqlService/Ping',
      request,
      metadata || {},
      methodDescriptor_SqlService_Ping,
      callback);
};


/**
 * @param {!proto.sql.PingConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.sql.PingConnectionRsp>}
 *     Promise that resolves to the response
 */
proto.sql.SqlServicePromiseClient.prototype.ping =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/sql.SqlService/Ping',
      request,
      metadata || {},
      methodDescriptor_SqlService_Ping);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.sql.QueryContextRqst,
 *   !proto.sql.QueryContextRsp>}
 */
const methodDescriptor_SqlService_QueryContext = new grpc.web.MethodDescriptor(
  '/sql.SqlService/QueryContext',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.sql.QueryContextRqst,
  proto.sql.QueryContextRsp,
  /**
   * @param {!proto.sql.QueryContextRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.sql.QueryContextRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.sql.QueryContextRqst,
 *   !proto.sql.QueryContextRsp>}
 */
const methodInfo_SqlService_QueryContext = new grpc.web.AbstractClientBase.MethodInfo(
  proto.sql.QueryContextRsp,
  /**
   * @param {!proto.sql.QueryContextRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.sql.QueryContextRsp.deserializeBinary
);


/**
 * @param {!proto.sql.QueryContextRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.sql.QueryContextRsp>}
 *     The XHR Node Readable Stream
 */
proto.sql.SqlServiceClient.prototype.queryContext =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/sql.SqlService/QueryContext',
      request,
      metadata || {},
      methodDescriptor_SqlService_QueryContext);
};


/**
 * @param {!proto.sql.QueryContextRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.sql.QueryContextRsp>}
 *     The XHR Node Readable Stream
 */
proto.sql.SqlServicePromiseClient.prototype.queryContext =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/sql.SqlService/QueryContext',
      request,
      metadata || {},
      methodDescriptor_SqlService_QueryContext);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.sql.ExecContextRqst,
 *   !proto.sql.ExecContextRsp>}
 */
const methodDescriptor_SqlService_ExecContext = new grpc.web.MethodDescriptor(
  '/sql.SqlService/ExecContext',
  grpc.web.MethodType.UNARY,
  proto.sql.ExecContextRqst,
  proto.sql.ExecContextRsp,
  /**
   * @param {!proto.sql.ExecContextRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.sql.ExecContextRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.sql.ExecContextRqst,
 *   !proto.sql.ExecContextRsp>}
 */
const methodInfo_SqlService_ExecContext = new grpc.web.AbstractClientBase.MethodInfo(
  proto.sql.ExecContextRsp,
  /**
   * @param {!proto.sql.ExecContextRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.sql.ExecContextRsp.deserializeBinary
);


/**
 * @param {!proto.sql.ExecContextRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.sql.ExecContextRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.sql.ExecContextRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.sql.SqlServiceClient.prototype.execContext =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/sql.SqlService/ExecContext',
      request,
      metadata || {},
      methodDescriptor_SqlService_ExecContext,
      callback);
};


/**
 * @param {!proto.sql.ExecContextRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.sql.ExecContextRsp>}
 *     Promise that resolves to the response
 */
proto.sql.SqlServicePromiseClient.prototype.execContext =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/sql.SqlService/ExecContext',
      request,
      metadata || {},
      methodDescriptor_SqlService_ExecContext);
};


module.exports = proto.sql;

