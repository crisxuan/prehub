import Link from "next/link";
import { ArrowUpRight, GitBranch, Search, Sparkles } from "lucide-react";
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
                支持 GitHub URL、owner/repo、自然语言场景和技术栈组合；精确仓库会尝试实时拉取 GitHub 元数据并写入候选库。
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
                  ? search.backendError
                    ? "搜索结果"
                    : `搜索结果 / ${search.total}`
                  : "开始探索"}
              </h2>
              <p className="mt-1 text-sm leading-6 text-slate-500">
                {hasQuery
                  ? "结果按本地质量评分、相关度和 GitHub 元数据排序。"
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
            {hasQuery && results.length === 0 && !search.backendError ? (
              <div className="rounded-lg border border-slate-200 bg-white p-8 text-sm leading-6 text-slate-600">
                没有找到匹配项目。优先试试完整 GitHub URL 或 owner/repo；如果仓库是新的、星标很少，也可以先提交到候选队列。
              </div>
            ) : null}
            {hasQuery && search.backendError ? (
              <div className="rounded-lg border border-amber-200 bg-amber-50 p-8 text-sm leading-6 text-amber-800">
                搜索服务暂时不可用，请稍后再试。如果问题持续，请联系管理员检查后端连接。
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
        ["精确收录", "粘贴 https://github.com/owner/repo，PreHub 会尝试实时读取 GitHub 元数据。"],
        ["场景搜索", "描述痛点，比如 agent 调试、RAG 数据治理、AI 绘图工作流。"],
        ["技术组合", "把语言、框架、领域组合起来，例如 Go CLI AI workflow。"],
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
