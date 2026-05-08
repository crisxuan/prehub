"use client";

import Link from "next/link";
import {
  CircleUserRound,
  FolderOpen,
  Radar,
  Search,
  Sparkles,
} from "lucide-react";
import { usePathname } from "next/navigation";

const navItems = [
  { href: "/", label: "今日推荐", icon: Sparkles },
  { href: "/radar", label: "Radar", icon: Radar },
  { href: "/search", label: "搜索", icon: Search },
  { href: "/daily", label: "归档", icon: FolderOpen },
  { href: "/admin", label: "Admin", icon: CircleUserRound },
];

export function SiteHeader() {
  const pathname = usePathname();

  return (
    <header className="sticky top-0 z-30 px-3 pt-2">
      <div className="mx-auto flex max-w-[1480px] items-center justify-between gap-4 rounded-lg border border-slate-200/80 bg-white/90 px-5 py-4 shadow-[0_14px_36px_rgba(15,23,42,0.08)] backdrop-blur-xl">
        <Link href="/" prefetch={false} className="flex items-center gap-3">
          <span className="grid h-11 w-11 place-items-center rounded-lg bg-gradient-to-br from-emerald-300 via-emerald-500 to-teal-700 text-lg font-bold text-white shadow-[0_12px_28px_rgba(16,185,129,0.28)]">
            P
          </span>
          <span>
            <span className="block text-lg font-bold tracking-tight text-slate-950">
              PreHub
            </span>
            <span className="block text-sm text-slate-500">
              GitHub 项目发现
            </span>
          </span>
        </Link>
        <nav className="flex min-w-0 items-center gap-2 overflow-x-auto">
          {navItems.map((item) => {
            const isActive =
              item.href === "/"
                ? pathname === "/"
                : pathname.startsWith(item.href);

            return (
              <Link
                key={item.href}
                href={item.href}
                prefetch={false}
                className={[
                  "inline-flex h-10 shrink-0 items-center gap-2 rounded-lg px-3 text-sm font-semibold transition",
                  isActive
                    ? "border border-slate-200 bg-white text-slate-950 shadow-sm"
                    : "text-slate-600 hover:bg-slate-100 hover:text-slate-950",
                ].join(" ")}
              >
                <item.icon
                  className={isActive ? "h-4 w-4 text-emerald-600" : "h-4 w-4"}
                />
                {item.label}
              </Link>
            );
          })}
        </nav>
      </div>
    </header>
  );
}
