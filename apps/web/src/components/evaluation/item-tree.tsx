"use client";

// @MX:NOTE: 평가항목 트리 — 평탄 리스트를 parent_id로 계층화한 후 collapsible 렌더.
// SPEC-AX-WEB-001 REQ-WEB-003 (트리 표시) — 모든 역할 읽기 허용.

import * as React from "react";
import { ChevronDown, ChevronRight } from "lucide-react";

import { cn } from "@/lib/utils";
import type { EvaluationItem } from "@/types/evaluation";

interface ItemTreeProps {
  /** 평탄화된 평가항목 리스트 — 백엔드 응답 그대로 */
  items: EvaluationItem[];
  /** 현재 선택된 항목 ID — 하이라이트 표시용 */
  selectedId: string | null;
  /** 항목 클릭 콜백 */
  onSelect: (id: string) => void;
}

/**
 * 트리 노드 — 클라이언트 측에서 재구성된 계층 구조.
 */
interface TreeNode {
  item: EvaluationItem;
  children: TreeNode[];
}

/**
 * 평가항목 트리 (좌측 패널).
 *
 * 백엔드는 평탄화된 리스트를 반환하므로 클라이언트가 parent_id로 트리를 재구성한다.
 * 루트 노드는 parent_id가 null/undefined인 항목들이며, code 오름차순으로 정렬한다.
 *
 * @MX:NOTE: 외부 상태관리 라이브러리 없이 useState로 expand 상태 관리 (PoC 범위).
 */
export function ItemTree({
  items,
  selectedId,
  onSelect,
}: ItemTreeProps): React.ReactElement {
  // 루트 노드 + 모든 비-루트 노드의 ID를 기본적으로 펼쳐 전체 트리가 한 번에 보이도록 함
  // 사용자가 toggle 시 Set에서 제거/추가
  const [expandedIds, setExpandedIds] = React.useState<Set<string>>(() => {
    const initial = new Set<string>();
    for (const item of items) {
      initial.add(item.id);
    }
    return initial;
  });

  // items가 바뀌면 (목록 새로고침) 펼침 상태를 모두 펼치도록 재초기화
  React.useEffect(() => {
    setExpandedIds(new Set(items.map((it) => it.id)));
  }, [items]);

  const roots = React.useMemo(() => buildTree(items), [items]);

  function toggle(id: string): void {
    setExpandedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }

  if (items.length === 0) {
    return (
      <p className="px-3 py-4 text-sm text-muted-foreground">
        등록된 평가항목이 없습니다.
      </p>
    );
  }

  return (
    <ul role="tree" aria-label="평가항목 트리" className="space-y-0.5">
      {roots.map((node) => (
        <TreeRow
          key={node.item.id}
          node={node}
          level={0}
          selectedId={selectedId}
          expandedIds={expandedIds}
          onToggle={toggle}
          onSelect={onSelect}
        />
      ))}
    </ul>
  );
}

interface TreeRowProps {
  node: TreeNode;
  level: number;
  selectedId: string | null;
  expandedIds: Set<string>;
  onToggle: (id: string) => void;
  onSelect: (id: string) => void;
}

function TreeRow({
  node,
  level,
  selectedId,
  expandedIds,
  onToggle,
  onSelect,
}: TreeRowProps): React.ReactElement {
  const hasChildren = node.children.length > 0;
  const isExpanded = expandedIds.has(node.item.id);
  const isSelected = selectedId === node.item.id;
  // 트리 인덴트 — 깊이당 0.75rem 들여쓰기 (시각적 위계 + 좁은 패널 폭 균형)
  const indentStyle: React.CSSProperties = {
    paddingLeft: `${0.5 + level * 0.75}rem`,
  };

  return (
    <li role="treeitem" aria-expanded={hasChildren ? isExpanded : undefined}>
      <div
        className={cn(
          "flex items-center gap-1 rounded-md py-1 pr-2 text-sm transition-colors hover:bg-accent/60",
          isSelected && "bg-accent text-accent-foreground font-medium",
        )}
        style={indentStyle}
      >
        {hasChildren ? (
          <button
            type="button"
            onClick={() => onToggle(node.item.id)}
            className="rounded p-0.5 text-muted-foreground hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            aria-label={isExpanded ? "하위 항목 접기" : "하위 항목 펼치기"}
          >
            {isExpanded ? (
              <ChevronDown className="h-3.5 w-3.5" aria-hidden="true" />
            ) : (
              <ChevronRight className="h-3.5 w-3.5" aria-hidden="true" />
            )}
          </button>
        ) : (
          <span className="inline-block w-4" aria-hidden="true" />
        )}

        <button
          type="button"
          onClick={() => onSelect(node.item.id)}
          className="flex min-w-0 flex-1 items-center gap-2 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:rounded-sm"
          aria-current={isSelected ? "true" : undefined}
        >
          <span className="shrink-0 font-mono text-xs text-muted-foreground">
            {node.item.code}
          </span>
          <span className="truncate" title={node.item.name}>
            {node.item.name}
          </span>
        </button>
      </div>

      {hasChildren && isExpanded && (
        <ul role="group">
          {node.children.map((child) => (
            <TreeRow
              key={child.item.id}
              node={child}
              level={level + 1}
              selectedId={selectedId}
              expandedIds={expandedIds}
              onToggle={onToggle}
              onSelect={onSelect}
            />
          ))}
        </ul>
      )}
    </li>
  );
}

/**
 * 평탄화된 항목 리스트에서 부모-자식 관계로 트리를 재구성.
 *
 * 루트 기준: parent_id가 null/undefined이거나, parent_id가 가리키는 부모가 리스트에 없는 경우
 * (백엔드가 부분 리스트를 반환했을 때 orphan을 루트로 승격해 표시 누락 방지).
 * 각 레벨은 code 오름차순으로 정렬한다.
 */
function buildTree(items: EvaluationItem[]): TreeNode[] {
  // 빠른 lookup을 위한 ID -> 노드 map
  const nodeMap = new Map<string, TreeNode>();
  for (const item of items) {
    nodeMap.set(item.id, { item, children: [] });
  }

  const roots: TreeNode[] = [];

  for (const node of nodeMap.values()) {
    const parentId = node.item.parent_id;
    if (parentId === null || parentId === undefined || parentId === "") {
      roots.push(node);
      continue;
    }
    const parent = nodeMap.get(parentId);
    if (parent) {
      parent.children.push(node);
    } else {
      // orphan (부모가 응답에 없음) — 루트로 승격
      roots.push(node);
    }
  }

  // 각 레벨 정렬 — code 오름차순 (자연순 비교: "A-2"가 "A-10"보다 앞)
  function sortRec(nodes: TreeNode[]): void {
    nodes.sort((a, b) =>
      a.item.code.localeCompare(b.item.code, "ko-KR", { numeric: true }),
    );
    for (const node of nodes) {
      sortRec(node.children);
    }
  }
  sortRec(roots);

  return roots;
}
