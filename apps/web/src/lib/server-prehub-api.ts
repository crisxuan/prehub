import {
  defaultCategory,
  normalizeCategory,
  type AdminOverview,
  type Candidate,
  type DailyPick,
  type DailyPickHistory,
  type RadarMetricsResponse,
  type RadarOverview,
  type RadarTrendItem,
  type Repository,
} from "@/lib/prehub-data";

async function fetchInternalJSON<T>(path: string): Promise<T | null> {
  const baseURL = resolveGoAPIBaseURL();
  if (!baseURL) {
    console.warn("PreHub Go API base URL is not configured", { path });
    return null;
  }

  try {
    const response = await fetch(`${baseURL}${path}`, {
      headers: {
        "x-internal-token": process.env.INTERNAL_API_TOKEN ?? "",
      },
      cache: "no-store",
    });
    if (!response.ok) {
      console.warn("PreHub Go API returned a non-OK response", {
        path,
        status: response.status,
        baseHost: safeHost(baseURL),
      });
      return null;
    }
    return (await response.json()) as T;
  } catch (error) {
    console.warn("PreHub Go API fetch failed", {
      path,
      baseHost: safeHost(baseURL),
      error: error instanceof Error ? error.message : String(error),
    });
    return null;
  }
}

export async function getTodayPick(category = defaultCategory): Promise<DailyPick> {
  const normalized = normalizeCategory(category);
  const payload = await fetchInternalJSON<DailyPick>(
    `/v1/daily-picks/today?category=${encodeURIComponent(normalized)}`,
  );
  return payload ?? unavailableDailyPick(normalized);
}

export async function getRecentDailyPicks(
  days = 7,
  category = defaultCategory,
): Promise<DailyPickHistory> {
  const normalized = normalizeCategory(category);
  const payload = await fetchInternalJSON<DailyPickHistory>(
    `/v1/daily-picks/recent?days=${encodeURIComponent(days)}&category=${encodeURIComponent(normalized)}`,
  );
  if (payload) {
    return payload;
  }
  const today = new Date();
  const from = new Date(today);
  from.setDate(today.getDate() - (days - 1));
  return {
    fromDate: from.toISOString().slice(0, 10),
    toDate: today.toISOString().slice(0, 10),
    days,
    category: normalized,
    picks: [],
  };
}

export async function getSearchResults(query: string) {
  return (
    (await fetchInternalJSON<{
      query: string;
      intent: string[];
      results: Repository[];
    }>(`/v1/search?q=${encodeURIComponent(query)}`)) ?? {
      query,
      intent: [],
      results: [],
    }
  );
}

export async function getRepository(owner: string, repo: string) {
  return (
    (await fetchInternalJSON<Repository>(
      `/v1/repositories/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}`,
    )) ?? null
  );
}

export async function getAdminOverview(): Promise<AdminOverview> {
  return (
    (await fetchInternalJSON<AdminOverview>("/v1/admin/overview")) ??
    unavailableAdminOverview()
  );
}

export async function getCandidates(): Promise<Candidate[]> {
  const payload = await fetchInternalJSON<{ candidates: Candidate[] }>(
    "/v1/admin/candidates",
  );
  return payload?.candidates ?? [];
}

export async function getRadarOverview(
  category = defaultCategory,
  window = "24h",
): Promise<RadarOverview> {
  const normalized = normalizeCategory(category);
  return (
    (await fetchInternalJSON<RadarOverview>(
      `/v1/radar/overview?category=${encodeURIComponent(normalized)}&window=${encodeURIComponent(window)}`,
    )) ?? unavailableRadarOverview(normalized, window)
  );
}

export async function getRadarTrending(
  category = defaultCategory,
  window = "24h",
  limit = 50,
): Promise<RadarTrendItem[]> {
  const normalized = normalizeCategory(category);
  const payload = await fetchInternalJSON<{ items: RadarTrendItem[] }>(
    `/v1/radar/trending?category=${encodeURIComponent(normalized)}&window=${encodeURIComponent(window)}&limit=${encodeURIComponent(limit)}`,
  );
  return payload?.items ?? [];
}

export async function getRadarMetrics(
  owner: string,
  repo: string,
  window = "24h",
): Promise<RadarMetricsResponse | null> {
  const payload = await fetchInternalJSON<RadarMetricsResponse>(
    `/v1/radar/repositories/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}/metrics?window=${encodeURIComponent(window)}`,
  );
  if (payload) {
    return payload;
  }
  return null;
}

function unavailableAdminOverview(): AdminOverview {
  return {
    candidateCount: 0,
    pendingReviewCount: 0,
    scheduledPickCount: 0,
    lastRateLimitStatus: "api-unavailable",
  };
}

function unavailableDailyPick(category: string): DailyPick {
  const today = new Date().toISOString().slice(0, 10);
  return {
    date: today,
    category,
    theme: "等待生产数据库",
    primary: {
      fullName: "prehub/setup-production-database",
      owner: "prehub",
      name: "setup-production-database",
      htmlUrl: "https://github.com/crisxuan/prehub",
      description:
        "Vercel 部署已经就绪，配置托管 PostgreSQL 的 DATABASE_URL 并执行 migrations 后会展示真实推荐。",
      language: "PostgreSQL",
      stars: 0,
      forks: 0,
      license: "internal",
      pushedAt: new Date().toISOString(),
      topics: ["vercel", "postgres", "deployment"],
      reason:
        "线上服务已部署，但还没有连接生产数据库；当前展示部署空态，避免首页 500。",
      caveat:
        "接入 Neon、Supabase 或 Vercel Postgres 后，需要执行 packages/db/migrations 下的 SQL。",
      summary:
        "配置生产 DATABASE_URL、INTERNAL_API_TOKEN、GITHUB_TOKEN，并运行数据库迁移后，PreHub 会恢复真实 GitHub 推荐与 Radar 数据。",
    },
    alternatives: [],
  };
}

function unavailableRadarOverview(
  category: string,
  window: string,
): RadarOverview {
  const now = new Date();
  const startedAt = new Date(now.getTime() - radarWindowMs(window));
  return {
    category,
    window,
    monitoredCount: 0,
    starDelta: 0,
    candidateCount: 0,
    apiHealth: {
      status: "api-unavailable",
    },
    dataCoverage: {
      complete: false,
      observedSince: now.toISOString(),
      observedUntil: now.toISOString(),
      windowStartedAt: startedAt.toISOString(),
    },
    topTrending: [],
    topPotential: [],
    recentEvents: [],
  };
}

function radarWindowMs(window: string) {
  switch (window) {
    case "1h":
      return 60 * 60 * 1000;
    case "7d":
      return 7 * 24 * 60 * 60 * 1000;
    case "30d":
      return 30 * 24 * 60 * 60 * 1000;
    default:
      return 24 * 60 * 60 * 1000;
  }
}

function resolveGoAPIBaseURL() {
  const explicit = process.env.GO_API_URL ?? process.env.API_URL;
  if (explicit) {
    return explicit.replace(/\/$/, "");
  }
  if (process.env.VERCEL_URL) {
    return `https://${process.env.VERCEL_URL}/api-go`;
  }
  return null;
}

function safeHost(rawURL: string) {
  try {
    return new URL(rawURL).host;
  } catch {
    return "invalid-url";
  }
}
