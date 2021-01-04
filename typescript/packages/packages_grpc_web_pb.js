/**
 * @fileoverview gRPC-Web generated client stub for packages
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.packages = require('./packages_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.packages.PackageDiscoveryClient =
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
proto.packages.PackageDiscoveryPromiseClient =
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
 *   !proto.packages.FindPackagesDescriptorRequest,
 *   !proto.packages.FindPackagesDescriptorResponse>}
 */
const methodDescriptor_PackageDiscovery_FindPackages = new grpc.web.MethodDescriptor(
  '/packages.PackageDiscovery/FindPackages',
  grpc.web.MethodType.UNARY,
  proto.packages.FindPackagesDescriptorRequest,
  proto.packages.FindPackagesDescriptorResponse,
  /**
   * @param {!proto.packages.FindPackagesDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.packages.FindPackagesDescriptorResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.packages.FindPackagesDescriptorRequest,
 *   !proto.packages.FindPackagesDescriptorResponse>}
 */
const methodInfo_PackageDiscovery_FindPackages = new grpc.web.AbstractClientBase.MethodInfo(
  proto.packages.FindPackagesDescriptorResponse,
  /**
   * @param {!proto.packages.FindPackagesDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.packages.FindPackagesDescriptorResponse.deserializeBinary
);


/**
 * @param {!proto.packages.FindPackagesDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.packages.FindPackagesDescriptorResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.packages.FindPackagesDescriptorResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.packages.PackageDiscoveryClient.prototype.findPackages =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/packages.PackageDiscovery/FindPackages',
      request,
      metadata || {},
      methodDescriptor_PackageDiscovery_FindPackages,
      callback);
};


/**
 * @param {!proto.packages.FindPackagesDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.packages.FindPackagesDescriptorResponse>}
 *     Promise that resolves to the response
 */
proto.packages.PackageDiscoveryPromiseClient.prototype.findPackages =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/packages.PackageDiscovery/FindPackages',
      request,
      metadata || {},
      methodDescriptor_PackageDiscovery_FindPackages);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.packages.GetPackageDescriptorRequest,
 *   !proto.packages.GetPackageDescriptorResponse>}
 */
const methodDescriptor_PackageDiscovery_GetPackageDescriptor = new grpc.web.MethodDescriptor(
  '/packages.PackageDiscovery/GetPackageDescriptor',
  grpc.web.MethodType.UNARY,
  proto.packages.GetPackageDescriptorRequest,
  proto.packages.GetPackageDescriptorResponse,
  /**
   * @param {!proto.packages.GetPackageDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.packages.GetPackageDescriptorResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.packages.GetPackageDescriptorRequest,
 *   !proto.packages.GetPackageDescriptorResponse>}
 */
const methodInfo_PackageDiscovery_GetPackageDescriptor = new grpc.web.AbstractClientBase.MethodInfo(
  proto.packages.GetPackageDescriptorResponse,
  /**
   * @param {!proto.packages.GetPackageDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.packages.GetPackageDescriptorResponse.deserializeBinary
);


/**
 * @param {!proto.packages.GetPackageDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.packages.GetPackageDescriptorResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.packages.GetPackageDescriptorResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.packages.PackageDiscoveryClient.prototype.getPackageDescriptor =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/packages.PackageDiscovery/GetPackageDescriptor',
      request,
      metadata || {},
      methodDescriptor_PackageDiscovery_GetPackageDescriptor,
      callback);
};


/**
 * @param {!proto.packages.GetPackageDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.packages.GetPackageDescriptorResponse>}
 *     Promise that resolves to the response
 */
proto.packages.PackageDiscoveryPromiseClient.prototype.getPackageDescriptor =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/packages.PackageDiscovery/GetPackageDescriptor',
      request,
      metadata || {},
      methodDescriptor_PackageDiscovery_GetPackageDescriptor);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.packages.GetPackagesDescriptorRequest,
 *   !proto.packages.GetPackagesDescriptorResponse>}
 */
const methodDescriptor_PackageDiscovery_GetPackagesDescriptor = new grpc.web.MethodDescriptor(
  '/packages.PackageDiscovery/GetPackagesDescriptor',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.packages.GetPackagesDescriptorRequest,
  proto.packages.GetPackagesDescriptorResponse,
  /**
   * @param {!proto.packages.GetPackagesDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.packages.GetPackagesDescriptorResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.packages.GetPackagesDescriptorRequest,
 *   !proto.packages.GetPackagesDescriptorResponse>}
 */
const methodInfo_PackageDiscovery_GetPackagesDescriptor = new grpc.web.AbstractClientBase.MethodInfo(
  proto.packages.GetPackagesDescriptorResponse,
  /**
   * @param {!proto.packages.GetPackagesDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.packages.GetPackagesDescriptorResponse.deserializeBinary
);


/**
 * @param {!proto.packages.GetPackagesDescriptorRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.packages.GetPackagesDescriptorResponse>}
 *     The XHR Node Readable Stream
 */
proto.packages.PackageDiscoveryClient.prototype.getPackagesDescriptor =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/packages.PackageDiscovery/GetPackagesDescriptor',
      request,
      metadata || {},
      methodDescriptor_PackageDiscovery_GetPackagesDescriptor);
};


/**
 * @param {!proto.packages.GetPackagesDescriptorRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.packages.GetPackagesDescriptorResponse>}
 *     The XHR Node Readable Stream
 */
proto.packages.PackageDiscoveryPromiseClient.prototype.getPackagesDescriptor =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/packages.PackageDiscovery/GetPackagesDescriptor',
      request,
      metadata || {},
      methodDescriptor_PackageDiscovery_GetPackagesDescriptor);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.packages.SetPackageDescriptorRequest,
 *   !proto.packages.SetPackageDescriptorResponse>}
 */
const methodDescriptor_PackageDiscovery_SetPackageDescriptor = new grpc.web.MethodDescriptor(
  '/packages.PackageDiscovery/SetPackageDescriptor',
  grpc.web.MethodType.UNARY,
  proto.packages.SetPackageDescriptorRequest,
  proto.packages.SetPackageDescriptorResponse,
  /**
   * @param {!proto.packages.SetPackageDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.packages.SetPackageDescriptorResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.packages.SetPackageDescriptorRequest,
 *   !proto.packages.SetPackageDescriptorResponse>}
 */
const methodInfo_PackageDiscovery_SetPackageDescriptor = new grpc.web.AbstractClientBase.MethodInfo(
  proto.packages.SetPackageDescriptorResponse,
  /**
   * @param {!proto.packages.SetPackageDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.packages.SetPackageDescriptorResponse.deserializeBinary
);


/**
 * @param {!proto.packages.SetPackageDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.packages.SetPackageDescriptorResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.packages.SetPackageDescriptorResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.packages.PackageDiscoveryClient.prototype.setPackageDescriptor =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/packages.PackageDiscovery/SetPackageDescriptor',
      request,
      metadata || {},
      methodDescriptor_PackageDiscovery_SetPackageDescriptor,
      callback);
};


/**
 * @param {!proto.packages.SetPackageDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.packages.SetPackageDescriptorResponse>}
 *     Promise that resolves to the response
 */
proto.packages.PackageDiscoveryPromiseClient.prototype.setPackageDescriptor =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/packages.PackageDiscovery/SetPackageDescriptor',
      request,
      metadata || {},
      methodDescriptor_PackageDiscovery_SetPackageDescriptor);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.packages.PublishPackageDescriptorRequest,
 *   !proto.packages.PublishPackageDescriptorResponse>}
 */
const methodDescriptor_PackageDiscovery_PublishPackageDescriptor = new grpc.web.MethodDescriptor(
  '/packages.PackageDiscovery/PublishPackageDescriptor',
  grpc.web.MethodType.UNARY,
  proto.packages.PublishPackageDescriptorRequest,
  proto.packages.PublishPackageDescriptorResponse,
  /**
   * @param {!proto.packages.PublishPackageDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.packages.PublishPackageDescriptorResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.packages.PublishPackageDescriptorRequest,
 *   !proto.packages.PublishPackageDescriptorResponse>}
 */
const methodInfo_PackageDiscovery_PublishPackageDescriptor = new grpc.web.AbstractClientBase.MethodInfo(
  proto.packages.PublishPackageDescriptorResponse,
  /**
   * @param {!proto.packages.PublishPackageDescriptorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.packages.PublishPackageDescriptorResponse.deserializeBinary
);


/**
 * @param {!proto.packages.PublishPackageDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.packages.PublishPackageDescriptorResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.packages.PublishPackageDescriptorResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.packages.PackageDiscoveryClient.prototype.publishPackageDescriptor =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/packages.PackageDiscovery/PublishPackageDescriptor',
      request,
      metadata || {},
      methodDescriptor_PackageDiscovery_PublishPackageDescriptor,
      callback);
};


/**
 * @param {!proto.packages.PublishPackageDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.packages.PublishPackageDescriptorResponse>}
 *     Promise that resolves to the response
 */
proto.packages.PackageDiscoveryPromiseClient.prototype.publishPackageDescriptor =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/packages.PackageDiscovery/PublishPackageDescriptor',
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
proto.packages.PackageRepositoryClient =
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
proto.packages.PackageRepositoryPromiseClient =
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
 *   !proto.packages.DownloadBundleRequest,
 *   !proto.packages.DownloadBundleResponse>}
 */
const methodDescriptor_PackageRepository_DownloadBundle = new grpc.web.MethodDescriptor(
  '/packages.PackageRepository/DownloadBundle',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.packages.DownloadBundleRequest,
  proto.packages.DownloadBundleResponse,
  /**
   * @param {!proto.packages.DownloadBundleRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.packages.DownloadBundleResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.packages.DownloadBundleRequest,
 *   !proto.packages.DownloadBundleResponse>}
 */
const methodInfo_PackageRepository_DownloadBundle = new grpc.web.AbstractClientBase.MethodInfo(
  proto.packages.DownloadBundleResponse,
  /**
   * @param {!proto.packages.DownloadBundleRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.packages.DownloadBundleResponse.deserializeBinary
);


/**
 * @param {!proto.packages.DownloadBundleRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.packages.DownloadBundleResponse>}
 *     The XHR Node Readable Stream
 */
proto.packages.PackageRepositoryClient.prototype.downloadBundle =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/packages.PackageRepository/DownloadBundle',
      request,
      metadata || {},
      methodDescriptor_PackageRepository_DownloadBundle);
};


/**
 * @param {!proto.packages.DownloadBundleRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.packages.DownloadBundleResponse>}
 *     The XHR Node Readable Stream
 */
proto.packages.PackageRepositoryPromiseClient.prototype.downloadBundle =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/packages.PackageRepository/DownloadBundle',
      request,
      metadata || {},
      methodDescriptor_PackageRepository_DownloadBundle);
};


module.exports = proto.packages;

