"use client";

import Link from "next/link";
import { useState, type ReactNode } from "react";
import { AlertTriangle, Check, ExternalLink, ThumbsDown, ThumbsUp } from "lucide-react";
import {
  formatCompactNumber,
  githubUrl,
  type Repository,
} from "@/lib/prehub-data";
import { RepoAvatar } from "@/components/repo-visuals";

type RepoCardProps = {
  repo: Repository;
  highlight?: boolean;
};

export function RepoCard({ repo, highlight = false }: RepoCardProps) {
  const [feedbackState, setFeedbackState] = useState<"none" | "like" | "dislike">("none");
  const [submitting, setSubmitting] = useState(false);

  async function handleFeedback(action: "like" | "dislike") {
    if (feedbackState !== "none" || submitting) return;
    setSubmitting(true);
    try {
      await fetch("/api/feedback", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action, repositoryFullName: repo.fullName }),
      });
      setFeedbackState(action);
    } catch {
      // Silently fail - feedback is non-critical
    } finally {
      setSubmitting(false);
    }
  }

  if (highlight) {
    return (
      <article className="relative overflow-hidden rounded-lg border border-emerald-300 bg-white p-5 shadow-[0_22px_70px_rgba(15,118,110,0.12)]">
        <RecommendationIllustration />
        <div className="relative z-10">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
            <div className="flex min-w-0 gap-5">
              <RepoAvatar repo={repo} size="lg" />
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <Link
                    href={`/r/${repo.owner}/${repo.name}`}
                    prefetch={false}
                    className="break-all font-mono text-2xl font-bold tracking-tight text-slate-950 hover:text-emerald-700"
                  >
                    {repo.fullName}
                  </Link>
                  <span className="grid h-5 w-5 place-items-center rounded-full bg-emerald-400 text-white">
                    <Check className="h-3.5 w-3.5" />
                  </span>
                </div>
                <p className="mt-3 max-w-3xl text-sm leading-6 text-slate-600">
                  {repo.description}
                </p>
                <TagRow repo={repo} />
              </div>
            </div>

            <div className="flex shrink-0 items-start gap-2">
              <a
                href={githubUrl(repo)}
                target="_blank"
                rel="noreferrer"
                className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-white px-4 text-sm font-semibold text-slate-700 shadow-sm transition hover:border-slate-300 hover:text-slate-950"
              >
                <ExternalLink className="h-4 w-4" />
                在 GitHub 查看
              </a>
              <div className="rounded-lg border border-slate-200 bg-white px-4 py-2 text-right shadow-sm">
                <div className="text-sm font-bold text-slate-950">
                  {formatCompactNumber(repo.stars)}
                </div>
                <div className="text-xs text-slate-500">stars</div>
              </div>
            </div>
          </div>

          <div className="mt-5 grid gap-4 border-t border-slate-200 pt-4 md:grid-cols-2">
            <InfoBlock
              icon={<ThumbsUp className="h-4 w-4" />}
              tone="emerald"
              title="推荐理由"
            >
              {repo.reason}
            </InfoBlock>
            <InfoBlock
              icon={<AlertTriangle className="h-4 w-4" />}
              tone="amber"
              title="注意事项"
            >
              {repo.caveat}
            </InfoBlock>
          </div>

          <FeedbackButtons
            feedbackState={feedbackState}
            submitting={submitting}
            onFeedback={handleFeedback}
          />
        </div>
      </article>
    );
  }

  return (
    <article className="rounded-lg border border-slate-200 bg-white p-5 shadow-[0_16px_44px_rgba(15,23,42,0.05)]">
      <div className="flex flex-col gap-4">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="flex min-w-0 gap-4">
            <RepoAvatar repo={repo} />
            <div className="min-w-0">
              <Link
                href={`/r/${repo.owner}/${repo.name}`}
                prefetch={false}
                className="break-all font-mono text-lg font-semibold text-slate-950 hover:text-emerald-700"
              >
                {repo.fullName}
              </Link>
              <p className="mt-2 max-w-2xl text-sm leading-6 text-slate-600">
                {repo.description}
              </p>
            </div>
          </div>
          <div className="flex items-start gap-2">
            <a
              href={githubUrl(repo)}
              target="_blank"
              rel="noreferrer"
              className="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-200 px-3 text-sm font-semibold text-slate-700 transition hover:border-slate-300 hover:text-slate-950"
            >
              <ExternalLink className="h-4 w-4" />
              GitHub
            </a>
            <div className="rounded-lg border border-slate-200 px-3 py-2 text-right">
              <div className="text-sm font-bold text-slate-950">
                {formatCompactNumber(repo.stars)}
              </div>
              <div className="text-xs text-slate-500">stars</div>
            </div>
          </div>
        </div>

        <TagRow repo={repo} />

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

        <FeedbackButtons
          feedbackState={feedbackState}
          submitting={submitting}
          onFeedback={handleFeedback}
        />
      </div>
    </article>
  );
}

function RecommendationIllustration() {
  return (
    <svg
      aria-hidden="true"
      className="pointer-events-none absolute inset-y-0 right-0 z-0 hidden h-full w-[46%] text-emerald-500 opacity-70 lg:block"
      viewBox="0 0 560 320"
      preserveAspectRatio="xMidYMid slice"
      fill="none"
    >
      <defs>
        <linearGradient id="prehubPlatform" x1="156" y1="88" x2="444" y2="260">
          <stop stopColor="#ecfdf5" stopOpacity="0.1" />
          <stop offset="0.58" stopColor="#6ee7b7" stopOpacity="0.28" />
          <stop offset="1" stopColor="#14b8a6" stopOpacity="0.1" />
        </linearGradient>
        <linearGradient id="prehubCubeTop" x1="292" y1="104" x2="410" y2="178">
          <stop stopColor="#ccfbf1" stopOpacity="0.8" />
          <stop offset="1" stopColor="#34d399" stopOpacity="0.34" />
        </linearGradient>
        <linearGradient id="prehubCubeSide" x1="398" y1="150" x2="398" y2="244">
          <stop stopColor="#5eead4" stopOpacity="0.58" />
          <stop offset="1" stopColor="#059669" stopOpacity="0.16" />
        </linearGradient>
        <filter id="prehubSoftBlur" x="-20%" y="-20%" width="140%" height="140%">
          <feGaussianBlur stdDeviation="10" />
        </filter>
        <filter id="prehubShadow" x="-40%" y="-40%" width="180%" height="180%">
          <feDropShadow dx="0" dy="14" stdDeviation="16" floodColor="#0f766e" floodOpacity="0.12" />
        </filter>
      </defs>

      <rect x="0" y="0" width="560" height="320" fill="url(#prehubPlatform)" opacity="0.42" />
      <path d="M118 200L274 112L488 224L326 304L118 200Z" fill="#ecfdf5" fillOpacity="0.34" />
      <path d="M118 200L274 112L488 224L326 304L118 200Z" stroke="#99f6e4" strokeOpacity="0.55" />
      <path d="M160 199L282 132L444 220" stroke="#5eead4" strokeOpacity="0.28" />
      <path d="M212 228L348 148L490 224" stroke="#5eead4" strokeOpacity="0.22" />
      <path d="M326 304L326 250" stroke="#14b8a6" strokeOpacity="0.24" />

      <ellipse
        cx="354"
        cy="232"
        rx="94"
        ry="28"
        fill="#10b981"
        fillOpacity="0.12"
        filter="url(#prehubSoftBlur)"
      />

      <g filter="url(#prehubShadow)">
        <path d="M312 112L386 148L334 176L260 140L312 112Z" fill="url(#prehubCubeTop)" />
        <path d="M260 140L334 176V246L260 208V140Z" fill="#a7f3d0" fillOpacity="0.38" />
        <path d="M334 176L386 148V218L334 246V176Z" fill="url(#prehubCubeSide)" />
        <path d="M312 112L386 148L334 176L260 140L312 112Z" stroke="#14b8a6" strokeOpacity="0.65" />
        <path d="M260 140L334 176V246L260 208V140Z" stroke="#14b8a6" strokeOpacity="0.42" />
        <path d="M334 176L386 148V218L334 246V176Z" stroke="#14b8a6" strokeOpacity="0.42" />
        <path d="M354 185L374 174" stroke="#ecfdf5" strokeWidth="5" strokeLinecap="round" opacity="0.64" />
        <path d="M354 205L374 194" stroke="#ecfdf5" strokeWidth="4" strokeLinecap="round" opacity="0.42" />
      </g>

      <g filter="url(#prehubShadow)">
        <path d="M196 72L246 48L246 118L196 144V72Z" fill="#99f6e4" fillOpacity="0.42" />
        <path d="M196 72L246 48L246 118L196 144V72Z" stroke="#14b8a6" strokeOpacity="0.52" />
        <path d="M211 86L232 76" stroke="#ecfdf5" strokeWidth="5" strokeLinecap="round" opacity="0.82" />
        <path d="M211 101L226 94" stroke="#ecfdf5" strokeWidth="4" strokeLinecap="round" opacity="0.52" />
      </g>

      <g filter="url(#prehubShadow)">
        <path d="M454 214L520 182L520 242L454 276V214Z" fill="#a7f3d0" fillOpacity="0.32" />
        <path d="M454 214L520 182L520 242L454 276V214Z" stroke="#14b8a6" strokeOpacity="0.44" />
        <path d="M480 223L493 217M493 217L493 231M493 217L505 224" stroke="#ecfdf5" strokeWidth="4" strokeLinecap="round" strokeLinejoin="round" opacity="0.62" />
      </g>

      <path d="M222 144V180L260 200" stroke="#14b8a6" strokeOpacity="0.35" />
      <path d="M422 218H454" stroke="#14b8a6" strokeOpacity="0.35" />
      <path d="M188 182V212L232 234" stroke="#14b8a6" strokeOpacity="0.26" />
      <circle cx="222" cy="180" r="5" fill="#10b981" fillOpacity="0.34" />
      <circle cx="260" cy="200" r="7" fill="#10b981" fillOpacity="0.34" />
      <circle cx="422" cy="218" r="7" fill="#10b981" fillOpacity="0.34" />
      <circle cx="232" cy="234" r="5" fill="#10b981" fillOpacity="0.28" />
      <circle cx="500" cy="174" r="4" fill="#10b981" fillOpacity="0.25" />
    </svg>
  );
}

function TagRow({ repo }: { repo: Repository }) {
  return (
    <div className="mt-3 flex flex-wrap gap-2">
      <span className="rounded-md bg-blue-50 px-2.5 py-1 text-xs font-semibold text-blue-700">
        {repo.language}
      </span>
      <span className="rounded-md bg-emerald-50 px-2.5 py-1 text-xs font-semibold text-emerald-700">
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
  );
}

function InfoBlock({
  children,
  icon,
  title,
  tone,
}: {
  children: ReactNode;
  icon: ReactNode;
  title: string;
  tone: "emerald" | "amber";
}) {
  return (
    <div className="flex gap-3">
      <span
        className={[
          "mt-0.5 grid h-7 w-7 shrink-0 place-items-center rounded-full text-white",
          tone === "emerald" ? "bg-emerald-500" : "bg-amber-400",
        ].join(" ")}
      >
        {icon}
      </span>
      <p className="text-sm leading-6 text-slate-700">
        <span className="font-bold text-slate-950">{title}</span>
        <br />
        {children}
      </p>
    </div>
  );
}

function FeedbackButtons({
  feedbackState,
  submitting,
  onFeedback,
}: {
  feedbackState: "none" | "like" | "dislike";
  submitting: boolean;
  onFeedback: (action: "like" | "dislike") => void;
}) {
  return (
    <div className="mt-4 flex items-center gap-2 border-t border-slate-100 pt-3">
      <span className="text-xs font-medium text-slate-500">这个推荐有帮助吗？</span>
      <button
        onClick={() => onFeedback("like")}
        disabled={feedbackState !== "none" || submitting}
        className={[
          "inline-flex items-center gap-1 rounded px-2 py-1 text-xs font-medium transition",
          feedbackState === "like"
            ? "bg-emerald-100 text-emerald-700"
            : "text-slate-600 hover:bg-emerald-50 hover:text-emerald-700",
          submitting || feedbackState !== "none" ? "cursor-not-allowed opacity-50" : "",
        ].join(" ")}
      >
        <ThumbsUp className="h-3.5 w-3.5" />
        {feedbackState === "like" ? "已感谢" : "有用"}
      </button>
      <button
        onClick={() => onFeedback("dislike")}
        disabled={feedbackState !== "none" || submitting}
        className={[
          "inline-flex items-center gap-1 rounded px-2 py-1 text-xs font-medium transition",
          feedbackState === "dislike"
            ? "bg-slate-100 text-slate-700"
            : "text-slate-600 hover:bg-slate-50 hover:text-slate-900",
          submitting || feedbackState !== "none" ? "cursor-not-allowed opacity-50" : "",
        ].join(" ")}
      >
        <ThumbsDown className="h-3.5 w-3.5" />
        {feedbackState === "dislike" ? "已记录" : "无用"}
      </button>
    </div>
  );
}
