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

## Daily GitHub Article

Generate a daily Markdown article for the local note workspace:

```bash
make daily-github-article
```

By default this writes `~/Downloads/note/github/YYYY-MM-DD-github-daily.md`.
The generator calls the GitHub API directly, scores fresh repositories, and writes a short, image-heavy "逛逛 GitHub" style article: link first, GitHub picbed image URLs, plain-language summary, and no related-projects section.
The main project is explained through six simple numbered sections so the article stays easy to read while still showing real project understanding. Images are cached under `~/Downloads/note/github/assets/YYYY-MM-DD-owner-repo/`, copied into this public GitHub repository under `docs/images/github-daily/`, committed, pushed, and referenced through `https://cdn.jsdelivr.net/gh/crisxuan/prehub@main/...`.

Useful overrides:

```bash
GITHUB_TOKEN=... make daily-github-article
GITHUB_DAILY_OUTPUT_DIR=/Users/lx/Downloads/note/github make daily-github-article
GITHUB_DAILY_IMAGE_BED_DIR=/Users/lx/Downloads/picbed GITHUB_DAILY_IMAGE_BED_BASE_URL=https://cdn.jsdelivr.net/gh/doggaifan/picbed GITHUB_DAILY_IMAGE_BED_PATH= make daily-github-article
cd backend && go run ./cmd/dailyarticle --category ai --force
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

## Vercel Deployment

PreHub can be deployed to Vercel with Services:

- Next.js web service: `apps/web` at `/`
- Go API service: `backend` at `/api-go`
- Radar backfill cron: `/api/cron/radar-backfill`

See [docs/vercel-deployment.md](docs/vercel-deployment.md) for project settings, required environment variables, migration steps, and cron notes.
