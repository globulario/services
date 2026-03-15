# Missing Explanation RPCs — Recommended Additions

The current RBAC/Resource proto surface answers "allowed or denied" but cannot explain *why*. The MCP layer works around this with composed tools that aggregate multiple RPCs, but server-side explanation would be more efficient and accurate.

## Recommended New RPCs

### 1. `RbacService.ExplainAccess`

Server-side explanation of an access decision with full audit trail.

```protobuf
rpc ExplainAccess(ExplainAccessRequest) returns (ExplainAccessResponse);

message ExplainAccessRequest {
  string subject = 1;
  rbac.SubjectType subject_type = 2;
  string path = 3;
  string permission = 4;
}

message ExplainAccessResponse {
  bool allowed = 1;
  bool explicit_deny = 2;
  string reason = 3;                        // human-readable
  repeated MatchedRule owner_matches = 4;
  repeated MatchedRule allow_matches = 5;
  repeated MatchedRule deny_matches = 6;
  repeated string expanded_groups = 7;      // groups subject belongs to
  repeated string expanded_organizations = 8;
  repeated string expanded_roles = 9;
  string precedence = 10;                   // "deny > allow > owner"
}

message MatchedRule {
  string rule_source = 1;   // "direct" | "group:admins" | "org:globular" | "role:editor"
  string permission = 2;
  string path = 3;
}
```

**Why needed:** The current `ValidateAccess` returns only `result` (bool) and `accessDenied` (bool). The MCP layer must call 4+ RPCs to reconstruct why. A server-side ExplainAccess would be a single call.

### 2. `RbacService.ExplainAction`

Server-side explanation of an action authorization decision.

```protobuf
rpc ExplainAction(ExplainActionRequest) returns (ExplainActionResponse);

message ExplainActionRequest {
  string subject = 1;
  rbac.SubjectType subject_type = 2;
  string action = 3;
  repeated rbac.ResourceInfos infos = 4;
}

message ExplainActionResponse {
  bool allowed = 1;
  string reason = 2;
  repeated ResourceCheckResult resource_checks = 3;
  repeated string failing_resources = 4;
  repeated string missing_permissions = 5;
  repeated string matched_roles = 6;
}

message ResourceCheckResult {
  int32 index = 1;
  string path = 2;
  string permission = 3;
  bool allowed = 4;
  string reason = 5;
}
```

**Why needed:** The current `ValidateAction` returns only a bool. The MCP layer must call `GetActionResourceInfos` + individual `ValidateAccess` per resource to find which check failed.

### 3. `RbacService.GetEffectiveSubjectPermissions`

Compute the effective permission set for a subject across all role inheritance paths.

```protobuf
rpc GetEffectiveSubjectPermissions(GetEffectiveSubjectPermissionsRequest)
    returns (GetEffectiveSubjectPermissionsResponse);

message GetEffectiveSubjectPermissionsRequest {
  string subject = 1;
  rbac.SubjectType subject_type = 2;
  string resource_type = 3;    // optional filter
  string path_prefix = 4;      // optional filter
}

message GetEffectiveSubjectPermissionsResponse {
  repeated string direct_roles = 1;
  repeated string inherited_roles = 2;
  repeated EffectivePermission allows = 3;
  repeated EffectivePermission denies = 4;
  repeated string effective_actions = 5;
}

message EffectivePermission {
  string path = 1;
  string permission = 2;
  string source = 3;  // "direct" | "group:X" | "org:Y" | "role:Z"
}
```

**Why needed:** Currently requires streaming `GetResourcePermissionsBySubject` + role binding lookup + account identity expansion. A single RPC would be much faster.

### 4. `ResourceService.GetSubjectIdentityGraph`

Return the full expanded identity graph for a subject.

```protobuf
rpc GetSubjectIdentityGraph(GetSubjectIdentityGraphRequest)
    returns (GetSubjectIdentityGraphResponse);

message GetSubjectIdentityGraphRequest {
  string subject = 1;
  rbac.SubjectType subject_type = 2;
}

message GetSubjectIdentityGraphResponse {
  string subject = 1;
  rbac.SubjectType subject_type = 2;
  repeated string direct_roles = 3;
  repeated GroupMembership groups = 4;
  repeated OrgMembership organizations = 5;
  repeated string all_inherited_roles = 6;
  repeated string all_effective_actions = 7;
}

message GroupMembership {
  string group_id = 1;
  string group_name = 2;
  repeated string roles = 3;
}

message OrgMembership {
  string org_id = 1;
  string org_name = 2;
  repeated string roles = 3;
}
```

**Why needed:** Currently requires `GetAccount` + streaming `GetGroups`/`GetOrganizations`/`GetRoles` + manual graph traversal. This is the most expensive composed operation in the MCP layer.

## Priority

1. `ExplainAccess` — eliminates 4-5 RPC round-trips per diagnostic call
2. `GetSubjectIdentityGraph` — eliminates 3-4 streaming collections
3. `ExplainAction` — eliminates N+2 calls (N = resource count per action)
4. `GetEffectiveSubjectPermissions` — useful but less critical with the others
