"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { recommendationCategories } from "@/lib/prehub-data";

type CandidateActionsProps = {
  candidateId: string;
  status: string;
};

type ActionState =
  | { status: "idle" }
  | { status: "working"; action: "approve" | "publish" }
  | { status: "error"; message: string }
  | { status: "done"; message: string };

export function CandidateActions({
  candidateId,
  status,
}: CandidateActionsProps) {
  const router = useRouter();
  const [category, setCategory] = useState("ai");
  const [state, setState] = useState<ActionState>({ status: "idle" });
  const isWorking = state.status === "working";
  const canApprove = status === "pending_review" || status === "scored";
  const canPublish =
    status === "approved" || status === "pending_review" || status === "scored";

  async function runAction(action: "approve" | "publish") {
    setState({ status: "working", action });

    const response = await fetch(
      `/api/admin/candidates/${candidateId}/${action}`,
      {
        method: "POST",
        headers: { "content-type": "application/json" },
        body:
          action === "publish"
            ? JSON.stringify({ theme: "今日开源项目", category })
            : JSON.stringify({}),
      },
    );
    const payload = await response.json();

    if (!response.ok) {
      setState({
        status: "error",
        message: payload.error ?? payload.message ?? "操作失败",
      });
      return;
    }

    setState({
      status: "done",
      message: action === "approve" ? "已通过" : "已发布",
    });
    router.refresh();
  }

  return (
    <div className="flex min-w-40 flex-col gap-2">
      <select
        value={category}
        onChange={(event) => setCategory(event.target.value)}
        disabled={isWorking}
        className="rounded-md border border-slate-300 bg-white px-2 py-2 text-xs font-medium text-slate-700 disabled:cursor-not-allowed disabled:bg-slate-100"
      >
        {recommendationCategories.map((item) => (
          <option key={item.slug} value={item.slug}>
            {item.label}
          </option>
        ))}
      </select>
      <div className="flex gap-2">
        <button
          type="button"
          disabled={!canApprove || isWorking}
          onClick={() => runAction("approve")}
          className="rounded-md border border-slate-300 px-3 py-2 text-xs font-semibold text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:border-slate-200 disabled:text-slate-400"
        >
          通过
        </button>
        <button
          type="button"
          disabled={!canPublish || isWorking}
          onClick={() => runAction("publish")}
          className="rounded-md bg-slate-950 px-3 py-2 text-xs font-semibold text-white transition hover:bg-emerald-700 disabled:cursor-not-allowed disabled:bg-slate-300"
        >
          发布今日
        </button>
      </div>
      {state.status === "working" ? (
        <span className="text-xs text-slate-500">处理中...</span>
      ) : null}
      {state.status === "done" ? (
        <span className="text-xs font-medium text-emerald-700">
          {state.message}
        </span>
      ) : null}
      {state.status === "error" ? (
        <span className="text-xs font-medium text-red-600">
          {state.message}
        </span>
      ) : null}
    </div>
  );
}
