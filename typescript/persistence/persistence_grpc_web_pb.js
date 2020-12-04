/**
 * @fileoverview gRPC-Web generated client stub for persistence
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.persistence = require('./persistence_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.persistence.PersistenceServiceClient =
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
proto.persistence.PersistenceServicePromiseClient =
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
 *   !proto.persistence.StopRequest,
 *   !proto.persistence.StopResponse>}
 */
const methodDescriptor_PersistenceService_Stop = new grpc.web.MethodDescriptor(
  '/persistence.PersistenceService/Stop',
  grpc.web.MethodType.UNARY,
  proto.persistence.StopRequest,
  proto.persistence.StopResponse,
  /**
   * @param {!proto.persistence.StopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.StopResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.persistence.StopRequest,
 *   !proto.persistence.StopResponse>}
 */
const methodInfo_PersistenceService_Stop = new grpc.web.AbstractClientBase.MethodInfo(
  proto.persistence.StopResponse,
  /**
   * @param {!proto.persistence.StopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.StopResponse.deserializeBinary
);


/**
 * @param {!proto.persistence.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.persistence.StopResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.StopResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServiceClient.prototype.stop =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/persistence.PersistenceService/Stop',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_Stop,
      callback);
};


/**
 * @param {!proto.persistence.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.persistence.StopResponse>}
 *     A native promise that resolves to the response
 */
proto.persistence.PersistenceServicePromiseClient.prototype.stop =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/persistence.PersistenceService/Stop',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_Stop);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.persistence.CreateDatabaseRqst,
 *   !proto.persistence.CreateDatabaseRsp>}
 */
const methodDescriptor_PersistenceService_CreateDatabase = new grpc.web.MethodDescriptor(
  '/persistence.PersistenceService/CreateDatabase',
  grpc.web.MethodType.UNARY,
  proto.persistence.CreateDatabaseRqst,
  proto.persistence.CreateDatabaseRsp,
  /**
   * @param {!proto.persistence.CreateDatabaseRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.CreateDatabaseRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.persistence.CreateDatabaseRqst,
 *   !proto.persistence.CreateDatabaseRsp>}
 */
const methodInfo_PersistenceService_CreateDatabase = new grpc.web.AbstractClientBase.MethodInfo(
  proto.persistence.CreateDatabaseRsp,
  /**
   * @param {!proto.persistence.CreateDatabaseRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.CreateDatabaseRsp.deserializeBinary
);


/**
 * @param {!proto.persistence.CreateDatabaseRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.persistence.CreateDatabaseRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.CreateDatabaseRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServiceClient.prototype.createDatabase =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/persistence.PersistenceService/CreateDatabase',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_CreateDatabase,
      callback);
};


/**
 * @param {!proto.persistence.CreateDatabaseRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.persistence.CreateDatabaseRsp>}
 *     A native promise that resolves to the response
 */
proto.persistence.PersistenceServicePromiseClient.prototype.createDatabase =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/persistence.PersistenceService/CreateDatabase',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_CreateDatabase);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.persistence.ConnectRqst,
 *   !proto.persistence.ConnectRsp>}
 */
const methodDescriptor_PersistenceService_Connect = new grpc.web.MethodDescriptor(
  '/persistence.PersistenceService/Connect',
  grpc.web.MethodType.UNARY,
  proto.persistence.ConnectRqst,
  proto.persistence.ConnectRsp,
  /**
   * @param {!proto.persistence.ConnectRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.ConnectRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.persistence.ConnectRqst,
 *   !proto.persistence.ConnectRsp>}
 */
const methodInfo_PersistenceService_Connect = new grpc.web.AbstractClientBase.MethodInfo(
  proto.persistence.ConnectRsp,
  /**
   * @param {!proto.persistence.ConnectRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.ConnectRsp.deserializeBinary
);


/**
 * @param {!proto.persistence.ConnectRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.persistence.ConnectRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.ConnectRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServiceClient.prototype.connect =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/persistence.PersistenceService/Connect',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_Connect,
      callback);
};


/**
 * @param {!proto.persistence.ConnectRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.persistence.ConnectRsp>}
 *     A native promise that resolves to the response
 */
proto.persistence.PersistenceServicePromiseClient.prototype.connect =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/persistence.PersistenceService/Connect',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_Connect);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.persistence.DisconnectRqst,
 *   !proto.persistence.DisconnectRsp>}
 */
const methodDescriptor_PersistenceService_Disconnect = new grpc.web.MethodDescriptor(
  '/persistence.PersistenceService/Disconnect',
  grpc.web.MethodType.UNARY,
  proto.persistence.DisconnectRqst,
  proto.persistence.DisconnectRsp,
  /**
   * @param {!proto.persistence.DisconnectRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.DisconnectRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.persistence.DisconnectRqst,
 *   !proto.persistence.DisconnectRsp>}
 */
const methodInfo_PersistenceService_Disconnect = new grpc.web.AbstractClientBase.MethodInfo(
  proto.persistence.DisconnectRsp,
  /**
   * @param {!proto.persistence.DisconnectRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.DisconnectRsp.deserializeBinary
);


/**
 * @param {!proto.persistence.DisconnectRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.persistence.DisconnectRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.DisconnectRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServiceClient.prototype.disconnect =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/persistence.PersistenceService/Disconnect',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_Disconnect,
      callback);
};


/**
 * @param {!proto.persistence.DisconnectRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.persistence.DisconnectRsp>}
 *     A native promise that resolves to the response
 */
proto.persistence.PersistenceServicePromiseClient.prototype.disconnect =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/persistence.PersistenceService/Disconnect',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_Disconnect);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.persistence.DeleteDatabaseRqst,
 *   !proto.persistence.DeleteDatabaseRsp>}
 */
const methodDescriptor_PersistenceService_DeleteDatabase = new grpc.web.MethodDescriptor(
  '/persistence.PersistenceService/DeleteDatabase',
  grpc.web.MethodType.UNARY,
  proto.persistence.DeleteDatabaseRqst,
  proto.persistence.DeleteDatabaseRsp,
  /**
   * @param {!proto.persistence.DeleteDatabaseRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.DeleteDatabaseRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.persistence.DeleteDatabaseRqst,
 *   !proto.persistence.DeleteDatabaseRsp>}
 */
const methodInfo_PersistenceService_DeleteDatabase = new grpc.web.AbstractClientBase.MethodInfo(
  proto.persistence.DeleteDatabaseRsp,
  /**
   * @param {!proto.persistence.DeleteDatabaseRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.DeleteDatabaseRsp.deserializeBinary
);


/**
 * @param {!proto.persistence.DeleteDatabaseRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.persistence.DeleteDatabaseRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.DeleteDatabaseRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServiceClient.prototype.deleteDatabase =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/persistence.PersistenceService/DeleteDatabase',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_DeleteDatabase,
      callback);
};


/**
 * @param {!proto.persistence.DeleteDatabaseRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.persistence.DeleteDatabaseRsp>}
 *     A native promise that resolves to the response
 */
proto.persistence.PersistenceServicePromiseClient.prototype.deleteDatabase =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/persistence.PersistenceService/DeleteDatabase',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_DeleteDatabase);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.persistence.CreateCollectionRqst,
 *   !proto.persistence.CreateCollectionRsp>}
 */
const methodDescriptor_PersistenceService_CreateCollection = new grpc.web.MethodDescriptor(
  '/persistence.PersistenceService/CreateCollection',
  grpc.web.MethodType.UNARY,
  proto.persistence.CreateCollectionRqst,
  proto.persistence.CreateCollectionRsp,
  /**
   * @param {!proto.persistence.CreateCollectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.CreateCollectionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.persistence.CreateCollectionRqst,
 *   !proto.persistence.CreateCollectionRsp>}
 */
const methodInfo_PersistenceService_CreateCollection = new grpc.web.AbstractClientBase.MethodInfo(
  proto.persistence.CreateCollectionRsp,
  /**
   * @param {!proto.persistence.CreateCollectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.CreateCollectionRsp.deserializeBinary
);


/**
 * @param {!proto.persistence.CreateCollectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.persistence.CreateCollectionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.CreateCollectionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServiceClient.prototype.createCollection =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/persistence.PersistenceService/CreateCollection',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_CreateCollection,
      callback);
};


/**
 * @param {!proto.persistence.CreateCollectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.persistence.CreateCollectionRsp>}
 *     A native promise that resolves to the response
 */
proto.persistence.PersistenceServicePromiseClient.prototype.createCollection =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/persistence.PersistenceService/CreateCollection',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_CreateCollection);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.persistence.DeleteCollectionRqst,
 *   !proto.persistence.DeleteCollectionRsp>}
 */
const methodDescriptor_PersistenceService_DeleteCollection = new grpc.web.MethodDescriptor(
  '/persistence.PersistenceService/DeleteCollection',
  grpc.web.MethodType.UNARY,
  proto.persistence.DeleteCollectionRqst,
  proto.persistence.DeleteCollectionRsp,
  /**
   * @param {!proto.persistence.DeleteCollectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.DeleteCollectionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.persistence.DeleteCollectionRqst,
 *   !proto.persistence.DeleteCollectionRsp>}
 */
const methodInfo_PersistenceService_DeleteCollection = new grpc.web.AbstractClientBase.MethodInfo(
  proto.persistence.DeleteCollectionRsp,
  /**
   * @param {!proto.persistence.DeleteCollectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.DeleteCollectionRsp.deserializeBinary
);


/**
 * @param {!proto.persistence.DeleteCollectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.persistence.DeleteCollectionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.DeleteCollectionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServiceClient.prototype.deleteCollection =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/persistence.PersistenceService/DeleteCollection',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_DeleteCollection,
      callback);
};


/**
 * @param {!proto.persistence.DeleteCollectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.persistence.DeleteCollectionRsp>}
 *     A native promise that resolves to the response
 */
proto.persistence.PersistenceServicePromiseClient.prototype.deleteCollection =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/persistence.PersistenceService/DeleteCollection',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_DeleteCollection);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.persistence.CreateConnectionRqst,
 *   !proto.persistence.CreateConnectionRsp>}
 */
const methodDescriptor_PersistenceService_CreateConnection = new grpc.web.MethodDescriptor(
  '/persistence.PersistenceService/CreateConnection',
  grpc.web.MethodType.UNARY,
  proto.persistence.CreateConnectionRqst,
  proto.persistence.CreateConnectionRsp,
  /**
   * @param {!proto.persistence.CreateConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.CreateConnectionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.persistence.CreateConnectionRqst,
 *   !proto.persistence.CreateConnectionRsp>}
 */
const methodInfo_PersistenceService_CreateConnection = new grpc.web.AbstractClientBase.MethodInfo(
  proto.persistence.CreateConnectionRsp,
  /**
   * @param {!proto.persistence.CreateConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.CreateConnectionRsp.deserializeBinary
);


/**
 * @param {!proto.persistence.CreateConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.persistence.CreateConnectionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.CreateConnectionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServiceClient.prototype.createConnection =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/persistence.PersistenceService/CreateConnection',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_CreateConnection,
      callback);
};


/**
 * @param {!proto.persistence.CreateConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.persistence.CreateConnectionRsp>}
 *     A native promise that resolves to the response
 */
proto.persistence.PersistenceServicePromiseClient.prototype.createConnection =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/persistence.PersistenceService/CreateConnection',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_CreateConnection);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.persistence.DeleteConnectionRqst,
 *   !proto.persistence.DeleteConnectionRsp>}
 */
const methodDescriptor_PersistenceService_DeleteConnection = new grpc.web.MethodDescriptor(
  '/persistence.PersistenceService/DeleteConnection',
  grpc.web.MethodType.UNARY,
  proto.persistence.DeleteConnectionRqst,
  proto.persistence.DeleteConnectionRsp,
  /**
   * @param {!proto.persistence.DeleteConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.DeleteConnectionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.persistence.DeleteConnectionRqst,
 *   !proto.persistence.DeleteConnectionRsp>}
 */
const methodInfo_PersistenceService_DeleteConnection = new grpc.web.AbstractClientBase.MethodInfo(
  proto.persistence.DeleteConnectionRsp,
  /**
   * @param {!proto.persistence.DeleteConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.DeleteConnectionRsp.deserializeBinary
);


/**
 * @param {!proto.persistence.DeleteConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.persistence.DeleteConnectionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.DeleteConnectionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServiceClient.prototype.deleteConnection =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/persistence.PersistenceService/DeleteConnection',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_DeleteConnection,
      callback);
};


/**
 * @param {!proto.persistence.DeleteConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.persistence.DeleteConnectionRsp>}
 *     A native promise that resolves to the response
 */
proto.persistence.PersistenceServicePromiseClient.prototype.deleteConnection =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/persistence.PersistenceService/DeleteConnection',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_DeleteConnection);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.persistence.PingConnectionRqst,
 *   !proto.persistence.PingConnectionRsp>}
 */
const methodDescriptor_PersistenceService_Ping = new grpc.web.MethodDescriptor(
  '/persistence.PersistenceService/Ping',
  grpc.web.MethodType.UNARY,
  proto.persistence.PingConnectionRqst,
  proto.persistence.PingConnectionRsp,
  /**
   * @param {!proto.persistence.PingConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.PingConnectionRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.persistence.PingConnectionRqst,
 *   !proto.persistence.PingConnectionRsp>}
 */
const methodInfo_PersistenceService_Ping = new grpc.web.AbstractClientBase.MethodInfo(
  proto.persistence.PingConnectionRsp,
  /**
   * @param {!proto.persistence.PingConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.PingConnectionRsp.deserializeBinary
);


/**
 * @param {!proto.persistence.PingConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.persistence.PingConnectionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.PingConnectionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServiceClient.prototype.ping =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/persistence.PersistenceService/Ping',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_Ping,
      callback);
};


/**
 * @param {!proto.persistence.PingConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.persistence.PingConnectionRsp>}
 *     A native promise that resolves to the response
 */
proto.persistence.PersistenceServicePromiseClient.prototype.ping =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/persistence.PersistenceService/Ping',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_Ping);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.persistence.CountRqst,
 *   !proto.persistence.CountRsp>}
 */
const methodDescriptor_PersistenceService_Count = new grpc.web.MethodDescriptor(
  '/persistence.PersistenceService/Count',
  grpc.web.MethodType.UNARY,
  proto.persistence.CountRqst,
  proto.persistence.CountRsp,
  /**
   * @param {!proto.persistence.CountRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.CountRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.persistence.CountRqst,
 *   !proto.persistence.CountRsp>}
 */
const methodInfo_PersistenceService_Count = new grpc.web.AbstractClientBase.MethodInfo(
  proto.persistence.CountRsp,
  /**
   * @param {!proto.persistence.CountRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.CountRsp.deserializeBinary
);


/**
 * @param {!proto.persistence.CountRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.persistence.CountRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.CountRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServiceClient.prototype.count =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/persistence.PersistenceService/Count',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_Count,
      callback);
};


/**
 * @param {!proto.persistence.CountRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.persistence.CountRsp>}
 *     A native promise that resolves to the response
 */
proto.persistence.PersistenceServicePromiseClient.prototype.count =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/persistence.PersistenceService/Count',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_Count);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.persistence.InsertOneRqst,
 *   !proto.persistence.InsertOneRsp>}
 */
const methodDescriptor_PersistenceService_InsertOne = new grpc.web.MethodDescriptor(
  '/persistence.PersistenceService/InsertOne',
  grpc.web.MethodType.UNARY,
  proto.persistence.InsertOneRqst,
  proto.persistence.InsertOneRsp,
  /**
   * @param {!proto.persistence.InsertOneRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.InsertOneRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.persistence.InsertOneRqst,
 *   !proto.persistence.InsertOneRsp>}
 */
const methodInfo_PersistenceService_InsertOne = new grpc.web.AbstractClientBase.MethodInfo(
  proto.persistence.InsertOneRsp,
  /**
   * @param {!proto.persistence.InsertOneRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.InsertOneRsp.deserializeBinary
);


/**
 * @param {!proto.persistence.InsertOneRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.persistence.InsertOneRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.InsertOneRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServiceClient.prototype.insertOne =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/persistence.PersistenceService/InsertOne',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_InsertOne,
      callback);
};


/**
 * @param {!proto.persistence.InsertOneRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.persistence.InsertOneRsp>}
 *     A native promise that resolves to the response
 */
proto.persistence.PersistenceServicePromiseClient.prototype.insertOne =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/persistence.PersistenceService/InsertOne',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_InsertOne);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.persistence.FindRqst,
 *   !proto.persistence.FindResp>}
 */
const methodDescriptor_PersistenceService_Find = new grpc.web.MethodDescriptor(
  '/persistence.PersistenceService/Find',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.persistence.FindRqst,
  proto.persistence.FindResp,
  /**
   * @param {!proto.persistence.FindRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.FindResp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.persistence.FindRqst,
 *   !proto.persistence.FindResp>}
 */
const methodInfo_PersistenceService_Find = new grpc.web.AbstractClientBase.MethodInfo(
  proto.persistence.FindResp,
  /**
   * @param {!proto.persistence.FindRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.FindResp.deserializeBinary
);


/**
 * @param {!proto.persistence.FindRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.FindResp>}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServiceClient.prototype.find =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/persistence.PersistenceService/Find',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_Find);
};


/**
 * @param {!proto.persistence.FindRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.FindResp>}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServicePromiseClient.prototype.find =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/persistence.PersistenceService/Find',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_Find);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.persistence.FindOneRqst,
 *   !proto.persistence.FindOneResp>}
 */
const methodDescriptor_PersistenceService_FindOne = new grpc.web.MethodDescriptor(
  '/persistence.PersistenceService/FindOne',
  grpc.web.MethodType.UNARY,
  proto.persistence.FindOneRqst,
  proto.persistence.FindOneResp,
  /**
   * @param {!proto.persistence.FindOneRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.FindOneResp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.persistence.FindOneRqst,
 *   !proto.persistence.FindOneResp>}
 */
const methodInfo_PersistenceService_FindOne = new grpc.web.AbstractClientBase.MethodInfo(
  proto.persistence.FindOneResp,
  /**
   * @param {!proto.persistence.FindOneRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.FindOneResp.deserializeBinary
);


/**
 * @param {!proto.persistence.FindOneRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.persistence.FindOneResp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.FindOneResp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServiceClient.prototype.findOne =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/persistence.PersistenceService/FindOne',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_FindOne,
      callback);
};


/**
 * @param {!proto.persistence.FindOneRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.persistence.FindOneResp>}
 *     A native promise that resolves to the response
 */
proto.persistence.PersistenceServicePromiseClient.prototype.findOne =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/persistence.PersistenceService/FindOne',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_FindOne);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.persistence.AggregateRqst,
 *   !proto.persistence.AggregateResp>}
 */
const methodDescriptor_PersistenceService_Aggregate = new grpc.web.MethodDescriptor(
  '/persistence.PersistenceService/Aggregate',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.persistence.AggregateRqst,
  proto.persistence.AggregateResp,
  /**
   * @param {!proto.persistence.AggregateRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.AggregateResp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.persistence.AggregateRqst,
 *   !proto.persistence.AggregateResp>}
 */
const methodInfo_PersistenceService_Aggregate = new grpc.web.AbstractClientBase.MethodInfo(
  proto.persistence.AggregateResp,
  /**
   * @param {!proto.persistence.AggregateRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.AggregateResp.deserializeBinary
);


/**
 * @param {!proto.persistence.AggregateRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.AggregateResp>}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServiceClient.prototype.aggregate =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/persistence.PersistenceService/Aggregate',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_Aggregate);
};


/**
 * @param {!proto.persistence.AggregateRqst} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.AggregateResp>}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServicePromiseClient.prototype.aggregate =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/persistence.PersistenceService/Aggregate',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_Aggregate);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.persistence.UpdateRqst,
 *   !proto.persistence.UpdateRsp>}
 */
const methodDescriptor_PersistenceService_Update = new grpc.web.MethodDescriptor(
  '/persistence.PersistenceService/Update',
  grpc.web.MethodType.UNARY,
  proto.persistence.UpdateRqst,
  proto.persistence.UpdateRsp,
  /**
   * @param {!proto.persistence.UpdateRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.UpdateRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.persistence.UpdateRqst,
 *   !proto.persistence.UpdateRsp>}
 */
const methodInfo_PersistenceService_Update = new grpc.web.AbstractClientBase.MethodInfo(
  proto.persistence.UpdateRsp,
  /**
   * @param {!proto.persistence.UpdateRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.UpdateRsp.deserializeBinary
);


/**
 * @param {!proto.persistence.UpdateRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.persistence.UpdateRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.UpdateRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServiceClient.prototype.update =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/persistence.PersistenceService/Update',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_Update,
      callback);
};


/**
 * @param {!proto.persistence.UpdateRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.persistence.UpdateRsp>}
 *     A native promise that resolves to the response
 */
proto.persistence.PersistenceServicePromiseClient.prototype.update =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/persistence.PersistenceService/Update',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_Update);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.persistence.UpdateOneRqst,
 *   !proto.persistence.UpdateOneRsp>}
 */
const methodDescriptor_PersistenceService_UpdateOne = new grpc.web.MethodDescriptor(
  '/persistence.PersistenceService/UpdateOne',
  grpc.web.MethodType.UNARY,
  proto.persistence.UpdateOneRqst,
  proto.persistence.UpdateOneRsp,
  /**
   * @param {!proto.persistence.UpdateOneRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.UpdateOneRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.persistence.UpdateOneRqst,
 *   !proto.persistence.UpdateOneRsp>}
 */
const methodInfo_PersistenceService_UpdateOne = new grpc.web.AbstractClientBase.MethodInfo(
  proto.persistence.UpdateOneRsp,
  /**
   * @param {!proto.persistence.UpdateOneRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.UpdateOneRsp.deserializeBinary
);


/**
 * @param {!proto.persistence.UpdateOneRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.persistence.UpdateOneRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.UpdateOneRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServiceClient.prototype.updateOne =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/persistence.PersistenceService/UpdateOne',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_UpdateOne,
      callback);
};


/**
 * @param {!proto.persistence.UpdateOneRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.persistence.UpdateOneRsp>}
 *     A native promise that resolves to the response
 */
proto.persistence.PersistenceServicePromiseClient.prototype.updateOne =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/persistence.PersistenceService/UpdateOne',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_UpdateOne);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.persistence.ReplaceOneRqst,
 *   !proto.persistence.ReplaceOneRsp>}
 */
const methodDescriptor_PersistenceService_ReplaceOne = new grpc.web.MethodDescriptor(
  '/persistence.PersistenceService/ReplaceOne',
  grpc.web.MethodType.UNARY,
  proto.persistence.ReplaceOneRqst,
  proto.persistence.ReplaceOneRsp,
  /**
   * @param {!proto.persistence.ReplaceOneRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.ReplaceOneRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.persistence.ReplaceOneRqst,
 *   !proto.persistence.ReplaceOneRsp>}
 */
const methodInfo_PersistenceService_ReplaceOne = new grpc.web.AbstractClientBase.MethodInfo(
  proto.persistence.ReplaceOneRsp,
  /**
   * @param {!proto.persistence.ReplaceOneRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.ReplaceOneRsp.deserializeBinary
);


/**
 * @param {!proto.persistence.ReplaceOneRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.persistence.ReplaceOneRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.ReplaceOneRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServiceClient.prototype.replaceOne =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/persistence.PersistenceService/ReplaceOne',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_ReplaceOne,
      callback);
};


/**
 * @param {!proto.persistence.ReplaceOneRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.persistence.ReplaceOneRsp>}
 *     A native promise that resolves to the response
 */
proto.persistence.PersistenceServicePromiseClient.prototype.replaceOne =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/persistence.PersistenceService/ReplaceOne',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_ReplaceOne);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.persistence.DeleteRqst,
 *   !proto.persistence.DeleteRsp>}
 */
const methodDescriptor_PersistenceService_Delete = new grpc.web.MethodDescriptor(
  '/persistence.PersistenceService/Delete',
  grpc.web.MethodType.UNARY,
  proto.persistence.DeleteRqst,
  proto.persistence.DeleteRsp,
  /**
   * @param {!proto.persistence.DeleteRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.DeleteRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.persistence.DeleteRqst,
 *   !proto.persistence.DeleteRsp>}
 */
const methodInfo_PersistenceService_Delete = new grpc.web.AbstractClientBase.MethodInfo(
  proto.persistence.DeleteRsp,
  /**
   * @param {!proto.persistence.DeleteRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.DeleteRsp.deserializeBinary
);


/**
 * @param {!proto.persistence.DeleteRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.persistence.DeleteRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.DeleteRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServiceClient.prototype.delete =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/persistence.PersistenceService/Delete',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_Delete,
      callback);
};


/**
 * @param {!proto.persistence.DeleteRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.persistence.DeleteRsp>}
 *     A native promise that resolves to the response
 */
proto.persistence.PersistenceServicePromiseClient.prototype.delete =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/persistence.PersistenceService/Delete',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_Delete);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.persistence.DeleteOneRqst,
 *   !proto.persistence.DeleteOneRsp>}
 */
const methodDescriptor_PersistenceService_DeleteOne = new grpc.web.MethodDescriptor(
  '/persistence.PersistenceService/DeleteOne',
  grpc.web.MethodType.UNARY,
  proto.persistence.DeleteOneRqst,
  proto.persistence.DeleteOneRsp,
  /**
   * @param {!proto.persistence.DeleteOneRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.DeleteOneRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.persistence.DeleteOneRqst,
 *   !proto.persistence.DeleteOneRsp>}
 */
const methodInfo_PersistenceService_DeleteOne = new grpc.web.AbstractClientBase.MethodInfo(
  proto.persistence.DeleteOneRsp,
  /**
   * @param {!proto.persistence.DeleteOneRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.DeleteOneRsp.deserializeBinary
);


/**
 * @param {!proto.persistence.DeleteOneRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.persistence.DeleteOneRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.DeleteOneRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServiceClient.prototype.deleteOne =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/persistence.PersistenceService/DeleteOne',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_DeleteOne,
      callback);
};


/**
 * @param {!proto.persistence.DeleteOneRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.persistence.DeleteOneRsp>}
 *     A native promise that resolves to the response
 */
proto.persistence.PersistenceServicePromiseClient.prototype.deleteOne =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/persistence.PersistenceService/DeleteOne',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_DeleteOne);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.persistence.RunAdminCmdRqst,
 *   !proto.persistence.RunAdminCmdRsp>}
 */
const methodDescriptor_PersistenceService_RunAdminCmd = new grpc.web.MethodDescriptor(
  '/persistence.PersistenceService/RunAdminCmd',
  grpc.web.MethodType.UNARY,
  proto.persistence.RunAdminCmdRqst,
  proto.persistence.RunAdminCmdRsp,
  /**
   * @param {!proto.persistence.RunAdminCmdRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.RunAdminCmdRsp.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.persistence.RunAdminCmdRqst,
 *   !proto.persistence.RunAdminCmdRsp>}
 */
const methodInfo_PersistenceService_RunAdminCmd = new grpc.web.AbstractClientBase.MethodInfo(
  proto.persistence.RunAdminCmdRsp,
  /**
   * @param {!proto.persistence.RunAdminCmdRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.persistence.RunAdminCmdRsp.deserializeBinary
);


/**
 * @param {!proto.persistence.RunAdminCmdRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.persistence.RunAdminCmdRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.persistence.RunAdminCmdRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.persistence.PersistenceServiceClient.prototype.runAdminCmd =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/persistence.PersistenceService/RunAdminCmd',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_RunAdminCmd,
      callback);
};


/**
 * @param {!proto.persistence.RunAdminCmdRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.persistence.RunAdminCmdRsp>}
 *     A native promise that resolves to the response
 */
proto.persistence.PersistenceServicePromiseClient.prototype.runAdminCmd =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/persistence.PersistenceService/RunAdminCmd',
      request,
      metadata || {},
      methodDescriptor_PersistenceService_RunAdminCmd);
};


module.exports = proto.persistence;

