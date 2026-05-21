// @MX:ANCHOR: 평가항목/점수 도메인 타입 — page/RSC/client 4+ 컴포넌트 공유.
// @MX:REASON: ItemTree/ItemDetail/ScoreList/ScoreForm + evaluation-items page + scores page fan_in >= 5.
// SPEC-AX-WEB-001 §1.3 evaluation-items + scores endpoints 카탈로그 정합.

/**
 * 평가항목 단일 노드 — `GET /api/v1/evaluation-items` 응답 배열 요소 및
 * `GET /api/v1/evaluation-items/{id}` 단건 응답의 공통 형식.
 *
 * 백엔드는 트리를 평탄화한 리스트로 반환한다. parent_id로 부모 참조,
 * depth/code로 표시 순서를 결정한다.
 */
export interface EvaluationItem {
  /** 항목 고유 ID (UUID v4 또는 백엔드가 부여한 식별자) */
  id: string;
  /** 항목 코드 (예: "A", "A-1", "A-1-1") — 정렬 키로 사용 */
  code: string;
  /** 한국어 항목명 */
  name: string;
  /** 상세 설명 (옵션) */
  description?: string;
  /** 부모 항목 ID — 루트 노드는 null/undefined */
  parent_id?: string | null;
  /** 가중치 (0.0 ~ 1.0 또는 % 단위, 백엔드 결정) */
  weight?: number;
  /** 만점 — 점수 입력 시 클라이언트 측 상한 검증에 사용 */
  max_score?: number;
  /** 트리 깊이 (루트 0) — 인덴트 표시용 */
  depth?: number;
  /** 등록 시각 (ISO-8601) */
  created_at: string;
}

/**
 * 평가항목 목록 응답 — `GET /api/v1/evaluation-items`.
 */
export interface EvaluationItemListResponse {
  items: EvaluationItem[];
  total: number;
}

/**
 * 점수 처리 상태 — 백엔드 enum과 정합.
 * draft/submitted/approved 외 값은 fail-soft로 원문 표시.
 */
export type ScoreStatus = "draft" | "submitted" | "approved";

/**
 * 단일 점수 레코드 — `GET /api/v1/scores/{id}`, `POST /api/v1/scores`,
 * `PUT /api/v1/scores/{id}` 응답 공통 형식 및 목록의 scores[] 요소.
 */
export interface Score {
  /** 점수 고유 ID */
  id: string;
  /** 채점 대상 평가항목 ID — 외래 키 */
  eval_item_id: string;
  /** 점수값 (숫자) */
  value: number;
  /** 평가자 코멘트 (옵션) */
  comment?: string;
  /** 워크플로 상태 */
  status?: string;
  /** 등록 시각 (ISO-8601) */
  created_at: string;
  /** 마지막 수정 시각 (ISO-8601, 옵션) */
  updated_at?: string;
  /** 등록자 sub (옵션) */
  created_by?: string;
}

/**
 * 점수 목록 응답 — `GET /api/v1/scores?eval_item_id={id}`.
 */
export interface ScoreListResponse {
  scores: Score[];
  total: number;
}

/**
 * 점수 생성 요청 본문 — `POST /api/v1/scores`.
 */
export interface CreateScoreRequest {
  eval_item_id: string;
  value: number;
  comment?: string;
}

/**
 * 점수 수정 요청 본문 — `PUT /api/v1/scores/{id}`.
 * eval_item_id는 immutable, value/comment만 갱신.
 */
export interface UpdateScoreRequest {
  value: number;
  comment?: string;
}

/**
 * 한국어 상태 라벨 매핑.
 * SPEC §7.7 i18n 정합 — 단일 ko 언어 지원.
 */
export const SCORE_STATUS_LABEL: Record<ScoreStatus, string> = {
  draft: "초안",
  submitted: "제출됨",
  approved: "승인됨",
};

/**
 * 점수 입력 시 기본 상한 — `max_score`가 항목에 없을 때 사용.
 * SPEC §1.3 score endpoints — 보수적 기본값 100.
 */
export const SCORE_DEFAULT_MAX = 100;
