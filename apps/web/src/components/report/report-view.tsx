"use client";

// @MX:NOTE: 범주 리포트 표시 — categoryId 변경 시 GET /api/v1/reports/category/{id} 호출.
// SPEC-AX-WEB-001 REQ-WEB-005 — 모든 역할 읽기 허용.
// REQ-WEB-005b — items 0건은 오류가 아닌 "데이터 없음" 정상 상태로 표시.

import * as React from "react";
import { Loader2 } from "lucide-react";

import type { CategoryReport, CategoryReportItem } from "@/types/report";

interface ReportViewProps {
  /** 표시할 범주 ID — null이면 안내 문구만 표시 */
  categoryId: string | null;
}

/**
 * 범주 리포트 표시 컴포넌트.
 *
 * categoryId 변경 시 자동 재조회한다. categoryId가 null이면 placeholder만 표시.
 */
export function ReportView({ categoryId }: ReportViewProps): React.ReactElement {
  const [report, setReport] = React.useState<CategoryReport | null>(null);
  const [loading, setLoading] = React.useState<boolean>(false);
  const [errorMessage, setErrorMessage] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (categoryId === null) {
      setReport(null);
      setErrorMessage(null);
      setLoading(false);
      return;
    }

    let cancelled = false;
    setLoading(true);
    setErrorMessage(null);

    fetch(`/api/v1/reports/category/${encodeURIComponent(categoryId)}`, {
      method: "GET",
      cache: "no-store",
    })
      .then(async (response) => {
        if (cancelled) return;
        if (!response.ok) {
          const msg = await safeParseErrorMessage(response);
          setErrorMessage(msg);
          setReport(null);
          return;
        }
        const data = (await response.json()) as CategoryReport;
        setReport(data);
      })
      .catch(() => {
        if (cancelled) return;
        setErrorMessage("리포트를 불러오지 못했습니다.");
        setReport(null);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [categoryId]);

  if (categoryId === null) {
    return (
      <p className="rounded-md border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
        범주를 선택하면 리포트가 표시됩니다.
      </p>
    );
  }

  if (loading) {
    return (
      <div className="flex items-center gap-2 px-3 py-4 text-sm text-muted-foreground">
        <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
        <span>리포트를 불러오는 중입니다...</span>
      </div>
    );
  }

  if (errorMessage !== null) {
    return (
      <p
        className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive"
        role="alert"
        aria-live="polite"
      >
        {errorMessage}
      </p>
    );
  }

  if (report === null) {
    // 토큰 검증 통과 + 백엔드 응답 정상이지만 본문이 비어있는 예외 케이스.
    return (
      <p className="rounded-md border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
        데이터 없음
      </p>
    );
  }

  return (
    <article className="space-y-4" aria-labelledby="report-heading">
      {/* 헤더: 범주명 + 등급 배지 */}
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div className="space-y-1">
          <h2
            id="report-heading"
            className="text-xl font-semibold tracking-tight"
          >
            {report.category_name}
          </h2>
          {report.category_code !== undefined && (
            <p className="text-xs font-mono text-muted-foreground">
              {report.category_code}
            </p>
          )}
        </div>
        <GradeBadge grade={report.grade} />
      </header>

      {/* 요약 카드: 가중 점수 / 전체 가중치 / 생성 일시 */}
      <section
        className="grid grid-cols-1 gap-3 sm:grid-cols-3"
        aria-label="범주 요약"
      >
        <SummaryCard
          label="가중 점수"
          value={formatScore(report.weighted_score)}
          emphasized
        />
        <SummaryCard
          label="전체 가중치"
          value={formatWeight(report.total_weight)}
        />
        <SummaryCard
          label="생성 일시"
          value={formatDateTime(report.generated_at)}
        />
      </section>

      {/* 항목 테이블 또는 0건 빈 상태 (REQ-WEB-005b) */}
      <section aria-label="평가항목별 상세">
        {report.items.length === 0 ? (
          <p className="rounded-md border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
            데이터 없음
          </p>
        ) : (
          <ItemsTable items={report.items} />
        )}
      </section>
    </article>
  );
}

interface GradeBadgeProps {
  grade: string;
}

/**
 * 등급 배지 — A/B/C/D/E 등 백엔드 원문을 그대로 표시한다.
 * 첫 글자(또는 한국어 등급 키워드)로 색상을 결정한다 — 미매칭 시 muted.
 */
function GradeBadge({ grade }: GradeBadgeProps): React.ReactElement {
  const classes = gradeColorClasses(grade);
  return (
    <span
      className={`inline-flex items-center rounded-md px-3 py-1 text-base font-semibold tabular-nums ${classes}`}
      aria-label={`등급 ${grade}`}
    >
      {grade}
    </span>
  );
}

/**
 * 등급 색상 결정 — 백엔드의 grade 원문을 정규화하여 색상 클래스를 반환한다.
 *
 * 매칭 규칙:
 * - "A" 또는 "우수" → emerald(녹색)
 * - "B" → blue(파랑)
 * - "C" 또는 "양호" → yellow(노랑)
 * - "D" → orange(주황)
 * - "E" 또는 "미달" → red(빨강)
 * - 그 외 → muted(중성)
 */
function gradeColorClasses(grade: string): string {
  const head = grade.trim().toUpperCase().charAt(0);
  if (head === "A" || grade.includes("우수")) {
    return "bg-emerald-100 text-emerald-700";
  }
  if (head === "B") {
    return "bg-blue-100 text-blue-700";
  }
  if (head === "C" || grade.includes("양호")) {
    return "bg-yellow-100 text-yellow-700";
  }
  if (head === "D") {
    return "bg-orange-100 text-orange-700";
  }
  if (head === "E" || grade.includes("미달")) {
    return "bg-red-100 text-red-700";
  }
  return "bg-muted text-muted-foreground";
}

interface SummaryCardProps {
  label: string;
  value: string;
  emphasized?: boolean;
}

function SummaryCard({
  label,
  value,
  emphasized,
}: SummaryCardProps): React.ReactElement {
  return (
    <div className="rounded-lg border bg-card p-4 shadow-sm">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p
        className={
          emphasized
            ? "mt-1 text-2xl font-semibold tabular-nums"
            : "mt-1 text-base tabular-nums"
        }
      >
        {value}
      </p>
    </div>
  );
}

interface ItemsTableProps {
  items: ReadonlyArray<CategoryReportItem>;
}

function ItemsTable({ items }: ItemsTableProps): React.ReactElement {
  return (
    <div className="overflow-x-auto rounded-md border">
      <table className="w-full text-sm">
        <caption className="sr-only">평가항목별 가중 점수</caption>
        <thead className="bg-muted/50">
          <tr>
            <th scope="col" className="px-3 py-2 text-left font-medium">
              코드
            </th>
            <th scope="col" className="px-3 py-2 text-left font-medium">
              평가항목명
            </th>
            <th scope="col" className="px-3 py-2 text-right font-medium">
              가중치
            </th>
            <th scope="col" className="px-3 py-2 text-right font-medium">
              점수
            </th>
            <th scope="col" className="px-3 py-2 text-right font-medium">
              가중 점수
            </th>
            <th scope="col" className="px-3 py-2 text-left font-medium">
              상태
            </th>
          </tr>
        </thead>
        <tbody>
          {items.map((item) => (
            <tr key={item.eval_item_id} className="border-t">
              <td className="px-3 py-2 font-mono text-xs text-muted-foreground">
                {item.eval_item_code ?? "—"}
              </td>
              <td className="px-3 py-2">{item.eval_item_name}</td>
              <td className="px-3 py-2 text-right tabular-nums">
                {formatWeight(item.weight)}
              </td>
              <td className="px-3 py-2 text-right tabular-nums">
                {item.score === null || item.score === undefined
                  ? "—"
                  : formatScore(item.score)}
              </td>
              <td className="px-3 py-2 text-right tabular-nums font-medium">
                {item.weighted_score === undefined
                  ? "—"
                  : formatScore(item.weighted_score)}
              </td>
              <td className="px-3 py-2 text-muted-foreground">
                {item.status ?? "—"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/**
 * 백엔드 에러 응답에서 한국어 메시지 추출 fallback.
 * SPEC-AX-WEB-001 REQ-WEB-CROSS-001 — 한국어 message 우선.
 */
async function safeParseErrorMessage(response: Response): Promise<string> {
  try {
    const body = (await response.json()) as {
      error?: { message?: string };
    };
    if (body.error?.message && typeof body.error.message === "string") {
      return body.error.message;
    }
  } catch {
    // ignore parse failure
  }
  if (response.status === 401) {
    return "인증이 만료되었습니다. 다시 로그인해주세요.";
  }
  if (response.status === 403) {
    return "리포트를 조회할 권한이 없습니다.";
  }
  if (response.status === 404) {
    return "해당 범주의 리포트를 찾을 수 없습니다.";
  }
  return "리포트를 불러오지 못했습니다.";
}

function formatScore(value: number): string {
  if (Number.isNaN(value)) return "—";
  // 소수 2자리까지 표시 — 정수면 소수점 생략
  return Number.isInteger(value) ? value.toString() : value.toFixed(2);
}

function formatWeight(value: number): string {
  if (Number.isNaN(value)) return "—";
  return Number.isInteger(value) ? value.toString() : value.toFixed(2);
}

function formatDateTime(iso: string): string {
  try {
    const date = new Date(iso);
    if (Number.isNaN(date.getTime())) return iso;
    return date.toLocaleString("ko-KR", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return iso;
  }
}
