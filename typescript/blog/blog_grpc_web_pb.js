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
 *   !proto.blog.GetBlogPostsByAuthorRequest,
 *   !proto.blog.GetBlogPostsByAuthorResponse>}
 */
const methodDescriptor_BlogService_GetBlogPostsByAuthor = new grpc.web.MethodDescriptor(
  '/blog.BlogService/GetBlogPostsByAuthor',
  grpc.web.MethodType.UNARY,
  proto.blog.GetBlogPostsByAuthorRequest,
  proto.blog.GetBlogPostsByAuthorResponse,
  /**
   * @param {!proto.blog.GetBlogPostsByAuthorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.blog.GetBlogPostsByAuthorResponse.deserializeBinary
);


/**
 * @param {!proto.blog.GetBlogPostsByAuthorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.blog.GetBlogPostsByAuthorResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.blog.GetBlogPostsByAuthorResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.blog.BlogServiceClient.prototype.getBlogPostsByAuthor =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/blog.BlogService/GetBlogPostsByAuthor',
      request,
      metadata || {},
      methodDescriptor_BlogService_GetBlogPostsByAuthor,
      callback);
};


/**
 * @param {!proto.blog.GetBlogPostsByAuthorRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.blog.GetBlogPostsByAuthorResponse>}
 *     Promise that resolves to the response
 */
proto.blog.BlogServicePromiseClient.prototype.getBlogPostsByAuthor =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/blog.BlogService/GetBlogPostsByAuthor',
      request,
      metadata || {},
      methodDescriptor_BlogService_GetBlogPostsByAuthor);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.blog.SearchBlogsPostRequest,
 *   !proto.blog.SearchBlogsPostResponse>}
 */
const methodDescriptor_BlogService_SearchBlogPosts = new grpc.web.MethodDescriptor(
  '/blog.BlogService/SearchBlogPosts',
  grpc.web.MethodType.UNARY,
  proto.blog.SearchBlogsPostRequest,
  proto.blog.SearchBlogsPostResponse,
  /**
   * @param {!proto.blog.SearchBlogsPostRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.blog.SearchBlogsPostResponse.deserializeBinary
);


/**
 * @param {!proto.blog.SearchBlogsPostRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.blog.SearchBlogsPostResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.blog.SearchBlogsPostResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.blog.BlogServiceClient.prototype.searchBlogPosts =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/blog.BlogService/SearchBlogPosts',
      request,
      metadata || {},
      methodDescriptor_BlogService_SearchBlogPosts,
      callback);
};


/**
 * @param {!proto.blog.SearchBlogsPostRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.blog.SearchBlogsPostResponse>}
 *     Promise that resolves to the response
 */
proto.blog.BlogServicePromiseClient.prototype.searchBlogPosts =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
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
 *   !proto.blog.AddLikeRequest,
 *   !proto.blog.AddLikeResponse>}
 */
const methodDescriptor_BlogService_AddLike = new grpc.web.MethodDescriptor(
  '/blog.BlogService/AddLike',
  grpc.web.MethodType.UNARY,
  proto.blog.AddLikeRequest,
  proto.blog.AddLikeResponse,
  /**
   * @param {!proto.blog.AddLikeRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.blog.AddLikeResponse.deserializeBinary
);


/**
 * @param {!proto.blog.AddLikeRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.blog.AddLikeResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.blog.AddLikeResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.blog.BlogServiceClient.prototype.addLike =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/blog.BlogService/AddLike',
      request,
      metadata || {},
      methodDescriptor_BlogService_AddLike,
      callback);
};


/**
 * @param {!proto.blog.AddLikeRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.blog.AddLikeResponse>}
 *     Promise that resolves to the response
 */
proto.blog.BlogServicePromiseClient.prototype.addLike =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/blog.BlogService/AddLike',
      request,
      metadata || {},
      methodDescriptor_BlogService_AddLike);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.blog.RemoveLikeRequest,
 *   !proto.blog.RemoveLikeResponse>}
 */
const methodDescriptor_BlogService_RemoveLike = new grpc.web.MethodDescriptor(
  '/blog.BlogService/RemoveLike',
  grpc.web.MethodType.UNARY,
  proto.blog.RemoveLikeRequest,
  proto.blog.RemoveLikeResponse,
  /**
   * @param {!proto.blog.RemoveLikeRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.blog.RemoveLikeResponse.deserializeBinary
);


/**
 * @param {!proto.blog.RemoveLikeRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.blog.RemoveLikeResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.blog.RemoveLikeResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.blog.BlogServiceClient.prototype.removeLike =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/blog.BlogService/RemoveLike',
      request,
      metadata || {},
      methodDescriptor_BlogService_RemoveLike,
      callback);
};


/**
 * @param {!proto.blog.RemoveLikeRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.blog.RemoveLikeResponse>}
 *     Promise that resolves to the response
 */
proto.blog.BlogServicePromiseClient.prototype.removeLike =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/blog.BlogService/RemoveLike',
      request,
      metadata || {},
      methodDescriptor_BlogService_RemoveLike);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.blog.AddLikeRequest,
 *   !proto.blog.AddLikeResponse>}
 */
const methodDescriptor_BlogService_AddDislike = new grpc.web.MethodDescriptor(
  '/blog.BlogService/AddDislike',
  grpc.web.MethodType.UNARY,
  proto.blog.AddLikeRequest,
  proto.blog.AddLikeResponse,
  /**
   * @param {!proto.blog.AddLikeRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.blog.AddLikeResponse.deserializeBinary
);


/**
 * @param {!proto.blog.AddLikeRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.blog.AddLikeResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.blog.AddLikeResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.blog.BlogServiceClient.prototype.addDislike =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/blog.BlogService/AddDislike',
      request,
      metadata || {},
      methodDescriptor_BlogService_AddDislike,
      callback);
};


/**
 * @param {!proto.blog.AddLikeRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.blog.AddLikeResponse>}
 *     Promise that resolves to the response
 */
proto.blog.BlogServicePromiseClient.prototype.addDislike =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/blog.BlogService/AddDislike',
      request,
      metadata || {},
      methodDescriptor_BlogService_AddDislike);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.blog.RemoveDislikeRequest,
 *   !proto.blog.RemoveDislikeResponse>}
 */
const methodDescriptor_BlogService_RemoveDislike = new grpc.web.MethodDescriptor(
  '/blog.BlogService/RemoveDislike',
  grpc.web.MethodType.UNARY,
  proto.blog.RemoveDislikeRequest,
  proto.blog.RemoveDislikeResponse,
  /**
   * @param {!proto.blog.RemoveDislikeRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.blog.RemoveDislikeResponse.deserializeBinary
);


/**
 * @param {!proto.blog.RemoveDislikeRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.blog.RemoveDislikeResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.blog.RemoveDislikeResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.blog.BlogServiceClient.prototype.removeDislike =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/blog.BlogService/RemoveDislike',
      request,
      metadata || {},
      methodDescriptor_BlogService_RemoveDislike,
      callback);
};


/**
 * @param {!proto.blog.RemoveDislikeRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.blog.RemoveDislikeResponse>}
 *     Promise that resolves to the response
 */
proto.blog.BlogServicePromiseClient.prototype.removeDislike =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/blog.BlogService/RemoveDislike',
      request,
      metadata || {},
      methodDescriptor_BlogService_RemoveDislike);
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

