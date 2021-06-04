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
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.file.FileServiceClient =
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
proto.file.FileServicePromiseClient =
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.file.StopRequest,
 *   !proto.file.StopResponse>}
 */
const methodInfo_FileService_Stop = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.file.StopResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.file.ReadDirRequest,
 *   !proto.file.ReadDirResponse>}
 */
const methodInfo_FileService_ReadDir = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {?Object<string, string>} metadata User defined
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.file.CreateDirRequest,
 *   !proto.file.CreateDirResponse>}
 */
const methodInfo_FileService_CreateDir = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.file.CreateDirResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.file.DeleteDirRequest,
 *   !proto.file.DeleteDirResponse>}
 */
const methodInfo_FileService_DeleteDir = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.file.DeleteDirResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.file.RenameRequest,
 *   !proto.file.RenameResponse>}
 */
const methodInfo_FileService_Rename = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.file.RenameResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.file.MoveRequest,
 *   !proto.file.MoveResponse>}
 */
const methodInfo_FileService_Move = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.file.MoveResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.file.CopyRequest,
 *   !proto.file.CopyResponse>}
 */
const methodInfo_FileService_Copy = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.file.CopyResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.file.CreateArchiveRequest,
 *   !proto.file.CreateArchiveResponse>}
 */
const methodInfo_FileService_CreateAchive = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.file.CreateArchiveResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.file.GetFileInfoRequest,
 *   !proto.file.GetFileInfoResponse>}
 */
const methodInfo_FileService_GetFileInfo = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.file.GetFileInfoResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.file.ReadFileRequest,
 *   !proto.file.ReadFileResponse>}
 */
const methodInfo_FileService_ReadFile = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {?Object<string, string>} metadata User defined
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.file.DeleteFileRequest,
 *   !proto.file.DeleteFileResponse>}
 */
const methodInfo_FileService_DeleteFile = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.file.DeleteFileResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.file.GetThumbnailsRequest,
 *   !proto.file.GetThumbnailsResponse>}
 */
const methodInfo_FileService_GetThumbnails = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {?Object<string, string>} metadata User defined
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.file.WriteExcelFileRequest,
 *   !proto.file.WriteExcelFileResponse>}
 */
const methodInfo_FileService_WriteExcelFile = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.file.WriteExcelFileResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.file.HtmlToPdfRqst,
 *   !proto.file.HtmlToPdfResponse>}
 */
const methodInfo_FileService_HtmlToPdf = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.file.HtmlToPdfResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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

