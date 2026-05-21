"use client";

// @MX:NOTE: 증빙 업로드 — drag-drop + 파일 선택 + 100MB 사전 검증.
// SPEC-AX-WEB-001 REQ-WEB-002 family — analyst/admin만 가시 (RoleGate 부모에서 차단).

import * as React from "react";
import { CloudUpload, FileText, Loader2, X } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  EVIDENCE_MAX_FILE_SIZE_BYTES,
  type Evidence,
} from "@/types/evidence";

interface EvidenceUploadProps {
  /** 업로드 성공 후 호출 — 부모가 목록 새로고침 트리거 가능 */
  onUploaded?: (evidence: Evidence) => void;
}

type UploadStatus =
  | { kind: "idle" }
  | { kind: "selected"; file: File }
  | { kind: "uploading"; file: File }
  | { kind: "success"; file: File; evidence: Evidence }
  | { kind: "error"; file: File | null; message: string };

/**
 * 드래그-드롭 + 파일 선택 업로드.
 *
 * 보안 경계 메모:
 *  - 클라이언트 검증은 UX 보조선 (네트워크 낭비 방지).
 *  - 최종 한도/MIME 검증은 백엔드가 신뢰의 원천이다.
 */
export function EvidenceUpload({
  onUploaded,
}: EvidenceUploadProps): React.ReactElement {
  const [status, setStatus] = React.useState<UploadStatus>({ kind: "idle" });
  const [dragOver, setDragOver] = React.useState<boolean>(false);
  const inputRef = React.useRef<HTMLInputElement>(null);

  function pickFile(file: File): void {
    if (file.size > EVIDENCE_MAX_FILE_SIZE_BYTES) {
      setStatus({
        kind: "error",
        file,
        message: "파일 크기는 100MB를 초과할 수 없습니다.",
      });
      return;
    }
    if (file.size === 0) {
      setStatus({
        kind: "error",
        file,
        message: "빈 파일은 업로드할 수 없습니다.",
      });
      return;
    }
    setStatus({ kind: "selected", file });
  }

  function clearSelection(): void {
    setStatus({ kind: "idle" });
    if (inputRef.current) {
      inputRef.current.value = "";
    }
  }

  async function startUpload(file: File): Promise<void> {
    setStatus({ kind: "uploading", file });

    const formData = new FormData();
    formData.set("file", file, file.name);

    try {
      const response = await fetch("/api/v1/evidences", {
        method: "POST",
        body: formData,
        // Content-Type은 브라우저가 multipart boundary와 함께 자동 설정
      });

      if (!response.ok) {
        const message = await readErrorMessage(response);
        setStatus({ kind: "error", file, message });
        return;
      }

      const evidence = (await response.json()) as Evidence;
      setStatus({ kind: "success", file, evidence });
      if (inputRef.current) {
        inputRef.current.value = "";
      }

      // 부모(목록)가 새로고침할 수 있도록 통지
      if (onUploaded) {
        onUploaded(evidence);
      }
      // window 이벤트도 동시에 발행 — list 컴포넌트가 직접 구독
      if (typeof window !== "undefined") {
        window.dispatchEvent(new CustomEvent("iroum-ax:evidence:refresh"));
      }
    } catch {
      setStatus({
        kind: "error",
        file,
        message: "업로드 중 네트워크 오류가 발생했습니다.",
      });
    }
  }

  function handleDrop(event: React.DragEvent<HTMLDivElement>): void {
    event.preventDefault();
    setDragOver(false);
    const file = event.dataTransfer.files?.[0];
    if (file) pickFile(file);
  }

  const uploading = status.kind === "uploading";
  const selectedFile =
    status.kind === "selected" || status.kind === "uploading"
      ? status.file
      : null;

  return (
    <section
      className="space-y-3 rounded-lg border bg-card p-4"
      aria-labelledby="evidence-upload-heading"
    >
      <header className="flex items-center justify-between">
        <h2 id="evidence-upload-heading" className="text-base font-semibold">
          증빙 파일 업로드
        </h2>
        <p className="text-xs text-muted-foreground">최대 100MB</p>
      </header>

      <div
        className={`relative flex flex-col items-center justify-center gap-2 rounded-md border-2 border-dashed p-6 transition-colors ${
          dragOver
            ? "border-primary bg-primary/5"
            : "border-input bg-background"
        }`}
        onDragOver={(e) => {
          e.preventDefault();
          if (!dragOver) setDragOver(true);
        }}
        onDragLeave={() => setDragOver(false)}
        onDrop={handleDrop}
        role="region"
        aria-label="파일 드롭 영역"
      >
        <CloudUpload
          className="h-8 w-8 text-muted-foreground"
          aria-hidden="true"
        />
        <p className="text-center text-sm text-muted-foreground">
          파일을 여기에 드래그하거나 버튼을 클릭하세요
        </p>

        <input
          ref={inputRef}
          type="file"
          className="sr-only"
          aria-label="증빙 파일 선택"
          onChange={(e) => {
            const file = e.target.files?.[0];
            if (file) pickFile(file);
          }}
        />

        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => inputRef.current?.click()}
            disabled={uploading}
          >
            파일 선택
          </Button>
          <Button
            type="button"
            size="sm"
            onClick={() => {
              if (selectedFile) {
                void startUpload(selectedFile);
              }
            }}
            disabled={!selectedFile || uploading}
          >
            {uploading ? (
              <>
                <Loader2
                  className="h-4 w-4 animate-spin"
                  aria-hidden="true"
                />
                업로드 중...
              </>
            ) : (
              "업로드"
            )}
          </Button>
        </div>
      </div>

      {selectedFile !== null && (
        <div className="flex items-center justify-between rounded-md border bg-muted/40 px-3 py-2 text-sm">
          <div className="flex min-w-0 items-center gap-2">
            <FileText className="h-4 w-4 shrink-0" aria-hidden="true" />
            <span className="truncate" title={selectedFile.name}>
              {selectedFile.name}
            </span>
            <span className="shrink-0 text-xs text-muted-foreground">
              {formatBytes(selectedFile.size)}
            </span>
          </div>
          {!uploading && (
            <button
              type="button"
              onClick={clearSelection}
              className="rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              aria-label="선택한 파일 제거"
            >
              <X className="h-4 w-4" />
            </button>
          )}
        </div>
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

      {status.kind === "success" && (
        <p
          className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700"
          role="status"
          aria-live="polite"
        >
          업로드가 완료되었습니다: {status.evidence.file_name}
        </p>
      )}
    </section>
  );
}

/**
 * 백엔드 표준 에러 envelope에서 한국어 메시지를 추출.
 */
async function readErrorMessage(response: Response): Promise<string> {
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
  if (response.status === 403)
    return "증빙을 업로드할 권한이 없습니다.";
  if (response.status === 413)
    return "파일이 너무 큽니다. 100MB 이하로 업로드해주세요.";
  if (response.status === 415)
    return "지원하지 않는 파일 형식입니다.";
  return "업로드 중 오류가 발생했습니다.";
}

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return "-";
  if (bytes < 1024) return `${bytes} B`;
  const kb = bytes / 1024;
  if (kb < 1024) return `${kb.toFixed(1)} KB`;
  const mb = kb / 1024;
  return `${mb.toFixed(1)} MB`;
}
