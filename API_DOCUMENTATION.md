# Worktime API Documentation

Base URL: `http://localhost:4000/v1`

Auth: Protected endpoints require `Authorization: Bearer <token>`.

## Health

### GET /health

Request: none  
Response: `200` with empty body

## Auth

### POST /v1/auth/register/

Request:
```json
{ "name": "Alice", "email": "alice@example.com", "password": "secret" }
```

Responses:
```json
// 201
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
```json
// 400
{ "error": "invalid request payload" }
```
```json
// 500
{ "error": "internal server error" }
```

### POST /v1/auth/login/

Request:
```json
{ "email": "alice@example.com", "password": "secret" }
```

Responses:
```json
// 200
{ "token": "jwt", "name": "Alice", "role": "user" }
```
```json
// 400
{ "error": "email or password is empty" }
```
```json
// 401
{ "error": "unauthorized" }
```

### POST /v1/auth/reset-password/

Request:
```json
{ "token": "reset-token", "new_password": "new", "confirm_password": "new" }
```

Responses:
```json
// 200
{ "message": "password updated" }
```
```json
// 400
{ "error": "invalid or expired token" }
```

## Statuses (admin)

### GET /v1/statuses

Request: none  
Responses:
```json
// 200
{ "statuses": [ { "id": 1, "name": "active" } ] }
```
```json
// 403
{ "error": "forbidden" }
```

## Projects

### GET /v1/projects

Request: none  
Responses:
```json
// 200
{
  "count": 1,
  "projects": [
    { "id": 1, "name": "Worktime", "status": { "id": 1, "name": "active" } }
  ]
}
```

### POST /v1/projects/ (admin)

Request:
```json
{ "name": "Worktime", "status_id": 1 }
```

Responses:
```json
// 201
{
  "message": "project created successfully",
  "project": { "project_id": 1, "project_name": "Worktime", "status_id": 1 }
}
```
```json
// 403
{ "error": "only admin can create a project" }
```

### PATCH /v1/project/{id}/ (admin)

Request:
```json
{ "name": "New Name", "status_id": 2 }
```

Responses:
```json
// 200
{ "message": "project updated successfully" }
```
```json
// 400
{ "error": "invalid id" }
```

## Work Sessions

### POST /v1/work-sessions/start/

Request:
```json
{ "project_id": 1, "note": "Daily standup" }
```

Responses:
```json
// 201
{
  "session": {
    "id": 10,
    "user_id": 1,
    "project_id": 1,
    "start_at": "2024-01-15T10:30:00Z",
    "end_at": null,
    "note": "Daily standup",
    "created_at": "2024-01-15T10:30:00Z"
  },
  "status": "active"
}
```
```json
// 400
{ "error": "project_id must be positive" }
```

### PATCH /v1/work-sessions/stop/{id}/

Request: none  
Responses:
```json
// 200
{ "message": "session stopped", "session_id": 10 }
```
```json
// 404
{ "error": "no active session" }
```

### GET /v1/work-sessions/list/

Query params: `page`, `page_size`, `search`, `active`, `project_id`, `user_id` (admin only)

Responses:
```json
// 200
{
  "result": [
    {
      "user": { "user_id": 1, "name": "Alice", "email": "alice@example.com", "is_active": true },
      "project": { "id": 1, "name": "Worktime", "status": { "id": 1, "name": "active" } },
      "sessions": {
        "id": 10,
        "start_at": "2024-01-15T10:30:00Z",
        "end_at": null,
        "note": "Daily standup",
        "created_at": "2024-01-15T10:30:00Z"
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

### GET /v1/work-sessions/reports/

Query params: `from` (required, RFC3339 or YYYY-MM-DD), `to` (required), `project_id`, `user_id` (admin only)

Responses:
```json
// 200
{
  "report": {
    "from": "2024-01-01",
    "to": "2024-01-31",
    "filters": { "user_id": 1, "project_id": 1 },
    "overall": { "total_sessions": 10, "total_durations": "12h30m" },
    "users": [],
    "projects": []
  }
}
```

## Users

### PATCH /v1/users/{id}/

Request:
```json
{
  "name": "Alice",
  "email": "alice@example.com",
  "old_password": "old",
  "new_password": "new",
  "role": "admin",
  "is_active": true
}
```

Responses:
```json
// 200
{
  "user": {
    "id": 1,
    "name": "Alice",
    "email": "alice@example.com",
    "role": "admin",
    "is_active": true,
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-16T10:30:00Z"
  }
}
```
```json
// 403
{ "error": "forbidden" }
```

### GET /v1/admin/users/ (admin)

Query params: `search`, `page`, `page_size`, `sort` (`id`, `email`, `name`; prefix `-` for desc), `is_active`, `is_locked`

Responses:
```json
// 200
{
  "result": [
    { "id": 1, "name": "Alice", "email": "alice@example.com", "role": "user", "is_active": true }
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
```json
// 200 (no users)
{ "result": [], "message": "no user found" }
```

### POST /v1/admin/reset-tokens/ (admin)

Request:
```json
{ "email": "alice@example.com" }
```

Responses:
```json
// 200
{ "reset_token": "token", "expires_at": "2024-01-15T10:40:00Z" }
```
```json
// 404
{ "error": "user not found" }
```
