// source: resource.proto
/**
 * @fileoverview
 * @enhanceable
 * @suppress {missingRequire} reports error on implicit type usages.
 * @suppress {messageConventions} JS Compiler reports an error if a variable or
 *     field starts with 'MSG_' and isn't a translatable message.
 * @public
 */
// GENERATED CODE -- DO NOT EDIT!
/* eslint-disable */
// @ts-nocheck

var jspb = require('google-protobuf');
var goog = jspb;
var global = Function('return this')();

goog.exportSymbol('proto.resource.AcceptPeerRqst', null, global);
goog.exportSymbol('proto.resource.AcceptPeerRsp', null, global);
goog.exportSymbol('proto.resource.Account', null, global);
goog.exportSymbol('proto.resource.AccountExistRqst', null, global);
goog.exportSymbol('proto.resource.AccountExistRsp', null, global);
goog.exportSymbol('proto.resource.AddAccountRoleRqst', null, global);
goog.exportSymbol('proto.resource.AddAccountRoleRsp', null, global);
goog.exportSymbol('proto.resource.AddApplicationActionsRqst', null, global);
goog.exportSymbol('proto.resource.AddApplicationActionsRsp', null, global);
goog.exportSymbol('proto.resource.AddGroupMemberAccountRqst', null, global);
goog.exportSymbol('proto.resource.AddGroupMemberAccountRsp', null, global);
goog.exportSymbol('proto.resource.AddOrganizationAccountRqst', null, global);
goog.exportSymbol('proto.resource.AddOrganizationAccountRsp', null, global);
goog.exportSymbol('proto.resource.AddOrganizationApplicationRqst', null, global);
goog.exportSymbol('proto.resource.AddOrganizationApplicationRsp', null, global);
goog.exportSymbol('proto.resource.AddOrganizationGroupRqst', null, global);
goog.exportSymbol('proto.resource.AddOrganizationGroupRsp', null, global);
goog.exportSymbol('proto.resource.AddOrganizationRoleRqst', null, global);
goog.exportSymbol('proto.resource.AddOrganizationRoleRsp', null, global);
goog.exportSymbol('proto.resource.AddPeerActionsRqst', null, global);
goog.exportSymbol('proto.resource.AddPeerActionsRsp', null, global);
goog.exportSymbol('proto.resource.AddRoleActionsRqst', null, global);
goog.exportSymbol('proto.resource.AddRoleActionsRsp', null, global);
goog.exportSymbol('proto.resource.Application', null, global);
goog.exportSymbol('proto.resource.ClearAllNotificationsRqst', null, global);
goog.exportSymbol('proto.resource.ClearAllNotificationsRsp', null, global);
goog.exportSymbol('proto.resource.ClearNotificationsByTypeRqst', null, global);
goog.exportSymbol('proto.resource.ClearNotificationsByTypeRsp', null, global);
goog.exportSymbol('proto.resource.Contact', null, global);
goog.exportSymbol('proto.resource.CreateApplicationRqst', null, global);
goog.exportSymbol('proto.resource.CreateApplicationRsp', null, global);
goog.exportSymbol('proto.resource.CreateGroupRqst', null, global);
goog.exportSymbol('proto.resource.CreateGroupRsp', null, global);
goog.exportSymbol('proto.resource.CreateNotificationRqst', null, global);
goog.exportSymbol('proto.resource.CreateNotificationRsp', null, global);
goog.exportSymbol('proto.resource.CreateOrganizationRqst', null, global);
goog.exportSymbol('proto.resource.CreateOrganizationRsp', null, global);
goog.exportSymbol('proto.resource.CreateRoleRqst', null, global);
goog.exportSymbol('proto.resource.CreateRoleRsp', null, global);
goog.exportSymbol('proto.resource.DeleteAccountRqst', null, global);
goog.exportSymbol('proto.resource.DeleteAccountRsp', null, global);
goog.exportSymbol('proto.resource.DeleteApplicationRqst', null, global);
goog.exportSymbol('proto.resource.DeleteApplicationRsp', null, global);
goog.exportSymbol('proto.resource.DeleteGroupRqst', null, global);
goog.exportSymbol('proto.resource.DeleteGroupRsp', null, global);
goog.exportSymbol('proto.resource.DeleteNotificationRqst', null, global);
goog.exportSymbol('proto.resource.DeleteNotificationRsp', null, global);
goog.exportSymbol('proto.resource.DeleteOrganizationRqst', null, global);
goog.exportSymbol('proto.resource.DeleteOrganizationRsp', null, global);
goog.exportSymbol('proto.resource.DeletePeerRqst', null, global);
goog.exportSymbol('proto.resource.DeletePeerRsp', null, global);
goog.exportSymbol('proto.resource.DeleteRoleRqst', null, global);
goog.exportSymbol('proto.resource.DeleteRoleRsp', null, global);
goog.exportSymbol('proto.resource.FindPackagesDescriptorRequest', null, global);
goog.exportSymbol('proto.resource.FindPackagesDescriptorResponse', null, global);
goog.exportSymbol('proto.resource.GetAccountRqst', null, global);
goog.exportSymbol('proto.resource.GetAccountRsp', null, global);
goog.exportSymbol('proto.resource.GetAccountsRqst', null, global);
goog.exportSymbol('proto.resource.GetAccountsRsp', null, global);
goog.exportSymbol('proto.resource.GetApplicationAliasRqst', null, global);
goog.exportSymbol('proto.resource.GetApplicationAliasRsp', null, global);
goog.exportSymbol('proto.resource.GetApplicationIconRqst', null, global);
goog.exportSymbol('proto.resource.GetApplicationIconRsp', null, global);
goog.exportSymbol('proto.resource.GetApplicationVersionRqst', null, global);
goog.exportSymbol('proto.resource.GetApplicationVersionRsp', null, global);
goog.exportSymbol('proto.resource.GetApplicationsRqst', null, global);
goog.exportSymbol('proto.resource.GetApplicationsRsp', null, global);
goog.exportSymbol('proto.resource.GetGroupsRqst', null, global);
goog.exportSymbol('proto.resource.GetGroupsRsp', null, global);
goog.exportSymbol('proto.resource.GetNotificationsRqst', null, global);
goog.exportSymbol('proto.resource.GetNotificationsRsp', null, global);
goog.exportSymbol('proto.resource.GetOrganizationsRqst', null, global);
goog.exportSymbol('proto.resource.GetOrganizationsRsp', null, global);
goog.exportSymbol('proto.resource.GetPackageBundleChecksumRequest', null, global);
goog.exportSymbol('proto.resource.GetPackageBundleChecksumResponse', null, global);
goog.exportSymbol('proto.resource.GetPackageDescriptorRequest', null, global);
goog.exportSymbol('proto.resource.GetPackageDescriptorResponse', null, global);
goog.exportSymbol('proto.resource.GetPackagesDescriptorRequest', null, global);
goog.exportSymbol('proto.resource.GetPackagesDescriptorResponse', null, global);
goog.exportSymbol('proto.resource.GetPeersRqst', null, global);
goog.exportSymbol('proto.resource.GetPeersRsp', null, global);
goog.exportSymbol('proto.resource.GetRolesRqst', null, global);
goog.exportSymbol('proto.resource.GetRolesRsp', null, global);
goog.exportSymbol('proto.resource.GetSessionRequest', null, global);
goog.exportSymbol('proto.resource.GetSessionResponse', null, global);
goog.exportSymbol('proto.resource.GetSessionsRequest', null, global);
goog.exportSymbol('proto.resource.GetSessionsResponse', null, global);
goog.exportSymbol('proto.resource.Group', null, global);
goog.exportSymbol('proto.resource.IsOrgnanizationMemberRqst', null, global);
goog.exportSymbol('proto.resource.IsOrgnanizationMemberRsp', null, global);
goog.exportSymbol('proto.resource.Notification', null, global);
goog.exportSymbol('proto.resource.NotificationType', null, global);
goog.exportSymbol('proto.resource.Organization', null, global);
goog.exportSymbol('proto.resource.PackageBundle', null, global);
goog.exportSymbol('proto.resource.PackageDescriptor', null, global);
goog.exportSymbol('proto.resource.PackageType', null, global);
goog.exportSymbol('proto.resource.Peer', null, global);
goog.exportSymbol('proto.resource.PeerApprovalState', null, global);
goog.exportSymbol('proto.resource.RegisterAccountRqst', null, global);
goog.exportSymbol('proto.resource.RegisterAccountRsp', null, global);
goog.exportSymbol('proto.resource.RegisterPeerRqst', null, global);
goog.exportSymbol('proto.resource.RegisterPeerRsp', null, global);
goog.exportSymbol('proto.resource.RejectPeerRqst', null, global);
goog.exportSymbol('proto.resource.RejectPeerRsp', null, global);
goog.exportSymbol('proto.resource.RemoveAccountRoleRqst', null, global);
goog.exportSymbol('proto.resource.RemoveAccountRoleRsp', null, global);
goog.exportSymbol('proto.resource.RemoveApplicationActionRqst', null, global);
goog.exportSymbol('proto.resource.RemoveApplicationActionRsp', null, global);
goog.exportSymbol('proto.resource.RemoveApplicationsActionRqst', null, global);
goog.exportSymbol('proto.resource.RemoveApplicationsActionRsp', null, global);
goog.exportSymbol('proto.resource.RemoveGroupMemberAccountRqst', null, global);
goog.exportSymbol('proto.resource.RemoveGroupMemberAccountRsp', null, global);
goog.exportSymbol('proto.resource.RemoveOrganizationAccountRqst', null, global);
goog.exportSymbol('proto.resource.RemoveOrganizationAccountRsp', null, global);
goog.exportSymbol('proto.resource.RemoveOrganizationApplicationRqst', null, global);
goog.exportSymbol('proto.resource.RemoveOrganizationApplicationRsp', null, global);
goog.exportSymbol('proto.resource.RemoveOrganizationGroupRqst', null, global);
goog.exportSymbol('proto.resource.RemoveOrganizationGroupRsp', null, global);
goog.exportSymbol('proto.resource.RemoveOrganizationRoleRqst', null, global);
goog.exportSymbol('proto.resource.RemoveOrganizationRoleRsp', null, global);
goog.exportSymbol('proto.resource.RemovePeerActionRqst', null, global);
goog.exportSymbol('proto.resource.RemovePeerActionRsp', null, global);
goog.exportSymbol('proto.resource.RemovePeersActionRqst', null, global);
goog.exportSymbol('proto.resource.RemovePeersActionRsp', null, global);
goog.exportSymbol('proto.resource.RemoveRoleActionRqst', null, global);
goog.exportSymbol('proto.resource.RemoveRoleActionRsp', null, global);
goog.exportSymbol('proto.resource.RemoveRolesActionRqst', null, global);
goog.exportSymbol('proto.resource.RemoveRolesActionRsp', null, global);
goog.exportSymbol('proto.resource.RemoveSessionRequest', null, global);
goog.exportSymbol('proto.resource.RemoveSessionResponse', null, global);
goog.exportSymbol('proto.resource.Role', null, global);
goog.exportSymbol('proto.resource.Session', null, global);
goog.exportSymbol('proto.resource.SessionState', null, global);
goog.exportSymbol('proto.resource.SetAccountContactRqst', null, global);
goog.exportSymbol('proto.resource.SetAccountContactRsp', null, global);
goog.exportSymbol('proto.resource.SetAccountPasswordRqst', null, global);
goog.exportSymbol('proto.resource.SetAccountPasswordRsp', null, global);
goog.exportSymbol('proto.resource.SetEmailRequest', null, global);
goog.exportSymbol('proto.resource.SetEmailResponse', null, global);
goog.exportSymbol('proto.resource.SetPackageBundleRequest', null, global);
goog.exportSymbol('proto.resource.SetPackageBundleResponse', null, global);
goog.exportSymbol('proto.resource.SetPackageDescriptorRequest', null, global);
goog.exportSymbol('proto.resource.SetPackageDescriptorResponse', null, global);
goog.exportSymbol('proto.resource.UpdateApplicationRqst', null, global);
goog.exportSymbol('proto.resource.UpdateApplicationRsp', null, global);
goog.exportSymbol('proto.resource.UpdateGroupRqst', null, global);
goog.exportSymbol('proto.resource.UpdateGroupRsp', null, global);
goog.exportSymbol('proto.resource.UpdateOrganizationRqst', null, global);
goog.exportSymbol('proto.resource.UpdateOrganizationRsp', null, global);
goog.exportSymbol('proto.resource.UpdateSessionRequest', null, global);
goog.exportSymbol('proto.resource.UpdateSessionResponse', null, global);
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
proto.resource.PackageBundle = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.PackageBundle, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.PackageBundle.displayName = 'proto.resource.PackageBundle';
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
proto.resource.SetPackageBundleRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.SetPackageBundleRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.SetPackageBundleRequest.displayName = 'proto.resource.SetPackageBundleRequest';
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
proto.resource.SetPackageBundleResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.SetPackageBundleResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.SetPackageBundleResponse.displayName = 'proto.resource.SetPackageBundleResponse';
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
proto.resource.GetPackageBundleChecksumRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetPackageBundleChecksumRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetPackageBundleChecksumRequest.displayName = 'proto.resource.GetPackageBundleChecksumRequest';
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
proto.resource.GetPackageBundleChecksumResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetPackageBundleChecksumResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetPackageBundleChecksumResponse.displayName = 'proto.resource.GetPackageBundleChecksumResponse';
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
proto.resource.PackageDescriptor = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.PackageDescriptor.repeatedFields_, null);
};
goog.inherits(proto.resource.PackageDescriptor, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.PackageDescriptor.displayName = 'proto.resource.PackageDescriptor';
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
proto.resource.GetPackageDescriptorRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetPackageDescriptorRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetPackageDescriptorRequest.displayName = 'proto.resource.GetPackageDescriptorRequest';
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
proto.resource.GetPackageDescriptorResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.GetPackageDescriptorResponse.repeatedFields_, null);
};
goog.inherits(proto.resource.GetPackageDescriptorResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetPackageDescriptorResponse.displayName = 'proto.resource.GetPackageDescriptorResponse';
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
proto.resource.GetPackagesDescriptorRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetPackagesDescriptorRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetPackagesDescriptorRequest.displayName = 'proto.resource.GetPackagesDescriptorRequest';
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
proto.resource.GetPackagesDescriptorResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.GetPackagesDescriptorResponse.repeatedFields_, null);
};
goog.inherits(proto.resource.GetPackagesDescriptorResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetPackagesDescriptorResponse.displayName = 'proto.resource.GetPackagesDescriptorResponse';
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
proto.resource.SetPackageDescriptorRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.SetPackageDescriptorRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.SetPackageDescriptorRequest.displayName = 'proto.resource.SetPackageDescriptorRequest';
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
proto.resource.SetPackageDescriptorResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.SetPackageDescriptorResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.SetPackageDescriptorResponse.displayName = 'proto.resource.SetPackageDescriptorResponse';
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
proto.resource.FindPackagesDescriptorRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.FindPackagesDescriptorRequest.repeatedFields_, null);
};
goog.inherits(proto.resource.FindPackagesDescriptorRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.FindPackagesDescriptorRequest.displayName = 'proto.resource.FindPackagesDescriptorRequest';
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
proto.resource.FindPackagesDescriptorResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.FindPackagesDescriptorResponse.repeatedFields_, null);
};
goog.inherits(proto.resource.FindPackagesDescriptorResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.FindPackagesDescriptorResponse.displayName = 'proto.resource.FindPackagesDescriptorResponse';
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
proto.resource.Role = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.Role.repeatedFields_, null);
};
goog.inherits(proto.resource.Role, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.Role.displayName = 'proto.resource.Role';
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
proto.resource.AddRoleActionsRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.AddRoleActionsRqst.repeatedFields_, null);
};
goog.inherits(proto.resource.AddRoleActionsRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AddRoleActionsRqst.displayName = 'proto.resource.AddRoleActionsRqst';
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
proto.resource.AddRoleActionsRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.AddRoleActionsRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AddRoleActionsRsp.displayName = 'proto.resource.AddRoleActionsRsp';
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
proto.resource.RemoveRoleActionRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveRoleActionRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveRoleActionRqst.displayName = 'proto.resource.RemoveRoleActionRqst';
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
proto.resource.RemoveRoleActionRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveRoleActionRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveRoleActionRsp.displayName = 'proto.resource.RemoveRoleActionRsp';
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
proto.resource.RemoveRolesActionRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveRolesActionRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveRolesActionRqst.displayName = 'proto.resource.RemoveRolesActionRqst';
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
proto.resource.RemoveRolesActionRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveRolesActionRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveRolesActionRsp.displayName = 'proto.resource.RemoveRolesActionRsp';
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
proto.resource.Account = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.Account.repeatedFields_, null);
};
goog.inherits(proto.resource.Account, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.Account.displayName = 'proto.resource.Account';
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
proto.resource.RegisterAccountRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RegisterAccountRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RegisterAccountRqst.displayName = 'proto.resource.RegisterAccountRqst';
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
proto.resource.RegisterAccountRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RegisterAccountRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RegisterAccountRsp.displayName = 'proto.resource.RegisterAccountRsp';
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
proto.resource.AccountExistRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.AccountExistRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AccountExistRqst.displayName = 'proto.resource.AccountExistRqst';
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
proto.resource.AccountExistRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.AccountExistRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AccountExistRsp.displayName = 'proto.resource.AccountExistRsp';
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
proto.resource.GetAccountsRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetAccountsRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetAccountsRqst.displayName = 'proto.resource.GetAccountsRqst';
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
proto.resource.GetAccountsRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.GetAccountsRsp.repeatedFields_, null);
};
goog.inherits(proto.resource.GetAccountsRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetAccountsRsp.displayName = 'proto.resource.GetAccountsRsp';
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
proto.resource.GetAccountRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetAccountRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetAccountRqst.displayName = 'proto.resource.GetAccountRqst';
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
proto.resource.GetAccountRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetAccountRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetAccountRsp.displayName = 'proto.resource.GetAccountRsp';
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
proto.resource.SetAccountPasswordRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.SetAccountPasswordRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.SetAccountPasswordRqst.displayName = 'proto.resource.SetAccountPasswordRqst';
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
proto.resource.SetAccountPasswordRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.SetAccountPasswordRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.SetAccountPasswordRsp.displayName = 'proto.resource.SetAccountPasswordRsp';
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
proto.resource.SetEmailRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.SetEmailRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.SetEmailRequest.displayName = 'proto.resource.SetEmailRequest';
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
proto.resource.SetEmailResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.SetEmailResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.SetEmailResponse.displayName = 'proto.resource.SetEmailResponse';
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
proto.resource.DeleteAccountRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.DeleteAccountRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.DeleteAccountRqst.displayName = 'proto.resource.DeleteAccountRqst';
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
proto.resource.DeleteAccountRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.DeleteAccountRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.DeleteAccountRsp.displayName = 'proto.resource.DeleteAccountRsp';
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
proto.resource.AddAccountRoleRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.AddAccountRoleRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AddAccountRoleRqst.displayName = 'proto.resource.AddAccountRoleRqst';
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
proto.resource.AddAccountRoleRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.AddAccountRoleRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AddAccountRoleRsp.displayName = 'proto.resource.AddAccountRoleRsp';
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
proto.resource.RemoveAccountRoleRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveAccountRoleRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveAccountRoleRqst.displayName = 'proto.resource.RemoveAccountRoleRqst';
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
proto.resource.RemoveAccountRoleRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveAccountRoleRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveAccountRoleRsp.displayName = 'proto.resource.RemoveAccountRoleRsp';
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
proto.resource.Contact = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.Contact, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.Contact.displayName = 'proto.resource.Contact';
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
proto.resource.SetAccountContactRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.SetAccountContactRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.SetAccountContactRqst.displayName = 'proto.resource.SetAccountContactRqst';
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
proto.resource.SetAccountContactRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.SetAccountContactRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.SetAccountContactRsp.displayName = 'proto.resource.SetAccountContactRsp';
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
proto.resource.CreateRoleRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.CreateRoleRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.CreateRoleRqst.displayName = 'proto.resource.CreateRoleRqst';
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
proto.resource.CreateRoleRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.CreateRoleRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.CreateRoleRsp.displayName = 'proto.resource.CreateRoleRsp';
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
proto.resource.GetRolesRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetRolesRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetRolesRqst.displayName = 'proto.resource.GetRolesRqst';
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
proto.resource.GetRolesRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.GetRolesRsp.repeatedFields_, null);
};
goog.inherits(proto.resource.GetRolesRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetRolesRsp.displayName = 'proto.resource.GetRolesRsp';
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
proto.resource.DeleteRoleRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.DeleteRoleRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.DeleteRoleRqst.displayName = 'proto.resource.DeleteRoleRqst';
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
proto.resource.DeleteRoleRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.DeleteRoleRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.DeleteRoleRsp.displayName = 'proto.resource.DeleteRoleRsp';
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
proto.resource.Application = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.Application.repeatedFields_, null);
};
goog.inherits(proto.resource.Application, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.Application.displayName = 'proto.resource.Application';
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
proto.resource.CreateApplicationRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.CreateApplicationRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.CreateApplicationRqst.displayName = 'proto.resource.CreateApplicationRqst';
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
proto.resource.CreateApplicationRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.CreateApplicationRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.CreateApplicationRsp.displayName = 'proto.resource.CreateApplicationRsp';
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
proto.resource.UpdateApplicationRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.UpdateApplicationRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.UpdateApplicationRqst.displayName = 'proto.resource.UpdateApplicationRqst';
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
proto.resource.UpdateApplicationRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.UpdateApplicationRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.UpdateApplicationRsp.displayName = 'proto.resource.UpdateApplicationRsp';
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
proto.resource.DeleteApplicationRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.DeleteApplicationRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.DeleteApplicationRqst.displayName = 'proto.resource.DeleteApplicationRqst';
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
proto.resource.DeleteApplicationRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.DeleteApplicationRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.DeleteApplicationRsp.displayName = 'proto.resource.DeleteApplicationRsp';
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
proto.resource.AddApplicationActionsRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.AddApplicationActionsRqst.repeatedFields_, null);
};
goog.inherits(proto.resource.AddApplicationActionsRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AddApplicationActionsRqst.displayName = 'proto.resource.AddApplicationActionsRqst';
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
proto.resource.AddApplicationActionsRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.AddApplicationActionsRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AddApplicationActionsRsp.displayName = 'proto.resource.AddApplicationActionsRsp';
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
proto.resource.RemoveApplicationActionRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveApplicationActionRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveApplicationActionRqst.displayName = 'proto.resource.RemoveApplicationActionRqst';
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
proto.resource.RemoveApplicationActionRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveApplicationActionRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveApplicationActionRsp.displayName = 'proto.resource.RemoveApplicationActionRsp';
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
proto.resource.RemoveApplicationsActionRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveApplicationsActionRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveApplicationsActionRqst.displayName = 'proto.resource.RemoveApplicationsActionRqst';
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
proto.resource.RemoveApplicationsActionRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveApplicationsActionRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveApplicationsActionRsp.displayName = 'proto.resource.RemoveApplicationsActionRsp';
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
proto.resource.GetApplicationsRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetApplicationsRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetApplicationsRqst.displayName = 'proto.resource.GetApplicationsRqst';
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
proto.resource.GetApplicationsRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.GetApplicationsRsp.repeatedFields_, null);
};
goog.inherits(proto.resource.GetApplicationsRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetApplicationsRsp.displayName = 'proto.resource.GetApplicationsRsp';
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
proto.resource.GetApplicationVersionRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetApplicationVersionRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetApplicationVersionRqst.displayName = 'proto.resource.GetApplicationVersionRqst';
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
proto.resource.GetApplicationVersionRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetApplicationVersionRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetApplicationVersionRsp.displayName = 'proto.resource.GetApplicationVersionRsp';
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
proto.resource.GetApplicationIconRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetApplicationIconRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetApplicationIconRqst.displayName = 'proto.resource.GetApplicationIconRqst';
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
proto.resource.GetApplicationIconRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetApplicationIconRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetApplicationIconRsp.displayName = 'proto.resource.GetApplicationIconRsp';
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
proto.resource.GetApplicationAliasRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetApplicationAliasRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetApplicationAliasRqst.displayName = 'proto.resource.GetApplicationAliasRqst';
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
proto.resource.GetApplicationAliasRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetApplicationAliasRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetApplicationAliasRsp.displayName = 'proto.resource.GetApplicationAliasRsp';
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
proto.resource.Group = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.Group.repeatedFields_, null);
};
goog.inherits(proto.resource.Group, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.Group.displayName = 'proto.resource.Group';
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
proto.resource.CreateGroupRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.CreateGroupRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.CreateGroupRqst.displayName = 'proto.resource.CreateGroupRqst';
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
proto.resource.CreateGroupRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.CreateGroupRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.CreateGroupRsp.displayName = 'proto.resource.CreateGroupRsp';
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
proto.resource.UpdateGroupRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.UpdateGroupRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.UpdateGroupRqst.displayName = 'proto.resource.UpdateGroupRqst';
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
proto.resource.UpdateGroupRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.UpdateGroupRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.UpdateGroupRsp.displayName = 'proto.resource.UpdateGroupRsp';
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
proto.resource.GetGroupsRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetGroupsRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetGroupsRqst.displayName = 'proto.resource.GetGroupsRqst';
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
proto.resource.GetGroupsRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.GetGroupsRsp.repeatedFields_, null);
};
goog.inherits(proto.resource.GetGroupsRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetGroupsRsp.displayName = 'proto.resource.GetGroupsRsp';
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
proto.resource.DeleteGroupRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.DeleteGroupRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.DeleteGroupRqst.displayName = 'proto.resource.DeleteGroupRqst';
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
proto.resource.DeleteGroupRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.DeleteGroupRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.DeleteGroupRsp.displayName = 'proto.resource.DeleteGroupRsp';
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
proto.resource.AddGroupMemberAccountRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.AddGroupMemberAccountRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AddGroupMemberAccountRqst.displayName = 'proto.resource.AddGroupMemberAccountRqst';
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
proto.resource.AddGroupMemberAccountRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.AddGroupMemberAccountRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AddGroupMemberAccountRsp.displayName = 'proto.resource.AddGroupMemberAccountRsp';
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
proto.resource.RemoveGroupMemberAccountRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveGroupMemberAccountRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveGroupMemberAccountRqst.displayName = 'proto.resource.RemoveGroupMemberAccountRqst';
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
proto.resource.RemoveGroupMemberAccountRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveGroupMemberAccountRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveGroupMemberAccountRsp.displayName = 'proto.resource.RemoveGroupMemberAccountRsp';
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
proto.resource.Organization = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.Organization.repeatedFields_, null);
};
goog.inherits(proto.resource.Organization, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.Organization.displayName = 'proto.resource.Organization';
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
proto.resource.CreateOrganizationRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.CreateOrganizationRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.CreateOrganizationRqst.displayName = 'proto.resource.CreateOrganizationRqst';
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
proto.resource.CreateOrganizationRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.CreateOrganizationRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.CreateOrganizationRsp.displayName = 'proto.resource.CreateOrganizationRsp';
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
proto.resource.UpdateOrganizationRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.UpdateOrganizationRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.UpdateOrganizationRqst.displayName = 'proto.resource.UpdateOrganizationRqst';
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
proto.resource.UpdateOrganizationRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.UpdateOrganizationRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.UpdateOrganizationRsp.displayName = 'proto.resource.UpdateOrganizationRsp';
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
proto.resource.GetOrganizationsRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetOrganizationsRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetOrganizationsRqst.displayName = 'proto.resource.GetOrganizationsRqst';
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
proto.resource.GetOrganizationsRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.GetOrganizationsRsp.repeatedFields_, null);
};
goog.inherits(proto.resource.GetOrganizationsRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetOrganizationsRsp.displayName = 'proto.resource.GetOrganizationsRsp';
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
proto.resource.AddOrganizationAccountRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.AddOrganizationAccountRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AddOrganizationAccountRqst.displayName = 'proto.resource.AddOrganizationAccountRqst';
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
proto.resource.AddOrganizationAccountRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.AddOrganizationAccountRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AddOrganizationAccountRsp.displayName = 'proto.resource.AddOrganizationAccountRsp';
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
proto.resource.AddOrganizationGroupRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.AddOrganizationGroupRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AddOrganizationGroupRqst.displayName = 'proto.resource.AddOrganizationGroupRqst';
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
proto.resource.AddOrganizationGroupRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.AddOrganizationGroupRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AddOrganizationGroupRsp.displayName = 'proto.resource.AddOrganizationGroupRsp';
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
proto.resource.AddOrganizationRoleRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.AddOrganizationRoleRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AddOrganizationRoleRqst.displayName = 'proto.resource.AddOrganizationRoleRqst';
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
proto.resource.AddOrganizationRoleRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.AddOrganizationRoleRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AddOrganizationRoleRsp.displayName = 'proto.resource.AddOrganizationRoleRsp';
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
proto.resource.AddOrganizationApplicationRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.AddOrganizationApplicationRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AddOrganizationApplicationRqst.displayName = 'proto.resource.AddOrganizationApplicationRqst';
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
proto.resource.AddOrganizationApplicationRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.AddOrganizationApplicationRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AddOrganizationApplicationRsp.displayName = 'proto.resource.AddOrganizationApplicationRsp';
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
proto.resource.RemoveOrganizationGroupRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveOrganizationGroupRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveOrganizationGroupRqst.displayName = 'proto.resource.RemoveOrganizationGroupRqst';
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
proto.resource.RemoveOrganizationGroupRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveOrganizationGroupRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveOrganizationGroupRsp.displayName = 'proto.resource.RemoveOrganizationGroupRsp';
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
proto.resource.RemoveOrganizationRoleRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveOrganizationRoleRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveOrganizationRoleRqst.displayName = 'proto.resource.RemoveOrganizationRoleRqst';
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
proto.resource.RemoveOrganizationRoleRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveOrganizationRoleRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveOrganizationRoleRsp.displayName = 'proto.resource.RemoveOrganizationRoleRsp';
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
proto.resource.RemoveOrganizationApplicationRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveOrganizationApplicationRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveOrganizationApplicationRqst.displayName = 'proto.resource.RemoveOrganizationApplicationRqst';
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
proto.resource.RemoveOrganizationApplicationRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveOrganizationApplicationRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveOrganizationApplicationRsp.displayName = 'proto.resource.RemoveOrganizationApplicationRsp';
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
proto.resource.RemoveOrganizationAccountRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveOrganizationAccountRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveOrganizationAccountRqst.displayName = 'proto.resource.RemoveOrganizationAccountRqst';
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
proto.resource.RemoveOrganizationAccountRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveOrganizationAccountRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveOrganizationAccountRsp.displayName = 'proto.resource.RemoveOrganizationAccountRsp';
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
proto.resource.IsOrgnanizationMemberRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.IsOrgnanizationMemberRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.IsOrgnanizationMemberRqst.displayName = 'proto.resource.IsOrgnanizationMemberRqst';
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
proto.resource.IsOrgnanizationMemberRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.IsOrgnanizationMemberRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.IsOrgnanizationMemberRsp.displayName = 'proto.resource.IsOrgnanizationMemberRsp';
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
proto.resource.DeleteOrganizationRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.DeleteOrganizationRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.DeleteOrganizationRqst.displayName = 'proto.resource.DeleteOrganizationRqst';
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
proto.resource.DeleteOrganizationRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.DeleteOrganizationRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.DeleteOrganizationRsp.displayName = 'proto.resource.DeleteOrganizationRsp';
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
proto.resource.Peer = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.Peer.repeatedFields_, null);
};
goog.inherits(proto.resource.Peer, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.Peer.displayName = 'proto.resource.Peer';
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
proto.resource.RegisterPeerRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RegisterPeerRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RegisterPeerRqst.displayName = 'proto.resource.RegisterPeerRqst';
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
proto.resource.RegisterPeerRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RegisterPeerRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RegisterPeerRsp.displayName = 'proto.resource.RegisterPeerRsp';
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
proto.resource.GetPeersRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetPeersRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetPeersRqst.displayName = 'proto.resource.GetPeersRqst';
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
proto.resource.GetPeersRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.GetPeersRsp.repeatedFields_, null);
};
goog.inherits(proto.resource.GetPeersRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetPeersRsp.displayName = 'proto.resource.GetPeersRsp';
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
proto.resource.DeletePeerRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.DeletePeerRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.DeletePeerRqst.displayName = 'proto.resource.DeletePeerRqst';
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
proto.resource.DeletePeerRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.DeletePeerRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.DeletePeerRsp.displayName = 'proto.resource.DeletePeerRsp';
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
proto.resource.AddPeerActionsRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.AddPeerActionsRqst.repeatedFields_, null);
};
goog.inherits(proto.resource.AddPeerActionsRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AddPeerActionsRqst.displayName = 'proto.resource.AddPeerActionsRqst';
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
proto.resource.AddPeerActionsRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.AddPeerActionsRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AddPeerActionsRsp.displayName = 'proto.resource.AddPeerActionsRsp';
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
proto.resource.RemovePeerActionRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemovePeerActionRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemovePeerActionRqst.displayName = 'proto.resource.RemovePeerActionRqst';
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
proto.resource.RemovePeerActionRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemovePeerActionRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemovePeerActionRsp.displayName = 'proto.resource.RemovePeerActionRsp';
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
proto.resource.RemovePeersActionRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemovePeersActionRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemovePeersActionRqst.displayName = 'proto.resource.RemovePeersActionRqst';
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
proto.resource.RemovePeersActionRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemovePeersActionRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemovePeersActionRsp.displayName = 'proto.resource.RemovePeersActionRsp';
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
proto.resource.AcceptPeerRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.AcceptPeerRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AcceptPeerRqst.displayName = 'proto.resource.AcceptPeerRqst';
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
proto.resource.AcceptPeerRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.AcceptPeerRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.AcceptPeerRsp.displayName = 'proto.resource.AcceptPeerRsp';
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
proto.resource.RejectPeerRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RejectPeerRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RejectPeerRqst.displayName = 'proto.resource.RejectPeerRqst';
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
proto.resource.RejectPeerRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RejectPeerRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RejectPeerRsp.displayName = 'proto.resource.RejectPeerRsp';
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
proto.resource.Notification = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.Notification, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.Notification.displayName = 'proto.resource.Notification';
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
proto.resource.CreateNotificationRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.CreateNotificationRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.CreateNotificationRqst.displayName = 'proto.resource.CreateNotificationRqst';
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
proto.resource.CreateNotificationRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.CreateNotificationRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.CreateNotificationRsp.displayName = 'proto.resource.CreateNotificationRsp';
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
proto.resource.GetNotificationsRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetNotificationsRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetNotificationsRqst.displayName = 'proto.resource.GetNotificationsRqst';
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
proto.resource.GetNotificationsRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.GetNotificationsRsp.repeatedFields_, null);
};
goog.inherits(proto.resource.GetNotificationsRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetNotificationsRsp.displayName = 'proto.resource.GetNotificationsRsp';
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
proto.resource.DeleteNotificationRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.DeleteNotificationRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.DeleteNotificationRqst.displayName = 'proto.resource.DeleteNotificationRqst';
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
proto.resource.DeleteNotificationRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.DeleteNotificationRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.DeleteNotificationRsp.displayName = 'proto.resource.DeleteNotificationRsp';
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
proto.resource.ClearAllNotificationsRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.ClearAllNotificationsRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.ClearAllNotificationsRqst.displayName = 'proto.resource.ClearAllNotificationsRqst';
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
proto.resource.ClearAllNotificationsRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.ClearAllNotificationsRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.ClearAllNotificationsRsp.displayName = 'proto.resource.ClearAllNotificationsRsp';
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
proto.resource.ClearNotificationsByTypeRqst = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.ClearNotificationsByTypeRqst, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.ClearNotificationsByTypeRqst.displayName = 'proto.resource.ClearNotificationsByTypeRqst';
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
proto.resource.ClearNotificationsByTypeRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.ClearNotificationsByTypeRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.ClearNotificationsByTypeRsp.displayName = 'proto.resource.ClearNotificationsByTypeRsp';
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
proto.resource.Session = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.Session, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.Session.displayName = 'proto.resource.Session';
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
proto.resource.UpdateSessionRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.UpdateSessionRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.UpdateSessionRequest.displayName = 'proto.resource.UpdateSessionRequest';
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
proto.resource.UpdateSessionResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.UpdateSessionResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.UpdateSessionResponse.displayName = 'proto.resource.UpdateSessionResponse';
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
proto.resource.RemoveSessionRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveSessionRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveSessionRequest.displayName = 'proto.resource.RemoveSessionRequest';
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
proto.resource.RemoveSessionResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.RemoveSessionResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.RemoveSessionResponse.displayName = 'proto.resource.RemoveSessionResponse';
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
proto.resource.GetSessionsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetSessionsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetSessionsRequest.displayName = 'proto.resource.GetSessionsRequest';
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
proto.resource.GetSessionsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.resource.GetSessionsResponse.repeatedFields_, null);
};
goog.inherits(proto.resource.GetSessionsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetSessionsResponse.displayName = 'proto.resource.GetSessionsResponse';
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
proto.resource.GetSessionRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetSessionRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetSessionRequest.displayName = 'proto.resource.GetSessionRequest';
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
proto.resource.GetSessionResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.resource.GetSessionResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.resource.GetSessionResponse.displayName = 'proto.resource.GetSessionResponse';
}



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
proto.resource.PackageBundle.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.PackageBundle.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.PackageBundle} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.PackageBundle.toObject = function(includeInstance, msg) {
  var f, obj = {
    packagedescriptor: (f = msg.getPackagedescriptor()) && proto.resource.PackageDescriptor.toObject(includeInstance, f),
    checksum: jspb.Message.getFieldWithDefault(msg, 2, ""),
    plaform: jspb.Message.getFieldWithDefault(msg, 3, ""),
    size: jspb.Message.getFieldWithDefault(msg, 4, 0),
    modified: jspb.Message.getFieldWithDefault(msg, 5, 0),
    binairies: msg.getBinairies_asB64()
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
 * @return {!proto.resource.PackageBundle}
 */
proto.resource.PackageBundle.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.PackageBundle;
  return proto.resource.PackageBundle.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.PackageBundle} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.PackageBundle}
 */
proto.resource.PackageBundle.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.PackageDescriptor;
      reader.readMessage(value,proto.resource.PackageDescriptor.deserializeBinaryFromReader);
      msg.setPackagedescriptor(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setChecksum(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setPlaform(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setSize(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setModified(value);
      break;
    case 6:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setBinairies(value);
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
proto.resource.PackageBundle.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.PackageBundle.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.PackageBundle} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.PackageBundle.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPackagedescriptor();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.resource.PackageDescriptor.serializeBinaryToWriter
    );
  }
  f = message.getChecksum();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getPlaform();
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
  f = message.getModified();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
  f = message.getBinairies_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      6,
      f
    );
  }
};


/**
 * optional PackageDescriptor packageDescriptor = 1;
 * @return {?proto.resource.PackageDescriptor}
 */
proto.resource.PackageBundle.prototype.getPackagedescriptor = function() {
  return /** @type{?proto.resource.PackageDescriptor} */ (
    jspb.Message.getWrapperField(this, proto.resource.PackageDescriptor, 1));
};


/**
 * @param {?proto.resource.PackageDescriptor|undefined} value
 * @return {!proto.resource.PackageBundle} returns this
*/
proto.resource.PackageBundle.prototype.setPackagedescriptor = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.resource.PackageBundle} returns this
 */
proto.resource.PackageBundle.prototype.clearPackagedescriptor = function() {
  return this.setPackagedescriptor(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.resource.PackageBundle.prototype.hasPackagedescriptor = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string checksum = 2;
 * @return {string}
 */
proto.resource.PackageBundle.prototype.getChecksum = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.PackageBundle} returns this
 */
proto.resource.PackageBundle.prototype.setChecksum = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string plaform = 3;
 * @return {string}
 */
proto.resource.PackageBundle.prototype.getPlaform = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.PackageBundle} returns this
 */
proto.resource.PackageBundle.prototype.setPlaform = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional int32 size = 4;
 * @return {number}
 */
proto.resource.PackageBundle.prototype.getSize = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.resource.PackageBundle} returns this
 */
proto.resource.PackageBundle.prototype.setSize = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional int64 modified = 5;
 * @return {number}
 */
proto.resource.PackageBundle.prototype.getModified = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.resource.PackageBundle} returns this
 */
proto.resource.PackageBundle.prototype.setModified = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional bytes Binairies = 6;
 * @return {string}
 */
proto.resource.PackageBundle.prototype.getBinairies = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * optional bytes Binairies = 6;
 * This is a type-conversion wrapper around `getBinairies()`
 * @return {string}
 */
proto.resource.PackageBundle.prototype.getBinairies_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getBinairies()));
};


/**
 * optional bytes Binairies = 6;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getBinairies()`
 * @return {!Uint8Array}
 */
proto.resource.PackageBundle.prototype.getBinairies_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getBinairies()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.resource.PackageBundle} returns this
 */
proto.resource.PackageBundle.prototype.setBinairies = function(value) {
  return jspb.Message.setProto3BytesField(this, 6, value);
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
proto.resource.SetPackageBundleRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.SetPackageBundleRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.SetPackageBundleRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.SetPackageBundleRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    bundle: (f = msg.getBundle()) && proto.resource.PackageBundle.toObject(includeInstance, f)
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
 * @return {!proto.resource.SetPackageBundleRequest}
 */
proto.resource.SetPackageBundleRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.SetPackageBundleRequest;
  return proto.resource.SetPackageBundleRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.SetPackageBundleRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.SetPackageBundleRequest}
 */
proto.resource.SetPackageBundleRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.PackageBundle;
      reader.readMessage(value,proto.resource.PackageBundle.deserializeBinaryFromReader);
      msg.setBundle(value);
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
proto.resource.SetPackageBundleRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.SetPackageBundleRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.SetPackageBundleRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.SetPackageBundleRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBundle();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.resource.PackageBundle.serializeBinaryToWriter
    );
  }
};


/**
 * optional PackageBundle bundle = 1;
 * @return {?proto.resource.PackageBundle}
 */
proto.resource.SetPackageBundleRequest.prototype.getBundle = function() {
  return /** @type{?proto.resource.PackageBundle} */ (
    jspb.Message.getWrapperField(this, proto.resource.PackageBundle, 1));
};


/**
 * @param {?proto.resource.PackageBundle|undefined} value
 * @return {!proto.resource.SetPackageBundleRequest} returns this
*/
proto.resource.SetPackageBundleRequest.prototype.setBundle = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.resource.SetPackageBundleRequest} returns this
 */
proto.resource.SetPackageBundleRequest.prototype.clearBundle = function() {
  return this.setBundle(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.resource.SetPackageBundleRequest.prototype.hasBundle = function() {
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
proto.resource.SetPackageBundleResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.SetPackageBundleResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.SetPackageBundleResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.SetPackageBundleResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.SetPackageBundleResponse}
 */
proto.resource.SetPackageBundleResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.SetPackageBundleResponse;
  return proto.resource.SetPackageBundleResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.SetPackageBundleResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.SetPackageBundleResponse}
 */
proto.resource.SetPackageBundleResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.SetPackageBundleResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.SetPackageBundleResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.SetPackageBundleResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.SetPackageBundleResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.SetPackageBundleResponse.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.SetPackageBundleResponse} returns this
 */
proto.resource.SetPackageBundleResponse.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.GetPackageBundleChecksumRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetPackageBundleChecksumRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetPackageBundleChecksumRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetPackageBundleChecksumRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.resource.GetPackageBundleChecksumRequest}
 */
proto.resource.GetPackageBundleChecksumRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetPackageBundleChecksumRequest;
  return proto.resource.GetPackageBundleChecksumRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetPackageBundleChecksumRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetPackageBundleChecksumRequest}
 */
proto.resource.GetPackageBundleChecksumRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.resource.GetPackageBundleChecksumRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetPackageBundleChecksumRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetPackageBundleChecksumRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetPackageBundleChecksumRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.resource.GetPackageBundleChecksumRequest.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetPackageBundleChecksumRequest} returns this
 */
proto.resource.GetPackageBundleChecksumRequest.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
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
proto.resource.GetPackageBundleChecksumResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetPackageBundleChecksumResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetPackageBundleChecksumResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetPackageBundleChecksumResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    checksum: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.resource.GetPackageBundleChecksumResponse}
 */
proto.resource.GetPackageBundleChecksumResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetPackageBundleChecksumResponse;
  return proto.resource.GetPackageBundleChecksumResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetPackageBundleChecksumResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetPackageBundleChecksumResponse}
 */
proto.resource.GetPackageBundleChecksumResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setChecksum(value);
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
proto.resource.GetPackageBundleChecksumResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetPackageBundleChecksumResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetPackageBundleChecksumResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetPackageBundleChecksumResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getChecksum();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string checksum = 1;
 * @return {string}
 */
proto.resource.GetPackageBundleChecksumResponse.prototype.getChecksum = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetPackageBundleChecksumResponse} returns this
 */
proto.resource.GetPackageBundleChecksumResponse.prototype.setChecksum = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.PackageDescriptor.repeatedFields_ = [8,9,10,11,12,13];



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
proto.resource.PackageDescriptor.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.PackageDescriptor.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.PackageDescriptor} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.PackageDescriptor.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, ""),
    name: jspb.Message.getFieldWithDefault(msg, 2, ""),
    type: jspb.Message.getFieldWithDefault(msg, 3, 0),
    publisherid: jspb.Message.getFieldWithDefault(msg, 4, ""),
    version: jspb.Message.getFieldWithDefault(msg, 6, ""),
    description: jspb.Message.getFieldWithDefault(msg, 7, ""),
    repositoriesList: (f = jspb.Message.getRepeatedField(msg, 8)) == null ? undefined : f,
    discoveriesList: (f = jspb.Message.getRepeatedField(msg, 9)) == null ? undefined : f,
    keywordsList: (f = jspb.Message.getRepeatedField(msg, 10)) == null ? undefined : f,
    actionsList: (f = jspb.Message.getRepeatedField(msg, 11)) == null ? undefined : f,
    rolesList: jspb.Message.toObjectList(msg.getRolesList(),
    proto.resource.Role.toObject, includeInstance),
    groupsList: jspb.Message.toObjectList(msg.getGroupsList(),
    proto.resource.Group.toObject, includeInstance),
    icon: jspb.Message.getFieldWithDefault(msg, 14, ""),
    alias: jspb.Message.getFieldWithDefault(msg, 15, "")
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
 * @return {!proto.resource.PackageDescriptor}
 */
proto.resource.PackageDescriptor.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.PackageDescriptor;
  return proto.resource.PackageDescriptor.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.PackageDescriptor} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.PackageDescriptor}
 */
proto.resource.PackageDescriptor.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setName(value);
      break;
    case 3:
      var value = /** @type {!proto.resource.PackageType} */ (reader.readEnum());
      msg.setType(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisherid(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setVersion(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setDescription(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.addRepositories(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.addDiscoveries(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.addKeywords(value);
      break;
    case 11:
      var value = /** @type {string} */ (reader.readString());
      msg.addActions(value);
      break;
    case 12:
      var value = new proto.resource.Role;
      reader.readMessage(value,proto.resource.Role.deserializeBinaryFromReader);
      msg.addRoles(value);
      break;
    case 13:
      var value = new proto.resource.Group;
      reader.readMessage(value,proto.resource.Group.deserializeBinaryFromReader);
      msg.addGroups(value);
      break;
    case 14:
      var value = /** @type {string} */ (reader.readString());
      msg.setIcon(value);
      break;
    case 15:
      var value = /** @type {string} */ (reader.readString());
      msg.setAlias(value);
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
proto.resource.PackageDescriptor.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.PackageDescriptor.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.PackageDescriptor} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.PackageDescriptor.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getType();
  if (f !== 0.0) {
    writer.writeEnum(
      3,
      f
    );
  }
  f = message.getPublisherid();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getVersion();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getDescription();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getRepositoriesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      8,
      f
    );
  }
  f = message.getDiscoveriesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      9,
      f
    );
  }
  f = message.getKeywordsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      10,
      f
    );
  }
  f = message.getActionsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      11,
      f
    );
  }
  f = message.getRolesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      12,
      f,
      proto.resource.Role.serializeBinaryToWriter
    );
  }
  f = message.getGroupsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      13,
      f,
      proto.resource.Group.serializeBinaryToWriter
    );
  }
  f = message.getIcon();
  if (f.length > 0) {
    writer.writeString(
      14,
      f
    );
  }
  f = message.getAlias();
  if (f.length > 0) {
    writer.writeString(
      15,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.resource.PackageDescriptor.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string name = 2;
 * @return {string}
 */
proto.resource.PackageDescriptor.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional PackageType type = 3;
 * @return {!proto.resource.PackageType}
 */
proto.resource.PackageDescriptor.prototype.getType = function() {
  return /** @type {!proto.resource.PackageType} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.resource.PackageType} value
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.setType = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
};


/**
 * optional string publisherId = 4;
 * @return {string}
 */
proto.resource.PackageDescriptor.prototype.getPublisherid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.setPublisherid = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string version = 6;
 * @return {string}
 */
proto.resource.PackageDescriptor.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string description = 7;
 * @return {string}
 */
proto.resource.PackageDescriptor.prototype.getDescription = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.setDescription = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * repeated string repositories = 8;
 * @return {!Array<string>}
 */
proto.resource.PackageDescriptor.prototype.getRepositoriesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 8));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.setRepositoriesList = function(value) {
  return jspb.Message.setField(this, 8, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.addRepositories = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 8, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.clearRepositoriesList = function() {
  return this.setRepositoriesList([]);
};


/**
 * repeated string discoveries = 9;
 * @return {!Array<string>}
 */
proto.resource.PackageDescriptor.prototype.getDiscoveriesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 9));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.setDiscoveriesList = function(value) {
  return jspb.Message.setField(this, 9, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.addDiscoveries = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 9, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.clearDiscoveriesList = function() {
  return this.setDiscoveriesList([]);
};


/**
 * repeated string keywords = 10;
 * @return {!Array<string>}
 */
proto.resource.PackageDescriptor.prototype.getKeywordsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 10));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.setKeywordsList = function(value) {
  return jspb.Message.setField(this, 10, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.addKeywords = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 10, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.clearKeywordsList = function() {
  return this.setKeywordsList([]);
};


/**
 * repeated string actions = 11;
 * @return {!Array<string>}
 */
proto.resource.PackageDescriptor.prototype.getActionsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 11));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.setActionsList = function(value) {
  return jspb.Message.setField(this, 11, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.addActions = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 11, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.clearActionsList = function() {
  return this.setActionsList([]);
};


/**
 * repeated Role roles = 12;
 * @return {!Array<!proto.resource.Role>}
 */
proto.resource.PackageDescriptor.prototype.getRolesList = function() {
  return /** @type{!Array<!proto.resource.Role>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.resource.Role, 12));
};


/**
 * @param {!Array<!proto.resource.Role>} value
 * @return {!proto.resource.PackageDescriptor} returns this
*/
proto.resource.PackageDescriptor.prototype.setRolesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 12, value);
};


/**
 * @param {!proto.resource.Role=} opt_value
 * @param {number=} opt_index
 * @return {!proto.resource.Role}
 */
proto.resource.PackageDescriptor.prototype.addRoles = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 12, opt_value, proto.resource.Role, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.clearRolesList = function() {
  return this.setRolesList([]);
};


/**
 * repeated Group groups = 13;
 * @return {!Array<!proto.resource.Group>}
 */
proto.resource.PackageDescriptor.prototype.getGroupsList = function() {
  return /** @type{!Array<!proto.resource.Group>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.resource.Group, 13));
};


/**
 * @param {!Array<!proto.resource.Group>} value
 * @return {!proto.resource.PackageDescriptor} returns this
*/
proto.resource.PackageDescriptor.prototype.setGroupsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 13, value);
};


/**
 * @param {!proto.resource.Group=} opt_value
 * @param {number=} opt_index
 * @return {!proto.resource.Group}
 */
proto.resource.PackageDescriptor.prototype.addGroups = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 13, opt_value, proto.resource.Group, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.clearGroupsList = function() {
  return this.setGroupsList([]);
};


/**
 * optional string icon = 14;
 * @return {string}
 */
proto.resource.PackageDescriptor.prototype.getIcon = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 14, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.setIcon = function(value) {
  return jspb.Message.setProto3StringField(this, 14, value);
};


/**
 * optional string alias = 15;
 * @return {string}
 */
proto.resource.PackageDescriptor.prototype.getAlias = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 15, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.PackageDescriptor} returns this
 */
proto.resource.PackageDescriptor.prototype.setAlias = function(value) {
  return jspb.Message.setProto3StringField(this, 15, value);
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
proto.resource.GetPackageDescriptorRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetPackageDescriptorRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetPackageDescriptorRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetPackageDescriptorRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    serviceid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    publisherid: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.GetPackageDescriptorRequest}
 */
proto.resource.GetPackageDescriptorRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetPackageDescriptorRequest;
  return proto.resource.GetPackageDescriptorRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetPackageDescriptorRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetPackageDescriptorRequest}
 */
proto.resource.GetPackageDescriptorRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setServiceid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisherid(value);
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
proto.resource.GetPackageDescriptorRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetPackageDescriptorRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetPackageDescriptorRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetPackageDescriptorRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getServiceid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getPublisherid();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string serviceId = 1;
 * @return {string}
 */
proto.resource.GetPackageDescriptorRequest.prototype.getServiceid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetPackageDescriptorRequest} returns this
 */
proto.resource.GetPackageDescriptorRequest.prototype.setServiceid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string publisherId = 2;
 * @return {string}
 */
proto.resource.GetPackageDescriptorRequest.prototype.getPublisherid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetPackageDescriptorRequest} returns this
 */
proto.resource.GetPackageDescriptorRequest.prototype.setPublisherid = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.GetPackageDescriptorResponse.repeatedFields_ = [1];



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
proto.resource.GetPackageDescriptorResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetPackageDescriptorResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetPackageDescriptorResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetPackageDescriptorResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    resultsList: jspb.Message.toObjectList(msg.getResultsList(),
    proto.resource.PackageDescriptor.toObject, includeInstance)
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
 * @return {!proto.resource.GetPackageDescriptorResponse}
 */
proto.resource.GetPackageDescriptorResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetPackageDescriptorResponse;
  return proto.resource.GetPackageDescriptorResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetPackageDescriptorResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetPackageDescriptorResponse}
 */
proto.resource.GetPackageDescriptorResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.PackageDescriptor;
      reader.readMessage(value,proto.resource.PackageDescriptor.deserializeBinaryFromReader);
      msg.addResults(value);
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
proto.resource.GetPackageDescriptorResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetPackageDescriptorResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetPackageDescriptorResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetPackageDescriptorResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResultsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.resource.PackageDescriptor.serializeBinaryToWriter
    );
  }
};


/**
 * repeated PackageDescriptor results = 1;
 * @return {!Array<!proto.resource.PackageDescriptor>}
 */
proto.resource.GetPackageDescriptorResponse.prototype.getResultsList = function() {
  return /** @type{!Array<!proto.resource.PackageDescriptor>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.resource.PackageDescriptor, 1));
};


/**
 * @param {!Array<!proto.resource.PackageDescriptor>} value
 * @return {!proto.resource.GetPackageDescriptorResponse} returns this
*/
proto.resource.GetPackageDescriptorResponse.prototype.setResultsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.resource.PackageDescriptor=} opt_value
 * @param {number=} opt_index
 * @return {!proto.resource.PackageDescriptor}
 */
proto.resource.GetPackageDescriptorResponse.prototype.addResults = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.resource.PackageDescriptor, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.GetPackageDescriptorResponse} returns this
 */
proto.resource.GetPackageDescriptorResponse.prototype.clearResultsList = function() {
  return this.setResultsList([]);
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
proto.resource.GetPackagesDescriptorRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetPackagesDescriptorRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetPackagesDescriptorRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetPackagesDescriptorRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    query: jspb.Message.getFieldWithDefault(msg, 1, ""),
    options: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.GetPackagesDescriptorRequest}
 */
proto.resource.GetPackagesDescriptorRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetPackagesDescriptorRequest;
  return proto.resource.GetPackagesDescriptorRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetPackagesDescriptorRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetPackagesDescriptorRequest}
 */
proto.resource.GetPackagesDescriptorRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setOptions(value);
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
proto.resource.GetPackagesDescriptorRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetPackagesDescriptorRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetPackagesDescriptorRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetPackagesDescriptorRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getQuery();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOptions();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string query = 1;
 * @return {string}
 */
proto.resource.GetPackagesDescriptorRequest.prototype.getQuery = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetPackagesDescriptorRequest} returns this
 */
proto.resource.GetPackagesDescriptorRequest.prototype.setQuery = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string options = 2;
 * @return {string}
 */
proto.resource.GetPackagesDescriptorRequest.prototype.getOptions = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetPackagesDescriptorRequest} returns this
 */
proto.resource.GetPackagesDescriptorRequest.prototype.setOptions = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.GetPackagesDescriptorResponse.repeatedFields_ = [1];



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
proto.resource.GetPackagesDescriptorResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetPackagesDescriptorResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetPackagesDescriptorResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetPackagesDescriptorResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    resultsList: jspb.Message.toObjectList(msg.getResultsList(),
    proto.resource.PackageDescriptor.toObject, includeInstance)
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
 * @return {!proto.resource.GetPackagesDescriptorResponse}
 */
proto.resource.GetPackagesDescriptorResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetPackagesDescriptorResponse;
  return proto.resource.GetPackagesDescriptorResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetPackagesDescriptorResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetPackagesDescriptorResponse}
 */
proto.resource.GetPackagesDescriptorResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.PackageDescriptor;
      reader.readMessage(value,proto.resource.PackageDescriptor.deserializeBinaryFromReader);
      msg.addResults(value);
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
proto.resource.GetPackagesDescriptorResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetPackagesDescriptorResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetPackagesDescriptorResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetPackagesDescriptorResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResultsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.resource.PackageDescriptor.serializeBinaryToWriter
    );
  }
};


/**
 * repeated PackageDescriptor results = 1;
 * @return {!Array<!proto.resource.PackageDescriptor>}
 */
proto.resource.GetPackagesDescriptorResponse.prototype.getResultsList = function() {
  return /** @type{!Array<!proto.resource.PackageDescriptor>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.resource.PackageDescriptor, 1));
};


/**
 * @param {!Array<!proto.resource.PackageDescriptor>} value
 * @return {!proto.resource.GetPackagesDescriptorResponse} returns this
*/
proto.resource.GetPackagesDescriptorResponse.prototype.setResultsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.resource.PackageDescriptor=} opt_value
 * @param {number=} opt_index
 * @return {!proto.resource.PackageDescriptor}
 */
proto.resource.GetPackagesDescriptorResponse.prototype.addResults = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.resource.PackageDescriptor, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.GetPackagesDescriptorResponse} returns this
 */
proto.resource.GetPackagesDescriptorResponse.prototype.clearResultsList = function() {
  return this.setResultsList([]);
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
proto.resource.SetPackageDescriptorRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.SetPackageDescriptorRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.SetPackageDescriptorRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.SetPackageDescriptorRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    packagedescriptor: (f = msg.getPackagedescriptor()) && proto.resource.PackageDescriptor.toObject(includeInstance, f)
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
 * @return {!proto.resource.SetPackageDescriptorRequest}
 */
proto.resource.SetPackageDescriptorRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.SetPackageDescriptorRequest;
  return proto.resource.SetPackageDescriptorRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.SetPackageDescriptorRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.SetPackageDescriptorRequest}
 */
proto.resource.SetPackageDescriptorRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.PackageDescriptor;
      reader.readMessage(value,proto.resource.PackageDescriptor.deserializeBinaryFromReader);
      msg.setPackagedescriptor(value);
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
proto.resource.SetPackageDescriptorRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.SetPackageDescriptorRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.SetPackageDescriptorRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.SetPackageDescriptorRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPackagedescriptor();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.resource.PackageDescriptor.serializeBinaryToWriter
    );
  }
};


/**
 * optional PackageDescriptor packageDescriptor = 1;
 * @return {?proto.resource.PackageDescriptor}
 */
proto.resource.SetPackageDescriptorRequest.prototype.getPackagedescriptor = function() {
  return /** @type{?proto.resource.PackageDescriptor} */ (
    jspb.Message.getWrapperField(this, proto.resource.PackageDescriptor, 1));
};


/**
 * @param {?proto.resource.PackageDescriptor|undefined} value
 * @return {!proto.resource.SetPackageDescriptorRequest} returns this
*/
proto.resource.SetPackageDescriptorRequest.prototype.setPackagedescriptor = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.resource.SetPackageDescriptorRequest} returns this
 */
proto.resource.SetPackageDescriptorRequest.prototype.clearPackagedescriptor = function() {
  return this.setPackagedescriptor(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.resource.SetPackageDescriptorRequest.prototype.hasPackagedescriptor = function() {
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
proto.resource.SetPackageDescriptorResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.SetPackageDescriptorResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.SetPackageDescriptorResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.SetPackageDescriptorResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.SetPackageDescriptorResponse}
 */
proto.resource.SetPackageDescriptorResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.SetPackageDescriptorResponse;
  return proto.resource.SetPackageDescriptorResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.SetPackageDescriptorResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.SetPackageDescriptorResponse}
 */
proto.resource.SetPackageDescriptorResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.SetPackageDescriptorResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.SetPackageDescriptorResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.SetPackageDescriptorResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.SetPackageDescriptorResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.SetPackageDescriptorResponse.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.SetPackageDescriptorResponse} returns this
 */
proto.resource.SetPackageDescriptorResponse.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.FindPackagesDescriptorRequest.repeatedFields_ = [1];



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
proto.resource.FindPackagesDescriptorRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.FindPackagesDescriptorRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.FindPackagesDescriptorRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.FindPackagesDescriptorRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    keywordsList: (f = jspb.Message.getRepeatedField(msg, 1)) == null ? undefined : f
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
 * @return {!proto.resource.FindPackagesDescriptorRequest}
 */
proto.resource.FindPackagesDescriptorRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.FindPackagesDescriptorRequest;
  return proto.resource.FindPackagesDescriptorRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.FindPackagesDescriptorRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.FindPackagesDescriptorRequest}
 */
proto.resource.FindPackagesDescriptorRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.addKeywords(value);
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
proto.resource.FindPackagesDescriptorRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.FindPackagesDescriptorRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.FindPackagesDescriptorRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.FindPackagesDescriptorRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getKeywordsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      1,
      f
    );
  }
};


/**
 * repeated string keywords = 1;
 * @return {!Array<string>}
 */
proto.resource.FindPackagesDescriptorRequest.prototype.getKeywordsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 1));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.FindPackagesDescriptorRequest} returns this
 */
proto.resource.FindPackagesDescriptorRequest.prototype.setKeywordsList = function(value) {
  return jspb.Message.setField(this, 1, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.FindPackagesDescriptorRequest} returns this
 */
proto.resource.FindPackagesDescriptorRequest.prototype.addKeywords = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 1, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.FindPackagesDescriptorRequest} returns this
 */
proto.resource.FindPackagesDescriptorRequest.prototype.clearKeywordsList = function() {
  return this.setKeywordsList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.FindPackagesDescriptorResponse.repeatedFields_ = [1];



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
proto.resource.FindPackagesDescriptorResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.FindPackagesDescriptorResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.FindPackagesDescriptorResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.FindPackagesDescriptorResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    resultsList: jspb.Message.toObjectList(msg.getResultsList(),
    proto.resource.PackageDescriptor.toObject, includeInstance)
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
 * @return {!proto.resource.FindPackagesDescriptorResponse}
 */
proto.resource.FindPackagesDescriptorResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.FindPackagesDescriptorResponse;
  return proto.resource.FindPackagesDescriptorResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.FindPackagesDescriptorResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.FindPackagesDescriptorResponse}
 */
proto.resource.FindPackagesDescriptorResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.PackageDescriptor;
      reader.readMessage(value,proto.resource.PackageDescriptor.deserializeBinaryFromReader);
      msg.addResults(value);
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
proto.resource.FindPackagesDescriptorResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.FindPackagesDescriptorResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.FindPackagesDescriptorResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.FindPackagesDescriptorResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResultsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.resource.PackageDescriptor.serializeBinaryToWriter
    );
  }
};


/**
 * repeated PackageDescriptor results = 1;
 * @return {!Array<!proto.resource.PackageDescriptor>}
 */
proto.resource.FindPackagesDescriptorResponse.prototype.getResultsList = function() {
  return /** @type{!Array<!proto.resource.PackageDescriptor>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.resource.PackageDescriptor, 1));
};


/**
 * @param {!Array<!proto.resource.PackageDescriptor>} value
 * @return {!proto.resource.FindPackagesDescriptorResponse} returns this
*/
proto.resource.FindPackagesDescriptorResponse.prototype.setResultsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.resource.PackageDescriptor=} opt_value
 * @param {number=} opt_index
 * @return {!proto.resource.PackageDescriptor}
 */
proto.resource.FindPackagesDescriptorResponse.prototype.addResults = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.resource.PackageDescriptor, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.FindPackagesDescriptorResponse} returns this
 */
proto.resource.FindPackagesDescriptorResponse.prototype.clearResultsList = function() {
  return this.setResultsList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.Role.repeatedFields_ = [5,6,7];



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
proto.resource.Role.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.Role.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.Role} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.Role.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, ""),
    name: jspb.Message.getFieldWithDefault(msg, 2, ""),
    domain: jspb.Message.getFieldWithDefault(msg, 3, ""),
    description: jspb.Message.getFieldWithDefault(msg, 4, ""),
    actionsList: (f = jspb.Message.getRepeatedField(msg, 5)) == null ? undefined : f,
    membersList: (f = jspb.Message.getRepeatedField(msg, 6)) == null ? undefined : f,
    organizationsList: (f = jspb.Message.getRepeatedField(msg, 7)) == null ? undefined : f
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
 * @return {!proto.resource.Role}
 */
proto.resource.Role.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.Role;
  return proto.resource.Role.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.Role} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.Role}
 */
proto.resource.Role.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setName(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setDomain(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setDescription(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.addActions(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.addMembers(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.addOrganizations(value);
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
proto.resource.Role.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.Role.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.Role} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.Role.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getDomain();
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
  f = message.getActionsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      5,
      f
    );
  }
  f = message.getMembersList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      6,
      f
    );
  }
  f = message.getOrganizationsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      7,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.resource.Role.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Role} returns this
 */
proto.resource.Role.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string name = 2;
 * @return {string}
 */
proto.resource.Role.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Role} returns this
 */
proto.resource.Role.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string domain = 3;
 * @return {string}
 */
proto.resource.Role.prototype.getDomain = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Role} returns this
 */
proto.resource.Role.prototype.setDomain = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string description = 4;
 * @return {string}
 */
proto.resource.Role.prototype.getDescription = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Role} returns this
 */
proto.resource.Role.prototype.setDescription = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * repeated string actions = 5;
 * @return {!Array<string>}
 */
proto.resource.Role.prototype.getActionsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 5));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.Role} returns this
 */
proto.resource.Role.prototype.setActionsList = function(value) {
  return jspb.Message.setField(this, 5, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.Role} returns this
 */
proto.resource.Role.prototype.addActions = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 5, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.Role} returns this
 */
proto.resource.Role.prototype.clearActionsList = function() {
  return this.setActionsList([]);
};


/**
 * repeated string members = 6;
 * @return {!Array<string>}
 */
proto.resource.Role.prototype.getMembersList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 6));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.Role} returns this
 */
proto.resource.Role.prototype.setMembersList = function(value) {
  return jspb.Message.setField(this, 6, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.Role} returns this
 */
proto.resource.Role.prototype.addMembers = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 6, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.Role} returns this
 */
proto.resource.Role.prototype.clearMembersList = function() {
  return this.setMembersList([]);
};


/**
 * repeated string organizations = 7;
 * @return {!Array<string>}
 */
proto.resource.Role.prototype.getOrganizationsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 7));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.Role} returns this
 */
proto.resource.Role.prototype.setOrganizationsList = function(value) {
  return jspb.Message.setField(this, 7, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.Role} returns this
 */
proto.resource.Role.prototype.addOrganizations = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 7, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.Role} returns this
 */
proto.resource.Role.prototype.clearOrganizationsList = function() {
  return this.setOrganizationsList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.AddRoleActionsRqst.repeatedFields_ = [2];



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
proto.resource.AddRoleActionsRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AddRoleActionsRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AddRoleActionsRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddRoleActionsRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    roleid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    actionsList: (f = jspb.Message.getRepeatedField(msg, 2)) == null ? undefined : f
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
 * @return {!proto.resource.AddRoleActionsRqst}
 */
proto.resource.AddRoleActionsRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AddRoleActionsRqst;
  return proto.resource.AddRoleActionsRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AddRoleActionsRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AddRoleActionsRqst}
 */
proto.resource.AddRoleActionsRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRoleid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.addActions(value);
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
proto.resource.AddRoleActionsRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AddRoleActionsRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AddRoleActionsRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddRoleActionsRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRoleid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getActionsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      2,
      f
    );
  }
};


/**
 * optional string roleId = 1;
 * @return {string}
 */
proto.resource.AddRoleActionsRqst.prototype.getRoleid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.AddRoleActionsRqst} returns this
 */
proto.resource.AddRoleActionsRqst.prototype.setRoleid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * repeated string actions = 2;
 * @return {!Array<string>}
 */
proto.resource.AddRoleActionsRqst.prototype.getActionsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.AddRoleActionsRqst} returns this
 */
proto.resource.AddRoleActionsRqst.prototype.setActionsList = function(value) {
  return jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.AddRoleActionsRqst} returns this
 */
proto.resource.AddRoleActionsRqst.prototype.addActions = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.AddRoleActionsRqst} returns this
 */
proto.resource.AddRoleActionsRqst.prototype.clearActionsList = function() {
  return this.setActionsList([]);
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
proto.resource.AddRoleActionsRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AddRoleActionsRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AddRoleActionsRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddRoleActionsRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.AddRoleActionsRsp}
 */
proto.resource.AddRoleActionsRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AddRoleActionsRsp;
  return proto.resource.AddRoleActionsRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AddRoleActionsRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AddRoleActionsRsp}
 */
proto.resource.AddRoleActionsRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.AddRoleActionsRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AddRoleActionsRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AddRoleActionsRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddRoleActionsRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.AddRoleActionsRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.AddRoleActionsRsp} returns this
 */
proto.resource.AddRoleActionsRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.RemoveRoleActionRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveRoleActionRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveRoleActionRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveRoleActionRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    roleid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    action: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.RemoveRoleActionRqst}
 */
proto.resource.RemoveRoleActionRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveRoleActionRqst;
  return proto.resource.RemoveRoleActionRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveRoleActionRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveRoleActionRqst}
 */
proto.resource.RemoveRoleActionRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRoleid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setAction(value);
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
proto.resource.RemoveRoleActionRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveRoleActionRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveRoleActionRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveRoleActionRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRoleid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getAction();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string roleId = 1;
 * @return {string}
 */
proto.resource.RemoveRoleActionRqst.prototype.getRoleid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemoveRoleActionRqst} returns this
 */
proto.resource.RemoveRoleActionRqst.prototype.setRoleid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string action = 2;
 * @return {string}
 */
proto.resource.RemoveRoleActionRqst.prototype.getAction = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemoveRoleActionRqst} returns this
 */
proto.resource.RemoveRoleActionRqst.prototype.setAction = function(value) {
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
proto.resource.RemoveRoleActionRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveRoleActionRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveRoleActionRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveRoleActionRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.RemoveRoleActionRsp}
 */
proto.resource.RemoveRoleActionRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveRoleActionRsp;
  return proto.resource.RemoveRoleActionRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveRoleActionRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveRoleActionRsp}
 */
proto.resource.RemoveRoleActionRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.RemoveRoleActionRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveRoleActionRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveRoleActionRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveRoleActionRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.RemoveRoleActionRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.RemoveRoleActionRsp} returns this
 */
proto.resource.RemoveRoleActionRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.RemoveRolesActionRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveRolesActionRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveRolesActionRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveRolesActionRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    action: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.RemoveRolesActionRqst}
 */
proto.resource.RemoveRolesActionRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveRolesActionRqst;
  return proto.resource.RemoveRolesActionRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveRolesActionRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveRolesActionRqst}
 */
proto.resource.RemoveRolesActionRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setAction(value);
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
proto.resource.RemoveRolesActionRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveRolesActionRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveRolesActionRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveRolesActionRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAction();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string action = 2;
 * @return {string}
 */
proto.resource.RemoveRolesActionRqst.prototype.getAction = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemoveRolesActionRqst} returns this
 */
proto.resource.RemoveRolesActionRqst.prototype.setAction = function(value) {
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
proto.resource.RemoveRolesActionRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveRolesActionRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveRolesActionRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveRolesActionRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.RemoveRolesActionRsp}
 */
proto.resource.RemoveRolesActionRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveRolesActionRsp;
  return proto.resource.RemoveRolesActionRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveRolesActionRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveRolesActionRsp}
 */
proto.resource.RemoveRolesActionRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.RemoveRolesActionRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveRolesActionRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveRolesActionRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveRolesActionRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.RemoveRolesActionRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.RemoveRolesActionRsp} returns this
 */
proto.resource.RemoveRolesActionRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.Account.repeatedFields_ = [6,7,8];



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
proto.resource.Account.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.Account.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.Account} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.Account.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, ""),
    name: jspb.Message.getFieldWithDefault(msg, 2, ""),
    email: jspb.Message.getFieldWithDefault(msg, 3, ""),
    password: jspb.Message.getFieldWithDefault(msg, 4, ""),
    domain: jspb.Message.getFieldWithDefault(msg, 5, ""),
    organizationsList: (f = jspb.Message.getRepeatedField(msg, 6)) == null ? undefined : f,
    groupsList: (f = jspb.Message.getRepeatedField(msg, 7)) == null ? undefined : f,
    rolesList: (f = jspb.Message.getRepeatedField(msg, 8)) == null ? undefined : f
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
 * @return {!proto.resource.Account}
 */
proto.resource.Account.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.Account;
  return proto.resource.Account.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.Account} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.Account}
 */
proto.resource.Account.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setName(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setEmail(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setPassword(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setDomain(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.addOrganizations(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.addGroups(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.addRoles(value);
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
proto.resource.Account.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.Account.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.Account} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.Account.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getEmail();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getPassword();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getDomain();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getOrganizationsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      6,
      f
    );
  }
  f = message.getGroupsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      7,
      f
    );
  }
  f = message.getRolesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      8,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.resource.Account.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Account} returns this
 */
proto.resource.Account.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string name = 2;
 * @return {string}
 */
proto.resource.Account.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Account} returns this
 */
proto.resource.Account.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string email = 3;
 * @return {string}
 */
proto.resource.Account.prototype.getEmail = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Account} returns this
 */
proto.resource.Account.prototype.setEmail = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string password = 4;
 * @return {string}
 */
proto.resource.Account.prototype.getPassword = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Account} returns this
 */
proto.resource.Account.prototype.setPassword = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string domain = 5;
 * @return {string}
 */
proto.resource.Account.prototype.getDomain = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Account} returns this
 */
proto.resource.Account.prototype.setDomain = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * repeated string organizations = 6;
 * @return {!Array<string>}
 */
proto.resource.Account.prototype.getOrganizationsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 6));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.Account} returns this
 */
proto.resource.Account.prototype.setOrganizationsList = function(value) {
  return jspb.Message.setField(this, 6, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.Account} returns this
 */
proto.resource.Account.prototype.addOrganizations = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 6, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.Account} returns this
 */
proto.resource.Account.prototype.clearOrganizationsList = function() {
  return this.setOrganizationsList([]);
};


/**
 * repeated string groups = 7;
 * @return {!Array<string>}
 */
proto.resource.Account.prototype.getGroupsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 7));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.Account} returns this
 */
proto.resource.Account.prototype.setGroupsList = function(value) {
  return jspb.Message.setField(this, 7, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.Account} returns this
 */
proto.resource.Account.prototype.addGroups = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 7, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.Account} returns this
 */
proto.resource.Account.prototype.clearGroupsList = function() {
  return this.setGroupsList([]);
};


/**
 * repeated string roles = 8;
 * @return {!Array<string>}
 */
proto.resource.Account.prototype.getRolesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 8));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.Account} returns this
 */
proto.resource.Account.prototype.setRolesList = function(value) {
  return jspb.Message.setField(this, 8, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.Account} returns this
 */
proto.resource.Account.prototype.addRoles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 8, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.Account} returns this
 */
proto.resource.Account.prototype.clearRolesList = function() {
  return this.setRolesList([]);
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
proto.resource.RegisterAccountRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RegisterAccountRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RegisterAccountRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RegisterAccountRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    account: (f = msg.getAccount()) && proto.resource.Account.toObject(includeInstance, f),
    confirmPassword: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.resource.RegisterAccountRqst}
 */
proto.resource.RegisterAccountRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RegisterAccountRqst;
  return proto.resource.RegisterAccountRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RegisterAccountRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RegisterAccountRqst}
 */
proto.resource.RegisterAccountRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Account;
      reader.readMessage(value,proto.resource.Account.deserializeBinaryFromReader);
      msg.setAccount(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setConfirmPassword(value);
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
proto.resource.RegisterAccountRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RegisterAccountRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RegisterAccountRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RegisterAccountRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAccount();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.resource.Account.serializeBinaryToWriter
    );
  }
  f = message.getConfirmPassword();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional Account account = 1;
 * @return {?proto.resource.Account}
 */
proto.resource.RegisterAccountRqst.prototype.getAccount = function() {
  return /** @type{?proto.resource.Account} */ (
    jspb.Message.getWrapperField(this, proto.resource.Account, 1));
};


/**
 * @param {?proto.resource.Account|undefined} value
 * @return {!proto.resource.RegisterAccountRqst} returns this
*/
proto.resource.RegisterAccountRqst.prototype.setAccount = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.resource.RegisterAccountRqst} returns this
 */
proto.resource.RegisterAccountRqst.prototype.clearAccount = function() {
  return this.setAccount(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.resource.RegisterAccountRqst.prototype.hasAccount = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string confirm_password = 3;
 * @return {string}
 */
proto.resource.RegisterAccountRqst.prototype.getConfirmPassword = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RegisterAccountRqst} returns this
 */
proto.resource.RegisterAccountRqst.prototype.setConfirmPassword = function(value) {
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
proto.resource.RegisterAccountRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RegisterAccountRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RegisterAccountRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RegisterAccountRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.resource.RegisterAccountRsp}
 */
proto.resource.RegisterAccountRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RegisterAccountRsp;
  return proto.resource.RegisterAccountRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RegisterAccountRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RegisterAccountRsp}
 */
proto.resource.RegisterAccountRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setResult(value);
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
proto.resource.RegisterAccountRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RegisterAccountRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RegisterAccountRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RegisterAccountRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string result = 1;
 * @return {string}
 */
proto.resource.RegisterAccountRsp.prototype.getResult = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RegisterAccountRsp} returns this
 */
proto.resource.RegisterAccountRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
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
proto.resource.AccountExistRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AccountExistRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AccountExistRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AccountExistRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.resource.AccountExistRqst}
 */
proto.resource.AccountExistRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AccountExistRqst;
  return proto.resource.AccountExistRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AccountExistRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AccountExistRqst}
 */
proto.resource.AccountExistRqst.deserializeBinaryFromReader = function(msg, reader) {
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
proto.resource.AccountExistRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AccountExistRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AccountExistRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AccountExistRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.resource.AccountExistRqst.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.AccountExistRqst} returns this
 */
proto.resource.AccountExistRqst.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
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
proto.resource.AccountExistRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AccountExistRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AccountExistRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AccountExistRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.AccountExistRsp}
 */
proto.resource.AccountExistRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AccountExistRsp;
  return proto.resource.AccountExistRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AccountExistRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AccountExistRsp}
 */
proto.resource.AccountExistRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.AccountExistRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AccountExistRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AccountExistRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AccountExistRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.AccountExistRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.AccountExistRsp} returns this
 */
proto.resource.AccountExistRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.GetAccountsRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetAccountsRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetAccountsRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetAccountsRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    query: jspb.Message.getFieldWithDefault(msg, 1, ""),
    options: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.GetAccountsRqst}
 */
proto.resource.GetAccountsRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetAccountsRqst;
  return proto.resource.GetAccountsRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetAccountsRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetAccountsRqst}
 */
proto.resource.GetAccountsRqst.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setOptions(value);
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
proto.resource.GetAccountsRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetAccountsRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetAccountsRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetAccountsRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getQuery();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOptions();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string query = 1;
 * @return {string}
 */
proto.resource.GetAccountsRqst.prototype.getQuery = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetAccountsRqst} returns this
 */
proto.resource.GetAccountsRqst.prototype.setQuery = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string options = 2;
 * @return {string}
 */
proto.resource.GetAccountsRqst.prototype.getOptions = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetAccountsRqst} returns this
 */
proto.resource.GetAccountsRqst.prototype.setOptions = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.GetAccountsRsp.repeatedFields_ = [1];



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
proto.resource.GetAccountsRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetAccountsRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetAccountsRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetAccountsRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    accountsList: jspb.Message.toObjectList(msg.getAccountsList(),
    proto.resource.Account.toObject, includeInstance)
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
 * @return {!proto.resource.GetAccountsRsp}
 */
proto.resource.GetAccountsRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetAccountsRsp;
  return proto.resource.GetAccountsRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetAccountsRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetAccountsRsp}
 */
proto.resource.GetAccountsRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Account;
      reader.readMessage(value,proto.resource.Account.deserializeBinaryFromReader);
      msg.addAccounts(value);
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
proto.resource.GetAccountsRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetAccountsRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetAccountsRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetAccountsRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAccountsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.resource.Account.serializeBinaryToWriter
    );
  }
};


/**
 * repeated Account accounts = 1;
 * @return {!Array<!proto.resource.Account>}
 */
proto.resource.GetAccountsRsp.prototype.getAccountsList = function() {
  return /** @type{!Array<!proto.resource.Account>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.resource.Account, 1));
};


/**
 * @param {!Array<!proto.resource.Account>} value
 * @return {!proto.resource.GetAccountsRsp} returns this
*/
proto.resource.GetAccountsRsp.prototype.setAccountsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.resource.Account=} opt_value
 * @param {number=} opt_index
 * @return {!proto.resource.Account}
 */
proto.resource.GetAccountsRsp.prototype.addAccounts = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.resource.Account, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.GetAccountsRsp} returns this
 */
proto.resource.GetAccountsRsp.prototype.clearAccountsList = function() {
  return this.setAccountsList([]);
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
proto.resource.GetAccountRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetAccountRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetAccountRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetAccountRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    accountid: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.resource.GetAccountRqst}
 */
proto.resource.GetAccountRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetAccountRqst;
  return proto.resource.GetAccountRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetAccountRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetAccountRqst}
 */
proto.resource.GetAccountRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setAccountid(value);
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
proto.resource.GetAccountRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetAccountRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetAccountRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetAccountRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAccountid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string accountId = 1;
 * @return {string}
 */
proto.resource.GetAccountRqst.prototype.getAccountid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetAccountRqst} returns this
 */
proto.resource.GetAccountRqst.prototype.setAccountid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
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
proto.resource.GetAccountRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetAccountRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetAccountRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetAccountRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    account: (f = msg.getAccount()) && proto.resource.Account.toObject(includeInstance, f)
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
 * @return {!proto.resource.GetAccountRsp}
 */
proto.resource.GetAccountRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetAccountRsp;
  return proto.resource.GetAccountRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetAccountRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetAccountRsp}
 */
proto.resource.GetAccountRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Account;
      reader.readMessage(value,proto.resource.Account.deserializeBinaryFromReader);
      msg.setAccount(value);
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
proto.resource.GetAccountRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetAccountRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetAccountRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetAccountRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAccount();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.resource.Account.serializeBinaryToWriter
    );
  }
};


/**
 * optional Account account = 1;
 * @return {?proto.resource.Account}
 */
proto.resource.GetAccountRsp.prototype.getAccount = function() {
  return /** @type{?proto.resource.Account} */ (
    jspb.Message.getWrapperField(this, proto.resource.Account, 1));
};


/**
 * @param {?proto.resource.Account|undefined} value
 * @return {!proto.resource.GetAccountRsp} returns this
*/
proto.resource.GetAccountRsp.prototype.setAccount = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.resource.GetAccountRsp} returns this
 */
proto.resource.GetAccountRsp.prototype.clearAccount = function() {
  return this.setAccount(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.resource.GetAccountRsp.prototype.hasAccount = function() {
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
proto.resource.SetAccountPasswordRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.SetAccountPasswordRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.SetAccountPasswordRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.SetAccountPasswordRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    accountid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    oldPassword: jspb.Message.getFieldWithDefault(msg, 2, ""),
    newPassword: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.resource.SetAccountPasswordRqst}
 */
proto.resource.SetAccountPasswordRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.SetAccountPasswordRqst;
  return proto.resource.SetAccountPasswordRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.SetAccountPasswordRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.SetAccountPasswordRqst}
 */
proto.resource.SetAccountPasswordRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setAccountid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setOldPassword(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setNewPassword(value);
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
proto.resource.SetAccountPasswordRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.SetAccountPasswordRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.SetAccountPasswordRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.SetAccountPasswordRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAccountid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOldPassword();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getNewPassword();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string accountId = 1;
 * @return {string}
 */
proto.resource.SetAccountPasswordRqst.prototype.getAccountid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.SetAccountPasswordRqst} returns this
 */
proto.resource.SetAccountPasswordRqst.prototype.setAccountid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string old_password = 2;
 * @return {string}
 */
proto.resource.SetAccountPasswordRqst.prototype.getOldPassword = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.SetAccountPasswordRqst} returns this
 */
proto.resource.SetAccountPasswordRqst.prototype.setOldPassword = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string new_password = 3;
 * @return {string}
 */
proto.resource.SetAccountPasswordRqst.prototype.getNewPassword = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.SetAccountPasswordRqst} returns this
 */
proto.resource.SetAccountPasswordRqst.prototype.setNewPassword = function(value) {
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
proto.resource.SetAccountPasswordRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.SetAccountPasswordRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.SetAccountPasswordRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.SetAccountPasswordRsp.toObject = function(includeInstance, msg) {
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
 * @return {!proto.resource.SetAccountPasswordRsp}
 */
proto.resource.SetAccountPasswordRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.SetAccountPasswordRsp;
  return proto.resource.SetAccountPasswordRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.SetAccountPasswordRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.SetAccountPasswordRsp}
 */
proto.resource.SetAccountPasswordRsp.deserializeBinaryFromReader = function(msg, reader) {
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
proto.resource.SetAccountPasswordRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.SetAccountPasswordRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.SetAccountPasswordRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.SetAccountPasswordRsp.serializeBinaryToWriter = function(message, writer) {
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
proto.resource.SetEmailRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.SetEmailRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.SetEmailRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.SetEmailRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    accountid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    oldemail: jspb.Message.getFieldWithDefault(msg, 2, ""),
    newemail: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.resource.SetEmailRequest}
 */
proto.resource.SetEmailRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.SetEmailRequest;
  return proto.resource.SetEmailRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.SetEmailRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.SetEmailRequest}
 */
proto.resource.SetEmailRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setAccountid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setOldemail(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setNewemail(value);
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
proto.resource.SetEmailRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.SetEmailRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.SetEmailRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.SetEmailRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAccountid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOldemail();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getNewemail();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string accountId = 1;
 * @return {string}
 */
proto.resource.SetEmailRequest.prototype.getAccountid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.SetEmailRequest} returns this
 */
proto.resource.SetEmailRequest.prototype.setAccountid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string oldEmail = 2;
 * @return {string}
 */
proto.resource.SetEmailRequest.prototype.getOldemail = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.SetEmailRequest} returns this
 */
proto.resource.SetEmailRequest.prototype.setOldemail = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string newEmail = 3;
 * @return {string}
 */
proto.resource.SetEmailRequest.prototype.getNewemail = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.SetEmailRequest} returns this
 */
proto.resource.SetEmailRequest.prototype.setNewemail = function(value) {
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
proto.resource.SetEmailResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.SetEmailResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.SetEmailResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.SetEmailResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.resource.SetEmailResponse}
 */
proto.resource.SetEmailResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.SetEmailResponse;
  return proto.resource.SetEmailResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.SetEmailResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.SetEmailResponse}
 */
proto.resource.SetEmailResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.resource.SetEmailResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.SetEmailResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.SetEmailResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.SetEmailResponse.serializeBinaryToWriter = function(message, writer) {
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
proto.resource.DeleteAccountRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.DeleteAccountRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.DeleteAccountRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteAccountRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.resource.DeleteAccountRqst}
 */
proto.resource.DeleteAccountRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.DeleteAccountRqst;
  return proto.resource.DeleteAccountRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.DeleteAccountRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.DeleteAccountRqst}
 */
proto.resource.DeleteAccountRqst.deserializeBinaryFromReader = function(msg, reader) {
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
proto.resource.DeleteAccountRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.DeleteAccountRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.DeleteAccountRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteAccountRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.resource.DeleteAccountRqst.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.DeleteAccountRqst} returns this
 */
proto.resource.DeleteAccountRqst.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
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
proto.resource.DeleteAccountRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.DeleteAccountRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.DeleteAccountRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteAccountRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.resource.DeleteAccountRsp}
 */
proto.resource.DeleteAccountRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.DeleteAccountRsp;
  return proto.resource.DeleteAccountRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.DeleteAccountRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.DeleteAccountRsp}
 */
proto.resource.DeleteAccountRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setResult(value);
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
proto.resource.DeleteAccountRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.DeleteAccountRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.DeleteAccountRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteAccountRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string result = 1;
 * @return {string}
 */
proto.resource.DeleteAccountRsp.prototype.getResult = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.DeleteAccountRsp} returns this
 */
proto.resource.DeleteAccountRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
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
proto.resource.AddAccountRoleRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AddAccountRoleRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AddAccountRoleRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddAccountRoleRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    accountid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    roleid: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.AddAccountRoleRqst}
 */
proto.resource.AddAccountRoleRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AddAccountRoleRqst;
  return proto.resource.AddAccountRoleRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AddAccountRoleRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AddAccountRoleRqst}
 */
proto.resource.AddAccountRoleRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setAccountid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setRoleid(value);
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
proto.resource.AddAccountRoleRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AddAccountRoleRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AddAccountRoleRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddAccountRoleRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAccountid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getRoleid();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string accountId = 1;
 * @return {string}
 */
proto.resource.AddAccountRoleRqst.prototype.getAccountid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.AddAccountRoleRqst} returns this
 */
proto.resource.AddAccountRoleRqst.prototype.setAccountid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string roleId = 2;
 * @return {string}
 */
proto.resource.AddAccountRoleRqst.prototype.getRoleid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.AddAccountRoleRqst} returns this
 */
proto.resource.AddAccountRoleRqst.prototype.setRoleid = function(value) {
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
proto.resource.AddAccountRoleRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AddAccountRoleRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AddAccountRoleRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddAccountRoleRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.AddAccountRoleRsp}
 */
proto.resource.AddAccountRoleRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AddAccountRoleRsp;
  return proto.resource.AddAccountRoleRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AddAccountRoleRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AddAccountRoleRsp}
 */
proto.resource.AddAccountRoleRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.AddAccountRoleRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AddAccountRoleRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AddAccountRoleRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddAccountRoleRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.AddAccountRoleRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.AddAccountRoleRsp} returns this
 */
proto.resource.AddAccountRoleRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.RemoveAccountRoleRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveAccountRoleRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveAccountRoleRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveAccountRoleRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    accountid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    roleid: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.RemoveAccountRoleRqst}
 */
proto.resource.RemoveAccountRoleRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveAccountRoleRqst;
  return proto.resource.RemoveAccountRoleRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveAccountRoleRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveAccountRoleRqst}
 */
proto.resource.RemoveAccountRoleRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setAccountid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setRoleid(value);
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
proto.resource.RemoveAccountRoleRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveAccountRoleRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveAccountRoleRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveAccountRoleRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAccountid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getRoleid();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string accountId = 1;
 * @return {string}
 */
proto.resource.RemoveAccountRoleRqst.prototype.getAccountid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemoveAccountRoleRqst} returns this
 */
proto.resource.RemoveAccountRoleRqst.prototype.setAccountid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string roleId = 2;
 * @return {string}
 */
proto.resource.RemoveAccountRoleRqst.prototype.getRoleid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemoveAccountRoleRqst} returns this
 */
proto.resource.RemoveAccountRoleRqst.prototype.setRoleid = function(value) {
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
proto.resource.RemoveAccountRoleRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveAccountRoleRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveAccountRoleRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveAccountRoleRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.RemoveAccountRoleRsp}
 */
proto.resource.RemoveAccountRoleRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveAccountRoleRsp;
  return proto.resource.RemoveAccountRoleRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveAccountRoleRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveAccountRoleRsp}
 */
proto.resource.RemoveAccountRoleRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.RemoveAccountRoleRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveAccountRoleRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveAccountRoleRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveAccountRoleRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.RemoveAccountRoleRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.RemoveAccountRoleRsp} returns this
 */
proto.resource.RemoveAccountRoleRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.Contact.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.Contact.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.Contact} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.Contact.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, ""),
    invitationtime: jspb.Message.getFieldWithDefault(msg, 2, 0),
    status: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.resource.Contact}
 */
proto.resource.Contact.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.Contact;
  return proto.resource.Contact.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.Contact} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.Contact}
 */
proto.resource.Contact.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {number} */ (reader.readInt64());
      msg.setInvitationtime(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setStatus(value);
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
proto.resource.Contact.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.Contact.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.Contact} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.Contact.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getInvitationtime();
  if (f !== 0) {
    writer.writeInt64(
      2,
      f
    );
  }
  f = message.getStatus();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.resource.Contact.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Contact} returns this
 */
proto.resource.Contact.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int64 invitationTime = 2;
 * @return {number}
 */
proto.resource.Contact.prototype.getInvitationtime = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.resource.Contact} returns this
 */
proto.resource.Contact.prototype.setInvitationtime = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string status = 3;
 * @return {string}
 */
proto.resource.Contact.prototype.getStatus = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Contact} returns this
 */
proto.resource.Contact.prototype.setStatus = function(value) {
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
proto.resource.SetAccountContactRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.SetAccountContactRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.SetAccountContactRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.SetAccountContactRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    accountid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    contact: (f = msg.getContact()) && proto.resource.Contact.toObject(includeInstance, f)
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
 * @return {!proto.resource.SetAccountContactRqst}
 */
proto.resource.SetAccountContactRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.SetAccountContactRqst;
  return proto.resource.SetAccountContactRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.SetAccountContactRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.SetAccountContactRqst}
 */
proto.resource.SetAccountContactRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setAccountid(value);
      break;
    case 2:
      var value = new proto.resource.Contact;
      reader.readMessage(value,proto.resource.Contact.deserializeBinaryFromReader);
      msg.setContact(value);
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
proto.resource.SetAccountContactRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.SetAccountContactRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.SetAccountContactRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.SetAccountContactRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAccountid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getContact();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.resource.Contact.serializeBinaryToWriter
    );
  }
};


/**
 * optional string accountId = 1;
 * @return {string}
 */
proto.resource.SetAccountContactRqst.prototype.getAccountid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.SetAccountContactRqst} returns this
 */
proto.resource.SetAccountContactRqst.prototype.setAccountid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional Contact contact = 2;
 * @return {?proto.resource.Contact}
 */
proto.resource.SetAccountContactRqst.prototype.getContact = function() {
  return /** @type{?proto.resource.Contact} */ (
    jspb.Message.getWrapperField(this, proto.resource.Contact, 2));
};


/**
 * @param {?proto.resource.Contact|undefined} value
 * @return {!proto.resource.SetAccountContactRqst} returns this
*/
proto.resource.SetAccountContactRqst.prototype.setContact = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.resource.SetAccountContactRqst} returns this
 */
proto.resource.SetAccountContactRqst.prototype.clearContact = function() {
  return this.setContact(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.resource.SetAccountContactRqst.prototype.hasContact = function() {
  return jspb.Message.getField(this, 2) != null;
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
proto.resource.SetAccountContactRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.SetAccountContactRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.SetAccountContactRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.SetAccountContactRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.SetAccountContactRsp}
 */
proto.resource.SetAccountContactRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.SetAccountContactRsp;
  return proto.resource.SetAccountContactRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.SetAccountContactRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.SetAccountContactRsp}
 */
proto.resource.SetAccountContactRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.SetAccountContactRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.SetAccountContactRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.SetAccountContactRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.SetAccountContactRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.SetAccountContactRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.SetAccountContactRsp} returns this
 */
proto.resource.SetAccountContactRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.CreateRoleRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.CreateRoleRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.CreateRoleRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.CreateRoleRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    role: (f = msg.getRole()) && proto.resource.Role.toObject(includeInstance, f)
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
 * @return {!proto.resource.CreateRoleRqst}
 */
proto.resource.CreateRoleRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.CreateRoleRqst;
  return proto.resource.CreateRoleRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.CreateRoleRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.CreateRoleRqst}
 */
proto.resource.CreateRoleRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Role;
      reader.readMessage(value,proto.resource.Role.deserializeBinaryFromReader);
      msg.setRole(value);
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
proto.resource.CreateRoleRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.CreateRoleRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.CreateRoleRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.CreateRoleRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRole();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.resource.Role.serializeBinaryToWriter
    );
  }
};


/**
 * optional Role role = 1;
 * @return {?proto.resource.Role}
 */
proto.resource.CreateRoleRqst.prototype.getRole = function() {
  return /** @type{?proto.resource.Role} */ (
    jspb.Message.getWrapperField(this, proto.resource.Role, 1));
};


/**
 * @param {?proto.resource.Role|undefined} value
 * @return {!proto.resource.CreateRoleRqst} returns this
*/
proto.resource.CreateRoleRqst.prototype.setRole = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.resource.CreateRoleRqst} returns this
 */
proto.resource.CreateRoleRqst.prototype.clearRole = function() {
  return this.setRole(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.resource.CreateRoleRqst.prototype.hasRole = function() {
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
proto.resource.CreateRoleRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.CreateRoleRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.CreateRoleRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.CreateRoleRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.CreateRoleRsp}
 */
proto.resource.CreateRoleRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.CreateRoleRsp;
  return proto.resource.CreateRoleRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.CreateRoleRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.CreateRoleRsp}
 */
proto.resource.CreateRoleRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.CreateRoleRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.CreateRoleRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.CreateRoleRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.CreateRoleRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.CreateRoleRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.CreateRoleRsp} returns this
 */
proto.resource.CreateRoleRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.GetRolesRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetRolesRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetRolesRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetRolesRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    query: jspb.Message.getFieldWithDefault(msg, 1, ""),
    options: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.GetRolesRqst}
 */
proto.resource.GetRolesRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetRolesRqst;
  return proto.resource.GetRolesRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetRolesRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetRolesRqst}
 */
proto.resource.GetRolesRqst.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setOptions(value);
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
proto.resource.GetRolesRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetRolesRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetRolesRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetRolesRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getQuery();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOptions();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string query = 1;
 * @return {string}
 */
proto.resource.GetRolesRqst.prototype.getQuery = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetRolesRqst} returns this
 */
proto.resource.GetRolesRqst.prototype.setQuery = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string options = 2;
 * @return {string}
 */
proto.resource.GetRolesRqst.prototype.getOptions = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetRolesRqst} returns this
 */
proto.resource.GetRolesRqst.prototype.setOptions = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.GetRolesRsp.repeatedFields_ = [1];



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
proto.resource.GetRolesRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetRolesRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetRolesRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetRolesRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    rolesList: jspb.Message.toObjectList(msg.getRolesList(),
    proto.resource.Role.toObject, includeInstance)
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
 * @return {!proto.resource.GetRolesRsp}
 */
proto.resource.GetRolesRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetRolesRsp;
  return proto.resource.GetRolesRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetRolesRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetRolesRsp}
 */
proto.resource.GetRolesRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Role;
      reader.readMessage(value,proto.resource.Role.deserializeBinaryFromReader);
      msg.addRoles(value);
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
proto.resource.GetRolesRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetRolesRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetRolesRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetRolesRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRolesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.resource.Role.serializeBinaryToWriter
    );
  }
};


/**
 * repeated Role roles = 1;
 * @return {!Array<!proto.resource.Role>}
 */
proto.resource.GetRolesRsp.prototype.getRolesList = function() {
  return /** @type{!Array<!proto.resource.Role>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.resource.Role, 1));
};


/**
 * @param {!Array<!proto.resource.Role>} value
 * @return {!proto.resource.GetRolesRsp} returns this
*/
proto.resource.GetRolesRsp.prototype.setRolesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.resource.Role=} opt_value
 * @param {number=} opt_index
 * @return {!proto.resource.Role}
 */
proto.resource.GetRolesRsp.prototype.addRoles = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.resource.Role, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.GetRolesRsp} returns this
 */
proto.resource.GetRolesRsp.prototype.clearRolesList = function() {
  return this.setRolesList([]);
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
proto.resource.DeleteRoleRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.DeleteRoleRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.DeleteRoleRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteRoleRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    roleid: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.resource.DeleteRoleRqst}
 */
proto.resource.DeleteRoleRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.DeleteRoleRqst;
  return proto.resource.DeleteRoleRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.DeleteRoleRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.DeleteRoleRqst}
 */
proto.resource.DeleteRoleRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRoleid(value);
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
proto.resource.DeleteRoleRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.DeleteRoleRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.DeleteRoleRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteRoleRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRoleid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string roleId = 1;
 * @return {string}
 */
proto.resource.DeleteRoleRqst.prototype.getRoleid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.DeleteRoleRqst} returns this
 */
proto.resource.DeleteRoleRqst.prototype.setRoleid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
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
proto.resource.DeleteRoleRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.DeleteRoleRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.DeleteRoleRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteRoleRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.DeleteRoleRsp}
 */
proto.resource.DeleteRoleRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.DeleteRoleRsp;
  return proto.resource.DeleteRoleRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.DeleteRoleRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.DeleteRoleRsp}
 */
proto.resource.DeleteRoleRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.DeleteRoleRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.DeleteRoleRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.DeleteRoleRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteRoleRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.DeleteRoleRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.DeleteRoleRsp} returns this
 */
proto.resource.DeleteRoleRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.Application.repeatedFields_ = [8,9];



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
proto.resource.Application.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.Application.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.Application} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.Application.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, ""),
    name: jspb.Message.getFieldWithDefault(msg, 2, ""),
    domain: jspb.Message.getFieldWithDefault(msg, 3, ""),
    password: jspb.Message.getFieldWithDefault(msg, 4, ""),
    path: jspb.Message.getFieldWithDefault(msg, 5, ""),
    version: jspb.Message.getFieldWithDefault(msg, 6, ""),
    description: jspb.Message.getFieldWithDefault(msg, 7, ""),
    actionsList: (f = jspb.Message.getRepeatedField(msg, 8)) == null ? undefined : f,
    keywordsList: (f = jspb.Message.getRepeatedField(msg, 9)) == null ? undefined : f,
    icon: jspb.Message.getFieldWithDefault(msg, 10, ""),
    alias: jspb.Message.getFieldWithDefault(msg, 11, ""),
    publisherid: jspb.Message.getFieldWithDefault(msg, 12, ""),
    creationDate: jspb.Message.getFieldWithDefault(msg, 13, 0),
    lastDeployed: jspb.Message.getFieldWithDefault(msg, 14, 0)
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
 * @return {!proto.resource.Application}
 */
proto.resource.Application.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.Application;
  return proto.resource.Application.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.Application} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.Application}
 */
proto.resource.Application.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setName(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setDomain(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setPassword(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setPath(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setVersion(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setDescription(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.addActions(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.addKeywords(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.setIcon(value);
      break;
    case 11:
      var value = /** @type {string} */ (reader.readString());
      msg.setAlias(value);
      break;
    case 12:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublisherid(value);
      break;
    case 13:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setCreationDate(value);
      break;
    case 14:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setLastDeployed(value);
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
proto.resource.Application.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.Application.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.Application} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.Application.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getDomain();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getPassword();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getPath();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getVersion();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getDescription();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getActionsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      8,
      f
    );
  }
  f = message.getKeywordsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      9,
      f
    );
  }
  f = message.getIcon();
  if (f.length > 0) {
    writer.writeString(
      10,
      f
    );
  }
  f = message.getAlias();
  if (f.length > 0) {
    writer.writeString(
      11,
      f
    );
  }
  f = message.getPublisherid();
  if (f.length > 0) {
    writer.writeString(
      12,
      f
    );
  }
  f = message.getCreationDate();
  if (f !== 0) {
    writer.writeInt64(
      13,
      f
    );
  }
  f = message.getLastDeployed();
  if (f !== 0) {
    writer.writeInt64(
      14,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.resource.Application.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Application} returns this
 */
proto.resource.Application.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string name = 2;
 * @return {string}
 */
proto.resource.Application.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Application} returns this
 */
proto.resource.Application.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string domain = 3;
 * @return {string}
 */
proto.resource.Application.prototype.getDomain = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Application} returns this
 */
proto.resource.Application.prototype.setDomain = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string password = 4;
 * @return {string}
 */
proto.resource.Application.prototype.getPassword = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Application} returns this
 */
proto.resource.Application.prototype.setPassword = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string path = 5;
 * @return {string}
 */
proto.resource.Application.prototype.getPath = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Application} returns this
 */
proto.resource.Application.prototype.setPath = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string version = 6;
 * @return {string}
 */
proto.resource.Application.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Application} returns this
 */
proto.resource.Application.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string description = 7;
 * @return {string}
 */
proto.resource.Application.prototype.getDescription = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Application} returns this
 */
proto.resource.Application.prototype.setDescription = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * repeated string actions = 8;
 * @return {!Array<string>}
 */
proto.resource.Application.prototype.getActionsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 8));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.Application} returns this
 */
proto.resource.Application.prototype.setActionsList = function(value) {
  return jspb.Message.setField(this, 8, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.Application} returns this
 */
proto.resource.Application.prototype.addActions = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 8, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.Application} returns this
 */
proto.resource.Application.prototype.clearActionsList = function() {
  return this.setActionsList([]);
};


/**
 * repeated string keywords = 9;
 * @return {!Array<string>}
 */
proto.resource.Application.prototype.getKeywordsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 9));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.Application} returns this
 */
proto.resource.Application.prototype.setKeywordsList = function(value) {
  return jspb.Message.setField(this, 9, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.Application} returns this
 */
proto.resource.Application.prototype.addKeywords = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 9, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.Application} returns this
 */
proto.resource.Application.prototype.clearKeywordsList = function() {
  return this.setKeywordsList([]);
};


/**
 * optional string icon = 10;
 * @return {string}
 */
proto.resource.Application.prototype.getIcon = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 10, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Application} returns this
 */
proto.resource.Application.prototype.setIcon = function(value) {
  return jspb.Message.setProto3StringField(this, 10, value);
};


/**
 * optional string alias = 11;
 * @return {string}
 */
proto.resource.Application.prototype.getAlias = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 11, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Application} returns this
 */
proto.resource.Application.prototype.setAlias = function(value) {
  return jspb.Message.setProto3StringField(this, 11, value);
};


/**
 * optional string publisherid = 12;
 * @return {string}
 */
proto.resource.Application.prototype.getPublisherid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 12, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Application} returns this
 */
proto.resource.Application.prototype.setPublisherid = function(value) {
  return jspb.Message.setProto3StringField(this, 12, value);
};


/**
 * optional int64 creation_date = 13;
 * @return {number}
 */
proto.resource.Application.prototype.getCreationDate = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 13, 0));
};


/**
 * @param {number} value
 * @return {!proto.resource.Application} returns this
 */
proto.resource.Application.prototype.setCreationDate = function(value) {
  return jspb.Message.setProto3IntField(this, 13, value);
};


/**
 * optional int64 last_deployed = 14;
 * @return {number}
 */
proto.resource.Application.prototype.getLastDeployed = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 14, 0));
};


/**
 * @param {number} value
 * @return {!proto.resource.Application} returns this
 */
proto.resource.Application.prototype.setLastDeployed = function(value) {
  return jspb.Message.setProto3IntField(this, 14, value);
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
proto.resource.CreateApplicationRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.CreateApplicationRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.CreateApplicationRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.CreateApplicationRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    application: (f = msg.getApplication()) && proto.resource.Application.toObject(includeInstance, f)
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
 * @return {!proto.resource.CreateApplicationRqst}
 */
proto.resource.CreateApplicationRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.CreateApplicationRqst;
  return proto.resource.CreateApplicationRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.CreateApplicationRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.CreateApplicationRqst}
 */
proto.resource.CreateApplicationRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Application;
      reader.readMessage(value,proto.resource.Application.deserializeBinaryFromReader);
      msg.setApplication(value);
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
proto.resource.CreateApplicationRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.CreateApplicationRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.CreateApplicationRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.CreateApplicationRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getApplication();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.resource.Application.serializeBinaryToWriter
    );
  }
};


/**
 * optional Application application = 1;
 * @return {?proto.resource.Application}
 */
proto.resource.CreateApplicationRqst.prototype.getApplication = function() {
  return /** @type{?proto.resource.Application} */ (
    jspb.Message.getWrapperField(this, proto.resource.Application, 1));
};


/**
 * @param {?proto.resource.Application|undefined} value
 * @return {!proto.resource.CreateApplicationRqst} returns this
*/
proto.resource.CreateApplicationRqst.prototype.setApplication = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.resource.CreateApplicationRqst} returns this
 */
proto.resource.CreateApplicationRqst.prototype.clearApplication = function() {
  return this.setApplication(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.resource.CreateApplicationRqst.prototype.hasApplication = function() {
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
proto.resource.CreateApplicationRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.CreateApplicationRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.CreateApplicationRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.CreateApplicationRsp.toObject = function(includeInstance, msg) {
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
 * @return {!proto.resource.CreateApplicationRsp}
 */
proto.resource.CreateApplicationRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.CreateApplicationRsp;
  return proto.resource.CreateApplicationRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.CreateApplicationRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.CreateApplicationRsp}
 */
proto.resource.CreateApplicationRsp.deserializeBinaryFromReader = function(msg, reader) {
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
proto.resource.CreateApplicationRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.CreateApplicationRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.CreateApplicationRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.CreateApplicationRsp.serializeBinaryToWriter = function(message, writer) {
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
proto.resource.UpdateApplicationRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.UpdateApplicationRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.UpdateApplicationRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.UpdateApplicationRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    applicationid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    values: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.UpdateApplicationRqst}
 */
proto.resource.UpdateApplicationRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.UpdateApplicationRqst;
  return proto.resource.UpdateApplicationRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.UpdateApplicationRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.UpdateApplicationRqst}
 */
proto.resource.UpdateApplicationRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setApplicationid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setValues(value);
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
proto.resource.UpdateApplicationRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.UpdateApplicationRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.UpdateApplicationRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.UpdateApplicationRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getApplicationid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getValues();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string applicationId = 1;
 * @return {string}
 */
proto.resource.UpdateApplicationRqst.prototype.getApplicationid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.UpdateApplicationRqst} returns this
 */
proto.resource.UpdateApplicationRqst.prototype.setApplicationid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string values = 2;
 * @return {string}
 */
proto.resource.UpdateApplicationRqst.prototype.getValues = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.UpdateApplicationRqst} returns this
 */
proto.resource.UpdateApplicationRqst.prototype.setValues = function(value) {
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
proto.resource.UpdateApplicationRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.UpdateApplicationRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.UpdateApplicationRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.UpdateApplicationRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.UpdateApplicationRsp}
 */
proto.resource.UpdateApplicationRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.UpdateApplicationRsp;
  return proto.resource.UpdateApplicationRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.UpdateApplicationRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.UpdateApplicationRsp}
 */
proto.resource.UpdateApplicationRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.UpdateApplicationRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.UpdateApplicationRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.UpdateApplicationRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.UpdateApplicationRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.UpdateApplicationRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.UpdateApplicationRsp} returns this
 */
proto.resource.UpdateApplicationRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.DeleteApplicationRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.DeleteApplicationRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.DeleteApplicationRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteApplicationRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    applicationid: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.resource.DeleteApplicationRqst}
 */
proto.resource.DeleteApplicationRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.DeleteApplicationRqst;
  return proto.resource.DeleteApplicationRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.DeleteApplicationRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.DeleteApplicationRqst}
 */
proto.resource.DeleteApplicationRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setApplicationid(value);
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
proto.resource.DeleteApplicationRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.DeleteApplicationRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.DeleteApplicationRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteApplicationRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getApplicationid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string applicationId = 1;
 * @return {string}
 */
proto.resource.DeleteApplicationRqst.prototype.getApplicationid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.DeleteApplicationRqst} returns this
 */
proto.resource.DeleteApplicationRqst.prototype.setApplicationid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
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
proto.resource.DeleteApplicationRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.DeleteApplicationRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.DeleteApplicationRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteApplicationRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.DeleteApplicationRsp}
 */
proto.resource.DeleteApplicationRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.DeleteApplicationRsp;
  return proto.resource.DeleteApplicationRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.DeleteApplicationRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.DeleteApplicationRsp}
 */
proto.resource.DeleteApplicationRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.DeleteApplicationRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.DeleteApplicationRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.DeleteApplicationRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteApplicationRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.DeleteApplicationRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.DeleteApplicationRsp} returns this
 */
proto.resource.DeleteApplicationRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.AddApplicationActionsRqst.repeatedFields_ = [2];



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
proto.resource.AddApplicationActionsRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AddApplicationActionsRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AddApplicationActionsRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddApplicationActionsRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    applicationid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    actionsList: (f = jspb.Message.getRepeatedField(msg, 2)) == null ? undefined : f
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
 * @return {!proto.resource.AddApplicationActionsRqst}
 */
proto.resource.AddApplicationActionsRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AddApplicationActionsRqst;
  return proto.resource.AddApplicationActionsRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AddApplicationActionsRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AddApplicationActionsRqst}
 */
proto.resource.AddApplicationActionsRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setApplicationid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.addActions(value);
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
proto.resource.AddApplicationActionsRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AddApplicationActionsRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AddApplicationActionsRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddApplicationActionsRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getApplicationid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getActionsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      2,
      f
    );
  }
};


/**
 * optional string applicationId = 1;
 * @return {string}
 */
proto.resource.AddApplicationActionsRqst.prototype.getApplicationid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.AddApplicationActionsRqst} returns this
 */
proto.resource.AddApplicationActionsRqst.prototype.setApplicationid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * repeated string actions = 2;
 * @return {!Array<string>}
 */
proto.resource.AddApplicationActionsRqst.prototype.getActionsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.AddApplicationActionsRqst} returns this
 */
proto.resource.AddApplicationActionsRqst.prototype.setActionsList = function(value) {
  return jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.AddApplicationActionsRqst} returns this
 */
proto.resource.AddApplicationActionsRqst.prototype.addActions = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.AddApplicationActionsRqst} returns this
 */
proto.resource.AddApplicationActionsRqst.prototype.clearActionsList = function() {
  return this.setActionsList([]);
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
proto.resource.AddApplicationActionsRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AddApplicationActionsRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AddApplicationActionsRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddApplicationActionsRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.AddApplicationActionsRsp}
 */
proto.resource.AddApplicationActionsRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AddApplicationActionsRsp;
  return proto.resource.AddApplicationActionsRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AddApplicationActionsRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AddApplicationActionsRsp}
 */
proto.resource.AddApplicationActionsRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.AddApplicationActionsRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AddApplicationActionsRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AddApplicationActionsRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddApplicationActionsRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.AddApplicationActionsRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.AddApplicationActionsRsp} returns this
 */
proto.resource.AddApplicationActionsRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.RemoveApplicationActionRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveApplicationActionRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveApplicationActionRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveApplicationActionRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    applicationid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    action: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.RemoveApplicationActionRqst}
 */
proto.resource.RemoveApplicationActionRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveApplicationActionRqst;
  return proto.resource.RemoveApplicationActionRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveApplicationActionRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveApplicationActionRqst}
 */
proto.resource.RemoveApplicationActionRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setApplicationid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setAction(value);
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
proto.resource.RemoveApplicationActionRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveApplicationActionRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveApplicationActionRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveApplicationActionRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getApplicationid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getAction();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string applicationId = 1;
 * @return {string}
 */
proto.resource.RemoveApplicationActionRqst.prototype.getApplicationid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemoveApplicationActionRqst} returns this
 */
proto.resource.RemoveApplicationActionRqst.prototype.setApplicationid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string action = 2;
 * @return {string}
 */
proto.resource.RemoveApplicationActionRqst.prototype.getAction = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemoveApplicationActionRqst} returns this
 */
proto.resource.RemoveApplicationActionRqst.prototype.setAction = function(value) {
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
proto.resource.RemoveApplicationActionRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveApplicationActionRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveApplicationActionRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveApplicationActionRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.RemoveApplicationActionRsp}
 */
proto.resource.RemoveApplicationActionRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveApplicationActionRsp;
  return proto.resource.RemoveApplicationActionRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveApplicationActionRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveApplicationActionRsp}
 */
proto.resource.RemoveApplicationActionRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.RemoveApplicationActionRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveApplicationActionRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveApplicationActionRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveApplicationActionRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.RemoveApplicationActionRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.RemoveApplicationActionRsp} returns this
 */
proto.resource.RemoveApplicationActionRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.RemoveApplicationsActionRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveApplicationsActionRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveApplicationsActionRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveApplicationsActionRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    action: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.RemoveApplicationsActionRqst}
 */
proto.resource.RemoveApplicationsActionRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveApplicationsActionRqst;
  return proto.resource.RemoveApplicationsActionRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveApplicationsActionRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveApplicationsActionRqst}
 */
proto.resource.RemoveApplicationsActionRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setAction(value);
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
proto.resource.RemoveApplicationsActionRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveApplicationsActionRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveApplicationsActionRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveApplicationsActionRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAction();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string action = 2;
 * @return {string}
 */
proto.resource.RemoveApplicationsActionRqst.prototype.getAction = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemoveApplicationsActionRqst} returns this
 */
proto.resource.RemoveApplicationsActionRqst.prototype.setAction = function(value) {
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
proto.resource.RemoveApplicationsActionRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveApplicationsActionRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveApplicationsActionRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveApplicationsActionRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.RemoveApplicationsActionRsp}
 */
proto.resource.RemoveApplicationsActionRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveApplicationsActionRsp;
  return proto.resource.RemoveApplicationsActionRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveApplicationsActionRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveApplicationsActionRsp}
 */
proto.resource.RemoveApplicationsActionRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.RemoveApplicationsActionRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveApplicationsActionRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveApplicationsActionRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveApplicationsActionRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.RemoveApplicationsActionRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.RemoveApplicationsActionRsp} returns this
 */
proto.resource.RemoveApplicationsActionRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.GetApplicationsRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetApplicationsRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetApplicationsRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetApplicationsRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    query: jspb.Message.getFieldWithDefault(msg, 1, ""),
    options: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.GetApplicationsRqst}
 */
proto.resource.GetApplicationsRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetApplicationsRqst;
  return proto.resource.GetApplicationsRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetApplicationsRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetApplicationsRqst}
 */
proto.resource.GetApplicationsRqst.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setOptions(value);
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
proto.resource.GetApplicationsRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetApplicationsRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetApplicationsRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetApplicationsRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getQuery();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOptions();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string query = 1;
 * @return {string}
 */
proto.resource.GetApplicationsRqst.prototype.getQuery = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetApplicationsRqst} returns this
 */
proto.resource.GetApplicationsRqst.prototype.setQuery = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string options = 2;
 * @return {string}
 */
proto.resource.GetApplicationsRqst.prototype.getOptions = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetApplicationsRqst} returns this
 */
proto.resource.GetApplicationsRqst.prototype.setOptions = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.GetApplicationsRsp.repeatedFields_ = [1];



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
proto.resource.GetApplicationsRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetApplicationsRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetApplicationsRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetApplicationsRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    applicationsList: jspb.Message.toObjectList(msg.getApplicationsList(),
    proto.resource.Application.toObject, includeInstance)
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
 * @return {!proto.resource.GetApplicationsRsp}
 */
proto.resource.GetApplicationsRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetApplicationsRsp;
  return proto.resource.GetApplicationsRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetApplicationsRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetApplicationsRsp}
 */
proto.resource.GetApplicationsRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Application;
      reader.readMessage(value,proto.resource.Application.deserializeBinaryFromReader);
      msg.addApplications(value);
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
proto.resource.GetApplicationsRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetApplicationsRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetApplicationsRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetApplicationsRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getApplicationsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.resource.Application.serializeBinaryToWriter
    );
  }
};


/**
 * repeated Application applications = 1;
 * @return {!Array<!proto.resource.Application>}
 */
proto.resource.GetApplicationsRsp.prototype.getApplicationsList = function() {
  return /** @type{!Array<!proto.resource.Application>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.resource.Application, 1));
};


/**
 * @param {!Array<!proto.resource.Application>} value
 * @return {!proto.resource.GetApplicationsRsp} returns this
*/
proto.resource.GetApplicationsRsp.prototype.setApplicationsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.resource.Application=} opt_value
 * @param {number=} opt_index
 * @return {!proto.resource.Application}
 */
proto.resource.GetApplicationsRsp.prototype.addApplications = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.resource.Application, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.GetApplicationsRsp} returns this
 */
proto.resource.GetApplicationsRsp.prototype.clearApplicationsList = function() {
  return this.setApplicationsList([]);
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
proto.resource.GetApplicationVersionRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetApplicationVersionRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetApplicationVersionRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetApplicationVersionRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.resource.GetApplicationVersionRqst}
 */
proto.resource.GetApplicationVersionRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetApplicationVersionRqst;
  return proto.resource.GetApplicationVersionRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetApplicationVersionRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetApplicationVersionRqst}
 */
proto.resource.GetApplicationVersionRqst.deserializeBinaryFromReader = function(msg, reader) {
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
proto.resource.GetApplicationVersionRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetApplicationVersionRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetApplicationVersionRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetApplicationVersionRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.resource.GetApplicationVersionRqst.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetApplicationVersionRqst} returns this
 */
proto.resource.GetApplicationVersionRqst.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
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
proto.resource.GetApplicationVersionRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetApplicationVersionRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetApplicationVersionRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetApplicationVersionRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    version: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.resource.GetApplicationVersionRsp}
 */
proto.resource.GetApplicationVersionRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetApplicationVersionRsp;
  return proto.resource.GetApplicationVersionRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetApplicationVersionRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetApplicationVersionRsp}
 */
proto.resource.GetApplicationVersionRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setVersion(value);
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
proto.resource.GetApplicationVersionRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetApplicationVersionRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetApplicationVersionRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetApplicationVersionRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getVersion();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string version = 1;
 * @return {string}
 */
proto.resource.GetApplicationVersionRsp.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetApplicationVersionRsp} returns this
 */
proto.resource.GetApplicationVersionRsp.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
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
proto.resource.GetApplicationIconRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetApplicationIconRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetApplicationIconRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetApplicationIconRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.resource.GetApplicationIconRqst}
 */
proto.resource.GetApplicationIconRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetApplicationIconRqst;
  return proto.resource.GetApplicationIconRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetApplicationIconRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetApplicationIconRqst}
 */
proto.resource.GetApplicationIconRqst.deserializeBinaryFromReader = function(msg, reader) {
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
proto.resource.GetApplicationIconRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetApplicationIconRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetApplicationIconRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetApplicationIconRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.resource.GetApplicationIconRqst.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetApplicationIconRqst} returns this
 */
proto.resource.GetApplicationIconRqst.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
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
proto.resource.GetApplicationIconRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetApplicationIconRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetApplicationIconRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetApplicationIconRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    icon: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.resource.GetApplicationIconRsp}
 */
proto.resource.GetApplicationIconRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetApplicationIconRsp;
  return proto.resource.GetApplicationIconRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetApplicationIconRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetApplicationIconRsp}
 */
proto.resource.GetApplicationIconRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setIcon(value);
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
proto.resource.GetApplicationIconRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetApplicationIconRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetApplicationIconRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetApplicationIconRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getIcon();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string icon = 1;
 * @return {string}
 */
proto.resource.GetApplicationIconRsp.prototype.getIcon = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetApplicationIconRsp} returns this
 */
proto.resource.GetApplicationIconRsp.prototype.setIcon = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
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
proto.resource.GetApplicationAliasRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetApplicationAliasRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetApplicationAliasRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetApplicationAliasRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.resource.GetApplicationAliasRqst}
 */
proto.resource.GetApplicationAliasRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetApplicationAliasRqst;
  return proto.resource.GetApplicationAliasRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetApplicationAliasRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetApplicationAliasRqst}
 */
proto.resource.GetApplicationAliasRqst.deserializeBinaryFromReader = function(msg, reader) {
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
proto.resource.GetApplicationAliasRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetApplicationAliasRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetApplicationAliasRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetApplicationAliasRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.resource.GetApplicationAliasRqst.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetApplicationAliasRqst} returns this
 */
proto.resource.GetApplicationAliasRqst.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
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
proto.resource.GetApplicationAliasRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetApplicationAliasRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetApplicationAliasRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetApplicationAliasRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    alias: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.resource.GetApplicationAliasRsp}
 */
proto.resource.GetApplicationAliasRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetApplicationAliasRsp;
  return proto.resource.GetApplicationAliasRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetApplicationAliasRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetApplicationAliasRsp}
 */
proto.resource.GetApplicationAliasRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setAlias(value);
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
proto.resource.GetApplicationAliasRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetApplicationAliasRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetApplicationAliasRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetApplicationAliasRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAlias();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string alias = 1;
 * @return {string}
 */
proto.resource.GetApplicationAliasRsp.prototype.getAlias = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetApplicationAliasRsp} returns this
 */
proto.resource.GetApplicationAliasRsp.prototype.setAlias = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.Group.repeatedFields_ = [5,6];



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
proto.resource.Group.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.Group.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.Group} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.Group.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, ""),
    name: jspb.Message.getFieldWithDefault(msg, 2, ""),
    domain: jspb.Message.getFieldWithDefault(msg, 3, ""),
    description: jspb.Message.getFieldWithDefault(msg, 4, ""),
    membersList: (f = jspb.Message.getRepeatedField(msg, 5)) == null ? undefined : f,
    organizationsList: (f = jspb.Message.getRepeatedField(msg, 6)) == null ? undefined : f
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
 * @return {!proto.resource.Group}
 */
proto.resource.Group.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.Group;
  return proto.resource.Group.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.Group} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.Group}
 */
proto.resource.Group.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setName(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setDomain(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setDescription(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.addMembers(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.addOrganizations(value);
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
proto.resource.Group.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.Group.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.Group} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.Group.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getDomain();
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
  f = message.getMembersList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      5,
      f
    );
  }
  f = message.getOrganizationsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      6,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.resource.Group.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Group} returns this
 */
proto.resource.Group.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string name = 2;
 * @return {string}
 */
proto.resource.Group.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Group} returns this
 */
proto.resource.Group.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string domain = 3;
 * @return {string}
 */
proto.resource.Group.prototype.getDomain = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Group} returns this
 */
proto.resource.Group.prototype.setDomain = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string description = 4;
 * @return {string}
 */
proto.resource.Group.prototype.getDescription = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Group} returns this
 */
proto.resource.Group.prototype.setDescription = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * repeated string members = 5;
 * @return {!Array<string>}
 */
proto.resource.Group.prototype.getMembersList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 5));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.Group} returns this
 */
proto.resource.Group.prototype.setMembersList = function(value) {
  return jspb.Message.setField(this, 5, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.Group} returns this
 */
proto.resource.Group.prototype.addMembers = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 5, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.Group} returns this
 */
proto.resource.Group.prototype.clearMembersList = function() {
  return this.setMembersList([]);
};


/**
 * repeated string organizations = 6;
 * @return {!Array<string>}
 */
proto.resource.Group.prototype.getOrganizationsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 6));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.Group} returns this
 */
proto.resource.Group.prototype.setOrganizationsList = function(value) {
  return jspb.Message.setField(this, 6, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.Group} returns this
 */
proto.resource.Group.prototype.addOrganizations = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 6, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.Group} returns this
 */
proto.resource.Group.prototype.clearOrganizationsList = function() {
  return this.setOrganizationsList([]);
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
proto.resource.CreateGroupRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.CreateGroupRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.CreateGroupRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.CreateGroupRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    group: (f = msg.getGroup()) && proto.resource.Group.toObject(includeInstance, f)
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
 * @return {!proto.resource.CreateGroupRqst}
 */
proto.resource.CreateGroupRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.CreateGroupRqst;
  return proto.resource.CreateGroupRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.CreateGroupRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.CreateGroupRqst}
 */
proto.resource.CreateGroupRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Group;
      reader.readMessage(value,proto.resource.Group.deserializeBinaryFromReader);
      msg.setGroup(value);
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
proto.resource.CreateGroupRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.CreateGroupRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.CreateGroupRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.CreateGroupRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getGroup();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.resource.Group.serializeBinaryToWriter
    );
  }
};


/**
 * optional Group group = 1;
 * @return {?proto.resource.Group}
 */
proto.resource.CreateGroupRqst.prototype.getGroup = function() {
  return /** @type{?proto.resource.Group} */ (
    jspb.Message.getWrapperField(this, proto.resource.Group, 1));
};


/**
 * @param {?proto.resource.Group|undefined} value
 * @return {!proto.resource.CreateGroupRqst} returns this
*/
proto.resource.CreateGroupRqst.prototype.setGroup = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.resource.CreateGroupRqst} returns this
 */
proto.resource.CreateGroupRqst.prototype.clearGroup = function() {
  return this.setGroup(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.resource.CreateGroupRqst.prototype.hasGroup = function() {
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
proto.resource.CreateGroupRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.CreateGroupRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.CreateGroupRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.CreateGroupRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.CreateGroupRsp}
 */
proto.resource.CreateGroupRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.CreateGroupRsp;
  return proto.resource.CreateGroupRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.CreateGroupRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.CreateGroupRsp}
 */
proto.resource.CreateGroupRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.CreateGroupRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.CreateGroupRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.CreateGroupRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.CreateGroupRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.CreateGroupRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.CreateGroupRsp} returns this
 */
proto.resource.CreateGroupRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.UpdateGroupRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.UpdateGroupRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.UpdateGroupRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.UpdateGroupRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    groupid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    values: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.UpdateGroupRqst}
 */
proto.resource.UpdateGroupRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.UpdateGroupRqst;
  return proto.resource.UpdateGroupRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.UpdateGroupRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.UpdateGroupRqst}
 */
proto.resource.UpdateGroupRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setGroupid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setValues(value);
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
proto.resource.UpdateGroupRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.UpdateGroupRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.UpdateGroupRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.UpdateGroupRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getGroupid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getValues();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string groupId = 1;
 * @return {string}
 */
proto.resource.UpdateGroupRqst.prototype.getGroupid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.UpdateGroupRqst} returns this
 */
proto.resource.UpdateGroupRqst.prototype.setGroupid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string values = 2;
 * @return {string}
 */
proto.resource.UpdateGroupRqst.prototype.getValues = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.UpdateGroupRqst} returns this
 */
proto.resource.UpdateGroupRqst.prototype.setValues = function(value) {
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
proto.resource.UpdateGroupRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.UpdateGroupRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.UpdateGroupRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.UpdateGroupRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.UpdateGroupRsp}
 */
proto.resource.UpdateGroupRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.UpdateGroupRsp;
  return proto.resource.UpdateGroupRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.UpdateGroupRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.UpdateGroupRsp}
 */
proto.resource.UpdateGroupRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.UpdateGroupRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.UpdateGroupRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.UpdateGroupRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.UpdateGroupRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.UpdateGroupRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.UpdateGroupRsp} returns this
 */
proto.resource.UpdateGroupRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.GetGroupsRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetGroupsRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetGroupsRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetGroupsRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    query: jspb.Message.getFieldWithDefault(msg, 1, ""),
    options: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.GetGroupsRqst}
 */
proto.resource.GetGroupsRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetGroupsRqst;
  return proto.resource.GetGroupsRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetGroupsRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetGroupsRqst}
 */
proto.resource.GetGroupsRqst.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setOptions(value);
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
proto.resource.GetGroupsRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetGroupsRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetGroupsRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetGroupsRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getQuery();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOptions();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string query = 1;
 * @return {string}
 */
proto.resource.GetGroupsRqst.prototype.getQuery = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetGroupsRqst} returns this
 */
proto.resource.GetGroupsRqst.prototype.setQuery = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string options = 2;
 * @return {string}
 */
proto.resource.GetGroupsRqst.prototype.getOptions = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetGroupsRqst} returns this
 */
proto.resource.GetGroupsRqst.prototype.setOptions = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.GetGroupsRsp.repeatedFields_ = [1];



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
proto.resource.GetGroupsRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetGroupsRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetGroupsRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetGroupsRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    groupsList: jspb.Message.toObjectList(msg.getGroupsList(),
    proto.resource.Group.toObject, includeInstance)
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
 * @return {!proto.resource.GetGroupsRsp}
 */
proto.resource.GetGroupsRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetGroupsRsp;
  return proto.resource.GetGroupsRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetGroupsRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetGroupsRsp}
 */
proto.resource.GetGroupsRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Group;
      reader.readMessage(value,proto.resource.Group.deserializeBinaryFromReader);
      msg.addGroups(value);
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
proto.resource.GetGroupsRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetGroupsRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetGroupsRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetGroupsRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getGroupsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.resource.Group.serializeBinaryToWriter
    );
  }
};


/**
 * repeated Group groups = 1;
 * @return {!Array<!proto.resource.Group>}
 */
proto.resource.GetGroupsRsp.prototype.getGroupsList = function() {
  return /** @type{!Array<!proto.resource.Group>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.resource.Group, 1));
};


/**
 * @param {!Array<!proto.resource.Group>} value
 * @return {!proto.resource.GetGroupsRsp} returns this
*/
proto.resource.GetGroupsRsp.prototype.setGroupsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.resource.Group=} opt_value
 * @param {number=} opt_index
 * @return {!proto.resource.Group}
 */
proto.resource.GetGroupsRsp.prototype.addGroups = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.resource.Group, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.GetGroupsRsp} returns this
 */
proto.resource.GetGroupsRsp.prototype.clearGroupsList = function() {
  return this.setGroupsList([]);
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
proto.resource.DeleteGroupRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.DeleteGroupRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.DeleteGroupRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteGroupRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    group: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.resource.DeleteGroupRqst}
 */
proto.resource.DeleteGroupRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.DeleteGroupRqst;
  return proto.resource.DeleteGroupRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.DeleteGroupRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.DeleteGroupRqst}
 */
proto.resource.DeleteGroupRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setGroup(value);
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
proto.resource.DeleteGroupRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.DeleteGroupRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.DeleteGroupRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteGroupRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getGroup();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string group = 1;
 * @return {string}
 */
proto.resource.DeleteGroupRqst.prototype.getGroup = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.DeleteGroupRqst} returns this
 */
proto.resource.DeleteGroupRqst.prototype.setGroup = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
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
proto.resource.DeleteGroupRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.DeleteGroupRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.DeleteGroupRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteGroupRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.DeleteGroupRsp}
 */
proto.resource.DeleteGroupRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.DeleteGroupRsp;
  return proto.resource.DeleteGroupRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.DeleteGroupRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.DeleteGroupRsp}
 */
proto.resource.DeleteGroupRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.DeleteGroupRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.DeleteGroupRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.DeleteGroupRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteGroupRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.DeleteGroupRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.DeleteGroupRsp} returns this
 */
proto.resource.DeleteGroupRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.AddGroupMemberAccountRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AddGroupMemberAccountRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AddGroupMemberAccountRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddGroupMemberAccountRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    groupid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    accountid: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.AddGroupMemberAccountRqst}
 */
proto.resource.AddGroupMemberAccountRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AddGroupMemberAccountRqst;
  return proto.resource.AddGroupMemberAccountRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AddGroupMemberAccountRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AddGroupMemberAccountRqst}
 */
proto.resource.AddGroupMemberAccountRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setGroupid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setAccountid(value);
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
proto.resource.AddGroupMemberAccountRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AddGroupMemberAccountRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AddGroupMemberAccountRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddGroupMemberAccountRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getGroupid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getAccountid();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string groupId = 1;
 * @return {string}
 */
proto.resource.AddGroupMemberAccountRqst.prototype.getGroupid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.AddGroupMemberAccountRqst} returns this
 */
proto.resource.AddGroupMemberAccountRqst.prototype.setGroupid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string accountId = 2;
 * @return {string}
 */
proto.resource.AddGroupMemberAccountRqst.prototype.getAccountid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.AddGroupMemberAccountRqst} returns this
 */
proto.resource.AddGroupMemberAccountRqst.prototype.setAccountid = function(value) {
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
proto.resource.AddGroupMemberAccountRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AddGroupMemberAccountRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AddGroupMemberAccountRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddGroupMemberAccountRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.AddGroupMemberAccountRsp}
 */
proto.resource.AddGroupMemberAccountRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AddGroupMemberAccountRsp;
  return proto.resource.AddGroupMemberAccountRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AddGroupMemberAccountRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AddGroupMemberAccountRsp}
 */
proto.resource.AddGroupMemberAccountRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.AddGroupMemberAccountRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AddGroupMemberAccountRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AddGroupMemberAccountRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddGroupMemberAccountRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.AddGroupMemberAccountRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.AddGroupMemberAccountRsp} returns this
 */
proto.resource.AddGroupMemberAccountRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.RemoveGroupMemberAccountRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveGroupMemberAccountRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveGroupMemberAccountRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveGroupMemberAccountRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    groupid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    accountid: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.RemoveGroupMemberAccountRqst}
 */
proto.resource.RemoveGroupMemberAccountRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveGroupMemberAccountRqst;
  return proto.resource.RemoveGroupMemberAccountRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveGroupMemberAccountRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveGroupMemberAccountRqst}
 */
proto.resource.RemoveGroupMemberAccountRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setGroupid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setAccountid(value);
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
proto.resource.RemoveGroupMemberAccountRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveGroupMemberAccountRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveGroupMemberAccountRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveGroupMemberAccountRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getGroupid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getAccountid();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string groupId = 1;
 * @return {string}
 */
proto.resource.RemoveGroupMemberAccountRqst.prototype.getGroupid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemoveGroupMemberAccountRqst} returns this
 */
proto.resource.RemoveGroupMemberAccountRqst.prototype.setGroupid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string accountId = 2;
 * @return {string}
 */
proto.resource.RemoveGroupMemberAccountRqst.prototype.getAccountid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemoveGroupMemberAccountRqst} returns this
 */
proto.resource.RemoveGroupMemberAccountRqst.prototype.setAccountid = function(value) {
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
proto.resource.RemoveGroupMemberAccountRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveGroupMemberAccountRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveGroupMemberAccountRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveGroupMemberAccountRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.RemoveGroupMemberAccountRsp}
 */
proto.resource.RemoveGroupMemberAccountRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveGroupMemberAccountRsp;
  return proto.resource.RemoveGroupMemberAccountRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveGroupMemberAccountRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveGroupMemberAccountRsp}
 */
proto.resource.RemoveGroupMemberAccountRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.RemoveGroupMemberAccountRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveGroupMemberAccountRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveGroupMemberAccountRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveGroupMemberAccountRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.RemoveGroupMemberAccountRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.RemoveGroupMemberAccountRsp} returns this
 */
proto.resource.RemoveGroupMemberAccountRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.Organization.repeatedFields_ = [7,8,9,10];



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
proto.resource.Organization.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.Organization.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.Organization} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.Organization.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, ""),
    domain: jspb.Message.getFieldWithDefault(msg, 2, ""),
    name: jspb.Message.getFieldWithDefault(msg, 3, ""),
    email: jspb.Message.getFieldWithDefault(msg, 4, ""),
    icon: jspb.Message.getFieldWithDefault(msg, 5, ""),
    description: jspb.Message.getFieldWithDefault(msg, 6, ""),
    accountsList: (f = jspb.Message.getRepeatedField(msg, 7)) == null ? undefined : f,
    groupsList: (f = jspb.Message.getRepeatedField(msg, 8)) == null ? undefined : f,
    rolesList: (f = jspb.Message.getRepeatedField(msg, 9)) == null ? undefined : f,
    applicationsList: (f = jspb.Message.getRepeatedField(msg, 10)) == null ? undefined : f
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
 * @return {!proto.resource.Organization}
 */
proto.resource.Organization.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.Organization;
  return proto.resource.Organization.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.Organization} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.Organization}
 */
proto.resource.Organization.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setDomain(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setEmail(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setIcon(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setDescription(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.addAccounts(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.addGroups(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.addRoles(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.addApplications(value);
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
proto.resource.Organization.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.Organization.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.Organization} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.Organization.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getDomain();
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
  f = message.getEmail();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getIcon();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getDescription();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getAccountsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      7,
      f
    );
  }
  f = message.getGroupsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      8,
      f
    );
  }
  f = message.getRolesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      9,
      f
    );
  }
  f = message.getApplicationsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      10,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.resource.Organization.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Organization} returns this
 */
proto.resource.Organization.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string domain = 2;
 * @return {string}
 */
proto.resource.Organization.prototype.getDomain = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Organization} returns this
 */
proto.resource.Organization.prototype.setDomain = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string name = 3;
 * @return {string}
 */
proto.resource.Organization.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Organization} returns this
 */
proto.resource.Organization.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string email = 4;
 * @return {string}
 */
proto.resource.Organization.prototype.getEmail = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Organization} returns this
 */
proto.resource.Organization.prototype.setEmail = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string icon = 5;
 * @return {string}
 */
proto.resource.Organization.prototype.getIcon = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Organization} returns this
 */
proto.resource.Organization.prototype.setIcon = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string description = 6;
 * @return {string}
 */
proto.resource.Organization.prototype.getDescription = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Organization} returns this
 */
proto.resource.Organization.prototype.setDescription = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * repeated string accounts = 7;
 * @return {!Array<string>}
 */
proto.resource.Organization.prototype.getAccountsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 7));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.Organization} returns this
 */
proto.resource.Organization.prototype.setAccountsList = function(value) {
  return jspb.Message.setField(this, 7, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.Organization} returns this
 */
proto.resource.Organization.prototype.addAccounts = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 7, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.Organization} returns this
 */
proto.resource.Organization.prototype.clearAccountsList = function() {
  return this.setAccountsList([]);
};


/**
 * repeated string groups = 8;
 * @return {!Array<string>}
 */
proto.resource.Organization.prototype.getGroupsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 8));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.Organization} returns this
 */
proto.resource.Organization.prototype.setGroupsList = function(value) {
  return jspb.Message.setField(this, 8, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.Organization} returns this
 */
proto.resource.Organization.prototype.addGroups = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 8, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.Organization} returns this
 */
proto.resource.Organization.prototype.clearGroupsList = function() {
  return this.setGroupsList([]);
};


/**
 * repeated string roles = 9;
 * @return {!Array<string>}
 */
proto.resource.Organization.prototype.getRolesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 9));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.Organization} returns this
 */
proto.resource.Organization.prototype.setRolesList = function(value) {
  return jspb.Message.setField(this, 9, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.Organization} returns this
 */
proto.resource.Organization.prototype.addRoles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 9, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.Organization} returns this
 */
proto.resource.Organization.prototype.clearRolesList = function() {
  return this.setRolesList([]);
};


/**
 * repeated string applications = 10;
 * @return {!Array<string>}
 */
proto.resource.Organization.prototype.getApplicationsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 10));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.Organization} returns this
 */
proto.resource.Organization.prototype.setApplicationsList = function(value) {
  return jspb.Message.setField(this, 10, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.Organization} returns this
 */
proto.resource.Organization.prototype.addApplications = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 10, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.Organization} returns this
 */
proto.resource.Organization.prototype.clearApplicationsList = function() {
  return this.setApplicationsList([]);
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
proto.resource.CreateOrganizationRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.CreateOrganizationRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.CreateOrganizationRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.CreateOrganizationRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    organization: (f = msg.getOrganization()) && proto.resource.Organization.toObject(includeInstance, f)
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
 * @return {!proto.resource.CreateOrganizationRqst}
 */
proto.resource.CreateOrganizationRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.CreateOrganizationRqst;
  return proto.resource.CreateOrganizationRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.CreateOrganizationRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.CreateOrganizationRqst}
 */
proto.resource.CreateOrganizationRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Organization;
      reader.readMessage(value,proto.resource.Organization.deserializeBinaryFromReader);
      msg.setOrganization(value);
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
proto.resource.CreateOrganizationRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.CreateOrganizationRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.CreateOrganizationRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.CreateOrganizationRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOrganization();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.resource.Organization.serializeBinaryToWriter
    );
  }
};


/**
 * optional Organization organization = 1;
 * @return {?proto.resource.Organization}
 */
proto.resource.CreateOrganizationRqst.prototype.getOrganization = function() {
  return /** @type{?proto.resource.Organization} */ (
    jspb.Message.getWrapperField(this, proto.resource.Organization, 1));
};


/**
 * @param {?proto.resource.Organization|undefined} value
 * @return {!proto.resource.CreateOrganizationRqst} returns this
*/
proto.resource.CreateOrganizationRqst.prototype.setOrganization = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.resource.CreateOrganizationRqst} returns this
 */
proto.resource.CreateOrganizationRqst.prototype.clearOrganization = function() {
  return this.setOrganization(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.resource.CreateOrganizationRqst.prototype.hasOrganization = function() {
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
proto.resource.CreateOrganizationRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.CreateOrganizationRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.CreateOrganizationRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.CreateOrganizationRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.CreateOrganizationRsp}
 */
proto.resource.CreateOrganizationRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.CreateOrganizationRsp;
  return proto.resource.CreateOrganizationRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.CreateOrganizationRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.CreateOrganizationRsp}
 */
proto.resource.CreateOrganizationRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.CreateOrganizationRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.CreateOrganizationRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.CreateOrganizationRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.CreateOrganizationRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.CreateOrganizationRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.CreateOrganizationRsp} returns this
 */
proto.resource.CreateOrganizationRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.UpdateOrganizationRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.UpdateOrganizationRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.UpdateOrganizationRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.UpdateOrganizationRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    organizationid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    values: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.UpdateOrganizationRqst}
 */
proto.resource.UpdateOrganizationRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.UpdateOrganizationRqst;
  return proto.resource.UpdateOrganizationRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.UpdateOrganizationRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.UpdateOrganizationRqst}
 */
proto.resource.UpdateOrganizationRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setOrganizationid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setValues(value);
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
proto.resource.UpdateOrganizationRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.UpdateOrganizationRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.UpdateOrganizationRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.UpdateOrganizationRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOrganizationid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getValues();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string organizationId = 1;
 * @return {string}
 */
proto.resource.UpdateOrganizationRqst.prototype.getOrganizationid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.UpdateOrganizationRqst} returns this
 */
proto.resource.UpdateOrganizationRqst.prototype.setOrganizationid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string values = 2;
 * @return {string}
 */
proto.resource.UpdateOrganizationRqst.prototype.getValues = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.UpdateOrganizationRqst} returns this
 */
proto.resource.UpdateOrganizationRqst.prototype.setValues = function(value) {
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
proto.resource.UpdateOrganizationRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.UpdateOrganizationRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.UpdateOrganizationRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.UpdateOrganizationRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.UpdateOrganizationRsp}
 */
proto.resource.UpdateOrganizationRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.UpdateOrganizationRsp;
  return proto.resource.UpdateOrganizationRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.UpdateOrganizationRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.UpdateOrganizationRsp}
 */
proto.resource.UpdateOrganizationRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.UpdateOrganizationRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.UpdateOrganizationRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.UpdateOrganizationRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.UpdateOrganizationRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.UpdateOrganizationRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.UpdateOrganizationRsp} returns this
 */
proto.resource.UpdateOrganizationRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.GetOrganizationsRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetOrganizationsRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetOrganizationsRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetOrganizationsRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    query: jspb.Message.getFieldWithDefault(msg, 1, ""),
    options: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.GetOrganizationsRqst}
 */
proto.resource.GetOrganizationsRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetOrganizationsRqst;
  return proto.resource.GetOrganizationsRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetOrganizationsRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetOrganizationsRqst}
 */
proto.resource.GetOrganizationsRqst.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setOptions(value);
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
proto.resource.GetOrganizationsRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetOrganizationsRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetOrganizationsRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetOrganizationsRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getQuery();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOptions();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string query = 1;
 * @return {string}
 */
proto.resource.GetOrganizationsRqst.prototype.getQuery = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetOrganizationsRqst} returns this
 */
proto.resource.GetOrganizationsRqst.prototype.setQuery = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string options = 2;
 * @return {string}
 */
proto.resource.GetOrganizationsRqst.prototype.getOptions = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetOrganizationsRqst} returns this
 */
proto.resource.GetOrganizationsRqst.prototype.setOptions = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.GetOrganizationsRsp.repeatedFields_ = [1];



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
proto.resource.GetOrganizationsRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetOrganizationsRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetOrganizationsRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetOrganizationsRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    organizationsList: jspb.Message.toObjectList(msg.getOrganizationsList(),
    proto.resource.Organization.toObject, includeInstance)
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
 * @return {!proto.resource.GetOrganizationsRsp}
 */
proto.resource.GetOrganizationsRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetOrganizationsRsp;
  return proto.resource.GetOrganizationsRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetOrganizationsRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetOrganizationsRsp}
 */
proto.resource.GetOrganizationsRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Organization;
      reader.readMessage(value,proto.resource.Organization.deserializeBinaryFromReader);
      msg.addOrganizations(value);
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
proto.resource.GetOrganizationsRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetOrganizationsRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetOrganizationsRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetOrganizationsRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOrganizationsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.resource.Organization.serializeBinaryToWriter
    );
  }
};


/**
 * repeated Organization organizations = 1;
 * @return {!Array<!proto.resource.Organization>}
 */
proto.resource.GetOrganizationsRsp.prototype.getOrganizationsList = function() {
  return /** @type{!Array<!proto.resource.Organization>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.resource.Organization, 1));
};


/**
 * @param {!Array<!proto.resource.Organization>} value
 * @return {!proto.resource.GetOrganizationsRsp} returns this
*/
proto.resource.GetOrganizationsRsp.prototype.setOrganizationsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.resource.Organization=} opt_value
 * @param {number=} opt_index
 * @return {!proto.resource.Organization}
 */
proto.resource.GetOrganizationsRsp.prototype.addOrganizations = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.resource.Organization, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.GetOrganizationsRsp} returns this
 */
proto.resource.GetOrganizationsRsp.prototype.clearOrganizationsList = function() {
  return this.setOrganizationsList([]);
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
proto.resource.AddOrganizationAccountRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AddOrganizationAccountRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AddOrganizationAccountRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddOrganizationAccountRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    organizationid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    accountid: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.AddOrganizationAccountRqst}
 */
proto.resource.AddOrganizationAccountRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AddOrganizationAccountRqst;
  return proto.resource.AddOrganizationAccountRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AddOrganizationAccountRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AddOrganizationAccountRqst}
 */
proto.resource.AddOrganizationAccountRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setOrganizationid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setAccountid(value);
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
proto.resource.AddOrganizationAccountRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AddOrganizationAccountRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AddOrganizationAccountRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddOrganizationAccountRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOrganizationid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getAccountid();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string organizationId = 1;
 * @return {string}
 */
proto.resource.AddOrganizationAccountRqst.prototype.getOrganizationid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.AddOrganizationAccountRqst} returns this
 */
proto.resource.AddOrganizationAccountRqst.prototype.setOrganizationid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string accountId = 2;
 * @return {string}
 */
proto.resource.AddOrganizationAccountRqst.prototype.getAccountid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.AddOrganizationAccountRqst} returns this
 */
proto.resource.AddOrganizationAccountRqst.prototype.setAccountid = function(value) {
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
proto.resource.AddOrganizationAccountRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AddOrganizationAccountRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AddOrganizationAccountRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddOrganizationAccountRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.AddOrganizationAccountRsp}
 */
proto.resource.AddOrganizationAccountRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AddOrganizationAccountRsp;
  return proto.resource.AddOrganizationAccountRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AddOrganizationAccountRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AddOrganizationAccountRsp}
 */
proto.resource.AddOrganizationAccountRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.AddOrganizationAccountRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AddOrganizationAccountRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AddOrganizationAccountRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddOrganizationAccountRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.AddOrganizationAccountRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.AddOrganizationAccountRsp} returns this
 */
proto.resource.AddOrganizationAccountRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.AddOrganizationGroupRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AddOrganizationGroupRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AddOrganizationGroupRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddOrganizationGroupRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    organizationid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    groupid: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.AddOrganizationGroupRqst}
 */
proto.resource.AddOrganizationGroupRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AddOrganizationGroupRqst;
  return proto.resource.AddOrganizationGroupRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AddOrganizationGroupRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AddOrganizationGroupRqst}
 */
proto.resource.AddOrganizationGroupRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setOrganizationid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setGroupid(value);
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
proto.resource.AddOrganizationGroupRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AddOrganizationGroupRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AddOrganizationGroupRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddOrganizationGroupRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOrganizationid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getGroupid();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string organizationId = 1;
 * @return {string}
 */
proto.resource.AddOrganizationGroupRqst.prototype.getOrganizationid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.AddOrganizationGroupRqst} returns this
 */
proto.resource.AddOrganizationGroupRqst.prototype.setOrganizationid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string groupId = 2;
 * @return {string}
 */
proto.resource.AddOrganizationGroupRqst.prototype.getGroupid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.AddOrganizationGroupRqst} returns this
 */
proto.resource.AddOrganizationGroupRqst.prototype.setGroupid = function(value) {
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
proto.resource.AddOrganizationGroupRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AddOrganizationGroupRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AddOrganizationGroupRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddOrganizationGroupRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.AddOrganizationGroupRsp}
 */
proto.resource.AddOrganizationGroupRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AddOrganizationGroupRsp;
  return proto.resource.AddOrganizationGroupRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AddOrganizationGroupRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AddOrganizationGroupRsp}
 */
proto.resource.AddOrganizationGroupRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.AddOrganizationGroupRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AddOrganizationGroupRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AddOrganizationGroupRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddOrganizationGroupRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.AddOrganizationGroupRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.AddOrganizationGroupRsp} returns this
 */
proto.resource.AddOrganizationGroupRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.AddOrganizationRoleRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AddOrganizationRoleRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AddOrganizationRoleRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddOrganizationRoleRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    organizationid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    roleid: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.AddOrganizationRoleRqst}
 */
proto.resource.AddOrganizationRoleRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AddOrganizationRoleRqst;
  return proto.resource.AddOrganizationRoleRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AddOrganizationRoleRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AddOrganizationRoleRqst}
 */
proto.resource.AddOrganizationRoleRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setOrganizationid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setRoleid(value);
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
proto.resource.AddOrganizationRoleRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AddOrganizationRoleRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AddOrganizationRoleRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddOrganizationRoleRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOrganizationid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getRoleid();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string organizationId = 1;
 * @return {string}
 */
proto.resource.AddOrganizationRoleRqst.prototype.getOrganizationid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.AddOrganizationRoleRqst} returns this
 */
proto.resource.AddOrganizationRoleRqst.prototype.setOrganizationid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string roleId = 2;
 * @return {string}
 */
proto.resource.AddOrganizationRoleRqst.prototype.getRoleid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.AddOrganizationRoleRqst} returns this
 */
proto.resource.AddOrganizationRoleRqst.prototype.setRoleid = function(value) {
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
proto.resource.AddOrganizationRoleRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AddOrganizationRoleRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AddOrganizationRoleRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddOrganizationRoleRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.AddOrganizationRoleRsp}
 */
proto.resource.AddOrganizationRoleRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AddOrganizationRoleRsp;
  return proto.resource.AddOrganizationRoleRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AddOrganizationRoleRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AddOrganizationRoleRsp}
 */
proto.resource.AddOrganizationRoleRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.AddOrganizationRoleRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AddOrganizationRoleRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AddOrganizationRoleRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddOrganizationRoleRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.AddOrganizationRoleRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.AddOrganizationRoleRsp} returns this
 */
proto.resource.AddOrganizationRoleRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.AddOrganizationApplicationRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AddOrganizationApplicationRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AddOrganizationApplicationRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddOrganizationApplicationRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    organizationid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    applicationid: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.AddOrganizationApplicationRqst}
 */
proto.resource.AddOrganizationApplicationRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AddOrganizationApplicationRqst;
  return proto.resource.AddOrganizationApplicationRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AddOrganizationApplicationRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AddOrganizationApplicationRqst}
 */
proto.resource.AddOrganizationApplicationRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setOrganizationid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setApplicationid(value);
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
proto.resource.AddOrganizationApplicationRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AddOrganizationApplicationRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AddOrganizationApplicationRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddOrganizationApplicationRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOrganizationid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getApplicationid();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string organizationId = 1;
 * @return {string}
 */
proto.resource.AddOrganizationApplicationRqst.prototype.getOrganizationid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.AddOrganizationApplicationRqst} returns this
 */
proto.resource.AddOrganizationApplicationRqst.prototype.setOrganizationid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string applicationId = 2;
 * @return {string}
 */
proto.resource.AddOrganizationApplicationRqst.prototype.getApplicationid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.AddOrganizationApplicationRqst} returns this
 */
proto.resource.AddOrganizationApplicationRqst.prototype.setApplicationid = function(value) {
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
proto.resource.AddOrganizationApplicationRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AddOrganizationApplicationRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AddOrganizationApplicationRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddOrganizationApplicationRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.AddOrganizationApplicationRsp}
 */
proto.resource.AddOrganizationApplicationRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AddOrganizationApplicationRsp;
  return proto.resource.AddOrganizationApplicationRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AddOrganizationApplicationRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AddOrganizationApplicationRsp}
 */
proto.resource.AddOrganizationApplicationRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.AddOrganizationApplicationRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AddOrganizationApplicationRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AddOrganizationApplicationRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddOrganizationApplicationRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.AddOrganizationApplicationRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.AddOrganizationApplicationRsp} returns this
 */
proto.resource.AddOrganizationApplicationRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.RemoveOrganizationGroupRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveOrganizationGroupRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveOrganizationGroupRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveOrganizationGroupRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    organizationid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    groupid: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.RemoveOrganizationGroupRqst}
 */
proto.resource.RemoveOrganizationGroupRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveOrganizationGroupRqst;
  return proto.resource.RemoveOrganizationGroupRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveOrganizationGroupRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveOrganizationGroupRqst}
 */
proto.resource.RemoveOrganizationGroupRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setOrganizationid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setGroupid(value);
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
proto.resource.RemoveOrganizationGroupRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveOrganizationGroupRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveOrganizationGroupRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveOrganizationGroupRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOrganizationid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getGroupid();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string organizationId = 1;
 * @return {string}
 */
proto.resource.RemoveOrganizationGroupRqst.prototype.getOrganizationid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemoveOrganizationGroupRqst} returns this
 */
proto.resource.RemoveOrganizationGroupRqst.prototype.setOrganizationid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string groupId = 2;
 * @return {string}
 */
proto.resource.RemoveOrganizationGroupRqst.prototype.getGroupid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemoveOrganizationGroupRqst} returns this
 */
proto.resource.RemoveOrganizationGroupRqst.prototype.setGroupid = function(value) {
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
proto.resource.RemoveOrganizationGroupRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveOrganizationGroupRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveOrganizationGroupRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveOrganizationGroupRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.RemoveOrganizationGroupRsp}
 */
proto.resource.RemoveOrganizationGroupRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveOrganizationGroupRsp;
  return proto.resource.RemoveOrganizationGroupRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveOrganizationGroupRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveOrganizationGroupRsp}
 */
proto.resource.RemoveOrganizationGroupRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.RemoveOrganizationGroupRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveOrganizationGroupRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveOrganizationGroupRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveOrganizationGroupRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.RemoveOrganizationGroupRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.RemoveOrganizationGroupRsp} returns this
 */
proto.resource.RemoveOrganizationGroupRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.RemoveOrganizationRoleRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveOrganizationRoleRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveOrganizationRoleRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveOrganizationRoleRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    organizationid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    roleid: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.RemoveOrganizationRoleRqst}
 */
proto.resource.RemoveOrganizationRoleRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveOrganizationRoleRqst;
  return proto.resource.RemoveOrganizationRoleRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveOrganizationRoleRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveOrganizationRoleRqst}
 */
proto.resource.RemoveOrganizationRoleRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setOrganizationid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setRoleid(value);
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
proto.resource.RemoveOrganizationRoleRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveOrganizationRoleRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveOrganizationRoleRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveOrganizationRoleRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOrganizationid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getRoleid();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string organizationId = 1;
 * @return {string}
 */
proto.resource.RemoveOrganizationRoleRqst.prototype.getOrganizationid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemoveOrganizationRoleRqst} returns this
 */
proto.resource.RemoveOrganizationRoleRqst.prototype.setOrganizationid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string roleId = 2;
 * @return {string}
 */
proto.resource.RemoveOrganizationRoleRqst.prototype.getRoleid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemoveOrganizationRoleRqst} returns this
 */
proto.resource.RemoveOrganizationRoleRqst.prototype.setRoleid = function(value) {
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
proto.resource.RemoveOrganizationRoleRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveOrganizationRoleRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveOrganizationRoleRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveOrganizationRoleRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.RemoveOrganizationRoleRsp}
 */
proto.resource.RemoveOrganizationRoleRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveOrganizationRoleRsp;
  return proto.resource.RemoveOrganizationRoleRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveOrganizationRoleRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveOrganizationRoleRsp}
 */
proto.resource.RemoveOrganizationRoleRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.RemoveOrganizationRoleRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveOrganizationRoleRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveOrganizationRoleRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveOrganizationRoleRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.RemoveOrganizationRoleRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.RemoveOrganizationRoleRsp} returns this
 */
proto.resource.RemoveOrganizationRoleRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.RemoveOrganizationApplicationRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveOrganizationApplicationRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveOrganizationApplicationRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveOrganizationApplicationRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    organizationid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    applicationid: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.RemoveOrganizationApplicationRqst}
 */
proto.resource.RemoveOrganizationApplicationRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveOrganizationApplicationRqst;
  return proto.resource.RemoveOrganizationApplicationRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveOrganizationApplicationRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveOrganizationApplicationRqst}
 */
proto.resource.RemoveOrganizationApplicationRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setOrganizationid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setApplicationid(value);
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
proto.resource.RemoveOrganizationApplicationRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveOrganizationApplicationRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveOrganizationApplicationRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveOrganizationApplicationRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOrganizationid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getApplicationid();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string organizationId = 1;
 * @return {string}
 */
proto.resource.RemoveOrganizationApplicationRqst.prototype.getOrganizationid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemoveOrganizationApplicationRqst} returns this
 */
proto.resource.RemoveOrganizationApplicationRqst.prototype.setOrganizationid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string applicationId = 2;
 * @return {string}
 */
proto.resource.RemoveOrganizationApplicationRqst.prototype.getApplicationid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemoveOrganizationApplicationRqst} returns this
 */
proto.resource.RemoveOrganizationApplicationRqst.prototype.setApplicationid = function(value) {
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
proto.resource.RemoveOrganizationApplicationRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveOrganizationApplicationRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveOrganizationApplicationRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveOrganizationApplicationRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.RemoveOrganizationApplicationRsp}
 */
proto.resource.RemoveOrganizationApplicationRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveOrganizationApplicationRsp;
  return proto.resource.RemoveOrganizationApplicationRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveOrganizationApplicationRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveOrganizationApplicationRsp}
 */
proto.resource.RemoveOrganizationApplicationRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.RemoveOrganizationApplicationRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveOrganizationApplicationRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveOrganizationApplicationRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveOrganizationApplicationRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.RemoveOrganizationApplicationRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.RemoveOrganizationApplicationRsp} returns this
 */
proto.resource.RemoveOrganizationApplicationRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.RemoveOrganizationAccountRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveOrganizationAccountRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveOrganizationAccountRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveOrganizationAccountRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    organizationid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    accountid: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.RemoveOrganizationAccountRqst}
 */
proto.resource.RemoveOrganizationAccountRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveOrganizationAccountRqst;
  return proto.resource.RemoveOrganizationAccountRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveOrganizationAccountRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveOrganizationAccountRqst}
 */
proto.resource.RemoveOrganizationAccountRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setOrganizationid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setAccountid(value);
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
proto.resource.RemoveOrganizationAccountRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveOrganizationAccountRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveOrganizationAccountRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveOrganizationAccountRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOrganizationid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getAccountid();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string organizationId = 1;
 * @return {string}
 */
proto.resource.RemoveOrganizationAccountRqst.prototype.getOrganizationid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemoveOrganizationAccountRqst} returns this
 */
proto.resource.RemoveOrganizationAccountRqst.prototype.setOrganizationid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string accountId = 2;
 * @return {string}
 */
proto.resource.RemoveOrganizationAccountRqst.prototype.getAccountid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemoveOrganizationAccountRqst} returns this
 */
proto.resource.RemoveOrganizationAccountRqst.prototype.setAccountid = function(value) {
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
proto.resource.RemoveOrganizationAccountRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveOrganizationAccountRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveOrganizationAccountRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveOrganizationAccountRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.RemoveOrganizationAccountRsp}
 */
proto.resource.RemoveOrganizationAccountRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveOrganizationAccountRsp;
  return proto.resource.RemoveOrganizationAccountRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveOrganizationAccountRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveOrganizationAccountRsp}
 */
proto.resource.RemoveOrganizationAccountRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.RemoveOrganizationAccountRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveOrganizationAccountRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveOrganizationAccountRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveOrganizationAccountRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.RemoveOrganizationAccountRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.RemoveOrganizationAccountRsp} returns this
 */
proto.resource.RemoveOrganizationAccountRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.IsOrgnanizationMemberRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.IsOrgnanizationMemberRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.IsOrgnanizationMemberRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.IsOrgnanizationMemberRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    organizationid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    accountid: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.IsOrgnanizationMemberRqst}
 */
proto.resource.IsOrgnanizationMemberRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.IsOrgnanizationMemberRqst;
  return proto.resource.IsOrgnanizationMemberRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.IsOrgnanizationMemberRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.IsOrgnanizationMemberRqst}
 */
proto.resource.IsOrgnanizationMemberRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setOrganizationid(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setAccountid(value);
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
proto.resource.IsOrgnanizationMemberRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.IsOrgnanizationMemberRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.IsOrgnanizationMemberRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.IsOrgnanizationMemberRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOrganizationid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getAccountid();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string organizationId = 1;
 * @return {string}
 */
proto.resource.IsOrgnanizationMemberRqst.prototype.getOrganizationid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.IsOrgnanizationMemberRqst} returns this
 */
proto.resource.IsOrgnanizationMemberRqst.prototype.setOrganizationid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string accountId = 2;
 * @return {string}
 */
proto.resource.IsOrgnanizationMemberRqst.prototype.getAccountid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.IsOrgnanizationMemberRqst} returns this
 */
proto.resource.IsOrgnanizationMemberRqst.prototype.setAccountid = function(value) {
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
proto.resource.IsOrgnanizationMemberRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.IsOrgnanizationMemberRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.IsOrgnanizationMemberRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.IsOrgnanizationMemberRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.IsOrgnanizationMemberRsp}
 */
proto.resource.IsOrgnanizationMemberRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.IsOrgnanizationMemberRsp;
  return proto.resource.IsOrgnanizationMemberRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.IsOrgnanizationMemberRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.IsOrgnanizationMemberRsp}
 */
proto.resource.IsOrgnanizationMemberRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.IsOrgnanizationMemberRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.IsOrgnanizationMemberRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.IsOrgnanizationMemberRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.IsOrgnanizationMemberRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.IsOrgnanizationMemberRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.IsOrgnanizationMemberRsp} returns this
 */
proto.resource.IsOrgnanizationMemberRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.DeleteOrganizationRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.DeleteOrganizationRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.DeleteOrganizationRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteOrganizationRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    organization: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.resource.DeleteOrganizationRqst}
 */
proto.resource.DeleteOrganizationRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.DeleteOrganizationRqst;
  return proto.resource.DeleteOrganizationRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.DeleteOrganizationRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.DeleteOrganizationRqst}
 */
proto.resource.DeleteOrganizationRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setOrganization(value);
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
proto.resource.DeleteOrganizationRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.DeleteOrganizationRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.DeleteOrganizationRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteOrganizationRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOrganization();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string organization = 1;
 * @return {string}
 */
proto.resource.DeleteOrganizationRqst.prototype.getOrganization = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.DeleteOrganizationRqst} returns this
 */
proto.resource.DeleteOrganizationRqst.prototype.setOrganization = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
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
proto.resource.DeleteOrganizationRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.DeleteOrganizationRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.DeleteOrganizationRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteOrganizationRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.DeleteOrganizationRsp}
 */
proto.resource.DeleteOrganizationRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.DeleteOrganizationRsp;
  return proto.resource.DeleteOrganizationRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.DeleteOrganizationRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.DeleteOrganizationRsp}
 */
proto.resource.DeleteOrganizationRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.DeleteOrganizationRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.DeleteOrganizationRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.DeleteOrganizationRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteOrganizationRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.DeleteOrganizationRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.DeleteOrganizationRsp} returns this
 */
proto.resource.DeleteOrganizationRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.Peer.repeatedFields_ = [6];



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
proto.resource.Peer.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.Peer.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.Peer} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.Peer.toObject = function(includeInstance, msg) {
  var f, obj = {
    domain: jspb.Message.getFieldWithDefault(msg, 1, ""),
    externalIpAddress: jspb.Message.getFieldWithDefault(msg, 2, ""),
    localIpAddress: jspb.Message.getFieldWithDefault(msg, 3, ""),
    mac: jspb.Message.getFieldWithDefault(msg, 4, ""),
    state: jspb.Message.getFieldWithDefault(msg, 5, 0),
    actionsList: (f = jspb.Message.getRepeatedField(msg, 6)) == null ? undefined : f
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
 * @return {!proto.resource.Peer}
 */
proto.resource.Peer.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.Peer;
  return proto.resource.Peer.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.Peer} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.Peer}
 */
proto.resource.Peer.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setDomain(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setExternalIpAddress(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setLocalIpAddress(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setMac(value);
      break;
    case 5:
      var value = /** @type {!proto.resource.PeerApprovalState} */ (reader.readEnum());
      msg.setState(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.addActions(value);
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
proto.resource.Peer.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.Peer.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.Peer} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.Peer.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDomain();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getExternalIpAddress();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getLocalIpAddress();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getMac();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getState();
  if (f !== 0.0) {
    writer.writeEnum(
      5,
      f
    );
  }
  f = message.getActionsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      6,
      f
    );
  }
};


/**
 * optional string domain = 1;
 * @return {string}
 */
proto.resource.Peer.prototype.getDomain = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Peer} returns this
 */
proto.resource.Peer.prototype.setDomain = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string external_ip_address = 2;
 * @return {string}
 */
proto.resource.Peer.prototype.getExternalIpAddress = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Peer} returns this
 */
proto.resource.Peer.prototype.setExternalIpAddress = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string local_ip_address = 3;
 * @return {string}
 */
proto.resource.Peer.prototype.getLocalIpAddress = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Peer} returns this
 */
proto.resource.Peer.prototype.setLocalIpAddress = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string mac = 4;
 * @return {string}
 */
proto.resource.Peer.prototype.getMac = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Peer} returns this
 */
proto.resource.Peer.prototype.setMac = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional PeerApprovalState state = 5;
 * @return {!proto.resource.PeerApprovalState}
 */
proto.resource.Peer.prototype.getState = function() {
  return /** @type {!proto.resource.PeerApprovalState} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {!proto.resource.PeerApprovalState} value
 * @return {!proto.resource.Peer} returns this
 */
proto.resource.Peer.prototype.setState = function(value) {
  return jspb.Message.setProto3EnumField(this, 5, value);
};


/**
 * repeated string actions = 6;
 * @return {!Array<string>}
 */
proto.resource.Peer.prototype.getActionsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 6));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.Peer} returns this
 */
proto.resource.Peer.prototype.setActionsList = function(value) {
  return jspb.Message.setField(this, 6, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.Peer} returns this
 */
proto.resource.Peer.prototype.addActions = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 6, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.Peer} returns this
 */
proto.resource.Peer.prototype.clearActionsList = function() {
  return this.setActionsList([]);
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
proto.resource.RegisterPeerRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RegisterPeerRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RegisterPeerRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RegisterPeerRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    peer: (f = msg.getPeer()) && proto.resource.Peer.toObject(includeInstance, f),
    publicKey: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.RegisterPeerRqst}
 */
proto.resource.RegisterPeerRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RegisterPeerRqst;
  return proto.resource.RegisterPeerRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RegisterPeerRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RegisterPeerRqst}
 */
proto.resource.RegisterPeerRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Peer;
      reader.readMessage(value,proto.resource.Peer.deserializeBinaryFromReader);
      msg.setPeer(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublicKey(value);
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
proto.resource.RegisterPeerRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RegisterPeerRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RegisterPeerRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RegisterPeerRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPeer();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.resource.Peer.serializeBinaryToWriter
    );
  }
  f = message.getPublicKey();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional Peer peer = 1;
 * @return {?proto.resource.Peer}
 */
proto.resource.RegisterPeerRqst.prototype.getPeer = function() {
  return /** @type{?proto.resource.Peer} */ (
    jspb.Message.getWrapperField(this, proto.resource.Peer, 1));
};


/**
 * @param {?proto.resource.Peer|undefined} value
 * @return {!proto.resource.RegisterPeerRqst} returns this
*/
proto.resource.RegisterPeerRqst.prototype.setPeer = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.resource.RegisterPeerRqst} returns this
 */
proto.resource.RegisterPeerRqst.prototype.clearPeer = function() {
  return this.setPeer(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.resource.RegisterPeerRqst.prototype.hasPeer = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string public_key = 2;
 * @return {string}
 */
proto.resource.RegisterPeerRqst.prototype.getPublicKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RegisterPeerRqst} returns this
 */
proto.resource.RegisterPeerRqst.prototype.setPublicKey = function(value) {
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
proto.resource.RegisterPeerRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RegisterPeerRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RegisterPeerRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RegisterPeerRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    peer: (f = msg.getPeer()) && proto.resource.Peer.toObject(includeInstance, f),
    publicKey: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.RegisterPeerRsp}
 */
proto.resource.RegisterPeerRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RegisterPeerRsp;
  return proto.resource.RegisterPeerRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RegisterPeerRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RegisterPeerRsp}
 */
proto.resource.RegisterPeerRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Peer;
      reader.readMessage(value,proto.resource.Peer.deserializeBinaryFromReader);
      msg.setPeer(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setPublicKey(value);
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
proto.resource.RegisterPeerRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RegisterPeerRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RegisterPeerRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RegisterPeerRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPeer();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.resource.Peer.serializeBinaryToWriter
    );
  }
  f = message.getPublicKey();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional Peer peer = 1;
 * @return {?proto.resource.Peer}
 */
proto.resource.RegisterPeerRsp.prototype.getPeer = function() {
  return /** @type{?proto.resource.Peer} */ (
    jspb.Message.getWrapperField(this, proto.resource.Peer, 1));
};


/**
 * @param {?proto.resource.Peer|undefined} value
 * @return {!proto.resource.RegisterPeerRsp} returns this
*/
proto.resource.RegisterPeerRsp.prototype.setPeer = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.resource.RegisterPeerRsp} returns this
 */
proto.resource.RegisterPeerRsp.prototype.clearPeer = function() {
  return this.setPeer(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.resource.RegisterPeerRsp.prototype.hasPeer = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string public_key = 2;
 * @return {string}
 */
proto.resource.RegisterPeerRsp.prototype.getPublicKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RegisterPeerRsp} returns this
 */
proto.resource.RegisterPeerRsp.prototype.setPublicKey = function(value) {
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
proto.resource.GetPeersRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetPeersRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetPeersRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetPeersRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    query: jspb.Message.getFieldWithDefault(msg, 1, ""),
    options: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.GetPeersRqst}
 */
proto.resource.GetPeersRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetPeersRqst;
  return proto.resource.GetPeersRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetPeersRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetPeersRqst}
 */
proto.resource.GetPeersRqst.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setOptions(value);
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
proto.resource.GetPeersRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetPeersRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetPeersRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetPeersRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getQuery();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOptions();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string query = 1;
 * @return {string}
 */
proto.resource.GetPeersRqst.prototype.getQuery = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetPeersRqst} returns this
 */
proto.resource.GetPeersRqst.prototype.setQuery = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string options = 2;
 * @return {string}
 */
proto.resource.GetPeersRqst.prototype.getOptions = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetPeersRqst} returns this
 */
proto.resource.GetPeersRqst.prototype.setOptions = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.GetPeersRsp.repeatedFields_ = [1];



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
proto.resource.GetPeersRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetPeersRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetPeersRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetPeersRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    peersList: jspb.Message.toObjectList(msg.getPeersList(),
    proto.resource.Peer.toObject, includeInstance)
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
 * @return {!proto.resource.GetPeersRsp}
 */
proto.resource.GetPeersRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetPeersRsp;
  return proto.resource.GetPeersRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetPeersRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetPeersRsp}
 */
proto.resource.GetPeersRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Peer;
      reader.readMessage(value,proto.resource.Peer.deserializeBinaryFromReader);
      msg.addPeers(value);
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
proto.resource.GetPeersRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetPeersRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetPeersRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetPeersRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPeersList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.resource.Peer.serializeBinaryToWriter
    );
  }
};


/**
 * repeated Peer peers = 1;
 * @return {!Array<!proto.resource.Peer>}
 */
proto.resource.GetPeersRsp.prototype.getPeersList = function() {
  return /** @type{!Array<!proto.resource.Peer>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.resource.Peer, 1));
};


/**
 * @param {!Array<!proto.resource.Peer>} value
 * @return {!proto.resource.GetPeersRsp} returns this
*/
proto.resource.GetPeersRsp.prototype.setPeersList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.resource.Peer=} opt_value
 * @param {number=} opt_index
 * @return {!proto.resource.Peer}
 */
proto.resource.GetPeersRsp.prototype.addPeers = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.resource.Peer, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.GetPeersRsp} returns this
 */
proto.resource.GetPeersRsp.prototype.clearPeersList = function() {
  return this.setPeersList([]);
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
proto.resource.DeletePeerRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.DeletePeerRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.DeletePeerRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeletePeerRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    peer: (f = msg.getPeer()) && proto.resource.Peer.toObject(includeInstance, f)
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
 * @return {!proto.resource.DeletePeerRqst}
 */
proto.resource.DeletePeerRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.DeletePeerRqst;
  return proto.resource.DeletePeerRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.DeletePeerRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.DeletePeerRqst}
 */
proto.resource.DeletePeerRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Peer;
      reader.readMessage(value,proto.resource.Peer.deserializeBinaryFromReader);
      msg.setPeer(value);
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
proto.resource.DeletePeerRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.DeletePeerRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.DeletePeerRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeletePeerRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPeer();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.resource.Peer.serializeBinaryToWriter
    );
  }
};


/**
 * optional Peer peer = 1;
 * @return {?proto.resource.Peer}
 */
proto.resource.DeletePeerRqst.prototype.getPeer = function() {
  return /** @type{?proto.resource.Peer} */ (
    jspb.Message.getWrapperField(this, proto.resource.Peer, 1));
};


/**
 * @param {?proto.resource.Peer|undefined} value
 * @return {!proto.resource.DeletePeerRqst} returns this
*/
proto.resource.DeletePeerRqst.prototype.setPeer = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.resource.DeletePeerRqst} returns this
 */
proto.resource.DeletePeerRqst.prototype.clearPeer = function() {
  return this.setPeer(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.resource.DeletePeerRqst.prototype.hasPeer = function() {
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
proto.resource.DeletePeerRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.DeletePeerRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.DeletePeerRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeletePeerRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.DeletePeerRsp}
 */
proto.resource.DeletePeerRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.DeletePeerRsp;
  return proto.resource.DeletePeerRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.DeletePeerRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.DeletePeerRsp}
 */
proto.resource.DeletePeerRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.DeletePeerRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.DeletePeerRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.DeletePeerRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeletePeerRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.DeletePeerRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.DeletePeerRsp} returns this
 */
proto.resource.DeletePeerRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.AddPeerActionsRqst.repeatedFields_ = [2];



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
proto.resource.AddPeerActionsRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AddPeerActionsRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AddPeerActionsRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddPeerActionsRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    mac: jspb.Message.getFieldWithDefault(msg, 1, ""),
    actionsList: (f = jspb.Message.getRepeatedField(msg, 2)) == null ? undefined : f
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
 * @return {!proto.resource.AddPeerActionsRqst}
 */
proto.resource.AddPeerActionsRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AddPeerActionsRqst;
  return proto.resource.AddPeerActionsRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AddPeerActionsRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AddPeerActionsRqst}
 */
proto.resource.AddPeerActionsRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setMac(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.addActions(value);
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
proto.resource.AddPeerActionsRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AddPeerActionsRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AddPeerActionsRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddPeerActionsRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMac();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getActionsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      2,
      f
    );
  }
};


/**
 * optional string mac = 1;
 * @return {string}
 */
proto.resource.AddPeerActionsRqst.prototype.getMac = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.AddPeerActionsRqst} returns this
 */
proto.resource.AddPeerActionsRqst.prototype.setMac = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * repeated string actions = 2;
 * @return {!Array<string>}
 */
proto.resource.AddPeerActionsRqst.prototype.getActionsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.resource.AddPeerActionsRqst} returns this
 */
proto.resource.AddPeerActionsRqst.prototype.setActionsList = function(value) {
  return jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.resource.AddPeerActionsRqst} returns this
 */
proto.resource.AddPeerActionsRqst.prototype.addActions = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.AddPeerActionsRqst} returns this
 */
proto.resource.AddPeerActionsRqst.prototype.clearActionsList = function() {
  return this.setActionsList([]);
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
proto.resource.AddPeerActionsRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AddPeerActionsRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AddPeerActionsRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddPeerActionsRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.AddPeerActionsRsp}
 */
proto.resource.AddPeerActionsRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AddPeerActionsRsp;
  return proto.resource.AddPeerActionsRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AddPeerActionsRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AddPeerActionsRsp}
 */
proto.resource.AddPeerActionsRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.AddPeerActionsRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AddPeerActionsRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AddPeerActionsRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AddPeerActionsRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.AddPeerActionsRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.AddPeerActionsRsp} returns this
 */
proto.resource.AddPeerActionsRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.RemovePeerActionRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemovePeerActionRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemovePeerActionRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemovePeerActionRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    mac: jspb.Message.getFieldWithDefault(msg, 1, ""),
    action: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.RemovePeerActionRqst}
 */
proto.resource.RemovePeerActionRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemovePeerActionRqst;
  return proto.resource.RemovePeerActionRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemovePeerActionRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemovePeerActionRqst}
 */
proto.resource.RemovePeerActionRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setMac(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setAction(value);
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
proto.resource.RemovePeerActionRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemovePeerActionRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemovePeerActionRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemovePeerActionRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMac();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getAction();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string mac = 1;
 * @return {string}
 */
proto.resource.RemovePeerActionRqst.prototype.getMac = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemovePeerActionRqst} returns this
 */
proto.resource.RemovePeerActionRqst.prototype.setMac = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string action = 2;
 * @return {string}
 */
proto.resource.RemovePeerActionRqst.prototype.getAction = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemovePeerActionRqst} returns this
 */
proto.resource.RemovePeerActionRqst.prototype.setAction = function(value) {
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
proto.resource.RemovePeerActionRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemovePeerActionRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemovePeerActionRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemovePeerActionRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.RemovePeerActionRsp}
 */
proto.resource.RemovePeerActionRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemovePeerActionRsp;
  return proto.resource.RemovePeerActionRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemovePeerActionRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemovePeerActionRsp}
 */
proto.resource.RemovePeerActionRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.RemovePeerActionRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemovePeerActionRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemovePeerActionRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemovePeerActionRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.RemovePeerActionRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.RemovePeerActionRsp} returns this
 */
proto.resource.RemovePeerActionRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.RemovePeersActionRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemovePeersActionRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemovePeersActionRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemovePeersActionRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    action: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.RemovePeersActionRqst}
 */
proto.resource.RemovePeersActionRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemovePeersActionRqst;
  return proto.resource.RemovePeersActionRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemovePeersActionRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemovePeersActionRqst}
 */
proto.resource.RemovePeersActionRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setAction(value);
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
proto.resource.RemovePeersActionRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemovePeersActionRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemovePeersActionRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemovePeersActionRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAction();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string action = 2;
 * @return {string}
 */
proto.resource.RemovePeersActionRqst.prototype.getAction = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemovePeersActionRqst} returns this
 */
proto.resource.RemovePeersActionRqst.prototype.setAction = function(value) {
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
proto.resource.RemovePeersActionRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemovePeersActionRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemovePeersActionRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemovePeersActionRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.RemovePeersActionRsp}
 */
proto.resource.RemovePeersActionRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemovePeersActionRsp;
  return proto.resource.RemovePeersActionRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemovePeersActionRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemovePeersActionRsp}
 */
proto.resource.RemovePeersActionRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.RemovePeersActionRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemovePeersActionRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemovePeersActionRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemovePeersActionRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.RemovePeersActionRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.RemovePeersActionRsp} returns this
 */
proto.resource.RemovePeersActionRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.AcceptPeerRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AcceptPeerRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AcceptPeerRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AcceptPeerRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    peer: (f = msg.getPeer()) && proto.resource.Peer.toObject(includeInstance, f)
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
 * @return {!proto.resource.AcceptPeerRqst}
 */
proto.resource.AcceptPeerRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AcceptPeerRqst;
  return proto.resource.AcceptPeerRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AcceptPeerRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AcceptPeerRqst}
 */
proto.resource.AcceptPeerRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Peer;
      reader.readMessage(value,proto.resource.Peer.deserializeBinaryFromReader);
      msg.setPeer(value);
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
proto.resource.AcceptPeerRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AcceptPeerRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AcceptPeerRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AcceptPeerRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPeer();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.resource.Peer.serializeBinaryToWriter
    );
  }
};


/**
 * optional Peer peer = 1;
 * @return {?proto.resource.Peer}
 */
proto.resource.AcceptPeerRqst.prototype.getPeer = function() {
  return /** @type{?proto.resource.Peer} */ (
    jspb.Message.getWrapperField(this, proto.resource.Peer, 1));
};


/**
 * @param {?proto.resource.Peer|undefined} value
 * @return {!proto.resource.AcceptPeerRqst} returns this
*/
proto.resource.AcceptPeerRqst.prototype.setPeer = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.resource.AcceptPeerRqst} returns this
 */
proto.resource.AcceptPeerRqst.prototype.clearPeer = function() {
  return this.setPeer(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.resource.AcceptPeerRqst.prototype.hasPeer = function() {
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
proto.resource.AcceptPeerRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.AcceptPeerRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.AcceptPeerRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AcceptPeerRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.AcceptPeerRsp}
 */
proto.resource.AcceptPeerRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.AcceptPeerRsp;
  return proto.resource.AcceptPeerRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.AcceptPeerRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.AcceptPeerRsp}
 */
proto.resource.AcceptPeerRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.AcceptPeerRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.AcceptPeerRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.AcceptPeerRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.AcceptPeerRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.AcceptPeerRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.AcceptPeerRsp} returns this
 */
proto.resource.AcceptPeerRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.RejectPeerRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RejectPeerRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RejectPeerRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RejectPeerRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    peer: (f = msg.getPeer()) && proto.resource.Peer.toObject(includeInstance, f)
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
 * @return {!proto.resource.RejectPeerRqst}
 */
proto.resource.RejectPeerRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RejectPeerRqst;
  return proto.resource.RejectPeerRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RejectPeerRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RejectPeerRqst}
 */
proto.resource.RejectPeerRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Peer;
      reader.readMessage(value,proto.resource.Peer.deserializeBinaryFromReader);
      msg.setPeer(value);
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
proto.resource.RejectPeerRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RejectPeerRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RejectPeerRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RejectPeerRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPeer();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.resource.Peer.serializeBinaryToWriter
    );
  }
};


/**
 * optional Peer peer = 1;
 * @return {?proto.resource.Peer}
 */
proto.resource.RejectPeerRqst.prototype.getPeer = function() {
  return /** @type{?proto.resource.Peer} */ (
    jspb.Message.getWrapperField(this, proto.resource.Peer, 1));
};


/**
 * @param {?proto.resource.Peer|undefined} value
 * @return {!proto.resource.RejectPeerRqst} returns this
*/
proto.resource.RejectPeerRqst.prototype.setPeer = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.resource.RejectPeerRqst} returns this
 */
proto.resource.RejectPeerRqst.prototype.clearPeer = function() {
  return this.setPeer(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.resource.RejectPeerRqst.prototype.hasPeer = function() {
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
proto.resource.RejectPeerRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RejectPeerRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RejectPeerRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RejectPeerRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.resource.RejectPeerRsp}
 */
proto.resource.RejectPeerRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RejectPeerRsp;
  return proto.resource.RejectPeerRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RejectPeerRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RejectPeerRsp}
 */
proto.resource.RejectPeerRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setResult(value);
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
proto.resource.RejectPeerRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RejectPeerRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RejectPeerRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RejectPeerRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool result = 1;
 * @return {boolean}
 */
proto.resource.RejectPeerRsp.prototype.getResult = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.resource.RejectPeerRsp} returns this
 */
proto.resource.RejectPeerRsp.prototype.setResult = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.resource.Notification.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.Notification.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.Notification} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.Notification.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, ""),
    date: jspb.Message.getFieldWithDefault(msg, 2, 0),
    message: jspb.Message.getFieldWithDefault(msg, 3, ""),
    recipient: jspb.Message.getFieldWithDefault(msg, 4, ""),
    sender: jspb.Message.getFieldWithDefault(msg, 5, ""),
    notificationType: jspb.Message.getFieldWithDefault(msg, 6, 0)
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
 * @return {!proto.resource.Notification}
 */
proto.resource.Notification.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.Notification;
  return proto.resource.Notification.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.Notification} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.Notification}
 */
proto.resource.Notification.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {number} */ (reader.readInt64());
      msg.setDate(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setMessage(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setRecipient(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setSender(value);
      break;
    case 6:
      var value = /** @type {!proto.resource.NotificationType} */ (reader.readEnum());
      msg.setNotificationType(value);
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
proto.resource.Notification.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.Notification.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.Notification} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.Notification.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getDate();
  if (f !== 0) {
    writer.writeInt64(
      2,
      f
    );
  }
  f = message.getMessage();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getRecipient();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getSender();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getNotificationType();
  if (f !== 0.0) {
    writer.writeEnum(
      6,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.resource.Notification.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Notification} returns this
 */
proto.resource.Notification.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int64 date = 2;
 * @return {number}
 */
proto.resource.Notification.prototype.getDate = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.resource.Notification} returns this
 */
proto.resource.Notification.prototype.setDate = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string message = 3;
 * @return {string}
 */
proto.resource.Notification.prototype.getMessage = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Notification} returns this
 */
proto.resource.Notification.prototype.setMessage = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string recipient = 4;
 * @return {string}
 */
proto.resource.Notification.prototype.getRecipient = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Notification} returns this
 */
proto.resource.Notification.prototype.setRecipient = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string sender = 5;
 * @return {string}
 */
proto.resource.Notification.prototype.getSender = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Notification} returns this
 */
proto.resource.Notification.prototype.setSender = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional NotificationType notification_type = 6;
 * @return {!proto.resource.NotificationType}
 */
proto.resource.Notification.prototype.getNotificationType = function() {
  return /** @type {!proto.resource.NotificationType} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {!proto.resource.NotificationType} value
 * @return {!proto.resource.Notification} returns this
 */
proto.resource.Notification.prototype.setNotificationType = function(value) {
  return jspb.Message.setProto3EnumField(this, 6, value);
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
proto.resource.CreateNotificationRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.CreateNotificationRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.CreateNotificationRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.CreateNotificationRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    notification: (f = msg.getNotification()) && proto.resource.Notification.toObject(includeInstance, f)
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
 * @return {!proto.resource.CreateNotificationRqst}
 */
proto.resource.CreateNotificationRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.CreateNotificationRqst;
  return proto.resource.CreateNotificationRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.CreateNotificationRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.CreateNotificationRqst}
 */
proto.resource.CreateNotificationRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Notification;
      reader.readMessage(value,proto.resource.Notification.deserializeBinaryFromReader);
      msg.setNotification(value);
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
proto.resource.CreateNotificationRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.CreateNotificationRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.CreateNotificationRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.CreateNotificationRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNotification();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.resource.Notification.serializeBinaryToWriter
    );
  }
};


/**
 * optional Notification notification = 1;
 * @return {?proto.resource.Notification}
 */
proto.resource.CreateNotificationRqst.prototype.getNotification = function() {
  return /** @type{?proto.resource.Notification} */ (
    jspb.Message.getWrapperField(this, proto.resource.Notification, 1));
};


/**
 * @param {?proto.resource.Notification|undefined} value
 * @return {!proto.resource.CreateNotificationRqst} returns this
*/
proto.resource.CreateNotificationRqst.prototype.setNotification = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.resource.CreateNotificationRqst} returns this
 */
proto.resource.CreateNotificationRqst.prototype.clearNotification = function() {
  return this.setNotification(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.resource.CreateNotificationRqst.prototype.hasNotification = function() {
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
proto.resource.CreateNotificationRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.CreateNotificationRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.CreateNotificationRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.CreateNotificationRsp.toObject = function(includeInstance, msg) {
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
 * @return {!proto.resource.CreateNotificationRsp}
 */
proto.resource.CreateNotificationRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.CreateNotificationRsp;
  return proto.resource.CreateNotificationRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.CreateNotificationRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.CreateNotificationRsp}
 */
proto.resource.CreateNotificationRsp.deserializeBinaryFromReader = function(msg, reader) {
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
proto.resource.CreateNotificationRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.CreateNotificationRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.CreateNotificationRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.CreateNotificationRsp.serializeBinaryToWriter = function(message, writer) {
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
proto.resource.GetNotificationsRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetNotificationsRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetNotificationsRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetNotificationsRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    recipient: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.resource.GetNotificationsRqst}
 */
proto.resource.GetNotificationsRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetNotificationsRqst;
  return proto.resource.GetNotificationsRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetNotificationsRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetNotificationsRqst}
 */
proto.resource.GetNotificationsRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRecipient(value);
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
proto.resource.GetNotificationsRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetNotificationsRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetNotificationsRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetNotificationsRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRecipient();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string recipient = 1;
 * @return {string}
 */
proto.resource.GetNotificationsRqst.prototype.getRecipient = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetNotificationsRqst} returns this
 */
proto.resource.GetNotificationsRqst.prototype.setRecipient = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.GetNotificationsRsp.repeatedFields_ = [1];



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
proto.resource.GetNotificationsRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetNotificationsRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetNotificationsRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetNotificationsRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    notificationsList: jspb.Message.toObjectList(msg.getNotificationsList(),
    proto.resource.Notification.toObject, includeInstance)
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
 * @return {!proto.resource.GetNotificationsRsp}
 */
proto.resource.GetNotificationsRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetNotificationsRsp;
  return proto.resource.GetNotificationsRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetNotificationsRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetNotificationsRsp}
 */
proto.resource.GetNotificationsRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Notification;
      reader.readMessage(value,proto.resource.Notification.deserializeBinaryFromReader);
      msg.addNotifications(value);
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
proto.resource.GetNotificationsRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetNotificationsRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetNotificationsRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetNotificationsRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getNotificationsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.resource.Notification.serializeBinaryToWriter
    );
  }
};


/**
 * repeated Notification notifications = 1;
 * @return {!Array<!proto.resource.Notification>}
 */
proto.resource.GetNotificationsRsp.prototype.getNotificationsList = function() {
  return /** @type{!Array<!proto.resource.Notification>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.resource.Notification, 1));
};


/**
 * @param {!Array<!proto.resource.Notification>} value
 * @return {!proto.resource.GetNotificationsRsp} returns this
*/
proto.resource.GetNotificationsRsp.prototype.setNotificationsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.resource.Notification=} opt_value
 * @param {number=} opt_index
 * @return {!proto.resource.Notification}
 */
proto.resource.GetNotificationsRsp.prototype.addNotifications = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.resource.Notification, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.GetNotificationsRsp} returns this
 */
proto.resource.GetNotificationsRsp.prototype.clearNotificationsList = function() {
  return this.setNotificationsList([]);
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
proto.resource.DeleteNotificationRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.DeleteNotificationRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.DeleteNotificationRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteNotificationRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, ""),
    recipient: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.DeleteNotificationRqst}
 */
proto.resource.DeleteNotificationRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.DeleteNotificationRqst;
  return proto.resource.DeleteNotificationRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.DeleteNotificationRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.DeleteNotificationRqst}
 */
proto.resource.DeleteNotificationRqst.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setRecipient(value);
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
proto.resource.DeleteNotificationRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.DeleteNotificationRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.DeleteNotificationRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteNotificationRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getRecipient();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.resource.DeleteNotificationRqst.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.DeleteNotificationRqst} returns this
 */
proto.resource.DeleteNotificationRqst.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string recipient = 2;
 * @return {string}
 */
proto.resource.DeleteNotificationRqst.prototype.getRecipient = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.DeleteNotificationRqst} returns this
 */
proto.resource.DeleteNotificationRqst.prototype.setRecipient = function(value) {
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
proto.resource.DeleteNotificationRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.DeleteNotificationRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.DeleteNotificationRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteNotificationRsp.toObject = function(includeInstance, msg) {
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
 * @return {!proto.resource.DeleteNotificationRsp}
 */
proto.resource.DeleteNotificationRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.DeleteNotificationRsp;
  return proto.resource.DeleteNotificationRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.DeleteNotificationRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.DeleteNotificationRsp}
 */
proto.resource.DeleteNotificationRsp.deserializeBinaryFromReader = function(msg, reader) {
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
proto.resource.DeleteNotificationRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.DeleteNotificationRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.DeleteNotificationRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.DeleteNotificationRsp.serializeBinaryToWriter = function(message, writer) {
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
proto.resource.ClearAllNotificationsRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.ClearAllNotificationsRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.ClearAllNotificationsRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.ClearAllNotificationsRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    recipient: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.resource.ClearAllNotificationsRqst}
 */
proto.resource.ClearAllNotificationsRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.ClearAllNotificationsRqst;
  return proto.resource.ClearAllNotificationsRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.ClearAllNotificationsRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.ClearAllNotificationsRqst}
 */
proto.resource.ClearAllNotificationsRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRecipient(value);
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
proto.resource.ClearAllNotificationsRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.ClearAllNotificationsRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.ClearAllNotificationsRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.ClearAllNotificationsRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRecipient();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string recipient = 1;
 * @return {string}
 */
proto.resource.ClearAllNotificationsRqst.prototype.getRecipient = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.ClearAllNotificationsRqst} returns this
 */
proto.resource.ClearAllNotificationsRqst.prototype.setRecipient = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
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
proto.resource.ClearAllNotificationsRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.ClearAllNotificationsRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.ClearAllNotificationsRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.ClearAllNotificationsRsp.toObject = function(includeInstance, msg) {
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
 * @return {!proto.resource.ClearAllNotificationsRsp}
 */
proto.resource.ClearAllNotificationsRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.ClearAllNotificationsRsp;
  return proto.resource.ClearAllNotificationsRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.ClearAllNotificationsRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.ClearAllNotificationsRsp}
 */
proto.resource.ClearAllNotificationsRsp.deserializeBinaryFromReader = function(msg, reader) {
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
proto.resource.ClearAllNotificationsRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.ClearAllNotificationsRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.ClearAllNotificationsRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.ClearAllNotificationsRsp.serializeBinaryToWriter = function(message, writer) {
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
proto.resource.ClearNotificationsByTypeRqst.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.ClearNotificationsByTypeRqst.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.ClearNotificationsByTypeRqst} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.ClearNotificationsByTypeRqst.toObject = function(includeInstance, msg) {
  var f, obj = {
    recipient: jspb.Message.getFieldWithDefault(msg, 1, ""),
    notificationType: jspb.Message.getFieldWithDefault(msg, 2, 0)
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
 * @return {!proto.resource.ClearNotificationsByTypeRqst}
 */
proto.resource.ClearNotificationsByTypeRqst.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.ClearNotificationsByTypeRqst;
  return proto.resource.ClearNotificationsByTypeRqst.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.ClearNotificationsByTypeRqst} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.ClearNotificationsByTypeRqst}
 */
proto.resource.ClearNotificationsByTypeRqst.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRecipient(value);
      break;
    case 2:
      var value = /** @type {!proto.resource.NotificationType} */ (reader.readEnum());
      msg.setNotificationType(value);
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
proto.resource.ClearNotificationsByTypeRqst.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.ClearNotificationsByTypeRqst.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.ClearNotificationsByTypeRqst} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.ClearNotificationsByTypeRqst.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRecipient();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getNotificationType();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
};


/**
 * optional string recipient = 1;
 * @return {string}
 */
proto.resource.ClearNotificationsByTypeRqst.prototype.getRecipient = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.ClearNotificationsByTypeRqst} returns this
 */
proto.resource.ClearNotificationsByTypeRqst.prototype.setRecipient = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional NotificationType notification_type = 2;
 * @return {!proto.resource.NotificationType}
 */
proto.resource.ClearNotificationsByTypeRqst.prototype.getNotificationType = function() {
  return /** @type {!proto.resource.NotificationType} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.resource.NotificationType} value
 * @return {!proto.resource.ClearNotificationsByTypeRqst} returns this
 */
proto.resource.ClearNotificationsByTypeRqst.prototype.setNotificationType = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
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
proto.resource.ClearNotificationsByTypeRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.ClearNotificationsByTypeRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.ClearNotificationsByTypeRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.ClearNotificationsByTypeRsp.toObject = function(includeInstance, msg) {
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
 * @return {!proto.resource.ClearNotificationsByTypeRsp}
 */
proto.resource.ClearNotificationsByTypeRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.ClearNotificationsByTypeRsp;
  return proto.resource.ClearNotificationsByTypeRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.ClearNotificationsByTypeRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.ClearNotificationsByTypeRsp}
 */
proto.resource.ClearNotificationsByTypeRsp.deserializeBinaryFromReader = function(msg, reader) {
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
proto.resource.ClearNotificationsByTypeRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.ClearNotificationsByTypeRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.ClearNotificationsByTypeRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.ClearNotificationsByTypeRsp.serializeBinaryToWriter = function(message, writer) {
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
proto.resource.Session.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.Session.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.Session} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.Session.toObject = function(includeInstance, msg) {
  var f, obj = {
    accountid: jspb.Message.getFieldWithDefault(msg, 1, ""),
    state: jspb.Message.getFieldWithDefault(msg, 2, 0),
    lastStateTime: jspb.Message.getFieldWithDefault(msg, 3, 0),
    expireAt: jspb.Message.getFieldWithDefault(msg, 4, 0)
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
 * @return {!proto.resource.Session}
 */
proto.resource.Session.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.Session;
  return proto.resource.Session.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.Session} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.Session}
 */
proto.resource.Session.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setAccountid(value);
      break;
    case 2:
      var value = /** @type {!proto.resource.SessionState} */ (reader.readEnum());
      msg.setState(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setLastStateTime(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setExpireAt(value);
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
proto.resource.Session.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.Session.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.Session} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.Session.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAccountid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getState();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
  f = message.getLastStateTime();
  if (f !== 0) {
    writer.writeInt64(
      3,
      f
    );
  }
  f = message.getExpireAt();
  if (f !== 0) {
    writer.writeInt64(
      4,
      f
    );
  }
};


/**
 * optional string accountId = 1;
 * @return {string}
 */
proto.resource.Session.prototype.getAccountid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.Session} returns this
 */
proto.resource.Session.prototype.setAccountid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional SessionState state = 2;
 * @return {!proto.resource.SessionState}
 */
proto.resource.Session.prototype.getState = function() {
  return /** @type {!proto.resource.SessionState} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.resource.SessionState} value
 * @return {!proto.resource.Session} returns this
 */
proto.resource.Session.prototype.setState = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional int64 last_state_time = 3;
 * @return {number}
 */
proto.resource.Session.prototype.getLastStateTime = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.resource.Session} returns this
 */
proto.resource.Session.prototype.setLastStateTime = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional int64 expire_at = 4;
 * @return {number}
 */
proto.resource.Session.prototype.getExpireAt = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.resource.Session} returns this
 */
proto.resource.Session.prototype.setExpireAt = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
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
proto.resource.UpdateSessionRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.UpdateSessionRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.UpdateSessionRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.UpdateSessionRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    session: (f = msg.getSession()) && proto.resource.Session.toObject(includeInstance, f)
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
 * @return {!proto.resource.UpdateSessionRequest}
 */
proto.resource.UpdateSessionRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.UpdateSessionRequest;
  return proto.resource.UpdateSessionRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.UpdateSessionRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.UpdateSessionRequest}
 */
proto.resource.UpdateSessionRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Session;
      reader.readMessage(value,proto.resource.Session.deserializeBinaryFromReader);
      msg.setSession(value);
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
proto.resource.UpdateSessionRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.UpdateSessionRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.UpdateSessionRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.UpdateSessionRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSession();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.resource.Session.serializeBinaryToWriter
    );
  }
};


/**
 * optional Session session = 1;
 * @return {?proto.resource.Session}
 */
proto.resource.UpdateSessionRequest.prototype.getSession = function() {
  return /** @type{?proto.resource.Session} */ (
    jspb.Message.getWrapperField(this, proto.resource.Session, 1));
};


/**
 * @param {?proto.resource.Session|undefined} value
 * @return {!proto.resource.UpdateSessionRequest} returns this
*/
proto.resource.UpdateSessionRequest.prototype.setSession = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.resource.UpdateSessionRequest} returns this
 */
proto.resource.UpdateSessionRequest.prototype.clearSession = function() {
  return this.setSession(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.resource.UpdateSessionRequest.prototype.hasSession = function() {
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
proto.resource.UpdateSessionResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.UpdateSessionResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.UpdateSessionResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.UpdateSessionResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.resource.UpdateSessionResponse}
 */
proto.resource.UpdateSessionResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.UpdateSessionResponse;
  return proto.resource.UpdateSessionResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.UpdateSessionResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.UpdateSessionResponse}
 */
proto.resource.UpdateSessionResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.resource.UpdateSessionResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.UpdateSessionResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.UpdateSessionResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.UpdateSessionResponse.serializeBinaryToWriter = function(message, writer) {
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
proto.resource.RemoveSessionRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveSessionRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveSessionRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveSessionRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    accountid: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.resource.RemoveSessionRequest}
 */
proto.resource.RemoveSessionRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveSessionRequest;
  return proto.resource.RemoveSessionRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveSessionRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveSessionRequest}
 */
proto.resource.RemoveSessionRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setAccountid(value);
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
proto.resource.RemoveSessionRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveSessionRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveSessionRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveSessionRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAccountid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string accountId = 1;
 * @return {string}
 */
proto.resource.RemoveSessionRequest.prototype.getAccountid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.RemoveSessionRequest} returns this
 */
proto.resource.RemoveSessionRequest.prototype.setAccountid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
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
proto.resource.RemoveSessionResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.RemoveSessionResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.RemoveSessionResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveSessionResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.resource.RemoveSessionResponse}
 */
proto.resource.RemoveSessionResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.RemoveSessionResponse;
  return proto.resource.RemoveSessionResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.RemoveSessionResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.RemoveSessionResponse}
 */
proto.resource.RemoveSessionResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.resource.RemoveSessionResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.RemoveSessionResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.RemoveSessionResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.RemoveSessionResponse.serializeBinaryToWriter = function(message, writer) {
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
proto.resource.GetSessionsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetSessionsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetSessionsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetSessionsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    query: jspb.Message.getFieldWithDefault(msg, 1, ""),
    options: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.resource.GetSessionsRequest}
 */
proto.resource.GetSessionsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetSessionsRequest;
  return proto.resource.GetSessionsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetSessionsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetSessionsRequest}
 */
proto.resource.GetSessionsRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      msg.setOptions(value);
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
proto.resource.GetSessionsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetSessionsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetSessionsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetSessionsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getQuery();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOptions();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string query = 1;
 * @return {string}
 */
proto.resource.GetSessionsRequest.prototype.getQuery = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetSessionsRequest} returns this
 */
proto.resource.GetSessionsRequest.prototype.setQuery = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string options = 2;
 * @return {string}
 */
proto.resource.GetSessionsRequest.prototype.getOptions = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetSessionsRequest} returns this
 */
proto.resource.GetSessionsRequest.prototype.setOptions = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.resource.GetSessionsResponse.repeatedFields_ = [1];



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
proto.resource.GetSessionsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetSessionsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetSessionsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetSessionsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    sessionsList: jspb.Message.toObjectList(msg.getSessionsList(),
    proto.resource.Session.toObject, includeInstance)
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
 * @return {!proto.resource.GetSessionsResponse}
 */
proto.resource.GetSessionsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetSessionsResponse;
  return proto.resource.GetSessionsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetSessionsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetSessionsResponse}
 */
proto.resource.GetSessionsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Session;
      reader.readMessage(value,proto.resource.Session.deserializeBinaryFromReader);
      msg.addSessions(value);
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
proto.resource.GetSessionsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetSessionsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetSessionsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetSessionsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSessionsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.resource.Session.serializeBinaryToWriter
    );
  }
};


/**
 * repeated Session sessions = 1;
 * @return {!Array<!proto.resource.Session>}
 */
proto.resource.GetSessionsResponse.prototype.getSessionsList = function() {
  return /** @type{!Array<!proto.resource.Session>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.resource.Session, 1));
};


/**
 * @param {!Array<!proto.resource.Session>} value
 * @return {!proto.resource.GetSessionsResponse} returns this
*/
proto.resource.GetSessionsResponse.prototype.setSessionsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.resource.Session=} opt_value
 * @param {number=} opt_index
 * @return {!proto.resource.Session}
 */
proto.resource.GetSessionsResponse.prototype.addSessions = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.resource.Session, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.resource.GetSessionsResponse} returns this
 */
proto.resource.GetSessionsResponse.prototype.clearSessionsList = function() {
  return this.setSessionsList([]);
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
proto.resource.GetSessionRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetSessionRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetSessionRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetSessionRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    accountid: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.resource.GetSessionRequest}
 */
proto.resource.GetSessionRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetSessionRequest;
  return proto.resource.GetSessionRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetSessionRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetSessionRequest}
 */
proto.resource.GetSessionRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setAccountid(value);
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
proto.resource.GetSessionRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetSessionRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetSessionRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetSessionRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAccountid();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string accountId = 1;
 * @return {string}
 */
proto.resource.GetSessionRequest.prototype.getAccountid = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.resource.GetSessionRequest} returns this
 */
proto.resource.GetSessionRequest.prototype.setAccountid = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
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
proto.resource.GetSessionResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.resource.GetSessionResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.resource.GetSessionResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetSessionResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    session: (f = msg.getSession()) && proto.resource.Session.toObject(includeInstance, f)
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
 * @return {!proto.resource.GetSessionResponse}
 */
proto.resource.GetSessionResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.resource.GetSessionResponse;
  return proto.resource.GetSessionResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.resource.GetSessionResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.resource.GetSessionResponse}
 */
proto.resource.GetSessionResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.resource.Session;
      reader.readMessage(value,proto.resource.Session.deserializeBinaryFromReader);
      msg.setSession(value);
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
proto.resource.GetSessionResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.resource.GetSessionResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.resource.GetSessionResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.resource.GetSessionResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSession();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.resource.Session.serializeBinaryToWriter
    );
  }
};


/**
 * optional Session session = 1;
 * @return {?proto.resource.Session}
 */
proto.resource.GetSessionResponse.prototype.getSession = function() {
  return /** @type{?proto.resource.Session} */ (
    jspb.Message.getWrapperField(this, proto.resource.Session, 1));
};


/**
 * @param {?proto.resource.Session|undefined} value
 * @return {!proto.resource.GetSessionResponse} returns this
*/
proto.resource.GetSessionResponse.prototype.setSession = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.resource.GetSessionResponse} returns this
 */
proto.resource.GetSessionResponse.prototype.clearSession = function() {
  return this.setSession(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.resource.GetSessionResponse.prototype.hasSession = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * @enum {number}
 */
proto.resource.PackageType = {
  UNKNOW_TYPE: 0,
  SERVICE_TYPE: 1,
  APPLICATION_TYPE: 2
};

/**
 * @enum {number}
 */
proto.resource.PeerApprovalState = {
  PEER_PENDING: 0,
  PEER_ACCETEP: 1,
  PEER_REJECTED: 2
};

/**
 * @enum {number}
 */
proto.resource.NotificationType = {
  USER_NOTIFICATION: 0,
  APPLICATION_NOTIFICATION: 1,
  SYSTEM_NOTIFICATION: 2
};

/**
 * @enum {number}
 */
proto.resource.SessionState = {
  ONLINE: 0,
  OFFLINE: 1,
  AWAY: 2
};

goog.object.extend(exports, proto.resource);
