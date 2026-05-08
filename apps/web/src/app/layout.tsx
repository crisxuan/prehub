import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "PreHub",
  description: "每天发现一个值得收藏的 GitHub 项目。",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="zh-CN" className="h-full antialiased">
      <body className="min-h-full flex flex-col">{children}</body>
    </html>
  );
}
