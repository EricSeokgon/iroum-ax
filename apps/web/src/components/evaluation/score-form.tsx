"use client";

// @MX:NOTE: 점수 입력/수정 폼 — analyst/admin에서만 노출 (RoleGate 부모에서 차단).
// SPEC-AX-WEB-001 REQ-WEB-004 family — 점수 CRUD.

import * as React from "react";
import { Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  SCORE_DEFAULT_MAX,
  type Score,
} from "@/types/evaluation";

interface ScoreFormProps {
  /** 채점 대상 평가항목 ID */
  evalItemId: string;
  /** 평가항목의 만점 — 클라이언트 측 상한 검증에 사용 (없으면 SCORE_DEFAULT_MAX) */
  maxScore?: number;
  /** 수정 모드 시 기존 점수 — 없으면 신규 입력 */
  existing?: Score | null;
  /** 저장 성공 후 호출 — 부모가 ScoreList 새로고침 트리거 */
  onSaved: (saved: Score) => void;
  /** 수정 모드 취소 콜백 (옵션) */
  onCancelEdit?: () => void;
}

type FormStatus =
  | { kind: "idle" }
  | { kind: "submitting" }
  | { kind: "error"; message: string }
  | { kind: "success"; message: string };

/**
 * 점수 입력/수정 폼.
 *
 * - existing이 null/undefined: POST /api/v1/scores (신규)
 * - existing 존재: PUT /api/v1/scores/{id} (수정)
 *
 * 보안 경계 메모: 백엔드 RBAC가 최종 인가의 신뢰의 원천.
 * UI는 UX 보조선이며 RoleGate가 차단하지 않은 경우에도 백엔드가 403으로 거부할 수 있다.
 */
export function ScoreForm({
  evalItemId,
  maxScore,
  existing,
  onSaved,
  onCancelEdit,
}: ScoreFormProps): React.ReactElement {
  const effectiveMax = maxScore ?? SCORE_DEFAULT_MAX;
  const isEditMode = existing !== undefined && existing !== null;

  // existing/evalItemId가 바뀌면 폼 상태를 재초기화 — 다른 항목/점수로 전환 시 stale 입력 방지
  const [valueText, setValueText] = React.useState<string>(() =>
    existing ? String(existing.value) : "",
  );
  const [comment, setComment] = React.useState<string>(() =>
    existing?.comment ?? "",
  );
  const [status, setStatus] = React.useState<FormStatus>({ kind: "idle" });

  React.useEffect(() => {
    setValueText(existing ? String(existing.value) : "");
    setComment(existing?.comment ?? "");
    setStatus({ kind: "idle" });
  }, [evalItemId, existing]);

  /**
   * 입력 검증 — 숫자 형식 + 범위 [0, effectiveMax].
   * Zod 의존성 도입 대신 PoC 범위에서 수동 검증으로 충분.
   */
  function validateValue(text: string): { ok: true; value: number } | { ok: false; message: string } {
    const trimmed = text.trim();
    if (trimmed === "") {
      return { ok: false, message: "점수를 입력해주세요." };
    }
    const num = Number(trimmed);
    if (!Number.isFinite(num)) {
      return { ok: false, message: "점수는 숫자만 입력 가능합니다." };
    }
    if (num < 0) {
      return { ok: false, message: "점수는 0 이상이어야 합니다." };
    }
    if (num > effectiveMax) {
      return {
        ok: false,
        message: `점수는 ${effectiveMax} 이하여야 합니다.`,
      };
    }
    return { ok: true, value: num };
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();

    const validation = validateValue(valueText);
    if (!validation.ok) {
      setStatus({ kind: "error", message: validation.message });
      return;
    }

    setStatus({ kind: "submitting" });

    const trimmedComment = comment.trim();
    const bodyForCreate = {
      eval_item_id: evalItemId,
      value: validation.value,
      ...(trimmedComment ? { comment: trimmedComment } : {}),
    };
    const bodyForUpdate = {
      value: validation.value,
      ...(trimmedComment ? { comment: trimmedComment } : {}),
    };

    // existing 존재 여부로 분기 — 타입 좁히기를 위해 별도 변수에 캡처
    const editTarget = existing ?? null;
    const url = editTarget
      ? `/api/v1/scores/${encodeURIComponent(editTarget.id)}`
      : "/api/v1/scores";
    const method = editTarget ? "PUT" : "POST";
    const body = editTarget ? bodyForUpdate : bodyForCreate;

    try {
      const response = await fetch(url, {
        method,
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
        cache: "no-store",
      });

      if (!response.ok) {
        const msg = await safeParseErrorMessage(response);
        setStatus({ kind: "error", message: msg });
        return;
      }

      const saved = (await response.json()) as Score;
      setStatus({
        kind: "success",
        message: "점수가 저장되었습니다.",
      });
      onSaved(saved);

      // 신규 입력 모드에서는 폼을 비워 다음 입력을 받을 수 있도록 함
      if (!isEditMode) {
        setValueText("");
        setComment("");
      }
    } catch {
      setStatus({
        kind: "error",
        message: "점수 저장에 실패했습니다.",
      });
    }
  }

  const submitting = status.kind === "submitting";

  return (
    <section
      className="space-y-3 rounded-lg border bg-card p-4"
      aria-labelledby="score-form-heading"
    >
      <header className="flex items-center justify-between">
        <h2 id="score-form-heading" className="text-base font-semibold">
          {isEditMode ? "점수 수정" : "점수 입력"}
        </h2>
        {isEditMode && onCancelEdit !== undefined && (
          <button
            type="button"
            onClick={onCancelEdit}
            className="text-xs text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            취소
          </button>
        )}
      </header>

      <form onSubmit={handleSubmit} className="space-y-3" noValidate>
        <div className="space-y-1">
          <label
            htmlFor="score-value"
            className="text-sm font-medium leading-none"
          >
            점수
          </label>
          <input
            id="score-value"
            type="number"
            inputMode="decimal"
            step="any"
            min={0}
            max={effectiveMax}
            value={valueText}
            onChange={(e) => setValueText(e.target.value)}
            disabled={submitting}
            required
            className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
            aria-describedby="score-value-hint"
          />
          <p
            id="score-value-hint"
            className="text-xs text-muted-foreground"
          >
            0 ~ {effectiveMax} 사이의 숫자
          </p>
        </div>

        <div className="space-y-1">
          <label
            htmlFor="score-comment"
            className="text-sm font-medium leading-none"
          >
            코멘트
          </label>
          <textarea
            id="score-comment"
            rows={3}
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            disabled={submitting}
            className="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
            placeholder="평가 근거를 입력하세요 (선택)"
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

        {status.kind === "success" && (
          <p
            className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700"
            role="status"
            aria-live="polite"
          >
            {status.message}
          </p>
        )}

        <div className="flex justify-end">
          <Button type="submit" disabled={submitting} size="sm">
            {submitting ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
                저장 중...
              </>
            ) : (
              "저장"
            )}
          </Button>
        </div>
      </form>
    </section>
  );
}

/**
 * 백엔드 표준 에러 envelope에서 한국어 메시지를 추출.
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
  if (response.status === 403) return "점수를 저장할 권한이 없습니다.";
  if (response.status === 404) return "평가항목을 찾을 수 없습니다.";
  if (response.status === 409) return "이미 등록된 점수가 있습니다.";
  if (response.status === 422) return "입력값이 올바르지 않습니다.";
  return "점수 저장에 실패했습니다.";
}
