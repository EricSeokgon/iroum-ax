import type { Metadata } from "next";

import "./globals.css";

// SPEC-AX-WEB-001 §2.5 한국어 단일 — html lang="ko"

export const metadata: Metadata = {
  title: {
    default: "iroum-ax 경영평가 대시보드",
    template: "%s | iroum-ax",
  },
  description:
    "한국 공공기관(KEPCO E&C) 경영평가 PoC — 증빙·점수·리뷰·리포트 통합 대시보드",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>): React.ReactElement {
  return (
    <html lang="ko" suppressHydrationWarning>
      <body className="min-h-screen bg-background font-sans antialiased">
        {children}
      </body>
    </html>
  );
}
