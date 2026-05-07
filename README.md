# PreHub

PreHub is a GitHub project discovery product with a public web experience and an admin console.

## Stack

- Public/Admin web: Next.js + TypeScript
- API and worker: Go
- Database: PostgreSQL + pgvector
- Queue: Postgres job table for MVP

## Local Development

```bash
cp .env.example .env
docker compose up --build
```

Docker maps the app to:

```text
Web: http://localhost:3100
Go API: http://localhost:8080
Postgres: localhost:55432
Redis: localhost:56379
```

The MVP already supports a real recommendation loop:

1. The worker searches GitHub and stores scored candidates in Postgres.
2. Admin users can submit a GitHub repository URL at `/admin/candidates`.
3. Candidate approval and publishing write to `repository_candidates` and `daily_picks`.
4. The public home page, search page, detail page, and daily pick API read from Go API/Postgres through the Next.js BFF.

Quick smoke test:

```bash
curl -sS -X POST http://localhost:3100/api/admin/repositories/submit \
  -H 'content-type: application/json' \
  -d '{"url":"https://github.com/charmbracelet/bubbletea"}'
```

Then open:

```text
Public web: http://localhost:3100
Admin queue: http://localhost:3100/admin/candidates
Today AI API: http://localhost:3100/api/daily-picks/today?category=ai
Recent AI API: http://localhost:3100/api/daily-picks/recent?days=7&category=ai
```

In separate terminals without Docker:

```bash
cd apps/web && npm run dev
```

```bash
cd backend && go run ./cmd/api
```

```bash
cd backend && go run ./cmd/worker
```
