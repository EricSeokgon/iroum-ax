// @MX:NOTE: 평가항목 페이지 — RSC가 초기 트리 fetch + 두-패널 클라이언트 컴포넌트 마운트.
// SPEC-AX-WEB-001 REQ-WEB-003/REQ-WEB-004 — Phase C 본 구현.

import { redirect } from "next/navigation";

import { TwoPanel } from "@/components/evaluation/two-panel";
import { apiFetch, ApiError } from "@/lib/api-client";
import { getServerSession } from "@/lib/auth";
import type { EvaluationItemListResponse } from "@/types/evaluation";

/**
 * 평가항목 페이지 (RSC).
 *
 * 초기 트리 데이터를 서버에서 fetch해 첫 페인트 깜빡임을 제거하고,
 * 두-패널 상호작용은 TwoPanel 클라이언트 컴포넌트가 담당한다.
 */
export default async function EvaluationItemsPage(): Promise<React.ReactElement> {
  const session = await getServerSession();
  if (!session) {
    redirect("/login");
  }

  const initial = await loadInitialItems();

  return (
    <section className="space-y-6">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">평가항목</h1>
        <p className="text-sm text-muted-foreground">
          평가항목 트리에서 항목을 선택해 상세 정보를 확인하고 점수를 입력합니다.
        </p>
      </header>

      <TwoPanel
        items={initial.kind === "ok" ? (initial.data.items ?? []) : []}
        fetchFailed={initial.kind === "error"}
        currentRole={session.role}
      />
    </section>
  );
}

type InitialLoadResult =
  | { kind: "ok"; data: EvaluationItemListResponse }
  | { kind: "error"; message: string };

async function loadInitialItems(): Promise<InitialLoadResult> {
  try {
    const data = await apiFetch<EvaluationItemListResponse>(
      "/api/v1/evaluation-items",
    );
    return { kind: "ok", data };
  } catch (err) {
    if (err instanceof ApiError) {
      return {
        kind: "error",
        message:
          err.status === 401
            ? "인증이 만료되었습니다. 다시 로그인해주세요."
            : err.status === 403
              ? "평가항목을 조회할 권한이 없습니다."
              : err.message,
      };
    }
    return {
      kind: "error",
      message: "평가항목을 불러오는 중 오류가 발생했습니다.",
    };
  }
}
