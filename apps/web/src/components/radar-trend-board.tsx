"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import {
  Activity,
  ArrowUpRight,
  LineChart,
  Star,
} from "lucide-react";
import { RepoAvatar } from "@/components/repo-visuals";
import {
  formatCompactNumber,
  type RadarEvent,
  type RadarMetricPoint,
  type RadarMetricsResponse,
  type RadarTrendItem,
} from "@/lib/prehub-data";

type RadarTrendBoardProps = {
  items: RadarTrendItem[];
  recentEvents: RadarEvent[];
  selectedWindow: string;
  categoryLabel: string;
  initialMetrics: RadarMetricsResponse | null;
};

export function RadarTrendBoard({
  items,
  recentEvents,
  selectedWindow,
  categoryLabel,
  initialMetrics,
}: RadarTrendBoardProps) {
  const [selectedFullName, setSelectedFullName] = useState(
    items[0]?.repository.fullName ?? "",
  );
  const [metricsByRepo, setMetricsByRepo] = useState<
    Record<string, RadarMetricsResponse>
  >(() =>
    initialMetrics ? { [initialMetrics.repository.fullName]: initialMetrics } : {},
  );
  const [loadingRepo, setLoadingRepo] = useState("");
  const [errorRepo, setErrorRepo] = useState("");

  const selectedItem = useMemo(
    () =>
      items.find((item) => item.repository.fullName === selectedFullName) ??
      items[0],
    [items, selectedFullName],
  );
  const selectedMetrics = selectedItem
    ? metricsByRepo[selectedItem.repository.fullName]
    : undefined;
  const hasPartialCoverage = items.some((item) => !item.dataCoverage?.complete);

  function selectItem(item: RadarTrendItem) {
    const metricKey = item.repository.fullName;
    setSelectedFullName(metricKey);
    setErrorRepo("");
    if (!metricsByRepo[metricKey]) {
      setLoadingRepo(metricKey);
    }
  }

  useEffect(() => {
    if (!selectedItem || selectedMetrics) {
      return;
    }
    const controller = new AbortController();
    const repo = selectedItem.repository;
    const metricKey = repo.fullName;

    fetch(
      `/api/radar/repositories/${encodeURIComponent(repo.owner)}/${encodeURIComponent(
        repo.name,
      )}/metrics?window=${encodeURIComponent(selectedWindow)}`,
      { signal: controller.signal },
    )
      .then((response) => {
        if (!response.ok) {
          throw new Error("metrics request failed");
        }
        return response.json() as Promise<RadarMetricsResponse>;
      })
      .then((payload) => {
        setMetricsByRepo((current) => ({
          ...current,
          [metricKey]: payload,
        }));
      })
      .catch((error) => {
        if (error instanceof DOMException && error.name === "AbortError") {
          return;
        }
        setErrorRepo(metricKey);
      })
      .finally(() => {
        setLoadingRepo((current) => (current === metricKey ? "" : current));
      });

    return () => controller.abort();
  }, [selectedItem, selectedMetrics, selectedWindow]);

  return (
    <section className="relative mx-auto grid max-w-[1480px] gap-6 px-5 pb-12 lg:grid-cols-[minmax(0,1.55fr)_420px]">
      <div className="rounded-lg border border-slate-200 bg-white/88 p-4 shadow-[0_24px_70px_rgba(15,23,42,0.08)] backdrop-blur-xl">
        <div className="mb-4 flex flex-wrap items-end justify-between gap-3">
          <div>
            <h2 className="inline-flex items-center gap-2 text-xl font-black text-slate-950">
              <LineChart className="h-5 w-5 text-emerald-700" />
              飙升榜 / {categoryLabel}
            </h2>
            <p className="mt-1 text-sm text-slate-500">
              点击行查看右侧曲线；点击项目名进入详情，点击 GitHub 打开仓库。
            </p>
            {hasPartialCoverage ? (
              <p className="mt-1 text-xs leading-5 text-amber-700">
                部分项目缺少完整 {selectedWindow} 历史基线，标记为 sampled 的数值是已采样区间内的观测增量。
              </p>
            ) : null}
          </div>
          <Link
            href="/admin/radar/watchlist"
            prefetch={false}
            className="inline-flex h-9 items-center gap-2 rounded-lg bg-slate-950 px-3 text-sm font-bold text-white hover:bg-emerald-700"
          >
            加入监控
            <ArrowUpRight className="h-4 w-4" />
          </Link>
        </div>

        <div className="overflow-hidden rounded-lg border border-slate-200 bg-white">
          {items.length > 0 ? (
            <div className="divide-y divide-slate-100">
              {items.map((item, index) => (
                <TrendRow
                  key={item.repository.fullName}
                  item={item}
                  rank={index + 1}
                  selected={item.repository.fullName === selectedItem?.repository.fullName}
                  onSelect={() => selectItem(item)}
                />
              ))}
            </div>
          ) : (
            <EmptyState text="当前分类还没有趋势数据。先从后台 watchlist 添加项目，等待两个采样点后会生成曲线。" />
          )}
        </div>
      </div>

      <div className="grid gap-6 self-start lg:sticky lg:top-28">
        <div className="rounded-lg border border-slate-200 bg-white/88 p-4 shadow-[0_24px_70px_rgba(15,23,42,0.08)] backdrop-blur-xl">
          <h2 className="inline-flex items-center gap-2 text-lg font-black text-slate-950">
            <Star className="h-5 w-5 text-emerald-600" />
            项目曲线预览
          </h2>
          {selectedItem ? (
            <div className="mt-4">
              <div className="flex items-center gap-3">
                <RepoAvatar repo={selectedItem.repository} />
                <div className="min-w-0">
                  <Link
                    href={`/r/${selectedItem.repository.owner}/${selectedItem.repository.name}`}
                    prefetch={false}
                    className="block truncate font-mono text-base font-bold text-slate-950 hover:text-emerald-700"
                  >
                    {selectedItem.repository.fullName}
                  </Link>
                  <p className="mt-1 text-sm text-slate-500">
                    {formatStarDeltaContext(selectedItem.starDelta, selectedWindow, selectedItem.dataCoverage)}
                  </p>
                </div>
              </div>
              {loadingRepo === selectedItem.repository.fullName ? (
                <EmptyState text="正在加载该项目曲线..." />
              ) : errorRepo === selectedItem.repository.fullName ? (
                <EmptyState text="曲线加载失败，稍后刷新再试。" />
              ) : selectedMetrics ? (
                <>
                  {!selectedMetrics.dataCoverage?.complete ? (
                    <p className="mt-3 rounded-lg bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800">
                      这条曲线目前只覆盖自 {formatCoverageSince(selectedMetrics.dataCoverage)} 起的采样数据。
                    </p>
                  ) : null}
                  <Sparkline points={selectedMetrics.points} />
                </>
              ) : (
                <EmptyState text="暂无可绘制曲线的采样点。" />
              )}
            </div>
          ) : (
            <EmptyState text="点击左侧项目查看曲线。" />
          )}
        </div>

        <div className="rounded-lg border border-slate-200 bg-white/88 p-4 shadow-[0_24px_70px_rgba(15,23,42,0.08)] backdrop-blur-xl">
          <h2 className="inline-flex items-center gap-2 text-lg font-black text-slate-950">
            <Activity className="h-5 w-5 text-emerald-600" />
            近期事件
          </h2>
          {recentEvents.length > 0 ? (
            <div className="mt-4 grid gap-3">
              {recentEvents.map((event) => (
                <div
                  key={`${event.repositoryFullName}-${event.eventType}-${event.occurredAt}`}
                  className="rounded-lg border border-slate-200 bg-white p-3 text-sm"
                >
                  <div className="font-mono font-bold text-slate-950">
                    {event.repositoryFullName}
                  </div>
                  <div className="mt-1 text-slate-500">
                    {event.eventType} · {event.actorLogin || "unknown"}
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <EmptyState text="事件流会在接入 repo events / GH Archive 后显示。" />
          )}
        </div>
      </div>
    </section>
  );
}

function TrendRow({
  item,
  rank,
  selected,
  onSelect,
}: {
  item: RadarTrendItem;
  rank: number;
  selected: boolean;
  onSelect: () => void;
}) {
  const repo = item.repository;
  const signalCount = Math.max(item.activityEvents, item.starDelta);

  return (
    <div
      aria-pressed={selected}
      className={[
        "grid cursor-pointer gap-4 p-4 transition md:grid-cols-[48px_minmax(0,1fr)_220px] md:items-center",
        selected
          ? "bg-emerald-50/80 ring-1 ring-inset ring-emerald-200"
          : "hover:bg-emerald-50/40",
      ].join(" ")}
      onClick={onSelect}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onSelect();
        }
      }}
      role="button"
      tabIndex={0}
    >
      <div className="font-mono text-xl font-black text-slate-300">
        {String(rank).padStart(2, "0")}
      </div>
      <div className="flex min-w-0 gap-4">
        <RepoAvatar repo={repo} />
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <Link
              href={`/r/${repo.owner}/${repo.name}`}
              prefetch={false}
              className="break-all font-mono text-lg font-bold text-slate-950 hover:text-emerald-700"
              onClick={(event) => event.stopPropagation()}
            >
              {repo.fullName}
            </Link>
            <a
              href={repo.htmlUrl}
              target="_blank"
              rel="noreferrer"
              className="inline-flex h-7 items-center gap-1 rounded-md border border-slate-200 bg-white px-2 text-xs font-bold text-slate-600 hover:text-emerald-700"
              onClick={(event) => event.stopPropagation()}
            >
              GitHub
              <ArrowUpRight className="h-3.5 w-3.5" />
            </a>
          </div>
          <p className="mt-2 line-clamp-2 text-sm leading-6 text-slate-600">
            {repo.description}
          </p>
          <p className="mt-2 text-sm leading-6 text-slate-500">
            {item.explanation}
          </p>
        </div>
      </div>
      <div className="grid grid-cols-3 gap-2 text-center">
        <MetricPill
          label={item.dataCoverage?.complete ? "stars" : "sampled"}
          value={`+${formatCompactNumber(item.starDelta)}`}
        />
        <MetricPill label="trend" value={Math.round(item.trendScore)} />
        <MetricPill label="signals" value={signalCount} />
      </div>
    </div>
  );
}

function MetricPill({ label, value }: { label: string | number; value: string | number }) {
  return (
    <div className="rounded-lg border border-slate-200 bg-white px-3 py-2">
      <div className="font-mono text-base font-black text-slate-950">
        {value}
      </div>
      <div className="mt-1 text-xs font-semibold text-slate-500">{label}</div>
    </div>
  );
}

function Sparkline({ points }: { points: RadarMetricPoint[] }) {
  if (points.length < 2) {
    return (
      <div className="mt-4 rounded-lg border border-dashed border-slate-300 bg-white p-5 text-sm text-slate-500">
        数据正在积累，至少需要两个采样点生成曲线。
      </div>
    );
  }
  const width = 360;
  const height = 150;
  const min = Math.min(...points.map((point) => point.stars));
  const max = Math.max(...points.map((point) => point.stars));
  const range = Math.max(1, max - min);
  const polyline = points
    .map((point, index) => {
      const x = (index / Math.max(1, points.length - 1)) * width;
      const y = height - ((point.stars - min) / range) * (height - 24) - 12;
      return `${x},${y}`;
    })
    .join(" ");

  return (
    <div className="mt-4 rounded-lg border border-slate-200 bg-white p-3">
      <svg
        aria-label="star curve"
        className="h-40 w-full"
        viewBox={`0 0 ${width} ${height}`}
        role="img"
      >
        <defs>
          <linearGradient id="radarSparkline" x1="0" x2="1" y1="0" y2="0">
            <stop stopColor="#047857" />
            <stop offset="1" stopColor="#14b8a6" />
          </linearGradient>
        </defs>
        <polyline
          fill="none"
          points={polyline}
          stroke="url(#radarSparkline)"
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth="4"
        />
      </svg>
      <div className="flex justify-between text-xs font-semibold text-slate-500">
        <span>{formatCompactNumber(min)}</span>
        <span>{formatCompactNumber(max)}</span>
      </div>
    </div>
  );
}

function EmptyState({ text }: { text: string }) {
  return (
    <div className="mt-4 rounded-lg border border-dashed border-slate-300 bg-white p-5 text-sm leading-6 text-slate-500">
      {text}
    </div>
  );
}

function formatStarDeltaContext(
  starDelta: number,
  selectedWindow: string,
  coverage?: RadarTrendItem["dataCoverage"],
) {
  if (!coverage || coverage.complete) {
    return `+${formatCompactNumber(starDelta)} stars / ${selectedWindow}`;
  }
  return `+${formatCompactNumber(starDelta)} sampled stars / 自 ${formatCoverageSince(coverage)}`;
}

function formatCoverageSince(coverage?: RadarTrendItem["dataCoverage"]) {
  if (!coverage?.observedSince) {
    return "开始采样";
  }
  const date = new Date(coverage.observedSince);
  if (Number.isNaN(date.getTime())) {
    return coverage.observedSince;
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}
