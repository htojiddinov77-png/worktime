Worktime API

## Overview

**Project purpose:** Worktime is a simple time-tracking backend that lets users start/stop work sessions against projects, manage projects and users, and produce summary reports for a date range.

**Tech stack:**
- Language: Go
- Router: chi (github.com/go-chi/chi/v5)
- Database: PostgreSQL (pgx driver)
- Migrations: goose
- Auth: JWT (HMAC HS256)

**API versioning & base URL:** All API endpoints are mounted under `/v1` (e.g. `/v1/auth/login`, `/v1/work-sessions/list/`). The server listens on a configurable port (env `WORKTIME_PORT`).

---

## Authentication & Security

- JWT usage: tokens are HMAC-signed using a secret from env `WORKTIME_JWT_SECRET` (fallback default `change-me-in-env-worktime-jwt-secret`). Access tokens are created with a 24h TTL.
- Authorization header format: `Authorization: Bearer <token>`; the middleware extracts the bearer token (`auth.ExtractBearerToken`).
- Token claims:
  - Access tokens: `user_id` (int64), `email` (string), `role` (string), `exp` (unix timestamp). These map to `auth.UserClaims` when verified.
  - Reset tokens: `user_id`, `email`, `role`, `is_active`, `exp`. Parsed using `ParseResetToken` into `auth.ResetClaims`.
- Role-based access: roles are strings: `user` (default) or `admin`. Handlers check `role` to allow admin-only actions.
- Rate limiting: none implemented in codebase (no rate limiter middleware found).
- Account lockout behavior: implemented in `user_store` and `TokenHandler`:
  - Failed login increments `failed_attempts` and sets `last_failed_login` (`LoginFail`).
  - If consecutive failed attempts exceed 4 (code checks `if existingUser.FailedAttempts+1 > 4`), the account is locked via `Lockout` (sets `is_locked = true`).
  - Login rejects if `is_locked` is true and `time.Since(last_failed_login) < 24h`.
  - On successful login the user is `Unlock`ed (`failed_attempts` reset, `is_locked=false`).

Limitations: There is no explicit refresh-token mechanism, no per-endpoint rate limiting, and lockout timeout is enforced via a 24h check based on `last_failed_login`.

---

## Global API Conventions

- Response envelope: all responses are JSON objects written via `utils.WriteJson` which marshals an `Envelope` (map[string]interface{}). Common keys observed: `error`, `user`, `token`, `message`, `result`, `metadata`, `statuses`, `projects`, `session`, `report`, `count`, `name`, `role`, `reset_token`, `expires_at`.
- Error responses: JSON envelope with an `error` key and an explanatory string. Status codes vary per handler (400, 401, 403, 404, 409, 500).
- HTTP status codes used in handlers:
  - 200 OK — general success
  - 201 Created — resource created (`register`, `start session`, `create project`)
  - 400 Bad Request — invalid payload, validation errors, missing params
  - 401 Unauthorized — authentication or inactive/locked user
  - 403 Forbidden — authorization failures (role checks)
  - 404 Not Found — resource not found
  - 409 Conflict — duplicate email
  - 500 Internal Server Error — unexpected errors
- Pagination: implemented using `Filter` in store layer. Query params supported: `page` (default 1), `page_size` (default 50). Page size validation: 1..1000. Page bounds validated.
- Sorting: `sort` query param with a safe-list per endpoint (e.g. users: `id`, `email`, `name`). A `-` prefix denotes descending (handlers sometimes rewrite `-` to ` DESC`).
- Filtering: endpoints expose filters via query parameters (details under each endpoint).

---

## Endpoints

All routes are under `/v1`.

Summary of top-level groups (from router):
- GET `/health` — health check (no auth)
- `/v1/auth` group: registration, login, password reset (no auth for register/login/reset-password)
- Authenticated group (middleware.Authenticate): statuses, projects, project create/update, work-sessions (start/stop/list/reports), user updates and admin user management.

Note: middleware.Authenticate is applied to the authenticated group. If `Authorization` header is missing, the middleware sets an anonymous user object; handlers perform role checks and return 401/403 as appropriate.

### POST /v1/auth/register/
- Purpose: create new user
- Auth: none
- Roles allowed: public
- Request body (JSON):

```json
{
  "name": "string",
  "email": "user@example.com",
  "password": "string"
}
```

- Validation:
  - `email` must match email regex; must be unique
  - `name` may not be empty (trimmed in update; not strictly enforced on register beyond being provided)
  - password is hashed and stored
- Responses:
  - 201 Created:
  ```json
  { "user": { "id": 1, "name": "...", "email": "...", "role": "user", "is_active": true, "created_at": "..." } }
  ```
  - 400 Bad Request for invalid payload or invalid email
  - 409/400 for duplicate email (handler returns 400 with message "Email is already exists")
  - 500 Internal Server Error on DB or hashing failures

### POST /v1/auth/login/
- Purpose: authenticate user and obtain JWT access token
- Auth: none
- Request body (JSON):
```json
{
  "email": "user@example.com",
  "password": "string"
}
```
- Behavior:
  - Verifies user exists and `is_active` is true
  - If user `is_locked` and last failed login less than 24h ago, login is rejected with 401
  - On password mismatch: increments failed attempts; if attempts exceed 4, lockout is applied; return 401
  - On success: unlocks the user, returns JWT token
- Responses:
  - 200 OK:
  ```json
  { "token": "<jwt>", "name": "User Name", "role": "user" }
  ```
  - 400 Bad Request for malformed request
  - 401 Unauthorized for invalid credentials, inactive or locked user
  - 500 Internal Server Error for DB or token generation errors

### POST /v1/auth/reset-password/
- Purpose: reset password using a reset token
- Auth: none (token provided in body)
- Request body (JSON):
```json
{
  "token": "<reset-token>",
  "new_password": "...",
  "confirm_password": "..."
}
```
- Behavior:
  - Validates token via `ParseResetToken` and ensures token user exists and email matches
  - Updates password via `UpdatePasswordPlain`
- Responses:
  - 200 OK: `{ "message": "password updated" }`
  - 400 Bad Request for token missing/invalid or mismatched passwords
  - 500 Internal Server Error for DB failures

### GET /v1/statuses
- Purpose: list available statuses
- Auth: authenticated group; `Authenticate` middleware runs, but this handler does not require admin — any authenticated user (or anonymous if middleware allowed) may get the list. Handlers do not require role check.
- Query params: none
- Response 200:
```json
{ "statuses": [ { "id":1, "name":"active" }, { "id":2, "name":"inactive" } ] }
```

### GET /v1/projects
- Purpose: list projects
- Auth: authenticated group (middleware applied); handler does not require admin
- Query params: none
- Response 200:
```json
{ "count": 3, "projects": [ { "id": 1, "name": "Proj A", "status": {"id":1, "name":"active"} } ] }
```

### POST /v1/projects/
- Purpose: create a new project
- Auth: required
- Roles allowed: admin only (handler checks `user.Role == "admin"`)
- Request body (JSON):
```json
{
  "name": "Project name",
  "status_id": 1
}
```
- Validation:
  - `name` non-empty
  - `status_id` must be positive
- Responses:
  - 201 Created:
  ```json
  { "message": "project created successfully", "project": { "project_id": 1, "project_name": "Project name" } }
  ```
  - 401/403 on unauthenticated/unauthorized
  - 400 for validation
  - 500 for DB errors

### PATCH /v1/project/{id}/
- Purpose: update project (name/status)
- Auth: required
- Roles allowed: admin only
- Path parameter: `{id}` numeric project id
- Request body (JSON): handler uses a struct with `name` and `status_id` but passes values to the store which expects project data; there is limited validation in handler besides JSON decoding.
- Responses:
  - 200 OK: `{ "message": "project updated succesfully" }`
  - 400 Bad Request for invalid id or payload
  - 401/403 for auth errors
  - 500 Internal Server Error for DB errors

### PATCH /v1/users/{id}/
- Purpose: update a user (self or admin can update); admin can change `role` and `is_active`
- Auth: required
- Roles allowed: admin can update any user; normal users can update only themselves
- Path parameter: `{id}` numeric
- Request body (JSON): allowed fields (all optional):
```json
{
  "name": "optional",
  "email": "optional@example.com",
  "old_password": "...",
  "new_password": "...",
  "role": "admin|user",      // admin-only
  "is_active": true|false     // admin-only
}
```
- Validation & behavior:
  - If non-admin tries to set `role` or `is_active` -> 403 Forbidden
  - To change password, both `old_password` and `new_password` must be present; old password matched against stored hash
  - If updating email, handler checks uniqueness
  - At least one updatable field must be provided
- Responses:
  - 200 OK: `{ "user": { ...updated user... } }`
  - 400 Bad Request for invalid payload or no fields
  - 401/403 as appropriate
  - 404 Not Found if user does not exist
  - 500 Internal Server Error on DB errors

### GET /v1/admin/users/
- Purpose: admin-only listing of users with search, filtering and pagination
- Auth: required
- Roles allowed: admin only (handler enforces `user.Role == "admin"`)
- Query parameters:
  - `search` (string) — prefix search against `email` or `name`
  - `page` (int, default 1)
  - `page_size` (int, default 50)
  - `sort` (string, default `id`) — safe list: `id`, `email`, `name`. Prefix `-` for descending.
  - `is_active` (boolean) — optional
  - `is_locked` (boolean) — optional
- Responses:
  - 200 OK: `{ "result": [ ...users... ], "metadata": { "current_page": 1, "page_size": 50, ... } }`
  - 400 Bad Request for invalid query params
  - 401 Unauthorized if not admin

### POST /v1/admin/reset-tokens/
- Purpose: admin generates a password reset token for a user
- Auth: required
- Roles allowed: admin only (handler checks role)
- Request body (JSON): `{ "email": "user@example.com" }`
- Response 200:
```json
{ "reset_token": "<token>", "expires_at": "RFC3339 timestamp" }
```
- Notes: reset token TTL is 10 minutes in code.

### POST /v1/work-sessions/start/
- Purpose: start a work session (creates an active session for the authenticated user)
- Auth: required
- Roles allowed: authenticated users
- Request body (JSON):
```json
{ "project_id": 123, "note": "optional note" }
```
- Validation:
  - `project_id` must be positive
  - The store layer enforces a unique active session per user via a DB unique index (`one_active_session_per_user`); starting a second active session will result in an error. Handler maps an index error containing `one_active_session_per_user` into a 400 with message about existing active session.
- Responses:
  - 201 Created:
```json
{ "session": {"id": 1, "user_id": 2, "project_id": 3, "start_at": "...", "end_at": null, "note": "..."}, "status": "active" }
```
  - 400 Bad Request for validation or existing active session
  - 401 Unauthorized if not authenticated
  - 500 Internal Server Error for DB errors

### PATCH /v1/work-sessions/stop/{id}/
- Purpose: stop an active work session (set `end_at`)
- Auth: required
- Roles allowed: user may stop their own session; handler uses session id and current user id in store update
- Path param: session id
- Responses:
  - 200 OK: `{ "message": "session stopped", "session_id": <id> }`
  - 400 Bad Request for invalid id
  - 401 Unauthorized
  - 404 Not Found: if no active session found for given id & user
  - 500 Internal Server Error for DB errors

### GET /v1/work-sessions/list/
- Purpose: list work sessions (supports pagination, filtering, search)
- Auth: required
- Roles allowed: admin can list across users and pass `user_id`; normal users can only list their own sessions
- Query parameters:
  - `page` (int, default 1)
  - `page_size` (int, default 50)
  - `search` (string) — searched against project name, user name, user email, session note (prefix match)
  - `active` (boolean) — `true` for active sessions (end_at IS NULL), `false` for finished sessions
  - `project_id` (int) — optional
  - `user_id` (int) — admin-only
- Response 200:
```json
{ "result": [ { "user": {...}, "project": {...}, "sessions": {...}, "status": "active" } ], "metadata": { "current_page":1, "page_size":50, "total_records": 123 } }
```

### GET /v1/work-sessions/reports/
- Purpose: generate summary report for a date range
- Auth: required
- Roles allowed: admin may request for any user via `user_id`; normal users are restricted to their own data (code forces `allowedUserID` to the authenticated user's id).
- Query parameters (required):
  - `from` (string) — required; accepts RFC3339 or `YYYY-MM-DD` (handler uses helper `parseTimeParam`)
  - `to` (string) — required; same formatting rules
  - `project_id` (int) — optional
  - `user_id` (int) — admin-only
- Behavior:
  - Handler normalizes dates to UTC day boundaries and produces an overall summary plus per-user and per-project breakdowns. Only sessions with `end_at IS NOT NULL` are counted in totals.
- Response 200:
```json
{ "report": { "from": "2025-01-01", "to": "2025-01-02", "filters": {...}, "overall": {"total_sessions": 10, "total_durations": "0 days, 08:30:00"}, "users": [...], "projects": [...] } }
```

---

## Core Resources

Only resources implemented in the codebase are documented below.

### Users
- Table: `users` (see Data Models)
- Operations:
  - Register (create) — `/v1/auth/register/`
  - Login — `/v1/auth/login/`
  - Update user — `/v1/users/{id}/`
  - List users (admin) — `/v1/admin/users/`
  - Generate reset token (admin) — `/v1/admin/reset-tokens/`
  - Reset password (public, via token) — `/v1/auth/reset-password/`

### Projects
- Table: `projects` with `status_id` referencing `statuses`
- Operations:
  - List — `/v1/projects`
  - Create (admin) — `/v1/projects/`
  - Update (admin) — `/v1/project/{id}/`

### Work sessions
- Table: `work_sessions` with `user_id`, `project_id`, `start_at`, `end_at`, `note`
- Enforced unique active session per user via DB unique index `one_active_session_per_user` (`WHERE end_at IS NULL`)
- Operations:
  - Start session — `/v1/work-sessions/start/`
  - Stop session — `/v1/work-sessions/stop/{id}/`
  - List sessions — `/v1/work-sessions/list/`
  - Summary report — `/v1/work-sessions/reports/`

---

## Data Models

Derived from `internal/store` types and migrations.

### User (table: `users`)
| Field | Type | Required | Description |
|---|---:|---:|---|
| `id` | integer (BIGSERIAL) | required | Primary key |
| `name` | string | required | User display name (VARCHAR(50)) |
| `email` | string | required, unique | Login email |
| `password_hash` | bytea | required | bcrypt hash (not returned in API) |
| `role` | string | required, default `user` | `user` or `admin` |
| `is_active` | boolean | optional, default false | Whether user may login |
| `is_locked` | boolean | optional, default false | Account lock flag (added by migration)
| `last_failed_login` | timestamptz | optional | Last failed login timestamp
| `failed_attempts` | int | required default 0 | Count of consecutive failed logins
| `created_at` | timestamp | required | Record creation time
| `updated_at` | timestamp | required | Record updated time

Notes: `password_hash` is not returned in JSON responses. API exposes `role` and `is_active` to admin updates.

### Project (table: `projects`)
| Field | Type | Required | Description |
|---|---:|---:|---|
| `id` | integer | required | PK (returned as `id` in list)
| `name` | string | optional | Project name
| `status_id` | integer | required | FK to `statuses(id)`

Store types also include JSON shapes:
- `ProjectRow` => `{ id, name, status: { id, name } }`
- `Project` used in create returns `{ project_id, project_name, status_id }`

### Status (table: `statuses`)
| Field | Type | Required | Description |
|---|---:|---:|---|
| `id` | integer | required | PK |
| `name` | string | required, unique | e.g. `active`, `inactive` |

### WorkSession (table: `work_sessions`)
| Field | Type | Required | Description |
|---|---:|---:|---|
| `id` | integer | required | PK |
| `user_id` | integer | required | FK to `users(id)` |
| `project_id` | integer | optional | FK to `projects(id)` |
| `start_at` | timestamptz | required | Session start timestamp |
| `end_at` | timestamptz | optional | Session end timestamp; `NULL` for active sessions |
| `note` | text | optional | Session note |
| `created_at` | timestamptz | optional | Record creation time |

Store layer JSON projections:
- `WorkSessionRow` includes nested `user` (id, name, email, is_active), `project` (id, name, status), `sessions` (id, start_at, end_at, note, created_at) and `status` (`active` or `inactive`).

---

## Role-Based Access Control (High-level)

| Endpoint | Auth required | Admin | User (self) |
|---|---:|---:|---:|
| POST /v1/auth/register/ | No | — | — |
| POST /v1/auth/login/ | No | — | — |
| POST /v1/auth/reset-password/ | No | — | — |
| GET /v1/statuses | Yes (middleware) | Read | Read |
| GET /v1/projects | Yes | Read | Read |
| POST /v1/projects/ | Yes | Create | — |
| PATCH /v1/project/{id}/ | Yes | Update | — |
| PATCH /v1/users/{id}/ | Yes | Update any | Update self only (admin may update any)
| GET /v1/admin/users/ | Yes | List all | forbidden |
| POST /v1/admin/reset-tokens/ | Yes | Create | forbidden |
| POST /v1/work-sessions/start/ | Yes | Start for self | Start for self |
| PATCH /v1/work-sessions/stop/{id}/ | Yes | Stop if matching user (admin not explicitly allowed to stop others in code) | Stop own session |
| GET /v1/work-sessions/list/ | Yes | Can pass `user_id` filter | Only own sessions (unless admin)
| GET /v1/work-sessions/reports/ | Yes | Admin: optional `user_id`; otherwise aggregate | User forced to self |

Notes: The code enforces role checks inside handlers; missing explicit checks are considered unauthorized by the handlers that read `middleware.GetUser`.

---

## Environment Differences

- Development-only behavior: `godotenv.Load()` is called in `main.go` to load a local `.env` (development convenience). Default JWT secret fallback exists when `WORKTIME_JWT_SECRET` is not set (not suitable for production).
- Production-only behavior: none explicit in code. Database DSN must be provided via `WORKTIME_DB_DSN` in all environments; server `WORKTIME_PORT` can be set.

---

## Limitations & Notes (derived from code)

- No explicit rate limiting middleware is implemented.
- Password strength rules are not enforced beyond non-empty; password hashing uses bcrypt with cost 12.
- The middleware `GetUser` will panic if user is not set in request context; however the router uses `Authenticate` for the authenticated group which always calls `SetUser` (either anonymous or validated claims), so `GetUser` is safe for routes under that middleware. Public routes (auth) do not call `GetUser`.
- Some handlers return ambiguous HTTP error codes/messages for duplicate email vs bad request — examples are in `HandleRegister` (returns 400 for existing email with message `Email is already exists`).
- Project update handler does not fully validate provided `name` and `status_id` fields before passing to store.UpdateProject; store.UpdateProject presently reads data from a newly created `project` variable and will use zero values if handler didn't set them (possible bug/limitation).

---

If you want, I can:
- Generate OpenAPI (Swagger) spec from these derived schemas.
- Add example curl commands for each endpoint.
- Run a quick static pass to generate JSON examples from code defaults.

File: `docs/api.md`
