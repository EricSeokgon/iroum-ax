// @MX:NOTE: 감사 로그 도메인 타입 — admin 전용 감사 로그 뷰어가 사용.
// SPEC-AX-WEB-001 Phase F REQ-WEB-007 — GET /api/v1/audit-logs 응답 형식.
// 백엔드 SPEC-AX-AUDIT-QUERY-001 의 응답 스키마와 정합.

/**
 * 단일 감사 로그 엔트리.
 * 백엔드는 created_at 기준 내림차순으로 정렬해 반환한다.
 */
export interface AuditLog {
  /** 감사 로그 고유 ID */
  id: string;
  /** 발생 액션 — 예: "SCORE_CREATED", "REVIEW_APPROVED" */
  action: string;
  /** 액션을 수행한 사용자 ID (Keycloak sub) */
  user_id: string;
  /** 대상 리소스 유형 — 예: "Score", "Review" (선택) */
  resource_type?: string;
  /** 대상 리소스 ID (선택) */
  resource_id?: string;
  /** 부가 메타데이터 (액션별 자유 형식) */
  metadata?: Record<string, unknown>;
  /** 생성 일시 (ISO-8601 UTC) */
  created_at: string;
}

/**
 * GET /api/v1/audit-logs 응답 형식.
 * total은 필터 적용 후 전체 건수, limit/offset은 페이지네이션 메타.
 */
export interface AuditLogListResponse {
  logs: AuditLog[];
  total: number;
  limit: number;
  offset: number;
}

/**
 * 감사 로그 조회 필터.
 * 모든 필드는 선택적 — 미지정 시 백엔드 기본 필터(없음) 적용.
 */
export interface AuditLogFilters {
  action?: string;
  user_id?: string;
  resource_type?: string;
  start_time?: string;
  end_time?: string;
  limit?: number;
  offset?: number;
}
