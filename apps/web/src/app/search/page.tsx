import Link from "next/link";
import { AlertCircle, ArrowUpRight, GitBranch, Search, Sparkles } from "lucide-react";
import { RepoCard } from "@/components/repo-card";
import { SiteHeader } from "@/components/site-header";
import { getSearchResults } from "@/lib/server-prehub-api";

type SearchPageProps = {
  searchParams: Promise<{ q?: string; page?: string }>;
};

export const dynamic = "force-dynamic";

export default async function SearchPage({ searchParams }: SearchPageProps) {
  const { q = "", page: pageStr = "" } = await searchParams;
  const query = q.trim();
  const page = parseInt(pageStr, 10) || 1;
  const search = query
    ? await getSearchResults(query, page)
    : { query: "", intent: [], results: [], total: 0, hasMore: false, page: 1, pageSize: 20 };
  const results = search.results;
  const hasQuery = query.length > 0;
  const totalPages = Math.max(1, Math.ceil(search.total / search.pageSize));
  const isBackendError = !!(search as typeof search & { backendError?: boolean }).backendError;
  const isExactMatch = /^https?:\/\/github\.com\//.test(query) || /^[a-zA-Z0-9_.-]+\/[a-zA-Z0-9_.-]+$/.test(query);

  return (
    <main className="min-h-screen bg-[#fbfdff] text-slate-950">
      <SiteHeader />
      <section className="mx-auto max-w-7xl px-5 py-8">
        <div className="rounded-lg border border-slate-200 bg-white/90 p-6 shadow-[0_24px_70px_rgba(15,23,42,0.06)]">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div>
              <p className="inline-flex items-center gap-2 text-sm font-bold text-emerald-700">
                <Sparkles className="h-4 w-4" />
                Search
              </p>
              <h1 className="mt-2 text-3xl font-black text-slate-950">
                找项目，不只搜关键词
              </h1>
              <p className="mt-2 max-w-3xl text-sm leading-6 text-slate-600">
                关键词搜索仅匹配已收录的候选库；输入 owner/repo 或 GitHub URL 可精确拉取任意公开仓库的元数据。
              </p>
            </div>
            <Link
              href="/admin/candidates"
              prefetch={false}
              className="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 text-sm font-bold text-slate-600 hover:text-emerald-700"
            >
              提交仓库
              <ArrowUpRight className="h-4 w-4" />
            </Link>
          </div>

          <form action="/search" className="mt-5 flex flex-col gap-3 md:flex-row">
            <label className="relative min-w-0 flex-1">
              <Search className="pointer-events-none absolute left-4 top-1/2 h-5 w-5 -translate-y-1/2 text-slate-400" />
              <input
                name="q"
                defaultValue={query}
                placeholder="搜索 GitHub URL、owner/repo、AI agent 监控、prompt workflow..."
                className="h-12 w-full min-w-0 rounded-lg border border-slate-200 bg-white pl-12 pr-4 text-sm text-slate-950 outline-none transition focus:border-emerald-500 focus:ring-2 focus:ring-emerald-100"
              />
            </label>
            <button className="inline-flex h-12 items-center justify-center gap-2 rounded-lg bg-slate-950 px-6 text-sm font-bold text-white transition hover:bg-emerald-700">
              <Search className="h-4 w-4" />
              搜索
            </button>
          </form>

          <div className="mt-4 flex flex-wrap gap-2">
            {searchExamples.map((example) => (
              <Link
                key={example.query}
                href={`/search?q=${encodeURIComponent(example.query)}`}
                prefetch={false}
                className="inline-flex items-center gap-2 rounded-lg bg-slate-50 px-3 py-2 text-xs font-semibold text-slate-600 ring-1 ring-slate-200 hover:bg-emerald-50 hover:text-emerald-700"
              >
                {example.kind === "github" ? (
                  <GitBranch className="h-3.5 w-3.5" />
                ) : (
                  <Search className="h-3.5 w-3.5" />
                )}
                {example.label}
              </Link>
            ))}
          </div>
        </div>

        <div className="mt-8">
          <div className="mb-4 flex flex-wrap items-end justify-between gap-3">
            <div>
              <h2 className="text-xl font-black text-slate-950">
                {hasQuery
                  ? isBackendError
                    ? "搜索结果"
                    : `搜索结果 / ${search.total}`
                  : "开始探索"}
              </h2>
              <p className="mt-1 text-sm leading-6 text-slate-500">
                {hasQuery
                  ? isExactMatch
                    ? "精确匹配模式：正在从 GitHub 实时拉取仓库元数据。"
                    : "关键词模式：仅搜索 PreHub 已收录的候选库。如需查找任意 GitHub 仓库，请输入 owner/repo 或完整 GitHub URL。"
                  : "先输入一个 GitHub 仓库地址，或用自然语言描述你要解决的问题。"}
              </p>
            </div>
            {search.intent.length > 0 ? (
              <div className="flex flex-wrap gap-2">
                {search.intent.map((chip) => (
                  <span
                    key={chip}
                    className="rounded-md bg-emerald-50 px-2.5 py-1 text-xs font-bold text-emerald-700"
                  >
                    {chip}
                  </span>
                ))}
              </div>
            ) : null}
          </div>

          <div className="grid gap-4">
            {results.map((repo) => (
              <RepoCard key={repo.fullName} repo={repo} />
            ))}
            {!hasQuery ? <SearchGuide /> : null}
            {hasQuery && isBackendError && results.length === 0 ? (
              <div className="flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 p-8 text-sm leading-6 text-amber-800">
                <AlertCircle className="mt-0.5 h-5 w-5 shrink-0 text-amber-500" />
                <div>
                  <p className="font-semibold">后端服务暂时不可用</p>
                  <p className="mt-1 text-amber-700">
                    搜索后端当前无法响应，请稍后重试。如需查找特定仓库，可以直接输入 owner/repo 或完整 GitHub URL 进行精确查询。
                  </p>
                </div>
              </div>
            ) : null}
            {hasQuery && !isBackendError && results.length === 0 ? (
              <div className="rounded-lg border border-slate-200 bg-white p-8 text-sm leading-6 text-slate-600">
                <p className="font-semibold text-slate-950">未在候选库中找到匹配项目</p>
                <p className="mt-2">
                  关键词搜索仅覆盖 PreHub 已收录的仓库。你可以：
                </p>
                <ul className="mt-2 list-inside list-disc space-y-1">
                  <li>输入 <code className="rounded bg-slate-100 px-1.5 py-0.5 text-xs font-mono">owner/repo</code>（如 <code className="rounded bg-slate-100 px-1.5 py-0.5 text-xs font-mono">vercel/next.js</code>）进行精确查询</li>
                  <li>粘贴完整的 GitHub URL 直接拉取仓库信息</li>
                  <li>尝试更简短的关键词，如 <code className="rounded bg-slate-100 px-1.5 py-0.5 text-xs font-mono">react</code>、<code className="rounded bg-slate-100 px-1.5 py-0.5 text-xs font-mono">go cli</code></li>
                  <li>前往 <Link href="/admin/candidates" className="font-semibold text-emerald-700 hover:underline">候选管理</Link> 提交新仓库</li>
                </ul>
              </div>
            ) : null}
          </div>

          {hasQuery && (search.hasMore || search.page > 1) && (
            <SearchPagination
              page={search.page}
              totalPages={totalPages}
              query={query}
            />
          )}
        </div>
      </section>
    </main>
  );
}

function SearchPagination({ page, totalPages, query }: { page: number; totalPages: number; query: string }) {
  return (
    <div className="mt-6 flex items-center justify-center gap-3">
      {page > 1 ? (
        <Link
          href={`/search?q=${encodeURIComponent(query)}&page=${page - 1}`}
          prefetch={false}
          className="rounded-lg border border-slate-200 bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:text-emerald-700"
        >
          ← 上一页
        </Link>
      ) : (
        <span className="invisible rounded-lg border border-slate-200 bg-white px-4 py-2 text-sm font-semibold text-slate-700">
          ← 上一页
        </span>
      )}

      <span className="text-sm font-medium text-slate-600">
        第 {page} / {totalPages} 页
      </span>

      {page < totalPages ? (
        <Link
          href={`/search?q=${encodeURIComponent(query)}&page=${page + 1}`}
          prefetch={false}
          className="rounded-lg border border-slate-200 bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:text-emerald-700"
        >
          下一页 →
        </Link>
      ) : (
        <span className="invisible rounded-lg border border-slate-200 bg-white px-4 py-2 text-sm font-semibold text-slate-700">
          下一页 →
        </span>
      )}
    </div>
  );
}

const searchExamples = [
  {
    label: "精确仓库 multica-ai/multica",
    query: "https://github.com/multica-ai/multica",
    kind: "github",
  },
  {
    label: "AI agent 成本监控",
    query: "AI agent cost observability dashboard",
    kind: "intent",
  },
  {
    label: "Prompt 工作流",
    query: "self hosted prompt workflow library",
    kind: "intent",
  },
] as const;

function SearchGuide() {
  return (
    <div className="grid gap-4 md:grid-cols-3">
      {[
        ["精确收录", "粘贴 https://github.com/owner/repo 或 owner/repo，PreHub 会实时读取 GitHub 元数据并写入候选库。"],
        ["候选库搜索", "用关键词搜索已收录的项目。关键词仅在 PreHub 本地候选库中匹配，不是 GitHub 全网搜索。"],
        ["场景组合", "把框架、领域、技术栈组合起来，例如 Go CLI AI workflow、react dashboard。"],
      ].map(([title, description]) => (
        <div
          key={title}
          className="rounded-lg border border-slate-200 bg-white p-5"
        >
          <div className="font-bold text-slate-950">{title}</div>
          <p className="mt-2 text-sm leading-6 text-slate-500">{description}</p>
        </div>
      ))}
    </div>
  );
}
