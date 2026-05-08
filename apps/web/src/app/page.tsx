import Link from "next/link";
import {
  Bot,
  ChevronRight,
  Code2,
  Database,
  GitBranch,
  ImageIcon,
  MessageSquare,
  Search,
  Server,
  Sparkles,
  Star,
  ThumbsUp,
  Wrench,
} from "lucide-react";
import { RepoCard } from "@/components/repo-card";
import { RepoAvatar } from "@/components/repo-visuals";
import { SiteHeader } from "@/components/site-header";
import {
  categoryLabel,
  categoryFilterOptions,
  formatCompactNumber,
  normalizeCategory,
  type Repository,
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
  const selectedLabel = categoryLabel(todayPick.category);
  const sideAlternatives = todayPick.alternatives.slice(0, 3);
  const gridAlternatives = todayPick.alternatives.slice(0, 4);
  const pickStatus =
    todayPick.theme === "自动推荐"
      ? "今天尚未人工发布，先展示机器自动推荐"
      : "机器发现候选，后台审核发布";

  return (
    <main className="min-h-screen overflow-hidden bg-[#fbfdff] text-slate-950">
      <div className="page-soft-grid pointer-events-none fixed inset-0" />
      <SiteHeader />
      <section className="relative">
        <div className="pointer-events-none absolute left-0 top-16 hidden h-72 w-40 border-y border-emerald-100/80 opacity-80 md:block" />
        <div className="pointer-events-none absolute left-0 top-36 hidden h-28 w-52 rounded-r-lg border border-l-0 border-emerald-200/70 md:block" />
        <div className="relative mx-auto grid max-w-[1380px] gap-8 px-5 pb-5 pt-10 lg:grid-cols-[minmax(0,1.35fr)_470px]">
          <div className="pt-3">
            <p className="flex flex-wrap items-center gap-2 text-sm font-bold text-emerald-700">
              <span>{todayPick.date}</span>
              <span className="text-emerald-400">·</span>
              <span>{selectedLabel}</span>
              <span className="text-emerald-400">/</span>
              <span>{todayPick.theme}</span>
            </p>
            <h1 className="mt-3 max-w-[760px] text-[clamp(2.25rem,5vw,4rem)] font-black leading-[1.08] tracking-tight text-slate-950">
              每天发现一个
              <br />
              值得收藏的{" "}
              <span className="bg-gradient-to-r from-emerald-700 to-teal-500 bg-clip-text text-transparent">
                {selectedLabel}项目
              </span>
            </h1>
            <p className="mt-5 max-w-[680px] text-base leading-7 text-slate-600">
              PreHub 把 GitHub 项目采集、评分、审核和推荐串起来，让优质开源项目不再只靠偶然刷到。
            </p>

            <div className="mt-7 flex max-w-[860px] flex-wrap gap-2">
              {categoryFilterOptions.map((item) => (
                <Link
                  key={item.slug}
                  href={`/?category=${item.slug}`}
                  className={[
                    "inline-flex h-9 items-center gap-2 rounded-lg px-3 text-sm font-semibold ring-1 transition",
                    item.slug === selectedCategory
                      ? "bg-gradient-to-r from-emerald-700 to-teal-500 text-white shadow-[0_12px_24px_rgba(16,185,129,0.22)] ring-emerald-500"
                      : "bg-white/85 text-slate-600 ring-slate-200 hover:bg-white hover:text-slate-950",
                  ].join(" ")}
                >
                  <CategoryIcon slug={item.slug} />
                  {item.label}
                </Link>
              ))}
            </div>

            <form
              action="/search"
              className="mt-5 flex max-w-[790px] items-center gap-3 rounded-lg border border-slate-200 bg-white p-2 shadow-[0_18px_50px_rgba(15,23,42,0.08)]"
            >
              <Search className="ml-3 h-5 w-5 shrink-0 text-slate-500" />
              <input
                name="q"
                placeholder="搜索 Next.js CMS、Go CLI、AI agent..."
                className="h-12 min-w-0 flex-1 bg-transparent text-sm text-slate-950 outline-none placeholder:text-slate-400"
              />
              <button className="inline-flex h-11 items-center gap-2 rounded-lg bg-gradient-to-r from-emerald-600 to-teal-600 px-6 text-sm font-bold text-white shadow-[0_10px_24px_rgba(13,148,136,0.22)] transition hover:from-emerald-700 hover:to-teal-700">
                <Search className="h-4 w-4" />
                搜索
              </button>
            </form>
          </div>

          <aside className="min-w-0 rounded-lg border border-slate-200 bg-white/88 p-4 shadow-[0_24px_70px_rgba(15,23,42,0.08)] backdrop-blur-xl">
            <div className="flex items-center justify-between">
              <h2 className="inline-flex items-center gap-2 text-xl font-black text-slate-950">
                <Sparkles className="h-5 w-5 text-emerald-600" />
                今日备选
              </h2>
              <Link
                href="/admin/candidates"
                className="inline-flex items-center gap-1 text-xs font-bold text-slate-500 hover:text-emerald-700"
              >
                查看全部
                <ChevronRight className="h-4 w-4" />
              </Link>
            </div>
            <div className="mt-4 grid gap-4">
              {sideAlternatives.map((repo) => (
                <SideRepoRow key={repo.fullName} repo={repo} />
              ))}
            </div>
            {sideAlternatives.length === 0 ? (
              <div className="mt-4 rounded-lg border border-dashed border-slate-300 bg-white p-4 text-sm text-slate-500">
                暂无备选项目，发布今日推荐时会自动从高分候选中补齐。
              </div>
            ) : null}
          </aside>
        </div>
      </section>

      <section className="relative mx-auto max-w-[1380px] px-5 pb-10 pt-2">
        <div className="rounded-lg border border-slate-200 bg-white/88 p-4 shadow-[0_24px_70px_rgba(15,23,42,0.08)] backdrop-blur-xl">
          <div className="mb-4 flex items-end justify-between gap-4">
            <div>
              <h2 className="inline-flex items-center gap-2 text-xl font-black text-slate-950">
                <ThumbsUp className="h-5 w-5 rounded-full bg-emerald-500 p-1 text-white" />
                今日主推 / {selectedLabel}
              </h2>
              <p className="mt-1 text-sm text-slate-500">{pickStatus}</p>
            </div>
            <Link
              href={`/daily?category=${selectedCategory}`}
              className="inline-flex h-9 items-center gap-1 rounded-lg px-3 text-sm font-bold text-slate-600 hover:bg-slate-100 hover:text-emerald-700"
            >
              查看归档
              <ChevronRight className="h-4 w-4" />
            </Link>
          </div>
          <RepoCard repo={todayPick.primary} highlight />

          <div>
            <h2 className="mt-5 inline-flex items-center gap-2 text-lg font-black text-slate-950">
              <GitBranch className="h-5 w-5 text-emerald-700" />
              备选项目
            </h2>
            <div className="mt-4 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              {gridAlternatives.map((repo) => (
                <MiniRepoCard key={repo.fullName} repo={repo} />
              ))}
            </div>
          </div>
          {gridAlternatives.length === 0 ? (
            <div className="mt-4 rounded-lg border border-slate-200 bg-white p-6 text-sm text-slate-600">
              还没有备选项目。下次发布今日推荐时，系统会自动附带 3 个高分候选。
            </div>
          ) : null}
        </div>
      </section>
    </main>
  );
}

function CategoryIcon({ slug }: { slug: string }) {
  const className = "h-4 w-4";
  switch (slug) {
    case "all":
      return <Sparkles className={className} />;
    case "ai-image":
      return <ImageIcon className={className} />;
    case "ai-prompts":
      return <MessageSquare className={className} />;
    case "ai-skills":
      return <GitBranch className={className} />;
    case "web":
      return <Code2 className={className} />;
    case "devtools":
      return <Wrench className={className} />;
    case "data":
      return <Database className={className} />;
    case "backend":
      return <Server className={className} />;
    case "ai":
    default:
      return <Bot className={className} />;
  }
}

function SideRepoRow({ repo }: { repo: Repository }) {
  return (
    <Link
      href={`/r/${repo.owner}/${repo.name}`}
      className="group flex w-full min-w-0 items-center gap-4 rounded-lg border border-slate-200 bg-white p-4 shadow-sm transition hover:border-emerald-200 hover:shadow-[0_14px_30px_rgba(16,185,129,0.1)]"
    >
      <RepoAvatar repo={repo} />
      <span className="min-w-0 flex-1">
        <span className="block truncate font-mono text-base font-bold text-slate-950 group-hover:text-emerald-700">
          {repo.fullName}
        </span>
        <span className="mt-2 block truncate text-sm text-slate-500">
          {repo.language} <span className="mx-1">·</span> {repo.license}
        </span>
      </span>
      <span className="flex shrink-0 items-center gap-2 border-l border-slate-200 pl-4 text-sm font-semibold text-slate-500">
        <Star className="h-4 w-4" />
        {formatCompactNumber(repo.stars)}
      </span>
    </Link>
  );
}

function MiniRepoCard({ repo }: { repo: Repository }) {
  return (
    <Link
      href={`/r/${repo.owner}/${repo.name}`}
      className="group flex min-h-28 flex-col justify-between rounded-lg border border-slate-200 bg-white p-4 shadow-sm transition hover:border-emerald-200 hover:shadow-[0_14px_30px_rgba(16,185,129,0.1)]"
    >
      <span className="flex gap-3">
        <RepoAvatar repo={repo} size="sm" />
        <span className="min-w-0">
          <span className="block truncate font-mono text-sm font-bold text-slate-950 group-hover:text-emerald-700">
            {repo.fullName}
          </span>
          <span className="mt-1 line-clamp-2 block text-sm leading-5 text-slate-600">
            {repo.description}
          </span>
        </span>
      </span>
      <span className="mt-4 flex items-center justify-between gap-3 text-xs text-slate-500">
        <span className="truncate">
          {repo.language} <span className="mx-1">·</span> {repo.license}
        </span>
        <span className="inline-flex shrink-0 items-center gap-1">
          <Star className="h-3.5 w-3.5" />
          {formatCompactNumber(repo.stars)}
        </span>
      </span>
    </Link>
  );
}
