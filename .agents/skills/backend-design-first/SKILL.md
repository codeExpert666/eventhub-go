---
name: backend-design-first
description: Use this skill when implementing or modifying backend features in this Go EventHub repository. It enforces a design-first workflow, Java-Go parity checks, docs/ai updates, and Go quality gates.
---

# Purpose

This skill keeps the Go port educational, reviewable, and aligned with the Java EventHub business contract.
It is intentionally lightweight: AGENTS.md is the source of detailed project rules, while this file is the execution checklist.

Use this skill for:
- new backend features
- API design or API contract changes
- database schema, sqlc query, or migration changes
- cache / concurrency / idempotency changes
- order, inventory, payment, notification, auth, and permission logic
- refactors that affect domain boundaries, layering, error handling, or engineering structure
- Java-Go parity decisions

Do not use this skill for:
- tiny typo-only changes
- pure formatting fixes
- trivial comment edits that do not change behavior or documentation policy

# Required Workflow

## Step 1: Scope
Before editing code, summarize:
- goal
- Java behavior or document source being mirrored
- scope / out of scope
- impacted modules
- important assumptions
- risks

## Step 2: Preflight Checks

Before design and implementation, check the relevant AGENTS.md rules instead of duplicating them here:
- layering and package layout: AGENTS.md sections 6 and 7
- HTTP handler / DTO boundaries: AGENTS.md sections 7.5 and 7.6
- service Command / Query / Result boundaries: AGENTS.md section 7.7
- dependency and interface rules: AGENTS.md section 7.8
- API, error, data, and JWT constraints: AGENTS.md section 8
- verification expectations: AGENTS.md sections 9 and 11

The design note must state:
- touched layers / packages, and important packages intentionally not touched
- DTO, service contract, repository, and dependency-boundary changes when applicable
- any deviation from AGENTS.md, with ADR follow-up when the deviation is architectural

The implementation note must state:
- file moves and package boundary changes
- DTO -> Command / Query -> Result / domain mappings when applicable
- concrete types, interfaces, and test doubles introduced, retained, removed, or intentionally avoided
- whether structure debt remains

## Step 3: Design Before Implementation
Produce a concise design that covers:
- domain objects
- API endpoints or message contracts
- error codes and failure semantics
- data model, indexes, and migration impact
- state transitions if any
- concurrency / idempotency / cache implications
- security / authorization implications
- Java-Go parity expectations
- testing strategy

## Step 4: Document Design
Before writing the design note, read and follow:
- `docs/templates/design-template.md`

Then create or update a design note under:
- `docs/ai/design/`

Suggested filename:
- `YYYY-MM-DD-<topic>-design.md`

Keep the same section order as the template unless the task clearly needs a different structure. If the structure changes, explain why in the document.

## Step 5: Implement
Make the smallest change set that closes the target loop.

Core boundaries to preserve:
- `handler -> service -> repository -> sqlc/database`
- concrete types by default; interfaces only for stable boundaries or capability-owner packages
- `internal/app/bootstrap` wires objects; `internal/http/router` binds routes
- handlers do not access database/sql, sqlc queries, or transaction handles directly
- service does not import `repository/mysql`, sqlc generated packages, or `database/sql`
- business errors use explicit errors, not `panic`
- JWT claims do not include roles, email, username, or user status

## Step 6: Verify
Run the smallest relevant verification that is feasible in the current repository:
- `gofmt` for changed Go files
- `go test ./...` when a Go module exists
- `go vet ./...` when a Go module exists
- `golangci-lint run` when configured or available
- `sqlc generate` when SQL or sqlc config changes
- migration tests when migrations change
- OpenAPI validation when API contracts change
- `make test` when Makefile conventions exist

If a command is not applicable, record the reason.

## Step 7: Document Implementation
Before writing the implementation note, read and follow:
- `docs/templates/implementation-note-template.md`

Then create or update an implementation note under:
- `docs/ai/implementation/`

Suggested filename:
- `YYYY-MM-DD-<topic>-implementation.md`

The implementation note must answer:
1. What problem was solved
2. Why this design was chosen
3. What alternatives were considered
4. Why alternatives were not used
5. What validation was performed
6. What limitations / next steps remain

## Step 8: Document Java-Go Parity
Before finishing, check whether the change affects Java-Go parity.

Read and update when the change touches API contracts, errors, data, workflows, auth/security, tests, repository behavior, or intentional Go-only structure:
- `docs/ai/parity/java-go-parity-matrix.md`

Each parity update should record:
- Java source or document reference
- Go target file, package, or document
- current status, such as `已对齐`, `规则已初始化`, `待迁移`, `待决策`, or `不适用`
- short reason for any intentional difference
- follow-up design, implementation note, or ADR link when more detail exists

If no parity update is needed, say why in the implementation note or final verification summary.

## Step 9: ADR When Needed
If the task introduces a meaningful architectural or engineering trade-off, read:
- `docs/templates/adr-template.md`

Then add or update:
- `docs/ai/adr/`

# Output Format After Completion

Use the completion summary format in AGENTS.md section 12.
