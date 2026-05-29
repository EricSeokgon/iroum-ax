import { redirect } from "next/navigation";

import { Sidebar } from "@/components/nav/sidebar";
import { UserMenu } from "@/components/nav/user-menu";
import { getServerSession } from "@/lib/auth";

// SPEC-AX-WEB-001 REQ-WEB-001b — 2차 방어선: middleware 통과 후
// JWT 디코드 실패/만료 시에도 RSC 단계에서 다시 안전 복귀.

export default async function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}): Promise<React.ReactElement> {
  const session = await getServerSession();

  if (!session) {
    // 토큰 디코드 실패 또는 만료 — 클라이언트로 어떤 데이터도 노출하지 않음
    redirect("/login");
  }

  return (
    <div className="flex h-screen">
      <Sidebar currentRole={session.role} />
      <div className="flex flex-1 flex-col overflow-hidden">
        <header
          className="flex h-14 items-center justify-end border-b bg-background px-6"
          aria-label="상단 메뉴"
        >
          <UserMenu session={session} />
        </header>
        <main className="flex-1 overflow-auto p-6">{children}</main>
      </div>
    </div>
  );
}
