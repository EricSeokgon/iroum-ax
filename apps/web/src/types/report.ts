// @MX:ANCHOR: 범주 리포트 도메인 타입 — page/CategorySelector/ReportView 3개 컴포넌트 공유.
// @MX:REASON: page RSC + CategorySelector(client) + ReportView(client) 동시 참조로 fan_in >= 3.
// SPEC-AX-WEB-001 REQ-WEB-005 §1.3 GET /api/v1/reports/category/{id} 응답 형식 정합.

/**
 * 범주 리포트 단건 응답 — `GET /api/v1/reports/category/{id}`.
 *
 * 가중치 적용 rollup 점수와 등급(grade)을 포함한다.
 * 백엔드는 점수가 0건이어도 200 OK + 빈 items 배열로 응답한다
 * (REQ-WEB-005b: 0건은 오류가 아닌 정상 상태).
 */
export interface CategoryReport {
  /** 범주(루트 평가항목) ID */
  category_id: string;
  /** 범주 한국어 명칭 */
  category_name: string;
  /** 범주 코드 (옵션) — 예: "A", "B" */
  category_code?: string;
  /** 가중치 합계 — 하위 항목 weight의 누적값 */
  total_weight: number;
  /** 가중 평균 점수 — sum(weight * score) / total_weight */
  weighted_score: number;
  /** 등급 — 백엔드가 결정 (예: "A"/"B"/"C"/"D" 또는 "우수"/"양호" 등). UI는 원문 그대로 표시. */
  grade: string;
  /** 하위 평가항목별 가중 점수 — 0건 시 빈 배열 */
  items: CategoryReportItem[];
  /** 리포트 생성 시각 (ISO-8601) */
  generated_at: string;
}

/**
 * 범주 리포트의 개별 항목 행 — CategoryReport.items[] 요소.
 */
export interface CategoryReportItem {
  /** 평가항목 ID */
  eval_item_id: string;
  /** 평가항목 한국어 명칭 */
  eval_item_name: string;
  /** 평가항목 코드 (옵션) */
  eval_item_code?: string;
  /** 적용된 가중치 */
  weight: number;
  /** 원본 점수 — 점수가 등록되지 않은 경우 null/undefined */
  score?: number | null;
  /** 가중 점수 — weight * score, 점수 없을 시 0 또는 undefined */
  weighted_score?: number;
  /** 항목별 상태 (옵션) — 예: "draft"/"submitted"/"approved" */
  status?: string;
}
