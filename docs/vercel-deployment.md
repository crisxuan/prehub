# Vercel Deployment

PreHub can run on Vercel as a multi-service deployment:

- `web`: Next.js app at `/`
- `api`: Go API at `/api-go`
- Vercel Cron: calls the Next.js cron route, which triggers Radar trend backfill in the Go API

The long-running Go worker is not deployed as a persistent process on Vercel. For the first Vercel version, use the HTTP admin endpoints and Vercel Cron for scheduled work. If PreHub later needs continuous queue processing, run the worker on a small external service such as Fly.io, Railway, Render, or a VM.

## Vercel Project Setup

1. Import `crisxuan/prehub` into Vercel.
2. Use the repository root as the project root.
3. Set the Framework Preset to `Services`.
4. Keep the root `vercel.json`; it defines:
   - `apps/web` as the Next.js service.
   - `backend` as the Go service.
   - `/api/cron/radar-backfill` as the scheduled Radar backfill job.

## Required Environment Variables

Set these in Vercel Project Settings:

```text
DATABASE_URL=postgres://...
INTERNAL_API_TOKEN=...
CRON_SECRET=...
GITHUB_TOKEN=...
GITHUB_API_VERSION=2026-03-10
PREHUB_CLICKHOUSE_URL=https://sql-clickhouse.clickhouse.com/
PREHUB_CLICKHOUSE_USER=demo
PREHUB_CLICKHOUSE_PASSWORD=
```

`GO_API_URL` is optional on Vercel Services. Vercel injects `API_URL` for the Go service, and the Next.js BFF uses `GO_API_URL` first, then falls back to `API_URL`.

Use a managed Postgres database with SSL enabled, for example Neon, Supabase, or Vercel Postgres. Redis is not required for the current Vercel MVP path.

## Database Migration

Before the first production deployment is used, apply migrations against the production database:

```bash
for file in packages/db/migrations/*.sql; do
  psql "$DATABASE_URL" -f "$file"
done
```

Do not commit `.env` or database credentials.

## Cron Behavior

`vercel.json` schedules Radar backfill daily by default because Vercel Hobby accounts only support daily cron jobs:

```json
{
  "path": "/api/cron/radar-backfill",
  "schedule": "0 0 * * *"
}
```

This keeps `1h`, `24h`, `7d`, and `30d` trend windows backed by GH Archive / ClickHouse data.

For Pro deployments, change the schedule to `0 * * * *` for hourly backfill or `*/5 * * * *` for tighter Radar freshness. The backend freshness guard keeps stale external windows from being displayed as current data.

## Manual Smoke Checks

After deployment:

```bash
curl -sS https://YOUR_DOMAIN/api/radar/overview?category=all\&window=1h
curl -sS https://YOUR_DOMAIN/api/search?q=https%3A%2F%2Fgithub.com%2Fmultica-ai%2Fmultica
```

The Go API can be checked through the service route:

```bash
curl -sS https://YOUR_DOMAIN/api-go/v1/health
```
