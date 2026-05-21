// @MX:NOTE: 증빙 관리 페이지 — RSC가 초기 목록 fetch + 클라이언트 컴포넌트 마운트.
// SPEC-AX-WEB-001 REQ-WEB-002 family — Phase B 본 구현.

import { redirect } from "next/navigation";

import { RoleGate } from "@/components/auth/role-gate";
import { EvidenceList } from "@/components/evidence/evidence-list";
import { EvidenceUpload } from "@/components/evidence/evidence-upload";
import { apiFetch, ApiError } from "@/lib/api-client";
import { getServerSession } from "@/lib/auth";
import type { EvidenceListResponse } from "@/types/evidence";

const INITIAL_LIMIT = 20;

/**
 * 서버 컴포넌트 — 인증 + 초기 목록 fetch를 server-side에서 수행해
 * 첫 페인트에서 깜빡임 없이 데이터를 노출한다.
 */
export default async function EvidencePage(): Promise<React.ReactElement> {
  const session = await getServerSession();
  // layout이 이미 동일 가드를 수행하지만, 본 페이지는 백엔드 호출을 하므로
  // session 정보가 반드시 필요해 2차 방어선을 다시 친다.
  if (!session) {
    redirect("/login");
  }

  const initial = await loadInitialList();

  return (
    <section className="space-y-6">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">증빙 관리</h1>
        <p className="text-sm text-muted-foreground">
          평가 근거가 되는 증빙 파일을 업로드하고 처리 상태를 확인합니다.
        </p>
      </header>

      <RoleGate currentRole={session.role} allow={["analyst", "admin"]}>
        <EvidenceUpload />
      </RoleGate>

      {initial.kind === "error" ? (
        <p
          className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive"
          role="alert"
          aria-live="polite"
        >
          {initial.message}
        </p>
      ) : (
        <EvidenceList initial={initial.data} />
      )}
    </section>
  );
}

type InitialLoadResult =
  | { kind: "ok"; data: EvidenceListResponse }
  | { kind: "error"; message: string };

/**
 * 초기 목록을 서버에서 fetch — 실패해도 페이지 자체는 살아남도록 결과를 객체로 wrap한다.
 * (RSC 전체가 500으로 빠지면 사용자가 업로드 자체를 시도할 수 없음)
 */
async function loadInitialList(): Promise<InitialLoadResult> {
  try {
    const data = await apiFetch<EvidenceListResponse>(
      `/api/v1/evidences?limit=${INITIAL_LIMIT}&offset=0`,
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
              ? "증빙 목록을 조회할 권한이 없습니다."
              : err.message,
      };
    }
    return {
      kind: "error",
      message: "증빙 목록을 불러오는 중 오류가 발생했습니다.",
    };
  }
}
