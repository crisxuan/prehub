import Link from "next/link";
import { SiteHeader } from "@/components/site-header";
import {
  categoryLabel,
  normalizeCategory,
  recommendationCategories,
} from "@/lib/prehub-data";
import { getRecentDailyPicks } from "@/lib/server-prehub-api";

type DailyPageProps = {
  searchParams: Promise<{ category?: string }>;
};

export const dynamic = "force-dynamic";

export default async function DailyPage({ searchParams }: DailyPageProps) {
  const { category } = await searchParams;
  const selectedCategory = normalizeCategory(category);
  const history = await getRecentDailyPicks(7, selectedCategory);

  return (
    <main className="min-h-screen bg-slate-50">
      <SiteHeader />
      <section className="mx-auto max-w-7xl px-5 py-8">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div>
            <h1 className="text-3xl font-semibold text-slate-950">
              近一周 {categoryLabel(history.category)} 推荐
            </h1>
            <p className="mt-2 text-sm text-slate-600">
              {history.fromDate} 至 {history.toDate}，共{" "}
              {history.picks.length} 条已发布推荐。
            </p>
          </div>
          <Link
            href="/admin/candidates"
            className="rounded-md border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-white"
          >
            去审核候选
          </Link>
        </div>

        <div className="mt-5 flex flex-wrap gap-2">
          {recommendationCategories.map((item) => (
            <Link
              key={item.slug}
              href={`/daily?category=${item.slug}`}
              className={[
                "rounded-md px-3 py-2 text-sm font-semibold ring-1 transition",
                item.slug === selectedCategory
                  ? "bg-emerald-600 text-white ring-emerald-600"
                  : "bg-white text-slate-600 ring-slate-200 hover:text-slate-950",
              ].join(" ")}
            >
              {item.label}
            </Link>
          ))}
        </div>

        <div className="mt-6 grid gap-4">
          {history.picks.map((pick) => (
            <article
              key={pick.date}
              className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm"
            >
              <div className="flex flex-wrap items-start justify-between gap-4">
                <div>
                  <p className="text-sm font-semibold text-emerald-700">
                    {pick.date}
                  </p>
                  <h2 className="mt-1 text-xl font-semibold text-slate-950">
                    {pick.theme}
                  </h2>
                  <p className="mt-2 font-mono text-sm font-semibold text-slate-700">
                    {pick.primary.fullName}
                  </p>
                  <p className="mt-2 max-w-3xl text-sm leading-6 text-slate-600">
                    {pick.primary.description}
                  </p>
                </div>
                <Link
                  href={`/r/${pick.primary.owner}/${pick.primary.name}`}
                  className="rounded-md bg-slate-950 px-4 py-2 text-sm font-semibold text-white hover:bg-emerald-700"
                >
                  查看项目
                </Link>
              </div>

              {pick.alternatives.length > 0 ? (
                <div className="mt-4 flex flex-wrap gap-2 border-t border-slate-100 pt-4">
                  {pick.alternatives.map((repo) => (
                    <Link
                      key={repo.fullName}
                      href={`/r/${repo.owner}/${repo.name}`}
                      className="rounded-md bg-slate-100 px-3 py-1.5 font-mono text-xs font-medium text-slate-600 hover:bg-emerald-50 hover:text-emerald-700"
                    >
                      {repo.fullName}
                    </Link>
                  ))}
                </div>
              ) : null}
            </article>
          ))}

          {history.picks.length === 0 ? (
            <div className="rounded-lg border border-slate-200 bg-white p-8 text-sm text-slate-600">
              近一周还没有已发布的每日推荐。可以先去后台审核并发布一个候选项目。
            </div>
          ) : null}
        </div>
      </section>
    </main>
  );
}
