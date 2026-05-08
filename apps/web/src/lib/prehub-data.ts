export type Repository = {
  fullName: string;
  owner: string;
  name: string;
  htmlUrl?: string;
  avatarUrl?: string;
  description: string;
  language: string;
  stars: number;
  forks: number;
  license: string;
  pushedAt: string;
  topics: string[];
  reason: string;
  caveat: string;
  summary: string;
};

export type DailyPick = {
  date: string;
  category: string;
  theme: string;
  primary: Repository;
  alternatives: Repository[];
};

export type DailyPickHistory = {
  fromDate: string;
  toDate: string;
  days: number;
  category: string;
  picks: DailyPick[];
};

export const allCategory = "all";
export const defaultCategory = "ai";

export const recommendationCategories = [
  { slug: "ai", label: "AI" },
  { slug: "ai-image", label: "AI 绘图/多模态" },
  { slug: "ai-prompts", label: "Prompt 技巧" },
  { slug: "ai-skills", label: "AI Skills/工作流" },
  { slug: "web", label: "Web 前端" },
  { slug: "devtools", label: "开发工具" },
  { slug: "data", label: "数据与数据库" },
  { slug: "backend", label: "后端基础设施" },
] as const;

export const categoryFilterOptions = [
  { slug: allCategory, label: "全部" },
  ...recommendationCategories,
] as const;

export function normalizeCategory(category?: string) {
  const value = category?.trim().toLowerCase();
  if (value === allCategory) {
    return allCategory;
  }
  return recommendationCategories.some((item) => item.slug === value)
    ? value!
    : defaultCategory;
}

export function categoryLabel(category: string) {
  if (category === allCategory) {
    return "全部";
  }
  return (
    recommendationCategories.find((item) => item.slug === category)?.label ??
    "AI"
  );
}

export type Candidate = {
  id: string;
  repository: Repository;
  status: string;
  qualityScore: number;
  source?: string;
  score?: {
    quality: number;
    popularity: number;
    freshness: number;
    momentum: number;
    documentation: number;
    maintenance: number;
    community: number;
    license: number;
    novelty: number;
  };
};

export type AdminOverview = {
  candidateCount: number;
  pendingReviewCount: number;
  scheduledPickCount: number;
  lastRateLimitStatus: string;
};

export type RadarTrendItem = {
  repository: Repository;
  window: string;
  starDelta: number;
  forkDelta: number;
  issueDelta: number;
  activityEvents: number;
  velocityScore: number;
  accelerationScore: number;
  trendScore: number;
  explanation: string;
  dataCoverage: RadarDataCoverage;
};

export type RadarDataCoverage = {
  complete: boolean;
  observedSince: string;
  observedUntil: string;
  windowStartedAt: string;
};

export type RadarEvent = {
  repositoryFullName: string;
  eventType: string;
  actorLogin: string;
  actorAvatarUrl: string;
  occurredAt: string;
};

export type RadarOverview = {
  category: string;
  window: string;
  monitoredCount: number;
  starDelta: number;
  candidateCount: number;
  apiHealth: {
    status: string;
    remaining?: string;
    resetAt?: string;
  };
  dataCoverage: RadarDataCoverage;
  topTrending: RadarTrendItem[];
  topPotential: RadarTrendItem[];
  recentEvents: RadarEvent[];
};

export type RadarMetricPoint = {
  capturedAt: string;
  stars: number;
  forks: number;
  openIssues: number;
};

export type RadarMetricsResponse = {
  repository: Repository;
  window: string;
  points: RadarMetricPoint[];
  summary: {
    starDelta: number;
    forkDelta: number;
    activityEvents: number;
  };
  dataCoverage: RadarDataCoverage;
};

export function formatCompactNumber(value: number) {
  return new Intl.NumberFormat("en", {
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(value);
}

export function githubUrl(repo: Repository) {
  return repo.htmlUrl ?? `https://github.com/${repo.owner}/${repo.name}`;
}
