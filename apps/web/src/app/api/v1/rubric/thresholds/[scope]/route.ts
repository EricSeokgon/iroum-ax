// @MX:NOTE: BFF 루브릭 임계값 update 프록시 — admin 전용 RubricThresholdTable inline edit이 호출.
// SPEC-AX-WEB-001 Phase F REQ-WEB-008 + 백엔드 SPEC-AX-RUBRIC-001 정합.

import { cookies } from "next/headers";
import { NextResponse, type NextRequest } from "next/server";

import { COOKIE_ACCESS_TOKEN, getBackendBaseUrl } from "@/lib/auth";

/**
 * PUT /api/v1/rubric/thresholds/{scope}
 *
 * scope는 path traversal 방지 차원에서 영문/숫자/콜론/하이픈/언더스코어 64자 이하로 제한.
 * (예: "global", "category:A", "category:eval-2024")
 */
export async function PUT(
  request: NextRequest,
  context: { params: Promise<{ scope: string }> },
): Promise<Response> {
  const { scope } = await context.params;

  if (!/^[A-Za-z0-9:_-]{1,64}$/.test(scope)) {
    return NextResponse.json(
      {
        error: {
          code: "INVALID_RUBRIC_SCOPE",
          message: "잘못된 임계값 범위 형식입니다.",
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

  const contentType = request.headers.get("content-type");
  if (!contentType || !contentType.startsWith("application/json")) {
    return NextResponse.json(
      {
        error: {
          code: "INVALID_CONTENT_TYPE",
          message: "application/json 형식의 요청만 허용됩니다.",
        },
      },
      { status: 415 },
    );
  }

  const bodyText = await request.text();
  const targetUrl = new URL(
    `/api/v1/rubric/thresholds/${encodeURIComponent(scope)}`,
    getBackendBaseUrl(),
  );

  const upstream = await fetch(targetUrl.toString(), {
    method: "PUT",
    headers: {
      Authorization: `Bearer ${accessToken}`,
      "Content-Type": "application/json",
    },
    body: bodyText,
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
