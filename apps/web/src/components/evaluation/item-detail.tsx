"use client";

// @MX:ANCHOR: 평가항목 상세 우측 패널 — page 내 단일 사용처이나 ScoreForm/ScoreList 2개 자식 오케스트레이션 진입점.
// @MX:REASON: 트리 선택 변경 시 detail/scores 재조회 + 권한 게이팅을 한 곳에서 일원화.
// SPEC-AX-WEB-001 REQ-WEB-003/REQ-WEB-004 — 두-패널 레이아웃의 우측.

import * as React from "react";
import { Loader2 } from "lucide-react";

import { RoleGate } from "@/components/auth/role-gate";
import { ScoreForm } from "@/components/evaluation/score-form";
import { ScoreList } from "@/components/evaluation/score-list";
import type { Role } from "@/types/auth";
import type { EvaluationItem, Score } from "@/types/evaluation";

interface ItemDetailProps {
  /** 선택된 평가항목 ID — null이면 비어있음 상태 */
  selectedId: string | null;
  /** 트리에서 이미 로드된 항목 — fallback 표시용 (없으면 detail fetch 결과만 사용) */
  fallbackItem?: EvaluationItem;
  /** 현재 사용자 역할 — 점수 입력 폼 게이팅 */
  currentRole: Role;
}

/**
 * 평가항목 상세 + 점수 이력 + 점수 입력 폼을 묶은 우측 패널.
 *
 * - selectedId 없음: 비어있음 안내
 * - selectedId 변경: GET /api/v1/evaluation-items/{id} 재조회 (트리에 이미 있는 fallbackItem 즉시 표시)
 * - ScoreForm 저장 성공: refreshKey++ → ScoreList 자동 새로고침
 * - 점수 수정: existing 상태로 ScoreForm을 수정 모드로 전환
 */
export function ItemDetail({
  selectedId,
  fallbackItem,
  currentRole,
}: ItemDetailProps): React.ReactElement {
  const [item, setItem] = React.useState<EvaluationItem | null>(
    fallbackItem ?? null,
  );
  const [loading, setLoading] = React.useState<boolean>(false);
  const [errorMessage, setErrorMessage] = React.useState<string | null>(null);
  const [refreshKey, setRefreshKey] = React.useState<number>(0);
  const [editingScore, setEditingScore] = React.useState<Score | null>(null);

  // 선택 변경 시 fallback을 즉시 노출하고 백엔드 상세를 재조회
  React.useEffect(() => {
    if (!selectedId) {
      setItem(null);
      setErrorMessage(null);
      setLoading(false);
      setEditingScore(null);
      return;
    }

    // 트리에 이미 있는 정보로 즉시 1차 표시
    if (fallbackItem && fallbackItem.id === selectedId) {
      setItem(fallbackItem);
    }
    // 항목 전환 시 편집 모드 초기화 — stale 편집 방지
    setEditingScore(null);

    let cancelled = false;
    setLoading(true);
    setErrorMessage(null);

    fetch(`/api/v1/evaluation-items/${encodeURIComponent(selectedId)}`, {
      method: "GET",
      cache: "no-store",
    })
      .then(async (response) => {
        if (cancelled) return;
        if (!response.ok) {
          const msg = await safeParseErrorMessage(response);
          setErrorMessage(msg);
          return;
        }
        const data = (await response.json()) as EvaluationItem;
        setItem(data);
      })
      .catch(() => {
        if (cancelled) return;
        setErrorMessage("평가항목 상세를 불러오는 중 오류가 발생했습니다.");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [selectedId, fallbackItem]);

  if (!selectedId) {
    return (
      <div className="flex h-full min-h-[16rem] items-center justify-center rounded-lg border border-dashed text-sm text-muted-foreground">
        항목을 선택하세요
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <section
        className="space-y-2 rounded-lg border bg-card p-4"
        aria-labelledby="item-detail-heading"
      >
        {loading && item === null ? (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
            <span>평가항목을 불러오는 중입니다...</span>
          </div>
        ) : errorMessage !== null && item === null ? (
          <p
            className="text-sm text-destructive"
            role="alert"
            aria-live="polite"
          >
            {errorMessage}
          </p>
        ) : item !== null ? (
          <>
            <div className="flex items-center gap-2">
              <span className="rounded bg-muted px-2 py-0.5 font-mono text-xs text-muted-foreground">
                {item.code}
              </span>
              {item.weight !== undefined && (
                <span className="text-xs text-muted-foreground">
                  가중치 {formatWeight(item.weight)}
                </span>
              )}
              {item.max_score !== undefined && (
                <span className="text-xs text-muted-foreground">
                  · 만점 {item.max_score}
                </span>
              )}
            </div>
            <h2
              id="item-detail-heading"
              className="text-xl font-semibold tracking-tight"
            >
              {item.name}
            </h2>
            {item.description !== undefined && item.description !== "" && (
              <p className="text-sm text-muted-foreground whitespace-pre-wrap">
                {item.description}
              </p>
            )}
          </>
        ) : null}
      </section>

      <RoleGate currentRole={currentRole} allow={["analyst", "admin"]}>
        <ScoreForm
          evalItemId={selectedId}
          maxScore={item?.max_score}
          existing={editingScore}
          onSaved={() => {
            setRefreshKey((k) => k + 1);
            setEditingScore(null);
          }}
          onCancelEdit={() => setEditingScore(null)}
        />
      </RoleGate>

      <section className="space-y-2" aria-labelledby="score-history-heading">
        <h3
          id="score-history-heading"
          className="text-sm font-semibold text-muted-foreground"
        >
          점수 이력
        </h3>
        <ScoreList
          evalItemId={selectedId}
          refreshKey={refreshKey}
          onEdit={
            currentRole === "analyst" || currentRole === "admin"
              ? (s) => setEditingScore(s)
              : undefined
          }
        />
      </section>
    </div>
  );
}

/**
 * 가중치를 한국어 사용자 친화 형식으로 표시.
 * 0~1 범위는 백분율로, 1 초과는 원본 숫자로 표시 (백엔드 결정에 따라 둘 다 허용).
 */
function formatWeight(weight: number): string {
  if (!Number.isFinite(weight)) return "—";
  if (weight > 0 && weight <= 1) {
    return `${(weight * 100).toFixed(1)}%`;
  }
  return String(weight);
}

async function safeParseErrorMessage(response: Response): Promise<string> {
  try {
    const body = (await response.json()) as {
      error?: { message?: string };
    };
    if (body.error?.message && typeof body.error.message === "string") {
      return body.error.message;
    }
  } catch {
    // ignore
  }
  if (response.status === 401)
    return "인증이 만료되었습니다. 다시 로그인해주세요.";
  if (response.status === 403) return "이 평가항목을 조회할 권한이 없습니다.";
  if (response.status === 404) return "평가항목을 찾을 수 없습니다.";
  return "평가항목 상세를 불러오는 중 오류가 발생했습니다.";
}
