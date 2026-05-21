// @MX:NOTE: BFF 리뷰어 배정 프록시 — admin 전용. ReviewCard의 SUBMITTED 액션 버튼이 호출.
// SPEC-AX-WEB-001 Phase E REQ-WEB-006 POST /api/v1/reviews/{id}/assign-reviewer 정합.

import { cookies } from "next/headers";
import { NextResponse, type NextRequest } from "next/server";

import { COOKIE_ACCESS_TOKEN, getBackendBaseUrl } from "@/lib/auth";

/**
 * POST /api/v1/reviews/{id}/assign-reviewer
 *
 * 본문: `{ reviewer_id: string }`.
 * 인가 검증은 백엔드 RBAC가 신뢰의 원천이며, 본 핸들러는 401만 사전 차단한다.
 */
export async function POST(
  request: NextRequest,
  context: { params: Promise<{ id: string }> },
): Promise<Response> {
  const { id } = await context.params;

  if (!/^[A-Za-z0-9_-]{1,64}$/.test(id)) {
    return NextResponse.json(
      {
        error: {
          code: "INVALID_REVIEW_ID",
          message: "잘못된 리뷰 ID 형식입니다.",
        },
      },
      { status: 400 },
    );
  }

  const cookieStore = await cookies();
  const accessToken = cookieStore.get(COOKIE_ACCESS_TOKEN)?.value;
  if (!accessToken) {
    return NextResponse.json(
      { error: { code: "UNAUTHENTICATED", message: "인증이 필요합니다." } },
      { status: 401 },
    );
  }

  // 본문 검증 — reviewer_id는 필수. 미지 필드는 무시.
  let raw: unknown;
  try {
    raw = await request.json();
  } catch {
    return NextResponse.json(
      {
        error: {
          code: "INVALID_BODY",
          message: "JSON 본문을 해석할 수 없습니다.",
        },
      },
      { status: 400 },
    );
  }
  if (raw === null || typeof raw !== "object") {
    return NextResponse.json(
      {
        error: {
          code: "INVALID_BODY",
          message: "본문은 JSON 객체여야 합니다.",
        },
      },
      { status: 400 },
    );
  }
  const record = raw as Record<string, unknown>;
  const reviewerId = record["reviewer_id"];
  if (typeof reviewerId !== "string" || reviewerId.trim() === "") {
    return NextResponse.json(
      {
        error: {
          code: "INVALID_REVIEWER_ID",
          message: "리뷰어 ID를 입력하세요.",
        },
      },
      { status: 400 },
    );
  }

  const targetUrl = new URL(
    `/api/v1/reviews/${encodeURIComponent(id)}/assign-reviewer`,
    getBackendBaseUrl(),
  );

  const upstream = await fetch(targetUrl.toString(), {
    method: "POST",
    headers: {
      Authorization: `Bearer ${accessToken}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ reviewer_id: reviewerId }),
    cache: "no-store",
  });

  return passthroughJson(upstream);
}

async function passthroughJson(upstream: Response): Promise<Response> {
  const status = upstream.status;
  if (status === 204) return new NextResponse(null, { status: 204 });

  const text = await upstream.text();
  if (!text) return NextResponse.json({}, { status });

  try {
    const json: unknown = JSON.parse(text);
    return NextResponse.json(json, { status });
  } catch {
    return NextResponse.json(
      {
        error: {
          code: `HTTP_${status}`,
          message: "백엔드 응답을 해석할 수 없습니다.",
        },
      },
      { status },
    );
  }
}
