// @MX:NOTE: 루브릭 임계값 관리 페이지 — admin 전용 RSC + 초기 데이터 prefetch.
// SPEC-AX-WEB-001 Phase F REQ-WEB-008 — /dashboard/rubric.
// 비-admin 접근 시 in-page error 표시 (redirect 대신, URL 유지).

import { redirect } from "next/navigation";

import { RubricThresholdTable } from "@/components/rubric/rubric-threshold-table";
import { apiFetch, ApiError } from "@/lib/api-client";
import { getServerSession } from "@/lib/auth";
import type { RubricThresholdListResponse } from "@/types/rubric";

/**
 * 루브릭 임계값 관리 페이지 (admin only).
 *
 * 동작:
 * 1) 세션 가드 — 미인증 시 /login redirect.
 * 2) admin 가드 — 비-admin 사용자는 in-page error로 차단.
 * 3) 초기 임계값 목록 사전 로드 — `GET /api/v1/rubric/thresholds`.
 * 4) 초기 로드 실패 시 빈 테이블 + 에러 메시지로 graceful degrade.
 */
export default async function RubricPage(): Promise<React.ReactElement> {
  const session = await getServerSession();
  if (!session) {
    redirect("/login");
  }

  if (session.role !== "admin") {
    return (
      <section className="space-y-4">
        <header className="space-y-1">
          <h1 className="text-2xl font-semibold tracking-tight">
            루브릭 임계값
          </h1>
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

  const initial = await loadInitialThresholds();

  return (
    <section className="space-y-6">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">루브릭 임계값</h1>
        <p className="text-sm text-muted-foreground">
          범위별 등급 임계값을 관리합니다.
        </p>
      </header>

      {initial.kind === "error" ? (
        <p
          className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive"
          role="alert"
          aria-live="polite"
        >
          {initial.message}
        </p>
      ) : (
        <RubricThresholdTable initial={initial.data} />
      )}
    </section>
  );
}

type InitialLoadResult =
  | { kind: "ok"; data: RubricThresholdListResponse }
  | { kind: "error"; message: string };

/**
 * 초기 임계값 목록을 서버에서 fetch — 실패해도 페이지가 살아남도록 결과를 객체로 wrap.
 */
async function loadInitialThresholds(): Promise<InitialLoadResult> {
  try {
    const data = await apiFetch<RubricThresholdListResponse>(
      "/api/v1/rubric/thresholds",
    );
    return { kind: "ok", data };
  } catch (err) {
    if (err instanceof ApiError) {
      return {
        kind: "error",
        message:
          err.status === 401
            ? "인증이 만료되었습니다. 다시 로그인해주세요."
            : err.status === 403
              ? "임계값 목록을 조회할 권한이 없습니다."
              : err.message,
      };
    }
    return {
      kind: "error",
      message: "임계값 목록을 불러오는 중 오류가 발생했습니다.",
    };
  }
}
