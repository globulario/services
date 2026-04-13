# The Globular Talk

A five-part introduction to Globular — what it is, how it works, and why it's different. Each part is a short audio talk (~3-5 minutes) with a full transcript below.

Listen in order for the complete picture, or jump to the part that interests you.

---

## Part 1: Why Globular Exists

*The origin story — frustration with opaque systems, and the question that started everything.*

<audio controls preload="none" style="width:100%">
  <source src="../talk/audio/Why%20Globular%20exists.mp3" type="audio/mpeg">
</audio>

??? note "Transcript"

    Globular did not start as a product. It started as frustration.

    Frustration with systems that are powerful, but hard to reason about. Frustration with infrastructure that feels indirect. You ask for something... and somewhere, somehow, something eventually makes it happen.

    So I asked a simple question: What if a cluster behaved exactly the way you describe it — explicitly, deterministically, and transparently?

    That is where Globular comes from.

    Globular is a distributed system platform. It has the same broad scope people associate with Kubernetes, but it is built on a very different philosophy.

    Instead of hiding complexity behind layers of abstraction, Globular exposes it, organizes it, and makes it predictable.

    At the core, everything is driven by workflows. Not controllers guessing what to do. Not invisible reconciliation loops. Workflows.

    And this changes everything. Because now, the system is not merely reacting. It is executing.

    There is a source of truth. And it is centralized. That source of truth is etcd. The cluster controller decides what needs to happen. The workflow engine orchestrates how it happens. The node agents execute the work on each machine.

    Each part has a clear responsibility. No ambiguity. No overlap.

    Globular does not try to be clever. It tries to be deterministic. Every change in the system is traceable, observable, and reproducible.

    And because everything is explicit, AI can reason about the system. Not as a black box. But as something that can understand what the cluster is doing, why it is doing it, and what went wrong.

    So if I had to summarize Globular in one sentence: It is a platform where distributed systems become explicit, observable, and controllable. Not hidden. Not magical. Just understandable.

    Because once you understand your system, you can trust it. And once you trust it, you can build anything on top of it.

---

## Part 2: How Globular Works

*Inside the architecture — the 4-layer state model, workflows, and the three actors.*

<audio controls preload="none" style="width:100%">
  <source src="../talk/audio/How%20Globular%20works.mp3" type="audio/mpeg">
</audio>

??? note "Transcript"

    So now that we understand why Globular exists, let's open it up and look inside. Not from a marketing perspective, but from how it actually works.

    Globular is built around a very simple idea: Everything that happens in the system is explicit. Nothing is hidden. Nothing is implied. Nothing "just happens."

    At the center of the system, there is a single source of truth. And that source of truth is etcd.

    If you want to understand Globular, you don't start with services, you don't start with nodes — you start with state.

    Globular organizes state into four layers: Artifact, Desired, Installed, Runtime.

    Think of it as a pipeline of reality.

    First, you have the artifact. This is what exists in the repository. Packages. Services. Applications. What *can* exist.

    Then, you have the desired state. This is what the cluster should look like. Stored in etcd. Defined explicitly. What *should* exist.

    Then comes the installed state. What is actually present on nodes. What has been deployed. What *does* exist.

    And finally, runtime. Health. Metrics. Real-time behavior. What *is happening* right now.

    This is the core of Globular. Not containers. Not pods. State... and convergence.

    Now the real question becomes: How does the system move from desired to installed?

    The answer is workflows.

    In Globular, nothing changes state without a workflow. No background magic. No invisible loops. Every action is defined. A workflow is a sequence of steps. Each step has an actor, an action, and a result.

    There are three key actors. The cluster controller — it watches the desired state and decides what must happen. The workflow service — it executes the steps, one by one. The node agent — it runs on each machine and performs the actual work.

    Each part has a single responsibility. No overlap. No confusion.

    So if you remember one thing from this: Globular is not about managing infrastructure. It is about controlling state transitions. Explicitly. Deterministically. Observably.

    And once you understand that, the whole system becomes simple.

---

## Part 3: Operating Globular

*Day-to-day operations — deploying, debugging, and thinking in state transitions.*

<audio controls preload="none" style="width:100%">
  <source src="../talk/audio/Operating%20Globular.mp3" type="audio/mpeg">
</audio>

??? note "Transcript"

    So now that we understand how Globular works, let's stop talking about the system and start using it.

    Because a platform only matters if you can operate it. And this is where Globular is different. You don't need to learn ten tools. You don't need to understand hidden layers. You follow the system.

    Let's walk through a real flow. From nothing to a running application.

    Step one: you install Globular. This gives you a node. A machine that can participate in the cluster.

    Step two: you start the node. The node connects to the cluster, registers itself, and begins reporting its state. At this point, you already have a system.

    Step three: deploy an application. In Globular, deploying is not a mystery. It's a command. What this does is simple. It takes your application, registers it in the system, updates the desired state, and triggers a workflow.

    That workflow now drives everything. The controller sees the change. The workflow service executes the steps. The node agent installs the application. And then the system verifies it.

    That's it. No hidden scheduler. No invisible behavior. Just a defined sequence.

    Now here's what's important. You didn't "run a deployment". You changed the state. And the system moved itself to match that state.

    Now let's say something goes wrong. The application doesn't start. What do you do? You look at the workflow. You inspect the execution. You see which step failed, what was attempted, what the system observed.

    You don't guess. You don't dig through logs blindly. You follow the system's own reasoning.

    Everything you do maps to a state change, a workflow, a result.

    After a while, you stop thinking in commands. You start thinking in state transitions. And that's when Globular clicks.

    Because now you're not operating a tool. You're controlling a system.

---

## Part 4: When Things Break

*Failure handling — diagnosis through workflows, drift detection, and convergence.*

<audio controls preload="none" style="width:100%">
  <source src="../talk/audio/When%20Things%20Break.mp3" type="audio/mpeg">
</audio>

??? note "Transcript"

    So far, everything we've seen looks clean. We install, we deploy, we run workflows. But real systems don't live in perfect conditions. Things fail.

    Services don't start. Nodes disappear. Workflows stop halfway.

    And this is where most systems become hard. This is where Globular becomes simple.

    Let's take a real example. You deploy a service... and nothing happens. What do you do?

    You don't restart everything. You don't guess. You don't start digging randomly through logs. You go to the workflow. Because in Globular, the failure is not hidden.

    You open the workflow execution. You see which step ran, which step failed, what the node reported. The system tells you the story. Not just the result — the entire execution.

    Let's say the node-agent failed to install the service. Maybe a dependency is missing. Maybe a binary is not accessible. Maybe a configuration is invalid. You don't infer this. You see it. And now you can act.

    You fix the issue, you re-trigger the workflow, or the system retries automatically. And the system converges again.

    Let's take another case. A node stops reporting. In Globular, you look at the state. You check last heartbeat, controller view, workflow activity. And immediately, you know what's missing.

    Now imagine something more subtle. The system partially succeeds. A service is deployed on one node but not on another. This is where most systems become dangerous. Because they look "almost correct."

    Globular does not accept "almost." It detects drift. Desired state says: "This service must exist everywhere." Installed state says: "It only exists here." Mismatch. And that triggers convergence.

    Globular is not reacting to errors. It is reasoning about state. It knows what should exist. It knows what does exist. And it resolves the difference.

    This is the difference between debugging behavior and controlling outcomes. You are not chasing problems. You are restoring truth.

---

## Part 5: AI + Globular

*How AI fits inside the system — not as a black box, but as a constrained operator.*

<audio controls preload="none" style="width:100%">
  <source src="../talk/audio/AI%20%2B%20Globular.mp3" type="audio/mpeg">
</audio>

??? note "Transcript"

    So far, we've seen something important. Globular is explicit. Globular is observable. Globular is deterministic. And that changes something fundamental.

    Because now, AI can understand the system. Not guess, not infer from logs, not react blindly — understand.

    In most infrastructures today, AI is external. It reads logs. It analyzes metrics. It tries to detect patterns. But it doesn't truly understand what the system is doing. Because the system itself is not explicit.

    In Globular, everything is explicit. State is explicit. Workflows are explicit. Actions are explicit. So AI doesn't have to guess. It can read. It can read the desired state, the installed state, the workflow history, the execution results. And from that, it can reason.

    It can detect drift. It can detect failure patterns. It can understand why something didn't converge. And more importantly, it can act.

    But this is where Globular draws a line. AI is not allowed to do anything. It operates under rules.

    It cannot bypass the source of truth. It cannot mutate state directly. It cannot execute hidden actions. Everything it does must go through the system.

    That means: AI observes, AI reasons, AI proposes, and if allowed, AI triggers workflows. The same workflows you would trigger. Nothing special.

    And that's important. Because now every AI action is visible, traceable, auditable. You can see what it did. You can see why it did it. You can verify the result.

    AI is no longer magic. It becomes an operator.

    In Globular, AI is structured into services. You have watchers that observe the system. You have memory that stores patterns and history. You have routers that influence traffic and decisions. You have executors that trigger actions.

    Each one has a role. And each one is constrained. This is not AI with full control. This is AI inside a system. Bounded by architecture.

    Most systems try to add AI on top. Globular is built so AI fits inside. It doesn't break the model. It uses it.

    So now you have a system that defines its state, executes explicitly, verifies itself, exposes everything — and can be understood by AI.

    Which means you can automate reasoning. Not just actions. Decisions.

    And that changes the role of the operator. You're no longer reacting to the system. You're supervising it.

    So if you look at Globular as a whole: it is a system you can understand, a system you can control, and a system AI can reason about. And that is what makes it different.
