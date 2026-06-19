"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import type { ReactNode } from "react";

export default function AdminLayout({
  children,
}: {
  children: ReactNode;
}) {
  const router = useRouter();
  const pathname = usePathname();

  async function handleLogout() {
    await fetch("/api/admin/auth", { method: "DELETE" });
    router.push("/");
    router.refresh();
  }

  const navItems = [
    { href: "/admin", label: "概览" },
    { href: "/admin/candidates", label: "候选队列" },
    { href: "/admin/radar/watchlist", label: "Radar 监控" },
  ];

  return (
    <div>
      <div className="border-b border-slate-200 bg-white px-5 py-3 flex items-center justify-between">
        <nav className="flex gap-4 text-sm">
          {navItems.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className={[
                "font-semibold hover:text-emerald-700 transition",
                pathname === item.href ? "text-emerald-700" : "text-slate-950",
              ].join(" ")}
            >
              {item.label}
            </Link>
          ))}
        </nav>
        <button
          onClick={handleLogout}
          className="text-sm font-semibold text-slate-600 hover:text-red-600 transition"
        >
          退出登录
        </button>
      </div>
      {children}
    </div>
  );
}
