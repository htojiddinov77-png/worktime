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

---

If you want, I can:

- add `curl` examples for each endpoint,
- export a Postman collection, or
- generate an OpenAPI spec for this surface.
