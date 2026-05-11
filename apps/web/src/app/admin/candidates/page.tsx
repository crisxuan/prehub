import { CandidateActions } from "@/components/candidate-actions";
import { SiteHeader } from "@/components/site-header";
import { SubmitRepoForm } from "@/components/submit-repo-form";
import { getCandidates } from "@/lib/server-prehub-api";

const statusLabel: Record<string, string> = {
  discovered: "已发现",
  pending_review: "待审核",
  scored: "已评分",
  approved: "已通过",
  published: "已发布",
};

export const dynamic = "force-dynamic";

export default async function AdminCandidatesPage() {
  const candidates = await getCandidates();

  return (
    <main className="min-h-screen bg-slate-50">
      <SiteHeader />
      <section className="mx-auto max-w-7xl px-5 py-8">
        <h1 className="text-3xl font-semibold text-slate-950">候选项目队列</h1>
        <div className="mt-6">
          <SubmitRepoForm />
        </div>
        <div className="mt-6 overflow-x-auto rounded-lg border border-slate-200 bg-white">
          <table className="w-full min-w-[980px] border-collapse text-left text-sm">
            <thead className="bg-slate-100 text-slate-600">
              <tr>
                <th className="px-4 py-3 font-semibold">Repository</th>
                <th className="px-4 py-3 font-semibold">Score</th>
                <th className="px-4 py-3 font-semibold">Status</th>
                <th className="px-4 py-3 font-semibold">Source</th>
                <th className="px-4 py-3 font-semibold">Reason</th>
                <th className="px-4 py-3 font-semibold">Actions</th>
              </tr>
            </thead>
            <tbody>
              {candidates.map((candidate) => (
                <tr key={candidate.id} className="border-t border-slate-100">
                  <td className="px-4 py-4 font-mono font-semibold text-slate-950">
                    {candidate.repository.fullName}
                  </td>
                  <td className="px-4 py-4 text-slate-700">
                    {candidate.qualityScore}
                  </td>
                  <td className="px-4 py-4">
                    <span className="rounded-md bg-emerald-50 px-2.5 py-1 text-xs font-medium text-emerald-700">
                      {statusLabel[candidate.status] ?? candidate.status}
                    </span>
                  </td>
                  <td className="px-4 py-4 text-slate-600">
                    {candidate.source ?? "seed"}
                  </td>
                  <td className="max-w-xl px-4 py-4 text-slate-600">
                    {candidate.repository.reason}
                  </td>
                  <td className="px-4 py-4">
                    <CandidateActions
                      candidateId={candidate.id}
                      status={candidate.status}
                    />
                  </td>
                </tr>
              ))}
              {candidates.length === 0 ? (
                <tr>
                  <td
                    colSpan={6}
                    className="border-t border-slate-100 px-4 py-10 text-center text-sm text-slate-500"
                  >
                    候选队列为空。先提交 GitHub URL，或等待 Radar/定时任务写入候选项目。
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>
    </main>
  );
}
