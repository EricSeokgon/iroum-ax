import { redirect } from "next/navigation";

import { getServerSession } from "@/lib/auth";

// 루트 진입 — 세션 보유 시 대시보드, 미보유 시 로그인으로 분기.

export default async function HomePage(): Promise<never> {
  const session = await getServerSession();

  if (session) {
    redirect("/dashboard");
  }
  redirect("/login");
}
