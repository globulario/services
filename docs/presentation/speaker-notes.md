# Globular Presentation — Speaker Notes

Read these notes aloud while advancing through the slides. Each section corresponds to one slide. Timing estimate: ~12-15 minutes total.

---

## Slide 1 — Title

> Globular. Distributed systems made explicit.
>
> This is a presentation about a different way to build and operate distributed applications. Not harder. Not easier. Just... clearer.
>
> Globular is an open-source microservices platform. No containers. No Kubernetes. No cloud provider required. Just Linux machines, compiled binaries, and a system you can actually understand.
>
> Let me show you what that means.

---

## Slide 2 — The Problem

> If you've ever operated distributed infrastructure, you know this feeling.
>
> Something fails. And now you're digging through logs across five different systems, trying to piece together what happened. That's opaque execution. The system did something, but it won't tell you what.
>
> *(advance fragment)*
>
> Then there's hidden state. Configuration files on some nodes, environment variables on others, secrets in a vault, feature flags in a database. Nobody knows the full picture. And when things drift apart, you don't find out until production breaks.
>
> *(advance fragment)*
>
> And if you're using containers, there's the container tax. Before you even run your code, you need a container runtime, an image registry, overlay networking, CNI plugins, storage drivers. That's a lot of infrastructure just to run a binary.
>
> *(advance fragment)*
>
> And finally, implicit convergence. Controllers that watch for changes and retry silently. They guess what to do. They hope for the best. And when they fail, they fail quietly.
>
> These are real problems. And they're not bugs — they're design choices. Globular makes different choices.

---

## Slide 3 — The Question

> So here's the question that started everything.
>
> What if a cluster behaved exactly the way you describe it? Explicitly. Deterministically. Transparently.
>
> Not a system that hides complexity behind layers of abstraction. A system that exposes it, organizes it, and makes it predictable.
>
> That's the idea behind Globular.

---

## Slide 4 — What is Globular

> Globular is an open-source platform for building and operating self-hosted distributed applications.
>
> *(advance fragment)*
>
> First: native binaries. Services are compiled Go binaries that run under systemd. No container runtime. No image registry. No overlay network. The binary runs directly on the host.
>
> *(advance fragment)*
>
> Second: workflow-driven. Every operation in the cluster — deploying a service, upgrading infrastructure, repairing drift — goes through a formal workflow. With defined phases, failure classification, and a complete audit trail.
>
> *(advance fragment)*
>
> Third: single source of truth. All configuration, all state, all service discovery lives in etcd. Not scattered across config files and environment variables. One place. Queryable. Watchable. Consistent.
>
> *(advance fragment)*
>
> And fourth: AI-ready. Because the system is explicit and observable, AI can actually reason about it. Not as a black box. But as something it can understand, diagnose, and act on — through the same workflows you use.

---

## Slide 5 — Three Principles

> Globular is built on three principles.
>
> *(advance fragment)*
>
> Explicit. Nothing is hidden. Nothing "just happens." Every state change has a cause. Every action has a record. If you can't see it, it doesn't exist.
>
> *(advance fragment)*
>
> Observable. Every action is a workflow. Every workflow is queryable. When something goes wrong, you don't guess — you look at the execution history and see exactly what happened, step by step.
>
> *(advance fragment)*
>
> Deterministic. Same input, same output. No invisible reconciliation loops. Failures are classified by type, and each type has a specific retry strategy. The system doesn't guess. It follows rules.
>
> *(advance fragment)*
>
> And here's why this matters: once you understand your system, you can trust it. And once you trust it, you can build anything on top of it.

---

## Slide 6 — The 4-Layer State Model

> This is the foundational concept in Globular. The four-layer state model.
>
> Every package in the cluster is tracked across four independent truth layers.
>
> *(advance fragment)*
>
> Layer one: the artifact. Does this version exist in the repository? Is it valid? Has the checksum been verified?
>
> *(advance fragment)*
>
> Layer two: the desired state. What version should be running? This is what the operator declares. It lives in etcd.
>
> *(advance fragment)*
>
> Layer three: the installed state. What version is actually on each node? This is what the node agent reports. It's ground truth.
>
> *(advance fragment)*
>
> Layer four: runtime health. Is the installed service actually running and passing health checks right now?
>
> *(advance fragment)*
>
> These four layers are never collapsed. The system does not assume that "desired" means "installed," or that "installed" means "healthy." Drift in any layer triggers investigation. No silent failures. No "almost correct."
>
> In Kubernetes, you see "desired not equal to observed" — but that could mean three completely different things. In Globular, you see exactly which layer is misaligned and why.

---

## Slide 7 — Workflows, Not Controllers

> In Globular, nothing changes state without a workflow.
>
> *(advance fragment)*
>
> Here's what a workflow looks like. Decision, fetch, install, configure, start, verify. Each phase has a specific actor. Each phase produces a result. And every attempt is recorded with timing, status, and error details.
>
> *(advance fragment)*
>
> When something fails, the failure is classified. Is it a configuration error? A missing package? A network timeout? A dependency problem? Each type has its own retry strategy.
>
> Network errors retry with backoff. Validation errors stop immediately — because retrying won't help. Dependency errors block and wait.
>
> And the system has built-in safety. A semaphore limits concurrent workflows to three at a time. A circuit breaker pauses everything when the cluster is unhealthy. A five-minute backoff prevents failed deployments from consuming resources.
>
> No cascading storms. Predictable under stress.

---

## Slide 8 — Three Actors

> The system has three actors, each with a single, clear responsibility.
>
> *(advance fragment)*
>
> The cluster controller decides what should happen. It watches the desired state, detects drift, and dispatches workflows.
>
> *(advance fragment)*
>
> The workflow service orchestrates how it happens. It executes steps, tracks results, handles retries.
>
> *(advance fragment)*
>
> The node agent performs the actual work. It manages systemd units, installs packages, runs health checks, and reports status.
>
> No overlap. No ambiguity. Each part has one job.

---

## Slide 9 — 28+ Built-in Services

> Globular comes with over twenty-eight services out of the box.
>
> *(advance fragment)*
>
> Identity and security: JWT authentication, per-resource RBAC, a full PKI with mutual TLS, and LDAP integration.
>
> Data and storage: MongoDB, BadgerDB, MinIO object storage, full-text search, and a publish-subscribe event bus.
>
> Infrastructure: an authoritative DNS server, an Envoy gateway with xDS, service discovery, and centralized logging.
>
> Operations: Prometheus monitoring, automated backup and restore, a cluster doctor that checks invariants and proposes fixes, and a built-in package repository.
>
> An AI layer: persistent memory backed by ScyllaDB, an AI executor powered by Claude, event watchers, and a traffic router.
>
> And application services: file management, media transcoding, SMTP mail, and gRPC-Web for browser clients.
>
> This isn't a framework where you bring your own everything. It's a platform. Everything you need to run distributed applications is included.

---

## Slide 10 — Globular vs. Kubernetes

> Let's be direct about how Globular compares to Kubernetes.
>
> *(advance fragment)*
>
> Kubernetes deploys container images. Globular deploys native binaries.
>
> Kubernetes uses kubelet and a container runtime. Globular uses systemd — which is already on every Linux machine.
>
> In Kubernetes, etcd is hidden and internal. In Globular, etcd is a user-facing API. You can query it, watch it, and build on it directly.
>
> When something fails in Kubernetes, controllers retry silently. In Globular, failures are classified and handled through formal workflows with a complete audit trail.
>
> Kubernetes events are ephemeral — they disappear. Globular workflow history is permanent. You can always answer "what happened and why."
>
> And the minimum footprint: Kubernetes needs a control plane, worker nodes, and a container runtime. Globular needs a binary, etcd, and systemd.
>
> To be clear: Globular is not a Kubernetes replacement for teams already running containers. It's an alternative for teams that don't need the container abstraction.

---

## Slide 11 — AI That Fits Inside

> Here's something different about Globular. Most systems try to add AI on top. Globular is built so AI fits inside.
>
> *(advance fragment)*
>
> The AI follows the same five-phase pattern: observe, diagnose, recommend, execute, verify.
>
> *(advance fragment)*
>
> AI reads the same state you read — etcd, workflows, metrics. It doesn't have a special view. It sees what you see.
>
> AI acts through the same workflows you use. There are no backdoors, no shell commands, no hidden mutations. If AI restarts a service, it goes through a workflow. That workflow has an audit trail.
>
> AI remembers past incidents in persistent memory. So when it sees the same error pattern a second time, it recognizes it and can respond faster.
>
> And AI is constrained. It respects RBAC. Every action is auditable. There's a tiered permission model: some things are automatic, some require human approval, and some are observe-only.
>
> Sixty-five MCP tools. Read-only by default. Every action traceable.

---

## Slide 12 — AI Quote

> AI is no longer magic. It becomes an operator.
>
> It observes. It reasons. It proposes. And if allowed, it triggers workflows. The same workflows you would trigger. Nothing special. Everything visible.
>
> *(pause)*
>
> That's a fundamentally different relationship with AI. It's not a chatbot bolted onto your infrastructure. It's a constrained participant in a system it can actually understand.

---

## Slide 13 — Who Is It For

> So who is Globular for?
>
> *(advance fragment)*
>
> On-premises teams. If you run your own hardware and you want distributed infrastructure without depending on a cloud provider. Full control over your machines, your data, your network.
>
> *(advance fragment)*
>
> Edge and IoT deployments. Globular has a lightweight footprint. No container runtime needed. It runs on any Linux machine with systemd. That means it works on small machines, remote sites, and embedded devices.
>
> *(advance fragment)*
>
> Appliance builders. If you're shipping a product that customers install and run on their own infrastructure. Globular can be the platform inside your appliance — handling service lifecycle, security, observability, and updates.
>
> *(advance fragment)*
>
> And teams that are tired of Kubernetes. If you deploy compiled binaries — Go, Rust, C++ — and you don't need pods, sidecars, or service meshes. You just need something simpler that gives you coordination, state management, and lifecycle automation.

---

## Slide 14 — What Globular Is Not

> Let me be honest about what Globular is not.
>
> *(advance fragment)*
>
> It's not a Kubernetes replacement for teams already running containerized workloads. If you have a container ecosystem that works, keep it.
>
> *(advance fragment)*
>
> It's not designed for polyglot applications with complex dependency trees. Python, Node.js, Java — these benefit from container isolation. Globular is built for compiled, statically-linked binaries.
>
> *(advance fragment)*
>
> It's not a cloud service. You own the machines. You run the platform. That's the point — full control, no lock-in.
>
> *(advance fragment)*
>
> And it's not magic. It's explicit, deterministic, and transparent. What you see is what you get. That's a feature, not a limitation.
>
> *(advance fragment)*
>
> Globular is the right choice when you deploy compiled binaries, want full control over the operating system, and need platform-level coordination without the container abstraction.

---

## Slide 15 — How It Looks

> Let me show you what it actually looks like to use Globular.
>
> You bootstrap your first node with a single command. You tell it which machine to connect to, what domain to use, and which profiles to apply. That's it. In about a minute, you have a running cluster.
>
> Deploying a service is one command. Set the desired version, and the system does the rest — creates a workflow, fetches the package, installs it, configures it, starts it, verifies it.
>
> You can see exactly what happened by listing workflows. Every operation has a run ID, a status, and a duration.
>
> And you can check that everything is aligned with a single repair dry-run. If all services are converged, it tells you. If something is drifted, planned, or unmanaged, it tells you exactly what and where.

---

## Slide 16 — When Things Break

> And when things break — because things always break — you don't guess.
>
> *(advance fragment)*
>
> You run one command, and you see the full picture. PostgreSQL on node one is installed and aligned. PostgreSQL on node two is drifted — installed version doesn't match desired. Redis on node one is planned — desired state was set but installation hasn't happened yet. And monitoring on node three is unmanaged — it's installed but nobody asked for it.
>
> *(advance fragment)*
>
> Four layers. Precise diagnosis. You know exactly what's wrong and why. Not "desired not equal to observed" for three different root causes. The actual root cause, identified by comparing the layers.

---

## Slide 17 — Why Globular (Summary)

> So let me bring it all together.
>
> *(advance each fragment)*
>
> Explicit. No hidden state. No invisible loops. No magic.
>
> Observable. Every action is a workflow. Every workflow is queryable.
>
> Deterministic. Classified failures. Controlled convergence. No guessing.
>
> Lightweight. Native binaries plus systemd. No containers required.
>
> AI-native. Built for AI to reason about — not bolted on after.
>
> Self-hosted. Your machines. Your data. Your control. No cloud lock-in.
>
> That's Globular. A platform where distributed systems become understandable. Because once you understand your system, you can trust it. And once you trust it, you can build anything on top of it.

---

## Slide 18 — Get Started

> If this resonates with you, getting started is straightforward.
>
> The documentation is at globular.io/docs. It includes a getting-started guide that takes you from zero to a running cluster in about fifteen minutes.
>
> The source code is on GitHub at globulario/services. It's open source, Apache 2.0 licensed, built with Go, and gRPC everywhere.
>
> One command to bootstrap. One command to deploy. One command to check that everything is aligned.
>
> Thank you for watching. If you have questions, the documentation covers everything — architecture, operations, security, AI integration, and more. And if you'd like to contribute, the GitHub repository is open.

---

## Tips for Recording

- **Pace**: Read slowly and deliberately. Leave 1-2 second pauses between paragraphs.
- **Fragments**: Advance fragments (press Space or Right Arrow) at the *(advance fragment)* markers.
- **Tone**: Conversational but confident. You're explaining something you built and believe in.
- **Pauses**: The quote slides (3, 12) benefit from a longer pause after the key line. Let it land.
- **Screen**: Present in fullscreen (press F in reveal.js). Resolution 1280x720 matches YouTube HD.
- **Recording**: Use OBS Studio to capture the browser window + your microphone audio.
- **Duration target**: 12-15 minutes feels right for YouTube. Don't rush.
