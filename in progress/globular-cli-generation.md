# Globular CLI — Generation System

## Purpose

`globular-cli` is the canonical tool for generating and managing services in Globular.

It enforces:
- consistent architecture
- deterministic structure
- separation between generated and handwritten code

It exists to prevent ad hoc service design and ensure all services follow Globular conventions.

---

## Core Principle

**Proto is the source of truth.**

- `.proto` defines contracts and data structures
- generated code reflects proto
- handwritten code implements behavior
- persistence is an adapter layer

---

## Workflow Overview

1. Define service intent
2. Create and review `.proto`
3. Generate structure with CLI
4. Implement handwritten logic
5. Build and validate
6. Publish and install

---

## Generation Commands

### Generate service

```bash
globular-cli generate service --proto <file> --lang <go|csharp> --out <dir>
```

Optional:
```bash
--store <none|sqlite|postgres|mongodb>
--persistence <native|ef>
--dry-run
--json
```

---

### Generate full stack

```bash
globular-cli generate all --proto <file> --lang <lang> --out <dir>
```

---

### Inspect proto before generation

```bash
globular-cli generate inspect --proto <file> --json
```

---

### Generate EF persistence (C# only)

```bash
globular-cli generate ef --proto <file> --store postgres --out <dir>
```

---

### Generate mapping layer

```bash
globular-cli generate mapping --proto <file> --out <dir>
```

---

## Language Rules

### Go
- preferred for infrastructure services
- minimal persistence abstraction
- direct logic implementation

### C#
- preferred for schema-driven services
- EF allowed as persistence layer only

---

## Persistence Rules

- `store` defines database backend
- `persistence` defines access style

Examples:

| Case        | store     | persistence |
|-------------|-----------|-------------|
| No storage  | none      | native      |
| SQL + EF    | postgres  | ef          |
| SQL manual  | postgres  | native      |

---

## Generated File Rules

Generated files:
- `*.generated.go`
- `*.generated.cs`

These:
- can be overwritten
- must not contain business logic

Handwritten files:
- contain logic
- must not be overwritten

---

## Strict Rules

1. Always define and approve proto first
2. Never create service structure manually if CLI exists
3. Never edit generated files manually
4. Business logic only in handwritten files
5. EF is persistence only, not architecture
6. Mapping must be explicit when needed
7. Generated code must remain disposable

---

## AI Interaction Model

AI assistants must:

- propose proto before generation
- choose language and storage from allowed options
- use CLI commands for structure
- implement only handwritten code
- follow generation rules strictly

---

## Example — Go service

```bash
globular-cli generate all \
  --proto ./proto/market.proto \
  --lang go \
  --store none \
  --out ./services/market
```

---

## Example — C# EF service

```bash
globular-cli generate all \
  --proto ./proto/catalog.proto \
  --lang csharp \
  --store postgres \
  --persistence ef \
  --out ./services/catalog
```

---

## Summary

`globular-cli generate` is the only approved way to create services.

It ensures:
- consistency
- predictability
- AI compatibility
- maintainable architecture
