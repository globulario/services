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
 *   !proto.title.CreatePublisherRequest,
 *   !proto.title.CreatePublisherResponse>}
 */
const methodDescriptor_TitleService_CreatePublisher = new grpc.web.MethodDescriptor(
  '/title.TitleService/CreatePublisher',
  grpc.web.MethodType.UNARY,
  proto.title.CreatePublisherRequest,
  proto.title.CreatePublisherResponse,
  /**
   * @param {!proto.title.CreatePublisherRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.title.CreatePublisherResponse.deserializeBinary
);


/**
 * @param {!proto.title.CreatePublisherRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.title.CreatePublisherResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.title.CreatePublisherResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.title.TitleServiceClient.prototype.createPublisher =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/title.TitleService/CreatePublisher',
      request,
      metadata || {},
      methodDescriptor_TitleService_CreatePublisher,
      callback);
};


/**
 * @param {!proto.title.CreatePublisherRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.title.CreatePublisherResponse>}
 *     Promise that resolves to the response
 */
proto.title.TitleServicePromiseClient.prototype.createPublisher =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/title.TitleService/CreatePublisher',
      request,
      metadata || {},
      methodDescriptor_TitleService_CreatePublisher);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.title.DeletePublisherRequest,
 *   !proto.title.DeletePublisherResponse>}
 */
const methodDescriptor_TitleService_DeletePublisher = new grpc.web.MethodDescriptor(
  '/title.TitleService/DeletePublisher',
  grpc.web.MethodType.UNARY,
  proto.title.DeletePublisherRequest,
  proto.title.DeletePublisherResponse,
  /**
   * @param {!proto.title.DeletePublisherRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.title.DeletePublisherResponse.deserializeBinary
);


/**
 * @param {!proto.title.DeletePublisherRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.title.DeletePublisherResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.title.DeletePublisherResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.title.TitleServiceClient.prototype.deletePublisher =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/title.TitleService/DeletePublisher',
      request,
      metadata || {},
      methodDescriptor_TitleService_DeletePublisher,
      callback);
};


/**
 * @param {!proto.title.DeletePublisherRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.title.DeletePublisherResponse>}
 *     Promise that resolves to the response
 */
proto.title.TitleServicePromiseClient.prototype.deletePublisher =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/title.TitleService/DeletePublisher',
      request,
      metadata || {},
      methodDescriptor_TitleService_DeletePublisher);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.title.GetPublisherByIdRequest,
 *   !proto.title.GetPublisherByIdResponse>}
 */
const methodDescriptor_TitleService_GetPublisherById = new grpc.web.MethodDescriptor(
  '/title.TitleService/GetPublisherById',
  grpc.web.MethodType.UNARY,
  proto.title.GetPublisherByIdRequest,
  proto.title.GetPublisherByIdResponse,
  /**
   * @param {!proto.title.GetPublisherByIdRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.title.GetPublisherByIdResponse.deserializeBinary
);


/**
 * @param {!proto.title.GetPublisherByIdRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.title.GetPublisherByIdResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.title.GetPublisherByIdResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.title.TitleServiceClient.prototype.getPublisherById =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/title.TitleService/GetPublisherById',
      request,
      metadata || {},
      methodDescriptor_TitleService_GetPublisherById,
      callback);
};


/**
 * @param {!proto.title.GetPublisherByIdRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.title.GetPublisherByIdResponse>}
 *     Promise that resolves to the response
 */
proto.title.TitleServicePromiseClient.prototype.getPublisherById =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/title.TitleService/GetPublisherById',
      request,
      metadata || {},
      methodDescriptor_TitleService_GetPublisherById);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.title.CreatePersonRequest,
 *   !proto.title.CreatePersonResponse>}
 */
const methodDescriptor_TitleService_CreatePerson = new grpc.web.MethodDescriptor(
  '/title.TitleService/CreatePerson',
  grpc.web.MethodType.UNARY,
  proto.title.CreatePersonRequest,
  proto.title.CreatePersonResponse,
  /**
   * @param {!proto.title.CreatePersonRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.title.CreatePersonResponse.deserializeBinary
);


/**
 * @param {!proto.title.CreatePersonRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.title.CreatePersonResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.title.CreatePersonResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.title.TitleServiceClient.prototype.createPerson =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/title.TitleService/CreatePerson',
      request,
      metadata || {},
      methodDescriptor_TitleService_CreatePerson,
      callback);
};


/**
 * @param {!proto.title.CreatePersonRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.title.CreatePersonResponse>}
 *     Promise that resolves to the response
 */
proto.title.TitleServicePromiseClient.prototype.createPerson =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/title.TitleService/CreatePerson',
      request,
      metadata || {},
      methodDescriptor_TitleService_CreatePerson);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.title.DeletePersonRequest,
 *   !proto.title.DeletePersonResponse>}
 */
const methodDescriptor_TitleService_DeletePerson = new grpc.web.MethodDescriptor(
  '/title.TitleService/DeletePerson',
  grpc.web.MethodType.UNARY,
  proto.title.DeletePersonRequest,
  proto.title.DeletePersonResponse,
  /**
   * @param {!proto.title.DeletePersonRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.title.DeletePersonResponse.deserializeBinary
);


/**
 * @param {!proto.title.DeletePersonRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.title.DeletePersonResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.title.DeletePersonResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.title.TitleServiceClient.prototype.deletePerson =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/title.TitleService/DeletePerson',
      request,
      metadata || {},
      methodDescriptor_TitleService_DeletePerson,
      callback);
};


/**
 * @param {!proto.title.DeletePersonRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.title.DeletePersonResponse>}
 *     Promise that resolves to the response
 */
proto.title.TitleServicePromiseClient.prototype.deletePerson =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/title.TitleService/DeletePerson',
      request,
      metadata || {},
      methodDescriptor_TitleService_DeletePerson);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.title.GetPersonByIdRequest,
 *   !proto.title.GetPersonByIdResponse>}
 */
const methodDescriptor_TitleService_GetPersonById = new grpc.web.MethodDescriptor(
  '/title.TitleService/GetPersonById',
  grpc.web.MethodType.UNARY,
  proto.title.GetPersonByIdRequest,
  proto.title.GetPersonByIdResponse,
  /**
   * @param {!proto.title.GetPersonByIdRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.title.GetPersonByIdResponse.deserializeBinary
);


/**
 * @param {!proto.title.GetPersonByIdRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.title.GetPersonByIdResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.title.GetPersonByIdResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.title.TitleServiceClient.prototype.getPersonById =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/title.TitleService/GetPersonById',
      request,
      metadata || {},
      methodDescriptor_TitleService_GetPersonById,
      callback);
};


/**
 * @param {!proto.title.GetPersonByIdRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.title.GetPersonByIdResponse>}
 *     Promise that resolves to the response
 */
proto.title.TitleServicePromiseClient.prototype.getPersonById =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/title.TitleService/GetPersonById',
      request,
      metadata || {},
      methodDescriptor_TitleService_GetPersonById);
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
 *   !proto.title.CreateVideoRequest,
 *   !proto.title.CreateVideoResponse>}
 */
const methodDescriptor_TitleService_CreateVideo = new grpc.web.MethodDescriptor(
  '/title.TitleService/CreateVideo',
  grpc.web.MethodType.UNARY,
  proto.title.CreateVideoRequest,
  proto.title.CreateVideoResponse,
  /**
   * @param {!proto.title.CreateVideoRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.title.CreateVideoResponse.deserializeBinary
);


/**
 * @param {!proto.title.CreateVideoRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.title.CreateVideoResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.title.CreateVideoResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.title.TitleServiceClient.prototype.createVideo =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/title.TitleService/CreateVideo',
      request,
      metadata || {},
      methodDescriptor_TitleService_CreateVideo,
      callback);
};


/**
 * @param {!proto.title.CreateVideoRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.title.CreateVideoResponse>}
 *     Promise that resolves to the response
 */
proto.title.TitleServicePromiseClient.prototype.createVideo =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/title.TitleService/CreateVideo',
      request,
      metadata || {},
      methodDescriptor_TitleService_CreateVideo);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.title.GetVideoByIdRequest,
 *   !proto.title.GetVideoByIdResponse>}
 */
const methodDescriptor_TitleService_GetVideoById = new grpc.web.MethodDescriptor(
  '/title.TitleService/GetVideoById',
  grpc.web.MethodType.UNARY,
  proto.title.GetVideoByIdRequest,
  proto.title.GetVideoByIdResponse,
  /**
   * @param {!proto.title.GetVideoByIdRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.title.GetVideoByIdResponse.deserializeBinary
);


/**
 * @param {!proto.title.GetVideoByIdRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.title.GetVideoByIdResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.title.GetVideoByIdResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.title.TitleServiceClient.prototype.getVideoById =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/title.TitleService/GetVideoById',
      request,
      metadata || {},
      methodDescriptor_TitleService_GetVideoById,
      callback);
};


/**
 * @param {!proto.title.GetVideoByIdRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.title.GetVideoByIdResponse>}
 *     Promise that resolves to the response
 */
proto.title.TitleServicePromiseClient.prototype.getVideoById =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/title.TitleService/GetVideoById',
      request,
      metadata || {},
      methodDescriptor_TitleService_GetVideoById);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.title.DeleteVideoRequest,
 *   !proto.title.DeleteVideoResponse>}
 */
const methodDescriptor_TitleService_DeleteVideo = new grpc.web.MethodDescriptor(
  '/title.TitleService/DeleteVideo',
  grpc.web.MethodType.UNARY,
  proto.title.DeleteVideoRequest,
  proto.title.DeleteVideoResponse,
  /**
   * @param {!proto.title.DeleteVideoRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.title.DeleteVideoResponse.deserializeBinary
);


/**
 * @param {!proto.title.DeleteVideoRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.title.DeleteVideoResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.title.DeleteVideoResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.title.TitleServiceClient.prototype.deleteVideo =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/title.TitleService/DeleteVideo',
      request,
      metadata || {},
      methodDescriptor_TitleService_DeleteVideo,
      callback);
};


/**
 * @param {!proto.title.DeleteVideoRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.title.DeleteVideoResponse>}
 *     Promise that resolves to the response
 */
proto.title.TitleServicePromiseClient.prototype.deleteVideo =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/title.TitleService/DeleteVideo',
      request,
      metadata || {},
      methodDescriptor_TitleService_DeleteVideo);
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
 *   !proto.title.GetFileVideosRequest,
 *   !proto.title.GetFileVideosResponse>}
 */
const methodDescriptor_TitleService_GetFileVideos = new grpc.web.MethodDescriptor(
  '/title.TitleService/GetFileVideos',
  grpc.web.MethodType.UNARY,
  proto.title.GetFileVideosRequest,
  proto.title.GetFileVideosResponse,
  /**
   * @param {!proto.title.GetFileVideosRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.title.GetFileVideosResponse.deserializeBinary
);


/**
 * @param {!proto.title.GetFileVideosRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.title.GetFileVideosResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.title.GetFileVideosResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.title.TitleServiceClient.prototype.getFileVideos =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/title.TitleService/GetFileVideos',
      request,
      metadata || {},
      methodDescriptor_TitleService_GetFileVideos,
      callback);
};


/**
 * @param {!proto.title.GetFileVideosRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.title.GetFileVideosResponse>}
 *     Promise that resolves to the response
 */
proto.title.TitleServicePromiseClient.prototype.getFileVideos =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/title.TitleService/GetFileVideos',
      request,
      metadata || {},
      methodDescriptor_TitleService_GetFileVideos);
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

