// @MX:ANCHOR: 증빙 도메인 타입 — page/RSC/client 컴포넌트가 공유하는 단일 정의.
// @MX:REASON: EvidenceList/Upload/Modal/Page 4개 이상 fan_in 예정 (Phase B 범위).
// SPEC-AX-WEB-001 §1.3 evidence endpoints 카탈로그 정합.

/**
 * 증빙 처리 상태 — 백엔드 enum과 정합.
 * Go control-plane이 신뢰의 원천이며, UI는 라벨링 용도로만 사용.
 */
export type EvidenceStatus = "pending" | "processed" | "failed";

/**
 * 증빙 단일 레코드 — `GET /api/v1/evidences/{id}`, `POST /api/v1/evidences`
 * 그리고 목록 `GET /api/v1/evidences` 응답의 evidences[] 요소 공통 형식.
 */
export interface Evidence {
  /** 증빙 고유 ID (UUID v4) */
  id: string;
  /** 원본 파일명 (사용자 업로드 시점 이름) */
  file_name: string;
  /** 파일 크기(byte) */
  file_size: number;
  /** MIME 타입 (예: application/pdf) */
  content_type: string;
  /** 처리 상태 */
  status: EvidenceStatus;
  /** 등록 시각 (ISO-8601) */
  created_at: string;
  /** 등록자 sub (옵션) */
  created_by?: string;
}

/**
 * 증빙 목록 응답 형식 — `GET /api/v1/evidences?limit=&offset=`.
 */
export interface EvidenceListResponse {
  evidences: Evidence[];
  total: number;
  limit: number;
  offset: number;
}

/**
 * 한국어 상태 라벨 매핑.
 * SPEC §7.7 i18n 정합 — 단일 ko 언어 지원.
 */
export const EVIDENCE_STATUS_LABEL: Record<EvidenceStatus, string> = {
  pending: "대기 중",
  processed: "처리 완료",
  failed: "처리 실패",
};

/**
 * 클라이언트 측 파일 크기 상한 (byte) — 100 MiB.
 * SPEC-AX-WEB-001 REQ-WEB-002 + Go 백엔드 multipart 한도와 정합.
 * 한도를 넘는 업로드는 백엔드 도달 전에 차단해 네트워크 낭비를 방지한다.
 */
export const EVIDENCE_MAX_FILE_SIZE_BYTES = 100 * 1024 * 1024;
