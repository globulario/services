/**
 * @fileoverview gRPC-Web generated client stub for services
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.services = require('./services_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.services.PackageDiscoveryClient =
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
proto.services.PackageDiscoveryPromiseClient =
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
 *   !proto.services.FindPackagesDescriptorRequest,
 *   !proto.services.FindPackagesDescriptorResponse>}
 */
const methodDescriptor_PackageDiscovery_FindPackages = new grpc.web.MethodDescriptor(
  '/services.PackageDiscovery/FindPackages',
  grpc.web.MethodType.UNARY,
  proto.services.FindPackagesDescriptorRequest,
  proto.services.FindPackagesDescriptorResponse,
  /**
   * @param {!proto.services.FindPackagesDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services.FindPackagesDescriptorResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.services.FindPackagesDescriptorRequest,
 *   !proto.services.FindPackagesDescriptorResponse>}
 */
const methodInfo_PackageDiscovery_FindPackages = new grpc.web.AbstractClientBase.MethodInfo(
  proto.services.FindPackagesDescriptorResponse,
  /**
   * @param {!proto.services.FindPackagesDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services.FindPackagesDescriptorResponse.deserializeBinary
);


/**
 * @param {!proto.services.FindPackagesDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.services.FindPackagesDescriptorResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.services.FindPackagesDescriptorResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.services.PackageDiscoveryClient.prototype.findPackages =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/services.PackageDiscovery/FindPackages',
      request,
      metadata || {},
      methodDescriptor_PackageDiscovery_FindPackages,
      callback);
};


/**
 * @param {!proto.services.FindPackagesDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.services.FindPackagesDescriptorResponse>}
 *     Promise that resolves to the response
 */
proto.services.PackageDiscoveryPromiseClient.prototype.findPackages =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/services.PackageDiscovery/FindPackages',
      request,
      metadata || {},
      methodDescriptor_PackageDiscovery_FindPackages);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.services.GetPackageDescriptorRequest,
 *   !proto.services.GetPackageDescriptorResponse>}
 */
const methodDescriptor_PackageDiscovery_GetPackageDescriptor = new grpc.web.MethodDescriptor(
  '/services.PackageDiscovery/GetPackageDescriptor',
  grpc.web.MethodType.UNARY,
  proto.services.GetPackageDescriptorRequest,
  proto.services.GetPackageDescriptorResponse,
  /**
   * @param {!proto.services.GetPackageDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services.GetPackageDescriptorResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.services.GetPackageDescriptorRequest,
 *   !proto.services.GetPackageDescriptorResponse>}
 */
const methodInfo_PackageDiscovery_GetPackageDescriptor = new grpc.web.AbstractClientBase.MethodInfo(
  proto.services.GetPackageDescriptorResponse,
  /**
   * @param {!proto.services.GetPackageDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services.GetPackageDescriptorResponse.deserializeBinary
);


/**
 * @param {!proto.services.GetPackageDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.services.GetPackageDescriptorResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.services.GetPackageDescriptorResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.services.PackageDiscoveryClient.prototype.getPackageDescriptor =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/services.PackageDiscovery/GetPackageDescriptor',
      request,
      metadata || {},
      methodDescriptor_PackageDiscovery_GetPackageDescriptor,
      callback);
};


/**
 * @param {!proto.services.GetPackageDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.services.GetPackageDescriptorResponse>}
 *     Promise that resolves to the response
 */
proto.services.PackageDiscoveryPromiseClient.prototype.getPackageDescriptor =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/services.PackageDiscovery/GetPackageDescriptor',
      request,
      metadata || {},
      methodDescriptor_PackageDiscovery_GetPackageDescriptor);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.services.GetPackagesDescriptorRequest,
 *   !proto.services.GetPackagesDescriptorResponse>}
 */
const methodDescriptor_PackageDiscovery_GetPackagesDescriptor = new grpc.web.MethodDescriptor(
  '/services.PackageDiscovery/GetPackagesDescriptor',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.services.GetPackagesDescriptorRequest,
  proto.services.GetPackagesDescriptorResponse,
  /**
   * @param {!proto.services.GetPackagesDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services.GetPackagesDescriptorResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.services.GetPackagesDescriptorRequest,
 *   !proto.services.GetPackagesDescriptorResponse>}
 */
const methodInfo_PackageDiscovery_GetPackagesDescriptor = new grpc.web.AbstractClientBase.MethodInfo(
  proto.services.GetPackagesDescriptorResponse,
  /**
   * @param {!proto.services.GetPackagesDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services.GetPackagesDescriptorResponse.deserializeBinary
);


/**
 * @param {!proto.services.GetPackagesDescriptorRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.services.GetPackagesDescriptorResponse>}
 *     The XHR Node Readable Stream
 */
proto.services.PackageDiscoveryClient.prototype.getPackagesDescriptor =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/services.PackageDiscovery/GetPackagesDescriptor',
      request,
      metadata || {},
      methodDescriptor_PackageDiscovery_GetPackagesDescriptor);
};


/**
 * @param {!proto.services.GetPackagesDescriptorRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.services.GetPackagesDescriptorResponse>}
 *     The XHR Node Readable Stream
 */
proto.services.PackageDiscoveryPromiseClient.prototype.getPackagesDescriptor =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/services.PackageDiscovery/GetPackagesDescriptor',
      request,
      metadata || {},
      methodDescriptor_PackageDiscovery_GetPackagesDescriptor);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.services.SetPackageDescriptorRequest,
 *   !proto.services.SetPackageDescriptorResponse>}
 */
const methodDescriptor_PackageDiscovery_SetPackageDescriptor = new grpc.web.MethodDescriptor(
  '/services.PackageDiscovery/SetPackageDescriptor',
  grpc.web.MethodType.UNARY,
  proto.services.SetPackageDescriptorRequest,
  proto.services.SetPackageDescriptorResponse,
  /**
   * @param {!proto.services.SetPackageDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services.SetPackageDescriptorResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.services.SetPackageDescriptorRequest,
 *   !proto.services.SetPackageDescriptorResponse>}
 */
const methodInfo_PackageDiscovery_SetPackageDescriptor = new grpc.web.AbstractClientBase.MethodInfo(
  proto.services.SetPackageDescriptorResponse,
  /**
   * @param {!proto.services.SetPackageDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services.SetPackageDescriptorResponse.deserializeBinary
);


/**
 * @param {!proto.services.SetPackageDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.services.SetPackageDescriptorResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.services.SetPackageDescriptorResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.services.PackageDiscoveryClient.prototype.setPackageDescriptor =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/services.PackageDiscovery/SetPackageDescriptor',
      request,
      metadata || {},
      methodDescriptor_PackageDiscovery_SetPackageDescriptor,
      callback);
};


/**
 * @param {!proto.services.SetPackageDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.services.SetPackageDescriptorResponse>}
 *     Promise that resolves to the response
 */
proto.services.PackageDiscoveryPromiseClient.prototype.setPackageDescriptor =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/services.PackageDiscovery/SetPackageDescriptor',
      request,
      metadata || {},
      methodDescriptor_PackageDiscovery_SetPackageDescriptor);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.services.PublishPackageDescriptorRequest,
 *   !proto.services.PublishPackageDescriptorResponse>}
 */
const methodDescriptor_PackageDiscovery_PublishPackageDescriptor = new grpc.web.MethodDescriptor(
  '/services.PackageDiscovery/PublishPackageDescriptor',
  grpc.web.MethodType.UNARY,
  proto.services.PublishPackageDescriptorRequest,
  proto.services.PublishPackageDescriptorResponse,
  /**
   * @param {!proto.services.PublishPackageDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services.PublishPackageDescriptorResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.services.PublishPackageDescriptorRequest,
 *   !proto.services.PublishPackageDescriptorResponse>}
 */
const methodInfo_PackageDiscovery_PublishPackageDescriptor = new grpc.web.AbstractClientBase.MethodInfo(
  proto.services.PublishPackageDescriptorResponse,
  /**
   * @param {!proto.services.PublishPackageDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services.PublishPackageDescriptorResponse.deserializeBinary
);


/**
 * @param {!proto.services.PublishPackageDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.services.PublishPackageDescriptorResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.services.PublishPackageDescriptorResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.services.PackageDiscoveryClient.prototype.publishPackageDescriptor =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/services.PackageDiscovery/PublishPackageDescriptor',
      request,
      metadata || {},
      methodDescriptor_PackageDiscovery_PublishPackageDescriptor,
      callback);
};


/**
 * @param {!proto.services.PublishPackageDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.services.PublishPackageDescriptorResponse>}
 *     Promise that resolves to the response
 */
proto.services.PackageDiscoveryPromiseClient.prototype.publishPackageDescriptor =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/services.PackageDiscovery/PublishPackageDescriptor',
      request,
      metadata || {},
      methodDescriptor_PackageDiscovery_PublishPackageDescriptor);
};


/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.services.PackageRepositoryClient =
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
proto.services.PackageRepositoryPromiseClient =
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
 *   !proto.services.DownloadBundleRequest,
 *   !proto.services.DownloadBundleResponse>}
 */
const methodDescriptor_PackageRepository_DownloadBundle = new grpc.web.MethodDescriptor(
  '/services.PackageRepository/DownloadBundle',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.services.DownloadBundleRequest,
  proto.services.DownloadBundleResponse,
  /**
   * @param {!proto.services.DownloadBundleRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services.DownloadBundleResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.services.DownloadBundleRequest,
 *   !proto.services.DownloadBundleResponse>}
 */
const methodInfo_PackageRepository_DownloadBundle = new grpc.web.AbstractClientBase.MethodInfo(
  proto.services.DownloadBundleResponse,
  /**
   * @param {!proto.services.DownloadBundleRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.services.DownloadBundleResponse.deserializeBinary
);


/**
 * @param {!proto.services.DownloadBundleRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.services.DownloadBundleResponse>}
 *     The XHR Node Readable Stream
 */
proto.services.PackageRepositoryClient.prototype.downloadBundle =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/services.PackageRepository/DownloadBundle',
      request,
      metadata || {},
      methodDescriptor_PackageRepository_DownloadBundle);
};


/**
 * @param {!proto.services.DownloadBundleRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.services.DownloadBundleResponse>}
 *     The XHR Node Readable Stream
 */
proto.services.PackageRepositoryPromiseClient.prototype.downloadBundle =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/services.PackageRepository/DownloadBundle',
      request,
      metadata || {},
      methodDescriptor_PackageRepository_DownloadBundle);
};


module.exports = proto.services;

