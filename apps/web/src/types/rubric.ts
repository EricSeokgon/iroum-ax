// @MX:NOTE: 루브릭 임계값 도메인 타입 — admin 전용 루브릭 설정 화면이 사용.
// SPEC-AX-WEB-001 Phase F REQ-WEB-008 — /api/v1/rubric/thresholds CRUD 응답 형식.
// 백엔드 SPEC-AX-RUBRIC-001 의 응답 스키마와 정합.

/**
 * 단일 루브릭 임계값 엔트리.
 * scope는 범위 식별자 — 예: "global", "category:A", "category:B".
 * grade_a/b/c/d는 등급별 최저점수.
 */
export interface RubricThreshold {
  /** 임계값 고유 ID */
  id: string;
  /** 적용 범위 — 예: "global", "category:A" */
  scope: string;
  /** A등급 최저점 */
  grade_a: number;
  /** B등급 최저점 */
  grade_b: number;
  /** C등급 최저점 */
  grade_c: number;
  /** D등급 최저점 */
  grade_d: number;
  /** 생성 일시 (ISO-8601 UTC) */
  created_at: string;
  /** 수정 일시 (ISO-8601 UTC, 선택) */
  updated_at?: string;
}

/**
 * GET /api/v1/rubric/thresholds 응답 형식.
 */
export interface RubricThresholdListResponse {
  thresholds: RubricThreshold[];
}

/**
 * POST/PUT 요청 본문 형식.
 * 신규 생성 시 scope 필수, 수정 시에는 URL path에 scope를 포함.
 */
export interface RubricThresholdInput {
  scope: string;
  grade_a: number;
  grade_b: number;
  grade_c: number;
  grade_d: number;
}
