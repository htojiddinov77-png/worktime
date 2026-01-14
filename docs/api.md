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
