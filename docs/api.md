# Worktime API Documentation

This file documents the HTTP API implemented by the repository source code. All descriptions are derived directly from the code (routers, handlers, middleware, store types and migrations). If something is ambiguous or missing in the code, it is listed in **Known Limitations**.

---

## 1) Scan: routes and endpoints

All routes registered in `internal/router/routes.go`:

- GET /health
- POST /v1/auth/register/
- POST /v1/auth/login/
- POST /v1/auth/reset-password/
- GET /v1/statuses
- GET /v1/projects
- PATCH /v1/project/{id}/
- POST /v1/projects/
- POST /v1/work-sessions/start/
- PATCH /v1/work-sessions/stop/{id}/
- GET /v1/work-sessions/list/
- GET /v1/work-sessions/reports/
- PATCH /v1/users/{id}/
- POST /v1/admin/reset-tokens/
- GET /v1/admin/users/

---

## 2) Confirmation: request & response structures per endpoint

The following is a concise confirmation of request bodies and primary response keys, derived from each handler.

- POST /v1/auth/register/
  - Request JSON: { "name": string, "email": string, "password": string }
  - Success response (201): Envelope with key `user` containing a `store.User` JSON projection (fields: `id`, `name`, `email`, `role`, `is_active`, `created_at`, `updated_at`)
  - Error responses: `error` string in envelope (400, 409/400, 500)

- POST /v1/auth/login/
  - Request JSON: { "email": string, "password": string }
  - Success response (200): Envelope with keys: `token` (JWT string), `name` (string), `role` (string)
  - Error responses: `error` string in envelope (400, 401, 500)

- POST /v1/auth/reset-password/
  - Request JSON: { "token": string, "new_password": string, "confirm_password": string }
  - Success response (200): { "message": "password updated" }
  - Error responses: `error` string (400, 500)

- GET /v1/statuses
  - No request body; Success (200): { "statuses": [ { "id": int, "name": string }, ... ] }

- GET /v1/projects
  - No request body; Success (200): { "count": int, "projects": [ ProjectRow ] }
  - `ProjectRow` JSON: { "id": int, "name": string, "status": { "id": int, "name": string } }

- POST /v1/projects/
  - Request JSON: { "name": string, "status_id": int }
  - Success (201): { "message": string, "project": { "project_id": int, "project_name": string } }
  - Errors: `error` string (400, 401, 403, 500)

- PATCH /v1/project/{id}/
  - Path param: `{id}`
  - Request JSON decoded as { "name": string, "status_id": int }
  - Success (200): { "message": "project updated succesfully" }
  - Errors: `error` string (400, 401, 403, 500)

- POST /v1/work-sessions/start/
  - Request JSON: { "project_id": int, "note": string }
  - Success (201): { "session": WorkSession, "status": "active" }
  - `WorkSession` JSON (from store.WorkSession): { "id": int, "user_id": int, "project_id": int, "start_at": timestamp, "end_at": null, "note": string, "created_at": timestamp }
  - Errors: `error` string (400, 401, 500). If DB index `one_active_session_per_user` prevents insert, handler returns 400 with message about an existing active session.

- PATCH /v1/work-sessions/stop/{id}/
  - Path param: `{id}`
  - Success (200): { "message": "session stopped", "session_id": id }
  - Errors: `error` string (400, 401, 404, 500)

- GET /v1/work-sessions/list/
  - Query params supported: `page` (int, default 1), `page_size` (int, default 50), `search` (string), `active` (bool), `project_id` (int), `user_id` (int, admin only)
  - Success (200): { "result": [ WorkSessionRow ], "metadata": Metadata }
  - `WorkSessionRow` JSON: { "user": {user fields}, "project": {project fields}, "sessions": {id,start_at,end_at,note,created_at}, "status": "active"|"inactive" }
  - Errors: `error` string (400, 401, 500)

- GET /v1/work-sessions/reports/
  - Query params (required): `from` (date or RFC3339), `to` (date or RFC3339)
  - Optional: `project_id` (int), `user_id` (int, admin-only)
  - Success (200): { "report": SummaryReport }
  - `SummaryReport` fields derived from store: `from`, `to`, `filters`, `overall` (total_sessions, total_durations), `users` (array), `projects` (array)
  - Errors: `error` string (400, 401, 500)

- PATCH /v1/users/{id}/
  - Path param: `{id}`
  - Request JSON (all optional fields): { "name": *string, "email": *string, "old_password": *string, "new_password": *string, "role": *string (admin-only), "is_active": *bool (admin-only) }
  - Success (200): { "user": store.User }
  - Errors: `error` string (400, 401, 403, 404, 500)

- POST /v1/admin/reset-tokens/
  - Request JSON: { "email": string }
  - Success (200): { "reset_token": string, "expires_at": RFC3339 timestamp }
  - Errors: `error` string (400, 401, 404, 500)

- GET /v1/admin/users/
  - Query params: `search`, `page`, `page_size`, `sort` (safe list: id,email,name), `is_active` (bool), `is_locked` (bool)
  - Success (200): { "result": [ users... ], "metadata": Metadata }
  - Errors: `error` string (400, 401, 500)

---

## 3) Final documentation (generated)

The full API documentation is below. It is derived strictly from the code. Known limitations and ambiguities discovered during analysis are listed at the end.

---

# Worktime API (v1)

Base URL: `/v1`

## Overview

- Purpose: Backend for simple time tracking — users start/stop sessions on projects, admins manage users/projects and generate summary reports.
- Tech stack: Go, chi router, PostgreSQL (pgx), goose migrations, JWT auth (github.com/golang-jwt/jwt/v5).

## Authentication & Security

- JWT: access tokens created with HMAC-SHA256, TTL 24 hours; signing key from `WORKTIME_JWT_SECRET` (fallback default present in code).
- Authorization header: `Authorization: Bearer <token>`
- Access token claims (in token payload): `user_id` (int), `email` (string), `role` (string), `exp` (unix timestamp). `auth.UserClaims` maps these fields.
- Reset token claims: includes `user_id`, `email`, `role`, `is_active`, `exp` (parsed into `auth.ResetClaims`). TTL for reset tokens is 10 minutes when created by admin.
- Role-based access: `role` is string `user` or `admin`. Handlers enforce admin-only actions where applicable.
- Rate limiting: not implemented.
- Account lockout: implemented in store and login handler — failed attempts increment `failed_attempts`; when attempts exceed 4 a `Lockout` sets `is_locked=true`. Login rejects if locked and last_failed_login < 24h.

## Global API Conventions

- All responses are JSON envelopes (map[string]interface{}) produced by `utils.WriteJson`.
- Error format: `{ "error": "message" }` and appropriate HTTP status codes are used in handlers.
- Pagination: `page` (default 1), `page_size` (default 50). `Filter` enforces page <= 10_000_000 and page_size <= 1000.
- Sorting: `sort` param with safe-list per handler; prefix `-` indicates descending.

---

## Endpoints (detailed)

Note: Authentication: routes under the router group that uses `Authenticate` will have user context available via middleware. Handlers check roles and return `401`/`403` as appropriate.

### GET /health
- Auth: none
- Success: 200 OK (empty body)

### POST /v1/auth/register/
- Auth: none
- Request body (JSON):

```json
{
  "name": "string",
  "email": "user@example.com",
  "password": "string"
}
```

- Validation: email regex enforced; uniqueness checked.
- Success (201):

```json
{ "user": { "id": 1, "name": "...", "email": "...", "role": "user", "is_active": true, "created_at": "...", "updated_at": "..." } }
```

- Errors: 400 invalid payload/email, 400 duplicate email (handler returns this message), 500 internal errors.

### POST /v1/auth/login/
- Auth: none
- Request body:

```json
{ "email": "user@example.com", "password": "string" }
```

- Behavior: checks `is_active`, rejects if `is_locked` and last_failed_login < 24h. On password mismatch, increments failed attempts; locks when >4 attempts.
- Success (200):

```json
{ "token": "<jwt>", "name": "User Name", "role": "user" }
```

- Errors: 400 malformed, 401 unauthorized (bad creds, inactive, locked), 500 internal.

### POST /v1/auth/reset-password/
- Auth: none
- Request body:

```json
{ "token": "<reset-token>", "new_password": "...", "confirm_password": "..." }
```

- Behavior: validates reset token (10-minute TTL when created), ensures token email matches DB email, updates password.
- Success (200): `{ "message": "password updated" }`
- Errors: 400 invalid token/data, 500 on DB errors.

### GET /v1/statuses
- Auth: middleware applied (route under authenticated group), but handler does not require a specific role.
- Success (200):

```json
{ "statuses": [ { "id": 1, "name": "active" }, { "id": 2, "name": "inactive" } ] }
```

### GET /v1/projects
- Auth: middleware applied
- Success (200):

```json
{ "count": 2, "projects": [ { "id": 1, "name": "Proj A", "status": { "id":1, "name":"active" } } ] }
```

### POST /v1/projects/
- Auth: required
- Roles: admin only
- Request body:

```json
{ "name": "Project name", "status_id": 1 }
```

- Success (201):

```json
{ "message": "project created successfully", "project": { "project_id": 1, "project_name": "Project name" } }
```

### PATCH /v1/project/{id}/
- Auth: required
- Roles: admin only
- Path param: `{id}`
- Request body:

```json
{ "name": "New name", "status_id": 2 }
```

- Success (200): `{ "message": "project updated succesfully" }`

### POST /v1/work-sessions/start/
- Auth: required
- Roles: authenticated users
- Request body:

```json
{ "project_id": 123, "note": "optional note" }
```

- Success (201):

```json
{ "session": { "id": 1, "user_id": 2, "project_id": 123, "start_at": "2025-01-01T12:00:00Z", "end_at": null, "note": "...", "created_at": "..." }, "status": "active" }
```

- Edge case: DB enforces unique active session per user via `one_active_session_per_user` index — if violated, handler maps that error to a 400 message advising to stop existing active session.

### PATCH /v1/work-sessions/stop/{id}/
- Auth: required
- Path param: `{id}`
- Behavior: stops session only for the authenticated user (store update uses `id` and `user_id`)
- Success (200): `{ "message": "session stopped", "session_id": <id> }`
- Errors: 404 if no active session found for id & user.

### GET /v1/work-sessions/list/
- Auth: required
- Query params:
  - `page` (int, default 1)
  - `page_size` (int, default 50)
  - `search` (string)
  - `active` (bool)
  - `project_id` (int)
  - `user_id` (int) — admin-only
- Success (200):

```json
{ "result": [ { "user": {...}, "project": {...}, "sessions": {...}, "status": "active" } ], "metadata": { "current_page":1, "page_size":50, "total_records": 123 } }
```

### GET /v1/work-sessions/reports/
- Auth: required
- Query params (required): `from` (date or RFC3339), `to` (date or RFC3339)
- Optional: `project_id` (int), `user_id` (int, admin-only)
- Success (200): `{ "report": SummaryReport }` where `SummaryReport` contains `from`, `to`, `filters`, `overall`, `users`, `projects` as defined in `internal/store/work_session.go`.

### PATCH /v1/users/{id}/
- Auth: required
- Roles: admin may update any user; regular users can update only themselves
- Request JSON (all optional):

```json
{
  "name": "...",
  "email": "...",
  "old_password": "...",
  "new_password": "...",
  "role": "user|admin",    // admin-only
  "is_active": true|false   // admin-only
}
```

- Success (200): `{ "user": { ...updated user... } }`

### POST /v1/admin/reset-tokens/
- Auth: required
- Roles: admin only
- Request JSON: `{ "email": "user@example.com" }`
- Success (200): `{ "reset_token": "<token>", "expires_at": "RFC3339" }`

### GET /v1/admin/users/
- Auth: required
- Roles: admin only
- Query params: `search`, `page`, `page_size`, `sort` (safe-list: id,email,name), `is_active`, `is_locked`
- Success (200): `{ "result": [ users... ], "metadata": Metadata }`

---

## Core resources

- Users: fields derived from `migrations/00001_users.sql` and `internal/store/user_store.go`.
- Projects: `projects` table with `status_id` FK to `statuses`.
- Statuses: `statuses` table seeded with `active` and `inactive`.
- Work sessions: `work_sessions` table with `user_id`, `project_id`, `start_at`, `end_at`, `note` and a unique partial index to prevent multiple active sessions per user.

## Data models (fields & types)

### User (table `users`)
| Field | Type | Required | Description |
|---|---:|---:|---|
| id | BIGSERIAL | yes | Primary key |
| name | VARCHAR(50) | yes | Display name |
| email | VARCHAR(255) | yes, unique | Login email |
| password_hash | BYTEA | yes | bcrypt hash (not returned via API) |
| is_active | BOOLEAN | default false | Account active flag |
| role | TEXT | default 'user' | `user` or `admin` |
| is_locked | BOOLEAN | default false | Account lock flag (migration) |
| last_failed_login | TIMESTAMPTZ | nullable | Last failed login time |
| failed_attempts | INT | default 0 | Consecutive failed login attempts |
| created_at | TIMESTAMPTZ | default now() | Created timestamp |
| updated_at | TIMESTAMPTZ | default now() | Updated timestamp |

### Project (table `projects`)
| Field | Type | Required | Description |
|---|---:|---:|---|
| id | BIGSERIAL | yes | PK |
| name | VARCHAR(255) | optional | Project name |
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
