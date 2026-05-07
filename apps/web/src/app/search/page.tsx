import { RepoCard } from "@/components/repo-card";
import { SiteHeader } from "@/components/site-header";
import { getSearchResults } from "@/lib/server-prehub-api";

type SearchPageProps = {
  searchParams: Promise<{ q?: string }>;
};

export const dynamic = "force-dynamic";

export default async function SearchPage({ searchParams }: SearchPageProps) {
  const { q = "" } = await searchParams;
  const search = await getSearchResults(q);
  const results = search.results;

  return (
    <main className="min-h-screen bg-slate-50">
      <SiteHeader />
      <section className="mx-auto max-w-7xl px-5 py-8">
        <div className="mb-6">
          <h1 className="text-3xl font-semibold text-slate-950">项目搜索</h1>
          <p className="mt-2 max-w-2xl text-sm leading-6 text-slate-600">
            输入自然语言或关键词，结果会展示推荐理由、风险提醒和关键元数据。
          </p>
        </div>
        <form className="flex max-w-3xl gap-3">
          <input
            name="q"
            defaultValue={q}
            placeholder="例如：适合内部工具的 TypeScript 表格库"
            className="min-w-0 flex-1 rounded-md border border-slate-300 bg-white px-4 py-3 text-sm text-slate-950 outline-none transition focus:border-emerald-500 focus:ring-2 focus:ring-emerald-100"
          />
          <button className="rounded-md bg-slate-950 px-5 py-3 text-sm font-semibold text-white transition hover:bg-emerald-700">
            搜索
          </button>
        </form>

        <div className="mt-5 flex flex-wrap gap-2">
          {["repository-discovery", "quality-score", "freshness"].map((chip) => (
            <span
              key={chip}
              className="rounded-md bg-white px-3 py-1.5 text-xs font-medium text-slate-600 ring-1 ring-slate-200"
            >
              {chip}
            </span>
          ))}
        </div>

        <div className="mt-8 grid gap-4">
          {results.map((repo) => (
            <RepoCard key={repo.fullName} repo={repo} />
          ))}
          {results.length === 0 ? (
            <div className="rounded-lg border border-slate-200 bg-white p-8 text-sm text-slate-600">
              没有找到匹配项目。可以换成语言、框架、场景或 topic 继续搜索。
            </div>
          ) : null}
        </div>
      </section>
    </main>
  );
}
