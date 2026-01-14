# Worktime API Documentation

This document is a precise API reference derived directly from the repository source code. It describes only behavior implemented in the codebase — no behavior is invented.

Generated from code scan on: 2026-01-14

---

## 1. Overview

- Purpose: Worktime is a REST backend for simple time tracking: user registration/login, starting/stopping work sessions tied to projects, admin management of users/projects, and summary reports over date ranges.
- Base URL (mounted in router): `/v1` (health endpoint at root `/health`)
- Tech stack: Go, chi router (`github.com/go-chi/chi/v5`), PostgreSQL (pgx driver), migrations via `goose`, JWT authentication (`github.com/golang-jwt/jwt/v5`).

Configuration relevant env variables (from code):
- `WORKTIME_DB_DSN` / `DATABASE_URL` — Postgres DSN (required in production)
- `WORKTIME_JWT_SECRET` / `JWT_SECRET` — JWT HMAC secret (code contains a default fallback; replace in production)
- `WORKTIME_PORT` / `SERVER_ADDRESS` — server listening port/address
- Rate limiter envs: `LIMITER_RPS`, `LIMITER_BURST`, `LIMITER_ENABLED` (see Environment Differences)

---

## 2. Authentication & Security

- Authentication type: JWT (HMAC-SHA256), bearer token in `Authorization` header.

- Authorization header format:

```
Authorization: Bearer <access_token>
```

- Access token creation (code): `internal/auth.NewJWTManager().CreateToken(userID, email, role)` — token claims (map) include:

  - `user_id` (integer)
  - `email` (string)
  - `role` (string) — code expects `user` or `admin`
  - `exp` (unix timestamp) — expiry. CreateToken uses 24-hour TTL in code.

- Verification: `JWTManager.VerifyToken` parses token into `UserClaims` with fields `Id (user_id)`, `Email`, `Role`, and `Expiry`. Middleware `Authenticate` extracts token with `ExtractBearerToken` and calls `VerifyToken`, placing `*auth.UserClaims` into request context.

- Reset tokens: created by `CreateResetToken(userID, email, role, isActive, ttl)` and include `user_id`, `email`, `role`, `is_active`, and `exp` as MapClaims. The admin handler generates reset tokens with 10-minute TTL. `ParseResetToken` verifies and returns `ResetClaims` (contains `UserID`, `Email`, `Role`). Note: `is_active` is included when creating the token but `ResetClaims` struct does not expose `is_active` in parsing (see Limitations).

- Role-based checks: handlers validate `role` string from claims in multiple places; expected roles are `admin` and `user`.

- Account lockout behavior (implemented in code):
  - On authentication failure, `UserStore.LoginFail` increments `failed_attempts` and updates `last_failed_login`.
  - After a failed login, if `existingUser.FailedAttempts+1 > 4` the `UserStore.Lockout` method is called (sets `is_locked = true`, resets `failed_attempts`).
  - On successful login, `UserStore.Unlock` is called.
  - `TokenHandler.LoginHandler` will reject login when `existingUser.IsLocked` and `time.Since(existingUser.LastFailedLogin.Time) < 24h`.

- Rate limiting: repository contains configuration for limiter (`internal/config.Config.Limiter`) with defaults (`LIMITER_RPS=2`, `LIMITER_BURST=4`, `LIMITER_ENABLED=true`), but there is no code applying a global rate limiter middleware. See Limitations.

---

## 3. Global API conventions

- Response envelope: all handlers use `utils.Envelope` (alias for `map[string]interface{}`) and `utils.WriteJson` to write JSON responses. There is not a single enforced nested `data` shape — handlers use top-level keys such as `error`, `user`, `token`, `result`, `metadata`, `session`, `project`, `report`, etc.

- Error format: `{ "error": "message" }` (most handlers follow this). HTTP status codes used by handlers include 200, 201, 400, 401, 403, 404, 409, 500.

- Pagination / filtering conventions (store layer):
  - Query params: `page` (int, default 1), `page_size` (int, default 50), `sort` (string). Several handlers supply a `SortSafeList` to validate `sort`.
  - Filter validation: `Filter.Validate()` enforces `page` between 1 and 10_000_000 and `page_size` between 1 and 1000; `sort` must be one of safe-list fields (handlers set safe-list per endpoint).
  - Metadata shape (returned by list endpoints):

```json
{ "current_page": 1, "page_size": 50, "first_page": 1, "last_page": 10, "total_records": 500 }
```

---

## 4. Endpoints (derived from `internal/router/routes.go` and handlers)

All endpoints shown exactly as mounted in code. Authentication requirement, allowed roles, query parameters, request body shapes, and example responses are included where they can be inferred from source.

- Base route prefix: `/v1`

### Health

GET /health

- Authentication: not required
- Response: `200 OK` (empty response body in router)

Example:

```
HTTP/1.1 200 OK
```

### Authentication

#### Register

POST /v1/auth/register/

- Authentication: not required
- Request body (JSON):

```json
{ "name": "string", "email": "user@example.com", "password": "string" }
```

- Validation (in code):
  - `email` must match regex `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
  - Email must not already exist (checked via `UserStore.GetUserByEmail`)

- Success: `201 Created`, body: `{ "user": <User object> }` where `User` fields are from the store model (see Data Models).
- Errors: `400` invalid payload or invalid email, `400` duplicate email (handler responds with message `Email is already exists`), `500` internal error.

Example success:

```json
{ "user": { "id": 1, "name": "Alice", "email": "alice@example.com", "role": "user", "is_active": true, "created_at": "..." } }
```

#### Login

POST /v1/auth/login/

- Authentication: not required
- Request body:

```json
{ "email": "user@example.com", "password": "string" }
```

- Behavior (code):
  - Verifies user exists and `IsActive == true`.
  - If `IsLocked` and `time.Since(last_failed_login) < 24h`, login is rejected.
  - On password mismatch: `UserStore.LoginFail` increments failure counter; if `FailedAttempts+1 > 4` then `UserStore.Lockout` is invoked.
  - On success: `UserStore.Unlock` is called and `JWTManager.CreateToken` issues an access token (24h TTL).

- Success: `200 OK` with envelope `{ "token": "<jwt>", "name": "User Name", "role": "user" }`
- Errors: `400` malformed input, `401` unauthorized for bad credentials/locked/inactive, `500` internal.

Example success:

```json
{ "token": "<jwt>", "name": "Alice", "role": "user" }
```

#### Reset password (public endpoint — token in body)

POST /v1/auth/reset-password/

- Authentication: not required (token provided in request body)
- Request body:

```json
{ "token": "<reset-token>", "new_password": "string", "confirm_password": "string" }
```

- Behavior: `UserHandler.HandleResetPassword` parses the reset token using `JWTManager.ParseResetToken` and verifies the `UserID` and `Email` from claims match a DB user; then updates password hash and persists via `UpdatePasswordPlain`.

- Success: `200 OK` `{ "message": "password updated" }`
- Errors: `400` invalid/expired token or password mismatch, `500` internal.

### Authenticated group (middleware.Authenticate applied)

The following endpoints are under the group which uses `app.Middleware.Authenticate` (this sets `auth.UserClaims` in request context). Most handlers call `middleware.GetUser(r)` to read the user.

#### Statuses

GET /v1/statuses

- Authentication: required (middleware run). Handler enforces `user.Role == "admin"` — only admins are allowed.
- Success: `200 OK` `{ "statuses": [ { "id": int, "name": string }, ... ] }`

#### Projects

GET /v1/projects

- Authentication: required
- Success: `200 OK` `{ "count": int, "projects": [ ProjectRow ] }`

POST /v1/projects/

- Authentication: required
- Roles allowed: admin only (handler verifies `user.Role == "admin"`)
- Request body:

```json
{ "name": "Project name", "status_id": 1 }
```

- Validation: `name` must be non-empty; `status_id` must be > 0. Handler checks logged-in user exists and is admin.
- Success: `201 Created` with envelope `{ "message": "project created successfully", "project": <Project> }` — the code populates `store.Project{ProjectName: name, StatusId: status_id}` then calls `projectStore.CreateProject`.
- Errors: `400` validation errors, `401/403` unauthorized/forbidden, `500` internal.

PATCH /v1/project/{id}/

- Authentication: required
- Roles allowed: admin only (handler requires admin role via `middleware.GetUser`)
- Path param: `{id}` numeric — `utils.ReadIdParam` parses/validates
- Request body (partial update):

```json
{ "name": "string (optional)", "status_id": int (optional) }
```

- Validation: at least one of `name` or `status_id` must be present; `name` trimmed non-empty if provided; `status_id` must be positive if provided.
- Success: `200 OK` `{ "message": "project updated successfully" }` on success; `400`/`401`/`403`/`404`/`500` as appropriate.

Notes / Edge cases: see Limitations section for a code-level caveat affecting create/update semantics.

#### Work sessions

Start session

POST /v1/work-sessions/start/

- Authentication: required
- Request body:

```json
{ "project_id": int, "note": "string" }
```

- Validation: `project_id` must be positive.
- Behavior: inserts a `work_sessions` row with `start_at=NOW()`. The store `StartSession` returns `id`, `start_at`, `created_at`.
- Success: `201 Created` `{ "session": <WorkSession object>, "status": "active" }`
- Errors: `400` invalid input. If store returns an error containing `one_active_session_per_user` the handler responds with `400` and message `you already have one active session.Stop it before starting a new sessions`.

Stop session

PATCH /v1/work-sessions/stop/{id}/

- Authentication: required
- Path param: `{id}` session id; `utils.ReadIdParam` used.
- Behavior: `workSessionStore.StopSession(ctx, sessionId, user.Id)` — updates the `end_at = NOW()` only if `end_at IS NULL` and `user_id` matches.
- Success: `200 OK` `{ "message": "session stopped", "session_id": <id> }`
- Errors: `400` invalid id, `401` unauthorized, `404` if no matching active session, `500` internal.

List sessions

GET /v1/work-sessions/list/

- Authentication: required
- Query params (inferred from handler):
  - `page` (int, default 1)
  - `page_size` (int, default 50)
  - `search` (string, optional) — searched against project name, user name, user email, and note
  - `active` (bool string `true`/`false`, optional) — filter active/inactive
  - `project_id` (int, optional)
  - `user_id` (int, optional) — admin-only; non-admins have `user_id` forced to their own id

- Pagination: `WorkSessionFilter.Validate()` used; metadata returned by `store.CalculateMetadata`.
- Success: `200 OK` `{ "result": [WorkSessionRow], "metadata": { ... } }`

Summary report

GET /v1/work-sessions/reports/

- Authentication: required
- Query params (required): `from`, `to` — supported formats: RFC3339 or `YYYY-MM-DD` (handler attempts both).
- Optional query params: `project_id` (int), `user_id` (int, admin only).
- Behavior: non-admins are forced to `user_id=self`. Admins may specify `user_id` or omit to get all users. The handler constructs `store.SummaryRangeFilter{FromDate, ToDate, ProjectID?, UserID?}` and calls `GetSummaryReport`.
- Success: `200 OK` `{ "report": <SummaryReport> }` where `SummaryReport` contains `from`, `to`, `filters`, `overall`, `users`, and `projects` sections (see Data Models).
- Errors: `400` missing/invalid `from`/`to`, invalid integers, `401` unauthorized, `500` internal.

#### Users & Admin

Update user

PATCH /v1/users/{id}/

- Authentication: required
- Roles / permissions: admin may update any user; non-admins may only update their own user record (handler verifies this).
- Request body (all fields optional):

```json
{
  "name": "string",
  "email": "email@example.com",
  "old_password": "string",
  "new_password": "string",
  "role": "user|admin",    // admin-only
  "is_active": true|false   // admin-only
}
```

- Validation & behavior:
  - Non-admins attempting to change `role` or `is_active` receive `403 Forbidden`.
  - To change password both `old_password` and `new_password` must be supplied; `old_password` is verified with the stored hash.
  - Email uniqueness is enforced via `GetUserByEmail`.
  - Success: `200 OK` `{ "user": <User> }` with updated user record.

Admin: list users

GET /v1/admin/users/

- Authentication: required (admin only)
- Query params: `search`, `page`, `page_size`, `sort` (safe-list: `id`, `email`, `name`), `is_active` (bool), `is_locked` (bool)
- Success: `200 OK` `{ "result": [User], "metadata": Metadata }`

Admin: generate reset token

POST /v1/admin/reset-tokens/

- Authentication: required (admin only)
- Request body: `{ "email": "user@example.com" }` — email normalized to lower-case in code
- Behavior: finds user by email; if found calls `JWT.CreateResetToken(user.Id, user.Email, user.Role, user.IsActive, ttl)` with `ttl = 10 * time.Minute` and returns token string and expiry timestamp formatted with `time.RFC3339`.
- Success: `200 OK` `{ "reset_token": "<token>", "expires_at": "RFC3339 timestamp" }`

---

## 5. Core resources (models used in API)

Models are defined in `internal/store/*.go`. Below are the fields and where they are used in API responses.

### User

Source: `internal/store/user_store.go`

| Field | Type | Description | Required |
|---|---:|---|---:|
| `Id` | int64 | primary identifier | yes |
| `Name` | string | display name | yes |
| `Email` | string | email address | yes |
| `PasswordHash` | password (internal) | password hash (not returned in API) | internal |
| `Role` | string | `user` or `admin` | yes |
| `IsActive` | bool | whether account is active | yes |
| `CreatedAt` | time.Time | created timestamp | yes |
| `UpdatedAt` | time.Time | updated timestamp | optional |
| `IsLocked` | bool | account lock flag (internal, not exposed by most endpoints) | internal |

Note: API responses expose `id`, `name`, `email`, `role`, `is_active`, and timestamps; internal fields for lockout are not generally returned by endpoints (list users does include `is_locked` field via SQL projection).

### Project / ProjectRow

Source: `internal/store/project.go`

Project (used in create response):

| Field | Type | Description | Required |
|---|---:|---|---:|
| `ProjectId` | int64 | id (returned on create) | yes |
| `ProjectName` | string | project name | yes |
| `StatusId` | int64 | FK to statuses | yes |

ProjectRow (used in list):

| Field | Type | Description |
|---|---:|---|
| `id` | int64 | project id |
| `name` | string | project name |
| `status` | object | `{ "id": int, "name": string }` project status |

### Status

Source: `internal/store/statuses.go`

| Field | Type | Description |
|---|---:|---|
| `id` | int64 | status id |
| `name` | string | status name |

### WorkSession / WorkSessionRow / WorkSessionResponse

Source: `internal/store/work_session.go`

WorkSession (used internally when creating):

| Field | Type | Description |
|---|---:|---|
| `Id` | int64 | session id |
| `UserId` | int64 | user id |
| `ProjectId` | int64 | project id |
| `StartAt` | time.Time | start timestamp |
| `EndAt` | *time.Time | nullable end timestamp |
| `Note` | string | free text note |

WorkSessionResponse (used in rows):

| Field | Type | Description |
|---|---:|---|
| `id` | int64 | session id |
| `start_at` | time.Time | start timestamp |
| `end_at` | *time.Time | nullable |
| `note` | string | note |
| `created_at` | time.Time | created timestamp |

WorkSessionRow (list row):

| Field | Type | Description |
|---|---:|---|
| `user` | object | `{ user_id, name, email, is_active }` |
| `project` | object | `ProjectRow` (nested) |
| `sessions` | object | `WorkSessionResponse` |
| `status` | string | derived `active` or `inactive` |

Summary report models: `SummaryReport`, `OverallSummary`, `UserSummary`, `ProjectSummary` — see `internal/store/work_session.go` for exact JSON keys and structures returned by the report endpoint.

---

## 6. Role-Based Access Control (quick table)

| Endpoint | Auth required | Admin | Regular user |
|---|---:|---:|---:|
| GET /health | No | — | — |
| POST /v1/auth/register/ | No | — | — |
| POST /v1/auth/login/ | No | — | — |
| POST /v1/auth/reset-password/ | No (token in body) | — | — |
| GET /v1/statuses | Yes | Yes | Forbidden (handler enforces admin) |
| GET /v1/projects | Yes | Read | Read |
| POST /v1/projects/ | Yes | Create | Forbidden |
| PATCH /v1/project/{id}/ | Yes | Update | Forbidden |
| PATCH /v1/users/{id}/ | Yes | Update any | Update self only |
| GET /v1/admin/users/ | Yes | List | Forbidden |
| POST /v1/admin/reset-tokens/ | Yes | Create | Forbidden |
| POST /v1/work-sessions/start/ | Yes | Start own | Start own |
| PATCH /v1/work-sessions/stop/{id}/ | Yes | Stop if user matches | Stop own |
| GET /v1/work-sessions/list/ | Yes | Can filter by user_id | Only own sessions (user_id forced) |
| GET /v1/work-sessions/reports/ | Yes | Admin may request other users or all; non-admin forced to self | Non-admin forced to self |

---

## 7. Environment Differences

- Development behavior (from `main.go` and `internal/config`):
  - `godotenv.Load()` is called in `main.go` to support local `.env` files (development convenience).
  - `internal/config.Load()` defaults: `ENV=development`, `LIMITER_ENABLED=true`, `LIMITER_RPS=2`, `LIMITER_BURST=4`. These are only configuration defaults; enabling does not automatically apply rate limiting unless code adds a limiter middleware (not present in current codebase).

- Production requirements:
  - `WORKTIME_DB_DSN` / `DATABASE_URL` must be correctly set for DB connectivity.
  - `WORKTIME_JWT_SECRET` / `JWT_SECRET` should be set to a secure, non-default value; code includes a fallback default for convenience which must be replaced for production.

---

## 8. Limitations & Code-level caveats (explicitly observed in source)

These are concrete limitations and caveats found in the code; they are not assumptions.

1. Rate limiting not applied: `internal/config` exposes limiter configuration, but there is no middleware wired to enforce rate limits.

2. Reset token fields mismatch: `CreateResetToken` includes an `is_active` claim when creating reset tokens, but `ResetClaims` used in `ParseResetToken` does not expose an `IsActive` field; the handler only reads `UserID` and `Email` from parsed claims.

3. Account lockout flow implemented in DB layer and used in `TokenHandler.LoginHandler` (see Authentication & Security), but details of lockout timing/thresholds are enforced by stored fields and handler checks as described above.

4. Minor inconsistencies in response envelopes across handlers: top-level keys differ between endpoints (e.g., `result` vs `projects` vs `project` vs `user`) — clients must handle per-endpoint shapes.

5. Error message and status variances: some handlers return `400` for duplication errors (register) rather than `409`.

6. Implementation notes affecting correctness: while deriving the API we noticed a few code-level issues that could impact behavior (these are recorded here because they are present in source):
   - The existing `docs` file previously noted two bugs; confirm by running/integration tests if you plan to rely on project create/update flows. (If you want, I can open the files and implement small fixes — ask to proceed.)

---

## 9. Where to look in the code

- Router and endpoints: [internal/router/routes.go](internal/router/routes.go)
- Handlers: [internal/api/](internal/api/) — `user_handler.go`, `token_handler.go`, `project_handler.go`, `status_handler.go`, `work_session_handler.go`
- Auth/JWT: [internal/auth/jwt.go](internal/auth/jwt.go)
- Middleware: [internal/middleware/middleware.go](internal/middleware/middleware.go)
- Store models and DB access: [internal/store/](internal/store/)
- Config defaults: [internal/config/config.go](internal/config/config.go)
- Main program: [main.go](main.go)

---

If you'd like, next steps I can take (pick one):
- Produce a compact OpenAPI 3.0 spec (YAML) generated from these derived shapes.
- Add `curl` examples for the most-used endpoints (login, start/stop session, report).
- Implement small fixes for the noted code-level issues (I can update handlers/store with tests).
# Worktime API Documentation

## Overview

Worktime is a RESTful backend for simple time-tracking: users can register, log in, start/stop work sessions on projects, and admins can manage users/projects and generate summary reports.

**Base URL (local dev):** `http://localhost:4000/v1`

Configuration notes:
- Port: `WORKTIME_PORT` (default 4000)
- Database DSN: `WORKTIME_DB_DSN` (required)
- JWT secret: `WORKTIME_JWT_SECRET` (fallback default present in code; do not use in production)

Tech stack: Go, chi router, PostgreSQL (pgx), goose migrations, JWT (HMAC-SHA256)

---

## Table of contents
1. [Authentication](#authentication)
2. [Response format](#response-format)
3. [Error handling](#error-handling)
4. [Pagination & filtering](#pagination--filtering)
5. [Endpoints](#endpoints)
   - Health
   - Authentication
   - Users
   - Projects
   - Statuses
   - Work sessions & Reports
6. [Data models](#data-models)
7. [Role-based access control](#role-based-access-control)
8. [Rate limiting & security](#rate-limiting--security)
9. [Development vs Production](#development-vs-production)
10. [Known limitations](#known-limitations)

---

## Authentication

The API uses JWT Bearer authentication. Include the token in the `Authorization` header:

```
Authorization: Bearer <access_token>
```

Obtaining a token
- POST `/v1/auth/login/` with JSON `{ "email": string, "password": string }`. Response includes `token` (string), `name` and `role`.

Token claims (access tokens):

| Field | Type | Description |
|---|---:|---|
| `user_id` | integer | user id |
| `email` | string | user email |
| `role` | string | `user` or `admin` |
| `exp` | integer | expiration (unix timestamp) |

Reset tokens (admin can generate) include `user_id`, `email`, `role`, `is_active`, and `exp`. Reset tokens are created with a 10-minute TTL by admin handler.

Role-based access: handlers check `role` string (`user` or `admin`) where applicable.

---

## Response format

All handlers write JSON via `utils.WriteJson` which emits an object envelope (map[string]interface{}). Common envelope keys used across handlers:

- `error` — string (for errors)
- `user`, `token`, `name`, `role` — auth responses
- `statuses` — list of status objects
- `projects`, `count` — project listing
- `session`, `status` — start session response
- `result`, `metadata` — list responses with pagination
- `report` — report object

There is no single enforced top-level `data` key; handlers vary. See Known Limitations for details.

---

## Error handling

Error responses use the envelope `{ "error": "message" }` and appropriate HTTP status codes.

Common status codes used:

| Status | Meaning |
|---:|---|
| 200 | OK |
| 201 | Created |
| 400 | Bad Request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not Found |
| 409 | Conflict (used in some handlers) |
| 500 | Internal Server Error |

---

## Pagination & filtering

Pagination params used by store filter helpers:

| Param | Type | Default | Notes |
|---|---:|---:|---|
| `page` | integer | 1 | Page must be 1..10_000_000
| `page_size` | integer | 50 | Page size must be 1..1000
| `sort` | string | `id` | Safe-list enforced per handler; prefix `-` for DESC

Metadata format returned by list endpoints (from `store.Metadata`):

```json
{
  "current_page": 1,
  "page_size": 50,
  "first_page": 1,
  "last_page": 10,
  "total_records": 500
}
```

---

## Endpoints

All endpoints are mounted under `/v1`.

### Health

```
GET /health
```

Auth: not required

Response: `200 OK` (empty body)

---

### Authentication endpoints

#### Register

```
POST /v1/auth/register/
```

Auth: not required

Request body:

```json
{ "name": "string", "email": "user@example.com", "password": "string" }
```

Validation:
- `email` must match regex in code
- email must be unique

Success (201):

```json
{ "user": { "id": 1, "name": "...", "email": "...", "role": "user", "is_active": true, "created_at": "...", "updated_at": "..." } }
```

Errors: 400 invalid payload/email, 400 duplicate email (handler responds with message `Email is already exists`), 500 internal error

#### Login

```
POST /v1/auth/login/
```

Auth: not required

Request body: `{ "email": string, "password": string }`

Behavior & validations:
- Reject if user not found or `is_active == false`
- If `is_locked` and `time.Since(last_failed_login) < 24h`, login is rejected
- On password mismatch: `LoginFail` increments `failed_attempts`; if `FailedAttempts+1 > 4` then `Lockout` is invoked
- On success: `Unlock` is called and a JWT is returned

Success (200):

```json
{ "token": "<jwt>", "name": "User Name", "role": "user" }
```

Errors: 400 malformed, 401 unauthorized (invalid credentials/inactive/locked), 500 internal

#### Reset password (public)

```
POST /v1/auth/reset-password/
```

Auth: not required (token provided in body)

Request body:

```json
{ "token": "<reset-token>", "new_password": "...", "confirm_password": "..." }
```

Behavior: parses reset token (`ParseResetToken`), verifies token user exists and email matches DB, sets new password via `UpdatePasswordPlain`.

Success (200): `{ "message": "password updated" }`

Errors: 400 invalid/expired token or passwords mismatch, 500 internal

---

### Users

#### Update user

```
PATCH /v1/users/{id}/
```

Auth: required

Roles: admin can update any user; regular users can update themselves only

Path param: `{id}`

Request body (all fields optional):

```json
{
  "name": "string",            // optional
  "email": "email@example.com", // optional
  "old_password": "string",    // optional (required to change password)
  "new_password": "string",    // optional
  "role": "user|admin",        // admin-only
  "is_active": true|false       // admin-only
}
```

Validations & behavior:
- If non-admin supplies `role` or `is_active` -> 403
- Email must be unique
- To change password both `old_password` and `new_password` are required and old password must match

Success (200): `{ "user": { ...updated user fields... } }`

Errors: 400 bad request, 401/403 unauthorized/forbidden, 404 user not found, 500 internal

#### List users (admin)

```
GET /v1/admin/users/
```

Auth: required (admin only)

Query params: `search`, `page`, `page_size`, `sort` (safe-list: `id`, `email`, `name`), `is_active` (bool), `is_locked` (bool)

Success (200): `{ "result": [ users... ], "metadata": Metadata }`

---

### Statuses

```
GET /v1/statuses
```

Auth: middleware applied but no explicit role required

Success (200): `{ "statuses": [ { "id": 1, "name": "active" }, ... ] }`

---

### Projects

#### List projects

```
GET /v1/projects
```

Auth: middleware applied

Success (200): `{ "count": int, "projects": [ ProjectRow ] }`

`ProjectRow` shape: `{ "id": int, "name": string, "status": { "id": int, "name": string } }`

#### Create project (admin)

```
POST /v1/projects/
```

Auth: required, admin only

Request body:

```json
{ "name": "Project name", "status_id": 1 }
```

Validations: `name` non-empty, `status_id` > 0

Success (201): `{ "message": "project created successfully", "project": { "project_id": int, "project_name": string } }`

#### Update project (admin)

```
PATCH /v1/project/{id}/
```

Auth: required, admin only

Request body: `{ "name": string, "status_id": int }`

Success (200): `{ "message": "project updated succesfully" }`

---

### Work sessions

#### Start session

```
POST /v1/work-sessions/start/
```

Auth: required

Request body:

```json
{ "project_id": int, "note": "string" }
```

Validation: `project_id` must be positive

Success (201):

# Worktime API — Quick Reference

Concise, example-driven API reference derived directly from the code. Base URL (default): `http://localhost:4000/v1`

Summary: user auth, projects, statuses, work sessions, and summary reports.

Auth & security
- JWT Bearer tokens: `Authorization: Bearer <token>`
- Access token claims: `user_id`, `email`, `role`, `exp` (24h TTL)
- Reset tokens include `is_active` and use a 10-minute TTL when generated by admin
- Account lockout: store increments `failed_attempts`; after >4 failures `is_locked=true`; login rejected if locked and last failed <24h

Global conventions
- Responses: JSON envelope (handlers use different top-level keys: `error`, `user`, `result`, `metadata`, `session`, `report`, etc.)
- Errors: `{ "error": "message" }` with appropriate HTTP status codes
- Pagination: `page` (default 1), `page_size` (default 50), `sort` with safe-list; `page_size` ≤ 1000

Endpoints (request + example responses)

1) Health
GET /health — no auth
200 OK
```
HTTP/1.1 200 OK
```

2) Register
POST /v1/auth/register/ — public
Request JSON
```json
{ "name": "Alice", "email": "alice@example.com", "password": "secret" }
```
Success (201)
```json
{ "user": { "id": 1, "name": "Alice", "email": "alice@example.com", "role": "user", "is_active": true, "created_at": "..." } }
```
Error (400)
```json
{ "error": "invalid email" }
```

3) Login
POST /v1/auth/login/ — public
Request JSON
```json
{ "email": "alice@example.com", "password": "secret" }
```
Success (200)
```json
{ "token": "<jwt>", "name": "Alice", "role": "user" }
```
Error (401)
```json
{ "error": "unauthorized" }
```

4) Reset password (using token)
POST /v1/auth/reset-password/ — public
Request JSON
```json
{ "token": "<reset-token>", "new_password": "newpass", "confirm_password": "newpass" }
```
Success (200)
```json
{ "message": "password updated" }
```

5) List statuses
GET /v1/statuses — auth middleware applied
Success (200)
```json
{ "statuses": [ { "id": 1, "name": "active" }, { "id": 2, "name": "inactive" } ] }
```

6) Projects — list
GET /v1/projects — auth
Success (200)
```json
{ "count": 2, "projects": [ { "id": 1, "name": "Website", "status": { "id":1, "name":"active" } } ] }
```

7) Create project (admin)
POST /v1/projects/ — auth, admin
Request
```json
{ "name": "Website", "status_id": 1 }
```
Success (201)
```json
{ "message": "project created successfully", "project": { "project_id": 5, "project_name": "Website" } }
```
Error (403)
```json
{ "error": "only admin can create a project" }
```

8) Update project (admin)
PATCH /v1/project/{id}/ — auth, admin
Request
```json
{ "name": "New name", "status_id": 2 }
```
Success (200)
```json
{ "message": "project updated succesfully" }
```

9) Start work session
POST /v1/work-sessions/start/ — auth
Request
```json
{ "project_id": 3, "note": "Morning tasks" }
```
Success (201)
```json
{ "session": { "id": 10, "user_id": 2, "project_id": 3, "start_at": "...", "end_at": null, "note": "Morning tasks" }, "status": "active" }
```
Error when active exists (400)
```json
{ "error": "you already have one active session.Stop it before starting a new sessions" }
```

10) Stop session
PATCH /v1/work-sessions/stop/{id}/ — auth
Success (200)
```json
{ "message": "session stopped", "session_id": 10 }
```
Error (404)
```json
{ "error": "no active session" }
```

11) List sessions
GET /v1/work-sessions/list/ — auth
Query params: `page`, `page_size`, `search`, `active` (true/false), `project_id`, `user_id` (admin-only)
Success (200)
```json
{ "result": [ /* WorkSessionRow objects */ ], "metadata": { "current_page": 1, "page_size": 50, "total_records": 10 } }
```

12) Summary report
GET /v1/work-sessions/reports/ — auth
Query params (required): `from` (YYYY-MM-DD or RFC3339), `to` (YYYY-MM-DD or RFC3339); optional `project_id`, `user_id` (admin)
Success (200)
```json
{ "report": { "from": "2025-01-01", "to": "2025-01-31", "overall": { "total_sessions": 10, "total_durations": "0 days, 10:00:00" }, "users": [...], "projects": [...] } }
```

13) Update user
PATCH /v1/users/{id}/ — auth
Allowed fields: `name`, `email`, `old_password`, `new_password`, `role` (admin), `is_active` (admin)
Success (200)
```json
{ "user": { "id": 2, "name": "Bob", "email": "bob@example.com", "role": "user", "is_active": true } }
```

14) Admin: list users
GET /v1/admin/users/ — auth, admin
Query params: `search`, `page`, `page_size`, `sort`, `is_active`, `is_locked`
Success (200)
```json
{ "result": [ /* users */ ], "metadata": { "current_page":1, "page_size":50 } }
```

15) Admin: generate reset token
POST /v1/admin/reset-tokens/ — auth, admin
Request: `{ "email": "user@example.com" }`
Success (200)
```json
{ "reset_token": "<token>", "expires_at": "2025-01-14T12:00:00Z" }
```

Known limitations (short)
- Project create/update handlers currently omit wiring `status_id` into the store update/create call (bug).
- `start session` may write two responses on DB unique-index error (missing early return).
- One handler path uses key `"error:"` instead of `"error"` (typo).
- Response envelope keys are inconsistent across handlers.

Files referenced: `internal/router/routes.go`, `internal/api/*`, `internal/store/*`, `migrations/*`.

If you'd like: I can generate a compact OpenAPI spec, add curl examples for the most-used endpoints, or fix the two project bugs now. Reply with which you want next.

| status_id | BIGINT | yes | FK to `statuses(id)` |
| created_at | TIMESTAMPTZ | default now() | Created timestamp |

JSON projections used in API: `ProjectRow` { id, name, status: { id, name } } and `Project` used for create returns { project_id, project_name }.

### Status (table `statuses`)
| Field | Type | Required | Description |
|---|---:|---:|---|
| id | BIGSERIAL | yes | PK |
| name | VARCHAR(255) | yes, unique | e.g., `active`, `inactive` |

### WorkSession (table `work_sessions`)
| Field | Type | Required | Description |
|---|---:|---:|---|
| id | BIGSERIAL | yes | PK |
| user_id | BIGINT | yes | FK users.id |
| project_id | BIGINT | optional | FK projects.id |
| start_at | TIMESTAMPTZ | yes | Session start |
| end_at | TIMESTAMPTZ | nullable | Session end; NULL if active |
| note | TEXT | optional | Free text note |
| created_at | TIMESTAMPTZ | optional | Created timestamp |

Store layer response objects: `WorkSessionRow` with nested `user`, `project`, `sessions`, and derived `status`.

---

## Role-based access summary (high-level)

| Endpoint | Auth required | Admin | User (self) |
|---|---:|---:|---:|
| POST /v1/auth/register/ | No | — | — |
| POST /v1/auth/login/ | No | — | — |
| POST /v1/auth/reset-password/ | No | — | — |
| GET /v1/statuses | Yes | Read | Read |
| GET /v1/projects | Yes | Read | Read |
| POST /v1/projects/ | Yes | Create | — |
| PATCH /v1/project/{id}/ | Yes | Update | — |
| PATCH /v1/users/{id}/ | Yes | Update any | Update self only |
| GET /v1/admin/users/ | Yes | List all | forbidden |
| POST /v1/admin/reset-tokens/ | Yes | Create | forbidden |
| POST /v1/work-sessions/start/ | Yes | Start own | Start own |
| PATCH /v1/work-sessions/stop/{id}/ | Yes | Stop if matching user | Stop own |
| GET /v1/work-sessions/list/ | Yes | Can filter by user_id | Only own sessions (unless admin) |
| GET /v1/work-sessions/reports/ | Yes | Admin optional user_id | Forced to self |

---

## Environment differences

- Development: `main.go` calls `godotenv.Load()` to read a `.env` file (local convenience).
- Production: must provide `WORKTIME_DB_DSN` and a secure `WORKTIME_JWT_SECRET` (code includes an insecure fallback default).

---

## Known Limitations & Ambiguities (observed in code)

1. Project creation/update bugs:
   - `POST /v1/projects/` handler validates `status_id` but constructs `store.Project` without setting `StatusId`, so created project may have missing/zero `status_id` in DB.
   - `PATCH /v1/project/{id}/` handler decodes `name` and `status_id` but does not pass them to `projectStore.UpdateProject`; the store `UpdateProject` constructs a fresh `Project{}` and uses zero values — update is effectively a no-op or will overwrite with zero values.

2. Handler double-write paths:
   - `POST /v1/work-sessions/start/`: when the DB returns an error and the error text contains `one_active_session_per_user`, the handler writes a 400 response but does not `return`, possibly falling through to write a 500 response as well. This can produce multiple writes to `http.ResponseWriter`.

3. Response key typo:
   - In one code path `HandleUpdateUser` an internal-server-error response uses key `"error:"` (with trailing colon) instead of `"error"` (typo), potentially breaking clients that parse `error`.

4. Authentication middleware & usage:
   - Middleware `Authenticate` sets user into request context. Handler code uses `middleware.GetUser(r)` which returns `(*auth.UserClaims, bool)`. Some earlier code paths assume a single return — ensure consistent usage.

5. Inconsistent status codes/messages:
   - Duplicate email on register returns 400 with message `Email is already exists` rather than 409 Conflict (inconsistent with common semantic conventions).

6. Rate limiting is not implemented.

If you want, I can:
- Produce an OpenAPI (Swagger) spec from these derived schemas.
- Add example curl commands and JSON examples for each endpoint.
- Fix the identified logical bugs in `internal/api/project_handler.go` and `internal/store/project.go`, and add a unit test for project create/update flows.

---

File generated from code scan on: 2026-01-14
