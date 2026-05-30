---
name: backend-design-first
description: Use this skill when implementing or modifying backend features in this Go EventHub repository. It enforces a design-first workflow, Java-Go parity checks, docs/ai updates, and Go quality gates.
---

# Purpose

This skill keeps the Go port educational, reviewable, and aligned with the Java EventHub business contract.

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

## Step 1: Understand And Scope
Before editing code, summarize:
- goal
- Java behavior or document source being mirrored
- scope / out of scope
- impacted modules
- important assumptions
- risks

## Step 2: Design Before Implementation
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

## Step 3: Document The Design
Before writing the design note, read and follow:
- `docs/templates/design-template.md`

Then create or update a design note under:
- `docs/ai/design/`

Suggested filename:
- `YYYY-MM-DD-<topic>-design.md`

Keep the same section order as the template unless the task clearly needs a different structure. If the structure changes, explain why in the document.

## Step 4: Implement The Smallest Viable Slice
Make the smallest change set that closes the target loop.

Respect the Go layering boundary:
- `handler -> service -> repository -> sqlc/database`

Do not let handlers access database/sql, sqlc queries, or transaction handles directly.
Do not use `panic` for business errors.
Do not put roles, email, username, or user status into JWT claims.

## Step 5: Verify
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

## Step 6: Document Implementation
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

## Step 7: Document Java-Go Parity
Before finishing, check whether the change affects Java-Go parity.

Read and update when applicable:
- `docs/ai/parity/java-go-parity-matrix.md`

Update the parity matrix when the task touches any of these:
- API paths, methods, request fields, response fields, status codes, pagination semantics, or OpenAPI contracts
- error codes, error messages, validation behavior, or business failure semantics
- database tables, columns, indexes, unique constraints, enum/status values, migrations, sqlc queries, or repository behavior
- business workflows, state machines, concurrency behavior, idempotency behavior, cache behavior, or transaction boundaries
- authentication, authorization, JWT claims, auth sessions, refresh tokens, or security boundaries
- testing strategy, fixtures, Java test parity, migration tests, or contract tests
- intentional Go-only implementation choices that preserve Java business semantics while using different structure

Each parity update should record:
- Java source or document reference
- Go target file, package, or document
- current status, such as `已对齐`, `规则已初始化`, `待迁移`, `待决策`, or `不适用`
- short reason for any intentional difference
- follow-up design, implementation note, or ADR link when more detail exists

If no parity update is needed, say why in the implementation note or final verification summary.

## Step 8: ADR When Needed
If the task introduces a meaningful architectural or engineering trade-off, first read:
- `docs/templates/adr-template.md`

Then add or update:
- `docs/ai/adr/`

Examples:
- choosing sqlc as the persistence code generation boundary
- choosing optimistic locking vs database conditional update
- choosing synchronous flow vs event-driven flow
- choosing monolith package boundary or service split
- changing JWT claim boundaries or auth session semantics

# Output Format After Completion

Always end with:
1. Design summary
2. Code change summary
3. Rationale
4. Alternatives
5. Risks / follow-ups
6. Updated documentation files
7. Verification results
