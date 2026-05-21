"use client";

// @MX:ANCHOR: Kanban 보드 — 4-칼럼 리뷰 시각화 + 액션 통합 컨테이너.
// @MX:REASON: page RSC가 초기 데이터 전달, ReviewCard 액션 콜백 / SubmitReviewForm refresh / window 이벤트 3종이 본 컴포넌트의 fetch 경로를 공유 (fan_in == 3).
// SPEC-AX-WEB-001 Phase E REQ-WEB-006.

import * as React from "react";
import { Loader2 } from "lucide-react";

import {
  ReviewCard,
  type ReviewCardActionKind,
} from "@/components/review/review-card";
import { ReviewDetail } from "@/components/review/review-detail";
import type { Role } from "@/types/auth";
import {
  REVIEW_STATUS_COLUMNS,
  REVIEW_STATUS_LABEL,
  type Review,
  type ReviewListResponse,
  type ReviewStatus,
} from "@/types/review";

interface KanbanBoardProps {
  /** RSC가 사전 로드한 초기 리뷰 목록 */
  initial: ReviewListResponse;
  /** 현재 사용자 역할 — admin만 카드 액션 버튼 표시 */
  currentRole: Role;
}

/**
 * Kanban 보드.
 *
 * - 4-칼럼(SUBMITTED/UNDER_REVIEW/APPROVED/REJECTED) 분류 표시.
 * - 카드 클릭 시 ReviewDetail 모달, admin 액션은 카드 내부에서 처리.
 * - SubmitReviewForm 성공 또는 카드 액션 성공 시 list 새로고침.
 *   외부 trigger는 `iroum-ax:review:refresh` 윈도우 이벤트로도 수신한다.
 *
 * @MX:NOTE: 외부 상태관리 라이브러리 없이 useState로 PoC 범위 충분.
 */
export function KanbanBoard({
  initial,
  currentRole,
}: KanbanBoardProps): React.ReactElement {
  const [data, setData] = React.useState<ReviewListResponse>(initial);
  const [loading, setLoading] = React.useState<boolean>(false);
  const [errorMessage, setErrorMessage] = React.useState<string | null>(null);
  const [successMessage, setSuccessMessage] = React.useState<string | null>(
    null,
  );
  const [selectedId, setSelectedId] = React.useState<string | null>(null);

  /**
   * SubmitReviewForm 성공/외부 trigger 시 list 새로고침.
   */
  React.useEffect(() => {
    function handleRefresh(): void {
      void fetchAll();
    }
    window.addEventListener("iroum-ax:review:refresh", handleRefresh);
    return () => {
      window.removeEventListener("iroum-ax:review:refresh", handleRefresh);
    };
    // fetchAll은 setState만 호출하므로 의존성 미포함 (effect 재구독 방지)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function fetchAll(): Promise<void> {
    setLoading(true);
    setErrorMessage(null);
    try {
      const response = await fetch("/api/v1/reviews", {
        method: "GET",
        cache: "no-store",
      });
      if (!response.ok) {
        const body = (await response.json().catch(() => ({}))) as {
          error?: { message?: string };
        };
        setErrorMessage(
          body.error?.message ?? "리뷰 목록을 불러오는 중 오류가 발생했습니다.",
        );
        return;
      }
      const next = (await response.json()) as ReviewListResponse;
      setData(next);
    } catch {
      setErrorMessage("리뷰 목록을 불러오는 중 오류가 발생했습니다.");
    } finally {
      setLoading(false);
    }
  }

  function handleActionSuccess(actionKind: ReviewCardActionKind): void {
    if (actionKind === "assign") {
      setSuccessMessage("리뷰어가 배정되었습니다.");
    } else if (actionKind === "approve") {
      setSuccessMessage("리뷰가 승인되었습니다.");
    } else {
      setSuccessMessage("리뷰가 반려되었습니다.");
    }
    setErrorMessage(null);
    void fetchAll();
  }

  function handleActionError(message: string): void {
    setErrorMessage(message);
    setSuccessMessage(null);
  }

  // 4-칼럼 분류 — 알 수 없는 상태는 어떤 칼럼에도 표시되지 않는다 (fail-soft).
  const grouped = groupByStatus(data.reviews ?? []);

  return (
    <div className="space-y-3">
      {errorMessage !== null && (
        <p
          className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive"
          role="alert"
          aria-live="polite"
        >
          {errorMessage}
        </p>
      )}

      {successMessage !== null && (
        <p
          className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700"
          role="status"
          aria-live="polite"
        >
          {successMessage}
        </p>
      )}

      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <p>총 {data.total.toLocaleString("ko-KR")}건</p>
        {loading && (
          <span className="inline-flex items-center gap-1.5">
            <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden="true" />
            새로고침 중...
          </span>
        )}
      </div>

      {/* 4-칼럼 그리드 — 가로 스크롤 허용. 모바일에서는 칼럼 폭 280px 보장 */}
      <div
        className="grid grid-flow-col auto-cols-[minmax(280px,1fr)] gap-4 overflow-x-auto pb-2"
        aria-label="리뷰 워크플로 칼럼"
      >
        {REVIEW_STATUS_COLUMNS.map((status) => (
          <Column
            key={status}
            status={status}
            reviews={grouped[status]}
            currentRole={currentRole}
            onOpen={setSelectedId}
            onActionSuccess={handleActionSuccess}
            onActionError={handleActionError}
          />
        ))}
      </div>

      <ReviewDetail
        reviewId={selectedId}
        onClose={() => setSelectedId(null)}
      />
    </div>
  );
}

interface ColumnProps {
  status: ReviewStatus;
  reviews: ReadonlyArray<Review>;
  currentRole: Role;
  onOpen: (id: string) => void;
  onActionSuccess: (kind: ReviewCardActionKind) => void;
  onActionError: (message: string) => void;
}

function Column({
  status,
  reviews,
  currentRole,
  onOpen,
  onActionSuccess,
  onActionError,
}: ColumnProps): React.ReactElement {
  const label = REVIEW_STATUS_LABEL[status];
  const headerClasses = columnHeaderClasses(status);

  return (
    <section
      className="flex h-[calc(100vh-22rem)] min-h-[24rem] flex-col rounded-lg border bg-muted/30"
      aria-labelledby={`column-${status}`}
    >
      <header
        className={`flex items-center justify-between rounded-t-lg px-3 py-2 ${headerClasses}`}
      >
        <h2
          id={`column-${status}`}
          className="text-sm font-semibold tracking-tight"
        >
          {label}
        </h2>
        <span className="rounded-full bg-background/70 px-2 py-0.5 text-xs font-medium tabular-nums">
          {reviews.length}
        </span>
      </header>
      <div className="flex-1 space-y-2 overflow-y-auto p-2">
        {reviews.length === 0 ? (
          <p className="px-2 py-4 text-center text-xs text-muted-foreground">
            없음
          </p>
        ) : (
          reviews.map((review) => (
            <ReviewCard
              key={review.id}
              review={review}
              currentRole={currentRole}
              onOpen={onOpen}
              onActionSuccess={onActionSuccess}
              onActionError={onActionError}
            />
          ))
        )}
      </div>
    </section>
  );
}

/**
 * 칼럼 헤더 색상 — 상태별 시각 구분.
 * 무지개색 회피, 의미론적 매핑(녹/적/청/회색) 적용.
 */
function columnHeaderClasses(status: ReviewStatus): string {
  switch (status) {
    case "SUBMITTED":
      return "bg-blue-100 text-blue-800";
    case "UNDER_REVIEW":
      return "bg-yellow-100 text-yellow-800";
    case "APPROVED":
      return "bg-emerald-100 text-emerald-800";
    case "REJECTED":
      return "bg-red-100 text-red-800";
  }
}

/**
 * 리뷰 배열을 상태별로 분류. 알 수 없는 status는 어떤 그룹에도 포함되지 않는다.
 */
function groupByStatus(
  reviews: ReadonlyArray<Review>,
): Record<ReviewStatus, Review[]> {
  const result: Record<ReviewStatus, Review[]> = {
    SUBMITTED: [],
    UNDER_REVIEW: [],
    APPROVED: [],
    REJECTED: [],
  };
  for (const review of reviews) {
    if (review.status in result) {
      result[review.status].push(review);
    }
  }
  return result;
}
