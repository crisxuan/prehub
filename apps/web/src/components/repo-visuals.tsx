"use client";

/* eslint-disable @next/next/no-img-element */

import { useState } from "react";

import type { Repository } from "@/lib/prehub-data";

const repoTones = [
  ["#0f172a", "#111827", "#14b8a6"],
  ["#2563eb", "#0891b2", "#67e8f9"],
  ["#7c3aed", "#4f46e5", "#c084fc"],
  ["#059669", "#0d9488", "#86efac"],
  ["#334155", "#0f766e", "#99f6e4"],
  ["#9333ea", "#db2777", "#f0abfc"],
] as const;

function hashRepo(repo: Repository) {
  return repo.fullName.split("").reduce((total, char) => {
    return total + char.charCodeAt(0);
  }, 0);
}

export function repoTone(repo: Repository) {
  return repoTones[hashRepo(repo) % repoTones.length];
}

function githubOwnerAvatar(repo: Repository) {
  const owner = repo.owner?.trim();
  if (!owner) {
    return "";
  }
  return `https://github.com/${encodeURIComponent(owner)}.png?size=128`;
}

function repoIconURL(repo: Repository) {
  return repo.avatarUrl?.trim() || githubOwnerAvatar(repo);
}

export function RepoAvatar({
  repo,
  size = "md",
}: {
  repo: Repository;
  size?: "sm" | "md" | "lg";
}) {
  const [failedIconURL, setFailedIconURL] = useState("");
  const [from, to, accent] = repoTone(repo);
  const sizeClass =
    size === "lg" ? "h-16 w-16 text-3xl" : size === "sm" ? "h-10 w-10 text-xl" : "h-14 w-14 text-2xl";
  const initial = repo.name.slice(0, 1).toLowerCase() || "p";
  const rawIconURL = repoIconURL(repo);
  const iconURL = rawIconURL && rawIconURL !== failedIconURL ? rawIconURL : "";

  return (
    <span
      aria-hidden="true"
      className={[
        "grid shrink-0 place-items-center overflow-hidden rounded-lg bg-white font-mono font-semibold text-white shadow-[0_14px_30px_rgba(15,23,42,0.18)] ring-1 ring-slate-200/80",
        sizeClass,
      ].join(" ")}
      style={{
        background: `linear-gradient(135deg, ${from}, ${to})`,
      }}
    >
      {iconURL ? (
        <img
          alt=""
          className="h-full w-full object-contain"
          referrerPolicy="no-referrer"
          src={iconURL}
          onError={() => setFailedIconURL(iconURL)}
        />
      ) : (
        <span
          className="drop-shadow-sm"
          style={{
            color: accent,
          }}
        >
          {initial}
        </span>
      )}
    </span>
  );
}
