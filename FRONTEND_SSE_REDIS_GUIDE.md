# Frontend Guide: Work Sessions SSE + Redis

This guide describes how real-time work-session updates currently work in this backend.
It is based on the current implementation in:
- `internal/api/work_session_handler.go`
- `internal/router/routes.go`
- `internal/middleware/middleware.go`

## 1. Relevant Routes

Base prefix: `/api/v1`

- `GET /api/v1/events/`
  Opens an SSE stream.
- `POST /api/v1/work-sessions/start/`
  Starts a work session and publishes a Redis event.
- `PATCH /api/v1/work-sessions/stop/{id}/`
  Stops a work session and publishes a Redis event.
- `GET /api/v1/work-sessions/list/`
  Regular REST endpoint for current state (used for initial load and resync).

All routes above are inside the auth middleware group.

## 2. Auth Behavior You Should Assume on Frontend

- Send `Authorization: Bearer <jwt>` for all protected routes, including SSE.
- JWT is returned by `POST /api/v1/auth/login/`.
- Current middleware allows requests without Authorization to pass as anonymous user.
  In practice, frontend must still treat SSE as authenticated and always send token, because work-session actions and data are user-specific and protected elsewhere.

## 3. Redis Role in This Project

Redis is used as a **Stream** (not Pub/Sub channel).

- Stream key: `worktime_events`
- Producer: `publishEvent()` in `work_session_handler.go`
- Command used: `XADD`
- Published on:
  - successful session start
  - successful session stop

Event fields written to Redis:
- `type` (`session_started` or `session_stopped`)
- `session_id`
- `user_id`

## 4. SSE Endpoint Behavior

### Response headers
- `Content-Type: text/event-stream`
- `Cache-Control: no-cache`
- `Connection: keep-alive`

### Initial frame
Server sends an SSE comment on connect:
- `: connected`

This is not a JSON event payload.

### Message frame format
Server sends only `data:` frames (no `event:` field), for example:
- `data: {"session_id":"12","type":"session_started","user_id":"4"}`

Important details:
- Values come from Redis stream map and may arrive as strings in JSON.
- No heartbeat/keepalive interval is emitted after connect.
- No event id (`id:`) is emitted.
- No replay cursor is exposed to frontend.

## 5. Delivery Model and Limits (Important for UI)

Current SSE reader uses Redis `XREAD` with `Streams: [worktime_events, "$"]` repeatedly.

Practical implications:
- Stream delivers only events that arrive after each read starts.
- Connection is live-only; it is not a historical event API.
- Reconnect does not resume from missed event id.
- If network disconnect happens, frontend can miss events during downtime.

Because of this, frontend should treat SSE as a real-time signal and rely on REST list endpoints to resync state when needed.

## 6. Event Semantics for Frontend

Current event types:
- `session_started`
- `session_stopped`

Payload fields available now:
- `type`
- `session_id`
- `user_id`

What is not included in SSE payload:
- project details
- note
- timestamps of start/stop
- full session object

So frontend should use SSE event as a trigger and use existing REST endpoints when full data is required.

## 7. Multi-User Visibility

Current SSE stream is global at backend level:
- there is no server-side per-user filtering in `ServeSSE`.
- connected clients can receive events for different users.

Frontend should apply user-aware filtering in UI logic (for example, based on `user_id`) according to product rules and role.

## 8. What Frontend Should Do

- Open SSE connection to `/api/v1/events/` after login.
- Include bearer token on SSE request.
- Parse `data:` payload and branch by `type`.
- Handle reconnect lifecycle for dropped SSE connections.
- On reconnect (or uncertain state), refresh work-session state through `GET /api/v1/work-sessions/list/`.
- Treat SSE as notification channel, not as full source-of-truth payload.
- Filter events in UI according to current user role and user id.

## 9. Optional Packages (No Code Required)

If your frontend client cannot send custom headers with native `EventSource`, use an SSE client package that supports Authorization headers, for example:
- `@microsoft/fetch-event-source`
- `eventsource` (environment-dependent usage)

Choose package based on your runtime (browser-only vs SSR/node).
