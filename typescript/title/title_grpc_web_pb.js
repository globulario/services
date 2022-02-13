/**
 * @fileoverview gRPC-Web generated client stub for title
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.title = require('./title_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.title.TitleServiceClient =
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
proto.title.TitleServicePromiseClient =
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
 *   !proto.title.CreateTitleRequest,
 *   !proto.title.CreateTitleResponse>}
 */
const methodDescriptor_TitleService_CreateTitle = new grpc.web.MethodDescriptor(
  '/title.TitleService/CreateTitle',
  grpc.web.MethodType.UNARY,
  proto.title.CreateTitleRequest,
  proto.title.CreateTitleResponse,
  /**
   * @param {!proto.title.CreateTitleRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.title.CreateTitleResponse.deserializeBinary
);


/**
 * @param {!proto.title.CreateTitleRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.title.CreateTitleResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.title.CreateTitleResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.title.TitleServiceClient.prototype.createTitle =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/title.TitleService/CreateTitle',
      request,
      metadata || {},
      methodDescriptor_TitleService_CreateTitle,
      callback);
};


/**
 * @param {!proto.title.CreateTitleRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.title.CreateTitleResponse>}
 *     Promise that resolves to the response
 */
proto.title.TitleServicePromiseClient.prototype.createTitle =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/title.TitleService/CreateTitle',
      request,
      metadata || {},
      methodDescriptor_TitleService_CreateTitle);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.title.GetTitleByIdRequest,
 *   !proto.title.GetTitleByIdResponse>}
 */
const methodDescriptor_TitleService_GetTitleById = new grpc.web.MethodDescriptor(
  '/title.TitleService/GetTitleById',
  grpc.web.MethodType.UNARY,
  proto.title.GetTitleByIdRequest,
  proto.title.GetTitleByIdResponse,
  /**
   * @param {!proto.title.GetTitleByIdRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.title.GetTitleByIdResponse.deserializeBinary
);


/**
 * @param {!proto.title.GetTitleByIdRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.title.GetTitleByIdResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.title.GetTitleByIdResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.title.TitleServiceClient.prototype.getTitleById =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/title.TitleService/GetTitleById',
      request,
      metadata || {},
      methodDescriptor_TitleService_GetTitleById,
      callback);
};


/**
 * @param {!proto.title.GetTitleByIdRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.title.GetTitleByIdResponse>}
 *     Promise that resolves to the response
 */
proto.title.TitleServicePromiseClient.prototype.getTitleById =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/title.TitleService/GetTitleById',
      request,
      metadata || {},
      methodDescriptor_TitleService_GetTitleById);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.title.DeleteTitleRequest,
 *   !proto.title.DeleteTitleResponse>}
 */
const methodDescriptor_TitleService_DeleteTitle = new grpc.web.MethodDescriptor(
  '/title.TitleService/DeleteTitle',
  grpc.web.MethodType.UNARY,
  proto.title.DeleteTitleRequest,
  proto.title.DeleteTitleResponse,
  /**
   * @param {!proto.title.DeleteTitleRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.title.DeleteTitleResponse.deserializeBinary
);


/**
 * @param {!proto.title.DeleteTitleRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.title.DeleteTitleResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.title.DeleteTitleResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.title.TitleServiceClient.prototype.deleteTitle =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/title.TitleService/DeleteTitle',
      request,
      metadata || {},
      methodDescriptor_TitleService_DeleteTitle,
      callback);
};


/**
 * @param {!proto.title.DeleteTitleRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.title.DeleteTitleResponse>}
 *     Promise that resolves to the response
 */
proto.title.TitleServicePromiseClient.prototype.deleteTitle =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/title.TitleService/DeleteTitle',
      request,
      metadata || {},
      methodDescriptor_TitleService_DeleteTitle);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.title.AssociateFileWithTitleRequest,
 *   !proto.title.AssociateFileWithTitleResponse>}
 */
const methodDescriptor_TitleService_AssociateFileWithTitle = new grpc.web.MethodDescriptor(
  '/title.TitleService/AssociateFileWithTitle',
  grpc.web.MethodType.UNARY,
  proto.title.AssociateFileWithTitleRequest,
  proto.title.AssociateFileWithTitleResponse,
  /**
   * @param {!proto.title.AssociateFileWithTitleRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.title.AssociateFileWithTitleResponse.deserializeBinary
);


/**
 * @param {!proto.title.AssociateFileWithTitleRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.title.AssociateFileWithTitleResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.title.AssociateFileWithTitleResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.title.TitleServiceClient.prototype.associateFileWithTitle =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/title.TitleService/AssociateFileWithTitle',
      request,
      metadata || {},
      methodDescriptor_TitleService_AssociateFileWithTitle,
      callback);
};


/**
 * @param {!proto.title.AssociateFileWithTitleRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.title.AssociateFileWithTitleResponse>}
 *     Promise that resolves to the response
 */
proto.title.TitleServicePromiseClient.prototype.associateFileWithTitle =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/title.TitleService/AssociateFileWithTitle',
      request,
      metadata || {},
      methodDescriptor_TitleService_AssociateFileWithTitle);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.title.DissociateFileWithTitleRequest,
 *   !proto.title.DissociateFileWithTitleResponse>}
 */
const methodDescriptor_TitleService_DissociateFileWithTitle = new grpc.web.MethodDescriptor(
  '/title.TitleService/DissociateFileWithTitle',
  grpc.web.MethodType.UNARY,
  proto.title.DissociateFileWithTitleRequest,
  proto.title.DissociateFileWithTitleResponse,
  /**
   * @param {!proto.title.DissociateFileWithTitleRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.title.DissociateFileWithTitleResponse.deserializeBinary
);


/**
 * @param {!proto.title.DissociateFileWithTitleRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.title.DissociateFileWithTitleResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.title.DissociateFileWithTitleResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.title.TitleServiceClient.prototype.dissociateFileWithTitle =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/title.TitleService/DissociateFileWithTitle',
      request,
      metadata || {},
      methodDescriptor_TitleService_DissociateFileWithTitle,
      callback);
};


/**
 * @param {!proto.title.DissociateFileWithTitleRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.title.DissociateFileWithTitleResponse>}
 *     Promise that resolves to the response
 */
proto.title.TitleServicePromiseClient.prototype.dissociateFileWithTitle =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/title.TitleService/DissociateFileWithTitle',
      request,
      metadata || {},
      methodDescriptor_TitleService_DissociateFileWithTitle);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.title.GetFileTitlesRequest,
 *   !proto.title.GetFileTitlesResponse>}
 */
const methodDescriptor_TitleService_GetFileTitles = new grpc.web.MethodDescriptor(
  '/title.TitleService/GetFileTitles',
  grpc.web.MethodType.UNARY,
  proto.title.GetFileTitlesRequest,
  proto.title.GetFileTitlesResponse,
  /**
   * @param {!proto.title.GetFileTitlesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.title.GetFileTitlesResponse.deserializeBinary
);


/**
 * @param {!proto.title.GetFileTitlesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.title.GetFileTitlesResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.title.GetFileTitlesResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.title.TitleServiceClient.prototype.getFileTitles =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/title.TitleService/GetFileTitles',
      request,
      metadata || {},
      methodDescriptor_TitleService_GetFileTitles,
      callback);
};


/**
 * @param {!proto.title.GetFileTitlesRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.title.GetFileTitlesResponse>}
 *     Promise that resolves to the response
 */
proto.title.TitleServicePromiseClient.prototype.getFileTitles =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/title.TitleService/GetFileTitles',
      request,
      metadata || {},
      methodDescriptor_TitleService_GetFileTitles);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.title.GetTitleFilesRequest,
 *   !proto.title.GetTitleFilesResponse>}
 */
const methodDescriptor_TitleService_GetTitleFiles = new grpc.web.MethodDescriptor(
  '/title.TitleService/GetTitleFiles',
  grpc.web.MethodType.UNARY,
  proto.title.GetTitleFilesRequest,
  proto.title.GetTitleFilesResponse,
  /**
   * @param {!proto.title.GetTitleFilesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.title.GetTitleFilesResponse.deserializeBinary
);


/**
 * @param {!proto.title.GetTitleFilesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.title.GetTitleFilesResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.title.GetTitleFilesResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.title.TitleServiceClient.prototype.getTitleFiles =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/title.TitleService/GetTitleFiles',
      request,
      metadata || {},
      methodDescriptor_TitleService_GetTitleFiles,
      callback);
};


/**
 * @param {!proto.title.GetTitleFilesRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.title.GetTitleFilesResponse>}
 *     Promise that resolves to the response
 */
proto.title.TitleServicePromiseClient.prototype.getTitleFiles =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/title.TitleService/GetTitleFiles',
      request,
      metadata || {},
      methodDescriptor_TitleService_GetTitleFiles);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.title.SearchTitlesRequest,
 *   !proto.title.SearchTitlesResponse>}
 */
const methodDescriptor_TitleService_SearchTitles = new grpc.web.MethodDescriptor(
  '/title.TitleService/SearchTitles',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.title.SearchTitlesRequest,
  proto.title.SearchTitlesResponse,
  /**
   * @param {!proto.title.SearchTitlesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.title.SearchTitlesResponse.deserializeBinary
);


/**
 * @param {!proto.title.SearchTitlesRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.title.SearchTitlesResponse>}
 *     The XHR Node Readable Stream
 */
proto.title.TitleServiceClient.prototype.searchTitles =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/title.TitleService/SearchTitles',
      request,
      metadata || {},
      methodDescriptor_TitleService_SearchTitles);
};


/**
 * @param {!proto.title.SearchTitlesRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.title.SearchTitlesResponse>}
 *     The XHR Node Readable Stream
 */
proto.title.TitleServicePromiseClient.prototype.searchTitles =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/title.TitleService/SearchTitles',
      request,
      metadata || {},
      methodDescriptor_TitleService_SearchTitles);
};


module.exports = proto.title;

