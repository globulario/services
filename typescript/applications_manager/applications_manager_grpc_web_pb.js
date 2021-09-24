/**
 * @fileoverview gRPC-Web generated client stub for applications_manager
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');


var resource_pb = require('../resource/resource_pb.js')
const proto = {};
proto.applications_manager = require('./applications_manager_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.applications_manager.ApplicationManagerServiceClient =
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
proto.applications_manager.ApplicationManagerServicePromiseClient =
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
 *   !proto.applications_manager.InstallApplicationRequest,
 *   !proto.applications_manager.InstallApplicationResponse>}
 */
const methodDescriptor_ApplicationManagerService_InstallApplication = new grpc.web.MethodDescriptor(
  '/applications_manager.ApplicationManagerService/InstallApplication',
  grpc.web.MethodType.UNARY,
  proto.applications_manager.InstallApplicationRequest,
  proto.applications_manager.InstallApplicationResponse,
  /**
   * @param {!proto.applications_manager.InstallApplicationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.applications_manager.InstallApplicationResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.applications_manager.InstallApplicationRequest,
 *   !proto.applications_manager.InstallApplicationResponse>}
 */
const methodInfo_ApplicationManagerService_InstallApplication = new grpc.web.AbstractClientBase.MethodInfo(
  proto.applications_manager.InstallApplicationResponse,
  /**
   * @param {!proto.applications_manager.InstallApplicationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.applications_manager.InstallApplicationResponse.deserializeBinary
);


/**
 * @param {!proto.applications_manager.InstallApplicationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.applications_manager.InstallApplicationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.applications_manager.InstallApplicationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.applications_manager.ApplicationManagerServiceClient.prototype.installApplication =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/applications_manager.ApplicationManagerService/InstallApplication',
      request,
      metadata || {},
      methodDescriptor_ApplicationManagerService_InstallApplication,
      callback);
};


/**
 * @param {!proto.applications_manager.InstallApplicationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.applications_manager.InstallApplicationResponse>}
 *     Promise that resolves to the response
 */
proto.applications_manager.ApplicationManagerServicePromiseClient.prototype.installApplication =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/applications_manager.ApplicationManagerService/InstallApplication',
      request,
      metadata || {},
      methodDescriptor_ApplicationManagerService_InstallApplication);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.applications_manager.UninstallApplicationRequest,
 *   !proto.applications_manager.UninstallApplicationResponse>}
 */
const methodDescriptor_ApplicationManagerService_UninstallApplication = new grpc.web.MethodDescriptor(
  '/applications_manager.ApplicationManagerService/UninstallApplication',
  grpc.web.MethodType.UNARY,
  proto.applications_manager.UninstallApplicationRequest,
  proto.applications_manager.UninstallApplicationResponse,
  /**
   * @param {!proto.applications_manager.UninstallApplicationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.applications_manager.UninstallApplicationResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.applications_manager.UninstallApplicationRequest,
 *   !proto.applications_manager.UninstallApplicationResponse>}
 */
const methodInfo_ApplicationManagerService_UninstallApplication = new grpc.web.AbstractClientBase.MethodInfo(
  proto.applications_manager.UninstallApplicationResponse,
  /**
   * @param {!proto.applications_manager.UninstallApplicationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.applications_manager.UninstallApplicationResponse.deserializeBinary
);


/**
 * @param {!proto.applications_manager.UninstallApplicationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.applications_manager.UninstallApplicationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.applications_manager.UninstallApplicationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.applications_manager.ApplicationManagerServiceClient.prototype.uninstallApplication =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/applications_manager.ApplicationManagerService/UninstallApplication',
      request,
      metadata || {},
      methodDescriptor_ApplicationManagerService_UninstallApplication,
      callback);
};


/**
 * @param {!proto.applications_manager.UninstallApplicationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.applications_manager.UninstallApplicationResponse>}
 *     Promise that resolves to the response
 */
proto.applications_manager.ApplicationManagerServicePromiseClient.prototype.uninstallApplication =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/applications_manager.ApplicationManagerService/UninstallApplication',
      request,
      metadata || {},
      methodDescriptor_ApplicationManagerService_UninstallApplication);
};


module.exports = proto.applications_manager;

