import Link from "next/link";
import {
  formatCompactNumber,
  githubUrl,
  type Repository,
} from "@/lib/prehub-data";

type RepoCardProps = {
  repo: Repository;
  highlight?: boolean;
};

export function RepoCard({ repo, highlight = false }: RepoCardProps) {
  return (
    <article
      className={[
        "rounded-lg border bg-white p-5 shadow-sm",
        highlight ? "border-emerald-300" : "border-slate-200",
      ].join(" ")}
    >
      <div className="flex flex-col gap-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <Link
              href={`/r/${repo.owner}/${repo.name}`}
              className="font-mono text-lg font-semibold text-slate-950 hover:text-emerald-700"
            >
              {repo.fullName}
            </Link>
            <p className="mt-2 max-w-2xl text-sm leading-6 text-slate-600">
              {repo.description}
            </p>
          </div>
          <div className="flex items-start gap-2">
            <a
              href={githubUrl(repo)}
              target="_blank"
              rel="noreferrer"
              className="rounded-md border border-slate-300 px-3 py-2 text-sm font-semibold text-slate-700 transition hover:border-slate-950 hover:text-slate-950"
            >
              GitHub
            </a>
            <div className="rounded-md border border-slate-200 px-3 py-2 text-right">
              <div className="text-sm font-semibold text-slate-950">
                {formatCompactNumber(repo.stars)}
              </div>
              <div className="text-xs text-slate-500">stars</div>
            </div>
          </div>
        </div>

        <div className="flex flex-wrap gap-2">
          <span className="rounded-md bg-blue-50 px-2.5 py-1 text-xs font-medium text-blue-700">
            {repo.language}
          </span>
          <span className="rounded-md bg-emerald-50 px-2.5 py-1 text-xs font-medium text-emerald-700">
            {repo.license}
          </span>
          {repo.topics.slice(0, 4).map((topic) => (
            <span
              key={topic}
              className="rounded-md bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-600"
            >
              {topic}
            </span>
          ))}
        </div>

        <div className="grid gap-3 border-t border-slate-100 pt-4 md:grid-cols-2">
          <p className="text-sm leading-6 text-slate-700">
            <span className="font-semibold text-slate-950">推荐理由：</span>
            {repo.reason}
          </p>
          <p className="text-sm leading-6 text-slate-700">
            <span className="font-semibold text-slate-950">注意事项：</span>
            {repo.caveat}
          </p>
        </div>
      </div>
    </article>
  );
}
