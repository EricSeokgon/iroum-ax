"use client";

// @MX:NOTE: 리뷰 상세 모달 — 카드 클릭 시 GET /api/v1/reviews/{id} 호출.
// SPEC-AX-WEB-001 Phase E REQ-WEB-006 — evidence-detail-modal 패턴 정합.

import * as React from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { Loader2, X } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  REVIEW_STATUS_LABEL,
  type Review,
  type ReviewStatus,
} from "@/types/review";

interface ReviewDetailProps {
  /** 표시할 리뷰 ID. null이면 모달 닫힘. */
  reviewId: string | null;
  /** 사용자가 모달을 닫을 때 호출되는 콜백 */
  onClose: () => void;
}

/**
 * 리뷰 상세 모달 — BFF 프록시(`/api/v1/reviews/{id}`)를 통해 백엔드 호출.
 * 토큰은 쿠키에 머무르므로 fetch는 credentials: 'same-origin' (브라우저 기본).
 */
export function ReviewDetail({
  reviewId,
  onClose,
}: ReviewDetailProps): React.ReactElement {
  const [review, setReview] = React.useState<Review | null>(null);
  const [loading, setLoading] = React.useState<boolean>(false);
  const [errorMessage, setErrorMessage] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!reviewId) {
      setReview(null);
      setErrorMessage(null);
      return;
    }

    let cancelled = false;
    setLoading(true);
    setErrorMessage(null);
    setReview(null);

    fetch(`/api/v1/reviews/${encodeURIComponent(reviewId)}`, {
      method: "GET",
      cache: "no-store",
    })
      .then(async (response) => {
        if (cancelled) return;
        if (!response.ok) {
          const message = await safeParseErrorMessage(response);
          setErrorMessage(message);
          return;
        }
        const data = (await response.json()) as Review;
        setReview(data);
      })
      .catch(() => {
        if (cancelled) return;
        setErrorMessage("리뷰 정보를 불러오는 중 오류가 발생했습니다.");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [reviewId]);

  const open = reviewId !== null;

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(next) => {
        if (!next) onClose();
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-50 bg-black/50" />
        <Dialog.Content
          className="fixed left-1/2 top-1/2 z-50 w-full max-w-lg -translate-x-1/2 -translate-y-1/2 rounded-lg border bg-background p-6 shadow-lg focus:outline-none"
          aria-describedby={undefined}
        >
          <div className="flex items-start justify-between">
            <Dialog.Title className="text-lg font-semibold">
              리뷰 상세
            </Dialog.Title>
            <Dialog.Close asChild>
              <button
                type="button"
                aria-label="닫기"
                className="rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                <X className="h-4 w-4" />
              </button>
            </Dialog.Close>
          </div>

          <div className="mt-4 min-h-[8rem]">
            {loading && (
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
                <span>리뷰 정보를 불러오는 중입니다...</span>
              </div>
            )}

            {!loading && errorMessage !== null && (
              <p
                className="text-sm text-destructive"
                role="alert"
                aria-live="polite"
              >
                {errorMessage}
              </p>
            )}

            {!loading && review !== null && (
              <dl className="grid grid-cols-1 gap-2 text-sm sm:grid-cols-[8rem_1fr]">
                <dt className="text-muted-foreground">ID</dt>
                <dd className="break-all font-mono text-xs">{review.id}</dd>

                <dt className="text-muted-foreground">상태</dt>
                <dd>{labelFromStatus(review.status)}</dd>

                {review.title !== undefined && (
                  <>
                    <dt className="text-muted-foreground">제목</dt>
                    <dd className="break-all">{review.title}</dd>
                  </>
                )}

                {review.comment !== undefined && (
                  <>
                    <dt className="text-muted-foreground">코멘트</dt>
                    <dd className="whitespace-pre-wrap break-words">
                      {review.comment}
                    </dd>
                  </>
                )}

                {review.submitted_by !== undefined && (
                  <>
                    <dt className="text-muted-foreground">제출자</dt>
                    <dd className="break-all">{review.submitted_by}</dd>
                  </>
                )}

                {review.reviewer_id !== undefined && (
                  <>
                    <dt className="text-muted-foreground">리뷰어</dt>
                    <dd className="break-all">{review.reviewer_id}</dd>
                  </>
                )}

                {review.submitted_at !== undefined && (
                  <>
                    <dt className="text-muted-foreground">제출 시각</dt>
                    <dd>{formatDateTime(review.submitted_at)}</dd>
                  </>
                )}

                {review.reviewed_at !== undefined && (
                  <>
                    <dt className="text-muted-foreground">처리 시각</dt>
                    <dd>{formatDateTime(review.reviewed_at)}</dd>
                  </>
                )}

                <dt className="text-muted-foreground">생성 시각</dt>
                <dd>{formatDateTime(review.created_at)}</dd>

                {review.updated_at !== undefined && (
                  <>
                    <dt className="text-muted-foreground">업데이트 시각</dt>
                    <dd>{formatDateTime(review.updated_at)}</dd>
                  </>
                )}
              </dl>
            )}
          </div>

          <div className="mt-6 flex justify-end">
            <Dialog.Close asChild>
              <Button variant="outline" type="button">
                닫기
              </Button>
            </Dialog.Close>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

/**
 * 백엔드 에러 응답에서 사용자 친화적 한국어 메시지를 추출.
 */
async function safeParseErrorMessage(response: Response): Promise<string> {
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
  if (response.status === 403) return "이 리뷰를 조회할 권한이 없습니다.";
  if (response.status === 404) return "리뷰를 찾을 수 없습니다.";
  return "리뷰 정보를 불러오는 중 오류가 발생했습니다.";
}

function labelFromStatus(status: string): string {
  if (status in REVIEW_STATUS_LABEL) {
    return REVIEW_STATUS_LABEL[status as ReviewStatus];
  }
  return status;
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
