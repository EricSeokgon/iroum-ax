"use client";

// @MX:NOTE: 리뷰 카드 — Kanban 칼럼의 한 항목. 클릭 시 상세 모달, 액션 버튼은 admin·상태별 조건부 노출.
// SPEC-AX-WEB-001 Phase E REQ-WEB-006 — admin only: 배정/승인/반려 (백엔드 RBAC가 최종 신뢰).

import * as React from "react";
import { Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import type { Role } from "@/types/auth";
import type { Review } from "@/types/review";

/**
 * 카드에서 수행 가능한 admin 액션 종류.
 * 부모(Kanban)가 액션별 토스트 문구를 결정하기 위해 콜백에 포함된다.
 */
export type ReviewCardActionKind = "assign" | "approve" | "reject";

interface ReviewCardProps {
  /** 표시할 리뷰 단건 */
  review: Review;
  /** 현재 사용자 역할 — admin만 액션 버튼 노출 */
  currentRole: Role;
  /** 카드 본문 클릭 시 상세 모달 열기 */
  onOpen: (id: string) => void;
  /** 액션 성공 후 부모(Kanban)에 알림 — 액션 종류 포함 */
  onActionSuccess: (kind: ReviewCardActionKind) => void;
  /** 액션 실패 시 부모에 메시지 전파 — 상단 토스트/알럿 영역 */
  onActionError: (message: string) => void;
}

type CardActionState =
  | { kind: "idle" }
  | { kind: "assign"; reviewerId: string }
  | { kind: "approve"; comment: string }
  | { kind: "reject"; comment: string }
  | { kind: "submitting" };

/**
 * 리뷰 카드.
 *
 * - 본문 영역(제목/메타) 클릭 → onOpen 호출(상세 모달).
 * - admin + 상태가 SUBMITTED → "리뷰어 배정" 버튼 노출.
 * - admin + 상태가 UNDER_REVIEW → "승인"/"반려" 버튼 노출.
 * - 액션 인라인 폼은 카드 내부에서 펼쳐진다 (별도 모달 없음 — 카드 단위 컴팩트 UX).
 */
export function ReviewCard({
  review,
  currentRole,
  onOpen,
  onActionSuccess,
  onActionError,
}: ReviewCardProps): React.ReactElement {
  const [action, setAction] = React.useState<CardActionState>({ kind: "idle" });

  const isAdmin = currentRole === "admin";
  const showAssign = isAdmin && review.status === "SUBMITTED";
  const showApproveReject = isAdmin && review.status === "UNDER_REVIEW";

  async function performAssign(reviewerId: string): Promise<void> {
    const trimmed = reviewerId.trim();
    if (trimmed === "") {
      onActionError("리뷰어 ID를 입력하세요.");
      return;
    }
    setAction({ kind: "submitting" });
    try {
      const response = await fetch(
        `/api/v1/reviews/${encodeURIComponent(review.id)}/assign-reviewer`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ reviewer_id: trimmed }),
        },
      );
      if (!response.ok) {
        onActionError(await readErrorMessage(response, "assign"));
        setAction({ kind: "idle" });
        return;
      }
      onActionSuccess("assign");
      setAction({ kind: "idle" });
    } catch {
      onActionError("요청에 실패했습니다.");
      setAction({ kind: "idle" });
    }
  }

  async function performApproveOrReject(
    kind: "approve" | "reject",
    comment: string,
  ): Promise<void> {
    setAction({ kind: "submitting" });
    const trimmed = comment.trim();
    const body: { comment?: string } = trimmed === "" ? {} : { comment: trimmed };
    try {
      const response = await fetch(
        `/api/v1/reviews/${encodeURIComponent(review.id)}/${kind}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        },
      );
      if (!response.ok) {
        onActionError(await readErrorMessage(response, kind));
        setAction({ kind: "idle" });
        return;
      }
      onActionSuccess(kind);
      setAction({ kind: "idle" });
    } catch {
      onActionError("요청에 실패했습니다.");
      setAction({ kind: "idle" });
    }
  }

  const submitting = action.kind === "submitting";

  // 카드 본문 클릭 → 상세 모달. 액션 폼이 열려있는 동안에는 클릭 무시.
  function handleOpen(): void {
    if (action.kind !== "idle") return;
    onOpen(review.id);
  }

  return (
    <article
      className="space-y-2 rounded-md border bg-card p-3 shadow-sm"
      aria-label={`리뷰 ${review.title ?? review.id}`}
    >
      {/* 본문 — 클릭 시 상세 */}
      <button
        type="button"
        onClick={handleOpen}
        className="flex w-full flex-col items-start gap-1 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm"
        disabled={action.kind !== "idle"}
      >
        <p className="line-clamp-2 text-sm font-medium">
          {review.title !== undefined && review.title.trim() !== ""
            ? review.title
            : review.id}
        </p>
        <p className="font-mono text-xs text-muted-foreground">
          {truncateId(review.id)}
        </p>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-muted-foreground">
          {review.submitted_by !== undefined && (
            <span aria-label={`제출자 ${review.submitted_by}`}>
              {review.submitted_by}
            </span>
          )}
          <span aria-label={`생성 시각 ${review.created_at}`}>
            {formatDateTime(review.created_at)}
          </span>
        </div>
      </button>

      {/* 액션 영역 */}
      {(showAssign || showApproveReject) && action.kind === "idle" && (
        <div className="flex flex-wrap gap-2 pt-1">
          {showAssign && (
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={() => setAction({ kind: "assign", reviewerId: "" })}
            >
              리뷰어 배정
            </Button>
          )}
          {showApproveReject && (
            <>
              <Button
                type="button"
                size="sm"
                onClick={() => setAction({ kind: "approve", comment: "" })}
              >
                승인
              </Button>
              <Button
                type="button"
                size="sm"
                variant="destructive"
                onClick={() => setAction({ kind: "reject", comment: "" })}
              >
                반려
              </Button>
            </>
          )}
        </div>
      )}

      {action.kind === "assign" && (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void performAssign(action.reviewerId);
          }}
          className="space-y-2 pt-1"
        >
          <label
            htmlFor={`reviewer-${review.id}`}
            className="block text-xs font-medium"
          >
            리뷰어 ID
          </label>
          <input
            id={`reviewer-${review.id}`}
            type="text"
            value={action.reviewerId}
            onChange={(e) =>
              setAction({ kind: "assign", reviewerId: e.target.value })
            }
            autoFocus
            disabled={submitting}
            className="block w-full rounded-md border bg-background px-2 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
          />
          <div className="flex items-center justify-end gap-1.5">
            <Button
              type="button"
              size="sm"
              variant="ghost"
              onClick={() => setAction({ kind: "idle" })}
            >
              취소
            </Button>
            <Button type="submit" size="sm">
              확인
            </Button>
          </div>
        </form>
      )}

      {(action.kind === "approve" || action.kind === "reject") && (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            const kind = action.kind === "approve" ? "approve" : "reject";
            void performApproveOrReject(kind, action.comment);
          }}
          className="space-y-2 pt-1"
        >
          <label
            htmlFor={`comment-${review.id}`}
            className="block text-xs font-medium"
          >
            코멘트 (선택)
          </label>
          <textarea
            id={`comment-${review.id}`}
            value={action.comment}
            onChange={(e) => {
              const next = e.target.value;
              setAction((prev) =>
                prev.kind === "approve"
                  ? { kind: "approve", comment: next }
                  : prev.kind === "reject"
                    ? { kind: "reject", comment: next }
                    : prev,
              );
            }}
            autoFocus
            rows={2}
            maxLength={2000}
            disabled={submitting}
            className="block w-full rounded-md border bg-background px-2 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
          />
          <div className="flex items-center justify-end gap-1.5">
            <Button
              type="button"
              size="sm"
              variant="ghost"
              onClick={() => setAction({ kind: "idle" })}
            >
              취소
            </Button>
            <Button
              type="submit"
              size="sm"
              variant={action.kind === "approve" ? "default" : "destructive"}
            >
              {action.kind === "approve" ? "승인" : "반려"}
            </Button>
          </div>
        </form>
      )}

      {submitting && (
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
          <Loader2 className="h-3 w-3 animate-spin" aria-hidden="true" />
          <span>처리 중...</span>
        </div>
      )}
    </article>
  );
}

/**
 * 백엔드 에러 envelope에서 한국어 메시지 추출.
 * 액션 종류별 맞춤 fallback을 제공.
 */
async function readErrorMessage(
  response: Response,
  action: "assign" | "approve" | "reject",
): Promise<string> {
  try {
    const body = (await response.json()) as {
      error?: { message?: string };
    };
    if (body.error?.message && typeof body.error.message === "string") {
      return body.error.message;
    }
  } catch {
    // ignore
  }
  if (response.status === 401)
    return "인증이 만료되었습니다. 다시 로그인해주세요.";
  if (response.status === 403) {
    if (action === "assign") return "리뷰어를 배정할 권한이 없습니다.";
    if (action === "approve") return "리뷰를 승인할 권한이 없습니다.";
    return "리뷰를 반려할 권한이 없습니다.";
  }
  if (response.status === 404) return "리뷰를 찾을 수 없습니다.";
  return "요청에 실패했습니다.";
}

/**
 * 긴 ID를 8자 + … 형태로 축약 표시.
 */
function truncateId(id: string): string {
  if (id.length <= 12) return id;
  return `${id.slice(0, 8)}…`;
}

function formatDateTime(iso: string): string {
  try {
    const date = new Date(iso);
    if (Number.isNaN(date.getTime())) return iso;
    return date.toLocaleString("ko-KR", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return iso;
  }
}
