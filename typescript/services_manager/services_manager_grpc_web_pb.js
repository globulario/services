/**
 * @fileoverview gRPC-Web generated client stub for services_manager
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.services_manager = require('./services_manager_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.services_manager.ServicesManagerServiceClient =
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
proto.services_manager.ServicesManagerServicePromiseClient =
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
 *   !proto.services_manager.InstallServiceRequest,
 *   !proto.services_manager.InstallServiceResponse>}
 */
const methodDescriptor_ServicesManagerService_InstallService = new grpc.web.MethodDescriptor(
  '/services_manager.ServicesManagerService/InstallService',
  grpc.web.MethodType.UNARY,
  proto.services_manager.InstallServiceRequest,
  proto.services_manager.InstallServiceResponse,
  /**
   * @param {!proto.services_manager.InstallServiceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services_manager.InstallServiceResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.services_manager.InstallServiceRequest,
 *   !proto.services_manager.InstallServiceResponse>}
 */
const methodInfo_ServicesManagerService_InstallService = new grpc.web.AbstractClientBase.MethodInfo(
  proto.services_manager.InstallServiceResponse,
  /**
   * @param {!proto.services_manager.InstallServiceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services_manager.InstallServiceResponse.deserializeBinary
);


/**
 * @param {!proto.services_manager.InstallServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.services_manager.InstallServiceResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.services_manager.InstallServiceResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.services_manager.ServicesManagerServiceClient.prototype.installService =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/services_manager.ServicesManagerService/InstallService',
      request,
      metadata || {},
      methodDescriptor_ServicesManagerService_InstallService,
      callback);
};


/**
 * @param {!proto.services_manager.InstallServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.services_manager.InstallServiceResponse>}
 *     Promise that resolves to the response
 */
proto.services_manager.ServicesManagerServicePromiseClient.prototype.installService =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/services_manager.ServicesManagerService/InstallService',
      request,
      metadata || {},
      methodDescriptor_ServicesManagerService_InstallService);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.services_manager.UninstallServiceRequest,
 *   !proto.services_manager.UninstallServiceResponse>}
 */
const methodDescriptor_ServicesManagerService_UninstallService = new grpc.web.MethodDescriptor(
  '/services_manager.ServicesManagerService/UninstallService',
  grpc.web.MethodType.UNARY,
  proto.services_manager.UninstallServiceRequest,
  proto.services_manager.UninstallServiceResponse,
  /**
   * @param {!proto.services_manager.UninstallServiceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services_manager.UninstallServiceResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.services_manager.UninstallServiceRequest,
 *   !proto.services_manager.UninstallServiceResponse>}
 */
const methodInfo_ServicesManagerService_UninstallService = new grpc.web.AbstractClientBase.MethodInfo(
  proto.services_manager.UninstallServiceResponse,
  /**
   * @param {!proto.services_manager.UninstallServiceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services_manager.UninstallServiceResponse.deserializeBinary
);


/**
 * @param {!proto.services_manager.UninstallServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.services_manager.UninstallServiceResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.services_manager.UninstallServiceResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.services_manager.ServicesManagerServiceClient.prototype.uninstallService =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/services_manager.ServicesManagerService/UninstallService',
      request,
      metadata || {},
      methodDescriptor_ServicesManagerService_UninstallService,
      callback);
};


/**
 * @param {!proto.services_manager.UninstallServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.services_manager.UninstallServiceResponse>}
 *     Promise that resolves to the response
 */
proto.services_manager.ServicesManagerServicePromiseClient.prototype.uninstallService =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/services_manager.ServicesManagerService/UninstallService',
      request,
      metadata || {},
      methodDescriptor_ServicesManagerService_UninstallService);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.services_manager.StopServiceInstanceRequest,
 *   !proto.services_manager.StopServiceInstanceResponse>}
 */
const methodDescriptor_ServicesManagerService_StopServiceInstance = new grpc.web.MethodDescriptor(
  '/services_manager.ServicesManagerService/StopServiceInstance',
  grpc.web.MethodType.UNARY,
  proto.services_manager.StopServiceInstanceRequest,
  proto.services_manager.StopServiceInstanceResponse,
  /**
   * @param {!proto.services_manager.StopServiceInstanceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services_manager.StopServiceInstanceResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.services_manager.StopServiceInstanceRequest,
 *   !proto.services_manager.StopServiceInstanceResponse>}
 */
const methodInfo_ServicesManagerService_StopServiceInstance = new grpc.web.AbstractClientBase.MethodInfo(
  proto.services_manager.StopServiceInstanceResponse,
  /**
   * @param {!proto.services_manager.StopServiceInstanceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services_manager.StopServiceInstanceResponse.deserializeBinary
);


/**
 * @param {!proto.services_manager.StopServiceInstanceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.services_manager.StopServiceInstanceResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.services_manager.StopServiceInstanceResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.services_manager.ServicesManagerServiceClient.prototype.stopServiceInstance =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/services_manager.ServicesManagerService/StopServiceInstance',
      request,
      metadata || {},
      methodDescriptor_ServicesManagerService_StopServiceInstance,
      callback);
};


/**
 * @param {!proto.services_manager.StopServiceInstanceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.services_manager.StopServiceInstanceResponse>}
 *     Promise that resolves to the response
 */
proto.services_manager.ServicesManagerServicePromiseClient.prototype.stopServiceInstance =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/services_manager.ServicesManagerService/StopServiceInstance',
      request,
      metadata || {},
      methodDescriptor_ServicesManagerService_StopServiceInstance);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.services_manager.StartServiceInstanceRequest,
 *   !proto.services_manager.StartServiceInstanceResponse>}
 */
const methodDescriptor_ServicesManagerService_StartServiceInstance = new grpc.web.MethodDescriptor(
  '/services_manager.ServicesManagerService/StartServiceInstance',
  grpc.web.MethodType.UNARY,
  proto.services_manager.StartServiceInstanceRequest,
  proto.services_manager.StartServiceInstanceResponse,
  /**
   * @param {!proto.services_manager.StartServiceInstanceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services_manager.StartServiceInstanceResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.services_manager.StartServiceInstanceRequest,
 *   !proto.services_manager.StartServiceInstanceResponse>}
 */
const methodInfo_ServicesManagerService_StartServiceInstance = new grpc.web.AbstractClientBase.MethodInfo(
  proto.services_manager.StartServiceInstanceResponse,
  /**
   * @param {!proto.services_manager.StartServiceInstanceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services_manager.StartServiceInstanceResponse.deserializeBinary
);


/**
 * @param {!proto.services_manager.StartServiceInstanceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.services_manager.StartServiceInstanceResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.services_manager.StartServiceInstanceResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.services_manager.ServicesManagerServiceClient.prototype.startServiceInstance =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/services_manager.ServicesManagerService/StartServiceInstance',
      request,
      metadata || {},
      methodDescriptor_ServicesManagerService_StartServiceInstance,
      callback);
};


/**
 * @param {!proto.services_manager.StartServiceInstanceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.services_manager.StartServiceInstanceResponse>}
 *     Promise that resolves to the response
 */
proto.services_manager.ServicesManagerServicePromiseClient.prototype.startServiceInstance =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/services_manager.ServicesManagerService/StartServiceInstance',
      request,
      metadata || {},
      methodDescriptor_ServicesManagerService_StartServiceInstance);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.services_manager.RestartAllServicesRequest,
 *   !proto.services_manager.RestartAllServicesResponse>}
 */
const methodDescriptor_ServicesManagerService_RestartAllServices = new grpc.web.MethodDescriptor(
  '/services_manager.ServicesManagerService/RestartAllServices',
  grpc.web.MethodType.UNARY,
  proto.services_manager.RestartAllServicesRequest,
  proto.services_manager.RestartAllServicesResponse,
  /**
   * @param {!proto.services_manager.RestartAllServicesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services_manager.RestartAllServicesResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.services_manager.RestartAllServicesRequest,
 *   !proto.services_manager.RestartAllServicesResponse>}
 */
const methodInfo_ServicesManagerService_RestartAllServices = new grpc.web.AbstractClientBase.MethodInfo(
  proto.services_manager.RestartAllServicesResponse,
  /**
   * @param {!proto.services_manager.RestartAllServicesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services_manager.RestartAllServicesResponse.deserializeBinary
);


/**
 * @param {!proto.services_manager.RestartAllServicesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.services_manager.RestartAllServicesResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.services_manager.RestartAllServicesResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.services_manager.ServicesManagerServiceClient.prototype.restartAllServices =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/services_manager.ServicesManagerService/RestartAllServices',
      request,
      metadata || {},
      methodDescriptor_ServicesManagerService_RestartAllServices,
      callback);
};


/**
 * @param {!proto.services_manager.RestartAllServicesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.services_manager.RestartAllServicesResponse>}
 *     Promise that resolves to the response
 */
proto.services_manager.ServicesManagerServicePromiseClient.prototype.restartAllServices =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/services_manager.ServicesManagerService/RestartAllServices',
      request,
      metadata || {},
      methodDescriptor_ServicesManagerService_RestartAllServices);
};


module.exports = proto.services_manager;

