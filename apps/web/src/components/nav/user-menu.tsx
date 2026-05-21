"use client";

import { useTransition } from "react";
import { useRouter } from "next/navigation";

import { Button } from "@/components/ui/button";
import type { Role, UserSession } from "@/types/auth";

const ROLE_LABEL: Record<Role, string> = {
  admin: "관리자",
  analyst: "평가자",
  viewer: "열람자",
};

interface UserMenuProps {
  session: UserSession;
}

export function UserMenu({ session }: UserMenuProps): React.ReactElement {
  const router = useRouter();
  const [isPending, startTransition] = useTransition();

  const handleLogout = (): void => {
    startTransition(async () => {
      await fetch("/api/auth/logout", { method: "POST" });
      router.replace("/login");
      router.refresh();
    });
  };

  const displayName = session.name ?? session.email ?? session.sub;

  return (
    <div className="flex items-center gap-4">
      <div className="text-right">
        <div className="text-sm font-medium leading-none">{displayName}</div>
        <div className="mt-1 text-xs text-muted-foreground">
          {ROLE_LABEL[session.role]}
        </div>
      </div>
      <Button
        variant="outline"
        size="sm"
        onClick={handleLogout}
        disabled={isPending}
        aria-label="로그아웃"
      >
        {isPending ? "로그아웃 중…" : "로그아웃"}
      </Button>
    </div>
  );
}
