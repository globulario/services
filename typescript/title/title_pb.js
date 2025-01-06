// source: title.proto
/**
 * @fileoverview
 * @enhanceable
 * @suppress {messageConventions} JS Compiler reports an error if a variable or
 *     field starts with 'MSG_' and isn't a translatable message.
 * @public
 */
// GENERATED CODE -- DO NOT EDIT!

var jspb = require('google-protobuf');
var goog = jspb;
var global = Function('return this')();

goog.exportSymbol('proto.title.Album', null, global);
goog.exportSymbol('proto.title.AssociateFileWithTitleRequest', null, global);
goog.exportSymbol('proto.title.AssociateFileWithTitleResponse', null, global);
goog.exportSymbol('proto.title.Audio', null, global);
goog.exportSymbol('proto.title.Audios', null, global);
goog.exportSymbol('proto.title.CreateAudioRequest', null, global);
goog.exportSymbol('proto.title.CreateAudioResponse', null, global);
goog.exportSymbol('proto.title.CreatePersonRequest', null, global);
goog.exportSymbol('proto.title.CreatePersonResponse', null, global);
goog.exportSymbol('proto.title.CreatePublisherRequest', null, global);
goog.exportSymbol('proto.title.CreatePublisherResponse', null, global);
goog.exportSymbol('proto.title.CreateTitleRequest', null, global);
goog.exportSymbol('proto.title.CreateTitleResponse', null, global);
goog.exportSymbol('proto.title.CreateVideoRequest', null, global);
goog.exportSymbol('proto.title.CreateVideoResponse', null, global);
goog.exportSymbol('proto.title.DeleteAlbumRequest', null, global);
goog.exportSymbol('proto.title.DeleteAlbumResponse', null, global);
goog.exportSymbol('proto.title.DeleteAudioRequest', null, global);
goog.exportSymbol('proto.title.DeleteAudioResponse', null, global);
goog.exportSymbol('proto.title.DeletePersonRequest', null, global);
goog.exportSymbol('proto.title.DeletePersonResponse', null, global);
goog.exportSymbol('proto.title.DeletePublisherRequest', null, global);
goog.exportSymbol('proto.title.DeletePublisherResponse', null, global);
goog.exportSymbol('proto.title.DeleteTitleRequest', null, global);
goog.exportSymbol('proto.title.DeleteTitleResponse', null, global);
goog.exportSymbol('proto.title.DeleteVideoRequest', null, global);
goog.exportSymbol('proto.title.DeleteVideoResponse', null, global);
goog.exportSymbol('proto.title.DissociateFileWithTitleRequest', null, global);
goog.exportSymbol('proto.title.DissociateFileWithTitleResponse', null, global);
goog.exportSymbol('proto.title.GetAlbumRequest', null, global);
goog.exportSymbol('proto.title.GetAlbumResponse', null, global);
goog.exportSymbol('proto.title.GetAudioByIdRequest', null, global);
goog.exportSymbol('proto.title.GetAudioByIdResponse', null, global);
goog.exportSymbol('proto.title.GetFileAudiosRequest', null, global);
goog.exportSymbol('proto.title.GetFileAudiosResponse', null, global);
goog.exportSymbol('proto.title.GetFileTitlesRequest', null, global);
goog.exportSymbol('proto.title.GetFileTitlesResponse', null, global);
goog.exportSymbol('proto.title.GetFileVideosRequest', null, global);
goog.exportSymbol('proto.title.GetFileVideosResponse', null, global);
goog.exportSymbol('proto.title.GetPersonByIdRequest', null, global);
goog.exportSymbol('proto.title.GetPersonByIdResponse', null, global);
goog.exportSymbol('proto.title.GetPublisherByIdRequest', null, global);
goog.exportSymbol('proto.title.GetPublisherByIdResponse', null, global);
goog.exportSymbol('proto.title.GetTitleByIdRequest', null, global);
goog.exportSymbol('proto.title.GetTitleByIdResponse', null, global);
goog.exportSymbol('proto.title.GetTitleFilesRequest', null, global);
goog.exportSymbol('proto.title.GetTitleFilesResponse', null, global);
goog.exportSymbol('proto.title.GetVideoByIdRequest', null, global);
goog.exportSymbol('proto.title.GetVideoByIdResponse', null, global);
goog.exportSymbol('proto.title.Person', null, global);
goog.exportSymbol('proto.title.Poster', null, global);
goog.exportSymbol('proto.title.Preview', null, global);
goog.exportSymbol('proto.title.Publisher', null, global);
goog.exportSymbol('proto.title.SearchFacet', null, global);
goog.exportSymbol('proto.title.SearchFacetTerm', null, global);
goog.exportSymbol('proto.title.SearchFacets', null, global);
goog.exportSymbol('proto.title.SearchHit', null, global);
goog.exportSymbol('proto.title.SearchHit.ResultCase', null, global);
goog.exportSymbol('proto.title.SearchPersonsRequest', null, global);
goog.exportSymbol('proto.title.SearchPersonsResponse', null, global);
goog.exportSymbol('proto.title.SearchPersonsResponse.ResultCase', null, global);
goog.exportSymbol('proto.title.SearchSummary', null, global);
goog.exportSymbol('proto.title.SearchTitlesRequest', null, global);
goog.exportSymbol('proto.title.SearchTitlesResponse', null, global);
goog.exportSymbol('proto.title.SearchTitlesResponse.ResultCase', null, global);
goog.exportSymbol('proto.title.Snippet', null, global);
goog.exportSymbol('proto.title.Title', null, global);
goog.exportSymbol('proto.title.Titles', null, global);
goog.exportSymbol('proto.title.UpdateTitleMetadataRequest', null, global);
goog.exportSymbol('proto.title.UpdateTitleMetadataResponse', null, global);
goog.exportSymbol('proto.title.UpdateVideoMetadataRequest', null, global);
goog.exportSymbol('proto.title.UpdateVideoMetadataResponse', null, global);
goog.exportSymbol('proto.title.Video', null, global);
goog.exportSymbol('proto.title.Videos', null, global);
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.Person = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.title.Person.repeatedFields_, null);
};
goog.inherits(proto.title.Person, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.Person.displayName = 'proto.title.Person';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.Poster = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.Poster, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.Poster.displayName = 'proto.title.Poster';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.Preview = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.Preview, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.Preview.displayName = 'proto.title.Preview';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.Publisher = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.Publisher, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.Publisher.displayName = 'proto.title.Publisher';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.Video = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.title.Video.repeatedFields_, null);
};
goog.inherits(proto.title.Video, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.Video.displayName = 'proto.title.Video';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.Videos = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.title.Videos.repeatedFields_, null);
};
goog.inherits(proto.title.Videos, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.Videos.displayName = 'proto.title.Videos';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.CreateVideoRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.CreateVideoRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.CreateVideoRequest.displayName = 'proto.title.CreateVideoRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.CreateVideoResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.CreateVideoResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.CreateVideoResponse.displayName = 'proto.title.CreateVideoResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.GetVideoByIdRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.GetVideoByIdRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.GetVideoByIdRequest.displayName = 'proto.title.GetVideoByIdRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.GetVideoByIdResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.title.GetVideoByIdResponse.repeatedFields_, null);
};
goog.inherits(proto.title.GetVideoByIdResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.GetVideoByIdResponse.displayName = 'proto.title.GetVideoByIdResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.DeleteVideoRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.DeleteVideoRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.DeleteVideoRequest.displayName = 'proto.title.DeleteVideoRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.DeleteVideoResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.DeleteVideoResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.DeleteVideoResponse.displayName = 'proto.title.DeleteVideoResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.UpdateVideoMetadataRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.UpdateVideoMetadataRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.UpdateVideoMetadataRequest.displayName = 'proto.title.UpdateVideoMetadataRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.UpdateVideoMetadataResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.UpdateVideoMetadataResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.UpdateVideoMetadataResponse.displayName = 'proto.title.UpdateVideoMetadataResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.Title = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.title.Title.repeatedFields_, null);
};
goog.inherits(proto.title.Title, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.Title.displayName = 'proto.title.Title';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.Titles = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.title.Titles.repeatedFields_, null);
};
goog.inherits(proto.title.Titles, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.Titles.displayName = 'proto.title.Titles';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.CreateTitleRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.CreateTitleRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.CreateTitleRequest.displayName = 'proto.title.CreateTitleRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.CreateTitleResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.CreateTitleResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.CreateTitleResponse.displayName = 'proto.title.CreateTitleResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.GetTitleByIdRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.GetTitleByIdRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.GetTitleByIdRequest.displayName = 'proto.title.GetTitleByIdRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.GetTitleByIdResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.title.GetTitleByIdResponse.repeatedFields_, null);
};
goog.inherits(proto.title.GetTitleByIdResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.GetTitleByIdResponse.displayName = 'proto.title.GetTitleByIdResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.DeleteTitleRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.DeleteTitleRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.DeleteTitleRequest.displayName = 'proto.title.DeleteTitleRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.DeleteTitleResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.DeleteTitleResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.DeleteTitleResponse.displayName = 'proto.title.DeleteTitleResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.UpdateTitleMetadataRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.UpdateTitleMetadataRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.UpdateTitleMetadataRequest.displayName = 'proto.title.UpdateTitleMetadataRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.UpdateTitleMetadataResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.UpdateTitleMetadataResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.UpdateTitleMetadataResponse.displayName = 'proto.title.UpdateTitleMetadataResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.AssociateFileWithTitleRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.AssociateFileWithTitleRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.AssociateFileWithTitleRequest.displayName = 'proto.title.AssociateFileWithTitleRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.AssociateFileWithTitleResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.AssociateFileWithTitleResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.AssociateFileWithTitleResponse.displayName = 'proto.title.AssociateFileWithTitleResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.DissociateFileWithTitleRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.DissociateFileWithTitleRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.DissociateFileWithTitleRequest.displayName = 'proto.title.DissociateFileWithTitleRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.DissociateFileWithTitleResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.DissociateFileWithTitleResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.DissociateFileWithTitleResponse.displayName = 'proto.title.DissociateFileWithTitleResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.GetFileTitlesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.GetFileTitlesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.GetFileTitlesRequest.displayName = 'proto.title.GetFileTitlesRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.GetFileTitlesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.GetFileTitlesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.GetFileTitlesResponse.displayName = 'proto.title.GetFileTitlesResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.GetFileVideosRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.GetFileVideosRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.GetFileVideosRequest.displayName = 'proto.title.GetFileVideosRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.GetFileVideosResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.GetFileVideosResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.GetFileVideosResponse.displayName = 'proto.title.GetFileVideosResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.GetTitleFilesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.GetTitleFilesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.GetTitleFilesRequest.displayName = 'proto.title.GetTitleFilesRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.GetTitleFilesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.title.GetTitleFilesResponse.repeatedFields_, null);
};
goog.inherits(proto.title.GetTitleFilesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.GetTitleFilesResponse.displayName = 'proto.title.GetTitleFilesResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.Snippet = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.title.Snippet.repeatedFields_, null);
};
goog.inherits(proto.title.Snippet, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.Snippet.displayName = 'proto.title.Snippet';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.SearchHit = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.title.SearchHit.repeatedFields_, proto.title.SearchHit.oneofGroups_);
};
goog.inherits(proto.title.SearchHit, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.SearchHit.displayName = 'proto.title.SearchHit';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.SearchSummary = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.SearchSummary, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.SearchSummary.displayName = 'proto.title.SearchSummary';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.SearchFacetTerm = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.SearchFacetTerm, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.SearchFacetTerm.displayName = 'proto.title.SearchFacetTerm';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.SearchFacet = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.title.SearchFacet.repeatedFields_, null);
};
goog.inherits(proto.title.SearchFacet, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.SearchFacet.displayName = 'proto.title.SearchFacet';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.SearchFacets = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.title.SearchFacets.repeatedFields_, null);
};
goog.inherits(proto.title.SearchFacets, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.SearchFacets.displayName = 'proto.title.SearchFacets';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.SearchTitlesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.title.SearchTitlesRequest.repeatedFields_, null);
};
goog.inherits(proto.title.SearchTitlesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.SearchTitlesRequest.displayName = 'proto.title.SearchTitlesRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.SearchTitlesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.title.SearchTitlesResponse.oneofGroups_);
};
goog.inherits(proto.title.SearchTitlesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.SearchTitlesResponse.displayName = 'proto.title.SearchTitlesResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.SearchPersonsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.title.SearchPersonsRequest.repeatedFields_, null);
};
goog.inherits(proto.title.SearchPersonsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.SearchPersonsRequest.displayName = 'proto.title.SearchPersonsRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.SearchPersonsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.title.SearchPersonsResponse.oneofGroups_);
};
goog.inherits(proto.title.SearchPersonsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.SearchPersonsResponse.displayName = 'proto.title.SearchPersonsResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.CreatePublisherRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.CreatePublisherRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.CreatePublisherRequest.displayName = 'proto.title.CreatePublisherRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.CreatePublisherResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.CreatePublisherResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.CreatePublisherResponse.displayName = 'proto.title.CreatePublisherResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.DeletePublisherRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.DeletePublisherRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.DeletePublisherRequest.displayName = 'proto.title.DeletePublisherRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.DeletePublisherResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.DeletePublisherResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.DeletePublisherResponse.displayName = 'proto.title.DeletePublisherResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.GetPublisherByIdRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.GetPublisherByIdRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.GetPublisherByIdRequest.displayName = 'proto.title.GetPublisherByIdRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.GetPublisherByIdResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.GetPublisherByIdResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.GetPublisherByIdResponse.displayName = 'proto.title.GetPublisherByIdResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.CreatePersonRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.CreatePersonRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.CreatePersonRequest.displayName = 'proto.title.CreatePersonRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.CreatePersonResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.CreatePersonResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.CreatePersonResponse.displayName = 'proto.title.CreatePersonResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.DeletePersonRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.DeletePersonRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.DeletePersonRequest.displayName = 'proto.title.DeletePersonRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.DeletePersonResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.DeletePersonResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.DeletePersonResponse.displayName = 'proto.title.DeletePersonResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.GetPersonByIdRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.GetPersonByIdRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.GetPersonByIdRequest.displayName = 'proto.title.GetPersonByIdRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.GetPersonByIdResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.GetPersonByIdResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.GetPersonByIdResponse.displayName = 'proto.title.GetPersonByIdResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.Audio = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.title.Audio.repeatedFields_, null);
};
goog.inherits(proto.title.Audio, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.Audio.displayName = 'proto.title.Audio';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.Album = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.title.Album.repeatedFields_, null);
};
goog.inherits(proto.title.Album, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.Album.displayName = 'proto.title.Album';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.Audios = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.title.Audios.repeatedFields_, null);
};
goog.inherits(proto.title.Audios, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.Audios.displayName = 'proto.title.Audios';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.CreateAudioRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.CreateAudioRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.CreateAudioRequest.displayName = 'proto.title.CreateAudioRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.CreateAudioResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.CreateAudioResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.CreateAudioResponse.displayName = 'proto.title.CreateAudioResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.GetAudioByIdRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.GetAudioByIdRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.GetAudioByIdRequest.displayName = 'proto.title.GetAudioByIdRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.GetAudioByIdResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.title.GetAudioByIdResponse.repeatedFields_, null);
};
goog.inherits(proto.title.GetAudioByIdResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.GetAudioByIdResponse.displayName = 'proto.title.GetAudioByIdResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.DeleteAudioRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.DeleteAudioRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.DeleteAudioRequest.displayName = 'proto.title.DeleteAudioRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.DeleteAudioResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.DeleteAudioResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.DeleteAudioResponse.displayName = 'proto.title.DeleteAudioResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.GetFileAudiosRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.GetFileAudiosRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.GetFileAudiosRequest.displayName = 'proto.title.GetFileAudiosRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.GetFileAudiosResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.GetFileAudiosResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.GetFileAudiosResponse.displayName = 'proto.title.GetFileAudiosResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.GetAlbumRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.GetAlbumRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.GetAlbumRequest.displayName = 'proto.title.GetAlbumRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.GetAlbumResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.GetAlbumResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.GetAlbumResponse.displayName = 'proto.title.GetAlbumResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.DeleteAlbumRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.DeleteAlbumRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.DeleteAlbumRequest.displayName = 'proto.title.DeleteAlbumRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.title.DeleteAlbumResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.title.DeleteAlbumResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.title.DeleteAlbumResponse.displayName = 'proto.title.DeleteAlbumResponse';
}

/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.title.Person.repeatedFields_ = [4,11,12,13,14];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.Person.prototype.toObject = function(opt_includeInstance) {
  return proto.title.Person.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.Person} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Person.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, ""),
    url: jspb.Message.getFieldWithDefault(msg, 2, ""),
    fullname: jspb.Message.getFieldWithDefault(msg, 3, ""),
    aliasesList: (f = jspb.Message.getRepeatedField(msg, 4)) == null ? undefined : f,
    picture: jspb.Message.getFieldWithDefault(msg, 5, ""),
    biography: jspb.Message.getFieldWithDefault(msg, 6, ""),
    careerstatus: jspb.Message.getFieldWithDefault(msg, 7, ""),
    gender: jspb.Message.getFieldWithDefault(msg, 8, ""),
    birthplace: jspb.Message.getFieldWithDefault(msg, 9, ""),
    birthdate: jspb.Message.getFieldWithDefault(msg, 10, ""),
    directingList: (f = jspb.Message.getRepeatedField(msg, 11)) == null ? undefined : f,
    writingList: (f = jspb.Message.getRepeatedField(msg, 12)) == null ? undefined : f,
    actingList: (f = jspb.Message.getRepeatedField(msg, 13)) == null ? undefined : f,
    castingList: (f = jspb.Message.getRepeatedField(msg, 14)) == null ? undefined : f
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.Person}
 */
proto.title.Person.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.Person;
  return proto.title.Person.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.Person} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.Person}
 */
proto.title.Person.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setUrl(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setFullname(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.addAliases(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setPicture(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setBiography(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setCareerstatus(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setGender(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setBirthplace(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.setBirthdate(value);
      break;
    case 11:
      var value = /** @type {string} */ (reader.readString());
      msg.addDirecting(value);
      break;
    case 12:
      var value = /** @type {string} */ (reader.readString());
      msg.addWriting(value);
      break;
    case 13:
      var value = /** @type {string} */ (reader.readString());
      msg.addActing(value);
      break;
    case 14:
      var value = /** @type {string} */ (reader.readString());
      msg.addCasting(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.Person.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.Person.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.Person} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Person.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getUrl();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getFullname();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getAliasesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      4,
      f
    );
  }
  f = message.getPicture();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getBiography();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getCareerstatus();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getGender();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
  f = message.getBirthplace();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
  f = message.getBirthdate();
  if (f.length > 0) {
    writer.writeString(
      10,
      f
    );
  }
  f = message.getDirectingList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      11,
      f
    );
  }
  f = message.getWritingList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      12,
      f
    );
  }
  f = message.getActingList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      13,
      f
    );
  }
  f = message.getCastingList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      14,
      f
    );
  }
};


/**
 * optional string ID = 1;
 * @return {string}
 */
proto.title.Person.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string URL = 2;
 * @return {string}
 */
proto.title.Person.prototype.getUrl = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.setUrl = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string FullName = 3;
 * @return {string}
 */
proto.title.Person.prototype.getFullname = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.setFullname = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * repeated string Aliases = 4;
 * @return {!Array<string>}
 */
proto.title.Person.prototype.getAliasesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 4));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.setAliasesList = function(value) {
  return jspb.Message.setField(this, 4, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.addAliases = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 4, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.clearAliasesList = function() {
  return this.setAliasesList([]);
};


/**
 * optional string Picture = 5;
 * @return {string}
 */
proto.title.Person.prototype.getPicture = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.setPicture = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string Biography = 6;
 * @return {string}
 */
proto.title.Person.prototype.getBiography = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.setBiography = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string CareerStatus = 7;
 * @return {string}
 */
proto.title.Person.prototype.getCareerstatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.setCareerstatus = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * optional string Gender = 8;
 * @return {string}
 */
proto.title.Person.prototype.getGender = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.setGender = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
};


/**
 * optional string BirthPlace = 9;
 * @return {string}
 */
proto.title.Person.prototype.getBirthplace = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.setBirthplace = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};


/**
 * optional string BirthDate = 10;
 * @return {string}
 */
proto.title.Person.prototype.getBirthdate = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 10, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.setBirthdate = function(value) {
  return jspb.Message.setProto3StringField(this, 10, value);
};


/**
 * repeated string Directing = 11;
 * @return {!Array<string>}
 */
proto.title.Person.prototype.getDirectingList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 11));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.setDirectingList = function(value) {
  return jspb.Message.setField(this, 11, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.addDirecting = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 11, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.clearDirectingList = function() {
  return this.setDirectingList([]);
};


/**
 * repeated string Writing = 12;
 * @return {!Array<string>}
 */
proto.title.Person.prototype.getWritingList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 12));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.setWritingList = function(value) {
  return jspb.Message.setField(this, 12, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.addWriting = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 12, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.clearWritingList = function() {
  return this.setWritingList([]);
};


/**
 * repeated string Acting = 13;
 * @return {!Array<string>}
 */
proto.title.Person.prototype.getActingList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 13));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.setActingList = function(value) {
  return jspb.Message.setField(this, 13, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.addActing = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 13, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.clearActingList = function() {
  return this.setActingList([]);
};


/**
 * repeated string Casting = 14;
 * @return {!Array<string>}
 */
proto.title.Person.prototype.getCastingList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 14));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.setCastingList = function(value) {
  return jspb.Message.setField(this, 14, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.addCasting = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 14, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.Person} returns this
 */
proto.title.Person.prototype.clearCastingList = function() {
  return this.setCastingList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.Poster.prototype.toObject = function(opt_includeInstance) {
  return proto.title.Poster.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.Poster} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Poster.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, ""),
    titleid: jspb.Message.getFieldWithDefault(msg, 2, ""),
    url: jspb.Message.getFieldWithDefault(msg, 3, ""),
    contenturl: jspb.Message.getFieldWithDefault(msg, 4, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.Poster}
 */
proto.title.Poster.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.Poster;
  return proto.title.Poster.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.Poster} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.Poster}
 */
proto.title.Poster.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setTitleid(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setUrl(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setContenturl(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.Poster.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.Poster.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.Poster} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Poster.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getTitleid();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getUrl();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getContenturl();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * optional string ID = 1;
 * @return {string}
 */
proto.title.Poster.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Poster} returns this
 */
proto.title.Poster.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string titleId = 2;
 * @return {string}
 */
proto.title.Poster.prototype.getTitleid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Poster} returns this
 */
proto.title.Poster.prototype.setTitleid = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string URL = 3;
 * @return {string}
 */
proto.title.Poster.prototype.getUrl = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Poster} returns this
 */
proto.title.Poster.prototype.setUrl = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string ContentUrl = 4;
 * @return {string}
 */
proto.title.Poster.prototype.getContenturl = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Poster} returns this
 */
proto.title.Poster.prototype.setContenturl = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.Preview.prototype.toObject = function(opt_includeInstance) {
  return proto.title.Preview.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.Preview} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Preview.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, ""),
    titleid: jspb.Message.getFieldWithDefault(msg, 2, ""),
    url: jspb.Message.getFieldWithDefault(msg, 3, ""),
    contenturl: jspb.Message.getFieldWithDefault(msg, 4, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.Preview}
 */
proto.title.Preview.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.Preview;
  return proto.title.Preview.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.Preview} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.Preview}
 */
proto.title.Preview.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setTitleid(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setUrl(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setContenturl(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.Preview.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.Preview.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.Preview} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Preview.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getTitleid();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getUrl();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getContenturl();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * optional string ID = 1;
 * @return {string}
 */
proto.title.Preview.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Preview} returns this
 */
proto.title.Preview.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string titleId = 2;
 * @return {string}
 */
proto.title.Preview.prototype.getTitleid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Preview} returns this
 */
proto.title.Preview.prototype.setTitleid = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string URL = 3;
 * @return {string}
 */
proto.title.Preview.prototype.getUrl = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Preview} returns this
 */
proto.title.Preview.prototype.setUrl = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string ContentUrl = 4;
 * @return {string}
 */
proto.title.Preview.prototype.getContenturl = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Preview} returns this
 */
proto.title.Preview.prototype.setContenturl = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.Publisher.prototype.toObject = function(opt_includeInstance) {
  return proto.title.Publisher.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.Publisher} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Publisher.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, ""),
    url: jspb.Message.getFieldWithDefault(msg, 2, ""),
    name: jspb.Message.getFieldWithDefault(msg, 3, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.Publisher}
 */
proto.title.Publisher.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.Publisher;
  return proto.title.Publisher.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.Publisher} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.Publisher}
 */
proto.title.Publisher.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setUrl(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.Publisher.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.Publisher.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.Publisher} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Publisher.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getUrl();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string ID = 1;
 * @return {string}
 */
proto.title.Publisher.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Publisher} returns this
 */
proto.title.Publisher.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string URL = 2;
 * @return {string}
 */
proto.title.Publisher.prototype.getUrl = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Publisher} returns this
 */
proto.title.Publisher.prototype.setUrl = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string Name = 3;
 * @return {string}
 */
proto.title.Publisher.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Publisher} returns this
 */
proto.title.Publisher.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.title.Video.repeatedFields_ = [10,11,12];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.Video.prototype.toObject = function(opt_includeInstance) {
  return proto.title.Video.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.Video} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Video.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, ""),
    url: jspb.Message.getFieldWithDefault(msg, 2, ""),
    title: jspb.Message.getFieldWithDefault(msg, 3, ""),
    description: jspb.Message.getFieldWithDefault(msg, 4, ""),
    publisherid: (f = msg.getPublisherid()) && proto.title.Publisher.toObject(includeInstance, f),
    count: jspb.Message.getFieldWithDefault(msg, 6, 0),
    rating: jspb.Message.getFloatingPointFieldWithDefault(msg, 7, 0.0),
    likes: jspb.Message.getFieldWithDefault(msg, 8, 0),
    date: jspb.Message.getFieldWithDefault(msg, 9, ""),
    genresList: (f = jspb.Message.getRepeatedField(msg, 10)) == null ? undefined : f,
    tagsList: (f = jspb.Message.getRepeatedField(msg, 11)) == null ? undefined : f,
    castingList: jspb.Message.toObjectList(msg.getCastingList(),
    proto.title.Person.toObject, includeInstance),
    poster: (f = msg.getPoster()) && proto.title.Poster.toObject(includeInstance, f),
    preview: (f = msg.getPreview()) && proto.title.Preview.toObject(includeInstance, f),
    duration: jspb.Message.getFieldWithDefault(msg, 15, 0),
    uuid: jspb.Message.getFieldWithDefault(msg, 16, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.Video}
 */
proto.title.Video.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.Video;
  return proto.title.Video.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.Video} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.Video}
 */
proto.title.Video.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setUrl(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setTitle(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setDescription(value);
      break;
    case 5:
      var value = new proto.title.Publisher;
      reader.readMessage(value,proto.title.Publisher.deserializeBinaryFromReader);
      msg.setPublisherid(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setCount(value);
      break;
    case 7:
      var value = /** @type {number} */ (reader.readFloat());
      msg.setRating(value);
      break;
    case 8:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setLikes(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setDate(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.addGenres(value);
      break;
    case 11:
      var value = /** @type {string} */ (reader.readString());
      msg.addTags(value);
      break;
    case 12:
      var value = new proto.title.Person;
      reader.readMessage(value,proto.title.Person.deserializeBinaryFromReader);
      msg.addCasting(value);
      break;
    case 13:
      var value = new proto.title.Poster;
      reader.readMessage(value,proto.title.Poster.deserializeBinaryFromReader);
      msg.setPoster(value);
      break;
    case 14:
      var value = new proto.title.Preview;
      reader.readMessage(value,proto.title.Preview.deserializeBinaryFromReader);
      msg.setPreview(value);
      break;
    case 15:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setDuration(value);
      break;
    case 16:
      var value = /** @type {string} */ (reader.readString());
      msg.setUuid(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.Video.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.Video.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.Video} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Video.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getUrl();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getTitle();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getDescription();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getPublisherid();
  if (f != null) {
    writer.writeMessage(
      5,
      f,
      proto.title.Publisher.serializeBinaryToWriter
    );
  }
  f = message.getCount();
  if (f !== 0) {
    writer.writeInt64(
      6,
      f
    );
  }
  f = message.getRating();
  if (f !== 0.0) {
    writer.writeFloat(
      7,
      f
    );
  }
  f = message.getLikes();
  if (f !== 0) {
    writer.writeInt64(
      8,
      f
    );
  }
  f = message.getDate();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
  f = message.getGenresList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      10,
      f
    );
  }
  f = message.getTagsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      11,
      f
    );
  }
  f = message.getCastingList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      12,
      f,
      proto.title.Person.serializeBinaryToWriter
    );
  }
  f = message.getPoster();
  if (f != null) {
    writer.writeMessage(
      13,
      f,
      proto.title.Poster.serializeBinaryToWriter
    );
  }
  f = message.getPreview();
  if (f != null) {
    writer.writeMessage(
      14,
      f,
      proto.title.Preview.serializeBinaryToWriter
    );
  }
  f = message.getDuration();
  if (f !== 0) {
    writer.writeInt32(
      15,
      f
    );
  }
  f = message.getUuid();
  if (f.length > 0) {
    writer.writeString(
      16,
      f
    );
  }
};


/**
 * optional string ID = 1;
 * @return {string}
 */
proto.title.Video.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Video} returns this
 */
proto.title.Video.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string URL = 2;
 * @return {string}
 */
proto.title.Video.prototype.getUrl = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Video} returns this
 */
proto.title.Video.prototype.setUrl = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string Title = 3;
 * @return {string}
 */
proto.title.Video.prototype.getTitle = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Video} returns this
 */
proto.title.Video.prototype.setTitle = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string Description = 4;
 * @return {string}
 */
proto.title.Video.prototype.getDescription = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Video} returns this
 */
proto.title.Video.prototype.setDescription = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional Publisher PublisherId = 5;
 * @return {?proto.title.Publisher}
 */
proto.title.Video.prototype.getPublisherid = function() {
  return /** @type{?proto.title.Publisher} */ (
    jspb.Message.getWrapperField(this, proto.title.Publisher, 5));
};


/**
 * @param {?proto.title.Publisher|undefined} value
 * @return {!proto.title.Video} returns this
*/
proto.title.Video.prototype.setPublisherid = function(value) {
  return jspb.Message.setWrapperField(this, 5, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.Video} returns this
 */
proto.title.Video.prototype.clearPublisherid = function() {
  return this.setPublisherid(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.Video.prototype.hasPublisherid = function() {
  return jspb.Message.getField(this, 5) != null;
};


/**
 * optional int64 Count = 6;
 * @return {number}
 */
proto.title.Video.prototype.getCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.Video} returns this
 */
proto.title.Video.prototype.setCount = function(value) {
  return jspb.Message.setProto3IntField(this, 6, value);
};


/**
 * optional float Rating = 7;
 * @return {number}
 */
proto.title.Video.prototype.getRating = function() {
  return /** @type {number} */ (jspb.Message.getFloatingPointFieldWithDefault(this, 7, 0.0));
};


/**
 * @param {number} value
 * @return {!proto.title.Video} returns this
 */
proto.title.Video.prototype.setRating = function(value) {
  return jspb.Message.setProto3FloatField(this, 7, value);
};


/**
 * optional int64 Likes = 8;
 * @return {number}
 */
proto.title.Video.prototype.getLikes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 8, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.Video} returns this
 */
proto.title.Video.prototype.setLikes = function(value) {
  return jspb.Message.setProto3IntField(this, 8, value);
};


/**
 * optional string Date = 9;
 * @return {string}
 */
proto.title.Video.prototype.getDate = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Video} returns this
 */
proto.title.Video.prototype.setDate = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};


/**
 * repeated string Genres = 10;
 * @return {!Array<string>}
 */
proto.title.Video.prototype.getGenresList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 10));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.title.Video} returns this
 */
proto.title.Video.prototype.setGenresList = function(value) {
  return jspb.Message.setField(this, 10, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.title.Video} returns this
 */
proto.title.Video.prototype.addGenres = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 10, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.Video} returns this
 */
proto.title.Video.prototype.clearGenresList = function() {
  return this.setGenresList([]);
};


/**
 * repeated string Tags = 11;
 * @return {!Array<string>}
 */
proto.title.Video.prototype.getTagsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 11));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.title.Video} returns this
 */
proto.title.Video.prototype.setTagsList = function(value) {
  return jspb.Message.setField(this, 11, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.title.Video} returns this
 */
proto.title.Video.prototype.addTags = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 11, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.Video} returns this
 */
proto.title.Video.prototype.clearTagsList = function() {
  return this.setTagsList([]);
};


/**
 * repeated Person Casting = 12;
 * @return {!Array<!proto.title.Person>}
 */
proto.title.Video.prototype.getCastingList = function() {
  return /** @type{!Array<!proto.title.Person>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.title.Person, 12));
};


/**
 * @param {!Array<!proto.title.Person>} value
 * @return {!proto.title.Video} returns this
*/
proto.title.Video.prototype.setCastingList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 12, value);
};


/**
 * @param {!proto.title.Person=} opt_value
 * @param {number=} opt_index
 * @return {!proto.title.Person}
 */
proto.title.Video.prototype.addCasting = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 12, opt_value, proto.title.Person, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.Video} returns this
 */
proto.title.Video.prototype.clearCastingList = function() {
  return this.setCastingList([]);
};


/**
 * optional Poster poster = 13;
 * @return {?proto.title.Poster}
 */
proto.title.Video.prototype.getPoster = function() {
  return /** @type{?proto.title.Poster} */ (
    jspb.Message.getWrapperField(this, proto.title.Poster, 13));
};


/**
 * @param {?proto.title.Poster|undefined} value
 * @return {!proto.title.Video} returns this
*/
proto.title.Video.prototype.setPoster = function(value) {
  return jspb.Message.setWrapperField(this, 13, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.Video} returns this
 */
proto.title.Video.prototype.clearPoster = function() {
  return this.setPoster(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.Video.prototype.hasPoster = function() {
  return jspb.Message.getField(this, 13) != null;
};


/**
 * optional Preview preview = 14;
 * @return {?proto.title.Preview}
 */
proto.title.Video.prototype.getPreview = function() {
  return /** @type{?proto.title.Preview} */ (
    jspb.Message.getWrapperField(this, proto.title.Preview, 14));
};


/**
 * @param {?proto.title.Preview|undefined} value
 * @return {!proto.title.Video} returns this
*/
proto.title.Video.prototype.setPreview = function(value) {
  return jspb.Message.setWrapperField(this, 14, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.Video} returns this
 */
proto.title.Video.prototype.clearPreview = function() {
  return this.setPreview(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.Video.prototype.hasPreview = function() {
  return jspb.Message.getField(this, 14) != null;
};


/**
 * optional int32 Duration = 15;
 * @return {number}
 */
proto.title.Video.prototype.getDuration = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 15, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.Video} returns this
 */
proto.title.Video.prototype.setDuration = function(value) {
  return jspb.Message.setProto3IntField(this, 15, value);
};


/**
 * optional string UUID = 16;
 * @return {string}
 */
proto.title.Video.prototype.getUuid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 16, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Video} returns this
 */
proto.title.Video.prototype.setUuid = function(value) {
  return jspb.Message.setProto3StringField(this, 16, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.title.Videos.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.Videos.prototype.toObject = function(opt_includeInstance) {
  return proto.title.Videos.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.Videos} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Videos.toObject = function(includeInstance, msg) {
  var f, obj = {
    videosList: jspb.Message.toObjectList(msg.getVideosList(),
    proto.title.Video.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.Videos}
 */
proto.title.Videos.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.Videos;
  return proto.title.Videos.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.Videos} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.Videos}
 */
proto.title.Videos.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.Video;
      reader.readMessage(value,proto.title.Video.deserializeBinaryFromReader);
      msg.addVideos(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.Videos.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.Videos.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.Videos} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Videos.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getVideosList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.title.Video.serializeBinaryToWriter
    );
  }
};


/**
 * repeated Video videos = 1;
 * @return {!Array<!proto.title.Video>}
 */
proto.title.Videos.prototype.getVideosList = function() {
  return /** @type{!Array<!proto.title.Video>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.title.Video, 1));
};


/**
 * @param {!Array<!proto.title.Video>} value
 * @return {!proto.title.Videos} returns this
*/
proto.title.Videos.prototype.setVideosList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.title.Video=} opt_value
 * @param {number=} opt_index
 * @return {!proto.title.Video}
 */
proto.title.Videos.prototype.addVideos = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.title.Video, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.Videos} returns this
 */
proto.title.Videos.prototype.clearVideosList = function() {
  return this.setVideosList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.CreateVideoRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.CreateVideoRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.CreateVideoRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.CreateVideoRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    video: (f = msg.getVideo()) && proto.title.Video.toObject(includeInstance, f),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.CreateVideoRequest}
 */
proto.title.CreateVideoRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.CreateVideoRequest;
  return proto.title.CreateVideoRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.CreateVideoRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.CreateVideoRequest}
 */
proto.title.CreateVideoRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.Video;
      reader.readMessage(value,proto.title.Video.deserializeBinaryFromReader);
      msg.setVideo(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.CreateVideoRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.CreateVideoRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.CreateVideoRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.CreateVideoRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getVideo();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.title.Video.serializeBinaryToWriter
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional Video video = 1;
 * @return {?proto.title.Video}
 */
proto.title.CreateVideoRequest.prototype.getVideo = function() {
  return /** @type{?proto.title.Video} */ (
    jspb.Message.getWrapperField(this, proto.title.Video, 1));
};


/**
 * @param {?proto.title.Video|undefined} value
 * @return {!proto.title.CreateVideoRequest} returns this
*/
proto.title.CreateVideoRequest.prototype.setVideo = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.CreateVideoRequest} returns this
 */
proto.title.CreateVideoRequest.prototype.clearVideo = function() {
  return this.setVideo(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.CreateVideoRequest.prototype.hasVideo = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.CreateVideoRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.CreateVideoRequest} returns this
 */
proto.title.CreateVideoRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.CreateVideoResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.CreateVideoResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.CreateVideoResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.CreateVideoResponse.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.CreateVideoResponse}
 */
proto.title.CreateVideoResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.CreateVideoResponse;
  return proto.title.CreateVideoResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.CreateVideoResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.CreateVideoResponse}
 */
proto.title.CreateVideoResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.CreateVideoResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.CreateVideoResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.CreateVideoResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.CreateVideoResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.GetVideoByIdRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.GetVideoByIdRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.GetVideoByIdRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetVideoByIdRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    videoid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.GetVideoByIdRequest}
 */
proto.title.GetVideoByIdRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.GetVideoByIdRequest;
  return proto.title.GetVideoByIdRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.GetVideoByIdRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.GetVideoByIdRequest}
 */
proto.title.GetVideoByIdRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setVideoid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.GetVideoByIdRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.GetVideoByIdRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.GetVideoByIdRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetVideoByIdRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getVideoid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string videoId = 1;
 * @return {string}
 */
proto.title.GetVideoByIdRequest.prototype.getVideoid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.GetVideoByIdRequest} returns this
 */
proto.title.GetVideoByIdRequest.prototype.setVideoid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.GetVideoByIdRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.GetVideoByIdRequest} returns this
 */
proto.title.GetVideoByIdRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.title.GetVideoByIdResponse.repeatedFields_ = [3];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.GetVideoByIdResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.GetVideoByIdResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.GetVideoByIdResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetVideoByIdResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    video: (f = msg.getVideo()) && proto.title.Video.toObject(includeInstance, f),
    filespathsList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.GetVideoByIdResponse}
 */
proto.title.GetVideoByIdResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.GetVideoByIdResponse;
  return proto.title.GetVideoByIdResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.GetVideoByIdResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.GetVideoByIdResponse}
 */
proto.title.GetVideoByIdResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.Video;
      reader.readMessage(value,proto.title.Video.deserializeBinaryFromReader);
      msg.setVideo(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.addFilespaths(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.GetVideoByIdResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.GetVideoByIdResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.GetVideoByIdResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetVideoByIdResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getVideo();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.title.Video.serializeBinaryToWriter
    );
  }
  f = message.getFilespathsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      3,
      f
    );
  }
};


/**
 * optional Video video = 1;
 * @return {?proto.title.Video}
 */
proto.title.GetVideoByIdResponse.prototype.getVideo = function() {
  return /** @type{?proto.title.Video} */ (
    jspb.Message.getWrapperField(this, proto.title.Video, 1));
};


/**
 * @param {?proto.title.Video|undefined} value
 * @return {!proto.title.GetVideoByIdResponse} returns this
*/
proto.title.GetVideoByIdResponse.prototype.setVideo = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.GetVideoByIdResponse} returns this
 */
proto.title.GetVideoByIdResponse.prototype.clearVideo = function() {
  return this.setVideo(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.GetVideoByIdResponse.prototype.hasVideo = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * repeated string filesPaths = 3;
 * @return {!Array<string>}
 */
proto.title.GetVideoByIdResponse.prototype.getFilespathsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.title.GetVideoByIdResponse} returns this
 */
proto.title.GetVideoByIdResponse.prototype.setFilespathsList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.title.GetVideoByIdResponse} returns this
 */
proto.title.GetVideoByIdResponse.prototype.addFilespaths = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.GetVideoByIdResponse} returns this
 */
proto.title.GetVideoByIdResponse.prototype.clearFilespathsList = function() {
  return this.setFilespathsList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.DeleteVideoRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.DeleteVideoRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.DeleteVideoRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeleteVideoRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    videoid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.DeleteVideoRequest}
 */
proto.title.DeleteVideoRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.DeleteVideoRequest;
  return proto.title.DeleteVideoRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.DeleteVideoRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.DeleteVideoRequest}
 */
proto.title.DeleteVideoRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setVideoid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.DeleteVideoRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.DeleteVideoRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.DeleteVideoRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeleteVideoRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getVideoid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string videoId = 1;
 * @return {string}
 */
proto.title.DeleteVideoRequest.prototype.getVideoid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.DeleteVideoRequest} returns this
 */
proto.title.DeleteVideoRequest.prototype.setVideoid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.DeleteVideoRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.DeleteVideoRequest} returns this
 */
proto.title.DeleteVideoRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.DeleteVideoResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.DeleteVideoResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.DeleteVideoResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeleteVideoResponse.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.DeleteVideoResponse}
 */
proto.title.DeleteVideoResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.DeleteVideoResponse;
  return proto.title.DeleteVideoResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.DeleteVideoResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.DeleteVideoResponse}
 */
proto.title.DeleteVideoResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.DeleteVideoResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.DeleteVideoResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.DeleteVideoResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeleteVideoResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.UpdateVideoMetadataRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.UpdateVideoMetadataRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.UpdateVideoMetadataRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.UpdateVideoMetadataRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    video: (f = msg.getVideo()) && proto.title.Video.toObject(includeInstance, f),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.UpdateVideoMetadataRequest}
 */
proto.title.UpdateVideoMetadataRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.UpdateVideoMetadataRequest;
  return proto.title.UpdateVideoMetadataRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.UpdateVideoMetadataRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.UpdateVideoMetadataRequest}
 */
proto.title.UpdateVideoMetadataRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.Video;
      reader.readMessage(value,proto.title.Video.deserializeBinaryFromReader);
      msg.setVideo(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.UpdateVideoMetadataRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.UpdateVideoMetadataRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.UpdateVideoMetadataRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.UpdateVideoMetadataRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getVideo();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.title.Video.serializeBinaryToWriter
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional Video video = 1;
 * @return {?proto.title.Video}
 */
proto.title.UpdateVideoMetadataRequest.prototype.getVideo = function() {
  return /** @type{?proto.title.Video} */ (
    jspb.Message.getWrapperField(this, proto.title.Video, 1));
};


/**
 * @param {?proto.title.Video|undefined} value
 * @return {!proto.title.UpdateVideoMetadataRequest} returns this
*/
proto.title.UpdateVideoMetadataRequest.prototype.setVideo = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.UpdateVideoMetadataRequest} returns this
 */
proto.title.UpdateVideoMetadataRequest.prototype.clearVideo = function() {
  return this.setVideo(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.UpdateVideoMetadataRequest.prototype.hasVideo = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.UpdateVideoMetadataRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.UpdateVideoMetadataRequest} returns this
 */
proto.title.UpdateVideoMetadataRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.UpdateVideoMetadataResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.UpdateVideoMetadataResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.UpdateVideoMetadataResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.UpdateVideoMetadataResponse.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.UpdateVideoMetadataResponse}
 */
proto.title.UpdateVideoMetadataResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.UpdateVideoMetadataResponse;
  return proto.title.UpdateVideoMetadataResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.UpdateVideoMetadataResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.UpdateVideoMetadataResponse}
 */
proto.title.UpdateVideoMetadataResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.UpdateVideoMetadataResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.UpdateVideoMetadataResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.UpdateVideoMetadataResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.UpdateVideoMetadataResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.title.Title.repeatedFields_ = [8,9,10,11,12,13,16];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.Title.prototype.toObject = function(opt_includeInstance) {
  return proto.title.Title.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.Title} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Title.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, ""),
    url: jspb.Message.getFieldWithDefault(msg, 2, ""),
    name: jspb.Message.getFieldWithDefault(msg, 3, ""),
    type: jspb.Message.getFieldWithDefault(msg, 4, ""),
    year: jspb.Message.getFieldWithDefault(msg, 5, 0),
    rating: jspb.Message.getFloatingPointFieldWithDefault(msg, 6, 0.0),
    ratingcount: jspb.Message.getFieldWithDefault(msg, 7, 0),
    directorsList: jspb.Message.toObjectList(msg.getDirectorsList(),
    proto.title.Person.toObject, includeInstance),
    writersList: jspb.Message.toObjectList(msg.getWritersList(),
    proto.title.Person.toObject, includeInstance),
    actorsList: jspb.Message.toObjectList(msg.getActorsList(),
    proto.title.Person.toObject, includeInstance),
    genresList: (f = jspb.Message.getRepeatedField(msg, 11)) == null ? undefined : f,
    languageList: (f = jspb.Message.getRepeatedField(msg, 12)) == null ? undefined : f,
    nationalitiesList: (f = jspb.Message.getRepeatedField(msg, 13)) == null ? undefined : f,
    description: jspb.Message.getFieldWithDefault(msg, 14, ""),
    poster: (f = msg.getPoster()) && proto.title.Poster.toObject(includeInstance, f),
    akaList: (f = jspb.Message.getRepeatedField(msg, 16)) == null ? undefined : f,
    duration: jspb.Message.getFieldWithDefault(msg, 17, ""),
    season: jspb.Message.getFieldWithDefault(msg, 18, 0),
    episode: jspb.Message.getFieldWithDefault(msg, 19, 0),
    serie: jspb.Message.getFieldWithDefault(msg, 20, ""),
    uuid: jspb.Message.getFieldWithDefault(msg, 21, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.Title}
 */
proto.title.Title.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.Title;
  return proto.title.Title.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.Title} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.Title}
 */
proto.title.Title.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setUrl(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setType(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setYear(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readFloat());
      msg.setRating(value);
      break;
    case 7:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setRatingcount(value);
      break;
    case 8:
      var value = new proto.title.Person;
      reader.readMessage(value,proto.title.Person.deserializeBinaryFromReader);
      msg.addDirectors(value);
      break;
    case 9:
      var value = new proto.title.Person;
      reader.readMessage(value,proto.title.Person.deserializeBinaryFromReader);
      msg.addWriters(value);
      break;
    case 10:
      var value = new proto.title.Person;
      reader.readMessage(value,proto.title.Person.deserializeBinaryFromReader);
      msg.addActors(value);
      break;
    case 11:
      var value = /** @type {string} */ (reader.readString());
      msg.addGenres(value);
      break;
    case 12:
      var value = /** @type {string} */ (reader.readString());
      msg.addLanguage(value);
      break;
    case 13:
      var value = /** @type {string} */ (reader.readString());
      msg.addNationalities(value);
      break;
    case 14:
      var value = /** @type {string} */ (reader.readString());
      msg.setDescription(value);
      break;
    case 15:
      var value = new proto.title.Poster;
      reader.readMessage(value,proto.title.Poster.deserializeBinaryFromReader);
      msg.setPoster(value);
      break;
    case 16:
      var value = /** @type {string} */ (reader.readString());
      msg.addAka(value);
      break;
    case 17:
      var value = /** @type {string} */ (reader.readString());
      msg.setDuration(value);
      break;
    case 18:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setSeason(value);
      break;
    case 19:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setEpisode(value);
      break;
    case 20:
      var value = /** @type {string} */ (reader.readString());
      msg.setSerie(value);
      break;
    case 21:
      var value = /** @type {string} */ (reader.readString());
      msg.setUuid(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.Title.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.Title.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.Title} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Title.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getUrl();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getType();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getYear();
  if (f !== 0) {
    writer.writeInt32(
      5,
      f
    );
  }
  f = message.getRating();
  if (f !== 0.0) {
    writer.writeFloat(
      6,
      f
    );
  }
  f = message.getRatingcount();
  if (f !== 0) {
    writer.writeInt32(
      7,
      f
    );
  }
  f = message.getDirectorsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      8,
      f,
      proto.title.Person.serializeBinaryToWriter
    );
  }
  f = message.getWritersList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      9,
      f,
      proto.title.Person.serializeBinaryToWriter
    );
  }
  f = message.getActorsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      10,
      f,
      proto.title.Person.serializeBinaryToWriter
    );
  }
  f = message.getGenresList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      11,
      f
    );
  }
  f = message.getLanguageList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      12,
      f
    );
  }
  f = message.getNationalitiesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      13,
      f
    );
  }
  f = message.getDescription();
  if (f.length > 0) {
    writer.writeString(
      14,
      f
    );
  }
  f = message.getPoster();
  if (f != null) {
    writer.writeMessage(
      15,
      f,
      proto.title.Poster.serializeBinaryToWriter
    );
  }
  f = message.getAkaList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      16,
      f
    );
  }
  f = message.getDuration();
  if (f.length > 0) {
    writer.writeString(
      17,
      f
    );
  }
  f = message.getSeason();
  if (f !== 0) {
    writer.writeInt32(
      18,
      f
    );
  }
  f = message.getEpisode();
  if (f !== 0) {
    writer.writeInt32(
      19,
      f
    );
  }
  f = message.getSerie();
  if (f.length > 0) {
    writer.writeString(
      20,
      f
    );
  }
  f = message.getUuid();
  if (f.length > 0) {
    writer.writeString(
      21,
      f
    );
  }
};


/**
 * optional string ID = 1;
 * @return {string}
 */
proto.title.Title.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string URL = 2;
 * @return {string}
 */
proto.title.Title.prototype.getUrl = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.setUrl = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string Name = 3;
 * @return {string}
 */
proto.title.Title.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string Type = 4;
 * @return {string}
 */
proto.title.Title.prototype.getType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.setType = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional int32 Year = 5;
 * @return {number}
 */
proto.title.Title.prototype.getYear = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.setYear = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional float Rating = 6;
 * @return {number}
 */
proto.title.Title.prototype.getRating = function() {
  return /** @type {number} */ (jspb.Message.getFloatingPointFieldWithDefault(this, 6, 0.0));
};


/**
 * @param {number} value
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.setRating = function(value) {
  return jspb.Message.setProto3FloatField(this, 6, value);
};


/**
 * optional int32 RatingCount = 7;
 * @return {number}
 */
proto.title.Title.prototype.getRatingcount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.setRatingcount = function(value) {
  return jspb.Message.setProto3IntField(this, 7, value);
};


/**
 * repeated Person Directors = 8;
 * @return {!Array<!proto.title.Person>}
 */
proto.title.Title.prototype.getDirectorsList = function() {
  return /** @type{!Array<!proto.title.Person>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.title.Person, 8));
};


/**
 * @param {!Array<!proto.title.Person>} value
 * @return {!proto.title.Title} returns this
*/
proto.title.Title.prototype.setDirectorsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 8, value);
};


/**
 * @param {!proto.title.Person=} opt_value
 * @param {number=} opt_index
 * @return {!proto.title.Person}
 */
proto.title.Title.prototype.addDirectors = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 8, opt_value, proto.title.Person, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.clearDirectorsList = function() {
  return this.setDirectorsList([]);
};


/**
 * repeated Person Writers = 9;
 * @return {!Array<!proto.title.Person>}
 */
proto.title.Title.prototype.getWritersList = function() {
  return /** @type{!Array<!proto.title.Person>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.title.Person, 9));
};


/**
 * @param {!Array<!proto.title.Person>} value
 * @return {!proto.title.Title} returns this
*/
proto.title.Title.prototype.setWritersList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 9, value);
};


/**
 * @param {!proto.title.Person=} opt_value
 * @param {number=} opt_index
 * @return {!proto.title.Person}
 */
proto.title.Title.prototype.addWriters = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 9, opt_value, proto.title.Person, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.clearWritersList = function() {
  return this.setWritersList([]);
};


/**
 * repeated Person Actors = 10;
 * @return {!Array<!proto.title.Person>}
 */
proto.title.Title.prototype.getActorsList = function() {
  return /** @type{!Array<!proto.title.Person>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.title.Person, 10));
};


/**
 * @param {!Array<!proto.title.Person>} value
 * @return {!proto.title.Title} returns this
*/
proto.title.Title.prototype.setActorsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 10, value);
};


/**
 * @param {!proto.title.Person=} opt_value
 * @param {number=} opt_index
 * @return {!proto.title.Person}
 */
proto.title.Title.prototype.addActors = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 10, opt_value, proto.title.Person, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.clearActorsList = function() {
  return this.setActorsList([]);
};


/**
 * repeated string Genres = 11;
 * @return {!Array<string>}
 */
proto.title.Title.prototype.getGenresList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 11));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.setGenresList = function(value) {
  return jspb.Message.setField(this, 11, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.addGenres = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 11, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.clearGenresList = function() {
  return this.setGenresList([]);
};


/**
 * repeated string Language = 12;
 * @return {!Array<string>}
 */
proto.title.Title.prototype.getLanguageList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 12));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.setLanguageList = function(value) {
  return jspb.Message.setField(this, 12, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.addLanguage = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 12, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.clearLanguageList = function() {
  return this.setLanguageList([]);
};


/**
 * repeated string Nationalities = 13;
 * @return {!Array<string>}
 */
proto.title.Title.prototype.getNationalitiesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 13));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.setNationalitiesList = function(value) {
  return jspb.Message.setField(this, 13, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.addNationalities = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 13, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.clearNationalitiesList = function() {
  return this.setNationalitiesList([]);
};


/**
 * optional string Description = 14;
 * @return {string}
 */
proto.title.Title.prototype.getDescription = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 14, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.setDescription = function(value) {
  return jspb.Message.setProto3StringField(this, 14, value);
};


/**
 * optional Poster Poster = 15;
 * @return {?proto.title.Poster}
 */
proto.title.Title.prototype.getPoster = function() {
  return /** @type{?proto.title.Poster} */ (
    jspb.Message.getWrapperField(this, proto.title.Poster, 15));
};


/**
 * @param {?proto.title.Poster|undefined} value
 * @return {!proto.title.Title} returns this
*/
proto.title.Title.prototype.setPoster = function(value) {
  return jspb.Message.setWrapperField(this, 15, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.clearPoster = function() {
  return this.setPoster(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.Title.prototype.hasPoster = function() {
  return jspb.Message.getField(this, 15) != null;
};


/**
 * repeated string AKA = 16;
 * @return {!Array<string>}
 */
proto.title.Title.prototype.getAkaList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 16));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.setAkaList = function(value) {
  return jspb.Message.setField(this, 16, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.addAka = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 16, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.clearAkaList = function() {
  return this.setAkaList([]);
};


/**
 * optional string Duration = 17;
 * @return {string}
 */
proto.title.Title.prototype.getDuration = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 17, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.setDuration = function(value) {
  return jspb.Message.setProto3StringField(this, 17, value);
};


/**
 * optional int32 Season = 18;
 * @return {number}
 */
proto.title.Title.prototype.getSeason = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 18, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.setSeason = function(value) {
  return jspb.Message.setProto3IntField(this, 18, value);
};


/**
 * optional int32 Episode = 19;
 * @return {number}
 */
proto.title.Title.prototype.getEpisode = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 19, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.setEpisode = function(value) {
  return jspb.Message.setProto3IntField(this, 19, value);
};


/**
 * optional string Serie = 20;
 * @return {string}
 */
proto.title.Title.prototype.getSerie = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 20, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.setSerie = function(value) {
  return jspb.Message.setProto3StringField(this, 20, value);
};


/**
 * optional string UUID = 21;
 * @return {string}
 */
proto.title.Title.prototype.getUuid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 21, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Title} returns this
 */
proto.title.Title.prototype.setUuid = function(value) {
  return jspb.Message.setProto3StringField(this, 21, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.title.Titles.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.Titles.prototype.toObject = function(opt_includeInstance) {
  return proto.title.Titles.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.Titles} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Titles.toObject = function(includeInstance, msg) {
  var f, obj = {
    titlesList: jspb.Message.toObjectList(msg.getTitlesList(),
    proto.title.Title.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.Titles}
 */
proto.title.Titles.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.Titles;
  return proto.title.Titles.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.Titles} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.Titles}
 */
proto.title.Titles.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.Title;
      reader.readMessage(value,proto.title.Title.deserializeBinaryFromReader);
      msg.addTitles(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.Titles.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.Titles.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.Titles} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Titles.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTitlesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.title.Title.serializeBinaryToWriter
    );
  }
};


/**
 * repeated Title titles = 1;
 * @return {!Array<!proto.title.Title>}
 */
proto.title.Titles.prototype.getTitlesList = function() {
  return /** @type{!Array<!proto.title.Title>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.title.Title, 1));
};


/**
 * @param {!Array<!proto.title.Title>} value
 * @return {!proto.title.Titles} returns this
*/
proto.title.Titles.prototype.setTitlesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.title.Title=} opt_value
 * @param {number=} opt_index
 * @return {!proto.title.Title}
 */
proto.title.Titles.prototype.addTitles = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.title.Title, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.Titles} returns this
 */
proto.title.Titles.prototype.clearTitlesList = function() {
  return this.setTitlesList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.CreateTitleRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.CreateTitleRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.CreateTitleRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.CreateTitleRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    title: (f = msg.getTitle()) && proto.title.Title.toObject(includeInstance, f),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.CreateTitleRequest}
 */
proto.title.CreateTitleRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.CreateTitleRequest;
  return proto.title.CreateTitleRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.CreateTitleRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.CreateTitleRequest}
 */
proto.title.CreateTitleRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.Title;
      reader.readMessage(value,proto.title.Title.deserializeBinaryFromReader);
      msg.setTitle(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.CreateTitleRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.CreateTitleRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.CreateTitleRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.CreateTitleRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTitle();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.title.Title.serializeBinaryToWriter
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional Title title = 1;
 * @return {?proto.title.Title}
 */
proto.title.CreateTitleRequest.prototype.getTitle = function() {
  return /** @type{?proto.title.Title} */ (
    jspb.Message.getWrapperField(this, proto.title.Title, 1));
};


/**
 * @param {?proto.title.Title|undefined} value
 * @return {!proto.title.CreateTitleRequest} returns this
*/
proto.title.CreateTitleRequest.prototype.setTitle = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.CreateTitleRequest} returns this
 */
proto.title.CreateTitleRequest.prototype.clearTitle = function() {
  return this.setTitle(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.CreateTitleRequest.prototype.hasTitle = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.CreateTitleRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.CreateTitleRequest} returns this
 */
proto.title.CreateTitleRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.CreateTitleResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.CreateTitleResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.CreateTitleResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.CreateTitleResponse.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.CreateTitleResponse}
 */
proto.title.CreateTitleResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.CreateTitleResponse;
  return proto.title.CreateTitleResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.CreateTitleResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.CreateTitleResponse}
 */
proto.title.CreateTitleResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.CreateTitleResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.CreateTitleResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.CreateTitleResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.CreateTitleResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.GetTitleByIdRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.GetTitleByIdRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.GetTitleByIdRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetTitleByIdRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    titleid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.GetTitleByIdRequest}
 */
proto.title.GetTitleByIdRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.GetTitleByIdRequest;
  return proto.title.GetTitleByIdRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.GetTitleByIdRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.GetTitleByIdRequest}
 */
proto.title.GetTitleByIdRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setTitleid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.GetTitleByIdRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.GetTitleByIdRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.GetTitleByIdRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetTitleByIdRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTitleid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string titleId = 1;
 * @return {string}
 */
proto.title.GetTitleByIdRequest.prototype.getTitleid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.GetTitleByIdRequest} returns this
 */
proto.title.GetTitleByIdRequest.prototype.setTitleid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.GetTitleByIdRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.GetTitleByIdRequest} returns this
 */
proto.title.GetTitleByIdRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.title.GetTitleByIdResponse.repeatedFields_ = [3];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.GetTitleByIdResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.GetTitleByIdResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.GetTitleByIdResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetTitleByIdResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    title: (f = msg.getTitle()) && proto.title.Title.toObject(includeInstance, f),
    filespathsList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.GetTitleByIdResponse}
 */
proto.title.GetTitleByIdResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.GetTitleByIdResponse;
  return proto.title.GetTitleByIdResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.GetTitleByIdResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.GetTitleByIdResponse}
 */
proto.title.GetTitleByIdResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.Title;
      reader.readMessage(value,proto.title.Title.deserializeBinaryFromReader);
      msg.setTitle(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.addFilespaths(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.GetTitleByIdResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.GetTitleByIdResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.GetTitleByIdResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetTitleByIdResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTitle();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.title.Title.serializeBinaryToWriter
    );
  }
  f = message.getFilespathsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      3,
      f
    );
  }
};


/**
 * optional Title title = 1;
 * @return {?proto.title.Title}
 */
proto.title.GetTitleByIdResponse.prototype.getTitle = function() {
  return /** @type{?proto.title.Title} */ (
    jspb.Message.getWrapperField(this, proto.title.Title, 1));
};


/**
 * @param {?proto.title.Title|undefined} value
 * @return {!proto.title.GetTitleByIdResponse} returns this
*/
proto.title.GetTitleByIdResponse.prototype.setTitle = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.GetTitleByIdResponse} returns this
 */
proto.title.GetTitleByIdResponse.prototype.clearTitle = function() {
  return this.setTitle(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.GetTitleByIdResponse.prototype.hasTitle = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * repeated string filesPaths = 3;
 * @return {!Array<string>}
 */
proto.title.GetTitleByIdResponse.prototype.getFilespathsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.title.GetTitleByIdResponse} returns this
 */
proto.title.GetTitleByIdResponse.prototype.setFilespathsList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.title.GetTitleByIdResponse} returns this
 */
proto.title.GetTitleByIdResponse.prototype.addFilespaths = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.GetTitleByIdResponse} returns this
 */
proto.title.GetTitleByIdResponse.prototype.clearFilespathsList = function() {
  return this.setFilespathsList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.DeleteTitleRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.DeleteTitleRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.DeleteTitleRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeleteTitleRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    titleid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.DeleteTitleRequest}
 */
proto.title.DeleteTitleRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.DeleteTitleRequest;
  return proto.title.DeleteTitleRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.DeleteTitleRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.DeleteTitleRequest}
 */
proto.title.DeleteTitleRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setTitleid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.DeleteTitleRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.DeleteTitleRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.DeleteTitleRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeleteTitleRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTitleid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string titleId = 1;
 * @return {string}
 */
proto.title.DeleteTitleRequest.prototype.getTitleid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.DeleteTitleRequest} returns this
 */
proto.title.DeleteTitleRequest.prototype.setTitleid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.DeleteTitleRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.DeleteTitleRequest} returns this
 */
proto.title.DeleteTitleRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.DeleteTitleResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.DeleteTitleResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.DeleteTitleResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeleteTitleResponse.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.DeleteTitleResponse}
 */
proto.title.DeleteTitleResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.DeleteTitleResponse;
  return proto.title.DeleteTitleResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.DeleteTitleResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.DeleteTitleResponse}
 */
proto.title.DeleteTitleResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.DeleteTitleResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.DeleteTitleResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.DeleteTitleResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeleteTitleResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.UpdateTitleMetadataRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.UpdateTitleMetadataRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.UpdateTitleMetadataRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.UpdateTitleMetadataRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    title: (f = msg.getTitle()) && proto.title.Title.toObject(includeInstance, f),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.UpdateTitleMetadataRequest}
 */
proto.title.UpdateTitleMetadataRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.UpdateTitleMetadataRequest;
  return proto.title.UpdateTitleMetadataRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.UpdateTitleMetadataRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.UpdateTitleMetadataRequest}
 */
proto.title.UpdateTitleMetadataRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.Title;
      reader.readMessage(value,proto.title.Title.deserializeBinaryFromReader);
      msg.setTitle(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.UpdateTitleMetadataRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.UpdateTitleMetadataRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.UpdateTitleMetadataRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.UpdateTitleMetadataRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTitle();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.title.Title.serializeBinaryToWriter
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional Title title = 1;
 * @return {?proto.title.Title}
 */
proto.title.UpdateTitleMetadataRequest.prototype.getTitle = function() {
  return /** @type{?proto.title.Title} */ (
    jspb.Message.getWrapperField(this, proto.title.Title, 1));
};


/**
 * @param {?proto.title.Title|undefined} value
 * @return {!proto.title.UpdateTitleMetadataRequest} returns this
*/
proto.title.UpdateTitleMetadataRequest.prototype.setTitle = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.UpdateTitleMetadataRequest} returns this
 */
proto.title.UpdateTitleMetadataRequest.prototype.clearTitle = function() {
  return this.setTitle(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.UpdateTitleMetadataRequest.prototype.hasTitle = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.UpdateTitleMetadataRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.UpdateTitleMetadataRequest} returns this
 */
proto.title.UpdateTitleMetadataRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.UpdateTitleMetadataResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.UpdateTitleMetadataResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.UpdateTitleMetadataResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.UpdateTitleMetadataResponse.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.UpdateTitleMetadataResponse}
 */
proto.title.UpdateTitleMetadataResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.UpdateTitleMetadataResponse;
  return proto.title.UpdateTitleMetadataResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.UpdateTitleMetadataResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.UpdateTitleMetadataResponse}
 */
proto.title.UpdateTitleMetadataResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.UpdateTitleMetadataResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.UpdateTitleMetadataResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.UpdateTitleMetadataResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.UpdateTitleMetadataResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.AssociateFileWithTitleRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.AssociateFileWithTitleRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.AssociateFileWithTitleRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.AssociateFileWithTitleRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    titleid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    filepath: jspb.Message.getFieldWithDefault(msg, 2, ""),
    indexpath: jspb.Message.getFieldWithDefault(msg, 3, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.AssociateFileWithTitleRequest}
 */
proto.title.AssociateFileWithTitleRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.AssociateFileWithTitleRequest;
  return proto.title.AssociateFileWithTitleRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.AssociateFileWithTitleRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.AssociateFileWithTitleRequest}
 */
proto.title.AssociateFileWithTitleRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setTitleid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setFilepath(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.AssociateFileWithTitleRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.AssociateFileWithTitleRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.AssociateFileWithTitleRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.AssociateFileWithTitleRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTitleid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getFilepath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string titleId = 1;
 * @return {string}
 */
proto.title.AssociateFileWithTitleRequest.prototype.getTitleid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.AssociateFileWithTitleRequest} returns this
 */
proto.title.AssociateFileWithTitleRequest.prototype.setTitleid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string filePath = 2;
 * @return {string}
 */
proto.title.AssociateFileWithTitleRequest.prototype.getFilepath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.AssociateFileWithTitleRequest} returns this
 */
proto.title.AssociateFileWithTitleRequest.prototype.setFilepath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string indexPath = 3;
 * @return {string}
 */
proto.title.AssociateFileWithTitleRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.AssociateFileWithTitleRequest} returns this
 */
proto.title.AssociateFileWithTitleRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.AssociateFileWithTitleResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.AssociateFileWithTitleResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.AssociateFileWithTitleResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.AssociateFileWithTitleResponse.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.AssociateFileWithTitleResponse}
 */
proto.title.AssociateFileWithTitleResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.AssociateFileWithTitleResponse;
  return proto.title.AssociateFileWithTitleResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.AssociateFileWithTitleResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.AssociateFileWithTitleResponse}
 */
proto.title.AssociateFileWithTitleResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.AssociateFileWithTitleResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.AssociateFileWithTitleResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.AssociateFileWithTitleResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.AssociateFileWithTitleResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.DissociateFileWithTitleRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.DissociateFileWithTitleRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.DissociateFileWithTitleRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DissociateFileWithTitleRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    titleid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    filepath: jspb.Message.getFieldWithDefault(msg, 2, ""),
    indexpath: jspb.Message.getFieldWithDefault(msg, 3, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.DissociateFileWithTitleRequest}
 */
proto.title.DissociateFileWithTitleRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.DissociateFileWithTitleRequest;
  return proto.title.DissociateFileWithTitleRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.DissociateFileWithTitleRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.DissociateFileWithTitleRequest}
 */
proto.title.DissociateFileWithTitleRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setTitleid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setFilepath(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.DissociateFileWithTitleRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.DissociateFileWithTitleRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.DissociateFileWithTitleRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DissociateFileWithTitleRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTitleid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getFilepath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string titleId = 1;
 * @return {string}
 */
proto.title.DissociateFileWithTitleRequest.prototype.getTitleid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.DissociateFileWithTitleRequest} returns this
 */
proto.title.DissociateFileWithTitleRequest.prototype.setTitleid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string filePath = 2;
 * @return {string}
 */
proto.title.DissociateFileWithTitleRequest.prototype.getFilepath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.DissociateFileWithTitleRequest} returns this
 */
proto.title.DissociateFileWithTitleRequest.prototype.setFilepath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string indexPath = 3;
 * @return {string}
 */
proto.title.DissociateFileWithTitleRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.DissociateFileWithTitleRequest} returns this
 */
proto.title.DissociateFileWithTitleRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.DissociateFileWithTitleResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.DissociateFileWithTitleResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.DissociateFileWithTitleResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DissociateFileWithTitleResponse.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.DissociateFileWithTitleResponse}
 */
proto.title.DissociateFileWithTitleResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.DissociateFileWithTitleResponse;
  return proto.title.DissociateFileWithTitleResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.DissociateFileWithTitleResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.DissociateFileWithTitleResponse}
 */
proto.title.DissociateFileWithTitleResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.DissociateFileWithTitleResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.DissociateFileWithTitleResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.DissociateFileWithTitleResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DissociateFileWithTitleResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.GetFileTitlesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.GetFileTitlesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.GetFileTitlesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetFileTitlesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    filepath: jspb.Message.getFieldWithDefault(msg, 1, ""),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.GetFileTitlesRequest}
 */
proto.title.GetFileTitlesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.GetFileTitlesRequest;
  return proto.title.GetFileTitlesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.GetFileTitlesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.GetFileTitlesRequest}
 */
proto.title.GetFileTitlesRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setFilepath(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.GetFileTitlesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.GetFileTitlesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.GetFileTitlesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetFileTitlesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getFilepath();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string filePath = 1;
 * @return {string}
 */
proto.title.GetFileTitlesRequest.prototype.getFilepath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.GetFileTitlesRequest} returns this
 */
proto.title.GetFileTitlesRequest.prototype.setFilepath = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.GetFileTitlesRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.GetFileTitlesRequest} returns this
 */
proto.title.GetFileTitlesRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.GetFileTitlesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.GetFileTitlesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.GetFileTitlesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetFileTitlesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    titles: (f = msg.getTitles()) && proto.title.Titles.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.GetFileTitlesResponse}
 */
proto.title.GetFileTitlesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.GetFileTitlesResponse;
  return proto.title.GetFileTitlesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.GetFileTitlesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.GetFileTitlesResponse}
 */
proto.title.GetFileTitlesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.Titles;
      reader.readMessage(value,proto.title.Titles.deserializeBinaryFromReader);
      msg.setTitles(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.GetFileTitlesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.GetFileTitlesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.GetFileTitlesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetFileTitlesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTitles();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.title.Titles.serializeBinaryToWriter
    );
  }
};


/**
 * optional Titles titles = 1;
 * @return {?proto.title.Titles}
 */
proto.title.GetFileTitlesResponse.prototype.getTitles = function() {
  return /** @type{?proto.title.Titles} */ (
    jspb.Message.getWrapperField(this, proto.title.Titles, 1));
};


/**
 * @param {?proto.title.Titles|undefined} value
 * @return {!proto.title.GetFileTitlesResponse} returns this
*/
proto.title.GetFileTitlesResponse.prototype.setTitles = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.GetFileTitlesResponse} returns this
 */
proto.title.GetFileTitlesResponse.prototype.clearTitles = function() {
  return this.setTitles(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.GetFileTitlesResponse.prototype.hasTitles = function() {
  return jspb.Message.getField(this, 1) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.GetFileVideosRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.GetFileVideosRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.GetFileVideosRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetFileVideosRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    filepath: jspb.Message.getFieldWithDefault(msg, 1, ""),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.GetFileVideosRequest}
 */
proto.title.GetFileVideosRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.GetFileVideosRequest;
  return proto.title.GetFileVideosRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.GetFileVideosRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.GetFileVideosRequest}
 */
proto.title.GetFileVideosRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setFilepath(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.GetFileVideosRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.GetFileVideosRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.GetFileVideosRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetFileVideosRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getFilepath();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string filePath = 1;
 * @return {string}
 */
proto.title.GetFileVideosRequest.prototype.getFilepath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.GetFileVideosRequest} returns this
 */
proto.title.GetFileVideosRequest.prototype.setFilepath = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.GetFileVideosRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.GetFileVideosRequest} returns this
 */
proto.title.GetFileVideosRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.GetFileVideosResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.GetFileVideosResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.GetFileVideosResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetFileVideosResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    videos: (f = msg.getVideos()) && proto.title.Videos.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.GetFileVideosResponse}
 */
proto.title.GetFileVideosResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.GetFileVideosResponse;
  return proto.title.GetFileVideosResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.GetFileVideosResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.GetFileVideosResponse}
 */
proto.title.GetFileVideosResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.Videos;
      reader.readMessage(value,proto.title.Videos.deserializeBinaryFromReader);
      msg.setVideos(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.GetFileVideosResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.GetFileVideosResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.GetFileVideosResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetFileVideosResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getVideos();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.title.Videos.serializeBinaryToWriter
    );
  }
};


/**
 * optional Videos videos = 1;
 * @return {?proto.title.Videos}
 */
proto.title.GetFileVideosResponse.prototype.getVideos = function() {
  return /** @type{?proto.title.Videos} */ (
    jspb.Message.getWrapperField(this, proto.title.Videos, 1));
};


/**
 * @param {?proto.title.Videos|undefined} value
 * @return {!proto.title.GetFileVideosResponse} returns this
*/
proto.title.GetFileVideosResponse.prototype.setVideos = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.GetFileVideosResponse} returns this
 */
proto.title.GetFileVideosResponse.prototype.clearVideos = function() {
  return this.setVideos(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.GetFileVideosResponse.prototype.hasVideos = function() {
  return jspb.Message.getField(this, 1) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.GetTitleFilesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.GetTitleFilesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.GetTitleFilesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetTitleFilesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    titleid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.GetTitleFilesRequest}
 */
proto.title.GetTitleFilesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.GetTitleFilesRequest;
  return proto.title.GetTitleFilesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.GetTitleFilesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.GetTitleFilesRequest}
 */
proto.title.GetTitleFilesRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setTitleid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.GetTitleFilesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.GetTitleFilesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.GetTitleFilesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetTitleFilesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTitleid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string titleId = 1;
 * @return {string}
 */
proto.title.GetTitleFilesRequest.prototype.getTitleid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.GetTitleFilesRequest} returns this
 */
proto.title.GetTitleFilesRequest.prototype.setTitleid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.GetTitleFilesRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.GetTitleFilesRequest} returns this
 */
proto.title.GetTitleFilesRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.title.GetTitleFilesResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.GetTitleFilesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.GetTitleFilesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.GetTitleFilesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetTitleFilesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    filepathsList: (f = jspb.Message.getRepeatedField(msg, 1)) == null ? undefined : f
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.GetTitleFilesResponse}
 */
proto.title.GetTitleFilesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.GetTitleFilesResponse;
  return proto.title.GetTitleFilesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.GetTitleFilesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.GetTitleFilesResponse}
 */
proto.title.GetTitleFilesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.addFilepaths(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.GetTitleFilesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.GetTitleFilesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.GetTitleFilesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetTitleFilesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getFilepathsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      1,
      f
    );
  }
};


/**
 * repeated string filePaths = 1;
 * @return {!Array<string>}
 */
proto.title.GetTitleFilesResponse.prototype.getFilepathsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 1));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.title.GetTitleFilesResponse} returns this
 */
proto.title.GetTitleFilesResponse.prototype.setFilepathsList = function(value) {
  return jspb.Message.setField(this, 1, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.title.GetTitleFilesResponse} returns this
 */
proto.title.GetTitleFilesResponse.prototype.addFilepaths = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 1, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.GetTitleFilesResponse} returns this
 */
proto.title.GetTitleFilesResponse.prototype.clearFilepathsList = function() {
  return this.setFilepathsList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.title.Snippet.repeatedFields_ = [2];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.Snippet.prototype.toObject = function(opt_includeInstance) {
  return proto.title.Snippet.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.Snippet} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Snippet.toObject = function(includeInstance, msg) {
  var f, obj = {
    field: jspb.Message.getFieldWithDefault(msg, 1, ""),
    fragmentsList: (f = jspb.Message.getRepeatedField(msg, 2)) == null ? undefined : f
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.Snippet}
 */
proto.title.Snippet.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.Snippet;
  return proto.title.Snippet.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.Snippet} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.Snippet}
 */
proto.title.Snippet.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setField(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.addFragments(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.Snippet.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.Snippet.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.Snippet} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Snippet.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getField();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getFragmentsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      2,
      f
    );
  }
};


/**
 * optional string field = 1;
 * @return {string}
 */
proto.title.Snippet.prototype.getField = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Snippet} returns this
 */
proto.title.Snippet.prototype.setField = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * repeated string fragments = 2;
 * @return {!Array<string>}
 */
proto.title.Snippet.prototype.getFragmentsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.title.Snippet} returns this
 */
proto.title.Snippet.prototype.setFragmentsList = function(value) {
  return jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.title.Snippet} returns this
 */
proto.title.Snippet.prototype.addFragments = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.Snippet} returns this
 */
proto.title.Snippet.prototype.clearFragmentsList = function() {
  return this.setFragmentsList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.title.SearchHit.repeatedFields_ = [3];

/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.title.SearchHit.oneofGroups_ = [[4,5,6,7]];

/**
 * @enum {number}
 */
proto.title.SearchHit.ResultCase = {
  RESULT_NOT_SET: 0,
  TITLE: 4,
  VIDEO: 5,
  AUDIO: 6,
  PERSON: 7
};

/**
 * @return {proto.title.SearchHit.ResultCase}
 */
proto.title.SearchHit.prototype.getResultCase = function() {
  return /** @type {proto.title.SearchHit.ResultCase} */(jspb.Message.computeOneofCase(this, proto.title.SearchHit.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.SearchHit.prototype.toObject = function(opt_includeInstance) {
  return proto.title.SearchHit.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.SearchHit} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.SearchHit.toObject = function(includeInstance, msg) {
  var f, obj = {
    index: jspb.Message.getFieldWithDefault(msg, 1, 0),
    score: jspb.Message.getFloatingPointFieldWithDefault(msg, 2, 0.0),
    snippetsList: jspb.Message.toObjectList(msg.getSnippetsList(),
    proto.title.Snippet.toObject, includeInstance),
    title: (f = msg.getTitle()) && proto.title.Title.toObject(includeInstance, f),
    video: (f = msg.getVideo()) && proto.title.Video.toObject(includeInstance, f),
    audio: (f = msg.getAudio()) && proto.title.Audio.toObject(includeInstance, f),
    person: (f = msg.getPerson()) && proto.title.Person.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.SearchHit}
 */
proto.title.SearchHit.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.SearchHit;
  return proto.title.SearchHit.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.SearchHit} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.SearchHit}
 */
proto.title.SearchHit.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setIndex(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readDouble());
      msg.setScore(value);
      break;
    case 3:
      var value = new proto.title.Snippet;
      reader.readMessage(value,proto.title.Snippet.deserializeBinaryFromReader);
      msg.addSnippets(value);
      break;
    case 4:
      var value = new proto.title.Title;
      reader.readMessage(value,proto.title.Title.deserializeBinaryFromReader);
      msg.setTitle(value);
      break;
    case 5:
      var value = new proto.title.Video;
      reader.readMessage(value,proto.title.Video.deserializeBinaryFromReader);
      msg.setVideo(value);
      break;
    case 6:
      var value = new proto.title.Audio;
      reader.readMessage(value,proto.title.Audio.deserializeBinaryFromReader);
      msg.setAudio(value);
      break;
    case 7:
      var value = new proto.title.Person;
      reader.readMessage(value,proto.title.Person.deserializeBinaryFromReader);
      msg.setPerson(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.SearchHit.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.SearchHit.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.SearchHit} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.SearchHit.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getIndex();
  if (f !== 0) {
    writer.writeInt32(
      1,
      f
    );
  }
  f = message.getScore();
  if (f !== 0.0) {
    writer.writeDouble(
      2,
      f
    );
  }
  f = message.getSnippetsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      3,
      f,
      proto.title.Snippet.serializeBinaryToWriter
    );
  }
  f = message.getTitle();
  if (f != null) {
    writer.writeMessage(
      4,
      f,
      proto.title.Title.serializeBinaryToWriter
    );
  }
  f = message.getVideo();
  if (f != null) {
    writer.writeMessage(
      5,
      f,
      proto.title.Video.serializeBinaryToWriter
    );
  }
  f = message.getAudio();
  if (f != null) {
    writer.writeMessage(
      6,
      f,
      proto.title.Audio.serializeBinaryToWriter
    );
  }
  f = message.getPerson();
  if (f != null) {
    writer.writeMessage(
      7,
      f,
      proto.title.Person.serializeBinaryToWriter
    );
  }
};


/**
 * optional int32 index = 1;
 * @return {number}
 */
proto.title.SearchHit.prototype.getIndex = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.SearchHit} returns this
 */
proto.title.SearchHit.prototype.setIndex = function(value) {
  return jspb.Message.setProto3IntField(this, 1, value);
};


/**
 * optional double score = 2;
 * @return {number}
 */
proto.title.SearchHit.prototype.getScore = function() {
  return /** @type {number} */ (jspb.Message.getFloatingPointFieldWithDefault(this, 2, 0.0));
};


/**
 * @param {number} value
 * @return {!proto.title.SearchHit} returns this
 */
proto.title.SearchHit.prototype.setScore = function(value) {
  return jspb.Message.setProto3FloatField(this, 2, value);
};


/**
 * repeated Snippet snippets = 3;
 * @return {!Array<!proto.title.Snippet>}
 */
proto.title.SearchHit.prototype.getSnippetsList = function() {
  return /** @type{!Array<!proto.title.Snippet>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.title.Snippet, 3));
};


/**
 * @param {!Array<!proto.title.Snippet>} value
 * @return {!proto.title.SearchHit} returns this
*/
proto.title.SearchHit.prototype.setSnippetsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 3, value);
};


/**
 * @param {!proto.title.Snippet=} opt_value
 * @param {number=} opt_index
 * @return {!proto.title.Snippet}
 */
proto.title.SearchHit.prototype.addSnippets = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 3, opt_value, proto.title.Snippet, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.SearchHit} returns this
 */
proto.title.SearchHit.prototype.clearSnippetsList = function() {
  return this.setSnippetsList([]);
};


/**
 * optional Title title = 4;
 * @return {?proto.title.Title}
 */
proto.title.SearchHit.prototype.getTitle = function() {
  return /** @type{?proto.title.Title} */ (
    jspb.Message.getWrapperField(this, proto.title.Title, 4));
};


/**
 * @param {?proto.title.Title|undefined} value
 * @return {!proto.title.SearchHit} returns this
*/
proto.title.SearchHit.prototype.setTitle = function(value) {
  return jspb.Message.setOneofWrapperField(this, 4, proto.title.SearchHit.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.SearchHit} returns this
 */
proto.title.SearchHit.prototype.clearTitle = function() {
  return this.setTitle(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.SearchHit.prototype.hasTitle = function() {
  return jspb.Message.getField(this, 4) != null;
};


/**
 * optional Video video = 5;
 * @return {?proto.title.Video}
 */
proto.title.SearchHit.prototype.getVideo = function() {
  return /** @type{?proto.title.Video} */ (
    jspb.Message.getWrapperField(this, proto.title.Video, 5));
};


/**
 * @param {?proto.title.Video|undefined} value
 * @return {!proto.title.SearchHit} returns this
*/
proto.title.SearchHit.prototype.setVideo = function(value) {
  return jspb.Message.setOneofWrapperField(this, 5, proto.title.SearchHit.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.SearchHit} returns this
 */
proto.title.SearchHit.prototype.clearVideo = function() {
  return this.setVideo(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.SearchHit.prototype.hasVideo = function() {
  return jspb.Message.getField(this, 5) != null;
};


/**
 * optional Audio audio = 6;
 * @return {?proto.title.Audio}
 */
proto.title.SearchHit.prototype.getAudio = function() {
  return /** @type{?proto.title.Audio} */ (
    jspb.Message.getWrapperField(this, proto.title.Audio, 6));
};


/**
 * @param {?proto.title.Audio|undefined} value
 * @return {!proto.title.SearchHit} returns this
*/
proto.title.SearchHit.prototype.setAudio = function(value) {
  return jspb.Message.setOneofWrapperField(this, 6, proto.title.SearchHit.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.SearchHit} returns this
 */
proto.title.SearchHit.prototype.clearAudio = function() {
  return this.setAudio(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.SearchHit.prototype.hasAudio = function() {
  return jspb.Message.getField(this, 6) != null;
};


/**
 * optional Person person = 7;
 * @return {?proto.title.Person}
 */
proto.title.SearchHit.prototype.getPerson = function() {
  return /** @type{?proto.title.Person} */ (
    jspb.Message.getWrapperField(this, proto.title.Person, 7));
};


/**
 * @param {?proto.title.Person|undefined} value
 * @return {!proto.title.SearchHit} returns this
*/
proto.title.SearchHit.prototype.setPerson = function(value) {
  return jspb.Message.setOneofWrapperField(this, 7, proto.title.SearchHit.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.SearchHit} returns this
 */
proto.title.SearchHit.prototype.clearPerson = function() {
  return this.setPerson(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.SearchHit.prototype.hasPerson = function() {
  return jspb.Message.getField(this, 7) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.SearchSummary.prototype.toObject = function(opt_includeInstance) {
  return proto.title.SearchSummary.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.SearchSummary} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.SearchSummary.toObject = function(includeInstance, msg) {
  var f, obj = {
    query: jspb.Message.getFieldWithDefault(msg, 1, ""),
    total: jspb.Message.getFieldWithDefault(msg, 2, 0),
    took: jspb.Message.getFieldWithDefault(msg, 3, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.SearchSummary}
 */
proto.title.SearchSummary.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.SearchSummary;
  return proto.title.SearchSummary.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.SearchSummary} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.SearchSummary}
 */
proto.title.SearchSummary.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setQuery(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readUint64());
      msg.setTotal(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setTook(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.SearchSummary.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.SearchSummary.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.SearchSummary} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.SearchSummary.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getQuery();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getTotal();
  if (f !== 0) {
    writer.writeUint64(
      2,
      f
    );
  }
  f = message.getTook();
  if (f !== 0) {
    writer.writeInt64(
      3,
      f
    );
  }
};


/**
 * optional string query = 1;
 * @return {string}
 */
proto.title.SearchSummary.prototype.getQuery = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.SearchSummary} returns this
 */
proto.title.SearchSummary.prototype.setQuery = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional uint64 total = 2;
 * @return {number}
 */
proto.title.SearchSummary.prototype.getTotal = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.SearchSummary} returns this
 */
proto.title.SearchSummary.prototype.setTotal = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional int64 took = 3;
 * @return {number}
 */
proto.title.SearchSummary.prototype.getTook = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.SearchSummary} returns this
 */
proto.title.SearchSummary.prototype.setTook = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.SearchFacetTerm.prototype.toObject = function(opt_includeInstance) {
  return proto.title.SearchFacetTerm.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.SearchFacetTerm} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.SearchFacetTerm.toObject = function(includeInstance, msg) {
  var f, obj = {
    term: jspb.Message.getFieldWithDefault(msg, 1, ""),
    count: jspb.Message.getFieldWithDefault(msg, 2, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.SearchFacetTerm}
 */
proto.title.SearchFacetTerm.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.SearchFacetTerm;
  return proto.title.SearchFacetTerm.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.SearchFacetTerm} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.SearchFacetTerm}
 */
proto.title.SearchFacetTerm.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setTerm(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setCount(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.SearchFacetTerm.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.SearchFacetTerm.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.SearchFacetTerm} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.SearchFacetTerm.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTerm();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getCount();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
};


/**
 * optional string term = 1;
 * @return {string}
 */
proto.title.SearchFacetTerm.prototype.getTerm = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.SearchFacetTerm} returns this
 */
proto.title.SearchFacetTerm.prototype.setTerm = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int32 count = 2;
 * @return {number}
 */
proto.title.SearchFacetTerm.prototype.getCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.SearchFacetTerm} returns this
 */
proto.title.SearchFacetTerm.prototype.setCount = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.title.SearchFacet.repeatedFields_ = [3];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.SearchFacet.prototype.toObject = function(opt_includeInstance) {
  return proto.title.SearchFacet.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.SearchFacet} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.SearchFacet.toObject = function(includeInstance, msg) {
  var f, obj = {
    field: jspb.Message.getFieldWithDefault(msg, 1, ""),
    total: jspb.Message.getFieldWithDefault(msg, 2, 0),
    termsList: jspb.Message.toObjectList(msg.getTermsList(),
    proto.title.SearchFacetTerm.toObject, includeInstance),
    other: jspb.Message.getFieldWithDefault(msg, 4, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.SearchFacet}
 */
proto.title.SearchFacet.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.SearchFacet;
  return proto.title.SearchFacet.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.SearchFacet} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.SearchFacet}
 */
proto.title.SearchFacet.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setField(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setTotal(value);
      break;
    case 3:
      var value = new proto.title.SearchFacetTerm;
      reader.readMessage(value,proto.title.SearchFacetTerm.deserializeBinaryFromReader);
      msg.addTerms(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setOther(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.SearchFacet.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.SearchFacet.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.SearchFacet} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.SearchFacet.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getField();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getTotal();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getTermsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      3,
      f,
      proto.title.SearchFacetTerm.serializeBinaryToWriter
    );
  }
  f = message.getOther();
  if (f !== 0) {
    writer.writeInt32(
      4,
      f
    );
  }
};


/**
 * optional string field = 1;
 * @return {string}
 */
proto.title.SearchFacet.prototype.getField = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.SearchFacet} returns this
 */
proto.title.SearchFacet.prototype.setField = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int32 total = 2;
 * @return {number}
 */
proto.title.SearchFacet.prototype.getTotal = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.SearchFacet} returns this
 */
proto.title.SearchFacet.prototype.setTotal = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * repeated SearchFacetTerm terms = 3;
 * @return {!Array<!proto.title.SearchFacetTerm>}
 */
proto.title.SearchFacet.prototype.getTermsList = function() {
  return /** @type{!Array<!proto.title.SearchFacetTerm>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.title.SearchFacetTerm, 3));
};


/**
 * @param {!Array<!proto.title.SearchFacetTerm>} value
 * @return {!proto.title.SearchFacet} returns this
*/
proto.title.SearchFacet.prototype.setTermsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 3, value);
};


/**
 * @param {!proto.title.SearchFacetTerm=} opt_value
 * @param {number=} opt_index
 * @return {!proto.title.SearchFacetTerm}
 */
proto.title.SearchFacet.prototype.addTerms = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 3, opt_value, proto.title.SearchFacetTerm, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.SearchFacet} returns this
 */
proto.title.SearchFacet.prototype.clearTermsList = function() {
  return this.setTermsList([]);
};


/**
 * optional int32 other = 4;
 * @return {number}
 */
proto.title.SearchFacet.prototype.getOther = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.SearchFacet} returns this
 */
proto.title.SearchFacet.prototype.setOther = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.title.SearchFacets.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.SearchFacets.prototype.toObject = function(opt_includeInstance) {
  return proto.title.SearchFacets.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.SearchFacets} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.SearchFacets.toObject = function(includeInstance, msg) {
  var f, obj = {
    facetsList: jspb.Message.toObjectList(msg.getFacetsList(),
    proto.title.SearchFacet.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.SearchFacets}
 */
proto.title.SearchFacets.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.SearchFacets;
  return proto.title.SearchFacets.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.SearchFacets} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.SearchFacets}
 */
proto.title.SearchFacets.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.SearchFacet;
      reader.readMessage(value,proto.title.SearchFacet.deserializeBinaryFromReader);
      msg.addFacets(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.SearchFacets.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.SearchFacets.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.SearchFacets} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.SearchFacets.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getFacetsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.title.SearchFacet.serializeBinaryToWriter
    );
  }
};


/**
 * repeated SearchFacet facets = 1;
 * @return {!Array<!proto.title.SearchFacet>}
 */
proto.title.SearchFacets.prototype.getFacetsList = function() {
  return /** @type{!Array<!proto.title.SearchFacet>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.title.SearchFacet, 1));
};


/**
 * @param {!Array<!proto.title.SearchFacet>} value
 * @return {!proto.title.SearchFacets} returns this
*/
proto.title.SearchFacets.prototype.setFacetsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.title.SearchFacet=} opt_value
 * @param {number=} opt_index
 * @return {!proto.title.SearchFacet}
 */
proto.title.SearchFacets.prototype.addFacets = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.title.SearchFacet, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.SearchFacets} returns this
 */
proto.title.SearchFacets.prototype.clearFacetsList = function() {
  return this.setFacetsList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.title.SearchTitlesRequest.repeatedFields_ = [2];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.SearchTitlesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.SearchTitlesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.SearchTitlesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.SearchTitlesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    query: jspb.Message.getFieldWithDefault(msg, 1, ""),
    fieldsList: (f = jspb.Message.getRepeatedField(msg, 2)) == null ? undefined : f,
    indexpath: jspb.Message.getFieldWithDefault(msg, 3, ""),
    size: jspb.Message.getFieldWithDefault(msg, 4, 0),
    offset: jspb.Message.getFieldWithDefault(msg, 5, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.SearchTitlesRequest}
 */
proto.title.SearchTitlesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.SearchTitlesRequest;
  return proto.title.SearchTitlesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.SearchTitlesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.SearchTitlesRequest}
 */
proto.title.SearchTitlesRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setQuery(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.addFields(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setSize(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setOffset(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.SearchTitlesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.SearchTitlesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.SearchTitlesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.SearchTitlesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getQuery();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getFieldsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      2,
      f
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getSize();
  if (f !== 0) {
    writer.writeInt32(
      4,
      f
    );
  }
  f = message.getOffset();
  if (f !== 0) {
    writer.writeInt32(
      5,
      f
    );
  }
};


/**
 * optional string query = 1;
 * @return {string}
 */
proto.title.SearchTitlesRequest.prototype.getQuery = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.SearchTitlesRequest} returns this
 */
proto.title.SearchTitlesRequest.prototype.setQuery = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * repeated string fields = 2;
 * @return {!Array<string>}
 */
proto.title.SearchTitlesRequest.prototype.getFieldsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.title.SearchTitlesRequest} returns this
 */
proto.title.SearchTitlesRequest.prototype.setFieldsList = function(value) {
  return jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.title.SearchTitlesRequest} returns this
 */
proto.title.SearchTitlesRequest.prototype.addFields = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.SearchTitlesRequest} returns this
 */
proto.title.SearchTitlesRequest.prototype.clearFieldsList = function() {
  return this.setFieldsList([]);
};


/**
 * optional string indexPath = 3;
 * @return {string}
 */
proto.title.SearchTitlesRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.SearchTitlesRequest} returns this
 */
proto.title.SearchTitlesRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional int32 size = 4;
 * @return {number}
 */
proto.title.SearchTitlesRequest.prototype.getSize = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.SearchTitlesRequest} returns this
 */
proto.title.SearchTitlesRequest.prototype.setSize = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional int32 offset = 5;
 * @return {number}
 */
proto.title.SearchTitlesRequest.prototype.getOffset = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.SearchTitlesRequest} returns this
 */
proto.title.SearchTitlesRequest.prototype.setOffset = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};



/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.title.SearchTitlesResponse.oneofGroups_ = [[1,2,3]];

/**
 * @enum {number}
 */
proto.title.SearchTitlesResponse.ResultCase = {
  RESULT_NOT_SET: 0,
  SUMMARY: 1,
  HIT: 2,
  FACETS: 3
};

/**
 * @return {proto.title.SearchTitlesResponse.ResultCase}
 */
proto.title.SearchTitlesResponse.prototype.getResultCase = function() {
  return /** @type {proto.title.SearchTitlesResponse.ResultCase} */(jspb.Message.computeOneofCase(this, proto.title.SearchTitlesResponse.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.SearchTitlesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.SearchTitlesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.SearchTitlesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.SearchTitlesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    summary: (f = msg.getSummary()) && proto.title.SearchSummary.toObject(includeInstance, f),
    hit: (f = msg.getHit()) && proto.title.SearchHit.toObject(includeInstance, f),
    facets: (f = msg.getFacets()) && proto.title.SearchFacets.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.SearchTitlesResponse}
 */
proto.title.SearchTitlesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.SearchTitlesResponse;
  return proto.title.SearchTitlesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.SearchTitlesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.SearchTitlesResponse}
 */
proto.title.SearchTitlesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.SearchSummary;
      reader.readMessage(value,proto.title.SearchSummary.deserializeBinaryFromReader);
      msg.setSummary(value);
      break;
    case 2:
      var value = new proto.title.SearchHit;
      reader.readMessage(value,proto.title.SearchHit.deserializeBinaryFromReader);
      msg.setHit(value);
      break;
    case 3:
      var value = new proto.title.SearchFacets;
      reader.readMessage(value,proto.title.SearchFacets.deserializeBinaryFromReader);
      msg.setFacets(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.SearchTitlesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.SearchTitlesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.SearchTitlesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.SearchTitlesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSummary();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.title.SearchSummary.serializeBinaryToWriter
    );
  }
  f = message.getHit();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.title.SearchHit.serializeBinaryToWriter
    );
  }
  f = message.getFacets();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.title.SearchFacets.serializeBinaryToWriter
    );
  }
};


/**
 * optional SearchSummary summary = 1;
 * @return {?proto.title.SearchSummary}
 */
proto.title.SearchTitlesResponse.prototype.getSummary = function() {
  return /** @type{?proto.title.SearchSummary} */ (
    jspb.Message.getWrapperField(this, proto.title.SearchSummary, 1));
};


/**
 * @param {?proto.title.SearchSummary|undefined} value
 * @return {!proto.title.SearchTitlesResponse} returns this
*/
proto.title.SearchTitlesResponse.prototype.setSummary = function(value) {
  return jspb.Message.setOneofWrapperField(this, 1, proto.title.SearchTitlesResponse.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.SearchTitlesResponse} returns this
 */
proto.title.SearchTitlesResponse.prototype.clearSummary = function() {
  return this.setSummary(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.SearchTitlesResponse.prototype.hasSummary = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional SearchHit hit = 2;
 * @return {?proto.title.SearchHit}
 */
proto.title.SearchTitlesResponse.prototype.getHit = function() {
  return /** @type{?proto.title.SearchHit} */ (
    jspb.Message.getWrapperField(this, proto.title.SearchHit, 2));
};


/**
 * @param {?proto.title.SearchHit|undefined} value
 * @return {!proto.title.SearchTitlesResponse} returns this
*/
proto.title.SearchTitlesResponse.prototype.setHit = function(value) {
  return jspb.Message.setOneofWrapperField(this, 2, proto.title.SearchTitlesResponse.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.SearchTitlesResponse} returns this
 */
proto.title.SearchTitlesResponse.prototype.clearHit = function() {
  return this.setHit(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.SearchTitlesResponse.prototype.hasHit = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional SearchFacets facets = 3;
 * @return {?proto.title.SearchFacets}
 */
proto.title.SearchTitlesResponse.prototype.getFacets = function() {
  return /** @type{?proto.title.SearchFacets} */ (
    jspb.Message.getWrapperField(this, proto.title.SearchFacets, 3));
};


/**
 * @param {?proto.title.SearchFacets|undefined} value
 * @return {!proto.title.SearchTitlesResponse} returns this
*/
proto.title.SearchTitlesResponse.prototype.setFacets = function(value) {
  return jspb.Message.setOneofWrapperField(this, 3, proto.title.SearchTitlesResponse.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.SearchTitlesResponse} returns this
 */
proto.title.SearchTitlesResponse.prototype.clearFacets = function() {
  return this.setFacets(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.SearchTitlesResponse.prototype.hasFacets = function() {
  return jspb.Message.getField(this, 3) != null;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.title.SearchPersonsRequest.repeatedFields_ = [2];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.SearchPersonsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.SearchPersonsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.SearchPersonsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.SearchPersonsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    query: jspb.Message.getFieldWithDefault(msg, 1, ""),
    fieldsList: (f = jspb.Message.getRepeatedField(msg, 2)) == null ? undefined : f,
    indexpath: jspb.Message.getFieldWithDefault(msg, 3, ""),
    size: jspb.Message.getFieldWithDefault(msg, 4, 0),
    offset: jspb.Message.getFieldWithDefault(msg, 5, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.SearchPersonsRequest}
 */
proto.title.SearchPersonsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.SearchPersonsRequest;
  return proto.title.SearchPersonsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.SearchPersonsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.SearchPersonsRequest}
 */
proto.title.SearchPersonsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setQuery(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.addFields(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setSize(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setOffset(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.SearchPersonsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.SearchPersonsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.SearchPersonsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.SearchPersonsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getQuery();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getFieldsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      2,
      f
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getSize();
  if (f !== 0) {
    writer.writeInt32(
      4,
      f
    );
  }
  f = message.getOffset();
  if (f !== 0) {
    writer.writeInt32(
      5,
      f
    );
  }
};


/**
 * optional string query = 1;
 * @return {string}
 */
proto.title.SearchPersonsRequest.prototype.getQuery = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.SearchPersonsRequest} returns this
 */
proto.title.SearchPersonsRequest.prototype.setQuery = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * repeated string fields = 2;
 * @return {!Array<string>}
 */
proto.title.SearchPersonsRequest.prototype.getFieldsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.title.SearchPersonsRequest} returns this
 */
proto.title.SearchPersonsRequest.prototype.setFieldsList = function(value) {
  return jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.title.SearchPersonsRequest} returns this
 */
proto.title.SearchPersonsRequest.prototype.addFields = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.SearchPersonsRequest} returns this
 */
proto.title.SearchPersonsRequest.prototype.clearFieldsList = function() {
  return this.setFieldsList([]);
};


/**
 * optional string indexPath = 3;
 * @return {string}
 */
proto.title.SearchPersonsRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.SearchPersonsRequest} returns this
 */
proto.title.SearchPersonsRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional int32 size = 4;
 * @return {number}
 */
proto.title.SearchPersonsRequest.prototype.getSize = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.SearchPersonsRequest} returns this
 */
proto.title.SearchPersonsRequest.prototype.setSize = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional int32 offset = 5;
 * @return {number}
 */
proto.title.SearchPersonsRequest.prototype.getOffset = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.SearchPersonsRequest} returns this
 */
proto.title.SearchPersonsRequest.prototype.setOffset = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};



/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.title.SearchPersonsResponse.oneofGroups_ = [[1,2,3]];

/**
 * @enum {number}
 */
proto.title.SearchPersonsResponse.ResultCase = {
  RESULT_NOT_SET: 0,
  SUMMARY: 1,
  HIT: 2,
  FACETS: 3
};

/**
 * @return {proto.title.SearchPersonsResponse.ResultCase}
 */
proto.title.SearchPersonsResponse.prototype.getResultCase = function() {
  return /** @type {proto.title.SearchPersonsResponse.ResultCase} */(jspb.Message.computeOneofCase(this, proto.title.SearchPersonsResponse.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.SearchPersonsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.SearchPersonsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.SearchPersonsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.SearchPersonsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    summary: (f = msg.getSummary()) && proto.title.SearchSummary.toObject(includeInstance, f),
    hit: (f = msg.getHit()) && proto.title.SearchHit.toObject(includeInstance, f),
    facets: (f = msg.getFacets()) && proto.title.SearchFacets.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.SearchPersonsResponse}
 */
proto.title.SearchPersonsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.SearchPersonsResponse;
  return proto.title.SearchPersonsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.SearchPersonsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.SearchPersonsResponse}
 */
proto.title.SearchPersonsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.SearchSummary;
      reader.readMessage(value,proto.title.SearchSummary.deserializeBinaryFromReader);
      msg.setSummary(value);
      break;
    case 2:
      var value = new proto.title.SearchHit;
      reader.readMessage(value,proto.title.SearchHit.deserializeBinaryFromReader);
      msg.setHit(value);
      break;
    case 3:
      var value = new proto.title.SearchFacets;
      reader.readMessage(value,proto.title.SearchFacets.deserializeBinaryFromReader);
      msg.setFacets(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.SearchPersonsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.SearchPersonsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.SearchPersonsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.SearchPersonsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSummary();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.title.SearchSummary.serializeBinaryToWriter
    );
  }
  f = message.getHit();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.title.SearchHit.serializeBinaryToWriter
    );
  }
  f = message.getFacets();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.title.SearchFacets.serializeBinaryToWriter
    );
  }
};


/**
 * optional SearchSummary summary = 1;
 * @return {?proto.title.SearchSummary}
 */
proto.title.SearchPersonsResponse.prototype.getSummary = function() {
  return /** @type{?proto.title.SearchSummary} */ (
    jspb.Message.getWrapperField(this, proto.title.SearchSummary, 1));
};


/**
 * @param {?proto.title.SearchSummary|undefined} value
 * @return {!proto.title.SearchPersonsResponse} returns this
*/
proto.title.SearchPersonsResponse.prototype.setSummary = function(value) {
  return jspb.Message.setOneofWrapperField(this, 1, proto.title.SearchPersonsResponse.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.SearchPersonsResponse} returns this
 */
proto.title.SearchPersonsResponse.prototype.clearSummary = function() {
  return this.setSummary(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.SearchPersonsResponse.prototype.hasSummary = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional SearchHit hit = 2;
 * @return {?proto.title.SearchHit}
 */
proto.title.SearchPersonsResponse.prototype.getHit = function() {
  return /** @type{?proto.title.SearchHit} */ (
    jspb.Message.getWrapperField(this, proto.title.SearchHit, 2));
};


/**
 * @param {?proto.title.SearchHit|undefined} value
 * @return {!proto.title.SearchPersonsResponse} returns this
*/
proto.title.SearchPersonsResponse.prototype.setHit = function(value) {
  return jspb.Message.setOneofWrapperField(this, 2, proto.title.SearchPersonsResponse.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.SearchPersonsResponse} returns this
 */
proto.title.SearchPersonsResponse.prototype.clearHit = function() {
  return this.setHit(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.SearchPersonsResponse.prototype.hasHit = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional SearchFacets facets = 3;
 * @return {?proto.title.SearchFacets}
 */
proto.title.SearchPersonsResponse.prototype.getFacets = function() {
  return /** @type{?proto.title.SearchFacets} */ (
    jspb.Message.getWrapperField(this, proto.title.SearchFacets, 3));
};


/**
 * @param {?proto.title.SearchFacets|undefined} value
 * @return {!proto.title.SearchPersonsResponse} returns this
*/
proto.title.SearchPersonsResponse.prototype.setFacets = function(value) {
  return jspb.Message.setOneofWrapperField(this, 3, proto.title.SearchPersonsResponse.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.SearchPersonsResponse} returns this
 */
proto.title.SearchPersonsResponse.prototype.clearFacets = function() {
  return this.setFacets(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.SearchPersonsResponse.prototype.hasFacets = function() {
  return jspb.Message.getField(this, 3) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.CreatePublisherRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.CreatePublisherRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.CreatePublisherRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.CreatePublisherRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    publisher: (f = msg.getPublisher()) && proto.title.Publisher.toObject(includeInstance, f),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.CreatePublisherRequest}
 */
proto.title.CreatePublisherRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.CreatePublisherRequest;
  return proto.title.CreatePublisherRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.CreatePublisherRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.CreatePublisherRequest}
 */
proto.title.CreatePublisherRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.Publisher;
      reader.readMessage(value,proto.title.Publisher.deserializeBinaryFromReader);
      msg.setPublisher(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.CreatePublisherRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.CreatePublisherRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.CreatePublisherRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.CreatePublisherRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublisher();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.title.Publisher.serializeBinaryToWriter
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional Publisher publisher = 1;
 * @return {?proto.title.Publisher}
 */
proto.title.CreatePublisherRequest.prototype.getPublisher = function() {
  return /** @type{?proto.title.Publisher} */ (
    jspb.Message.getWrapperField(this, proto.title.Publisher, 1));
};


/**
 * @param {?proto.title.Publisher|undefined} value
 * @return {!proto.title.CreatePublisherRequest} returns this
*/
proto.title.CreatePublisherRequest.prototype.setPublisher = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.CreatePublisherRequest} returns this
 */
proto.title.CreatePublisherRequest.prototype.clearPublisher = function() {
  return this.setPublisher(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.CreatePublisherRequest.prototype.hasPublisher = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.CreatePublisherRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.CreatePublisherRequest} returns this
 */
proto.title.CreatePublisherRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.CreatePublisherResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.CreatePublisherResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.CreatePublisherResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.CreatePublisherResponse.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.CreatePublisherResponse}
 */
proto.title.CreatePublisherResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.CreatePublisherResponse;
  return proto.title.CreatePublisherResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.CreatePublisherResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.CreatePublisherResponse}
 */
proto.title.CreatePublisherResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.CreatePublisherResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.CreatePublisherResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.CreatePublisherResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.CreatePublisherResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.DeletePublisherRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.DeletePublisherRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.DeletePublisherRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeletePublisherRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    publisherid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.DeletePublisherRequest}
 */
proto.title.DeletePublisherRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.DeletePublisherRequest;
  return proto.title.DeletePublisherRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.DeletePublisherRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.DeletePublisherRequest}
 */
proto.title.DeletePublisherRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisherid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.DeletePublisherRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.DeletePublisherRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.DeletePublisherRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeletePublisherRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublisherid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string publisherId = 1;
 * @return {string}
 */
proto.title.DeletePublisherRequest.prototype.getPublisherid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.DeletePublisherRequest} returns this
 */
proto.title.DeletePublisherRequest.prototype.setPublisherid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.DeletePublisherRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.DeletePublisherRequest} returns this
 */
proto.title.DeletePublisherRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.DeletePublisherResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.DeletePublisherResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.DeletePublisherResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeletePublisherResponse.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.DeletePublisherResponse}
 */
proto.title.DeletePublisherResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.DeletePublisherResponse;
  return proto.title.DeletePublisherResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.DeletePublisherResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.DeletePublisherResponse}
 */
proto.title.DeletePublisherResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.DeletePublisherResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.DeletePublisherResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.DeletePublisherResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeletePublisherResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.GetPublisherByIdRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.GetPublisherByIdRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.GetPublisherByIdRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetPublisherByIdRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    publisherid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.GetPublisherByIdRequest}
 */
proto.title.GetPublisherByIdRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.GetPublisherByIdRequest;
  return proto.title.GetPublisherByIdRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.GetPublisherByIdRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.GetPublisherByIdRequest}
 */
proto.title.GetPublisherByIdRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisherid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.GetPublisherByIdRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.GetPublisherByIdRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.GetPublisherByIdRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetPublisherByIdRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublisherid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string publisherId = 1;
 * @return {string}
 */
proto.title.GetPublisherByIdRequest.prototype.getPublisherid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.GetPublisherByIdRequest} returns this
 */
proto.title.GetPublisherByIdRequest.prototype.setPublisherid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.GetPublisherByIdRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.GetPublisherByIdRequest} returns this
 */
proto.title.GetPublisherByIdRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.GetPublisherByIdResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.GetPublisherByIdResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.GetPublisherByIdResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetPublisherByIdResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    publisher: (f = msg.getPublisher()) && proto.title.Publisher.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.GetPublisherByIdResponse}
 */
proto.title.GetPublisherByIdResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.GetPublisherByIdResponse;
  return proto.title.GetPublisherByIdResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.GetPublisherByIdResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.GetPublisherByIdResponse}
 */
proto.title.GetPublisherByIdResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.Publisher;
      reader.readMessage(value,proto.title.Publisher.deserializeBinaryFromReader);
      msg.setPublisher(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.GetPublisherByIdResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.GetPublisherByIdResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.GetPublisherByIdResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetPublisherByIdResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublisher();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.title.Publisher.serializeBinaryToWriter
    );
  }
};


/**
 * optional Publisher publisher = 1;
 * @return {?proto.title.Publisher}
 */
proto.title.GetPublisherByIdResponse.prototype.getPublisher = function() {
  return /** @type{?proto.title.Publisher} */ (
    jspb.Message.getWrapperField(this, proto.title.Publisher, 1));
};


/**
 * @param {?proto.title.Publisher|undefined} value
 * @return {!proto.title.GetPublisherByIdResponse} returns this
*/
proto.title.GetPublisherByIdResponse.prototype.setPublisher = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.GetPublisherByIdResponse} returns this
 */
proto.title.GetPublisherByIdResponse.prototype.clearPublisher = function() {
  return this.setPublisher(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.GetPublisherByIdResponse.prototype.hasPublisher = function() {
  return jspb.Message.getField(this, 1) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.CreatePersonRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.CreatePersonRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.CreatePersonRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.CreatePersonRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    person: (f = msg.getPerson()) && proto.title.Person.toObject(includeInstance, f),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.CreatePersonRequest}
 */
proto.title.CreatePersonRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.CreatePersonRequest;
  return proto.title.CreatePersonRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.CreatePersonRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.CreatePersonRequest}
 */
proto.title.CreatePersonRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.Person;
      reader.readMessage(value,proto.title.Person.deserializeBinaryFromReader);
      msg.setPerson(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.CreatePersonRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.CreatePersonRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.CreatePersonRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.CreatePersonRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPerson();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.title.Person.serializeBinaryToWriter
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional Person person = 1;
 * @return {?proto.title.Person}
 */
proto.title.CreatePersonRequest.prototype.getPerson = function() {
  return /** @type{?proto.title.Person} */ (
    jspb.Message.getWrapperField(this, proto.title.Person, 1));
};


/**
 * @param {?proto.title.Person|undefined} value
 * @return {!proto.title.CreatePersonRequest} returns this
*/
proto.title.CreatePersonRequest.prototype.setPerson = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.CreatePersonRequest} returns this
 */
proto.title.CreatePersonRequest.prototype.clearPerson = function() {
  return this.setPerson(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.CreatePersonRequest.prototype.hasPerson = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.CreatePersonRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.CreatePersonRequest} returns this
 */
proto.title.CreatePersonRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.CreatePersonResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.CreatePersonResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.CreatePersonResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.CreatePersonResponse.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.CreatePersonResponse}
 */
proto.title.CreatePersonResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.CreatePersonResponse;
  return proto.title.CreatePersonResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.CreatePersonResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.CreatePersonResponse}
 */
proto.title.CreatePersonResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.CreatePersonResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.CreatePersonResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.CreatePersonResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.CreatePersonResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.DeletePersonRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.DeletePersonRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.DeletePersonRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeletePersonRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    personid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.DeletePersonRequest}
 */
proto.title.DeletePersonRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.DeletePersonRequest;
  return proto.title.DeletePersonRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.DeletePersonRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.DeletePersonRequest}
 */
proto.title.DeletePersonRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPersonid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.DeletePersonRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.DeletePersonRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.DeletePersonRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeletePersonRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPersonid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string personId = 1;
 * @return {string}
 */
proto.title.DeletePersonRequest.prototype.getPersonid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.DeletePersonRequest} returns this
 */
proto.title.DeletePersonRequest.prototype.setPersonid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.DeletePersonRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.DeletePersonRequest} returns this
 */
proto.title.DeletePersonRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.DeletePersonResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.DeletePersonResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.DeletePersonResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeletePersonResponse.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.DeletePersonResponse}
 */
proto.title.DeletePersonResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.DeletePersonResponse;
  return proto.title.DeletePersonResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.DeletePersonResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.DeletePersonResponse}
 */
proto.title.DeletePersonResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.DeletePersonResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.DeletePersonResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.DeletePersonResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeletePersonResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.GetPersonByIdRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.GetPersonByIdRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.GetPersonByIdRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetPersonByIdRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    personid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.GetPersonByIdRequest}
 */
proto.title.GetPersonByIdRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.GetPersonByIdRequest;
  return proto.title.GetPersonByIdRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.GetPersonByIdRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.GetPersonByIdRequest}
 */
proto.title.GetPersonByIdRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPersonid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.GetPersonByIdRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.GetPersonByIdRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.GetPersonByIdRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetPersonByIdRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPersonid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string personId = 1;
 * @return {string}
 */
proto.title.GetPersonByIdRequest.prototype.getPersonid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.GetPersonByIdRequest} returns this
 */
proto.title.GetPersonByIdRequest.prototype.setPersonid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.GetPersonByIdRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.GetPersonByIdRequest} returns this
 */
proto.title.GetPersonByIdRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.GetPersonByIdResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.GetPersonByIdResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.GetPersonByIdResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetPersonByIdResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    person: (f = msg.getPerson()) && proto.title.Person.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.GetPersonByIdResponse}
 */
proto.title.GetPersonByIdResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.GetPersonByIdResponse;
  return proto.title.GetPersonByIdResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.GetPersonByIdResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.GetPersonByIdResponse}
 */
proto.title.GetPersonByIdResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.Person;
      reader.readMessage(value,proto.title.Person.deserializeBinaryFromReader);
      msg.setPerson(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.GetPersonByIdResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.GetPersonByIdResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.GetPersonByIdResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetPersonByIdResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPerson();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.title.Person.serializeBinaryToWriter
    );
  }
};


/**
 * optional Person person = 1;
 * @return {?proto.title.Person}
 */
proto.title.GetPersonByIdResponse.prototype.getPerson = function() {
  return /** @type{?proto.title.Person} */ (
    jspb.Message.getWrapperField(this, proto.title.Person, 1));
};


/**
 * @param {?proto.title.Person|undefined} value
 * @return {!proto.title.GetPersonByIdResponse} returns this
*/
proto.title.GetPersonByIdResponse.prototype.setPerson = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.GetPersonByIdResponse} returns this
 */
proto.title.GetPersonByIdResponse.prototype.clearPerson = function() {
  return this.setPerson(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.GetPersonByIdResponse.prototype.hasPerson = function() {
  return jspb.Message.getField(this, 1) != null;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.title.Audio.repeatedFields_ = [8];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.Audio.prototype.toObject = function(opt_includeInstance) {
  return proto.title.Audio.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.Audio} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Audio.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, ""),
    url: jspb.Message.getFieldWithDefault(msg, 2, ""),
    artist: jspb.Message.getFieldWithDefault(msg, 3, ""),
    albumartist: jspb.Message.getFieldWithDefault(msg, 4, ""),
    album: jspb.Message.getFieldWithDefault(msg, 5, ""),
    comment: jspb.Message.getFieldWithDefault(msg, 6, ""),
    composer: jspb.Message.getFieldWithDefault(msg, 7, ""),
    genresList: (f = jspb.Message.getRepeatedField(msg, 8)) == null ? undefined : f,
    lyrics: jspb.Message.getFieldWithDefault(msg, 9, ""),
    title: jspb.Message.getFieldWithDefault(msg, 10, ""),
    year: jspb.Message.getFieldWithDefault(msg, 11, 0),
    discnumber: jspb.Message.getFieldWithDefault(msg, 12, 0),
    disctotal: jspb.Message.getFieldWithDefault(msg, 13, 0),
    tracknumber: jspb.Message.getFieldWithDefault(msg, 14, 0),
    tracktotal: jspb.Message.getFieldWithDefault(msg, 15, 0),
    poster: (f = msg.getPoster()) && proto.title.Poster.toObject(includeInstance, f),
    duration: jspb.Message.getFieldWithDefault(msg, 17, 0),
    uuid: jspb.Message.getFieldWithDefault(msg, 19, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.Audio}
 */
proto.title.Audio.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.Audio;
  return proto.title.Audio.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.Audio} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.Audio}
 */
proto.title.Audio.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setUrl(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setArtist(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setAlbumartist(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setAlbum(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setComment(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setComposer(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.addGenres(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setLyrics(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.setTitle(value);
      break;
    case 11:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setYear(value);
      break;
    case 12:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setDiscnumber(value);
      break;
    case 13:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setDisctotal(value);
      break;
    case 14:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setTracknumber(value);
      break;
    case 15:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setTracktotal(value);
      break;
    case 16:
      var value = new proto.title.Poster;
      reader.readMessage(value,proto.title.Poster.deserializeBinaryFromReader);
      msg.setPoster(value);
      break;
    case 17:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setDuration(value);
      break;
    case 19:
      var value = /** @type {string} */ (reader.readString());
      msg.setUuid(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.Audio.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.Audio.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.Audio} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Audio.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getUrl();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getArtist();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getAlbumartist();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getAlbum();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getComment();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getComposer();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getGenresList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      8,
      f
    );
  }
  f = message.getLyrics();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
  f = message.getTitle();
  if (f.length > 0) {
    writer.writeString(
      10,
      f
    );
  }
  f = message.getYear();
  if (f !== 0) {
    writer.writeInt32(
      11,
      f
    );
  }
  f = message.getDiscnumber();
  if (f !== 0) {
    writer.writeInt32(
      12,
      f
    );
  }
  f = message.getDisctotal();
  if (f !== 0) {
    writer.writeInt32(
      13,
      f
    );
  }
  f = message.getTracknumber();
  if (f !== 0) {
    writer.writeInt32(
      14,
      f
    );
  }
  f = message.getTracktotal();
  if (f !== 0) {
    writer.writeInt32(
      15,
      f
    );
  }
  f = message.getPoster();
  if (f != null) {
    writer.writeMessage(
      16,
      f,
      proto.title.Poster.serializeBinaryToWriter
    );
  }
  f = message.getDuration();
  if (f !== 0) {
    writer.writeInt32(
      17,
      f
    );
  }
  f = message.getUuid();
  if (f.length > 0) {
    writer.writeString(
      19,
      f
    );
  }
};


/**
 * optional string ID = 1;
 * @return {string}
 */
proto.title.Audio.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Audio} returns this
 */
proto.title.Audio.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string URL = 2;
 * @return {string}
 */
proto.title.Audio.prototype.getUrl = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Audio} returns this
 */
proto.title.Audio.prototype.setUrl = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string Artist = 3;
 * @return {string}
 */
proto.title.Audio.prototype.getArtist = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Audio} returns this
 */
proto.title.Audio.prototype.setArtist = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string AlbumArtist = 4;
 * @return {string}
 */
proto.title.Audio.prototype.getAlbumartist = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Audio} returns this
 */
proto.title.Audio.prototype.setAlbumartist = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string Album = 5;
 * @return {string}
 */
proto.title.Audio.prototype.getAlbum = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Audio} returns this
 */
proto.title.Audio.prototype.setAlbum = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string Comment = 6;
 * @return {string}
 */
proto.title.Audio.prototype.getComment = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Audio} returns this
 */
proto.title.Audio.prototype.setComment = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string Composer = 7;
 * @return {string}
 */
proto.title.Audio.prototype.getComposer = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Audio} returns this
 */
proto.title.Audio.prototype.setComposer = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * repeated string Genres = 8;
 * @return {!Array<string>}
 */
proto.title.Audio.prototype.getGenresList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 8));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.title.Audio} returns this
 */
proto.title.Audio.prototype.setGenresList = function(value) {
  return jspb.Message.setField(this, 8, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.title.Audio} returns this
 */
proto.title.Audio.prototype.addGenres = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 8, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.Audio} returns this
 */
proto.title.Audio.prototype.clearGenresList = function() {
  return this.setGenresList([]);
};


/**
 * optional string Lyrics = 9;
 * @return {string}
 */
proto.title.Audio.prototype.getLyrics = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Audio} returns this
 */
proto.title.Audio.prototype.setLyrics = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};


/**
 * optional string Title = 10;
 * @return {string}
 */
proto.title.Audio.prototype.getTitle = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 10, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Audio} returns this
 */
proto.title.Audio.prototype.setTitle = function(value) {
  return jspb.Message.setProto3StringField(this, 10, value);
};


/**
 * optional int32 Year = 11;
 * @return {number}
 */
proto.title.Audio.prototype.getYear = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 11, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.Audio} returns this
 */
proto.title.Audio.prototype.setYear = function(value) {
  return jspb.Message.setProto3IntField(this, 11, value);
};


/**
 * optional int32 DiscNumber = 12;
 * @return {number}
 */
proto.title.Audio.prototype.getDiscnumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 12, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.Audio} returns this
 */
proto.title.Audio.prototype.setDiscnumber = function(value) {
  return jspb.Message.setProto3IntField(this, 12, value);
};


/**
 * optional int32 DiscTotal = 13;
 * @return {number}
 */
proto.title.Audio.prototype.getDisctotal = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 13, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.Audio} returns this
 */
proto.title.Audio.prototype.setDisctotal = function(value) {
  return jspb.Message.setProto3IntField(this, 13, value);
};


/**
 * optional int32 TrackNumber = 14;
 * @return {number}
 */
proto.title.Audio.prototype.getTracknumber = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 14, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.Audio} returns this
 */
proto.title.Audio.prototype.setTracknumber = function(value) {
  return jspb.Message.setProto3IntField(this, 14, value);
};


/**
 * optional int32 TrackTotal = 15;
 * @return {number}
 */
proto.title.Audio.prototype.getTracktotal = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 15, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.Audio} returns this
 */
proto.title.Audio.prototype.setTracktotal = function(value) {
  return jspb.Message.setProto3IntField(this, 15, value);
};


/**
 * optional Poster Poster = 16;
 * @return {?proto.title.Poster}
 */
proto.title.Audio.prototype.getPoster = function() {
  return /** @type{?proto.title.Poster} */ (
    jspb.Message.getWrapperField(this, proto.title.Poster, 16));
};


/**
 * @param {?proto.title.Poster|undefined} value
 * @return {!proto.title.Audio} returns this
*/
proto.title.Audio.prototype.setPoster = function(value) {
  return jspb.Message.setWrapperField(this, 16, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.Audio} returns this
 */
proto.title.Audio.prototype.clearPoster = function() {
  return this.setPoster(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.Audio.prototype.hasPoster = function() {
  return jspb.Message.getField(this, 16) != null;
};


/**
 * optional int32 Duration = 17;
 * @return {number}
 */
proto.title.Audio.prototype.getDuration = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 17, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.Audio} returns this
 */
proto.title.Audio.prototype.setDuration = function(value) {
  return jspb.Message.setProto3IntField(this, 17, value);
};


/**
 * optional string UUID = 19;
 * @return {string}
 */
proto.title.Audio.prototype.getUuid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 19, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Audio} returns this
 */
proto.title.Audio.prototype.setUuid = function(value) {
  return jspb.Message.setProto3StringField(this, 19, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.title.Album.repeatedFields_ = [4];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.Album.prototype.toObject = function(opt_includeInstance) {
  return proto.title.Album.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.Album} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Album.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, ""),
    artist: jspb.Message.getFieldWithDefault(msg, 2, ""),
    year: jspb.Message.getFieldWithDefault(msg, 3, 0),
    genresList: (f = jspb.Message.getRepeatedField(msg, 4)) == null ? undefined : f,
    tracks: (f = msg.getTracks()) && proto.title.Audios.toObject(includeInstance, f),
    poster: (f = msg.getPoster()) && proto.title.Poster.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.Album}
 */
proto.title.Album.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.Album;
  return proto.title.Album.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.Album} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.Album}
 */
proto.title.Album.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setArtist(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setYear(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.addGenres(value);
      break;
    case 5:
      var value = new proto.title.Audios;
      reader.readMessage(value,proto.title.Audios.deserializeBinaryFromReader);
      msg.setTracks(value);
      break;
    case 6:
      var value = new proto.title.Poster;
      reader.readMessage(value,proto.title.Poster.deserializeBinaryFromReader);
      msg.setPoster(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.Album.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.Album.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.Album} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Album.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getArtist();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getYear();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
  f = message.getGenresList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      4,
      f
    );
  }
  f = message.getTracks();
  if (f != null) {
    writer.writeMessage(
      5,
      f,
      proto.title.Audios.serializeBinaryToWriter
    );
  }
  f = message.getPoster();
  if (f != null) {
    writer.writeMessage(
      6,
      f,
      proto.title.Poster.serializeBinaryToWriter
    );
  }
};


/**
 * optional string ID = 1;
 * @return {string}
 */
proto.title.Album.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Album} returns this
 */
proto.title.Album.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string Artist = 2;
 * @return {string}
 */
proto.title.Album.prototype.getArtist = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.Album} returns this
 */
proto.title.Album.prototype.setArtist = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int32 Year = 3;
 * @return {number}
 */
proto.title.Album.prototype.getYear = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.title.Album} returns this
 */
proto.title.Album.prototype.setYear = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * repeated string Genres = 4;
 * @return {!Array<string>}
 */
proto.title.Album.prototype.getGenresList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 4));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.title.Album} returns this
 */
proto.title.Album.prototype.setGenresList = function(value) {
  return jspb.Message.setField(this, 4, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.title.Album} returns this
 */
proto.title.Album.prototype.addGenres = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 4, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.Album} returns this
 */
proto.title.Album.prototype.clearGenresList = function() {
  return this.setGenresList([]);
};


/**
 * optional Audios tracks = 5;
 * @return {?proto.title.Audios}
 */
proto.title.Album.prototype.getTracks = function() {
  return /** @type{?proto.title.Audios} */ (
    jspb.Message.getWrapperField(this, proto.title.Audios, 5));
};


/**
 * @param {?proto.title.Audios|undefined} value
 * @return {!proto.title.Album} returns this
*/
proto.title.Album.prototype.setTracks = function(value) {
  return jspb.Message.setWrapperField(this, 5, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.Album} returns this
 */
proto.title.Album.prototype.clearTracks = function() {
  return this.setTracks(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.Album.prototype.hasTracks = function() {
  return jspb.Message.getField(this, 5) != null;
};


/**
 * optional Poster Poster = 6;
 * @return {?proto.title.Poster}
 */
proto.title.Album.prototype.getPoster = function() {
  return /** @type{?proto.title.Poster} */ (
    jspb.Message.getWrapperField(this, proto.title.Poster, 6));
};


/**
 * @param {?proto.title.Poster|undefined} value
 * @return {!proto.title.Album} returns this
*/
proto.title.Album.prototype.setPoster = function(value) {
  return jspb.Message.setWrapperField(this, 6, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.Album} returns this
 */
proto.title.Album.prototype.clearPoster = function() {
  return this.setPoster(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.Album.prototype.hasPoster = function() {
  return jspb.Message.getField(this, 6) != null;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.title.Audios.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.Audios.prototype.toObject = function(opt_includeInstance) {
  return proto.title.Audios.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.Audios} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Audios.toObject = function(includeInstance, msg) {
  var f, obj = {
    audiosList: jspb.Message.toObjectList(msg.getAudiosList(),
    proto.title.Audio.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.Audios}
 */
proto.title.Audios.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.Audios;
  return proto.title.Audios.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.Audios} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.Audios}
 */
proto.title.Audios.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.Audio;
      reader.readMessage(value,proto.title.Audio.deserializeBinaryFromReader);
      msg.addAudios(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.Audios.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.Audios.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.Audios} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.Audios.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAudiosList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.title.Audio.serializeBinaryToWriter
    );
  }
};


/**
 * repeated Audio audios = 1;
 * @return {!Array<!proto.title.Audio>}
 */
proto.title.Audios.prototype.getAudiosList = function() {
  return /** @type{!Array<!proto.title.Audio>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.title.Audio, 1));
};


/**
 * @param {!Array<!proto.title.Audio>} value
 * @return {!proto.title.Audios} returns this
*/
proto.title.Audios.prototype.setAudiosList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.title.Audio=} opt_value
 * @param {number=} opt_index
 * @return {!proto.title.Audio}
 */
proto.title.Audios.prototype.addAudios = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.title.Audio, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.Audios} returns this
 */
proto.title.Audios.prototype.clearAudiosList = function() {
  return this.setAudiosList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.CreateAudioRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.CreateAudioRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.CreateAudioRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.CreateAudioRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    audio: (f = msg.getAudio()) && proto.title.Audio.toObject(includeInstance, f),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.CreateAudioRequest}
 */
proto.title.CreateAudioRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.CreateAudioRequest;
  return proto.title.CreateAudioRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.CreateAudioRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.CreateAudioRequest}
 */
proto.title.CreateAudioRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.Audio;
      reader.readMessage(value,proto.title.Audio.deserializeBinaryFromReader);
      msg.setAudio(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.CreateAudioRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.CreateAudioRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.CreateAudioRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.CreateAudioRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAudio();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.title.Audio.serializeBinaryToWriter
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional Audio audio = 1;
 * @return {?proto.title.Audio}
 */
proto.title.CreateAudioRequest.prototype.getAudio = function() {
  return /** @type{?proto.title.Audio} */ (
    jspb.Message.getWrapperField(this, proto.title.Audio, 1));
};


/**
 * @param {?proto.title.Audio|undefined} value
 * @return {!proto.title.CreateAudioRequest} returns this
*/
proto.title.CreateAudioRequest.prototype.setAudio = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.CreateAudioRequest} returns this
 */
proto.title.CreateAudioRequest.prototype.clearAudio = function() {
  return this.setAudio(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.CreateAudioRequest.prototype.hasAudio = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.CreateAudioRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.CreateAudioRequest} returns this
 */
proto.title.CreateAudioRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.CreateAudioResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.CreateAudioResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.CreateAudioResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.CreateAudioResponse.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.CreateAudioResponse}
 */
proto.title.CreateAudioResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.CreateAudioResponse;
  return proto.title.CreateAudioResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.CreateAudioResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.CreateAudioResponse}
 */
proto.title.CreateAudioResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.CreateAudioResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.CreateAudioResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.CreateAudioResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.CreateAudioResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.GetAudioByIdRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.GetAudioByIdRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.GetAudioByIdRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetAudioByIdRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    audioid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.GetAudioByIdRequest}
 */
proto.title.GetAudioByIdRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.GetAudioByIdRequest;
  return proto.title.GetAudioByIdRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.GetAudioByIdRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.GetAudioByIdRequest}
 */
proto.title.GetAudioByIdRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setAudioid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.GetAudioByIdRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.GetAudioByIdRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.GetAudioByIdRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetAudioByIdRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAudioid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string audioId = 1;
 * @return {string}
 */
proto.title.GetAudioByIdRequest.prototype.getAudioid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.GetAudioByIdRequest} returns this
 */
proto.title.GetAudioByIdRequest.prototype.setAudioid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.GetAudioByIdRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.GetAudioByIdRequest} returns this
 */
proto.title.GetAudioByIdRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.title.GetAudioByIdResponse.repeatedFields_ = [2];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.GetAudioByIdResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.GetAudioByIdResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.GetAudioByIdResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetAudioByIdResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    audio: (f = msg.getAudio()) && proto.title.Audio.toObject(includeInstance, f),
    filespathsList: (f = jspb.Message.getRepeatedField(msg, 2)) == null ? undefined : f
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.GetAudioByIdResponse}
 */
proto.title.GetAudioByIdResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.GetAudioByIdResponse;
  return proto.title.GetAudioByIdResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.GetAudioByIdResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.GetAudioByIdResponse}
 */
proto.title.GetAudioByIdResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.Audio;
      reader.readMessage(value,proto.title.Audio.deserializeBinaryFromReader);
      msg.setAudio(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.addFilespaths(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.GetAudioByIdResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.GetAudioByIdResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.GetAudioByIdResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetAudioByIdResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAudio();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.title.Audio.serializeBinaryToWriter
    );
  }
  f = message.getFilespathsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      2,
      f
    );
  }
};


/**
 * optional Audio audio = 1;
 * @return {?proto.title.Audio}
 */
proto.title.GetAudioByIdResponse.prototype.getAudio = function() {
  return /** @type{?proto.title.Audio} */ (
    jspb.Message.getWrapperField(this, proto.title.Audio, 1));
};


/**
 * @param {?proto.title.Audio|undefined} value
 * @return {!proto.title.GetAudioByIdResponse} returns this
*/
proto.title.GetAudioByIdResponse.prototype.setAudio = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.GetAudioByIdResponse} returns this
 */
proto.title.GetAudioByIdResponse.prototype.clearAudio = function() {
  return this.setAudio(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.GetAudioByIdResponse.prototype.hasAudio = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * repeated string filesPaths = 2;
 * @return {!Array<string>}
 */
proto.title.GetAudioByIdResponse.prototype.getFilespathsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.title.GetAudioByIdResponse} returns this
 */
proto.title.GetAudioByIdResponse.prototype.setFilespathsList = function(value) {
  return jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.title.GetAudioByIdResponse} returns this
 */
proto.title.GetAudioByIdResponse.prototype.addFilespaths = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.title.GetAudioByIdResponse} returns this
 */
proto.title.GetAudioByIdResponse.prototype.clearFilespathsList = function() {
  return this.setFilespathsList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.DeleteAudioRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.DeleteAudioRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.DeleteAudioRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeleteAudioRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    audioid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.DeleteAudioRequest}
 */
proto.title.DeleteAudioRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.DeleteAudioRequest;
  return proto.title.DeleteAudioRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.DeleteAudioRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.DeleteAudioRequest}
 */
proto.title.DeleteAudioRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setAudioid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.DeleteAudioRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.DeleteAudioRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.DeleteAudioRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeleteAudioRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAudioid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string audioId = 1;
 * @return {string}
 */
proto.title.DeleteAudioRequest.prototype.getAudioid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.DeleteAudioRequest} returns this
 */
proto.title.DeleteAudioRequest.prototype.setAudioid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.DeleteAudioRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.DeleteAudioRequest} returns this
 */
proto.title.DeleteAudioRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.DeleteAudioResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.DeleteAudioResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.DeleteAudioResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeleteAudioResponse.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.DeleteAudioResponse}
 */
proto.title.DeleteAudioResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.DeleteAudioResponse;
  return proto.title.DeleteAudioResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.DeleteAudioResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.DeleteAudioResponse}
 */
proto.title.DeleteAudioResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.DeleteAudioResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.DeleteAudioResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.DeleteAudioResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeleteAudioResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.GetFileAudiosRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.GetFileAudiosRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.GetFileAudiosRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetFileAudiosRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    filepath: jspb.Message.getFieldWithDefault(msg, 1, ""),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.GetFileAudiosRequest}
 */
proto.title.GetFileAudiosRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.GetFileAudiosRequest;
  return proto.title.GetFileAudiosRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.GetFileAudiosRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.GetFileAudiosRequest}
 */
proto.title.GetFileAudiosRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setFilepath(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.GetFileAudiosRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.GetFileAudiosRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.GetFileAudiosRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetFileAudiosRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getFilepath();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string filePath = 1;
 * @return {string}
 */
proto.title.GetFileAudiosRequest.prototype.getFilepath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.GetFileAudiosRequest} returns this
 */
proto.title.GetFileAudiosRequest.prototype.setFilepath = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.GetFileAudiosRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.GetFileAudiosRequest} returns this
 */
proto.title.GetFileAudiosRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.GetFileAudiosResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.GetFileAudiosResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.GetFileAudiosResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetFileAudiosResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    audios: (f = msg.getAudios()) && proto.title.Audios.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.GetFileAudiosResponse}
 */
proto.title.GetFileAudiosResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.GetFileAudiosResponse;
  return proto.title.GetFileAudiosResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.GetFileAudiosResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.GetFileAudiosResponse}
 */
proto.title.GetFileAudiosResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.Audios;
      reader.readMessage(value,proto.title.Audios.deserializeBinaryFromReader);
      msg.setAudios(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.GetFileAudiosResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.GetFileAudiosResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.GetFileAudiosResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetFileAudiosResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAudios();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.title.Audios.serializeBinaryToWriter
    );
  }
};


/**
 * optional Audios audios = 1;
 * @return {?proto.title.Audios}
 */
proto.title.GetFileAudiosResponse.prototype.getAudios = function() {
  return /** @type{?proto.title.Audios} */ (
    jspb.Message.getWrapperField(this, proto.title.Audios, 1));
};


/**
 * @param {?proto.title.Audios|undefined} value
 * @return {!proto.title.GetFileAudiosResponse} returns this
*/
proto.title.GetFileAudiosResponse.prototype.setAudios = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.GetFileAudiosResponse} returns this
 */
proto.title.GetFileAudiosResponse.prototype.clearAudios = function() {
  return this.setAudios(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.GetFileAudiosResponse.prototype.hasAudios = function() {
  return jspb.Message.getField(this, 1) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.GetAlbumRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.GetAlbumRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.GetAlbumRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetAlbumRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    albumid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.GetAlbumRequest}
 */
proto.title.GetAlbumRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.GetAlbumRequest;
  return proto.title.GetAlbumRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.GetAlbumRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.GetAlbumRequest}
 */
proto.title.GetAlbumRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setAlbumid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.GetAlbumRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.GetAlbumRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.GetAlbumRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetAlbumRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAlbumid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string albumId = 1;
 * @return {string}
 */
proto.title.GetAlbumRequest.prototype.getAlbumid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.GetAlbumRequest} returns this
 */
proto.title.GetAlbumRequest.prototype.setAlbumid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.GetAlbumRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.GetAlbumRequest} returns this
 */
proto.title.GetAlbumRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.GetAlbumResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.GetAlbumResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.GetAlbumResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetAlbumResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    album: (f = msg.getAlbum()) && proto.title.Album.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.GetAlbumResponse}
 */
proto.title.GetAlbumResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.GetAlbumResponse;
  return proto.title.GetAlbumResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.GetAlbumResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.GetAlbumResponse}
 */
proto.title.GetAlbumResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.title.Album;
      reader.readMessage(value,proto.title.Album.deserializeBinaryFromReader);
      msg.setAlbum(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.GetAlbumResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.GetAlbumResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.GetAlbumResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.GetAlbumResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAlbum();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.title.Album.serializeBinaryToWriter
    );
  }
};


/**
 * optional Album album = 1;
 * @return {?proto.title.Album}
 */
proto.title.GetAlbumResponse.prototype.getAlbum = function() {
  return /** @type{?proto.title.Album} */ (
    jspb.Message.getWrapperField(this, proto.title.Album, 1));
};


/**
 * @param {?proto.title.Album|undefined} value
 * @return {!proto.title.GetAlbumResponse} returns this
*/
proto.title.GetAlbumResponse.prototype.setAlbum = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.title.GetAlbumResponse} returns this
 */
proto.title.GetAlbumResponse.prototype.clearAlbum = function() {
  return this.setAlbum(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.title.GetAlbumResponse.prototype.hasAlbum = function() {
  return jspb.Message.getField(this, 1) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.DeleteAlbumRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.title.DeleteAlbumRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.DeleteAlbumRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeleteAlbumRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    albumid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    indexpath: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.DeleteAlbumRequest}
 */
proto.title.DeleteAlbumRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.DeleteAlbumRequest;
  return proto.title.DeleteAlbumRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.DeleteAlbumRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.DeleteAlbumRequest}
 */
proto.title.DeleteAlbumRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setAlbumid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setIndexpath(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.DeleteAlbumRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.DeleteAlbumRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.DeleteAlbumRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeleteAlbumRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAlbumid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getIndexpath();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string albumId = 1;
 * @return {string}
 */
proto.title.DeleteAlbumRequest.prototype.getAlbumid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.DeleteAlbumRequest} returns this
 */
proto.title.DeleteAlbumRequest.prototype.setAlbumid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string indexPath = 2;
 * @return {string}
 */
proto.title.DeleteAlbumRequest.prototype.getIndexpath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.title.DeleteAlbumRequest} returns this
 */
proto.title.DeleteAlbumRequest.prototype.setIndexpath = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.title.DeleteAlbumResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.title.DeleteAlbumResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.title.DeleteAlbumResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeleteAlbumResponse.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.title.DeleteAlbumResponse}
 */
proto.title.DeleteAlbumResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.title.DeleteAlbumResponse;
  return proto.title.DeleteAlbumResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.title.DeleteAlbumResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.title.DeleteAlbumResponse}
 */
proto.title.DeleteAlbumResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.title.DeleteAlbumResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.title.DeleteAlbumResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.title.DeleteAlbumResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.title.DeleteAlbumResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};


goog.object.extend(exports, proto.title);
