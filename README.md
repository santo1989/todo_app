# Daily Notes App (Go + Gin)

This project has been upgraded from PHP to a Go backend with a dynamic drag-and-drop daily notes board.

## Features

- Go backend using Gin framework
- SQLite database (`daily_notes.db`) with automatic initialization
- User authentication (register, login, logout)
- Per-user daily notes by selected date
- Drag-and-drop notes across statuses: `To Do`, `In Progress`, `Done`
- Reordering persisted via API
- Create, edit, and delete notes from the UI

## Run

1. Install Go (1.22+ recommended)
2. In project root, run:

```bash
go mod tidy
go run .
```

3. Open: `http://localhost:8080`

## Docker Run

Use Docker Compose to build and start the app with a persistent SQLite volume:

```bash
docker compose up --build
```

Open: `http://localhost:8080`

## API

- `GET /api/auth/me`
- `POST /api/auth/register`
- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/notes?date=YYYY-MM-DD`
- `POST /api/notes`
- `PATCH /api/notes/:id`
- `PATCH /api/notes/reorder`
- `DELETE /api/notes/:id`

## Notes

- Legacy PHP files were removed.
- Static assets are served from `Assets/` and HTML template from `templates/`.
