# Globular CLI — MCP Help System for AI

## Purpose

This document defines how AI agents, such as Claude, should interact with `globular-cli` using MCP.

The goal is to ensure:
- predictable command usage
- correct flag selection
- no hallucinated commands
- consistent workflows

---

## Available MCP Tools

### 1. `globular_cli.help`

Returns help for a CLI command.

#### Request
```json
{
  "command_path": "generate service",
  "format": "json"
}
```

#### Response
Includes:
- description
- required flags
- optional flags
- allowed values
- examples
- rules
- follow-up commands

---

### 2. `globular_cli.workflow`

Returns a full workflow for a task.

Example:
```json
{
  "task": "create_service"
}
```

Response:
- step-by-step instructions
- required commands
- expected outputs

---

### 3. `globular_cli.rules`

Returns core Globular AI rules.

Includes:
- proto is source of truth
- use CLI for generation
- do not edit generated files
- EF is persistence only
- propose before executing

---

### 4. `globular_cli.examples`

Returns validated CLI examples.

---

## Standard Workflow — Create Service

### Step 1 — Retrieve rules

Call:
```json
globular_cli.rules
```

---

### Step 2 — Retrieve workflow

```json
globular_cli.workflow
```

Task:
```json
{
  "task": "create_service"
}
```

---

### Step 3 — Propose service

AI must propose:
- name
- purpose
- language
- storage
- persistence style
- proto

AI must wait for user approval.

---

### Step 4 — Inspect proto

```bash
globular-cli generate inspect --proto <file> --json
```

---

### Step 5 — Get command help

```json
globular_cli.help
```

```json
{
  "command_path": "generate service"
}
```

---

### Step 6 — Generate service

Example:

```bash
globular-cli generate all \
  --proto ./proto/service.proto \
  --lang go \
  --store none \
  --out ./services/service
```

---

### Step 7 — Implement logic

Rules:
- edit handwritten files only
- do not modify generated files

---

### Step 8 — Build and validate

```bash
go build ./...
go test ./...
```

or

```bash
dotnet build
dotnet test
```

---

### Step 9 — Publish

```bash
globular-cli package publish --path <service>
```

---

### Step 10 — Install

```bash
globular-cli service install --name <service>
```

---

### Step 11 — Observe via MCP

Use MCP tools to verify:
- service health
- cluster state
- rollout success

---

## Command Selection Rules

AI must:
- always check help before using a command
- only use commands returned by the help system
- never invent commands
- use only allowed flag values

---

## Safety Rules

AI must not:
- modify generated files
- bypass CLI for structure
- deploy to production without explicit approval
- execute destructive commands without confirmation

---

## Output Behavior

AI must:
- explain actions before executing
- show commands clearly
- report results after execution
- stop and ask when uncertain

---

## Summary

AI must act as a structured operator:

- propose
- verify
- generate
- implement
- validate
- deploy
- observe

All actions must be:
- CLI-driven
- rule-compliant
- predictable
