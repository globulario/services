/**
 * @fileoverview gRPC-Web generated client stub for file
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.file = require('./file_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.file.FileServiceClient =
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
proto.file.FileServicePromiseClient =
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
 *   !proto.file.StopRequest,
 *   !proto.file.StopResponse>}
 */
const methodDescriptor_FileService_Stop = new grpc.web.MethodDescriptor(
  '/file.FileService/Stop',
  grpc.web.MethodType.UNARY,
  proto.file.StopRequest,
  proto.file.StopResponse,
  /**
   * @param {!proto.file.StopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.StopResponse.deserializeBinary
);


/**
 * @param {!proto.file.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.StopResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.StopResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.stop =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/Stop',
      request,
      metadata || {},
      methodDescriptor_FileService_Stop,
      callback);
};


/**
 * @param {!proto.file.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.StopResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.stop =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/Stop',
      request,
      metadata || {},
      methodDescriptor_FileService_Stop);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.ReadDirRequest,
 *   !proto.file.ReadDirResponse>}
 */
const methodDescriptor_FileService_ReadDir = new grpc.web.MethodDescriptor(
  '/file.FileService/ReadDir',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.file.ReadDirRequest,
  proto.file.ReadDirResponse,
  /**
   * @param {!proto.file.ReadDirRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.ReadDirResponse.deserializeBinary
);


/**
 * @param {!proto.file.ReadDirRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.file.ReadDirResponse>}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.readDir =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/file.FileService/ReadDir',
      request,
      metadata || {},
      methodDescriptor_FileService_ReadDir);
};


/**
 * @param {!proto.file.ReadDirRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.file.ReadDirResponse>}
 *     The XHR Node Readable Stream
 */
proto.file.FileServicePromiseClient.prototype.readDir =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/file.FileService/ReadDir',
      request,
      metadata || {},
      methodDescriptor_FileService_ReadDir);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.CreateDirRequest,
 *   !proto.file.CreateDirResponse>}
 */
const methodDescriptor_FileService_CreateDir = new grpc.web.MethodDescriptor(
  '/file.FileService/CreateDir',
  grpc.web.MethodType.UNARY,
  proto.file.CreateDirRequest,
  proto.file.CreateDirResponse,
  /**
   * @param {!proto.file.CreateDirRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.CreateDirResponse.deserializeBinary
);


/**
 * @param {!proto.file.CreateDirRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.CreateDirResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.CreateDirResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.createDir =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/CreateDir',
      request,
      metadata || {},
      methodDescriptor_FileService_CreateDir,
      callback);
};


/**
 * @param {!proto.file.CreateDirRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.CreateDirResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.createDir =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/CreateDir',
      request,
      metadata || {},
      methodDescriptor_FileService_CreateDir);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.DeleteDirRequest,
 *   !proto.file.DeleteDirResponse>}
 */
const methodDescriptor_FileService_DeleteDir = new grpc.web.MethodDescriptor(
  '/file.FileService/DeleteDir',
  grpc.web.MethodType.UNARY,
  proto.file.DeleteDirRequest,
  proto.file.DeleteDirResponse,
  /**
   * @param {!proto.file.DeleteDirRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.DeleteDirResponse.deserializeBinary
);


/**
 * @param {!proto.file.DeleteDirRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.DeleteDirResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.DeleteDirResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.deleteDir =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/DeleteDir',
      request,
      metadata || {},
      methodDescriptor_FileService_DeleteDir,
      callback);
};


/**
 * @param {!proto.file.DeleteDirRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.DeleteDirResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.deleteDir =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/DeleteDir',
      request,
      metadata || {},
      methodDescriptor_FileService_DeleteDir);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.RenameRequest,
 *   !proto.file.RenameResponse>}
 */
const methodDescriptor_FileService_Rename = new grpc.web.MethodDescriptor(
  '/file.FileService/Rename',
  grpc.web.MethodType.UNARY,
  proto.file.RenameRequest,
  proto.file.RenameResponse,
  /**
   * @param {!proto.file.RenameRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.RenameResponse.deserializeBinary
);


/**
 * @param {!proto.file.RenameRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.RenameResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.RenameResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.rename =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/Rename',
      request,
      metadata || {},
      methodDescriptor_FileService_Rename,
      callback);
};


/**
 * @param {!proto.file.RenameRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.RenameResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.rename =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/Rename',
      request,
      metadata || {},
      methodDescriptor_FileService_Rename);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.MoveRequest,
 *   !proto.file.MoveResponse>}
 */
const methodDescriptor_FileService_Move = new grpc.web.MethodDescriptor(
  '/file.FileService/Move',
  grpc.web.MethodType.UNARY,
  proto.file.MoveRequest,
  proto.file.MoveResponse,
  /**
   * @param {!proto.file.MoveRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.MoveResponse.deserializeBinary
);


/**
 * @param {!proto.file.MoveRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.MoveResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.MoveResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.move =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/Move',
      request,
      metadata || {},
      methodDescriptor_FileService_Move,
      callback);
};


/**
 * @param {!proto.file.MoveRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.MoveResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.move =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/Move',
      request,
      metadata || {},
      methodDescriptor_FileService_Move);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.CopyRequest,
 *   !proto.file.CopyResponse>}
 */
const methodDescriptor_FileService_Copy = new grpc.web.MethodDescriptor(
  '/file.FileService/Copy',
  grpc.web.MethodType.UNARY,
  proto.file.CopyRequest,
  proto.file.CopyResponse,
  /**
   * @param {!proto.file.CopyRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.CopyResponse.deserializeBinary
);


/**
 * @param {!proto.file.CopyRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.CopyResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.CopyResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.copy =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/Copy',
      request,
      metadata || {},
      methodDescriptor_FileService_Copy,
      callback);
};


/**
 * @param {!proto.file.CopyRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.CopyResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.copy =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/Copy',
      request,
      metadata || {},
      methodDescriptor_FileService_Copy);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.CreateArchiveRequest,
 *   !proto.file.CreateArchiveResponse>}
 */
const methodDescriptor_FileService_CreateAchive = new grpc.web.MethodDescriptor(
  '/file.FileService/CreateAchive',
  grpc.web.MethodType.UNARY,
  proto.file.CreateArchiveRequest,
  proto.file.CreateArchiveResponse,
  /**
   * @param {!proto.file.CreateArchiveRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.CreateArchiveResponse.deserializeBinary
);


/**
 * @param {!proto.file.CreateArchiveRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.CreateArchiveResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.CreateArchiveResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.createAchive =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/CreateAchive',
      request,
      metadata || {},
      methodDescriptor_FileService_CreateAchive,
      callback);
};


/**
 * @param {!proto.file.CreateArchiveRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.CreateArchiveResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.createAchive =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/CreateAchive',
      request,
      metadata || {},
      methodDescriptor_FileService_CreateAchive);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.GetFileInfoRequest,
 *   !proto.file.GetFileInfoResponse>}
 */
const methodDescriptor_FileService_GetFileInfo = new grpc.web.MethodDescriptor(
  '/file.FileService/GetFileInfo',
  grpc.web.MethodType.UNARY,
  proto.file.GetFileInfoRequest,
  proto.file.GetFileInfoResponse,
  /**
   * @param {!proto.file.GetFileInfoRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.GetFileInfoResponse.deserializeBinary
);


/**
 * @param {!proto.file.GetFileInfoRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.GetFileInfoResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.GetFileInfoResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.getFileInfo =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/GetFileInfo',
      request,
      metadata || {},
      methodDescriptor_FileService_GetFileInfo,
      callback);
};


/**
 * @param {!proto.file.GetFileInfoRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.GetFileInfoResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.getFileInfo =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/GetFileInfo',
      request,
      metadata || {},
      methodDescriptor_FileService_GetFileInfo);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.ReadFileRequest,
 *   !proto.file.ReadFileResponse>}
 */
const methodDescriptor_FileService_ReadFile = new grpc.web.MethodDescriptor(
  '/file.FileService/ReadFile',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.file.ReadFileRequest,
  proto.file.ReadFileResponse,
  /**
   * @param {!proto.file.ReadFileRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.ReadFileResponse.deserializeBinary
);


/**
 * @param {!proto.file.ReadFileRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.file.ReadFileResponse>}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.readFile =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/file.FileService/ReadFile',
      request,
      metadata || {},
      methodDescriptor_FileService_ReadFile);
};


/**
 * @param {!proto.file.ReadFileRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.file.ReadFileResponse>}
 *     The XHR Node Readable Stream
 */
proto.file.FileServicePromiseClient.prototype.readFile =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/file.FileService/ReadFile',
      request,
      metadata || {},
      methodDescriptor_FileService_ReadFile);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.DeleteFileRequest,
 *   !proto.file.DeleteFileResponse>}
 */
const methodDescriptor_FileService_DeleteFile = new grpc.web.MethodDescriptor(
  '/file.FileService/DeleteFile',
  grpc.web.MethodType.UNARY,
  proto.file.DeleteFileRequest,
  proto.file.DeleteFileResponse,
  /**
   * @param {!proto.file.DeleteFileRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.DeleteFileResponse.deserializeBinary
);


/**
 * @param {!proto.file.DeleteFileRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.DeleteFileResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.DeleteFileResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.deleteFile =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/DeleteFile',
      request,
      metadata || {},
      methodDescriptor_FileService_DeleteFile,
      callback);
};


/**
 * @param {!proto.file.DeleteFileRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.DeleteFileResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.deleteFile =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/DeleteFile',
      request,
      metadata || {},
      methodDescriptor_FileService_DeleteFile);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.GetThumbnailsRequest,
 *   !proto.file.GetThumbnailsResponse>}
 */
const methodDescriptor_FileService_GetThumbnails = new grpc.web.MethodDescriptor(
  '/file.FileService/GetThumbnails',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.file.GetThumbnailsRequest,
  proto.file.GetThumbnailsResponse,
  /**
   * @param {!proto.file.GetThumbnailsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.GetThumbnailsResponse.deserializeBinary
);


/**
 * @param {!proto.file.GetThumbnailsRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.file.GetThumbnailsResponse>}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.getThumbnails =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/file.FileService/GetThumbnails',
      request,
      metadata || {},
      methodDescriptor_FileService_GetThumbnails);
};


/**
 * @param {!proto.file.GetThumbnailsRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.file.GetThumbnailsResponse>}
 *     The XHR Node Readable Stream
 */
proto.file.FileServicePromiseClient.prototype.getThumbnails =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/file.FileService/GetThumbnails',
      request,
      metadata || {},
      methodDescriptor_FileService_GetThumbnails);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.CreateVideoPreviewRequest,
 *   !proto.file.CreateVideoPreviewResponse>}
 */
const methodDescriptor_FileService_CreateVideoPreview = new grpc.web.MethodDescriptor(
  '/file.FileService/CreateVideoPreview',
  grpc.web.MethodType.UNARY,
  proto.file.CreateVideoPreviewRequest,
  proto.file.CreateVideoPreviewResponse,
  /**
   * @param {!proto.file.CreateVideoPreviewRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.CreateVideoPreviewResponse.deserializeBinary
);


/**
 * @param {!proto.file.CreateVideoPreviewRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.CreateVideoPreviewResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.CreateVideoPreviewResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.createVideoPreview =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/CreateVideoPreview',
      request,
      metadata || {},
      methodDescriptor_FileService_CreateVideoPreview,
      callback);
};


/**
 * @param {!proto.file.CreateVideoPreviewRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.CreateVideoPreviewResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.createVideoPreview =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/CreateVideoPreview',
      request,
      metadata || {},
      methodDescriptor_FileService_CreateVideoPreview);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.CreateVideoTimeLineRequest,
 *   !proto.file.CreateVideoTimeLineResponse>}
 */
const methodDescriptor_FileService_CreateVideoTimeLine = new grpc.web.MethodDescriptor(
  '/file.FileService/CreateVideoTimeLine',
  grpc.web.MethodType.UNARY,
  proto.file.CreateVideoTimeLineRequest,
  proto.file.CreateVideoTimeLineResponse,
  /**
   * @param {!proto.file.CreateVideoTimeLineRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.CreateVideoTimeLineResponse.deserializeBinary
);


/**
 * @param {!proto.file.CreateVideoTimeLineRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.CreateVideoTimeLineResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.CreateVideoTimeLineResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.createVideoTimeLine =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/CreateVideoTimeLine',
      request,
      metadata || {},
      methodDescriptor_FileService_CreateVideoTimeLine,
      callback);
};


/**
 * @param {!proto.file.CreateVideoTimeLineRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.CreateVideoTimeLineResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.createVideoTimeLine =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/CreateVideoTimeLine',
      request,
      metadata || {},
      methodDescriptor_FileService_CreateVideoTimeLine);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.ConvertVideoToMpeg4H264Request,
 *   !proto.file.ConvertVideoToMpeg4H264Response>}
 */
const methodDescriptor_FileService_ConvertVideoToMpeg4H264 = new grpc.web.MethodDescriptor(
  '/file.FileService/ConvertVideoToMpeg4H264',
  grpc.web.MethodType.UNARY,
  proto.file.ConvertVideoToMpeg4H264Request,
  proto.file.ConvertVideoToMpeg4H264Response,
  /**
   * @param {!proto.file.ConvertVideoToMpeg4H264Request} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.ConvertVideoToMpeg4H264Response.deserializeBinary
);


/**
 * @param {!proto.file.ConvertVideoToMpeg4H264Request} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.ConvertVideoToMpeg4H264Response)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.ConvertVideoToMpeg4H264Response>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.convertVideoToMpeg4H264 =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/ConvertVideoToMpeg4H264',
      request,
      metadata || {},
      methodDescriptor_FileService_ConvertVideoToMpeg4H264,
      callback);
};


/**
 * @param {!proto.file.ConvertVideoToMpeg4H264Request} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.ConvertVideoToMpeg4H264Response>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.convertVideoToMpeg4H264 =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/ConvertVideoToMpeg4H264',
      request,
      metadata || {},
      methodDescriptor_FileService_ConvertVideoToMpeg4H264);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.ConvertVideoToHlsRequest,
 *   !proto.file.ConvertVideoToHlsResponse>}
 */
const methodDescriptor_FileService_ConvertVideoToHls = new grpc.web.MethodDescriptor(
  '/file.FileService/ConvertVideoToHls',
  grpc.web.MethodType.UNARY,
  proto.file.ConvertVideoToHlsRequest,
  proto.file.ConvertVideoToHlsResponse,
  /**
   * @param {!proto.file.ConvertVideoToHlsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.ConvertVideoToHlsResponse.deserializeBinary
);


/**
 * @param {!proto.file.ConvertVideoToHlsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.ConvertVideoToHlsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.ConvertVideoToHlsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.convertVideoToHls =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/ConvertVideoToHls',
      request,
      metadata || {},
      methodDescriptor_FileService_ConvertVideoToHls,
      callback);
};


/**
 * @param {!proto.file.ConvertVideoToHlsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.ConvertVideoToHlsResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.convertVideoToHls =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/ConvertVideoToHls',
      request,
      metadata || {},
      methodDescriptor_FileService_ConvertVideoToHls);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.StartProcessVideoRequest,
 *   !proto.file.StartProcessVideoResponse>}
 */
const methodDescriptor_FileService_StartProcessVideo = new grpc.web.MethodDescriptor(
  '/file.FileService/StartProcessVideo',
  grpc.web.MethodType.UNARY,
  proto.file.StartProcessVideoRequest,
  proto.file.StartProcessVideoResponse,
  /**
   * @param {!proto.file.StartProcessVideoRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.StartProcessVideoResponse.deserializeBinary
);


/**
 * @param {!proto.file.StartProcessVideoRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.StartProcessVideoResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.StartProcessVideoResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.startProcessVideo =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/StartProcessVideo',
      request,
      metadata || {},
      methodDescriptor_FileService_StartProcessVideo,
      callback);
};


/**
 * @param {!proto.file.StartProcessVideoRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.StartProcessVideoResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.startProcessVideo =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/StartProcessVideo',
      request,
      metadata || {},
      methodDescriptor_FileService_StartProcessVideo);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.StopProcessVideoRequest,
 *   !proto.file.StopProcessVideoResponse>}
 */
const methodDescriptor_FileService_StopProcessVideo = new grpc.web.MethodDescriptor(
  '/file.FileService/StopProcessVideo',
  grpc.web.MethodType.UNARY,
  proto.file.StopProcessVideoRequest,
  proto.file.StopProcessVideoResponse,
  /**
   * @param {!proto.file.StopProcessVideoRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.StopProcessVideoResponse.deserializeBinary
);


/**
 * @param {!proto.file.StopProcessVideoRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.StopProcessVideoResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.StopProcessVideoResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.stopProcessVideo =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/StopProcessVideo',
      request,
      metadata || {},
      methodDescriptor_FileService_StopProcessVideo,
      callback);
};


/**
 * @param {!proto.file.StopProcessVideoRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.StopProcessVideoResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.stopProcessVideo =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/StopProcessVideo',
      request,
      metadata || {},
      methodDescriptor_FileService_StopProcessVideo);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.IsProcessVideoRequest,
 *   !proto.file.IsProcessVideoResponse>}
 */
const methodDescriptor_FileService_IsProcessVideo = new grpc.web.MethodDescriptor(
  '/file.FileService/IsProcessVideo',
  grpc.web.MethodType.UNARY,
  proto.file.IsProcessVideoRequest,
  proto.file.IsProcessVideoResponse,
  /**
   * @param {!proto.file.IsProcessVideoRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.IsProcessVideoResponse.deserializeBinary
);


/**
 * @param {!proto.file.IsProcessVideoRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.IsProcessVideoResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.IsProcessVideoResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.isProcessVideo =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/IsProcessVideo',
      request,
      metadata || {},
      methodDescriptor_FileService_IsProcessVideo,
      callback);
};


/**
 * @param {!proto.file.IsProcessVideoRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.IsProcessVideoResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.isProcessVideo =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/IsProcessVideo',
      request,
      metadata || {},
      methodDescriptor_FileService_IsProcessVideo);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.SetVideoConversionRequest,
 *   !proto.file.SetVideoConversionResponse>}
 */
const methodDescriptor_FileService_SetVideoConversion = new grpc.web.MethodDescriptor(
  '/file.FileService/SetVideoConversion',
  grpc.web.MethodType.UNARY,
  proto.file.SetVideoConversionRequest,
  proto.file.SetVideoConversionResponse,
  /**
   * @param {!proto.file.SetVideoConversionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.SetVideoConversionResponse.deserializeBinary
);


/**
 * @param {!proto.file.SetVideoConversionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.SetVideoConversionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.SetVideoConversionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.setVideoConversion =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/SetVideoConversion',
      request,
      metadata || {},
      methodDescriptor_FileService_SetVideoConversion,
      callback);
};


/**
 * @param {!proto.file.SetVideoConversionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.SetVideoConversionResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.setVideoConversion =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/SetVideoConversion',
      request,
      metadata || {},
      methodDescriptor_FileService_SetVideoConversion);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.SetVideoStreamConversionRequest,
 *   !proto.file.SetVideoStreamConversionResponse>}
 */
const methodDescriptor_FileService_SetVideoStreamConversion = new grpc.web.MethodDescriptor(
  '/file.FileService/SetVideoStreamConversion',
  grpc.web.MethodType.UNARY,
  proto.file.SetVideoStreamConversionRequest,
  proto.file.SetVideoStreamConversionResponse,
  /**
   * @param {!proto.file.SetVideoStreamConversionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.SetVideoStreamConversionResponse.deserializeBinary
);


/**
 * @param {!proto.file.SetVideoStreamConversionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.SetVideoStreamConversionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.SetVideoStreamConversionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.setVideoStreamConversion =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/SetVideoStreamConversion',
      request,
      metadata || {},
      methodDescriptor_FileService_SetVideoStreamConversion,
      callback);
};


/**
 * @param {!proto.file.SetVideoStreamConversionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.SetVideoStreamConversionResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.setVideoStreamConversion =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/SetVideoStreamConversion',
      request,
      metadata || {},
      methodDescriptor_FileService_SetVideoStreamConversion);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.SetStartVideoConversionHourRequest,
 *   !proto.file.SetStartVideoConversionHourResponse>}
 */
const methodDescriptor_FileService_SetStartVideoConversionHour = new grpc.web.MethodDescriptor(
  '/file.FileService/SetStartVideoConversionHour',
  grpc.web.MethodType.UNARY,
  proto.file.SetStartVideoConversionHourRequest,
  proto.file.SetStartVideoConversionHourResponse,
  /**
   * @param {!proto.file.SetStartVideoConversionHourRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.SetStartVideoConversionHourResponse.deserializeBinary
);


/**
 * @param {!proto.file.SetStartVideoConversionHourRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.SetStartVideoConversionHourResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.SetStartVideoConversionHourResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.setStartVideoConversionHour =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/SetStartVideoConversionHour',
      request,
      metadata || {},
      methodDescriptor_FileService_SetStartVideoConversionHour,
      callback);
};


/**
 * @param {!proto.file.SetStartVideoConversionHourRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.SetStartVideoConversionHourResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.setStartVideoConversionHour =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/SetStartVideoConversionHour',
      request,
      metadata || {},
      methodDescriptor_FileService_SetStartVideoConversionHour);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.SetMaximumVideoConversionDelayRequest,
 *   !proto.file.SetMaximumVideoConversionDelayResponse>}
 */
const methodDescriptor_FileService_SetMaximumVideoConversionDelay = new grpc.web.MethodDescriptor(
  '/file.FileService/SetMaximumVideoConversionDelay',
  grpc.web.MethodType.UNARY,
  proto.file.SetMaximumVideoConversionDelayRequest,
  proto.file.SetMaximumVideoConversionDelayResponse,
  /**
   * @param {!proto.file.SetMaximumVideoConversionDelayRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.SetMaximumVideoConversionDelayResponse.deserializeBinary
);


/**
 * @param {!proto.file.SetMaximumVideoConversionDelayRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.SetMaximumVideoConversionDelayResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.SetMaximumVideoConversionDelayResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.setMaximumVideoConversionDelay =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/SetMaximumVideoConversionDelay',
      request,
      metadata || {},
      methodDescriptor_FileService_SetMaximumVideoConversionDelay,
      callback);
};


/**
 * @param {!proto.file.SetMaximumVideoConversionDelayRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.SetMaximumVideoConversionDelayResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.setMaximumVideoConversionDelay =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/SetMaximumVideoConversionDelay',
      request,
      metadata || {},
      methodDescriptor_FileService_SetMaximumVideoConversionDelay);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.GetVideoConversionErrorsRequest,
 *   !proto.file.GetVideoConversionErrorsResponse>}
 */
const methodDescriptor_FileService_GetVideoConversionErrors = new grpc.web.MethodDescriptor(
  '/file.FileService/GetVideoConversionErrors',
  grpc.web.MethodType.UNARY,
  proto.file.GetVideoConversionErrorsRequest,
  proto.file.GetVideoConversionErrorsResponse,
  /**
   * @param {!proto.file.GetVideoConversionErrorsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.GetVideoConversionErrorsResponse.deserializeBinary
);


/**
 * @param {!proto.file.GetVideoConversionErrorsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.GetVideoConversionErrorsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.GetVideoConversionErrorsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.getVideoConversionErrors =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/GetVideoConversionErrors',
      request,
      metadata || {},
      methodDescriptor_FileService_GetVideoConversionErrors,
      callback);
};


/**
 * @param {!proto.file.GetVideoConversionErrorsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.GetVideoConversionErrorsResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.getVideoConversionErrors =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/GetVideoConversionErrors',
      request,
      metadata || {},
      methodDescriptor_FileService_GetVideoConversionErrors);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.ClearVideoConversionErrorsRequest,
 *   !proto.file.ClearVideoConversionErrorsResponse>}
 */
const methodDescriptor_FileService_ClearVideoConversionErrors = new grpc.web.MethodDescriptor(
  '/file.FileService/ClearVideoConversionErrors',
  grpc.web.MethodType.UNARY,
  proto.file.ClearVideoConversionErrorsRequest,
  proto.file.ClearVideoConversionErrorsResponse,
  /**
   * @param {!proto.file.ClearVideoConversionErrorsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.ClearVideoConversionErrorsResponse.deserializeBinary
);


/**
 * @param {!proto.file.ClearVideoConversionErrorsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.ClearVideoConversionErrorsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.ClearVideoConversionErrorsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.clearVideoConversionErrors =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/ClearVideoConversionErrors',
      request,
      metadata || {},
      methodDescriptor_FileService_ClearVideoConversionErrors,
      callback);
};


/**
 * @param {!proto.file.ClearVideoConversionErrorsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.ClearVideoConversionErrorsResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.clearVideoConversionErrors =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/ClearVideoConversionErrors',
      request,
      metadata || {},
      methodDescriptor_FileService_ClearVideoConversionErrors);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.ClearVideoConversionErrorRequest,
 *   !proto.file.ClearVideoConversionErrorResponse>}
 */
const methodDescriptor_FileService_ClearVideoConversionError = new grpc.web.MethodDescriptor(
  '/file.FileService/ClearVideoConversionError',
  grpc.web.MethodType.UNARY,
  proto.file.ClearVideoConversionErrorRequest,
  proto.file.ClearVideoConversionErrorResponse,
  /**
   * @param {!proto.file.ClearVideoConversionErrorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.ClearVideoConversionErrorResponse.deserializeBinary
);


/**
 * @param {!proto.file.ClearVideoConversionErrorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.ClearVideoConversionErrorResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.ClearVideoConversionErrorResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.clearVideoConversionError =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/ClearVideoConversionError',
      request,
      metadata || {},
      methodDescriptor_FileService_ClearVideoConversionError,
      callback);
};


/**
 * @param {!proto.file.ClearVideoConversionErrorRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.ClearVideoConversionErrorResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.clearVideoConversionError =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/ClearVideoConversionError',
      request,
      metadata || {},
      methodDescriptor_FileService_ClearVideoConversionError);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.WriteExcelFileRequest,
 *   !proto.file.WriteExcelFileResponse>}
 */
const methodDescriptor_FileService_WriteExcelFile = new grpc.web.MethodDescriptor(
  '/file.FileService/WriteExcelFile',
  grpc.web.MethodType.UNARY,
  proto.file.WriteExcelFileRequest,
  proto.file.WriteExcelFileResponse,
  /**
   * @param {!proto.file.WriteExcelFileRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.WriteExcelFileResponse.deserializeBinary
);


/**
 * @param {!proto.file.WriteExcelFileRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.WriteExcelFileResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.WriteExcelFileResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.writeExcelFile =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/WriteExcelFile',
      request,
      metadata || {},
      methodDescriptor_FileService_WriteExcelFile,
      callback);
};


/**
 * @param {!proto.file.WriteExcelFileRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.WriteExcelFileResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.writeExcelFile =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/WriteExcelFile',
      request,
      metadata || {},
      methodDescriptor_FileService_WriteExcelFile);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.HtmlToPdfRqst,
 *   !proto.file.HtmlToPdfResponse>}
 */
const methodDescriptor_FileService_HtmlToPdf = new grpc.web.MethodDescriptor(
  '/file.FileService/HtmlToPdf',
  grpc.web.MethodType.UNARY,
  proto.file.HtmlToPdfRqst,
  proto.file.HtmlToPdfResponse,
  /**
   * @param {!proto.file.HtmlToPdfRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.HtmlToPdfResponse.deserializeBinary
);


/**
 * @param {!proto.file.HtmlToPdfRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.HtmlToPdfResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.HtmlToPdfResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.htmlToPdf =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/HtmlToPdf',
      request,
      metadata || {},
      methodDescriptor_FileService_HtmlToPdf,
      callback);
};


/**
 * @param {!proto.file.HtmlToPdfRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.HtmlToPdfResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.htmlToPdf =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/HtmlToPdf',
      request,
      metadata || {},
      methodDescriptor_FileService_HtmlToPdf);
};


module.exports = proto.file;

