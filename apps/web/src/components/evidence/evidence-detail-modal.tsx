"use client";

// @MX:NOTE: 증빙 상세 모달 — 행 클릭 시 GET /api/v1/evidences/{id} 호출.
// SPEC-AX-WEB-001 §1.3 evidences detail endpoint 정합.

import * as React from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { Loader2, X } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  EVIDENCE_STATUS_LABEL,
  type Evidence,
  type EvidenceStatus,
} from "@/types/evidence";

interface EvidenceDetailModalProps {
  /** 모달이 표시되는 동안 유지되는 증빙 ID. null이면 모달 닫힘. */
  evidenceId: string | null;
  /** 사용자가 모달을 닫을 때 호출되는 콜백 */
  onClose: () => void;
}

/**
 * 증빙 상세 모달 — BFF 프록시(`/api/v1/evidences/{id}`)를 통해 백엔드 호출.
 * 토큰은 쿠키에 머무르므로 fetch는 credentials: 'same-origin' (브라우저 기본).
 */
export function EvidenceDetailModal({
  evidenceId,
  onClose,
}: EvidenceDetailModalProps): React.ReactElement {
  const [evidence, setEvidence] = React.useState<Evidence | null>(null);
  const [loading, setLoading] = React.useState<boolean>(false);
  const [errorMessage, setErrorMessage] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!evidenceId) {
      setEvidence(null);
      setErrorMessage(null);
      return;
    }

    let cancelled = false;
    setLoading(true);
    setErrorMessage(null);
    setEvidence(null);

    fetch(`/api/v1/evidences/${encodeURIComponent(evidenceId)}`, {
      method: "GET",
      cache: "no-store",
    })
      .then(async (response) => {
        if (cancelled) return;
        if (!response.ok) {
          const message = await safeParseErrorMessage(response);
          setErrorMessage(message);
          return;
        }
        const data = (await response.json()) as Evidence;
        setEvidence(data);
      })
      .catch(() => {
        if (cancelled) return;
        setErrorMessage("증빙 상세를 불러오는 중 오류가 발생했습니다.");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [evidenceId]);

  const open = evidenceId !== null;

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(next) => {
        if (!next) onClose();
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-50 bg-black/50" />
        <Dialog.Content
          className="fixed left-1/2 top-1/2 z-50 w-full max-w-lg -translate-x-1/2 -translate-y-1/2 rounded-lg border bg-background p-6 shadow-lg focus:outline-none"
          aria-describedby={undefined}
        >
          <div className="flex items-start justify-between">
            <Dialog.Title className="text-lg font-semibold">
              증빙 상세
            </Dialog.Title>
            <Dialog.Close asChild>
              <button
                type="button"
                aria-label="닫기"
                className="rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                <X className="h-4 w-4" />
              </button>
            </Dialog.Close>
          </div>

          <div className="mt-4 min-h-[8rem]">
            {loading && (
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
                <span>증빙 정보를 불러오는 중입니다...</span>
              </div>
            )}

            {!loading && errorMessage !== null && (
              <p
                className="text-sm text-destructive"
                role="alert"
                aria-live="polite"
              >
                {errorMessage}
              </p>
            )}

            {!loading && evidence !== null && (
              <dl className="grid grid-cols-1 gap-2 text-sm sm:grid-cols-[8rem_1fr]">
                <dt className="text-muted-foreground">ID</dt>
                <dd className="break-all font-mono text-xs">{evidence.id}</dd>

                <dt className="text-muted-foreground">파일명</dt>
                <dd className="break-all">{evidence.file_name}</dd>

                <dt className="text-muted-foreground">크기</dt>
                <dd>{formatBytes(evidence.file_size)}</dd>

                <dt className="text-muted-foreground">유형</dt>
                <dd className="break-all">{evidence.content_type}</dd>

                <dt className="text-muted-foreground">상태</dt>
                <dd>{labelFromStatus(evidence.status)}</dd>

                <dt className="text-muted-foreground">등록일</dt>
                <dd>{formatDateTime(evidence.created_at)}</dd>

                {evidence.created_by !== undefined && (
                  <>
                    <dt className="text-muted-foreground">등록자</dt>
                    <dd className="break-all">{evidence.created_by}</dd>
                  </>
                )}
              </dl>
            )}
          </div>

          <div className="mt-6 flex justify-end">
            <Dialog.Close asChild>
              <Button variant="outline" type="button">
                닫기
              </Button>
            </Dialog.Close>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

/**
 * 백엔드 에러 응답에서 사용자 친화적 한국어 메시지를 추출.
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
  if (response.status === 401) return "인증이 만료되었습니다. 다시 로그인해주세요.";
  if (response.status === 403) return "이 증빙을 조회할 권한이 없습니다.";
  if (response.status === 404) return "증빙을 찾을 수 없습니다.";
  return "증빙 정보를 불러오는 중 오류가 발생했습니다.";
}

/**
 * byte 수를 한국어 단위로 포맷.
 * KB/MB는 1024 기반 (IEC binary).
 */
function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return "-";
  if (bytes < 1024) return `${bytes} B`;
  const kb = bytes / 1024;
  if (kb < 1024) return `${kb.toFixed(1)} KB`;
  const mb = kb / 1024;
  return `${mb.toFixed(1)} MB`;
}

/**
 * ISO-8601 문자열을 한국 로컬 시간 형식으로 변환.
 */
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

function labelFromStatus(status: string): string {
  // 백엔드가 알 수 없는 status를 보낼 경우 원문 표시 (fail-soft)
  if (status in EVIDENCE_STATUS_LABEL) {
    return EVIDENCE_STATUS_LABEL[status as EvidenceStatus];
  }
  return status;
}
