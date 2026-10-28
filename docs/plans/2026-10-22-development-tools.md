# Development Tools Implementation Plan

> Execute the approved three features in this checkout. The user will review and commit; do not stage, commit or push.

**Goal:** Reduce repetitive work for GoFrame developers through a complete module scaffold, useful CRUD query templates and explicit versioned PostgreSQL migrations.

**Architecture:** Keep the existing GoFrame layers and dependencies. Development commands dispatch before loading runtime configuration. The scaffold writes readable module code and edits two dedicated registration functions through the Go AST. Migrations use PostgreSQL transactions, an advisory lock and a checksum ledger; application startup never migrates implicitly.

**Tech Stack:** Go 1.26.10, GoFrame v2.7.2, database/sql, existing lib/pq and gofrs/flock, Go standard AST/template packages.

## Approved scope and contracts

- `make module NAME=article`: generate user-owned CRUD at `/api/v1/article/items`, a numbered SQL file, service registration and protected routes. Refuse conflicting names and existing files. Read the module path from go.mod. Generation requires no database, Redis or external CLI; the resulting code remains compatible with GoFrame generation.
- `model.ListQuery`: pagination, keyword, RFC3339 createdFrom/createdTo, sortBy and sortOrder. Search title with bound parameters and literal LIKE wildcard handling; restrict ordering to known fields and add a stable ID tie-breaker. Count and list use identical owner/filter constraints.
- `make db-new NAME=add_article_status`, `make db-status`, `make db-up`, `make db-baseline`. The ledger stores version, filename, checksum and applied timestamp. Every migration and its ledger row commit together. A lock serializes database upgrades. Changed or missing applied scripts fail verification.
- Preserve `make db-init` as first initialization of the two built-in SQL files, recording both versions. Existing databases without history require explicit baseline after validation of the original schema. Baseline never recreates business tables. No automatic destructive rollback.

## Task 1: CRUD query model and example

**Files:** api/example/v1/example.go; internal/controller/example/example_v1_list.go; internal/logic/example/example.go; new internal/model/list_query.go and internal/library/query/.

1. Write query tests for invalid order/date, date bounds, literal wildcard handling and stable ordering; confirm failures.
2. Implement validation and filtering with the existing ORM, then update example request/controller/service signature.
3. Regenerate service interfaces once the shared work is ready.
4. Verify real PostgreSQL owner isolation and matching list/count with keyword/time/order filters.

## Task 2: Migration core

**Files:** new internal/migration/ with parser, file allocation, runner and tests. Original numbered SQL files remain unchanged.

1. Write tests for filenames, duplicate versions, checksums, complete file creation and SQL transaction boundary handling.
2. Implement Create/CreateWithContent and Runner Status/Init/Up/Baseline.
3. Integration-test fresh initialization, pending execution, no-op rerun, drift refusal, failed-SQL rollback, concurrent execution and explicit baseline in independent temporary schemas.
4. Include PostgreSQL functions/quoted SQL in transaction validation, and reject scripts that could end the runner's transaction.

## Task 3: Module scaffold

**Files:** new internal/scaffold/; internal/bootstrap/modules.go; internal/router/modules.go.

1. Write filesystem tests for names, file collisions, module path detection, AST registration and cleanup after failures.
2. Generate API, Controller, Logic, Service, View and SQL from fixed readable templates.
3. Reuse ListQuery/query.Apply and dao.Store; compile without database-generated DAO files so CLI migrations remain usable immediately after generation.
4. Preflight all changes, lock generation, preserve existing registrations and roll back only files written by this invocation on failure.
5. Verify a generated module in a temporary complete checkout with GoFrame regeneration, build and an authenticated CRUD round trip.

## Task 4: CLI and wiring

**Files:** internal/cmd/cmd.go and new tooling.go/tooling_test.go; internal/bootstrap/app.go; internal/router/routes.go; Makefile; .gitignore; .dockerignore.

1. Write CLI tests showing db-new and module do not require runtime configuration, invalid arguments fail and help succeeds.
2. Add explicit commands and tabular migration status, configurable directory/timeout for database commands.
3. Wire dedicated module registration functions into HTTP service setup and the authenticated route group.
4. Add make targets and ignore coordination lock files. Preserve existing config/data and existing API defaults.

## Task 5: Documentation and acceptance

**Files:** README.md; docs/development.md; new docs/migrations.md; docs/testing.md; .github/workflows/ci.yml where useful.

1. Document a concrete article module walkthrough, fresh and existing database workflows, supported filtering/sorting and expected outputs.
2. Run focused tests first, then go test ./..., go vet ./..., go build ./... and meaningful race checks.
3. Run migration/HTTP tests against task-owned PostgreSQL/Redis, using only isolated *_test databases and temporary schemas/directories.
4. Recheck source references, ignored local secrets and git diff --check. Stop task-owned temporary services. Leave changes uncommitted.

## Verification commands

```sh
go test ./internal/cmd ./internal/migration ./internal/scaffold ./internal/library/query
go test ./...
go vet ./...
go build ./...
go test -race ./internal/migration ./internal/scaffold ./internal/library/query
go test -tags=integration ./internal/migration ./internal/bootstrap -count=1 -timeout=120s
git diff --check
```

Expected: passing tests, no vet/build errors, generated module compiles and its own-user CRUD works; unapplied scripts are shown as pending, successful upgrades become applied, failed upgrades leave no partial schema/ledger change for that version. Cloud services and deployment validation are outside this development-tooling change.
