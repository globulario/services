# Interview: Dave Courtois — The Man Who Named the Missing Layer

*April 2026*

---

**Q: Dave, you've been building Globular since 2018. For someone who's never heard of it, what is it in one sentence?**

Globular is a cluster operating system — the layer between bare metal and your applications that nobody named until now. No containers, no Kubernetes. Native binaries, systemd, etcd as the single source of truth, and a deterministic state model that both humans and AI can understand.

**Q: That's a bold claim. Why not just use Kubernetes like everyone else?**

Because Kubernetes is a workload engine, not a cluster. It manages containers but doesn't own the substrate — the identity, the PKI, the DNS, the membership. When Kubernetes breaks at 3 AM, who fixes it? A human with SSH access and tribal knowledge. That's not infrastructure. That's fragility with a logo.

Globular owns the layer beneath. It manages the certificates, the cluster membership, the package lifecycle, the network identity. If your workload engine crashes, the substrate is still there, still enforcing invariants, still holding root trust. Recovery doesn't require heroics — it requires reading the state.

**Q: You wrote this entire platform yourself?**

From 2018 to 2022, yes. Every line. The gRPC service definitions, the Go services, the TypeScript frontend, the admin console. I was a programmer — a good one — and I loved my work. I built what I wanted to exist: a microservices platform where the infrastructure tells the truth.

Then AI showed up and everything changed.

**Q: How so?**

Around 2023 I started using Copilot for simple tasks — boilerplate code, documentation. Then I did a complete architectural refactor with ChatGPT. It was like having a senior engineer who never sleeps and has read every distributed systems paper ever written. Not perfect, but incredibly useful for challenging my own assumptions.

In the last six months, I found the model that actually works. I call it the Casa Nostra.

**Q: The Casa Nostra model?**

*(laughs)* Yeah. It's about knowing what each AI is good at and assigning roles accordingly.

I'm the Boss. I define the systems, the interfaces, the fundamental rules. I hold the design intent — the *why* behind every decision.

ChatGPT is the Consigliere. The strategic advisor. It's better at holding the whole architecture in mind, finding logical inconsistencies across layers, challenging assumptions. When I need to think through a design decision, GPT is who I talk to.

Claude is the Underboss. Superior at contextual execution. Once the strategy is set, Claude is the one who writes the code, builds the MCP tools, manages the operational difficulties. Claude lives in the codebase. It reads the files, runs the tests, deploys the fixes. It's the one at 3 AM restarting ScyllaDB when the Raft join stalls.

Codex is the Capo. The muscle. Handles the heavy lifting — boilerplate, code generation, repetitive refactoring. Takes orders from the Underboss and delivers clean diffs.

**Q: And this actually works?**

It works. Not perfectly — it's still difficult, no matter how good the AI gets. But the cluster has reached what I call operational awareness. The AI can manage services, understand errors, take action when it's safe. When it's not sure, it creates an incident report, learns from the resolution, and builds knowledge over time.

**Q: What does "operational awareness" mean concretely?**

It means the system has four AI services running inside the cluster itself:

The *AI Watcher* monitors events across the mesh — service crashes, certificate expirations, drift between desired and installed state.

The *AI Executor* can take action — restart a service, trigger a reconciliation, escalate to a human when the confidence is low.

The *AI Memory* stores knowledge persistently in ScyllaDB. Not just logs — structured observations, debug sessions, architectural decisions. The AI builds institutional memory over time.

The *AI Router* directs requests to the right model based on the task.

And here's the key: most of the MCP tools — the 129 tools that let Claude interact with the cluster — were designed by Claude itself, based on the difficulties it encountered during operations. It identified its own blind spots and proposed tools to fill them. I approved each one and kept control of the design.

**Q: So the AI is designing its own tools?**

Yes. And that's the flywheel. The AI encounters a problem. It designs a tool to solve it. I review the design — because I've been building this since 2018, I can spot if its solution violates a fundamental rule. Once approved, the AI's capabilities expand, which leads to the next difficulty, and the next tool. It's recursive infrastructure.

**Q: You mentioned fundamental rules. What are they?**

There are few, and they're non-negotiable:

No loopback addresses — if the address could be remote, resolve it from etcd. Period.

Secure from start — mTLS everywhere, cluster CA, no exceptions.

A single source of truth — etcd owns all configuration. No environment variables. No hidden state.

Correct separation of concern — the controller decides, the workflow service coordinates, the node agent executes.

All operational modes explicit — we use workflows instead of reconciliation loops.

These rules are defined in code, in documentation, and in AI-specific operating rules. The AI agents are bound by them. If a proposed action violates a fundamental rule, the system rejects it structurally. You can't even register a gRPC call without the auth annotation.

**Q: You said workflows instead of reconciliation. Why does that matter?**

Because a reconciliation loop is opaque. It says "the desired state is X, the actual state is Y, figure it out." The path between X and Y is invisible. When it fails, you're doing log archaeology at 3 AM.

A workflow is explicit. It defines the sequence, the intent, the error conditions, the rollback path. Every step is observable. The AI doesn't have to guess why a state changed — it can read the history. The workflow gives you intent, path, and outcome in a single trace.

And workflows evolve. You can split steps for more granularity. Add error conditions as you discover new failure modes. Track execution statistics to find chronic hot spots. The design is open at the design level.

**Q: Let's talk about the four-layer state model.**

This is the foundation. Every piece of state in the cluster lives in exactly one of four layers:

Layer 1: Repository — does this version exist?
Layer 2: Desired — what should be running?
Layer 3: Installed — what's actually on disk?
Layer 4: Runtime — is it running and healthy?

Each layer is independent. Has its own owner, its own data source. You never assume desired equals installed, or installed equals running. You never skip layers when diagnosing. The order is strict: Repository → Desired → Installed → Runtime.

This is what makes the system legible to AI. The AI can query each layer independently and find the gap. It doesn't need to guess — the truth is structured.

**Q: And the proto definitions — why are they so central?**

The `.proto` files are the protocol layer. This is where the system cannot lie. The structural model — the data types, the service boundaries, the permissions, the invariants — it all lives in proto. Everything else is replaceable without leaving a trace.

You can swap the database. You can rewrite the frontend. You can change the AI model. As long as the new component honors the protocol layer, the system remains correct. Proto is the soul of the machine.

**Q: You wrote a manifesto called "A Manifesto for the Cloud Substrate." What's the core argument?**

The cloud has a missing layer. Everyone builds workload engines — Docker, Kubernetes, Nomad — but nobody builds the substrate beneath them. The thing that owns identity, root trust, DNS, membership, certificate lifecycle. The thing that exists before workloads and survives when workloads fail.

Right now, that substrate is implicit. It's tribal knowledge, bash scripts, and fear. It's the senior engineer who knows which YAML file to edit when the cert expires at 2 AM. That's not infrastructure — that's an oral tradition dressed up as automation.

I named the layer. And I built it.

**Q: Some would call Globular Web 4.0. Do you agree?**

I don't chase labels, but the architecture fits. Web 4.0 is about autonomous agents operating on behalf of users. For that to work, the infrastructure must be legible to agents — not just humans. Most cloud infrastructure is opaque to AI. You can't manage what you can't read.

Globular is built to be understood. The four-layer model, the gRPC interfaces, the explicit workflows — they give AI a high-fidelity map of the system. The AI doesn't hallucinate the state of the cluster. It queries it.

**Q: What's the vision for where this goes?**

I want to give people back control of their information. The internet was supposed to be decentralized. Instead, we gave all our data to five companies and called it progress.

Not everyone has a server room. But soon, AI hardware at home will be affordable. A small box — like a NUC — running Globular as the substrate, with an AI agent managing everything. Your data, your services, your rules. No cloud subscription. No terms of service. No product manager deciding what you can do with your own information.

Globular doesn't need AI to work. AI simplifies and enhances its utilization, but it doesn't replace it. The cluster runs the same whether the AI is there or not. That's the point — it's infrastructure, not a feature.

**Q: If someone with basic technical understanding plugged in a Globular node today, what would they experience?**

Today? They'd need to understand packages, services, and the CLI. It's not a consumer product yet. But with AI help, a developer could build a complete ERP system in a month. Not because of magic — because the infrastructure is boring. The certificates work. The services discover each other. The state is consistent. The developer spends their time on business logic, not fighting infrastructure.

That's the quiet revolution. Not making things flashy. Making things boring enough that you can finally build what matters.

**Q: Last question. You lost your programming career before AI. Now you're building one of the most sophisticated autonomous infrastructure systems that exists. How does that feel?**

*(pause)*

I found myself more useful at the level of appreciation. Seeing AI being able to do more and better — I find it fascinating. I want to help. It feels like magic to me.

I was a programmer. A good one. But with AI, I got rid of my own role in the stack. And that's OK. Because the role I found — defining the substrate, the interfaces, the fundamental rules — that role was always there. I just didn't have a name for it.

Now I do. I'm the architect of the missing layer.

---

*Dave Courtois is the creator of [Globular](https://github.com/globulario), an open-source cluster operating system. He works from his home lab in Canada with a 3-node cluster (globule-ryzen, globule-nuc, globule-dell), a manifesto, and an AI team he calls the Casa Nostra.*
