/**
 * @fileoverview gRPC-Web generated client stub for config
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.config = require('./config_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.config.ConfigServiceClient =
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
proto.config.ConfigServicePromiseClient =
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
 *   !proto.config.SetServiceConfigurationRequest,
 *   !proto.config.SetServiceConfigurationResponse>}
 */
const methodDescriptor_ConfigService_SetServiceConfiguration = new grpc.web.MethodDescriptor(
  '/config.ConfigService/SetServiceConfiguration',
  grpc.web.MethodType.UNARY,
  proto.config.SetServiceConfigurationRequest,
  proto.config.SetServiceConfigurationResponse,
  /**
   * @param {!proto.config.SetServiceConfigurationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.config.SetServiceConfigurationResponse.deserializeBinary
);


/**
 * @param {!proto.config.SetServiceConfigurationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.config.SetServiceConfigurationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.config.SetServiceConfigurationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.config.ConfigServiceClient.prototype.setServiceConfiguration =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/config.ConfigService/SetServiceConfiguration',
      request,
      metadata || {},
      methodDescriptor_ConfigService_SetServiceConfiguration,
      callback);
};


/**
 * @param {!proto.config.SetServiceConfigurationRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.config.SetServiceConfigurationResponse>}
 *     Promise that resolves to the response
 */
proto.config.ConfigServicePromiseClient.prototype.setServiceConfiguration =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/config.ConfigService/SetServiceConfiguration',
      request,
      metadata || {},
      methodDescriptor_ConfigService_SetServiceConfiguration);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.config.GetServiceConfigurationRequest,
 *   !proto.config.GetServiceConfigurationResponse>}
 */
const methodDescriptor_ConfigService_GetServiceConfiguration = new grpc.web.MethodDescriptor(
  '/config.ConfigService/GetServiceConfiguration',
  grpc.web.MethodType.UNARY,
  proto.config.GetServiceConfigurationRequest,
  proto.config.GetServiceConfigurationResponse,
  /**
   * @param {!proto.config.GetServiceConfigurationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.config.GetServiceConfigurationResponse.deserializeBinary
);


/**
 * @param {!proto.config.GetServiceConfigurationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.config.GetServiceConfigurationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.config.GetServiceConfigurationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.config.ConfigServiceClient.prototype.getServiceConfiguration =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/config.ConfigService/GetServiceConfiguration',
      request,
      metadata || {},
      methodDescriptor_ConfigService_GetServiceConfiguration,
      callback);
};


/**
 * @param {!proto.config.GetServiceConfigurationRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.config.GetServiceConfigurationResponse>}
 *     Promise that resolves to the response
 */
proto.config.ConfigServicePromiseClient.prototype.getServiceConfiguration =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/config.ConfigService/GetServiceConfiguration',
      request,
      metadata || {},
      methodDescriptor_ConfigService_GetServiceConfiguration);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.config.GetServiceConfigurationByIdRequest,
 *   !proto.config.GetServiceConfigurationByIdResponse>}
 */
const methodDescriptor_ConfigService_GetServiceConfigurationById = new grpc.web.MethodDescriptor(
  '/config.ConfigService/GetServiceConfigurationById',
  grpc.web.MethodType.UNARY,
  proto.config.GetServiceConfigurationByIdRequest,
  proto.config.GetServiceConfigurationByIdResponse,
  /**
   * @param {!proto.config.GetServiceConfigurationByIdRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.config.GetServiceConfigurationByIdResponse.deserializeBinary
);


/**
 * @param {!proto.config.GetServiceConfigurationByIdRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.config.GetServiceConfigurationByIdResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.config.GetServiceConfigurationByIdResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.config.ConfigServiceClient.prototype.getServiceConfigurationById =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/config.ConfigService/GetServiceConfigurationById',
      request,
      metadata || {},
      methodDescriptor_ConfigService_GetServiceConfigurationById,
      callback);
};


/**
 * @param {!proto.config.GetServiceConfigurationByIdRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.config.GetServiceConfigurationByIdResponse>}
 *     Promise that resolves to the response
 */
proto.config.ConfigServicePromiseClient.prototype.getServiceConfigurationById =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/config.ConfigService/GetServiceConfigurationById',
      request,
      metadata || {},
      methodDescriptor_ConfigService_GetServiceConfigurationById);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.config.GetServicesConfigurationsByNameRequest,
 *   !proto.config.GetServicesConfigurationsByNameResponse>}
 */
const methodDescriptor_ConfigService_GetServicesConfigurationsByName = new grpc.web.MethodDescriptor(
  '/config.ConfigService/GetServicesConfigurationsByName',
  grpc.web.MethodType.UNARY,
  proto.config.GetServicesConfigurationsByNameRequest,
  proto.config.GetServicesConfigurationsByNameResponse,
  /**
   * @param {!proto.config.GetServicesConfigurationsByNameRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.config.GetServicesConfigurationsByNameResponse.deserializeBinary
);


/**
 * @param {!proto.config.GetServicesConfigurationsByNameRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.config.GetServicesConfigurationsByNameResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.config.GetServicesConfigurationsByNameResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.config.ConfigServiceClient.prototype.getServicesConfigurationsByName =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/config.ConfigService/GetServicesConfigurationsByName',
      request,
      metadata || {},
      methodDescriptor_ConfigService_GetServicesConfigurationsByName,
      callback);
};


/**
 * @param {!proto.config.GetServicesConfigurationsByNameRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.config.GetServicesConfigurationsByNameResponse>}
 *     Promise that resolves to the response
 */
proto.config.ConfigServicePromiseClient.prototype.getServicesConfigurationsByName =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/config.ConfigService/GetServicesConfigurationsByName',
      request,
      metadata || {},
      methodDescriptor_ConfigService_GetServicesConfigurationsByName);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.config.GetServicesConfigurationsRequest,
 *   !proto.config.GetServicesConfigurationsResponse>}
 */
const methodDescriptor_ConfigService_GetServicesConfigurations = new grpc.web.MethodDescriptor(
  '/config.ConfigService/GetServicesConfigurations',
  grpc.web.MethodType.UNARY,
  proto.config.GetServicesConfigurationsRequest,
  proto.config.GetServicesConfigurationsResponse,
  /**
   * @param {!proto.config.GetServicesConfigurationsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.config.GetServicesConfigurationsResponse.deserializeBinary
);


/**
 * @param {!proto.config.GetServicesConfigurationsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.config.GetServicesConfigurationsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.config.GetServicesConfigurationsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.config.ConfigServiceClient.prototype.getServicesConfigurations =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/config.ConfigService/GetServicesConfigurations',
      request,
      metadata || {},
      methodDescriptor_ConfigService_GetServicesConfigurations,
      callback);
};


/**
 * @param {!proto.config.GetServicesConfigurationsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.config.GetServicesConfigurationsResponse>}
 *     Promise that resolves to the response
 */
proto.config.ConfigServicePromiseClient.prototype.getServicesConfigurations =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/config.ConfigService/GetServicesConfigurations',
      request,
      metadata || {},
      methodDescriptor_ConfigService_GetServicesConfigurations);
};


module.exports = proto.config;

