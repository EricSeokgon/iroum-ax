"use client";

// @MX:NOTE: 감사 로그 테이블 — admin 전용. 필터 + 페이지네이션 + 읽기 전용.
// SPEC-AX-WEB-001 Phase F REQ-WEB-007 — GET /api/v1/audit-logs 호출 (BFF 경유).
// CSV export는 SPEC §3 비목표 — 어떤 export UI도 노출하지 않는다.

import * as React from "react";
import { Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import type {
  AuditLog,
  AuditLogFilters,
  AuditLogListResponse,
} from "@/types/audit";

interface AuditLogTableProps {
  /** RSC가 사전 로드한 초기 데이터 (필터 없음, offset=0) */
  initial: AuditLogListResponse;
}

const PAGE_SIZE = 20;

type FetchStatus =
  | { kind: "idle" }
  | { kind: "loading" }
  | { kind: "error"; message: string };

/**
 * 감사 로그 뷰어 본체.
 *
 * 동작:
 * 1) 초기에는 RSC가 fetch한 데이터 표시.
 * 2) 필터 입력 후 "조회" 시 클라이언트 fetch — offset=0으로 재로드.
 * 3) "이전"/"다음" 페이지네이션 시 현재 필터 유지하며 offset 변경.
 * 4) 모든 출력은 읽기 전용 — edit/delete UI 없음.
 */
export function AuditLogTable({
  initial,
}: AuditLogTableProps): React.ReactElement {
  // 입력 중인(아직 미적용) 필터 — "조회" 클릭 시에만 적용된다.
  const [draftFilters, setDraftFilters] = React.useState<AuditLogFilters>({});
  // 실제 fetch에 사용 중인 필터 — pagination이 참조.
  const [appliedFilters, setAppliedFilters] = React.useState<AuditLogFilters>(
    {},
  );
  const [offset, setOffset] = React.useState<number>(0);
  const [data, setData] = React.useState<AuditLogListResponse>(initial);
  const [status, setStatus] = React.useState<FetchStatus>({ kind: "idle" });

  /**
   * 필터/offset 조합으로 BFF 호출 — 호출 자체는 한곳에서만.
   * 본 함수는 React state(setData/setStatus)도 함께 갱신해 일관성 보장.
   */
  const loadLogs = React.useCallback(
    async (filters: AuditLogFilters, nextOffset: number): Promise<void> => {
      setStatus({ kind: "loading" });
      try {
        const params = new URLSearchParams();
        if (filters.action) params.set("action", filters.action);
        if (filters.user_id) params.set("user_id", filters.user_id);
        if (filters.resource_type)
          params.set("resource_type", filters.resource_type);
        if (filters.start_time) params.set("start_time", filters.start_time);
        if (filters.end_time) params.set("end_time", filters.end_time);
        params.set("limit", String(PAGE_SIZE));
        params.set("offset", String(nextOffset));

        const response = await fetch(`/api/v1/audit-logs?${params.toString()}`, {
          cache: "no-store",
        });

        if (!response.ok) {
          const message = await readErrorMessage(response);
          setStatus({ kind: "error", message });
          return;
        }

        const body = (await response.json()) as AuditLogListResponse;
        setData(body);
        setStatus({ kind: "idle" });
      } catch {
        setStatus({
          kind: "error",
          message: "감사 로그를 불러오는 중 오류가 발생했습니다.",
        });
      }
    },
    [],
  );

  function handleQuery(event: React.FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    setAppliedFilters(draftFilters);
    setOffset(0);
    void loadLogs(draftFilters, 0);
  }

  function handleReset(): void {
    setDraftFilters({});
    setAppliedFilters({});
    setOffset(0);
    void loadLogs({}, 0);
  }

  function handlePrev(): void {
    const next = Math.max(0, offset - PAGE_SIZE);
    setOffset(next);
    void loadLogs(appliedFilters, next);
  }

  function handleNext(): void {
    const next = offset + PAGE_SIZE;
    setOffset(next);
    void loadLogs(appliedFilters, next);
  }

  const loading = status.kind === "loading";
  const totalPages = Math.max(1, Math.ceil(data.total / PAGE_SIZE));
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1;
  const hasPrev = offset > 0;
  const hasNext = offset + PAGE_SIZE < data.total;

  return (
    <div className="space-y-4">
      <form
        onSubmit={handleQuery}
        className="space-y-3 rounded-lg border bg-card p-4"
        aria-labelledby="audit-log-filter-heading"
      >
        <h2 id="audit-log-filter-heading" className="text-base font-semibold">
          필터
        </h2>

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <div className="space-y-1">
            <label htmlFor="filter-action" className="text-sm font-medium">
              액션
            </label>
            <input
              id="filter-action"
              type="text"
              value={draftFilters.action ?? ""}
              onChange={(e) =>
                setDraftFilters((prev) => ({
                  ...prev,
                  action: e.target.value || undefined,
                }))
              }
              disabled={loading}
              maxLength={100}
              placeholder="예: SCORE_CREATED"
              className="block w-full rounded-md border bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
            />
          </div>

          <div className="space-y-1">
            <label htmlFor="filter-user-id" className="text-sm font-medium">
              사용자 ID
            </label>
            <input
              id="filter-user-id"
              type="text"
              value={draftFilters.user_id ?? ""}
              onChange={(e) =>
                setDraftFilters((prev) => ({
                  ...prev,
                  user_id: e.target.value || undefined,
                }))
              }
              disabled={loading}
              maxLength={100}
              className="block w-full rounded-md border bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
            />
          </div>

          <div className="space-y-1">
            <label htmlFor="filter-start-time" className="text-sm font-medium">
              시작 일시
            </label>
            <input
              id="filter-start-time"
              type="datetime-local"
              value={draftFilters.start_time ?? ""}
              onChange={(e) =>
                setDraftFilters((prev) => ({
                  ...prev,
                  start_time: e.target.value || undefined,
                }))
              }
              disabled={loading}
              className="block w-full rounded-md border bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
            />
          </div>

          <div className="space-y-1">
            <label htmlFor="filter-end-time" className="text-sm font-medium">
              종료 일시
            </label>
            <input
              id="filter-end-time"
              type="datetime-local"
              value={draftFilters.end_time ?? ""}
              onChange={(e) =>
                setDraftFilters((prev) => ({
                  ...prev,
                  end_time: e.target.value || undefined,
                }))
              }
              disabled={loading}
              className="block w-full rounded-md border bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
            />
          </div>
        </div>

        <div className="flex items-center justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            onClick={handleReset}
            disabled={loading}
          >
            초기화
          </Button>
          <Button type="submit" disabled={loading}>
            {loading ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
                조회 중...
              </>
            ) : (
              "조회"
            )}
          </Button>
        </div>
      </form>

      {status.kind === "error" && (
        <p
          className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive"
          role="alert"
          aria-live="polite"
        >
          {status.message}
        </p>
      )}

      <div className="overflow-x-auto rounded-lg border bg-card">
        <table className="min-w-full divide-y text-sm">
          <thead className="bg-muted/50 text-left">
            <tr>
              <th scope="col" className="px-4 py-2 font-medium">
                액션
              </th>
              <th scope="col" className="px-4 py-2 font-medium">
                사용자 ID
              </th>
              <th scope="col" className="px-4 py-2 font-medium">
                리소스 유형
              </th>
              <th scope="col" className="px-4 py-2 font-medium">
                리소스 ID
              </th>
              <th scope="col" className="px-4 py-2 font-medium">
                일시
              </th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {data.logs.length === 0 ? (
              <tr>
                <td
                  colSpan={5}
                  className="px-4 py-6 text-center text-muted-foreground"
                >
                  감사 로그가 없습니다.
                </td>
              </tr>
            ) : (
              data.logs.map((log) => <AuditLogRow key={log.id} log={log} />)
            )}
          </tbody>
        </table>
      </div>

      <div className="flex items-center justify-between text-sm">
        <p className="text-muted-foreground">
          전체 {data.total}건 / {currentPage}페이지 (총 {totalPages}페이지)
        </p>
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={handlePrev}
            disabled={loading || !hasPrev}
          >
            이전
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={handleNext}
            disabled={loading || !hasNext}
          >
            다음
          </Button>
        </div>
      </div>
    </div>
  );
}

/**
 * 단일 감사 로그 행 — 메타데이터 null/undefined 안전 처리.
 */
function AuditLogRow({ log }: { log: AuditLog }): React.ReactElement {
  return (
    <tr className="hover:bg-muted/30">
      <td className="px-4 py-2 font-medium">{log.action}</td>
      <td className="px-4 py-2 font-mono text-xs">{log.user_id}</td>
      <td className="px-4 py-2">{log.resource_type ?? "—"}</td>
      <td className="px-4 py-2 font-mono text-xs">{log.resource_id ?? "—"}</td>
      <td className="px-4 py-2 whitespace-nowrap">
        {formatDateTime(log.created_at)}
      </td>
    </tr>
  );
}

/**
 * ISO-8601 → 사용자 친화적 한국어 시간 표시.
 * 파싱 실패 시 원본 그대로 노출 (사용자가 raw 값을 확인할 수 있도록).
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
      second: "2-digit",
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
  if (response.status === 401) return "인증이 만료되었습니다. 다시 로그인해주세요.";
  if (response.status === 403) return "감사 로그를 조회할 권한이 없습니다.";
  return "감사 로그를 불러오는 중 오류가 발생했습니다.";
}
