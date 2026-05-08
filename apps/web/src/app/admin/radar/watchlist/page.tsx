import Link from "next/link";
import { ArrowLeft, Radio } from "lucide-react";
import { RadarWatchlistForm } from "@/components/radar-watchlist-form";
import { SiteHeader } from "@/components/site-header";

export const dynamic = "force-dynamic";

export default function AdminRadarWatchlistPage() {
  return (
    <main className="min-h-screen bg-slate-50">
      <SiteHeader />
      <section className="mx-auto max-w-7xl px-5 py-8">
        <Link
          href="/radar"
          prefetch={false}
          className="inline-flex items-center gap-2 text-sm font-bold text-emerald-700 hover:text-emerald-800"
        >
          <ArrowLeft className="h-4 w-4" />
          返回 Radar
        </Link>
        <div className="mt-5 flex flex-wrap items-end justify-between gap-4">
          <div>
            <h1 className="inline-flex items-center gap-2 text-3xl font-black text-slate-950">
              <Radio className="h-7 w-7 text-emerald-600" />
              Radar Watchlist
            </h1>
            <p className="mt-2 max-w-2xl text-sm leading-6 text-slate-600">
              把 GitHub 项目加入监控后，系统会立即采样一次 stars/forks，并进入对应刷新等级。后续趋势榜和 star 曲线都从这里积累。
            </p>
          </div>
        </div>

        <div className="mt-6">
          <RadarWatchlistForm />
        </div>

        <div className="mt-6 rounded-lg border border-slate-200 bg-white p-5">
          <h2 className="text-lg font-bold text-slate-950">刷新等级</h2>
          <div className="mt-4 grid gap-3 md:grid-cols-4">
            {[
              ["hot", "今日推荐、人工重点关注，10 分钟刷新"],
              ["watch", "用户/管理员关注，30 分钟刷新"],
              ["candidate", "候选队列项目，6 小时刷新"],
              ["archive", "低活跃归档，24 小时刷新"],
            ].map(([tier, description]) => (
              <div
                key={tier}
                className="rounded-lg border border-slate-200 bg-slate-50 p-4"
              >
                <div className="font-mono text-sm font-black text-slate-950">
                  {tier}
                </div>
                <div className="mt-2 text-sm leading-6 text-slate-600">
                  {description}
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>
    </main>
  );
}
