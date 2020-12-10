/**
 * @fileoverview gRPC-Web generated client stub for search
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.search = require('./search_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.search.SearchServiceClient =
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
proto.search.SearchServicePromiseClient =
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
 *   !proto.search.StopRequest,
 *   !proto.search.StopResponse>}
 */
const methodDescriptor_SearchService_Stop = new grpc.web.MethodDescriptor(
  '/search.SearchService/Stop',
  grpc.web.MethodType.UNARY,
  proto.search.StopRequest,
  proto.search.StopResponse,
  /**
   * @param {!proto.search.StopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.search.StopResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.search.StopRequest,
 *   !proto.search.StopResponse>}
 */
const methodInfo_SearchService_Stop = new grpc.web.AbstractClientBase.MethodInfo(
  proto.search.StopResponse,
  /**
   * @param {!proto.search.StopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.search.StopResponse.deserializeBinary
);


/**
 * @param {!proto.search.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.search.StopResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.search.StopResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.search.SearchServiceClient.prototype.stop =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/search.SearchService/Stop',
      request,
      metadata || {},
      methodDescriptor_SearchService_Stop,
      callback);
};


/**
 * @param {!proto.search.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.search.StopResponse>}
 *     Promise that resolves to the response
 */
proto.search.SearchServicePromiseClient.prototype.stop =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/search.SearchService/Stop',
      request,
      metadata || {},
      methodDescriptor_SearchService_Stop);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.search.GetEngineVersionRequest,
 *   !proto.search.GetEngineVersionResponse>}
 */
const methodDescriptor_SearchService_GetEngineVersion = new grpc.web.MethodDescriptor(
  '/search.SearchService/GetEngineVersion',
  grpc.web.MethodType.UNARY,
  proto.search.GetEngineVersionRequest,
  proto.search.GetEngineVersionResponse,
  /**
   * @param {!proto.search.GetEngineVersionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.search.GetEngineVersionResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.search.GetEngineVersionRequest,
 *   !proto.search.GetEngineVersionResponse>}
 */
const methodInfo_SearchService_GetEngineVersion = new grpc.web.AbstractClientBase.MethodInfo(
  proto.search.GetEngineVersionResponse,
  /**
   * @param {!proto.search.GetEngineVersionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.search.GetEngineVersionResponse.deserializeBinary
);


/**
 * @param {!proto.search.GetEngineVersionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.search.GetEngineVersionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.search.GetEngineVersionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.search.SearchServiceClient.prototype.getEngineVersion =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/search.SearchService/GetEngineVersion',
      request,
      metadata || {},
      methodDescriptor_SearchService_GetEngineVersion,
      callback);
};


/**
 * @param {!proto.search.GetEngineVersionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.search.GetEngineVersionResponse>}
 *     Promise that resolves to the response
 */
proto.search.SearchServicePromiseClient.prototype.getEngineVersion =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/search.SearchService/GetEngineVersion',
      request,
      metadata || {},
      methodDescriptor_SearchService_GetEngineVersion);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.search.IndexJsonObjectRequest,
 *   !proto.search.IndexJsonObjectResponse>}
 */
const methodDescriptor_SearchService_IndexJsonObject = new grpc.web.MethodDescriptor(
  '/search.SearchService/IndexJsonObject',
  grpc.web.MethodType.UNARY,
  proto.search.IndexJsonObjectRequest,
  proto.search.IndexJsonObjectResponse,
  /**
   * @param {!proto.search.IndexJsonObjectRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.search.IndexJsonObjectResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.search.IndexJsonObjectRequest,
 *   !proto.search.IndexJsonObjectResponse>}
 */
const methodInfo_SearchService_IndexJsonObject = new grpc.web.AbstractClientBase.MethodInfo(
  proto.search.IndexJsonObjectResponse,
  /**
   * @param {!proto.search.IndexJsonObjectRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.search.IndexJsonObjectResponse.deserializeBinary
);


/**
 * @param {!proto.search.IndexJsonObjectRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.search.IndexJsonObjectResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.search.IndexJsonObjectResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.search.SearchServiceClient.prototype.indexJsonObject =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/search.SearchService/IndexJsonObject',
      request,
      metadata || {},
      methodDescriptor_SearchService_IndexJsonObject,
      callback);
};


/**
 * @param {!proto.search.IndexJsonObjectRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.search.IndexJsonObjectResponse>}
 *     Promise that resolves to the response
 */
proto.search.SearchServicePromiseClient.prototype.indexJsonObject =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/search.SearchService/IndexJsonObject',
      request,
      metadata || {},
      methodDescriptor_SearchService_IndexJsonObject);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.search.IndexFileRequest,
 *   !proto.search.IndexFileResponse>}
 */
const methodDescriptor_SearchService_IndexFile = new grpc.web.MethodDescriptor(
  '/search.SearchService/IndexFile',
  grpc.web.MethodType.UNARY,
  proto.search.IndexFileRequest,
  proto.search.IndexFileResponse,
  /**
   * @param {!proto.search.IndexFileRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.search.IndexFileResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.search.IndexFileRequest,
 *   !proto.search.IndexFileResponse>}
 */
const methodInfo_SearchService_IndexFile = new grpc.web.AbstractClientBase.MethodInfo(
  proto.search.IndexFileResponse,
  /**
   * @param {!proto.search.IndexFileRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.search.IndexFileResponse.deserializeBinary
);


/**
 * @param {!proto.search.IndexFileRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.search.IndexFileResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.search.IndexFileResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.search.SearchServiceClient.prototype.indexFile =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/search.SearchService/IndexFile',
      request,
      metadata || {},
      methodDescriptor_SearchService_IndexFile,
      callback);
};


/**
 * @param {!proto.search.IndexFileRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.search.IndexFileResponse>}
 *     Promise that resolves to the response
 */
proto.search.SearchServicePromiseClient.prototype.indexFile =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/search.SearchService/IndexFile',
      request,
      metadata || {},
      methodDescriptor_SearchService_IndexFile);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.search.IndexDirRequest,
 *   !proto.search.IndexDirResponse>}
 */
const methodDescriptor_SearchService_IndexDir = new grpc.web.MethodDescriptor(
  '/search.SearchService/IndexDir',
  grpc.web.MethodType.UNARY,
  proto.search.IndexDirRequest,
  proto.search.IndexDirResponse,
  /**
   * @param {!proto.search.IndexDirRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.search.IndexDirResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.search.IndexDirRequest,
 *   !proto.search.IndexDirResponse>}
 */
const methodInfo_SearchService_IndexDir = new grpc.web.AbstractClientBase.MethodInfo(
  proto.search.IndexDirResponse,
  /**
   * @param {!proto.search.IndexDirRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.search.IndexDirResponse.deserializeBinary
);


/**
 * @param {!proto.search.IndexDirRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.search.IndexDirResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.search.IndexDirResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.search.SearchServiceClient.prototype.indexDir =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/search.SearchService/IndexDir',
      request,
      metadata || {},
      methodDescriptor_SearchService_IndexDir,
      callback);
};


/**
 * @param {!proto.search.IndexDirRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.search.IndexDirResponse>}
 *     Promise that resolves to the response
 */
proto.search.SearchServicePromiseClient.prototype.indexDir =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/search.SearchService/IndexDir',
      request,
      metadata || {},
      methodDescriptor_SearchService_IndexDir);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.search.CountRequest,
 *   !proto.search.CountResponse>}
 */
const methodDescriptor_SearchService_Count = new grpc.web.MethodDescriptor(
  '/search.SearchService/Count',
  grpc.web.MethodType.UNARY,
  proto.search.CountRequest,
  proto.search.CountResponse,
  /**
   * @param {!proto.search.CountRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.search.CountResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.search.CountRequest,
 *   !proto.search.CountResponse>}
 */
const methodInfo_SearchService_Count = new grpc.web.AbstractClientBase.MethodInfo(
  proto.search.CountResponse,
  /**
   * @param {!proto.search.CountRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.search.CountResponse.deserializeBinary
);


/**
 * @param {!proto.search.CountRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.search.CountResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.search.CountResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.search.SearchServiceClient.prototype.count =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/search.SearchService/Count',
      request,
      metadata || {},
      methodDescriptor_SearchService_Count,
      callback);
};


/**
 * @param {!proto.search.CountRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.search.CountResponse>}
 *     Promise that resolves to the response
 */
proto.search.SearchServicePromiseClient.prototype.count =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/search.SearchService/Count',
      request,
      metadata || {},
      methodDescriptor_SearchService_Count);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.search.DeleteDocumentRequest,
 *   !proto.search.DeleteDocumentResponse>}
 */
const methodDescriptor_SearchService_DeleteDocument = new grpc.web.MethodDescriptor(
  '/search.SearchService/DeleteDocument',
  grpc.web.MethodType.UNARY,
  proto.search.DeleteDocumentRequest,
  proto.search.DeleteDocumentResponse,
  /**
   * @param {!proto.search.DeleteDocumentRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.search.DeleteDocumentResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.search.DeleteDocumentRequest,
 *   !proto.search.DeleteDocumentResponse>}
 */
const methodInfo_SearchService_DeleteDocument = new grpc.web.AbstractClientBase.MethodInfo(
  proto.search.DeleteDocumentResponse,
  /**
   * @param {!proto.search.DeleteDocumentRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.search.DeleteDocumentResponse.deserializeBinary
);


/**
 * @param {!proto.search.DeleteDocumentRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.search.DeleteDocumentResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.search.DeleteDocumentResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.search.SearchServiceClient.prototype.deleteDocument =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/search.SearchService/DeleteDocument',
      request,
      metadata || {},
      methodDescriptor_SearchService_DeleteDocument,
      callback);
};


/**
 * @param {!proto.search.DeleteDocumentRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.search.DeleteDocumentResponse>}
 *     Promise that resolves to the response
 */
proto.search.SearchServicePromiseClient.prototype.deleteDocument =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/search.SearchService/DeleteDocument',
      request,
      metadata || {},
      methodDescriptor_SearchService_DeleteDocument);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.search.SearchDocumentsRequest,
 *   !proto.search.SearchDocumentsResponse>}
 */
const methodDescriptor_SearchService_SearchDocuments = new grpc.web.MethodDescriptor(
  '/search.SearchService/SearchDocuments',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.search.SearchDocumentsRequest,
  proto.search.SearchDocumentsResponse,
  /**
   * @param {!proto.search.SearchDocumentsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.search.SearchDocumentsResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.search.SearchDocumentsRequest,
 *   !proto.search.SearchDocumentsResponse>}
 */
const methodInfo_SearchService_SearchDocuments = new grpc.web.AbstractClientBase.MethodInfo(
  proto.search.SearchDocumentsResponse,
  /**
   * @param {!proto.search.SearchDocumentsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.search.SearchDocumentsResponse.deserializeBinary
);


/**
 * @param {!proto.search.SearchDocumentsRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.search.SearchDocumentsResponse>}
 *     The XHR Node Readable Stream
 */
proto.search.SearchServiceClient.prototype.searchDocuments =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/search.SearchService/SearchDocuments',
      request,
      metadata || {},
      methodDescriptor_SearchService_SearchDocuments);
};


/**
 * @param {!proto.search.SearchDocumentsRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.search.SearchDocumentsResponse>}
 *     The XHR Node Readable Stream
 */
proto.search.SearchServicePromiseClient.prototype.searchDocuments =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/search.SearchService/SearchDocuments',
      request,
      metadata || {},
      methodDescriptor_SearchService_SearchDocuments);
};


module.exports = proto.search;

