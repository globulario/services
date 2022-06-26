/**
 * @fileoverview gRPC-Web generated client stub for blog
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');

const proto = {};
proto.blog = require('./blog_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.blog.BlogServiceClient =
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
proto.blog.BlogServicePromiseClient =
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
 *   !proto.blog.CreateBlogPostRequest,
 *   !proto.blog.CreateBlogPostResponse>}
 */
const methodDescriptor_BlogService_CreateBlogPost = new grpc.web.MethodDescriptor(
  '/blog.BlogService/CreateBlogPost',
  grpc.web.MethodType.UNARY,
  proto.blog.CreateBlogPostRequest,
  proto.blog.CreateBlogPostResponse,
  /**
   * @param {!proto.blog.CreateBlogPostRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.blog.CreateBlogPostResponse.deserializeBinary
);


/**
 * @param {!proto.blog.CreateBlogPostRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.blog.CreateBlogPostResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.blog.CreateBlogPostResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.blog.BlogServiceClient.prototype.createBlogPost =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/blog.BlogService/CreateBlogPost',
      request,
      metadata || {},
      methodDescriptor_BlogService_CreateBlogPost,
      callback);
};


/**
 * @param {!proto.blog.CreateBlogPostRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.blog.CreateBlogPostResponse>}
 *     Promise that resolves to the response
 */
proto.blog.BlogServicePromiseClient.prototype.createBlogPost =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/blog.BlogService/CreateBlogPost',
      request,
      metadata || {},
      methodDescriptor_BlogService_CreateBlogPost);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.blog.SaveBlogPostRequest,
 *   !proto.blog.SaveBlogPostResponse>}
 */
const methodDescriptor_BlogService_SaveBlogPost = new grpc.web.MethodDescriptor(
  '/blog.BlogService/SaveBlogPost',
  grpc.web.MethodType.UNARY,
  proto.blog.SaveBlogPostRequest,
  proto.blog.SaveBlogPostResponse,
  /**
   * @param {!proto.blog.SaveBlogPostRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.blog.SaveBlogPostResponse.deserializeBinary
);


/**
 * @param {!proto.blog.SaveBlogPostRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.blog.SaveBlogPostResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.blog.SaveBlogPostResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.blog.BlogServiceClient.prototype.saveBlogPost =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/blog.BlogService/SaveBlogPost',
      request,
      metadata || {},
      methodDescriptor_BlogService_SaveBlogPost,
      callback);
};


/**
 * @param {!proto.blog.SaveBlogPostRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.blog.SaveBlogPostResponse>}
 *     Promise that resolves to the response
 */
proto.blog.BlogServicePromiseClient.prototype.saveBlogPost =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/blog.BlogService/SaveBlogPost',
      request,
      metadata || {},
      methodDescriptor_BlogService_SaveBlogPost);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.blog.GetBlogPostsByAuthorsRequest,
 *   !proto.blog.GetBlogPostsByAuthorsResponse>}
 */
const methodDescriptor_BlogService_GetBlogPostsByAuthors = new grpc.web.MethodDescriptor(
  '/blog.BlogService/GetBlogPostsByAuthors',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.blog.GetBlogPostsByAuthorsRequest,
  proto.blog.GetBlogPostsByAuthorsResponse,
  /**
   * @param {!proto.blog.GetBlogPostsByAuthorsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.blog.GetBlogPostsByAuthorsResponse.deserializeBinary
);


/**
 * @param {!proto.blog.GetBlogPostsByAuthorsRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.blog.GetBlogPostsByAuthorsResponse>}
 *     The XHR Node Readable Stream
 */
proto.blog.BlogServiceClient.prototype.getBlogPostsByAuthors =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/blog.BlogService/GetBlogPostsByAuthors',
      request,
      metadata || {},
      methodDescriptor_BlogService_GetBlogPostsByAuthors);
};


/**
 * @param {!proto.blog.GetBlogPostsByAuthorsRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.blog.GetBlogPostsByAuthorsResponse>}
 *     The XHR Node Readable Stream
 */
proto.blog.BlogServicePromiseClient.prototype.getBlogPostsByAuthors =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/blog.BlogService/GetBlogPostsByAuthors',
      request,
      metadata || {},
      methodDescriptor_BlogService_GetBlogPostsByAuthors);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.blog.GetBlogPostsRequest,
 *   !proto.blog.GetBlogPostsResponse>}
 */
const methodDescriptor_BlogService_GetBlogPosts = new grpc.web.MethodDescriptor(
  '/blog.BlogService/GetBlogPosts',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.blog.GetBlogPostsRequest,
  proto.blog.GetBlogPostsResponse,
  /**
   * @param {!proto.blog.GetBlogPostsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.blog.GetBlogPostsResponse.deserializeBinary
);


/**
 * @param {!proto.blog.GetBlogPostsRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.blog.GetBlogPostsResponse>}
 *     The XHR Node Readable Stream
 */
proto.blog.BlogServiceClient.prototype.getBlogPosts =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/blog.BlogService/GetBlogPosts',
      request,
      metadata || {},
      methodDescriptor_BlogService_GetBlogPosts);
};


/**
 * @param {!proto.blog.GetBlogPostsRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.blog.GetBlogPostsResponse>}
 *     The XHR Node Readable Stream
 */
proto.blog.BlogServicePromiseClient.prototype.getBlogPosts =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/blog.BlogService/GetBlogPosts',
      request,
      metadata || {},
      methodDescriptor_BlogService_GetBlogPosts);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.blog.SearchBlogPostsRequest,
 *   !proto.blog.SearchBlogPostsResponse>}
 */
const methodDescriptor_BlogService_SearchBlogPosts = new grpc.web.MethodDescriptor(
  '/blog.BlogService/SearchBlogPosts',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.blog.SearchBlogPostsRequest,
  proto.blog.SearchBlogPostsResponse,
  /**
   * @param {!proto.blog.SearchBlogPostsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.blog.SearchBlogPostsResponse.deserializeBinary
);


/**
 * @param {!proto.blog.SearchBlogPostsRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.blog.SearchBlogPostsResponse>}
 *     The XHR Node Readable Stream
 */
proto.blog.BlogServiceClient.prototype.searchBlogPosts =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/blog.BlogService/SearchBlogPosts',
      request,
      metadata || {},
      methodDescriptor_BlogService_SearchBlogPosts);
};


/**
 * @param {!proto.blog.SearchBlogPostsRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.blog.SearchBlogPostsResponse>}
 *     The XHR Node Readable Stream
 */
proto.blog.BlogServicePromiseClient.prototype.searchBlogPosts =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/blog.BlogService/SearchBlogPosts',
      request,
      metadata || {},
      methodDescriptor_BlogService_SearchBlogPosts);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.blog.DeleteBlogPostRequest,
 *   !proto.blog.DeleteBlogPostResponse>}
 */
const methodDescriptor_BlogService_DeleteBlogPost = new grpc.web.MethodDescriptor(
  '/blog.BlogService/DeleteBlogPost',
  grpc.web.MethodType.UNARY,
  proto.blog.DeleteBlogPostRequest,
  proto.blog.DeleteBlogPostResponse,
  /**
   * @param {!proto.blog.DeleteBlogPostRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.blog.DeleteBlogPostResponse.deserializeBinary
);


/**
 * @param {!proto.blog.DeleteBlogPostRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.blog.DeleteBlogPostResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.blog.DeleteBlogPostResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.blog.BlogServiceClient.prototype.deleteBlogPost =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/blog.BlogService/DeleteBlogPost',
      request,
      metadata || {},
      methodDescriptor_BlogService_DeleteBlogPost,
      callback);
};


/**
 * @param {!proto.blog.DeleteBlogPostRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.blog.DeleteBlogPostResponse>}
 *     Promise that resolves to the response
 */
proto.blog.BlogServicePromiseClient.prototype.deleteBlogPost =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/blog.BlogService/DeleteBlogPost',
      request,
      metadata || {},
      methodDescriptor_BlogService_DeleteBlogPost);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.blog.AddEmojiRequest,
 *   !proto.blog.AddEmojiResponse>}
 */
const methodDescriptor_BlogService_AddEmoji = new grpc.web.MethodDescriptor(
  '/blog.BlogService/AddEmoji',
  grpc.web.MethodType.UNARY,
  proto.blog.AddEmojiRequest,
  proto.blog.AddEmojiResponse,
  /**
   * @param {!proto.blog.AddEmojiRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.blog.AddEmojiResponse.deserializeBinary
);


/**
 * @param {!proto.blog.AddEmojiRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.blog.AddEmojiResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.blog.AddEmojiResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.blog.BlogServiceClient.prototype.addEmoji =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/blog.BlogService/AddEmoji',
      request,
      metadata || {},
      methodDescriptor_BlogService_AddEmoji,
      callback);
};


/**
 * @param {!proto.blog.AddEmojiRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.blog.AddEmojiResponse>}
 *     Promise that resolves to the response
 */
proto.blog.BlogServicePromiseClient.prototype.addEmoji =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/blog.BlogService/AddEmoji',
      request,
      metadata || {},
      methodDescriptor_BlogService_AddEmoji);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.blog.RemoveEmojiRequest,
 *   !proto.blog.RemoveEmojiResponse>}
 */
const methodDescriptor_BlogService_RemoveEmoji = new grpc.web.MethodDescriptor(
  '/blog.BlogService/RemoveEmoji',
  grpc.web.MethodType.UNARY,
  proto.blog.RemoveEmojiRequest,
  proto.blog.RemoveEmojiResponse,
  /**
   * @param {!proto.blog.RemoveEmojiRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.blog.RemoveEmojiResponse.deserializeBinary
);


/**
 * @param {!proto.blog.RemoveEmojiRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.blog.RemoveEmojiResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.blog.RemoveEmojiResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.blog.BlogServiceClient.prototype.removeEmoji =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/blog.BlogService/RemoveEmoji',
      request,
      metadata || {},
      methodDescriptor_BlogService_RemoveEmoji,
      callback);
};


/**
 * @param {!proto.blog.RemoveEmojiRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.blog.RemoveEmojiResponse>}
 *     Promise that resolves to the response
 */
proto.blog.BlogServicePromiseClient.prototype.removeEmoji =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/blog.BlogService/RemoveEmoji',
      request,
      metadata || {},
      methodDescriptor_BlogService_RemoveEmoji);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.blog.AddCommentRequest,
 *   !proto.blog.AddCommentResponse>}
 */
const methodDescriptor_BlogService_AddComment = new grpc.web.MethodDescriptor(
  '/blog.BlogService/AddComment',
  grpc.web.MethodType.UNARY,
  proto.blog.AddCommentRequest,
  proto.blog.AddCommentResponse,
  /**
   * @param {!proto.blog.AddCommentRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.blog.AddCommentResponse.deserializeBinary
);


/**
 * @param {!proto.blog.AddCommentRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.blog.AddCommentResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.blog.AddCommentResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.blog.BlogServiceClient.prototype.addComment =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/blog.BlogService/AddComment',
      request,
      metadata || {},
      methodDescriptor_BlogService_AddComment,
      callback);
};


/**
 * @param {!proto.blog.AddCommentRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.blog.AddCommentResponse>}
 *     Promise that resolves to the response
 */
proto.blog.BlogServicePromiseClient.prototype.addComment =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/blog.BlogService/AddComment',
      request,
      metadata || {},
      methodDescriptor_BlogService_AddComment);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.blog.RemoveCommentRequest,
 *   !proto.blog.RemoveCommentResponse>}
 */
const methodDescriptor_BlogService_RemoveComment = new grpc.web.MethodDescriptor(
  '/blog.BlogService/RemoveComment',
  grpc.web.MethodType.UNARY,
  proto.blog.RemoveCommentRequest,
  proto.blog.RemoveCommentResponse,
  /**
   * @param {!proto.blog.RemoveCommentRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.blog.RemoveCommentResponse.deserializeBinary
);


/**
 * @param {!proto.blog.RemoveCommentRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.blog.RemoveCommentResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.blog.RemoveCommentResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.blog.BlogServiceClient.prototype.removeComment =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/blog.BlogService/RemoveComment',
      request,
      metadata || {},
      methodDescriptor_BlogService_RemoveComment,
      callback);
};


/**
 * @param {!proto.blog.RemoveCommentRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.blog.RemoveCommentResponse>}
 *     Promise that resolves to the response
 */
proto.blog.BlogServicePromiseClient.prototype.removeComment =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/blog.BlogService/RemoveComment',
      request,
      metadata || {},
      methodDescriptor_BlogService_RemoveComment);
};


module.exports = proto.blog;

