// @MX:NOTE: 감사 로그 페이지 — admin 전용 RSC + 초기 데이터 prefetch.
// SPEC-AX-WEB-001 Phase F REQ-WEB-007 — /dashboard/audit-logs.
// 비-admin 접근 시 in-page error 표시 (redirect 대신, URL 유지).

import { redirect } from "next/navigation";

import { AuditLogTable } from "@/components/audit/audit-log-table";
import { apiFetch, ApiError } from "@/lib/api-client";
import { getServerSession } from "@/lib/auth";
import type { AuditLogListResponse } from "@/types/audit";

const INITIAL_PAGE_SIZE = 20;

/**
 * 감사 로그 뷰어 페이지 (admin only).
 *
 * 동작:
 * 1) 세션 가드 — 미인증 시 /login redirect.
 * 2) admin 가드 — 비-admin 사용자는 in-page error로 차단.
 * 3) 초기 감사 로그 목록 사전 로드 — `GET /api/v1/audit-logs?limit=20&offset=0`.
 * 4) 초기 로드 실패 시 빈 테이블 + 에러 메시지로 graceful degrade.
 */
export default async function AuditLogsPage(): Promise<React.ReactElement> {
  const session = await getServerSession();
  if (!session) {
    redirect("/login");
  }

  // RBAC 가드 — 백엔드 RBAC가 신뢰의 원천이지만 UI 단에서도 명시적으로 차단.
  if (session.role !== "admin") {
    return (
      <section className="space-y-4">
        <header className="space-y-1">
          <h1 className="text-2xl font-semibold tracking-tight">감사 로그</h1>
        </header>
        <p
          className="rounded-md border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive"
          role="alert"
          aria-live="polite"
        >
          권한이 없습니다. 관리자만 접근할 수 있습니다.
        </p>
      </section>
    );
  }

  const initial = await loadInitialLogs();

  // 사전 로드 실패 시 빈 데이터를 넘기고 fetchFailed=true로 표시.
  // AuditLogTable이 마운트 후 브라우저 fetch로 자동 재시도한다.
  const initialData =
    initial.kind === "ok"
      ? initial.data
      : { logs: [], total: 0, limit: 20, offset: 0 };
  const fetchFailed = initial.kind === "error";

  return (
    <section className="space-y-6">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">감사 로그</h1>
        <p className="text-sm text-muted-foreground">
          시스템 액션 이력을 검색하고 조회합니다.
        </p>
      </header>

      <AuditLogTable initial={initialData} fetchFailed={fetchFailed} />
    </section>
  );
}

type InitialLoadResult =
  | { kind: "ok"; data: AuditLogListResponse }
  | { kind: "error"; message: string };

/**
 * 초기 감사 로그 목록을 서버에서 fetch — 실패해도 페이지가 살아남도록 결과를 객체로 wrap.
 */
async function loadInitialLogs(): Promise<InitialLoadResult> {
  try {
    const data = await apiFetch<AuditLogListResponse>(
      `/api/v1/audit-logs?limit=${INITIAL_PAGE_SIZE}&offset=0`,
    );
    // 백엔드 응답 구조 검증 — logs 배열 부재 시 구조 불일치로 처리해 client-side 재시도 유도.
    if (!Array.isArray((data as { logs?: unknown }).logs)) {
      return { kind: "error", message: "감사 로그 응답 형식이 올바르지 않습니다." };
    }
    return { kind: "ok", data };
  } catch (err) {
    if (err instanceof ApiError) {
      return {
        kind: "error",
        message:
          err.status === 401
            ? "인증이 만료되었습니다. 다시 로그인해주세요."
            : err.status === 403
              ? "감사 로그를 조회할 권한이 없습니다."
              : err.message,
      };
    }
    return {
      kind: "error",
      message: "감사 로그를 불러오는 중 오류가 발생했습니다.",
    };
  }
}
