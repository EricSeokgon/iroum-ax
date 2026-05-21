"use client";

// @MX:NOTE: 리뷰 제출 폼 — analyst/admin만 가시. POST /api/v1/reviews 호출.
// SPEC-AX-WEB-001 Phase E REQ-WEB-006 — RoleGate 부모에서 차단.

import * as React from "react";
import { Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";

interface SubmitReviewFormProps {
  /** 제출 성공 후 호출 — 부모가 Kanban 새로고침 트리거 가능 */
  onSubmitted?: () => void;
}

type SubmitStatus =
  | { kind: "idle" }
  | { kind: "submitting" }
  | { kind: "success" }
  | { kind: "error"; message: string };

/**
 * 리뷰 제출 폼.
 *
 * 단일 진입점 — 사용자가 "리뷰 제출" 버튼 클릭 시 펼침/접힘 토글.
 * 빈 title/comment도 백엔드가 허용한다는 SPEC 기준이지만 UX 차원에서
 * title 미입력 시 제출 버튼을 비활성화하지는 않는다 (백엔드 결정 존중).
 */
export function SubmitReviewForm({
  onSubmitted,
}: SubmitReviewFormProps): React.ReactElement {
  const [open, setOpen] = React.useState<boolean>(false);
  const [title, setTitle] = React.useState<string>("");
  const [comment, setComment] = React.useState<string>("");
  const [status, setStatus] = React.useState<SubmitStatus>({ kind: "idle" });

  function reset(): void {
    setTitle("");
    setComment("");
    setStatus({ kind: "idle" });
  }

  async function handleSubmit(
    event: React.FormEvent<HTMLFormElement>,
  ): Promise<void> {
    event.preventDefault();
    setStatus({ kind: "submitting" });

    const body: { title?: string; comment?: string } = {};
    const trimmedTitle = title.trim();
    const trimmedComment = comment.trim();
    if (trimmedTitle !== "") body.title = trimmedTitle;
    if (trimmedComment !== "") body.comment = trimmedComment;

    try {
      const response = await fetch("/api/v1/reviews", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });

      if (!response.ok) {
        const message = await readErrorMessage(response);
        setStatus({ kind: "error", message });
        return;
      }

      setStatus({ kind: "success" });
      setTitle("");
      setComment("");
      setOpen(false);

      if (onSubmitted) onSubmitted();
      if (typeof window !== "undefined") {
        window.dispatchEvent(new CustomEvent("iroum-ax:review:refresh"));
      }
    } catch {
      setStatus({
        kind: "error",
        message: "요청에 실패했습니다.",
      });
    }
  }

  const submitting = status.kind === "submitting";

  if (!open) {
    return (
      <div className="flex flex-col items-start gap-2">
        <Button
          type="button"
          onClick={() => {
            reset();
            setOpen(true);
          }}
        >
          + 리뷰 제출
        </Button>
        {status.kind === "success" && (
          <p
            className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700"
            role="status"
            aria-live="polite"
          >
            리뷰가 제출되었습니다.
          </p>
        )}
      </div>
    );
  }

  return (
    <form
      onSubmit={handleSubmit}
      className="space-y-3 rounded-lg border bg-card p-4"
      aria-labelledby="submit-review-form-heading"
    >
      <header className="flex items-center justify-between">
        <h2
          id="submit-review-form-heading"
          className="text-base font-semibold"
        >
          새 리뷰 제출
        </h2>
      </header>

      <div className="space-y-1">
        <label htmlFor="review-title" className="text-sm font-medium">
          제목
        </label>
        <input
          id="review-title"
          type="text"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          disabled={submitting}
          maxLength={200}
          className="block w-full rounded-md border bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
        />
      </div>

      <div className="space-y-1">
        <label htmlFor="review-comment" className="text-sm font-medium">
          코멘트 (선택)
        </label>
        <textarea
          id="review-comment"
          value={comment}
          onChange={(e) => setComment(e.target.value)}
          disabled={submitting}
          rows={3}
          maxLength={2000}
          className="block w-full rounded-md border bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
        />
      </div>

      {status.kind === "error" && (
        <p
          className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive"
          role="alert"
          aria-live="polite"
        >
          {status.message}
        </p>
      )}

      <div className="flex items-center justify-end gap-2">
        <Button
          type="button"
          variant="outline"
          onClick={() => {
            reset();
            setOpen(false);
          }}
          disabled={submitting}
        >
          취소
        </Button>
        <Button type="submit" disabled={submitting}>
          {submitting ? (
            <>
              <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
              제출 중...
            </>
          ) : (
            "리뷰 제출"
          )}
        </Button>
      </div>
    </form>
  );
}

/**
 * 백엔드 표준 에러 envelope에서 한국어 메시지를 추출.
 */
async function readErrorMessage(response: Response): Promise<string> {
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
  if (response.status === 403)
    return "리뷰를 제출할 권한이 없습니다.";
  return "요청에 실패했습니다.";
}
