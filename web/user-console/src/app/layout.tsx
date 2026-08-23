import type { Metadata } from "next";

import "./globals.css";

export const metadata: Metadata = {
  title: "Tenara Console",
  description: "Ship apps by talking to your coding agent",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN">
      <body className="min-h-screen bg-neutral-950 text-neutral-100 antialiased">
        <main className="mx-auto max-w-2xl px-6 py-12">{children}</main>
      </body>
    </html>
  );
}
