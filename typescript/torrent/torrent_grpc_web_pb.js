/**
 * @fileoverview gRPC-Web generated client stub for torrent
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.torrent = require('./torrent_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.torrent.TorrentServiceClient =
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
proto.torrent.TorrentServicePromiseClient =
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
 *   !proto.torrent.DownloadTorrentRequest,
 *   !proto.torrent.DownloadTorrentResponse>}
 */
const methodDescriptor_TorrentService_DownloadTorrent = new grpc.web.MethodDescriptor(
  '/torrent.TorrentService/DownloadTorrent',
  grpc.web.MethodType.UNARY,
  proto.torrent.DownloadTorrentRequest,
  proto.torrent.DownloadTorrentResponse,
  /**
   * @param {!proto.torrent.DownloadTorrentRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.torrent.DownloadTorrentResponse.deserializeBinary
);


/**
 * @param {!proto.torrent.DownloadTorrentRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.torrent.DownloadTorrentResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.torrent.DownloadTorrentResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.torrent.TorrentServiceClient.prototype.downloadTorrent =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/torrent.TorrentService/DownloadTorrent',
      request,
      metadata || {},
      methodDescriptor_TorrentService_DownloadTorrent,
      callback);
};


/**
 * @param {!proto.torrent.DownloadTorrentRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.torrent.DownloadTorrentResponse>}
 *     Promise that resolves to the response
 */
proto.torrent.TorrentServicePromiseClient.prototype.downloadTorrent =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/torrent.TorrentService/DownloadTorrent',
      request,
      metadata || {},
      methodDescriptor_TorrentService_DownloadTorrent);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.torrent.GetTorrentInfosRequest,
 *   !proto.torrent.GetTorrentInfosResponse>}
 */
const methodDescriptor_TorrentService_GetTorrentInfos = new grpc.web.MethodDescriptor(
  '/torrent.TorrentService/GetTorrentInfos',
  grpc.web.MethodType.UNARY,
  proto.torrent.GetTorrentInfosRequest,
  proto.torrent.GetTorrentInfosResponse,
  /**
   * @param {!proto.torrent.GetTorrentInfosRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.torrent.GetTorrentInfosResponse.deserializeBinary
);


/**
 * @param {!proto.torrent.GetTorrentInfosRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.torrent.GetTorrentInfosResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.torrent.GetTorrentInfosResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.torrent.TorrentServiceClient.prototype.getTorrentInfos =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/torrent.TorrentService/GetTorrentInfos',
      request,
      metadata || {},
      methodDescriptor_TorrentService_GetTorrentInfos,
      callback);
};


/**
 * @param {!proto.torrent.GetTorrentInfosRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.torrent.GetTorrentInfosResponse>}
 *     Promise that resolves to the response
 */
proto.torrent.TorrentServicePromiseClient.prototype.getTorrentInfos =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/torrent.TorrentService/GetTorrentInfos',
      request,
      metadata || {},
      methodDescriptor_TorrentService_GetTorrentInfos);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.torrent.DropTorrentRequest,
 *   !proto.torrent.DropTorrentResponse>}
 */
const methodDescriptor_TorrentService_DropTorrent = new grpc.web.MethodDescriptor(
  '/torrent.TorrentService/DropTorrent',
  grpc.web.MethodType.UNARY,
  proto.torrent.DropTorrentRequest,
  proto.torrent.DropTorrentResponse,
  /**
   * @param {!proto.torrent.DropTorrentRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.torrent.DropTorrentResponse.deserializeBinary
);


/**
 * @param {!proto.torrent.DropTorrentRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.torrent.DropTorrentResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.torrent.DropTorrentResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.torrent.TorrentServiceClient.prototype.dropTorrent =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/torrent.TorrentService/DropTorrent',
      request,
      metadata || {},
      methodDescriptor_TorrentService_DropTorrent,
      callback);
};


/**
 * @param {!proto.torrent.DropTorrentRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.torrent.DropTorrentResponse>}
 *     Promise that resolves to the response
 */
proto.torrent.TorrentServicePromiseClient.prototype.dropTorrent =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/torrent.TorrentService/DropTorrent',
      request,
      metadata || {},
      methodDescriptor_TorrentService_DropTorrent);
};


module.exports = proto.torrent;
