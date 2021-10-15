/**
 * @fileoverview gRPC-Web generated client stub for mail
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.mail = require('./mail_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.mail.MailServiceClient =
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
proto.mail.MailServicePromiseClient =
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
 *   !proto.mail.StopRequest,
 *   !proto.mail.StopResponse>}
 */
const methodDescriptor_MailService_Stop = new grpc.web.MethodDescriptor(
  '/mail.MailService/Stop',
  grpc.web.MethodType.UNARY,
  proto.mail.StopRequest,
  proto.mail.StopResponse,
  /**
   * @param {!proto.mail.StopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.mail.StopResponse.deserializeBinary
);


/**
 * @param {!proto.mail.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.mail.StopResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.mail.StopResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.mail.MailServiceClient.prototype.stop =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/mail.MailService/Stop',
      request,
      metadata || {},
      methodDescriptor_MailService_Stop,
      callback);
};


/**
 * @param {!proto.mail.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.mail.StopResponse>}
 *     Promise that resolves to the response
 */
proto.mail.MailServicePromiseClient.prototype.stop =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/mail.MailService/Stop',
      request,
      metadata || {},
      methodDescriptor_MailService_Stop);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.mail.CreateConnectionRqst,
 *   !proto.mail.CreateConnectionRsp>}
 */
const methodDescriptor_MailService_CreateConnection = new grpc.web.MethodDescriptor(
  '/mail.MailService/CreateConnection',
  grpc.web.MethodType.UNARY,
  proto.mail.CreateConnectionRqst,
  proto.mail.CreateConnectionRsp,
  /**
   * @param {!proto.mail.CreateConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.mail.CreateConnectionRsp.deserializeBinary
);


/**
 * @param {!proto.mail.CreateConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.mail.CreateConnectionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.mail.CreateConnectionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.mail.MailServiceClient.prototype.createConnection =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/mail.MailService/CreateConnection',
      request,
      metadata || {},
      methodDescriptor_MailService_CreateConnection,
      callback);
};


/**
 * @param {!proto.mail.CreateConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.mail.CreateConnectionRsp>}
 *     Promise that resolves to the response
 */
proto.mail.MailServicePromiseClient.prototype.createConnection =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/mail.MailService/CreateConnection',
      request,
      metadata || {},
      methodDescriptor_MailService_CreateConnection);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.mail.DeleteConnectionRqst,
 *   !proto.mail.DeleteConnectionRsp>}
 */
const methodDescriptor_MailService_DeleteConnection = new grpc.web.MethodDescriptor(
  '/mail.MailService/DeleteConnection',
  grpc.web.MethodType.UNARY,
  proto.mail.DeleteConnectionRqst,
  proto.mail.DeleteConnectionRsp,
  /**
   * @param {!proto.mail.DeleteConnectionRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.mail.DeleteConnectionRsp.deserializeBinary
);


/**
 * @param {!proto.mail.DeleteConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.mail.DeleteConnectionRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.mail.DeleteConnectionRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.mail.MailServiceClient.prototype.deleteConnection =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/mail.MailService/DeleteConnection',
      request,
      metadata || {},
      methodDescriptor_MailService_DeleteConnection,
      callback);
};


/**
 * @param {!proto.mail.DeleteConnectionRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.mail.DeleteConnectionRsp>}
 *     Promise that resolves to the response
 */
proto.mail.MailServicePromiseClient.prototype.deleteConnection =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/mail.MailService/DeleteConnection',
      request,
      metadata || {},
      methodDescriptor_MailService_DeleteConnection);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.mail.SendEmailRqst,
 *   !proto.mail.SendEmailRsp>}
 */
const methodDescriptor_MailService_SendEmail = new grpc.web.MethodDescriptor(
  '/mail.MailService/SendEmail',
  grpc.web.MethodType.UNARY,
  proto.mail.SendEmailRqst,
  proto.mail.SendEmailRsp,
  /**
   * @param {!proto.mail.SendEmailRqst} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.mail.SendEmailRsp.deserializeBinary
);


/**
 * @param {!proto.mail.SendEmailRqst} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.mail.SendEmailRsp)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.mail.SendEmailRsp>|undefined}
 *     The XHR Node Readable Stream
 */
proto.mail.MailServiceClient.prototype.sendEmail =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/mail.MailService/SendEmail',
      request,
      metadata || {},
      methodDescriptor_MailService_SendEmail,
      callback);
};


/**
 * @param {!proto.mail.SendEmailRqst} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.mail.SendEmailRsp>}
 *     Promise that resolves to the response
 */
proto.mail.MailServicePromiseClient.prototype.sendEmail =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/mail.MailService/SendEmail',
      request,
      metadata || {},
      methodDescriptor_MailService_SendEmail);
};


module.exports = proto.mail;

