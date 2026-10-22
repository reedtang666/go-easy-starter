# Go Easy Starter Implementation Plan

> Execution: follow the executing-plans skill, task by task, in the authorized target repository only.

**Goal:** Implement the approved, reusable GoFrame API starter with authentication, all six storage drivers, queues, scheduling, optional WebSocket and runnable deployment documentation.

**Architecture:** Explicit dependency initialization by process role. GoFrame API/controller/service/logic/DAO boundaries, PostgreSQL persistence, Redis authoritative sessions, native vendor storage SDK adapters. Configuration and runtime dependencies belong to this starter.

**Tech Stack:** Go 1.26.10, GoFrame v2.7.2, PostgreSQL, Redis, Asynq, JWT, bcrypt, gorilla/websocket, existing vendor SDKs.

## Tasks

1. **Foundation and configuration** — Create go.mod, internal/config, bootstrap lifecycle, response/context libraries, main.go, setup command and config example. Tests first for missing secrets, explicit environment overrides, idempotent setup and role isolation. Run `go test ./internal/config/... ./internal/library/response/...`.
2. **Token/session core** — Implement the generic token/session library in internal/library/auth. Test expiry, claims, atomic rotation, encrypted replay, replacement, invalid-input non-revocation and legitimate replay revocation. Run targeted unit and Redis tests.
3. **Persistence and users** — Create versioned SQL, DAO/DO/entity and user/auth services. Test registration uniqueness, bcrypt, credential-version invalidation, authorization and transaction boundaries using isolated PostgreSQL integration tests.
4. **Storage** — Create internal/library/storager with local/UCloud/COS/OSS/Qiniu/MinIO. Tests first for path confinement, public/private access, signature expiry, SDK errors, stream closing and unsupported capabilities. Add file ownership service and multipart state with concurrency tests.
5. **Queue and scheduler** — Create internal/queue and scheduler. Test payload validation, delayed enqueue, retry propagation, idempotent effects and independent role startup/shutdown.
6. **HTTP API** — Create api/*/v1, controllers, services and routes. HTTP tests for envelope, validation, CORS, limits, privacy, authentication boundaries and representative user/file/CRUD journeys.
7. **WebSocket** — Add one-time ticket exchange, origin restrictions, single writer, heartbeat/session checks and shutdown. Test ticket replay, expiration and revoked sessions.
8. **Developer/deployment delivery** — Add Makefile, Compose, Docker, systemd, pinned generation configuration, CI and Chinese README/docs with exact success criteria. Validate shell/YAML and executable command paths.
9. **Full verification** — Run `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...`, integration tests when isolated infrastructure is available, dependency checks and source-business scan. Record actual results and cloud verification limits.

For each functional unit: first write behavior tests, verify failure, implement, rerun, then continue. Do not add tests that merely mirror generated code. Do not commit or push without a task-specific user request.
