"use client";

// @MX:NOTE: 선택된 평가항목의 점수 이력 — 항목 선택 시 GET /api/v1/scores?eval_item_id={id} 호출.
// SPEC-AX-WEB-001 REQ-WEB-004 — 모든 역할 읽기 허용.

import * as React from "react";
import { Loader2, Pencil } from "lucide-react";

import {
  SCORE_STATUS_LABEL,
  type Score,
  type ScoreListResponse,
  type ScoreStatus,
} from "@/types/evaluation";

interface ScoreListProps {
  /** 점수 조회 대상 평가항목 ID */
  evalItemId: string;
  /** 점수 새로고침 트리거 — ScoreForm 저장 성공 시 부모가 증가시킴 */
  refreshKey: number;
  /** 점수 편집 시작 콜백 — analyst/admin에서만 표시됨, 부모가 권한 게이트 */
  onEdit?: (score: Score) => void;
}

/**
 * 선택된 평가항목의 점수 이력 테이블.
 *
 * evalItemId / refreshKey 변경 시 자동 재조회.
 */
export function ScoreList({
  evalItemId,
  refreshKey,
  onEdit,
}: ScoreListProps): React.ReactElement {
  const [scores, setScores] = React.useState<Score[]>([]);
  const [loading, setLoading] = React.useState<boolean>(false);
  const [errorMessage, setErrorMessage] = React.useState<string | null>(null);

  React.useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setErrorMessage(null);

    const params = new URLSearchParams({ eval_item_id: evalItemId });
    fetch(`/api/v1/scores?${params.toString()}`, {
      method: "GET",
      cache: "no-store",
    })
      .then(async (response) => {
        if (cancelled) return;
        if (!response.ok) {
          const msg = await safeParseErrorMessage(response);
          setErrorMessage(msg);
          setScores([]);
          return;
        }
        const data = (await response.json()) as ScoreListResponse;
        setScores(data.scores ?? []);
      })
      .catch(() => {
        if (cancelled) return;
        setErrorMessage("점수 이력을 불러오는 중 오류가 발생했습니다.");
        setScores([]);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [evalItemId, refreshKey]);

  if (loading) {
    return (
      <div className="flex items-center gap-2 px-3 py-4 text-sm text-muted-foreground">
        <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
        <span>점수 이력을 불러오는 중입니다...</span>
      </div>
    );
  }

  if (errorMessage !== null) {
    return (
      <p
        className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive"
        role="alert"
        aria-live="polite"
      >
        {errorMessage}
      </p>
    );
  }

  if (scores.length === 0) {
    return (
      <p className="rounded-md border border-dashed px-3 py-4 text-center text-sm text-muted-foreground">
        등록된 점수가 없습니다.
      </p>
    );
  }

  return (
    <div className="overflow-x-auto rounded-md border">
      <table className="w-full text-sm">
        <caption className="sr-only">점수 이력</caption>
        <thead className="bg-muted/50">
          <tr>
            <th scope="col" className="px-3 py-2 text-left font-medium">
              점수
            </th>
            <th scope="col" className="px-3 py-2 text-left font-medium">
              코멘트
            </th>
            <th scope="col" className="px-3 py-2 text-left font-medium">
              상태
            </th>
            <th scope="col" className="px-3 py-2 text-left font-medium">
              등록일
            </th>
            {onEdit !== undefined && (
              <th scope="col" className="px-3 py-2 text-right font-medium">
                작업
              </th>
            )}
          </tr>
        </thead>
        <tbody>
          {scores.map((s) => (
            <tr key={s.id} className="border-t">
              <td className="px-3 py-2 font-medium tabular-nums">{s.value}</td>
              <td className="px-3 py-2 text-muted-foreground">
                {s.comment ?? "—"}
              </td>
              <td className="px-3 py-2">
                <StatusBadge status={s.status} />
              </td>
              <td className="px-3 py-2 text-muted-foreground">
                {formatDateTime(s.updated_at ?? s.created_at)}
              </td>
              {onEdit !== undefined && (
                <td className="px-3 py-2 text-right">
                  <button
                    type="button"
                    onClick={() => onEdit(s)}
                    className="inline-flex items-center gap-1 rounded px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    aria-label="이 점수 수정"
                  >
                    <Pencil className="h-3.5 w-3.5" aria-hidden="true" />
                    수정
                  </button>
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

interface StatusBadgeProps {
  status?: string;
}

function StatusBadge({ status }: StatusBadgeProps): React.ReactElement {
  if (!status) {
    return <span className="text-muted-foreground">—</span>;
  }
  const known = status in SCORE_STATUS_LABEL;
  const label = known ? SCORE_STATUS_LABEL[status as ScoreStatus] : status;
  const classes =
    status === "approved"
      ? "bg-emerald-100 text-emerald-700"
      : status === "submitted"
        ? "bg-blue-100 text-blue-700"
        : "bg-muted text-muted-foreground";
  return (
    <span
      className={`inline-flex items-center rounded px-2 py-0.5 text-xs font-medium ${classes}`}
    >
      {label}
    </span>
  );
}

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
  if (response.status === 403) return "점수를 조회할 권한이 없습니다.";
  return "점수 이력을 불러오는 중 오류가 발생했습니다.";
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
