// @MX:ANCHOR: 리뷰 워크플로 도메인 타입 — KanbanBoard/ReviewCard/ReviewDetail/SubmitReviewForm 공유.
// @MX:REASON: 4개 client 컴포넌트 + RSC page + 5개 BFF route handler가 본 타입을 참조 (fan_in >= 5).
// SPEC-AX-WEB-001 Phase E — REQ-WEB-006 리뷰 워크플로 (Kanban 보드).

/**
 * 리뷰 상태 — 백엔드 SPEC 정합.
 * 4-칼럼 Kanban의 각 칼럼이 한 상태에 대응한다.
 */
export type ReviewStatus =
  | "SUBMITTED"
  | "UNDER_REVIEW"
  | "APPROVED"
  | "REJECTED";

/**
 * 리뷰 단건 — `GET /api/v1/reviews`, `GET /api/v1/reviews/{id}` 응답 형식.
 *
 * 백엔드가 일부 필드를 누락해서 보내도 UI가 깨지지 않도록
 * 식별자(id)와 상태(status)만 필수로 두고 나머지는 모두 optional이다.
 */
export interface Review {
  /** 리뷰 고유 ID */
  id: string;
  /** 현재 상태 */
  status: ReviewStatus;
  /** 리뷰 제목 (옵션) */
  title?: string;
  /** 리뷰 코멘트 (옵션) */
  comment?: string;
  /** 배정된 리뷰어 ID (옵션) — admin assign-reviewer 액션이 설정 */
  reviewer_id?: string;
  /** 제출자 ID (옵션) */
  submitted_by?: string;
  /** 제출 시각 ISO-8601 (옵션) */
  submitted_at?: string;
  /** 승인/반려 처리 시각 ISO-8601 (옵션) */
  reviewed_at?: string;
  /** 레코드 생성 시각 ISO-8601 */
  created_at: string;
  /** 레코드 업데이트 시각 ISO-8601 (옵션) */
  updated_at?: string;
}

/**
 * 리뷰 목록 응답 — `GET /api/v1/reviews`.
 */
export interface ReviewListResponse {
  /** 리뷰 배열 (0건 시 빈 배열) */
  reviews: Review[];
  /** 전체 건수 */
  total: number;
}

/**
 * 리뷰 상태별 한국어 라벨 — Kanban 칼럼 헤더에 사용.
 *
 * @MX:NOTE: 백엔드가 알 수 없는 status를 보낼 경우 호출부에서 원문 fallback 처리.
 */
export const REVIEW_STATUS_LABEL: Record<ReviewStatus, string> = {
  SUBMITTED: "제출됨",
  UNDER_REVIEW: "검토 중",
  APPROVED: "승인됨",
  REJECTED: "반려됨",
};

/**
 * Kanban 칼럼 순서 — 좌→우 진행 순서를 강제.
 *
 * SUBMITTED → UNDER_REVIEW → APPROVED/REJECTED (병렬 종착).
 */
export const REVIEW_STATUS_COLUMNS: ReadonlyArray<ReviewStatus> = [
  "SUBMITTED",
  "UNDER_REVIEW",
  "APPROVED",
  "REJECTED",
];
