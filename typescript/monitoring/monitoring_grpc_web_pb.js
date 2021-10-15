/**
 * @fileoverview gRPC-Web generated client stub for monitoring
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.monitoring = require('./monitoring_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.monitoring.MonitoringServiceClient =
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
proto.monitoring.MonitoringServicePromiseClient =
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
 *   !proto.monitoring.StopRequest,
 *   !proto.monitoring.StopResponse>}
 */
const methodDescriptor_MonitoringService_Stop = new grpc.web.MethodDescriptor(
  '/monitoring.MonitoringService/Stop',
  grpc.web.MethodType.UNARY,
  proto.monitoring.StopRequest,
  proto.monitoring.StopResponse,
  /**
   * @param {!proto.monitoring.StopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.monitoring.StopResponse.deserializeBinary
);


/**
 * @param {!proto.monitoring.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.monitoring.StopResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.monitoring.StopResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.monitoring.MonitoringServiceClient.prototype.stop =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/monitoring.MonitoringService/Stop',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_Stop,
      callback);
};


/**
 * @param {!proto.monitoring.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.monitoring.StopResponse>}
 *     Promise that resolves to the response
 */
proto.monitoring.MonitoringServicePromiseClient.prototype.stop =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/monitoring.MonitoringService/Stop',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_Stop);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.monitoring.CreateConnectionRqst,
 *   !proto.monitoring.CreateConnectionRsp>}
 */
const methodDescriptor_MonitoringService_CreateConnection = new grpc.web.MethodDescriptor(
  '/monitoring.MonitoringService/CreateConnection',
  grpc.web.MethodType.UNARY,
  proto.monitoring.CreateConnectionRqst,
  proto.monitoring.CreateConnectionRsp,
  /**
   * @param {!proto.monitoring.CreateConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.monitoring.CreateConnectionRsp.deserializeBinary
);


/**
 * @param {!proto.monitoring.CreateConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.monitoring.CreateConnectionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.monitoring.CreateConnectionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.monitoring.MonitoringServiceClient.prototype.createConnection =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/monitoring.MonitoringService/CreateConnection',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_CreateConnection,
      callback);
};


/**
 * @param {!proto.monitoring.CreateConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.monitoring.CreateConnectionRsp>}
 *     Promise that resolves to the response
 */
proto.monitoring.MonitoringServicePromiseClient.prototype.createConnection =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/monitoring.MonitoringService/CreateConnection',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_CreateConnection);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.monitoring.DeleteConnectionRqst,
 *   !proto.monitoring.DeleteConnectionRsp>}
 */
const methodDescriptor_MonitoringService_DeleteConnection = new grpc.web.MethodDescriptor(
  '/monitoring.MonitoringService/DeleteConnection',
  grpc.web.MethodType.UNARY,
  proto.monitoring.DeleteConnectionRqst,
  proto.monitoring.DeleteConnectionRsp,
  /**
   * @param {!proto.monitoring.DeleteConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.monitoring.DeleteConnectionRsp.deserializeBinary
);


/**
 * @param {!proto.monitoring.DeleteConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.monitoring.DeleteConnectionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.monitoring.DeleteConnectionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.monitoring.MonitoringServiceClient.prototype.deleteConnection =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/monitoring.MonitoringService/DeleteConnection',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_DeleteConnection,
      callback);
};


/**
 * @param {!proto.monitoring.DeleteConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.monitoring.DeleteConnectionRsp>}
 *     Promise that resolves to the response
 */
proto.monitoring.MonitoringServicePromiseClient.prototype.deleteConnection =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/monitoring.MonitoringService/DeleteConnection',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_DeleteConnection);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.monitoring.AlertsRequest,
 *   !proto.monitoring.AlertsResponse>}
 */
const methodDescriptor_MonitoringService_Alerts = new grpc.web.MethodDescriptor(
  '/monitoring.MonitoringService/Alerts',
  grpc.web.MethodType.UNARY,
  proto.monitoring.AlertsRequest,
  proto.monitoring.AlertsResponse,
  /**
   * @param {!proto.monitoring.AlertsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.monitoring.AlertsResponse.deserializeBinary
);


/**
 * @param {!proto.monitoring.AlertsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.monitoring.AlertsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.monitoring.AlertsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.monitoring.MonitoringServiceClient.prototype.alerts =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/monitoring.MonitoringService/Alerts',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_Alerts,
      callback);
};


/**
 * @param {!proto.monitoring.AlertsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.monitoring.AlertsResponse>}
 *     Promise that resolves to the response
 */
proto.monitoring.MonitoringServicePromiseClient.prototype.alerts =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/monitoring.MonitoringService/Alerts',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_Alerts);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.monitoring.AlertManagersRequest,
 *   !proto.monitoring.AlertManagersResponse>}
 */
const methodDescriptor_MonitoringService_AlertManagers = new grpc.web.MethodDescriptor(
  '/monitoring.MonitoringService/AlertManagers',
  grpc.web.MethodType.UNARY,
  proto.monitoring.AlertManagersRequest,
  proto.monitoring.AlertManagersResponse,
  /**
   * @param {!proto.monitoring.AlertManagersRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.monitoring.AlertManagersResponse.deserializeBinary
);


/**
 * @param {!proto.monitoring.AlertManagersRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.monitoring.AlertManagersResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.monitoring.AlertManagersResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.monitoring.MonitoringServiceClient.prototype.alertManagers =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/monitoring.MonitoringService/AlertManagers',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_AlertManagers,
      callback);
};


/**
 * @param {!proto.monitoring.AlertManagersRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.monitoring.AlertManagersResponse>}
 *     Promise that resolves to the response
 */
proto.monitoring.MonitoringServicePromiseClient.prototype.alertManagers =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/monitoring.MonitoringService/AlertManagers',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_AlertManagers);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.monitoring.CleanTombstonesRequest,
 *   !proto.monitoring.CleanTombstonesResponse>}
 */
const methodDescriptor_MonitoringService_CleanTombstones = new grpc.web.MethodDescriptor(
  '/monitoring.MonitoringService/CleanTombstones',
  grpc.web.MethodType.UNARY,
  proto.monitoring.CleanTombstonesRequest,
  proto.monitoring.CleanTombstonesResponse,
  /**
   * @param {!proto.monitoring.CleanTombstonesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.monitoring.CleanTombstonesResponse.deserializeBinary
);


/**
 * @param {!proto.monitoring.CleanTombstonesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.monitoring.CleanTombstonesResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.monitoring.CleanTombstonesResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.monitoring.MonitoringServiceClient.prototype.cleanTombstones =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/monitoring.MonitoringService/CleanTombstones',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_CleanTombstones,
      callback);
};


/**
 * @param {!proto.monitoring.CleanTombstonesRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.monitoring.CleanTombstonesResponse>}
 *     Promise that resolves to the response
 */
proto.monitoring.MonitoringServicePromiseClient.prototype.cleanTombstones =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/monitoring.MonitoringService/CleanTombstones',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_CleanTombstones);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.monitoring.ConfigRequest,
 *   !proto.monitoring.ConfigResponse>}
 */
const methodDescriptor_MonitoringService_Config = new grpc.web.MethodDescriptor(
  '/monitoring.MonitoringService/Config',
  grpc.web.MethodType.UNARY,
  proto.monitoring.ConfigRequest,
  proto.monitoring.ConfigResponse,
  /**
   * @param {!proto.monitoring.ConfigRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.monitoring.ConfigResponse.deserializeBinary
);


/**
 * @param {!proto.monitoring.ConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.monitoring.ConfigResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.monitoring.ConfigResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.monitoring.MonitoringServiceClient.prototype.config =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/monitoring.MonitoringService/Config',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_Config,
      callback);
};


/**
 * @param {!proto.monitoring.ConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.monitoring.ConfigResponse>}
 *     Promise that resolves to the response
 */
proto.monitoring.MonitoringServicePromiseClient.prototype.config =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/monitoring.MonitoringService/Config',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_Config);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.monitoring.DeleteSeriesRequest,
 *   !proto.monitoring.DeleteSeriesResponse>}
 */
const methodDescriptor_MonitoringService_DeleteSeries = new grpc.web.MethodDescriptor(
  '/monitoring.MonitoringService/DeleteSeries',
  grpc.web.MethodType.UNARY,
  proto.monitoring.DeleteSeriesRequest,
  proto.monitoring.DeleteSeriesResponse,
  /**
   * @param {!proto.monitoring.DeleteSeriesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.monitoring.DeleteSeriesResponse.deserializeBinary
);


/**
 * @param {!proto.monitoring.DeleteSeriesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.monitoring.DeleteSeriesResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.monitoring.DeleteSeriesResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.monitoring.MonitoringServiceClient.prototype.deleteSeries =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/monitoring.MonitoringService/DeleteSeries',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_DeleteSeries,
      callback);
};


/**
 * @param {!proto.monitoring.DeleteSeriesRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.monitoring.DeleteSeriesResponse>}
 *     Promise that resolves to the response
 */
proto.monitoring.MonitoringServicePromiseClient.prototype.deleteSeries =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/monitoring.MonitoringService/DeleteSeries',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_DeleteSeries);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.monitoring.FlagsRequest,
 *   !proto.monitoring.FlagsResponse>}
 */
const methodDescriptor_MonitoringService_Flags = new grpc.web.MethodDescriptor(
  '/monitoring.MonitoringService/Flags',
  grpc.web.MethodType.UNARY,
  proto.monitoring.FlagsRequest,
  proto.monitoring.FlagsResponse,
  /**
   * @param {!proto.monitoring.FlagsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.monitoring.FlagsResponse.deserializeBinary
);


/**
 * @param {!proto.monitoring.FlagsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.monitoring.FlagsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.monitoring.FlagsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.monitoring.MonitoringServiceClient.prototype.flags =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/monitoring.MonitoringService/Flags',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_Flags,
      callback);
};


/**
 * @param {!proto.monitoring.FlagsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.monitoring.FlagsResponse>}
 *     Promise that resolves to the response
 */
proto.monitoring.MonitoringServicePromiseClient.prototype.flags =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/monitoring.MonitoringService/Flags',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_Flags);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.monitoring.LabelNamesRequest,
 *   !proto.monitoring.LabelNamesResponse>}
 */
const methodDescriptor_MonitoringService_LabelNames = new grpc.web.MethodDescriptor(
  '/monitoring.MonitoringService/LabelNames',
  grpc.web.MethodType.UNARY,
  proto.monitoring.LabelNamesRequest,
  proto.monitoring.LabelNamesResponse,
  /**
   * @param {!proto.monitoring.LabelNamesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.monitoring.LabelNamesResponse.deserializeBinary
);


/**
 * @param {!proto.monitoring.LabelNamesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.monitoring.LabelNamesResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.monitoring.LabelNamesResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.monitoring.MonitoringServiceClient.prototype.labelNames =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/monitoring.MonitoringService/LabelNames',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_LabelNames,
      callback);
};


/**
 * @param {!proto.monitoring.LabelNamesRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.monitoring.LabelNamesResponse>}
 *     Promise that resolves to the response
 */
proto.monitoring.MonitoringServicePromiseClient.prototype.labelNames =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/monitoring.MonitoringService/LabelNames',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_LabelNames);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.monitoring.LabelValuesRequest,
 *   !proto.monitoring.LabelValuesResponse>}
 */
const methodDescriptor_MonitoringService_LabelValues = new grpc.web.MethodDescriptor(
  '/monitoring.MonitoringService/LabelValues',
  grpc.web.MethodType.UNARY,
  proto.monitoring.LabelValuesRequest,
  proto.monitoring.LabelValuesResponse,
  /**
   * @param {!proto.monitoring.LabelValuesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.monitoring.LabelValuesResponse.deserializeBinary
);


/**
 * @param {!proto.monitoring.LabelValuesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.monitoring.LabelValuesResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.monitoring.LabelValuesResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.monitoring.MonitoringServiceClient.prototype.labelValues =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/monitoring.MonitoringService/LabelValues',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_LabelValues,
      callback);
};


/**
 * @param {!proto.monitoring.LabelValuesRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.monitoring.LabelValuesResponse>}
 *     Promise that resolves to the response
 */
proto.monitoring.MonitoringServicePromiseClient.prototype.labelValues =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/monitoring.MonitoringService/LabelValues',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_LabelValues);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.monitoring.QueryRequest,
 *   !proto.monitoring.QueryResponse>}
 */
const methodDescriptor_MonitoringService_Query = new grpc.web.MethodDescriptor(
  '/monitoring.MonitoringService/Query',
  grpc.web.MethodType.UNARY,
  proto.monitoring.QueryRequest,
  proto.monitoring.QueryResponse,
  /**
   * @param {!proto.monitoring.QueryRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.monitoring.QueryResponse.deserializeBinary
);


/**
 * @param {!proto.monitoring.QueryRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.monitoring.QueryResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.monitoring.QueryResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.monitoring.MonitoringServiceClient.prototype.query =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/monitoring.MonitoringService/Query',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_Query,
      callback);
};


/**
 * @param {!proto.monitoring.QueryRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.monitoring.QueryResponse>}
 *     Promise that resolves to the response
 */
proto.monitoring.MonitoringServicePromiseClient.prototype.query =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/monitoring.MonitoringService/Query',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_Query);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.monitoring.QueryRangeRequest,
 *   !proto.monitoring.QueryRangeResponse>}
 */
const methodDescriptor_MonitoringService_QueryRange = new grpc.web.MethodDescriptor(
  '/monitoring.MonitoringService/QueryRange',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.monitoring.QueryRangeRequest,
  proto.monitoring.QueryRangeResponse,
  /**
   * @param {!proto.monitoring.QueryRangeRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.monitoring.QueryRangeResponse.deserializeBinary
);


/**
 * @param {!proto.monitoring.QueryRangeRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.monitoring.QueryRangeResponse>}
 *     The XHR Node Readable Stream
 */
proto.monitoring.MonitoringServiceClient.prototype.queryRange =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/monitoring.MonitoringService/QueryRange',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_QueryRange);
};


/**
 * @param {!proto.monitoring.QueryRangeRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.monitoring.QueryRangeResponse>}
 *     The XHR Node Readable Stream
 */
proto.monitoring.MonitoringServicePromiseClient.prototype.queryRange =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/monitoring.MonitoringService/QueryRange',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_QueryRange);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.monitoring.SeriesRequest,
 *   !proto.monitoring.SeriesResponse>}
 */
const methodDescriptor_MonitoringService_Series = new grpc.web.MethodDescriptor(
  '/monitoring.MonitoringService/Series',
  grpc.web.MethodType.UNARY,
  proto.monitoring.SeriesRequest,
  proto.monitoring.SeriesResponse,
  /**
   * @param {!proto.monitoring.SeriesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.monitoring.SeriesResponse.deserializeBinary
);


/**
 * @param {!proto.monitoring.SeriesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.monitoring.SeriesResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.monitoring.SeriesResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.monitoring.MonitoringServiceClient.prototype.series =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/monitoring.MonitoringService/Series',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_Series,
      callback);
};


/**
 * @param {!proto.monitoring.SeriesRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.monitoring.SeriesResponse>}
 *     Promise that resolves to the response
 */
proto.monitoring.MonitoringServicePromiseClient.prototype.series =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/monitoring.MonitoringService/Series',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_Series);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.monitoring.SnapshotRequest,
 *   !proto.monitoring.SnapshotResponse>}
 */
const methodDescriptor_MonitoringService_Snapshot = new grpc.web.MethodDescriptor(
  '/monitoring.MonitoringService/Snapshot',
  grpc.web.MethodType.UNARY,
  proto.monitoring.SnapshotRequest,
  proto.monitoring.SnapshotResponse,
  /**
   * @param {!proto.monitoring.SnapshotRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.monitoring.SnapshotResponse.deserializeBinary
);


/**
 * @param {!proto.monitoring.SnapshotRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.monitoring.SnapshotResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.monitoring.SnapshotResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.monitoring.MonitoringServiceClient.prototype.snapshot =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/monitoring.MonitoringService/Snapshot',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_Snapshot,
      callback);
};


/**
 * @param {!proto.monitoring.SnapshotRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.monitoring.SnapshotResponse>}
 *     Promise that resolves to the response
 */
proto.monitoring.MonitoringServicePromiseClient.prototype.snapshot =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/monitoring.MonitoringService/Snapshot',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_Snapshot);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.monitoring.RulesRequest,
 *   !proto.monitoring.RulesResponse>}
 */
const methodDescriptor_MonitoringService_Rules = new grpc.web.MethodDescriptor(
  '/monitoring.MonitoringService/Rules',
  grpc.web.MethodType.UNARY,
  proto.monitoring.RulesRequest,
  proto.monitoring.RulesResponse,
  /**
   * @param {!proto.monitoring.RulesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.monitoring.RulesResponse.deserializeBinary
);


/**
 * @param {!proto.monitoring.RulesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.monitoring.RulesResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.monitoring.RulesResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.monitoring.MonitoringServiceClient.prototype.rules =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/monitoring.MonitoringService/Rules',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_Rules,
      callback);
};


/**
 * @param {!proto.monitoring.RulesRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.monitoring.RulesResponse>}
 *     Promise that resolves to the response
 */
proto.monitoring.MonitoringServicePromiseClient.prototype.rules =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/monitoring.MonitoringService/Rules',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_Rules);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.monitoring.TargetsRequest,
 *   !proto.monitoring.TargetsResponse>}
 */
const methodDescriptor_MonitoringService_Targets = new grpc.web.MethodDescriptor(
  '/monitoring.MonitoringService/Targets',
  grpc.web.MethodType.UNARY,
  proto.monitoring.TargetsRequest,
  proto.monitoring.TargetsResponse,
  /**
   * @param {!proto.monitoring.TargetsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.monitoring.TargetsResponse.deserializeBinary
);


/**
 * @param {!proto.monitoring.TargetsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.monitoring.TargetsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.monitoring.TargetsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.monitoring.MonitoringServiceClient.prototype.targets =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/monitoring.MonitoringService/Targets',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_Targets,
      callback);
};


/**
 * @param {!proto.monitoring.TargetsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.monitoring.TargetsResponse>}
 *     Promise that resolves to the response
 */
proto.monitoring.MonitoringServicePromiseClient.prototype.targets =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/monitoring.MonitoringService/Targets',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_Targets);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.monitoring.TargetsMetadataRequest,
 *   !proto.monitoring.TargetsMetadataResponse>}
 */
const methodDescriptor_MonitoringService_TargetsMetadata = new grpc.web.MethodDescriptor(
  '/monitoring.MonitoringService/TargetsMetadata',
  grpc.web.MethodType.UNARY,
  proto.monitoring.TargetsMetadataRequest,
  proto.monitoring.TargetsMetadataResponse,
  /**
   * @param {!proto.monitoring.TargetsMetadataRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.monitoring.TargetsMetadataResponse.deserializeBinary
);


/**
 * @param {!proto.monitoring.TargetsMetadataRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.monitoring.TargetsMetadataResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.monitoring.TargetsMetadataResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.monitoring.MonitoringServiceClient.prototype.targetsMetadata =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/monitoring.MonitoringService/TargetsMetadata',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_TargetsMetadata,
      callback);
};


/**
 * @param {!proto.monitoring.TargetsMetadataRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.monitoring.TargetsMetadataResponse>}
 *     Promise that resolves to the response
 */
proto.monitoring.MonitoringServicePromiseClient.prototype.targetsMetadata =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/monitoring.MonitoringService/TargetsMetadata',
      request,
      metadata || {},
      methodDescriptor_MonitoringService_TargetsMetadata);
};


module.exports = proto.monitoring;

