/**
 * @fileoverview gRPC-Web generated client stub for conversation
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.conversation = require('./conversation_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.conversation.ConversationServiceClient =
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
proto.conversation.ConversationServicePromiseClient =
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
 *   !proto.conversation.StopRequest,
 *   !proto.conversation.StopResponse>}
 */
const methodDescriptor_ConversationService_Stop = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/Stop',
  grpc.web.MethodType.UNARY,
  proto.conversation.StopRequest,
  proto.conversation.StopResponse,
  /**
   * @param {!proto.conversation.StopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.StopResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.StopRequest,
 *   !proto.conversation.StopResponse>}
 */
const methodInfo_ConversationService_Stop = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.StopResponse,
  /**
   * @param {!proto.conversation.StopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.StopResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.conversation.StopResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.StopResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.stop =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/conversation.ConversationService/Stop',
      request,
      metadata || {},
      methodDescriptor_ConversationService_Stop,
      callback);
};


/**
 * @param {!proto.conversation.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.conversation.StopResponse>}
 *     Promise that resolves to the response
 */
proto.conversation.ConversationServicePromiseClient.prototype.stop =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/conversation.ConversationService/Stop',
      request,
      metadata || {},
      methodDescriptor_ConversationService_Stop);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.conversation.ConnectRequest,
 *   !proto.conversation.ConnectResponse>}
 */
const methodDescriptor_ConversationService_Connect = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/Connect',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.conversation.ConnectRequest,
  proto.conversation.ConnectResponse,
  /**
   * @param {!proto.conversation.ConnectRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.ConnectResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.ConnectRequest,
 *   !proto.conversation.ConnectResponse>}
 */
const methodInfo_ConversationService_Connect = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.ConnectResponse,
  /**
   * @param {!proto.conversation.ConnectRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.ConnectResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.ConnectRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.ConnectResponse>}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.connect =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/conversation.ConversationService/Connect',
      request,
      metadata || {},
      methodDescriptor_ConversationService_Connect);
};


/**
 * @param {!proto.conversation.ConnectRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.ConnectResponse>}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServicePromiseClient.prototype.connect =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/conversation.ConversationService/Connect',
      request,
      metadata || {},
      methodDescriptor_ConversationService_Connect);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.conversation.DisconnectRequest,
 *   !proto.conversation.DisconnectResponse>}
 */
const methodDescriptor_ConversationService_Disconnect = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/Disconnect',
  grpc.web.MethodType.UNARY,
  proto.conversation.DisconnectRequest,
  proto.conversation.DisconnectResponse,
  /**
   * @param {!proto.conversation.DisconnectRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.DisconnectResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.DisconnectRequest,
 *   !proto.conversation.DisconnectResponse>}
 */
const methodInfo_ConversationService_Disconnect = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.DisconnectResponse,
  /**
   * @param {!proto.conversation.DisconnectRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.DisconnectResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.DisconnectRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.conversation.DisconnectResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.DisconnectResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.disconnect =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/conversation.ConversationService/Disconnect',
      request,
      metadata || {},
      methodDescriptor_ConversationService_Disconnect,
      callback);
};


/**
 * @param {!proto.conversation.DisconnectRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.conversation.DisconnectResponse>}
 *     Promise that resolves to the response
 */
proto.conversation.ConversationServicePromiseClient.prototype.disconnect =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/conversation.ConversationService/Disconnect',
      request,
      metadata || {},
      methodDescriptor_ConversationService_Disconnect);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.conversation.CreateConversationRequest,
 *   !proto.conversation.CreateConversationResponse>}
 */
const methodDescriptor_ConversationService_CreateConversation = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/CreateConversation',
  grpc.web.MethodType.UNARY,
  proto.conversation.CreateConversationRequest,
  proto.conversation.CreateConversationResponse,
  /**
   * @param {!proto.conversation.CreateConversationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.CreateConversationResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.CreateConversationRequest,
 *   !proto.conversation.CreateConversationResponse>}
 */
const methodInfo_ConversationService_CreateConversation = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.CreateConversationResponse,
  /**
   * @param {!proto.conversation.CreateConversationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.CreateConversationResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.CreateConversationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.conversation.CreateConversationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.CreateConversationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.createConversation =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/conversation.ConversationService/CreateConversation',
      request,
      metadata || {},
      methodDescriptor_ConversationService_CreateConversation,
      callback);
};


/**
 * @param {!proto.conversation.CreateConversationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.conversation.CreateConversationResponse>}
 *     Promise that resolves to the response
 */
proto.conversation.ConversationServicePromiseClient.prototype.createConversation =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/conversation.ConversationService/CreateConversation',
      request,
      metadata || {},
      methodDescriptor_ConversationService_CreateConversation);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.conversation.DeleteConversationRequest,
 *   !proto.conversation.DeleteConversationResponse>}
 */
const methodDescriptor_ConversationService_DeleteConversation = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/DeleteConversation',
  grpc.web.MethodType.UNARY,
  proto.conversation.DeleteConversationRequest,
  proto.conversation.DeleteConversationResponse,
  /**
   * @param {!proto.conversation.DeleteConversationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.DeleteConversationResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.DeleteConversationRequest,
 *   !proto.conversation.DeleteConversationResponse>}
 */
const methodInfo_ConversationService_DeleteConversation = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.DeleteConversationResponse,
  /**
   * @param {!proto.conversation.DeleteConversationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.DeleteConversationResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.DeleteConversationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.conversation.DeleteConversationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.DeleteConversationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.deleteConversation =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/conversation.ConversationService/DeleteConversation',
      request,
      metadata || {},
      methodDescriptor_ConversationService_DeleteConversation,
      callback);
};


/**
 * @param {!proto.conversation.DeleteConversationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.conversation.DeleteConversationResponse>}
 *     Promise that resolves to the response
 */
proto.conversation.ConversationServicePromiseClient.prototype.deleteConversation =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/conversation.ConversationService/DeleteConversation',
      request,
      metadata || {},
      methodDescriptor_ConversationService_DeleteConversation);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.conversation.FindConversationsRequest,
 *   !proto.conversation.FindConversationsResponse>}
 */
const methodDescriptor_ConversationService_FindConversations = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/FindConversations',
  grpc.web.MethodType.UNARY,
  proto.conversation.FindConversationsRequest,
  proto.conversation.FindConversationsResponse,
  /**
   * @param {!proto.conversation.FindConversationsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.FindConversationsResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.FindConversationsRequest,
 *   !proto.conversation.FindConversationsResponse>}
 */
const methodInfo_ConversationService_FindConversations = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.FindConversationsResponse,
  /**
   * @param {!proto.conversation.FindConversationsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.FindConversationsResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.FindConversationsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.conversation.FindConversationsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.FindConversationsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.findConversations =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/conversation.ConversationService/FindConversations',
      request,
      metadata || {},
      methodDescriptor_ConversationService_FindConversations,
      callback);
};


/**
 * @param {!proto.conversation.FindConversationsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.conversation.FindConversationsResponse>}
 *     Promise that resolves to the response
 */
proto.conversation.ConversationServicePromiseClient.prototype.findConversations =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/conversation.ConversationService/FindConversations',
      request,
      metadata || {},
      methodDescriptor_ConversationService_FindConversations);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.conversation.JoinConversationRequest,
 *   !proto.conversation.JoinConversationResponse>}
 */
const methodDescriptor_ConversationService_JoinConversation = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/JoinConversation',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.conversation.JoinConversationRequest,
  proto.conversation.JoinConversationResponse,
  /**
   * @param {!proto.conversation.JoinConversationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.JoinConversationResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.JoinConversationRequest,
 *   !proto.conversation.JoinConversationResponse>}
 */
const methodInfo_ConversationService_JoinConversation = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.JoinConversationResponse,
  /**
   * @param {!proto.conversation.JoinConversationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.JoinConversationResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.JoinConversationRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.JoinConversationResponse>}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.joinConversation =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/conversation.ConversationService/JoinConversation',
      request,
      metadata || {},
      methodDescriptor_ConversationService_JoinConversation);
};


/**
 * @param {!proto.conversation.JoinConversationRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.JoinConversationResponse>}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServicePromiseClient.prototype.joinConversation =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/conversation.ConversationService/JoinConversation',
      request,
      metadata || {},
      methodDescriptor_ConversationService_JoinConversation);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.conversation.LeaveConversationRequest,
 *   !proto.conversation.LeaveConversationResponse>}
 */
const methodDescriptor_ConversationService_LeaveConversation = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/LeaveConversation',
  grpc.web.MethodType.UNARY,
  proto.conversation.LeaveConversationRequest,
  proto.conversation.LeaveConversationResponse,
  /**
   * @param {!proto.conversation.LeaveConversationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.LeaveConversationResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.LeaveConversationRequest,
 *   !proto.conversation.LeaveConversationResponse>}
 */
const methodInfo_ConversationService_LeaveConversation = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.LeaveConversationResponse,
  /**
   * @param {!proto.conversation.LeaveConversationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.LeaveConversationResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.LeaveConversationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.conversation.LeaveConversationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.LeaveConversationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.leaveConversation =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/conversation.ConversationService/LeaveConversation',
      request,
      metadata || {},
      methodDescriptor_ConversationService_LeaveConversation,
      callback);
};


/**
 * @param {!proto.conversation.LeaveConversationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.conversation.LeaveConversationResponse>}
 *     Promise that resolves to the response
 */
proto.conversation.ConversationServicePromiseClient.prototype.leaveConversation =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/conversation.ConversationService/LeaveConversation',
      request,
      metadata || {},
      methodDescriptor_ConversationService_LeaveConversation);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.conversation.GetConversationsRequest,
 *   !proto.conversation.GetConversationsResponse>}
 */
const methodDescriptor_ConversationService_GetConversations = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/GetConversations',
  grpc.web.MethodType.UNARY,
  proto.conversation.GetConversationsRequest,
  proto.conversation.GetConversationsResponse,
  /**
   * @param {!proto.conversation.GetConversationsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.GetConversationsResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.GetConversationsRequest,
 *   !proto.conversation.GetConversationsResponse>}
 */
const methodInfo_ConversationService_GetConversations = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.GetConversationsResponse,
  /**
   * @param {!proto.conversation.GetConversationsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.GetConversationsResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.GetConversationsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.conversation.GetConversationsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.GetConversationsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.getConversations =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/conversation.ConversationService/GetConversations',
      request,
      metadata || {},
      methodDescriptor_ConversationService_GetConversations,
      callback);
};


/**
 * @param {!proto.conversation.GetConversationsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.conversation.GetConversationsResponse>}
 *     Promise that resolves to the response
 */
proto.conversation.ConversationServicePromiseClient.prototype.getConversations =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/conversation.ConversationService/GetConversations',
      request,
      metadata || {},
      methodDescriptor_ConversationService_GetConversations);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.conversation.KickoutFromConversationRequest,
 *   !proto.conversation.KickoutFromConversationResponse>}
 */
const methodDescriptor_ConversationService_KickoutFromConversation = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/KickoutFromConversation',
  grpc.web.MethodType.UNARY,
  proto.conversation.KickoutFromConversationRequest,
  proto.conversation.KickoutFromConversationResponse,
  /**
   * @param {!proto.conversation.KickoutFromConversationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.KickoutFromConversationResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.KickoutFromConversationRequest,
 *   !proto.conversation.KickoutFromConversationResponse>}
 */
const methodInfo_ConversationService_KickoutFromConversation = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.KickoutFromConversationResponse,
  /**
   * @param {!proto.conversation.KickoutFromConversationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.KickoutFromConversationResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.KickoutFromConversationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.conversation.KickoutFromConversationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.KickoutFromConversationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.kickoutFromConversation =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/conversation.ConversationService/KickoutFromConversation',
      request,
      metadata || {},
      methodDescriptor_ConversationService_KickoutFromConversation,
      callback);
};


/**
 * @param {!proto.conversation.KickoutFromConversationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.conversation.KickoutFromConversationResponse>}
 *     Promise that resolves to the response
 */
proto.conversation.ConversationServicePromiseClient.prototype.kickoutFromConversation =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/conversation.ConversationService/KickoutFromConversation',
      request,
      metadata || {},
      methodDescriptor_ConversationService_KickoutFromConversation);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.conversation.SendInvitationRequest,
 *   !proto.conversation.SendInvitationResponse>}
 */
const methodDescriptor_ConversationService_SendInvitation = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/SendInvitation',
  grpc.web.MethodType.UNARY,
  proto.conversation.SendInvitationRequest,
  proto.conversation.SendInvitationResponse,
  /**
   * @param {!proto.conversation.SendInvitationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.SendInvitationResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.SendInvitationRequest,
 *   !proto.conversation.SendInvitationResponse>}
 */
const methodInfo_ConversationService_SendInvitation = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.SendInvitationResponse,
  /**
   * @param {!proto.conversation.SendInvitationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.SendInvitationResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.SendInvitationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.conversation.SendInvitationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.SendInvitationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.sendInvitation =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/conversation.ConversationService/SendInvitation',
      request,
      metadata || {},
      methodDescriptor_ConversationService_SendInvitation,
      callback);
};


/**
 * @param {!proto.conversation.SendInvitationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.conversation.SendInvitationResponse>}
 *     Promise that resolves to the response
 */
proto.conversation.ConversationServicePromiseClient.prototype.sendInvitation =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/conversation.ConversationService/SendInvitation',
      request,
      metadata || {},
      methodDescriptor_ConversationService_SendInvitation);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.conversation.AcceptInvitationRequest,
 *   !proto.conversation.AcceptInvitationResponse>}
 */
const methodDescriptor_ConversationService_AcceptInvitation = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/AcceptInvitation',
  grpc.web.MethodType.UNARY,
  proto.conversation.AcceptInvitationRequest,
  proto.conversation.AcceptInvitationResponse,
  /**
   * @param {!proto.conversation.AcceptInvitationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.AcceptInvitationResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.AcceptInvitationRequest,
 *   !proto.conversation.AcceptInvitationResponse>}
 */
const methodInfo_ConversationService_AcceptInvitation = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.AcceptInvitationResponse,
  /**
   * @param {!proto.conversation.AcceptInvitationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.AcceptInvitationResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.AcceptInvitationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.conversation.AcceptInvitationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.AcceptInvitationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.acceptInvitation =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/conversation.ConversationService/AcceptInvitation',
      request,
      metadata || {},
      methodDescriptor_ConversationService_AcceptInvitation,
      callback);
};


/**
 * @param {!proto.conversation.AcceptInvitationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.conversation.AcceptInvitationResponse>}
 *     Promise that resolves to the response
 */
proto.conversation.ConversationServicePromiseClient.prototype.acceptInvitation =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/conversation.ConversationService/AcceptInvitation',
      request,
      metadata || {},
      methodDescriptor_ConversationService_AcceptInvitation);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.conversation.DeclineInvitationRequest,
 *   !proto.conversation.DeclineInvitationResponse>}
 */
const methodDescriptor_ConversationService_DeclineInvitation = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/DeclineInvitation',
  grpc.web.MethodType.UNARY,
  proto.conversation.DeclineInvitationRequest,
  proto.conversation.DeclineInvitationResponse,
  /**
   * @param {!proto.conversation.DeclineInvitationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.DeclineInvitationResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.DeclineInvitationRequest,
 *   !proto.conversation.DeclineInvitationResponse>}
 */
const methodInfo_ConversationService_DeclineInvitation = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.DeclineInvitationResponse,
  /**
   * @param {!proto.conversation.DeclineInvitationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.DeclineInvitationResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.DeclineInvitationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.conversation.DeclineInvitationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.DeclineInvitationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.declineInvitation =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/conversation.ConversationService/DeclineInvitation',
      request,
      metadata || {},
      methodDescriptor_ConversationService_DeclineInvitation,
      callback);
};


/**
 * @param {!proto.conversation.DeclineInvitationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.conversation.DeclineInvitationResponse>}
 *     Promise that resolves to the response
 */
proto.conversation.ConversationServicePromiseClient.prototype.declineInvitation =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/conversation.ConversationService/DeclineInvitation',
      request,
      metadata || {},
      methodDescriptor_ConversationService_DeclineInvitation);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.conversation.RevokeInvitationRequest,
 *   !proto.conversation.RevokeInvitationResponse>}
 */
const methodDescriptor_ConversationService_RevokeInvitation = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/RevokeInvitation',
  grpc.web.MethodType.UNARY,
  proto.conversation.RevokeInvitationRequest,
  proto.conversation.RevokeInvitationResponse,
  /**
   * @param {!proto.conversation.RevokeInvitationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.RevokeInvitationResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.RevokeInvitationRequest,
 *   !proto.conversation.RevokeInvitationResponse>}
 */
const methodInfo_ConversationService_RevokeInvitation = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.RevokeInvitationResponse,
  /**
   * @param {!proto.conversation.RevokeInvitationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.RevokeInvitationResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.RevokeInvitationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.conversation.RevokeInvitationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.RevokeInvitationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.revokeInvitation =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/conversation.ConversationService/RevokeInvitation',
      request,
      metadata || {},
      methodDescriptor_ConversationService_RevokeInvitation,
      callback);
};


/**
 * @param {!proto.conversation.RevokeInvitationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.conversation.RevokeInvitationResponse>}
 *     Promise that resolves to the response
 */
proto.conversation.ConversationServicePromiseClient.prototype.revokeInvitation =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/conversation.ConversationService/RevokeInvitation',
      request,
      metadata || {},
      methodDescriptor_ConversationService_RevokeInvitation);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.conversation.GetReceivedInvitationsRequest,
 *   !proto.conversation.GetReceivedInvitationsResponse>}
 */
const methodDescriptor_ConversationService_GetReceivedInvitations = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/GetReceivedInvitations',
  grpc.web.MethodType.UNARY,
  proto.conversation.GetReceivedInvitationsRequest,
  proto.conversation.GetReceivedInvitationsResponse,
  /**
   * @param {!proto.conversation.GetReceivedInvitationsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.GetReceivedInvitationsResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.GetReceivedInvitationsRequest,
 *   !proto.conversation.GetReceivedInvitationsResponse>}
 */
const methodInfo_ConversationService_GetReceivedInvitations = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.GetReceivedInvitationsResponse,
  /**
   * @param {!proto.conversation.GetReceivedInvitationsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.GetReceivedInvitationsResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.GetReceivedInvitationsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.conversation.GetReceivedInvitationsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.GetReceivedInvitationsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.getReceivedInvitations =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/conversation.ConversationService/GetReceivedInvitations',
      request,
      metadata || {},
      methodDescriptor_ConversationService_GetReceivedInvitations,
      callback);
};


/**
 * @param {!proto.conversation.GetReceivedInvitationsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.conversation.GetReceivedInvitationsResponse>}
 *     Promise that resolves to the response
 */
proto.conversation.ConversationServicePromiseClient.prototype.getReceivedInvitations =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/conversation.ConversationService/GetReceivedInvitations',
      request,
      metadata || {},
      methodDescriptor_ConversationService_GetReceivedInvitations);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.conversation.GetSentInvitationsRequest,
 *   !proto.conversation.GetSentInvitationsResponse>}
 */
const methodDescriptor_ConversationService_GetSentInvitations = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/GetSentInvitations',
  grpc.web.MethodType.UNARY,
  proto.conversation.GetSentInvitationsRequest,
  proto.conversation.GetSentInvitationsResponse,
  /**
   * @param {!proto.conversation.GetSentInvitationsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.GetSentInvitationsResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.GetSentInvitationsRequest,
 *   !proto.conversation.GetSentInvitationsResponse>}
 */
const methodInfo_ConversationService_GetSentInvitations = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.GetSentInvitationsResponse,
  /**
   * @param {!proto.conversation.GetSentInvitationsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.GetSentInvitationsResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.GetSentInvitationsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.conversation.GetSentInvitationsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.GetSentInvitationsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.getSentInvitations =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/conversation.ConversationService/GetSentInvitations',
      request,
      metadata || {},
      methodDescriptor_ConversationService_GetSentInvitations,
      callback);
};


/**
 * @param {!proto.conversation.GetSentInvitationsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.conversation.GetSentInvitationsResponse>}
 *     Promise that resolves to the response
 */
proto.conversation.ConversationServicePromiseClient.prototype.getSentInvitations =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/conversation.ConversationService/GetSentInvitations',
      request,
      metadata || {},
      methodDescriptor_ConversationService_GetSentInvitations);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.conversation.SendMessageRequest,
 *   !proto.conversation.SendMessageResponse>}
 */
const methodDescriptor_ConversationService_SendMessage = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/SendMessage',
  grpc.web.MethodType.UNARY,
  proto.conversation.SendMessageRequest,
  proto.conversation.SendMessageResponse,
  /**
   * @param {!proto.conversation.SendMessageRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.SendMessageResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.SendMessageRequest,
 *   !proto.conversation.SendMessageResponse>}
 */
const methodInfo_ConversationService_SendMessage = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.SendMessageResponse,
  /**
   * @param {!proto.conversation.SendMessageRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.SendMessageResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.SendMessageRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.conversation.SendMessageResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.SendMessageResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.sendMessage =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/conversation.ConversationService/SendMessage',
      request,
      metadata || {},
      methodDescriptor_ConversationService_SendMessage,
      callback);
};


/**
 * @param {!proto.conversation.SendMessageRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.conversation.SendMessageResponse>}
 *     Promise that resolves to the response
 */
proto.conversation.ConversationServicePromiseClient.prototype.sendMessage =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/conversation.ConversationService/SendMessage',
      request,
      metadata || {},
      methodDescriptor_ConversationService_SendMessage);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.conversation.DeleteMessageRequest,
 *   !proto.conversation.DeleteMessageResponse>}
 */
const methodDescriptor_ConversationService_DeleteMessage = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/DeleteMessage',
  grpc.web.MethodType.UNARY,
  proto.conversation.DeleteMessageRequest,
  proto.conversation.DeleteMessageResponse,
  /**
   * @param {!proto.conversation.DeleteMessageRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.DeleteMessageResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.DeleteMessageRequest,
 *   !proto.conversation.DeleteMessageResponse>}
 */
const methodInfo_ConversationService_DeleteMessage = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.DeleteMessageResponse,
  /**
   * @param {!proto.conversation.DeleteMessageRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.DeleteMessageResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.DeleteMessageRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.conversation.DeleteMessageResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.DeleteMessageResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.deleteMessage =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/conversation.ConversationService/DeleteMessage',
      request,
      metadata || {},
      methodDescriptor_ConversationService_DeleteMessage,
      callback);
};


/**
 * @param {!proto.conversation.DeleteMessageRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.conversation.DeleteMessageResponse>}
 *     Promise that resolves to the response
 */
proto.conversation.ConversationServicePromiseClient.prototype.deleteMessage =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/conversation.ConversationService/DeleteMessage',
      request,
      metadata || {},
      methodDescriptor_ConversationService_DeleteMessage);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.conversation.FindMessagesRequest,
 *   !proto.conversation.FindMessagesResponse>}
 */
const methodDescriptor_ConversationService_FindMessages = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/FindMessages',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.conversation.FindMessagesRequest,
  proto.conversation.FindMessagesResponse,
  /**
   * @param {!proto.conversation.FindMessagesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.FindMessagesResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.FindMessagesRequest,
 *   !proto.conversation.FindMessagesResponse>}
 */
const methodInfo_ConversationService_FindMessages = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.FindMessagesResponse,
  /**
   * @param {!proto.conversation.FindMessagesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.FindMessagesResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.FindMessagesRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.FindMessagesResponse>}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.findMessages =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/conversation.ConversationService/FindMessages',
      request,
      metadata || {},
      methodDescriptor_ConversationService_FindMessages);
};


/**
 * @param {!proto.conversation.FindMessagesRequest} request The request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.FindMessagesResponse>}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServicePromiseClient.prototype.findMessages =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/conversation.ConversationService/FindMessages',
      request,
      metadata || {},
      methodDescriptor_ConversationService_FindMessages);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.conversation.LikeMessageRqst,
 *   !proto.conversation.LikeMessageResponse>}
 */
const methodDescriptor_ConversationService_LikeMessage = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/LikeMessage',
  grpc.web.MethodType.UNARY,
  proto.conversation.LikeMessageRqst,
  proto.conversation.LikeMessageResponse,
  /**
   * @param {!proto.conversation.LikeMessageRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.LikeMessageResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.LikeMessageRqst,
 *   !proto.conversation.LikeMessageResponse>}
 */
const methodInfo_ConversationService_LikeMessage = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.LikeMessageResponse,
  /**
   * @param {!proto.conversation.LikeMessageRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.LikeMessageResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.LikeMessageRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.conversation.LikeMessageResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.LikeMessageResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.likeMessage =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/conversation.ConversationService/LikeMessage',
      request,
      metadata || {},
      methodDescriptor_ConversationService_LikeMessage,
      callback);
};


/**
 * @param {!proto.conversation.LikeMessageRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.conversation.LikeMessageResponse>}
 *     Promise that resolves to the response
 */
proto.conversation.ConversationServicePromiseClient.prototype.likeMessage =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/conversation.ConversationService/LikeMessage',
      request,
      metadata || {},
      methodDescriptor_ConversationService_LikeMessage);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.conversation.DislikeMessageRqst,
 *   !proto.conversation.DislikeMessageResponse>}
 */
const methodDescriptor_ConversationService_DislikeMessage = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/DislikeMessage',
  grpc.web.MethodType.UNARY,
  proto.conversation.DislikeMessageRqst,
  proto.conversation.DislikeMessageResponse,
  /**
   * @param {!proto.conversation.DislikeMessageRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.DislikeMessageResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.DislikeMessageRqst,
 *   !proto.conversation.DislikeMessageResponse>}
 */
const methodInfo_ConversationService_DislikeMessage = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.DislikeMessageResponse,
  /**
   * @param {!proto.conversation.DislikeMessageRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.DislikeMessageResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.DislikeMessageRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.conversation.DislikeMessageResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.DislikeMessageResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.dislikeMessage =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/conversation.ConversationService/DislikeMessage',
      request,
      metadata || {},
      methodDescriptor_ConversationService_DislikeMessage,
      callback);
};


/**
 * @param {!proto.conversation.DislikeMessageRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.conversation.DislikeMessageResponse>}
 *     Promise that resolves to the response
 */
proto.conversation.ConversationServicePromiseClient.prototype.dislikeMessage =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/conversation.ConversationService/DislikeMessage',
      request,
      metadata || {},
      methodDescriptor_ConversationService_DislikeMessage);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.conversation.SetMessageReadRqst,
 *   !proto.conversation.SetMessageReadResponse>}
 */
const methodDescriptor_ConversationService_SetMessageRead = new grpc.web.MethodDescriptor(
  '/conversation.ConversationService/SetMessageRead',
  grpc.web.MethodType.UNARY,
  proto.conversation.SetMessageReadRqst,
  proto.conversation.SetMessageReadResponse,
  /**
   * @param {!proto.conversation.SetMessageReadRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.SetMessageReadResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.conversation.SetMessageReadRqst,
 *   !proto.conversation.SetMessageReadResponse>}
 */
const methodInfo_ConversationService_SetMessageRead = new grpc.web.AbstractClientBase.MethodInfo(
  proto.conversation.SetMessageReadResponse,
  /**
   * @param {!proto.conversation.SetMessageReadRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.conversation.SetMessageReadResponse.deserializeBinary
);


/**
 * @param {!proto.conversation.SetMessageReadRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.conversation.SetMessageReadResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.conversation.SetMessageReadResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.conversation.ConversationServiceClient.prototype.setMessageRead =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/conversation.ConversationService/SetMessageRead',
      request,
      metadata || {},
      methodDescriptor_ConversationService_SetMessageRead,
      callback);
};


/**
 * @param {!proto.conversation.SetMessageReadRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.conversation.SetMessageReadResponse>}
 *     Promise that resolves to the response
 */
proto.conversation.ConversationServicePromiseClient.prototype.setMessageRead =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/conversation.ConversationService/SetMessageRead',
      request,
      metadata || {},
      methodDescriptor_ConversationService_SetMessageRead);
};


module.exports = proto.conversation;

