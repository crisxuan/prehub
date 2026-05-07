import Link from "next/link";
import { SiteHeader } from "@/components/site-header";
import { getAdminOverview, getTodayPick } from "@/lib/server-prehub-api";

export const dynamic = "force-dynamic";

export default async function AdminPage() {
  const [adminOverview, todayPick] = await Promise.all([
    getAdminOverview(),
    getTodayPick(),
  ]);

  return (
    <main className="min-h-screen bg-slate-50">
      <SiteHeader />
      <section className="mx-auto max-w-7xl px-5 py-8">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div>
            <h1 className="text-3xl font-semibold text-slate-950">
              Admin Console
            </h1>
            <p className="mt-2 text-sm text-slate-600">
              采集、审核、排期和运营视图的最小入口。
            </p>
          </div>
          <Link
            href="/admin/candidates"
            className="rounded-md bg-slate-950 px-4 py-2 text-sm font-semibold text-white hover:bg-emerald-700"
          >
            打开候选队列
          </Link>
        </div>

        <div className="mt-8 grid gap-4 md:grid-cols-3">
          {[
            ["待审核", adminOverview.pendingReviewCount.toString()],
            ["已排期", todayPick.primary.fullName],
            ["候选总数", adminOverview.candidateCount.toString()],
          ].map(([label, value]) => (
            <div
              key={label}
              className="rounded-lg border border-slate-200 bg-white p-5"
            >
              <div className="text-sm text-slate-500">{label}</div>
              <div className="mt-3 break-words font-mono text-xl font-semibold text-slate-950">
                {value}
              </div>
            </div>
          ))}
        </div>

        <div className="mt-6 rounded-lg border border-slate-200 bg-white p-5">
          <h2 className="text-lg font-semibold text-slate-950">采集状态</h2>
          <dl className="mt-4 grid gap-4 text-sm md:grid-cols-3">
            <div>
              <dt className="text-slate-500">GitHub rate limit</dt>
              <dd className="mt-1 font-mono font-semibold text-slate-950">
                {adminOverview.lastRateLimitStatus}
              </dd>
            </div>
            <div>
              <dt className="text-slate-500">Scheduled picks</dt>
              <dd className="mt-1 font-mono font-semibold text-slate-950">
                {adminOverview.scheduledPickCount}
              </dd>
            </div>
            <div>
              <dt className="text-slate-500">Worker</dt>
              <dd className="mt-1 font-mono font-semibold text-slate-950">
                idle
              </dd>
            </div>
          </dl>
        </div>
      </section>
    </main>
  );
}
