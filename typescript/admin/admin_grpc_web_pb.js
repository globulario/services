/**
 * @fileoverview gRPC-Web generated client stub for admin
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.admin = require('./admin_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.admin.AdminServiceClient =
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
proto.admin.AdminServicePromiseClient =
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
 *   !proto.admin.DownloadGlobularRequest,
 *   !proto.admin.DownloadGlobularResponse>}
 */
const methodDescriptor_AdminService_DownloadGlobular = new grpc.web.MethodDescriptor(
  '/admin.AdminService/DownloadGlobular',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.admin.DownloadGlobularRequest,
  proto.admin.DownloadGlobularResponse,
  /**
   * @param {!proto.admin.DownloadGlobularRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.admin.DownloadGlobularResponse.deserializeBinary
);


/**
 * @param {!proto.admin.DownloadGlobularRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.admin.DownloadGlobularResponse>}
 *     The XHR Node Readable Stream
 */
proto.admin.AdminServiceClient.prototype.downloadGlobular =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/admin.AdminService/DownloadGlobular',
      request,
      metadata || {},
      methodDescriptor_AdminService_DownloadGlobular);
};


/**
 * @param {!proto.admin.DownloadGlobularRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.admin.DownloadGlobularResponse>}
 *     The XHR Node Readable Stream
 */
proto.admin.AdminServicePromiseClient.prototype.downloadGlobular =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/admin.AdminService/DownloadGlobular',
      request,
      metadata || {},
      methodDescriptor_AdminService_DownloadGlobular);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.admin.GetCertificatesRequest,
 *   !proto.admin.GetCertificatesResponse>}
 */
const methodDescriptor_AdminService_GetCertificates = new grpc.web.MethodDescriptor(
  '/admin.AdminService/GetCertificates',
  grpc.web.MethodType.UNARY,
  proto.admin.GetCertificatesRequest,
  proto.admin.GetCertificatesResponse,
  /**
   * @param {!proto.admin.GetCertificatesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.admin.GetCertificatesResponse.deserializeBinary
);


/**
 * @param {!proto.admin.GetCertificatesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.admin.GetCertificatesResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.admin.GetCertificatesResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.admin.AdminServiceClient.prototype.getCertificates =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/admin.AdminService/GetCertificates',
      request,
      metadata || {},
      methodDescriptor_AdminService_GetCertificates,
      callback);
};


/**
 * @param {!proto.admin.GetCertificatesRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.admin.GetCertificatesResponse>}
 *     Promise that resolves to the response
 */
proto.admin.AdminServicePromiseClient.prototype.getCertificates =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/admin.AdminService/GetCertificates',
      request,
      metadata || {},
      methodDescriptor_AdminService_GetCertificates);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.admin.HasRunningProcessRequest,
 *   !proto.admin.HasRunningProcessResponse>}
 */
const methodDescriptor_AdminService_HasRunningProcess = new grpc.web.MethodDescriptor(
  '/admin.AdminService/HasRunningProcess',
  grpc.web.MethodType.UNARY,
  proto.admin.HasRunningProcessRequest,
  proto.admin.HasRunningProcessResponse,
  /**
   * @param {!proto.admin.HasRunningProcessRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.admin.HasRunningProcessResponse.deserializeBinary
);


/**
 * @param {!proto.admin.HasRunningProcessRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.admin.HasRunningProcessResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.admin.HasRunningProcessResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.admin.AdminServiceClient.prototype.hasRunningProcess =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/admin.AdminService/HasRunningProcess',
      request,
      metadata || {},
      methodDescriptor_AdminService_HasRunningProcess,
      callback);
};


/**
 * @param {!proto.admin.HasRunningProcessRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.admin.HasRunningProcessResponse>}
 *     Promise that resolves to the response
 */
proto.admin.AdminServicePromiseClient.prototype.hasRunningProcess =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/admin.AdminService/HasRunningProcess',
      request,
      metadata || {},
      methodDescriptor_AdminService_HasRunningProcess);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.admin.RunCmdRequest,
 *   !proto.admin.RunCmdResponse>}
 */
const methodDescriptor_AdminService_RunCmd = new grpc.web.MethodDescriptor(
  '/admin.AdminService/RunCmd',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.admin.RunCmdRequest,
  proto.admin.RunCmdResponse,
  /**
   * @param {!proto.admin.RunCmdRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.admin.RunCmdResponse.deserializeBinary
);


/**
 * @param {!proto.admin.RunCmdRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.admin.RunCmdResponse>}
 *     The XHR Node Readable Stream
 */
proto.admin.AdminServiceClient.prototype.runCmd =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/admin.AdminService/RunCmd',
      request,
      metadata || {},
      methodDescriptor_AdminService_RunCmd);
};


/**
 * @param {!proto.admin.RunCmdRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.admin.RunCmdResponse>}
 *     The XHR Node Readable Stream
 */
proto.admin.AdminServicePromiseClient.prototype.runCmd =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/admin.AdminService/RunCmd',
      request,
      metadata || {},
      methodDescriptor_AdminService_RunCmd);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.admin.SetEnvironmentVariableRequest,
 *   !proto.admin.SetEnvironmentVariableResponse>}
 */
const methodDescriptor_AdminService_SetEnvironmentVariable = new grpc.web.MethodDescriptor(
  '/admin.AdminService/SetEnvironmentVariable',
  grpc.web.MethodType.UNARY,
  proto.admin.SetEnvironmentVariableRequest,
  proto.admin.SetEnvironmentVariableResponse,
  /**
   * @param {!proto.admin.SetEnvironmentVariableRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.admin.SetEnvironmentVariableResponse.deserializeBinary
);


/**
 * @param {!proto.admin.SetEnvironmentVariableRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.admin.SetEnvironmentVariableResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.admin.SetEnvironmentVariableResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.admin.AdminServiceClient.prototype.setEnvironmentVariable =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/admin.AdminService/SetEnvironmentVariable',
      request,
      metadata || {},
      methodDescriptor_AdminService_SetEnvironmentVariable,
      callback);
};


/**
 * @param {!proto.admin.SetEnvironmentVariableRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.admin.SetEnvironmentVariableResponse>}
 *     Promise that resolves to the response
 */
proto.admin.AdminServicePromiseClient.prototype.setEnvironmentVariable =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/admin.AdminService/SetEnvironmentVariable',
      request,
      metadata || {},
      methodDescriptor_AdminService_SetEnvironmentVariable);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.admin.GetEnvironmentVariableRequest,
 *   !proto.admin.GetEnvironmentVariableResponse>}
 */
const methodDescriptor_AdminService_GetEnvironmentVariable = new grpc.web.MethodDescriptor(
  '/admin.AdminService/GetEnvironmentVariable',
  grpc.web.MethodType.UNARY,
  proto.admin.GetEnvironmentVariableRequest,
  proto.admin.GetEnvironmentVariableResponse,
  /**
   * @param {!proto.admin.GetEnvironmentVariableRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.admin.GetEnvironmentVariableResponse.deserializeBinary
);


/**
 * @param {!proto.admin.GetEnvironmentVariableRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.admin.GetEnvironmentVariableResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.admin.GetEnvironmentVariableResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.admin.AdminServiceClient.prototype.getEnvironmentVariable =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/admin.AdminService/GetEnvironmentVariable',
      request,
      metadata || {},
      methodDescriptor_AdminService_GetEnvironmentVariable,
      callback);
};


/**
 * @param {!proto.admin.GetEnvironmentVariableRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.admin.GetEnvironmentVariableResponse>}
 *     Promise that resolves to the response
 */
proto.admin.AdminServicePromiseClient.prototype.getEnvironmentVariable =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/admin.AdminService/GetEnvironmentVariable',
      request,
      metadata || {},
      methodDescriptor_AdminService_GetEnvironmentVariable);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.admin.UnsetEnvironmentVariableRequest,
 *   !proto.admin.UnsetEnvironmentVariableResponse>}
 */
const methodDescriptor_AdminService_UnsetEnvironmentVariable = new grpc.web.MethodDescriptor(
  '/admin.AdminService/UnsetEnvironmentVariable',
  grpc.web.MethodType.UNARY,
  proto.admin.UnsetEnvironmentVariableRequest,
  proto.admin.UnsetEnvironmentVariableResponse,
  /**
   * @param {!proto.admin.UnsetEnvironmentVariableRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.admin.UnsetEnvironmentVariableResponse.deserializeBinary
);


/**
 * @param {!proto.admin.UnsetEnvironmentVariableRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.admin.UnsetEnvironmentVariableResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.admin.UnsetEnvironmentVariableResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.admin.AdminServiceClient.prototype.unsetEnvironmentVariable =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/admin.AdminService/UnsetEnvironmentVariable',
      request,
      metadata || {},
      methodDescriptor_AdminService_UnsetEnvironmentVariable,
      callback);
};


/**
 * @param {!proto.admin.UnsetEnvironmentVariableRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.admin.UnsetEnvironmentVariableResponse>}
 *     Promise that resolves to the response
 */
proto.admin.AdminServicePromiseClient.prototype.unsetEnvironmentVariable =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/admin.AdminService/UnsetEnvironmentVariable',
      request,
      metadata || {},
      methodDescriptor_AdminService_UnsetEnvironmentVariable);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.admin.KillProcessRequest,
 *   !proto.admin.KillProcessResponse>}
 */
const methodDescriptor_AdminService_KillProcess = new grpc.web.MethodDescriptor(
  '/admin.AdminService/KillProcess',
  grpc.web.MethodType.UNARY,
  proto.admin.KillProcessRequest,
  proto.admin.KillProcessResponse,
  /**
   * @param {!proto.admin.KillProcessRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.admin.KillProcessResponse.deserializeBinary
);


/**
 * @param {!proto.admin.KillProcessRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.admin.KillProcessResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.admin.KillProcessResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.admin.AdminServiceClient.prototype.killProcess =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/admin.AdminService/KillProcess',
      request,
      metadata || {},
      methodDescriptor_AdminService_KillProcess,
      callback);
};


/**
 * @param {!proto.admin.KillProcessRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.admin.KillProcessResponse>}
 *     Promise that resolves to the response
 */
proto.admin.AdminServicePromiseClient.prototype.killProcess =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/admin.AdminService/KillProcess',
      request,
      metadata || {},
      methodDescriptor_AdminService_KillProcess);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.admin.KillProcessesRequest,
 *   !proto.admin.KillProcessesResponse>}
 */
const methodDescriptor_AdminService_KillProcesses = new grpc.web.MethodDescriptor(
  '/admin.AdminService/KillProcesses',
  grpc.web.MethodType.UNARY,
  proto.admin.KillProcessesRequest,
  proto.admin.KillProcessesResponse,
  /**
   * @param {!proto.admin.KillProcessesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.admin.KillProcessesResponse.deserializeBinary
);


/**
 * @param {!proto.admin.KillProcessesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.admin.KillProcessesResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.admin.KillProcessesResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.admin.AdminServiceClient.prototype.killProcesses =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/admin.AdminService/KillProcesses',
      request,
      metadata || {},
      methodDescriptor_AdminService_KillProcesses,
      callback);
};


/**
 * @param {!proto.admin.KillProcessesRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.admin.KillProcessesResponse>}
 *     Promise that resolves to the response
 */
proto.admin.AdminServicePromiseClient.prototype.killProcesses =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/admin.AdminService/KillProcesses',
      request,
      metadata || {},
      methodDescriptor_AdminService_KillProcesses);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.admin.GetPidsRequest,
 *   !proto.admin.GetPidsResponse>}
 */
const methodDescriptor_AdminService_GetPids = new grpc.web.MethodDescriptor(
  '/admin.AdminService/GetPids',
  grpc.web.MethodType.UNARY,
  proto.admin.GetPidsRequest,
  proto.admin.GetPidsResponse,
  /**
   * @param {!proto.admin.GetPidsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.admin.GetPidsResponse.deserializeBinary
);


/**
 * @param {!proto.admin.GetPidsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.admin.GetPidsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.admin.GetPidsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.admin.AdminServiceClient.prototype.getPids =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/admin.AdminService/GetPids',
      request,
      metadata || {},
      methodDescriptor_AdminService_GetPids,
      callback);
};


/**
 * @param {!proto.admin.GetPidsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.admin.GetPidsResponse>}
 *     Promise that resolves to the response
 */
proto.admin.AdminServicePromiseClient.prototype.getPids =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/admin.AdminService/GetPids',
      request,
      metadata || {},
      methodDescriptor_AdminService_GetPids);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.admin.SaveConfigRequest,
 *   !proto.admin.SaveConfigRequest>}
 */
const methodDescriptor_AdminService_SaveConfig = new grpc.web.MethodDescriptor(
  '/admin.AdminService/SaveConfig',
  grpc.web.MethodType.UNARY,
  proto.admin.SaveConfigRequest,
  proto.admin.SaveConfigRequest,
  /**
   * @param {!proto.admin.SaveConfigRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.admin.SaveConfigRequest.deserializeBinary
);


/**
 * @param {!proto.admin.SaveConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.admin.SaveConfigRequest)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.admin.SaveConfigRequest>|undefined}
 *     The XHR Node Readable Stream
 */
proto.admin.AdminServiceClient.prototype.saveConfig =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/admin.AdminService/SaveConfig',
      request,
      metadata || {},
      methodDescriptor_AdminService_SaveConfig,
      callback);
};


/**
 * @param {!proto.admin.SaveConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.admin.SaveConfigRequest>}
 *     Promise that resolves to the response
 */
proto.admin.AdminServicePromiseClient.prototype.saveConfig =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/admin.AdminService/SaveConfig',
      request,
      metadata || {},
      methodDescriptor_AdminService_SaveConfig);
};


module.exports = proto.admin;

