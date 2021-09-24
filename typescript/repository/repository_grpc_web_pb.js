/**
 * @fileoverview gRPC-Web generated client stub for repository
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
proto.repository = require('./repository_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.repository.PackageRepositoryClient =
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
proto.repository.PackageRepositoryPromiseClient =
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
 *   !proto.repository.DownloadBundleRequest,
 *   !proto.repository.DownloadBundleResponse>}
 */
const methodDescriptor_PackageRepository_DownloadBundle = new grpc.web.MethodDescriptor(
  '/repository.PackageRepository/DownloadBundle',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.repository.DownloadBundleRequest,
  proto.repository.DownloadBundleResponse,
  /**
   * @param {!proto.repository.DownloadBundleRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.repository.DownloadBundleResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.repository.DownloadBundleRequest,
 *   !proto.repository.DownloadBundleResponse>}
 */
const methodInfo_PackageRepository_DownloadBundle = new grpc.web.AbstractClientBase.MethodInfo(
  proto.repository.DownloadBundleResponse,
  /**
   * @param {!proto.repository.DownloadBundleRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.repository.DownloadBundleResponse.deserializeBinary
);


/**
 * @param {!proto.repository.DownloadBundleRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.repository.DownloadBundleResponse>}
 *     The XHR Node Readable Stream
 */
proto.repository.PackageRepositoryClient.prototype.downloadBundle =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/repository.PackageRepository/DownloadBundle',
      request,
      metadata || {},
      methodDescriptor_PackageRepository_DownloadBundle);
};


/**
 * @param {!proto.repository.DownloadBundleRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.repository.DownloadBundleResponse>}
 *     The XHR Node Readable Stream
 */
proto.repository.PackageRepositoryPromiseClient.prototype.downloadBundle =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/repository.PackageRepository/DownloadBundle',
      request,
      metadata || {},
      methodDescriptor_PackageRepository_DownloadBundle);
};


module.exports = proto.repository;

