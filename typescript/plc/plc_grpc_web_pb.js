/**
 * @fileoverview gRPC-Web generated client stub for plc
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.plc = require('./plc_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.plc.PlcServiceClient =
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
proto.plc.PlcServicePromiseClient =
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
 *   !proto.plc.StopRequest,
 *   !proto.plc.StopResponse>}
 */
const methodDescriptor_PlcService_Stop = new grpc.web.MethodDescriptor(
  '/plc.PlcService/Stop',
  grpc.web.MethodType.UNARY,
  proto.plc.StopRequest,
  proto.plc.StopResponse,
  /**
   * @param {!proto.plc.StopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc.StopResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.plc.StopRequest,
 *   !proto.plc.StopResponse>}
 */
const methodInfo_PlcService_Stop = new grpc.web.AbstractClientBase.MethodInfo(
  proto.plc.StopResponse,
  /**
   * @param {!proto.plc.StopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc.StopResponse.deserializeBinary
);


/**
 * @param {!proto.plc.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.plc.StopResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.plc.StopResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.plc.PlcServiceClient.prototype.stop =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/plc.PlcService/Stop',
      request,
      metadata || {},
      methodDescriptor_PlcService_Stop,
      callback);
};


/**
 * @param {!proto.plc.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.plc.StopResponse>}
 *     Promise that resolves to the response
 */
proto.plc.PlcServicePromiseClient.prototype.stop =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/plc.PlcService/Stop',
      request,
      metadata || {},
      methodDescriptor_PlcService_Stop);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.plc.CreateConnectionRqst,
 *   !proto.plc.CreateConnectionRsp>}
 */
const methodDescriptor_PlcService_CreateConnection = new grpc.web.MethodDescriptor(
  '/plc.PlcService/CreateConnection',
  grpc.web.MethodType.UNARY,
  proto.plc.CreateConnectionRqst,
  proto.plc.CreateConnectionRsp,
  /**
   * @param {!proto.plc.CreateConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc.CreateConnectionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.plc.CreateConnectionRqst,
 *   !proto.plc.CreateConnectionRsp>}
 */
const methodInfo_PlcService_CreateConnection = new grpc.web.AbstractClientBase.MethodInfo(
  proto.plc.CreateConnectionRsp,
  /**
   * @param {!proto.plc.CreateConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc.CreateConnectionRsp.deserializeBinary
);


/**
 * @param {!proto.plc.CreateConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.plc.CreateConnectionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.plc.CreateConnectionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.plc.PlcServiceClient.prototype.createConnection =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/plc.PlcService/CreateConnection',
      request,
      metadata || {},
      methodDescriptor_PlcService_CreateConnection,
      callback);
};


/**
 * @param {!proto.plc.CreateConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.plc.CreateConnectionRsp>}
 *     Promise that resolves to the response
 */
proto.plc.PlcServicePromiseClient.prototype.createConnection =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/plc.PlcService/CreateConnection',
      request,
      metadata || {},
      methodDescriptor_PlcService_CreateConnection);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.plc.GetConnectionRqst,
 *   !proto.plc.GetConnectionRsp>}
 */
const methodDescriptor_PlcService_GetConnection = new grpc.web.MethodDescriptor(
  '/plc.PlcService/GetConnection',
  grpc.web.MethodType.UNARY,
  proto.plc.GetConnectionRqst,
  proto.plc.GetConnectionRsp,
  /**
   * @param {!proto.plc.GetConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc.GetConnectionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.plc.GetConnectionRqst,
 *   !proto.plc.GetConnectionRsp>}
 */
const methodInfo_PlcService_GetConnection = new grpc.web.AbstractClientBase.MethodInfo(
  proto.plc.GetConnectionRsp,
  /**
   * @param {!proto.plc.GetConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc.GetConnectionRsp.deserializeBinary
);


/**
 * @param {!proto.plc.GetConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.plc.GetConnectionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.plc.GetConnectionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.plc.PlcServiceClient.prototype.getConnection =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/plc.PlcService/GetConnection',
      request,
      metadata || {},
      methodDescriptor_PlcService_GetConnection,
      callback);
};


/**
 * @param {!proto.plc.GetConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.plc.GetConnectionRsp>}
 *     Promise that resolves to the response
 */
proto.plc.PlcServicePromiseClient.prototype.getConnection =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/plc.PlcService/GetConnection',
      request,
      metadata || {},
      methodDescriptor_PlcService_GetConnection);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.plc.CloseConnectionRqst,
 *   !proto.plc.CloseConnectionRsp>}
 */
const methodDescriptor_PlcService_CloseConnection = new grpc.web.MethodDescriptor(
  '/plc.PlcService/CloseConnection',
  grpc.web.MethodType.UNARY,
  proto.plc.CloseConnectionRqst,
  proto.plc.CloseConnectionRsp,
  /**
   * @param {!proto.plc.CloseConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc.CloseConnectionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.plc.CloseConnectionRqst,
 *   !proto.plc.CloseConnectionRsp>}
 */
const methodInfo_PlcService_CloseConnection = new grpc.web.AbstractClientBase.MethodInfo(
  proto.plc.CloseConnectionRsp,
  /**
   * @param {!proto.plc.CloseConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc.CloseConnectionRsp.deserializeBinary
);


/**
 * @param {!proto.plc.CloseConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.plc.CloseConnectionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.plc.CloseConnectionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.plc.PlcServiceClient.prototype.closeConnection =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/plc.PlcService/CloseConnection',
      request,
      metadata || {},
      methodDescriptor_PlcService_CloseConnection,
      callback);
};


/**
 * @param {!proto.plc.CloseConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.plc.CloseConnectionRsp>}
 *     Promise that resolves to the response
 */
proto.plc.PlcServicePromiseClient.prototype.closeConnection =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/plc.PlcService/CloseConnection',
      request,
      metadata || {},
      methodDescriptor_PlcService_CloseConnection);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.plc.DeleteConnectionRqst,
 *   !proto.plc.DeleteConnectionRsp>}
 */
const methodDescriptor_PlcService_DeleteConnection = new grpc.web.MethodDescriptor(
  '/plc.PlcService/DeleteConnection',
  grpc.web.MethodType.UNARY,
  proto.plc.DeleteConnectionRqst,
  proto.plc.DeleteConnectionRsp,
  /**
   * @param {!proto.plc.DeleteConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc.DeleteConnectionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.plc.DeleteConnectionRqst,
 *   !proto.plc.DeleteConnectionRsp>}
 */
const methodInfo_PlcService_DeleteConnection = new grpc.web.AbstractClientBase.MethodInfo(
  proto.plc.DeleteConnectionRsp,
  /**
   * @param {!proto.plc.DeleteConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc.DeleteConnectionRsp.deserializeBinary
);


/**
 * @param {!proto.plc.DeleteConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.plc.DeleteConnectionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.plc.DeleteConnectionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.plc.PlcServiceClient.prototype.deleteConnection =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/plc.PlcService/DeleteConnection',
      request,
      metadata || {},
      methodDescriptor_PlcService_DeleteConnection,
      callback);
};


/**
 * @param {!proto.plc.DeleteConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.plc.DeleteConnectionRsp>}
 *     Promise that resolves to the response
 */
proto.plc.PlcServicePromiseClient.prototype.deleteConnection =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/plc.PlcService/DeleteConnection',
      request,
      metadata || {},
      methodDescriptor_PlcService_DeleteConnection);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.plc.ReadTagRqst,
 *   !proto.plc.ReadTagRsp>}
 */
const methodDescriptor_PlcService_ReadTag = new grpc.web.MethodDescriptor(
  '/plc.PlcService/ReadTag',
  grpc.web.MethodType.UNARY,
  proto.plc.ReadTagRqst,
  proto.plc.ReadTagRsp,
  /**
   * @param {!proto.plc.ReadTagRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc.ReadTagRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.plc.ReadTagRqst,
 *   !proto.plc.ReadTagRsp>}
 */
const methodInfo_PlcService_ReadTag = new grpc.web.AbstractClientBase.MethodInfo(
  proto.plc.ReadTagRsp,
  /**
   * @param {!proto.plc.ReadTagRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc.ReadTagRsp.deserializeBinary
);


/**
 * @param {!proto.plc.ReadTagRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.plc.ReadTagRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.plc.ReadTagRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.plc.PlcServiceClient.prototype.readTag =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/plc.PlcService/ReadTag',
      request,
      metadata || {},
      methodDescriptor_PlcService_ReadTag,
      callback);
};


/**
 * @param {!proto.plc.ReadTagRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.plc.ReadTagRsp>}
 *     Promise that resolves to the response
 */
proto.plc.PlcServicePromiseClient.prototype.readTag =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/plc.PlcService/ReadTag',
      request,
      metadata || {},
      methodDescriptor_PlcService_ReadTag);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.plc.WriteTagRqst,
 *   !proto.plc.WriteTagRsp>}
 */
const methodDescriptor_PlcService_WriteTag = new grpc.web.MethodDescriptor(
  '/plc.PlcService/WriteTag',
  grpc.web.MethodType.UNARY,
  proto.plc.WriteTagRqst,
  proto.plc.WriteTagRsp,
  /**
   * @param {!proto.plc.WriteTagRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc.WriteTagRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.plc.WriteTagRqst,
 *   !proto.plc.WriteTagRsp>}
 */
const methodInfo_PlcService_WriteTag = new grpc.web.AbstractClientBase.MethodInfo(
  proto.plc.WriteTagRsp,
  /**
   * @param {!proto.plc.WriteTagRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.plc.WriteTagRsp.deserializeBinary
);


/**
 * @param {!proto.plc.WriteTagRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.plc.WriteTagRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.plc.WriteTagRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.plc.PlcServiceClient.prototype.writeTag =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/plc.PlcService/WriteTag',
      request,
      metadata || {},
      methodDescriptor_PlcService_WriteTag,
      callback);
};


/**
 * @param {!proto.plc.WriteTagRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.plc.WriteTagRsp>}
 *     Promise that resolves to the response
 */
proto.plc.PlcServicePromiseClient.prototype.writeTag =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/plc.PlcService/WriteTag',
      request,
      metadata || {},
      methodDescriptor_PlcService_WriteTag);
};


module.exports = proto.plc;

