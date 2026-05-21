"use client";

// @MX:NOTE: 범주 선택 드롭다운 — 루트 평가항목 목록에서 선택, 선택 변경 시 부모에 통지.
// SPEC-AX-WEB-001 REQ-WEB-005 — 범주는 루트(parent_id=null) 평가항목.

import * as React from "react";

import type { EvaluationItem } from "@/types/evaluation";

interface CategorySelectorProps {
  /** 선택 가능한 루트 범주 목록 — page RSC가 사전 필터링한 리스트 */
  categories: ReadonlyArray<EvaluationItem>;
  /** 현재 선택된 범주 ID — 미선택 시 null */
  selectedId: string | null;
  /** 선택 변경 콜백 */
  onSelect: (categoryId: string) => void;
}

/**
 * 범주(루트 평가항목) 선택기.
 *
 * 단순 native <select>로 구현 — shadcn Select가 의존성에 없어 a11y/키보드는
 * 브라우저 기본 동작에 위임한다. 옵션은 코드 사전순으로 정렬해 사용자가
 * 예측 가능한 순서로 보도록 한다.
 */
export function CategorySelector({
  categories,
  selectedId,
  onSelect,
}: CategorySelectorProps): React.ReactElement {
  const sorted = React.useMemo(
    () => [...categories].sort((a, b) => a.code.localeCompare(b.code, "ko")),
    [categories],
  );

  if (sorted.length === 0) {
    return (
      <p className="rounded-md border border-dashed px-3 py-2 text-sm text-muted-foreground">
        선택할 수 있는 범주가 없습니다.
      </p>
    );
  }

  return (
    <div className="flex flex-col gap-1 sm:max-w-md">
      <label
        htmlFor="report-category-selector"
        className="text-sm font-medium"
      >
        범주 선택
      </label>
      <select
        id="report-category-selector"
        value={selectedId ?? ""}
        onChange={(event) => {
          const next = event.target.value;
          if (next !== "") onSelect(next);
        }}
        className="block w-full rounded-md border bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <option value="" disabled>
          범주를 선택하세요
        </option>
        {sorted.map((category) => (
          <option key={category.id} value={category.id}>
            {category.code} — {category.name}
          </option>
        ))}
      </select>
    </div>
  );
}
