// @MX:NOTE: BFF 리뷰 반려 프록시 — admin 전용. ReviewCard의 UNDER_REVIEW 액션 버튼이 호출.
// SPEC-AX-WEB-001 Phase E REQ-WEB-006 POST /api/v1/reviews/{id}/reject 정합.

import { cookies } from "next/headers";
import { NextResponse, type NextRequest } from "next/server";

import { COOKIE_ACCESS_TOKEN, getBackendBaseUrl } from "@/lib/auth";

/**
 * POST /api/v1/reviews/{id}/reject
 *
 * 본문: `{ comment?: string }` (선택).
 * 인가 검증은 백엔드 RBAC에 위임.
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

  const body = await safeReadCommentBody(request);

  const targetUrl = new URL(
    `/api/v1/reviews/${encodeURIComponent(id)}/reject`,
    getBackendBaseUrl(),
  );

  const upstream = await fetch(targetUrl.toString(), {
    method: "POST",
    headers: {
      Authorization: `Bearer ${accessToken}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
    cache: "no-store",
  });

  return passthroughJson(upstream);
}

/**
 * 빈 본문 허용 — comment 필드만 화이트리스트.
 */
async function safeReadCommentBody(
  request: NextRequest,
): Promise<{ comment?: string }> {
  try {
    const raw = (await request.json()) as unknown;
    if (raw !== null && typeof raw === "object") {
      const record = raw as Record<string, unknown>;
      if (typeof record["comment"] === "string") {
        return { comment: record["comment"] };
      }
    }
  } catch {
    // 빈 본문은 정상
  }
  return {};
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
