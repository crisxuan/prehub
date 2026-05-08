"use client";

import { useState, type FormEvent } from "react";
import { Loader2, Plus } from "lucide-react";
import { recommendationCategories } from "@/lib/prehub-data";

const tierOptions = [
  { value: "hot", label: "hot / 10 分钟" },
  { value: "watch", label: "watch / 30 分钟" },
  { value: "candidate", label: "candidate / 6 小时" },
  { value: "archive", label: "archive / 24 小时" },
];

export function RadarWatchlistForm() {
  const [status, setStatus] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSubmitting(true);
    setStatus("");
    const form = event.currentTarget;
    const data = new FormData(form);
    const response = await fetch("/api/admin/radar/watchlist", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        url: data.get("url"),
        category: data.get("category"),
        tier: data.get("tier"),
      }),
    });
    const payload = await response.json().catch(() => ({}));
    setIsSubmitting(false);
    if (!response.ok) {
      setStatus(payload.error ?? "加入监控失败");
      return;
    }
    setStatus(`${payload.repository?.fullName ?? "项目"} 已加入 Radar 监控`);
    form.reset();
  }

  return (
    <form
      onSubmit={handleSubmit}
      className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm"
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_180px_180px_auto]">
        <label className="block">
          <span className="text-sm font-semibold text-slate-700">
            GitHub URL
          </span>
          <input
            required
            name="url"
            placeholder="https://github.com/snyk/agent-scan"
            className="mt-2 h-11 w-full rounded-lg border border-slate-200 px-3 text-sm outline-none focus:border-emerald-400"
          />
        </label>
        <label className="block">
          <span className="text-sm font-semibold text-slate-700">分类</span>
          <select
            name="category"
            defaultValue="ai"
            className="mt-2 h-11 w-full rounded-lg border border-slate-200 px-3 text-sm outline-none focus:border-emerald-400"
          >
            {recommendationCategories.map((category) => (
              <option key={category.slug} value={category.slug}>
                {category.label}
              </option>
            ))}
          </select>
        </label>
        <label className="block">
          <span className="text-sm font-semibold text-slate-700">等级</span>
          <select
            name="tier"
            defaultValue="candidate"
            className="mt-2 h-11 w-full rounded-lg border border-slate-200 px-3 text-sm outline-none focus:border-emerald-400"
          >
            {tierOptions.map((tier) => (
              <option key={tier.value} value={tier.value}>
                {tier.label}
              </option>
            ))}
          </select>
        </label>
        <button
          disabled={isSubmitting}
          className="mt-7 inline-flex h-11 items-center justify-center gap-2 rounded-lg bg-slate-950 px-5 text-sm font-bold text-white transition hover:bg-emerald-700 disabled:cursor-wait disabled:opacity-70"
        >
          {isSubmitting ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <Plus className="h-4 w-4" />
          )}
          加入
        </button>
      </div>
      {status ? (
        <div className="mt-4 rounded-lg bg-emerald-50 px-3 py-2 text-sm font-semibold text-emerald-700">
          {status}
        </div>
      ) : null}
    </form>
  );
}
