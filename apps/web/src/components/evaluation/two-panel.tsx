"use client";

// @MX:NOTE: 평가항목 두-패널 클라이언트 오케스트레이션 — 트리 선택 상태 공유.
// SPEC-AX-WEB-001 REQ-WEB-003/REQ-WEB-004 — 페이지(RSC)가 초기 items와 role을 주입.

import * as React from "react";

import { ItemDetail } from "@/components/evaluation/item-detail";
import { ItemTree } from "@/components/evaluation/item-tree";
import type { Role } from "@/types/auth";
import type { EvaluationItem } from "@/types/evaluation";

interface TwoPanelProps {
  /** 서버에서 미리 로드한 평탄화된 평가항목 리스트 */
  items: EvaluationItem[];
  /** 현재 사용자 역할 — RoleGate 분기 */
  currentRole: Role;
  /** 초기 선택 ID — 없으면 첫 루트 항목 자동 선택 */
  initialSelectedId?: string;
  /** RSC fetch 실패 시 true — 마운트 후 브라우저 fetch로 자동 재시도 */
  fetchFailed?: boolean;
}

/**
 * 두-패널 레이아웃 클라이언트 컴포넌트.
 *
 * - 좌측: ItemTree (selectedId 단일 출처)
 * - 우측: ItemDetail (selectedId 기반 상세 + 점수 폼/이력)
 *
 * 페이지는 RSC로 인증/초기 fetch를 담당하고, 본 컴포넌트가 선택 상태만 보유.
 */
export function TwoPanel({
  items: initialItems,
  currentRole,
  initialSelectedId,
  fetchFailed,
}: TwoPanelProps): React.ReactElement {
  const [items, setItems] = React.useState<EvaluationItem[]>(initialItems);

  const [selectedId, setSelectedId] = React.useState<string | null>(() => {
    if (initialSelectedId) return initialSelectedId;
    // initialItems의 첫 루트(또는 첫 요소)를 기본 선택해 우측 패널 즉시 활성화
    if (initialItems.length === 0) return null;
    const firstRoot = initialItems.find(
      (it) => it.parent_id === null || it.parent_id === undefined,
    );
    return (firstRoot ?? initialItems[0]!).id;
  });

  // RSC fetch 실패 시 마운트 후 브라우저 fetch로 재시도
  React.useEffect(() => {
    if (!fetchFailed) return;
    let cancelled = false;
    const doFetch = async (): Promise<void> => {
      try {
        const response = await fetch("/api/v1/evaluation-items", {
          method: "GET",
          cache: "no-store",
        });
        if (!response.ok || cancelled) return;
        const data = (await response.json()) as { items?: EvaluationItem[] };
        if (!cancelled) {
          const fetched = data.items ?? [];
          setItems(fetched);
          setSelectedId((prev) => {
            if (prev) return prev;
            const firstRoot = fetched.find(
              (it) => it.parent_id === null || it.parent_id === undefined,
            );
            return (firstRoot ?? fetched[0])?.id ?? null;
          });
        }
      } catch {
        // fetch 실패 — 빈 트리 유지
      }
    };
    void doFetch();
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fetchFailed]);

  // 트리에서 선택된 항목의 fallback 정보를 전달 — 상세 fetch 도중 빈 화면 방지
  const fallbackItem = React.useMemo(() => {
    if (!selectedId) return undefined;
    return items.find((it) => it.id === selectedId);
  }, [items, selectedId]);

  return (
    <div className="grid grid-cols-1 gap-4 lg:grid-cols-[320px_minmax(0,1fr)]">
      <aside
        className="rounded-lg border bg-card p-2"
        aria-label="평가항목 트리"
      >
        <h2 className="px-2 pb-2 pt-1 text-sm font-semibold text-muted-foreground">
          평가항목 트리
        </h2>
        <ItemTree
          items={items}
          selectedId={selectedId}
          onSelect={setSelectedId}
        />
      </aside>

      <div className="min-w-0">
        <ItemDetail
          selectedId={selectedId}
          fallbackItem={fallbackItem}
          currentRole={currentRole}
        />
      </div>
    </div>
  );
}
