// @MX:NOTE: 범주 리포트 페이지 — RSC가 루트 범주를 사전 로드하고 클라이언트에 전달.
// SPEC-AX-WEB-001 REQ-WEB-005 (Phase D) — `/dashboard/reports`.
// 모든 역할(viewer/analyst/admin) 읽기 허용 — RoleGate 불필요.

import { redirect } from "next/navigation";

import { ReportsClient } from "@/components/report/reports-client";
import { apiFetch, ApiError } from "@/lib/api-client";
import { getServerSession } from "@/lib/auth";
import type {
  EvaluationItem,
  EvaluationItemListResponse,
} from "@/types/evaluation";

/**
 * 범주 리포트 페이지.
 *
 * 동작:
 * 1) 세션 가드 — 미인증 시 /login redirect (DashboardLayout 2차 방어와 중복 방어).
 * 2) 루트 범주 목록 사전 로드 — `GET /api/v1/evaluation-items` 호출 후
 *    parent_id 부재 또는 depth === 0 항목만 필터링.
 * 3) 클라이언트 컴포넌트(ReportsClient)에 목록을 전달, 선택/리포트 fetch는 클라이언트가 담당.
 *
 * 범주 목록 조회 실패는 페이지 차원의 차단 오류로 처리 — 선택할 대상이 없으면 UI가 무의미.
 */
export default async function ReportsPage(): Promise<React.ReactElement> {
  const session = await getServerSession();
  if (!session) {
    redirect("/login");
  }

  const initial = await loadRootCategories();

  return (
    <section className="space-y-6">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">범주 리포트</h1>
        <p className="text-sm text-muted-foreground">
          범주를 선택하면 가중치 적용 점수와 등급이 표시됩니다.
        </p>
      </header>

      {initial.kind === "error" ? (
        <p
          className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive"
          role="alert"
          aria-live="polite"
        >
          {initial.message}
        </p>
      ) : (
        <ReportsClient categories={initial.categories} />
      )}
    </section>
  );
}

type InitialLoadResult =
  | { kind: "ok"; categories: ReadonlyArray<EvaluationItem> }
  | { kind: "error"; message: string };

/**
 * 루트 범주(parent_id === null/undefined 또는 depth === 0) 추출.
 *
 * 백엔드는 평탄화된 리스트를 반환하므로 클라이언트 측에서 루트만 골라낸다.
 * 두 조건 중 하나라도 만족하면 루트로 인정 — 백엔드 구현 차이 흡수.
 */
async function loadRootCategories(): Promise<InitialLoadResult> {
  try {
    const data = await apiFetch<EvaluationItemListResponse>(
      "/api/v1/evaluation-items",
    );
    const roots = (data.items ?? []).filter(
      (item) =>
        item.parent_id === null ||
        item.parent_id === undefined ||
        item.depth === 0,
    );
    return { kind: "ok", categories: roots };
  } catch (err) {
    if (err instanceof ApiError) {
      return {
        kind: "error",
        message:
          err.status === 401
            ? "인증이 만료되었습니다. 다시 로그인해주세요."
            : err.status === 403
              ? "범주 목록을 조회할 권한이 없습니다."
              : err.message,
      };
    }
    return {
      kind: "error",
      message: "범주 목록을 불러오지 못했습니다.",
    };
  }
}
