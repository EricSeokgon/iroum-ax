"use client";

// @MX:NOTE: 범주 리포트 클라이언트 오케스트레이터 — 선택 상태 보관 + 자식 컴포넌트 조합.
// SPEC-AX-WEB-001 REQ-WEB-005 — page RSC가 사전 로드한 범주 목록을 받아 상호작용 담당.

import * as React from "react";

import { CategorySelector } from "@/components/report/category-selector";
import { ReportView } from "@/components/report/report-view";
import type { EvaluationItem } from "@/types/evaluation";

interface ReportsClientProps {
  /** RSC가 사전 필터링한 루트 범주 목록 */
  categories: ReadonlyArray<EvaluationItem>;
}

/**
 * 페이지 본문 — Selector + Report 조합.
 *
 * 초기 진입 시 선택 없음 → ReportView가 안내 문구만 표시.
 * 범주가 1개뿐인 경우에도 자동 선택하지 않는다 — 사용자가 의도적으로 선택하도록.
 */
export function ReportsClient({
  categories,
}: ReportsClientProps): React.ReactElement {
  const [selectedId, setSelectedId] = React.useState<string | null>(null);

  return (
    <div className="space-y-6">
      <CategorySelector
        categories={categories}
        selectedId={selectedId}
        onSelect={setSelectedId}
      />
      <ReportView categoryId={selectedId} />
    </div>
  );
}
