// @MX:NOTE: BFF 증빙 상세 프록시 — 모달이 클릭 시점에 호출.
// SPEC-AX-WEB-001 §1.3 GET /api/v1/evidences/{id} 정합.

import { cookies } from "next/headers";
import { NextResponse, type NextRequest } from "next/server";

import { COOKIE_ACCESS_TOKEN, getBackendBaseUrl } from "@/lib/auth";

/**
 * GET /api/v1/evidences/{id}
 *
 * 동적 세그먼트는 신뢰할 수 없는 사용자 입력이므로 path traversal 방지 차원에서
 * UUID 형식(영문자/숫자/하이픈)만 통과시킨다.
 */
export async function GET(
  _request: NextRequest,
  context: { params: Promise<{ id: string }> },
): Promise<Response> {
  const { id } = await context.params;

  if (!/^[A-Za-z0-9-]{1,64}$/.test(id)) {
    return NextResponse.json(
      {
        error: {
          code: "INVALID_EVIDENCE_ID",
          message: "잘못된 증빙 ID 형식입니다.",
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
    `/api/v1/evidences/${encodeURIComponent(id)}`,
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
