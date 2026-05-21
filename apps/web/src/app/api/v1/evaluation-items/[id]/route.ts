// @MX:NOTE: BFF 평가항목 상세 프록시 — 항목 선택 시 ItemDetail이 호출.
// SPEC-AX-WEB-001 §1.3 GET /api/v1/evaluation-items/{id} 정합.

import { cookies } from "next/headers";
import { NextResponse, type NextRequest } from "next/server";

import { COOKIE_ACCESS_TOKEN, getBackendBaseUrl } from "@/lib/auth";

/**
 * GET /api/v1/evaluation-items/{id}
 *
 * 동적 세그먼트는 path traversal 방지 차원에서
 * 영문/숫자/하이픈/언더스코어 64자 이하로 제한한다.
 */
export async function GET(
  _request: NextRequest,
  context: { params: Promise<{ id: string }> },
): Promise<Response> {
  const { id } = await context.params;

  if (!/^[A-Za-z0-9_-]{1,64}$/.test(id)) {
    return NextResponse.json(
      {
        error: {
          code: "INVALID_ITEM_ID",
          message: "잘못된 평가항목 ID 형식입니다.",
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

  const targetUrl = new URL(
    `/api/v1/evaluation-items/${encodeURIComponent(id)}`,
    getBackendBaseUrl(),
  );

  const upstream = await fetch(targetUrl.toString(), {
    method: "GET",
    headers: { Authorization: `Bearer ${accessToken}` },
    cache: "no-store",
  });

  if (upstream.status === 204) {
    return new NextResponse(null, { status: 204 });
  }

  const text = await upstream.text();
  if (!text) {
    return NextResponse.json({}, { status: upstream.status });
  }
  try {
    const json: unknown = JSON.parse(text);
    return NextResponse.json(json, { status: upstream.status });
  } catch {
    return NextResponse.json(
      {
        error: {
          code: `HTTP_${upstream.status}`,
          message: "백엔드 응답을 해석할 수 없습니다.",
        },
      },
      { status: upstream.status },
    );
  }
}
