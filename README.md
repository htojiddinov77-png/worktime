# Worktime API

A RESTful API for tracking work sessions and managing projects, built with Go.

## Overview

Worktime is a time-tracking application that allows users to log work sessions against projects. It provides features for user management, project management, session tracking, and reporting.

## Features

- User authentication and authorization (JWT-based)
- Project management
- Work session tracking (start/stop sessions)
- Time reporting and analytics
- Admin functionality for user management
- Password reset functionality
- CORS support for frontend integration

## Technology Stack

- **Language**: Go 1.24.5
- **Web Framework**: Chi v5
- **Database**: PostgreSQL
- **Authentication**: JWT (JSON Web Tokens)
- **Migrations**: Goose v3
- **Environment**: godotenv for local development

## Project Structure

```
worktime/
├── main.go                 # Application entry point
├── go.mod                  # Go module dependencies
├── internal/
│   ├── api/                # HTTP handlers
│   │   ├── user_handler.go
│   │   ├── project_handler.go
│   │   ├── work_session_handler.go
│   │   └── token_handler.go
│   ├── app/
│   │   └── app.go          # Application setup
│   ├── auth/
│   │   └── jwt.go          # JWT management
│   ├── config/
│   │   └── config.go       # Configuration management
│   ├── middleware/
│   │   └── middleware.go   # HTTP middleware
│   ├── router/
│   │   └── routes.go       # Route definitions
│   ├── store/              # Data access layer
│   │   ├── database.go
│   │   ├── user_store.go
│   │   ├── project.go
│   │   ├── work_session.go
│   │   └── filters.go
│   └── utils/
│       └── utils.go        # Utility functions
└── migrations/             # Database migrations
    ├── 00001_users.sql
    ├── 00002_statuses.sql
    ├── 00003_projects.sql
    ├── 00004_work_sessions.sql
    ├── 00005_unique_active_session.sql
    └── 00006_add_user_lockout.sql
```

## Database Schema

### Users
- `id`: Primary key
- `name`: User's full name
- `email`: Unique email address
- `password_hash`: Hashed password
- `is_active`: Account activation status
- `role`: User role (default: 'user')
- `created_at`, `updated_at`: Timestamps

### Statuses
- `id`: Primary key
- `name`: Status name (active/inactive)

### Projects
- `id`: Primary key
- `name`: Project name
- `status_id`: Foreign key to statuses
- `created_at`: Timestamp

### Work Sessions
- `id`: Primary key
- `user_id`: Foreign key to users
- `project_id`: Foreign key to projects (nullable)
- `start_at`: Session start time
- `end_at`: Session end time (nullable)
- `note`: Optional session notes
- `created_at`: Timestamp

## API Endpoints

### Health Check
- `GET /health` - Health check endpoint

### Authentication (Public)
- `POST /v1/auth/register/` - Register a new user
- `POST /v1/auth/login/` - User login
- `POST /v1/auth/reset-password/` - Reset user password

### Projects (Protected)
- `GET /v1/projects` - List user's projects
- `PATCH /v1/project/{id}/` - Update a project

### Work Sessions (Protected)
- `POST /v1/work-sessions/start/` - Start a new work session
- `PATCH /v1/work-sessions/stop/{id}/` - Stop a work session
- `GET /v1/work-sessions/list/` - List user's work sessions
- `GET /v1/work-sessions/reports/` - Get summary reports

### Users (Protected)
- `PATCH /v1/users/{id}/` - Update user information

### Admin (Protected, Admin Only)
- `POST /v1/admin/reset-tokens/` - Generate password reset tokens
- `GET /v1/admin/users/` - List all users
- `POST /v1/projects/` - Create a new project

## Authentication

The API uses JWT (JSON Web Tokens) for authentication. Include the JWT token in the `Authorization` header for protected endpoints:

```
Authorization: Bearer <your-jwt-token>
```

## Configuration

The application uses environment variables for configuration:

### Required
- `WORKTIME_DB_DSN`: PostgreSQL connection string
- `JWT_SECRET`: Secret key for JWT signing

### Optional
- `WORKTIME_PORT`: Server port (default: 4000)
- `ENV`: Environment (development/production)
- `SERVER_ADDRESS`: Server address (default: :4000)
- `LIMITER_RPS`: Rate limiter requests per second (default: 2)
- `LIMITER_BURST`: Rate limiter burst size (default: 4)
- `LIMITER_ENABLED`: Enable rate limiting (default: true)

## Setup and Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/htojiddinov77-png/worktime.git
   cd worktime
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Set up environment variables**
   Create a `.env` file in the root directory:
   ```
   WORKTIME_DB_DSN=postgres://user:password@localhost:5432/worktime?sslmode=disable
   JWT_SECRET=your-secret-key-here
   ```

4. **Set up the database**
   - Create a PostgreSQL database named `worktime`
   - Run migrations:
     ```bash
     go run main.go migrate
     ```

5. **Run the application**
   ```bash
   go run main.go
   ```

   Or with a custom port:
   ```bash
   go run main.go -port 8080
   ```

## Development

### Running Tests
```bash
go test ./...
```

### Database Migrations
To create a new migration:
```bash
goose -dir migrations postgres "your-connection-string" create migration_name sql
```

To run migrations:
```bash
go run main.go migrate
```

## API Usage Examples

### Register a User
```bash
curl -X POST http://localhost:4000/v1/auth/register/ \
  -H "Content-Type: application/json" \
  -d '{
    "name": "John Doe",
    "email": "john@example.com",
    "password": "securepassword"
  }'
```

### Login
```bash
curl -X POST http://localhost:4000/v1/auth/login/ \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john@example.com",
    "password": "securepassword"
  }'
```

### Start a Work Session
```bash
curl -X POST http://localhost:4000/v1/work-sessions/start/ \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "project_id": 1,
    "note": "Working on feature X"
  }'
```

### Get Summary Report
```bash
curl -X GET "http://localhost:4000/v1/work-sessions/reports/?from=2024-01-01&to=2024-12-31" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

## CORS Configuration

The API includes CORS middleware allowing requests from:
- `http://localhost:5173` (Vite dev server)
- `http://localhost:3000` (React dev server)

## Error Handling

The API returns JSON responses with appropriate HTTP status codes. Error responses follow this format:

```json
{
  "error": "Error message description"
}
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the MIT License.</content>
<parameter name="filePath">c:\worktime\README.md