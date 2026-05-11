import Link from "next/link";
import {
  Bot,
  Database,
  GitBranch,
  ImageIcon,
  MessageSquare,
  Radio,
  Sparkles,
  Wrench,
  Code2,
  Server,
} from "lucide-react";
import { RadarTrendBoard } from "@/components/radar-trend-board";
import { SiteHeader } from "@/components/site-header";
import {
  categoryLabel,
  categoryFilterOptions,
  formatCompactNumber,
  normalizeCategory,
} from "@/lib/prehub-data";
import {
  getRadarOverview,
  getRadarMetrics,
} from "@/lib/server-prehub-api";

type RadarPageProps = {
  searchParams: Promise<{ category?: string; window?: string }>;
};

const radarWindows = ["1h", "24h", "7d", "30d"] as const;

export const dynamic = "force-dynamic";

export default async function RadarPage({ searchParams }: RadarPageProps) {
  const { category, window } = await searchParams;
  const selectedCategory = normalizeCategory(category);
  const selectedWindow = normalizeWindow(window);
  const overview = await getRadarOverview(selectedCategory, selectedWindow);
  const firstTrendingRepo = overview.topTrending[0]?.repository;
  const initialMetrics = firstTrendingRepo
    ? await getRadarMetrics(
        firstTrendingRepo.owner,
        firstTrendingRepo.name,
        selectedWindow,
      )
    : null;
  const selectedCategoryLabel = categoryLabel(selectedCategory);
  const starKpiLabel = overview.dataCoverage?.complete
    ? `${selectedWindow} star 增量`
    : "采样 star 增量";
  const dataCoverageValue = overview.dataCoverage?.complete
    ? "完整窗口"
    : `自 ${formatCoverageKpi(overview.dataCoverage?.observedSince)} 起`;
  const dataObservedUntil = formatCoverageKpi(
    overview.dataCoverage?.observedUntil,
  );
  return (
    <main className="min-h-screen overflow-hidden bg-[#fbfdff] text-slate-950">
      <div className="page-soft-grid pointer-events-none fixed inset-0" />
      <SiteHeader />

      <section className="relative mx-auto max-w-[1480px] px-5 pb-8 pt-10">
        <div className="grid gap-6 lg:grid-cols-[minmax(0,1.35fr)_420px]">
          <div>
            <p className="inline-flex items-center gap-2 text-sm font-bold text-emerald-700">
              <Radio className="h-4 w-4" />
              GitHub Radar · {selectedCategoryLabel} ·{" "}
              {selectedWindow}
            </p>
            <h1 className="mt-3 max-w-4xl text-[clamp(2.2rem,4.5vw,4.5rem)] font-black leading-[1.05] tracking-tight text-slate-950">
              发现正在
              <span className="bg-gradient-to-r from-emerald-700 to-teal-500 bg-clip-text text-transparent">
                飙升
              </span>
              的 GitHub 项目
            </h1>
            <p className="mt-5 max-w-3xl text-base leading-7 text-slate-600">
              Radar 会持续采样 stars、forks、issues 和 GitHub 活动，把高增长项目变成可解释的趋势榜，再进入每日推荐候选。
            </p>

            <div className="mt-7 flex flex-wrap gap-2">
              {categoryFilterOptions.map((item) => (
                <Link
                  key={item.slug}
                  href={`/radar?category=${item.slug}&window=${selectedWindow}`}
                  prefetch={false}
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

            <div className="mt-5 flex flex-wrap gap-2">
              {radarWindows.map((item) => (
                <Link
                  key={item}
                  href={`/radar?category=${selectedCategory}&window=${item}`}
                  prefetch={false}
                  className={[
                    "inline-flex h-9 items-center rounded-lg px-4 text-sm font-bold ring-1 transition",
                    item === selectedWindow
                      ? "bg-slate-950 text-white ring-slate-950"
                      : "bg-white text-slate-600 ring-slate-200 hover:text-slate-950",
                  ].join(" ")}
                >
                  {item}
                </Link>
              ))}
            </div>
          </div>

          <aside className="rounded-lg border border-slate-200 bg-white/88 p-4 shadow-[0_24px_70px_rgba(15,23,42,0.08)] backdrop-blur-xl">
            <div className="flex items-center justify-between">
              <h2 className="inline-flex items-center gap-2 text-lg font-black text-slate-950">
                <Sparkles className="h-5 w-5 text-emerald-600" />
                Radar 状态
              </h2>
              <span className="rounded-md bg-emerald-50 px-2.5 py-1 text-xs font-bold text-emerald-700">
                {overview.apiHealth.status}
              </span>
            </div>
            <div className="mt-4 grid grid-cols-2 gap-3">
              <KpiCard label="已监控仓库" value={overview.monitoredCount} />
              <KpiCard label={starKpiLabel} value={`+${formatCompactNumber(overview.starDelta)}`} />
              <KpiCard label="本地候选库" value={overview.candidateCount} />
              <KpiCard label="数据覆盖" value={dataCoverageValue} />
            </div>
            <p className="mt-3 text-xs leading-5 text-slate-500">
              当前是 PreHub 的 Radar 采样池：从本地候选库和后台 watchlist 中挑选仓库持续采样，不代表 GitHub 全量仓库。
            </p>
            <p className="mt-2 text-xs leading-5 text-slate-500">
              数据截至 {dataObservedUntil}；如果调度器没有按时回填，窗口会自动降级为采样统计。
            </p>
            {!overview.dataCoverage?.complete ? (
              <p className="mt-2 text-xs leading-5 text-amber-700">
                {selectedWindow} 历史基线还没攒满，当前增量按已采样区间统计，完整窗口会随着持续采样自动变准。
              </p>
            ) : null}
          </aside>
        </div>
      </section>

      <RadarTrendBoard
        categoryLabel={selectedCategoryLabel}
        initialMetrics={initialMetrics}
        items={overview.topTrending}
        key={`${selectedCategory}:${selectedWindow}`}
        recentEvents={overview.recentEvents}
        selectedWindow={selectedWindow}
      />
    </main>
  );
}

function KpiCard({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded-lg border border-slate-200 bg-white p-4">
      <div className="text-xs font-semibold text-slate-500">{label}</div>
      <div className="mt-2 break-words font-mono text-xl font-black text-slate-950">
        {typeof value === "number" ? formatCompactNumber(value) : value}
      </div>
    </div>
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

function normalizeWindow(window?: string) {
  return radarWindows.some((item) => item === window) ? window! : "24h";
}

function formatCoverageKpi(value?: string) {
  if (!value) {
    return "开始采样";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}
