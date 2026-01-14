# Worktime API Documentation

## Overview

Worktime is a RESTful API for tracking work sessions, projects, statuses and users. The service is implemented in Go and uses JWT authentication. This document is a complete, production-ready reference of endpoints and every response shape emitted by the server.

**Base URL:** `http://localhost:4000/v1`

---

## Conventions

- All responses are JSON unless otherwise noted.
- Fields are shown using exact JSON names from the code and in the same casing used by the code.
- Nullable fields are explicitly marked `(nullable)`.
- Each endpoint includes:
  1. All response variants (admin / regular / empty / errors)
  2. Full JSON examples for each variant
  3. A `Response Schema` section describing each field, its type, and nullable status
- Where the exact shape is inferred from related code rather than explicit handler logic it is marked:

⚠️ Response shape inferred – verify in handler

---

## Endpoints (complete responses)

### GET /v1/health

- Auth: not required

Responses

- 200 OK — empty body (handler sets status only)

Example (HTTP 200):

```
<empty body, HTTP 200>
```

Response Schema: no JSON body.

---

### POST /v1/auth/register/

- Auth: not required
- Handler: `UserHandler.HandleRegister`

Responses

- 201 Created — success

Example (201):

```json
{
  "user": {
    "id": 1,
    "name": "Alice",
    "email": "alice@example.com",
    "role": "user",
    "is_active": true,
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "0001-01-01T00:00:00Z"
  }
}
```

- 400 Bad Request — invalid JSON, invalid email, or duplicate

Examples (400):

```json
{ "error": "invalid request payload" }
```

```json
{ "error": "invalid email" }
```

```json
{ "error": "Email is already exists" }
```

- 500 Internal Server Error — hashing or DB

```json
{ "error": "internal server error" }
```

Response Schema

- `user` (object)
  - `id` integer (non-null)
  - `name` string (non-null)
  - `email` string (non-null)
  - `role` string (non-null)
  - `is_active` boolean (non-null)
  - `created_at` string (RFC3339, non-null)
  - `updated_at` string (RFC3339, non-null) — zero-value if not set

---

### POST /v1/auth/login/

- Auth: not required
- Handler: `TokenHandler.LoginHandler`

Responses

- 200 OK — success

Example (200):

```json
{
  "token": "eyJhbGciOi...",
  "name": "Alice",
  "role": "user"
}
```

- 400 Bad Request — invalid payload or missing fields

```json
{ "error": "invalid request payload" }
```

or

```json
{ "error": "email or password is empty" }
```

- 401 Unauthorized — wrong password, inactive, or locked

Examples (401):

```json
{ "error": "unauthorized" }
```

```json
{ "error": "password is incorrect" }
```

```json
{ "error": "user is inactive" }
```

- 500 Internal Server Error — DB error

```json
{ "error": "internal server error" }
```

Response Schema

- `token` string (non-null): JWT
- `name` string (non-null)
- `role` string (non-null)

---

### POST /v1/auth/reset-password/

- Auth: not required
- Handler: `UserHandler.HandleResetPassword`

Responses

- 200 OK — success

```json
{ "message": "password updated" }
```

- 400 Bad Request — invalid request, missing token, mismatched passwords, invalid/expired token

Examples (400):

```json
{ "error": "invalid JSON body" }
```

```json
{ "error": "token is required" }
```

```json
{ "error": "password fields are required" }
```

```json
{ "error": "passwords do not match" }
```

```json
{ "error": "invalid or expired token" }
```

```json
{ "error": "invalid token user" }
```

- 500 Internal Server Error

```json
{ "error": "internal server error" }
```

Response Schema

- `message` string

---

### GET /v1/statuses

- Auth: required (admin only)
- Handler: `StatusHandler.HandleGetAllStatuses`

Responses

- 200 OK — success (admin)

```json
{
  "statuses": [
    { "id": 1, "name": "In Progress" },
    { "id": 2, "name": "Completed" }
  ]
}
```

- 200 OK — empty

```json
{ "statuses": [] }
```

- 403 Forbidden — non-admin

```json
{ "error": "forbidden" }
```

- 500 Internal Server Error

```json
{ "error": "internal server error" }
```

Response Schema

- `statuses` array of objects
  - each: `id` integer, `name` string

---

### GET /v1/projects

- Auth: required
- Handler: `ProjectHandler.HandleListProjects`

Responses

- 200 OK — success

```json
{
  "count": 2,
  "projects": [
    { "id": 1, "name": "Project A", "status": { "id": 1, "name": "In Progress" } },
    { "id": 2, "name": "Project B", "status": { "id": 2, "name": "Completed" } }
  ]
}
```

- 200 OK — empty

```json
{ "count": 0, "projects": [] }
```

- 500 Internal Server Error

```json
{ "error": "internal server error" }
```

Response Schema

- `count` integer
- `projects` array of objects
  - `id` integer
  - `name` string
  - `status` object { `id` integer, `name` string }

---

### POST /v1/projects/

- Auth: required (admin only)
- Handler: `ProjectHandler.HandleCreateProject`

Responses

- 201 Created — success

```json
{
  "message": "project created successfully",
  "project": {
    "project_id": 10,
    "project_name": "New Project",
    "status_id": 1
  }
}
```

- 400 Bad Request — validation

```json
{ "error": "name can't be empty" }
```

```json
{ "error": "status_id must be positive" }
```

- 401 / 403 / 500 variants

```json
{ "error": "unauthorized" }
```

```json
{ "error": "only admin can create a project" }
```

```json
{ "error": "internal server error" }
```

Response Schema

- `message` string
- `project` object
  - `project_id` integer
  - `project_name` string
  - `status_id` integer

---

### PATCH /v1/project/{id}/

- Auth: required (admin only)
- Handler: `ProjectHandler.HandleUpdateProject`

Responses

- 200 OK — success

```json
{ "message": "project updated successfully" }
```

- 400 Bad Request — invalid payload or validation

```json
{ "error": "invalid request payload" }
```

```json
{ "error": "at least one field is required: name or status_id" }
```

- 401 / 403 / 404 / 500 variants

```json
{ "error": "unauthorized" }
```

```json
{ "error": "forbidden" }
```

```json
{ "error": "not found" }
```

```json
{ "error": "internal server error" }
```

Response Schema

- `message` string

---

### PATCH /v1/users/{id}/

- Auth: required
- Handler: `UserHandler.HandleUpdateUser`

Behavior: admin vs regular enforced in handler (see code). Admins can set `role` and `is_active` on any user. Regular users can edit only their own record and cannot set admin-only fields.

Responses

- 200 OK — success, returns updated user object

Example (200):

```json
{
  "user": {
    "id": 3,
    "name": "Carol Updated",
    "email": "carol@example.com",
    "role": "user",
    "is_active": true,
    "created_at": "2024-01-05T08:00:00Z",
    "updated_at": "2024-01-14T11:11:11Z"
  }
}
```

- 400 / 401 / 403 / 404 / 500 error variants. Examples:

```json
{ "error": "invalid request payload" }
```

```json
{ "error": "forbidden" }
```

```json
{ "error": "user not found" }
```

Response Schema

- `user` object same shape as registration response

---

### GET /v1/admin/users/

- Auth: required (admin only)
- Handler: `UserHandler.HandleListUsers`

Responses

- 200 OK — success (list + metadata)

Example (200):

```json
{
  "result": [
    {
      "id": 1,
      "email": "admin@example.com",
      "name": "Admin",
      "role": "admin",
      "is_active": true,
      "created_at": "2024-01-01T10:00:00Z",
      "updated_at": "0001-01-01T00:00:00Z"
    }
  ],
  "metadata": {
    "current_page": 1,
    "page_size": 50,
    "first_page": 1,
    "last_page": 1,
    "total_records": 1
  }
}
```

- 200 OK — empty

```json
{ "result": [], "metadata": {} }
```

- 401 / 403 errors

```json
{ "error": "unauthorized" }
```

Response Schema

- `result` array of user objects
- `metadata` object (see Metadata description above)

---

### POST /v1/admin/reset-tokens/

- Auth: admin only
- Handler: `UserHandler.HandleGenerateResetToken`

Responses

- 200 OK — success

```json
{
  "reset_token": "eyJhbGci...",
  "expires_at": "2024-01-14T12:34:56Z"
}
```

- 400 / 401 / 404 / 500 variants

```json
{ "error": "invalid JSON body" }
```

```json
{ "error": "email is required" }
```

```json
{ "error": "user not found" }
```

Response Schema

- `reset_token` string
- `expires_at` string (RFC3339)

---

### POST /v1/work-sessions/start/

- Auth: required
- Handler: `WorkSessionHandler.HandleStartSession`

Responses

- 201 Created — success

```json
{
  "session": {
    "id": 123,
    "user_id": 5,
    "project_id": 7,
    "start_at": "2026-01-14T10:00:00Z",
    "end_at": null,
    "note": "Working on feature X",
    "created_at": "2026-01-14T10:00:00Z"
  },
  "status": "active"
}
```

- 400 / 401 / 500 variants. Examples:

```json
{ "error": "project_id must be positive" }
```

```json
{ "error": "you already have one active session.Stop it before starting a new sessions" }
```

Response Schema

- `session` object (`WorkSession`)
  - `id` integer
  - `user_id` integer
  - `project_id` integer
  - `start_at` string (RFC3339)
  - `end_at` string (RFC3339) (nullable)
  - `note` string
  - `created_at` string (RFC3339)
- `status` string

---

### PATCH /v1/work-sessions/stop/{id}/

- Auth: required
- Handler: `WorkSessionHandler.HandleStopSession`

Responses

- 200 OK — success

```json
{ "message": "session stopped", "session_id": 123 }
```

- 400 / 401 / 404 / 500 variants. Examples:

```json
{ "error": "invalid id" }
```

```json
{ "error": "Unauthorized" }
```

```json
{ "error": "no active session" }
```

Response Schema

- `message` string
- `session_id` integer

---

### GET /v1/work-sessions/list/

- Auth: required
- Handler: `WorkSessionHandler.HandleListSessions`

Behavior notes:

- Regular users: response contains only their sessions (handler forces `filter.UserID = myID`).
- Admin users: can request sessions across users using `user_id` query param.

Responses

- 200 OK — admin example (multiple users)

```json
{
  "result": [
    {
      "user": {
        "user_id": 2,
        "name": "User Two",
        "email": "two@example.com",
        "is_active": true
      },
      "project": {
        "id": 5,
        "name": "Project A",
        "status": { "id": 1, "name": "In Progress" }
      },
      "sessions": {
        "id": 201,
        "start_at": "2026-01-10T09:00:00Z",
        "end_at": "2026-01-10T11:00:00Z",
        "note": "Worked on tasks",
        "created_at": "2026-01-10T09:00:00Z"
      },
      "status": "inactive"
    }
  ],
  "metadata": {
    "current_page": 1,
    "page_size": 50,
    "first_page": 1,
    "last_page": 1,
    "total_records": 1
  }
}
```

- 200 OK — regular user example (own sessions)

```json
{
  "result": [
    {
      "user": {
        "user_id": 5,
        "name": "Regular User",
        "email": "reg@example.com",
        "is_active": true
      },
      "project": {
        "id": 7,
        "name": "Project X",
        "status": { "id": 2, "name": "Completed" }
      },
      "sessions": {
        "id": 301,
        "start_at": "2026-01-12T08:30:00Z",
        "end_at": null,
        "note": "Ongoing work",
        "created_at": "2026-01-12T08:30:00Z"
      },
      "status": "active"
    }
  ],
  "metadata": {
    "current_page": 1,
    "page_size": 50,
    "first_page": 1,
    "last_page": 1,
    "total_records": 1
  }
}
```

- 200 OK — empty

```json
{ "result": [], "metadata": {} }
```

- Errors (400 / 401 / 500) — examples:

```json
{ "error": "invalid user_id" }
```

```json
{ "error": "active must be true or false" }
```

Response Schema

- `result` array of `WorkSessionRow` objects
  - `user` (`UserResponse`): `user_id` int, `name` string, `email` string, `is_active` bool
  - `project` (`ProjectRow`): `id` int, `name` string, `status` { `id` int, `name` string }
  - `sessions` (`WorkSessionResponse`): `id` int, `start_at` string, `end_at` string (nullable), `note` string, `created_at` string
  - `status` string (`active`/`inactive`)
- `metadata` object: see Metadata

---

### GET /v1/work-sessions/reports/

- Auth: required
- Handler: `WorkSessionHandler.HandleGetSummaryReport`
- Query params (required): `from`, `to` (RFC3339 or `YYYY-MM-DD`). Optional: `project_id`. `user_id` accepted but enforced by admin-only behavior in handler.

Behavior summary:

- The store returns a `SummaryReport` that always contains `from`, `to`, `filters`, and `overall`.
- The report includes `users[]` (each `UserSummary` contains `projects[]`) and `projects[]` (array of `ProjectSummary`) where applicable.

⚠️ Response shape inferred – verify `projects[]` top-level presence if your client depends on it (store guarantees `users[]`).

Responses

#### Admin Response (200 OK) — grouped by user → projects

```json
{
  "report": {
    "from": "2026-01-01",
    "to": "2026-01-07",
    "filters": { "user_id": null, "project_id": null },
    "overall": { "total_sessions": 10, "total_durations": "0 days, 42:15:30" },
    "users": [
      {
        "user_id": 2,
        "user_name": "User Two",
        "user_email": "two@example.com",
        "is_active": true,
        "total_sessions": 6,
        "total_durations": "0 days, 26:00:00",
        "projects": [
          {
            "project_id": 5,
            "project_name": "Project A",
            "status": "In Progress",
            "total_sessions": 4,
            "total_durations": "0 days, 18:00:00",
            "users": [
              {
                "user_id": 2,
                "user_name": "User Two",
                "user_email": "two@example.com",
                "is_active": true,
                "total_sessions": 4,
                "total_durations": "0 days, 18:00:00",
                "projects": []
              }
            ]
          }
        ]
      }
    ],
    "projects": []
  }
}
```

Explanation (admin): `users[]` lists users with completed sessions in the date range; each `UserSummary.projects[]` lists the projects that user worked on, and nested `ProjectSummary.users[]` may contain per-user breakdowns for that project.

#### Regular User Response (200 OK) — single user

```json
{
  "report": {
    "from": "2026-01-01",
    "to": "2026-01-07",
    "filters": { "user_id": 5, "project_id": null },
    "overall": { "total_sessions": 3, "total_durations": "0 days, 08:30:00" },
    "users": [
      {
        "user_id": 5,
        "user_name": "Regular User",
        "user_email": "reg@example.com",
        "is_active": true,
        "total_sessions": 3,
        "total_durations": "0 days, 08:30:00",
        "projects": [
          {
            "project_id": 7,
            "project_name": "Project X",
            "status": "Completed",
            "total_sessions": 2,
            "total_durations": "0 days, 05:00:00",
            "users": []
          },
          {
            "project_id": 9,
            "project_name": "Project Y",
            "status": "In Progress",
            "total_sessions": 1,
            "total_durations": "0 days, 03:30:00",
            "users": []
          }
        ]
      }
    ],
    "projects": []
  }
}
```

Errors

- 400 Bad Request (missing `from`/`to`)

```json
{ "error": "from and to are required" }
```

- 400 Bad Request (invalid date)

```json
{ "error": "invalid from" }
```

- 401 Unauthorized (not authenticated)

```json
{ "error": "unauthorized" }
```

- 500 Internal Server Error

```json
{ "error": "internal server error" }
```

Response Schema (SummaryReport)

- `report` object
  - `from` string (YYYY-MM-DD)
  - `to` string (YYYY-MM-DD)
  - `filters` object
    - `user_id` integer (nullable)
    - `project_id` integer (nullable)
  - `overall` object
    - `total_sessions` integer
    - `total_durations` string (format: `N days, HH:MM:SS`)
  - `users` array of `UserSummary`
    - `user_id` integer
    - `user_name` string
    - `user_email` string
    - `is_active` boolean
    - `total_sessions` integer
    - `total_durations` string
    - `projects` array of `ProjectSummary`
  - `projects` array of `ProjectSummary` (may be empty) — ⚠️ inferred

`ProjectSummary` fields:

- `project_id` integer
- `project_name` string
- `status` string
- `total_sessions` integer
- `total_durations` string
- `users` array of `UserSummary` (optional)

---

## Metadata (pagination)

Common metadata object returned by listing endpoints:

```json
{
  "current_page": 1,
  "page_size": 50,
  "first_page": 1,
  "last_page": 1,
  "total_records": 42
}
```

Field definitions:

- `current_page` integer
- `page_size` integer
- `first_page` integer
- `last_page` integer
- `total_records` integer

---

If you'd like, I can now:

- add `curl` examples for each endpoint (including auth header usage),
- generate an OpenAPI v3 spec from these shapes, or
- produce a Postman/Insomnia collection with these examples.
# Worktime API Documentation

## Overview

Worktime is a RESTful API for tracking work sessions, projects, statuses and users. The service is implemented in Go and uses JWT authentication. This document is a concise, copy-pasteable reference for the public endpoints, request/response shapes, and access rules.

**Base URL:** `http://localhost:4000/v1`

---

## Authentication

The API uses JWT Bearer tokens. Include the token in the `Authorization` header:

```
Authorization: Bearer <token>
```

### Obtain a token

POST `/v1/auth/login/` with JSON body:

```json
{
  "email": "user@example.com",
  "password": "password"
}
```

Response (200):

```json
{
  "token": "<jwt>",
  "name": "User Name",
  "role": "user"
}
```

Notes:
- Failed logins increment a failure counter. After 5 failed attempts the account is locked for 24 hours.

---

## Response Envelope

Handlers use a consistent JSON envelope. Common shapes:

- Error:

```json
{ "error": "message" }
```

- Created / Success object:

```json
{ "user": { ... } }
```

- List with metadata:

```json
{ "result": [ ... ], "metadata": { "current_page": 1, "page_size": 50, "total_records": 123 } }
```

---

## Error codes

- `200` OK
- `201` Created
- `400` Bad Request
- `401` Unauthorized
- `403` Forbidden
- `404` Not Found
- `500` Internal Server Error

---

## Pagination & Filtering (common)

- `page` - integer (default `1`)
- `page_size` - integer (default `50`)
- `sort` - string (default `id`, prefix with `-` for desc)

Metadata example:

```json
{"metadata":{"current_page":1,"page_size":50,"first_page":1,"last_page":3,"total_records":125}}
```

---

## Endpoints

### Health

GET `/v1/health`

- Auth: not required
- Response: `200 OK` (empty body with 200 status)

### Authentication

POST `/v1/auth/register/`

- Auth: not required
- Body:

```json
{ "name": "Alice", "email": "a@b.com", "password": "secret" }
```

- Response: `201 Created`

POST `/v1/auth/login/`

- Auth: not required (see above)

POST `/v1/auth/reset-password/`

- Auth: not required
- Body: `{ "token": "...", "new_password": "...", "confirm_password": "..." }`

---

### Users

PUT/PATCH `/v1/users/{id}/`

- Auth: required
- Purpose: update a user. Non-admins can only update their own user; admins may update any user and set `role` and `is_active`.
- Body (partial fields allowed):

```json
{
  "name": "New Name",
  "email": "new@example.com",
  "old_password": "old",
  "new_password": "new",
  "role": "admin",         // admin-only
  "is_active": true         // admin-only
}
```

- Response: `200 OK` with `user` object.

GET `/v1/admin/users/`

- Auth: required (admin only)
- Query params: `search`, `page`, `page_size`, `sort`, `is_active`, `is_locked`
- Response: list of users + `metadata`.

POST `/v1/admin/reset-tokens/`

- Auth: required (admin only)
- Body: `{ "email": "user@example.com" }` — returns a short-lived reset token and expiry.

---

### Projects

GET `/v1/projects`

- Auth: required
- Response: `200 OK` with `{ "count": N, "projects": [...] }`

POST `/v1/projects/`

- Auth: required (admin only)
- Body:

```json
{ "name": "Project Name", "status_id": 1 }
```

- Response: `201 Created` with created `project` object.

PATCH `/v1/project/{id}/`

- Auth: required (admin only)
- Body: partial update: `{ "name": "...", "status_id": 2 }`

---

### Statuses

GET `/v1/statuses`

- Auth: required (admin only)
- Response: `200 OK` with `{ "statuses": [...] }`

---

### Work Sessions

POST `/v1/work-sessions/start/`

- Auth: required
- Body:

```json
{ "project_id": 123, "note": "Working on X" }
```

- Response: `201 Created` with `{ "session": { ... }, "status": "active" }`

PATCH `/v1/work-sessions/stop/{id}/`

- Auth: required
- Path param: session id
- Stops an active session for the authenticated user (or returns 404 if none).
- Response: `200 OK` `{ "message": "session stopped", "session_id": id }`

GET `/v1/work-sessions/list/`

- Auth: required
- Query params: `search`, `active` (true/false), `project_id`, `user_id` (admin only), `page`, `page_size`, `sort`
- Non-admin users can only list their own sessions; admins can filter by `user_id`.
- Response: `200 OK` with `{ "result": [...], "metadata": {...} }`

GET `/v1/work-sessions/reports/`

- Auth: required
- Query params (required): `from`, `to` (RFC3339 or `YYYY-MM-DD`), optional `project_id`, optional `user_id` (admin only)
- Response: `200 OK` with `{ "report": { ... } }`

---

## Data Models (Representative)

User:

```json
{
  "id": 1,
  "name": "Alice",
  "email": "a@b.com",
  "role": "user",
  "is_active": true
}
```

Project:

```json
{
  "id": 1,
  "project_name": "Project X",
  "status_id": 1
}
```

WorkSession:

```json
{
  "id": 12,
  "user_id": 3,
  "project_id": 5,
  "note": "...",
  "start_at": "2025-01-02T15:04:05Z",
  "end_at": null
}
```

Status:

```json
{ "id": 1, "name": "In Progress" }
```

---

## Rate Limiting & Security

- Account lockout: after 5 failed login attempts a user is locked for 24 hours.
- Passwords are hashed; password reset uses short-lived reset tokens.
- Rate limiter is configurable via env vars: `LIMITER_RPS`, `LIMITER_BURST`, `LIMITER_ENABLED`.

## Development vs Production

- Default server address: `:4000`. Use `ENV` and `SERVER_ADDRESS` to change.
- JWT secret: `JWT_SECRET` env var.


