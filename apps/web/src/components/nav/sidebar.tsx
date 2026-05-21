import Link from "next/link";

import { cn } from "@/lib/utils";
import type { Role } from "@/types/auth";

// SPEC-AX-WEB-001 §1.4 + 5.6/5.7 — RBAC 기반 메뉴 가시성.

interface NavItem {
  href: string;
  label: string;
  allow: ReadonlyArray<Role>;
}

const NAV_ITEMS: ReadonlyArray<NavItem> = [
  { href: "/dashboard/evidence", label: "증빙 관리", allow: ["viewer", "analyst", "admin"] },
  { href: "/dashboard/evaluation-items", label: "평가 항목", allow: ["viewer", "analyst", "admin"] },
  { href: "/dashboard/scores", label: "점수 입력", allow: ["analyst", "admin"] },
  { href: "/dashboard/reports", label: "리포트", allow: ["viewer", "analyst", "admin"] },
  { href: "/dashboard/reviews", label: "리뷰 워크플로", allow: ["viewer", "analyst", "admin"] },
  { href: "/dashboard/audit-logs", label: "감사 로그", allow: ["admin"] },
  { href: "/dashboard/rubric", label: "루브릭 설정", allow: ["admin"] },
];

interface SidebarProps {
  currentRole: Role;
  /** 현재 경로 (active state 표시용, 선택) */
  activePath?: string;
}

export function Sidebar({
  currentRole,
  activePath,
}: SidebarProps): React.ReactElement {
  const visibleItems = NAV_ITEMS.filter((item) =>
    item.allow.includes(currentRole),
  );

  return (
    <aside
      aria-label="주요 메뉴"
      className="flex h-full w-60 flex-col border-r bg-muted/30 px-3 py-6"
    >
      <div className="px-3 pb-6 text-lg font-semibold tracking-tight">
        경영평가
      </div>
      <nav className="flex flex-1 flex-col gap-1">
        {visibleItems.map((item) => (
          <Link
            key={item.href}
            href={item.href}
            className={cn(
              "rounded-md px-3 py-2 text-sm transition-colors hover:bg-accent hover:text-accent-foreground",
              activePath === item.href &&
                "bg-accent text-accent-foreground font-medium",
            )}
          >
            {item.label}
          </Link>
        ))}
      </nav>
    </aside>
  );
}
