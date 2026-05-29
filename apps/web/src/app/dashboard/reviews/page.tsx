// @MX:NOTE: 리뷰 워크플로 페이지 — RSC가 초기 리뷰 목록 fetch + Kanban 보드 마운트.
// SPEC-AX-WEB-001 Phase E REQ-WEB-006 — `/dashboard/reviews`.
// 모든 역할(viewer/analyst/admin) 읽기 허용 — SubmitReviewForm은 RoleGate로 차단.

import { redirect } from "next/navigation";

import { RoleGate } from "@/components/auth/role-gate";
import { KanbanBoard } from "@/components/review/kanban-board";
import { SubmitReviewForm } from "@/components/review/submit-review-form";
import { apiFetch, ApiError } from "@/lib/api-client";
import { getServerSession } from "@/lib/auth";
import type { ReviewListResponse } from "@/types/review";

/**
 * 리뷰 워크플로 페이지.
 *
 * 동작:
 * 1) 세션 가드 — 미인증 시 /login redirect (DashboardLayout과 중복 방어).
 * 2) 초기 리뷰 목록 사전 로드 — `GET /api/v1/reviews` 호출.
 * 3) analyst/admin 한정 SubmitReviewForm 노출, 모든 역할에 KanbanBoard 노출.
 *
 * 초기 로드 실패는 페이지 차원의 차단 오류로 처리하지 않고 빈 보드를 표시한다 —
 * 사용자가 SubmitReviewForm으로 새 리뷰 제출은 시도할 수 있도록 한다.
 */
export default async function ReviewsPage(): Promise<React.ReactElement> {
  const session = await getServerSession();
  if (!session) {
    redirect("/login");
  }

  const initial = await loadInitialReviews();

  return (
    <section className="space-y-6">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">리뷰 워크플로</h1>
        <p className="text-sm text-muted-foreground">
          제출된 리뷰의 진행 상태를 칼럼별로 확인하고 처리합니다.
        </p>
      </header>

      <RoleGate currentRole={session.role} allow={["analyst", "admin"]}>
        <SubmitReviewForm />
      </RoleGate>

      <KanbanBoard
        initial={initial.kind === "ok" ? initial.data : { reviews: [], total: 0 }}
        fetchFailed={initial.kind === "error"}
        currentRole={session.role}
      />
    </section>
  );
}

type InitialLoadResult =
  | { kind: "ok"; data: ReviewListResponse }
  | { kind: "error"; message: string };

/**
 * 초기 리뷰 목록을 서버에서 fetch — 실패해도 페이지가 살아남도록 결과를 객체로 wrap.
 */
async function loadInitialReviews(): Promise<InitialLoadResult> {
  try {
    const data = await apiFetch<ReviewListResponse>("/api/v1/reviews");
    return { kind: "ok", data };
  } catch (err) {
    if (err instanceof ApiError) {
      return {
        kind: "error",
        message:
          err.status === 401
            ? "인증이 만료되었습니다. 다시 로그인해주세요."
            : err.status === 403
              ? "리뷰 목록을 조회할 권한이 없습니다."
              : err.message,
      };
    }
    return {
      kind: "error",
      message: "리뷰 목록을 불러오는 중 오류가 발생했습니다.",
    };
  }
}
