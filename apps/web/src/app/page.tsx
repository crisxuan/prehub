import Link from "next/link";
import { RepoCard } from "@/components/repo-card";
import { SiteHeader } from "@/components/site-header";
import {
  categoryLabel,
  normalizeCategory,
  recommendationCategories,
} from "@/lib/prehub-data";
import { getTodayPick } from "@/lib/server-prehub-api";

type HomeProps = {
  searchParams: Promise<{ category?: string }>;
};

export const dynamic = "force-dynamic";

export default async function Home({ searchParams }: HomeProps) {
  const { category } = await searchParams;
  const selectedCategory = normalizeCategory(category);
  const todayPick = await getTodayPick(selectedCategory);

  return (
    <main className="min-h-screen bg-slate-50">
      <SiteHeader />
      <section className="border-b border-slate-200 bg-white">
        <div className="mx-auto grid max-w-7xl gap-8 px-5 py-10 lg:grid-cols-[1.2fr_0.8fr]">
          <div>
            <p className="text-sm font-semibold text-emerald-700">
              {todayPick.date} / {categoryLabel(todayPick.category)} /{" "}
              {todayPick.theme}
            </p>
            <h1 className="mt-3 max-w-3xl text-4xl font-semibold text-slate-950">
              每天发现一个值得收藏的 {categoryLabel(todayPick.category)} 项目
            </h1>
            <p className="mt-4 max-w-2xl text-base leading-7 text-slate-600">
              PreHub 把 GitHub 项目采集、评分、审核和推荐串起来，让优质开源项目不再只靠偶然刷到。
            </p>
            <div className="mt-5 flex flex-wrap gap-2">
              {recommendationCategories.map((item) => (
                <Link
                  key={item.slug}
                  href={`/?category=${item.slug}`}
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
            <form action="/search" className="mt-7 flex max-w-2xl gap-3">
              <input
                name="q"
                placeholder="搜索 Next.js CMS、Go CLI、AI agent..."
                className="min-w-0 flex-1 rounded-md border border-slate-300 bg-white px-4 py-3 text-sm text-slate-950 outline-none transition focus:border-emerald-500 focus:ring-2 focus:ring-emerald-100"
              />
              <button className="rounded-md bg-slate-950 px-5 py-3 text-sm font-semibold text-white transition hover:bg-emerald-700">
                搜索
              </button>
            </form>
          </div>
          <div className="grid content-start gap-3 rounded-lg border border-slate-200 bg-slate-50 p-5">
            <div className="flex items-center justify-between">
              <span className="text-sm font-semibold text-slate-950">
                今日备选
              </span>
              <Link
                href="/admin/candidates"
                className="text-sm font-medium text-emerald-700 hover:text-emerald-800"
              >
                审核
              </Link>
            </div>
            {todayPick.alternatives.map((repo) => (
              <Link
                key={repo.fullName}
                href={`/r/${repo.owner}/${repo.name}`}
                className="rounded-md border border-slate-200 bg-white p-3 transition hover:border-emerald-300"
              >
                <span className="block font-mono text-sm font-semibold text-slate-950">
                  {repo.fullName}
                </span>
                <span className="mt-1 block text-xs text-slate-500">
                  {repo.language} / {repo.license}
                </span>
              </Link>
            ))}
            {todayPick.alternatives.length === 0 ? (
              <div className="rounded-md border border-dashed border-slate-300 bg-white p-4 text-sm text-slate-500">
                暂无备选项目，发布今日推荐时会自动从高分候选中补齐。
              </div>
            ) : null}
          </div>
        </div>
      </section>

      <section className="mx-auto max-w-7xl px-5 py-8">
        <div className="mb-4 flex items-end justify-between gap-4">
          <div>
            <h2 className="text-xl font-semibold text-slate-950">
              今日主推 / {categoryLabel(todayPick.category)}
            </h2>
            <p className="mt-1 text-sm text-slate-500">
              机器发现候选，后台审核发布。
            </p>
          </div>
          <Link
            href={`/daily?category=${selectedCategory}`}
            className="rounded-md border border-slate-300 px-3 py-2 text-sm font-medium text-slate-700 hover:bg-white"
          >
            查看归档
          </Link>
        </div>
        <RepoCard repo={todayPick.primary} highlight />

        <h2 className="mt-10 text-xl font-semibold text-slate-950">备选项目</h2>
        <div className="mt-4 grid gap-4">
          {todayPick.alternatives.map((repo) => (
            <RepoCard key={repo.fullName} repo={repo} />
          ))}
          {todayPick.alternatives.length === 0 ? (
            <div className="rounded-lg border border-slate-200 bg-white p-6 text-sm text-slate-600">
              还没有备选项目。下次发布今日推荐时，系统会自动附带 3 个高分候选。
            </div>
          ) : null}
        </div>
      </section>
    </main>
  );
}
