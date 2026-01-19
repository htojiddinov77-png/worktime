# Worktime API Documentation

## Overview
Worktime is a time tracking API with user authentication, projects, statuses, and work sessions.

## Base URL
`http://localhost:4000/v1/`

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
 "name": "Jane Doe",
 "role": "user",
 "token": "jwt.token.here",

}
```

### POST /auth/reset-password/{token}
Reset a password using a reset token.

Request Body:
| Field | Type | Required | Validation |
| --- | --- | --- | --- |
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

### GET /statuses/
List all project statuses (admin-only).

Response: `200 OK`
```json
{
    "statuses": [
        {
            "id": 1,
            "name": "active"
        },
        {
            "id": 2,
            "name": "inactive"
        }
    ]
}
```

---

## Project Endpoints

### GET /projects
List projects.
Admin can see, active sessions.Regular user can't see active_sessions.
Response: `200 OK`
```json
{
    "count": 3,
    "projects": [
        {
            "name": "Cosmos",
            "status": {
                "id": 1,
                "name": "active"
            },
            "total_durations": "25841 minutes",
            "active_sessions": [
                {
                    "id": 1,
                    "user": {
                        "id": 6,
                        "name": "nobody",
                        "email": "nobody@gmail.com"
                    },
                    "start_at": "2026-01-19T11:30:26.919913-05:00",
                    "active_minutes": 12
                }
            ]
        },
        {
            "name": "LLC opening",
            "status": {
                "id": 1,
                "name": "active"
            },
            "total_durations": "20 minutes",
            "active_sessions": []
        },
        {
            "name": "Recruiting",
            "status": {
                "id": 1,
                "name": "active"
            },
            "total_durations": "6986 minutes",
            "active_sessions": []
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
for regular user 
```json
{
    "metadata": {
        "current_page": 1,
        "page_size": 50,
        "first_page": 1,
        "last_page": 1,
        "total_records": 1
    },
    "result": [
        {
            "user": {
                "user_id": 6,
                "name": "nobody",
                "email": "nobody@gmail.com",
                "is_active": true
            },
            "project": {
                "id": 2,
                "name": "LLC opening",
                "status": {
                    "id": 1,
                    "name": "active"
                }
            },
            "sessions": {
                "id": 19,
                "start_at": "2026-01-13T15:05:52.566651-05:00",
                "end_at": "2026-01-13T15:08:31.518004-05:00",
                "note": "opening LLC with Florida",
                "created_at": "2026-01-13T15:05:52.566651-05:00"
            },
            "status": "inactive"
        }
    ]
}
```

for admin
```json
{
    "metadata": {
        "current_page": 1,
        "page_size": 50,
        "first_page": 1,
        "last_page": 1,
        "total_records": 10
    },
    "result": [
        {
            "user": {
                "user_id": 1,
                "name": "Nuriddin",
                "email": "1111@gmail.com",
                "is_active": true
            },
            "project": {
                "id": 3,
                "name": "Recruiting",
                "status": {
                    "id": 1,
                    "name": "active"
                }
            },
            "sessions": {
                "id": 21,
                "start_at": "2026-01-14T15:06:32.363536-05:00",
                "end_at": null,
                "note": "Hiring driver",
                "created_at": "2026-01-14T15:06:32.363536-05:00"
            },
            "status": "active"
        },
        {
            "user": {
                "user_id": 6,
                "name": "nobody",
                "email": "nobody@gmail.com",
                "is_active": true
            },
            "project": {
                "id": 2,
                "name": "LLC opening",
                "status": {
                    "id": 1,
                    "name": "active"
                }
            },
            "sessions": {
                "id": 19,
                "start_at": "2026-01-13T15:05:52.566651-05:00",
                "end_at": "2026-01-13T15:08:31.518004-05:00",
                "note": "opening LLC with Florida",
                "created_at": "2026-01-13T15:05:52.566651-05:00"
            },
            "status": "inactive"
        },
        {
            "user": {
                "user_id": 3,
                "name": "Hasanboy",
                "email": "2222@gmail.com",
                "is_active": true
            },
            "project": {
                "id": 2,
                "name": "LLC opening",
                "status": {
                    "id": 1,
                    "name": "active"
                }
            },
            "sessions": {
                "id": 18,
                "start_at": "2026-01-13T15:02:32.104552-05:00",
                "end_at": "2026-01-13T15:06:32.723786-05:00",
                "note": "opening fucking LLC",
                "created_at": "2026-01-13T15:02:32.104552-05:00"
            },
            "status": "inactive"
        },
        {
            "user": {
                "user_id": 4,
                "name": "Ali",
                "email": "3333@gmail.com",
                "is_active": true
            },
            "project": {
                "id": 2,
                "name": "LLC opening",
                "status": {
                    "id": 1,
                    "name": "active"
                }
            },
            "sessions": {
                "id": 17,
                "start_at": "2026-01-13T15:02:18.995294-05:00",
                "end_at": "2026-01-13T15:06:22.617625-05:00",
                "note": "opening fucking LLC",
                "created_at": "2026-01-13T15:02:18.995294-05:00"
            },
            "status": "inactive"
        },
        {
            "user": {
                "user_id": 4,
                "name": "Ali",
                "email": "3333@gmail.com",
                "is_active": true
            },
            "project": {
                "id": 1,
                "name": "Cosmos",
                "status": {
                    "id": 1,
                    "name": "active"
                }
            },
            "sessions": {
                "id": 7,
                "start_at": "2026-01-13T14:55:47.750331-05:00",
                "end_at": "2026-01-13T15:00:31.373384-05:00",
                "note": "Cosmos",
                "created_at": "2026-01-13T14:55:47.750331-05:00"
            },
            "status": "inactive"
        },
        {
            "user": {
                "user_id": 3,
                "name": "Hasanboy",
                "email": "2222@gmail.com",
                "is_active": true
            },
            "project": {
                "id": 1,
                "name": "Cosmos",
                "status": {
                    "id": 1,
                    "name": "active"
                }
            },
            "sessions": {
                "id": 5,
                "start_at": "2025-12-26T16:34:44.839577-05:00",
                "end_at": "2026-01-13T15:01:21.244038-05:00",
                "note": "",
                "created_at": "2025-12-26T16:34:44.839577-05:00"
            },
            "status": "inactive"
        },
        {
            "user": {
                "user_id": 1,
                "name": "Nuriddin",
                "email": "1111@gmail.com",
                "is_active": true
            },
            "project": {
                "id": 2,
                "name": "LLC opening",
                "status": {
                    "id": 1,
                    "name": "active"
                }
            },
            "sessions": {
                "id": 4,
                "start_at": "2025-12-26T16:22:40.613695-05:00",
                "end_at": "2025-12-26T16:32:35.736038-05:00",
                "note": "",
                "created_at": "2025-12-26T16:22:40.613695-05:00"
            },
            "status": "inactive"
        },
        {
            "user": {
                "user_id": 1,
                "name": "Nuriddin",
                "email": "1111@gmail.com",
                "is_active": true
            },
            "project": {
                "id": 1,
                "name": "Cosmos",
                "status": {
                    "id": 1,
                    "name": "active"
                }
            },
            "sessions": {
                "id": 3,
                "start_at": "2025-12-23T17:34:08.444308-05:00",
                "end_at": "2025-12-23T17:36:50.697734-05:00",
                "note": "",
                "created_at": "2025-12-23T17:34:08.444308-05:00"
            },
            "status": "inactive"
        },
        {
            "user": {
                "user_id": 1,
                "name": "Nuriddin",
                "email": "1111@gmail.com",
                "is_active": true
            },
            "project": {
                "id": 1,
                "name": "Cosmos",
                "status": {
                    "id": 1,
                    "name": "active"
                }
            },
            "sessions": {
                "id": 2,
                "start_at": "2025-12-22T14:33:33.008725-05:00",
                "end_at": "2025-12-22T14:34:59.655623-05:00",
                "note": "working with cosmos again",
                "created_at": "2025-12-22T14:33:33.008725-05:00"
            },
            "status": "inactive"
        },
        {
            "user": {
                "user_id": 1,
                "name": "Nuriddin",
                "email": "1111@gmail.com",
                "is_active": true
            },
            "project": {
                "id": 1,
                "name": "Cosmos",
                "status": {
                    "id": 1,
                    "name": "active"
                }
            },
            "sessions": {
                "id": 1,
                "start_at": "2025-12-22T13:55:46.540247-05:00",
                "end_at": "2025-12-22T14:02:04.611948-05:00",
                "note": "working with cosmos",
                "created_at": "2025-12-22T13:55:46.540247-05:00"
            },
            "status": "inactive"
        }
    ]
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

### POST /admin/reset-tokens/{token}
Generate a reset token for a user (admin-only).

Request Body:
| Field | Type | Required | Validation |
| --- | --- | --- | --- |
| email | string | Yes | Must be non-empty |

Response: `200 OK`
```json
{
 "expires_at": "2024-01-01T10:10:00Z",
 "reset_link": "http://localhost:4000/v1/auth/reset-password/n-qtBuK_I4os",
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
    "metadata": {
        "current_page": 1,
        "page_size": 50,
        "first_page": 1,
        "last_page": 1,
        "total_records": 5
    },
    "result": [
        {
            "id": 1,
            "name": "Nuriddin",
            "email": "1111@gmail.com",
            "role": "admin",
            "is_active": true,
            "created_at": "2025-12-22T13:32:02.198667-05:00",
            "updated_at": "0001-01-01T00:00:00Z"
        },
        {
            "id": 3,
            "name": "Hasanboy",
            "email": "2222@gmail.com",
            "role": "user",
            "is_active": true,
            "created_at": "2025-12-22T14:40:32.880166-05:00",
            "updated_at": "0001-01-01T00:00:00Z"
        },
        {
            "id": 4,
            "name": "Ali",
            "email": "3333@gmail.com",
            "role": "user",
            "is_active": true,
            "created_at": "2025-12-22T14:42:52.469669-05:00",
            "updated_at": "0001-01-01T00:00:00Z"
        },
        {
            "id": 5,
            "name": "",
            "email": "ksjdbfkas@gmail.com",
            "role": "user",
            "is_active": true,
            "created_at": "2025-12-30T14:20:54.149253-05:00",
            "updated_at": "0001-01-01T00:00:00Z"
        },
        {
            "id": 6,
            "name": "nobody",
            "email": "nobody@gmail.com",
            "role": "user",
            "is_active": true,
            "created_at": "2026-01-13T15:04:31.36551-05:00",
            "updated_at": "0001-01-01T00:00:00Z"
        }
    ]
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
