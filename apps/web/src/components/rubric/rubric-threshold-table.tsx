"use client";

// @MX:NOTE: 루브릭 임계값 테이블 — admin 전용. 목록 + inline edit + 생성 폼 통합.
// SPEC-AX-WEB-001 Phase F REQ-WEB-008 — GET/POST/PUT /api/v1/rubric/thresholds.

import * as React from "react";
import { Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { RubricThresholdForm } from "@/components/rubric/rubric-threshold-form";
import type {
  RubricThreshold,
  RubricThresholdListResponse,
} from "@/types/rubric";

interface RubricThresholdTableProps {
  /** RSC가 사전 로드한 초기 목록 */
  initial: RubricThresholdListResponse;
}

type LoadStatus =
  | { kind: "idle" }
  | { kind: "loading" }
  | { kind: "error"; message: string }
  | { kind: "success"; message: string };

/**
 * 루브릭 임계값 관리 본체.
 *
 * 상태:
 * - thresholds: 현재 표시 중인 목록
 * - editingScope: inline edit 중인 행의 scope (null이면 편집 중 아님)
 * - showCreate: 신규 생성 폼 표시 여부
 */
export function RubricThresholdTable({
  initial,
}: RubricThresholdTableProps): React.ReactElement {
  const [thresholds, setThresholds] = React.useState<RubricThreshold[]>(
    initial.thresholds,
  );
  const [editingScope, setEditingScope] = React.useState<string | null>(null);
  const [showCreate, setShowCreate] = React.useState<boolean>(false);
  const [status, setStatus] = React.useState<LoadStatus>({ kind: "idle" });

  /**
   * 목록 새로고침 — 생성/수정 성공 후 호출.
   */
  const refresh = React.useCallback(async (): Promise<void> => {
    setStatus({ kind: "loading" });
    try {
      const response = await fetch("/api/v1/rubric/thresholds", {
        cache: "no-store",
      });
      if (!response.ok) {
        const message = await readErrorMessage(response);
        setStatus({ kind: "error", message });
        return;
      }
      const body = (await response.json()) as RubricThresholdListResponse;
      setThresholds(body.thresholds);
      setStatus({ kind: "idle" });
    } catch {
      setStatus({
        kind: "error",
        message: "임계값 목록을 불러오는 중 오류가 발생했습니다.",
      });
    }
  }, []);

  function handleCreated(): void {
    setShowCreate(false);
    setStatus({ kind: "success", message: "임계값이 저장되었습니다." });
    void refresh();
  }

  function handleUpdated(): void {
    setEditingScope(null);
    setStatus({ kind: "success", message: "임계값이 저장되었습니다." });
    void refresh();
  }

  const loading = status.kind === "loading";

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        {status.kind === "success" && (
          <p
            className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700"
            role="status"
            aria-live="polite"
          >
            {status.message}
          </p>
        )}
        {status.kind === "error" && (
          <p
            className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive"
            role="alert"
            aria-live="polite"
          >
            {status.message}
          </p>
        )}
        <div className="ml-auto">
          {!showCreate && (
            <Button type="button" onClick={() => setShowCreate(true)}>
              + 새 임계값 추가
            </Button>
          )}
        </div>
      </div>

      {showCreate && (
        <RubricThresholdForm
          onCreated={handleCreated}
          onCancel={() => setShowCreate(false)}
        />
      )}

      <div className="overflow-x-auto rounded-lg border bg-card">
        <table className="min-w-full divide-y text-sm">
          <thead className="bg-muted/50 text-left">
            <tr>
              <th scope="col" className="px-4 py-2 font-medium">
                범위(scope)
              </th>
              <th scope="col" className="px-4 py-2 font-medium">
                A등급
              </th>
              <th scope="col" className="px-4 py-2 font-medium">
                B등급
              </th>
              <th scope="col" className="px-4 py-2 font-medium">
                C등급
              </th>
              <th scope="col" className="px-4 py-2 font-medium">
                D등급
              </th>
              <th scope="col" className="px-4 py-2 font-medium">
                수정일
              </th>
              <th scope="col" className="px-4 py-2 font-medium">
                <span className="sr-only">작업</span>
              </th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {thresholds.length === 0 ? (
              <tr>
                <td
                  colSpan={7}
                  className="px-4 py-6 text-center text-muted-foreground"
                >
                  {loading
                    ? "임계값을 불러오는 중..."
                    : "등록된 임계값이 없습니다."}
                </td>
              </tr>
            ) : (
              thresholds.map((t) =>
                editingScope === t.scope ? (
                  <InlineEditRow
                    key={t.id}
                    threshold={t}
                    onCancel={() => setEditingScope(null)}
                    onUpdated={handleUpdated}
                  />
                ) : (
                  <RubricRow
                    key={t.id}
                    threshold={t}
                    onEdit={() => setEditingScope(t.scope)}
                  />
                ),
              )
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

/**
 * 읽기 전용 행 — "수정" 버튼 클릭 시 inline edit row로 전환.
 */
function RubricRow({
  threshold,
  onEdit,
}: {
  threshold: RubricThreshold;
  onEdit: () => void;
}): React.ReactElement {
  return (
    <tr className="hover:bg-muted/30">
      <td className="px-4 py-2 font-medium">{threshold.scope}</td>
      <td className="px-4 py-2">{threshold.grade_a}</td>
      <td className="px-4 py-2">{threshold.grade_b}</td>
      <td className="px-4 py-2">{threshold.grade_c}</td>
      <td className="px-4 py-2">{threshold.grade_d}</td>
      <td className="px-4 py-2 whitespace-nowrap text-muted-foreground">
        {formatDateTime(threshold.updated_at ?? threshold.created_at)}
      </td>
      <td className="px-4 py-2 text-right">
        <Button type="button" variant="outline" size="sm" onClick={onEdit}>
          수정
        </Button>
      </td>
    </tr>
  );
}

/**
 * inline 수정 행 — 4개 number input + 저장/취소 버튼.
 * scope는 PUT path에 고정 — 편집 불가.
 */
function InlineEditRow({
  threshold,
  onCancel,
  onUpdated,
}: {
  threshold: RubricThreshold;
  onCancel: () => void;
  onUpdated: () => void;
}): React.ReactElement {
  const [gradeA, setGradeA] = React.useState<string>(String(threshold.grade_a));
  const [gradeB, setGradeB] = React.useState<string>(String(threshold.grade_b));
  const [gradeC, setGradeC] = React.useState<string>(String(threshold.grade_c));
  const [gradeD, setGradeD] = React.useState<string>(String(threshold.grade_d));
  const [submitting, setSubmitting] = React.useState<boolean>(false);
  const [errorMessage, setErrorMessage] = React.useState<string | null>(null);

  async function handleSave(): Promise<void> {
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
      setErrorMessage("등급별 최저점은 숫자여야 합니다.");
      return;
    }

    setSubmitting(true);
    setErrorMessage(null);
    try {
      const response = await fetch(
        `/api/v1/rubric/thresholds/${encodeURIComponent(threshold.scope)}`,
        {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            scope: threshold.scope,
            grade_a: parsedA,
            grade_b: parsedB,
            grade_c: parsedC,
            grade_d: parsedD,
          }),
        },
      );

      if (!response.ok) {
        const message = await readErrorMessage(response);
        setErrorMessage(message);
        setSubmitting(false);
        return;
      }

      setSubmitting(false);
      onUpdated();
    } catch {
      setErrorMessage("저장에 실패했습니다.");
      setSubmitting(false);
    }
  }

  return (
    <>
      <tr className="bg-muted/20">
        <td className="px-4 py-2 font-medium">{threshold.scope}</td>
        <td className="px-4 py-2">
          <input
            type="number"
            value={gradeA}
            onChange={(e) => setGradeA(e.target.value)}
            disabled={submitting}
            min={0}
            max={100}
            step={1}
            aria-label={`${threshold.scope} A등급 최저점`}
            className="w-20 rounded-md border bg-background px-2 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
          />
        </td>
        <td className="px-4 py-2">
          <input
            type="number"
            value={gradeB}
            onChange={(e) => setGradeB(e.target.value)}
            disabled={submitting}
            min={0}
            max={100}
            step={1}
            aria-label={`${threshold.scope} B등급 최저점`}
            className="w-20 rounded-md border bg-background px-2 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
          />
        </td>
        <td className="px-4 py-2">
          <input
            type="number"
            value={gradeC}
            onChange={(e) => setGradeC(e.target.value)}
            disabled={submitting}
            min={0}
            max={100}
            step={1}
            aria-label={`${threshold.scope} C등급 최저점`}
            className="w-20 rounded-md border bg-background px-2 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
          />
        </td>
        <td className="px-4 py-2">
          <input
            type="number"
            value={gradeD}
            onChange={(e) => setGradeD(e.target.value)}
            disabled={submitting}
            min={0}
            max={100}
            step={1}
            aria-label={`${threshold.scope} D등급 최저점`}
            className="w-20 rounded-md border bg-background px-2 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
          />
        </td>
        <td className="px-4 py-2 whitespace-nowrap text-muted-foreground">
          {formatDateTime(threshold.updated_at ?? threshold.created_at)}
        </td>
        <td className="px-4 py-2">
          <div className="flex items-center justify-end gap-1">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={onCancel}
              disabled={submitting}
            >
              취소
            </Button>
            <Button
              type="button"
              size="sm"
              onClick={() => {
                void handleSave();
              }}
              disabled={submitting}
            >
              {submitting ? (
                <>
                  <Loader2 className="h-3 w-3 animate-spin" aria-hidden="true" />
                  저장 중...
                </>
              ) : (
                "저장"
              )}
            </Button>
          </div>
        </td>
      </tr>
      {errorMessage && (
        <tr>
          <td colSpan={7} className="px-4 py-2">
            <p
              className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive"
              role="alert"
              aria-live="polite"
            >
              {errorMessage}
            </p>
          </td>
        </tr>
      )}
    </>
  );
}

/**
 * ISO-8601 → 사용자 친화적 한국어 시간 표시.
 */
function formatDateTime(iso: string): string {
  try {
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return iso;
    return d.toLocaleString("ko-KR", {
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
    return "임계값을 수정할 권한이 없습니다.";
  if (response.status === 404)
    return "해당 범위의 임계값을 찾을 수 없습니다.";
  return "저장에 실패했습니다.";
}
