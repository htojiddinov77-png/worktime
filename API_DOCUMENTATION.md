# Worktime API Documentation

## Overview
Worktime is a time tracking API with user authentication, projects, statuses, and work sessions.

## Base URL
`{BASE_URL}/v1`

## Table of Contents
- Authentication
- Token Claims
- Response Format
- Error Handling
- HTTP Status Codes
- Pagination and Filtering
- Endpoints
- Data Models
- Role-Based Access Control
- Rate Limiting and Security

## Authentication
Use a JWT in the `Authorization` header:

`Authorization: Bearer <token>`

### Obtaining Token
Authenticate using `POST /auth/login/` and include the returned token in subsequent requests.

## Token Claims
### Access Token (JWT)
| Field | Type | Description |
| --- | --- | --- |
| user_id | integer | User ID |
| email | string | User email |
| role | string | User role: `user` or `admin` |
| exp | integer | Expiration time (Unix seconds) |

### Reset Token (JWT)
| Field | Type | Description |
| --- | --- | --- |
| user_id | integer | User ID |
| email | string | User email |
| role | string | User role: `user` or `admin` |
| is_active | boolean | User active status at token creation |
| exp | integer | Expiration time (Unix seconds) |

## Response Format
All responses are returned in JSON format.

### Success Response
```json
{
 "message": "ok"
}
```

### Single Object Response
```json
{
 "user": {
  "id": 1,
  "name": "Jane Doe",
  "email": "jane@example.com",
  "role": "user",
  "is_active": true,
  "created_at": "2024-01-01T10:00:00Z",
  "updated_at": "2024-01-01T10:00:00Z"
 }
}
```

### Message Response
```json
{
 "message": "project updated successfully"
}
```

## Error Handling
### Error Response Format
```json
{
 "error": "error message"
}
```

## HTTP Status Codes
| Status code | Description |
| --- | --- |
| 200 | OK |
| 201 | Created |
| 400 | Bad Request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not Found |
| 409 | Conflict |
| 500 | Internal Server Error |

## Pagination and Filtering
### Pagination Parameters
| Parameter | Type | Description |
| --- | --- | --- |
| page | integer | Page number (default: 1) |
| page_size | integer | Items per page (default: 50) |

### Metadata Response
```json
{
 "metadata": {
  "current_page": 1,
  "page_size": 50,
  "first_page": 1,
  "last_page": 10,
  "total_records": 500
 }
}
```

## Endpoints
Base path: `/v1`

| Method | URL | Auth Required |
| --- | --- | --- |
| POST | /auth/register/ | No |
| POST | /auth/login/ | No |
| POST | /auth/reset-password/ | No |
| GET | /statuses | Yes (admin) |
| GET | /projects | Yes |
| POST | /projects/ | Yes (admin) |
| PATCH | /project/{id}/ | Yes (admin) |
| POST | /work-sessions/start/ | Yes |
| PATCH | /work-sessions/stop/{id}/ | Yes |
| GET | /work-sessions/list/ | Yes |
| GET | /work-sessions/reports/ | Yes |
| PATCH | /users/{id}/ | Yes |
| POST | /admin/reset-tokens/ | Yes (admin) |
| GET | /admin/users/ | Yes (admin) |

---

## Authentication Endpoints

### POST /auth/register/
Create a new user.

Request Body:
| Field | Type | Required | Validation |
| --- | --- | --- | --- |
| name | string | Yes | No server-side validation |
| email | string | Yes | Must match email regex |
| password | string | Yes | No server-side validation |

Response: `201 Created`
```json
{
 "user": {
  "id": 1,
  "name": "Jane Doe",
  "email": "jane@example.com",
  "role": "user",
  "is_active": true,
  "created_at": "2024-01-01T10:00:00Z",
  "updated_at": "2024-01-01T10:00:00Z"
 }
}
```

### POST /auth/login/
Authenticate and return a JWT.

Request Body:
| Field | Type | Required | Validation |
| --- | --- | --- | --- |
| email | string | Yes | Must be non-empty |
| password | string | Yes | Must be non-empty |

Response: `200 OK`
```json
{
 "token": "jwt.token.here",
 "name": "Jane Doe",
 "role": "user"
}
```

### POST /auth/reset-password/
Reset a password using a reset token.

Request Body:
| Field | Type | Required | Validation |
| --- | --- | --- | --- |
| token | string | Yes | Must be non-empty |
| new_password | string | Yes | Must be non-empty |
| confirm_password | string | Yes | Must match `new_password` |

Response: `200 OK`
```json
{
 "message": "password updated"
}
```

---

## Status Endpoints

### GET /statuses
List all project statuses (admin-only).

Response: `200 OK`
```json
{
 "statuses": [
  {
   "id": 1,
   "name": "active"
  }
 ]
}
```

---

## Project Endpoints

### GET /projects
List projects.

Response: `200 OK`
```json
{
 "count": 2,
 "projects": [
  {
   "id": 10,
   "name": "Website Redesign",
   "status": {
    "id": 1,
    "name": "active"
   }
  }
 ]
}
```

### POST /projects/
Create a project (admin-only).

Request Body:
| Field | Type | Required | Validation |
| --- | --- | --- | --- |
| name | string | Yes | Must be non-empty |
| status_id | integer | Yes | Must be positive |

Response: `201 Created`
```json
{
 "message": "project created successfully",
 "project": {
  "project_id": 10,
  "project_name": "Website Redesign",
  "status_id": 1
 }
}
```

### PATCH /project/{id}/
Update a project (admin-only).

Request Body:
| Field | Type | Required | Validation |
| --- | --- | --- | --- |
| name | string | No | Must be non-empty |
| status_id | integer | No | Must be positive |

Response: `200 OK`
```json
{
 "message": "project updated successfully"
}
```

---

## Work Session Endpoints

### POST /work-sessions/start/
Start a work session.

Request Body:
| Field | Type | Required | Validation |
| --- | --- | --- | --- |
| project_id | integer | Yes | Must be positive |
| note | string | No | Trimmed |

Response: `201 Created`
```json
{
 "session": {
  "id": 100,
  "user_id": 1,
  "project_id": 10,
  "start_at": "2024-01-01T10:00:00Z",
  "end_at": null,
  "note": "Initial design work",
  "created_at": "2024-01-01T10:00:00Z"
 },
 "status": "active"
}
```

### PATCH /work-sessions/stop/{id}/
Stop a work session.

Response: `200 OK`
```json
{
 "message": "session stopped",
 "session_id": 100
}
```

### GET /work-sessions/list/
List work sessions.

Query Parameters:
| Parameter | Type | Description |
| --- | --- | --- |
| page | integer | Page number |
| page_size | integer | Items per page |
| search | string | Search by project name, user name, email, or note |
| active | boolean | Filter by active status |
| project_id | integer | Filter by project ID |
| user_id | integer | Filter by user ID (admin-only) |

Response: `200 OK`
```json
{
 "result": [
  {
   "user": {
    "user_id": 1,
    "name": "Jane Doe",
    "email": "jane@example.com",
    "is_active": true
   },
   "project": {
    "id": 10,
    "name": "Website Redesign",
    "status": {
     "id": 1,
     "name": "active"
    }
   },
   "sessions": {
    "id": 100,
    "start_at": "2024-01-01T10:00:00Z",
    "end_at": "2024-01-01T12:00:00Z",
    "note": "Initial design work",
    "created_at": "2024-01-01T10:00:00Z"
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

### GET /work-sessions/reports/
Get a summary report.

Query Parameters:
| Parameter | Type | Description |
| --- | --- | --- |
| from | string | Required, `YYYY-MM-DD` or RFC3339 |
| to | string | Required, `YYYY-MM-DD` or RFC3339 |
| project_id | integer | Optional |
| user_id | integer | Optional (admin-only) |

Response: `200 OK`
```json
{
 "report": {
  "from": "2024-01-01",
  "to": "2024-01-31",
  "filters": {
   "user_id": 1,
   "project_id": 10
  },
  "overall": {
   "total_sessions": 12,
   "total_durations": "0 days, 12:30:00"
  },
  "users": [
   {
    "user_id": 1,
    "user_name": "Jane Doe",
    "user_email": "jane@example.com",
    "is_active": true,
    "total_sessions": 12,
    "total_durations": "0 days, 12:30:00",
    "projects": [
     {
      "project_id": 10,
      "project_name": "Website Redesign",
      "status": "active",
      "total_sessions": 12,
      "total_durations": "0 days, 12:30:00"
     }
    ]
   }
  ]
 }
}
```

---

## User Endpoints

### PATCH /users/{id}/
Update user profile. Admins can update any user; normal users can update only themselves.

Request Body:
| Field | Type | Required | Validation |
| --- | --- | --- | --- |
| name | string | No | Must be non-empty |
| email | string | No | Must be valid email |
| old_password | string | No | Required with `new_password` to change password |
| new_password | string | No | Required with `old_password` to change password |
| role | string | No | Admin-only, `user` or `admin` |
| is_active | boolean | No | Admin-only |

Response: `200 OK`
```json
{
 "user": {
  "id": 1,
  "name": "Jane Doe",
  "email": "jane@example.com",
  "role": "user",
  "is_active": true,
  "created_at": "2024-01-01T10:00:00Z",
  "updated_at": "2024-01-02T10:00:00Z"
 }
}
```

### POST /admin/reset-tokens/
Generate a reset token for a user (admin-only).

Request Body:
| Field | Type | Required | Validation |
| --- | --- | --- | --- |
| email | string | Yes | Must be non-empty |

Response: `200 OK`
```json
{
 "reset_token": "jwt.reset.token",
 "expires_at": "2024-01-01T10:10:00Z"
}
```

### GET /admin/users/
List users (admin-only).

Query Parameters:
| Parameter | Type | Description |
| --- | --- | --- |
| search | string | Search by name or email |
| page | integer | Page number |
| page_size | integer | Items per page |
| sort | string | `id`, `email`, `name` (prefix with `-` for desc) |
| is_active | boolean | Filter by active status |
| is_locked | boolean | Filter by lock status |

Response: `200 OK`
```json
{
 "result": [
  {
   "id": 1,
   "name": "Jane Doe",
   "email": "jane@example.com",
   "role": "user",
   "is_active": true,
   "created_at": "2024-01-01T10:00:00Z",
   "updated_at": "2024-01-01T10:00:00Z"
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

---

## Data Models

### User
```json
{
 "id": 1,
 "name": "Jane Doe",
 "email": "jane@example.com",
 "role": "user",
 "is_active": true,
 "created_at": "2024-01-01T10:00:00Z",
 "updated_at": "2024-01-01T10:00:00Z"
}
```

### Project
```json
{
 "project_id": 10,
 "project_name": "Website Redesign",
 "status_id": 1
}
```

### Project Row
```json
{
 "id": 10,
 "name": "Website Redesign",
 "status": {
  "id": 1,
  "name": "active"
 }
}
```

### Status
```json
{
 "id": 1,
 "name": "active"
}
```

### Work Session
```json
{
 "id": 100,
 "user_id": 1,
 "project_id": 10,
 "start_at": "2024-01-01T10:00:00Z",
 "end_at": null,
 "note": "Initial design work",
 "created_at": "2024-01-01T10:00:00Z"
}
```

### Work Session Row
```json
{
 "user": {
  "user_id": 1,
  "name": "Jane Doe",
  "email": "jane@example.com",
  "is_active": true
 },
 "project": {
  "id": 10,
  "name": "Website Redesign",
  "status": {
   "id": 1,
   "name": "active"
  }
 },
 "sessions": {
  "id": 100,
  "start_at": "2024-01-01T10:00:00Z",
  "end_at": "2024-01-01T12:00:00Z",
  "note": "Initial design work",
  "created_at": "2024-01-01T10:00:00Z"
 },
 "status": "inactive"
}
```

### Summary Report
```json
{
 "from": "2024-01-01",
 "to": "2024-01-31",
 "filters": {
  "user_id": 1,
  "project_id": 10
 },
 "overall": {
  "total_sessions": 12,
  "total_durations": "0 days, 12:30:00"
 },
 "users": [
  {
   "user_id": 1,
   "user_name": "Jane Doe",
   "user_email": "jane@example.com",
   "is_active": true,
   "total_sessions": 12,
   "total_durations": "0 days, 12:30:00",
   "projects": [
    {
     "project_id": 10,
     "project_name": "Website Redesign",
     "status": "active",
     "total_sessions": 12,
     "total_durations": "0 days, 12:30:00"
    }
   ]
  }
 ]
}
```

---

## Role-Based Access Control
| Endpoint | User | Admin |
| --- | --- | --- |
| POST /auth/register/ | Yes | Yes |
| POST /auth/login/ | Yes | Yes |
| POST /auth/reset-password/ | Yes | Yes |
| GET /statuses | No | Yes |
| GET /projects | Yes | Yes |
| POST /projects/ | No | Yes |
| PATCH /project/{id}/ | No | Yes |
| POST /work-sessions/start/ | Yes | Yes |
| PATCH /work-sessions/stop/{id}/ | Yes | Yes |
| GET /work-sessions/list/ | Yes | Yes |
| GET /work-sessions/reports/ | Yes | Yes |
| PATCH /users/{id}/ | Self only | Yes |
| POST /admin/reset-tokens/ | No | Yes |
| GET /admin/users/ | No | Yes |

## Rate Limiting and Security
- No explicit rate limiting is implemented.
- Login attempts are locked out after 5 failed attempts, with a 24-hour lock window.
- Password reset tokens expire after 10 minutes.
