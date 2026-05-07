import Link from "next/link";

const navItems = [
  { href: "/", label: "今日推荐" },
  { href: "/search", label: "搜索" },
  { href: "/daily", label: "归档" },
  { href: "/admin", label: "Admin" },
];

export function SiteHeader() {
  return (
    <header className="border-b border-slate-200 bg-white/90 backdrop-blur">
      <div className="mx-auto flex max-w-7xl items-center justify-between px-5 py-4">
        <Link href="/" className="flex items-center gap-3">
          <span className="grid h-9 w-9 place-items-center rounded-md bg-emerald-600 text-sm font-semibold text-white">
            P
          </span>
          <span>
            <span className="block text-base font-semibold text-slate-950">
              PreHub
            </span>
            <span className="block text-xs text-slate-500">
              GitHub 项目发现
            </span>
          </span>
        </Link>
        <nav className="flex items-center gap-1">
          {navItems.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className="rounded-md px-3 py-2 text-sm font-medium text-slate-600 transition hover:bg-slate-100 hover:text-slate-950"
            >
              {item.label}
            </Link>
          ))}
        </nav>
      </div>
    </header>
  );
}
