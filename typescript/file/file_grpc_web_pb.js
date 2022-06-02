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
 *   !proto.file.ProcessVideoRequest,
 *   !proto.file.ProcessVideoResponse>}
 */
const methodDescriptor_FileService_ProcessVideo = new grpc.web.MethodDescriptor(
  '/file.FileService/ProcessVideo',
  grpc.web.MethodType.UNARY,
  proto.file.ProcessVideoRequest,
  proto.file.ProcessVideoResponse,
  /**
   * @param {!proto.file.ProcessVideoRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.ProcessVideoResponse.deserializeBinary
);


/**
 * @param {!proto.file.ProcessVideoRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.ProcessVideoResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.ProcessVideoResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.processVideo =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/ProcessVideo',
      request,
      metadata || {},
      methodDescriptor_FileService_ProcessVideo,
      callback);
};


/**
 * @param {!proto.file.ProcessVideoRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.ProcessVideoResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.processVideo =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/ProcessVideo',
      request,
      metadata || {},
      methodDescriptor_FileService_ProcessVideo);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.SetVideoConvertionRequest,
 *   !proto.file.SetVideoConvertionResponse>}
 */
const methodDescriptor_FileService_SetVideoConvertion = new grpc.web.MethodDescriptor(
  '/file.FileService/SetVideoConvertion',
  grpc.web.MethodType.UNARY,
  proto.file.SetVideoConvertionRequest,
  proto.file.SetVideoConvertionResponse,
  /**
   * @param {!proto.file.SetVideoConvertionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.SetVideoConvertionResponse.deserializeBinary
);


/**
 * @param {!proto.file.SetVideoConvertionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.SetVideoConvertionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.SetVideoConvertionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.setVideoConvertion =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/SetVideoConvertion',
      request,
      metadata || {},
      methodDescriptor_FileService_SetVideoConvertion,
      callback);
};


/**
 * @param {!proto.file.SetVideoConvertionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.SetVideoConvertionResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.setVideoConvertion =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/SetVideoConvertion',
      request,
      metadata || {},
      methodDescriptor_FileService_SetVideoConvertion);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.SetVideoStreamConvertionRequest,
 *   !proto.file.SetVideoStreamConvertionResponse>}
 */
const methodDescriptor_FileService_SetVideoStreamConvertion = new grpc.web.MethodDescriptor(
  '/file.FileService/SetVideoStreamConvertion',
  grpc.web.MethodType.UNARY,
  proto.file.SetVideoStreamConvertionRequest,
  proto.file.SetVideoStreamConvertionResponse,
  /**
   * @param {!proto.file.SetVideoStreamConvertionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.SetVideoStreamConvertionResponse.deserializeBinary
);


/**
 * @param {!proto.file.SetVideoStreamConvertionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.SetVideoStreamConvertionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.SetVideoStreamConvertionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.setVideoStreamConvertion =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/SetVideoStreamConvertion',
      request,
      metadata || {},
      methodDescriptor_FileService_SetVideoStreamConvertion,
      callback);
};


/**
 * @param {!proto.file.SetVideoStreamConvertionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.SetVideoStreamConvertionResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.setVideoStreamConvertion =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/SetVideoStreamConvertion',
      request,
      metadata || {},
      methodDescriptor_FileService_SetVideoStreamConvertion);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.SetStartVideoConvertionHourRequest,
 *   !proto.file.SetStartVideoConvertionHourResponse>}
 */
const methodDescriptor_FileService_SetStartVideoConvertionHour = new grpc.web.MethodDescriptor(
  '/file.FileService/SetStartVideoConvertionHour',
  grpc.web.MethodType.UNARY,
  proto.file.SetStartVideoConvertionHourRequest,
  proto.file.SetStartVideoConvertionHourResponse,
  /**
   * @param {!proto.file.SetStartVideoConvertionHourRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.SetStartVideoConvertionHourResponse.deserializeBinary
);


/**
 * @param {!proto.file.SetStartVideoConvertionHourRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.SetStartVideoConvertionHourResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.SetStartVideoConvertionHourResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.setStartVideoConvertionHour =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/SetStartVideoConvertionHour',
      request,
      metadata || {},
      methodDescriptor_FileService_SetStartVideoConvertionHour,
      callback);
};


/**
 * @param {!proto.file.SetStartVideoConvertionHourRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.SetStartVideoConvertionHourResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.setStartVideoConvertionHour =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/SetStartVideoConvertionHour',
      request,
      metadata || {},
      methodDescriptor_FileService_SetStartVideoConvertionHour);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.SetMaximumVideoConvertionDelayRequest,
 *   !proto.file.SetMaximumVideoConvertionDelayResponse>}
 */
const methodDescriptor_FileService_SetMaximumVideoConvertionDelay = new grpc.web.MethodDescriptor(
  '/file.FileService/SetMaximumVideoConvertionDelay',
  grpc.web.MethodType.UNARY,
  proto.file.SetMaximumVideoConvertionDelayRequest,
  proto.file.SetMaximumVideoConvertionDelayResponse,
  /**
   * @param {!proto.file.SetMaximumVideoConvertionDelayRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.SetMaximumVideoConvertionDelayResponse.deserializeBinary
);


/**
 * @param {!proto.file.SetMaximumVideoConvertionDelayRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.SetMaximumVideoConvertionDelayResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.SetMaximumVideoConvertionDelayResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.setMaximumVideoConvertionDelay =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/SetMaximumVideoConvertionDelay',
      request,
      metadata || {},
      methodDescriptor_FileService_SetMaximumVideoConvertionDelay,
      callback);
};


/**
 * @param {!proto.file.SetMaximumVideoConvertionDelayRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.SetMaximumVideoConvertionDelayResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.setMaximumVideoConvertionDelay =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/SetMaximumVideoConvertionDelay',
      request,
      metadata || {},
      methodDescriptor_FileService_SetMaximumVideoConvertionDelay);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.GetVideoConvertionErrorsRequest,
 *   !proto.file.GetVideoConvertionErrorsResponse>}
 */
const methodDescriptor_FileService_GetVideoConvertionErrors = new grpc.web.MethodDescriptor(
  '/file.FileService/GetVideoConvertionErrors',
  grpc.web.MethodType.UNARY,
  proto.file.GetVideoConvertionErrorsRequest,
  proto.file.GetVideoConvertionErrorsResponse,
  /**
   * @param {!proto.file.GetVideoConvertionErrorsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.GetVideoConvertionErrorsResponse.deserializeBinary
);


/**
 * @param {!proto.file.GetVideoConvertionErrorsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.GetVideoConvertionErrorsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.GetVideoConvertionErrorsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.getVideoConvertionErrors =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/GetVideoConvertionErrors',
      request,
      metadata || {},
      methodDescriptor_FileService_GetVideoConvertionErrors,
      callback);
};


/**
 * @param {!proto.file.GetVideoConvertionErrorsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.GetVideoConvertionErrorsResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.getVideoConvertionErrors =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/GetVideoConvertionErrors',
      request,
      metadata || {},
      methodDescriptor_FileService_GetVideoConvertionErrors);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.ClearVideoConvertionErrorsRequest,
 *   !proto.file.ClearVideoConvertionErrorsResponse>}
 */
const methodDescriptor_FileService_ClearVideoConvertionErrors = new grpc.web.MethodDescriptor(
  '/file.FileService/ClearVideoConvertionErrors',
  grpc.web.MethodType.UNARY,
  proto.file.ClearVideoConvertionErrorsRequest,
  proto.file.ClearVideoConvertionErrorsResponse,
  /**
   * @param {!proto.file.ClearVideoConvertionErrorsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.ClearVideoConvertionErrorsResponse.deserializeBinary
);


/**
 * @param {!proto.file.ClearVideoConvertionErrorsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.ClearVideoConvertionErrorsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.ClearVideoConvertionErrorsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.clearVideoConvertionErrors =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/ClearVideoConvertionErrors',
      request,
      metadata || {},
      methodDescriptor_FileService_ClearVideoConvertionErrors,
      callback);
};


/**
 * @param {!proto.file.ClearVideoConvertionErrorsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.ClearVideoConvertionErrorsResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.clearVideoConvertionErrors =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/ClearVideoConvertionErrors',
      request,
      metadata || {},
      methodDescriptor_FileService_ClearVideoConvertionErrors);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.file.ClearVideoConvertionErrorRequest,
 *   !proto.file.ClearVideoConvertionErrorResponse>}
 */
const methodDescriptor_FileService_ClearVideoConvertionError = new grpc.web.MethodDescriptor(
  '/file.FileService/ClearVideoConvertionError',
  grpc.web.MethodType.UNARY,
  proto.file.ClearVideoConvertionErrorRequest,
  proto.file.ClearVideoConvertionErrorResponse,
  /**
   * @param {!proto.file.ClearVideoConvertionErrorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.file.ClearVideoConvertionErrorResponse.deserializeBinary
);


/**
 * @param {!proto.file.ClearVideoConvertionErrorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.file.ClearVideoConvertionErrorResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.file.ClearVideoConvertionErrorResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.file.FileServiceClient.prototype.clearVideoConvertionError =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/file.FileService/ClearVideoConvertionError',
      request,
      metadata || {},
      methodDescriptor_FileService_ClearVideoConvertionError,
      callback);
};


/**
 * @param {!proto.file.ClearVideoConvertionErrorRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.file.ClearVideoConvertionErrorResponse>}
 *     Promise that resolves to the response
 */
proto.file.FileServicePromiseClient.prototype.clearVideoConvertionError =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/file.FileService/ClearVideoConvertionError',
      request,
      metadata || {},
      methodDescriptor_FileService_ClearVideoConvertionError);
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

