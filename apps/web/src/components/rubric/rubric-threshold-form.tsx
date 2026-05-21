"use client";

// @MX:NOTE: 루브릭 임계값 신규 생성 폼 — admin 전용.
// SPEC-AX-WEB-001 Phase F REQ-WEB-008 — POST /api/v1/rubric/thresholds.
// 등급별 최저점은 A > B > C > D 단조감소를 클라이언트가 1차 검증 (백엔드가 최종).

import * as React from "react";
import { Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import type { RubricThresholdInput } from "@/types/rubric";

interface RubricThresholdFormProps {
  /** 생성 성공 후 호출 — 부모가 목록 새로고침 트리거 */
  onCreated: () => void;
  /** 취소 클릭 시 폼 접기 */
  onCancel: () => void;
}

type SubmitStatus =
  | { kind: "idle" }
  | { kind: "submitting" }
  | { kind: "error"; message: string };

/**
 * 신규 임계값 생성 폼.
 *
 * 검증 정책:
 * - scope: 영문/숫자/콜론/하이픈/언더스코어 1~64자 (path traversal 방지)
 * - grade_a/b/c/d: 정수 0~100, A > B > C > D 단조감소 권장
 *   (단조감소 위반은 경고 표시만 — 백엔드가 최종 결정)
 */
export function RubricThresholdForm({
  onCreated,
  onCancel,
}: RubricThresholdFormProps): React.ReactElement {
  const [scope, setScope] = React.useState<string>("");
  const [gradeA, setGradeA] = React.useState<string>("90");
  const [gradeB, setGradeB] = React.useState<string>("80");
  const [gradeC, setGradeC] = React.useState<string>("70");
  const [gradeD, setGradeD] = React.useState<string>("60");
  const [status, setStatus] = React.useState<SubmitStatus>({ kind: "idle" });

  async function handleSubmit(
    event: React.FormEvent<HTMLFormElement>,
  ): Promise<void> {
    event.preventDefault();

    const trimmedScope = scope.trim();
    if (!/^[A-Za-z0-9:_-]{1,64}$/.test(trimmedScope)) {
      setStatus({
        kind: "error",
        message:
          "범위는 영문/숫자/콜론/하이픈/언더스코어 1~64자만 허용됩니다.",
      });
      return;
    }

    const parsedA = Number(gradeA);
    const parsedB = Number(gradeB);
    const parsedC = Number(gradeC);
    const parsedD = Number(gradeD);
    if (
      !Number.isFinite(parsedA) ||
      !Number.isFinite(parsedB) ||
      !Number.isFinite(parsedC) ||
      !Number.isFinite(parsedD)
    ) {
      setStatus({ kind: "error", message: "등급별 최저점은 숫자여야 합니다." });
      return;
    }

    setStatus({ kind: "submitting" });

    const body: RubricThresholdInput = {
      scope: trimmedScope,
      grade_a: parsedA,
      grade_b: parsedB,
      grade_c: parsedC,
      grade_d: parsedD,
    };

    try {
      const response = await fetch("/api/v1/rubric/thresholds", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });

      if (!response.ok) {
        const message = await readErrorMessage(response);
        setStatus({ kind: "error", message });
        return;
      }

      setStatus({ kind: "idle" });
      onCreated();
    } catch {
      setStatus({
        kind: "error",
        message: "저장에 실패했습니다.",
      });
    }
  }

  const submitting = status.kind === "submitting";

  return (
    <form
      onSubmit={handleSubmit}
      className="space-y-3 rounded-lg border bg-card p-4"
      aria-labelledby="rubric-form-heading"
    >
      <h2 id="rubric-form-heading" className="text-base font-semibold">
        새 임계값 추가
      </h2>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-5">
        <div className="space-y-1 lg:col-span-1">
          <label htmlFor="new-scope" className="text-sm font-medium">
            범위(scope)
          </label>
          <input
            id="new-scope"
            type="text"
            value={scope}
            onChange={(e) => setScope(e.target.value)}
            disabled={submitting}
            required
            maxLength={64}
            placeholder="global / category:A"
            className="block w-full rounded-md border bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
          />
        </div>

        <GradeInput
          id="new-grade-a"
          label="A등급 최저점"
          value={gradeA}
          onChange={setGradeA}
          disabled={submitting}
        />
        <GradeInput
          id="new-grade-b"
          label="B등급 최저점"
          value={gradeB}
          onChange={setGradeB}
          disabled={submitting}
        />
        <GradeInput
          id="new-grade-c"
          label="C등급 최저점"
          value={gradeC}
          onChange={setGradeC}
          disabled={submitting}
        />
        <GradeInput
          id="new-grade-d"
          label="D등급 최저점"
          value={gradeD}
          onChange={setGradeD}
          disabled={submitting}
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
          onClick={onCancel}
          disabled={submitting}
        >
          취소
        </Button>
        <Button type="submit" disabled={submitting}>
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
  );
}

/**
 * 등급별 최저점 number input — 0~100 정수 권장.
 */
function GradeInput({
  id,
  label,
  value,
  onChange,
  disabled,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (next: string) => void;
  disabled: boolean;
}): React.ReactElement {
  return (
    <div className="space-y-1">
      <label htmlFor={id} className="text-sm font-medium">
        {label}
      </label>
      <input
        id={id}
        type="number"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        required
        min={0}
        max={100}
        step={1}
        className="block w-full rounded-md border bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
      />
    </div>
  );
}

/**
 * 백엔드 표준 에러 envelope에서 한국어 메시지를 추출.
 */
async function readErrorMessage(response: Response): Promise<string> {
  try {
    const body = (await response.json()) as { error?: { message?: string } };
    if (body.error?.message && typeof body.error.message === "string") {
      return body.error.message;
    }
  } catch {
    // ignore
  }
  if (response.status === 401)
    return "인증이 만료되었습니다. 다시 로그인해주세요.";
  if (response.status === 403)
    return "임계값을 생성할 권한이 없습니다.";
  if (response.status === 409)
    return "이미 동일한 범위의 임계값이 존재합니다.";
  return "저장에 실패했습니다.";
}
