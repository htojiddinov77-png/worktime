# Worktime Backend (local dev for frontend)

This Go service provides the backend APIs for the Worktime frontend. It runs on a local HTTP server and exposes JSON APIs under `/v1`.

Frontend needs:
- Base URL: `http://localhost:4000` (default)
- Port: `4000` by default, override with `WORKTIME_PORT` or `-port`
- CORS allowed origins: `http://localhost:5173`, `http://localhost:4000`
- Auth: JWT in header `Authorization: Bearer <token>`

## Prerequisites
- Go `1.24.5` (from `go.mod`)
- Local PostgreSQL (running and reachable)
- Goose CLI (for migrations):
  ```bash
  go install github.com/pressly/goose/v3/cmd/goose@v3.26.0
  ```

## Quick start
```bash
git clone https://github.com/htojiddinov77-png/worktime.git
cd worktime

cp .env.example .env

createdb worktime

set -a
source .env
set +a
goose -dir migrations postgres "$WORKTIME_DB_DSN" up

go run .
```

## Environment variables
Only the variables below are read by the app:
- `WORKTIME_PORT` - Port for the HTTP server (default: `4000`)
- `WORKTIME_DB_DSN` - PostgreSQL DSN used by the app and migrations
- `WORKTIME_JWT_SECRET` - Secret for signing JWTs

## Database setup
- Create DB (example):
  ```bash
  createdb worktime
  ```
- Run migrations (uses `WORKTIME_DB_DSN`):
  ```bash
  goose -dir migrations postgres "$WORKTIME_DB_DSN" up
  ```
- Migrations live in `migrations/`

## Common errors and fixes
- `go.mod file not found` -> Run commands from the repo root (`worktime/`).
- `connection refused` / `pq: ...` -> PostgreSQL is not running or DSN is wrong.
- `WORKTIME_DB_DSN is not set` -> Set `WORKTIME_DB_DSN` (see `.env.example`).
- `WORKTIME_JWT_SECRET` missing -> Set `WORKTIME_JWT_SECRET` (app falls back to a default, but set your own for testing).
- `goose: command not found` -> Install Goose (`go install ...` above) and ensure `$GOPATH/bin` is on your `PATH`.

## Frontend notes
- Authorization header example:
  ```
  Authorization: Bearer <jwt>
  ```
- Health check: no dedicated endpoint; use `POST /v1/auth/login/` (it will return `400` or `401` without a valid body, which still confirms the server is responding).
