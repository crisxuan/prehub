.PHONY: dev-web dev-api dev-worker daily-github-article test-web test-go build-web build-go docker-config docker-build-backend

INTERNAL_API_TOKEN ?= change-me
GO_API_URL ?= http://localhost:8080
PORT ?= 8080
GITHUB_DAILY_OUTPUT_DIR ?= $(HOME)/Downloads/note/github
GITHUB_DAILY_IMAGE_BED_DIR ?= $(CURDIR)
GITHUB_DAILY_IMAGE_BED_BASE_URL ?= https://cdn.jsdelivr.net/gh/crisxuan/prehub@main
GITHUB_DAILY_IMAGE_BED_PATH ?= docs/images/github-daily

dev-web:
	cd apps/web && GO_API_URL=$(GO_API_URL) INTERNAL_API_TOKEN=$(INTERNAL_API_TOKEN) npm run dev -- -p 3100

dev-api:
	cd backend && INTERNAL_API_TOKEN=$(INTERNAL_API_TOKEN) PORT=$(PORT) go run ./cmd/api

dev-worker:
	cd backend && INTERNAL_API_TOKEN=$(INTERNAL_API_TOKEN) go run ./cmd/worker

daily-github-article:
	cd backend && GITHUB_DAILY_OUTPUT_DIR="$(GITHUB_DAILY_OUTPUT_DIR)" GITHUB_DAILY_IMAGE_BED_DIR="$(GITHUB_DAILY_IMAGE_BED_DIR)" GITHUB_DAILY_IMAGE_BED_BASE_URL="$(GITHUB_DAILY_IMAGE_BED_BASE_URL)" GITHUB_DAILY_IMAGE_BED_PATH="$(GITHUB_DAILY_IMAGE_BED_PATH)" go run ./cmd/dailyarticle

test-web:
	cd apps/web && npm run lint && npm run build

test-go:
	cd backend && go test ./...

build-web:
	cd apps/web && npm run build

build-go:
	cd backend && go build -o /tmp/prehub-api ./cmd/api && go build -o /tmp/prehub-worker ./cmd/worker

docker-config:
	docker compose config

docker-build-backend:
	docker build backend
