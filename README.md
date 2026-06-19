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

Keep the `change-me` values only for local development. Replace `INTERNAL_API_TOKEN`, `ADMIN_PASSWORD`, `CRON_SECRET`, `SCHEDULER_SECRET`, and `POSTGRES_PASSWORD` before using Compose outside a private local machine.

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
set -a && source .env && set +a
curl -sS -X POST http://localhost:8080/v1/admin/repositories/submit \
  -H 'content-type: application/json' \
  -H "x-internal-token: ${INTERNAL_API_TOKEN}" \
  -d '{"url":"https://github.com/charmbracelet/bubbletea"}'
```

Then open:

```text
Public web: http://localhost:3100
Admin login: http://localhost:3100/admin/login
Admin queue: http://localhost:3100/admin/candidates
Today AI API: http://localhost:3100/api/daily-picks/today?category=ai
Recent AI API: http://localhost:3100/api/daily-picks/recent?days=7&category=ai
```

The admin password is `ADMIN_PASSWORD` from `.env`; the default local value is shown in `.env.example`.

## Daily GitHub Article

Generate a daily Markdown article for the local note workspace:

```bash
make daily-github-article
```

By default this writes `~/Downloads/note/github/YYYY-MM-DD-github-daily.md`.
The generator calls the GitHub API directly, scores fresh repositories, and writes a short, image-heavy "逛逛 GitHub" style article: link first, public image URLs, plain-language summary, and no related-projects section.
The main project is explained through six simple numbered sections so the article stays easy to read while still showing real project understanding. Images are cached under `~/Downloads/note/github/assets/YYYY-MM-DD-owner-repo/`, copied into `GITHUB_DAILY_IMAGE_BED_DIR`, optionally synced with `GITHUB_DAILY_IMAGE_BED_SYNC_COMMAND`, and referenced through `GITHUB_DAILY_IMAGE_BED_BASE_URL`.

The default Makefile image bed is `https://cdn.jsdelivr.net/gh/crisxuan/prehub@main/docs/images/github-daily/...`. In jsDelivr's GitHub syntax, `prehub@main` means "read files from the `main` branch of `crisxuan/prehub`". This is required because the repo default branch is `main`; omitting `@main` may resolve through `master` or an unexpected cached ref.

For no-VPN publishable articles, replace the default with a mainland-accessible image host such as Tencent COS, Aliyun OSS, Qiniu, UpYun, or a domain/CDN you control. GitHub/jsDelivr remains a convenient Git-backed image bed, but it is not a reliable no-VPN delivery path in China.

To also create a shared Feishu Docs copy, enable the optional Feishu sync. The first version imports the generated Markdown as a new Feishu `docx` document and keeps the article's public image URLs as-is:

```dotenv
GITHUB_DAILY_FEISHU_SYNC=true
GITHUB_DAILY_FEISHU_SYNC_MODE=cli
GITHUB_DAILY_FEISHU_APP_ID=cli_xxx
GITHUB_DAILY_FEISHU_APP_SECRET=xxx
GITHUB_DAILY_FEISHU_FOLDER_TOKEN=fldxxx
```

Put those values in `.env.local`, then run:

```bash
make daily-github-article
```

`GITHUB_DAILY_FEISHU_SYNC_MODE=cli` uses the local `lark-cli` user login to import the Markdown into Feishu. Run `lark-cli auth login --domain drive,docs` once before enabling the daily automation. `GITHUB_DAILY_FEISHU_SYNC_MODE=api` keeps the app-identity API path: get `tenant_access_token`, upload the Markdown file, create an import task, and poll until the document URL is returned.

The target folder must be writable by the configured Feishu identity. Optional settings:

```bash
GITHUB_DAILY_FEISHU_DOC_NAME='今日 Github 推荐'
GITHUB_DAILY_FEISHU_IMPORT_TIMEOUT=2m
GITHUB_DAILY_FEISHU_POLL_INTERVAL=2s
```

The Go generator does not call an LLM. If a richer Codex/LLM-assisted article is needed, generate the compact brief first and use that as the writing context instead of pasting the full README or previous article draft:

```bash
make daily-github-article-brief
```

Useful overrides:

```bash
GITHUB_TOKEN=... make daily-github-article
GITHUB_DAILY_OUTPUT_DIR=/Users/lx/Downloads/note/github make daily-github-article
GITHUB_DAILY_IMAGE_BED_DIR=/Users/lx/Downloads/github-daily-images GITHUB_DAILY_IMAGE_BED_BASE_URL=https://img.example.com GITHUB_DAILY_IMAGE_BED_PATH=github-daily GITHUB_DAILY_IMAGE_BED_SYNC_COMMAND='ossutil sync . oss://example-bucket --update' make daily-github-article
GITHUB_DAILY_IMAGE_BED_BASE_URL=https://cdn.jsdelivr.net/gh/crisxuan/prehub@main GITHUB_DAILY_IMAGE_BED_PUSH=true GITHUB_DAILY_ALLOW_GITHUB_IMAGE_HOST=true make daily-github-article
cd backend && go run ./cmd/dailyarticle --category ai --force
cd backend && go run ./cmd/dailyarticle --category ai --brief
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
