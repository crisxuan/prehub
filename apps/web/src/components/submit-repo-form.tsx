"use client";

import { useRouter } from "next/navigation";
import type { FormEvent } from "react";
import { useState } from "react";
import type { Candidate } from "@/lib/prehub-data";

type SubmitState =
  | { status: "idle" }
  | { status: "submitting" }
  | { status: "success"; candidate?: Candidate; message: string }
  | { status: "error"; message: string };

export function SubmitRepoForm() {
  const router = useRouter();
  const [state, setState] = useState<SubmitState>({ status: "idle" });

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setState({ status: "submitting" });

    const formData = new FormData(event.currentTarget);
    const url = String(formData.get("url") ?? "");

    let response: Response;
    try {
      response = await fetch("/api/admin/repositories/submit", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          url,
          source: "admin_submit",
          priority: "normal",
        }),
      });
    } catch {
      setState({ status: "error", message: "网络请求失败，请稍后重试" });
      return;
    }

    const payload = await response.json().catch(() => ({}));
    if (!response.ok) {
      setState({
        status: "error",
        message: payload.error ?? payload.message ?? "提交失败",
      });
      return;
    }

    setState({
      status: "success",
      candidate: payload.candidate,
      message: "已提交到候选队列",
    });
    router.refresh();
  }

  return (
    <section className="rounded-lg border border-slate-200 bg-white p-5">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h2 className="text-lg font-semibold text-slate-950">提交仓库</h2>
          <p className="mt-1 text-sm text-slate-500">
            支持 github.com/owner/repo 格式。
          </p>
        </div>
        <span className="rounded-md bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-600">
          admin_submit
        </span>
      </div>

      <form onSubmit={onSubmit} className="mt-4 flex flex-col gap-3 md:flex-row">
        <input
          name="url"
          required
          placeholder="https://github.com/vercel/next.js"
          className="min-w-0 flex-1 rounded-md border border-slate-300 bg-white px-4 py-3 text-sm text-slate-950 outline-none transition focus:border-emerald-500 focus:ring-2 focus:ring-emerald-100"
        />
        <button
          disabled={state.status === "submitting"}
          className="rounded-md bg-slate-950 px-5 py-3 text-sm font-semibold text-white transition hover:bg-emerald-700 disabled:cursor-not-allowed disabled:bg-slate-400"
        >
          {state.status === "submitting" ? "提交中" : "提交"}
        </button>
      </form>

      {state.status === "success" ? (
        <div className="mt-4 rounded-md border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-900">
          <div className="font-semibold">{state.message}</div>
          {state.candidate ? (
            <div className="mt-2 font-mono">
              {state.candidate.repository.fullName} / score{" "}
              {state.candidate.qualityScore}
            </div>
          ) : null}
        </div>
      ) : null}

      {state.status === "error" ? (
        <div className="mt-4 rounded-md border border-red-200 bg-red-50 p-4 text-sm font-medium text-red-700">
          {state.message}
        </div>
      ) : null}
    </section>
  );
}
