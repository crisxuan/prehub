export type Repository = {
  fullName: string;
  owner: string;
  name: string;
  htmlUrl?: string;
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

export function normalizeCategory(category?: string) {
  const value = category?.trim().toLowerCase();
  return recommendationCategories.some((item) => item.slug === value)
    ? value!
    : defaultCategory;
}

export function categoryLabel(category: string) {
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

export const repositories: Repository[] = [
  {
    fullName: "vercel/next.js",
    owner: "vercel",
    name: "next.js",
    htmlUrl: "https://github.com/vercel/next.js",
    description: "The React framework for production web applications.",
    language: "TypeScript",
    stars: 133000,
    forks: 28000,
    license: "MIT",
    pushedAt: "2026-05-04T09:30:00Z",
    topics: ["react", "nextjs", "framework", "typescript"],
    reason: "适合构建需要 SEO、服务端渲染和快速迭代的现代 Web 产品。",
    caveat: "框架能力很完整，简单静态站点可能会觉得偏重。",
    summary:
      "Next.js 提供 App Router、Server Components、Route Handlers 和完整的 React 生产应用工具链。",
  },
  {
    fullName: "langchain-ai/langchain",
    owner: "langchain-ai",
    name: "langchain",
    htmlUrl: "https://github.com/langchain-ai/langchain",
    description: "Build context-aware reasoning applications.",
    language: "Python",
    stars: 118000,
    forks: 19000,
    license: "MIT",
    pushedAt: "2026-05-03T16:00:00Z",
    topics: ["ai", "llm", "agents", "python"],
    reason: "适合快速搭建 LLM 应用、工具调用和 agent workflow 原型。",
    caveat: "生态变化快，生产使用前需要锁定版本并评估抽象成本。",
    summary:
      "LangChain 是 LLM 应用开发框架，覆盖模型调用、检索、工具、agent 和工作流编排。",
  },
  {
    fullName: "charmbracelet/bubbletea",
    owner: "charmbracelet",
    name: "bubbletea",
    htmlUrl: "https://github.com/charmbracelet/bubbletea",
    description: "A powerful little TUI framework.",
    language: "Go",
    stars: 32000,
    forks: 900,
    license: "MIT",
    pushedAt: "2026-04-29T12:00:00Z",
    topics: ["go", "cli", "tui", "terminal"],
    reason: "适合用 Go 构建终端工具、交互式 CLI 和开发者效率产品。",
    caveat: "TUI 交互模型需要适应 Elm-style update loop。",
    summary:
      "Bubble Tea 是 Go 生态里成熟的 TUI 框架，常用于构建漂亮的终端应用。",
  },
  {
    fullName: "supabase/supabase",
    owner: "supabase",
    name: "supabase",
    htmlUrl: "https://github.com/supabase/supabase",
    description: "The open source Firebase alternative.",
    language: "TypeScript",
    stars: 89000,
    forks: 9400,
    license: "Apache-2.0",
    pushedAt: "2026-05-01T08:15:00Z",
    topics: ["database", "postgres", "self-hosted", "typescript"],
    reason: "适合想要 Postgres、Auth、Storage 和实时能力的一体化开源后端。",
    caveat: "完整自托管需要理解多个服务组件，运维复杂度不低。",
    summary:
      "Supabase 以 Postgres 为核心，提供数据库、认证、存储、边缘函数和实时订阅能力。",
  },
];

export const todayPick: DailyPick = {
  date: "2026-05-07",
  category: defaultCategory,
  theme: "现代 Web 与开发者工具",
  primary: repositories[0],
  alternatives: repositories.slice(1),
};

export function buildRecentDailyPickHistory(days = 7): DailyPickHistory {
  const toDate = new Date(`${todayPick.date}T00:00:00Z`);
  const fromDate = new Date(toDate);
  fromDate.setUTCDate(toDate.getUTCDate() - (days - 1));

  return {
    fromDate: fromDate.toISOString().slice(0, 10),
    toDate: todayPick.date,
    days,
    category: defaultCategory,
    picks: [todayPick],
  };
}

export const candidates: Candidate[] = [
  {
    id: "cand_nextjs",
    repository: repositories[0],
    status: "pending_review",
    qualityScore: 92,
  },
  {
    id: "cand_langchain",
    repository: repositories[1],
    status: "scored",
    qualityScore: 87,
  },
  {
    id: "cand_bubbletea",
    repository: repositories[2],
    status: "approved",
    qualityScore: 81,
  },
];

export const adminOverview: AdminOverview = {
  candidateCount: candidates.length,
  pendingReviewCount: candidates.filter(
    (candidate) => candidate.status === "pending_review",
  ).length,
  scheduledPickCount: 1,
  lastRateLimitStatus: "not checked",
};

export function searchRepositories(query: string) {
  const normalized = query.trim().toLowerCase();
  if (!normalized) {
    return repositories;
  }

  return repositories.filter((repo) => {
    return [
      repo.fullName,
      repo.description,
      repo.language,
      repo.license,
      ...repo.topics,
    ]
      .join(" ")
      .toLowerCase()
      .includes(normalized);
  });
}

export function findRepository(owner: string, repo: string) {
  return repositories.find(
    (item) => item.fullName.toLowerCase() === `${owner}/${repo}`.toLowerCase(),
  );
}

export function parseGitHubRepositoryURL(rawURL: string) {
  const value = rawURL
    .trim()
    .replace(/\.git$/, "")
    .replace(/^https?:\/\//, "")
    .replace(/^git@/, "")
    .replace(/^github\.com[:/]/, "");

  const [owner, repo] = value.split("/");
  if (!owner || !repo) {
    return null;
  }

  return { owner, repo };
}

export function buildSubmittedCandidate(rawURL: string): Candidate | null {
  const parsed = parseGitHubRepositoryURL(rawURL);
  if (!parsed) {
    return null;
  }

  const known = findRepository(parsed.owner, parsed.repo);
  if (known) {
    return {
      id: `submit_${known.owner}_${known.name}`,
      repository: known,
      status: "pending_review",
      qualityScore: 78,
      source: "admin_submit",
    };
  }

  const repository: Repository = {
    fullName: `${parsed.owner}/${parsed.repo}`,
    owner: parsed.owner,
    name: parsed.repo,
    htmlUrl: `https://github.com/${parsed.owner}/${parsed.repo}`,
    description: "等待 Go API 连接 GitHub 后补全仓库元数据。",
    language: "Unknown",
    stars: 0,
    forks: 0,
    license: "unknown",
    pushedAt: new Date().toISOString(),
    topics: ["submitted"],
    reason: "管理员手动提交，等待采集任务拉取 GitHub 元数据并评分。",
    caveat: "当前为本地回退结果，尚未经过 GitHub API enrichment。",
    summary: "该项目已进入候选提交链路，后续会补全 README 摘要、topics、languages 和评分。",
  };

  return {
    id: `submit_${parsed.owner}_${parsed.repo}`,
    repository,
    status: "pending_review",
    qualityScore: 0,
    source: "admin_submit_fallback",
  };
}

export function formatCompactNumber(value: number) {
  return new Intl.NumberFormat("en", {
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(value);
}

export function githubUrl(repo: Repository) {
  return repo.htmlUrl ?? `https://github.com/${repo.owner}/${repo.name}`;
}
