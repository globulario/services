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
proto.services.ServiceDiscoveryClient =
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
proto.services.ServiceDiscoveryPromiseClient =
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
const methodDescriptor_ServiceDiscovery_FindPackages = new grpc.web.MethodDescriptor(
  '/services.ServiceDiscovery/FindPackages',
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
const methodInfo_ServiceDiscovery_FindPackages = new grpc.web.AbstractClientBase.MethodInfo(
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
proto.services.ServiceDiscoveryClient.prototype.findPackages =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/services.ServiceDiscovery/FindPackages',
      request,
      metadata || {},
      methodDescriptor_ServiceDiscovery_FindPackages,
      callback);
};


/**
 * @param {!proto.services.FindPackagesDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.services.FindPackagesDescriptorResponse>}
 *     A native promise that resolves to the response
 */
proto.services.ServiceDiscoveryPromiseClient.prototype.findPackages =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/services.ServiceDiscovery/FindPackages',
      request,
      metadata || {},
      methodDescriptor_ServiceDiscovery_FindPackages);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.services.GetPackageDescriptorRequest,
 *   !proto.services.GetPackageDescriptorResponse>}
 */
const methodDescriptor_ServiceDiscovery_GetPackageDescriptor = new grpc.web.MethodDescriptor(
  '/services.ServiceDiscovery/GetPackageDescriptor',
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
const methodInfo_ServiceDiscovery_GetPackageDescriptor = new grpc.web.AbstractClientBase.MethodInfo(
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
proto.services.ServiceDiscoveryClient.prototype.getPackageDescriptor =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/services.ServiceDiscovery/GetPackageDescriptor',
      request,
      metadata || {},
      methodDescriptor_ServiceDiscovery_GetPackageDescriptor,
      callback);
};


/**
 * @param {!proto.services.GetPackageDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.services.GetPackageDescriptorResponse>}
 *     A native promise that resolves to the response
 */
proto.services.ServiceDiscoveryPromiseClient.prototype.getPackageDescriptor =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/services.ServiceDiscovery/GetPackageDescriptor',
      request,
      metadata || {},
      methodDescriptor_ServiceDiscovery_GetPackageDescriptor);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.services.GetPackagesDescriptorRequest,
 *   !proto.services.GetPackagesDescriptorResponse>}
 */
const methodDescriptor_ServiceDiscovery_GetPackagesDescriptor = new grpc.web.MethodDescriptor(
  '/services.ServiceDiscovery/GetPackagesDescriptor',
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
const methodInfo_ServiceDiscovery_GetPackagesDescriptor = new grpc.web.AbstractClientBase.MethodInfo(
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
proto.services.ServiceDiscoveryClient.prototype.getPackagesDescriptor =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/services.ServiceDiscovery/GetPackagesDescriptor',
      request,
      metadata || {},
      methodDescriptor_ServiceDiscovery_GetPackagesDescriptor);
};


/**
 * @param {!proto.services.GetPackagesDescriptorRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.services.GetPackagesDescriptorResponse>}
 *     The XHR Node Readable Stream
 */
proto.services.ServiceDiscoveryPromiseClient.prototype.getPackagesDescriptor =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/services.ServiceDiscovery/GetPackagesDescriptor',
      request,
      metadata || {},
      methodDescriptor_ServiceDiscovery_GetPackagesDescriptor);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.services.SetPackageDescriptorRequest,
 *   !proto.services.SetPackageDescriptorResponse>}
 */
const methodDescriptor_ServiceDiscovery_SetPackageDescriptor = new grpc.web.MethodDescriptor(
  '/services.ServiceDiscovery/SetPackageDescriptor',
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
const methodInfo_ServiceDiscovery_SetPackageDescriptor = new grpc.web.AbstractClientBase.MethodInfo(
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
proto.services.ServiceDiscoveryClient.prototype.setPackageDescriptor =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/services.ServiceDiscovery/SetPackageDescriptor',
      request,
      metadata || {},
      methodDescriptor_ServiceDiscovery_SetPackageDescriptor,
      callback);
};


/**
 * @param {!proto.services.SetPackageDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.services.SetPackageDescriptorResponse>}
 *     A native promise that resolves to the response
 */
proto.services.ServiceDiscoveryPromiseClient.prototype.setPackageDescriptor =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/services.ServiceDiscovery/SetPackageDescriptor',
      request,
      metadata || {},
      methodDescriptor_ServiceDiscovery_SetPackageDescriptor);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.services.PublishPackageDescriptorRequest,
 *   !proto.services.PublishPackageDescriptorResponse>}
 */
const methodDescriptor_ServiceDiscovery_PublishPackageDescriptor = new grpc.web.MethodDescriptor(
  '/services.ServiceDiscovery/PublishPackageDescriptor',
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
const methodInfo_ServiceDiscovery_PublishPackageDescriptor = new grpc.web.AbstractClientBase.MethodInfo(
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
proto.services.ServiceDiscoveryClient.prototype.publishPackageDescriptor =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/services.ServiceDiscovery/PublishPackageDescriptor',
      request,
      metadata || {},
      methodDescriptor_ServiceDiscovery_PublishPackageDescriptor,
      callback);
};


/**
 * @param {!proto.services.PublishPackageDescriptorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.services.PublishPackageDescriptorResponse>}
 *     A native promise that resolves to the response
 */
proto.services.ServiceDiscoveryPromiseClient.prototype.publishPackageDescriptor =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/services.ServiceDiscovery/PublishPackageDescriptor',
      request,
      metadata || {},
      methodDescriptor_ServiceDiscovery_PublishPackageDescriptor);
};


/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.services.ServiceRepositoryClient =
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
proto.services.ServiceRepositoryPromiseClient =
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
const methodDescriptor_ServiceRepository_DownloadBundle = new grpc.web.MethodDescriptor(
  '/services.ServiceRepository/DownloadBundle',
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
const methodInfo_ServiceRepository_DownloadBundle = new grpc.web.AbstractClientBase.MethodInfo(
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
proto.services.ServiceRepositoryClient.prototype.downloadBundle =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/services.ServiceRepository/DownloadBundle',
      request,
      metadata || {},
      methodDescriptor_ServiceRepository_DownloadBundle);
};


/**
 * @param {!proto.services.DownloadBundleRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.services.DownloadBundleResponse>}
 *     The XHR Node Readable Stream
 */
proto.services.ServiceRepositoryPromiseClient.prototype.downloadBundle =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/services.ServiceRepository/DownloadBundle',
      request,
      metadata || {},
      methodDescriptor_ServiceRepository_DownloadBundle);
};


module.exports = proto.services;

