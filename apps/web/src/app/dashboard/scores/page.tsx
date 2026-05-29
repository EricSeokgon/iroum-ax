// @MX:NOTE: 점수 입력 진입 페이지 — 평가항목 페이지로 안내 (점수는 항목 단위로 입력).
// SPEC-AX-WEB-001 REQ-WEB-004 — 점수는 항상 평가항목과 함께 컨텍스트로 표시된다.

import Link from "next/link";
import { redirect } from "next/navigation";

import { apiFetch, ApiError } from "@/lib/api-client";
import { getServerSession } from "@/lib/auth";
import {
  SCORE_STATUS_LABEL,
  type ScoreListResponse,
  type ScoreStatus,
} from "@/types/evaluation";

/**
 * 점수 입력 페이지.
 *
 * 점수는 항상 평가항목 단위로 입력하는 것이 UX 원칙이므로,
 * 본 페이지는 최근 점수 이력만 보여주고 평가항목 페이지로 안내한다.
 */
export default async function ScoresPage(): Promise<React.ReactElement> {
  const session = await getServerSession();
  if (!session) {
    redirect("/login");
  }

  const initial = await loadRecentScores();

  return (
    <section className="space-y-6">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">점수 입력</h1>
        <p className="text-sm text-muted-foreground">
          점수 입력은 평가항목 페이지에서 항목을 선택한 후 진행해주세요.
        </p>
      </header>

      <div className="rounded-md border bg-muted/30 p-4 text-sm">
        <Link
          href="/dashboard/evaluation-items"
          className="font-medium text-primary underline-offset-2 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          평가항목 페이지로 이동 →
        </Link>
      </div>

      <section className="space-y-2" aria-labelledby="recent-scores-heading">
        <h2
          id="recent-scores-heading"
          className="text-sm font-semibold text-muted-foreground"
        >
          최근 점수
        </h2>

        {initial.kind === "error" ? (
          <p
            className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive"
            role="alert"
            aria-live="polite"
          >
            {initial.message}
          </p>
        ) : initial.data.scores.length === 0 ? (
          <p className="rounded-md border border-dashed px-3 py-4 text-center text-sm text-muted-foreground">
            등록된 점수가 없습니다.
          </p>
        ) : (
          <div className="overflow-x-auto rounded-md border">
            <table className="w-full text-sm">
              <caption className="sr-only">최근 점수 이력</caption>
              <thead className="bg-muted/50">
                <tr>
                  <th scope="col" className="px-3 py-2 text-left font-medium">
                    평가항목 ID
                  </th>
                  <th scope="col" className="px-3 py-2 text-left font-medium">
                    점수
                  </th>
                  <th scope="col" className="px-3 py-2 text-left font-medium">
                    코멘트
                  </th>
                  <th scope="col" className="px-3 py-2 text-left font-medium">
                    상태
                  </th>
                  <th scope="col" className="px-3 py-2 text-left font-medium">
                    등록일
                  </th>
                </tr>
              </thead>
              <tbody>
                {initial.data.scores.map((s) => (
                  <tr key={s.id} className="border-t">
                    <td className="px-3 py-2 break-all font-mono text-xs text-muted-foreground">
                      {s.eval_item_id}
                    </td>
                    <td className="px-3 py-2 font-medium tabular-nums">
                      {s.value}
                    </td>
                    <td className="px-3 py-2 text-muted-foreground">
                      {s.comment ?? "—"}
                    </td>
                    <td className="px-3 py-2">
                      {s.status ? labelOfStatus(s.status) : "—"}
                    </td>
                    <td className="px-3 py-2 text-muted-foreground">
                      {formatDateTime(s.updated_at ?? s.created_at)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </section>
  );
}

type InitialLoadResult =
  | { kind: "ok"; data: ScoreListResponse }
  | { kind: "error"; message: string };

async function loadRecentScores(): Promise<InitialLoadResult> {
  try {
    const data = await apiFetch<ScoreListResponse>(
      "/api/v1/scores?limit=20&offset=0",
    );
    return { kind: "ok", data };
  } catch (err) {
    if (err instanceof ApiError) {
      return {
        kind: "error",
        message:
          err.status === 401
            ? "인증이 만료되었습니다. 다시 로그인해주세요."
            : err.status === 403
              ? "점수를 조회할 권한이 없습니다."
              : err.message,
      };
    }
    return {
      kind: "error",
      message: "점수 이력을 불러오는 중 오류가 발생했습니다.",
    };
  }
}

function labelOfStatus(status: string): string {
  if (status in SCORE_STATUS_LABEL) {
    return SCORE_STATUS_LABEL[status as ScoreStatus];
  }
  return status;
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
