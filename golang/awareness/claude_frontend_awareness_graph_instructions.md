# Claude Instructions — Define Frontend Awareness Graph for Globular Admin

## Purpose

Define the frontend-awareness graph for `globular-admin` and the related packages without implementing it yet.

The goal is **not** to mirror the frontend code structure. The goal is to remove the assumptions an AI agent normally makes when it does not have the whole project in context.

Frontend awareness must help an agent answer the same questions a good programmer asks before changing code:

- What is the user/system intent?
- Which feature or capability owns this change?
- Does a component, SDK function, page, or backend contract already exist?
- Where should this code live?
- What data objects are being managed?
- What permissions apply?
- What backend effects happen?
- What async states must be represented?
- What user-visible state must not be removed?
- What tests and invariants protect this behavior?
- What will this change impact?

The graph is an **assumption-killer**, not a second copy of the code.

---

## Context From the Discussion

The pain point was not that Claude cannot read individual files. The pain point was that Claude often does not know the whole project shape while editing locally.

If Claude had infinite context, it would naturally reason about the whole system:

- frontend package boundaries
- existing reusable components
- SDK connectors
- backend services
- RBAC and permissions
- async backend workflows/events
- UI state obligations
- feature ownership
- previous architectural decisions
- invariants and forbidden fixes

Since Claude does not have infinite context, the graph should answer the questions Claude would otherwise guess.

We reached this conclusion:

> Frontend awareness is essentially the same pattern as backend awareness. The graph objects and edges are different, but the mechanism is the same: extract facts, connect them, attach intent/invariants, then use impact traversal and preflight before edits.

Backend awareness protects convergence, runtime safety, workflow correctness, and service contracts.

Frontend awareness protects intent, capability placement, SDK usage, permission visibility, state representation, user-flow continuity, component reuse, data ownership, and async lifecycle meaning.

---

## Existing Code Shape to Account For

The current frontend codebase is split across several packages:

- `web.tar.gz` — main admin app pages/widgets.
- `src.tar.gz` — media app shell/pages.
- `sdk.tar.gz` — TypeScript SDK/connectors to backend services.
- `components.tar.gz` — reusable web components, file explorer, permissions, sharing, upload, UI utilities.
- `package.json` — `@globular/components`, depending on `@globular/media` and `@globular/sdk`.

Observed shape:

- Around 76k frontend LOC across `web`, `src`, `sdk`, and `components`.
- Around 142 `customElements.define(...)` registrations across the frontend/component packages.
- Around 57 admin pages in `web/src/pages`.
- Media app pages include watching, search, settings, about, and login.
- SDK package contains media, CMS/file, RBAC, workflow, repository, cluster, metrics, event, DNS, persistence, and service connectors.
- Components package contains file explorer, file uploader, permissions manager, sharing widgets, disk space manager, dialogs, split views, lists, tables, menus, markdown/image helpers, etc.

This is large enough that Claude will make local guesses unless awareness provides navigable project context.

---

## Reuse the Existing Awareness Architecture

Do **not** build a second awareness system.

Extend the existing awareness model:

- graph nodes
- graph edges
- edge classes / confidence
- invariants
- forbidden fixes
- required tests
- preflight
- semantic traversal
- context freshness
- fix ledger / learning loop
- enforcement / audit
- impact reports

Frontend awareness should be a new set of extractors, node types, edge types, and invariants attached to the same graph substrate.

The current backend awareness already has the right pattern:

```text
extractors → graph nodes/edges → semantic traversal → preflight → enforcement → learning/fix ledger
```

Frontend awareness should follow the same pattern:

```text
frontend extractors → graph nodes/edges → capability/intent graph → preflight → impact report → required tests/invariants
```

---

## Core Principle

Do not graph the DOM. Graph the meaning.

Avoid this as the primary model:

```text
app → route → page → layout → component → div → button
```

Use this instead:

```text
capability → intent → actors → system objects → permissions → entrypoints → SDK/backend calls → effects → user-visible states → invariants → tests
```

Components, pages, routes, SDK functions, files, and backend handlers are **implementation evidence** attached to capabilities.

The graph must survive UI refactors. If `MediaUploadPanel` is replaced by `DropzoneUploader`, the protected capability should remain unchanged.

---

## What We Do Not Want AI to Break

Frontend awareness must protect:

1. **Intent**  
   Why this feature/component/function exists.

2. **Capability ownership**  
   Which domain owns the feature: media, file explorer, RBAC, cluster, repository, workflow, etc.

3. **SDK usage contract**  
   Which SDK function is the approved path to backend functionality.

4. **Permission boundary**  
   What permission must be checked and how permission denial must be represented.

5. **User-visible state semantics**  
   Loading, denied, quota exceeded, pending, completed, failed, degraded, stale, etc.

6. **Async lifecycle**  
   Backend operations often continue after the frontend action returns. Example: upload success does not mean preview/indexing is complete.

7. **System object ownership**  
   Which object is being managed: resource path, media file, service, workflow run, package, user, group, role, etc.

8. **Backend effects**  
   Events, workflows, indexing, previews, storage writes, ownership changes.

9. **Reuse decisions**  
   Whether to reuse an existing component, create a domain component, create a shared component, or only modify page-local code.

10. **Required tests and invariants**  
   What must prove the feature still works after changes.

Do **not** protect:

- every `div`
- every CSS class
- every layout nesting
- decorative icons
- file/folder structure as permanent truth
- component names as permanent architecture
- raw visual placement unless it encodes a real UX contract

---

## Object Model

Define graph nodes around stable project meaning, with source-code nodes as evidence.

### Primary nodes

```text
capability
intent
domain
system_object
data_object
permission
ui_state
async_lifecycle
backend_effect
invariant
required_test
architectural_decision
```

### Implementation evidence nodes

```text
source_file
ts_module
web_component
frontend_component
page
route
sdk_function
backend_endpoint
grpc_method
http_handler
backend_service
function
class
custom_element
store
hook
worker
```

### Example capability node

```text
node: capability:media.youtube_import
name: YouTube media import
intent: Import a remote video URL into the media library using backend yt-dlp/media support.
domain: media
stability: protected
```

### Example system/data objects

```text
system_object:resource_path
system_object:media_file
system_object:preview_artifact
system_object:index_document
system_object:user_account
system_object:rbac_role
system_object:workflow_run
system_object:service_instance
```

### Example UI states

```text
ui_state:idle
ui_state:validating_url
ui_state:downloading
ui_state:uploading
ui_state:uploaded
ui_state:processing_video
ui_state:preview_pending
ui_state:preview_ready
ui_state:indexing_pending
ui_state:indexed
ui_state:permission_denied
ui_state:quota_exceeded
ui_state:backend_unavailable
ui_state:failed
```

---

## Edge Model

Edges are the real intelligence. They answer programmer questions.

### Core edges

```text
belongs_to_domain
implements_capability
participates_in_capability
exposes_entrypoint_for
uses_sdk
maps_to_backend
calls_rpc
calls_http
requires_permission
touches_object
produces_object
emits_event
consumes_event
starts_workflow
observes_state
displays_state
must_display_state
has_async_state
protected_by
forbidden_by
proven_by
reuses_component
supersedes_component
owned_by_package
located_in_file
imports
calls
renders
```

### Example frontend/backend bridge

```text
component:MediaImportPanel
  exposes_entrypoint_for -> capability:media.youtube_import

component:MediaImportPanel
  uses_sdk -> sdk_function:media.uploadVideoByUrl

sdk_function:media.uploadVideoByUrl
  maps_to_backend -> backend_service:media.MediaService

capability:media.youtube_import
  requires_permission -> permission:resource.write

capability:media.youtube_import
  touches_object -> system_object:resource_path

capability:media.youtube_import
  produces_object -> system_object:media_file

capability:media.youtube_import
  triggers -> capability:media.preview_generation

capability:media.youtube_import
  triggers -> capability:media.indexing

capability:media.youtube_import
  must_display_state -> ui_state:downloading

capability:media.youtube_import
  must_display_state -> ui_state:permission_denied

capability:media.youtube_import
  protected_by -> invariant:imported_video_is_not_necessarily_preview_ready
```

---

## Edge Classes and Confidence

Reuse the existing awareness idea of edge class/confidence.

Not all frontend facts are equal.

### Decision edges

These are strong and should influence preflight.

```text
capability -> protected_by -> invariant
capability -> requires_permission -> permission
capability -> must_display_state -> ui_state
capability -> uses_sdk -> sdk_function
capability -> touches_object -> system_object
```

### Structural edges

These are useful for impact traversal.

```text
component -> located_in_file -> source_file
component -> imports -> module
component -> renders -> child_component
page -> exposes_entrypoint_for -> capability
```

### Information edges

These are weak context, useful but not law.

```text
component -> has_css_class -> className
component -> has_attribute -> non-contract attribute
component -> contains_dom_element -> div/button/input
```

Avoid flooding the graph with low-value information edges.

---

## Extractor Plan

Create deterministic extractors first. Do not rely on LLM interpretation for basic structure.

### 1. TypeScript / JavaScript module extractor

Extract:

- files
- imports
- exports
- classes
- functions
- constants
- async functions
- workers
- SDK imports
- direct backend/generated-client imports
- direct `fetch`/RPC calls

Output:

```text
source_file
ts_module
function
class
sdk_function usage
imports/calls edges
```

### 2. Web component extractor

Extract:

- `customElements.define(name, class)`
- class name
- file path
- observed attributes when present
- public setters/getters
- emitted DOM events: `dispatchEvent`, `CustomEvent`
- listened events: `addEventListener`
- lifecycle callbacks: `connectedCallback`, `disconnectedCallback`, `attributeChangedCallback`
- shadow DOM usage

Output:

```text
web_component
custom_element
observes_attribute
emits_dom_event
listens_to_dom_event
located_in_file
```

Only promote attributes/events to contract-level facts when they are linked to a capability, data object, permission, or state.

### 3. Page / route extractor

Extract:

- route definitions
- page custom elements
- page files
- page imports
- page-level SDK calls
- page-level widgets/components

Output:

```text
page
route
exposes_entrypoint_for
renders
located_in_file
```

Do not treat page layout as protected unless it represents a functional responsibility.

### 4. SDK extractor

Extract from `@globular/sdk`:

- exported SDK functions
- service domains
- backend client construction
- RPC/HTTP mapping when visible
- input/output types when visible
- callback/progress semantics
- error handling patterns

Output:

```text
sdk_function
backend_service
grpc_method/http_endpoint
maps_to_backend
returns_data_object
accepts_data_object
emits_progress
```

The SDK is the key bridge between frontend intent and backend contracts.

### 5. Component package extractor

Extract reusable components from `@globular/components`:

- file explorer
- uploader
- permission manager
- sharing widgets
- disk space manager
- dialogs
- lists/tables/menus
- search views

Output:

```text
reusable_component
component_domain
candidate_reuse_for capability
owned_by_package
```

### 6. Manual intent/invariant extractor

Some facts cannot be safely inferred from code.

Add explicit annotation files for:

```text
frontend_capabilities.yaml
frontend_invariants.yaml
frontend_decisions.yaml
frontend_required_tests.yaml
frontend_forbidden_fixes.yaml
```

These files should define intent, ownership, protected states, and architectural decisions.

---

## What Must Be Manual vs Inferred

### Inferred from code

```text
file/module/component existence
custom element registration
imports
exports
SDK calls
routes
DOM events
attributes
function calls
generated client usage
direct fetch/RPC usage
```

### Inferred from SDK/backend bridge

```text
SDK function maps to backend service
backend service domain
input/output shape
known RPC/HTTP contract
likely data object touched
progress callback shape
```

### Manual / curated

```text
feature intent
capability ownership
whether a component is canonical/reusable
permission meaning
required UI states
async lifecycle semantics
forbidden shortcuts
required tests
architectural decisions
```

Do not pretend manual facts can be safely inferred from code. The goal is trustworthy awareness, not graph fantasy.

---

## Agent Usage Model

Claude should use frontend awareness before implementing or refactoring frontend code.

### Before implementing a feature

Claude asks awareness:

```text
Resolve this user request to capabilities.
Find existing pages/components/SDK functions/backend contracts.
Show package ownership and reuse candidates.
Show required permissions, states, invariants, and tests.
```

### Before editing a component

Claude asks awareness:

```text
Which capabilities does this component participate in?
Which SDK/backend contracts does it use?
Which user-visible states must it preserve?
Which permissions and failures must remain visible?
Which tests/invariants apply?
What is the impact radius?
```

### Before editing an SDK function

Claude asks awareness:

```text
Who calls this SDK function?
Which capabilities depend on it?
Which pages/components expose it?
Which backend method does it map to?
Which UI states depend on its errors/progress/results?
What tests must run?
```

### Before creating a new component

Claude asks awareness:

```text
Does an existing component already cover this role?
Should this be page-local, domain-level, shared component, or SDK-only?
What public attributes/properties/events should exist?
What capability does it implement?
Does it own backend calls or receive data from parent?
What permission/error/pending states are mandatory?
Where should the file live?
```

### Before changing layout

Claude asks awareness:

```text
Does this layout region encode a functional responsibility?
Will this remove a capability entrypoint?
Will this hide a required state?
Will this break permission visibility or action reachability?
```

---

## Required Preflight Modes

Add frontend-aware preflight modes without breaking existing backend preflight.

Suggested commands:

```bash
globular awareness preflight --frontend <file-or-dir>
globular awareness trace-capability media.youtube_import
globular awareness trace-sdk media.uploadVideoByUrl
globular awareness impact --frontend <changed-file>
globular awareness check-frontend-contracts
globular awareness explain-ui-state preview_pending
globular awareness suggest-placement "import youtube video into media library"
```

### Preflight output must include

```text
matched capabilities
files/components/pages involved
SDK/backend contracts involved
permissions involved
required UI states
async lifecycle notes
invariants
forbidden fixes
required tests
impact radius
unknowns / blind spots
```

If the graph cannot answer a key question, it must say so explicitly:

```text
UNKNOWN: no known capability owner for this request
UNKNOWN: no SDK mapping found for this backend action
UNKNOWN: no required UI state contract found
```

Unknowns are safer than invented certainty.

---

## Example Vertical Slice — YouTube Media Import

Use this as the first design example.

User intent:

```text
Upload/import on the cluster a video file from YouTube using backend yt-dlp support. The frontend may use drag-and-drop or URL input.
```

### Capability

```text
capability: media.youtube_import
intent: Import a remote video URL into the media library using backend media/yt-dlp support.
domain: media
```

### Questions awareness must answer

```text
1. Does this capability already exist?
2. Which domain owns it?
3. Which app/page currently handles media watching/search/settings?
4. Which SDK function imports/downloads remote media?
5. Which backend service owns the operation?
6. What destination object is required? resource_path? media library? channel?
7. What permissions are required?
8. What progress/error states are exposed by the SDK/backend?
9. What UI states are required?
10. Should this be a reusable component or page-local feature?
11. Should it reuse file uploader/dropzone components?
12. What happens after import? preview? indexing? search visibility?
13. What tests prove it works?
14. What invariant protects it?
```

### Expected graph answer shape

```text
capability:media.youtube_import
  belongs_to_domain -> domain:media
  uses_sdk -> sdk_function:media.uploadVideoByUrl
  requires_input -> data_object:youtube_url
  requires_input -> system_object:resource_path
  produces_object -> system_object:media_file
  triggers -> capability:media.preview_generation
  triggers -> capability:media.indexing
  requires_permission -> permission:resource.write
  must_display_state -> ui_state:validating_url
  must_display_state -> ui_state:downloading
  must_display_state -> ui_state:processing_video
  must_display_state -> ui_state:preview_pending
  must_display_state -> ui_state:indexing_pending
  must_display_state -> ui_state:permission_denied
  must_display_state -> ui_state:quota_exceeded
  protected_by -> invariant:remote_import_success_is_not_preview_ready
  proven_by -> test:media_youtube_import_progress
```

### Forbidden fixes

```text
Do not call yt-dlp directly from frontend.
Do not bypass @globular/sdk.
Do not call generated backend clients directly from a page if an SDK connector exists.
Do not collapse permission denied into generic failure.
Do not show preview-ready immediately after import completion unless preview artifact exists.
Do not duplicate generic uploader/dropzone logic if an approved reusable component exists.
```

### Required tests

```text
URL validation failure
permission denied
quota exceeded / insufficient storage
import progress displayed
backend failure displayed
import complete while preview pending
preview becomes visible later
media appears in search/list after indexing
```

---

## Frontend Invariant Examples

### SDK boundary

```text
Invariant: Frontend backend calls must use approved SDK connectors unless explicitly marked as low-level/generated-client code.
```

### Permission visibility

```text
Invariant: A protected user action must expose permission-denied state separately from generic failure.
```

### Async lifecycle

```text
Invariant: Upload/import completion must not be represented as preview/index readiness unless the preview/index artifact is confirmed.
```

### Reuse

```text
Invariant: Page-local code must not duplicate an existing reusable component when the reusable component already owns the capability contract.
```

### Data ownership

```text
Invariant: UI features that mutate resource paths must preserve resource ownership and permission semantics.
```

### Component boundary

```text
Invariant: A reusable component must expose capability-relevant state through public attributes/properties/events rather than hidden page assumptions.
```

---

## Debugging Expectations

Frontend awareness will help debugging, but not in exactly the same way as backend awareness.

Backend awareness can often trace hard causal chains:

```text
etcd key → workflow step → service runtime → invariant violation
```

Frontend awareness will more often provide:

```text
component → capability → SDK function → backend service → expected state → missing UI representation
```

It is strongest when the bug crosses the frontend/backend boundary.

Example:

```text
YouTube import completed but video is not visible.
```

Awareness should help determine whether:

- import succeeded but indexing is pending
- preview is pending but UI hides pending cards
- SDK progress is swallowed
- media list is search-backed and not refreshed
- permission prevents listing the imported file
- backend event/worker failed

Frontend awareness is less about proving root cause from runtime facts and more about preventing/locating broken assumptions.

---

## Impact Measurement

The main value is impact traversal.

When a file changes, awareness should answer:

```text
Which capabilities are impacted?
Which SDK functions are impacted?
Which pages/components are impacted?
Which user-visible states may be impacted?
Which permissions may be impacted?
Which backend services/events/workflows may be impacted?
Which tests must run?
Which invariants/forbidden fixes apply?
```

Impact radius should use edge count/depth/dimension just like backend awareness.

Suggested edge depth defaults:

```text
changed file → component/function → capability → SDK/backend → permission/state/invariant/test
```

Do not expand into low-value DOM/style edges by default.

---

## Implementation Phases

### Phase F0 — Design only

Deliver:

- agent question catalog
- node model
- edge model
- extractor plan
- manual annotation schema
- first vertical slice for `media.youtube_import`
- risks/blind spots

Do not implement yet.

### Phase F1 — Structural extraction

Build deterministic extractors for:

- TypeScript/JavaScript modules
- imports/exports
- custom elements
- pages/routes
- SDK exported functions
- direct backend/generated-client usage

### Phase F2 — SDK/backend bridge

Map:

```text
frontend SDK function → backend service/RPC/HTTP endpoint → data object/effect
```

This is the main bridge between frontend and existing backend awareness.

### Phase F3 — Capability/invariant annotations

Add curated frontend files:

```text
frontend_capabilities.yaml
frontend_invariants.yaml
frontend_decisions.yaml
frontend_forbidden_fixes.yaml
frontend_required_tests.yaml
```

Start with media capabilities.

### Phase F4 — Preflight and impact reports

Add:

```text
--frontend preflight
trace-capability
trace-sdk
check-frontend-contracts
suggest-placement
```

### Phase F5 — Media vertical slice

Implement and validate awareness for:

```text
media.youtube_import
media.upload
media.preview_generation
media.indexing
media.search_visibility
```

### Phase F6 — Expand to admin capabilities

Add domains:

```text
rbac
cluster
services
repository
workflow
observability
storage
networking
```

---

## Deliverable Claude Must Produce First

Claude should produce a design report, not code.

Required sections:

1. Current frontend package map.
2. Current awareness reuse map.
3. Agent question catalog.
4. Proposed node types.
5. Proposed edge types.
6. Edge classes and confidence model.
7. Extractor plan.
8. Manual annotation schema.
9. Preflight/impact command design.
10. First vertical slice: `media.youtube_import`.
11. Invariants and forbidden fixes.
12. Required tests.
13. Known blind spots.
14. Estimated LOC by phase.
15. Implementation order.

---

## Final Rule

Frontend awareness should not tell Claude everything in the code.

It should tell Claude what it must not guess.

The code is implementation.  
The graph is project memory.  
The invariant is the law.  
The preflight is the gate.  
The impact report is the flashlight.

Build the graph so an AI agent can ask the project what is true before it edits.

---

# Claude Addendum — Risks and Patterns From Backend Awareness (2026-05-10)

After shipping detector mapping + CI ratchets in backend awareness, several
class-level risks become visible for frontend. Capture them here BEFORE F1
extraction starts so we don't repeat the same mistakes.

## F-A1. The Prefix-Bug Class Will Recur — Define Naming Conventions Up-Front

Backend awareness shipped with a ~3-week orphan-coverage bug because the
failure_modes table stored ids un-prefixed while graph nodes stored them with
a `failure_mode:` prefix. The lookup never matched.

Frontend will have many more namespaces:

```text
capability:media.youtube_import
domain:media
ui_state:downloading
permission:resource.write
sdk_function:media.uploadVideoByUrl
component:MediaImportPanel
custom_element:media-import-panel
page:/admin/media/import
route:GET /admin/media/import
backend_service:media.MediaService
data_object:youtube_url
system_object:media_file
```

12 namespaces is ~3x the backend count. Without an enforced convention this
becomes a maintenance disaster.

**Required before F1**:

1. Single source of truth for prefixes:

   ```go
   // golang/awareness/extractors/frontend/ids.go
   const (
       NodePrefixCapability     = "capability:"
       NodePrefixDomain         = "domain:"
       NodePrefixUIState        = "ui_state:"
       NodePrefixPermission     = "permission:"
       NodePrefixSDKFunction    = "sdk_function:"
       NodePrefixComponent      = "component:"
       NodePrefixCustomElement  = "custom_element:"
       NodePrefixPage           = "page:"
       NodePrefixRoute          = "route:"
       NodePrefixBackendService = "backend_service:"
       NodePrefixDataObject     = "data_object:"
       NodePrefixSystemObject   = "system_object:"
   )
   func CapabilityNodeID(id string) string { return NodePrefixCapability + id }
   // ... one helper per type, and a TrimPrefix companion for each
   ```

2. EVERY extractor uses these helpers. No string concatenation of prefixes
   inside extractor code. Catch in code review.

3. Coverage / preflight / MCP code uses the same helpers when looking up by id.

4. A single regression test that, for each namespace, builds one node and
   asserts the lookup round-trip works:
   `TestNamespaceConvention_RoundTripsAllPrefixes`.

This is cheap to enforce on day 1 and catastrophically expensive to fix later.

## F-A2. Reuse the Mapping-File Idiom for Cross-Layer Joins

Backend's `detector_mapping.yaml` worked because it kept two extractor outputs
loosely coupled and reviewable by humans. Frontend should reuse the pattern
for every cross-layer join:

```text
component_capability_mapping.yaml      # component → capability
sdk_capability_mapping.yaml            # sdk_function → capability
sdk_backend_mapping.yaml               # sdk_function → backend_service/rpc
page_capability_mapping.yaml           # page → capability
permission_capability_mapping.yaml     # permission → capability (gates)
```

Each mapping file is:

- human-readable (operators can review)
- additive (each row stands alone)
- testable (an extractor reads + emits typed edges)
- diff-friendly (PR review surfaces capability changes)

The alternative — scattering capability ids inside @ts-doc comments or YAML
front-matter inside source files — is much harder to review and prone to
silent drift.

**Rule**: never put a cross-layer relationship inside source code annotations
when a mapping file would do.

## F-A3. customElements Registry Is the Spine — Extract It First

142 `customElements.define(name, class)` calls is the closest thing the
frontend has to a manifest. Every component, every page, every dialog, every
widget is one of those calls. F1 extraction should start there:

```text
walk all *.ts/*.js files
parse customElements.define(<name>, <class>) calls
emit one custom_element:<name> node per registration
record file path, class name, observed attributes
```

Everything else (capability mapping, component reuse analysis, SDK call
extraction) hangs off this spine. Without it, the graph has no anchor.

## F-A4. Hard No on node_modules

Hard rule for the extractor:

```text
node_modules/  → skip entirely
dist/          → skip entirely
build/         → skip entirely
.cache/        → skip entirely
*.min.js       → skip entirely
generated/     → only if explicitly listed (some SDK clients live here)
```

A single mis-step that walks node_modules will produce O(100k) source_file
nodes and explode the graph. Make this part of the extractor's entry path,
not an afterthought.

## F-A5. Frontend Code Drifts Faster Than Backend — Adapt the Ratchet

Backend code changes O(10) PRs/week affecting the awareness graph. Frontend
code changes O(50+) PRs/week. The CI ratchet must be more permissive:

- Backend: `--min-well-covered` floor never decreases.
- Frontend: `--min-well-covered` floor decreases by N% per quarter is allowed
  if explicitly approved, OR the floor only applies to capabilities annotated
  `stability: protected`.

The list of "protected" capabilities is the actual ratchet. Everything else
floats. This avoids the trap of CI failing every other PR because a UI
refactor moved a component without breaking its capability.

## F-A6. Frontend Awareness Needs a Per-Capability Trust Envelope Too

The trust envelope from `assurance.Compose()` should apply to frontend
results identically. A frontend capability with:

- mitigation: yes (an existing component owns it)
- test: yes (e2e test linked)
- detector: no (frontend has no equivalent of cluster_doctor)

should be `well_covered` if "detector" is interpreted as "user-observable
state contract". The trust envelope must clearly say:

- "matched a known capability with full UI-state contracts and tests" → trusted
- "matched a capability with NO required UI states declared" → limited
- "no capability match" → unknown
- "graph stale" → stale

The trust envelope's `coverage` axis needs a small extension: detector
becomes "runtime/observability evidence" generally. For frontend, that's
required UI state declarations + e2e tests; for backend, doctor rules + alerts.

## F-A7. Test Fixtures Should Use Real Media-Capability Source

The doc says start with `media.youtube_import` as the vertical slice. The
test fixture for that slice should NOT be hand-written YAML — it should be
produced by running the extractor against the actual `web/`, `sdk/`, and
`components/` packages and capturing the output. That way:

- the test exercises the real extractor code path
- regressions in the extractor are caught
- the fixture stays in sync with the package layout

Backend awareness benefited enormously from `TestExtract_LiveDoctorRulesProduceNodes`
running against the real source tree. Replicate that here.

## F-A8. Awareness Bundle Must Include Frontend Manifest

The current awareness bundle ships backend YAML + graph.db. When frontend
awareness lands, the bundle must include:

- frontend_capabilities.yaml + the rest of the curated YAMLs
- the typescript-extracted facts (custom_element registrations, SDK functions)
- the mapping files

Otherwise an agent running `awareness preflight` on frontend code will find
nothing and silently say "trusted, no match" — exactly the rubber-stamp
failure the assurance layer was built to prevent.

The bundle build pipeline (`build/build-services.sh` or wherever it's
defined) needs a frontend extraction step before the tarball.

## F-A9. Estimated LOC by Phase (rough)

Based on backend equivalents:

| Phase | Backend equivalent | Frontend estimate |
|---|---|---|
| F1 structural | goast extractor (~2.5k LOC) | TS module extractor (~3k LOC) |
| F2 SDK bridge | proto extractor (~1.5k LOC) | SDK + RPC mapping (~2k LOC) |
| F3 annotations | failure_modes.yaml + extractor (~600 LOC) | 5 yaml files + 5 loaders (~1.5k LOC) |
| F4 preflight/impact | analysis package (~3k LOC) | analysis/frontend (~2.5k LOC) |
| F5 vertical slice | manual extractor + tests (~800 LOC) | media/* (~600 LOC) |
| F6 expansion | per-domain seeds | per-domain seeds |

Total F1–F5: ~10k LOC. Plan accordingly.

## F-A10. Risk: Frontend Awareness Could Become a Distraction From Trust Wiring

The trust envelope wiring (the OTHER document) is the highest-leverage work
right now. Frontend awareness is a much larger investment with delayed payoff.

**Recommendation**: do NOT start F1 until trust envelope wiring is shipped
and observed working in production for at least one cycle. Otherwise the
frontend extractors will race ahead of the assurance discipline they need
to inherit, and we'll be re-doing the orphan-coverage bug at frontend scale.

Order:
1. Trust envelope (claude_awareness_next_pr_instructions.md) — ship first
2. Observe one cycle, fix any envelope/MCP issues
3. THEN start F1 frontend structural extraction
4. Frontend awareness inherits the assurance/coverage/freshness machinery
   from day 1, instead of having to retrofit it
