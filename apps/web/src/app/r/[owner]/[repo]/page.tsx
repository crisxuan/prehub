import Link from "next/link";
import { notFound } from "next/navigation";
import { RepoCard } from "@/components/repo-card";
import { SiteHeader } from "@/components/site-header";
import { githubUrl, repositories } from "@/lib/prehub-data";
import { getRepository, getSearchResults } from "@/lib/server-prehub-api";

type RepoPageProps = {
  params: Promise<{ owner: string; repo: string }>;
};

export const dynamic = "force-dynamic";

export default async function RepoPage({ params }: RepoPageProps) {
  const { owner, repo } = await params;
  const repository = await getRepository(owner, repo);

  if (!repository) {
    notFound();
  }

  const related = await getSearchResults(repository.language || repository.owner);
  const alternativeSource = related.results.length > 0 ? related.results : repositories;
  const alternatives = alternativeSource.filter(
    (item) => item.fullName !== repository.fullName,
  );

  return (
    <main className="min-h-screen bg-slate-50">
      <SiteHeader />
      <section className="mx-auto max-w-7xl px-5 py-8">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <Link href="/" className="text-sm font-medium text-emerald-700">
            返回今日推荐
          </Link>
          <a
            href={githubUrl(repository)}
            target="_blank"
            rel="noreferrer"
            className="rounded-md bg-slate-950 px-4 py-2 text-sm font-semibold text-white transition hover:bg-emerald-700"
          >
            打开 GitHub
          </a>
        </div>
        <div className="mt-5">
          <RepoCard repo={repository} highlight />
        </div>

        <div className="mt-6 rounded-lg border border-slate-200 bg-white p-6">
          <h2 className="text-lg font-semibold text-slate-950">README 摘要</h2>
          <p className="mt-3 max-w-3xl text-sm leading-7 text-slate-700">
            {repository.summary}
          </p>
          <div className="mt-5 grid gap-3 text-sm text-slate-600 md:grid-cols-3">
            <div className="rounded-md bg-slate-50 p-4">
              <div className="font-semibold text-slate-950">Stars</div>
              <div className="mt-1">{repository.stars.toLocaleString()}</div>
            </div>
            <div className="rounded-md bg-slate-50 p-4">
              <div className="font-semibold text-slate-950">Forks</div>
              <div className="mt-1">{repository.forks.toLocaleString()}</div>
            </div>
            <div className="rounded-md bg-slate-50 p-4">
              <div className="font-semibold text-slate-950">Last push</div>
              <div className="mt-1">
                {new Date(repository.pushedAt).toLocaleDateString("zh-CN")}
              </div>
            </div>
          </div>
        </div>

        <h2 className="mt-8 text-xl font-semibold text-slate-950">相似项目</h2>
        <div className="mt-4 grid gap-4">
          {alternatives.slice(0, 2).map((item) => (
            <RepoCard key={item.fullName} repo={item} />
          ))}
        </div>
      </section>
    </main>
  );
}
