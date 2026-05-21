"use client";

// @MX:NOTE: 증빙 목록 — 페이지네이션 + 행 클릭으로 상세 모달 표시.
// SPEC-AX-WEB-001 REQ-WEB-002 family — 목록은 모든 역할 읽기 허용.

import * as React from "react";
import { ChevronLeft, ChevronRight, Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { EvidenceDetailModal } from "@/components/evidence/evidence-detail-modal";
import {
  EVIDENCE_STATUS_LABEL,
  type Evidence,
  type EvidenceListResponse,
  type EvidenceStatus,
} from "@/types/evidence";

/** 페이지당 기본 항목 수 — UX/네트워크 절충 (10~50 권장 범위 중간). */
const DEFAULT_LIMIT = 20;

interface EvidenceListProps {
  /** 서버에서 미리 로드한 초기 목록 — 첫 렌더 깜빡임 제거 */
  initial: EvidenceListResponse;
}

/**
 * 증빙 목록 — RSC에서 초기 데이터를 받고 페이지 변경 시 클라이언트 측에서 재조회.
 *
 * @MX:NOTE: 외부 상태관리 라이브러리를 도입하지 않고 useState로 충분 (PoC 범위).
 */
export function EvidenceList({ initial }: EvidenceListProps): React.ReactElement {
  const [data, setData] = React.useState<EvidenceListResponse>(initial);
  const [loading, setLoading] = React.useState<boolean>(false);
  const [errorMessage, setErrorMessage] = React.useState<string | null>(null);
  const [selectedId, setSelectedId] = React.useState<string | null>(null);

  /**
   * @MX:NOTE: 외부에서 강제 새로고침할 수 있도록 window 이벤트를 구독.
   * Upload 성공 시 dispatch 되는 'iroum-ax:evidence:refresh' 이벤트를 받아 목록 재조회.
   */
  React.useEffect(() => {
    function handleRefresh(): void {
      void fetchPage(0);
    }
    window.addEventListener("iroum-ax:evidence:refresh", handleRefresh);
    return () => {
      window.removeEventListener("iroum-ax:evidence:refresh", handleRefresh);
    };
    // fetchPage는 setState만 호출하므로 의존성에 포함하지 않는다 (effect 재구독 방지)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function fetchPage(offset: number): Promise<void> {
    setLoading(true);
    setErrorMessage(null);
    try {
      const params = new URLSearchParams({
        limit: String(data.limit),
        offset: String(offset),
      });
      const response = await fetch(`/api/v1/evidences?${params.toString()}`, {
        method: "GET",
        cache: "no-store",
      });
      if (!response.ok) {
        const body = (await response.json().catch(() => ({}))) as {
          error?: { message?: string };
        };
        setErrorMessage(
          body.error?.message ?? "증빙 목록을 불러오는 중 오류가 발생했습니다.",
        );
        return;
      }
      const next = (await response.json()) as EvidenceListResponse;
      setData(next);
    } catch {
      setErrorMessage("증빙 목록을 불러오는 중 오류가 발생했습니다.");
    } finally {
      setLoading(false);
    }
  }

  const totalPages = data.limit > 0 ? Math.ceil(data.total / data.limit) : 0;
  const currentPage = data.limit > 0 ? Math.floor(data.offset / data.limit) + 1 : 1;
  const hasPrev = data.offset > 0;
  const hasNext = data.offset + data.limit < data.total;

  const evidences: Evidence[] = data.evidences ?? [];
  const isEmpty = evidences.length === 0;

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

      <div className="overflow-x-auto rounded-md border" aria-busy={loading}>
        <table className="w-full text-sm">
          <caption className="sr-only">증빙 목록</caption>
          <thead className="bg-muted/50">
            <tr>
              <th scope="col" className="px-3 py-2 text-left font-medium">
                파일명
              </th>
              <th scope="col" className="px-3 py-2 text-left font-medium">
                크기
              </th>
              <th scope="col" className="px-3 py-2 text-left font-medium">
                유형
              </th>
              <th scope="col" className="px-3 py-2 text-left font-medium">
                상태
              </th>
              <th scope="col" className="px-3 py-2 text-left font-medium">
                등록일
              </th>
            </tr>
          </thead>
          <tbody>
            {isEmpty && !loading && (
              <tr>
                <td
                  colSpan={5}
                  className="px-3 py-8 text-center text-muted-foreground"
                >
                  등록된 증빙이 없습니다.
                </td>
              </tr>
            )}

            {evidences.map((ev) => (
              <tr
                key={ev.id}
                className="cursor-pointer border-t hover:bg-accent/50 focus-within:bg-accent/50"
                onClick={() => setSelectedId(ev.id)}
              >
                <td className="px-3 py-2">
                  <button
                    type="button"
                    className="text-left underline-offset-2 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    onClick={(e) => {
                      e.stopPropagation();
                      setSelectedId(ev.id);
                    }}
                    aria-label={`${ev.file_name} 상세 보기`}
                  >
                    {ev.file_name}
                  </button>
                </td>
                <td className="px-3 py-2 tabular-nums">
                  {formatBytes(ev.file_size)}
                </td>
                <td className="px-3 py-2 text-muted-foreground">
                  {ev.content_type}
                </td>
                <td className="px-3 py-2">
                  <StatusBadge status={ev.status} />
                </td>
                <td className="px-3 py-2 text-muted-foreground">
                  {formatDateTime(ev.created_at)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="flex items-center justify-between text-sm">
        <p className="text-muted-foreground">
          총 {data.total.toLocaleString("ko-KR")}건
          {totalPages > 0 && (
            <>
              {" "}
              · {currentPage} / {totalPages} 페이지
            </>
          )}
        </p>

        <div className="flex items-center gap-2">
          {loading && (
            <Loader2
              className="h-4 w-4 animate-spin text-muted-foreground"
              aria-label="목록을 불러오는 중"
            />
          )}
          <Button
            variant="outline"
            size="sm"
            type="button"
            disabled={loading || !hasPrev}
            onClick={() =>
              void fetchPage(Math.max(0, data.offset - data.limit))
            }
          >
            <ChevronLeft className="h-4 w-4" aria-hidden="true" />
            이전 페이지
          </Button>
          <Button
            variant="outline"
            size="sm"
            type="button"
            disabled={loading || !hasNext}
            onClick={() => void fetchPage(data.offset + data.limit)}
          >
            다음 페이지
            <ChevronRight className="h-4 w-4" aria-hidden="true" />
          </Button>
        </div>
      </div>

      <EvidenceDetailModal
        evidenceId={selectedId}
        onClose={() => setSelectedId(null)}
      />
    </div>
  );
}

interface StatusBadgeProps {
  status: string;
}

function StatusBadge({ status }: StatusBadgeProps): React.ReactElement {
  const known = status in EVIDENCE_STATUS_LABEL;
  const label = known
    ? EVIDENCE_STATUS_LABEL[status as EvidenceStatus]
    : status;

  const classes =
    status === "processed"
      ? "bg-emerald-100 text-emerald-700"
      : status === "failed"
        ? "bg-destructive/10 text-destructive"
        : "bg-muted text-muted-foreground";

  return (
    <span
      className={`inline-flex items-center rounded px-2 py-0.5 text-xs font-medium ${classes}`}
    >
      {label}
    </span>
  );
}

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return "-";
  if (bytes < 1024) return `${bytes} B`;
  const kb = bytes / 1024;
  if (kb < 1024) return `${kb.toFixed(1)} KB`;
  const mb = kb / 1024;
  return `${mb.toFixed(1)} MB`;
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

export const EVIDENCE_DEFAULT_LIMIT = DEFAULT_LIMIT;
